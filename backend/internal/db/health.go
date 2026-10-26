package db

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthCheck returns an http.HandlerFunc that verifies the database is
// actually reachable by pinging pool with a short-timeout context. On
// success it writes 200 "ok" (same body as the previous no-op handler, to
// avoid changing the happy-path contract); on failure it writes 503 with a
// short error body.
func HealthCheck(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			slog.Error("healthz: db ping failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("db unreachable"))
			return
		}

		_, _ = w.Write([]byte("ok"))
	}
}
