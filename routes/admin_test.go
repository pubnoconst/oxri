package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

// Every administrative route, hit without a session, must be refused by the
// admin adapter (GET -> sign in and come back; POST -> 403) and change nothing.
// A route registered without adminOnly/freshAdminOnly fails here.
func TestAdminRoutesRefuseAnonymous(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	authSvc, session := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	cap, ok := authSvc.Authenticate(session.Value)
	if !ok {
		t.Fatalf("expected test session to yield an AdminCap")
	}
	entry, err := db.ParseValidatedEntry(db.JournalInput{
		MediaURL: "https://x.com/user/status/1", Tags: []string{"Log"}, Body: "Seed.", Published: true,
	})
	if err != nil {
		t.Fatalf("ParseValidatedEntry: %v", err)
	}
	seed, err := store.CreateValidated(ctx, cap, entry)
	if err != nil {
		t.Fatalf("CreateValidated: %v", err)
	}
	id := strconv.FormatInt(seed.ID, 10)

	router := NewRouter(Config{Store: store, AuthService: authSvc})

	gets := []string{
		"/journals/" + id + "/edit",
		"/journals/vacate-space/",
		"/book-ii/edit",
	}
	for _, path := range gets {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login?next=") {
			t.Errorf("anonymous GET %s: expected 303 to /login?next=…, got %d %q", path, rec.Code, rec.Header().Get("Location"))
		}
	}

	posts := []string{
		"/journals",
		"/journals/batch",
		"/journals/vacate-space/",
		"/journals/" + id,
		"/journals/" + id + "/delete",
		"/journals/" + id + "/publish",
		"/journals/" + id + "/unpublish",
		"/journals/" + id + "/bookmark",
		"/book-ii",
	}
	for _, path := range posts {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("action=delete_all&body=x&media_url=https://x.com/a/status/2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", "same-origin") // pass CSRF: we are testing the auth gate
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("anonymous POST %s: expected 403, got %d", path, rec.Code)
		}
	}

	// Nothing changed.
	all, err := store.List(ctx, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 || !all[0].Published || all[0].Bookmarked || all[0].Body != "Seed." {
		t.Errorf("anonymous requests changed journals: %+v", all)
	}
}
