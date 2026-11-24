package routes

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

// Destructive actions ask in place (views/confirm) instead of confirm():
// the GET returns the prompt (fragment for HTMX, page otherwise), and the
// POST refuses to act unless the prompt's button sent confirmed=1.
func TestDestructiveActionsAskFirst(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	authSvc, sess := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	router := NewRouter(Config{AuthService: authSvc, Store: store})

	do := func(method, path string, form url.Values, htmx, signedIn bool) *httptest.ResponseRecorder {
		var req *http.Request
		if form != nil {
			req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		if htmx {
			req.Header.Set("HX-Request", "true")
		}
		if signedIn {
			req.AddCookie(sess)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	count := func() int {
		body := html.UnescapeString(do(http.MethodGet, "/journals/content", nil, false, true).Body.String())
		return strings.Count(body, `<article class="journal-card`)
	}
	for i := 1; i <= 3; i++ {
		form := url.Values{"media_url": {fmt.Sprintf("https://example.com/%d.png", i)}, "tags": {"Log"}, "body": {fmt.Sprintf("Entry %d.", i)}, "published": {"1"}}
		if rec := do(http.MethodPost, "/journals", form, false, true); rec.Code != http.StatusSeeOther {
			t.Fatalf("create %d: %d", i, rec.Code)
		}
	}
	if count() != 3 {
		t.Fatalf("setup: want 3 entries, got %d", count())
	}
	list, _ := store.List(ctx, true)
	one := fmt.Sprintf("/journals/%d/delete", list[0].ID)
	two, three := fmt.Sprint(list[1].ID), fmt.Sprint(list[2].ID)

	// The catalog never relies on the browser's dialog.
	catalog := do(http.MethodGet, "/journals/content", nil, false, true).Body.String()
	if strings.Contains(catalog, "data-confirm=") {
		t.Errorf("catalog still uses data-confirm (browser confirm())")
	}
	if !strings.Contains(catalog, `hx-get="`+one+`"`) {
		t.Errorf("entry delete should fetch its prompt with HTMX")
	}

	// Signed out: no prompt, sent to sign in.
	if rec := do(http.MethodGet, one, nil, true, false); rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Errorf("signed-out prompt: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	// HTMX gets a fragment; a plain GET gets a whole page. Both post confirmed=1.
	frag := do(http.MethodGet, one, nil, true, true).Body.String()
	if strings.Contains(frag, "<html") || !strings.Contains(frag, "Delete this journal entry permanently?") ||
		!strings.Contains(frag, `action="`+one+`"`) || !strings.Contains(frag, `name="confirmed" value="1"`) {
		t.Errorf("inline prompt: %s", frag)
	}
	if page := do(http.MethodGet, one, nil, false, true).Body.String(); !strings.Contains(page, "<html") || !strings.Contains(page, "Delete this journal entry permanently?") {
		t.Errorf("no-JS prompt should be a page")
	}

	// A POST without the answer is sent to the question, and deletes nothing.
	if rec := do(http.MethodPost, one, url.Values{}, false, true); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != one || count() != 3 {
		t.Errorf("unconfirmed delete: %d %q, %d entries", rec.Code, rec.Header().Get("Location"), count())
	}
	if rec := do(http.MethodPost, one, url.Values{"confirmed": {"1"}}, false, true); rec.Code != http.StatusSeeOther || count() != 2 {
		t.Errorf("confirmed delete: %d, %d entries", rec.Code, count())
	}

	// Batch: the prompt counts the selection and submits the batch form.
	bf := do(http.MethodGet, "/journals/batch/confirm?action=delete_selected&ids="+two+"&ids="+three, nil, true, true).Body.String()
	for _, want := range []string{"Delete 2 selected entries permanently?", `form="batch-form"`, `name="action" value="delete_selected"`, `name="confirmed" value="1"`} {
		if !strings.Contains(html.UnescapeString(bf), want) {
			t.Errorf("batch prompt missing %q: %s", want, bf)
		}
	}
	if rec := do(http.MethodGet, "/journals/batch/confirm?action=publish_selected", nil, true, true); rec.Code != http.StatusBadRequest {
		t.Errorf("non-destructive batch actions have no prompt: %d", rec.Code)
	}
	if rec := do(http.MethodPost, "/journals/batch", url.Values{"action": {"delete_all"}}, false, true); rec.Code != http.StatusBadRequest || count() != 2 {
		t.Errorf("unconfirmed delete_all: %d, %d entries", rec.Code, count())
	}
	if rec := do(http.MethodPost, "/journals/batch", url.Values{"action": {"delete_all"}, "confirmed": {"1"}}, false, true); rec.Code != http.StatusSeeOther || count() != 0 {
		t.Errorf("confirmed delete_all: %d, %d entries", rec.Code, count())
	}
}
