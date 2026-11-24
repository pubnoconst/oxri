package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/oxri/oxri-go/internal/domain"
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
	// PassageJSON is the Bible passage the entry quotes, checked against its provider
	// (domain.EncodePassage), or "" when it quotes none.
	PassageJSON string `json:"passage,omitempty"`
}

// Passage decodes the quoted passage; ok is false when there is none (or it
// no longer validates, which leaves the media link to journal-media.js).
func (j Journal) Passage() (domain.Passage, bool) {
	if j.PassageJSON == "" {
		return domain.Passage{}, false
	}
	p, err := domain.DecodePassage(j.PassageJSON)
	return p, err == nil
}

func nullablePassage(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
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

func nullableTag(tag string) sql.NullString {
	if tag != "" {
		return sql.NullString{String: tag, Valid: true}
	}
	return sql.NullString{Valid: false}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateValidated atomically checks the storage quota and inserts a validated domain entry
// inside an explicit database transaction serialized by writeMu.
func (s *Store) CreateValidated(ctx context.Context, cap domain.AdminCap, entry domain.ValidatedJournalEntry) (*Journal, error) {
	if err := requireCap(cap); err != nil {
		return nil, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := s.checkQuotaForCreateTx(ctx, tx, entry); err != nil {
		return nil, err
	}

	id := s.nextID()
	createdAtMs := id
	t1 := nullableTag(entry.TagAt(0))
	t2 := nullableTag(entry.TagAt(1))
	t3 := nullableTag(entry.TagAt(2))

	const q = `
INSERT INTO journals (id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published, passage)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	if _, err := tx.ExecContext(ctx, q, id, createdAtMs, entry.MediaURL(), t1, t2, t3, entry.Body(), boolToInt(entry.Bookmarked()), boolToInt(entry.Published()), nullablePassage(entry.PassageJSON())); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.invalidateCache()
	s.clearPendingJournal()

	return &Journal{
		ID:          id,
		CreatedAt:   time.UnixMilli(createdAtMs),
		MediaURL:    entry.MediaURL(),
		Tag1:        t1.String,
		Tag2:        t2.String,
		Tag3:        t3.String,
		Body:        entry.Body(),
		Bookmarked:  entry.Bookmarked(),
		Published:   entry.Published(),
		PassageJSON: entry.PassageJSON(),
	}, nil
}

// UpdateValidated atomically checks the storage quota and updates an existing entry
// inside an explicit database transaction serialized by writeMu.
func (s *Store) UpdateValidated(ctx context.Context, cap domain.AdminCap, id int64, entry domain.ValidatedJournalEntry) error {
	if err := requireCap(cap); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := s.checkQuotaForUpdateTx(ctx, tx, id, entry); err != nil {
		return err
	}

	t1 := nullableTag(entry.TagAt(0))
	t2 := nullableTag(entry.TagAt(1))
	t3 := nullableTag(entry.TagAt(2))

	const q = `
UPDATE journals
SET media_url = ?, tag1 = ?, tag2 = ?, tag3 = ?, body = ?, bookmarked = ?, published = ?, passage = ?
WHERE id = ?;`
	res, err := tx.ExecContext(ctx, q, entry.MediaURL(), t1, t2, t3, entry.Body(), boolToInt(entry.Bookmarked()), boolToInt(entry.Published()), nullablePassage(entry.PassageJSON()), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidateCache()
	s.clearPendingJournal()
	return nil
}

// SetPublished sets the published status on the given IDs.
func (s *Store) SetPublished(ctx context.Context, cap domain.AdminCap, ids []int64, published bool) error {
	if err := requireCap(cap); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, boolToInt(published))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := fmt.Sprintf("UPDATE journals SET published = ? WHERE id IN (%s);", strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, q, args...)
	if err == nil {
		s.invalidateCache()
	}
	return err
}

// SetAllPublished sets the published status across all journal entries.
func (s *Store) SetAllPublished(ctx context.Context, cap domain.AdminCap, published bool) error {
	if err := requireCap(cap); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx, `UPDATE journals SET published = ?;`, boolToInt(published))
	if err == nil {
		s.invalidateCache()
	}
	return err
}

// ToggleBookmark atomically flips the bookmarked bit on an entry and returns its new state.
func (s *Store) ToggleBookmark(ctx context.Context, cap domain.AdminCap, id int64) (bool, error) {
	if err := requireCap(cap); err != nil {
		return false, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const q = `UPDATE journals SET bookmarked = 1 - bookmarked WHERE id = ?;`
	res, err := tx.ExecContext(ctx, q, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return false, ErrNotFound
	}
	var bookmarked int
	if err := tx.QueryRowContext(ctx, `SELECT bookmarked FROM journals WHERE id = ?;`, id).Scan(&bookmarked); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	s.invalidateCache()
	return bookmarked == 1, nil
}

// Delete removes one or more entries by ID permanently.
func (s *Store) Delete(ctx context.Context, cap domain.AdminCap, ids []int64) error {
	if err := requireCap(cap); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	err := deleteTx(ctx, s.db, ids)
	if err == nil {
		s.invalidateCache()
	}
	return err
}

func deleteTx(ctx context.Context, q queryExecer, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	sqlQuery := fmt.Sprintf("DELETE FROM journals WHERE id IN (%s);", strings.Join(placeholders, ","))
	_, err := q.ExecContext(ctx, sqlQuery, args...)
	return err
}

// DeleteAll removes all entries from the journals table.
func (s *Store) DeleteAll(ctx context.Context, cap domain.AdminCap) error {
	if err := requireCap(cap); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM journals;`)
	if err == nil {
		s.invalidateCache()
	}
	return err
}
