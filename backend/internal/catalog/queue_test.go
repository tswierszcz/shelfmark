package catalog

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// countingErrFinder always returns a transient error and counts how many
// times FindRepo was called, letting tests assert the queue's retry count.
type countingErrFinder struct {
	calls atomic.Int32
}

func (f *countingErrFinder) FindRepo(ctx context.Context, group, artifact, version string) (string, bool, error) {
	f.calls.Add(1)
	return "", false, errors.New("transient failure")
}

func pollUntilStatus(t *testing.T, store *Store, libraryID, batchID, wantStatus string, timeout time.Duration) Library {
	t.Helper()
	ctx := context.Background()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
		if err != nil {
			t.Fatalf("ListLibraries failed: %v", err)
		}
		for _, l := range libs {
			if l.ID == libraryID && l.Status == wantStatus {
				return l
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("library %q did not reach status %q within %s", libraryID, wantStatus, timeout)
	return Library{}
}

func TestQueue_RepoFoundAndConfident_ReachesCategorized(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	lib := seedOneLibrary(t, store, "66666666-6666-6666-6666-666666666661",
		NewLibrary{Purl: "pkg:maven/com.example/foo@1.0.0", Name: "foo", Ecosystem: "maven"})

	taxonomy, err := store.ListTaxonomy(context.Background())
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryName := taxonomy[0].Name

	finders := []RepoFinder{&fakeRepoFinder{repo: "example/foo", found: true}}
	topics := &fakeTopicsFetcher{topics: []string{categoryName}}
	llm := &fakeCategorizer{categories: []string{categoryName}, confident: true}

	queue := NewQueue(store, finders, topics, llm, 2)
	queue.Enqueue(lib)

	pollUntilStatus(t, store, lib.ID, lib.UploadBatchID, "categorized", 5*time.Second)
}

func TestQueue_NoRepoFound_ReachesNeedsManual(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	lib := seedOneLibrary(t, store, "66666666-6666-6666-6666-666666666662",
		NewLibrary{Purl: "pkg:maven/com.example/bar@1.0.0", Name: "bar", Ecosystem: "maven"})

	finders := []RepoFinder{&fakeRepoFinder{found: false}}
	topics := &fakeTopicsFetcher{}
	llm := &fakeCategorizer{}

	queue := NewQueue(store, finders, topics, llm, 2)
	queue.Enqueue(lib)

	pollUntilStatus(t, store, lib.ID, lib.UploadBatchID, "needs_manual", 5*time.Second)
}

func TestQueue_TransientFinderError_RetriesThenMarksFailed(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	lib := seedOneLibrary(t, store, "66666666-6666-6666-6666-666666666663",
		NewLibrary{Purl: "pkg:maven/com.example/baz@1.0.0", Name: "baz", Ecosystem: "maven"})

	finder := &countingErrFinder{}
	finders := []RepoFinder{finder}
	topics := &fakeTopicsFetcher{}
	llm := &fakeCategorizer{}

	queue := NewQueue(store, finders, topics, llm, 1)
	queue.Enqueue(lib)

	pollUntilStatus(t, store, lib.ID, lib.UploadBatchID, "failed", 10*time.Second)

	if got := finder.calls.Load(); got != int32(maxAttempts) {
		t.Fatalf("expected %d attempts (initial + retries), got %d", maxAttempts, got)
	}
}

// artifactRoutingFinder resolves each library by its artifact ID, letting
// one batch mix outcomes: a repo found, no repo found, or a transient error.
type artifactRoutingFinder struct {
	repos  map[string]string
	errFor map[string]bool
}

func (f *artifactRoutingFinder) FindRepo(ctx context.Context, group, artifact, version string) (string, bool, error) {
	if f.errFor[artifact] {
		return "", false, errors.New("transient failure")
	}
	repo, ok := f.repos[artifact]
	return repo, ok, nil
}

func TestQueue_BatchSettles_CountsReconcileAndFailureReasonPersists(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	const batchID = "77777777-7777-7777-7777-777777777771"
	created := []NewLibrary{
		{Purl: "pkg:maven/com.example/good@1.0.0", Name: "good", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/unknown@1.0.0", Name: "unknown", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/broken@1.0.0", Name: "broken", Ecosystem: "maven"},
	}
	if err := store.CreateLibraries(ctx, batchID, created); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}
	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}

	taxonomy, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryName := taxonomy[0].Name

	finders := []RepoFinder{&artifactRoutingFinder{
		repos:  map[string]string{"good": "example/good"},
		errFor: map[string]bool{"broken": true},
	}}
	topics := &fakeTopicsFetcher{topics: []string{categoryName}}
	llm := &fakeCategorizer{categories: []string{categoryName}, confident: true}

	queue := NewQueue(store, finders, topics, llm, 2)
	for _, lib := range libs {
		queue.Enqueue(lib)
	}

	wantStatus := map[string]string{"good": "categorized", "unknown": "needs_manual", "broken": "failed"}
	settled := map[string]Library{}
	for _, lib := range libs {
		settled[lib.Name] = pollUntilStatus(t, store, lib.ID, batchID, wantStatus[lib.Name], 10*time.Second)
	}

	progress, err := store.BatchProgress(ctx, batchID)
	if err != nil {
		t.Fatalf("BatchProgress failed: %v", err)
	}
	total := 0
	for status, count := range progress {
		if status != "categorized" && status != "needs_manual" && status != "failed" {
			t.Errorf("batch has %d libraries in non-terminal status %q", count, status)
		}
		total += count
	}
	if total != len(created) {
		t.Fatalf("batch progress counts sum to %d, want %d created libraries (progress: %v)", total, len(created), progress)
	}

	failed := settled["broken"]
	if failed.ErrorDetail == nil {
		t.Fatalf("failed library has no error_detail, want the exhausted-retry reason")
	}
	if !strings.Contains(*failed.ErrorDetail, "transient failure") {
		t.Fatalf("failed library error_detail = %q, want it to contain %q", *failed.ErrorDetail, "transient failure")
	}
	if settled["good"].ErrorDetail != nil {
		t.Fatalf("categorized library has error_detail %q, want nil", *settled["good"].ErrorDetail)
	}
}

// shutdownQueue registers a cleanup that stops q's workers so they don't
// leak into later tests.
func shutdownQueue(t *testing.T, q *Queue) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q.Shutdown(ctx)
	})
}

// successFakes returns topics/LLM fakes that categorize confidently into
// the first seeded category.
func successFakes(t *testing.T, store *Store) (TopicsFetcher, Categorizer) {
	t.Helper()
	taxonomy, err := store.ListTaxonomy(context.Background())
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	name := taxonomy[0].Name
	return &fakeTopicsFetcher{topics: []string{name}}, &fakeCategorizer{categories: []string{name}, confident: true}
}

// panickingFinder panics for one artifact and resolves every other one.
type panickingFinder struct {
	panicFor string
}

func (f *panickingFinder) FindRepo(ctx context.Context, group, artifact, version string) (string, bool, error) {
	if artifact == f.panicFor {
		panic("boom from upstream")
	}
	return "example/" + artifact, true, nil
}

func TestQueue_PanicInPipeline_MarksFailedAndKeepsServing(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	sink := captureLogs(t)

	bad := seedOneLibrary(t, store, "88888888-8888-8888-8888-888888888881",
		NewLibrary{Purl: "pkg:maven/com.example/explode@1.0.0", Name: "explode", Ecosystem: "maven"})
	good := seedOneLibrary(t, store, "88888888-8888-8888-8888-888888888882",
		NewLibrary{Purl: "pkg:maven/com.example/fine@1.0.0", Name: "fine", Ecosystem: "maven"})

	topics, llm := successFakes(t, store)
	// A single worker proves the same goroutine survives the panic.
	queue := NewQueue(store, []RepoFinder{&panickingFinder{panicFor: "explode"}}, topics, llm, 1)
	shutdownQueue(t, queue)

	queue.Enqueue(bad)
	queue.Enqueue(good)

	failed := pollUntilStatus(t, store, bad.ID, bad.UploadBatchID, "failed", 5*time.Second)
	if failed.ErrorDetail == nil || !strings.HasPrefix(*failed.ErrorDetail, "panic:") {
		t.Fatalf("error_detail = %v, want prefix %q", failed.ErrorDetail, "panic:")
	}
	pollUntilStatus(t, store, good.ID, good.UploadBatchID, "categorized", 5*time.Second)

	lines := sink.Find("queue: panic processing library", "library_id", bad.ID)
	if len(lines) != 1 {
		t.Fatalf("expected 1 panic log line for %s, got %d", bad.ID, len(lines))
	}
	if lines[0]["level"] != "ERROR" {
		t.Fatalf("panic log level = %v, want ERROR", lines[0]["level"])
	}
	if stack, _ := lines[0]["stack"].(string); stack == "" {
		t.Fatalf("panic log line has no stack: %v", lines[0])
	}
}

func TestQueue_AttemptFailures_AreLogged(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	sink := captureLogs(t)

	lib := seedOneLibrary(t, store, "88888888-8888-8888-8888-888888888883",
		NewLibrary{Purl: "pkg:maven/com.example/flaky@1.0.0", Name: "flaky", Ecosystem: "maven"})

	queue := NewQueue(store, []RepoFinder{&countingErrFinder{}}, &fakeTopicsFetcher{}, &fakeCategorizer{}, 1)
	shutdownQueue(t, queue)
	queue.Enqueue(lib)

	pollUntilStatus(t, store, lib.ID, lib.UploadBatchID, "failed", 10*time.Second)

	attempts := sink.Find("queue: attempt failed", "library_id", lib.ID)
	if len(attempts) != maxAttempts {
		t.Fatalf("expected %d 'attempt failed' lines, got %d", maxAttempts, len(attempts))
	}
	for _, line := range attempts {
		if line["level"] != "WARN" {
			t.Fatalf("attempt log level = %v, want WARN", line["level"])
		}
		if errText, _ := line["error"].(string); !strings.Contains(errText, "transient failure") {
			t.Fatalf("attempt log error = %v, want it to contain the cause", line["error"])
		}
	}
	failed := sink.Find("queue: library failed", "library_id", lib.ID)
	if len(failed) != 1 {
		t.Fatalf("expected 1 'library failed' line, got %d", len(failed))
	}
	if failed[0]["level"] != "ERROR" {
		t.Fatalf("library failed log level = %v, want ERROR", failed[0]["level"])
	}
}

// ageLibrary rewrites a library's status and pushes its updated_at into
// the past via raw SQL.
func ageLibrary(t *testing.T, store *Store, id, status string, age time.Duration) {
	t.Helper()
	_, err := store.pool.Exec(context.Background(),
		`UPDATE libraries SET status = $1, updated_at = now() - make_interval(secs => $2) WHERE id = $3`,
		status, age.Seconds(), id)
	if err != nil {
		t.Fatalf("aging library failed: %v", err)
	}
}

func TestQueue_Sweep_RequeuesStuckLibraries(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	stuck := seedOneLibrary(t, store, "88888888-8888-8888-8888-888888888884",
		NewLibrary{Purl: "pkg:maven/com.example/stuck@1.0.0", Name: "stuck", Ecosystem: "maven"})
	fresh := seedOneLibrary(t, store, "88888888-8888-8888-8888-888888888885",
		NewLibrary{Purl: "pkg:maven/com.example/fresh@1.0.0", Name: "fresh", Ecosystem: "maven"})
	ageLibrary(t, store, stuck.ID, "processing", 20*time.Minute)

	topics, llm := successFakes(t, store)
	queue := NewQueue(store, []RepoFinder{&fakeRepoFinder{repo: "example/x", found: true}}, topics, llm, 2)
	shutdownQueue(t, queue)

	n, err := queue.Sweep(context.Background(), StuckLibraryAge)
	if err != nil {
		t.Fatalf("Sweep failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("Sweep requeued %d libraries, want 1", n)
	}

	pollUntilStatus(t, store, stuck.ID, stuck.UploadBatchID, "categorized", 5*time.Second)
	// Give a wrongly-enqueued fresh row time to be picked up before checking.
	time.Sleep(300 * time.Millisecond)
	pollUntilStatus(t, store, fresh.ID, fresh.UploadBatchID, "pending", time.Second)
}

// blockingFinder blocks until released or until its ctx is cancelled,
// recording whether it observed cancellation.
type blockingFinder struct {
	started   chan struct{}
	release   chan struct{}
	sawCancel atomic.Bool
}

func newBlockingFinder() *blockingFinder {
	return &blockingFinder{started: make(chan struct{}, 10), release: make(chan struct{})}
}

func (f *blockingFinder) FindRepo(ctx context.Context, group, artifact, version string) (string, bool, error) {
	f.started <- struct{}{}
	select {
	case <-f.release:
		return "example/" + artifact, true, nil
	case <-ctx.Done():
		f.sawCancel.Store(true)
		return "", false, ctx.Err()
	}
}

func TestQueue_Shutdown_WaitsForInFlightThenStops(t *testing.T) {
	t.Run("released before deadline", func(t *testing.T) {
		pool := startTestPostgres(t)
		store := NewStore(pool)

		inFlight := seedOneLibrary(t, store, "99999999-9999-9999-9999-999999999991",
			NewLibrary{Purl: "pkg:maven/com.example/slow@1.0.0", Name: "slow", Ecosystem: "maven"})
		late := seedOneLibrary(t, store, "99999999-9999-9999-9999-999999999992",
			NewLibrary{Purl: "pkg:maven/com.example/late@1.0.0", Name: "late", Ecosystem: "maven"})

		finder := newBlockingFinder()
		topics, llm := successFakes(t, store)
		queue := NewQueue(store, []RepoFinder{finder}, topics, llm, 1)
		shutdownQueue(t, queue)

		queue.Enqueue(inFlight)
		<-finder.started

		go func() {
			time.Sleep(200 * time.Millisecond)
			close(finder.release)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := queue.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown returned %v, want nil", err)
		}
		pollUntilStatus(t, store, inFlight.ID, inFlight.UploadBatchID, "categorized", time.Second)

		queue.Enqueue(late)
		time.Sleep(200 * time.Millisecond)
		pollUntilStatus(t, store, late.ID, late.UploadBatchID, "pending", time.Second)
	})

	t.Run("never released", func(t *testing.T) {
		pool := startTestPostgres(t)
		store := NewStore(pool)

		inFlight := seedOneLibrary(t, store, "99999999-9999-9999-9999-999999999993",
			NewLibrary{Purl: "pkg:maven/com.example/hung@1.0.0", Name: "hung", Ecosystem: "maven"})
		late := seedOneLibrary(t, store, "99999999-9999-9999-9999-999999999994",
			NewLibrary{Purl: "pkg:maven/com.example/later@1.0.0", Name: "later", Ecosystem: "maven"})

		finder := newBlockingFinder()
		topics, llm := successFakes(t, store)
		queue := NewQueue(store, []RepoFinder{finder}, topics, llm, 1)
		shutdownQueue(t, queue)

		queue.Enqueue(inFlight)
		<-finder.started

		const deadline = 300 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()
		start := time.Now()
		err := queue.Shutdown(ctx)
		elapsed := time.Since(start)

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown returned %v, want context.DeadlineExceeded", err)
		}
		if elapsed > deadline+shutdownGrace+500*time.Millisecond {
			t.Fatalf("Shutdown took %s, want at most deadline + %s", elapsed, shutdownGrace)
		}
		if !finder.sawCancel.Load() {
			t.Fatalf("in-flight attempt did not observe cancellation")
		}
		pollUntilStatus(t, store, inFlight.ID, inFlight.UploadBatchID, "processing", time.Second)

		queue.Enqueue(late)
		time.Sleep(200 * time.Millisecond)
		pollUntilStatus(t, store, late.ID, late.UploadBatchID, "pending", time.Second)
	})
}
