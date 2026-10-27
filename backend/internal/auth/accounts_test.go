package auth

import "testing"

func setAllAccountEnv(t *testing.T) {
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD_HASH", "admin-hash")
	t.Setenv("ARCHITECT_EMAIL", "architect@example.com")
	t.Setenv("ARCHITECT_PASSWORD_HASH", "architect-hash")
	t.Setenv("VIEWER_EMAIL", "viewer@example.com")
	t.Setenv("VIEWER_PASSWORD_HASH", "viewer-hash")
}

func TestLoadAccounts_Success(t *testing.T) {
	setAllAccountEnv(t)

	accounts, err := LoadAccounts()
	if err != nil {
		t.Fatalf("LoadAccounts() unexpected error: %v", err)
	}

	admin, ok := accounts["admin@example.com"]
	if !ok {
		t.Fatalf("expected admin account to be present")
	}
	if admin.Role != RoleAdmin || admin.PasswordHash != "admin-hash" {
		t.Errorf("admin account = %+v, want Role=%s PasswordHash=admin-hash", admin, RoleAdmin)
	}

	architect, ok := accounts["architect@example.com"]
	if !ok || architect.Role != RoleArchitect || architect.PasswordHash != "architect-hash" {
		t.Errorf("architect account = %+v, ok=%v", architect, ok)
	}

	viewer, ok := accounts["viewer@example.com"]
	if !ok || viewer.Role != RoleViewer || viewer.PasswordHash != "viewer-hash" {
		t.Errorf("viewer account = %+v, ok=%v", viewer, ok)
	}
}

func TestLoadAccounts_MissingVar(t *testing.T) {
	cases := []string{
		"ADMIN_EMAIL",
		"ADMIN_PASSWORD_HASH",
		"ARCHITECT_EMAIL",
		"ARCHITECT_PASSWORD_HASH",
		"VIEWER_EMAIL",
		"VIEWER_PASSWORD_HASH",
	}

	for _, missing := range cases {
		t.Run(missing, func(t *testing.T) {
			setAllAccountEnv(t)
			t.Setenv(missing, "")

			if _, err := LoadAccounts(); err == nil {
				t.Fatalf("LoadAccounts() expected error when %s is missing, got nil", missing)
			}
		})
	}
}
