package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func testAccounts(t *testing.T, password string) Accounts {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() unexpected error: %v", err)
	}
	return Accounts{
		"admin@example.com": {Role: RoleAdmin, PasswordHash: string(hash)},
	}
}

func loginRequestBody(t *testing.T, email, password string) *bytes.Reader {
	body, err := json.Marshal(loginRequest{Email: email, Password: password})
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}
	return bytes.NewReader(body)
}

func TestLoginHandler_CorrectCredentials(t *testing.T) {
	setSessionSecret(t)
	accounts := testAccounts(t, "correct-password")

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin@example.com", "correct-password"))
	rec := httptest.NewRecorder()

	LoginHandler(accounts)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Email != "admin@example.com" || resp.Role != RoleAdmin {
		t.Errorf("response = %+v, want Email=admin@example.com Role=%s", resp, RoleAdmin)
	}

	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s cookie to be set on success", SessionCookieName)
	}
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	setSessionSecret(t)
	accounts := testAccounts(t, "correct-password")

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin@example.com", "wrong-password"))
	rec := httptest.NewRecorder()

	LoginHandler(accounts)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandler_UnknownEmail(t *testing.T) {
	setSessionSecret(t)
	accounts := testAccounts(t, "correct-password")

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "nobody@example.com", "correct-password"))
	rec := httptest.NewRecorder()

	LoginHandler(accounts)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogoutHandler_ClearsSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	rec := httptest.NewRecorder()

	LogoutHandler()(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

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
