package db

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// startTestPostgres starts a throwaway Postgres container via the docker CLI
// and returns a DATABASE_URL pointing at it, plus registers a cleanup via
// t.Cleanup that stops and removes the container. The test is skipped (not
// failed) if Docker isn't available.
func startTestPostgres(t *testing.T) string {
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

	containerName := fmt.Sprintf("shelfmark-db-test-%d", time.Now().UnixNano())
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

	// Connect (used by waitForPostgres) reads DATABASE_URL from the
	// environment, so set it before polling for readiness.
	t.Setenv("DATABASE_URL", databaseURL)
	waitForPostgres(t, databaseURL)

	return databaseURL
}

// findFreePort asks the OS for an ephemeral port by briefly binding a
// listener and releasing it, so the Postgres container can bind it instead.
func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPostgres(t *testing.T, databaseURL string) {
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

		pool, err := Connect(ctx)
		if err == nil {
			pool.Close()
			return
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
}

func TestConnectAndMigrate_Integration(t *testing.T) {
	databaseURL := startTestPostgres(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer pool.Close()

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Running Migrate again should be a no-op (ErrNoChange treated as success).
	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("second Migrate call failed: %v", err)
	}

	var found int
	row := pool.QueryRow(ctx, "SELECT 1 FROM pg_extension WHERE extname = 'pgcrypto'")
	if err := row.Scan(&found); err != nil {
		t.Fatalf("expected pgcrypto extension to be present: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected to find pgcrypto extension, got %d rows", found)
	}
}

func TestMigrate_ParseErrorDoesNotLeakPassword(t *testing.T) {
	const (
		password = "s3cr3t-pw"
		host     = "db.example.internal"
	)
	// The non-numeric port makes url.Parse fail before any connection attempt.
	databaseURL := "postgres://app:" + password + "@" + host + ":notaport/shelfmark"

	err := Migrate(databaseURL)
	if err == nil {
		t.Fatal("Migrate() expected an error for an unparseable DATABASE_URL, got nil")
	}
	if got, want := err.Error(), "db: failed to parse DATABASE_URL"; got != want {
		t.Errorf("Migrate() error = %q, want %q", got, want)
	}
	for _, secret := range []string{password, host} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("Migrate() error leaks %q: %q", secret, err.Error())
		}
	}
}
