// Package loading provides the Gomponents awaiting-confirmation state ("Confirming, please wait.").
package loading

import (
	_ "embed"
	"fmt"

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

// CharToken represents a single rendered character in the magnifying-wave phrase.
type CharToken struct {
	Index   int
	Text    string
	IsSpace bool
}

// SplitPhrase splits a phrase into individual character tokens preserving word spaces.
func SplitPhrase(phrase string) []CharToken {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	runes := []rune(phrase)
	out := make([]CharToken, 0, len(runes))
	for i, r := range runes {
		if r == ' ' {
			out = append(out, CharToken{Index: i, Text: "\u00A0", IsSpace: true})
		} else {
			out = append(out, CharToken{Index: i, Text: string(r), IsSpace: false})
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
			h.Style(fmt.Sprintf("--i: %d;", ch.Index)),
			g.Attr("aria-hidden", "true"),
			g.Text(ch.Text),
		))
	}
	return nodes
}

// Page renders the standalone awaiting confirmation view inside the base layout shell.
func Page(phrase string) g.Node {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	return layout.Doc(
		"RES IMPERII",
		StylePath+"?v=1",
		Stage(phrase),
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
		h.P(
			h.Class("loading-phrase"),
			g.Attr("aria-label", phrase),
			g.Group(renderPhraseChars(phrase)),
		),
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
	attrs = append(attrs, h.P(
		h.Class("loading-phrase"),
		g.Attr("aria-label", phrase),
		g.Group(renderPhraseChars(phrase)),
	))

	return g.Group([]g.Node{
		h.Link(h.Rel("stylesheet"), h.Href(StylePath+"?v=1")),
		h.Div(attrs...),
	})
}
