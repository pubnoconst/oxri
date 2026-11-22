package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// IsValidBookSlug reports whether slug is one of the canonical Res Imperii books.
func IsValidBookSlug(slug string) bool {
	_, ok := canonicalBooks[slug]
	return ok
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

	s.bookCacheMu.RLock()
	if s.cachedBooks != nil {
		if b, found := s.cachedBooks[slug]; found && time.Since(s.bookCacheAt[slug]) < readCacheTTL {
			s.bookCacheMu.RUnlock()
			cp := b
			return &cp, nil
		}
	}
	s.bookCacheMu.RUnlock()

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
func (s *Store) SaveBook(ctx context.Context, slug, rawBody string) (*Book, error) {
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

	s.writeMu.Lock()
	_, execErr := s.db.ExecContext(ctx,
		`INSERT INTO books (slug, title, body, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET title = excluded.title, body = excluded.body, updated_at = excluded.updated_at;`,
		slug, title, body, now.UnixMilli(),
	)
	s.writeMu.Unlock()
	if execErr != nil {
		return nil, execErr
	}

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
