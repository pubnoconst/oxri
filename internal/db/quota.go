package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// DefaultMaxStorageBytes is 90% of Turso's 5 GB free tier (4.5 GB).
	DefaultMaxStorageBytes int64 = 4_500_000_000

	// RowOverheadBytes accounts for SQLite B-tree cell headers and fixed-width
	// integer columns (id, created_at, bookmarked, published).
	RowOverheadBytes int64 = 82
)

// ErrQuotaExceeded is returned when creating or updating a journal entry would
// exceed the configured Turso storage limit.
var ErrQuotaExceeded = errors.New("storage quota exceeded")

// QuotaExceededError provides exact byte estimates when a journal write exceeds
// the configured storage budget.
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

// EstimatedBytes returns the estimated byte footprint of a persisted journal entry.
func (j Journal) EstimatedBytes() int64 {
	return RowOverheadBytes +
		int64(len(j.MediaURL)) +
		int64(len(j.Tag1)) +
		int64(len(j.Tag2)) +
		int64(len(j.Tag3)) +
		int64(len(j.Body))
}

// EstimateInputBytes returns the estimated byte footprint of a validated or raw JournalInput.
func EstimateInputBytes(in JournalInput) int64 {
	total := RowOverheadBytes + int64(len(strings.TrimSpace(in.MediaURL))) + int64(len(in.Body))
	for i, tag := range in.Tags {
		if i >= MaxTagCount {
			break
		}
		total += int64(len(FormatTagTitleCase(tag)))
	}
	return total
}

// PendingSubmission holds an author's stashed journal form payload when a create
// or update operation is redirected to /journals/vacate-space/.
type PendingSubmission struct {
	IsEdit     bool
	EntryID    int64
	MediaURL   string
	TagsRaw    string
	Body       string
	Published  bool
	Bookmarked bool
	EntryBytes int64
}

// SetPendingSubmission stashes a pending journal form submission for resubmission after vacating space.
func (s *Store) SetPendingSubmission(sub *PendingSubmission) {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.pendingSub = sub
}

// GetPendingSubmission returns the currently stashed pending journal submission, if any.
func (s *Store) GetPendingSubmission() *PendingSubmission {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	if s.pendingSub == nil {
		return nil
	}
	cp := *s.pendingSub
	return &cp
}

// ClearPendingSubmission clears any stashed pending journal submission.
func (s *Store) ClearPendingSubmission() {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.pendingSub = nil
}

// RequiredBytesForPending calculates how many more bytes must still be vacated for the
// stashed pending submission (or returns 0 if it now fits within MaxStorageBytes).
func (s *Store) RequiredBytesForPending(ctx context.Context) (required int64, entryBytes int64, used int64, limit int64, err error) {
	used, err = s.UsedStorageBytes(ctx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	limit = s.MaxStorageBytes()
	sub := s.GetPendingSubmission()
	if sub == nil {
		if used > limit {
			return used - limit, 0, used, limit, nil
		}
		return 0, 0, used, limit, nil
	}
	entryBytes = sub.EntryBytes
	projected := used + entryBytes
	if sub.IsEdit && sub.EntryID > 0 {
		if existing, getErr := s.Get(ctx, sub.EntryID, true); getErr == nil {
			projected = (used - existing.EstimatedBytes()) + entryBytes
		}
	}
	if projected > limit {
		required = projected - limit
	}
	return required, entryBytes, used, limit, nil
}

func (s *Store) SetMaxStorageBytes(limit int64) {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.maxStorageBytes = limit
}

// MaxStorageBytes returns the effective storage limit in bytes.
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

// UsedStorageBytes returns the total estimated byte footprint across all rows in `journals`.
func (s *Store) UsedStorageBytes(ctx context.Context) (int64, error) {
	const q = `
SELECT COALESCE(SUM(
  82 +
  length(media_url) +
  length(COALESCE(tag1, '')) +
  length(COALESCE(tag2, '')) +
  length(COALESCE(tag3, '')) +
  length(body)
), 0) FROM journals;`
	var used int64
	if err := s.db.QueryRowContext(ctx, q).Scan(&used); err != nil {
		return 0, err
	}
	return used, nil
}

func (s *Store) checkQuotaForCreate(ctx context.Context, valid JournalInput) error {
	used, err := s.UsedStorageBytes(ctx)
	if err != nil {
		return err
	}
	limit := s.MaxStorageBytes()
	entryBytes := EstimateInputBytes(valid)
	if used+entryBytes > limit {
		return &QuotaExceededError{
			RequiredBytes: (used + entryBytes) - limit,
			EntryBytes:    entryBytes,
			UsedBytes:     used,
			LimitBytes:    limit,
		}
	}
	return nil
}

func (s *Store) checkQuotaForUpdate(ctx context.Context, id int64, valid JournalInput) error {
	existing, err := s.Get(ctx, id, true)
	if err != nil {
		return err
	}
	used, err := s.UsedStorageBytes(ctx)
	if err != nil {
		return err
	}
	limit := s.MaxStorageBytes()
	newBytes := EstimateInputBytes(valid)
	oldBytes := existing.EstimatedBytes()
	projected := (used - oldBytes) + newBytes
	if projected > limit {
		return &QuotaExceededError{
			RequiredBytes: projected - limit,
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
	const q = `
SELECT id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published
FROM journals
WHERE published = 0 OR bookmarked = 0
ORDER BY
  CASE WHEN published = 0 THEN 0 ELSE 1 END ASC,
  created_at ASC,
  id ASC;`

	rows, err := s.db.QueryContext(ctx, q)
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
			// When requiredBytes <= 0, still allow previewing all eligible entries if needed,
			// or break if we only want the minimum needed.
			break
		}
	}
	return plan, rows.Err()
}

// AutoVacateBytes deletes unpublished entries (prioritized, oldest first) and then
// un-bookmarked entries (oldest first) until at least requiredBytes have been freed.
func (s *Store) AutoVacateBytes(ctx context.Context, requiredBytes int64) (AutoVacatePlan, error) {
	target := requiredBytes
	if target <= 0 {
		target = 1
	}
	plan, err := s.PreviewAutoVacate(ctx, target)
	if err != nil {
		return plan, err
	}
	if len(plan.CandidateIDs) == 0 {
		return plan, nil
	}
	if err := s.Delete(ctx, plan.CandidateIDs); err != nil {
		return plan, err
	}
	return plan, nil
}
