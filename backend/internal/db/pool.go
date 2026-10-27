package db

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pgx connection pool using DATABASE_URL, failing fast if the
// env var is missing or the connection cannot be established — matching the
// LoadAccounts fail-fast convention from internal/auth/accounts.go.
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("db: missing required env var DATABASE_URL")
	}

	// pgx only redacts passwords in parse errors on a best-effort basis, so
	// the parse error is deliberately dropped rather than wrapped.
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("db: failed to parse DATABASE_URL")
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("db: failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: failed to connect to database: %w", err)
	}

	return pool, nil
}
