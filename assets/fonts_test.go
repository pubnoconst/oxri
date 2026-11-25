package assets

import (
	"regexp"
	"strings"
	"testing"
)

// Only weight 400 is served (HIG §2: no bold), every face is a woff2 with a
// content-hashed URL, and every role has its Greek (no face switch midway).
func TestFontFacesAreRegularAndHashed(t *testing.T) {
	css := fontCSS()
	faces := regexp.MustCompile(`@font-face\{[^}]*\}`).FindAllString(css, -1)
	if len(faces) != len(fontFaces())+len(allFallbacks()) {
		t.Fatalf("got %d @font-face rules", len(faces))
	}
	for _, f := range fontFaces() {
		if !strings.HasSuffix(f.file.Path(), ".woff2") || !strings.Contains(css, f.file.URL()) {
			t.Errorf("%s: not a hashed woff2 URL in the stylesheet", f.file.Path())
		}
	}
	// Every face, the fallbacks included, swaps: a face without font-display
	// defaults to "auto", which Chrome treats as block and hides text for up
	// to 3 s (it held /login's first text back to 4.6 s).
	for _, f := range faces {
		if !strings.Contains(f, "font-display:swap") {
			t.Errorf("face without font-display:swap: %.80s", f)
		}
	}
	if strings.Contains(css, "font-weight:") && regexp.MustCompile(`font-weight:(?:[^4]|4[^0]|40[^0])`).MatchString(css) {
		t.Error("a face other than weight 400")
	}
	for _, role := range []string{"Res Book", "Res UI", "Res Entry", "Res Quote"} {
		var greek, greekExt bool
		for _, f := range fontFaces() {
			if f.family == role && f.style == "normal" {
				greek = greek || f.rng == rangeGreek
				greekExt = greekExt || f.rng == rangeGreekExt
			}
		}
		if !greek || !greekExt {
			t.Errorf("%s lacks Greek or polytonic Greek", role)
		}
	}
	if len(FontLicences) != 5 {
		t.Errorf("each of the five families needs its licence beside it")
	}
}
