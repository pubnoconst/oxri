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
		"/assets/css/site.css?v=1",
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
