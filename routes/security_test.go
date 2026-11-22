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
		name         string
		form         url.Values
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
