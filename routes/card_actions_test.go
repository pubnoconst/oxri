package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/domain"
)

// The catalog's entry cards are frozen (HIG §4, owner 2026-11-25): the
// author can edit, bookmark, publish or unpublish, and delete an entry from
// its card in the catalog, without opening it, and every action but Edit
// lands back on the same catalog page (search, tags and page kept). Readers
// see none of these controls. This test fails if any of that is lost.
func TestCatalogCardsKeepTheirActions(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	authSvc, session := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	cap, ok := authSvc.Authenticate(session.Value)
	if !ok {
		t.Fatal("no admin capability")
	}
	router := NewRouter(Config{Store: store, AuthService: authSvc})

	create := func(body string, published bool) int64 {
		e, err := domain.ParseJournalEntry("https://example.org/a.png", []string{"Good"}, body, false, published)
		if err != nil {
			t.Fatal(err)
		}
		j, err := store.CreateValidated(ctx, cap, e)
		if err != nil {
			t.Fatal(err)
		}
		return j.ID
	}
	published := create("A published entry", true)
	draft := create("An unpublished draft", false)

	get := func(signedIn bool) string {
		req := httptest.NewRequest(http.MethodGet, "/journals", nil)
		if signedIn {
			req.AddCookie(session)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	page := get(true)
	for _, id := range []int64{published, draft} {
		for _, want := range []string{
			fmt.Sprintf(`href="/journals/%d/edit"`, id),
			fmt.Sprintf(`action="/journals/%d/bookmark"`, id),
			fmt.Sprintf(`href="/journals/%d/delete"`, id),
			fmt.Sprintf(`hx-get="/journals/%d/delete"`, id), // the prompt opens in place
		} {
			if !strings.Contains(page, want) {
				t.Errorf("entry %d's card lacks %s", id, want)
			}
		}
	}
	if !strings.Contains(page, fmt.Sprintf(`action="/journals/%d/unpublish"`, published)) {
		t.Error("a published entry's card lacks Unpublish")
	}
	if !strings.Contains(page, fmt.Sprintf(`action="/journals/%d/publish"`, draft)) {
		t.Error("a draft's card lacks Publish")
	}

	if reader := get(false); strings.Contains(reader, "/bookmark") || strings.Contains(reader, "/edit") ||
		strings.Contains(reader, "/delete") || strings.Contains(reader, "/publish") {
		t.Error("a reader is shown the author's card controls")
	}

	// Each action returns to the catalog page it was taken from.
	from := "http://example.com/journals?page=1&tag=Good"
	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Referer", from)
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	back := "/journals?page=1&tag=Good"
	check := func(what string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != back {
			t.Errorf("%s: got %d to %q, want 303 back to %q", what, rec.Code, rec.Header().Get("Location"), back)
		}
	}

	check("bookmark", post(fmt.Sprintf("/journals/%d/bookmark", published), nil))
	if e, _ := store.Get(ctx, published, true); e == nil || !e.Bookmarked {
		t.Error("bookmark did not take")
	}
	check("unpublish", post(fmt.Sprintf("/journals/%d/unpublish", published), nil))
	if e, _ := store.Get(ctx, published, true); e == nil || e.Published {
		t.Error("unpublish did not take")
	}
	check("publish", post(fmt.Sprintf("/journals/%d/publish", draft), nil))
	if e, _ := store.Get(ctx, draft, true); e == nil || !e.Published {
		t.Error("publish did not take")
	}
	check("delete", post(fmt.Sprintf("/journals/%d/delete", draft), url.Values{"confirmed": {"1"}}))
	if e, _ := store.Get(ctx, draft, true); e != nil {
		t.Error("delete did not take")
	}
}
