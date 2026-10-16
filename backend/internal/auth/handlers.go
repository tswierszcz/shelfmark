package auth

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Email string `json:"email"`
	Role  Role   `json:"role"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type meResponse struct {
	Email string `json:"email"`
	Role  Role   `json:"role"`
}

// LoginHandler returns a handler for POST /api/login: it verifies the
// submitted credentials against accounts and, on success, issues a session
// cookie and returns the account's email and role as JSON.
func LoginHandler(accounts Accounts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidCredentials(w)
			return
		}

		account, ok := accounts[req.Email]
		if !ok {
			writeInvalidCredentials(w)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(req.Password)); err != nil {
			writeInvalidCredentials(w)
			return
		}

		if err := IssueSession(w, r, req.Email, account.Role); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(errorResponse{Error: "could not issue session"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{Email: req.Email, Role: account.Role})
	}
}

// LogoutHandler returns a handler for POST /api/logout: it clears the
// session cookie and responds with 204 No Content.
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ClearSession(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

// MeHandler returns a handler for GET /api/me: it reports the email and role
// of the caller's verified session. It is expected to be wrapped with
// RequireRole so a Session is always present in the request context.
func MeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := SessionFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meResponse{Email: session.Email, Role: session.Role})
	}
}

func writeInvalidCredentials(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(errorResponse{Error: "invalid credentials"})
}
