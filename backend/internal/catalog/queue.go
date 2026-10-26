package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// queueBufferSize is generous since this queue is in-memory only and
// bounded in practice by upload size, not by sustained throughput.
const queueBufferSize = 1000

// maxAttempts is the total number of ProcessLibrary attempts per library
// (the initial attempt plus up to 2 retries) before giving up to
// "failed".
const maxAttempts = 3

// retryBackoff is the fixed delay between retry attempts.
const retryBackoff = 500 * time.Millisecond

// processAttemptTimeout bounds a single ProcessLibrary call. There's no
// per-request context to inherit in a background worker, so each attempt
// gets its own short-timeout context derived from the queue-owned context,
// which Shutdown cancels when its deadline expires.
const processAttemptTimeout = 30 * time.Second

// statusWriteTimeout bounds the terminal "failed" status writes, which use
// a fresh context because the queue context may already be cancelled.
const statusWriteTimeout = 5 * time.Second

// shutdownGrace is how long Shutdown waits for workers to exit after it has
// cancelled the queue context on deadline expiry.
const shutdownGrace = 2 * time.Second

// StuckLibraryAge is the age after which a library still "pending" or
// "processing" is considered orphaned (by a restart, deploy, panic or failed
// status write) and is re-enqueued by the boot sweep.
const StuckLibraryAge = 10 * time.Minute

// Queue is an in-memory, bounded worker pool that drives ProcessLibrary
// across libraries concurrently, retrying transient errors a couple of
// times before giving up to "failed". State does not persist across
// process restarts; the boot Sweep picks up rows orphaned by a restart.
type Queue struct {
	store   *Store
	finders []RepoFinder
	topics  TopicsFetcher
	llm     Categorizer
	ch      chan Library

	// ctx is the parent of every attempt context; cancel aborts in-flight
	// attempts when Shutdown's deadline expires.
	ctx    context.Context
	cancel context.CancelFunc

	// stop is closed by Shutdown to make workers stop taking jobs and the
	// retry loop stop starting attempts. q.ch is never closed.
	stop     chan struct{}
	stopOnce sync.Once

	workers  sync.WaitGroup
	inFlight atomic.Int32
}

// NewQueue constructs a Queue and starts workers goroutines consuming from
// its internal channel.
func NewQueue(store *Store, finders []RepoFinder, topics TopicsFetcher, llm Categorizer, workers int) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &Queue{
		store:   store,
		finders: finders,
		topics:  topics,
		llm:     llm,
		ch:      make(chan Library, queueBufferSize),
		ctx:     ctx,
		cancel:  cancel,
		stop:    make(chan struct{}),
	}

	for i := 0; i < workers; i++ {
		q.workers.Add(1)
		go q.worker()
	}

	return q
}

// Enqueue sends lib onto the internal channel, implementing Enqueuer. A
// full channel legitimately blocks the caller briefly; that's acceptable
// here rather than adding complexity to avoid it. After Shutdown the job is
// dropped (the row stays "pending" for the next boot sweep) instead of
// blocking forever on a channel nobody drains.
func (q *Queue) Enqueue(lib Library) {
	if q.stopped() {
		q.logDropped(lib)
		return
	}
	select {
	case q.ch <- lib:
	case <-q.stop:
		q.logDropped(lib)
	}
}

func (q *Queue) logDropped(lib Library) {
	slog.Default().Warn("queue: enqueue after shutdown, job dropped", libraryAttrs(lib)...)
}

func (q *Queue) stopped() bool {
	select {
	case <-q.stop:
		return true
	default:
		return false
	}
}

// Shutdown stops the workers from taking new jobs and waits for in-flight
// process calls to finish until ctx is done. If ctx expires first, it
// cancels the queue context so in-flight attempts return promptly (their
// libraries stay "processing" for the next boot sweep), waits up to
// shutdownGrace more for the workers to exit, and returns ctx.Err().
func (q *Queue) Shutdown(ctx context.Context) error {
	q.stopOnce.Do(func() { close(q.stop) })

	done := make(chan struct{})
	go func() {
		q.workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		q.cancel()
		slog.Default().Info("queue: shutdown",
			"in_flight_abandoned", 0,
			"pending_in_channel", len(q.ch))
		return nil
	case <-ctx.Done():
	}

	abandoned := q.inFlight.Load()
	q.cancel()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
	}
	slog.Default().Info("queue: shutdown",
		"in_flight_abandoned", abandoned,
		"pending_in_channel", len(q.ch))
	return ctx.Err()
}

// Sweep re-enqueues every library that has been "pending" or "processing"
// for longer than olderThan, stopping early if the queue is shut down. It
// returns the number of libraries re-enqueued.
func (q *Queue) Sweep(ctx context.Context, olderThan time.Duration) (int, error) {
	libs, err := q.store.ListStuckLibraries(ctx, olderThan)
	if err != nil {
		slog.Default().Error("queue: boot sweep failed", "error", err.Error())
		return 0, err
	}

	requeued := 0
	for _, lib := range libs {
		if q.stopped() || ctx.Err() != nil {
			break
		}
		q.Enqueue(lib)
		requeued++
	}

	slog.Default().Info("queue: boot sweep", "requeued", requeued, "stuck", len(libs))
	return requeued, nil
}

func (q *Queue) worker() {
	defer q.workers.Done()
	for {
		// Prefer the stop signal over a ready job.
		if q.stopped() {
			return
		}
		select {
		case <-q.stop:
			return
		case lib := <-q.ch:
			if q.stopped() {
				// Lost the race with Shutdown; the row stays "pending"
				// for the next boot sweep.
				return
			}
			q.process(lib)
		}
	}
}

// process drives one library through "processing" -> ProcessLibrary,
// retrying transient (non-nil-error) failures up to maxAttempts times
// before marking the library "failed". ProcessLibrary resolving the
// cascade to "needs_manual" internally (nil error) is not a failure and is
// not retried. A panic is recovered, logged and recorded as "failed" so one
// bad upstream response cannot crash the server.
func (q *Queue) process(lib Library) {
	q.inFlight.Add(1)
	defer q.inFlight.Add(-1)

	defer func() {
		if rec := recover(); rec != nil {
			slog.Default().Error("queue: panic processing library",
				append(libraryAttrs(lib),
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()))...)
			q.markFailed(lib, fmt.Sprintf("panic: %v", rec))
		}
	}()

	statusCtx, cancelStatus := context.WithTimeout(q.ctx, processAttemptTimeout)
	err := q.store.UpdateLibraryStatus(statusCtx, lib.ID, "processing", nil, nil)
	cancelStatus()
	if err != nil {
		slog.Default().Error("queue: failed to mark library processing",
			append(libraryAttrs(lib), "error", err.Error())...)
		return
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-time.After(retryBackoff):
			case <-q.stop:
				// Shutting down: don't start an attempt we may not finish.
				// The library stays "processing" for the next boot sweep.
				return
			}
		}

		attemptCtx, cancel := context.WithTimeout(q.ctx, processAttemptTimeout)
		err := ProcessLibrary(attemptCtx, q.store, q.finders, q.topics, q.llm, lib)
		cancel()

		if err == nil {
			return
		}
		lastErr = err
		slog.Default().Warn("queue: attempt failed",
			append(libraryAttrs(lib),
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"error", err.Error())...)

		if q.stopped() {
			return
		}
	}

	slog.Default().Error("queue: library failed",
		append(libraryAttrs(lib), "attempts", maxAttempts, "error", lastErr.Error())...)
	q.markFailed(lib, lastErr.Error())
}

// markFailed records lib as "failed" with detail, using a fresh short
// timeout context since the queue context may already be cancelled.
func (q *Queue) markFailed(lib Library, detail string) {
	ctx, cancel := context.WithTimeout(context.Background(), statusWriteTimeout)
	defer cancel()
	if err := q.store.UpdateLibraryStatus(ctx, lib.ID, "failed", nil, &detail); err != nil {
		slog.Default().Error("queue: failed to mark library failed",
			append(libraryAttrs(lib), "error", err.Error())...)
	}
}

func libraryAttrs(lib Library) []any {
	return []any{"library_id", lib.ID, "purl", lib.Purl, "batch_id", lib.UploadBatchID}
}
