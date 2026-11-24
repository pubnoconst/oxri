package routes

import (
	"context"
	"net/http"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/tweet"
	"github.com/oxri/oxri-go/views/journals"
)

// attachTweets gives each entry that links to a post on X the site's own
// drawing of it (views/tweet), from its cached snapshot. Entries whose post
// has no snapshot yet keep X's embed this time, and the filler is asked for
// one in the background, so the page never waits on X.
func attachTweets(ctx context.Context, store *db.Store, filler *tweet.Filler, items []journals.RenderedEntry) {
	var ids []tweet.ID
	at := map[string][]int{}
	for i, it := range items {
		if id, ok := tweet.ParseURL(it.Entry.MediaURL); ok {
			if _, seen := at[id.String()]; !seen {
				ids = append(ids, id)
			}
			at[id.String()] = append(at[id.String()], i)
		}
	}
	if len(ids) == 0 {
		return
	}
	snaps, stored := tweet.Resolve(ctx, store, ids)
	for key, s := range snaps {
		for _, i := range at[key] {
			snap := s
			items[i].Tweet = &snap
		}
	}
	filler.Want(ids, stored)
}

// liveTweetIDs lists the posts some entry still links to, so the filler can
// prune snapshots of the rest.
func liveTweetIDs(store *db.Store) func(context.Context) (map[string]bool, error) {
	return func(ctx context.Context) (map[string]bool, error) {
		entries, err := store.List(ctx, true)
		if err != nil {
			return nil, err
		}
		keep := map[string]bool{}
		for _, e := range entries {
			if id, ok := tweet.ParseURL(e.MediaURL); ok {
				keep[id.String()] = true
			}
		}
		return keep, nil
	}
}

// NewTweetFiller starts the background snapshot fetcher for the server.
func NewTweetFiller(store *db.Store, client *http.Client) *tweet.Filler {
	if store == nil {
		return nil
	}
	return tweet.NewFiller(store, client, liveTweetIDs(store))
}
