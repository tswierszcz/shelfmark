package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrEmailTaken indicates Create was called with an email that already has a
// users row, surfaced by the table's unique constraint on email.
var ErrEmailTaken = errors.New("auth: email already in use")

// UserSummary is a users row with the password omitted, for listing accounts
// without ever selecting the hash.
type UserSummary struct {
	Email string `json:"email"`
	Role  Role   `json:"role"`
}

// UsersStore wraps a shared *pgxpool.Pool with the SQL needed to persist
// accounts, matching catalog.Store's pattern of taking the pool directly but
// scoped to the auth package.
type UsersStore struct {
	pool *pgxpool.Pool
}

// NewUsersStore constructs a UsersStore from an existing connection pool.
func NewUsersStore(pool *pgxpool.Pool) *UsersStore {
	return &UsersStore{pool: pool}
}

// GetByEmail looks up the account with the given email. It returns
// found=false (with no error) when no such row exists, rather than treating
// that as an error condition.
func (s *UsersStore) GetByEmail(ctx context.Context, email string) (Account, bool, error) {
	var account Account
	row := s.pool.QueryRow(ctx, `SELECT password_hash, role FROM users WHERE email = $1`, email)
	if err := row.Scan(&account.PasswordHash, &account.Role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, false, nil
		}
		return Account{}, false, fmt.Errorf("auth: failed to look up user by email: %w", err)
	}

	return account, true, nil
}

// Create inserts a new users row. A duplicate email is reported as
// ErrEmailTaken rather than the raw constraint-violation error, following
// catalog/store.go's pgerrcode mapping convention.
func (s *UsersStore) Create(ctx context.Context, email, passwordHash string, role Role) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3)`,
		email, passwordHash, role,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrEmailTaken
		}
		return fmt.Errorf("auth: failed to create user: %w", err)
	}

	return nil
}

// List returns every account's email and role, ordered by email. The
// password hash is never selected.
func (s *UsersStore) List(ctx context.Context) ([]UserSummary, error) {
	rows, err := s.pool.Query(ctx, `SELECT email, role FROM users ORDER BY email`)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to list users: %w", err)
	}
	defer rows.Close()

	users := []UserSummary{}
	for rows.Next() {
		var u UserSummary
		if err := rows.Scan(&u.Email, &u.Role); err != nil {
			return nil, fmt.Errorf("auth: failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: failed to list users: %w", err)
	}

	return users, nil
}

// Count returns the number of rows in users.
func (s *UsersStore) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("auth: failed to count users: %w", err)
	}

	return count, nil
}

// SeedFromEnvIfEmpty bootstraps the users table from the ADMIN_/ARCHITECT_/
// VIEWER_ env vars exactly once: if the table already has rows, it returns
// immediately; otherwise it loads the 3 env-var accounts and creates each
// one, propagating a LoadAccounts error (missing env var) as fatal, since an
// empty table with no seed source would mean nobody could ever log in.
func (s *UsersStore) SeedFromEnvIfEmpty(ctx context.Context) error {
	count, err := s.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	accounts, err := LoadAccounts()
	if err != nil {
		return fmt.Errorf("auth: failed to load accounts for seeding: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("auth: failed to begin seed transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for email, account := range accounts {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3)`,
			email, account.PasswordHash, account.Role,
		); err != nil {
			// Identify the account by role, never by email, to keep PII out of logs.
			return fmt.Errorf("auth: failed to seed %s account: %w", account.Role, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("auth: failed to commit seed transaction: %w", err)
	}

	return nil
}
