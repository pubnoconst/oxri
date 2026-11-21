// Package routes wires the application HTTP router, middleware, and handlers.
package routes

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/oxri/oxri-go/views/landing"
)

// Config configures the application router dependencies and static asset paths.
type Config struct {
	AssetsDir string
}

// NewRouter builds and returns the top-level Chi HTTP handler.
func NewRouter(cfg Config) http.Handler {
	r := chi.NewRouter()

	// 1. Resilience Middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// 2. Component CSS Routes (each component owns its own CSS route)
	r.Get(landing.StylePath, serveCSS(landing.StyleCSS))

	// 3. Static Assets (/assets/*)
	assetsDir := cfg.AssetsDir
	if assetsDir == "" {
		assetsDir = "assets"
	}
	fileServer := http.FileServer(http.Dir(assetsDir))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer))

	// 4. Page Routes
	r.Get("/", handleLanding())

	return r
}

func handleLanding() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		isAuth := false // Auth capability check will be plugged in Phase 4
		_ = landing.Page(isAuth).Render(w)
	}
}

func serveCSS(css string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	}
}
