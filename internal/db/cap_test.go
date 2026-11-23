package db

import (
	"context"

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
