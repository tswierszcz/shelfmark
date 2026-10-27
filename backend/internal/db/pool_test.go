package db

import (
	"context"
	"strings"
	"testing"
)

func TestConnect_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Connect(context.Background())
	if err == nil {
		t.Fatal("expected an error when DATABASE_URL is unset, got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("expected error to mention DATABASE_URL, got: %v", err)
	}
}

func TestConnect_MalformedDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "not-a-valid-connection-string://[::::]")

	_, err := Connect(context.Background())
	if err == nil {
		t.Fatal("expected an error for a malformed DATABASE_URL, got nil")
	}
}

func TestConnect_ErrorDoesNotLeakPassword(t *testing.T) {
	const (
		password = "s3cr3t-pw"
		host     = "db.example.internal"
	)
	// The non-numeric port makes pgxpool.ParseConfig fail before any dial.
	t.Setenv("DATABASE_URL", "postgres://app:"+password+"@"+host+":notaport/shelfmark")

	_, err := Connect(context.Background())
	if err == nil {
		t.Fatal("Connect() expected an error for an unparseable DATABASE_URL, got nil")
	}
	if got, want := err.Error(), "db: failed to parse DATABASE_URL"; got != want {
		t.Errorf("Connect() error = %q, want %q", got, want)
	}
	for _, secret := range []string{password, host} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("Connect() error leaks %q: %q", secret, err.Error())
		}
	}
}
