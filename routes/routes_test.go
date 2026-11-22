package routes

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"golang.org/x/crypto/ssh"
)

func generateTestKeys(t *testing.T) (ssh.Signer, string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	pubKeyStr := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
	return signer, pubKeyStr
}

func getFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

func TestLandingPageAndComponentCSS(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	// 1. Landing Page HTML
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, expected := range []string{
		"RES·IMPERII",
		"Book I",
		"Book II",
		"Book III",
		"Journals",
		"Log in",
		"/assets/css/site.css?v=7",
		"/views/landing/landing.css?v=1",
		"/assets/js/htmx.min.js",
		"/assets/js/theme-toggle.js?v=2",
		"/assets/js/navigation-feedback.js?v=1",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q in landing page HTML", expected)
		}
	}

	// 2. Component-owned CSS path (/views/landing/landing.css)
	cssReq := httptest.NewRequest(http.MethodGet, "/views/landing/landing.css", nil)
	cssRec := httptest.NewRecorder()
	router.ServeHTTP(cssRec, cssReq)

	if cssRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /views/landing/landing.css, got %d", cssRec.Code)
	}
	if ct := cssRec.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("expected text/css content type, got %q", ct)
	}
	cssBody, _ := io.ReadAll(cssRec.Body)
	if !strings.Contains(string(cssBody), ".landing-title") {
		t.Errorf("missing .landing-title in landing component CSS")
	}

	// 3. Global site.css (/assets/css/site.css)
	siteReq := httptest.NewRequest(http.MethodGet, "/assets/css/site.css", nil)
	siteRec := httptest.NewRecorder()
	router.ServeHTTP(siteRec, siteReq)

	if siteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /assets/css/site.css, got %d", siteRec.Code)
	}
	siteBody, _ := io.ReadAll(siteRec.Body)
	if !strings.Contains(string(siteBody), "--font-serif") || !strings.Contains(string(siteBody), ".btn-encouraged") {
		t.Errorf("missing design tokens or button classes in site.css")
	}

	// 4. Vendored HTMX (/assets/js/htmx.min.js)
	htmxReq := httptest.NewRequest(http.MethodGet, "/assets/js/htmx.min.js", nil)
	htmxRec := httptest.NewRecorder()
	router.ServeHTTP(htmxRec, htmxReq)

	if htmxRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /assets/js/htmx.min.js, got %d", htmxRec.Code)
	}
}

func TestErrorPages_NotFound(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	req := httptest.NewRequest(http.MethodGet, "/non-existent-page-xyz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, expected := range []string{
		"RES IMPERII",
		"404",
		"This page could not be found.",
		"Reference code",
		"data:image/png;base64,",
		"Return home",
		"/views/errorpage/errorpage.css",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q in 404 HTML body", expected)
		}
	}
}

func TestErrorPages_MethodNotAllowed(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("evil payload"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "405") {
		t.Errorf("expected 405 code in body, got: %s", body)
	}
	if !strings.Contains(body, "Return home") {
		t.Errorf("expected Return home in body")
	}
}

func TestErrorPages_PanicRecovery_NoLeak(t *testing.T) {
	testHandler := safeRecoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic: database connection failed or simulated exception")
	}))

	// Trigger simulated panic
	req := httptest.NewRequest(http.MethodGet, "/simulated-panic", nil)
	rec := httptest.NewRecorder()
	testHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Must contain client-safe presentation
	for _, expected := range []string{
		"RES IMPERII",
		"500",
		"Something went wrong. Please try again later.",
		"Reference code",
		"Return home",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing client-safe %q in 500 HTML body", expected)
		}
	}

	// Must NEVER leak internal traces or panic reasons
	for _, forbidden := range []string{
		"test panic",
		"database connection failed",
		"goroutine",
		"runtime/debug",
		"main.go",
		"routes.go",
		"stack",
	} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
			t.Errorf("CRITICAL SECURITY LEAK: 500 body contains internal string %q", forbidden)
		}
	}
}

func TestErrorPages_CSS(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	req := httptest.NewRequest(http.MethodGet, "/views/errorpage/errorpage.css", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /views/errorpage/errorpage.css, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("expected text/css, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ".error-page") || !strings.Contains(body, ".error-code") {
		t.Errorf("missing error page css classes")
	}
}

func TestBookIPage(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	req := httptest.NewRequest(http.MethodGet, "/book-i", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /book-i, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, expected := range []string{
		"Book I - RES IMPERII",
		"/views/booki/booki.css",
		"On Cause",
		"Causal Succession",
		"Substance, Cosmos and Substrate",
		"back-link",
		"back-icon",
		"Back",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing expected content %q in /book-i HTML", expected)
		}
	}
}

func TestBookICSS(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	req := httptest.NewRequest(http.MethodGet, "/views/booki/booki.css", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /views/booki/booki.css, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("expected text/css, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ".book-article") || !strings.Contains(body, ".book-content") {
		t.Errorf("missing book-i CSS classes in stylesheet")
	}
}

func TestAuthRoutes_FullFlow(t *testing.T) {
	signer, pubKeyStr := generateTestKeys(t)
	sshAddr := getFreePort(t)

	authSvc, err := auth.NewService(pubKeyStr, "", sshAddr)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() { _ = authSvc.Close() }()

	if err := authSvc.StartSSHListener(); err != nil {
		t.Fatalf("StartSSHListener failed: %v", err)
	}

	router := NewRouter(Config{AssetsDir: "../assets", AuthService: authSvc})

	// 1. Unauthenticated /pilot/user/settings redirects to /login
	reqSettings := httptest.NewRequest(http.MethodGet, "/pilot/user/settings", nil)
	recSettings := httptest.NewRecorder()
	router.ServeHTTP(recSettings, reqSettings)
	if recSettings.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on unauthenticated /pilot/user/settings, got %d", recSettings.Code)
	}
	if loc := recSettings.Header().Get("Location"); !strings.Contains(loc, "/login") {
		t.Errorf("expected redirect to /login, got %q", loc)
	}

	// 2. Request /login?next=/pilot/user/settings
	reqLogin := httptest.NewRequest(http.MethodGet, "/login?next=/pilot/user/settings", nil)
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)

	if recLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on /login, got %d", recLogin.Code)
	}
	loginBody := recLogin.Body.String()
	if !strings.Contains(loginBody, "AUTHENTICATION") || !strings.Contains(loginBody, "Copy Command") {
		t.Errorf("login markup missing expected headers or copy button")
	}

	// Extract token from login HTML
	re := regexp.MustCompile(`token=([0-9]{6})`)
	matches := re.FindStringSubmatch(loginBody)
	if len(matches) < 2 {
		t.Fatalf("failed to extract 6-digit token from login body: %s", loginBody)
	}
	token := matches[1]

	// Extract binding cookie
	var bindingCookie *http.Cookie
	for _, c := range recLogin.Result().Cookies() {
		if c.Name == auth.BindingCookieName {
			bindingCookie = c
			break
		}
	}
	if bindingCookie == nil || bindingCookie.Value == "" {
		t.Fatalf("expected %s cookie in /login response", auth.BindingCookieName)
	}

	// 3. Poll /auth/status before approval returns 204 No Content
	reqPoll := httptest.NewRequest(http.MethodGet, "/auth/status?token="+token+"&next=/pilot/user/settings", nil)
	reqPoll.AddCookie(bindingCookie)
	recPoll := httptest.NewRecorder()
	router.ServeHTTP(recPoll, reqPoll)
	if recPoll.Code != http.StatusNoContent {
		t.Fatalf("expected 204 before approval, got %d", recPoll.Code)
	}

	// 4. Client signs the challenge via SSH
	clientConfig := &ssh.ClientConfig{
		User: token,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", sshAddr, clientConfig)
	if err != nil {
		t.Fatalf("ssh.Dial failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("client.NewSession failed: %v", err)
	}
	_ = session.Run(token)
	_ = session.Close()

	// 5. Poll /auth/status after approval returns HX-Redirect
	reqPollApproved := httptest.NewRequest(http.MethodGet, "/auth/status?token="+token+"&next=/pilot/user/settings", nil)
	reqPollApproved.Header.Set("HX-Request", "true")
	reqPollApproved.AddCookie(bindingCookie)
	recPollApproved := httptest.NewRecorder()
	router.ServeHTTP(recPollApproved, reqPollApproved)

	if recPollApproved.Code != http.StatusOK {
		t.Fatalf("expected 200 on HX-Request approval, got %d", recPollApproved.Code)
	}
	if hxRedir := recPollApproved.Header().Get("HX-Redirect"); hxRedir != "/pilot/user/settings" {
		t.Errorf("expected HX-Redirect /pilot/user/settings, got %q", hxRedir)
	}

	// Extract authenticated session cookie
	var sessionCookie *http.Cookie
	for _, c := range recPollApproved.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected %s cookie in approval response", auth.SessionCookieName)
	}

	// 6. Access /pilot/user/settings with session cookie returns 200 OK
	reqSettingsAuth := httptest.NewRequest(http.MethodGet, "/pilot/user/settings", nil)
	reqSettingsAuth.AddCookie(sessionCookie)
	recSettingsAuth := httptest.NewRecorder()
	router.ServeHTTP(recSettingsAuth, reqSettingsAuth)

	if recSettingsAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated /pilot/user/settings, got %d", recSettingsAuth.Code)
	}
	settingsBody := recSettingsAuth.Body.String()
	for _, expected := range []string{
		"PILOT / EXPERIMENTAL · SETTINGS",
		"Security",
		"Authentication",
		"Administrative Session Active",
		"Loopback SSH Key",
		"Passkeys (FIDO2 / WebAuthn)",
		"Log out",
	} {
		if !strings.Contains(settingsBody, expected) {
			t.Errorf("missing %q in settings page", expected)
		}
	}

	// 7. Landing page with session cookie reflects authenticated state
	reqLandingAuth := httptest.NewRequest(http.MethodGet, "/", nil)
	reqLandingAuth.AddCookie(sessionCookie)
	recLandingAuth := httptest.NewRecorder()
	router.ServeHTTP(recLandingAuth, reqLandingAuth)

	landingBody := recLandingAuth.Body.String()
	if !strings.Contains(landingBody, "Settings") || !strings.Contains(landingBody, "Log out") {
		t.Errorf("authenticated landing page missing Settings or Log out link")
	}

	// 8. Logout revokes session and redirects to /
	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	recLogout := httptest.NewRecorder()
	router.ServeHTTP(recLogout, reqLogout)

	if recLogout.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on logout, got %d", recLogout.Code)
	}

	// 9. After logout, /pilot/user/settings again redirects to /login
	reqSettingsPostLogout := httptest.NewRequest(http.MethodGet, "/pilot/user/settings", nil)
	reqSettingsPostLogout.AddCookie(sessionCookie)
	recSettingsPostLogout := httptest.NewRecorder()
	router.ServeHTTP(recSettingsPostLogout, reqSettingsPostLogout)

	if recSettingsPostLogout.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after logout, got %d", recSettingsPostLogout.Code)
	}
}

func TestAuthRoutes_CSS(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	for _, path := range []string{
		"/views/login/login.css",
		"/views/pilot/settings.css",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on %s, got %d", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
			t.Errorf("expected text/css on %s, got %q", path, ct)
		}
	}
}

func TestTOTPAuthRoutes_FullFlow(t *testing.T) {
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}

	authSvc, err := auth.NewServiceWithOptions(auth.ServiceOptions{
		TOTPSecret: secret,
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	defer authSvc.Close()

	router := NewRouter(Config{
		AssetsDir:   "../assets",
		AuthService: authSvc,
	})

	// 1. GET /login renders the 6-digit TOTP form
	reqGet := httptest.NewRequest(http.MethodGet, "/login?next=/journals", nil)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET /login, got %d", recGet.Code)
	}
	if !strings.Contains(recGet.Body.String(), `name="code"`) {
		t.Fatalf("expected TOTP input in /login HTML")
	}

	// 2. POST /login with wrong code returns 401
	badForm := url.Values{"code": {"000000"}, "next": {"/journals"}}
	reqBad := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(badForm.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()
	router.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid TOTP code, got %d", recBad.Code)
	}

	// 3. POST /login with valid TOTP code issues session cookie and redirects 303
	validCode, err := auth.GenerateTOTPCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	goodForm := url.Values{"code": {validCode}, "next": {"/journals"}}
	reqGood := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(goodForm.Encode()))
	reqGood.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recGood := httptest.NewRecorder()
	router.ServeHTTP(recGood, reqGood)
	if recGood.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on valid TOTP code, got %d", recGood.Code)
	}
	if loc := recGood.Header().Get("Location"); loc != "/journals" {
		t.Fatalf("expected redirect to /journals, got %q", loc)
	}
	var sessionCookie *http.Cookie
	for _, c := range recGood.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected session cookie after TOTP login")
	}

	// 4. Replaying the same 6-digit code must be rejected with 401
	reqReplay := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(goodForm.Encode()))
	reqReplay.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recReplay := httptest.NewRecorder()
	router.ServeHTTP(recReplay, reqReplay)
	if recReplay.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replayed TOTP code, got %d", recReplay.Code)
	}

	// 5. Authenticated /pilot/user/settings succeeds and shows TOTP
	reqSettings := httptest.NewRequest(http.MethodGet, "/pilot/user/settings", nil)
	reqSettings.AddCookie(sessionCookie)
	recSettings := httptest.NewRecorder()
	router.ServeHTTP(recSettings, reqSettings)
	if recSettings.Code != http.StatusOK {
		t.Fatalf("expected 200 on /pilot/user/settings, got %d", recSettings.Code)
	}
	if !strings.Contains(recSettings.Body.String(), "TOTP (RFC 6238)") {
		t.Errorf("expected TOTP (RFC 6238) in pilot settings page")
	}
}
