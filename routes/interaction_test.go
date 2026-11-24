package routes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// HIG §5 "Links and buttons": nothing moves or grows on hover or focus. Links
// light up and underline, controls tint; the only movement in the system is
// a button sinking a pixel while pressed (.btn:active). This guards every
// stylesheet against a stray scale or slide creeping back in.
func TestHoverAndFocusNeverMove(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../assets/css/*.css", "../views/*/*.css"} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, found...)
	}
	if len(files) < 5 {
		t.Fatalf("found only %d stylesheets; is the test running from routes/?", len(files))
	}

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	rule := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	state := regexp.MustCompile(`:hover|:focus-visible|:focus-within|:focus\b`)
	motion := regexp.MustCompile(`transform\s*:[^;]*\b(scale|translate[XY]?|rotate)\s*\(`)

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		css := comments.ReplaceAllString(string(b), "")
		for _, m := range rule.FindAllStringSubmatch(css, -1) {
			selector := strings.Join(strings.Fields(m[1]), " ")
			if state.MatchString(selector) && motion.MatchString(m[2]) {
				t.Errorf("%s: %q moves on hover/focus: %s", filepath.Base(f), selector, strings.TrimSpace(m[2]))
			}
		}
	}
}
