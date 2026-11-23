package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
)

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
		"/assets/css/site.css?v=9",
		"/views/landing/landing.css?v=2",
		"/assets/js/htmx.min.js",
		"/assets/js/theme-toggle.js?v=2",
		"/assets/js/navigation-feedback.js?v=2",
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
		"<h1>Book I</h1>",
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

func TestAuthRoutes_CSS(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	for _, path := range []string{
		"/views/login/login.css",
		"/views/settings/settings.css",
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
		TOTPSecret:       secret,
		TOTPConfirmDelay: 60 * time.Millisecond,
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

	extractBindingCookie := func(rec *httptest.ResponseRecorder) *http.Cookie {
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.BindingCookieName {
				return c
			}
		}
		return nil
	}

	// 2. POST /login with wrong code redirects 303 to /login/confirm (does not leak failure immediately)
	badForm := url.Values{"code": {"000000"}, "next": {"/journals"}}
	reqBad := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(badForm.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()
	router.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 to /login/confirm on TOTP submit, got %d", recBad.Code)
	}
	badConfirmLoc := recBad.Header().Get("Location")
	if !strings.HasPrefix(badConfirmLoc, "/login/confirm?id=") {
		t.Fatalf("expected /login/confirm redirect, got %q", badConfirmLoc)
	}
	badID := strings.TrimPrefix(badConfirmLoc, "/login/confirm?id=")
	badBind := extractBindingCookie(recBad)
	if badBind == nil {
		t.Fatalf("expected binding cookie after TOTP submit")
	}

	// Confirm page renders ripple, loading bar, and "Confirming, wait."
	reqConfirmPage := httptest.NewRequest(http.MethodGet, badConfirmLoc, nil)
	reqConfirmPage.AddCookie(badBind)
	recConfirmPage := httptest.NewRecorder()
	router.ServeHTTP(recConfirmPage, reqConfirmPage)
	if recConfirmPage.Code != http.StatusOK {
		t.Fatalf("expected 200 on /login/confirm, got %d", recConfirmPage.Code)
	}
	confirmHTML := recConfirmPage.Body.String()
	for _, expected := range []string{"Confirming, wait.", "card-water-ripple", "auth-cooldown-fill"} {
		if !strings.Contains(confirmHTML, expected) {
			t.Errorf("missing %q in /login/confirm HTML", expected)
		}
	}

	// Polling before cooldown elapses returns 204 No Content
	reqPollEarly := httptest.NewRequest(http.MethodGet, "/auth/totp-status?id="+badID, nil)
	reqPollEarly.Header.Set("HX-Request", "true")
	reqPollEarly.AddCookie(badBind)
	recPollEarly := httptest.NewRecorder()
	router.ServeHTTP(recPollEarly, reqPollEarly)
	if recPollEarly.Code != http.StatusNoContent {
		t.Fatalf("expected 204 while confirming, got %d", recPollEarly.Code)
	}

	// After cooldown, polling redirects back to /login?err=invalid
	time.Sleep(75 * time.Millisecond)
	reqPollBad := httptest.NewRequest(http.MethodGet, "/auth/totp-status?id="+badID, nil)
	reqPollBad.Header.Set("HX-Request", "true")
	reqPollBad.AddCookie(badBind)
	recPollBad := httptest.NewRecorder()
	router.ServeHTTP(recPollBad, reqPollBad)
	if recPollBad.Code != http.StatusOK || !strings.Contains(recPollBad.Header().Get("HX-Redirect"), "err=invalid") {
		t.Fatalf("expected HX-Redirect with err=invalid, got status=%d hx=%q", recPollBad.Code, recPollBad.Header().Get("HX-Redirect"))
	}

	// 3. POST /login with valid TOTP code confirms after cooldown and sets session cookie
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
		t.Fatalf("expected 303 on valid TOTP submit, got %d", recGood.Code)
	}
	goodID := strings.TrimPrefix(recGood.Header().Get("Location"), "/login/confirm?id=")
	goodBind := extractBindingCookie(recGood)
	if goodBind == nil {
		t.Fatalf("expected binding cookie on valid TOTP submit")
	}

	time.Sleep(75 * time.Millisecond)
	reqPollGood := httptest.NewRequest(http.MethodGet, "/auth/totp-status?id="+goodID, nil)
	reqPollGood.Header.Set("HX-Request", "true")
	reqPollGood.AddCookie(goodBind)
	recPollGood := httptest.NewRecorder()
	router.ServeHTTP(recPollGood, reqPollGood)
	if recPollGood.Code != http.StatusOK || recPollGood.Header().Get("HX-Redirect") != "/journals" {
		t.Fatalf("expected HX-Redirect /journals, got status=%d hx=%q", recPollGood.Code, recPollGood.Header().Get("HX-Redirect"))
	}
	var sessionCookie *http.Cookie
	for _, c := range recPollGood.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected session cookie after TOTP confirmation")
	}

	// 4. Authenticated /settings succeeds and shows TOTP
	reqSettings := httptest.NewRequest(http.MethodGet, "/settings", nil)
	reqSettings.AddCookie(sessionCookie)
	recSettings := httptest.NewRecorder()
	router.ServeHTTP(recSettings, reqSettings)
	if recSettings.Code != http.StatusOK {
		t.Fatalf("expected 200 on /settings, got %d", recSettings.Code)
	}
	if !strings.Contains(recSettings.Body.String(), "TOTP (RFC 6238)") {
		t.Errorf("expected TOTP (RFC 6238) in settings page")
	}

	// 5. Logout revokes session and redirects to /
	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	recLogout := httptest.NewRecorder()
	router.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on logout, got %d", recLogout.Code)
	}

	// 6. After logout, /settings redirects to /login
	reqAfterLogout := httptest.NewRequest(http.MethodGet, "/settings", nil)
	reqAfterLogout.AddCookie(sessionCookie)
	recAfterLogout := httptest.NewRecorder()
	router.ServeHTTP(recAfterLogout, reqAfterLogout)
	if recAfterLogout.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after logout, got %d", recAfterLogout.Code)
	}
}
