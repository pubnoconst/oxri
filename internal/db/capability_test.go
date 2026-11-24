package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/domain"
)

// Every administrative write must refuse a zero (forged) AdminCap and leave
// the database exactly as it was.
func TestAdminWritesRejectZeroCap(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry, err := store.Create(ctx, JournalInput{
		MediaURL:  "https://x.com/user/status/123",
		Tags:      []string{"Log"},
		Body:      "Seed entry.",
		Published: true,
	})
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	const rpID = "localhost"
	if err := store.SavePasskey(ctx, testCap, PasskeyRecord{
		ID: "cred-1", RPID: rpID, Name: "Key", CredentialJSON: `{"id":"AQID"}`, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed SavePasskey: %v", err)
	}
	bookBefore, err := store.GetBook(ctx, "book-ii")
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	validated, err := ParseValidatedEntry(JournalInput{
		MediaURL: "https://x.com/user/status/456", Tags: []string{"Log"}, Body: "Forged.",
	})
	if err != nil {
		t.Fatalf("ParseValidatedEntry: %v", err)
	}

	var forged domain.AdminCap // the zero value: no authority
	writes := map[string]func() error{
		"SaveBook":        func() error { _, err := store.SaveBook(ctx, forged, "book-ii", "# Forged"); return err },
		"CreateValidated": func() error { _, err := store.CreateValidated(ctx, forged, validated); return err },
		"UpdateValidated": func() error { return store.UpdateValidated(ctx, forged, entry.ID, validated) },
		"SetPublished":    func() error { return store.SetPublished(ctx, forged, []int64{entry.ID}, false) },
		"SetAllPublished": func() error { return store.SetAllPublished(ctx, forged, false) },
		"ToggleBookmark":  func() error { _, err := store.ToggleBookmark(ctx, forged, entry.ID); return err },
		"Delete":          func() error { return store.Delete(ctx, forged, []int64{entry.ID}) },
		"DeleteAll":       func() error { return store.DeleteAll(ctx, forged) },
		"SavePasskey": func() error {
			return store.SavePasskey(ctx, forged, PasskeyRecord{ID: "cred-2", RPID: rpID, Name: "X", CredentialJSON: `{}`, CreatedAt: time.Now().UTC()})
		},
		"DeletePasskey":   func() error { return store.DeletePasskey(ctx, forged, "cred-1", rpID) },
		"AutoVacateBytes": func() error { _, err := store.AutoVacateBytes(ctx, forged, 1); return err },
		"SetPendingSave":  func() error { return store.SetPendingSave(forged, PendingJournal{Body: "x"}) },
	}
	for name, write := range writes {
		if err := write(); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("%s with a zero AdminCap: expected ErrUnauthorized, got %v", name, err)
		}
	}

	// Nothing changed.
	all, err := store.List(ctx, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 || all[0].ID != entry.ID || !all[0].Published || all[0].Bookmarked || all[0].Body != "Seed entry." {
		t.Errorf("journals changed by forged writes: %+v", all)
	}
	if n, err := store.CountPasskeys(ctx, rpID); err != nil || n != 1 {
		t.Errorf("passkeys changed by forged writes: count=%d err=%v", n, err)
	}
	if bookAfter, err := store.GetBook(ctx, "book-ii"); err != nil || bookAfter.Body != bookBefore.Body {
		t.Errorf("book changed by forged write")
	}
	if store.GetPendingSave() != nil {
		t.Errorf("pending submission stored by forged write")
	}
}
