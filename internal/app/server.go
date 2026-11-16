package app

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/pages/booki"
	"github.com/oxri/oxri-go/internal/pages/editor"
	"github.com/oxri/oxri-go/internal/pages/errorpage"
	"github.com/oxri/oxri-go/internal/pages/journals"
	"github.com/oxri/oxri-go/internal/pages/landing"
	"github.com/oxri/oxri-go/internal/pages/login"
)

// NewServer configures and returns the main application HTTP handler using Chi.
func NewServer(assetsDir string, authSvc *auth.Service) http.Handler {
	r := chi.NewRouter()

	// Resilience Middleware
	r.Use(RecoverMiddleware)
	r.Use(TimeoutMiddleware(30 * time.Second))

	// Session Middleware
	if authSvc != nil {
		r.Use(auth.SessionMiddleware(authSvc))
	}

	// Custom 404 & 405 error pages
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		RenderError(w, req, http.StatusNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		RenderError(w, req, http.StatusMethodNotAllowed)
	})

	// Landing Page
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		isAuth := auth.IsAuthenticated(req.Context())
		_ = landing.Page(isAuth).Render(req.Context(), w)
	})
	r.Get("/page-css/landing.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(landing.StyleCSS))
	})

	// Book I Article
	r.Get("/book-i", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		content := booki.RenderMarkdown()
		_ = booki.Page(content).Render(req.Context(), w)
	})
	r.Get("/page-css/book-i.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(booki.StyleCSS))
	})

	// Journals
	r.Get("/journals", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		isAuth := auth.IsAuthenticated(req.Context())
		_ = journals.Page(isAuth).Render(req.Context(), w)
	})
	r.Get("/page-css/journals.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(journals.StyleCSS))
	})

	// Journal Editor
	r.Get("/journal-editor", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		isAuth := auth.IsAuthenticated(req.Context())
		_ = editor.Page(isAuth).Render(req.Context(), w)
	})
	r.Get("/page-css/journal-editor.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(editor.StyleCSS))
	})

	// Login & Auth Routes
	r.Get("/login", func(w http.ResponseWriter, req *http.Request) {
		next := req.URL.Query().Get("next")
		if next == "" {
			next = "/"
		}

		if auth.IsAuthenticated(req.Context()) {
			http.Redirect(w, req, next, http.StatusSeeOther)
			return
		}

		if authSvc == nil {
			RenderError(w, req, http.StatusServiceUnavailable)
			return
		}

		token, err := authSvc.CreatePendingLogin(next)
		if err != nil {
			RenderError(w, req, http.StatusInternalServerError)
			return
		}

		sshPort := "2222"
		if addr := authSvc.SSHAddr(); addr != "" {
			if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
				sshPort = p
			}
		}

		sshCommand := fmt.Sprintf("ssh -p %s %s@localhost", sshPort, token)
		keyComment := authSvc.AuthorizedKeyComment()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = login.Page(token, sshCommand, next, keyComment, sshPort).Render(req.Context(), w)
	})
	r.Get("/page-css/login.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(login.StyleCSS))
	})

	// Polling Endpoint for HTMX
	r.Get("/auth/status", func(w http.ResponseWriter, req *http.Request) {
		if authSvc == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		token := req.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check if approved and consume
		sessionID, redirectURL, ok := authSvc.ConsumeApprovedLogin(token)
		if ok {
			if redirectURL == "" {
				redirectURL = "/"
			}
			http.SetCookie(w, &http.Cookie{
				Name:     auth.SessionCookieName,
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(auth.SessionDuration.Seconds()),
			})
			w.Header().Set("HX-Redirect", redirectURL)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authorized"))
			return
		}

		// Still pending?
		approved, _, found := authSvc.CheckPendingLogin(token)
		if found && !approved {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("waiting"))
			return
		}

		// Expired or not found
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusGone)
		w.Write([]byte("expired"))
	})

	// Logout
	r.HandleFunc("/logout", func(w http.ResponseWriter, req *http.Request) {
		if authSvc != nil {
			cookie, err := req.Cookie(auth.SessionCookieName)
			if err == nil && cookie.Value != "" {
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

		http.Redirect(w, req, "/", http.StatusSeeOther)
	})

	// Error Page Stylesheet
	r.Get("/page-css/error.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(errorpage.StyleCSS))
	})

	// Static Assets
	fileServer := http.FileServer(http.Dir(assetsDir))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer))

	return r
}
