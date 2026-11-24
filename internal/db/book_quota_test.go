package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBooksCountTowardStorageAndGrowthIsQuotaChecked(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	books := seededBookBytes(t, store)
	if used, _ := store.UsedStorageBytes(ctx); used != books {
		t.Fatalf("fresh store: usage %d should equal the seeded books %d", used, books)
	}

	// One journal entry, then a limit with 50 bytes of headroom.
	entry, err := store.Create(ctx, JournalInput{MediaURL: "https://example.com/a.png", Tags: []string{"Log"}, Body: strings.Repeat("j", 400), Published: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	used, _ := store.UsedStorageBytes(ctx)
	store.SetMaxStorageBytes(used + 50)

	// Growing Book II by 200 bytes is refused, with the exact shortfall.
	before, _ := store.GetBook(ctx, "book-ii")
	grown := before.Body + "\n\n" + strings.Repeat("b", 198)
	_, err = store.SaveBook(ctx, testCap, "book-ii", grown)
	var qe *QuotaExceededError
	if !errors.As(err, &qe) || !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("growing a book past the limit: expected QuotaExceededError, got %v", err)
	}
	if want := int64(200 - 50); qe.RequiredBytes != want {
		t.Fatalf("RequiredBytes = %d, want %d", qe.RequiredBytes, want)
	}
	if after, _ := store.GetBook(ctx, "book-ii"); after.Body != before.Body {
		t.Fatalf("refused save must not change the book")
	}

	// Hold it: Make Room must free exactly the shortfall, and only journals count.
	pb := PendingBook{Slug: "book-ii", Body: grown, BookBytes: BookBytes("book-ii", "Book II", grown)}
	if err := store.SetPendingSave(testCap, pb); err != nil {
		t.Fatalf("SetPendingSave: %v", err)
	}
	required, saveBytes, _, _, err := store.RequiredBytesForPending(ctx)
	if err != nil || required != 150 || saveBytes != pb.BookBytes {
		t.Fatalf("RequiredBytesForPending = (%d, %d, %v), want (150, %d)", required, saveBytes, err, pb.BookBytes)
	}

	// A journal write does not drop a held book save.
	if err := store.Update(ctx, entry.ID, JournalInput{MediaURL: "https://example.com/a.png", Tags: []string{"Log"}, Body: "short", Published: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, ok := store.GetPendingSave().(PendingBook); !ok {
		t.Fatalf("a journal save must not clear a held book save")
	}

	// Shrinking the journal freed space: the book now fits, saves, and the hold clears.
	if _, err := store.SaveBook(ctx, testCap, "book-ii", grown); err != nil {
		t.Fatalf("SaveBook after making room: %v", err)
	}
	if store.GetPendingSave() != nil {
		t.Fatalf("saving the held book must clear the hold")
	}

	// Shrinking a book is always allowed, even when storage is over the limit.
	store.SetMaxStorageBytes(1)
	if _, err := store.SaveBook(ctx, testCap, "book-ii", "# Book II\n\nShorter."); err != nil {
		t.Fatalf("shrinking a book while over the limit must be allowed, got %v", err)
	}
}

func TestBookPendingCannotVacateBooks(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	// No journals at all: nothing can be vacated, whatever the shortfall.
	store.SetMaxStorageBytes(seededBookBytes(t, store))
	body := strings.Repeat("b", 5000)
	if err := store.SetPendingSave(testCap, PendingBook{Slug: "book-iii", Body: body, BookBytes: BookBytes("book-iii", "Book III", body)}); err != nil {
		t.Fatalf("SetPendingSave: %v", err)
	}
	required, _, _, _, err := store.RequiredBytesForPending(ctx)
	if err != nil || required != 0 {
		t.Fatalf("with no journals the vacatable amount is 0 (books are never evicted); got required=%d err=%v", required, err)
	}
}
