package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
)

func createTestTOTPAuth(t *testing.T) (*auth.Service, *http.Cookie) {
	t.Helper()
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret failed: %v", err)
	}
	authSvc, err := auth.NewServiceWithOptions(auth.ServiceOptions{
		TOTPSecret: secret,
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions failed: %v", err)
	}
	code, err := auth.GenerateTOTPCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateTOTPCode failed: %v", err)
	}
	sessID, err := authSvc.AuthenticateTOTP(code)
	if err != nil {
		t.Fatalf("AuthenticateTOTP failed: %v", err)
	}
	return authSvc, &http.Cookie{
		Name:  auth.SessionCookieName,
		Value: sessID,
		Path:  "/",
	}
}

func TestJournalsEditorVacateAndLoadingFlow(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open memory failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	authSvc, sessionCookie := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	router := NewRouter(Config{
		AuthService: authSvc,
		Store:       store,
	})

	// 1. Component CSS & /loading page
	for _, cssPath := range []string{
		"/views/journals/journals.css",
		"/views/editor/editor.css",
		"/views/loading/loading.css",
	} {
		req := httptest.NewRequest(http.MethodGet, cssPath, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on %s, got %d", cssPath, rec.Code)
		}
	}

	reqLoad := httptest.NewRequest(http.MethodGet, "/loading", nil)
	recLoad := httptest.NewRecorder()
	router.ServeHTTP(recLoad, reqLoad)
	if recLoad.Code != http.StatusOK || !strings.Contains(recLoad.Body.String(), "Confirming, please wait.") {
		t.Fatalf("expected /loading to render Confirming, please wait.")
	}

	// 2. Authenticated admin session ready from createTestTOTPAuth
	sessCookie := sessionCookie

	// 3. Create an unpublished entry with tag "Wip" and a published entry with tag "Log"
	formDraft := url.Values{
		"media_url": {"https://example.com/draft.png"},
		"tags":      {"Wip"},
		"body":      {"Unpublished draft entry with plenty of bytes to vacate for the incoming entry."},
	}
	reqCreateDraft := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(formDraft.Encode()))
	reqCreateDraft.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCreateDraft.AddCookie(sessCookie)
	recCreateDraft := httptest.NewRecorder()
	router.ServeHTTP(recCreateDraft, reqCreateDraft)
	if recCreateDraft.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on draft create, got %d", recCreateDraft.Code)
	}

	formPub := url.Values{
		"media_url":  {"https://example.com/pub.png"},
		"tags":       {"Log"},
		"body":       {"Published journal entry"},
		"published":  {"1"},
		"bookmarked": {"1"},
	}
	reqCreatePub := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(formPub.Encode()))
	reqCreatePub.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCreatePub.AddCookie(sessCookie)
	recCreatePub := httptest.NewRecorder()
	router.ServeHTTP(recCreatePub, reqCreatePub)
	if recCreatePub.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on pub create, got %d", recCreatePub.Code)
	}

	// 4. Verify public GET /journals/new does NOT leak unpublished tag "Wip"
	reqNewPublic := httptest.NewRequest(http.MethodGet, "/journals/new", nil)
	recNewPublic := httptest.NewRecorder()
	router.ServeHTTP(recNewPublic, reqNewPublic)
	if recNewPublic.Code != http.StatusOK {
		t.Fatalf("expected 200 on public /journals/new, got %d", recNewPublic.Code)
	}
	if strings.Contains(recNewPublic.Body.String(), `data-existing-tags="Log,Wip"`) || strings.Contains(recNewPublic.Body.String(), "Wip") {
		t.Errorf("SECURITY LEAK: public /journals/new leaked unpublished tag 'Wip'")
	}

	// 5. Verify public GET /journals renders the instant DB-independent shell,
	// and GET /journals/content populates only the published entry
	reqCatShell := httptest.NewRequest(http.MethodGet, "/journals", nil)
	recCatShell := httptest.NewRecorder()
	router.ServeHTTP(recCatShell, reqCatShell)
	if recCatShell.Code != http.StatusOK {
		t.Fatalf("expected 200 on /journals shell, got %d", recCatShell.Code)
	}
	if !strings.Contains(recCatShell.Body.String(), `hx-get="/journals/content"`) {
		t.Errorf("expected /journals shell to wire hx-get=\"/journals/content\"")
	}

	reqCatPublic := httptest.NewRequest(http.MethodGet, "/journals/content", nil)
	recCatPublic := httptest.NewRecorder()
	router.ServeHTTP(recCatPublic, reqCatPublic)
	if recCatPublic.Code != http.StatusOK {
		t.Fatalf("expected 200 on /journals/content, got %d", recCatPublic.Code)
	}
	if !strings.Contains(recCatPublic.Body.String(), "Published journal entry") {
		t.Errorf("expected published entry on public catalog content")
	}
	if strings.Contains(recCatPublic.Body.String(), "Unpublished draft entry") {
		t.Errorf("expected unpublished draft to be hidden on public catalog content")
	}

	// 6. Cap storage to current used bytes so next create redirects to /journals/vacate-space/
	used, err := store.UsedStorageBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	store.SetMaxStorageBytes(used)

	formOverflow := url.Values{
		"media_url": {"https://example.com/overflow.png"},
		"tags":      {"New"},
		"body":      {"Entry that triggers Make Room vacate-space flow"},
		"published": {"1"},
	}
	reqOverflow := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(formOverflow.Encode()))
	reqOverflow.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqOverflow.AddCookie(sessCookie)
	recOverflow := httptest.NewRecorder()
	router.ServeHTTP(recOverflow, reqOverflow)
	if recOverflow.Code != http.StatusSeeOther || recOverflow.Header().Get("Location") != "/journals/vacate-space/" {
		t.Fatalf("expected 303 redirect to /journals/vacate-space/, got %d (loc=%q)", recOverflow.Code, recOverflow.Header().Get("Location"))
	}

	// 7. Render /journals/vacate-space/
	reqVacatePage := httptest.NewRequest(http.MethodGet, "/journals/vacate-space/", nil)
	reqVacatePage.AddCookie(sessCookie)
	recVacatePage := httptest.NewRecorder()
	router.ServeHTTP(recVacatePage, reqVacatePage)
	if recVacatePage.Code != http.StatusOK {
		t.Fatalf("expected 200 on /journals/vacate-space/, got %d", recVacatePage.Code)
	}
	vacateBody := recVacatePage.Body.String()
	for _, exp := range []string{
		"Make Room",
		"Make Room Automatically",
		"Clear space automatically",
		"Choose Entries to Remove",
		"Confirming, please wait.",
	} {
		if !strings.Contains(vacateBody, exp) {
			t.Errorf("missing %q on vacate-space page", exp)
		}
	}

	// 8. Submit auto_vacate -> redirects to /journals/vacate-space/?vacated=1 with auto-resubmit ready
	reqAutoVacate := httptest.NewRequest(http.MethodPost, "/journals/vacate-space/", strings.NewReader("action=auto_vacate"))
	reqAutoVacate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqAutoVacate.AddCookie(sessCookie)
	recAutoVacate := httptest.NewRecorder()
	router.ServeHTTP(recAutoVacate, reqAutoVacate)
	if recAutoVacate.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after auto_vacate, got %d", recAutoVacate.Code)
	}

	entriesAfter, _ := store.List(ctx, true)
	if len(entriesAfter) != 1 {
		t.Fatalf("expected 1 entry remaining after auto_vacate evicted draft, got %d", len(entriesAfter))
	}
	remainingID := entriesAfter[0].ID

	// 9. Single entry permalink & Edit page
	reqSingle := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/journals/%d", remainingID), nil)
	recSingle := httptest.NewRecorder()
	router.ServeHTTP(recSingle, reqSingle)
	if recSingle.Code != http.StatusOK || !strings.Contains(recSingle.Body.String(), "Published journal entry") {
		t.Errorf("expected 200 on single entry permalink")
	}

	reqEdit := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/journals/%d/edit", remainingID), nil)
	reqEdit.AddCookie(sessCookie)
	recEdit := httptest.NewRecorder()
	router.ServeHTTP(recEdit, reqEdit)
	if recEdit.Code != http.StatusOK || !strings.Contains(recEdit.Body.String(), "Edit entry") {
		t.Errorf("expected 200 on edit entry form")
	}
}
