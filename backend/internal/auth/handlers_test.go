package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/tswierszcz/shelfmark/backend/internal/db"
	"github.com/tswierszcz/shelfmark/backend/internal/obs"
)

// startTestPostgres starts a throwaway Postgres container via the docker CLI
// and returns a connected, migrated *pgxpool.Pool, plus registers cleanup
// via t.Cleanup that closes the pool and stops/removes the container. The
// test is skipped (not failed) if Docker isn't available.
//
// This mirrors catalog/store_test.go's helper of the same shape (internal/db's
// own version is unexported from that package, so it can't be reused
// directly); kept intentionally small rather than factored into a shared
// test-helper package for just two callers.
func startTestPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available, skipping integration test")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not reachable, skipping integration test")
	}

	port, err := findFreePort()
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}

	containerName := fmt.Sprintf("shelfmark-auth-test-%d", time.Now().UnixNano())
	runArgs := []string{
		"run", "--rm", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:5432", port),
		"-e", "POSTGRES_PASSWORD=postgres",
		"--tmpfs", "/var/lib/postgresql/data",
		"postgres:16-alpine",
	}
	out, err := exec.Command("docker", runArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed: %v\n%s", err, out)
	}

	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", "-v", containerName).Run()
	})

	databaseURL := fmt.Sprintf("postgres://postgres:postgres@localhost:%d/postgres?sslmode=disable", port)
	t.Setenv("DATABASE_URL", databaseURL)

	pool := waitForPostgres(t, databaseURL)

	if err := db.Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPostgres(t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("postgres did not become ready in time: %v", lastErr)
		default:
		}

		pool, err := db.Connect(ctx)
		if err == nil {
			return pool
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
}

// testUsersStore returns a *UsersStore backed by a fresh, migrated test
// database, pre-populated with a single admin@example.com account hashed
// from password via bcrypt.
func testUsersStore(t *testing.T, password string) *UsersStore {
	t.Helper()

	pool := startTestPostgres(t)
	store := NewUsersStore(pool)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() unexpected error: %v", err)
	}
	if err := store.Create(context.Background(), "admin@example.com", string(hash), RoleAdmin); err != nil {
		t.Fatalf("store.Create() unexpected error: %v", err)
	}

	return store
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
	store := testUsersStore(t, "correct-password")

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin@example.com", "correct-password"))
	rec := httptest.NewRecorder()

	LoginHandler(store)(rec, req)

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
	store := testUsersStore(t, "correct-password")

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin@example.com", "wrong-password"))
	rec := httptest.NewRecorder()

	LoginHandler(store)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandler_UnknownEmail(t *testing.T) {
	setSessionSecret(t)
	store := testUsersStore(t, "correct-password")

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "nobody@example.com", "correct-password"))
	rec := httptest.NewRecorder()

	LoginHandler(store)(rec, req)

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

func TestListUsersHandler_NoUsersReturnsEmptyArrayNotNull(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewUsersStore(pool)

	// An admin account always exists in practice (seeded on first boot via
	// SeedFromEnvIfEmpty), but the function itself shouldn't rely on that;
	// force the zero-row condition directly. This test's own throwaway
	// container isolates the DELETE from every other test.
	if _, err := pool.Exec(context.Background(), `DELETE FROM users`); err != nil {
		t.Fatalf("failed to clear users: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()

	ListUsersHandler(store)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// A nil slice would marshal to the literal "null\n" instead, which a
	// frontend .map() over the decoded response would crash on.
	if rec.Body.String() != "[]\n" {
		t.Fatalf("expected body %q, got %q", "[]\n", rec.Body.String())
	}
}

func TestListUsers_5xx_LogsCause(t *testing.T) {
	pool := startTestPostgres(t)
	store := NewUsersStore(pool)
	// A closed pool makes the List query fail; Close is idempotent, so the
	// helper's own t.Cleanup(pool.Close) is still fine.
	pool.Close()

	sink := captureLogs(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/users", ListUsersHandler(store))
	handler := obs.Middleware(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body.Error != "could not list users" {
		t.Fatalf("expected unchanged client message, got %q", body.Error)
	}

	requestID := rec.Header().Get("X-Request-Id")
	lines := sink.Find("could not list users", "level", "ERROR", "request_id", requestID, "route", "GET /api/users")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one Error line for request %s, got %d: %v", requestID, len(lines), sink.Lines())
	}
	if errAttr, _ := lines[0]["error"].(string); !strings.Contains(errAttr, "closed pool") {
		t.Fatalf("expected error attr to carry the pgx cause, got %q", errAttr)
	}
}
