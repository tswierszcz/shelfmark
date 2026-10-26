package catalog

import (
	"context"
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
// gets its own short-timeout context derived from context.Background().
const processAttemptTimeout = 30 * time.Second

// Queue is an in-memory, bounded worker pool that drives ProcessLibrary
// across libraries concurrently, retrying transient errors a couple of
// times before giving up to "failed". State does not persist across
// process restarts.
type Queue struct {
	store   *Store
	finders []RepoFinder
	topics  TopicsFetcher
	llm     Categorizer
	ch      chan Library
}

// NewQueue constructs a Queue and starts workers goroutines consuming from
// its internal channel.
func NewQueue(store *Store, finders []RepoFinder, topics TopicsFetcher, llm Categorizer, workers int) *Queue {
	q := &Queue{
		store:   store,
		finders: finders,
		topics:  topics,
		llm:     llm,
		ch:      make(chan Library, queueBufferSize),
	}

	for i := 0; i < workers; i++ {
		go q.worker()
	}

	return q
}

// Enqueue sends lib onto the internal channel, implementing Enqueuer. A
// full channel legitimately blocks the caller briefly; that's acceptable
// here rather than adding complexity to avoid it.
func (q *Queue) Enqueue(lib Library) {
	q.ch <- lib
}

func (q *Queue) worker() {
	for lib := range q.ch {
		q.process(lib)
	}
}

// process drives one library through "processing" -> ProcessLibrary,
// retrying transient (non-nil-error) failures up to maxAttempts times
// before marking the library "failed". ProcessLibrary resolving the
// cascade to "needs_manual" internally (nil error) is not a failure and is
// not retried.
func (q *Queue) process(lib Library) {
	ctx := context.Background()

	if err := q.store.UpdateLibraryStatus(ctx, lib.ID, "processing", nil, nil); err != nil {
		return
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(retryBackoff)
		}

		attemptCtx, cancel := context.WithTimeout(ctx, processAttemptTimeout)
		err := ProcessLibrary(attemptCtx, q.store, q.finders, q.topics, q.llm, lib)
		cancel()

		if err == nil {
			return
		}
		lastErr = err
	}

	errMsg := lastErr.Error()
	_ = q.store.UpdateLibraryStatus(ctx, lib.ID, "failed", nil, &errMsg)
}
