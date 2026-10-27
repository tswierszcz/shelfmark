package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/auth/authtest"
)

// expectedRoles is the role regression lock for every registered route,
// written independently of routes() from PRD §Access Control and the route
// table in research.md. A nil value marks a public route. Changing a route's
// role set must be a deliberate edit here, reviewed against the PRD.
var expectedRoles = map[string][]auth.Role{
	"GET /healthz":     nil,
	"POST /api/login":  nil,
	"POST /api/logout": nil,
	"GET /":            nil,

	"GET /api/me":                                        {auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer},
	"POST /api/users":                                    {auth.RoleAdmin},
	"GET /api/users":                                     {auth.RoleAdmin},
	"POST /api/catalog/upload":                           {auth.RoleAdmin},
	"GET /api/catalog/batches/{batchID}":                 {auth.RoleAdmin, auth.RoleArchitect},
	"GET /api/catalog/libraries":                         {auth.RoleAdmin},
	"GET /api/catalog/categories":                        {auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer},
	"POST /api/catalog/libraries/{libraryID}/categories": {auth.RoleAdmin},
	"GET /api/organization/libraries":                    {auth.RoleArchitect, auth.RoleViewer},
	"POST /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic":   {auth.RoleArchitect},
	"DELETE /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic": {auth.RoleArchitect},
	"POST /api/projects/upload":            {auth.RoleArchitect},
	"GET /api/projects":                    {auth.RoleArchitect, auth.RoleViewer},
	"GET /api/projects/{projectID}/report": {auth.RoleArchitect, auth.RoleViewer},
}

// pathValues are concrete values substituted for pattern wildcards so the
// request matches the registered route.
var pathValues = map[string]string{
	"{batchID}":    "3f2b8c1e-6a4d-4e1f-9b7a-1c2d3e4f5a6b",
	"{libraryID}":  "7a1c9e2b-4d3f-4a8e-b6c5-2e1f0a9b8c7d",
	"{categoryID}": "c4e8a2f6-1b3d-4c5e-8f7a-9b0c1d2e3f4a",
	"{projectID}":  "e9d8c7b6-a5f4-4e3d-9c2b-1a0f9e8d7c6b",
}

func sortedRoles(roles []auth.Role) []auth.Role {
	out := slices.Clone(roles)
	slices.Sort(out)
	return out
}

func TestRoutes_RoleSetsMatchExpected(t *testing.T) {
	seen := make(map[string]bool)
	for _, rt := range routes(deps{}) {
		key := rt.Method + " " + rt.Pattern
		if seen[key] {
			t.Errorf("route %q registered more than once", key)
		}
		seen[key] = true

		want, ok := expectedRoles[key]
		if !ok {
			t.Errorf("route %q is not listed in expectedRoles", key)
			continue
		}
		if (want == nil) != (rt.Roles == nil) {
			t.Errorf("route %q: public mismatch: got roles %v, want %v", key, rt.Roles, want)
			continue
		}
		if !slices.Equal(sortedRoles(rt.Roles), sortedRoles(want)) {
			t.Errorf("route %q: got roles %v, want %v", key, rt.Roles, want)
		}
	}
	for key := range expectedRoles {
		if !seen[key] {
			t.Errorf("expectedRoles lists %q but routes() does not register it", key)
		}
	}
}

func TestRoutes_DeniesDisallowedCallers(t *testing.T) {
	t.Setenv("SESSION_SECRET", "routes-test-secret")

	mux := newMux(deps{StaticDir: t.TempDir()})

	callers := []struct {
		name string
		role auth.Role // "" means no session
	}{
		{"no session", ""},
		{"admin", auth.RoleAdmin},
		{"architect", auth.RoleArchitect},
		{"viewer", auth.RoleViewer},
	}

	cookies := make(map[auth.Role]*http.Cookie)
	for _, c := range callers {
		if c.role == "" {
			continue
		}
		cookies[c.role] = authtest.SessionCookie(t, string(c.role)+"@example.com", c.role)
	}

	for key, allowed := range expectedRoles {
		if allowed == nil {
			continue
		}
		method, pattern, _ := strings.Cut(key, " ")
		path := pattern
		for wildcard, value := range pathValues {
			path = strings.ReplaceAll(path, wildcard, value)
		}
		if strings.Contains(path, "{") {
			t.Fatalf("route %q: no concrete value for a path wildcard", key)
		}

		for _, c := range callers {
			wantStatus := http.StatusUnauthorized
			if c.role != "" {
				if slices.Contains(allowed, c.role) {
					continue // allowed cell: not part of the denial matrix
				}
				wantStatus = http.StatusForbidden
			}

			t.Run(key+"/"+c.name, func(t *testing.T) {
				req := httptest.NewRequest(method, path, nil)
				if c.role != "" {
					req.AddCookie(cookies[c.role])
				}
				rec := httptest.NewRecorder()

				mux.ServeHTTP(rec, req)

				if rec.Code != wantStatus {
					t.Errorf("%s %s as %s: got status %d, want %d", method, path, c.name, rec.Code, wantStatus)
				}
			})
		}
	}
}
