// Package tweet draws a post on X as a universal clipping: a slip of paper laid
// on the card (depth 2) with the author anchored at the top-left and three
// layouts that grow strictly with what the post holds (HIG §4):
//
//   - Caption (media and <= 80 characters, 1C): full-bleed media; the author
//     pill and caption stay hidden at rest and ease in on hover or focus.
//   - Snippet (media and > 80 characters): top-left author, compact square
//     thumbnail on the left, statement text on the right.
//   - Statement (no media): top-left author, full-width statement below.
//
// No script from X loads unless the reader taps a video's play mark
// (journal-media.js).
package tweet

import (
	_ "embed"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/oxri/oxri-go/internal/tweet"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// StyleCSS is the tweet box's stylesheet; views that show tweets fold it into
// their own component stylesheet.
//
//go:embed tweet.css
var StyleCSS string

// ImageSize is which of X's sizes a clipping's images are fetched at.
type ImageSize string

const (
	// Small (680 px wide) for the catalog's cards: a fraction of the bytes
	// and decoding work, and sharp at a card's size.
	Small ImageSize = "small"
	// Medium (1200 px wide) for an entry on its own page.
	Medium ImageSize = "medium"
)

// MaxCaptionRunes separates a short image caption (<= 80 characters, overlaid
// on full-bleed media) from a rich statement paired with media (> 80 characters).
const MaxCaptionRunes = 80

// Archetype is which of the three universal clipping shapes a post takes.
type Archetype string

const (
	Caption   Archetype = "caption"
	Snippet   Archetype = "snippet"
	Statement Archetype = "statement"
)

// Classify returns the clipping archetype for s.
func Classify(s tweet.Snapshot) Archetype {
	chars := utf8.RuneCountInString(strings.TrimSpace(s.Text))
	if len(s.Media) > 0 {
		if chars <= MaxCaptionRunes {
			return Caption
		}
		return Snippet
	}
	return Statement
}

// sized asks X's image server for one of its sizes (its "name" parameter).
func sized(raw string, size ImageSize) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("name", string(size))
	u.RawQuery = q.Encode()
	return u.String()
}

func clampAspectRatio(w, hgt int) string {
	if w <= 0 || hgt <= 0 {
		return "aspect-ratio: 4 / 3"
	}
	r := float64(w) / float64(hgt)
	if r < 0.85 {
		return "aspect-ratio: 85 / 100"
	}
	if r > 1.85 {
		return "aspect-ratio: 185 / 100"
	}
	return fmt.Sprintf("aspect-ratio: %d / %d", w, hgt)
}

func avatarInitial(s tweet.Snapshot) string {
	src := strings.TrimSpace(s.Name)
	if src == "" {
		src = strings.TrimSpace(s.Handle)
	}
	if src == "" {
		return "X"
	}
	r, _ := utf8.DecodeRuneInString(src)
	return strings.ToUpper(string(r))
}

func authorRow(s tweet.Snapshot, postURL string) g.Node {
	var avatar g.Node
	if s.AvatarURL != "" {
		avatar = h.Img(h.Class("tweet-avatar"), h.Src(s.AvatarURL), h.Alt(""),
			h.Width("48"), h.Height("48"), g.Attr("loading", "lazy"), g.Attr("decoding", "async"), g.Attr("referrerpolicy", "no-referrer"))
	} else {
		avatar = h.Span(h.Class("tweet-avatar-fallback"), g.Attr("aria-hidden", "true"), g.Text(avatarInitial(s)))
	}
	return h.Div(h.Class("tweet-author-row"),
		avatar,
		h.Div(h.Class("tweet-identity"),
			h.Span(h.Class("tweet-name"), g.Text(s.Name)),
			h.Span(h.Class("tweet-handle"), g.Text("@"+s.Handle)),
		),
		h.A(h.Class("tweet-ext link-quiet"), h.Href(postURL), h.Target("_blank"), h.Rel("noopener noreferrer"),
			g.Attr("aria-label", "View post on X"), g.Text("↗")),
	)
}

func mediaNode(m tweet.Media, size ImageSize, idx, total int, smallPlay bool) g.Node {
	img := h.Img(h.Src(sized(m.URL, size)), h.Alt(mediaAlt(m, idx, total)),
		h.Width(fmt.Sprint(m.Width)), h.Height(fmt.Sprint(m.Height)),
		g.Attr("loading", "lazy"), g.Attr("decoding", "async"), g.Attr("referrerpolicy", "no-referrer"))
	if m.Kind == tweet.Video {
		markCls := "tweet-play-mark"
		if smallPlay {
			markCls += " is-sm"
		}
		return h.Button(h.Type("button"), h.Class("tweet-play"), g.Attr("data-tweet-play", ""),
			g.Attr("aria-label", "Play video from X"),
			img,
			h.Span(h.Class(markCls), g.Attr("aria-hidden", "true")),
		)
	}
	return img
}

// Box is the universal clipping for s, its images fetched at size.
func Box(s tweet.Snapshot, size ImageSize) g.Node {
	post := s.PostURL()
	arch := Classify(s)
	trimmed := strings.TrimSpace(s.Text)

	switch arch {
	case Caption:
		first := s.Media[0]
		var countBadge g.Node
		if len(s.Media) > 1 {
			countBadge = h.Span(h.Class("tweet-media-count"), g.Textf("1/%d", len(s.Media)))
		}
		var overlay g.Node
		if trimmed != "" {
			overlay = h.FigCaption(h.Class("tweet-caption-overlay"), paragraphs(trimmed))
		}
		return h.Figure(
			h.Class("tweet-box is-caption"),
			g.Attr("data-tweet-id", s.ID),
			g.Attr("data-tweet-user", s.Handle),
			g.Attr("tabindex", "0"),
			authorRow(s, post),
			h.Div(h.Class("tweet-caption-stage"), h.Style(clampAspectRatio(first.Width, first.Height)),
				mediaNode(first, size, 0, len(s.Media), false),
				countBadge,
				overlay,
			),
		)

	case Snippet:
		first := s.Media[0]
		var countBadge g.Node
		if len(s.Media) > 1 {
			countBadge = h.Span(h.Class("tweet-media-count"), g.Textf("1/%d", len(s.Media)))
		}
		return h.Figure(
			h.Class("tweet-box is-snippet"),
			g.Attr("data-tweet-id", s.ID),
			g.Attr("data-tweet-user", s.Handle),
			authorRow(s, post),
			h.Div(h.Class("tweet-snippet-row"),
				h.Div(h.Class("tweet-snippet-thumb"),
					mediaNode(first, size, 0, len(s.Media), true),
					countBadge,
				),
				h.BlockQuote(h.Class("tweet-text"), g.Attr("cite", post), paragraphs(trimmed)),
			),
			quoted(s.Quoted),
		)

	default:
		return h.Figure(
			h.Class("tweet-box is-statement"),
			g.Attr("data-tweet-id", s.ID),
			g.Attr("data-tweet-user", s.Handle),
			authorRow(s, post),
			g.If(trimmed != "", h.BlockQuote(h.Class("tweet-text"), g.Attr("cite", post), paragraphs(trimmed))),
			quoted(s.Quoted),
		)
	}
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
