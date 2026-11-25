package journals

import (
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/tweet"
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
		_ = mediaSlot(RenderedEntry{Entry: db.Journal{MediaURL: url}}, true).Render(&b)
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

// A catalog card asks X for its small images; an entry's own page for
// medium (a third of the bytes in a card: RATIONALE 084).
func TestClippingImagesAreSizedForWhereTheyShow(t *testing.T) {
	snap := tweet.Snapshot{ID: "1", Name: "A", Handle: "a", Text: "x",
		Media: []tweet.Media{{Kind: tweet.Photo, URL: "https://pbs.twimg.com/media/abc.jpg", Width: 1200, Height: 900}}}
	render := func(collage bool) string {
		var b strings.Builder
		_ = mediaSlot(RenderedEntry{Entry: db.Journal{MediaURL: "https://x.com/a/status/1"}, Tweet: &snap}, collage).Render(&b)
		return b.String()
	}
	if card := render(true); !strings.Contains(card, `src="https://pbs.twimg.com/media/abc.jpg?name=small"`) {
		t.Errorf("a catalog card must ask for X's small image: %s", card)
	}
	if page := render(false); !strings.Contains(page, `src="https://pbs.twimg.com/media/abc.jpg?name=medium"`) {
		t.Errorf("an entry page must ask for X's medium image: %s", page)
	}
}

// A card shows the server's still: a linked image as the still itself, a GIF
// or video as a drawn tap tile showing it; the media's own host is not asked.
func TestCardsShowTheServersStill(t *testing.T) {
	render := func(mediaURL string, th *Thumb, collage bool) string {
		var b strings.Builder
		_ = mediaSlot(RenderedEntry{Entry: db.Journal{MediaURL: mediaURL}, Thumb: th}, collage).Render(&b)
		return b.String()
	}
	still := "/thumb/0123456789abcdef01234567.jpg?v=1"
	img := render("https://example.org/a.png", &Thumb{URL: still, Kind: "image", Width: 680, Height: 510}, true)
	if !strings.Contains(img, `<img src="`+still+`"`) || !strings.Contains(img, `data-media-mounted=`) || strings.Contains(img, "example.org/a.png\" alt") {
		t.Errorf("a card's linked image must be the server's still, drawn and mounted: %s", img)
	}
	if page := render("https://example.org/a.png", &Thumb{URL: still, Kind: "image", Width: 680, Height: 510}, false); strings.Contains(page, still) {
		t.Errorf("an entry's own page shows the image itself: %s", page)
	}
	for _, u := range []string{"https://example.org/a.gif", "https://youtu.be/dQw4w9WgXcQ"} {
		kind := "gif"
		if strings.Contains(u, "youtu") {
			kind = "video"
		}
		for _, collage := range []bool{true, false} {
			out := render(u, &Thumb{URL: still, Kind: kind, Width: 680, Height: 383}, collage)
			if !strings.Contains(out, `<button type="button" class="media-facade" data-media-play`) || !strings.Contains(out, `<img src="`+still+`"`) ||
				!strings.Contains(out, `data-media-still="`+still+`"`) || strings.Contains(out, "<iframe") {
				t.Errorf("%s (card %v) must be a drawn tap tile showing the still, with no player: %s", u, collage, out)
			}
		}
	}
}
