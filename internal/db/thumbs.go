package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Media thumbnails: a small still for an entry's linked image, GIF, YouTube
// or Vimeo video, made once by the server (internal/thumb) so a reader sees
// the media's look without their browser fetching the media itself until
// they tap it (HIG §1.7). Keyed by the media link.
//
// Like tweet snapshots they are a cache of public data, not the author's
// writing: not counted against the storage quota, written by the
// background fetcher (no AdminCap), and pruned when no entry links to them.
// The metadata of all of them is kept in memory; the image bytes stay in the
// database, with the most recently served few cached here.

const mediaThumbsTableDDL = `
CREATE TABLE IF NOT EXISTS media_thumbs (
  key TEXT PRIMARY KEY CHECK(length(key) = 24),
  url TEXT NOT NULL UNIQUE CHECK(length(url) BETWEEN 1 AND 128),
  status TEXT NOT NULL CHECK(status IN ('ok', 'unavailable')),
  body BLOB NOT NULL CHECK(length(body) <= 262144),
  width INTEGER NOT NULL CHECK(width BETWEEN 0 AND 4096),
  height INTEGER NOT NULL CHECK(height BETWEEN 0 AND 4096),
  fetched_at INTEGER NOT NULL
);`

// ThumbStatus is whether a still could be made for the link.
type ThumbStatus string

const (
	ThumbOK          ThumbStatus = "ok"
	ThumbUnavailable ThumbStatus = "unavailable" // the media could not be fetched or decoded
)

// MaxThumbBytes caps one stored thumbnail (a JPEG of at most 680 px).
const MaxThumbBytes = 256 << 10

// ErrInvalidThumb is a thumbnail outside the table's rules.
var ErrInvalidThumb = errors.New("invalid thumbnail")

// StoredThumb is a thumbnail's record, without its bytes.
type StoredThumb struct {
	Key           string // served at /thumb/{Key}.jpg
	Status        ThumbStatus
	Width, Height int
	FetchedAt     time.Time
}

// ThumbKey is the key a media link's thumbnail is stored and served under.
func ThumbKey(mediaURL string) string {
	sum := sha256.Sum256([]byte(mediaURL))
	return hex.EncodeToString(sum[:])[:24]
}

type thumbCache struct {
	mu     sync.RWMutex
	loaded bool
	byURL  map[string]StoredThumb

	bodyMu    sync.Mutex
	bodies    map[string][]byte // key -> JPEG, the most recently served
	bodyOrder []string
}

const thumbBodyCache = 64

func (s *Store) loadThumbsLocked(ctx context.Context) error {
	if s.thumbs.loaded {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key, url, status, width, height, fetched_at FROM media_thumbs;`)
	if err != nil {
		return err
	}
	defer rows.Close()
	byURL := map[string]StoredThumb{}
	for rows.Next() {
		var t StoredThumb
		var url, status string
		var ms int64
		if err := rows.Scan(&t.Key, &url, &status, &t.Width, &t.Height, &ms); err != nil {
			return err
		}
		t.Status, t.FetchedAt = ThumbStatus(status), time.UnixMilli(ms)
		byURL[url] = t
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.thumbs.byURL, s.thumbs.loaded = byURL, true
	return nil
}

// Thumbs returns the stored thumbnails for these media links (links without
// one are absent).
func (s *Store) Thumbs(ctx context.Context, urls []string) (map[string]StoredThumb, error) {
	s.thumbs.mu.RLock()
	if s.thumbs.loaded {
		out := pickThumbs(s.thumbs.byURL, urls)
		s.thumbs.mu.RUnlock()
		return out, nil
	}
	s.thumbs.mu.RUnlock()
	s.thumbs.mu.Lock()
	defer s.thumbs.mu.Unlock()
	if err := s.loadThumbsLocked(ctx); err != nil {
		return nil, err
	}
	return pickThumbs(s.thumbs.byURL, urls), nil
}

func pickThumbs(all map[string]StoredThumb, urls []string) map[string]StoredThumb {
	out := make(map[string]StoredThumb, len(urls))
	for _, u := range urls {
		if t, ok := all[u]; ok {
			out[u] = t
		}
	}
	return out
}

// PutThumb stores (or replaces) the thumbnail of a media link. An
// unavailable one has no body and no size.
func (s *Store) PutThumb(ctx context.Context, mediaURL string, status ThumbStatus, body []byte, width, height int) error {
	if mediaURL == "" || len(mediaURL) > 128 || len(body) > MaxThumbBytes ||
		(status != ThumbOK && status != ThumbUnavailable) || (status == ThumbOK && (len(body) == 0 || width <= 0 || height <= 0)) {
		return ErrInvalidThumb
	}
	if body == nil {
		body = []byte{}
	}
	key, now := ThumbKey(mediaURL), time.Now()
	s.thumbs.mu.Lock()
	defer s.thumbs.mu.Unlock()
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO media_thumbs (key, url, status, body, width, height, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?);`,
		key, mediaURL, string(status), body, width, height, now.UnixMilli(),
	); err != nil {
		return err
	}
	if s.thumbs.loaded {
		s.thumbs.byURL[mediaURL] = StoredThumb{Key: key, Status: status, Width: width, Height: height, FetchedAt: now}
	}
	s.thumbs.bodyMu.Lock()
	delete(s.thumbs.bodies, key)
	s.thumbs.bodyMu.Unlock()
	return nil
}

// ThumbBody is a stored thumbnail's JPEG, by key; nil if there is none.
func (s *Store) ThumbBody(ctx context.Context, key string) ([]byte, error) {
	s.thumbs.bodyMu.Lock()
	if b, ok := s.thumbs.bodies[key]; ok {
		s.thumbs.bodyMu.Unlock()
		return b, nil
	}
	s.thumbs.bodyMu.Unlock()

	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM media_thumbs WHERE key = ? AND status = 'ok';`, key).Scan(&body)
	if err != nil {
		return nil, nil //nolint:nilerr // absent or unreadable: no thumbnail
	}
	s.thumbs.bodyMu.Lock()
	defer s.thumbs.bodyMu.Unlock()
	if s.thumbs.bodies == nil {
		s.thumbs.bodies = map[string][]byte{}
	}
	if _, ok := s.thumbs.bodies[key]; !ok {
		s.thumbs.bodyOrder = append(s.thumbs.bodyOrder, key)
		if len(s.thumbs.bodyOrder) > thumbBodyCache {
			delete(s.thumbs.bodies, s.thumbs.bodyOrder[0])
			s.thumbs.bodyOrder = s.thumbs.bodyOrder[1:]
		}
	}
	s.thumbs.bodies[key] = body
	return body, nil
}

// PruneThumbs deletes the thumbnails of media no entry links to any more.
func (s *Store) PruneThumbs(ctx context.Context, keep map[string]bool) error {
	s.thumbs.mu.Lock()
	defer s.thumbs.mu.Unlock()
	if err := s.loadThumbsLocked(ctx); err != nil {
		return err
	}
	for url := range s.thumbs.byURL {
		if keep[url] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM media_thumbs WHERE url = ?;`, url); err != nil {
			return err
		}
		delete(s.thumbs.byURL, url)
		s.thumbs.bodyMu.Lock()
		delete(s.thumbs.bodies, ThumbKey(url))
		s.thumbs.bodyMu.Unlock()
	}
	return nil
}
