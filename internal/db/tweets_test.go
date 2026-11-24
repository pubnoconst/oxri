package db

import (
	"context"
	"strings"
	"testing"
)

func TestTweetSnapshotCache(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if got, err := store.TweetSnapshots(ctx, []string{"1"}); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	if err := store.PutTweetSnapshot(ctx, "1", TweetOK, []byte(`{"id":"1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.PutTweetSnapshot(ctx, "2", TweetUnavailable, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := store.TweetSnapshots(ctx, []string{"1", "2", "3"})
	if len(got) != 2 || got["1"].Status != TweetOK || string(got["1"].Body) != `{"id":"1"}` || got["2"].Status != TweetUnavailable {
		t.Fatalf("read back: %+v", got)
	}

	// Rejected before reaching the table.
	for _, bad := range []struct {
		id     string
		status TweetStatus
		body   string
	}{
		{"1a", TweetOK, "{}"},
		{"", TweetOK, "{}"},
		{"1", "maybe", "{}"},
		{"1", TweetOK, strings.Repeat("x", maxTweetBody+1)},
	} {
		if err := store.PutTweetSnapshot(ctx, bad.id, bad.status, []byte(bad.body)); err == nil {
			t.Errorf("accepted %q %q (%d bytes)", bad.id, bad.status, len(bad.body))
		}
	}

	// Pruning keeps only the tweets entries still link to, in the table too.
	if err := store.PruneTweetSnapshots(ctx, map[string]bool{"2": true}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tweet_snapshots`).Scan(&n); err != nil || n != 1 {
		t.Errorf("rows after prune: %d %v", n, err)
	}
	if got, _ := store.TweetSnapshots(ctx, []string{"1", "2"}); len(got) != 1 || got["2"].Status != TweetUnavailable {
		t.Errorf("cache after prune: %+v", got)
	}
}
