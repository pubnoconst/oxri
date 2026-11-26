// Package passage draws a Bible passage an entry quotes as a universal
// clipping: the same slip of paper as a post from X (views/tweet), with the
// reference anchored at the top-left, the version on the right, and the verses
// directly below. The text is stored plain and set here as escaped text.
//
// scripture-picker.js draws the editor's preview with the same classes.
package passage

import (
	_ "embed"

	"github.com/oxri/oxri-go/internal/domain"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// StyleCSS is the passage clipping's own rules; views that show passages
// fold it into their component stylesheet after views/tweet's.
//
//go:embed passage.css
var StyleCSS string

// Box is the clipping for p.
func Box(p domain.Passage) g.Node {
	return h.Figure(h.Class("tweet-box passage-box"),
		h.Div(h.Class("passage-head"),
			h.Span(h.Class("passage-reference"), g.Text(p.Reference)),
			h.Span(h.Class("passage-version"), g.Text(p.Version)),
		),
		h.BlockQuote(h.Class("tweet-text passage-text"), h.P(g.Text(p.Text))),
	)
}
