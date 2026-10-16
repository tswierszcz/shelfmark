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
