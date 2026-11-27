package labsite

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/views/journals"
)

// TestSiteServesSeededContentAndLabImages covers what both labs rely on: a
// catalog rendered from seeded entries, the photos served in place of X, and
// the count of them for the report.
func TestSiteServesSeededContentAndLabImages(t *testing.T) {
	site, err := New(context.Background(), 14, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer site.Close()
	t.Cleanup(func() { journals.CatalogPageSize = 24 })
	srv := httptest.NewServer(site.Handler())
	defer srv.Close()

	get := func(path string) (*http.Response, string) {
		t.Helper()
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return res, string(body)
	}

	res, body := get("/journals")
	for _, want := range []string{"What is good?", "John 3:16", "pbs.twimg.com/media/lab000000.jpg"} {
		if res.StatusCode != http.StatusOK || !strings.Contains(body, want) {
			t.Errorf("/journals: status %d, holds %q: %t", res.StatusCode, want, strings.Contains(body, want))
		}
	}
	for _, tc := range []struct{ query, size string }{{"", "large"}, {"?name=small", "small"}} {
		res, body := get(ImagePath + tc.query)
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" || len(body) != len(images[tc.size]) {
			t.Errorf("GET %s%s: status %d, type %q, %d bytes, want the %s photo's %d",
				ImagePath, tc.query, res.StatusCode, res.Header.Get("Content-Type"), len(body), tc.size, len(images[tc.size]))
		}
	}

	served := len(images["large"]) + len(images["small"])
	res, body = get(StatsPath)
	if want := `{"count":2,"bytes":` + strconv.Itoa(served) + "}" + "\n"; res.StatusCode != http.StatusOK || body != want {
		t.Errorf("GET %s: status %d, body %q, want %q", StatsPath, res.StatusCode, body, want)
	}
	if count, bytes := site.Images(); count != 2 || bytes != int64(served) {
		t.Errorf("Images() = %d, %d; want 2, %d", count, bytes, served)
	}

	res, body = get("/")
	if res.StatusCode != http.StatusOK || strings.Contains(body, "Sign out") {
		t.Errorf("/: status %d; a reader must be signed out, so the page offers no Sign out: %t", res.StatusCode, strings.Contains(body, "Sign out"))
	}

	// The lab ends the server with a POST; anything else is refused.
	if res, _ := get(StopPath); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET %s: status %d, want %d", StopPath, res.StatusCode, http.StatusMethodNotAllowed)
	}
	stops := 0
	site.OnStop(func() { stops++ })
	for i := 0; i < 2; i++ {
		res, err := http.Post(srv.URL+StopPath, "text/plain", nil)
		if err != nil {
			t.Fatalf("POST %s: %v", StopPath, err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Errorf("POST %s: status %d, want %d", StopPath, res.StatusCode, http.StatusNoContent)
		}
	}
	if stops != 1 {
		t.Errorf("asked twice to stop, the server was told %d times; want once", stops)
	}
}
