package db

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tswierszcz/shelfmark/backend/internal/obs"
	"github.com/tswierszcz/shelfmark/backend/internal/obs/obstest"
)

// captureLogs installs a capturing JSON logger for the duration of the test.
func captureLogs(t *testing.T) *obstest.Sink { return obstest.CaptureWith(t, obs.NewLogger) }

func TestHealthCheck_DBDown_Logs(t *testing.T) {
	// pgxpool.New connects lazily, so no database is needed: closing the pool
	// makes every Ping fail the same way a closed production pool would.
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:postgres@127.0.0.1:1/postgres?sslmode=disable")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	pool.Close()

	sink := captureLogs(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	HealthCheck(pool)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if rec.Body.String() != "db unreachable" {
		t.Fatalf("expected unchanged body %q, got %q", "db unreachable", rec.Body.String())
	}

	var found []map[string]any
	for _, line := range sink.Lines() {
		if line["msg"] == "healthz: db ping failed" && line["level"] == "ERROR" {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one Error line, got %d: %v", len(found), sink.Lines())
	}
	if errAttr, _ := found[0]["error"].(string); errAttr == "" {
		t.Fatalf("expected a non-empty error attr, got %v", found[0])
	}
}
