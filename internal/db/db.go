package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

const (
	MaxMediaURLLength = 128
	MaxTagCount       = 3
	MaxTagLength      = 4
)

var (
	ErrNotFound         = errors.New("journal entry not found")
	ErrInvalidInput     = errors.New("invalid journal input")
)

// Journal represents a persisted journal entry in the `journals` table.
type Journal struct {
	ID         int64
	CreatedAt  time.Time
	MediaURL   string
	Tag1       string
	Tag2       string
	Tag3       string
	Body       string
	Bookmarked bool
	Published  bool
}

// Tags returns a slice of all non-empty tags on the entry.
func (j Journal) Tags() []string {
	var out []string
	if j.Tag1 != "" {
		out = append(out, j.Tag1)
	}
	if j.Tag2 != "" {
		out = append(out, j.Tag2)
	}
	if j.Tag3 != "" {
		out = append(out, j.Tag3)
	}
	return out
}

// TagsString returns comma-separated tags suitable for pre-filling the editor input.
func (j Journal) TagsString() string {
	return strings.Join(j.Tags(), ", ")
}

// JournalInput carries validated user input for creating or updating a journal entry.
type JournalInput struct {
	MediaURL   string
	Tags       []string
	Body       string
	Bookmarked bool
	Published  bool
}

// ParseTags splits a comma-separated tag string into trimmed non-empty tags.
func ParseTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}

// ValidateInput enforces all database schema constraints before persistence.
func ValidateInput(in JournalInput) (JournalInput, error) {
	in.MediaURL = strings.TrimSpace(in.MediaURL)
	if in.MediaURL == "" {
		return in, fmt.Errorf("%w: media link is required", ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.MediaURL) > MaxMediaURLLength {
		return in, fmt.Errorf("%w: media link must be at most %d characters", ErrInvalidInput, MaxMediaURLLength)
	}
	u, err := url.ParseRequestURI(in.MediaURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return in, fmt.Errorf("%w: media link must be a valid http or https URL", ErrInvalidInput)
	}

	if len(in.Tags) > MaxTagCount {
		return in, fmt.Errorf("%w: at most %d tags are allowed", ErrInvalidInput, MaxTagCount)
	}
	cleanedTags := make([]string, 0, len(in.Tags))
	for _, tag := range in.Tags {
		t := strings.TrimSpace(tag)
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > MaxTagLength {
			return in, fmt.Errorf("%w: tag %q exceeds %d characters", ErrInvalidInput, t, MaxTagLength)
		}
		cleanedTags = append(cleanedTags, t)
	}
	in.Tags = cleanedTags

	if strings.TrimSpace(in.Body) == "" {
		return in, fmt.Errorf("%w: journal content cannot be empty", ErrInvalidInput)
	}

	return in, nil
}

// Store wraps SQL operations against Turso / libSQL or local SQLite.
type Store struct {
	db     *sql.DB
	idMu   sync.Mutex
	lastID int64
}

// Open connects to a Turso/libSQL URL (when libsql:// or https://) or a local SQLite path
// (file:... or :memory:), verifies connectivity, and runs schema migrations.
func Open(ctx context.Context, dbURL, authToken string) (*Store, error) {
	dbURL = strings.TrimSpace(dbURL)
	if dbURL == "" {
		dbURL = "file:res_imperii.db"
	}

	var sqlDB *sql.DB
	if dbURL == ":memory:" || strings.HasPrefix(dbURL, "file:") {
		db, err := sql.Open("sqlite", dbURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite database: %w", err)
		}
		sqlDB = db
	} else {
		var opts []libsql.Option
		if authToken != "" {
			opts = append(opts, libsql.WithAuthToken(authToken))
		}
		connector, err := libsql.NewConnector(dbURL, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create libsql connector: %w", err)
		}
		sqlDB = sql.OpenDB(connector)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	store := &Store{db: sqlDB}
	if err := store.EnsureSchema(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("schema initialization failed: %w", err)
	}

	return store, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// EnsureSchema creates the journals table if missing and adds the `published` column
// if upgrading an existing database schema.
func (s *Store) EnsureSchema(ctx context.Context) error {
	const createTableSQL = `
CREATE TABLE IF NOT EXISTS journals (
  id INTEGER PRIMARY KEY,
  created_at INTEGER NOT NULL DEFAULT (unixepoch() * 1000),
  media_url TEXT NOT NULL CHECK(length(media_url) <= 128 AND length(media_url) > 0),
  tag1 TEXT CHECK(tag1 IS NULL OR length(tag1) <= 4),
  tag2 TEXT CHECK(tag2 IS NULL OR length(tag2) <= 4),
  tag3 TEXT CHECK(tag3 IS NULL OR length(tag3) <= 4),
  body TEXT NOT NULL,
  bookmarked INTEGER NOT NULL DEFAULT 0 CHECK(bookmarked IN (0,1)),
  published INTEGER NOT NULL DEFAULT 1 CHECK(published IN (0,1))
);`
	if _, err := s.db.ExecContext(ctx, createTableSQL); err != nil {
		return err
	}

	// Inspect columns via PRAGMA table_info(journals) to check if `published` exists.
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(journals);`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasPublished := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "published" {
			hasPublished = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasPublished {
		const alterSQL = `ALTER TABLE journals ADD COLUMN published INTEGER NOT NULL DEFAULT 1 CHECK(published IN (0,1));`
		if _, err := s.db.ExecContext(ctx, alterSQL); err != nil {
			return fmt.Errorf("failed to add published column: %w", err)
		}
	}

	return nil
}

func (s *Store) nextID() int64 {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	now := time.Now().UnixMilli()
	if now <= s.lastID {
		now = s.lastID + 1
	}
	s.lastID = now
	return now
}

func nullableTag(tags []string, idx int) sql.NullString {
	if idx < len(tags) && tags[idx] != "" {
		return sql.NullString{String: tags[idx], Valid: true}
	}
	return sql.NullString{Valid: false}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Create validates and inserts a new journal entry, returning the persisted record.
func (s *Store) Create(ctx context.Context, in JournalInput) (*Journal, error) {
	valid, err := ValidateInput(in)
	if err != nil {
		return nil, err
	}

	id := s.nextID()
	createdAtMs := id
	t1 := nullableTag(valid.Tags, 0)
	t2 := nullableTag(valid.Tags, 1)
	t3 := nullableTag(valid.Tags, 2)

	const q = `
INSERT INTO journals (id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`
	if _, err := s.db.ExecContext(ctx, q, id, createdAtMs, valid.MediaURL, t1, t2, t3, valid.Body, boolToInt(valid.Bookmarked), boolToInt(valid.Published)); err != nil {
		return nil, err
	}

	return &Journal{
		ID:         id,
		CreatedAt:  time.UnixMilli(createdAtMs),
		MediaURL:   valid.MediaURL,
		Tag1:       t1.String,
		Tag2:       t2.String,
		Tag3:       t3.String,
		Body:       valid.Body,
		Bookmarked: valid.Bookmarked,
		Published:  valid.Published,
	}, nil
}

// List returns journal entries ordered newest first. When includeUnpublished is false,
// only rows with published = 1 are returned.
func (s *Store) List(ctx context.Context, includeUnpublished bool) ([]Journal, error) {
	q := `SELECT id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published FROM journals`
	if !includeUnpublished {
		q += ` WHERE published = 1`
	}
	q += ` ORDER BY created_at DESC, id DESC;`

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
	return entries, rows.Err()
}

// Get fetches a single entry by ID. Returns ErrNotFound if absent or hidden.
func (s *Store) Get(ctx context.Context, id int64, includeUnpublished bool) (*Journal, error) {
	q := `SELECT id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published FROM journals WHERE id = ?`
	if !includeUnpublished {
		q += ` AND published = 1`
	}
	q += `;`

	row := s.db.QueryRowContext(ctx, q, id)
	j, err := scanJournal(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

// Update validates and overwrites an existing entry's mutable fields.
func (s *Store) Update(ctx context.Context, id int64, in JournalInput) error {
	valid, err := ValidateInput(in)
	if err != nil {
		return err
	}

	t1 := nullableTag(valid.Tags, 0)
	t2 := nullableTag(valid.Tags, 1)
	t3 := nullableTag(valid.Tags, 2)

	const q = `
UPDATE journals
SET media_url = ?, tag1 = ?, tag2 = ?, tag3 = ?, body = ?, bookmarked = ?, published = ?
WHERE id = ?;`
	res, err := s.db.ExecContext(ctx, q, valid.MediaURL, t1, t2, t3, valid.Body, boolToInt(valid.Bookmarked), boolToInt(valid.Published), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPublished sets the published status on the given IDs.
func (s *Store) SetPublished(ctx context.Context, ids []int64, published bool) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, boolToInt(published))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := fmt.Sprintf("UPDATE journals SET published = ? WHERE id IN (%s);", strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}

// SetAllPublished sets the published status across all journal entries.
func (s *Store) SetAllPublished(ctx context.Context, published bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE journals SET published = ?;`, boolToInt(published))
	return err
}

// ToggleBookmark flips the bookmarked bit on an entry and returns its new state.
func (s *Store) ToggleBookmark(ctx context.Context, id int64) (bool, error) {
	const q = `UPDATE journals SET bookmarked = 1 - bookmarked WHERE id = ?;`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return false, ErrNotFound
	}
	var bookmarked int
	if err := s.db.QueryRowContext(ctx, `SELECT bookmarked FROM journals WHERE id = ?;`, id).Scan(&bookmarked); err != nil {
		return false, err
	}
	return bookmarked == 1, nil
}

// Delete removes one or more entries by ID permanently.
func (s *Store) Delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf("DELETE FROM journals WHERE id IN (%s);", strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}

// DeleteAll removes all entries from the journals table.
func (s *Store) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM journals;`)
	return err
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
