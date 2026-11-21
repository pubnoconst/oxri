// Package routes wires the application HTTP router, middleware, and handlers.
package routes

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/oxri/oxri-go/views/errorpage"
	"github.com/oxri/oxri-go/views/landing"
)

// Config configures the application router dependencies and static asset paths.
type Config struct {
	AssetsDir string
}

// NewRouter builds and returns the top-level Chi HTTP handler.
func NewRouter(cfg Config) http.Handler {
	r := chi.NewRouter()

	// 1. Resilience Middleware (Never leak stack traces or internal details to client)
	r.Use(safeRecoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// 2. Custom 404 & 405 Handlers
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		errorpage.Render(w, http.StatusNotFound, "")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		errorpage.Render(w, http.StatusMethodNotAllowed, "")
	})

	// 3. Component CSS Routes (each component owns its own CSS route)
	r.Get(landing.StylePath, serveCSS(landing.StyleCSS))
	r.Get(errorpage.StylePath, serveCSS(errorpage.StyleCSS))

	// 4. Static Assets (/assets/*)
	assetsDir := cfg.AssetsDir
	if assetsDir == "" {
		assetsDir = "assets"
	}
	fileServer := http.FileServer(http.Dir(assetsDir))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer))

	// 5. Page Routes
	r.Get("/", handleLanding())

	// 6. Test panic route (only for development/testing if needed)
	r.Get("/_test/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic: database connection failed or simulated exception")
	})

	return r
}

// safeRecoverer catches panics, logs the stack trace internally with an incident reference,
// and serves a clean, secure error page to the client with ZERO leaked stack traces.
func safeRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if rvr == http.ErrAbortHandler {
					panic(rvr)
				}
				ref := errorpage.NewReference()
				// Internal server log captures stack trace and incident ref for operator investigation
				log.Printf("[PANIC RECOVERED] Ref: %s | URL: %s | Panic: %v\nStack:\n%s",
					ref, r.URL.Path, rvr, string(debug.Stack()))

				// Client receives only sanitized HTML error page with the exact reference code
				errorpage.RenderWithReference(w, http.StatusInternalServerError, "", ref)
			}
		}()
		next.ServeHTTP(w, r)
	})
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
