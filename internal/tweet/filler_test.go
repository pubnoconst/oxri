package tweet

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

// fakeX answers every request with the sample post and counts them.
type fakeX struct{ calls atomic.Int32 }

func (f *fakeX) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls.Add(1)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(sample)),
	}, nil
}

// A save fetches the new entry's post before it redirects, so the page after
// it draws the site's own box, not X's embed; once stored, it is not fetched
// again.
func TestNowStoresTheSnapshotBeforeTheSaveReturns(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	x := &fakeX{}
	f := NewFiller(store, &http.Client{Transport: x}, nil)

	f.Now(ctx, "https://example.com/picture.png")
	if x.calls.Load() != 0 {
		t.Fatal("a link that is not a post on X must not reach X")
	}

	f.Now(ctx, "https://x.com/a_writer/status/123")
	got, err := store.TweetSnapshots(ctx, []string{"123"})
	if err != nil || got["123"].Status != db.TweetOK {
		t.Fatalf("snapshot not stored on return: %+v %v", got, err)
	}

	f.Now(ctx, "https://x.com/a_writer/status/123")
	if n := x.calls.Load(); n != 1 {
		t.Errorf("X asked %d times; a stored post is not fetched again", n)
	}

	var nilFiller *Filler
	nilFiller.Now(ctx, "https://x.com/a_writer/status/123") // no panic
}
