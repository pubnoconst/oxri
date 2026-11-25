package thumb

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/db"
)

// Filler makes missing thumbnails in the background, one at a time, as
// tweet.Filler does for tweet snapshots: a page never waits on another site.
// A card whose thumbnail is not made yet falls back to the browser-side tile
// (journal-media.js) this time.
type Filler struct {
	store *db.Store
	maker *Maker
	live  func(context.Context) (map[string]bool, error)

	queue chan string

	mu       sync.Mutex
	queued   map[string]bool
	retryAt  map[string]time.Time
	prunedAt time.Time
}

const (
	queueSize        = 64
	fetchPause       = 400 * time.Millisecond
	failureBackoff   = 30 * time.Minute
	unavailableAfter = 7 * 24 * time.Hour // try a failed link again after a week
	pruneEvery       = 30 * time.Minute
	saveWait         = 5 * time.Second // the most a save waits for a thumbnail (Now)
)

// NewFiller starts the background maker. live lists the media links some
// entry still has (thumbnails of the rest are pruned).
func NewFiller(store *db.Store, maker *Maker, live func(context.Context) (map[string]bool, error)) *Filler {
	f := &Filler{store: store, maker: maker, live: live, queue: make(chan string, queueSize),
		queued: map[string]bool{}, retryAt: map[string]time.Time{}}
	go f.run()
	return f
}

// due reports whether url needs a thumbnail made (none, or a failed one old
// enough to try again) and is not waiting out a back-off.
func (f *Filler) due(url string, have map[string]db.StoredThumb, now time.Time) bool {
	if kind, _ := Source(url); kind == "" {
		return false
	}
	if t, ok := have[url]; ok && (t.Status == db.ThumbOK || now.Sub(t.FetchedAt) < unavailableAfter) {
		return false
	}
	return !now.Before(f.retryAt[url])
}

// Want asks for thumbnails of urls that have none. It never blocks. A nil
// Filler (tests, or no network) does nothing.
func (f *Filler) Want(urls []string, have map[string]db.StoredThumb) {
	if f == nil {
		return
	}
	now := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range urls {
		if f.queued[u] || !f.due(u, have, now) {
			continue
		}
		select {
		case f.queue <- u:
			f.queued[u] = true
		default:
			return // full: asked for again on a later view
		}
	}
}

// Now makes mediaURL's thumbnail if it has none, waiting at most saveWait:
// a save calls it, so the card it leads to shows the still at once. On
// failure or timeout nothing changes; the background tries again later.
func (f *Filler) Now(ctx context.Context, mediaURL string) {
	if f == nil {
		return
	}
	have, err := f.store.Thumbs(ctx, []string{mediaURL})
	if err != nil {
		return
	}
	f.mu.Lock()
	due := f.due(mediaURL, have, time.Now())
	f.mu.Unlock()
	if !due {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, saveWait)
	defer cancel()
	f.make(ctx, mediaURL)
}

func (f *Filler) run() {
	for u := range f.queue {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTime+2*time.Second)
		f.make(ctx, u)
		cancel()
		f.mu.Lock()
		delete(f.queued, u)
		idle := len(f.queue) == 0
		f.mu.Unlock()
		if idle {
			f.maybePrune()
		}
		time.Sleep(fetchPause)
	}
}

func (f *Filler) make(ctx context.Context, mediaURL string) {
	jpg, w, h, err := f.maker.Make(ctx, mediaURL)
	switch {
	case err == nil:
		if err := f.store.PutThumb(ctx, mediaURL, db.ThumbOK, jpg, w, h); err != nil {
			log.Printf("[THUMB] store failed: %v", err)
		}
	case errors.Is(err, ErrNoStill):
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		f.backOff(mediaURL)
	default:
		// Fetched but unusable (not an image, too large, blocked, gone):
		// recorded, so it is not asked for again for a week.
		log.Printf("[THUMB] %v", err)
		_ = f.store.PutThumb(ctx, mediaURL, db.ThumbUnavailable, nil, 0, 0)
	}
}

func (f *Filler) backOff(url string) {
	f.mu.Lock()
	f.retryAt[url] = time.Now().Add(failureBackoff)
	f.mu.Unlock()
}

// maybePrune drops thumbnails no entry links to, at most every pruneEvery.
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
	if err := f.store.PruneThumbs(ctx, keep); err != nil {
		log.Printf("[THUMB] prune failed: %v", err)
	}
}
