package auth

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setSessionSecret(t *testing.T) {
	t.Setenv("SESSION_SECRET", "test-secret-value")
}

// issueAndExtractCookie calls IssueSession against a test recorder and
// returns the resulting cookie.
func issueAndExtractCookie(t *testing.T, email string, role Role) *http.Cookie {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)

	if err := IssueSession(rec, req, email, role); err != nil {
		t.Fatalf("IssueSession() unexpected error: %v", err)
	}

	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			return c
		}
	}
	t.Fatalf("expected %s cookie to be set", SessionCookieName)
	return nil
}

func requestWithCookie(cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	return req
}

func TestIssueSession_RoundTripsEmailAndRole(t *testing.T) {
	setSessionSecret(t)

	cookie := issueAndExtractCookie(t, "user@example.com", RoleAdmin)

	if !cookie.HttpOnly {
		t.Errorf("expected cookie to be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", cookie.SameSite)
	}
	if cookie.Secure {
		t.Errorf("expected Secure=false for non-TLS request")
	}

	session, err := VerifySession(requestWithCookie(cookie))
	if err != nil {
		t.Fatalf("VerifySession() unexpected error: %v", err)
	}
	if session.Email != "user@example.com" || session.Role != RoleAdmin {
		t.Errorf("session = %+v, want Email=user@example.com Role=%s", session, RoleAdmin)
	}
}

func TestIssueSession_SecureOverHTTPS(t *testing.T) {
	setSessionSecret(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	req.TLS = &tls.ConnectionState{}

	if err := IssueSession(rec, req, "user@example.com", RoleViewer); err != nil {
		t.Fatalf("IssueSession() unexpected error: %v", err)
	}

	cookies := rec.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			found = c
		}
	}
	if found == nil {
		t.Fatalf("expected session cookie to be set")
	}
	if !found.Secure {
		t.Errorf("expected Secure=true for HTTPS request")
	}
}

func TestVerifySession_MissingCookie(t *testing.T) {
	setSessionSecret(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := VerifySession(req); err == nil {
		t.Fatalf("VerifySession() expected error for missing cookie, got nil")
	}
}

func TestVerifySession_TamperedSignature(t *testing.T) {
	setSessionSecret(t)

	cookie := issueAndExtractCookie(t, "user@example.com", RoleAdmin)

	// Flip the last character of the signature to corrupt it.
	tampered := *cookie
	if len(tampered.Value) == 0 {
		t.Fatalf("expected non-empty cookie value")
	}
	tampered.Value = tampered.Value[:len(tampered.Value)-1] + flipChar(tampered.Value[len(tampered.Value)-1])

	if _, err := VerifySession(requestWithCookie(&tampered)); err == nil {
		t.Fatalf("VerifySession() expected error for tampered signature, got nil")
	}
}

func TestVerifySession_ExpiredSession(t *testing.T) {
	setSessionSecret(t)

	secret, err := sessionSecret()
	if err != nil {
		t.Fatalf("sessionSecret() unexpected error: %v", err)
	}

	expiredPayload := fmt.Sprintf("%s|%s|%d", "user@example.com", RoleViewer, time.Now().Add(-1*time.Hour).Unix())
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(expiredPayload))
	signature := signPayload(secret, expiredPayload)
	value := encodedPayload + "." + signature

	cookie := &http.Cookie{Name: SessionCookieName, Value: value}

	_, err = VerifySession(requestWithCookie(cookie))
	if err == nil {
		t.Fatalf("VerifySession() expected error for expired session, got nil")
	}
}

func TestClearSession(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearSession(rec, httptest.NewRequest(http.MethodPost, "/api/logout", nil))

	cookies := rec.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			found = c
		}
	}
	if found == nil {
		t.Fatalf("expected %s cookie to be set", SessionCookieName)
	}
	if found.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1, got %d", found.MaxAge)
	}
}

func TestSessionCookie_SecureBehindProxy(t *testing.T) {
	setSessionSecret(t)

	tests := []struct {
		name           string
		tls            bool
		forwardedProto string
		wantSecure     bool
	}{
		{name: "plain HTTP, no header", wantSecure: false},
		{name: "proxy forwards https", forwardedProto: "https", wantSecure: true},
		{name: "proxy forwards http", forwardedProto: "http", wantSecure: false},
		{name: "direct TLS", tls: true, wantSecure: true},
	}

	cookies := []struct {
		name  string
		write func(t *testing.T, w http.ResponseWriter, r *http.Request)
	}{
		{name: "login", write: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
			if err := IssueSession(w, r, "user@example.com", RoleViewer); err != nil {
				t.Fatalf("IssueSession() unexpected error: %v", err)
			}
		}},
		{name: "logout", write: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
			ClearSession(w, r)
		}},
	}

	for _, tt := range tests {
		for _, c := range cookies {
			t.Run(tt.name+"/"+c.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
				if tt.tls {
					req.TLS = &tls.ConnectionState{}
				}
				if tt.forwardedProto != "" {
					req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
				}
				rec := httptest.NewRecorder()

				c.write(t, rec, req)

				var found *http.Cookie
				for _, ck := range rec.Result().Cookies() {
					if ck.Name == SessionCookieName {
						found = ck
					}
				}
				if found == nil {
					t.Fatalf("expected %s cookie to be set", SessionCookieName)
				}
				if found.Secure != tt.wantSecure {
					t.Errorf("Secure = %v, want %v", found.Secure, tt.wantSecure)
				}
			})
		}
	}
}

func flipChar(b byte) string {
	if b == 'a' {
		return "b"
	}
	return "a"
}
