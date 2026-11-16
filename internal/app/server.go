package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/pages/booki"
	"github.com/oxri/oxri-go/internal/pages/editor"
	"github.com/oxri/oxri-go/internal/pages/errorpage"
	"github.com/oxri/oxri-go/internal/pages/journals"
	"github.com/oxri/oxri-go/internal/pages/landing"
)

// NewServer configures and returns the main application HTTP handler using Chi.
func NewServer(assetsDir string) http.Handler {
	r := chi.NewRouter()

	// Resilience Middleware
	r.Use(RecoverMiddleware)
	r.Use(TimeoutMiddleware(30 * time.Second))

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
		_ = landing.Page().Render(req.Context(), w)
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
		_ = journals.Page().Render(req.Context(), w)
	})
	r.Get("/page-css/journals.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(journals.StyleCSS))
	})

	// Journal Editor
	r.Get("/journal-editor", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = editor.Page().Render(req.Context(), w)
	})
	r.Get("/page-css/journal-editor.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(editor.StyleCSS))
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
