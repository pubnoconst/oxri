package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

// The journal body cap counts characters (Unicode code points), not bytes:
// accented Latin and Greek take 2 bytes each in UTF-8 but count as one.
func TestJournalBodyLengthCap(t *testing.T) {
	const media = "https://x.com/user/status/1"
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"exactly the cap (ASCII)", strings.Repeat("a", domain.MaxJournalBodyLength), true},
		{"one over the cap (ASCII)", strings.Repeat("a", domain.MaxJournalBodyLength+1), false},
		{"exactly the cap (Greek, ~2 bytes each)", strings.Repeat("λ", domain.MaxJournalBodyLength), true},
		{"one over the cap (Greek)", strings.Repeat("λ", domain.MaxJournalBodyLength+1), false},
		{"exactly the cap (accented Latin)", strings.Repeat("ō", domain.MaxJournalBodyLength), true},
	}
	for _, tc := range cases {
		_, err := domain.ParseJournalEntry(media, []string{"Log"}, tc.body, false, true)
		if tc.ok && err != nil {
			t.Errorf("%s: expected accepted, got %v", tc.name, err)
		}
		if !tc.ok && !errors.Is(err, domain.ErrInvalidJournalInput) {
			t.Errorf("%s: expected ErrInvalidJournalInput, got %v", tc.name, err)
		}
	}
	if domain.MaxJournalBodyLength != 2000 {
		t.Fatalf("journal body cap changed to %d; update the owner-facing hint and RATIONALE", domain.MaxJournalBodyLength)
	}
}
