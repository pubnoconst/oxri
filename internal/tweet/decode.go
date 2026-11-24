package tweet

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrUnavailable means X has no public post for the id (deleted, private,
// age-gated or suspended). The page keeps X's own embed for it.
var ErrUnavailable = errors.New("tweet: post unavailable")

// The parts of the syndication response the box uses.
type rawURL struct {
	URL     string `json:"url"`
	Display string `json:"display_url"`
}

type rawUser struct {
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
	Avatar     string `json:"profile_image_url_https"`
}

type rawMedia struct {
	Type     string `json:"type"` // photo, video, animated_gif
	URL      string `json:"media_url_https"`
	Original struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"original_info"`
}

type rawTweet struct {
	Typename     string `json:"__typename"`
	IDStr        string `json:"id_str"`
	Text         string `json:"text"`
	CreatedAt    string `json:"created_at"`
	DisplayRange []int  `json:"display_text_range"`
	Entities     struct {
		URLs []rawURL `json:"urls"`
	} `json:"entities"`
	User         rawUser     `json:"user"`
	MediaDetails []rawMedia  `json:"mediaDetails"`
	Quoted       *rawTweet   `json:"quoted_tweet"`
	Tombstone    interface{} `json:"tombstone"`
}

// Decode turns X's response for id into a Snapshot, keeping only what the
// box draws and checking all of it.
func Decode(id ID, body []byte) (Snapshot, error) {
	var t rawTweet
	if err := json.Unmarshal(body, &t); err != nil {
		return Snapshot{}, err
	}
	if t.Typename == "TweetTombstone" || t.Tombstone != nil || t.IDStr == "" {
		return Snapshot{}, ErrUnavailable
	}
	if t.IDStr != id.String() {
		return Snapshot{}, errors.New("tweet: response is for a different post")
	}
	handle := t.User.ScreenName
	if !handlePattern.MatchString(handle) {
		return Snapshot{}, errors.New("tweet: invalid author handle")
	}
	created, err := time.Parse(time.RFC3339, t.CreatedAt)
	if err != nil {
		return Snapshot{}, errors.New("tweet: invalid date")
	}

	s := Snapshot{
		ID:      id.String(),
		Name:    clip(cleanText(t.User.Name), maxName),
		Handle:  handle,
		Created: created.UTC(),
		Text:    clip(visibleText(t.Text, t.DisplayRange, t.Entities.URLs), maxText),
	}
	if s.Name == "" {
		s.Name = handle
	}
	if avatar, ok := imageURL(t.User.Avatar); ok {
		s.AvatarURL = avatar
	}
	for _, m := range t.MediaDetails {
		if len(s.Media) == maxMedia {
			break
		}
		u, ok := imageURL(m.URL)
		w, h := m.Original.Width, m.Original.Height
		if !ok || w <= 0 || h <= 0 || w > maxPixels || h > maxPixels {
			continue
		}
		kind := Photo
		if m.Type == "video" || m.Type == "animated_gif" {
			kind = Video
		} else if m.Type != "photo" {
			continue
		}
		s.Media = append(s.Media, Media{Kind: kind, URL: u, Width: w, Height: h})
	}
	if q := t.Quoted; q != nil && handlePattern.MatchString(q.User.ScreenName) {
		text := clip(visibleText(q.Text, q.DisplayRange, q.Entities.URLs), maxText/2)
		if text != "" {
			s.Quoted = &Quoted{Name: clip(cleanText(q.User.Name), maxName), Handle: q.User.ScreenName, Text: text}
		}
	}
	if s.Text == "" && len(s.Media) == 0 {
		return Snapshot{}, ErrUnavailable
	}
	return s, nil
}

// Encode stores a snapshot as JSON.
func (s Snapshot) Encode() ([]byte, error) { return json.Marshal(s) }

// Load reads a stored snapshot, re-checking it (it was written by this code,
// but the database is outside the process).
func Load(body []byte) (Snapshot, bool) {
	var s Snapshot
	if err := json.Unmarshal(body, &s); err != nil {
		return Snapshot{}, false
	}
	if _, ok := newID(s.ID); !ok || !handlePattern.MatchString(s.Handle) {
		return Snapshot{}, false
	}
	if s.AvatarURL != "" {
		if _, ok := imageURL(s.AvatarURL); !ok {
			s.AvatarURL = ""
		}
	}
	kept := s.Media[:0]
	for _, m := range s.Media {
		if _, ok := imageURL(m.URL); ok && m.Width > 0 && m.Height > 0 && (m.Kind == Photo || m.Kind == Video) {
			kept = append(kept, m)
		}
	}
	s.Media = kept
	if strings.TrimSpace(s.Text) == "" && len(s.Media) == 0 {
		return Snapshot{}, false
	}
	return s, true
}
