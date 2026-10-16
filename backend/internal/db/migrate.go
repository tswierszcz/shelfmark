package db

import (
	"embed"
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5" database driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations
var migrationsFS embed.FS

// Migrate applies any pending migrations embedded in the migrations/
// directory against the database identified by databaseURL, using
// golang-migrate. golang-migrate manages its own connections internally
// (it does not share the pgxpool.Pool used for the app's own queries), so
// this takes the connection string directly rather than an existing pool.
// migrate.ErrNoChange (no pending migrations) is treated as success.
func Migrate(databaseURL string) error {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("db: failed to load embedded migrations: %w", err)
	}

	dbURL, err := pgx5DatabaseURL(databaseURL)
	if err != nil {
		return fmt.Errorf("db: failed to parse DATABASE_URL: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, dbURL)
	if err != nil {
		return fmt.Errorf("db: failed to initialize migration runner: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: failed to apply migrations: %w", err)
	}

	return nil
}

// pgx5DatabaseURL rewrites databaseURL's scheme so golang-migrate resolves
// it to the pgx/v5 database driver (registered under "pgx5"), which itself
// rewrites the scheme to "postgres" before connecting.
func pgx5DatabaseURL(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	u.Scheme = "pgx5"
	return u.String(), nil
}
