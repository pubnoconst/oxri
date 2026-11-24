package routes

import (
	"context"
	"errors"
	"fmt"
	"github.com/oxri/oxri-go/internal/domain"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/paging"
	"github.com/oxri/oxri-go/internal/tweet"
	"github.com/oxri/oxri-go/internal/verse"
	"github.com/oxri/oxri-go/serverfns"
	"github.com/oxri/oxri-go/views/confirm"
	"github.com/oxri/oxri-go/views/editor"
	"github.com/oxri/oxri-go/views/errorpage"
	"github.com/oxri/oxri-go/views/journals"
)

func renderInternalError(w http.ResponseWriter, r *http.Request, status int, cause error) {
	ref := errorpage.NewReference()
	if cause != nil {
		log.Printf("[HTTP %d] Ref: %s | Method: %s | URL: %s | Cause: %v", status, ref, r.Method, r.URL.Path, cause)
	}
	errorpage.RenderWithReference(w, status, "", ref)
}

// handleJournalsCatalog renders the /journals page frame immediately (0ms, independent of DB).
// The browser then fetches /journals/content asynchronously via HTMX to populate the catalog.
func handleJournalsCatalog(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, authErr := serverfns.RequireAuth(authSvc, r)
		isAuth := (authErr == nil)

		contentURL := "/journals/content"
		if r.URL.RawQuery != "" {
			contentURL += "?" + r.URL.RawQuery
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = journals.CatalogShellPage(isAuth, contentURL).Render(w)
	}
}

// handleJournalsCatalogContent queries the remote Turso DB and renders the catalog fragment
// swapped into #journals-catalog-stage by HTMX.
func handleJournalsCatalogContent(store *db.Store, authSvc *auth.Service, tweets *tweet.Filler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, authErr := serverfns.RequireAuth(authSvc, r)
		isAuth := (authErr == nil)

		q := db.SanitizeSearchQuery(r.URL.Query().Get("q"))
		var selectedTags []string
		for _, rawTag := range r.URL.Query()["tag"] {
			if norm := db.FormatTagTitleCase(rawTag); norm != "" {
				if _, ok := db.ValidateTagChars(norm); ok {
					selectedTags = append(selectedTags, norm)
				}
			}
		}

		var entries []db.Journal
		var allTags []string
		var page paging.Page
		if store != nil {
			var err error
			entries, err = store.ListFiltered(r.Context(), db.ListFilter{
				IncludeUnpublished: isAuth,
				Query:              q,
				Tags:               selectedTags,
			})
			if err != nil {
				renderInternalError(w, r, http.StatusInternalServerError, err)
				return
			}
			allTags, _ = store.TagsByUse(r.Context(), isAuth)
		}
		// Only this page's entries are rendered (Markdown included): the DOM and
		// the number of tweet iframes stay bounded however large the archive grows.
		entries, page = paging.Slice(entries, paging.Parse(r.URL.Query().Get("page")), journals.CatalogPageSize)

		rendered := journals.RenderEntries(entries)
		attachTweets(r.Context(), store, tweets, rendered)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = journals.CatalogContentFragment(isAuth, journals.CatalogView{
			Entries:      rendered,
			AllTags:      allTags,
			Query:        q,
			SelectedTags: selectedTags,
			Page:         page,
		}).Render(w)
	}
}

func handleJournalNew(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, authErr := serverfns.RequireAuth(authSvc, r)
		isAuth := (authErr == nil)

		var existingTags []string
		if store != nil {
			// Non-blocking read of cached tags (never blocks page load on remote DB)
			existingTags = store.TagsByUseCachedOnly(isAuth)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = editor.FormPage(isAuth, editor.FormData{
			Published:    true,
			Body:         editor.DefaultBody,
			ExistingTags: existingTags,
		}).Render(w)
	}
}

func handleJournalEntry(store *db.Store, authSvc *auth.Service, tweets *tweet.Filler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseEntryID(r)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		_, authErr := serverfns.RequireAuth(authSvc, r)
		isAuth := (authErr == nil)

		entry, err := store.Get(r.Context(), id, isAuth)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				errorpage.Render(w, http.StatusNotFound, "")
				return
			}
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		rendered := []journals.RenderedEntry{journals.RenderSingle(*entry)}
		attachTweets(r.Context(), store, tweets, rendered)
		_ = journals.EntryPage(isAuth, rendered[0]).Render(w)
	}
}

func handleJournalCreate(store *db.Store, authSvc *auth.Service, passages *verse.Source) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}

		in, formData := parseJournalForm(r)
		validated, err := validateJournal(r.Context(), passages, in, formData.Scripture, nil)
		if err != nil {
			sanitizeFormDataForEcho(&formData)
			formData.ErrorMsg = err.Error()
			formData.ExistingTags, _ = store.TagsByUse(r.Context(), true)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = editor.FormPage(true, formData).Render(w)
			return
		}
		if _, err := store.CreateValidated(r.Context(), cap, validated); err != nil {
			if errors.Is(err, db.ErrQuotaExceeded) {
				sanitizeFormDataForEcho(&formData)
				_ = store.SetPendingSave(cap, db.PendingJournal{
					IsEdit:     false,
					MediaURL:   formData.MediaURL,
					TagsRaw:    formData.Tags,
					Body:       formData.Body,
					Scripture:  formData.Scripture,
					Published:  formData.Published,
					Bookmarked: formData.Bookmarked,
					EntryBytes: validated.EstimatedBytes(db.RowOverheadBytes),
				})
				http.Redirect(w, r, "/journals/vacate-space/", http.StatusSeeOther)
				return
			}
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, "/journals", http.StatusSeeOther)
	})
}

func handleJournalEditForm(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		id, ok := parseEntryID(r)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		entry, err := store.Get(r.Context(), id, true)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				errorpage.Render(w, http.StatusNotFound, "")
				return
			}
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		existingTags, _ := store.TagsByUse(r.Context(), true)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := editor.FormData{
			IsEdit:       true,
			EntryID:      entry.ID,
			MediaURL:     entry.MediaURL,
			Tags:         entry.TagsString(),
			Body:         entry.Body,
			Bookmarked:   entry.Bookmarked,
			Published:    entry.Published,
			ExistingTags: existingTags,
		}
		if p, ok := entry.Passage(); ok {
			data.MediaURL = "" // the provider's chapter file, not a link to show
			data.Scripture = scriptureFromPassage(p)
		}
		_ = editor.FormPage(true, data).Render(w)
	})
}

func handleJournalUpdate(store *db.Store, authSvc *auth.Service, passages *verse.Source) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		id, ok := parseEntryID(r)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		in, formData := parseJournalForm(r)
		formData.IsEdit = true
		formData.EntryID = id
		prev, getErr := store.Get(r.Context(), id, true)
		if getErr != nil && !errors.Is(getErr, db.ErrNotFound) {
			renderInternalError(w, r, http.StatusInternalServerError, getErr)
			return
		}
		validated, err := validateJournal(r.Context(), passages, in, formData.Scripture, prev)
		if err != nil {
			sanitizeFormDataForEcho(&formData)
			formData.ErrorMsg = err.Error()
			formData.ExistingTags, _ = store.TagsByUse(r.Context(), true)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = editor.FormPage(true, formData).Render(w)
			return
		}
		if err := store.UpdateValidated(r.Context(), cap, id, validated); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				errorpage.Render(w, http.StatusNotFound, "")
				return
			}
			if errors.Is(err, db.ErrQuotaExceeded) {
				sanitizeFormDataForEcho(&formData)
				_ = store.SetPendingSave(cap, db.PendingJournal{
					IsEdit:     true,
					EntryID:    id,
					MediaURL:   formData.MediaURL,
					TagsRaw:    formData.Tags,
					Body:       formData.Body,
					Scripture:  formData.Scripture,
					Published:  formData.Published,
					Bookmarked: formData.Bookmarked,
					EntryBytes: validated.EstimatedBytes(db.RowOverheadBytes),
				})
				http.Redirect(w, r, "/journals/vacate-space/", http.StatusSeeOther)
				return
			}
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, "/journals", http.StatusSeeOther)
	})
}

func handleJournalVacateSpacePage(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		entries, err := store.List(r.Context(), true)
		if err != nil {
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		requiredBytes, entryBytes, usedBytes, limitBytes, err := store.RequiredBytesForPending(r.Context())
		if err != nil {
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		autoPlan, err := store.PreviewAutoVacate(r.Context(), requiredBytes)
		if err != nil {
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		pending := store.GetPendingSave()
		autoResubmit := r.URL.Query().Get("vacated") == "1" && requiredBytes <= 0 && pending != nil

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = journals.VacateSpacePage(journals.VacateSpaceView{
			Entries:           journals.RenderEntries(entries),
			RequiredBytes:     requiredBytes,
			PendingEntryBytes: entryBytes,
			UsedBytes:         usedBytes,
			LimitBytes:        limitBytes,
			AutoPlan:          autoPlan,
			Pending:           pending,
			AutoResubmitReady: autoResubmit,
		}).Render(w)
	})
}

func handleJournalVacateSpaceSubmit(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		_ = r.ParseForm()
		action := r.FormValue("action")

		switch action {
		case "auto_vacate":
			requiredBytes, _, _, _, err := store.RequiredBytesForPending(r.Context())
			if err != nil {
				renderInternalError(w, r, http.StatusInternalServerError, err)
				return
			}
			if requiredBytes <= 0 {
				if rawReq := r.FormValue("required_bytes"); rawReq != "" {
					if parsed, parseErr := strconv.ParseInt(rawReq, 10, 64); parseErr == nil && parsed > 0 {
						requiredBytes = parsed
					}
				}
			}
			if _, err := store.AutoVacateBytes(r.Context(), cap, requiredBytes); err != nil {
				renderInternalError(w, r, http.StatusInternalServerError, err)
				return
			}
		case "delete_selected":
			var ids []int64
			for _, rawID := range r.Form["ids"] {
				if id, err := strconv.ParseInt(rawID, 10, 64); err == nil && id > 0 {
					ids = append(ids, id)
				}
			}
			if err := store.Delete(r.Context(), cap, ids); err != nil {
				renderInternalError(w, r, http.StatusInternalServerError, err)
				return
			}
		default:
			errorpage.Render(w, http.StatusBadRequest, "")
			return
		}

		http.Redirect(w, r, "/journals/vacate-space/?vacated=1", http.StatusSeeOther)
	})
}

func handleJournalDelete(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		id, ok := parseEntryID(r)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if !confirm.Confirmed(r) {
			http.Redirect(w, r, fmt.Sprintf("/journals/%d/delete", id), http.StatusSeeOther)
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		if err := store.Delete(r.Context(), cap, []int64{id}); err != nil {
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, catalogReturnTarget(r), http.StatusSeeOther)
	})
}

func handleJournalPublish(store *db.Store, authSvc *auth.Service, published bool) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		id, ok := parseEntryID(r)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		if err := store.SetPublished(r.Context(), cap, []int64{id}, published); err != nil {
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, entryReturnTarget(r, id), http.StatusSeeOther)
	})
}

func handleJournalBookmark(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		id, ok := parseEntryID(r)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		if _, err := store.ToggleBookmark(r.Context(), cap, id); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				errorpage.Render(w, http.StatusNotFound, "")
				return
			}
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, entryReturnTarget(r, id), http.StatusSeeOther)
	})
}

func handleJournalBatch(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		if store == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "")
			return
		}
		_ = r.ParseForm()
		action := r.FormValue("action")
		ids := parseIDs(r.Form["ids"])
		if _, mustConfirm := batchPrompts[action]; mustConfirm && !confirm.Confirmed(r) {
			errorpage.Render(w, http.StatusBadRequest, "Confirm that first.")
			return
		}

		var err error
		switch action {
		case "publish_selected":
			err = store.SetPublished(r.Context(), cap, ids, true)
		case "unpublish_selected":
			err = store.SetPublished(r.Context(), cap, ids, false)
		case "delete_selected":
			err = store.Delete(r.Context(), cap, ids)
		case "unpublish_all":
			err = store.SetAllPublished(r.Context(), cap, false)
		case "delete_all":
			err = store.DeleteAll(r.Context(), cap)
		default:
			errorpage.Render(w, http.StatusBadRequest, "")
			return
		}
		if err != nil {
			renderInternalError(w, r, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, catalogReturnTarget(r), http.StatusSeeOther)
	})
}

// parseIDs keeps the positive integer ids among untrusted form values.
func parseIDs(raw []string) []int64 {
	var ids []int64
	for _, rawID := range raw {
		if id, err := strconv.ParseInt(rawID, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func parseEntryID(r *http.Request) (int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	return id, err == nil && id > 0
}

// validateJournal turns the posted fields into an entry ready to store:
// the media is either the link, or the Bible passage the scripture fields
// name, checked against its provider (prev is the entry being edited).
func validateJournal(ctx context.Context, passages *verse.Source, in db.JournalInput, s db.ScriptureInput, prev *db.Journal) (domain.ValidatedJournalEntry, error) {
	choice, isBible, err := scriptureChoice(&in, s)
	if err != nil {
		return domain.ValidatedJournalEntry{}, err
	}
	entry, err := db.ParseValidatedEntry(in)
	if err != nil || !isBible {
		return entry, err
	}
	return quotePassage(ctx, passages, entry, choice, s.Text, prev)
}

func parseJournalForm(r *http.Request) (db.JournalInput, editor.FormData) {
	_ = r.ParseForm()
	mediaURL := r.FormValue("media_url")
	tagsRaw := r.FormValue("tags")
	body := r.FormValue("body")
	scripture := db.ScriptureInput{
		Kind:              r.FormValue("media_kind"),
		Scripture:         r.FormValue("scripture"),
		Book:              r.FormValue("verse_book"),
		From:              r.FormValue("verse_from"),
		To:                r.FormValue("verse_to"),
		TranslationChoice: r.FormValue("verse_tr_choice"),
		Translation:       r.FormValue("verse_tr"),
		Text:              r.FormValue("verse_text"),
	}
	published := r.FormValue("published") == "1"
	bookmarked := r.FormValue("bookmarked") == "1"

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
		Scripture:  scripture,
		Published:  published,
		Bookmarked: bookmarked,
	}
	return in, formData
}

func sanitizeFormDataForEcho(fd *editor.FormData) {
	if fd == nil {
		return
	}
	fd.MediaURL = db.SanitizeMediaURLInput(fd.MediaURL)
	fd.Tags = db.SanitizeTagsRawInput(fd.Tags)
	fd.Body = db.SanitizeProseInput(fd.Body)
	fd.Scripture = sanitizeScriptureForEcho(fd.Scripture)
}

func entryReturnTarget(r *http.Request, id int64) string {
	ref := r.Header.Get("Referer")
	entryPath := fmt.Sprintf("/journals/%d", id)
	if strings.Contains(ref, entryPath) {
		return entryPath
	}
	return catalogReturnTarget(r)
}

// catalogReturnTarget sends the author back to the catalog page (with its
// search, tags and page) that an action was taken from. Only a same-host
// Referer whose path is exactly /journals is trusted, and only its path and
// query are used, so this can never redirect off-site. Otherwise: /journals.
// A page that no longer exists (e.g. after deleting its entries) clamps to
// the last page when rendered.
func catalogReturnTarget(r *http.Request) string {
	ref, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || ref.Host != r.Host || ref.Path != "/journals" {
		return "/journals"
	}
	if ref.RawQuery == "" {
		return "/journals"
	}
	return "/journals?" + url.Values(ref.Query()).Encode()
}
