package db

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tswierszcz/shelfmark/backend/internal/obs"
)

// logSink is a mutex-guarded writer that collects JSON log lines. It mirrors
// the helper of the same name in the auth, catalog and obs test packages.
type logSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

// Lines returns every captured log line decoded as a JSON object.
func (s *logSink) Lines() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines := make([]map[string]any, 0)
	for _, raw := range strings.Split(s.buf.String(), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			lines = append(lines, m)
		}
	}
	return lines
}

// captureLogs installs a JSON logger over a sink as the slog default for the
// duration of the test.
func captureLogs(t *testing.T) *logSink {
	t.Helper()
	sink := &logSink{}
	prev := slog.Default()
	slog.SetDefault(obs.NewLogger(sink))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return sink
}

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
