package routes

import (
	"errors"
	"github.com/oxri/oxri-go/internal/domain"
	"net/http"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/serverfns"
	"github.com/oxri/oxri-go/views/booki"
	"github.com/oxri/oxri-go/views/errorpage"
)

func handleBookView(store *db.Store, authSvc *auth.Service, slug string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		title, ok := db.BookTitle(slug)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}

		_, authErr := serverfns.RequireAuth(authSvc, r)
		isAuth := (authErr == nil)

		body := "# " + title + "\n\n*Forthcoming.*"
		if store != nil {
			doneDB := timed(r.Context(), "db")
			b, err := store.GetBook(r.Context(), slug)
			doneDB()
			if err != nil {
				renderInternalError(w, r, http.StatusInternalServerError, err)
				return
			}
			title = b.Title
			body = b.Body
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = booki.BookPage(slug, title, body, isAuth).Render(w)
	}
}

func handleBookEditForm(store *db.Store, authSvc *auth.Service, slug string) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		title, ok := db.BookTitle(slug)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		b, err := store.GetBook(r.Context(), slug)
		if err != nil {
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = booki.EditPage(slug, title, b.Body, "").Render(w)
	})
}

func handleBookSave(store *db.Store, authSvc *auth.Service, slug string, limiter *ipRateLimiter) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		title, ok := db.BookTitle(slug)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			errorpage.Render(w, http.StatusTooManyRequests, "Too many save requests. Please wait briefly.")
			return
		}

		if err := r.ParseForm(); err != nil {
			errorpage.Render(w, http.StatusBadRequest, "")
			return
		}

		rawBody := r.FormValue("body")
		if _, err := store.SaveBook(r.Context(), cap, slug, rawBody); err != nil {
			if errors.Is(err, db.ErrQuotaExceeded) {
				// The book is valid but storage is full: hold it and send the author
				// to Make Room. Only journal entries are vacated; once enough space is
				// free, the page saves the book automatically.
				if body, vErr := db.ValidateBookBody(rawBody); vErr == nil {
					_ = store.SetPendingSave(cap, db.PendingBook{Slug: slug, Body: body, BookBytes: db.BookBytes(slug, title, body)})
					http.Redirect(w, r, "/journals/vacate-space/", http.StatusSeeOther)
					return
				}
			}
			if errors.Is(err, db.ErrInvalidInput) || errors.Is(err, db.ErrBookTooLarge) {
				safeEcho := db.SanitizeProseInput(rawBody)
				if len(safeEcho) > db.MaxBookBodyBytes {
					safeEcho = safeEcho[:db.MaxBookBodyBytes]
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = booki.EditPage(slug, title, safeEcho, err.Error()).Render(w)
				return
			}
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}

		http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
	})
}
