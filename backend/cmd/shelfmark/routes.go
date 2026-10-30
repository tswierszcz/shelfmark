package main

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/catalog"
	"github.com/tswierszcz/shelfmark/backend/internal/db"
)

// deps holds everything the HTTP handlers capture at construction time.
// Handler constructors only store these, so a denial-path request (rejected
// by auth.RequireRole before the handler runs) works with nil dependencies.
type deps struct {
	Pool      *pgxpool.Pool
	Users     *auth.UsersStore
	Catalog   *catalog.Store
	Queue     catalog.Enqueuer
	StaticDir string
}

// route is one registered endpoint. A nil Roles means the route is public;
// otherwise the handler is wrapped with auth.RequireRole(Roles...).
type route struct {
	Method  string
	Pattern string
	Roles   []auth.Role
	Handler http.Handler
}

// routes returns every HTTP route the server registers, in registration order.
func routes(d deps) []route {
	admin := []auth.Role{auth.RoleAdmin}
	architect := []auth.Role{auth.RoleArchitect}
	// adminArchitect lets the architect who uploads a project SBOM poll its
	// categorization batch, alongside admins polling catalog uploads.
	adminArchitect := []auth.Role{auth.RoleAdmin, auth.RoleArchitect}
	architectViewer := []auth.Role{auth.RoleArchitect, auth.RoleViewer}
	everyone := []auth.Role{auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer}

	return []route{
		{"GET", "/healthz", nil, db.HealthCheck(d.Pool)},
		{"POST", "/api/login", nil, auth.LoginHandler(d.Users)},
		{"POST", "/api/logout", nil, auth.LogoutHandler()},
		{"GET", "/api/me", everyone, auth.MeHandler()},
		{"POST", "/api/users", admin, auth.CreateUserHandler(d.Users)},
		{"GET", "/api/users", admin, auth.ListUsersHandler(d.Users)},
		{"POST", "/api/catalog/upload", admin, catalog.UploadHandler(d.Catalog, d.Queue)},
		{"GET", "/api/catalog/batches/{batchID}", adminArchitect, catalog.BatchProgressHandler(d.Catalog)},
		{"GET", "/api/catalog/libraries", admin, catalog.ListLibrariesHandler(d.Catalog)},
		{"GET", "/api/catalog/categories", everyone, catalog.ListTaxonomyHandler(d.Catalog)},
		{"GET", "/api/dashboard", everyone, catalog.DashboardHandler(d.Catalog)},
		{"POST", "/api/catalog/libraries/{libraryID}/categories", admin, catalog.AttachCategoryHandler(d.Catalog)},
		{"GET", "/api/organization/libraries", architectViewer, catalog.ListOrganizationLibrariesHandler(d.Catalog)},
		{"POST", "/api/organization/libraries/{libraryID}/categories/{categoryID}/strategic", architect, catalog.MarkStrategicHandler(d.Catalog)},
		{"DELETE", "/api/organization/libraries/{libraryID}/categories/{categoryID}/strategic", architect, catalog.UnmarkStrategicHandler(d.Catalog)},
		{"POST", "/api/projects/upload", architect, catalog.ProjectUploadHandler(d.Catalog, d.Queue)},
		{"GET", "/api/projects", architectViewer, catalog.ListProjectsHandler(d.Catalog)},
		{"DELETE", "/api/projects/{projectID}", architect, catalog.DeleteProjectHandler(d.Catalog)},
		{"GET", "/api/projects/{projectID}/report", architectViewer, catalog.ProjectReportHandler(d.Catalog)},
		{"GET", "/", nil, spaHandler(d.StaticDir)},
	}
}

// newMux registers every route from routes(d) on a fresh ServeMux, gating
// protected routes with auth.RequireRole.
func newMux(d deps) *http.ServeMux {
	mux := http.NewServeMux()
	for _, rt := range routes(d) {
		h := rt.Handler
		if rt.Roles != nil {
			h = auth.RequireRole(rt.Roles...)(h.ServeHTTP)
		}
		mux.Handle(rt.Method+" "+rt.Pattern, h)
	}
	return mux
}
