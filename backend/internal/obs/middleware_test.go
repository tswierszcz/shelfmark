package obs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// logSink is a mutex-guarded writer that collects JSON log lines.
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

// Find returns the lines whose msg equals msg and whose attributes match every
// key/value pair in attrs (values compared via fmt.Sprint).
func (s *logSink) Find(msg string, attrs ...any) []map[string]any {
	found := make([]map[string]any, 0)
	for _, line := range s.Lines() {
		if line["msg"] != msg {
			continue
		}
		match := true
		for i := 0; i+1 < len(attrs); i += 2 {
			key := fmt.Sprint(attrs[i])
			if fmt.Sprint(line[key]) != fmt.Sprint(attrs[i+1]) {
				match = false
				break
			}
		}
		if match {
			found = append(found, line)
		}
	}
	return found
}

// captureLogs installs a JSON logger over a sink as the slog default for the
// duration of the test.
func captureLogs(t *testing.T) *logSink {
	t.Helper()
	sink := &logSink{}
	prev := slog.Default()
	slog.SetDefault(NewLogger(sink))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return sink
}

func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/x", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})
	mux.HandleFunc("GET /assets/ok.js", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /assets/broken.js", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	return mux
}

func TestMiddleware_AccessLog(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantLine   bool
		wantRoute  string
	}{
		{name: "api path 200 is logged", path: "/api/x", wantStatus: http.StatusOK, wantLine: true, wantRoute: "GET /api/x"},
		{name: "asset 200 is not logged", path: "/assets/ok.js", wantStatus: http.StatusOK, wantLine: false},
		{name: "asset 500 is logged", path: "/assets/broken.js", wantStatus: http.StatusInternalServerError, wantLine: true, wantRoute: "GET /assets/broken.js"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sink := captureLogs(t)
			handler := Middleware(newTestMux())

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			id := rec.Header().Get("X-Request-Id")
			if len(id) != 16 {
				t.Fatalf("X-Request-Id = %q, want 16 hex chars", id)
			}

			lines := sink.Find("request", "request_id", id)
			if !tc.wantLine {
				if len(lines) != 0 {
					t.Fatalf("expected no access line, got %v", lines)
				}
				return
			}
			if len(lines) != 1 {
				t.Fatalf("expected one access line, got %d: %v", len(lines), sink.Lines())
			}
			line := lines[0]
			if line["method"] != http.MethodGet {
				t.Errorf("method = %v, want GET", line["method"])
			}
			if line["route"] != tc.wantRoute {
				t.Errorf("route = %v, want %q", line["route"], tc.wantRoute)
			}
			if line["status"] != float64(tc.wantStatus) {
				t.Errorf("status = %v, want %d", line["status"], tc.wantStatus)
			}
			if _, ok := line["duration_ms"]; !ok {
				t.Errorf("expected duration_ms in %v", line)
			}
			if line["service"] != "shelfmark" || line["version"] != Version {
				t.Errorf("expected service/version tags, got %v", line)
			}
		})
	}
}

func TestMiddleware_RecoversPanic(t *testing.T) {
	sink := captureLogs(t)
	handler := Middleware(newTestMux())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/boom", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if body["error"] != "internal server error" {
		t.Errorf("body = %v", body)
	}

	id := rec.Header().Get("X-Request-Id")
	panics := sink.Find("panic", "request_id", id)
	if len(panics) != 1 {
		t.Fatalf("expected one panic line, got %d: %v", len(panics), sink.Lines())
	}
	if panics[0]["level"] != "ERROR" {
		t.Errorf("panic level = %v, want ERROR", panics[0]["level"])
	}
	if panics[0]["panic"] != "kaboom" {
		t.Errorf("panic = %v, want kaboom", panics[0]["panic"])
	}
	if stack, _ := panics[0]["stack"].(string); stack == "" {
		t.Errorf("expected non-empty stack")
	}

	access := sink.Find("request", "request_id", id, "status", 500)
	if len(access) != 1 {
		t.Fatalf("expected one access line with status 500, got %d: %v", len(access), sink.Lines())
	}
}

func TestMiddleware_RequestID(t *testing.T) {
	tests := []struct {
		name   string
		header string
		reuse  bool
	}{
		{name: "valid Fly-Request-Id is reused", header: "01HABCDEF-fly-123", reuse: true},
		{name: "invalid Fly-Request-Id is replaced", header: "bad id;injected", reuse: false},
		{name: "overlong Fly-Request-Id is replaced", header: strings.Repeat("a", 65), reuse: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sink := captureLogs(t)
			handler := Middleware(newTestMux())

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
			req.Header.Set("Fly-Request-Id", tc.header)
			handler.ServeHTTP(rec, req)

			id := rec.Header().Get("X-Request-Id")
			if tc.reuse && id != tc.header {
				t.Fatalf("X-Request-Id = %q, want %q", id, tc.header)
			}
			if !tc.reuse && (id == tc.header || len(id) != 16) {
				t.Fatalf("X-Request-Id = %q, want a fresh 16-char id", id)
			}
			if got := sink.Find("request", "request_id", id); len(got) != 1 {
				t.Fatalf("expected one access line for %q, got %d", id, len(got))
			}
		})
	}
}

func TestRequestInfo_NilSafe(t *testing.T) {
	ri := FromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	ri.SetUser("admin", "abc")
	if attrs := ri.Attrs(); len(attrs) != 0 {
		t.Fatalf("Attrs on absent RequestInfo = %v, want empty", attrs)
	}
}

func TestLogError_IncludesRequestAttrsAndError(t *testing.T) {
	sink := captureLogs(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/fail", func(w http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).SetUser("admin", "0123456789ab")
		LogError(r, "fail: doing thing", fmt.Errorf("outer: %w", fmt.Errorf("inner")), "library_id", 7)
		w.WriteHeader(http.StatusInternalServerError)
	})

	rec := httptest.NewRecorder()
	Middleware(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/fail", nil))

	id := rec.Header().Get("X-Request-Id")
	lines := sink.Find("fail: doing thing", "request_id", id)
	if len(lines) != 1 {
		t.Fatalf("expected one error line, got %v", sink.Lines())
	}
	line := lines[0]
	if line["level"] != "ERROR" || line["error"] != "outer: inner" || line["route"] != "GET /api/fail" ||
		line["role"] != "admin" || line["user"] != "0123456789ab" || line["library_id"] != float64(7) {
		t.Errorf("unexpected error line: %v", line)
	}
}
