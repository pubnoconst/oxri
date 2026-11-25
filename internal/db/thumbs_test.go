package db

import (
	"bytes"
	"context"
	"testing"
)

func TestThumbsStoreServeAndPrune(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	a, b := "https://example.org/a.gif", "https://youtu.be/abcdefg"
	if got, err := store.Thumbs(ctx, []string{a}); err != nil || len(got) != 0 {
		t.Fatalf("empty store: %v %v", got, err)
	}
	jpg := []byte{0xff, 0xd8, 0xff, 1, 2, 3}
	if err := store.PutThumb(ctx, a, ThumbOK, jpg, 680, 510); err != nil {
		t.Fatal(err)
	}
	if err := store.PutThumb(ctx, b, ThumbUnavailable, nil, 0, 0); err != nil {
		t.Fatal(err)
	}
	got, err := store.Thumbs(ctx, []string{a, b, "https://other"})
	if err != nil || len(got) != 2 || got[a].Status != ThumbOK || got[a].Width != 680 || got[b].Status != ThumbUnavailable || got[a].Key != ThumbKey(a) {
		t.Fatalf("Thumbs: %+v %v", got, err)
	}
	if body, _ := store.ThumbBody(ctx, ThumbKey(a)); !bytes.Equal(body, jpg) {
		t.Errorf("ThumbBody: got %v", body)
	}
	if body, _ := store.ThumbBody(ctx, ThumbKey(b)); body != nil {
		t.Errorf("an unavailable thumbnail has no body to serve")
	}

	for _, bad := range []struct {
		url    string
		status ThumbStatus
		body   []byte
		w, h   int
	}{
		{"", ThumbOK, jpg, 1, 1},
		{a, ThumbOK, nil, 1, 1},
		{a, ThumbOK, jpg, 0, 1},
		{a, "maybe", jpg, 1, 1},
		{a, ThumbOK, make([]byte, MaxThumbBytes+1), 1, 1},
	} {
		if err := store.PutThumb(ctx, bad.url, bad.status, bad.body, bad.w, bad.h); err == nil {
			t.Errorf("PutThumb(%q, %q, %d bytes, %dx%d) must be refused", bad.url, bad.status, len(bad.body), bad.w, bad.h)
		}
	}

	if err := store.PruneThumbs(ctx, map[string]bool{b: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Thumbs(ctx, []string{a, b}); len(got) != 1 || got[b].Status != ThumbUnavailable {
		t.Errorf("after prune: %+v", got)
	}
	if body, _ := store.ThumbBody(ctx, ThumbKey(a)); body != nil {
		t.Errorf("a pruned thumbnail must not be served (from the cache either)")
	}
}
