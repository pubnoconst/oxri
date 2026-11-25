package journals

import (
	"net/url"
	"regexp"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// mediaKind is what an entry's media link will become in the browser
// (journal-media.js createMedia), known here so the server can give its slot
// the right space before any script runs (HIG §1.7: no sudden displacement).
// The rules mirror createMedia's; TestMediaKindMatchesTheBrowser pins them.
type mediaKind string

const (
	mediaVideo mediaKind = "video" // YouTube, Vimeo, or a video file: 16:9
	mediaAudio mediaKind = "audio" // an audio file: the player's height
	mediaImage mediaKind = "image" // an image file: a fixed frame, the image fitted inside
	mediaTweet mediaKind = "tweet" // a post on X without a stored snapshot: X's embed, height unknown
	mediaLink  mediaKind = "link"  // anything else: the link card, drawn by the server
)

var (
	youtubeID   = regexp.MustCompile(`^[\w-]{6,}$`)
	vimeoID     = regexp.MustCompile(`^\d+$`)
	tweetStatus = regexp.MustCompile(`(?i)(?:^|/)(?:status|statuses)/(\d+)`)
	videoFile   = regexp.MustCompile(`(?i)\.(mp4|webm|ogg|mov)$`)
	audioFile   = regexp.MustCompile(`(?i)\.(mp3|wav|m4a|aac|flac)$`)
	imageFile   = regexp.MustCompile(`(?i)\.(avif|gif|jpe?g|png|webp|svg)$`)
)

func classifyMedia(raw string) mediaKind {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return mediaLink
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	path := strings.ToLower(u.Path)
	segments := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	last := ""
	if len(segments) > 0 {
		last = segments[len(segments)-1]
	}
	switch host {
	case "youtu.be":
		if youtubeID.MatchString(strings.TrimPrefix(u.Path, "/")) {
			return mediaVideo
		}
	case "youtube.com", "m.youtube.com":
		id := last
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		}
		if youtubeID.MatchString(id) {
			return mediaVideo
		}
	case "vimeo.com", "player.vimeo.com":
		if vimeoID.MatchString(last) {
			return mediaVideo
		}
	case "twitter.com", "x.com", "mobile.twitter.com", "mobile.x.com":
		if tweetStatus.MatchString(u.Path) {
			return mediaTweet
		}
	}
	switch {
	case videoFile.MatchString(path):
		return mediaVideo
	case audioFile.MatchString(path):
		return mediaAudio
	case imageFile.MatchString(path):
		return mediaImage
	}
	return mediaLink
}

// linkCard is the card for a link with no inline media, the same one
// journal-media.js draws for media that fails to load (createLinkFallback),
// drawn here so it is in place when the page arrives.
func linkCard(raw string) g.Node {
	site := raw
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		site = strings.TrimPrefix(u.Hostname(), "www.")
	}
	return h.Div(h.Class("tweet-fallback-card"),
		h.Span(h.Class("tweet-fallback-badge"), g.Text("External Link")),
		h.A(h.Href(raw), h.Target("_blank"), h.Rel("noopener noreferrer"), h.Title(raw), g.Text("Open on "+site)),
	)
}
