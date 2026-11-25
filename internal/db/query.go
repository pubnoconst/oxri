package db

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// ListFilter specifies optional search and tag criteria when listing journal entries.
type ListFilter struct {
	IncludeUnpublished bool
	Query              string
	Tags               []string
}

// allEntriesCached serves the in-memory snapshot of every entry. The
// snapshot is stale-while-revalidate (HIG §1.6): once older than
// readCacheTTL it is still served at once, and one background refresh brings
// it up to date, so no reader waits on the database. (Before, the first
// reader after it expired waited for the round trip: a 0.7–1.1 s spike
// about every two minutes, found by Server-Timing.) The TTL keeps entries
// written by another server (a second host on the same database) from
// staying stale for long. Only a snapshot that was never loaded, or that a
// write here discarded, is loaded while the reader waits, so the author's
// own changes always show at once.
func (s *Store) allEntriesCached(ctx context.Context) ([]Journal, error) {
	s.cacheMu.RLock()
	if s.cacheValid {
		cp := make([]Journal, len(s.cachedEntries))
		copy(cp, s.cachedEntries)
		stale := time.Since(s.cacheLoadedAt) >= readCacheTTL
		s.cacheMu.RUnlock()
		if stale {
			s.refreshEntriesInBackground()
		}
		return cp, nil
	}
	s.cacheMu.RUnlock()
	return s.loadEntries(ctx)
}

// refreshEntriesInBackground reloads the snapshot once, however many readers
// find it stale.
func (s *Store) refreshEntriesInBackground() {
	if !s.entriesRefreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.entriesRefreshing.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = s.loadEntries(ctx)
	}()
}

// loadEntries reads every entry from the database into the snapshot.
func (s *Store) loadEntries(ctx context.Context) ([]Journal, error) {
	const q = `SELECT ` + journalColumns + ` FROM journals ORDER BY created_at DESC, id DESC;`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Journal
	for rows.Next() {
		j, err := scanJournal(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cachedEntries = make([]Journal, len(entries))
	copy(s.cachedEntries, entries)
	s.cacheValid = true
	s.cacheLoadedAt = time.Now()
	s.cacheMu.Unlock()

	return entries, nil
}

// List returns journal entries ordered newest first. When includeUnpublished is false,
// only rows with published = 1 are returned.
func (s *Store) List(ctx context.Context, includeUnpublished bool) ([]Journal, error) {
	return s.ListFiltered(ctx, ListFilter{IncludeUnpublished: includeUnpublished})
}

// ListFiltered returns journal entries matching optional search query and tag filters
// from the warm in-memory snapshot (or a single DB fetch on cache miss).
func (s *Store) ListFiltered(ctx context.Context, filter ListFilter) ([]Journal, error) {
	all, err := s.allEntriesCached(ctx)
	if err != nil {
		return nil, err
	}

	qText := strings.ToLower(SanitizeSearchQuery(filter.Query))

	var requiredTags []string
	for _, rawTag := range filter.Tags {
		t := FormatTagTitleCase(rawTag)
		if t == "" || utf8.RuneCountInString(t) > MaxTagLength {
			continue
		}
		if _, ok := ValidateTagChars(t); !ok {
			continue
		}
		requiredTags = append(requiredTags, t)
	}

	var out []Journal
	for _, j := range all {
		if !filter.IncludeUnpublished && !j.Published {
			continue
		}
		if qText != "" {
			matchedQuery := strings.Contains(strings.ToLower(j.Body), qText) ||
				strings.Contains(strings.ToLower(j.Tag1), qText) ||
				strings.Contains(strings.ToLower(j.Tag2), qText) ||
				strings.Contains(strings.ToLower(j.Tag3), qText)
			if !matchedQuery {
				continue
			}
		}
		if len(requiredTags) > 0 {
			matchedAllTags := true
			for _, reqTag := range requiredTags {
				if j.Tag1 != reqTag && j.Tag2 != reqTag && j.Tag3 != reqTag {
					matchedAllTags = false
					break
				}
			}
			if !matchedAllTags {
				continue
			}
		}
		out = append(out, j)
	}
	return out, nil
}

// TagsByUse returns the distinct tags of visible entries, most used first
// (ties alphabetical). The catalog shows the head of this list and the editor
// suggests from it, so an archive with many tags still leads with the ones
// the author actually uses.
func (s *Store) TagsByUse(ctx context.Context, includeUnpublished bool) ([]string, error) {
	all, err := s.allEntriesCached(ctx)
	if err != nil {
		return nil, err
	}
	return tagsByUse(all, includeUnpublished), nil
}

// TagsByUseCachedOnly is TagsByUse from the in-memory read cache only: it
// never waits on the database (a cold cache returns nil and warms itself in
// the background).
func (s *Store) TagsByUseCachedOnly(includeUnpublished bool) []string {
	s.cacheMu.RLock()
	valid := s.cacheValid
	stale := valid && time.Since(s.cacheLoadedAt) >= readCacheTTL
	var snapshot []Journal
	if stale {
		defer s.refreshEntriesInBackground()
	}
	if valid {
		snapshot = make([]Journal, len(s.cachedEntries))
		copy(snapshot, s.cachedEntries)
	}
	s.cacheMu.RUnlock()

	if !valid {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = s.allEntriesCached(ctx)
		}()
		return nil
	}
	return tagsByUse(snapshot, includeUnpublished)
}

// tagsByUse counts each tag over the visible entries and orders them by
// count, then name.
func tagsByUse(entries []Journal, includeUnpublished bool) []string {
	counts := make(map[string]int)
	for _, j := range entries {
		if !includeUnpublished && !j.Published {
			continue
		}
		for _, t := range []string{j.Tag1, j.Tag2, j.Tag3} {
			if t != "" {
				counts[t]++
			}
		}
	}
	tags := make([]string, 0, len(counts))
	for t := range counts {
		tags = append(tags, t)
	}
	sort.Slice(tags, func(a, b int) bool {
		if counts[tags[a]] != counts[tags[b]] {
			return counts[tags[a]] > counts[tags[b]]
		}
		return tags[a] < tags[b]
	})
	return tags
}

// Get fetches a single entry by ID. Returns ErrNotFound if absent or hidden.
func (s *Store) Get(ctx context.Context, id int64, includeUnpublished bool) (*Journal, error) {
	all, err := s.allEntriesCached(ctx)
	if err != nil {
		return nil, err
	}
	for _, j := range all {
		if j.ID == id {
			if !includeUnpublished && !j.Published {
				return nil, ErrNotFound
			}
			cp := j
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func getTx(ctx context.Context, q queryExecer, id int64, includeUnpublished bool) (*Journal, error) {
	sqlQuery := `SELECT ` + journalColumns + ` FROM journals WHERE id = ?`
	if !includeUnpublished {
		sqlQuery += ` AND published = 1`
	}
	sqlQuery += `;`

	row := q.QueryRowContext(ctx, sqlQuery, id)
	j, err := scanJournal(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

// journalColumns is the column list scanJournal reads, in its order.
const journalColumns = `id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published, passage`

type scanner interface {
	Scan(dest ...any) error
}

func scanJournal(s scanner) (Journal, error) {
	var j Journal
	var createdAtMs int64
	var t1, t2, t3, passage sql.NullString
	var bookmarked, published int

	if err := s.Scan(&j.ID, &createdAtMs, &j.MediaURL, &t1, &t2, &t3, &j.Body, &bookmarked, &published, &passage); err != nil {
		return j, err
	}
	j.CreatedAt = time.UnixMilli(createdAtMs)
	j.Tag1 = t1.String
	j.Tag2 = t2.String
	j.Tag3 = t3.String
	j.Bookmarked = bookmarked == 1
	j.Published = published == 1
	j.PassageJSON = passage.String
	return j, nil
}
