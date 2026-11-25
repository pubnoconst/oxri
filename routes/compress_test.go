package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oxri/oxri-go/assets"
)

// Text goes out compressed; fonts (already compressed) do not.
func TestTextIsCompressedFontsAreNot(t *testing.T) {
	router := NewRouter(Config{})
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	for _, path := range []string{"/", assets.SiteCSS.URL(), assets.HTMXJS.URL(), assets.FontsCSS.URL()} {
		if rec := get(path); rec.Header().Get("Content-Encoding") != "gzip" {
			t.Errorf("%s: Content-Encoding %q, want gzip", path, rec.Header().Get("Content-Encoding"))
		}
	}
	if rec := get("/assets/fonts/figtree-latin-400.woff2"); rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("a woff2 font was compressed again: %q", rec.Header().Get("Content-Encoding"))
	}
}
