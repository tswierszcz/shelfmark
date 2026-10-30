package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tswierszcz/shelfmark/backend/internal/obs"
)

// libraryStatuses lists every libraries.status value (the CHECK constraint
// in migration 0002), so DashboardStats reports each one even at zero.
var libraryStatuses = []string{"pending", "processing", "categorized", "needs_manual", "failed"}

// maxDashboardHotspots caps how many hotspot categories DashboardStats
// returns; UnreviewedHotspots still counts all of them.
const maxDashboardHotspots = 5

// EcosystemCount is the number of catalog libraries in one ecosystem.
type EcosystemCount struct {
	Ecosystem string `json:"ecosystem"`
	Count     int    `json:"count"`
}

// DashboardLibraries summarizes the catalog's libraries.
type DashboardLibraries struct {
	Total       int              `json:"total"`
	ByStatus    map[string]int   `json:"by_status"`
	ByEcosystem []EcosystemCount `json:"by_ecosystem"`
}

// DashboardCategories compares the taxonomy with the categories that at
// least one library is attached to.
type DashboardCategories struct {
	Total int `json:"total"`
	InUse int `json:"in_use"`
}

// Hotspot is a category with at least two distinct libraries that are a
// direct dependency of at least one project. Reviewed follows
// strategicIndex.isReviewed.
type Hotspot struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	LibraryCount int    `json:"library_count"`
	Reviewed     bool   `json:"reviewed"`
}

// DashboardStats is the aggregate view served by GET /api/dashboard. Its
// slices and map are never nil, so it never marshals a JSON null.
type DashboardStats struct {
	Libraries          DashboardLibraries  `json:"libraries"`
	Categories         DashboardCategories `json:"categories"`
	Projects           int                 `json:"projects"`
	StrategicMarks     int                 `json:"strategic_marks"`
	Hotspots           []Hotspot           `json:"hotspots"`
	UnreviewedHotspots int                 `json:"unreviewed_hotspots"`
}

// DashboardStats computes the dashboard aggregates. Hotspots are ordered by
// library count descending, then category name, and truncated to
// maxDashboardHotspots; UnreviewedHotspots counts every hotspot category.
func (s *Store) DashboardStats(ctx context.Context) (DashboardStats, error) {
	stats := DashboardStats{
		Libraries: DashboardLibraries{
			ByStatus:    make(map[string]int, len(libraryStatuses)),
			ByEcosystem: []EcosystemCount{},
		},
		Hotspots: []Hotspot{},
	}
	for _, status := range libraryStatuses {
		stats.Libraries.ByStatus[status] = 0
	}

	statusRows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM libraries GROUP BY status`)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("catalog: failed to count libraries by status: %w", err)
	}
	for statusRows.Next() {
		var (
			status string
			count  int
		)
		if err := statusRows.Scan(&status, &count); err != nil {
			statusRows.Close()
			return DashboardStats{}, fmt.Errorf("catalog: failed to scan library status count: %w", err)
		}
		stats.Libraries.ByStatus[status] = count
		stats.Libraries.Total += count
	}
	statusRows.Close()
	if err := statusRows.Err(); err != nil {
		return DashboardStats{}, fmt.Errorf("catalog: failed to count libraries by status: %w", err)
	}

	ecosystemRows, err := s.pool.Query(ctx,
		`SELECT ecosystem, count(*) FROM libraries GROUP BY ecosystem ORDER BY count(*) DESC, ecosystem`,
	)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("catalog: failed to count libraries by ecosystem: %w", err)
	}
	for ecosystemRows.Next() {
		var ec EcosystemCount
		if err := ecosystemRows.Scan(&ec.Ecosystem, &ec.Count); err != nil {
			ecosystemRows.Close()
			return DashboardStats{}, fmt.Errorf("catalog: failed to scan ecosystem count: %w", err)
		}
		stats.Libraries.ByEcosystem = append(stats.Libraries.ByEcosystem, ec)
	}
	ecosystemRows.Close()
	if err := ecosystemRows.Err(); err != nil {
		return DashboardStats{}, fmt.Errorf("catalog: failed to count libraries by ecosystem: %w", err)
	}

	if err := s.pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM categories),
		        (SELECT count(DISTINCT category_id) FROM library_categories),
		        (SELECT count(*) FROM projects),
		        (SELECT count(*) FROM strategic_marks WHERE active)`,
	).Scan(&stats.Categories.Total, &stats.Categories.InUse, &stats.Projects, &stats.StrategicMarks); err != nil {
		return DashboardStats{}, fmt.Errorf("catalog: failed to count dashboard totals: %w", err)
	}

	idx, err := s.loadStrategicIndex(ctx)
	if err != nil {
		return DashboardStats{}, err
	}

	// A library counts toward a category only when it is a direct
	// dependency of at least one project; catalog-only and transitive-only
	// libraries are not redundancy the organization ships.
	hotspotRows, err := s.pool.Query(ctx,
		`SELECT c.id, c.name, count(DISTINCT lc.library_id) AS library_count
		 FROM library_categories lc
		 JOIN categories c ON c.id = lc.category_id
		 WHERE EXISTS (
		     SELECT 1 FROM project_libraries pl
		     WHERE pl.library_id = lc.library_id AND pl.is_direct
		 )
		 GROUP BY c.id, c.name
		 HAVING count(DISTINCT lc.library_id) >= 2
		 ORDER BY library_count DESC, c.name`,
	)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("catalog: failed to list hotspots: %w", err)
	}
	defer hotspotRows.Close()

	for hotspotRows.Next() {
		var h Hotspot
		if err := hotspotRows.Scan(&h.CategoryID, &h.CategoryName, &h.LibraryCount); err != nil {
			return DashboardStats{}, fmt.Errorf("catalog: failed to scan hotspot: %w", err)
		}
		h.Reviewed = idx.isReviewed(h.CategoryID)
		if !h.Reviewed {
			stats.UnreviewedHotspots++
		}
		if len(stats.Hotspots) < maxDashboardHotspots {
			stats.Hotspots = append(stats.Hotspots, h)
		}
	}
	if err := hotspotRows.Err(); err != nil {
		return DashboardStats{}, fmt.Errorf("catalog: failed to list hotspots: %w", err)
	}

	return stats, nil
}

// DashboardHandler returns a handler for GET /api/dashboard: it reports the
// aggregate catalog, project and redundancy statistics for any signed-in
// role. It carries counts and category names only, no admin-only fields.
func DashboardHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := store.DashboardStats(r.Context())
		if err != nil {
			writeCatalogError(w, r, http.StatusInternalServerError, "failed to load dashboard statistics", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			obs.LogWarn(r, "failed to encode response", err)
		}
	}
}
