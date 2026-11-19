package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"golang.org/x/crypto/ssh"
)

func newTestServerWithAll(t *testing.T) (http.Handler, *auth.Service, *db.Store) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("NewPublicKey: %v", err)
	}
	authKeyStr := string(ssh.MarshalAuthorizedKey(sshPub))

	tmpDir := t.TempDir()
	hostKeyPath := filepath.Join(tmpDir, "host_key")

	svc, err := auth.NewService(authKeyStr, hostKeyPath, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	store, err := db.Open(context.Background(), ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	return NewServer("../../assets", svc, store), svc, store
}

func newTestServerWithAuth(t *testing.T) (http.Handler, *auth.Service) {
	server, svc, _ := newTestServerWithAll(t)
	return server, svc
}

func newTestServer(t *testing.T) http.Handler {
	server, _ := newTestServerWithAuth(t)
	return server
}

func TestLandingPage(t *testing.T) {
	server, svc := newTestServerWithAuth(t)

	// 1. Unauthenticated landing page
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "hx-boost") {
		t.Errorf("landing page should not contain hx-boost")
	}
	if !strings.Contains(body, "RES·IMPERII") {
		t.Errorf("missing wordmark in landing body")
	}
	if !strings.Contains(body, "Book I") || !strings.Contains(body, "Journals") {
		t.Errorf("missing book or journal links")
	}
	if !strings.Contains(body, "Log in") {
		t.Errorf("unauthenticated landing page should contain 'Log in' link")
	}
	if !strings.Contains(body, "alpinejs") {
		t.Errorf("missing Alpine.js in layout")
	}
	if !strings.Contains(body, "htmx.org") {
		t.Errorf("missing HTMX in layout")
	}

	// 2. Authenticated landing page shows "Log out"
	token, _ := svc.CreatePendingLogin("/")
	svc.ApprovePendingLogin(token, "test")
	sessID, _, _ := svc.ConsumeApprovedLogin(token)

	authReq := httptest.NewRequest(http.MethodGet, "/", nil)
	authReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessID})
	authRec := httptest.NewRecorder()
	server.ServeHTTP(authRec, authReq)

	authBody := authRec.Body.String()
	if !strings.Contains(authBody, "Log out") {
		t.Errorf("authenticated landing page should contain 'Log out' link")
	}
	if !strings.Contains(authBody, "/pilot/user/settings") {
		t.Errorf("authenticated landing page should contain Settings link")
	}
}

func TestBookIPage(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/book-i", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "book-article") {
		t.Errorf("missing book-article container")
	}
	if !strings.Contains(body, "/page-css/book-i.css") {
		t.Errorf("missing book-i stylesheet link")
	}
}

func TestJournalsPage(t *testing.T) {
	server, svc := newTestServerWithAuth(t)
	req := httptest.NewRequest(http.MethodGet, "/journals", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "journals-page") {
		t.Errorf("missing journals-page container")
	}
	if !strings.Contains(body, "/page-css/journals.css") {
		t.Errorf("missing journals stylesheet link")
	}
	if !strings.Contains(body, "Author sign in") {
		t.Errorf("unauthenticated journals view should offer sign in link")
	}

	// 2. Authenticated journals view shows Settings and Sign out
	token, _ := svc.CreatePendingLogin("/journals")
	svc.ApprovePendingLogin(token, "test")
	sessID, _, _ := svc.ConsumeApprovedLogin(token)

	authReq := httptest.NewRequest(http.MethodGet, "/journals", nil)
	authReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessID})
	authRec := httptest.NewRecorder()
	server.ServeHTTP(authRec, authReq)

	authBody := authRec.Body.String()
	if !strings.Contains(authBody, "Sign out") {
		t.Errorf("authenticated journals view should offer Sign out link")
	}
	if strings.Contains(authBody, "/pilot/user/settings") {
		t.Errorf("authenticated journals view should NOT expose user Settings link")
	}
}

func TestJournalEditorPage(t *testing.T) {
	server := newTestServer(t)

	// 1. Primary route: /journals/new
	req := httptest.NewRequest(http.MethodGet, "/journals/new", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "journal-editor-page") {
		t.Errorf("missing journal-editor-page container")
	}
	if !strings.Contains(body, "/page-css/journals-new.css") {
		t.Errorf("missing journals-new stylesheet link")
	}
	if !strings.Contains(body, "widgets.js") {
		t.Errorf("missing Twitter widgets.js script tag in editor")
	}
	if !strings.Contains(body, "journal-media.js?v=1") {
		t.Errorf("missing journal-media.js script tag in editor")
	}
	if !strings.Contains(body, "journal-editor.js?v=8") {
		t.Errorf("missing versioned journal-editor.js script tag")
	}
	if !strings.Contains(body, `maxlength="128"`) {
		t.Errorf("missing maxlength=128 constraint on media_url input")
	}
	if !strings.Contains(body, "Twitter post link") {
		t.Errorf("missing Twitter media hint in editor")
	}
	if !strings.Contains(body, "Preview") {
		t.Errorf("missing minimal Preview heading")
	}
	if strings.Contains(body, "Live preview") {
		t.Errorf("editor must not contain 'Live preview'")
	}
	if !strings.Contains(body, "btn btn-encouraged") {
		t.Errorf("missing standardized encouraged publish button")
	}

	// 2. Legacy route redirect: /journal-editor -> 301 to /journals/new
	oldReq := httptest.NewRequest(http.MethodGet, "/journal-editor", nil)
	oldRec := httptest.NewRecorder()
	server.ServeHTTP(oldRec, oldReq)

	if oldRec.Code != http.StatusMovedPermanently || oldRec.Header().Get("Location") != "/journals/new" {
		t.Errorf("expected 301 redirect to /journals/new, got code=%d loc=%s", oldRec.Code, oldRec.Header().Get("Location"))
	}
}

func TestLoginPageAndAuthPolling(t *testing.T) {
	server, svc := newTestServerWithAuth(t)

	// 1. Visit /login without query param (should default to landing page "/")
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /login, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "AUTHENTICATION") {
		t.Errorf("missing AUTHENTICATION title on login page")
	}
	if !strings.Contains(body, "ssh -p") {
		t.Errorf("missing ssh command in login page body")
	}
	if !strings.Contains(body, "auth-poller") {
		t.Errorf("missing auth poller element in login page")
	}
	if !strings.Contains(body, "Copy Command") {
		t.Errorf("missing copy button in login page")
	}
	if strings.Contains(body, "outlook.com") || strings.Contains(body, "git.dev.noconst") {
		t.Errorf("login page must not leak author email address")
	}

	// Extract the 6-digit challenge token from the page
	re := regexp.MustCompile(`token=([0-9]{6})`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 {
		t.Fatalf("could not extract challenge token from login body: %s", body)
	}
	token := matches[1]

	// 2. Poll /auth/status before approval
	pollReq := httptest.NewRequest(http.MethodGet, "/auth/status?token="+token, nil)
	pollRec := httptest.NewRecorder()
	server.ServeHTTP(pollRec, pollReq)

	if pollRec.Code != http.StatusOK || pollRec.Body.String() != "waiting" {
		t.Fatalf("expected waiting status, got code=%d body=%s", pollRec.Code, pollRec.Body.String())
	}

	// 3. Approve the challenge (simulating SSH client)
	if !svc.ApprovePendingLogin(token, "test-user") {
		t.Fatalf("failed to approve pending challenge")
	}

	// 4. Poll /auth/status after approval -> must land back on "/"
	pollReq2 := httptest.NewRequest(http.MethodGet, "/auth/status?token="+token, nil)
	pollRec2 := httptest.NewRecorder()
	server.ServeHTTP(pollRec2, pollReq2)

	if pollRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on approved poll, got %d", pollRec2.Code)
	}
	if pollRec2.Header().Get("HX-Redirect") != "/" {
		t.Errorf("expected HX-Redirect: /, got %q", pollRec2.Header().Get("HX-Redirect"))
	}

	// Ensure cookie was issued
	cookies := pollRec2.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("missing or empty session cookie after auth approval")
	}

	// 5. Authenticated request to /
	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeReq.AddCookie(sessionCookie)
	homeRec := httptest.NewRecorder()
	server.ServeHTTP(homeRec, homeReq)

	if homeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated /, got %d", homeRec.Code)
	}
	if !strings.Contains(homeRec.Body.String(), "Log out") {
		t.Errorf("authenticated landing view should display 'Log out' link")
	}

	// 6. Visiting /login while authenticated should redirect to /
	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginReq.AddCookie(sessionCookie)
	loginRec := httptest.NewRecorder()
	server.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusSeeOther || loginRec.Header().Get("Location") != "/" {
		t.Errorf("expected redirect to /, got status=%d loc=%s", loginRec.Code, loginRec.Header().Get("Location"))
	}

	// 7. Logout revokes the session and clears cookie
	logoutReq := httptest.NewRequest(http.MethodGet, "/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	server.ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusSeeOther || logoutRec.Header().Get("Location") != "/" {
		t.Errorf("expected redirect to /, got status=%d loc=%s", logoutRec.Code, logoutRec.Header().Get("Location"))
	}
	if svc.ValidateSession(sessionCookie.Value) {
		t.Errorf("session %s should be revoked after logout", sessionCookie.Value)
	}
}

func TestBookIINotFound(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/book-ii", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "404") {
		t.Errorf("missing 404 in error body")
	}
	if !strings.Contains(body, "This page could not be found.") {
		t.Errorf("missing 404 message in error body")
	}
	if !strings.Contains(body, "data:image/png;base64,") {
		t.Errorf("missing QR code data URI in error body")
	}
	if !strings.Contains(body, "/page-css/error.css") {
		t.Errorf("missing error page stylesheet link")
	}
}

func TestPanicRecovery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := RecoverMiddleware(mux)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "500") {
		t.Errorf("expected 500 in body, got: %s", body)
	}
	if strings.Contains(body, "test panic") {
		t.Errorf("error page should never expose panic details")
	}
}

func TestTimeoutHandling(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("ok"))
	})
	handler := TimeoutMiddleware(10 * time.Millisecond)(mux)

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Wait briefly for timeout to complete
	time.Sleep(20 * time.Millisecond)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "504") {
		t.Errorf("expected 504 in body")
	}
}

func TestPilotUserSettingsPage(t *testing.T) {
	server, svc := newTestServerWithAuth(t)

	// 1. Unauthenticated request to /pilot/user/settings redirects to /login?next=...
	req := httptest.NewRequest(http.MethodGet, "/pilot/user/settings", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 See Other redirect for unauthenticated settings, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/login") || !strings.Contains(loc, "pilot") {
		t.Errorf("expected redirect to login with next param, got %s", loc)
	}

	// 2. Unauthenticated request to /pilot redirects to login
	pilotReq := httptest.NewRequest(http.MethodGet, "/pilot", nil)
	pilotRec := httptest.NewRecorder()
	server.ServeHTTP(pilotRec, pilotReq)
	if pilotRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for unauthenticated /pilot, got %d", pilotRec.Code)
	}

	// 3. Authenticated request to /pilot/user/settings succeeds
	token, _ := svc.CreatePendingLogin("/pilot/user/settings")
	svc.ApprovePendingLogin(token, "test")
	sessID, _, _ := svc.ConsumeApprovedLogin(token)

	authReq := httptest.NewRequest(http.MethodGet, "/pilot/user/settings", nil)
	authReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessID})
	authRec := httptest.NewRecorder()
	server.ServeHTTP(authRec, authReq)

	if authRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for authenticated settings, got %d", authRec.Code)
	}
	body := authRec.Body.String()
	if !strings.Contains(body, "Security & Authentication") {
		t.Errorf("missing Security & Authentication heading in body")
	}
	if !strings.Contains(body, "PILOT / EXPERIMENTAL") {
		t.Errorf("missing PILOT / EXPERIMENTAL eyebrow in body")
	}
	if !strings.Contains(body, "Loopback SSH Key") {
		t.Errorf("missing Loopback SSH Key method card")
	}
	if !strings.Contains(body, "Passkeys (FIDO2 / WebAuthn)") {
		t.Errorf("missing Passkeys method card")
	}
	if !strings.Contains(body, "Add Passkey") {
		t.Errorf("missing Add Passkey button")
	}
	if !strings.Contains(body, "disabled") {
		t.Errorf("expected disabled attribute on auth mutation controls")
	}
	if !strings.Contains(body, "HTTPS") {
		t.Errorf("missing explanation notice regarding HTTPS and domain origin")
	}

	// 4. Authenticated request to /pilot redirects to /pilot/user/settings
	authPilotReq := httptest.NewRequest(http.MethodGet, "/pilot", nil)
	authPilotReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessID})
	authPilotRec := httptest.NewRecorder()
	server.ServeHTTP(authPilotRec, authPilotReq)

	if authPilotRec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected 307 Temporary Redirect for authenticated /pilot, got %d", authPilotRec.Code)
	}
	if authPilotRec.Header().Get("Location") != "/pilot/user/settings" {
		t.Errorf("expected redirect location /pilot/user/settings, got %s", authPilotRec.Header().Get("Location"))
	}
}

func TestPageStylesheets(t *testing.T) {
	server := newTestServer(t)
	cssRoutes := []string{
		"/page-css/landing.css",
		"/page-css/book-i.css",
		"/page-css/journals.css",
		"/page-css/journals-new.css",
		"/page-css/journal-editor.css",
		"/page-css/pilot-settings.css",
		"/page-css/login.css",
		"/page-css/error.css",
	}

	for _, route := range cssRoutes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("route %s returned status %d", route, rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/css") {
			t.Errorf("route %s has content-type %s, expected text/css", route, ct)
		}
		bytes, _ := io.ReadAll(rec.Body)
		if len(bytes) == 0 {
			t.Errorf("route %s returned empty stylesheet", route)
		}
	}
}

func TestJournalPersistenceAndManagement(t *testing.T) {
	server, svc, store := newTestServerWithAll(t)

	// 1. Unauthenticated write endpoints redirect to /login
	unauthPost := httptest.NewRequest(http.MethodPost, "/journals", nil)
	unauthRec := httptest.NewRecorder()
	server.ServeHTTP(unauthRec, unauthPost)
	if unauthRec.Code != http.StatusSeeOther || !strings.Contains(unauthRec.Header().Get("Location"), "/login") {
		t.Fatalf("expected 303 redirect to login for unauthenticated POST /journals, got %d", unauthRec.Code)
	}

	// Authenticate admin session
	token, _ := svc.CreatePendingLogin("/journals")
	svc.ApprovePendingLogin(token, "test")
	sessID, _, _ := svc.ConsumeApprovedLogin(token)
	cookie := &http.Cookie{Name: auth.SessionCookieName, Value: sessID}

	// 2. Server rejects constraint violation (>4 char tag) with 422 and preserves input
	badForm := url.Values{
		"media_url": {"https://x.com/user/status/12345"},
		"tags":      {"toolong"},
		"body":      {"# Test body"},
		"published": {"1"},
	}
	badReq := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(badForm.Encode()))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badReq.AddCookie(cookie)
	badRec := httptest.NewRecorder()
	server.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity for oversized tag, got %d", badRec.Code)
	}
	if !strings.Contains(badRec.Body.String(), "exceeds 4 characters") {
		t.Errorf("expected tag validation error message in body")
	}

	// 2b. Server rejects duplicate tags (case-insensitive) with 422
	dupForm := url.Values{
		"media_url": {"https://x.com/user/status/12345"},
		"tags":      {"log, LOG"},
		"body":      {"# Test body"},
		"published": {"1"},
	}
	dupReq := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(dupForm.Encode()))
	dupReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	dupReq.AddCookie(cookie)
	dupRec := httptest.NewRecorder()
	server.ServeHTTP(dupRec, dupReq)
	if dupRec.Code != http.StatusUnprocessableEntity || !strings.Contains(dupRec.Body.String(), "duplicate tag") {
		t.Fatalf("expected 422 duplicate tag error, got code=%d", dupRec.Code)
	}

	// 2c. Server rejects emoji in body or tags with 422 ("ALL INPUTS ARE EVIL")
	emojiForm := url.Values{
		"media_url": {"https://x.com/user/status/12345"},
		"tags":      {"Log"},
		"body":      {"# Entry with emoji 🚀"},
		"published": {"1"},
	}
	emojiReq := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(emojiForm.Encode()))
	emojiReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emojiReq.AddCookie(cookie)
	emojiRec := httptest.NewRecorder()
	server.ServeHTTP(emojiRec, emojiReq)
	if emojiRec.Code != http.StatusUnprocessableEntity || !strings.Contains(emojiRec.Body.String(), "no emojis") {
		t.Fatalf("expected 422 emoji rejection error, got code=%d", emojiRec.Code)
	}

	// 3. Valid POST creates entry, normalizes tags to Title Case, and redirects to /journals
	goodForm := url.Values{
		"media_url":  {"https://x.com/user/status/12345"},
		"tags":       {"go, dev"},
		"body":       {"# Valid Journal Entry\n\nPersisted in DB."},
		"published":  {"1"},
		"bookmarked": {"1"},
	}
	goodReq := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(goodForm.Encode()))
	goodReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	goodReq.AddCookie(cookie)
	goodRec := httptest.NewRecorder()
	server.ServeHTTP(goodRec, goodReq)
	if goodRec.Code != http.StatusSeeOther || goodRec.Header().Get("Location") != "/journals" {
		t.Fatalf("expected 303 redirect to /journals, got %d loc=%s", goodRec.Code, goodRec.Header().Get("Location"))
	}

	entries, _ := store.List(context.Background(), true)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in DB, got %d", len(entries))
	}
	if entries[0].Tag1 != "Go" || entries[0].Tag2 != "Dev" {
		t.Errorf("expected Title Case tags [Go, Dev], got [%s, %s]", entries[0].Tag1, entries[0].Tag2)
	}
	entryID := entries[0].ID

	// 3b. Editor populates data-existing-tags for autocomplete
	newEditorReq := httptest.NewRequest(http.MethodGet, "/journals/new", nil)
	newEditorRec := httptest.NewRecorder()
	server.ServeHTTP(newEditorRec, newEditorReq)
	if !strings.Contains(newEditorRec.Body.String(), `data-existing-tags="Dev,Go"`) {
		t.Errorf("expected data-existing-tags=\"Dev,Go\" in /journals/new for autocomplete")
	}

	// 4. Public /journals displays collage preview with link to full entry, search, and tag filter
	pubReq := httptest.NewRequest(http.MethodGet, "/journals", nil)
	pubRec := httptest.NewRecorder()
	server.ServeHTTP(pubRec, pubReq)
	pubBody := pubRec.Body.String()
	if pubRec.Code != http.StatusOK || !strings.Contains(pubBody, "Valid Journal Entry") {
		t.Errorf("expected published entry on public /journals, code=%d", pubRec.Code)
	}
	if !strings.Contains(pubBody, "is-collage-grid") || !strings.Contains(pubBody, fmt.Sprintf("/journals/%d", entryID)) {
		t.Errorf("expected collage grid and clickable link to /journals/%d", entryID)
	}

	searchHitReq := httptest.NewRequest(http.MethodGet, "/journals?q=Persisted", nil)
	searchHitRec := httptest.NewRecorder()
	server.ServeHTTP(searchHitRec, searchHitReq)
	if !strings.Contains(searchHitRec.Body.String(), "Valid Journal Entry") {
		t.Errorf("expected search query ?q=Persisted to find entry")
	}

	searchMissReq := httptest.NewRequest(http.MethodGet, "/journals?q=NonexistentKeyword", nil)
	searchMissRec := httptest.NewRecorder()
	server.ServeHTTP(searchMissRec, searchMissReq)
	if strings.Contains(searchMissRec.Body.String(), "Valid Journal Entry") || !strings.Contains(searchMissRec.Body.String(), "No matching entries found.") {
		t.Errorf("expected search miss to show 'No matching entries found.'")
	}

	tagFilterReq := httptest.NewRequest(http.MethodGet, "/journals?tag=Go", nil)
	tagFilterRec := httptest.NewRecorder()
	server.ServeHTTP(tagFilterRec, tagFilterReq)
	if !strings.Contains(tagFilterRec.Body.String(), "Valid Journal Entry") {
		t.Errorf("expected tag filter ?tag=Go to find entry")
	}

	// 5. Unpublish single entry hides it from public /journals and /journals/{id}
	unpubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/journals/%d/unpublish", entryID), nil)
	unpubReq.AddCookie(cookie)
	unpubRec := httptest.NewRecorder()
	server.ServeHTTP(unpubRec, unpubReq)
	if unpubRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on unpublish, got %d", unpubRec.Code)
	}

	pubReq2 := httptest.NewRequest(http.MethodGet, "/journals", nil)
	pubRec2 := httptest.NewRecorder()
	server.ServeHTTP(pubRec2, pubReq2)
	if strings.Contains(pubRec2.Body.String(), "Valid Journal Entry") {
		t.Errorf("unpublished entry should be hidden from public /journals")
	}

	singlePubReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/journals/%d", entryID), nil)
	singlePubRec := httptest.NewRecorder()
	server.ServeHTTP(singlePubRec, singlePubReq)
	if singlePubRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unpublished entry accessed by public, got %d", singlePubRec.Code)
	}

	// 6. Authenticated catalog shows unpublished entry and Edit page loads it
	authCatReq := httptest.NewRequest(http.MethodGet, "/journals", nil)
	authCatReq.AddCookie(cookie)
	authCatRec := httptest.NewRecorder()
	server.ServeHTTP(authCatRec, authCatReq)
	if !strings.Contains(authCatRec.Body.String(), "Unpublished") || !strings.Contains(authCatRec.Body.String(), "Select all") {
		t.Errorf("authenticated catalog missing Unpublished badge or batch bar")
	}

	editReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/journals/%d/edit", entryID), nil)
	editReq.AddCookie(cookie)
	editRec := httptest.NewRecorder()
	server.ServeHTTP(editRec, editReq)
	if editRec.Code != http.StatusOK || !strings.Contains(editRec.Body.String(), "Save changes") {
		t.Errorf("expected 200 and Save changes on edit page, code=%d", editRec.Code)
	}

	// 7. POST /journals/{id} updates entry and re-publishes it
	updateForm := url.Values{
		"media_url": {"https://example.com/photo.jpg"},
		"tags":      {"edit"},
		"body":      {"# Updated Content"},
		"published": {"1"},
	}
	updateReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/journals/%d", entryID), strings.NewReader(updateForm.Encode()))
	updateReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	updateReq.AddCookie(cookie)
	updateRec := httptest.NewRecorder()
	server.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on update, got %d", updateRec.Code)
	}

	// 8. Batch delete_all clears all entries
	batchForm := url.Values{"action": {"delete_all"}}
	batchReq := httptest.NewRequest(http.MethodPost, "/journals/batch", strings.NewReader(batchForm.Encode()))
	batchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	batchReq.AddCookie(cookie)
	batchRec := httptest.NewRecorder()
	server.ServeHTTP(batchRec, batchReq)
	if batchRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on batch delete_all, got %d", batchRec.Code)
	}

	afterAll, _ := store.List(context.Background(), true)
	if len(afterAll) != 0 {
		t.Errorf("expected 0 entries after batch delete_all, got %d", len(afterAll))
	}
}
