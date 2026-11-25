package routes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestBlurComesFromGlassTokens: the site has exactly two glasses (HIG §4),
// float and veil, and they are its only blur. Every backdrop-filter names one
// of their tokens; no filter blurs content; the blur values live only in
// site.css's glass tokens.
func TestBlurComesFromGlassTokens(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../assets/css/*.css", "../views/*/*.css"} {
		found, _ := filepath.Glob(pattern)
		files = append(files, found...)
	}
	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	supports := regexp.MustCompile(`@supports\s*\([^{]*\)`)
	backdrop := regexp.MustCompile(`backdrop-filter\s*:\s*([^;]+);`)
	filterBlur := regexp.MustCompile(`(?:^|[^-])filter\s*:[^;]*blur\(`)
	glass := regexp.MustCompile(`^var\(--depth-(overlay|veil)-blur\)$`)
	blurDef := regexp.MustCompile(`--([a-z0-9-]+)\s*:[^;]*blur\(`)

	if len(files) == 0 {
		t.Fatal("no stylesheets found")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		css := supports.ReplaceAllString(comments.ReplaceAllString(string(b), ""), "")
		for _, m := range backdrop.FindAllStringSubmatch(css, -1) {
			if v := strings.TrimSpace(m[1]); !glass.MatchString(v) {
				t.Errorf("%s: backdrop-filter: %s; use var(--depth-overlay-blur) or var(--depth-veil-blur)", f, v)
			}
		}
		if filterBlur.MatchString(css) {
			t.Errorf("%s: filter: blur() over content; only overlays blur, and only what lies beneath them", f)
		}
		for _, m := range blurDef.FindAllStringSubmatch(css, -1) {
			if filepath.Base(f) != "site.css" || (m[1] != "depth-overlay-blur" && m[1] != "depth-veil-blur") {
				t.Errorf("%s: --%s defines a blur; the only blurs are --depth-overlay-blur and --depth-veil-blur in site.css", f, m[1])
			}
		}
	}
}
