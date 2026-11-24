package routes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// HIG §4 Radii: surfaces take their radius from the tokens (--radius-card,
// --radius-control, --radius-pill) or 50% for a circle, so a clipping and a
// raw image laid on the same card can't drift apart again. Literal radii are
// allowed only where the HIG names an exception.
func TestRadiiComeFromTokens(t *testing.T) {
	// Every selector in a rule must name one of these for a literal radius.
	exceptions := []string{
		`input[type="checkbox"]`, // a glyph, not a surface
		"tweet-embed-container",  // matches the frame X draws in its iframe
		"twitter-tweet",
	}
	allowed := func(selector string) bool {
		for _, part := range strings.Split(selector, ",") {
			ok := false
			for _, e := range exceptions {
				if strings.Contains(part, e) {
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
		return true
	}
	var files []string
	for _, pattern := range []string{"../assets/css/*.css", "../views/*/*.css"} {
		found, _ := filepath.Glob(pattern)
		files = append(files, found...)
	}
	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	rule := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	radius := regexp.MustCompile(`border(?:-[a-z]+)*-radius\s*:\s*([^;]+);`)
	token := regexp.MustCompile(`^(var\(--radius-(card|control|pill)\)|50%|0)(\s*!important)?$`)

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		css := comments.ReplaceAllString(string(b), "")
		for _, m := range rule.FindAllStringSubmatch(css, -1) {
			selector := strings.Join(strings.Fields(m[1]), " ")
			for _, r := range radius.FindAllStringSubmatch(m[2], -1) {
				value := strings.TrimSpace(r[1])
				if !token.MatchString(value) && !allowed(selector) {
					t.Errorf("%s: %q has border-radius %q; use --radius-card, --radius-control or --radius-pill", filepath.Base(f), selector, value)
				}
			}
		}
	}
}
