// Package catalog provides the data model and store layer for Shelfmark's
// library catalog: a curated capability taxonomy, the libraries seeded into
// it, and the (library, category) attachments produced either by the
// automated categorization pipeline or by manual Admin correction.
package catalog

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Category is one entry in the curated taxonomy seeded by migration
// 0002_catalog_schema.
type Category struct {
	ID   string
	Name string
}

// NewLibrary holds the fields needed to insert a new libraries row, before
// it has an ID, status, or timestamps assigned by the database.
type NewLibrary struct {
	Purl      string
	Name      string
	Ecosystem string
}

// Library is a full libraries row.
type Library struct {
	ID            string
	Purl          string
	Name          string
	Ecosystem     string
	GithubRepo    *string
	Status        string
	UploadBatchID string
}

// LibraryFilter narrows ListLibraries. Every field is optional (zero value
// means "don't filter on this").
type LibraryFilter struct {
	Status    string
	Category  string
	Ecosystem string
	BatchID   string
}

// BatchProgress holds the count of libraries in each status for one upload
// batch, keyed by status value (e.g. "pending", "categorized").
type BatchProgress map[string]int

// Store wraps a shared *pgxpool.Pool with the SQL this slice needs,
// matching db.HealthCheck's pattern of taking the pool directly rather than
// opening its own connection.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore constructs a Store from an existing connection pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateLibraries bulk-inserts libs under batchID. Re-uploading a purl that
// already exists is a safe no-op (ON CONFLICT DO NOTHING) rather than an
// error, since the same SBOM/CSV may legitimately be uploaded more than
// once.
func (s *Store) CreateLibraries(ctx context.Context, batchID string, libs []NewLibrary) error {
	if len(libs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, lib := range libs {
		batch.Queue(
			`INSERT INTO libraries (purl, name, ecosystem, upload_batch_id)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (purl) DO NOTHING`,
			lib.Purl, lib.Name, lib.Ecosystem, batchID,
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range libs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("catalog: failed to insert library: %w", err)
		}
	}

	return nil
}

// ListTaxonomy returns every seeded category, ordered by name.
func (s *Store) ListTaxonomy(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM categories ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to list taxonomy: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan category: %w", err)
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to list taxonomy: %w", err)
	}

	return categories, nil
}

// ListLibraries returns libraries matching filter. Every filter field is
// optional; an empty filter returns every library.
func (s *Store) ListLibraries(ctx context.Context, filter LibraryFilter) ([]Library, error) {
	query := `SELECT DISTINCT l.id, l.purl, l.name, l.ecosystem, l.github_repo, l.status, l.upload_batch_id
	          FROM libraries l`
	var (
		conditions []string
		args       []any
	)

	if filter.Category != "" {
		query += ` JOIN library_categories lc ON lc.library_id = l.id
		           JOIN categories c ON c.id = lc.category_id`
		args = append(args, filter.Category)
		conditions = append(conditions, fmt.Sprintf("c.name = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("l.status = $%d", len(args)))
	}
	if filter.Ecosystem != "" {
		args = append(args, filter.Ecosystem)
		conditions = append(conditions, fmt.Sprintf("l.ecosystem = $%d", len(args)))
	}
	if filter.BatchID != "" {
		args = append(args, filter.BatchID)
		conditions = append(conditions, fmt.Sprintf("l.upload_batch_id = $%d", len(args)))
	}

	for i, cond := range conditions {
		if i == 0 {
			query += " WHERE " + cond
		} else {
			query += " AND " + cond
		}
	}
	query += " ORDER BY l.purl"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to list libraries: %w", err)
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
		return nil, fmt.Errorf("catalog: failed to list libraries: %w", err)
	}

	return libraries, nil
}

// UpdateLibraryStatus sets a library's status and, when githubRepo is
// non-nil, its github_repo column too (a nil githubRepo leaves the existing
// value untouched, so callers that haven't resolved a repo yet don't need
// to pass the column's current value back).
func (s *Store) UpdateLibraryStatus(ctx context.Context, id string, status string, githubRepo *string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE libraries
		 SET status = $1,
		     github_repo = COALESCE($2, github_repo),
		     updated_at = now()
		 WHERE id = $3`,
		status, githubRepo, id,
	)
	if err != nil {
		return fmt.Errorf("catalog: failed to update library status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("catalog: no library found with id %q", id)
	}

	return nil
}

// AttachCategory upserts a (library, category) attachment, overwriting the
// recorded source on a repeat call rather than erroring — re-running the
// categorization pipeline, or a manual correction, should be idempotent.
func (s *Store) AttachCategory(ctx context.Context, libraryID, categoryID string, source string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO library_categories (library_id, category_id, source)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (library_id, category_id) DO UPDATE SET source = EXCLUDED.source`,
		libraryID, categoryID, source,
	)
	if err != nil {
		return fmt.Errorf("catalog: failed to attach category: %w", err)
	}

	return nil
}

// BatchProgress returns the count of libraries in each status for batchID.
func (s *Store) BatchProgress(ctx context.Context, batchID string) (BatchProgress, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT status, count(*) FROM libraries WHERE upload_batch_id = $1 GROUP BY status`,
		batchID,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to get batch progress: %w", err)
	}
	defer rows.Close()

	progress := BatchProgress{}
	for rows.Next() {
		var (
			status string
			count  int
		)
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan batch progress: %w", err)
		}
		progress[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to get batch progress: %w", err)
	}

	return progress, nil
}
