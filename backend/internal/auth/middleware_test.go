package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
