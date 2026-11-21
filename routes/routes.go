// Package routes wires the application HTTP router, middleware, and handlers.
package routes

import (
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/serverfns"
	"github.com/oxri/oxri-go/views/booki"
	"github.com/oxri/oxri-go/views/errorpage"
	"github.com/oxri/oxri-go/views/landing"
	"github.com/oxri/oxri-go/views/login"
	"github.com/oxri/oxri-go/views/pilot"
)

// Config configures the application router dependencies and static asset paths.
type Config struct {
	AssetsDir   string
	AuthService *auth.Service
	Store       *db.Store
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
	r.Get(booki.StylePath, serveCSS(booki.StyleCSS))
	r.Get(login.StylePath, serveCSS(login.StyleCSS))
	r.Get(pilot.StylePath, serveCSS(pilot.StyleCSS))

	// 4. Static Assets (/assets/*)
	assetsDir := cfg.AssetsDir
	if assetsDir == "" {
		assetsDir = "assets"
	}
	fileServer := http.FileServer(http.Dir(assetsDir))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer))

	// 5. Public Page Routes
	r.Get("/", handleLanding(cfg.AuthService))
	r.Get("/book-i", handleBookI())

	// 6. Authentication Routes
	r.Get("/login", handleLogin(cfg.AuthService))
	r.Get("/auth/status", handleAuthStatus(cfg.AuthService))
	r.HandleFunc("/logout", handleLogout(cfg.AuthService)) // Handles both GET and POST

	// 7. Protected Profile / Pilot Settings Routes
	r.Get("/pilot/user/settings", handlePilotSettings(cfg.AuthService))

	return r
}

func safeRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if rvr == http.ErrAbortHandler {
					panic(rvr)
				}
				ref := errorpage.NewReference()
				log.Printf("[PANIC RECOVERED] Ref: %s | URL: %s | Panic: %v\nStack:\n%s",
					ref, r.URL.Path, rvr, string(debug.Stack()))

				errorpage.RenderWithReference(w, http.StatusInternalServerError, "", ref)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func handleLanding(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := serverfns.RequireAuth(authSvc, r)
		isAuth := (err == nil)
		_ = landing.Page(isAuth).Render(w)
	}
}

func handleBookI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = booki.Page().Render(w)
	}
}

func handleLogin(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next := serverfns.SanitizeRedirect(r.URL.Query().Get("next"))
		if _, err := serverfns.RequireAuth(authSvc, r); err == nil {
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}

		if authSvc == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "Authentication service is currently unavailable.")
			return
		}

		token, bindingCookie, err := authSvc.CreateChallenge(r.RemoteAddr, next)
		if err != nil {
			errorpage.Render(w, http.StatusInternalServerError, "Failed to create authentication challenge.")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.BindingCookieName,
			Value:    bindingCookie,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   300,
		})

		sshCmd := serverfns.BuildSSHCommand(token, authSvc.SSHAddr())
		sshPort := "2222"
		if parts := strings.Split(authSvc.SSHAddr(), ":"); len(parts) == 2 {
			sshPort = parts[1]
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = login.Page(token, sshCmd, next, sshPort).Render(w)
	}
}

func handleAuthStatus(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authSvc == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		bindingCookie, err := r.Cookie(auth.BindingCookieName)
		if err != nil || bindingCookie.Value == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		sessionID, redirectURL, err := authSvc.RedeemChallenge(token, bindingCookie.Value)
		if err != nil {
			if err == auth.ErrNotApproved {
				// HTMX poller receives 204 No Content and continues polling
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Clear temporary binding cookie
		http.SetCookie(w, &http.Cookie{
			Name:     auth.BindingCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		// Set authenticated session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(auth.SessionDuration.Seconds()),
		})

		target := serverfns.SanitizeRedirect(redirectURL)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

func handlePilotSettings(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := serverfns.RequireAuth(authSvc, r); err != nil {
			http.Redirect(w, r, "/login?next=/pilot/user/settings", http.StatusSeeOther)
			return
		}

		keyType := "ssh-ed25519"
		if authSvc != nil && authSvc.KeyType() != "" {
			keyType = authSvc.KeyType()
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = pilot.SettingsPage(keyType).Render(w)
	}
}

func handleLogout(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
			if authSvc != nil {
				authSvc.RevokeSession(cookie.Value)
			}
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func serveCSS(css string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	}
}
