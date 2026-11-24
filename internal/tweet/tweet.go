// Package tweet turns a link to a post on X into a Snapshot the site can draw
// itself (views/tweet), so a journal page shows tweets without X's script.
//
// The data comes from X's public syndication endpoint, the one X's own embeds
// use (Fetch). It is third-party input: Decode keeps only what the tweet box
// draws, caps every length, and keeps image URLs only from X's image host.
package tweet

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ID is a tweet's numeric id. Only ParseURL makes one, so a value is
// always 1–20 ASCII digits and safe in a URL path or a database key.
type ID struct{ s string }

// String is the id's digits.
func (id ID) String() string { return id.s }

// IsZero reports whether id was never set.
func (id ID) IsZero() bool { return id.s == "" }

var digits = regexp.MustCompile(`^[0-9]{1,20}$`)

func newID(s string) (ID, bool) {
	if !digits.MatchString(s) {
		return ID{}, false
	}
	return ID{s}, true
}

var statusPath = regexp.MustCompile(`^/([A-Za-z0-9_]{1,15}|i(?:/web)?)/status(?:es)?/([0-9]{1,20})(?:/|$)`)

// ParseURL finds the tweet id in a link to a post on X (x.com, twitter.com and
// their mobile hosts). ok is false for anything else.
func ParseURL(raw string) (id ID, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ID{}, false
	}
	switch strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") {
	case "x.com", "twitter.com", "mobile.x.com", "mobile.twitter.com":
	default:
		return ID{}, false
	}
	m := statusPath.FindStringSubmatch(u.Path)
	if m == nil {
		return ID{}, false
	}
	return newID(m[2])
}

// MediaKind is what a tweet's attachment is.
type MediaKind string

const (
	Photo MediaKind = "photo"
	Video MediaKind = "video" // video or GIF: URL is its poster frame
)

// Media is one attachment, with its pixel size so the page can reserve the
// space before the image arrives.
type Media struct {
	Kind   MediaKind `json:"kind"`
	URL    string    `json:"url"`
	Width  int       `json:"w"`
	Height int       `json:"h"`
}

// Quoted is the post a tweet quotes: who, and what it says (text only).
type Quoted struct {
	Name   string `json:"name"`
	Handle string `json:"handle"`
	Text   string `json:"text"`
}

// Snapshot is what the tweet box draws. It is stored as JSON.
type Snapshot struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Handle    string    `json:"handle"`
	AvatarURL string    `json:"avatar,omitempty"`
	Created   time.Time `json:"created"`
	Text      string    `json:"text"`
	Media     []Media   `json:"media,omitempty"`
	Quoted    *Quoted   `json:"quoted,omitempty"`
}

// PostURL is the tweet on X.
func (s Snapshot) PostURL() string {
	return "https://x.com/" + url.PathEscape(s.Handle) + "/status/" + s.ID
}

// Limits on what is kept from X's response.
const (
	maxText   = 1200 // characters of tweet text (X allows long posts; the box doesn't)
	maxName   = 80
	maxMedia  = 4
	maxPixels = 10000
)

// imageURL keeps a URL only if it is an https image on X's image host.
func imageURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "pbs.twimg.com" || u.User != nil || u.Port() != "" {
		return "", false
	}
	return u.String(), true
}

var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)

// clip cuts s to at most n characters, on a character boundary.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n])) + "…"
}

// cleanText drops control characters other than newlines and tabs, and
// collapses runs of blank lines.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x200b && r <= 0x200f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return -1
		}
		return r
	}, s)
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// visibleText is a tweet's text as the reader sees it on X: the display range
// only (no trailing media link), t.co links shown as their display form, and
// X's HTML escaping undone (the page escapes it again when drawing).
func visibleText(text string, rangeStartEnd []int, links []rawURL) string {
	r := []rune(text)
	if len(rangeStartEnd) == 2 {
		start, end := rangeStartEnd[0], rangeStartEnd[1]
		if start >= 0 && end <= len(r) && start <= end {
			r = r[start:end]
		}
	}
	s := string(r)
	for _, l := range links {
		if l.URL != "" && l.Display != "" {
			s = strings.ReplaceAll(s, l.URL, l.Display)
		}
	}
	return cleanText(html.UnescapeString(s))
}
