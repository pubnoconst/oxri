package app

import (
	"net/http"
	"time"

	"github.com/oxri/oxri-go/internal/pages/booki"
	"github.com/oxri/oxri-go/internal/pages/editor"
	"github.com/oxri/oxri-go/internal/pages/errorpage"
	"github.com/oxri/oxri-go/internal/pages/journals"
	"github.com/oxri/oxri-go/internal/pages/landing"
)

// NewServer configures and returns the main application HTTP handler.
func NewServer(assetsDir string) http.Handler {
	mux := http.NewServeMux()

	// Landing Page
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = landing.Page().Render(r.Context(), w)
	})
	mux.HandleFunc("GET /page-css/landing.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(landing.StyleCSS))
	})

	// Book I Article
	mux.HandleFunc("GET /book-i", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		content := booki.RenderMarkdown()
		_ = booki.Page(content).Render(r.Context(), w)
	})
	mux.HandleFunc("GET /page-css/book-i.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(booki.StyleCSS))
	})

	// Journals
	mux.HandleFunc("GET /journals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = journals.Page().Render(r.Context(), w)
	})
	mux.HandleFunc("GET /page-css/journals.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(journals.StyleCSS))
	})

	// Journal Editor
	mux.HandleFunc("GET /journal-editor", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = editor.Page().Render(r.Context(), w)
	})
	mux.HandleFunc("GET /page-css/journal-editor.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(editor.StyleCSS))
	})

	// Error Page Stylesheet
	mux.HandleFunc("GET /page-css/error.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(errorpage.StyleCSS))
	})

	// Static Assets
	fileServer := http.FileServer(http.Dir(assetsDir))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", fileServer))

	// Fallback 404 for any other path
	mux.HandleFunc("/{path...}", func(w http.ResponseWriter, r *http.Request) {
		RenderError(w, r, http.StatusNotFound)
	})

	// Middleware chain: Panic recovery -> 30s timeout -> Mux
	handler := TimeoutMiddleware(30*time.Second, mux)
	handler = RecoverMiddleware(handler)

	return handler
}
