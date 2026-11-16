package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer() http.Handler {
	return NewServer("../../assets")
}

func TestLandingPage(t *testing.T) {
	server := newTestServer()
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
	server := newTestServer()
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
	server := newTestServer()
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
}

func TestJournalEditorPage(t *testing.T) {
	server := newTestServer()
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

func TestBookIINotFound(t *testing.T) {
	server := newTestServer()
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
	server := newTestServer()
	cssRoutes := []string{
		"/page-css/landing.css",
		"/page-css/book-i.css",
		"/page-css/journals.css",
		"/page-css/journal-editor.css",
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
