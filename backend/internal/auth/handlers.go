package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/tswierszcz/shelfmark/backend/internal/obs"
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

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

// LoginHandler returns a handler for POST /api/login: it verifies the
// submitted credentials against store and, on success, issues a session
// cookie and returns the account's email and role as JSON.
func LoginHandler(store *UsersStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<10)

		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeInvalidCredentials(w)
			return
		}

		account, found, err := store.GetByEmail(r.Context(), req.Email)
		if err != nil {
			writeUserError(w, r, http.StatusInternalServerError, "could not verify credentials", err)
			return
		}
		if !found {
			writeInvalidCredentials(w)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(req.Password)); err != nil {
			writeInvalidCredentials(w)
			return
		}

		if err := IssueSession(w, r, req.Email, account.Role); err != nil {
			writeUserError(w, r, http.StatusInternalServerError, "could not issue session", err)
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

// CreateUserHandler returns a handler for POST /api/users: it decodes an
// email/password/role payload, validates it, hashes the password, and
// persists a new account. It responds 400 on invalid input, 409 when the
// email is already taken, and 204 on success.
func CreateUserHandler(store *UsersStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<10)

		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeUserError(w, r, http.StatusBadRequest, "invalid request body", err)
			return
		}

		if req.Email == "" || req.Password == "" {
			writeUserError(w, r, http.StatusBadRequest, "email and password are required", nil)
			return
		}
		if len(req.Password) < 8 {
			writeUserError(w, r, http.StatusBadRequest, "password must be at least 8 characters", nil)
			return
		}
		switch req.Role {
		case RoleAdmin, RoleArchitect, RoleViewer:
		default:
			writeUserError(w, r, http.StatusBadRequest, "role must be one of admin, architect, viewer", nil)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeUserError(w, r, http.StatusInternalServerError, "could not hash password", err)
			return
		}

		if err := store.Create(r.Context(), req.Email, string(hash), req.Role); err != nil {
			if errors.Is(err, ErrEmailTaken) {
				writeUserError(w, r, http.StatusConflict, "email already in use", err)
				return
			}
			writeUserError(w, r, http.StatusInternalServerError, "could not create user", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ListUsersHandler returns a handler for GET /api/users: it returns every
// account's email and role as a JSON array.
func ListUsersHandler(store *UsersStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := store.List(r.Context())
		if err != nil {
			writeUserError(w, r, http.StatusInternalServerError, "could not list users", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(users); err != nil {
			obs.LogWarn(r, "failed to encode response", err)
		}
	}
}

// writeUserError writes msg as the JSON error body with the given status. A
// 5xx also logs msg and err with the request's attributes; 4xx responses are
// not logged. Callers must never pass the request body, email or password as
// part of err or msg.
func writeUserError(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	if status >= http.StatusInternalServerError {
		obs.LogError(r, msg, err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

func writeInvalidCredentials(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(errorResponse{Error: "invalid credentials"})
}
