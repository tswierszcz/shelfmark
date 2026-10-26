// Package authtest issues real signed session cookies for tests in other
// packages, so handler and route tests can exercise auth.RequireRole-wrapped
// handlers without a real login. Test-support code: import it only from
// _test.go files.
package authtest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
)

// testSessionSecret is the signing key SessionCookie sets when the test has
// not configured SESSION_SECRET itself.
const testSessionSecret = "authtest-session-secret"

// SessionCookie issues a signed session for email and role via
// auth.IssueSession and returns the session cookie. If SESSION_SECRET is
// unset it is set for the duration of the test via t.Setenv, so callers must
// not use t.Parallel.
func SessionCookie(t testing.TB, email string, role auth.Role) *http.Cookie {
	t.Helper()

	if os.Getenv("SESSION_SECRET") == "" {
		t.Setenv("SESSION_SECRET", testSessionSecret)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	if err := auth.IssueSession(rec, req, email, role); err != nil {
		t.Fatalf("IssueSession(%s, %s): %v", email, role, err)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatalf("IssueSession(%s, %s) set no %s cookie", email, role, auth.SessionCookieName)
	return nil
}
