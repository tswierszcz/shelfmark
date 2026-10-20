package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Project is a full projects row: a named, free-text-identified grouping of
// libraries uploaded together via a project SBOM.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ProjectLibraryEntry is one row to insert into project_libraries, as built
// by the upload handler from a resolved Library plus the SBOM parser's
// direct/transitive classification.
type ProjectLibraryEntry struct {
	LibraryID string
	IsDirect  bool
}

// ProjectReportRow is one (library, category) pair of a project's
// redundancy report, as returned by ProjectReport. A library attached to
// multiple categories produces multiple rows.
type ProjectReportRow struct {
	LibraryID       string  `json:"library_id"`
	Purl            string  `json:"purl"`
	Name            string  `json:"name"`
	Ecosystem       string  `json:"ecosystem"`
	GithubRepo      *string `json:"github_repo"`
	Status          string  `json:"status"`
	IsDirect        bool    `json:"is_direct"`
	CategoryID      string  `json:"category_id"`
	CategoryName    string  `json:"category_name"`
	Strategic       bool    `json:"strategic"`
	RetireCandidate bool    `json:"retire_candidate"`
}

// FindOrCreateProject looks up a project by case-insensitive name, creating
// it if absent. A found project has its updated_at touched so the project's
// "last uploaded" time stays current, following the upsert idiom used by
// AttachCategory/MarkStrategic.
func (s *Store) FindOrCreateProject(ctx context.Context, name, createdBy string) (*Project, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO projects (name, created_by)
		 VALUES ($1, $2)
		 ON CONFLICT (lower(name)) DO UPDATE SET updated_at = now()
		 RETURNING id, name, created_by, created_at`,
		name, createdBy,
	)

	var p Project
	if err := row.Scan(&p.ID, &p.Name, &p.CreatedBy, &p.CreatedAt); err != nil {
		return nil, fmt.Errorf("catalog: failed to find or create project %q: %w", name, err)
	}

	return &p, nil
}

// LibrariesByPurls resolves full Library rows for purls, independent of
// upload_batch_id. CreateLibraries + a BatchID filter misses purls that
// already existed under a different batch, so callers that need every
// resolved library for a purl list (regardless of when it was first
// ingested) use this instead.
func (s *Store) LibrariesByPurls(ctx context.Context, purls []string) ([]Library, error) {
	if len(purls) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, purl, name, ecosystem, github_repo, status, upload_batch_id
		 FROM libraries
		 WHERE purl = ANY($1)`,
		purls,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to list libraries by purl: %w", err)
	}
	defer rows.Close()

	var libraries []Library
	for rows.Next() {
		var l Library
		if err := rows.Scan(&l.ID, &l.Purl, &l.Name, &l.Ecosystem, &l.GithubRepo, &l.Status, &l.UploadBatchID); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan library: %w", err)
		}
		libraries = append(libraries, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to list libraries by purl: %w", err)
	}

	return libraries, nil
}

// ReplaceProjectLibraries replaces projectID's recorded library membership
// with entries in a single transaction (delete then bulk-insert), so a
// re-upload's library set never partially overlaps the previous one.
func (s *Store) ReplaceProjectLibraries(ctx context.Context, projectID string, entries []ProjectLibraryEntry) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("catalog: failed to begin project library replace: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM project_libraries WHERE project_id = $1`, projectID); err != nil {
		return fmt.Errorf("catalog: failed to clear project libraries: %w", err)
	}

	if len(entries) > 0 {
		batch := &pgx.Batch{}
		for _, entry := range entries {
			batch.Queue(
				`INSERT INTO project_libraries (project_id, library_id, is_direct) VALUES ($1, $2, $3)`,
				projectID, entry.LibraryID, entry.IsDirect,
			)
		}

		br := tx.SendBatch(ctx, batch)
		for range entries {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("catalog: failed to insert project library: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return fmt.Errorf("catalog: failed to insert project library: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("catalog: failed to commit project library replace: %w", err)
	}

	return nil
}

// projectExists reports whether a project with the given ID exists, letting
// ProjectReportHandler distinguish a nonexistent project (404) from one that
// exists but has no recorded libraries yet (200 with an empty report).
func (s *Store) projectExists(ctx context.Context, projectID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM projects WHERE id = $1)`,
		projectID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("catalog: failed to check project existence: %w", err)
	}

	return exists, nil
}

// ListProjects returns every project, most recently updated first, for the
// frontend's project picker.
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, created_by, created_at FROM projects ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to list projects: %w", err)
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to list projects: %w", err)
	}

	return projects, nil
}

// ProjectReport computes projectID's redundancy report: one row per
// (library, category) pair of every library recorded against the project,
// with RetireCandidate set for a direct library that is not itself the
// strategic pick in a category some library (not necessarily this one) has
// already been reviewed for.
func (s *Store) ProjectReport(ctx context.Context, projectID string) ([]ProjectReportRow, error) {
	reviewedCategories, err := s.reviewedCategories(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT pl.library_id, l.purl, l.name, l.ecosystem, l.github_repo, l.status, pl.is_direct,
		        c.id, c.name, (sm.library_id IS NOT NULL) AS strategic
		 FROM project_libraries pl
		 JOIN libraries l ON l.id = pl.library_id
		 JOIN library_categories lc ON lc.library_id = l.id
		 JOIN categories c ON c.id = lc.category_id
		 LEFT JOIN strategic_marks sm
		        ON sm.library_id = l.id AND sm.category_id = c.id AND sm.active
		 WHERE pl.project_id = $1
		 ORDER BY l.purl, c.name`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to compute project report: %w", err)
	}
	defer rows.Close()

	report := []ProjectReportRow{}
	for rows.Next() {
		var row ProjectReportRow
		if err := rows.Scan(
			&row.LibraryID, &row.Purl, &row.Name, &row.Ecosystem, &row.GithubRepo, &row.Status, &row.IsDirect,
			&row.CategoryID, &row.CategoryName, &row.Strategic,
		); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan project report row: %w", err)
		}
		row.RetireCandidate = row.IsDirect && reviewedCategories[row.CategoryID] && !row.Strategic
		report = append(report, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to compute project report: %w", err)
	}

	return report, nil
}

// reviewedCategories returns, keyed by category ID, whether any library at
// all currently holds an active strategic mark on that category. ProjectReport
// needs this to tell "no library has been reviewed for this category yet"
// (never a retire candidate) apart from "this library lost out to another
// library's strategic pick" (a retire candidate).
func (s *Store) reviewedCategories(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT category_id FROM strategic_marks WHERE active`,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to list reviewed categories: %w", err)
	}
	defer rows.Close()

	reviewed := map[string]bool{}
	for rows.Next() {
		var categoryID string
		if err := rows.Scan(&categoryID); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan reviewed category: %w", err)
		}
		reviewed[categoryID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to list reviewed categories: %w", err)
	}

	return reviewed, nil
}
