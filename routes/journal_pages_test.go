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
	"github.com/oxri/oxri-go/views/journals"
)

func TestJournalCatalogIsPaginated(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	authSvc, sess := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	router := NewRouter(Config{AuthService: authSvc, Store: store})

	do := func(method, path string, form url.Values, referer string) *httptest.ResponseRecorder {
		var req *http.Request
		if form != nil {
			req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		req.AddCookie(sess)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// 30 published entries, created oldest first: "Entry 30" is newest. The
	// even ones are tagged Dev so a filter spans pages too.
	const n = 30
	for i := 1; i <= n; i++ {
		tags := "Log"
		if i%2 == 0 {
			tags = "Log, Dev"
		}
		form := url.Values{"media_url": {fmt.Sprintf("https://example.com/%d.png", i)}, "tags": {tags}, "body": {fmt.Sprintf("Entry %d marker.", i)}, "published": {"1"}}
		if rec := do(http.MethodPost, "/journals", form, ""); rec.Code != http.StatusSeeOther {
			t.Fatalf("create %d: %d", i, rec.Code)
		}
	}

	page := func(path string) string {
		rec := do(http.MethodGet, path, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, rec.Code)
		}
		return html.UnescapeString(rec.Body.String())
	}
	cards := func(body string) int { return strings.Count(body, "<article class=\"journal-card") }
	size := journals.CatalogPageSize

	p1 := page("/journals/content")
	if got := cards(p1); got != size {
		t.Fatalf("page 1 shows %d cards, want %d", got, size)
	}
	for _, want := range []string{"Entry 30 marker.", "Page 1 of 3", `href="/journals?page=2"`, "Select all on this page"} {
		if !strings.Contains(p1, want) {
			t.Errorf("page 1: missing %q", want)
		}
	}
	if strings.Contains(p1, "Entry 18 marker.") {
		t.Errorf("page 1 must not include page 2's entries")
	}

	p3 := page("/journals/content?page=3")
	if got := cards(p3); got != n-2*size {
		t.Fatalf("page 3 shows %d cards, want %d", got, n-2*size)
	}
	if !strings.Contains(p3, "Page 3 of 3") || !strings.Contains(p3, "Entry 1 marker.") || !strings.Contains(p3, `href="/journals?page=2"`) {
		t.Errorf("page 3: wrong status, entries or Newer link")
	}
	if strings.Contains(p3, `rel="next"`) {
		t.Errorf("the last page has no Older link")
	}

	// Untrusted page numbers clamp rather than error.
	if !strings.Contains(page("/journals/content?page=999"), "Page 3 of 3") || !strings.Contains(page("/journals/content?page=abc"), "Page 1 of 3") {
		t.Errorf("out-of-range or junk page numbers must clamp")
	}

	// Filters carry across pages: 15 Dev entries -> 2 pages.
	dev := page("/journals/content?tag=Dev&page=2")
	if !strings.Contains(dev, "Page 2 of 2") || !strings.Contains(dev, `href="/journals?tag=Dev"`) || cards(dev) != 15-size {
		t.Errorf("filtered page 2: want Page 2 of 2, a Newer link keeping tag=Dev, and %d cards (got %d)", 15-size, cards(dev))
	}

	// One page: no pagination, plain "Select all".
	one := page("/journals/content?q=Entry+7+marker")
	if strings.Contains(one, "journals-pagination") || !strings.Contains(one, ">Select all<") {
		t.Errorf("a single page must render no pagination and the plain Select all label")
	}

	// Actions return to the page (and filters) they were taken from…
	list, _ := store.List(ctx, true)
	from := "http://example.com/journals?tag=Dev&page=2"
	rec := do(http.MethodPost, fmt.Sprintf("/journals/%d/bookmark", list[0].ID), url.Values{}, from)
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/journals?page=2&tag=Dev" {
		t.Errorf("bookmark from a catalog page: got %d %q, want 303 back to that page", rec.Code, loc)
	}
	rec = do(http.MethodPost, "/journals/batch", url.Values{"action": {"publish_selected"}, "ids": {fmt.Sprint(list[0].ID)}}, from)
	if loc := rec.Header().Get("Location"); loc != "/journals?page=2&tag=Dev" {
		t.Errorf("batch from a catalog page: got %q", loc)
	}
	// …but never off-site or to another path.
	for _, ref := range []string{"https://evil.example/journals?page=2", "http://example.com/login?next=x", "http://example.com/journals/5?page=2", ""} {
		rec = do(http.MethodPost, fmt.Sprintf("/journals/%d/delete", list[1].ID), url.Values{"confirmed": {"1"}}, ref)
		if loc := rec.Header().Get("Location"); loc != "/journals" {
			t.Errorf("Referer %q: redirected to %q, want /journals", ref, loc)
		}
		list, _ = store.List(ctx, true)
	}
}
