package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/serverfns"
)

func TestSecurityHeadersAndNoDirectoryListing(t *testing.T) {
	router := NewRouter(Config{AssetsDir: "../assets"})

	// 1. Verify Security Headers on GET /
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /, got %d", rec.Code)
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "object-src 'none'") {
		t.Errorf("missing expected directives in Content-Security-Policy: %q", csp)
	}
	if strings.Contains(csp, "unsafe-eval") {
		t.Errorf("Content-Security-Policy must not contain 'unsafe-eval': %q", csp)
	}
	foundScriptSrc := false
	for _, dir := range strings.Split(csp, ";") {
		trimmed := strings.TrimSpace(dir)
		if strings.HasPrefix(trimmed, "script-src ") {
			foundScriptSrc = true
			if strings.Contains(trimmed, "unsafe-inline") {
				t.Errorf("Content-Security-Policy script-src must not contain 'unsafe-inline': %q", trimmed)
			}
			if !strings.Contains(trimmed, "'self'") {
				t.Errorf("Content-Security-Policy script-src missing 'self': %q", trimmed)
			}
		}
	}
	if !foundScriptSrc {
		t.Errorf("Content-Security-Policy missing script-src directive: %q", csp)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected X-Frame-Options: DENY, got %q", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy: strict-origin-when-cross-origin, got %q", got)
	}

	// 2. Verify Directory Listings are blocked on /assets/ and /assets/js/
	for _, dirPath := range []string{"/assets/", "/assets/js/", "/assets/css/"} {
		dReq := httptest.NewRequest(http.MethodGet, dirPath, nil)
		dRec := httptest.NewRecorder()
		router.ServeHTTP(dRec, dReq)
		if dRec.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found on directory path %s, got %d", dirPath, dRec.Code)
		}
	}

	// 3. Verify locally vendored editor scripts are served cleanly
	for _, scriptPath := range []string{"/assets/js/markdown-it.min.js", "/assets/js/purify.min.js"} {
		sReq := httptest.NewRequest(http.MethodGet, scriptPath, nil)
		sRec := httptest.NewRecorder()
		router.ServeHTTP(sRec, sReq)
		if sRec.Code != http.StatusOK {
			t.Errorf("expected 200 OK on vendored script %s, got %d", scriptPath, sRec.Code)
		}
	}
}

func TestCSRFProtectionAndServerSideInputSanitization(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open memory failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	authSvc, sessCookie := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	router := NewRouter(Config{
		AssetsDir:   "../assets",
		AuthService: authSvc,
		Store:       store,
	})

	// 1. Cross-Site POST blocked by Sec-Fetch-Site
	crossReq := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader("media_url=https://example.com/a.png&body=Test"))
	crossReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crossReq.Header.Set("Sec-Fetch-Site", "cross-site")
	crossReq.AddCookie(sessCookie)
	crossRec := httptest.NewRecorder()
	router.ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden on cross-site POST, got %d", crossRec.Code)
	}

	// 2. Server-side rejection & sanitization of evil input bypassing client JS
	evilCases := []struct {
		name          string
		form          url.Values
		forbiddenEcho string
	}{
		{
			name: "emoji in body",
			form: url.Values{
				"media_url": {"https://example.com/valid.png"},
				"tags":      {"Log"},
				"body":      {"Bypassing client JS with emoji 🚀 in prose"},
			},
			forbiddenEcho: "🚀",
		},
		{
			name: "non-Latin/Greek script in body",
			form: url.Values{
				"media_url": {"https://example.com/valid.png"},
				"tags":      {"Log"},
				"body":      {"Bypassing client JS with Cyrillic мир"},
			},
			forbiddenEcho: "мир",
		},
		{
			name: "emoji in tag",
			form: url.Values{
				"media_url": {"https://example.com/valid.png"},
				"tags":      {"Log🔥"},
				"body":      {"Valid prose content"},
			},
			forbiddenEcho: "🔥",
		},
		{
			name: "SSRF localhost media URL",
			form: url.Values{
				"media_url": {"https://localhost/internal-admin"},
				"tags":      {"Log"},
				"body":      {"Valid prose content"},
			},
			forbiddenEcho: "",
		},
		{
			name: "SSRF cloud metadata IP media URL",
			form: url.Values{
				"media_url": {"https://169.254.169.254/latest/meta-data"},
				"tags":      {"Log"},
				"body":      {"Valid prose content"},
			},
			forbiddenEcho: "",
		},
	}

	for _, tc := range evilCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(sessCookie)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422 Unprocessable Entity for %s, got %d", tc.name, rec.Code)
			}
			if tc.forbiddenEcho != "" && strings.Contains(rec.Body.String(), tc.forbiddenEcho) {
				t.Errorf("SECURITY LEAK: server echoed disallowed character %q in 422 HTML response", tc.forbiddenEcho)
			}
		})
	}

	// Verify 0 rows were persisted from any evil request
	all, err := store.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 persisted entries after evil requests, got %d", len(all))
	}
}

func TestProxyIPTrustAndSpoofDefense(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("TRUSTED_PROXY_HOPS", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "")

	// 1. Even with TRUST_PROXY=true, untrusted RemoteAddr must NOT trust forwarded headers
	t.Setenv("TRUST_PROXY", "true")
	untrustedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	untrustedReq.RemoteAddr = "192.0.2.9:4321"
	untrustedReq.Header.Set("True-Client-IP", "198.51.100.1")
	untrustedReq.Header.Set("CF-Connecting-IP", "198.51.100.2")
	untrustedReq.Header.Set("X-Forwarded-For", "198.51.100.3")
	untrustedReq.Header.Set("X-Forwarded-Proto", "https")

	if got := clientIPFromRequest(untrustedReq); got != "192.0.2.9" {
		t.Fatalf("expected peer IP 192.0.2.9 for untrusted proxy RemoteAddr, got %q", got)
	}
	if isHTTPSRequest(untrustedReq) {
		t.Fatalf("expected X-Forwarded-Proto to be ignored from untrusted RemoteAddr")
	}

	router := NewRouter(Config{AssetsDir: "../assets"})
	hstsRec := httptest.NewRecorder()
	router.ServeHTTP(hstsRec, untrustedReq)
	if got := hstsRec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS header when X-Forwarded-Proto comes from untrusted RemoteAddr, got %q", got)
	}

	// 2. Trusted CGNAT (Render-style 100.64.0.0/10) RemoteAddr ignores left-side XFF spoof
	xffReq := httptest.NewRequest(http.MethodGet, "/", nil)
	xffReq.RemoteAddr = "100.64.12.34:443"
	xffReq.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.77")
	xffReq.Header.Set("X-Forwarded-Proto", "https")
	if got := clientIPFromRequest(xffReq); got != "198.51.100.77" {
		t.Fatalf("expected rightmost X-Forwarded-For hop 198.51.100.77, got %q", got)
	}
	if !isHTTPSRequest(xffReq) {
		t.Fatalf("expected X-Forwarded-Proto=https to be honored from trusted proxy RemoteAddr")
	}

	// 3. True-Client-IP takes precedence over XFF on trusted proxy requests
	tcReq := httptest.NewRequest(http.MethodGet, "/", nil)
	tcReq.RemoteAddr = "100.64.12.34:443"
	tcReq.Header.Set("True-Client-IP", "203.0.113.5")
	tcReq.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.77")
	if got := clientIPFromRequest(tcReq); got != "203.0.113.5" {
		t.Fatalf("expected True-Client-IP 203.0.113.5, got %q", got)
	}

	// 4. TRUST_PROXY=false ignores forwarded headers even from private network RemoteAddr
	t.Setenv("TRUST_PROXY", "false")
	noTrustReq := httptest.NewRequest(http.MethodGet, "/", nil)
	noTrustReq.RemoteAddr = "10.0.0.5:1234"
	noTrustReq.Header.Set("True-Client-IP", "203.0.113.5")
	noTrustReq.Header.Set("X-Forwarded-For", "198.51.100.77")
	if got := clientIPFromRequest(noTrustReq); got != "10.0.0.5" {
		t.Fatalf("expected 10.0.0.5 when TRUST_PROXY=false, got %q", got)
	}
}

func TestLogoutPostOnly(t *testing.T) {
	authSvc, sessionCookie := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	router := NewRouter(Config{
		AssetsDir:   "../assets",
		AuthService: authSvc,
	})

	// 1. GET /logout must return 405 Method Not Allowed and preserve the session
	getReq := httptest.NewRequest(http.MethodGet, "/logout", nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed on GET /logout, got %d", getRec.Code)
	}

	checkReq := httptest.NewRequest(http.MethodGet, "/settings", nil)
	checkReq.AddCookie(sessionCookie)
	checkRec := httptest.NewRecorder()
	router.ServeHTTP(checkRec, checkReq)
	if checkRec.Code != http.StatusOK {
		t.Fatalf("expected session to remain active after GET /logout, got %d", checkRec.Code)
	}

	// 2. POST /logout must revoke session and redirect 303 to /
	postReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	postReq.AddCookie(sessionCookie)
	postRec := httptest.NewRecorder()
	router.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusSeeOther || postRec.Header().Get("Location") != "/" {
		t.Fatalf("expected 303 to / on POST /logout, got %d loc=%q", postRec.Code, postRec.Header().Get("Location"))
	}

	afterReq := httptest.NewRequest(http.MethodGet, "/settings", nil)
	afterReq.AddCookie(sessionCookie)
	afterRec := httptest.NewRecorder()
	router.ServeHTTP(afterRec, afterReq)
	if afterRec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect to /login after POST /logout, got %d", afterRec.Code)
	}
}

func TestPersistentTOTPReplayAcrossRestart(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	now := time.Unix(1700000015, 0).UTC()

	svc1, err := auth.NewServiceWithOptions(auth.ServiceOptions{
		TOTPSecret:  secret,
		ReplayStore: store,
		NowFunc:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions(1): %v", err)
	}

	code, err := auth.GenerateTOTPCode(secret, now)
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	if _, err := svc1.AuthenticateTOTPForIP("198.51.100.10", code); err != nil {
		t.Fatalf("AuthenticateTOTPForIP(1): %v", err)
	}
	_ = svc1.Close()

	// New Service backed by the same DB store (simulating server restart within 30s)
	svc2, err := auth.NewServiceWithOptions(auth.ServiceOptions{
		TOTPSecret:  secret,
		ReplayStore: store,
		NowFunc:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions(2): %v", err)
	}
	defer func() { _ = svc2.Close() }()

	if _, err := svc2.AuthenticateTOTPForIP("198.51.100.10", code); !errors.Is(err, auth.ErrTOTPReplayed) {
		t.Fatalf("expected ErrTOTPReplayed across restart with db.Store, got %v", err)
	}
}

func TestPilotRoutesDisabled(t *testing.T) {
	authSvc, sessionCookie := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	router := NewRouter(Config{
		AssetsDir:   "../assets",
		AuthService: authSvc,
	})

	removedEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/pilot"},
		{http.MethodGet, "/pilot/user/settings"},
		{http.MethodPost, "/pilot/passkeys/begin"},
		{http.MethodPost, "/pilot/passkeys/finish"},
		{http.MethodPost, "/pilot/passkeys/abc/delete"},
		{http.MethodGet, "/views/pilot/settings.css"},
	}

	for _, tc := range removedEndpoints {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.AddCookie(sessionCookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found on removed route %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}

	if got := serverfns.SanitizeRedirect("/pilot/user/settings"); got != "/" {
		t.Errorf("expected SanitizeRedirect(/pilot/user/settings) -> /, got %q", got)
	}
	if got := serverfns.SanitizeRedirect("/settings"); got != "/settings" {
		t.Errorf("expected SanitizeRedirect(/settings) -> /settings, got %q", got)
	}
}
