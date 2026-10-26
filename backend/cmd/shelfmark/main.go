package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/tswierszcz/shelfmark/backend/internal/auth"
	"github.com/tswierszcz/shelfmark/backend/internal/catalog"
	"github.com/tswierszcz/shelfmark/backend/internal/db"
	"github.com/tswierszcz/shelfmark/backend/internal/obs"
)

func main() {
	slog.SetDefault(obs.NewLogger(os.Stdout))

	addr := ":" + envOr("PORT", "8080")
	staticDir := envOr("STATIC_DIR", "../web/build/client")

	pool, err := db.Connect(context.Background())
	if err != nil {
		fatal("db: failed to connect", err)
	}
	if err := db.Migrate(os.Getenv("DATABASE_URL")); err != nil {
		pool.Close()
		fatal("db: failed to migrate", err)
	}

	usersStore := auth.NewUsersStore(pool)
	if err := usersStore.SeedFromEnvIfEmpty(context.Background()); err != nil {
		pool.Close()
		fatal("auth: failed to seed accounts", err)
	}

	catalogStore := catalog.NewStore(pool)

	openRouterClient, err := catalog.NewOpenRouterClient()
	if err != nil {
		fatal("catalog: failed to construct openrouter client", err)
	}
	finders := []catalog.RepoFinder{catalog.NewDepsDevClient(), openRouterClient}
	catalogQueue := catalog.NewQueue(catalogStore, finders, catalog.NewGitHubClient(), openRouterClient, 5)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Re-enqueue libraries orphaned by a previous restart. Runs in the
	// background so a large backlog (Enqueue blocks on a full channel) can't
	// delay serving or the health check.
	go catalogQueue.Sweep(ctx, catalog.StuckLibraryAge)

	mux := newMux(deps{
		Pool:      pool,
		Users:     usersStore,
		Catalog:   catalogStore,
		Queue:     catalogQueue,
		StaticDir: staticDir,
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           obs.Middleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		// Read/Write timeouts are sized for 10MB catalog/project uploads.
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr, "static_dir", staticDir)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			fatal("http: server stopped", err)
		}
	case <-ctx.Done():
	}
	stop()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownBudget)
	defer cancel()

	// The HTTP server and the queue drain concurrently under one budget: an
	// upload handler blocked in Enqueue on a full channel is released by the
	// queue's stop signal (the job is dropped and stays pending for the next
	// boot sweep), so it cannot hold srv.Shutdown for the whole budget. The
	// pool closes only after both are done.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("http: shutdown incomplete", "error", err.Error())
		}
	}()
	go func() {
		defer wg.Done()
		if err := catalogQueue.Shutdown(shutdownCtx); err != nil {
			slog.Error("queue: shutdown incomplete", "error", err.Error())
		}
	}()
	wg.Wait()
	pool.Close()
	slog.Info("shutdown complete")
}

// shutdownBudget bounds graceful shutdown (HTTP drain, queue drain, pool
// close), staying inside Fly's default kill timeout.
const shutdownBudget = 25 * time.Second

// fatal logs a boot/serve failure at Error level and exits with status 1.
func fatal(msg string, err error) {
	slog.Error(msg, "error", err.Error())
	os.Exit(1)
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
