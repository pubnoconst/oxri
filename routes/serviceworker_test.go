package routes

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/assets"
	"github.com/oxri/oxri-go/views/offline"
)

// The service worker is served from the root (so its scope is the whole site),
// always revalidated, with its cache name tied to the offline page; the
// offline page is self-contained, and every page registers the worker.
func TestServiceWorkerAndOfflinePage(t *testing.T) {
	router := NewRouter(Config{})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	sw := get(ServiceWorkerPath)
	if sw.Code != http.StatusOK || !strings.HasPrefix(sw.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("sw.js: %d %q", sw.Code, sw.Header().Get("Content-Type"))
	}
	if sw.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("sw.js must revalidate so deploys reach phones: %q", sw.Header().Get("Cache-Control"))
	}
	body := sw.Body.String()
	if strings.Contains(body, offlineHashMarker) || !regexp.MustCompile(`"offline-[0-9a-f]{12}"`).MatchString(body) {
		t.Errorf("sw.js cache name is not tied to the offline page's hash")
	}
	if !strings.Contains(body, `"`+offline.Path+`"`) {
		t.Errorf("sw.js does not cache %s", offline.Path)
	}

	page := get(offline.Path)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "You&#39;re offline") {
		t.Fatalf("offline page: %d %s", page.Code, page.Body.String())
	}
	// Cached alone, so it may depend on nothing else.
	for _, banned := range []string{"<script", "<link", "/assets/"} {
		if strings.Contains(page.Body.String(), banned) {
			t.Errorf("offline page must be self-contained; found %q", banned)
		}
	}

	if !strings.Contains(get("/").Body.String(), assets.OfflineJS.URL()) {
		t.Errorf("pages must load offline.js, which registers the worker")
	}
}
