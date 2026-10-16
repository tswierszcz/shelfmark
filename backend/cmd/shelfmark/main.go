package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	addr := ":" + envOr("PORT", "8080")
	staticDir := envOr("STATIC_DIR", "../web/build/client")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
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
