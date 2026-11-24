package db

import (
	"context"
	"reflect"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func testPassageJSON(t *testing.T) string {
	t.Helper()
	enc, err := domain.EncodePassage(domain.Passage{
		Translation: "eng_kjv", Book: "JHN", Chapter: 3, From: 16, To: 16,
		Reference: "John 3:16", Version: "KJV",
		Text: "For God so loved the world, that he gave his only begotten Son",
	})
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

func TestPassageStoredWithEntry(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	enc := testPassageJSON(t)
	created, err := store.Create(ctx, JournalInput{
		MediaURL: "https://bible.helloao.org/api/eng_kjv/JHN/3.json", Body: "Love.", Published: true, PassageJSON: enc,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, created.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := got.Passage()
	if !ok || got.PassageJSON != enc || p.Reference != "John 3:16" {
		t.Fatalf("passage = %+v, %v (json %q)", p, ok, got.PassageJSON)
	}

	// The passage counts against the quota, in Go and in SQL alike.
	used, err := store.UsedStorageBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	books, err := store.db.QueryContext(ctx, bookStorageBytesSQL)
	if err != nil {
		t.Fatal(err)
	}
	var bookBytes int64
	for books.Next() {
		_ = books.Scan(&bookBytes)
	}
	books.Close()
	if used-bookBytes != got.EstimatedBytes() {
		t.Errorf("SQL counts %d journal bytes, Go %d", used-bookBytes, got.EstimatedBytes())
	}

	// A backup carries it.
	snap, err := store.ExportSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dst.Close() }()
	if err := dst.RestoreSnapshot(ctx, snap); err != nil {
		t.Fatal(err)
	}
	restored, err := dst.Get(ctx, created.ID, true)
	if err != nil || restored.PassageJSON != enc {
		t.Fatalf("restored passage %q, %v", restored.PassageJSON, err)
	}

	// An entry without one stays without.
	plain, err := store.Create(ctx, JournalInput{MediaURL: "https://example.com/a.png", Body: "Plain.", Published: true})
	if err != nil {
		t.Fatal(err)
	}
	if g, _ := store.Get(ctx, plain.ID, true); g.PassageJSON != "" {
		t.Errorf("plain entry has passage %q", g.PassageJSON)
	}
}

func TestPassageMigrationAddsColumn(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	// Back to a v7 database: no passage column.
	for _, q := range []string{`ALTER TABLE journals DROP COLUMN passage;`, `DELETE FROM schema_migrations WHERE version >= 8;`} {
		if _, err := store.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if v, _ := store.SchemaVersion(ctx); v != CurrentSchemaVersion {
		t.Errorf("schema version %d", v)
	}
	if _, err := store.Create(ctx, JournalInput{
		MediaURL: "https://bible.helloao.org/api/eng_kjv/JHN/3.json", Body: "x", PassageJSON: testPassageJSON(t),
	}); err != nil {
		t.Fatalf("create after migration: %v", err)
	}
}

func TestRestoreRejectsBadPassage(t *testing.T) {
	_, err := ParseValidatedEntry(JournalInput{MediaURL: "https://example.com/a.png", Body: "x", PassageJSON: `{"ref":"","text":"y"}`})
	if err == nil {
		t.Fatal("invalid stored passage accepted")
	}
	in := JournalInput{MediaURL: "https://example.com/a.png", Body: "x", PassageJSON: testPassageJSON(t)}
	e, err := ParseValidatedEntry(in)
	if err != nil || !reflect.DeepEqual(e.PassageJSON(), in.PassageJSON) {
		t.Fatalf("valid stored passage: %v", err)
	}
}
