package auth

import (
	"fmt"
	"os"
)

// Role identifies the privilege level of an account.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleArchitect Role = "architect"
	RoleViewer    Role = "viewer"
)

// Account holds the role and bcrypt password hash for a single login.
type Account struct {
	Role         Role
	PasswordHash string
}

// Accounts is a lookup of account by email.
type Accounts map[string]Account

// LoadAccounts reads the three role accounts (Admin, Architect, Viewer) from
// environment variables, failing fast if any is missing or malformed.
func LoadAccounts() (Accounts, error) {
	accounts := Accounts{}

	roles := []struct {
		role        Role
		emailVar    string
		passwordVar string
	}{
		{RoleAdmin, "ADMIN_EMAIL", "ADMIN_PASSWORD_HASH"},
		{RoleArchitect, "ARCHITECT_EMAIL", "ARCHITECT_PASSWORD_HASH"},
		{RoleViewer, "VIEWER_EMAIL", "VIEWER_PASSWORD_HASH"},
	}

	for _, r := range roles {
		email := os.Getenv(r.emailVar)
		if email == "" {
			return nil, fmt.Errorf("auth: missing required env var %s", r.emailVar)
		}
		hash := os.Getenv(r.passwordVar)
		if hash == "" {
			return nil, fmt.Errorf("auth: missing required env var %s", r.passwordVar)
		}
		accounts[email] = Account{Role: r.role, PasswordHash: hash}
	}

	return accounts, nil
}
