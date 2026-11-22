package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// List returns journal entries ordered newest first. When includeUnpublished is false,
// only rows with published = 1 are returned.
func (s *Store) List(ctx context.Context, includeUnpublished bool) ([]Journal, error) {
	return s.ListFiltered(ctx, ListFilter{IncludeUnpublished: includeUnpublished})
}

// ListFiltered returns journal entries matching optional search query and tag filters.
func (s *Store) ListFiltered(ctx context.Context, filter ListFilter) ([]Journal, error) {
	var clauses []string
	var args []any

	if !filter.IncludeUnpublished {
		clauses = append(clauses, "published = 1")
	}

	qText := SanitizeSearchQuery(filter.Query)
	if qText != "" {
		likePattern := "%" + strings.ToLower(qText) + "%"
		clauses = append(clauses, "(LOWER(body) LIKE ? OR LOWER(COALESCE(tag1, '')) LIKE ? OR LOWER(COALESCE(tag2, '')) LIKE ? OR LOWER(COALESCE(tag3, '')) LIKE ?)")
		args = append(args, likePattern, likePattern, likePattern, likePattern)
	}

	for _, rawTag := range filter.Tags {
		t := FormatTagTitleCase(rawTag)
		if t == "" || utf8.RuneCountInString(t) > MaxTagLength {
			continue
		}
		if _, ok := ValidateTagChars(t); !ok {
			continue
		}
		clauses = append(clauses, "(tag1 = ? OR tag2 = ? OR tag3 = ?)")
		args = append(args, t, t, t)
	}

	q := `SELECT id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published FROM journals`
	if len(clauses) > 0 {
		q += ` WHERE ` + strings.Join(clauses, " AND ")
	}
	q += ` ORDER BY created_at DESC, id DESC;`

	rows, err := s.db.QueryContext(ctx, q, args...)
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
	return entries, rows.Err()
}

// DistinctTags returns all unique non-empty tags currently stored across tag1, tag2, and tag3,
// sorted alphabetically.
func (s *Store) DistinctTags(ctx context.Context, includeUnpublished bool) ([]string, error) {
	whereClause := ""
	if !includeUnpublished {
		whereClause = " AND published = 1"
	}
	q := fmt.Sprintf(`
SELECT DISTINCT tag FROM (
  SELECT tag1 AS tag FROM journals WHERE tag1 IS NOT NULL AND tag1 != ''%s
  UNION
  SELECT tag2 AS tag FROM journals WHERE tag2 IS NOT NULL AND tag2 != ''%s
  UNION
  SELECT tag3 AS tag FROM journals WHERE tag3 IS NOT NULL AND tag3 != ''%s
) ORDER BY tag ASC;`, whereClause, whereClause, whereClause)

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags, rows.Err()
}

// Get fetches a single entry by ID. Returns ErrNotFound if absent or hidden.
func (s *Store) Get(ctx context.Context, id int64, includeUnpublished bool) (*Journal, error) {
	return getTx(ctx, s.db, id, includeUnpublished)
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
