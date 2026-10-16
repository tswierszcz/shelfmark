package db

import (
	"context"
	"os"
	"strings"
	"testing"
)

// withEnv sets key to value for the duration of the test and restores the
// previous value (or unsets it) afterward.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestConnect_MissingDatabaseURL(t *testing.T) {
	prev, had := os.LookupEnv("DATABASE_URL")
	os.Unsetenv("DATABASE_URL")
	t.Cleanup(func() {
		if had {
			os.Setenv("DATABASE_URL", prev)
		}
	})

	_, err := Connect(context.Background())
	if err == nil {
		t.Fatal("expected an error when DATABASE_URL is unset, got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("expected error to mention DATABASE_URL, got: %v", err)
	}
}

func TestConnect_MalformedDatabaseURL(t *testing.T) {
	withEnv(t, "DATABASE_URL", "not-a-valid-connection-string://[::::]")

	_, err := Connect(context.Background())
	if err == nil {
		t.Fatal("expected an error for a malformed DATABASE_URL, got nil")
	}
}
