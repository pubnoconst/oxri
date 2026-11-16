package app

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"golang.org/x/crypto/ssh"
)

func newTestServerWithAuth(t *testing.T) (http.Handler, *auth.Service) {
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

	return NewServer("../../assets", svc), svc
}

func newTestServer(t *testing.T) http.Handler {
	server, _ := newTestServerWithAuth(t)
	return server
}

func TestLandingPage(t *testing.T) {
	server := newTestServer(t)
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
	if !strings.Contains(body, "alpinejs") {
		t.Errorf("missing Alpine.js in layout")
	}
	if !strings.Contains(body, "htmx.org") {
		t.Errorf("missing HTMX in layout")
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
	server := newTestServer(t)
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
}

func TestJournalEditorPage(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/journal-editor", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "journal-editor-page") {
		t.Errorf("missing journal-editor-page container")
	}
	if !strings.Contains(body, "/page-css/journal-editor.css") {
		t.Errorf("missing journal-editor stylesheet link")
	}
}

func TestLoginPageAndAuthPolling(t *testing.T) {
	server, svc := newTestServerWithAuth(t)

	// 1. Visit /login
	req := httptest.NewRequest(http.MethodGet, "/login?next=/journals", nil)
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

	// 4. Poll /auth/status after approval
	pollReq2 := httptest.NewRequest(http.MethodGet, "/auth/status?token="+token, nil)
	pollRec2 := httptest.NewRecorder()
	server.ServeHTTP(pollRec2, pollReq2)

	if pollRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on approved poll, got %d", pollRec2.Code)
	}
	if pollRec2.Header().Get("HX-Redirect") != "/journals" {
		t.Errorf("expected HX-Redirect: /journals, got %q", pollRec2.Header().Get("HX-Redirect"))
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

	// 5. Authenticated request to /journals
	authReq := httptest.NewRequest(http.MethodGet, "/journals", nil)
	authReq.AddCookie(sessionCookie)
	authRec := httptest.NewRecorder()
	server.ServeHTTP(authRec, authReq)

	if authRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated /journals, got %d", authRec.Code)
	}
	if !strings.Contains(authRec.Body.String(), "Sign out") {
		t.Errorf("authenticated journals view should display 'Sign out' link")
	}

	// 6. Visiting /login while authenticated should redirect to /journals
	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginReq.AddCookie(sessionCookie)
	loginRec := httptest.NewRecorder()
	server.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusSeeOther || loginRec.Header().Get("Location") != "/journals" {
		t.Errorf("expected redirect to /journals, got status=%d loc=%s", loginRec.Code, loginRec.Header().Get("Location"))
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

func TestPageStylesheets(t *testing.T) {
	server := newTestServer(t)
	cssRoutes := []string{
		"/page-css/landing.css",
		"/page-css/book-i.css",
		"/page-css/journals.css",
		"/page-css/journal-editor.css",
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
