// Package catalog provides the data model and store layer for Shelfmark's
// library catalog: a curated capability taxonomy, the libraries seeded into
// it, and the (library, category) attachments produced either by the
// automated categorization pipeline or by manual Admin correction.
package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrStrategicMarkNotFound indicates the (library, category) pair targeted
// by UnmarkStrategic has no active strategic mark to deactivate.
var ErrStrategicMarkNotFound = errors.New("catalog: no active strategic mark found")

// ErrInvalidReference indicates MarkStrategic was called with a libraryID or
// categoryID that doesn't exist, surfaced by strategic_marks' foreign key
// constraints.
var ErrInvalidReference = errors.New("catalog: library or category does not exist")

// ErrStrategicCapReached indicates MarkStrategic was called for a 3rd
// distinct library within the same (category, ecosystem) pair, exceeding
// FR-007's cap of two strategic libraries per category/ecosystem.
var ErrStrategicCapReached = errors.New("catalog: strategic cap reached for this category and ecosystem")

// Category is one entry in the curated taxonomy seeded by migration
// 0002_catalog_schema.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	ID            string  `json:"id"`
	Purl          string  `json:"purl"`
	Name          string  `json:"name"`
	Ecosystem     string  `json:"ecosystem"`
	GithubRepo    *string `json:"github_repo"`
	Status        string  `json:"status"`
	UploadBatchID string  `json:"upload_batch_id"`
	IsDirect      bool    `json:"is_direct"`
}

// LibraryFilter narrows ListLibraries. Every field is optional (zero value
// means "don't filter on this"). Strategic is a tri-state: nil means "don't
// filter on strategic status", true/false means "has/has-not at least one
// active strategic_marks row".
type LibraryFilter struct {
	Status    string
	Category  string
	Ecosystem string
	BatchID   string
	Strategic *bool
}

// StrategicMark is one active strategic designation of a library against a
// category, as returned by StrategicMarksByLibrary.
type StrategicMark struct {
	CategoryID    string `json:"category_id"`
	CategoryName  string `json:"category_name"`
	Justification string `json:"justification"`
	MarkedBy      string `json:"marked_by"`
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
	query := `SELECT DISTINCT l.id, l.purl, l.name, l.ecosystem, l.github_repo, l.status, l.upload_batch_id,
	          EXISTS (SELECT 1 FROM project_libraries pl WHERE pl.library_id = l.id AND pl.is_direct) AS is_direct
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
	if filter.Strategic != nil {
		existsSubquery := `EXISTS (SELECT 1 FROM strategic_marks sm WHERE sm.library_id = l.id AND sm.active)`
		if *filter.Strategic {
			conditions = append(conditions, existsSubquery)
		} else {
			conditions = append(conditions, "NOT "+existsSubquery)
		}
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
		if err := rows.Scan(&l.ID, &l.Purl, &l.Name, &l.Ecosystem, &l.GithubRepo, &l.Status, &l.UploadBatchID, &l.IsDirect); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan library: %w", err)
		}
		libraries = append(libraries, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to list libraries: %w", err)
	}

	return libraries, nil
}

// CategoryNamesByLibrary returns the names of categories attached to each
// library in libraryIDs, keyed by library ID. A library with no attachments
// is simply absent from the returned map (callers should treat a missing
// key the same as an empty slice).
func (s *Store) CategoryNamesByLibrary(ctx context.Context, libraryIDs []string) (map[string][]string, error) {
	result := map[string][]string{}
	if len(libraryIDs) == 0 {
		return result, nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT lc.library_id, c.name
		 FROM library_categories lc
		 JOIN categories c ON c.id = lc.category_id
		 WHERE lc.library_id::text = ANY($1)
		 ORDER BY c.name`,
		libraryIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to list categories by library: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			libraryID string
			name      string
		)
		if err := rows.Scan(&libraryID, &name); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan library category: %w", err)
		}
		result[libraryID] = append(result[libraryID], name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to list categories by library: %w", err)
	}

	return result, nil
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

// MarkStrategic upserts an active strategic designation of libraryID against
// categoryID, overwriting the recorded justification/markedBy/markedAt on a
// repeat call rather than erroring, following AttachCategory's upsert idiom.
func (s *Store) MarkStrategic(ctx context.Context, libraryID, categoryID, justification, markedBy string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("catalog: failed to begin mark strategic: %w", err)
	}
	defer tx.Rollback(ctx)

	var ecosystem string
	if err := tx.QueryRow(ctx, `SELECT ecosystem FROM libraries WHERE id = $1`, libraryID).Scan(&ecosystem); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("catalog: library %q or category %q does not exist: %w", libraryID, categoryID, ErrInvalidReference)
		}
		return fmt.Errorf("catalog: failed to look up library ecosystem: %w", err)
	}

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(DISTINCT sm.library_id)
		 FROM strategic_marks sm
		 JOIN libraries l ON l.id = sm.library_id
		 WHERE sm.active AND sm.category_id = $1 AND l.ecosystem = $2 AND sm.library_id != $3`,
		categoryID, ecosystem, libraryID,
	).Scan(&count); err != nil {
		return fmt.Errorf("catalog: failed to count strategic marks: %w", err)
	}
	if count >= 2 {
		return ErrStrategicCapReached
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO strategic_marks (library_id, category_id, active, justification, marked_by)
		 VALUES ($1, $2, true, $3, $4)
		 ON CONFLICT (library_id, category_id) DO UPDATE
		 SET active = true, justification = EXCLUDED.justification, marked_by = EXCLUDED.marked_by, marked_at = now()`,
		libraryID, categoryID, justification, markedBy,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return fmt.Errorf("catalog: library %q or category %q does not exist: %w", libraryID, categoryID, ErrInvalidReference)
		}
		return fmt.Errorf("catalog: failed to mark strategic: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("catalog: failed to commit mark strategic: %w", err)
	}

	return nil
}

// UnmarkStrategic deactivates an existing active strategic designation of
// libraryID against categoryID, recording the reason via justification and
// markedBy. It returns an error if the (library, category) pair is not
// currently marked strategic, matching UpdateLibraryStatus's not-found
// convention.
func (s *Store) UnmarkStrategic(ctx context.Context, libraryID, categoryID, justification, markedBy string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE strategic_marks
		 SET active = false, justification = $3, marked_by = $4, marked_at = now()
		 WHERE library_id = $1 AND category_id = $2 AND active = true`,
		libraryID, categoryID, justification, markedBy,
	)
	if err != nil {
		return fmt.Errorf("catalog: failed to unmark strategic: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("catalog: no active strategic mark found for library %q, category %q: %w", libraryID, categoryID, ErrStrategicMarkNotFound)
	}

	return nil
}

// StrategicMarksByLibrary returns the active strategic marks for each
// library in libraryIDs, keyed by library ID. A library with no active marks
// is simply absent from the returned map (callers should treat a missing key
// the same as an empty slice), mirroring CategoryNamesByLibrary's contract.
func (s *Store) StrategicMarksByLibrary(ctx context.Context, libraryIDs []string) (map[string][]StrategicMark, error) {
	result := map[string][]StrategicMark{}
	if len(libraryIDs) == 0 {
		return result, nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT sm.library_id, sm.category_id, c.name, sm.justification, sm.marked_by
		 FROM strategic_marks sm
		 JOIN categories c ON c.id = sm.category_id
		 WHERE sm.active AND sm.library_id::text = ANY($1)
		 ORDER BY c.name`,
		libraryIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog: failed to list strategic marks by library: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			libraryID string
			mark      StrategicMark
		)
		if err := rows.Scan(&libraryID, &mark.CategoryID, &mark.CategoryName, &mark.Justification, &mark.MarkedBy); err != nil {
			return nil, fmt.Errorf("catalog: failed to scan strategic mark: %w", err)
		}
		result[libraryID] = append(result[libraryID], mark)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: failed to list strategic marks by library: %w", err)
	}

	return result, nil
}
