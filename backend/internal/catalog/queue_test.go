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
