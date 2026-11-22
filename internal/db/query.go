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

func (s *Store) allEntriesCached(ctx context.Context) ([]Journal, error) {
	s.cacheMu.RLock()
	if s.cacheValid && time.Since(s.cacheLoadedAt) < readCacheTTL {
		cp := make([]Journal, len(s.cachedEntries))
		copy(cp, s.cachedEntries)
		s.cacheMu.RUnlock()
		return cp, nil
	}
	s.cacheMu.RUnlock()

	const q = `SELECT id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published FROM journals ORDER BY created_at DESC, id DESC;`
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

// DistinctTags returns all unique non-empty tags currently stored across tag1, tag2, and tag3,
// sorted alphabetically.
func (s *Store) DistinctTags(ctx context.Context, includeUnpublished bool) ([]string, error) {
	all, err := s.allEntriesCached(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var tags []string
	for _, j := range all {
		if !includeUnpublished && !j.Published {
			continue
		}
		for _, t := range []string{j.Tag1, j.Tag2, j.Tag3} {
			if t == "" {
				continue
			}
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				tags = append(tags, t)
			}
		}
	}
	sort.Strings(tags)
	return tags, nil
}

// DistinctTagsCachedOnly returns distinct tags if the in-memory read cache is currently warm,
// without blocking on a remote network round-trip (and triggers an async cache warm if cold).
func (s *Store) DistinctTagsCachedOnly(includeUnpublished bool) []string {
	s.cacheMu.RLock()
	valid := s.cacheValid && time.Since(s.cacheLoadedAt) < readCacheTTL
	var snapshot []Journal
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

	seen := make(map[string]struct{})
	var tags []string
	for _, j := range snapshot {
		if !includeUnpublished && !j.Published {
			continue
		}
		for _, t := range []string{j.Tag1, j.Tag2, j.Tag3} {
			if t == "" {
				continue
			}
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				tags = append(tags, t)
			}
		}
	}
	sort.Strings(tags)
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
	sqlQuery := `SELECT id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published FROM journals WHERE id = ?`
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

type scanner interface {
	Scan(dest ...any) error
}

func scanJournal(s scanner) (Journal, error) {
	var j Journal
	var createdAtMs int64
	var t1, t2, t3 sql.NullString
	var bookmarked, published int

	if err := s.Scan(&j.ID, &createdAtMs, &j.MediaURL, &t1, &t2, &t3, &j.Body, &bookmarked, &published); err != nil {
		return j, err
	}
	j.CreatedAt = time.UnixMilli(createdAtMs)
	j.Tag1 = t1.String
	j.Tag2 = t2.String
	j.Tag3 = t3.String
	j.Bookmarked = bookmarked == 1
	j.Published = published == 1
	return j, nil
}
