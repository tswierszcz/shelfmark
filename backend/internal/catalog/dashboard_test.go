package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// attachTestCategory attaches each library to categoryID.
func attachTestCategory(t *testing.T, store *Store, categoryID string, libs ...Library) {
	t.Helper()
	for _, l := range libs {
		if err := store.AttachCategory(context.Background(), l.ID, categoryID, "manual"); err != nil {
			t.Fatalf("AttachCategory(%s) failed: %v", l.Name, err)
		}
	}
}

// newTestProject creates a project whose dependencies are direct and
// transitive.
func newTestProject(t *testing.T, store *Store, name string, direct, transitive []Library) string {
	t.Helper()
	ctx := context.Background()

	p, err := store.FindOrCreateProject(ctx, name, "architect@example.test")
	if err != nil {
		t.Fatalf("FindOrCreateProject(%s) failed: %v", name, err)
	}
	entries := []ProjectLibraryEntry{}
	for _, l := range direct {
		entries = append(entries, ProjectLibraryEntry{LibraryID: l.ID, IsDirect: true})
	}
	for _, l := range transitive {
		entries = append(entries, ProjectLibraryEntry{LibraryID: l.ID, IsDirect: false})
	}
	if err := store.ReplaceProjectLibraries(ctx, p.ID, entries); err != nil {
		t.Fatalf("ReplaceProjectLibraries(%s) failed: %v", name, err)
	}
	return p.ID
}

func dashboardStats(t *testing.T, store *Store) DashboardStats {
	t.Helper()
	stats, err := store.DashboardStats(context.Background())
	if err != nil {
		t.Fatalf("DashboardStats failed: %v", err)
	}
	return stats
}

func hotspotByName(stats DashboardStats, name string) (Hotspot, bool) {
	for _, h := range stats.Hotspots {
		if h.CategoryName == name {
			return h, true
		}
	}
	return Hotspot{}, false
}

func TestDashboardStats_EmptyDatabase(t *testing.T) {
	pool := startTestPostgres(t)
	stats := dashboardStats(t, NewStore(pool))

	if stats.Libraries.Total != 0 {
		t.Errorf("expected 0 libraries, got %d", stats.Libraries.Total)
	}
	for _, status := range []string{"pending", "processing", "categorized", "needs_manual", "failed"} {
		count, ok := stats.Libraries.ByStatus[status]
		if !ok || count != 0 {
			t.Errorf("expected by_status[%q] present and 0, got %d (present %v)", status, count, ok)
		}
	}
	if stats.Libraries.ByEcosystem == nil || len(stats.Libraries.ByEcosystem) != 0 {
		t.Errorf("expected empty non-nil by_ecosystem, got %#v", stats.Libraries.ByEcosystem)
	}
	if stats.Hotspots == nil || len(stats.Hotspots) != 0 {
		t.Errorf("expected empty non-nil hotspots, got %#v", stats.Hotspots)
	}
	if stats.Categories.Total != 14 || stats.Categories.InUse != 0 {
		t.Errorf("expected categories 14 total / 0 in use, got %+v", stats.Categories)
	}
	if stats.Projects != 0 || stats.StrategicMarks != 0 || stats.UnreviewedHotspots != 0 {
		t.Errorf("expected zero projects, marks and unreviewed hotspots, got %+v", stats)
	}
}

func TestDashboardStats_HotspotCountsOnlyDirectProjectLibraries(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	logging := newTestCategory(t, pool, "dash-logging")
	single := newTestCategory(t, pool, "dash-single")

	// dash-logging: two direct, one transitive only, one catalog only.
	libs := newTestLibraries(t, store, pool, "maven", "log", 4)
	attachTestCategory(t, store, logging, libs...)
	// dash-single: one direct library only.
	lone := newTestLibraries(t, store, pool, "maven", "lone", 1)
	attachTestCategory(t, store, single, lone...)

	newTestProject(t, store, "alpha", []Library{libs[0], lone[0]}, []Library{libs[2]})
	// libs[1] is direct in beta; libs[0] direct in both counts once.
	newTestProject(t, store, "beta", []Library{libs[0], libs[1]}, nil)

	stats := dashboardStats(t, store)

	h, ok := hotspotByName(stats, "dash-logging")
	if !ok {
		t.Fatalf("expected dash-logging hotspot, got %+v", stats.Hotspots)
	}
	if h.LibraryCount != 2 {
		t.Errorf("expected dash-logging library_count 2 (direct only), got %d", h.LibraryCount)
	}
	if _, ok := hotspotByName(stats, "dash-single"); ok {
		t.Errorf("a category with one direct library must not be a hotspot: %+v", stats.Hotspots)
	}
	if stats.Projects != 2 {
		t.Errorf("expected 2 projects, got %d", stats.Projects)
	}
	if stats.Categories.InUse != 2 {
		t.Errorf("expected 2 categories in use, got %d", stats.Categories.InUse)
	}
}

func TestDashboardStats_HotspotsTopFiveOrderedByCountThenName(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)

	// Library counts per category; dash-c and dash-d tie at 3, dash-f is
	// the 6th and must be cut off.
	sizes := []struct {
		name string
		n    int
	}{
		{"dash-d", 3},
		{"dash-a", 5},
		{"dash-c", 3},
		{"dash-b", 4},
		{"dash-e", 2},
		{"dash-f", 2},
	}
	direct := []Library{}
	for _, s := range sizes {
		cat := newTestCategory(t, pool, s.name)
		libs := newTestLibraries(t, store, pool, "maven", s.name, s.n)
		attachTestCategory(t, store, cat, libs...)
		direct = append(direct, libs...)
	}
	newTestProject(t, store, "alpha", direct, nil)

	stats := dashboardStats(t, store)

	got := []string{}
	for _, h := range stats.Hotspots {
		got = append(got, fmt.Sprintf("%s:%d", h.CategoryName, h.LibraryCount))
	}
	want := []string{"dash-a:5", "dash-b:4", "dash-c:3", "dash-d:3", "dash-e:2"}
	if !slices.Equal(got, want) {
		t.Errorf("expected hotspots %v, got %v", want, got)
	}
	if stats.UnreviewedHotspots != 6 {
		t.Errorf("expected all 6 hotspot categories unreviewed, got %d", stats.UnreviewedHotspots)
	}
}

func TestDashboardStats_ReviewedFollowsActiveStrategicMark(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	marked := newTestCategory(t, pool, "dash-marked")
	unmarked := newTestCategory(t, pool, "dash-unmarked")
	markedLibs := newTestLibraries(t, store, pool, "maven", "marked", 2)
	unmarkedLibs := newTestLibraries(t, store, pool, "maven", "unmarked", 2)
	attachTestCategory(t, store, marked, markedLibs...)
	attachTestCategory(t, store, unmarked, unmarkedLibs...)
	newTestProject(t, store, "alpha", append(slices.Clone(markedLibs), unmarkedLibs...), nil)

	if err := store.MarkStrategic(ctx, markedLibs[0].ID, marked, "keep", "architect@example.test"); err != nil {
		t.Fatalf("MarkStrategic failed: %v", err)
	}
	// A mark that was later withdrawn is inactive and must not count.
	if err := store.MarkStrategic(ctx, unmarkedLibs[0].ID, unmarked, "keep", "architect@example.test"); err != nil {
		t.Fatalf("MarkStrategic failed: %v", err)
	}
	if err := store.UnmarkStrategic(ctx, unmarkedLibs[0].ID, unmarked, "changed mind", "architect@example.test"); err != nil {
		t.Fatalf("UnmarkStrategic failed: %v", err)
	}

	stats := dashboardStats(t, store)

	if h, ok := hotspotByName(stats, "dash-marked"); !ok || !h.Reviewed {
		t.Errorf("expected dash-marked reviewed, got %+v (found %v)", h, ok)
	}
	if h, ok := hotspotByName(stats, "dash-unmarked"); !ok || h.Reviewed {
		t.Errorf("expected dash-unmarked not reviewed, got %+v (found %v)", h, ok)
	}
	if stats.UnreviewedHotspots != 1 {
		t.Errorf("expected 1 unreviewed hotspot, got %d", stats.UnreviewedHotspots)
	}
	if stats.StrategicMarks != 1 {
		t.Errorf("expected 1 active strategic mark, got %d", stats.StrategicMarks)
	}
}

func TestDashboardStats_StatusAndEcosystemCounts(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewStore(pool)
	ctx := context.Background()

	maven := newTestLibraries(t, store, pool, "maven", "m", 3)
	npm := newTestLibraries(t, store, pool, "npm", "n", 2)
	for lib, status := range map[string]string{
		maven[0].ID: "categorized",
		maven[1].ID: "needs_manual",
		npm[0].ID:   "failed",
	} {
		if err := store.UpdateLibraryStatus(ctx, lib, status, nil, nil); err != nil {
			t.Fatalf("UpdateLibraryStatus failed: %v", err)
		}
	}

	stats := dashboardStats(t, store)

	if stats.Libraries.Total != 5 {
		t.Errorf("expected 5 libraries, got %d", stats.Libraries.Total)
	}
	wantStatus := map[string]int{"pending": 2, "processing": 0, "categorized": 1, "needs_manual": 1, "failed": 1}
	for status, want := range wantStatus {
		if got := stats.Libraries.ByStatus[status]; got != want {
			t.Errorf("by_status[%q]: expected %d, got %d", status, want, got)
		}
	}
	wantEco := []EcosystemCount{{Ecosystem: "maven", Count: 3}, {Ecosystem: "npm", Count: 2}}
	if !slices.Equal(stats.Libraries.ByEcosystem, wantEco) {
		t.Errorf("expected by_ecosystem %+v, got %+v", wantEco, stats.Libraries.ByEcosystem)
	}
}

func TestDashboardHandler_StoreError_Returns500(t *testing.T) {
	pool := startTestPostgres(t)
	pool.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()
	DashboardHandler(NewStore(pool))(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Fatalf("expected a JSON error body, got %q", rec.Body.String())
	}
}
