package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/oxri/oxri-go/internal/domain"
)

const (
	// DefaultMaxStorageBytes is 90% of Turso's 5 GB free tier (4.5 GB).
	DefaultMaxStorageBytes int64 = 4_500_000_000

	// RowOverheadBytes accounts for SQLite B-tree cell headers and fixed-width
	// integer columns (id, created_at, bookmarked, published).
	RowOverheadBytes int64 = 82
)

// ErrQuotaExceeded is returned when creating or updating a journal entry would
// exceed the configured content storage budget.
var ErrQuotaExceeded = errors.New("storage quota exceeded")

// QuotaExceededError provides exact UTF-8 byte estimates when a journal write exceeds
// the configured content storage budget.
type QuotaExceededError struct {
	RequiredBytes int64
	EntryBytes    int64
	UsedBytes     int64
	LimitBytes    int64
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("%s: entry requires %d bytes (%d bytes over %d byte limit)",
		ErrQuotaExceeded.Error(), e.EntryBytes, e.RequiredBytes, e.LimitBytes)
}

func (e *QuotaExceededError) Unwrap() error {
	return ErrQuotaExceeded
}

// EstimatedBytes returns the exact UTF-8 content + row header byte footprint of a persisted journal entry.
func (j Journal) EstimatedBytes() int64 {
	return RowOverheadBytes +
		int64(len(j.MediaURL)) +
		int64(len(j.Tag1)) +
		int64(len(j.Tag2)) +
		int64(len(j.Tag3)) +
		int64(len(j.Body)) +
		int64(len(j.PassageJSON))
}

// SetMaxStorageBytes overrides the maximum storage limit (useful in unit tests).
func (s *Store) SetMaxStorageBytes(limit int64) {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.maxStorageBytes = limit
}

// MaxStorageBytes returns the effective content storage budget limit in bytes.
func (s *Store) MaxStorageBytes() int64 {
	s.idMu.Lock()
	limit := s.maxStorageBytes
	s.idMu.Unlock()
	if limit > 0 {
		return limit
	}
	if raw := strings.TrimSpace(os.Getenv("TURSO_MAX_STORAGE_BYTES")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}
	return DefaultMaxStorageBytes
}

// Storage accounting uses length(CAST(col AS BLOB)) so SQLite measures exact
// UTF-8 byte counts rather than Unicode code points, matching Go's len(str) for
// multi-byte Latin diacritics and Greek characters. Every row also costs
// RowOverheadBytes. Books count toward the budget like journals (one honest
// usage number), but only journals can be vacated: books are never evicted.
var (
	journalStorageBytesSQL = fmt.Sprintf(`
SELECT COALESCE(SUM(
  %d +
  length(CAST(media_url AS BLOB)) +
  length(CAST(COALESCE(tag1, '') AS BLOB)) +
  length(CAST(COALESCE(tag2, '') AS BLOB)) +
  length(CAST(COALESCE(tag3, '') AS BLOB)) +
  length(CAST(body AS BLOB)) +
  length(CAST(COALESCE(passage, '') AS BLOB))
), 0) FROM journals`, RowOverheadBytes)

	bookStorageBytesSQL = fmt.Sprintf(`
SELECT COALESCE(SUM(
  %d +
  length(CAST(slug AS BLOB)) +
  length(CAST(title AS BLOB)) +
  length(CAST(body AS BLOB))
), 0) FROM books`, RowOverheadBytes)

	usedStorageBytesSQL = "SELECT (" + journalStorageBytesSQL + ") + (" + bookStorageBytesSQL + ");"
)

// BookBytes is a book row's storage footprint, matching bookStorageBytesSQL.
func BookBytes(slug, title, body string) int64 {
	return RowOverheadBytes + int64(len(slug)) + int64(len(title)) + int64(len(body))
}

// UsedStorageBytes returns the total storage footprint of journals and books.
func (s *Store) UsedStorageBytes(ctx context.Context) (int64, error) {
	return usedStorageBytesTx(ctx, s.db)
}

func usedStorageBytesTx(ctx context.Context, q queryExecer) (int64, error) {
	var used int64
	if err := q.QueryRowContext(ctx, usedStorageBytesSQL).Scan(&used); err != nil {
		return 0, err
	}
	return used, nil
}

// journalStorageBytes is the part of usage that vacating can free.
func (s *Store) journalStorageBytes(ctx context.Context) (int64, error) {
	var n int64
	if err := s.db.QueryRowContext(ctx, journalStorageBytesSQL+";").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// checkQuotaForCreateTx evaluates projected usage <= limit inside the active write transaction,
// even when the database is empty (used == 0).
func (s *Store) checkQuotaForCreateTx(ctx context.Context, q queryExecer, entry domain.ValidatedJournalEntry) error {
	used, err := usedStorageBytesTx(ctx, q)
	if err != nil {
		return err
	}
	limit := s.MaxStorageBytes()
	entryBytes := entry.EstimatedBytes(RowOverheadBytes)
	projected := used + entryBytes
	if projected > limit {
		req := projected - limit
		if used > 0 && req > used {
			req = used
		}
		return &QuotaExceededError{
			RequiredBytes: req,
			EntryBytes:    entryBytes,
			UsedBytes:     used,
			LimitBytes:    limit,
		}
	}
	return nil
}

// checkQuotaForUpdateTx evaluates (used - oldBytes + newBytes) <= limit inside the active write transaction,
// even when no other rows exist (otherBytes == 0).
func (s *Store) checkQuotaForUpdateTx(ctx context.Context, q queryExecer, id int64, entry domain.ValidatedJournalEntry) error {
	existing, err := getTx(ctx, q, id, true)
	if err != nil {
		return err
	}
	used, err := usedStorageBytesTx(ctx, q)
	if err != nil {
		return err
	}
	oldBytes := existing.EstimatedBytes()
	otherBytes := used - oldBytes
	if otherBytes < 0 {
		otherBytes = 0
	}
	limit := s.MaxStorageBytes()
	newBytes := entry.EstimatedBytes(RowOverheadBytes)
	projected := otherBytes + newBytes
	if projected > limit {
		req := projected - limit
		if otherBytes > 0 && req > otherBytes {
			req = otherBytes
		}
		return &QuotaExceededError{
			RequiredBytes: req,
			EntryBytes:    newBytes,
			UsedBytes:     used,
			LimitBytes:    limit,
		}
	}
	return nil
}

// AutoVacatePlan summarizes which entries will be removed by prioritized automatic eviction
// (unpublished entries first, then published un-bookmarked entries, oldest first).
type AutoVacatePlan struct {
	CandidateIDs      []int64
	UnpublishedCount  int
	UnbookmarkedCount int
	TotalFreedBytes   int64
}

// PreviewAutoVacate computes the prioritized set of unpublished (first) and un-bookmarked (second)
// entries needed to vacate at least requiredBytes.
func (s *Store) PreviewAutoVacate(ctx context.Context, requiredBytes int64) (AutoVacatePlan, error) {
	return previewAutoVacateTx(ctx, s.db, requiredBytes)
}

func previewAutoVacateTx(ctx context.Context, q queryExecer, requiredBytes int64) (AutoVacatePlan, error) {
	sqlQuery := `
SELECT ` + journalColumns + `
FROM journals
WHERE published = 0 OR bookmarked = 0
ORDER BY
  CASE WHEN published = 0 THEN 0 ELSE 1 END ASC,
  created_at ASC,
  id ASC;`

	rows, err := q.QueryContext(ctx, sqlQuery)
	if err != nil {
		return AutoVacatePlan{}, err
	}
	defer rows.Close()

	var plan AutoVacatePlan
	for rows.Next() {
		j, err := scanJournal(rows)
		if err != nil {
			return AutoVacatePlan{}, err
		}
		if requiredBytes > 0 && plan.TotalFreedBytes >= requiredBytes {
			break
		}
		plan.CandidateIDs = append(plan.CandidateIDs, j.ID)
		plan.TotalFreedBytes += j.EstimatedBytes()
		if !j.Published {
			plan.UnpublishedCount++
		} else {
			plan.UnbookmarkedCount++
		}
		if requiredBytes <= 0 {
			break
		}
	}
	return plan, rows.Err()
}

// AutoVacateBytes atomically computes the prioritized eviction plan and deletes the selected
// unpublished (first) and un-bookmarked (second) entries inside a single database transaction.
func (s *Store) AutoVacateBytes(ctx context.Context, cap domain.AdminCap, requiredBytes int64) (AutoVacatePlan, error) {
	if err := requireCap(cap); err != nil {
		return AutoVacatePlan{}, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	target := requiredBytes
	if target <= 0 {
		target = 1
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AutoVacatePlan{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	plan, err := previewAutoVacateTx(ctx, tx, target)
	if err != nil {
		return plan, err
	}
	if len(plan.CandidateIDs) == 0 {
		return plan, nil
	}
	if err := deleteTx(ctx, tx, plan.CandidateIDs); err != nil {
		return plan, err
	}
	if err := tx.Commit(); err != nil {
		return plan, err
	}
	s.invalidateCache()
	return plan, nil
}
