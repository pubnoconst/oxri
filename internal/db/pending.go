package db

import (
	"context"

	"github.com/oxri/oxri-go/internal/domain"
)

// PendingSave is a save held on the Make Room page (/journals/vacate-space/)
// until enough journal space is vacated. It is exactly one of PendingJournal or
// PendingBook: the interface is sealed (unexported method), so no other kind
// can exist, a half-journal/half-book state is unrepresentable, and every
// consumer must handle both kinds in a type switch.
type PendingSave interface {
	// Bytes is the storage footprint the held save will take once written.
	Bytes() int64
	pendingSave()
}

// PendingJournal is a journal create (IsEdit false) or edit held while space
// is vacated.
type PendingJournal struct {
	IsEdit     bool
	EntryID    int64
	MediaURL   string
	TagsRaw    string
	Body       string
	Published  bool
	Bookmarked bool
	EntryBytes int64
}

func (p PendingJournal) Bytes() int64 { return p.EntryBytes }
func (PendingJournal) pendingSave()   {}

// PendingBook is a book save held while journal space is vacated. Books are
// never evicted to make room; only journal entries are.
type PendingBook struct {
	Slug      string
	Body      string
	BookBytes int64
}

func (p PendingBook) Bytes() int64 { return p.BookBytes }
func (PendingBook) pendingSave()   {}

// SetPendingSave holds a save for the Make Room page, replacing any other.
func (s *Store) SetPendingSave(cap domain.AdminCap, p PendingSave) error {
	if err := requireCap(cap); err != nil {
		return err
	}
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.pendingSave = p
	return nil
}

// GetPendingSave returns the held save, or nil. Both kinds are plain values,
// so callers get a copy.
func (s *Store) GetPendingSave() PendingSave {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	return s.pendingSave
}

// clearPendingJournal drops a held journal save (after a journal write
// succeeds). A held book save is left alone.
func (s *Store) clearPendingJournal() {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	if _, ok := s.pendingSave.(PendingJournal); ok {
		s.pendingSave = nil
	}
}

// clearPendingBook drops a held save for this book (after it is written).
func (s *Store) clearPendingBook(slug string) {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	if p, ok := s.pendingSave.(PendingBook); ok && p.Slug == slug {
		s.pendingSave = nil
	}
}

// RequiredBytesForPending reports how many more bytes must be vacated for the
// held save to fit (0 when it already fits), the save's own footprint, and the
// current usage and limit. Only journal bytes can be vacated, so the result is
// capped at what vacating could actually free.
func (s *Store) RequiredBytesForPending(ctx context.Context) (required, saveBytes, used, limit int64, err error) {
	if used, err = s.UsedStorageBytes(ctx); err != nil {
		return 0, 0, 0, 0, err
	}
	vacatable, err := s.journalStorageBytes(ctx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	limit = s.MaxStorageBytes()

	projected := used
	switch p := s.GetPendingSave().(type) {
	case nil:
	case PendingJournal:
		saveBytes = p.EntryBytes
		projected = used + saveBytes
		if p.IsEdit && p.EntryID > 0 {
			// The entry being edited is replaced, not vacated.
			if existing, getErr := s.Get(ctx, p.EntryID, true); getErr == nil {
				projected -= existing.EstimatedBytes()
				vacatable -= existing.EstimatedBytes()
			}
		}
	case PendingBook:
		saveBytes = p.BookBytes
		projected = used + saveBytes
		if title, ok := BookTitle(p.Slug); ok {
			if current, getErr := s.GetBook(ctx, p.Slug); getErr == nil {
				projected -= BookBytes(p.Slug, title, current.Body)
			}
		}
	}

	if projected > limit {
		required = projected - limit
		if vacatable < 0 {
			vacatable = 0
		}
		if required > vacatable {
			required = vacatable
		}
	}
	return required, saveBytes, used, limit, nil
}
