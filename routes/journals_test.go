package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"golang.org/x/crypto/ssh"
)

func createAuthenticatedSessionCookie(t *testing.T, router http.Handler, signer ssh.Signer, sshAddr string) *http.Cookie {
	t.Helper()
	reqLogin := httptest.NewRequest(http.MethodGet, "/login?next=/journals", nil)
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)

	re := regexp.MustCompile(`token=([0-9]{6})`)
	matches := re.FindStringSubmatch(recLogin.Body.String())
	if len(matches) < 2 {
		t.Fatalf("failed to extract token from login page")
	}
	token := matches[1]

	var bindingCookie *http.Cookie
	for _, c := range recLogin.Result().Cookies() {
		if c.Name == auth.BindingCookieName {
			bindingCookie = c
			break
		}
	}

	clientConfig := &ssh.ClientConfig{
		User:            token,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", sshAddr, clientConfig)
	if err != nil {
		t.Fatalf("ssh.Dial failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	_ = sess.Run(token)
	_ = sess.Close()

	reqPoll := httptest.NewRequest(http.MethodGet, "/auth/status?token="+token+"&next=/journals", nil)
	reqPoll.AddCookie(bindingCookie)
	recPoll := httptest.NewRecorder()
	router.ServeHTTP(recPoll, reqPoll)

	for _, c := range recPoll.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatalf("failed to acquire session cookie")
	return nil
}

func TestJournalsEditorVacateAndLoadingFlow(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open memory failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	signer, pubKeyStr := generateTestKeys(t)
	sshAddr := getFreePort(t)
	authSvc, err := auth.NewService(pubKeyStr, "", sshAddr)
	if err != nil {
		t.Fatalf("auth.NewService failed: %v", err)
	}
	defer func() { _ = authSvc.Close() }()
	if err := authSvc.StartSSHListener(); err != nil {
		t.Fatalf("StartSSHListener failed: %v", err)
	}

	router := NewRouter(Config{
		AssetsDir:   "../assets",
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

	// 2. Authenticate admin session
	sessCookie := createAuthenticatedSessionCookie(t, router, signer, sshAddr)

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

	// 5. Verify public GET /journals shows only published entry
	reqCatPublic := httptest.NewRequest(http.MethodGet, "/journals", nil)
	recCatPublic := httptest.NewRecorder()
	router.ServeHTTP(recCatPublic, reqCatPublic)
	if recCatPublic.Code != http.StatusOK {
		t.Fatalf("expected 200 on /journals, got %d", recCatPublic.Code)
	}
	if !strings.Contains(recCatPublic.Body.String(), "Published journal entry") {
		t.Errorf("expected published entry on public catalog")
	}
	if strings.Contains(recCatPublic.Body.String(), "Unpublished draft entry") {
		t.Errorf("expected unpublished draft to be hidden on public catalog")
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
		"Clear Space Automatically",
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
