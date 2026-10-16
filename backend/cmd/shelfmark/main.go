package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/db"
)

func main() {
	addr := ":" + envOr("PORT", "8080")
	staticDir := envOr("STATIC_DIR", "../web/build/client")

	accounts, err := auth.LoadAccounts()
	if err != nil {
		log.Fatalf("auth: failed to load accounts: %v", err)
	}

	pool, err := db.Connect(context.Background())
	if err != nil {
		log.Fatalf("db: failed to connect: %v", err)
	}
	if err := db.Migrate(os.Getenv("DATABASE_URL")); err != nil {
		pool.Close()
		log.Fatalf("db: failed to migrate: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", db.HealthCheck(pool))
	mux.HandleFunc("POST /api/login", auth.LoginHandler(accounts))
	mux.HandleFunc("POST /api/logout", auth.LogoutHandler())
	mux.HandleFunc("GET /api/me", auth.RequireRole(auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer)(auth.MeHandler()))
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
