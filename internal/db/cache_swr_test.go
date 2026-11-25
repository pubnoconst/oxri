package db

import (
	"context"
	"testing"
	"time"
)

// The entry snapshot is stale-while-revalidate: past readCacheTTL a reader
// is served the old snapshot at once (never waits on the database), one
// background refresh brings in what another server wrote, and a write made
// here still shows at once.
func TestEntrySnapshotIsStaleWhileRevalidate(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.Create(ctx, JournalInput{MediaURL: "https://example.org/a.png", Body: "First", Published: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.List(ctx, true); len(got) != 1 {
		t.Fatalf("after a local write: %d entries, want 1", len(got))
	}

	// Another server writes straight to the database; this snapshot is now
	// past its TTL.
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO journals (id, created_at, media_url, body, bookmarked, published) VALUES (?, ?, ?, ?, 0, 1);`,
		store.nextID(), time.Now().UnixMilli(), "https://example.org/b.png", "Second"); err != nil {
		t.Fatal(err)
	}
	store.cacheMu.Lock()
	store.cacheLoadedAt = time.Now().Add(-2 * readCacheTTL)
	store.cacheMu.Unlock()

	if got, _ := store.List(ctx, true); len(got) != 1 {
		t.Fatalf("a stale snapshot must be served at once: got %d entries, want the old 1", len(got))
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got, _ := store.List(ctx, true); len(got) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the background refresh never brought in the other server's entry")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A book past its TTL is served at once and refreshed in the background.
func TestBookCacheIsStaleWhileRevalidate(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if _, err := store.SaveBook(ctx, testCap, "book-ii", "# Book II\n\nOld."); err != nil {
		t.Fatal(err)
	}
	if b, _ := store.GetBook(ctx, "book-ii"); b == nil || b.Body != "# Book II\n\nOld." {
		t.Fatalf("after a local save: %+v", b)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE books SET body = ? WHERE slug = 'book-ii';`, "# Book II\n\nNew."); err != nil {
		t.Fatal(err)
	}
	store.bookCacheMu.Lock()
	store.bookCacheAt["book-ii"] = time.Now().Add(-2 * readCacheTTL)
	store.bookCacheMu.Unlock()

	if b, _ := store.GetBook(ctx, "book-ii"); b == nil || b.Body != "# Book II\n\nOld." {
		t.Fatalf("a stale book must be served at once: %+v", b)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, _ := store.GetBook(ctx, "book-ii"); b != nil && b.Body == "# Book II\n\nNew." {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the background refresh never brought in the other server's edit")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
