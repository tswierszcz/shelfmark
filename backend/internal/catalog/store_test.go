package catalog

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tswierszcz/shelfmark/backend/internal/db"
)

// startTestPostgres starts a throwaway Postgres container via the docker CLI
// and returns a connected, migrated *pgxpool.Pool, plus registers cleanup
// via t.Cleanup that closes the pool and stops/removes the container. The
// test is skipped (not failed) if Docker isn't available.
//
// This duplicates internal/db's migrate_test.go helper of the same shape
// (its version is unexported from that package, so it can't be reused
// directly); kept intentionally small rather than factored into a shared
// test-helper package for just two callers.
func startTestPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available, skipping integration test")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not reachable, skipping integration test")
	}

	port, err := findFreePort()
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}

	containerName := fmt.Sprintf("shelfmark-catalog-test-%d", time.Now().UnixNano())
	runArgs := []string{
		"run", "--rm", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:5432", port),
		"-e", "POSTGRES_PASSWORD=postgres",
		"postgres:16-alpine",
	}
	out, err := exec.Command("docker", runArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed: %v\n%s", err, out)
	}

	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", containerName).Run()
	})

	databaseURL := fmt.Sprintf("postgres://postgres:postgres@localhost:%d/postgres?sslmode=disable", port)
	t.Setenv("DATABASE_URL", databaseURL)

	pool := waitForPostgres(t, databaseURL)

	if err := db.Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPostgres(t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("postgres did not become ready in time: %v", lastErr)
		default:
		}

		pool, err := db.Connect(ctx)
		if err == nil {
			return pool
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
}

func TestListTaxonomy_SeededByMigration(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}

	wantNames := []string{
		"logging", "http-client", "serialization", "orm-persistence",
		"dependency-injection", "testing", "build-tooling", "security-crypto",
		"messaging", "metrics-observability", "web-framework", "utility",
		"json-processing", "validation",
	}
	if len(categories) != len(wantNames) {
		t.Fatalf("expected %d seeded categories, got %d: %+v", len(wantNames), len(categories), categories)
	}

	got := map[string]bool{}
	for _, c := range categories {
		if c.ID == "" {
			t.Fatalf("category %q has empty ID", c.Name)
		}
		got[c.Name] = true
	}
	for _, name := range wantNames {
		if !got[name] {
			t.Fatalf("expected seeded category %q, not found in %+v", name, categories)
		}
	}
}

func TestCreateLibraries_IdempotentOnDuplicatePurl(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "11111111-1111-1111-1111-111111111111"
	lib := NewLibrary{Purl: "pkg:maven/com.example/foo@1.0.0", Name: "foo", Ecosystem: "maven"}

	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("first CreateLibraries failed: %v", err)
	}

	// Re-inserting the same purl (even under a different batch) must be a
	// safe no-op, not an error.
	otherBatchID := "22222222-2222-2222-2222-222222222222"
	if err := store.CreateLibraries(ctx, otherBatchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("second CreateLibraries (duplicate purl) failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}

	count := 0
	for _, l := range libs {
		if l.Purl == lib.Purl {
			count++
			if l.UploadBatchID != batchID {
				t.Fatalf("expected duplicate insert to leave original batch_id %q, got %q", batchID, l.UploadBatchID)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row for duplicate purl, found %d", count)
	}
}

func TestAttachCategory_UpsertsWithoutErrorOnRepeat(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "33333333-3333-3333-3333-333333333333"
	lib := NewLibrary{Purl: "pkg:maven/com.example/bar@2.0.0", Name: "bar", Ecosystem: "maven"}
	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library in batch, got %d", len(libs))
	}
	libraryID := libs[0].ID

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.AttachCategory(ctx, libraryID, categoryID, "github"); err != nil {
		t.Fatalf("first AttachCategory failed: %v", err)
	}
	// Repeat call with a different source must upsert, not error.
	if err := store.AttachCategory(ctx, libraryID, categoryID, "manual"); err != nil {
		t.Fatalf("second AttachCategory (repeat) failed: %v", err)
	}

	var source string
	row := pool.QueryRow(ctx,
		`SELECT source FROM library_categories WHERE library_id = $1 AND category_id = $2`,
		libraryID, categoryID,
	)
	if err := row.Scan(&source); err != nil {
		t.Fatalf("failed to read library_categories row: %v", err)
	}
	if source != "manual" {
		t.Fatalf("expected upsert to update source to %q, got %q", "manual", source)
	}
}

func TestBatchProgress_ReturnsCorrectCountsForMixedStatusBatch(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "44444444-4444-4444-4444-444444444444"
	libs := []NewLibrary{
		{Purl: "pkg:maven/com.example/a@1.0.0", Name: "a", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/b@1.0.0", Name: "b", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/c@1.0.0", Name: "c", Ecosystem: "maven"},
	}
	if err := store.CreateLibraries(ctx, batchID, libs); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	listed, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 libraries in batch, got %d", len(listed))
	}

	// Drive two of the three into non-pending statuses; leave the third pending.
	repo := "github.com/example/a"
	if err := store.UpdateLibraryStatus(ctx, listed[0].ID, "categorized", &repo); err != nil {
		t.Fatalf("UpdateLibraryStatus failed: %v", err)
	}
	if err := store.UpdateLibraryStatus(ctx, listed[1].ID, "needs_manual", nil); err != nil {
		t.Fatalf("UpdateLibraryStatus failed: %v", err)
	}

	progress, err := store.BatchProgress(ctx, batchID)
	if err != nil {
		t.Fatalf("BatchProgress failed: %v", err)
	}

	want := BatchProgress{"categorized": 1, "needs_manual": 1, "pending": 1}
	for status, count := range want {
		if progress[status] != count {
			t.Fatalf("expected %d libraries with status %q, got %d (full: %+v)", count, status, progress[status], progress)
		}
	}
}

func TestMarkStrategic_UpsertsWithoutErrorOnRepeat(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "55555555-5555-5555-5555-555555555555"
	lib := NewLibrary{Purl: "pkg:maven/com.example/strategic-mark@1.0.0", Name: "strategic-mark", Ecosystem: "maven"}
	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library in batch, got %d", len(libs))
	}
	libraryID := libs[0].ID

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, libraryID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("first MarkStrategic failed: %v", err)
	}
	// Repeat call with different fields must upsert, not error.
	if err := store.MarkStrategic(ctx, libraryID, categoryID, "re-confirmed critical dependency", "bob"); err != nil {
		t.Fatalf("second MarkStrategic (repeat) failed: %v", err)
	}

	marks, err := store.StrategicMarksByLibrary(ctx, []string{libraryID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}
	got := marks[libraryID]
	if len(got) != 1 {
		t.Fatalf("expected 1 active strategic mark, got %d: %+v", len(got), got)
	}
	if got[0].Justification != "re-confirmed critical dependency" || got[0].MarkedBy != "bob" {
		t.Fatalf("expected upsert to update justification/marked_by, got %+v", got[0])
	}
	if got[0].CategoryID != categoryID || got[0].CategoryName != categories[0].Name {
		t.Fatalf("expected category fields to match %q/%q, got %+v", categoryID, categories[0].Name, got[0])
	}
}

func TestUnmarkStrategic_OnNeverMarkedPairReturnsError(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "66666666-6666-6666-6666-666666666666"
	lib := NewLibrary{Purl: "pkg:maven/com.example/never-marked@1.0.0", Name: "never-marked", Ecosystem: "maven"}
	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	libraryID := libs[0].ID

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.UnmarkStrategic(ctx, libraryID, categoryID, "not actually strategic", "carol"); err == nil {
		t.Fatalf("expected UnmarkStrategic on a never-marked pair to return an error, got nil")
	}
}

func TestUnmarkStrategic_HappyPathDeactivatesMark(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "77777777-7777-7777-7777-777777777777"
	lib := NewLibrary{Purl: "pkg:maven/com.example/unmark-me@1.0.0", Name: "unmark-me", Ecosystem: "maven"}
	if err := store.CreateLibraries(ctx, batchID, []NewLibrary{lib}); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	libraryID := libs[0].ID

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, libraryID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic failed: %v", err)
	}

	if err := store.UnmarkStrategic(ctx, libraryID, categoryID, "no longer critical", "dave"); err != nil {
		t.Fatalf("UnmarkStrategic failed: %v", err)
	}

	marks, err := store.StrategicMarksByLibrary(ctx, []string{libraryID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}
	if got := marks[libraryID]; len(got) != 0 {
		t.Fatalf("expected no active strategic marks after unmark, got %+v", got)
	}

	// A second unmark on the now-inactive pair must fail (active = true
	// predicate no longer matches).
	if err := store.UnmarkStrategic(ctx, libraryID, categoryID, "still not critical", "dave"); err == nil {
		t.Fatalf("expected UnmarkStrategic on an already-inactive pair to return an error, got nil")
	}
}

func TestStrategicMarksByLibrary_ReflectsOnlyActiveMarks(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "88888888-8888-8888-8888-888888888888"
	libs := []NewLibrary{
		{Purl: "pkg:maven/com.example/active-mark@1.0.0", Name: "active-mark", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/inactive-mark@1.0.0", Name: "inactive-mark", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/no-mark@1.0.0", Name: "no-mark", Ecosystem: "maven"},
	}
	if err := store.CreateLibraries(ctx, batchID, libs); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	listed, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 libraries in batch, got %d", len(listed))
	}
	byPurl := map[string]Library{}
	for _, l := range listed {
		byPurl[l.Purl] = l
	}
	activeLib := byPurl["pkg:maven/com.example/active-mark@1.0.0"]
	inactiveLib := byPurl["pkg:maven/com.example/inactive-mark@1.0.0"]
	noMarkLib := byPurl["pkg:maven/com.example/no-mark@1.0.0"]

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, activeLib.ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic(active) failed: %v", err)
	}
	if err := store.MarkStrategic(ctx, inactiveLib.ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic(inactive) failed: %v", err)
	}
	if err := store.UnmarkStrategic(ctx, inactiveLib.ID, categoryID, "no longer critical", "bob"); err != nil {
		t.Fatalf("UnmarkStrategic(inactive) failed: %v", err)
	}

	marks, err := store.StrategicMarksByLibrary(ctx, []string{activeLib.ID, inactiveLib.ID, noMarkLib.ID})
	if err != nil {
		t.Fatalf("StrategicMarksByLibrary failed: %v", err)
	}

	if got := marks[activeLib.ID]; len(got) != 1 {
		t.Fatalf("expected 1 active mark for active-mark library, got %d: %+v", len(got), got)
	}
	if got := marks[inactiveLib.ID]; len(got) != 0 {
		t.Fatalf("expected 0 active marks for unmarked (deactivated) library, got %d: %+v", len(got), got)
	}
	if got := marks[noMarkLib.ID]; len(got) != 0 {
		t.Fatalf("expected 0 active marks for never-marked library, got %d: %+v", len(got), got)
	}
}

func TestListLibraries_StrategicFilter(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	batchID := "99999999-9999-9999-9999-999999999999"
	libs := []NewLibrary{
		{Purl: "pkg:maven/com.example/strategic-filter-yes@1.0.0", Name: "strategic-filter-yes", Ecosystem: "maven"},
		{Purl: "pkg:maven/com.example/strategic-filter-no@1.0.0", Name: "strategic-filter-no", Ecosystem: "maven"},
	}
	if err := store.CreateLibraries(ctx, batchID, libs); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}

	listed, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	byPurl := map[string]Library{}
	for _, l := range listed {
		byPurl[l.Purl] = l
	}
	strategicLib := byPurl["pkg:maven/com.example/strategic-filter-yes@1.0.0"]

	categories, err := store.ListTaxonomy(ctx)
	if err != nil {
		t.Fatalf("ListTaxonomy failed: %v", err)
	}
	categoryID := categories[0].ID

	if err := store.MarkStrategic(ctx, strategicLib.ID, categoryID, "critical dependency", "alice"); err != nil {
		t.Fatalf("MarkStrategic failed: %v", err)
	}

	trueVal, falseVal := true, false

	strategicOnly, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID, Strategic: &trueVal})
	if err != nil {
		t.Fatalf("ListLibraries(Strategic=true) failed: %v", err)
	}
	if len(strategicOnly) != 1 || strategicOnly[0].Purl != strategicLib.Purl {
		t.Fatalf("expected only the strategic library, got %+v", strategicOnly)
	}

	nonStrategicOnly, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID, Strategic: &falseVal})
	if err != nil {
		t.Fatalf("ListLibraries(Strategic=false) failed: %v", err)
	}
	if len(nonStrategicOnly) != 1 || nonStrategicOnly[0].Purl != "pkg:maven/com.example/strategic-filter-no@1.0.0" {
		t.Fatalf("expected only the non-strategic library, got %+v", nonStrategicOnly)
	}

	allLibs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries (no filter) failed: %v", err)
	}
	if len(allLibs) != 2 {
		t.Fatalf("expected both libraries with no Strategic filter, got %+v", allLibs)
	}
}
