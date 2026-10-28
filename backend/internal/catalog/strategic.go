package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrStrategicMarkNotFound indicates the (library, category) pair targeted
// by UnmarkStrategic has no active strategic mark to deactivate.
var ErrStrategicMarkNotFound = errors.New("catalog: no active strategic mark found")

// ErrStrategicCapReached indicates MarkStrategic was called for a 3rd
// distinct library within the same (category, ecosystem) pair, exceeding
// FR-007's cap of two strategic libraries per category/ecosystem.
var ErrStrategicCapReached = errors.New("catalog: strategic cap reached for this category and ecosystem")

// ErrStrategicMarkBusy indicates MarkStrategic gave up waiting for the
// per-(category, ecosystem) advisory lock after strategicLockTimeout because
// another mark in the same pair held it. The caller may retry.
var ErrStrategicMarkBusy = errors.New("catalog: strategic marking is busy, please retry")

// StrategicMark is one active strategic designation of a library against a
// category, as returned by StrategicMarksByLibrary.
type StrategicMark struct {
	CategoryID    string `json:"category_id"`
	CategoryName  string `json:"category_name"`
	Justification string `json:"justification"`
	MarkedBy      string `json:"marked_by"`
}

// afterCapCheck is a test seam: when non-nil, MarkStrategic calls it right
// after the cap check passes and before the insert, so tests can hold
// transactions inside the check-then-insert window. Always nil in production.
// Package-level state: tests that set it must not use t.Parallel.
var afterCapCheck func()

// strategicLockTimeout bounds how long MarkStrategic waits for the strategic
// cap advisory lock (a Postgres lock_timeout value). Package var so tests can
// shorten it; tests that set it must not use t.Parallel.
var strategicLockTimeout = "5s"

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

	// Bound the lock wait so a slow holder can't pin pool connections
	// indefinitely; SET LOCAL lasts only until this transaction ends.
	// strategicLockTimeout is a package constant-in-practice, not user input.
	if _, err := tx.Exec(ctx, fmt.Sprintf(`SET LOCAL lock_timeout = '%s'`, strategicLockTimeout)); err != nil {
		return fmt.Errorf("catalog: failed to set strategic lock timeout: %w", err)
	}

	// Serialize check-then-insert per (category, ecosystem): at READ
	// COMMITTED two concurrent marks would otherwise both count below the
	// cap and both insert. The xact lock is released on commit/rollback.
	// The key uses the ecosystem read above, before locking; that is correct
	// only while libraries.ecosystem is never updated (no UPDATE of it exists
	// today). If it ever becomes mutable, re-read it under the lock.
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`,
		categoryID, ecosystem,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.LockNotAvailable {
			return fmt.Errorf("catalog: timed out waiting for strategic cap lock: %w", ErrStrategicMarkBusy)
		}
		return fmt.Errorf("catalog: failed to lock strategic cap: %w", err)
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
	if afterCapCheck != nil {
		afterCapCheck()
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
