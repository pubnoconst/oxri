package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBooksSchemaPersistenceAndSecurity(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	// 1. Default seeded rows exist for Book I, II, III
	for _, slug := range []string{"book-i", "book-ii", "book-iii"} {
		b, err := store.GetBook(ctx, slug)
		if err != nil {
			t.Fatalf("GetBook(%s): %v", slug, err)
		}
		if b.Slug != slug || b.Body == "" {
			t.Fatalf("unexpected seeded book: %+v", b)
		}
	}

	// 2. Unknown slug is rejected
	if _, err := store.GetBook(ctx, "book-iv"); !errors.Is(err, ErrInvalidBookSlug) {
		t.Fatalf("expected ErrInvalidBookSlug, got %v", err)
	}
	if _, err := store.SaveBook(ctx, testCap, "book-iv", "# Nope"); !errors.Is(err, ErrInvalidBookSlug) {
		t.Fatalf("expected ErrInvalidBookSlug on SaveBook, got %v", err)
	}

	// 3. Oversized payload (> 128 KiB) is rejected (anti-DDoS)
	huge := strings.Repeat("a", MaxBookBodyBytes+1)
	if _, err := store.SaveBook(ctx, testCap, "book-ii", huge); !errors.Is(err, ErrBookTooLarge) {
		t.Fatalf("expected ErrBookTooLarge, got %v", err)
	}

	// 4. Disallowed characters (emojis / zero-width) are rejected
	if _, err := store.SaveBook(ctx, testCap, "book-ii", "# Book II 🚀"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for emoji, got %v", err)
	}

	// 5. Valid Latin + Greek Markdown persists and updates cache
	md := "# Book II\n\n## On Λόγος\n\nA calm chapter."
	saved, err := store.SaveBook(ctx, testCap, "book-ii", md)
	if err != nil {
		t.Fatalf("SaveBook: %v", err)
	}
	if saved.Body != md {
		t.Fatalf("unexpected saved body: %q", saved.Body)
	}
	got, err := store.GetBook(ctx, "book-ii")
	if err != nil || got.Body != md {
		t.Fatalf("GetBook after save mismatch: got=%q err=%v", got.Body, err)
	}
}
