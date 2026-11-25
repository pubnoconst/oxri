package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/oxri/oxri-go/internal/domain"
	"strings"
	"time"
)

const (
	// MaxBookBodyBytes caps each book's Markdown body at 128 KiB to prevent
	// memory/parser DDoS and keep storage predictable.
	MaxBookBodyBytes = 128 * 1024
)

var (
	ErrInvalidBookSlug = errors.New("unknown book slug")
	ErrBookTooLarge    = errors.New("book content exceeds 128 KiB limit")
)

// Book represents a persisted Res Imperii Book (Book I, II, or III).
type Book struct {
	Slug      string
	Title     string
	Body      string
	UpdatedAt time.Time
}

var canonicalBooks = map[string]string{
	"book-i":   "Book I",
	"book-ii":  "Book II",
	"book-iii": "Book III",
}

// BookTitle returns the canonical title ("Book I", "Book II", "Book III") for slug.
func BookTitle(slug string) (string, bool) {
	t, ok := canonicalBooks[slug]
	return t, ok
}

func defaultBooksSeed() []Book {
	now := time.Now().UTC()
	return []Book{
		{Slug: "book-i", Title: "Book I", Body: "# Book I\n\n*Forthcoming.*", UpdatedAt: now},
		{Slug: "book-ii", Title: "Book II", Body: "# Book II\n\n*Forthcoming.*", UpdatedAt: now},
		{Slug: "book-iii", Title: "Book III", Body: "# Book III\n\n*Forthcoming.*", UpdatedAt: now},
	}
}

// ValidateBookBody enforces strict size and character-set invariants on Book Markdown.
func ValidateBookBody(raw string) (string, error) {
	body := strings.ReplaceAll(raw, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%w: book content cannot be empty", ErrInvalidInput)
	}
	if len(body) > MaxBookBodyBytes {
		return "", ErrBookTooLarge
	}
	if _, ok := ValidateProseChars(body); !ok {
		return "", fmt.Errorf("%w: only Latin, Greek, numbers, and standard punctuation are allowed (no emojis)", ErrInvalidInput)
	}
	return body, nil
}

// GetBook returns the persisted Book for slug from the warm in-memory cache or Turso.
func (s *Store) GetBook(ctx context.Context, slug string) (*Book, error) {
	title, ok := canonicalBooks[slug]
	if !ok {
		return nil, ErrInvalidBookSlug
	}
	if s == nil || s.db == nil {
		return nil, errors.New("database store is not initialized")
	}

	// Stale-while-revalidate, as allEntriesCached: a cached book is served at
	// once, and once older than readCacheTTL one background read refreshes it.
	s.bookCacheMu.RLock()
	if s.cachedBooks != nil {
		if b, found := s.cachedBooks[slug]; found {
			stale := time.Since(s.bookCacheAt[slug]) >= readCacheTTL
			s.bookCacheMu.RUnlock()
			if stale {
				s.refreshBookInBackground(slug)
			}
			cp := b
			return &cp, nil
		}
	}
	s.bookCacheMu.RUnlock()
	return s.loadBook(ctx, slug, title)
}

// refreshBookInBackground rereads one book once, however many readers find
// it stale.
func (s *Store) refreshBookInBackground(slug string) {
	if _, running := s.booksRefreshing.LoadOrStore(slug, struct{}{}); running {
		return
	}
	go func() {
		defer s.booksRefreshing.Delete(slug)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = s.loadBook(ctx, slug, canonicalBooks[slug])
	}()
}

// loadBook reads one book from the database into the cache.
func (s *Store) loadBook(ctx context.Context, slug, title string) (*Book, error) {

	var b Book
	var updatedMs int64
	err := s.db.QueryRowContext(ctx,
		`SELECT slug, title, body, updated_at FROM books WHERE slug = ?;`, slug,
	).Scan(&b.Slug, &b.Title, &b.Body, &updatedMs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &Book{
				Slug:      slug,
				Title:     title,
				Body:      fmt.Sprintf("# %s\n\n*Forthcoming.*", title),
				UpdatedAt: time.Now().UTC(),
			}, nil
		}
		return nil, err
	}
	b.UpdatedAt = time.UnixMilli(updatedMs).UTC()

	s.bookCacheMu.Lock()
	if s.cachedBooks == nil {
		s.cachedBooks = make(map[string]Book)
		s.bookCacheAt = make(map[string]time.Time)
	}
	s.cachedBooks[slug] = b
	s.bookCacheAt[slug] = time.Now()
	s.bookCacheMu.Unlock()

	cp := b
	return &cp, nil
}

// SaveBook validates and persists a Book's Markdown body into Turso and invalidates its read cache.
func (s *Store) SaveBook(ctx context.Context, cap domain.AdminCap, slug, rawBody string) (*Book, error) {
	if err := requireCap(cap); err != nil {
		return nil, err
	}
	title, ok := canonicalBooks[slug]
	if !ok {
		return nil, ErrInvalidBookSlug
	}
	if s == nil || s.db == nil {
		return nil, errors.New("database store is not initialized")
	}

	body, err := ValidateBookBody(rawBody)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// Books count toward the storage budget. The check and the write share one
	// transaction (as journals do), and only a save that grows the book can be
	// refused: shrinking a book is always allowed, even when storage is over.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	used, err := usedStorageBytesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	var oldBody string
	var oldBytes int64
	if scanErr := tx.QueryRowContext(ctx, `SELECT body FROM books WHERE slug = ?;`, slug).Scan(&oldBody); scanErr == nil {
		oldBytes = BookBytes(slug, title, oldBody)
	}
	newBytes := BookBytes(slug, title, body)
	limit := s.MaxStorageBytes()
	if projected := used - oldBytes + newBytes; newBytes > oldBytes && projected > limit {
		return nil, &QuotaExceededError{RequiredBytes: projected - limit, EntryBytes: newBytes, UsedBytes: used, LimitBytes: limit}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO books (slug, title, body, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET title = excluded.title, body = excluded.body, updated_at = excluded.updated_at;`,
		slug, title, body, now.UnixMilli(),
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.clearPendingBook(slug)

	b := Book{
		Slug:      slug,
		Title:     title,
		Body:      body,
		UpdatedAt: now,
	}

	s.bookCacheMu.Lock()
	if s.cachedBooks == nil {
		s.cachedBooks = make(map[string]Book)
		s.bookCacheAt = make(map[string]time.Time)
	}
	s.cachedBooks[slug] = b
	s.bookCacheAt[slug] = time.Now()
	s.bookCacheMu.Unlock()

	return &b, nil
}
