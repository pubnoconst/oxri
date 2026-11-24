// Package tweet draws a post on X as the site's own quiet tweet box: a
// quotation on the card's linen, set in the site's type, with no script from
// X. The data is a snapshot taken once (internal/tweet).
//
// A video or GIF shows its poster with a play mark; tapping it swaps the box
// for X's own embed (journal-media.js), which is the only time X's script
// loads for that post.
package tweet

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/oxri/oxri-go/internal/tweet"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// StyleCSS is the tweet box's stylesheet; views that show tweets fold it into
// their own component stylesheet.
//
//go:embed tweet.css
var StyleCSS string

// Box is the tweet box for s.
func Box(s tweet.Snapshot) g.Node {
	post := s.PostURL()
	return h.Figure(h.Class("tweet-box"), g.Attr("data-tweet-id", s.ID), g.Attr("data-tweet-user", s.Handle),
		h.FigCaption(h.Class("tweet-byline"),
			g.If(s.AvatarURL != "", h.Img(h.Class("tweet-avatar"), h.Src(s.AvatarURL), h.Alt(""),
				h.Width("48"), h.Height("48"), g.Attr("loading", "lazy"), g.Attr("decoding", "async"), g.Attr("referrerpolicy", "no-referrer"))),
			h.Span(h.Class("tweet-name"), g.Text(s.Name)),
			h.Span(h.Class("tweet-handle"), g.Text("@"+s.Handle)),
			h.Span(h.Class("tweet-dot"), g.Attr("aria-hidden", "true"), g.Text("·")),
			h.Time(h.DateTime(s.Created.Format("2006-01-02")), g.Text(s.Created.Format("Jan 2, 2006"))),
		),
		g.If(s.Text != "", h.BlockQuote(h.Class("tweet-text"), g.Attr("cite", post), paragraphs(s.Text))),
		quoted(s.Quoted),
		media(s),
		h.A(h.Class("tweet-source"), h.Href(post), h.Target("_blank"), h.Rel("noopener noreferrer"), g.Text("View on X")),
	)
}

// paragraphs sets blank-line-separated text as paragraphs and single line
// breaks as <br>, all as escaped text.
func paragraphs(text string) g.Node {
	var nodes []g.Node
	for _, para := range strings.Split(text, "\n\n") {
		lines := strings.Split(strings.TrimSpace(para), "\n")
		var parts []g.Node
		for i, line := range lines {
			if i > 0 {
				parts = append(parts, h.Br())
			}
			parts = append(parts, g.Text(line))
		}
		nodes = append(nodes, h.P(parts...))
	}
	return g.Group(nodes)
}

func quoted(q *tweet.Quoted) g.Node {
	if q == nil {
		return nil
	}
	return h.Div(h.Class("tweet-quoted"),
		h.P(h.Class("tweet-quoted-by"),
			h.Span(h.Class("tweet-name"), g.Text(q.Name)),
			h.Span(h.Class("tweet-handle"), g.Text("@"+q.Handle)),
		),
		h.Div(h.Class("tweet-quoted-text"), paragraphs(q.Text)),
	)
}

// media lays out 1–4 attachments. Every image carries its size so the box
// never jumps when it arrives.
func media(s tweet.Snapshot) g.Node {
	if len(s.Media) == 0 {
		return nil
	}
	items := make([]g.Node, 0, len(s.Media))
	for i, m := range s.Media {
		img := h.Img(h.Src(m.URL), h.Alt(mediaAlt(m, i, len(s.Media))),
			h.Width(fmt.Sprint(m.Width)), h.Height(fmt.Sprint(m.Height)),
			g.Attr("loading", "lazy"), g.Attr("decoding", "async"), g.Attr("referrerpolicy", "no-referrer"))
		if m.Kind == tweet.Video {
			items = append(items, h.Button(h.Type("button"), h.Class("tweet-play"), g.Attr("data-tweet-play", ""),
				g.Attr("aria-label", "Play video from X"),
				img,
				h.Span(h.Class("tweet-play-mark"), g.Attr("aria-hidden", "true")),
			))
			continue
		}
		items = append(items, img)
	}
	style := ""
	if len(s.Media) == 1 {
		// A single attachment keeps its own shape (CSS caps the height).
		style = fmt.Sprintf("aspect-ratio: %d / %d", s.Media[0].Width, s.Media[0].Height)
	}
	return h.Div(h.Class(fmt.Sprintf("tweet-media tweet-media-%d", len(s.Media))), g.If(style != "", h.Style(style)), g.Group(items))
}

func mediaAlt(m tweet.Media, i, n int) string {
	what := "Image"
	if m.Kind == tweet.Video {
		what = "Video"
	}
	if n == 1 {
		return what + " from the post"
	}
	return fmt.Sprintf("%s %d of %d from the post", what, i+1, n)
}
