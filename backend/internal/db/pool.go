package db

import (
	"context"
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

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: failed to connect to database: %w", err)
	}

	return pool, nil
}
