package catalog

import (
	"context"
	"errors"
	"testing"
)

// fakeRepoFinder is a test-only RepoFinder that returns a canned result (or
// error) regardless of its arguments.
type fakeRepoFinder struct {
	repo  string
	found bool
	err   error
}

func (f *fakeRepoFinder) FindRepo(ctx context.Context, group, artifact, version string) (string, bool, error) {
	return f.repo, f.found, f.err
}

// fakeTopicsFetcher is a test-only TopicsFetcher returning a canned list
// (or error).
type fakeTopicsFetcher struct {
	topics []string
	err    error
}

func (f *fakeTopicsFetcher) Topics(ctx context.Context, repo string) ([]string, error) {
	return f.topics, f.err
}

// fakeCategorizer is a test-only Categorizer returning a canned result (or
// error).
type fakeCategorizer struct {
	categories []string
	confident  bool
	err        error
}

func (f *fakeCategorizer) Categorize(ctx context.Context, purl string, githubTopics []string, taxonomy []string) ([]string, bool, error) {
	return f.categories, f.confident, f.err
}

func seedOneLibrary(t *testing.T, store *Store, batchID string, lib NewLibrary) Library {
	t.Helper()
	ctx := context.Background()

	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library seeded, got %d", len(libs))
	}
	return libs[0]
}

func TestProcessLibrary_RepoFoundAndConfident_MarksCategorizedWithSplitSources(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	lib := seedOneLibrary(t, store, "55555555-5555-5555-5555-555555555551",
		NewLibrary{Purl: "pkg:maven/com.example/foo@1.0.0", Name: "foo", Ecosystem: "maven"})

	taxonomy, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	if len(taxonomy) < 2 {
		t.Fatalf("expected at least 2 seeded categories, got %d", len(taxonomy))
	}
	// Pick two real taxonomy category names: one that will match a GitHub
	// topic (source "github"), one that won't (source "llm").
	githubSourcedName := taxonomy[0].Name
	llmSourcedName := taxonomy[1].Name

	finders := []RepoFinder{&fakeRepoFinder{repo: "example/foo", found: true}}
	topics := &fakeTopicsFetcher{topics: []string{githubSourcedName}}
	llm := &fakeCategorizer{categories: []string{githubSourcedName, llmSourcedName}, confident: true}

	if err := ProcessLibrary(ctx, store, finders, topics, llm, lib); err != nil {
		t.Fatalf("ProcessLibrary failed: %v", err)
	}

	got, err := store.ListLibraries(ctx, LibraryFilter{BatchID: lib.UploadBatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 library, got %d", len(got))
	}
	if got[0].Status != "categorized" {
		t.Fatalf("expected status %q, got %q", "categorized", got[0].Status)
	}
	if got[0].GithubRepo == nil || *got[0].GithubRepo != "example/foo" {
		t.Fatalf("expected github_repo %q, got %+v", "example/foo", got[0].GithubRepo)
	}

	rows, err := pool.Query(ctx,
		`SELECT c.name, lc.source FROM library_categories lc JOIN categories c ON c.id = lc.category_id WHERE lc.library_id = $1`,
		lib.ID,
	)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	sources := map[string]string{}
	for rows.Next() {
		var name, source string
		if err := rows.Scan(&name, &source); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		sources[name] = source
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	if sources[githubSourcedName] != "github" {
		t.Errorf("expected %q to be sourced \"github\", got %q", githubSourcedName, sources[githubSourcedName])
	}
	if sources[llmSourcedName] != "llm" {
		t.Errorf("expected %q to be sourced \"llm\", got %q", llmSourcedName, sources[llmSourcedName])
	}
}

func TestProcessLibrary_NoRepoFound_MarksNeedsManual(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	lib := seedOneLibrary(t, store, "55555555-5555-5555-5555-555555555552",
		NewLibrary{Purl: "pkg:maven/com.example/bar@1.0.0", Name: "bar", Ecosystem: "maven"})

	finders := []RepoFinder{
		&fakeRepoFinder{found: false},
		&fakeRepoFinder{found: false},
	}
	topics := &fakeTopicsFetcher{}
	llm := &fakeCategorizer{}

	if err := ProcessLibrary(ctx, store, finders, topics, llm, lib); err != nil {
		t.Fatalf("ProcessLibrary failed: %v", err)
	}

	got, err := store.ListLibraries(ctx, LibraryFilter{BatchID: lib.UploadBatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(got) != 1 || got[0].Status != "needs_manual" {
		t.Fatalf("expected status %q, got %+v", "needs_manual", got)
	}
}

func TestProcessLibrary_RepoFoundButNotConfident_MarksNeedsManual(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	lib := seedOneLibrary(t, store, "55555555-5555-5555-5555-555555555553",
		NewLibrary{Purl: "pkg:maven/com.example/baz@1.0.0", Name: "baz", Ecosystem: "maven"})

	finders := []RepoFinder{&fakeRepoFinder{repo: "example/baz", found: true}}
	topics := &fakeTopicsFetcher{topics: []string{"some-topic"}}
	llm := &fakeCategorizer{categories: nil, confident: false}

	if err := ProcessLibrary(ctx, store, finders, topics, llm, lib); err != nil {
		t.Fatalf("ProcessLibrary failed: %v", err)
	}

	got, err := store.ListLibraries(ctx, LibraryFilter{BatchID: lib.UploadBatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(got) != 1 || got[0].Status != "needs_manual" {
		t.Fatalf("expected status %q, got %+v", "needs_manual", got)
	}
	// Repo was resolved even though categorization wasn't confident; it
	// should still be recorded.
	if got[0].GithubRepo == nil || *got[0].GithubRepo != "example/baz" {
		t.Fatalf("expected github_repo %q to be recorded, got %+v", "example/baz", got[0].GithubRepo)
	}
}

func TestProcessLibrary_SecondFinderUsedWhenFirstDoesNotFind(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	lib := seedOneLibrary(t, store, "55555555-5555-5555-5555-555555555554",
		NewLibrary{Purl: "pkg:maven/com.example/qux@1.0.0", Name: "qux", Ecosystem: "maven"})

	finders := []RepoFinder{
		&fakeRepoFinder{found: false},
		&fakeRepoFinder{repo: "example/qux", found: true},
	}
	topics := &fakeTopicsFetcher{topics: nil}
	llm := &fakeCategorizer{categories: nil, confident: false}

	if err := ProcessLibrary(ctx, store, finders, topics, llm, lib); err != nil {
		t.Fatalf("ProcessLibrary failed: %v", err)
	}

	got, err := store.ListLibraries(ctx, LibraryFilter{BatchID: lib.UploadBatchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(got) != 1 || got[0].GithubRepo == nil || *got[0].GithubRepo != "example/qux" {
		t.Fatalf("expected fallback finder's repo to be used, got %+v", got)
	}
}

func TestProcessLibrary_FinderError_ReturnsErrorWithoutStatusChange(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	lib := seedOneLibrary(t, store, "55555555-5555-5555-5555-555555555555",
		NewLibrary{Purl: "pkg:maven/com.example/quux@1.0.0", Name: "quux", Ecosystem: "maven"})

	finders := []RepoFinder{&fakeRepoFinder{err: errors.New("transient failure")}}
	topics := &fakeTopicsFetcher{}
	llm := &fakeCategorizer{}

	err := ProcessLibrary(ctx, store, finders, topics, llm, lib)
	if err == nil {
		t.Fatal("expected ProcessLibrary to return an error for a transient finder failure")
	}

	got, err2 := store.ListLibraries(ctx, LibraryFilter{BatchID: lib.UploadBatchID})
	if err2 != nil {
		t.Fatalf("ListLibraries failed: %v", err2)
	}
	if len(got) != 1 || got[0].Status != "pending" {
		t.Fatalf("expected status to remain %q on transient error, got %+v", "pending", got)
	}
}
