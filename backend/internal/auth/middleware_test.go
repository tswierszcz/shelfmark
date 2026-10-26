package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tswierszcz/shelfmark/backend/internal/obs"
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
			if fmt.Sprint(line[fmt.Sprint(attrs[i])]) != fmt.Sprint(attrs[i+1]) {
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
	slog.SetDefault(obs.NewLogger(sink))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return sink
}

func withSessionSecret(t *testing.T) {
	t.Helper()
	t.Setenv("SESSION_SECRET", "test-secret")
}

func issueTestSession(t *testing.T, email string, role Role) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := IssueSession(rec, req, email, role); err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("IssueSession did not set a cookie")
	}
	return cookies[0]
}

func TestRequireRole_AllowedRolePassesThrough(t *testing.T) {
	withSessionSecret(t)

	cookie := issueTestSession(t, "admin@example.com", RoleAdmin)

	called := false
	handler := RequireRole(RoleAdmin)(func(w http.ResponseWriter, r *http.Request) {
		called = true
		session, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatalf("expected session in context")
		}
		if session.Email != "admin@example.com" || session.Role != RoleAdmin {
			t.Fatalf("unexpected session: %+v", session)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if !called {
		t.Fatalf("expected wrapped handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestRequireRole_NoCookieReturns401(t *testing.T) {
	withSessionSecret(t)

	handler := RequireRole(RoleAdmin)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("wrapped handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestRequireRole_InvalidCookieReturns401(t *testing.T) {
	withSessionSecret(t)

	handler := RequireRole(RoleAdmin)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("wrapped handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "not-a-valid-session"})
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestRequireRole_ExpiredCookieReturns401(t *testing.T) {
	withSessionSecret(t)

	secret, err := sessionSecret()
	if err != nil {
		t.Fatalf("sessionSecret: %v", err)
	}
	payload := "viewer@example.com|viewer|1"
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := signPayload(secret, payload)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: encodedPayload + "." + signature})
	rec := httptest.NewRecorder()

	handler := RequireRole(RoleViewer)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("wrapped handler should not be called")
	})

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestRequireRole_ValidSessionWrongRoleReturns403(t *testing.T) {
	withSessionSecret(t)

	cookie := issueTestSession(t, "viewer@example.com", RoleViewer)

	handler := RequireRole(RoleAdmin)(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("wrapped handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestRequireRole_RecordsRoleAndHashedUserInAccessLog(t *testing.T) {
	withSessionSecret(t)
	sink := captureLogs(t)

	const email = "Admin@Example.com"
	cookie := issueTestSession(t, email, RoleAdmin)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/me", RequireRole(RoleAdmin)(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	obs.Middleware(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	id := rec.Header().Get("X-Request-Id")
	lines := sink.Find("request", "request_id", id)
	if len(lines) != 1 {
		t.Fatalf("expected one access line, got %v", sink.Lines())
	}
	line := lines[0]
	if line["role"] != string(RoleAdmin) {
		t.Errorf("role = %v, want %s", line["role"], RoleAdmin)
	}
	user, _ := line["user"].(string)
	if len(user) != 12 {
		t.Errorf("user = %q, want 12 hex chars", user)
	}
	if user != UserHash(email) {
		t.Errorf("user = %q, want UserHash(email) = %q", user, UserHash(email))
	}
	raw, _ := json.Marshal(line)
	if strings.Contains(strings.ToLower(string(raw)), "admin@example.com") {
		t.Errorf("access line leaks the email: %s", raw)
	}
}

func TestUserHash(t *testing.T) {
	t.Setenv("SESSION_SECRET", "")
	if got := UserHash("user@example.com"); got != "" {
		t.Fatalf("UserHash without secret = %q, want empty", got)
	}

	withSessionSecret(t)
	h := UserHash("user@example.com")
	if len(h) != 12 {
		t.Fatalf("UserHash length = %d, want 12", len(h))
	}
	if UserHash("USER@example.com") != h {
		t.Errorf("UserHash should be case-insensitive")
	}
	if UserHash("other@example.com") == h {
		t.Errorf("UserHash should differ for different emails")
	}
}

func TestRequireRole_WithoutObsMiddlewareDoesNotPanic(t *testing.T) {
	withSessionSecret(t)
	cookie := issueTestSession(t, "viewer@example.com", RoleViewer)

	handler := RequireRole(RoleViewer)(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestRequireRole_LogsSessionFailures(t *testing.T) {
	newHandler := func(t *testing.T) http.Handler {
		t.Helper()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/me", RequireRole(RoleAdmin)(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("wrapped handler should not be called")
		}))
		return obs.Middleware(mux)
	}

	// tamper flips the last character of the cookie's signature so the
	// payload stays well-formed but the HMAC no longer matches.
	tamper := func(c *http.Cookie) *http.Cookie {
		v := []byte(c.Value)
		if v[len(v)-1] == 'A' {
			v[len(v)-1] = 'B'
		} else {
			v[len(v)-1] = 'A'
		}
		return &http.Cookie{Name: c.Name, Value: string(v)}
	}

	tests := []struct {
		name      string
		setup     func(t *testing.T) *http.Cookie
		wantLevel string // "" means no line at all for this request
		wantMsg   string
	}{
		{
			name: "secret unset logs error",
			setup: func(t *testing.T) *http.Cookie {
				withSessionSecret(t)
				c := issueTestSession(t, "admin@example.com", RoleAdmin)
				t.Setenv("SESSION_SECRET", "")
				return c
			},
			wantLevel: "ERROR",
			wantMsg:   "session secret not configured",
		},
		{
			name: "tampered signature logs warn",
			setup: func(t *testing.T) *http.Cookie {
				withSessionSecret(t)
				return tamper(issueTestSession(t, "admin@example.com", RoleAdmin))
			},
			wantLevel: "WARN",
			wantMsg:   "session signature invalid",
		},
		{
			name: "no cookie logs nothing",
			setup: func(t *testing.T) *http.Cookie {
				withSessionSecret(t)
				return nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cookie := tc.setup(t)
			sink := captureLogs(t)

			req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			if cookie != nil {
				req.AddCookie(cookie)
			}
			rec := httptest.NewRecorder()

			newHandler(t).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("expected bare 401 with no body, got %q", rec.Body.String())
			}

			requestID := rec.Header().Get("X-Request-Id")
			var events []map[string]any
			for _, line := range sink.Lines() {
				if line["request_id"] == requestID && line["msg"] != "request" { // skip the access-log line
					events = append(events, line)
				}
			}

			if tc.wantLevel == "" {
				if len(events) != 0 {
					t.Fatalf("expected no log line, got %v", events)
				}
				return
			}
			if len(events) != 1 {
				t.Fatalf("expected exactly one log line, got %d: %v", len(events), events)
			}
			if events[0]["level"] != tc.wantLevel || events[0]["msg"] != tc.wantMsg {
				t.Fatalf("expected %s %q, got %v", tc.wantLevel, tc.wantMsg, events[0])
			}
		})
	}
}
