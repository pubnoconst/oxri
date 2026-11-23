package db

import (
	"context"
	"testing"
	"time"
)

func TestAuthStateTOTPWatermarkPersistence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	step, err := store.LoadLastTOTPStep(ctx)
	if err != nil {
		t.Fatalf("LoadLastTOTPStep(initial): %v", err)
	}
	if step != -1 {
		t.Fatalf("expected initial TOTP step -1, got %d", step)
	}

	if err := store.SaveLastTOTPStep(ctx, 100); err != nil {
		t.Fatalf("SaveLastTOTPStep(100): %v", err)
	}
	// Monotonic check: lower step must not overwrite higher watermark
	if err := store.SaveLastTOTPStep(ctx, 90); err != nil {
		t.Fatalf("SaveLastTOTPStep(90): %v", err)
	}
	got, err := store.LoadLastTOTPStep(ctx)
	if err != nil || got != 100 {
		t.Fatalf("expected watermark to remain 100, got %d (err=%v)", got, err)
	}

	if err := store.SaveLastTOTPStep(ctx, 105); err != nil {
		t.Fatalf("SaveLastTOTPStep(105): %v", err)
	}
	got, err = store.LoadLastTOTPStep(ctx)
	if err != nil || got != 105 {
		t.Fatalf("expected watermark to advance to 105, got %d (err=%v)", got, err)
	}
}

func TestExportAndRestoreSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	srcStore, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open(src): %v", err)
	}
	defer func() { _ = srcStore.Close() }()

	created, err := srcStore.Create(ctx, JournalInput{
		MediaURL:   "https://example.com/archive.png",
		Tags:       []string{"Arch"},
		Body:       "Canonical journal record preserved across backup/restore.",
		Bookmarked: true,
		Published:  true,
	})
	if err != nil {
		t.Fatalf("Create journal: %v", err)
	}

	if _, err := srcStore.SaveBook(ctx, testCap, "book-i", "# Book I\n\nRestored philosophical treatise."); err != nil {
		t.Fatalf("SaveBook: %v", err)
	}

	if err := srcStore.SavePasskey(ctx, testCap, PasskeyRecord{
		ID:             "cred-backup-1",
		RPID:           "res-imperii.onrender.com",
		Name:           "Proton Pass",
		CredentialJSON: `{"id":"AQID"}`,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SavePasskey: %v", err)
	}

	snap, err := srcStore.ExportSnapshot(ctx)
	if err != nil {
		t.Fatalf("ExportSnapshot: %v", err)
	}
	if len(snap.Journals) != 1 || len(snap.Books) != 3 || len(snap.Passkeys) != 1 {
		t.Fatalf("unexpected snapshot counts: journals=%d books=%d passkeys=%d",
			len(snap.Journals), len(snap.Books), len(snap.Passkeys))
	}

	dstStore, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open(dst): %v", err)
	}
	defer func() { _ = dstStore.Close() }()

	if err := dstStore.RestoreSnapshot(ctx, snap); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}

	gotJournal, err := dstStore.Get(ctx, created.ID, true)
	if err != nil {
		t.Fatalf("Get restored journal: %v", err)
	}
	if gotJournal.Body != created.Body || !gotJournal.Bookmarked || gotJournal.Tag1 != "Arch" {
		t.Fatalf("restored journal mismatch: %+v", gotJournal)
	}

	gotBook, err := dstStore.GetBook(ctx, "book-i")
	if err != nil {
		t.Fatalf("GetBook restored: %v", err)
	}
	if gotBook.Body != "# Book I\n\nRestored philosophical treatise." {
		t.Fatalf("restored book body mismatch: %q", gotBook.Body)
	}

	pks, err := dstStore.ListPasskeys(ctx, "res-imperii.onrender.com")
	if err != nil || len(pks) != 1 || pks[0].ID != "cred-backup-1" {
		t.Fatalf("restored passkeys mismatch: %+v (err=%v)", pks, err)
	}
}
