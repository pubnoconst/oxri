package routes

import (
	"github.com/oxri/oxri-go/assets"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/views/landing"
)

// HIG §2: no bold. Hierarchy is size, never weight; only weight 400 (and its
// italic) is loaded, and the browser may not fake a heavier one.
func TestNoBold(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../assets/css/*.css", "../views/*/*.css"} {
		found, _ := filepath.Glob(pattern)
		files = append(files, found...)
	}
	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	weight := regexp.MustCompile(`font-weight\s*:\s*([^;]+);`)
	weightInShorthand := regexp.MustCompile(`(?m)^\s*font\s*:[^;]*\b(bold|bolder|[5-9]00)\b`)
	sawSynthesisOff := false
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		css := comments.ReplaceAllString(string(b), "")
		for _, m := range weight.FindAllStringSubmatch(css, -1) {
			if v := strings.TrimSpace(m[1]); v != "400" && v != "normal" && v != "inherit" {
				t.Errorf("%s: font-weight %s; use size for hierarchy, not weight", filepath.Base(f), v)
			}
		}
		if m := weightInShorthand.FindString(css); m != "" {
			t.Errorf("%s: bold in a font shorthand: %q", filepath.Base(f), strings.TrimSpace(m))
		}
		if strings.Contains(css, "font-synthesis-weight: none") {
			sawSynthesisOff = true
		}
	}
	if !sawSynthesisOff {
		t.Error("site.css must stop the browser faking bold (font-synthesis-weight: none)")
	}

	// The fonts are the site's own (assets/fonts.go), and nothing asks a
	// third party for them.
	var page strings.Builder
	_ = landing.Page(false).Render(&page)
	if !strings.Contains(page.String(), assets.FontsCSS.URL()) || strings.Contains(page.String(), "fonts.googleapis.com") {
		t.Error("pages must link the self-hosted font stylesheet, not Google Fonts")
	}
}
