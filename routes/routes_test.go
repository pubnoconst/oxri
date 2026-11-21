package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		"/assets/css/site.css?v=2",
		"/views/landing/landing.css?v=1",
		"/assets/js/htmx.min.js",
		"/assets/js/theme-toggle.js?v=2",
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
