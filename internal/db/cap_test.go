package db

import (
	"context"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

// testCap is the administrative capability used by this package's tests.
// The minter can be claimed once per process; nothing else in the db test
// binary claims it (internal/auth, the production claimant, is not imported).
var testCap = func() domain.AdminCap {
	mint := domain.SetupCapMinter()
	if mint == nil {
		panic("db tests: AdminCap minter already claimed")
	}
	return mint()
}()

// Create and Update are test-only conveniences over the single production
// write path (ParseValidatedEntry + CreateValidated/UpdateValidated), keeping
// the table-driven validation tests readable.
func (s *Store) Create(ctx context.Context, in JournalInput) (*Journal, error) {
	entry, err := ParseValidatedEntry(in)
	if err != nil {
		return nil, err
	}
	return s.CreateValidated(ctx, testCap, entry)
}

func (s *Store) Update(ctx context.Context, id int64, in JournalInput) error {
	entry, err := ParseValidatedEntry(in)
	if err != nil {
		return err
	}
	return s.UpdateValidated(ctx, testCap, id, entry)
}

// seededBookBytes is the storage the three seeded books use; books count
// toward the budget, so quota tests build their limits on top of it.
func seededBookBytes(t *testing.T, s *Store) int64 {
	t.Helper()
	var total int64
	for _, slug := range []string{"book-i", "book-ii", "book-iii"} {
		b, err := s.GetBook(context.Background(), slug)
		if err != nil {
			t.Fatalf("GetBook(%s): %v", slug, err)
		}
		total += BookBytes(b.Slug, b.Title, b.Body)
	}
	return total
}
