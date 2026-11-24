package routes

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

// A book save that would push storage over the limit is held and sent to
// Make Room; vacating journal entries lets it save, and the book is never
// evicted.
func TestBookSaveOverQuotaGoesThroughMakeRoom(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	authSvc, sess := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	router := NewRouter(Config{AuthService: authSvc, Store: store})

	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(sess)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(sess)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// An unpublished draft is the journal space that can be vacated.
	if rec := post("/journals", url.Values{"media_url": {"https://example.com/d.png"}, "tags": {"Wip"}, "body": {strings.Repeat("d", 1500)}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("create draft: want 303, got %d", rec.Code)
	}
	used, _ := store.UsedStorageBytes(ctx)
	store.SetMaxStorageBytes(used + 100)

	book := "# Book II\n\n" + strings.Repeat("Λόγος ", 100) // ~1.1 KB: over the 100 B headroom
	rec := post("/book-ii", url.Values{"body": {book}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/journals/vacate-space/" {
		t.Fatalf("over-quota book save: want 303 to Make Room, got %d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
	if b, _ := store.GetBook(ctx, "book-ii"); strings.Contains(b.Body, "Λόγος") {
		t.Fatalf("an over-quota book save must not be written")
	}

	page := get("/journals/vacate-space/")
	if page.Code != http.StatusOK {
		t.Fatalf("Make Room: want 200, got %d", page.Code)
	}
	body := html.UnescapeString(page.Body.String())
	for _, want := range []string{
		`action="/book-ii"`,
		"Save Book II now",
		"Storage is full. Remove a few older journal entries",
		"your changes to Book II will be saved automatically",
		strings.TrimSpace(book[:40]),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Make Room for a held book: missing %q", want)
		}
	}
	if strings.Contains(body, `name="media_url"`) {
		t.Errorf("a held book must not render journal fields")
	}

	// Make room automatically (evicts the draft), then the page resubmits.
	if rec := post("/journals/vacate-space/", url.Values{"action": {"auto_vacate"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("auto_vacate: want 303, got %d", rec.Code)
	}
	if list, _ := store.List(ctx, true); len(list) != 0 {
		t.Fatalf("auto_vacate should have evicted the draft, %d left", len(list))
	}
	if after := get("/journals/vacate-space/?vacated=1"); !strings.Contains(after.Body.String(), `data-auto-resubmit="true"`) {
		t.Fatalf("after making room the held book should auto-resubmit")
	}
	if rec := post("/book-ii", url.Values{"body": {book}}); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/book-ii" {
		t.Fatalf("resubmitted book: want 303 to /book-ii, got %d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
	if b, _ := store.GetBook(ctx, "book-ii"); !strings.Contains(b.Body, "Λόγος") {
		t.Fatalf("the book should be saved after making room")
	}
	if store.GetPendingSave() != nil {
		t.Fatalf("the hold should clear once the book is saved")
	}
}
