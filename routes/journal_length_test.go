package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/domain"
)

// A journal body over the cap is refused end to end (nothing stored), and the
// editor shows the cap from the same domain constant.
func TestJournalBodyCapEndToEnd(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	authSvc, session := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	router := NewRouter(Config{Store: store, AuthService: authSvc})

	post := func(body string) *httptest.ResponseRecorder {
		form := url.Values{"media_url": {"https://example.com/a.png"}, "tags": {"Log"}, "body": {body}, "published": {"1"}}
		req := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	if rec := post(strings.Repeat("a", domain.MaxJournalBodyLength+1)); rec.Code == http.StatusSeeOther {
		t.Fatalf("over-cap body was accepted (303)")
	}
	if all, _ := store.List(ctx, true); len(all) != 0 {
		t.Fatalf("over-cap body was stored: %d entries", len(all))
	}
	if rec := post(strings.Repeat("λ", domain.MaxJournalBodyLength)); rec.Code != http.StatusSeeOther {
		t.Fatalf("a body exactly at the cap (Greek) should be accepted, got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/journals/new", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	html := rec.Body.String()
	cap := strconv.Itoa(domain.MaxJournalBodyLength)
	for _, want := range []string{
		`maxlength="` + cap + `"`,
		`id="body-char-count"`,
		" / " + cap + "</span>", // the counter (the new-entry form is pre-filled, so not necessarily 0)
		"Up to 2,000 characters.",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("editor is missing %q", want)
		}
	}
}
