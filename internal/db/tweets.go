package db

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"time"
)

// Tweet snapshots: what the site's own tweet box draws for a post on X
// (internal/tweet, views/tweet), keyed by tweet id.
//
// They are a cache of public data, not the author's writing: they are not
// counted against the storage quota, are written by the background fetcher
// rather than by an author action (so no AdminCap), and are pruned when no
// entry links to the tweet any more. Each is small (a few hundred bytes; the
// column is capped at 16 KB) and there is at most one per tweet in the
// archive. The db package stores the JSON as given; internal/tweet decodes
// and re-checks it.
const tweetSnapshotsTableDDL = `
CREATE TABLE IF NOT EXISTS tweet_snapshots (
  id TEXT PRIMARY KEY CHECK(length(id) BETWEEN 1 AND 20 AND id NOT GLOB '*[^0-9]*'),
  status TEXT NOT NULL CHECK(status IN ('ok', 'unavailable')),
  body TEXT NOT NULL CHECK(length(body) <= 16384),
  fetched_at INTEGER NOT NULL
);`

// TweetStatus is whether X had a public post for the id.
type TweetStatus string

const (
	TweetOK          TweetStatus = "ok"
	TweetUnavailable TweetStatus = "unavailable" // deleted, private or withheld: keep X's embed
)

// StoredTweet is one cached snapshot.
type StoredTweet struct {
	Status    TweetStatus
	Body      []byte // snapshot JSON (empty when unavailable)
	FetchedAt time.Time
}

// ErrInvalidTweet rejects an id or body the table would not accept.
var ErrInvalidTweet = errors.New("invalid tweet snapshot")

const maxTweetBody = 16384

var tweetIDPattern = regexp.MustCompile(`^[0-9]{1,20}$`)

// tweetCache holds every snapshot in memory: there are only as many as there
// are tweets in the archive, and pages read them on every render.
type tweetCache struct {
	mu     sync.RWMutex
	loaded bool
	byID   map[string]StoredTweet
}

func (s *Store) loadTweetsLocked(ctx context.Context) error {
	if s.tweets.loaded {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, status, body, fetched_at FROM tweet_snapshots;`)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[string]StoredTweet{}
	for rows.Next() {
		var id, status, body string
		var fetched int64
		if err := rows.Scan(&id, &status, &body, &fetched); err != nil {
			return err
		}
		byID[id] = StoredTweet{Status: TweetStatus(status), Body: []byte(body), FetchedAt: time.UnixMilli(fetched)}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.tweets.byID = byID
	s.tweets.loaded = true
	return nil
}

// TweetSnapshots returns the cached snapshots for ids (missing ids are absent).
func (s *Store) TweetSnapshots(ctx context.Context, ids []string) (map[string]StoredTweet, error) {
	s.tweets.mu.RLock()
	if s.tweets.loaded {
		out := pickTweets(s.tweets.byID, ids)
		s.tweets.mu.RUnlock()
		return out, nil
	}
	s.tweets.mu.RUnlock()

	s.tweets.mu.Lock()
	defer s.tweets.mu.Unlock()
	if err := s.loadTweetsLocked(ctx); err != nil {
		return nil, err
	}
	return pickTweets(s.tweets.byID, ids), nil
}

func pickTweets(all map[string]StoredTweet, ids []string) map[string]StoredTweet {
	out := make(map[string]StoredTweet, len(ids))
	for _, id := range ids {
		if t, ok := all[id]; ok {
			out[id] = t
		}
	}
	return out
}

// PutTweetSnapshot records what X returned for id.
func (s *Store) PutTweetSnapshot(ctx context.Context, id string, status TweetStatus, body []byte) error {
	if !tweetIDPattern.MatchString(id) || len(body) > maxTweetBody || (status != TweetOK && status != TweetUnavailable) {
		return ErrInvalidTweet
	}
	now := time.Now()
	s.tweets.mu.Lock()
	defer s.tweets.mu.Unlock()
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO tweet_snapshots (id, status, body, fetched_at) VALUES (?, ?, ?, ?);`,
		id, string(status), string(body), now.UnixMilli(),
	); err != nil {
		return err
	}
	if s.tweets.loaded {
		s.tweets.byID[id] = StoredTweet{Status: status, Body: append([]byte(nil), body...), FetchedAt: now}
	}
	return nil
}

// PruneTweetSnapshots deletes the snapshots of tweets no entry links to.
func (s *Store) PruneTweetSnapshots(ctx context.Context, keep map[string]bool) error {
	s.tweets.mu.Lock()
	defer s.tweets.mu.Unlock()
	if err := s.loadTweetsLocked(ctx); err != nil {
		return err
	}
	for id := range s.tweets.byID {
		if keep[id] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM tweet_snapshots WHERE id = ?;`, id); err != nil {
			return err
		}
		delete(s.tweets.byID, id)
	}
	return nil
}
