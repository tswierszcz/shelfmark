package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/catalog"
	"github.com/tswierszcz/shelfmark/backend/internal/db"
)

func main() {
	addr := ":" + envOr("PORT", "8080")
	staticDir := envOr("STATIC_DIR", "../web/build/client")

	pool, err := db.Connect(context.Background())
	if err != nil {
		log.Fatalf("db: failed to connect: %v", err)
	}
	if err := db.Migrate(os.Getenv("DATABASE_URL")); err != nil {
		pool.Close()
		log.Fatalf("db: failed to migrate: %v", err)
	}

	usersStore := auth.NewUsersStore(pool)
	if err := usersStore.SeedFromEnvIfEmpty(context.Background()); err != nil {
		pool.Close()
		log.Fatalf("auth: failed to seed accounts: %v", err)
	}

	catalogStore := catalog.NewStore(pool)

	openRouterClient, err := catalog.NewOpenRouterClient()
	if err != nil {
		log.Fatalf("catalog: failed to construct openrouter client: %v", err)
	}
	finders := []catalog.RepoFinder{catalog.NewDepsDevClient(), openRouterClient}
	catalogQueue := catalog.NewQueue(catalogStore, finders, catalog.NewGitHubClient(), openRouterClient, 5)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", db.HealthCheck(pool))
	mux.HandleFunc("POST /api/login", auth.LoginHandler(usersStore))
	mux.HandleFunc("POST /api/logout", auth.LogoutHandler())
	mux.HandleFunc("GET /api/me", auth.RequireRole(auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer)(auth.MeHandler()))
	mux.HandleFunc("POST /api/users", auth.RequireRole(auth.RoleAdmin)(auth.CreateUserHandler(usersStore)))
	mux.HandleFunc("GET /api/users", auth.RequireRole(auth.RoleAdmin)(auth.ListUsersHandler(usersStore)))
	mux.HandleFunc("POST /api/catalog/upload", auth.RequireRole(auth.RoleAdmin)(catalog.UploadHandler(catalogStore, catalogQueue)))
	mux.HandleFunc("GET /api/catalog/batches/{batchID}", auth.RequireRole(auth.RoleAdmin)(catalog.BatchProgressHandler(catalogStore)))
	mux.HandleFunc("GET /api/catalog/libraries", auth.RequireRole(auth.RoleAdmin)(catalog.ListLibrariesHandler(catalogStore)))
	mux.HandleFunc("GET /api/catalog/categories", auth.RequireRole(auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer)(catalog.ListTaxonomyHandler(catalogStore)))
	mux.HandleFunc("POST /api/catalog/libraries/{libraryID}/categories", auth.RequireRole(auth.RoleAdmin)(catalog.AttachCategoryHandler(catalogStore)))
	mux.HandleFunc("GET /api/organization/libraries", auth.RequireRole(auth.RoleArchitect, auth.RoleViewer)(catalog.ListOrganizationLibrariesHandler(catalogStore)))
	mux.HandleFunc("POST /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic", auth.RequireRole(auth.RoleArchitect)(catalog.MarkStrategicHandler(catalogStore)))
	mux.HandleFunc("DELETE /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic", auth.RequireRole(auth.RoleArchitect)(catalog.UnmarkStrategicHandler(catalogStore)))
	mux.HandleFunc("POST /api/projects/upload", auth.RequireRole(auth.RoleArchitect)(catalog.ProjectUploadHandler(catalogStore, catalogQueue)))
	mux.HandleFunc("GET /api/projects", auth.RequireRole(auth.RoleArchitect, auth.RoleViewer)(catalog.ListProjectsHandler(catalogStore)))
	mux.HandleFunc("GET /api/projects/{projectID}/report", auth.RequireRole(auth.RoleArchitect, auth.RoleViewer)(catalog.ProjectReportHandler(catalogStore)))
	mux.Handle("GET /", spaHandler(staticDir))

	log.Printf("listening on %s, serving SPA from %s", addr, staticDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// spaHandler serves the built frontend from dir. Paths that are not files fall back
// to index.html so client-side routes survive a page reload.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := os.Stat(filepath.Join(dir, filepath.Clean(r.URL.Path)))
		if err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
