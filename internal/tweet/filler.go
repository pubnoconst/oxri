package tweet

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/db"
)

// Filler fetches missing snapshots in the background, one at a time, so a
// page never waits on X: a tweet without a snapshot is drawn by X's embed
// this time and by the site's own box from the next view on.
//
// It is polite and bounded: one request at a time with a pause between them,
// a short queue (extra requests are dropped and asked for again on a later
// view), and a back-off for posts that failed or that X says are unavailable.
type Filler struct {
	store  *db.Store
	client *http.Client
	live   func(context.Context) (map[string]bool, error)

	queue chan ID

	mu       sync.Mutex
	queued   map[string]bool
	retryAt  map[string]time.Time
	prunedAt time.Time
}

const (
	fetchTimeout     = 8 * time.Second
	saveWait         = 4 * time.Second // the most a save waits for X (Now)
	fetchPause       = 600 * time.Millisecond
	failureBackoff   = 15 * time.Minute
	unavailableAfter = 7 * 24 * time.Hour // ask X again about a missing post
	pruneEvery       = 30 * time.Minute
	queueSize        = 64
)

// NewFiller starts the background fetcher. live lists the tweet ids some
// entry still links to (snapshots of the rest are pruned).
func NewFiller(store *db.Store, client *http.Client, live func(context.Context) (map[string]bool, error)) *Filler {
	f := &Filler{
		store:   store,
		client:  client,
		live:    live,
		queue:   make(chan ID, queueSize),
		queued:  map[string]bool{},
		retryAt: map[string]time.Time{},
	}
	go f.run()
	return f
}

// Want asks for snapshots of ids that have none (or whose post was
// unavailable long enough ago to ask again). It never blocks. A nil Filler
// (tests, or no network) does nothing.
func (f *Filler) Want(ids []ID, have map[string]db.StoredTweet) {
	if f == nil {
		return
	}
	now := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		key := id.String()
		if t, ok := have[key]; ok {
			if t.Status == db.TweetOK || now.Sub(t.FetchedAt) < unavailableAfter {
				continue
			}
		}
		if f.queued[key] || now.Before(f.retryAt[key]) {
			continue
		}
		select {
		case f.queue <- id:
			f.queued[key] = true
		default:
			return // full: asked for again on a later view
		}
	}
}

func (f *Filler) run() {
	for id := range f.queue {
		f.fill(id)
		f.mu.Lock()
		delete(f.queued, id.String())
		idle := len(f.queue) == 0
		f.mu.Unlock()
		if idle {
			f.maybePrune()
		}
		time.Sleep(fetchPause)
	}
}

// Now fetches the snapshot of the post rawURL links to, if it is a post on
// X with none stored, and waits for it (up to saveWait). A save calls it, so
// the page it leads to draws the new entry's post in the site's own box
// rather than X's embed. On failure or timeout nothing changes: the page
// falls back to the embed and the background fetch tries again later.
func (f *Filler) Now(ctx context.Context, rawURL string) {
	if f == nil {
		return
	}
	id, ok := ParseURL(rawURL)
	if !ok {
		return
	}
	_, stored := Resolve(ctx, f.store, []ID{id})
	key := id.String()
	if t, ok := stored[key]; ok && (t.Status == db.TweetOK || time.Since(t.FetchedAt) < unavailableAfter) {
		return
	}
	f.mu.Lock()
	wait := time.Now().Before(f.retryAt[key])
	f.mu.Unlock()
	if wait {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, saveWait)
	defer cancel()
	f.fillCtx(ctx, id)
}

func (f *Filler) fill(id ID) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	f.fillCtx(ctx, id)
}

func (f *Filler) fillCtx(ctx context.Context, id ID) {
	snap, err := Fetch(ctx, f.client, id)
	switch {
	case errors.Is(err, ErrUnavailable):
		_ = f.store.PutTweetSnapshot(ctx, id.String(), db.TweetUnavailable, nil)
	case err != nil:
		log.Printf("[TWEET] fetch %s failed: %v", id, err)
		f.mu.Lock()
		f.retryAt[id.String()] = time.Now().Add(failureBackoff)
		f.mu.Unlock()
	default:
		body, encErr := snap.Encode()
		if encErr == nil {
			encErr = f.store.PutTweetSnapshot(ctx, id.String(), db.TweetOK, body)
		}
		if encErr != nil {
			log.Printf("[TWEET] store %s failed: %v", id, encErr)
		}
	}
}

// maybePrune drops snapshots no entry links to, at most every pruneEvery.
func (f *Filler) maybePrune() {
	f.mu.Lock()
	if time.Since(f.prunedAt) < pruneEvery || f.live == nil {
		f.mu.Unlock()
		return
	}
	f.prunedAt = time.Now()
	f.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	keep, err := f.live(ctx)
	if err != nil {
		return
	}
	if err := f.store.PruneTweetSnapshots(ctx, keep); err != nil {
		log.Printf("[TWEET] prune failed: %v", err)
	}
}

// Resolve returns the drawable snapshots among ids, and the stored records
// (for Want).
func Resolve(ctx context.Context, store *db.Store, ids []ID) (map[string]Snapshot, map[string]db.StoredTweet) {
	if store == nil || len(ids) == 0 {
		return nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = id.String()
	}
	stored, err := store.TweetSnapshots(ctx, keys)
	if err != nil {
		return nil, nil
	}
	out := make(map[string]Snapshot, len(stored))
	for id, t := range stored {
		if t.Status != db.TweetOK {
			continue
		}
		if s, ok := Load(t.Body); ok && s.ID == id {
			out[id] = s
		}
	}
	return out, stored
}
