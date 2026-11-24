package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/domain"
	"github.com/oxri/oxri-go/views/confirm"
	"github.com/oxri/oxri-go/views/errorpage"
)

// Confirmation prompts for destructive actions (views/confirm). Each GET
// returns the prompt; the matching POST refuses to act without confirmed=1.

// handleJournalDeleteConfirm: GET /journals/{id}/delete.
func handleJournalDeleteConfirm(authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, _ domain.AdminCap) {
		id, ok := parseEntryID(r)
		if !ok {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		confirm.Render(w, r, "Delete entry", confirm.Prompt{
			Question: "Delete this journal entry permanently?",
			Confirm:  "Delete entry",
			Danger:   true,
			Target:   confirm.PostTo(fmt.Sprintf("/journals/%d/delete", id)),
			KeepHref: entryReturnTarget(r, id),
		})
	})
}

// batchPrompts are the batch actions that must be confirmed. The others
// (publish or unpublish the selection) act at once: they are undone by the
// opposite button.
var batchPrompts = map[string]func(selected int) (confirm.Prompt, bool){
	"delete_selected": func(n int) (confirm.Prompt, bool) {
		if n == 0 {
			return confirm.Prompt{}, false
		}
		return confirm.Prompt{
			Question: fmt.Sprintf("Delete %s permanently?", entriesPhrase(n, "selected ")),
			Confirm:  "Delete " + entriesPhrase(n, ""),
			Danger:   true,
		}, true
	},
	"delete_all": func(int) (confirm.Prompt, bool) {
		return confirm.Prompt{
			Question: "Delete every journal entry permanently? This can't be undone.",
			Confirm:  "Delete all entries",
			Danger:   true,
		}, true
	},
	"unpublish_all": func(int) (confirm.Prompt, bool) {
		return confirm.Prompt{
			Question: "Hide every journal entry from the public catalog?",
			Confirm:  "Unpublish all",
		}, true
	},
}

// entriesPhrase: "1 selected entry", "3 selected entries".
func entriesPhrase(n int, qualifier string) string {
	if n == 1 {
		return "1 " + qualifier + "entry"
	}
	return fmt.Sprintf("%d %sentries", n, qualifier)
}

// handleJournalBatchConfirm: GET /journals/batch/confirm?action=…&ids=…
// (HTMX only: the batch bar lives in the HTMX-loaded catalog).
func handleJournalBatchConfirm(authSvc *auth.Service) http.HandlerFunc {
	return adminOnly(authSvc, func(w http.ResponseWriter, r *http.Request, _ domain.AdminCap) {
		action := r.URL.Query().Get("action")
		prompt, known := batchPrompts[action]
		if !known {
			errorpage.Render(w, http.StatusBadRequest, "")
			return
		}
		p, ok := prompt(len(parseIDs(r.URL.Query()["ids"])))
		if !ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<p class="confirm-question">Select entries first.</p>`))
			return
		}
		p.Target = confirm.SubmitForm("batch-form")
		p.Submit = confirm.Field{Name: "action", Value: action}
		p.KeepHref = "/journals"
		confirm.Render(w, r, "Confirm", p)
	})
}

// handlePasskeyDeleteConfirm: GET /settings/passkeys/{id}/delete.
func handlePasskeyDeleteConfirm(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return freshAdminOnly(authSvc, denyToSettingsSignIn, func(w http.ResponseWriter, r *http.Request, _ domain.AdminCap) {
		id := strings.TrimSpace(chi.URLParam(r, "id"))
		name := ""
		if rpID, _, supported := auth.ResolveWebAuthnOrigin(r, isHTTPSRequest(r)); supported && store != nil {
			if recs, err := store.ListPasskeys(r.Context(), rpID); err == nil {
				for _, rec := range recs {
					if rec.ID == id {
						name = rec.Name
					}
				}
			}
		}
		if name == "" {
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
		confirm.Render(w, r, "Remove passkey", confirm.Prompt{
			Question: fmt.Sprintf("Remove the passkey “%s”? You won't be able to sign in with it.", name),
			Confirm:  "Remove passkey",
			Danger:   true,
			Target:   confirm.PostTo("/settings/passkeys/" + url.PathEscape(id) + "/delete"),
			KeepHref: "/settings",
		})
	})
}
