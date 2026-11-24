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

// Dots renders the site's loader: a 3x3 dot matrix and a label (site.css
// .dot-loading). The JS twin is dotLoading() in journal-media.js.
func Dots(label string) g.Node {
	return h.Span(h.Class("dot-loading"),
		dotMatrix(),
		h.Span(h.Class("dot-loading-label"), g.Text(label)),
	)
}

// dotMatrix is the 3x3 matrix alone (nine dots; the sweep is in site.css).
// The Confirming phrase uses it directly, beside the phrase.
func dotMatrix() g.Node {
	dots := make([]g.Node, 9)
	for i := range dots {
		dots[i] = h.I()
	}
	return h.Span(h.Class("dot-matrix"), g.Attr("aria-hidden", "true"), g.Group(dots))
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
		h.Div(h.Class("loading-phrase-row"), dotMatrix(), phraseNode(phrase)),
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
	attrs = append(attrs, h.Div(h.Class("loading-phrase-row"), dotMatrix(), phraseNode(phrase)))

	return g.Group([]g.Node{
		h.Link(h.Rel("stylesheet"), h.Href(Style.URL())),
		h.Div(attrs...),
	})
}
