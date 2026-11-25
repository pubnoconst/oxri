package assets

import (
	"fmt"
	"strings"
)

// The site's typefaces are served from the site itself (HIG §2, §1.6): no
// third-party connection, content-hashed URLs cached forever, and one
// stylesheet (FontsCSS) generated here so every font URL carries its hash.
//
// Each role is one family assembled from two faces by unicode-range, so a
// phrase never changes face midway:
//
//	Res Book   EB Garamond, Latin and Greek (it has its own polytonic Greek)
//	Res UI     Figtree for Latin, Didact Gothic for all Greek
//	Res Entry  Playpen Sans for Latin, Didact Gothic for all Greek
//	Res Quote  Happy Monkey for Latin, Didact Gothic for all Greek
//
// Only weight 400 is served (HIG §2: no bold), with a true italic for the
// book and UI faces. Each role also has a metric-matched fallback (a local
// system font resized to the web font's width and line metrics), so text
// does not reflow when the web font arrives.
//
// The files are Google Fonts' own subsets of these OFL-licensed families
// (licences alongside, fonts/*-OFL.txt).

// Google Fonts' subset ranges.
const (
	rangeLatin    = "U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304, U+0308, U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD"
	rangeLatinExt = "U+0100-02BA, U+02BD-02C5, U+02C7-02CC, U+02CE-02D7, U+02DD-02FF, U+0304, U+0308, U+0329, U+1D00-1DBF, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C4, U+2113, U+2C60-2C7F, U+A720-A7FF"
	rangeGreek    = "U+0370-0377, U+037A-037F, U+0384-038A, U+038C, U+038E-03A1, U+03A3-03FF"
	rangeGreekExt = "U+1F00-1FFF"
)

var (
	ebGaramondLatin          = file("fonts/eb-garamond-latin-400.woff2")
	ebGaramondLatinExt       = file("fonts/eb-garamond-latin-ext-400.woff2")
	ebGaramondGreek          = file("fonts/eb-garamond-greek-400.woff2")
	ebGaramondGreekExt       = file("fonts/eb-garamond-greek-ext-400.woff2")
	ebGaramondLatinItalic    = file("fonts/eb-garamond-latin-400-italic.woff2")
	ebGaramondLatinExtItalic = file("fonts/eb-garamond-latin-ext-400-italic.woff2")
	ebGaramondGreekItalic    = file("fonts/eb-garamond-greek-400-italic.woff2")
	ebGaramondGreekExtItalic = file("fonts/eb-garamond-greek-ext-400-italic.woff2")

	figtreeLatin          = file("fonts/figtree-latin-400.woff2")
	figtreeLatinExt       = file("fonts/figtree-latin-ext-400.woff2")
	figtreeLatinItalic    = file("fonts/figtree-latin-400-italic.woff2")
	figtreeLatinExtItalic = file("fonts/figtree-latin-ext-400-italic.woff2")

	playpenLatin    = file("fonts/playpen-sans-latin-400.woff2")
	playpenLatinExt = file("fonts/playpen-sans-latin-ext-400.woff2")

	happyMonkeyLatin    = file("fonts/happy-monkey-latin-400.woff2")
	happyMonkeyLatinExt = file("fonts/happy-monkey-latin-ext-400.woff2")

	didactGreek    = file("fonts/didact-gothic-greek-400.woff2")
	didactGreekExt = file("fonts/didact-gothic-greek-ext-400.woff2")
)

// FontLicences are the families' SIL Open Font Licences, served beside them.
var FontLicences = []Asset{
	file("fonts/eb-garamond-OFL.txt"),
	file("fonts/figtree-OFL.txt"),
	file("fonts/playpen-sans-OFL.txt"),
	file("fonts/happy-monkey-OFL.txt"),
	file("fonts/didact-gothic-OFL.txt"),
}

type fontFace struct {
	family, style, rng string
	file               Asset
}

// greekFor gives a role Didact Gothic for every Greek letter.
func greekFor(family string) []fontFace {
	return []fontFace{
		{family, "normal", rangeGreek, didactGreek},
		{family, "normal", rangeGreekExt, didactGreekExt},
	}
}

func fontFaces() []fontFace {
	faces := []fontFace{
		{"Res Book", "normal", rangeLatin, ebGaramondLatin},
		{"Res Book", "normal", rangeLatinExt, ebGaramondLatinExt},
		{"Res Book", "normal", rangeGreek, ebGaramondGreek},
		{"Res Book", "normal", rangeGreekExt, ebGaramondGreekExt},
		{"Res Book", "italic", rangeLatin, ebGaramondLatinItalic},
		{"Res Book", "italic", rangeLatinExt, ebGaramondLatinExtItalic},
		{"Res Book", "italic", rangeGreek, ebGaramondGreekItalic},
		{"Res Book", "italic", rangeGreekExt, ebGaramondGreekExtItalic},

		{"Res UI", "normal", rangeLatin, figtreeLatin},
		{"Res UI", "normal", rangeLatinExt, figtreeLatinExt},
		{"Res UI", "italic", rangeLatin, figtreeLatinItalic},
		{"Res UI", "italic", rangeLatinExt, figtreeLatinExtItalic},

		{"Res Entry", "normal", rangeLatin, playpenLatin},
		{"Res Entry", "normal", rangeLatinExt, playpenLatinExt},

		{"Res Quote", "normal", rangeLatin, happyMonkeyLatin},
		{"Res Quote", "normal", rangeLatinExt, happyMonkeyLatinExt},
	}
	faces = append(faces, greekFor("Res UI")...)
	faces = append(faces, greekFor("Res Entry")...)
	faces = append(faces, greekFor("Res Quote")...)
	return faces
}

// fallbackFace is a local system font resized to match a role's web font,
// so swapping in the web font moves nothing. The percentages (fbBook …) are
// measured in Chrome against each web font's Latin face: size-adjust is the
// web font's width over the local font's for the same sample text; ascent
// and descent are the web font's own, divided by size-adjust.
type fallbackFace struct {
	family                      string
	locals                      []string
	sizeAdjust, ascent, descent float64 // percent
}

type fbMetrics struct{ size, ascent, descent float64 }

// Measured 2026-11-25 against Arial (sans roles) and Georgia (book); see
// fallbackFace.
var (
	fbBook  = fbMetrics{85.4, 118.2, 35.1}  // EB Garamond over Georgia
	fbUI    = fbMetrics{99.3, 95.7, 25.2}   // Figtree over Arial
	fbEntry = fbMetrics{108.5, 107.8, 31.3} // Playpen Sans over Arial
	fbQuote = fbMetrics{110.3, 86.1, 23.6}  // Happy Monkey over Arial
)

var fallbacks = []fallbackFace{
	{"Res Book Fallback", []string{"Georgia", "Times New Roman", "Noto Serif", "Droid Serif"}, fbBook.size, fbBook.ascent, fbBook.descent},
	{"Res UI Fallback", []string{"Arial", "Helvetica", "Roboto", "Liberation Sans"}, fbUI.size, fbUI.ascent, fbUI.descent},
	{"Res Entry Fallback", []string{"Arial", "Helvetica", "Roboto", "Liberation Sans"}, fbEntry.size, fbEntry.ascent, fbEntry.descent},
	{"Res Quote Fallback", []string{"Arial", "Helvetica", "Roboto", "Liberation Sans"}, fbQuote.size, fbQuote.ascent, fbQuote.descent},
}

func fontCSS() string {
	var b strings.Builder
	b.WriteString("/* Generated by assets/fonts.go: the site's self-hosted faces (HIG §2). */\n")
	for _, f := range fontFaces() {
		fmt.Fprintf(&b, "@font-face{font-family:%q;font-style:%s;font-weight:400;font-display:swap;src:url(%s) format(\"woff2\");unicode-range:%s}\n",
			f.family, f.style, f.file.URL(), f.rng)
	}
	for _, f := range fallbacks {
		locals := make([]string, len(f.locals))
		for i, l := range f.locals {
			locals[i] = fmt.Sprintf("local(%q)", l)
		}
		fmt.Fprintf(&b, "@font-face{font-family:%q;font-display:swap;src:%s;size-adjust:%.1f%%;ascent-override:%.1f%%;descent-override:%.1f%%;line-gap-override:0%%}\n",
			f.family, strings.Join(locals, ","), f.sizeAdjust, f.ascent, f.descent)
	}
	return b.String()
}

// FontsCSS is the generated @font-face stylesheet, linked before site.css.
var FontsCSS = Component("/assets/fonts.css", fontCSS())

// No face is preloaded. Measured in the lab (PROFILING.md): preloading the
// UI's Latin face made Chrome hold back the text that uses it until about
// 4.6 s on /login, against 1.15 s without the preload.
