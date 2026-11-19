package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/pages/booki"
	"github.com/oxri/oxri-go/internal/pages/editor"
	"github.com/oxri/oxri-go/internal/pages/errorpage"
	"github.com/oxri/oxri-go/internal/pages/journals"
	"github.com/oxri/oxri-go/internal/pages/landing"
	"github.com/oxri/oxri-go/internal/pages/login"
	"github.com/oxri/oxri-go/internal/pages/pilot"
)

// NewServer configures and returns the main application HTTP handler using Chi.
// Domain route handlers live in small, focused modules (handlers_journals.go,
// handlers_auth.go, handlers_pilot.go).
func NewServer(assetsDir string, authSvc *auth.Service, store *db.Store) http.Handler {
	r := chi.NewRouter()

	// Resilience & Input Hardening Middleware
	r.Use(RecoverMiddleware)
	r.Use(TimeoutMiddleware(30 * time.Second))
	r.Use(MaxBodyBytesMiddleware(DefaultMaxBodyBytes))
	r.Use(CrossOriginMiddleware)

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
	r.Get("/page-css/landing.css", serveCSS(landing.StyleCSS))

	// Book I Article
	r.Get("/book-i", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		content := booki.RenderMarkdown()
		_ = booki.Page(content).Render(req.Context(), w)
	})
	r.Get("/page-css/book-i.css", serveCSS(booki.StyleCSS))

	// Journals Public & Composition Routes
	r.Get("/journals", handleJournalsCatalog(store))
	r.Get("/journals/new", handleJournalNew(store))
	r.Get("/journal-editor", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/journals/new", http.StatusMovedPermanently)
	})
	r.Get("/journals/{id}", handleJournalEntry(store))
	r.Get("/page-css/journals.css", serveCSS(journals.StyleCSS))
	r.Get("/page-css/journals-new.css", serveCSS(editor.StyleCSS))
	r.Get("/page-css/journal-editor.css", serveCSS(editor.StyleCSS))

	// Protected Journal Author Management Routes
	r.Group(func(adminRouter chi.Router) {
		useRequireAuth(adminRouter, authSvc)
		adminRouter.Post("/journals", handleJournalCreate(store))
		adminRouter.Get("/journals/{id}/edit", handleJournalEditForm(store))
		adminRouter.Post("/journals/{id}", handleJournalUpdate(store))
		adminRouter.Post("/journals/{id}/delete", handleJournalDelete(store))
		adminRouter.Post("/journals/{id}/publish", handleJournalPublish(store, true))
		adminRouter.Post("/journals/{id}/unpublish", handleJournalPublish(store, false))
		adminRouter.Post("/journals/{id}/bookmark", handleJournalBookmark(store))
		adminRouter.Post("/journals/batch", handleJournalBatch(store))
	})

	// Login & Auth Routes
	r.Get("/login", handleLogin(authSvc))
	r.Get("/auth/status", handleAuthStatus(authSvc))
	r.HandleFunc("/logout", handleLogout(authSvc))
	r.Get("/page-css/login.css", serveCSS(login.StyleCSS))

	// Pilot Experimental Namespace (Protected)
	r.Group(func(pilotRouter chi.Router) {
		useRequireAuth(pilotRouter, authSvc)
		pilotRouter.Get("/pilot", handlePilotRedirect())
		pilotRouter.Get("/pilot/user/settings", handlePilotSettings(authSvc))
	})
	r.Get("/page-css/pilot-settings.css", serveCSS(pilot.StyleCSS))
	r.Get("/page-css/error.css", serveCSS(errorpage.StyleCSS))

	// Static Assets
	fileServer := http.FileServer(http.Dir(assetsDir))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer))

	return r
}

func serveCSS(css string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	}
}

func useRequireAuth(router chi.Router, authSvc *auth.Service) {
	if authSvc != nil {
		router.Use(auth.RequireAuth(authSvc))
		return
	}
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			RenderError(w, req, http.StatusServiceUnavailable)
		})
	})
}
