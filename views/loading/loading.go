// Package loading provides the Gomponents awaiting-confirmation state ("Confirming, please wait.").
package loading

import (
	_ "embed"
	"fmt"
	"math"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed loading.css
var StyleCSS string

// StylePath is the public route for the loading component CSS.
const StylePath = "/views/loading/loading.css"

// DefaultPhrase is the non-technical confirmation message displayed during loading transitions.
const DefaultPhrase = "Confirming, please wait."

// CharToken represents a single rendered character in the rippling phrase.
// Distance is how many characters it sits from the phrase's centre; the CSS
// ripple delays each character by it, so the wave visibly spreads outward
// from the middle like the water rings behind it.
type CharToken struct {
	Index    int
	Distance float64
	Text     string
	IsSpace  bool
}

// SplitPhrase splits a phrase into individual character tokens preserving word spaces.
func SplitPhrase(phrase string) []CharToken {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	runes := []rune(phrase)
	centre := float64(len(runes)-1) / 2
	out := make([]CharToken, 0, len(runes))
	for i, r := range runes {
		d := math.Abs(float64(i) - centre)
		if r == ' ' {
			out = append(out, CharToken{Index: i, Distance: d, Text: "\u00A0", IsSpace: true})
		} else {
			out = append(out, CharToken{Index: i, Distance: d, Text: string(r), IsSpace: false})
		}
	}
	return out
}

func renderPhraseChars(phrase string) []g.Node {
	tokens := SplitPhrase(phrase)
	nodes := make([]g.Node, 0, len(tokens))
	for _, ch := range tokens {
		cls := "loading-char"
		if ch.IsSpace {
			cls += " is-space"
		}
		nodes = append(nodes, h.Span(
			h.Class(cls),
			h.Style(fmt.Sprintf("--d: %.1f;", ch.Distance)),
			g.Attr("aria-hidden", "true"),
			g.Text(ch.Text),
		))
	}
	return nodes
}

// phraseNode renders the confirmation phrase. The whole linen surface around
// it (page or overlay) is what water-ripple.js refracts; see
// assets/js/loading-transition.js.
func phraseNode(phrase string) g.Node {
	return h.P(
		h.Class("loading-phrase"),
		g.Attr("aria-label", phrase),
		g.Group(renderPhraseChars(phrase)),
	)
}

// rippleRings renders the same sunlit water-drop rings used by the journals
// catalog, centred on the phrase, so every "please wait" state in
// the app speaks one visual language: water.
func rippleRings() g.Node {
	return h.Div(
		h.Class("card-water-ripple loading-water-ripple"),
		g.Attr("style", "--ripple-x:50%;--ripple-y:50%;--ripple-diameter:150vmax;"),
		g.Attr("aria-hidden", "true"),
		h.Span(h.Class("card-water-wave wave-1")),
		h.Span(h.Class("card-water-wave wave-2")),
		h.Span(h.Class("card-water-wave wave-3")),
	)
}

// Page renders the standalone awaiting confirmation view inside the base layout shell.
func Page(phrase string) g.Node {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	return layout.Doc(
		"RES IMPERII",
		StylePath+"?v="+styleVersion,
		Stage(phrase),
		h.Script(h.Src("/assets/js/water-ripple.js?v=2")),
		h.Script(h.Src(TransitionScriptPath)),
	)
}

// styleVersion is the cache key for loading.css.
const styleVersion = "3"

// TransitionScriptPath is the script that shows the overlay on submit and
// drives the phrase refraction; pages embedding Overlay must include it
// together with /assets/js/water-ripple.js.
const TransitionScriptPath = "/assets/js/loading-transition.js?v=4"

// Stage renders the centered loading phrase stage.
func Stage(phrase string) g.Node {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	return h.Main(
		h.Class("loading-page"),
		g.Attr("aria-live", "polite"),
		g.Attr("aria-busy", "true"),
		phraseNode(phrase),
		rippleRings(),
	)
}

// Overlay renders the full-screen confirmation overlay for form submissions and auto-resubmits.
func Overlay(phrase string, initiallyVisible bool) g.Node {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	cls := "loading-overlay"
	if initiallyVisible {
		cls += " is-visible"
	}
	attrs := []g.Node{
		h.ID("confirm-loading-overlay"),
		h.Class(cls),
		g.Attr("aria-live", "polite"),
		g.Attr("aria-busy", "true"),
	}
	if !initiallyVisible {
		attrs = append(attrs, g.Attr("hidden", ""))
	}
	attrs = append(attrs, phraseNode(phrase), rippleRings())

	return g.Group([]g.Node{
		h.Link(h.Rel("stylesheet"), h.Href(StylePath+"?v="+styleVersion)),
		h.Div(attrs...),
	})
}
