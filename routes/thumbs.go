package routes

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/thumb"
	"github.com/oxri/oxri-go/views/journals"
)

// attachThumbs gives each entry whose media is a linked image, a GIF or a
// YouTube or Vimeo video its server-made still (internal/thumb), so the card
// shows the media's look without the reader's browser fetching the media
// (HIG §1.7). Entries whose still is not made yet keep the browser-side tile
// this time, and the filler is asked for one in the background.
func attachThumbs(ctx context.Context, store *db.Store, filler *thumb.Filler, items []journals.RenderedEntry) {
	if store == nil {
		return
	}
	var urls []string
	for _, it := range items {
		if it.Tweet == nil && it.Passage == nil {
			if kind, _ := thumb.Source(it.Entry.MediaURL); kind != "" {
				urls = append(urls, it.Entry.MediaURL)
			}
		}
	}
	if len(urls) == 0 {
		return
	}
	have, err := store.Thumbs(ctx, urls)
	if err != nil {
		return
	}
	for i := range items {
		t, ok := have[items[i].Entry.MediaURL]
		if !ok || t.Status != db.ThumbOK || items[i].Tweet != nil || items[i].Passage != nil {
			continue
		}
		kind, _ := thumb.Source(items[i].Entry.MediaURL)
		items[i].Thumb = &journals.Thumb{
			URL:  fmt.Sprintf("/thumb/%s.jpg?v=%d", t.Key, t.FetchedAt.UnixMilli()),
			Kind: kind, Width: t.Width, Height: t.Height,
		}
	}
	filler.Want(urls, have)
}

var thumbKeyPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

// handleThumb serves a stored still. Its URL carries the time it was made
// (?v=), so a current one is cached for good; a re-made still gets a new URL.
func handleThumb(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		if store == nil || !thumbKeyPattern.MatchString(key) {
			http.NotFound(w, r)
			return
		}
		body, err := store.ThumbBody(r.Context(), key)
		if err != nil || len(body) == 0 {
			http.NotFound(w, r)
			return
		}
		if _, err := strconv.ParseInt(r.URL.Query().Get("v"), 10, 64); err == nil {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}
}

// liveMediaURLs lists the media links some entry still has, so the filler can
// prune stills of the rest.
func liveMediaURLs(store *db.Store) func(context.Context) (map[string]bool, error) {
	return func(ctx context.Context) (map[string]bool, error) {
		entries, err := store.List(ctx, true)
		if err != nil {
			return nil, err
		}
		keep := map[string]bool{}
		for _, e := range entries {
			keep[e.MediaURL] = true
		}
		return keep, nil
	}
}

// NewThumbFiller starts the background thumbnail maker for the server.
func NewThumbFiller(store *db.Store) *thumb.Filler {
	if store == nil {
		return nil
	}
	return thumb.NewFiller(store, thumb.NewMaker(), liveMediaURLs(store))
}
