package catalog

import (
	"context"
	"errors"
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
