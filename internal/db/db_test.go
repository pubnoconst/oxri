package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		Tags:     []string{"log", "History", "Notes123"},
		Body:     "# RES·IMPERII\n\nEnglish 123, Latin Rōma æterna, and Greek Λόγος — no emojis.",
	}
	if _, err := ValidateInput(valid); err != nil {
		t.Fatalf("expected valid ASCII tags and Latin/Greek prose to pass, got: %v", err)
	}

	cases := []struct {
		name string
		mod  func(*JournalInput)
	}{
		{"empty media url", func(in *JournalInput) { in.MediaURL = "" }},
		{"invalid media url scheme", func(in *JournalInput) { in.MediaURL = "ftp://example.com/a.mp4" }},
		{"media url over 128 chars", func(in *JournalInput) { in.MediaURL = "https://example.com/" + strings.Repeat("a", 115) }},
		{"emoji in media url", func(in *JournalInput) { in.MediaURL = "https://example.com/🚀" }},
		{"more than 3 tags", func(in *JournalInput) { in.Tags = []string{"a", "b", "c", "d"} }},
		{"tag over 8 chars", func(in *JournalInput) { in.Tags = []string{"biography"} }},
		{"duplicate tags exact", func(in *JournalInput) { in.Tags = []string{"Log", "Log"} }},
		{"duplicate tags case-insensitive", func(in *JournalInput) { in.Tags = []string{"log", "LOG"} }},
		{"emoji in tag", func(in *JournalInput) { in.Tags = []string{"😀"} }},
		{"punctuation in tag", func(in *JournalInput) { in.Tags = []string{"a-b"} }},
		{"non-ASCII accented Latin in tag", func(in *JournalInput) { in.Tags = []string{"Rōma"} }},
		{"non-ASCII Greek in tag", func(in *JournalInput) { in.Tags = []string{"Νόος"} }},
		{"non-Latin/Greek script in tag", func(in *JournalInput) { in.Tags = []string{"Тест"} }},
		{"empty body", func(in *JournalInput) { in.Body = "   \n\t " }},
		{"emoji in body", func(in *JournalInput) { in.Body = "Hello world 🚀" }},
		{"non-Latin/Greek script in body", func(in *JournalInput) { in.Body = "Hello мир" }},
		{"zero-width spoofing char in body", func(in *JournalInput) { in.Body = "Hello\u200Bworld" }},
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
	if _, err := rawDB.ExecContext(ctx, `INSERT INTO journals (id, created_at, media_url, tag1, tag2, tag3, body, bookmarked) VALUES (1, 1000, 'https://example.com/old.jpg', 'log', 'LOG', 'dev', 'Legacy row', 1);`); err != nil {
		t.Fatal(err)
	}

	store := &Store{db: rawDB}
	v0, err := store.SchemaVersion(ctx)
	if err != nil || v0 != 0 {
		t.Fatalf("expected initial schema version 0, got %d (err=%v)", v0, err)
	}

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	v2, err := store.SchemaVersion(ctx)
	if err != nil || v2 != CurrentSchemaVersion {
		t.Fatalf("expected schema version %d after migration, got %d (err=%v)", CurrentSchemaVersion, v2, err)
	}

	// Verify migrated row preserved data, normalized tags to Title Case, and cleared duplicate tag2
	migrated, err := store.Get(ctx, 1, false)
	if err != nil {
		t.Fatalf("expected migrated row id=1, got err=%v", err)
	}
	if migrated.Tag1 != "Log" || migrated.Tag2 != "" || migrated.Tag3 != "Dev" || !migrated.Published {
		t.Errorf("unexpected migrated row state: %+v", migrated)
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

	// Verify empty and file: URLs are rejected (no local disk databases allowed)
	if _, err := Open(ctx, "", ""); err == nil {
		t.Fatalf("expected Open with empty dbURL to fail")
	}
	if _, err := Open(ctx, "file:local.db", ""); err == nil {
		t.Fatalf("expected Open with file: dbURL to fail")
	}

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
		t.Errorf("expected 1 search result for 'Updated', got %+v (err=%v)", searchRes, err)
	}

	tagRes, err := store.ListFiltered(ctx, ListFilter{IncludeUnpublished: false, Tags: []string{"log"}})
	if err != nil || len(tagRes) != 1 || tagRes[0].ID != e3.ID {
		t.Errorf("expected 1 tag filter result for 'log', got %+v (err=%v)", tagRes, err)
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

func TestStorageQuotaAndAutoVacatePriority(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("Open memory db failed: %v", err)
	}
	defer store.Close()

	// Create 3 entries:
	// 1. Published + Bookmarked (must be preserved by auto-vacate)
	eBookmarked, err := store.Create(ctx, JournalInput{
		MediaURL:   "https://example.com/bm.png",
		Tags:       []string{"keep"},
		Body:       "Bookmarked published entry that must survive auto-eviction.",
		Bookmarked: true,
		Published:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Published + Un-bookmarked (priority 2 for auto-vacate)
	eUnbookmarked, err := store.Create(ctx, JournalInput{
		MediaURL:   "https://example.com/unbm.png",
		Tags:       []string{"pub"},
		Body:       "Published un-bookmarked entry evicted second after drafts.",
		Bookmarked: false,
		Published:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Unpublished (priority 1 for auto-vacate, even if bookmarked)
	eUnpublished, err := store.Create(ctx, JournalInput{
		MediaURL:   "https://example.com/draft.png",
		Tags:       []string{"wip"},
		Body:       "Unpublished draft entry evicted first by prioritized auto-vacate.",
		Bookmarked: true,
		Published:  false,
	})
	if err != nil {
		t.Fatal(err)
	}

	used, err := store.UsedStorageBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expectedUsed := eBookmarked.EstimatedBytes() + eUnbookmarked.EstimatedBytes() + eUnpublished.EstimatedBytes()
	if used != expectedUsed {
		t.Fatalf("UsedStorageBytes = %d, expected sum of entry estimates %d", used, expectedUsed)
	}

	// Cap storage at current used bytes so any new entry exceeds quota
	store.SetMaxStorageBytes(used)

	newInput := JournalInput{
		MediaURL:  "https://example.com/new.png",
		Tags:      []string{"new"},
		Body:      "Incoming entry that exceeds the storage cap.",
		Published: true,
	}
	_, err = store.Create(ctx, newInput)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded on Create, got %v", err)
	}
	var qErr *QuotaExceededError
	if !errors.As(err, &qErr) {
		t.Fatalf("expected *QuotaExceededError, got %T", err)
	}
	if qErr.RequiredBytes != EstimateInputBytes(newInput) {
		t.Errorf("RequiredBytes = %d, expected %d", qErr.RequiredBytes, EstimateInputBytes(newInput))
	}

	// Auto-vacate only enough bytes for one entry -> must delete eUnpublished first!
	plan1, err := store.AutoVacateBytes(ctx, eUnpublished.EstimatedBytes())
	if err != nil {
		t.Fatalf("AutoVacateBytes step 1 failed: %v", err)
	}
	if plan1.UnpublishedCount != 1 || plan1.UnbookmarkedCount != 0 {
		t.Errorf("expected 1 unpublished and 0 unbookmarked deleted first, got %+v", plan1)
	}
	if _, err := store.Get(ctx, eUnpublished.ID, true); err != ErrNotFound {
		t.Errorf("expected eUnpublished to be deleted first")
	}
	if _, err := store.Get(ctx, eUnbookmarked.ID, true); err != nil {
		t.Errorf("expected eUnbookmarked to still exist after step 1")
	}

	// Now newInput fits and Create succeeds!
	if _, err := store.Create(ctx, newInput); err != nil {
		t.Fatalf("expected Create to succeed after vacating unpublished entry, got %v", err)
	}

	// Auto-vacate again -> must delete eUnbookmarked next while preserving eBookmarked
	plan2, err := store.AutoVacateBytes(ctx, eUnbookmarked.EstimatedBytes())
	if err != nil {
		t.Fatalf("AutoVacateBytes step 2 failed: %v", err)
	}
	if plan2.UnpublishedCount != 0 || plan2.UnbookmarkedCount != 1 {
		t.Errorf("expected 0 unpublished and 1 unbookmarked deleted second, got %+v", plan2)
	}
	if _, err := store.Get(ctx, eBookmarked.ID, true); err != nil {
		t.Errorf("expected published bookmarked entry to be preserved, got err=%v", err)
	}
}

func TestQuotaEnforcedOnEmptyDatabaseAndSingleEntryUpdate(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// 1. Empty database (used == 0) must still reject an entry exceeding MaxStorageBytes
	store.SetMaxStorageBytes(100)
	oversized := JournalInput{
		MediaURL:  "https://example.com/large.png",
		Tags:      []string{"Log"},
		Body:      strings.Repeat("a", 120),
		Published: true,
	}
	if _, err := store.Create(ctx, oversized); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded on empty database when entry > limit, got %v", err)
	}

	// 2. Single-entry database (otherBytes == 0) must still reject an Update exceeding MaxStorageBytes
	small := JournalInput{
		MediaURL:  "https://example.com/a.png",
		Tags:      []string{"Log"},
		Body:      "Short",
		Published: true,
	}
	store.SetMaxStorageBytes(EstimateInputBytes(small) + 10)
	created, err := store.Create(ctx, small)
	if err != nil {
		t.Fatalf("Create small failed: %v", err)
	}

	if err := store.Update(ctx, created.ID, oversized); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded when updating sole entry past limit, got %v", err)
	}
}

func TestMultiByteUTF8LatinAndGreekByteEstimationMatchesSQLite(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	entry, err := store.Create(ctx, JournalInput{
		MediaURL:  "https://example.com/roma.png",
		Tags:      []string{"History", "Log2026"},
		Body:      "Rōma æterna — Λόγος καὶ ἀρετή.",
		Published: true,
	})
	if err != nil {
		t.Fatalf("Create with multi-byte Latin/Greek failed: %v", err)
	}

	usedSQL, err := store.UsedStorageBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	estGo := entry.EstimatedBytes()
	if usedSQL != estGo {
		t.Errorf("SQLite byte count (%d) != Go UTF-8 EstimatedBytes (%d) for multi-byte Latin/Greek", usedSQL, estGo)
	}
}

func TestConcurrentCreatesNeverExceedQuota(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	sample := JournalInput{
		MediaURL:  "https://example.com/item.png",
		Tags:      []string{"Log"},
		Body:      "Concurrent atomic quota verification entry.",
		Published: true,
	}
	singleBytes := EstimateInputBytes(sample)

	// Allow room for at most 3 entries
	limit := singleBytes * 3
	store.SetMaxStorageBytes(limit)

	const workers = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	quotaErrCount := 0

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, createErr := store.Create(ctx, sample)
			mu.Lock()
			defer mu.Unlock()
			if createErr == nil {
				successCount++
			} else if errors.Is(createErr, ErrQuotaExceeded) {
				quotaErrCount++
			} else {
				t.Errorf("unexpected error during concurrent Create: %v", createErr)
			}
		}()
	}
	wg.Wait()

	if successCount != 3 || quotaErrCount != 7 {
		t.Errorf("expected 3 successes and 7 ErrQuotaExceeded, got %d successes and %d quota errors", successCount, quotaErrCount)
	}

	used, err := store.UsedStorageBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if used > limit {
		t.Errorf("UsedStorageBytes (%d) exceeded limit (%d) under concurrent writes", used, limit)
	}
}
