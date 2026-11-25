package journals

import (
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

// The server must know what each link becomes in the browser
// (journal-media.js createMedia) to reserve its space: the same rules.
func TestMediaKindMatchesTheBrowser(t *testing.T) {
	for raw, want := range map[string]mediaKind{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ": mediaVideo,
		"https://youtu.be/dQw4w9WgXcQ":                mediaVideo,
		"https://m.youtube.com/shorts/dQw4w9WgXcQ":    mediaVideo,
		"https://youtube.com/watch?v=x":               mediaLink, // id too short: no embed
		"https://vimeo.com/76979871":                  mediaVideo,
		"https://vimeo.com/channels/staffpicks":       mediaLink,
		"https://x.com/someone/status/1234567890":     mediaTweet,
		"https://twitter.com/i/statuses/1234567890":   mediaTweet,
		"https://x.com/someone":                       mediaLink,
		"https://example.org/clip.MP4":                mediaVideo,
		"https://example.org/talk.mp3":                mediaAudio,
		"https://example.org/photo.jpeg":              mediaImage,
		"https://example.org/photo.png?size=large":    mediaImage, // the query is not the path
		"https://example.org/article":                 mediaLink,
		"https://example.org/photo.png/view":          mediaLink,
	} {
		if got := classifyMedia(raw); got != want {
			t.Errorf("classifyMedia(%q) = %q, want %q", raw, got, want)
		}
	}
}

// A plain link's card is drawn by the server, so it is in place when the
// page arrives; other media get a slot sized by their kind.
func TestMediaSlotReservesItsSpace(t *testing.T) {
	render := func(url string) string {
		var b strings.Builder
		_ = mediaSlot(RenderedEntry{Entry: db.Journal{MediaURL: url}}).Render(&b)
		return b.String()
	}
	link := render("https://www.example.org/article")
	for _, want := range []string{`is-link`, `data-media-mounted=`, `tweet-fallback-card`, `Open on example.org`, `rel="noopener noreferrer"`} {
		if !strings.Contains(link, want) {
			t.Errorf("link slot lacks %s: %s", want, link)
		}
	}
	for url, class := range map[string]string{
		"https://youtu.be/dQw4w9WgXcQ":   "is-video",
		"https://example.org/a.mp3":      "is-audio",
		"https://example.org/photo.webp": "is-image",
	} {
		if out := render(url); !strings.Contains(out, class) || strings.Contains(out, "data-media-mounted") {
			t.Errorf("%s: want an empty %s slot for the browser to fill: %s", url, class, out)
		}
	}
}
