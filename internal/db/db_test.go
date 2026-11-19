package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvFiles(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "db.env")
	content := "# comment\nTEST_TURSO_URL=\"libsql://example.turso.io\"\nTEST_TURSO_TOKEN='secret-token'\n"
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	os.Unsetenv("TEST_TURSO_URL")
	os.Unsetenv("TEST_TURSO_TOKEN")
	defer os.Unsetenv("TEST_TURSO_URL")
	defer os.Unsetenv("TEST_TURSO_TOKEN")

	if err := LoadEnvFiles(envPath, filepath.Join(dir, "nonexistent.env")); err != nil {
		t.Fatalf("LoadEnvFiles failed: %v", err)
	}

	if got := os.Getenv("TEST_TURSO_URL"); got != "libsql://example.turso.io" {
		t.Errorf("expected libsql://example.turso.io, got %q", got)
	}
	if got := os.Getenv("TEST_TURSO_TOKEN"); got != "secret-token" {
		t.Errorf("expected secret-token, got %q", got)
	}
}

func TestValidateInput(t *testing.T) {
	valid := JournalInput{
		MediaURL: "https://x.com/user/status/123",
		Tags:     []string{"log", "dev", "go"},
		Body:     "# Hello World",
	}
	if _, err := ValidateInput(valid); err != nil {
		t.Fatalf("expected valid input to pass, got: %v", err)
	}

	cases := []struct {
		name string
		mod  func(*JournalInput)
	}{
		{"empty media url", func(in *JournalInput) { in.MediaURL = "" }},
		{"invalid media url scheme", func(in *JournalInput) { in.MediaURL = "ftp://example.com/a.mp4" }},
		{"media url over 128 chars", func(in *JournalInput) { in.MediaURL = "https://example.com/" + strings.Repeat("a", 115) }},
		{"more than 3 tags", func(in *JournalInput) { in.Tags = []string{"a", "b", "c", "d"} }},
		{"tag over 4 chars", func(in *JournalInput) { in.Tags = []string{"notes"} }},
		{"duplicate tags exact", func(in *JournalInput) { in.Tags = []string{"Log", "Log"} }},
		{"duplicate tags case-insensitive", func(in *JournalInput) { in.Tags = []string{"log", "LOG"} }},
		{"empty body", func(in *JournalInput) { in.Body = "   \n\t " }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			tc.mod(&in)
			if _, err := ValidateInput(in); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestSchemaMigrationAddsPublishedColumn(t *testing.T) {
	ctx := context.Background()
	rawDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	// Simulate existing Turso table without `published` column
	const legacySchema = `
CREATE TABLE journals (
  id INTEGER PRIMARY KEY,
  created_at INTEGER NOT NULL DEFAULT (unixepoch() * 1000),
  media_url TEXT NOT NULL CHECK(length(media_url) <= 128 AND length(media_url) > 0),
  tag1 TEXT CHECK(tag1 IS NULL OR length(tag1) <= 4),
  tag2 TEXT CHECK(tag2 IS NULL OR length(tag2) <= 4),
  tag3 TEXT CHECK(tag3 IS NULL OR length(tag3) <= 4),
  body TEXT NOT NULL,
  bookmarked INTEGER NOT NULL DEFAULT 0 CHECK(bookmarked IN (0,1))
);`
	if _, err := rawDB.ExecContext(ctx, legacySchema); err != nil {
		t.Fatal(err)
	}

	store := &Store{db: rawDB}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify idempotent re-run
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("second EnsureSchema failed: %v", err)
	}

	// Insert should work cleanly with published flag
	entry, err := store.Create(ctx, JournalInput{
		MediaURL:  "https://example.com/photo.jpg",
		Tags:      []string{"pic"},
		Body:      "Legacy migration test",
		Published: true,
	})
	if err != nil {
		t.Fatalf("Create after migration failed: %v", err)
	}
	if !entry.Published {
		t.Errorf("expected Published=true")
	}
}

func TestStoreCRUDAndBatchLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("Open memory db failed: %v", err)
	}
	defer store.Close()

	// 1. Create entries
	e1, err := store.Create(ctx, JournalInput{
		MediaURL:   "https://example.com/one.png",
		Tags:       []string{"one", "two"},
		Body:       "First entry",
		Bookmarked: false,
		Published:  true,
	})
	if err != nil {
		t.Fatalf("Create e1 failed: %v", err)
	}

	e2, err := store.Create(ctx, JournalInput{
		MediaURL:   "https://example.com/two.png",
		Tags:       []string{"wip"},
		Body:       "Second entry (unpublished)",
		Bookmarked: true,
		Published:  false,
	})
	if err != nil {
		t.Fatalf("Create e2 failed: %v", err)
	}

	e3, err := store.Create(ctx, JournalInput{
		MediaURL:   "https://example.com/three.png",
		Tags:       []string{"log", "dev", "go"},
		Body:       "Third entry",
		Bookmarked: false,
		Published:  true,
	})
	if err != nil {
		t.Fatalf("Create e3 failed: %v", err)
	}

	// 2. Public list excludes unpublished
	pubList, err := store.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(pubList) != 2 {
		t.Fatalf("expected 2 published entries, got %d", len(pubList))
	}

	// 3. Admin list includes unpublished
	allList, err := store.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(allList) != 3 {
		t.Fatalf("expected 3 total entries, got %d", len(allList))
	}

	// 4. Get single entry (public hides unpublished)
	if _, err := store.Get(ctx, e2.ID, false); err != ErrNotFound {
		t.Errorf("expected ErrNotFound for unpublished public Get, got %v", err)
	}
	gotE2, err := store.Get(ctx, e2.ID, true)
	if err != nil || gotE2.Body != "Second entry (unpublished)" {
		t.Errorf("expected e2 for admin Get, got %v, err=%v", gotE2, err)
	}

	// 5. Update entry
	err = store.Update(ctx, e2.ID, JournalInput{
		MediaURL:   "https://example.com/updated.png",
		Tags:       []string{"edit"},
		Body:       "Updated second entry",
		Bookmarked: false,
		Published:  true,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	updated, err := store.Get(ctx, e2.ID, false)
	if err != nil || !updated.Published || updated.Tag1 != "Edit" || updated.Body != "Updated second entry" {
		t.Errorf("Update verification failed: %+v, err=%v", updated, err)
	}

	// 5b. DistinctTags & ListFiltered (search + tag filter)
	tags, err := store.DistinctTags(ctx, false)
	if err != nil {
		t.Fatalf("DistinctTags failed: %v", err)
	}
	if len(tags) < 3 {
		t.Errorf("expected at least 3 distinct tags, got %v", tags)
	}

	searchRes, err := store.ListFiltered(ctx, ListFilter{IncludeUnpublished: false, Query: "Updated"})
	if err != nil || len(searchRes) != 1 || searchRes[0].ID != e2.ID {
		t.Errorf("expected search for 'Updated' to return e2, got %+v, err=%v", searchRes, err)
	}

	tagRes, err := store.ListFiltered(ctx, ListFilter{IncludeUnpublished: false, Tags: []string{"log", "dev"}})
	if err != nil || len(tagRes) != 1 || tagRes[0].ID != e3.ID {
		t.Errorf("expected tag filter [Log, Dev] to return e3, got %+v, err=%v", tagRes, err)
	}

	// 5c. Verify DB-level CHECK constraints reject non-Title-Case or duplicate tags
	_, rawErr1 := store.db.ExecContext(ctx, `INSERT INTO journals (id, created_at, media_url, tag1, body) VALUES (999, 999, 'https://example.com/a', 'lowercase', 'body');`)
	if rawErr1 == nil {
		t.Errorf("expected SQLite CHECK constraint to reject non-Title-Case tag")
	}
	_, rawErr2 := store.db.ExecContext(ctx, `INSERT INTO journals (id, created_at, media_url, tag1, tag2, body) VALUES (998, 998, 'https://example.com/a', 'Log', 'Log', 'body');`)
	if rawErr2 == nil {
		t.Errorf("expected SQLite CHECK constraint to reject duplicate tag1 and tag2")
	}

	// 6. ToggleBookmark
	bm, err := store.ToggleBookmark(ctx, e1.ID)
	if err != nil || !bm {
		t.Errorf("expected bookmark true, got %v, err=%v", bm, err)
	}

	// 7. Batch SetPublished (unpublish e1 and e3)
	if err := store.SetPublished(ctx, []int64{e1.ID, e3.ID}, false); err != nil {
		t.Fatal(err)
	}
	pubAfterUnpub, _ := store.List(ctx, false)
	if len(pubAfterUnpub) != 1 {
		t.Errorf("expected 1 published entry remaining, got %d", len(pubAfterUnpub))
	}

	// 8. SetAllPublished(true)
	if err := store.SetAllPublished(ctx, true); err != nil {
		t.Fatal(err)
	}
	pubAll, _ := store.List(ctx, false)
	if len(pubAll) != 3 {
		t.Errorf("expected 3 published entries, got %d", len(pubAll))
	}

	// 9. Delete selected
	if err := store.Delete(ctx, []int64{e1.ID, e2.ID}); err != nil {
		t.Fatal(err)
	}
	remaining, _ := store.List(ctx, true)
	if len(remaining) != 1 || remaining[0].ID != e3.ID {
		t.Errorf("expected only e3 remaining, got %+v", remaining)
	}

	// 10. DeleteAll
	if err := store.DeleteAll(ctx); err != nil {
		t.Fatal(err)
	}
	empty, _ := store.List(ctx, true)
	if len(empty) != 0 {
		t.Errorf("expected 0 entries after DeleteAll, got %d", len(empty))
	}
}
