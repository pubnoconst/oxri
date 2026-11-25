package routes

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// The one inline script (the theme initialiser) runs before anything is
// drawn, and the CSP allows exactly it, by the hash of the bytes the page
// actually carries; every page has the theme switch.
func TestThemeInitIsTheOnlyInlineScriptAndTheCSPAllowsIt(t *testing.T) {
	router := NewRouter(Config{})
	for _, path := range []string{"/", "/login", "/book-i", "/journals"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		page := rec.Body.String()

		inline := regexp.MustCompile(`<script>(.*?)</script>`).FindAllStringSubmatch(page, -1)
		if len(inline) != 1 {
			t.Fatalf("%s: %d inline scripts, want exactly the theme initialiser", path, len(inline))
		}
		head := page[:strings.Index(page, "</head>")]
		if !strings.Contains(head, "<script>") || strings.Index(head, "<script>") > strings.Index(head, `rel="stylesheet"`) {
			t.Errorf("%s: the theme initialiser must run in <head>, before the stylesheets", path)
		}
		sum := sha256.Sum256([]byte(inline[0][1]))
		want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
		csp := rec.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self' "+want) || strings.Contains(csp, "unsafe-inline' https://platform") {
			t.Errorf("%s: CSP %q does not allow exactly the inline script %s", path, csp, want)
		}
		if !strings.Contains(page, `data-theme-toggle`) {
			t.Errorf("%s: no theme switch", path)
		}
	}
}
