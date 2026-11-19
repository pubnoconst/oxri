package app

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/pages/editor"
	"github.com/oxri/oxri-go/internal/pages/journals"
)

func handleJournalsCatalog(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
				RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
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
	}
}

func handleJournalNew(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
	}
}

func handleJournalEntry(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = journals.EntryPage(isAuth, journals.RenderSingle(*entry)).Render(req.Context(), w)
	}
}

func handleJournalCreate(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, req, "/journals", http.StatusSeeOther)
	}
}

func handleJournalEditForm(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
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
	}
}

func handleJournalUpdate(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, req, "/journals", http.StatusSeeOther)
	}
}

func handleJournalDelete(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, req, "/journals", http.StatusSeeOther)
	}
}

func handleJournalPublish(store *db.Store, published bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, req, entryReturnTarget(req, id), http.StatusSeeOther)
	}
}

func handleJournalBookmark(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, req, entryReturnTarget(req, id), http.StatusSeeOther)
	}
}

func handleJournalBatch(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, req, "/journals", http.StatusSeeOther)
	}
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
