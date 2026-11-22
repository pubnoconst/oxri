package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

func TestBooksReadEditAndSecurity(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	authSvc, sessionCookie := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	router := NewRouter(Config{
		AssetsDir:   "../assets",
		AuthService: authSvc,
		Store:       store,
	})

	// 1. Public read works for Book I, II, III without Edit link
	for _, slug := range []string{"book-i", "book-ii", "book-iii"} {
		req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on /%s, got %d", slug, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "/"+slug+"/edit") {
			t.Errorf("unauthenticated /%s must not show Edit link", slug)
		}
	}

	// 2. Unauthenticated GET /book-ii/edit redirects to /login; POST /book-ii returns 403
	reqEditAnon := httptest.NewRequest(http.MethodGet, "/book-ii/edit", nil)
	recEditAnon := httptest.NewRecorder()
	router.ServeHTTP(recEditAnon, reqEditAnon)
	if recEditAnon.Code != http.StatusSeeOther || !strings.Contains(recEditAnon.Header().Get("Location"), "/login?next=/book-ii/edit") {
		t.Fatalf("expected 303 redirect to /login?next=/book-ii/edit, got %d loc=%q", recEditAnon.Code, recEditAnon.Header().Get("Location"))
	}

	reqPostAnon := httptest.NewRequest(http.MethodPost, "/book-ii", strings.NewReader("body=evil"))
	reqPostAnon.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPostAnon := httptest.NewRecorder()
	router.ServeHTTP(recPostAnon, reqPostAnon)
	if recPostAnon.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on unauthenticated POST /book-ii, got %d", recPostAnon.Code)
	}

	// 3. Authenticated POST with emoji is rejected (422) and emoji is not echoed back
	emojiForm := url.Values{"body": {"# Book II 🔥"}}
	reqEmoji := httptest.NewRequest(http.MethodPost, "/book-ii", strings.NewReader(emojiForm.Encode()))
	reqEmoji.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqEmoji.AddCookie(sessionCookie)
	recEmoji := httptest.NewRecorder()
	router.ServeHTTP(recEmoji, reqEmoji)
	if recEmoji.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for emoji body, got %d", recEmoji.Code)
	}
	if strings.Contains(recEmoji.Body.String(), "🔥") {
		t.Errorf("disallowed emoji was echoed back in 422 response")
	}

	// 4. Authenticated POST with XSS attempt is persisted safely and stripped on render
	xssForm := url.Values{"body": {"# Book III\n\n<script>alert(1)</script>\n\n[Click](javascript:alert(1))\n\nSafe prose."}}
	reqSave := httptest.NewRequest(http.MethodPost, "/book-iii", strings.NewReader(xssForm.Encode()))
	reqSave.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqSave.AddCookie(sessionCookie)
	recSave := httptest.NewRecorder()
	router.ServeHTTP(recSave, reqSave)
	if recSave.Code != http.StatusSeeOther || recSave.Header().Get("Location") != "/book-iii" {
		t.Fatalf("expected 303 redirect to /book-iii, got %d loc=%q", recSave.Code, recSave.Header().Get("Location"))
	}

	reqRead := httptest.NewRequest(http.MethodGet, "/book-iii", nil)
	reqRead.AddCookie(sessionCookie)
	recRead := httptest.NewRecorder()
	router.ServeHTTP(recRead, reqRead)
	if recRead.Code != http.StatusOK {
		t.Fatalf("expected 200 on /book-iii, got %d", recRead.Code)
	}
	html := recRead.Body.String()
	if !strings.Contains(html, "Safe prose.") || !strings.Contains(html, `href="/book-iii/edit"`) {
		t.Errorf("expected saved prose and Edit link in authenticated /book-iii")
	}
	if strings.Contains(strings.ToLower(html), "alert(1)") || strings.Contains(strings.ToLower(html), "javascript:") {
		t.Errorf("XSS payload leaked into rendered /book-iii HTML")
	}
}
