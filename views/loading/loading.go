// Package loading provides the Gomponents awaiting-confirmation state ("Confirming, please wait.").
package loading

import (
	_ "embed"

	"github.com/oxri/oxri-go/assets"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed loading.css
var StyleCSS string

// StylePath is the public route for the loading component CSS.
const StylePath = "/views/loading/loading.css"

// Style is the content-hashed handle for StyleCSS (URL and route handler).
var Style = assets.Component(StylePath, StyleCSS)

// DefaultPhrase is the non-technical confirmation message displayed during loading transitions.
const DefaultPhrase = "Confirming, please wait."

// phraseNode renders the confirmation phrase. It stands still; the dot matrix
// beside it is the motion.
func phraseNode(phrase string) g.Node {
	return h.P(h.Class("loading-phrase"), g.Text(phrase))
}

// Motion is what a dot matrix means; assets/js/dot-matrix.js draws it. The
// only values are the three below (the field is unexported), so a view can't
// ask for a motion that doesn't exist.
type Motion struct{ name string }

var (
	// Puzzle: the site is working on something you did (sign-in, saving).
	Puzzle = Motion{"puzzle"}
	// Twinkle: something is downloading (tweets, the catalog, a page).
	Twinkle = Motion{"twinkle"}
	// Comet: waiting on you (passkey prompts).
	Comet = Motion{"comet"}
)

// Dots renders the site's loader: a 3x3 dot matrix in the given motion and a
// label (site.css .dot-loading). The JS twin is ResImperiiDots.create.
func Dots(label string, m Motion) g.Node {
	return h.Span(h.Class("dot-loading"),
		dotMatrix(m),
		h.Span(h.Class("dot-loading-label"), g.Text(label)),
	)
}

// dotMatrix is the 3x3 matrix alone (nine dots). dot-matrix.js starts it with
// a fresh seed; the Confirming phrase uses it directly, beside the phrase.
func dotMatrix(m Motion) g.Node {
	dots := make([]g.Node, 9)
	for i := range dots {
		dots[i] = h.I()
	}
	return h.Span(h.Class("dot-matrix"), g.Attr("data-motion", m.name), g.Attr("aria-hidden", "true"), g.Group(dots))
}

// Page renders the standalone awaiting confirmation view inside the base layout shell.
func Page(phrase string) g.Node {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	return layout.Doc(
		"RES IMPERII",
		Style.URL(),
		Stage(phrase),
		h.Script(h.Src(assets.LoadingTransitionJS.URL())),
	)
}

// Stage renders the centered loading phrase stage.
func Stage(phrase string) g.Node {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	return h.Main(
		h.Class("loading-page"),
		g.Attr("aria-live", "polite"),
		g.Attr("aria-busy", "true"),
		h.Div(h.Class("loading-phrase-row"), dotMatrix(Puzzle), phraseNode(phrase)),
	)
}

// Overlay renders the full-screen confirmation overlay for form submissions and auto-resubmits.
// Pages embedding it must also load assets.LoadingTransitionJS.
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
	attrs = append(attrs, h.Div(h.Class("loading-phrase-row"), dotMatrix(Puzzle), phraseNode(phrase)))

	return g.Group([]g.Node{
		h.Link(h.Rel("stylesheet"), h.Href(Style.URL())),
		h.Div(attrs...),
	})
}
