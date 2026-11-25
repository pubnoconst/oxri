package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/tweet"
)

// A tweet with a snapshot is drawn by the server (the site's own box, no
// X script, marked mounted so the browser leaves it); one without stays an
// empty slot for X's embed. Third-party text is escaped.
func TestTweetsDrawnFromSnapshots(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	authSvc, sess := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	router := NewRouter(Config{AuthService: authSvc, Store: store})

	post := func(media string) {
		form := url.Values{"media_url": {media}, "tags": {"Log"}, "body": {"Entry."}, "published": {"1"}}
		req := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(sess)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("create: %d", rec.Code)
		}
	}
	const withSnap = "https://x.com/a_writer/status/111"
	const without = "https://x.com/other/status/222"
	post(withSnap)
	post(without)

	snap := tweet.Snapshot{
		ID: "111", Name: "A <Writer>", Handle: "a_writer",
		AvatarURL: "https://pbs.twimg.com/profile_images/1/a_normal.jpg",
		Created:   time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		Text:      "First <script>alert(1)</script>\n\nSecond",
		Media:     []tweet.Media{{Kind: tweet.Video, URL: "https://pbs.twimg.com/media/p.jpg", Width: 1280, Height: 720}},
	}
	body, _ := snap.Encode()
	if err := store.PutTweetSnapshot(ctx, "111", db.TweetOK, body); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/journals", nil))
	page := rec.Body.String()

	for _, want := range []string{
		`data-media-mounted="` + withSnap + `"`,
		`class="tweet-box" data-tweet-id="111" data-tweet-user="a_writer"`,
		`A &lt;Writer&gt;`,
		`First &lt;script&gt;alert(1)&lt;/script&gt;`,
		`<p>Second</p>`,
		`width="1280" height="720"`,
		`data-tweet-play`,
		`referrerpolicy="no-referrer"`,
		`href="https://x.com/a_writer/status/111"`,
		`<div class="journal-media-slot journal-preview-media" data-media-url="` + without + `"></div>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("catalog missing %q", want)
		}
	}
	if strings.Contains(page, "<script>alert") {
		t.Error("tweet text must be escaped")
	}
	if strings.Contains(page, "platform.twitter.com") {
		t.Error("the catalog must not load X's script itself")
	}
}
