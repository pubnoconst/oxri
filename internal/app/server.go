package app

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
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
func NewServer(assetsDir string, authSvc *auth.Service, store *db.Store) http.Handler {
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

	// Journals Public & Composition Routes
	r.Get("/journals", func(w http.ResponseWriter, req *http.Request) {
		isAuth := auth.IsAuthenticated(req.Context())
		q := db.SanitizeSearchQuery(req.URL.Query().Get("q"))
		var selectedTags []string
		for _, rawTag := range req.URL.Query()["tag"] {
			if norm := db.FormatTagTitleCase(rawTag); norm != "" {
				if _, ok := db.ValidateTagChars(norm); ok {
					selectedTags = append(selectedTags, norm)
				}
			}
		}

		var entries []db.Journal
		var allTags []string
		if store != nil {
			var err error
			entries, err = store.ListFiltered(req.Context(), db.ListFilter{
				IncludeUnpublished: isAuth,
				Query:              q,
				Tags:               selectedTags,
			})
			if err != nil {
				RenderError(w, req, http.StatusInternalServerError)
				return
			}
			allTags, _ = store.DistinctTags(req.Context(), isAuth)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = journals.CatalogPage(isAuth, journals.CatalogView{
			Entries:      journals.RenderEntries(entries),
			AllTags:      allTags,
			Query:        q,
			SelectedTags: selectedTags,
		}).Render(req.Context(), w)
	})
	r.Get("/page-css/journals.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(journals.StyleCSS))
	})

	// Journal Editor / Create New Entry
	r.Get("/journals/new", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		isAuth := auth.IsAuthenticated(req.Context())
		var existingTags []string
		if store != nil {
			existingTags, _ = store.DistinctTags(req.Context(), true)
		}
		_ = editor.FormPage(isAuth, editor.FormData{
			Published:    true,
			Body:         editor.DefaultBody,
			ExistingTags: existingTags,
		}).Render(req.Context(), w)
	})
	r.Get("/journal-editor", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/journals/new", http.StatusMovedPermanently)
	})
	r.Get("/page-css/journals-new.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(editor.StyleCSS))
	})
	r.Get("/page-css/journal-editor.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(editor.StyleCSS))
	})

	// Single Journal Entry Permalink
	r.Get("/journals/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseEntryID(req)
		if !ok {
			RenderError(w, req, http.StatusNotFound)
			return
		}
		if store == nil {
			RenderError(w, req, http.StatusServiceUnavailable)
			return
		}
		isAuth := auth.IsAuthenticated(req.Context())
		entry, err := store.Get(req.Context(), id, isAuth)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				RenderError(w, req, http.StatusNotFound)
				return
			}
			RenderError(w, req, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = journals.EntryPage(isAuth, journals.RenderSingle(*entry)).Render(req.Context(), w)
	})

	// Protected Journal Author Management Routes
	r.Group(func(adminRouter chi.Router) {
		if authSvc != nil {
			adminRouter.Use(auth.RequireAuth(authSvc))
		} else {
			adminRouter.Use(requireAuthServiceUnavailable)
		}

		adminRouter.Post("/journals", func(w http.ResponseWriter, req *http.Request) {
			if store == nil {
				RenderError(w, req, http.StatusServiceUnavailable)
				return
			}
			in, formData := parseJournalForm(req)
			if _, err := store.Create(req.Context(), in); err != nil {
				if errors.Is(err, db.ErrInvalidInput) {
					formData.ErrorMsg = err.Error()
					formData.ExistingTags, _ = store.DistinctTags(req.Context(), true)
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusUnprocessableEntity)
					_ = editor.FormPage(true, formData).Render(req.Context(), w)
					return
				}
				RenderError(w, req, http.StatusInternalServerError)
				return
			}
			http.Redirect(w, req, "/journals", http.StatusSeeOther)
		})

		adminRouter.Get("/journals/{id}/edit", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseEntryID(req)
			if !ok {
				RenderError(w, req, http.StatusNotFound)
				return
			}
			if store == nil {
				RenderError(w, req, http.StatusServiceUnavailable)
				return
			}
			entry, err := store.Get(req.Context(), id, true)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					RenderError(w, req, http.StatusNotFound)
					return
				}
				RenderError(w, req, http.StatusInternalServerError)
				return
			}
			existingTags, _ := store.DistinctTags(req.Context(), true)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = editor.FormPage(true, editor.FormData{
				IsEdit:       true,
				EntryID:      entry.ID,
				MediaURL:     entry.MediaURL,
				Tags:         entry.TagsString(),
				Body:         entry.Body,
				Bookmarked:   entry.Bookmarked,
				Published:    entry.Published,
				ExistingTags: existingTags,
			}).Render(req.Context(), w)
		})

		adminRouter.Post("/journals/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseEntryID(req)
			if !ok {
				RenderError(w, req, http.StatusNotFound)
				return
			}
			if store == nil {
				RenderError(w, req, http.StatusServiceUnavailable)
				return
			}
			in, formData := parseJournalForm(req)
			formData.IsEdit = true
			formData.EntryID = id
			if err := store.Update(req.Context(), id, in); err != nil {
				if errors.Is(err, db.ErrInvalidInput) {
					formData.ErrorMsg = err.Error()
					formData.ExistingTags, _ = store.DistinctTags(req.Context(), true)
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusUnprocessableEntity)
					_ = editor.FormPage(true, formData).Render(req.Context(), w)
					return
				}
				if errors.Is(err, db.ErrNotFound) {
					RenderError(w, req, http.StatusNotFound)
					return
				}
				RenderError(w, req, http.StatusInternalServerError)
				return
			}
			http.Redirect(w, req, "/journals", http.StatusSeeOther)
		})

		adminRouter.Post("/journals/{id}/delete", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseEntryID(req)
			if !ok {
				RenderError(w, req, http.StatusNotFound)
				return
			}
			if store == nil {
				RenderError(w, req, http.StatusServiceUnavailable)
				return
			}
			if err := store.Delete(req.Context(), []int64{id}); err != nil {
				RenderError(w, req, http.StatusInternalServerError)
				return
			}
			http.Redirect(w, req, "/journals", http.StatusSeeOther)
		})

		adminRouter.Post("/journals/{id}/publish", func(w http.ResponseWriter, req *http.Request) {
			handleSingleStatusChange(w, req, store, true)
		})

		adminRouter.Post("/journals/{id}/unpublish", func(w http.ResponseWriter, req *http.Request) {
			handleSingleStatusChange(w, req, store, false)
		})

		adminRouter.Post("/journals/{id}/bookmark", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseEntryID(req)
			if !ok {
				RenderError(w, req, http.StatusNotFound)
				return
			}
			if store == nil {
				RenderError(w, req, http.StatusServiceUnavailable)
				return
			}
			if _, err := store.ToggleBookmark(req.Context(), id); err != nil {
				if errors.Is(err, db.ErrNotFound) {
					RenderError(w, req, http.StatusNotFound)
					return
				}
				RenderError(w, req, http.StatusInternalServerError)
				return
			}
			http.Redirect(w, req, entryReturnTarget(req, id), http.StatusSeeOther)
		})

		adminRouter.Post("/journals/batch", func(w http.ResponseWriter, req *http.Request) {
			if store == nil {
				RenderError(w, req, http.StatusServiceUnavailable)
				return
			}
			_ = req.ParseForm()
			action := req.FormValue("action")
			var ids []int64
			for _, rawID := range req.Form["ids"] {
				if id, err := strconv.ParseInt(rawID, 10, 64); err == nil && id > 0 {
					ids = append(ids, id)
				}
			}

			var err error
			switch action {
			case "publish_selected":
				err = store.SetPublished(req.Context(), ids, true)
			case "unpublish_selected":
				err = store.SetPublished(req.Context(), ids, false)
			case "delete_selected":
				err = store.Delete(req.Context(), ids)
			case "unpublish_all":
				err = store.SetAllPublished(req.Context(), false)
			case "delete_all":
				err = store.DeleteAll(req.Context())
			default:
				RenderError(w, req, http.StatusBadRequest)
				return
			}
			if err != nil {
				RenderError(w, req, http.StatusInternalServerError)
				return
			}
			http.Redirect(w, req, "/journals", http.StatusSeeOther)
		})
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
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = login.Page(token, sshCommand, next, sshPort).Render(req.Context(), w)
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

	// Pilot Experimental Namespace (Protected)
	r.Group(func(pilotRouter chi.Router) {
		if authSvc != nil {
			pilotRouter.Use(auth.RequireAuth(authSvc))
		} else {
			pilotRouter.Use(requireAuthServiceUnavailable)
		}

		pilotRouter.Get("/pilot", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "/pilot/user/settings", http.StatusTemporaryRedirect)
		})

		pilotRouter.Get("/pilot/user/settings", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			keyType := "ssh-ed25519"
			if authSvc != nil {
				keyType = authSvc.AuthorizedKeyType()
			}
			_ = pilot.SettingsPage(keyType).Render(req.Context(), w)
		})
	})
	r.Get("/page-css/pilot-settings.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte(pilot.StyleCSS))
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

func requireAuthServiceUnavailable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		RenderError(w, req, http.StatusServiceUnavailable)
	})
}

func parseEntryID(req *http.Request) (int64, bool) {
	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	return id, err == nil && id > 0
}

func parseJournalForm(req *http.Request) (db.JournalInput, editor.FormData) {
	_ = req.ParseForm()
	mediaURL := req.FormValue("media_url")
	tagsRaw := req.FormValue("tags")
	body := req.FormValue("body")
	published := req.FormValue("published") == "1"
	bookmarked := req.FormValue("bookmarked") == "1"

	in := db.JournalInput{
		MediaURL:   mediaURL,
		Tags:       db.ParseTags(tagsRaw),
		Body:       body,
		Published:  published,
		Bookmarked: bookmarked,
	}
	formData := editor.FormData{
		MediaURL:   mediaURL,
		Tags:       tagsRaw,
		Body:       body,
		Published:  published,
		Bookmarked: bookmarked,
	}
	return in, formData
}

func entryReturnTarget(req *http.Request, id int64) string {
	ref := req.Header.Get("Referer")
	entryPath := fmt.Sprintf("/journals/%d", id)
	if strings.Contains(ref, entryPath) {
		return entryPath
	}
	return "/journals"
}

func handleSingleStatusChange(w http.ResponseWriter, req *http.Request, store *db.Store, published bool) {
	id, ok := parseEntryID(req)
	if !ok {
		RenderError(w, req, http.StatusNotFound)
		return
	}
	if store == nil {
		RenderError(w, req, http.StatusServiceUnavailable)
		return
	}
	if err := store.SetPublished(req.Context(), []int64{id}, published); err != nil {
		RenderError(w, req, http.StatusInternalServerError)
		return
	}
	http.Redirect(w, req, entryReturnTarget(req, id), http.StatusSeeOther)
}
