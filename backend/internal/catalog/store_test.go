package catalog

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"sync"
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
		"--tmpfs", "/var/lib/postgresql/data",
		"postgres:16-alpine",
	}
	out, err := exec.Command("docker", runArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed: %v\n%s", err, out)
	}

	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", "-v", containerName).Run()
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
	if err := store.UpdateLibraryStatus(ctx, listed[0].ID, "categorized", &repo, nil); err != nil {
		t.Fatalf("UpdateLibraryStatus failed: %v", err)
	}
	if err := store.UpdateLibraryStatus(ctx, listed[1].ID, "needs_manual", nil, nil); err != nil {
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

func TestStore_ListStuckLibraries_RespectsAgeAndStatus(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	const batchID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
	type row struct {
		name, status string
		age          time.Duration
	}
	rows := []row{
		{"old-pending", "pending", 20 * time.Minute},
		{"old-processing", "processing", 25 * time.Minute},
		{"old-categorized", "categorized", 20 * time.Minute},
		{"new-pending", "pending", time.Minute},
		{"new-processing", "processing", time.Minute},
		{"new-categorized", "categorized", time.Minute},
	}
	newLibs := make([]NewLibrary, 0, len(rows))
	for _, r := range rows {
		newLibs = append(newLibs, NewLibrary{Purl: "pkg:maven/com.example/" + r.name + "@1.0.0", Name: r.name, Ecosystem: "maven"})
	}
	if err := store.CreateLibraries(ctx, batchID, newLibs); err != nil {
		t.Fatalf("CreateLibraries failed: %v", err)
	}
	libs, err := store.ListLibraries(ctx, LibraryFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}
	idByName := map[string]string{}
	for _, l := range libs {
		idByName[l.Name] = l.ID
	}
	for _, r := range rows {
		if _, err := pool.Exec(ctx,
			`UPDATE libraries SET status = $1, updated_at = now() - make_interval(secs => $2) WHERE id = $3`,
			r.status, r.age.Seconds(), idByName[r.name]); err != nil {
			t.Fatalf("aging %s failed: %v", r.name, err)
		}
	}

	stuck, err := store.ListStuckLibraries(ctx, 10*time.Minute)
	if err != nil {
		t.Fatalf("ListStuckLibraries failed: %v", err)
	}
	got := make([]string, 0, len(stuck))
	for _, l := range stuck {
		got = append(got, l.Name)
	}
	// Oldest first.
	want := []string{"old-processing", "old-pending"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("ListStuckLibraries = %v, want %v", got, want)
	}

	empty, err := store.ListStuckLibraries(ctx, time.Hour)
	if err != nil {
		t.Fatalf("ListStuckLibraries failed: %v", err)
	}
	if empty == nil {
		t.Fatalf("ListStuckLibraries returned nil on empty result, want an empty slice")
	}
	if len(empty) != 0 {
		t.Fatalf("ListStuckLibraries(1h) = %d rows, want 0", len(empty))
	}
}

// newTestCategory inserts a fresh category so a test owns an empty
// (category, ecosystem) pair, independent of the seeded taxonomy.
func newTestCategory(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO categories (name) VALUES ($1) RETURNING id`, name,
	).Scan(&id); err != nil {
		t.Fatalf("insert category %q failed: %v", name, err)
	}
	return id
}

// newTestLibraries creates n libraries of one ecosystem under a fresh upload
// batch and returns them in creation order.
func newTestLibraries(t *testing.T, store *Store, pool *pgxpool.Pool, ecosystem, prefix string, n int) []Library {
	t.Helper()
	ctx := context.Background()

	var batchID string
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&batchID); err != nil {
		t.Fatalf("generate batch id failed: %v", err)
	}

	newLibs := make([]NewLibrary, n)
	for i := range newLibs {
		name := fmt.Sprintf("%s-%d", prefix, i)
		newLibs[i] = NewLibrary{Purl: fmt.Sprintf("pkg:%s/com.example/%s@1.0.0", ecosystem, name), Name: name, Ecosystem: ecosystem}
	}
	if err := store.CreateLibraries(ctx, batchID, newLibs); err != nil {
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
	libs := make([]Library, n)
	for i, nl := range newLibs {
		l, ok := byPurl[nl.Purl]
		if !ok {
			t.Fatalf("library %q not created", nl.Purl)
		}
		libs[i] = l
	}
	return libs
}

// countActiveStrategicMarks counts active marks in a (category, ecosystem)
// pair directly in SQL, independent of MarkStrategic's own check.
func countActiveStrategicMarks(t *testing.T, pool *pgxpool.Pool, categoryID, ecosystem string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM strategic_marks sm JOIN libraries l ON l.id = sm.library_id
		 WHERE sm.active AND sm.category_id = $1 AND l.ecosystem = $2`,
		categoryID, ecosystem,
	).Scan(&count); err != nil {
		t.Fatalf("count active strategic marks failed: %v", err)
	}
	return count
}

// holdStrategicLock takes MarkStrategic's advisory lock for (categoryID,
// ecosystem) in a separate pool transaction and keeps it open. The returned
// release func rolls the holder back (idempotent; also registered with
// t.Cleanup).
func holdStrategicLock(t *testing.T, pool *pgxpool.Pool, categoryID, ecosystem string) (release func()) {
	t.Helper()
	ctx := context.Background()

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin lock holder failed: %v", err)
	}
	if _, err := holder.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`,
		categoryID, ecosystem,
	); err != nil {
		holder.Rollback(ctx)
		t.Fatalf("lock holder failed to take advisory lock: %v", err)
	}

	var once sync.Once
	release = func() {
		once.Do(func() { holder.Rollback(ctx) })
	}
	t.Cleanup(release)
	return release
}

// setStrategicLockTimeout overrides the package-level lock timeout for one
// test. Tests that call it must not use t.Parallel.
func setStrategicLockTimeout(t *testing.T, value string) {
	t.Helper()
	previous := strategicLockTimeout
	t.Cleanup(func() { strategicLockTimeout = previous })
	strategicLockTimeout = value
}
