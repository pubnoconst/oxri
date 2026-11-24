package tweet

import (
	"errors"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	good := map[string]string{
		"https://x.com/PAHoyeck/status/2098867227583713602":                 "2098867227583713602",
		"https://twitter.com/someone/status/20":                             "20",
		"https://mobile.x.com/a_b/statuses/123/photo/1":                     "123",
		"https://x.com/Truth_En_Love/status/2124269415378161775?a=zt3&s=19": "2124269415378161775",
		"https://www.x.com/i/web/status/42":                                 "42",
		"http://x.com/someone/status/7":                                     "7",
	}
	for raw, want := range good {
		id, ok := ParseURL(raw)
		if !ok || id.String() != want {
			t.Errorf("ParseURL(%q) = %q, %v; want %q", raw, id, ok, want)
		}
	}
	for _, raw := range []string{
		"https://example.com/someone/status/1",
		"https://x.com.evil.example/someone/status/1",
		"https://x.com/someone",
		"https://x.com/someone/status/abc",
		"https://x.com/someone/status/123456789012345678901",
		"javascript:alert(1)//x.com/a/status/1",
		"ftp://x.com/a/status/1",
	} {
		if _, ok := ParseURL(raw); ok {
			t.Errorf("ParseURL(%q) accepted", raw)
		}
	}
}

// The token is the one X's embed script computes (checked against V8's
// Number.prototype.toString(36) for 2,006 ids in the tweet id range).
func TestTokenMatchesXsScript(t *testing.T) {
	for id, want := range map[string]string{
		"2098867227583713602": "535sah84q9h",
		"2124269415378161775": "55dl7l89wfi",
		"2113377241409126407": "54fdc24fh6k",
	} {
		v, _ := newID(id)
		if got := token(v); got != want {
			t.Errorf("token(%s) = %s, want %s", id, got, want)
		}
	}
}

// A synthetic syndication response, shaped like X's.
const sample = `{
  "__typename": "Tweet",
  "id_str": "123",
  "created_at": "2026-09-12T20:11:56.000Z",
  "display_text_range": [0, 71],
  "text": "Fish &amp; chips <b>bold</b>\n\n\n\nsee https://t.co/abc and more text here https://t.co/media",
  "entities": {"urls": [{"url": "https://t.co/abc", "display_url": "example.com/page"}]},
  "user": {"name": "A\u202eWriter", "screen_name": "a_writer", "profile_image_url_https": "https://pbs.twimg.com/profile_images/1/a_normal.jpg"},
  "mediaDetails": [
    {"type": "photo", "media_url_https": "https://pbs.twimg.com/media/one.jpg", "original_info": {"width": 528, "height": 387}},
    {"type": "video", "media_url_https": "https://pbs.twimg.com/ext_tw_video_thumb/2/pu/img/p.jpg", "original_info": {"width": 1280, "height": 720}},
    {"type": "photo", "media_url_https": "https://evil.example/x.jpg", "original_info": {"width": 10, "height": 10}},
    {"type": "photo", "media_url_https": "http://pbs.twimg.com/media/plain.jpg", "original_info": {"width": 10, "height": 10}},
    {"type": "photo", "media_url_https": "https://pbs.twimg.com/media/zero.jpg", "original_info": {"width": 0, "height": 10}}
  ],
  "quoted_tweet": {"id_str": "9", "text": "Quoted words", "display_text_range": [0, 12], "user": {"name": "Q", "screen_name": "quoter"}}
}`

func TestDecodeKeepsOnlyWhatTheBoxDraws(t *testing.T) {
	id, _ := newID("123")
	s, err := Decode(id, []byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if s.Handle != "a_writer" || s.Name != "AWriter" {
		t.Errorf("author: %q %q (bidi controls must be dropped)", s.Name, s.Handle)
	}
	if want := "Fish & chips <b>bold</b>\n\nsee example.com/page and more text here"; s.Text != want {
		t.Errorf("text:\n got %q\nwant %q", s.Text, want)
	}
	if len(s.Media) != 2 || s.Media[0].Kind != Photo || s.Media[1].Kind != Video || s.Media[0].Width != 528 {
		t.Errorf("media: %+v (only https pbs.twimg.com with a size)", s.Media)
	}
	if s.Quoted == nil || s.Quoted.Handle != "quoter" || s.Quoted.Text != "Quoted words" {
		t.Errorf("quoted: %+v", s.Quoted)
	}
	if s.PostURL() != "https://x.com/a_writer/status/123" {
		t.Errorf("post url %q", s.PostURL())
	}

	// Stored and loaded back unchanged.
	body, _ := s.Encode()
	back, ok := Load(body)
	if !ok || back.Text != s.Text || len(back.Media) != 2 {
		t.Errorf("round trip: %+v %v", back, ok)
	}
	// A tampered row loses foreign image hosts.
	tampered := strings.Replace(string(body), "https://pbs.twimg.com/media/one.jpg", "https://evil.example/one.jpg", 1)
	if back, ok := Load([]byte(tampered)); !ok || len(back.Media) != 1 {
		t.Errorf("tampered media kept: %+v", back.Media)
	}
}

func TestDecodeRefusesTheWrongOrMissingPost(t *testing.T) {
	id, _ := newID("123")
	other, _ := newID("124")
	if _, err := Decode(other, []byte(sample)); err == nil {
		t.Error("a response for another post must be refused")
	}
	if _, err := Decode(id, []byte(`{"__typename":"TweetTombstone","tombstone":{}}`)); !errors.Is(err, ErrUnavailable) {
		t.Errorf("tombstone: %v", err)
	}
	bad := strings.Replace(sample, `"a_writer"`, `"bad handle!"`, 1)
	if _, err := Decode(id, []byte(bad)); err == nil {
		t.Error("an invalid handle must be refused")
	}
}

func TestLongTextIsClipped(t *testing.T) {
	long := strings.Repeat("word ", 400)
	got := visibleText(long, nil, nil)
	if c := clip(got, maxText); len([]rune(c)) > maxText+1 || !strings.HasSuffix(c, "…") {
		t.Errorf("clip: %d runes", len([]rune(c)))
	}
}
