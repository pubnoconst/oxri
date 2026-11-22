package domain_test

import (
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestParseProse(t *testing.T) {
	// Normal prose
	raw := "This is a refuge for thoughts.\r\nIt has multiple lines.\n"
	p, err := domain.ParseProse(raw, 500)
	if err != nil {
		t.Fatalf("ParseProse unexpected error: %v", err)
	}

	expected := "This is a refuge for thoughts.\nIt has multiple lines."
	if p.String() != expected {
		t.Errorf("ParseProse normalized string = %q, want %q", p.String(), expected)
	}

	if p.Len() != len([]rune(expected)) {
		t.Errorf("ParseProse.Len() = %d, want %d", p.Len(), len([]rune(expected)))
	}

	// Reject null bytes
	withNull := "malicious\x00content"
	_, err = domain.ParseProse(withNull, 500)
	if err != domain.ErrProseNullBytes {
		t.Errorf("ParseProse with null byte expected ErrProseNullBytes, got %v", err)
	}

	// Reject length exceeding maxLen
	tooLong := strings.Repeat("x", 101)
	_, err = domain.ParseProse(tooLong, 100)
	if err == nil {
		t.Errorf("ParseProse exceeding maxLen expected error, got nil")
	}

	// Allow Latin, Greek, numbers, and standard punctuation
	if _, err := domain.ParseProse("Rōma æterna, Λόγος 123 — RES·IMPERII.", 500); err != nil {
		t.Errorf("ParseProse rejected valid Latin/Greek/numbers: %v", err)
	}

	// Reject emojis and non-Latin/Greek scripts
	for _, bad := range []string{"Hello 🚀", "Hello мир", "Zero\u200BWidth"} {
		if _, err := domain.ParseProse(bad, 500); err == nil {
			t.Errorf("ParseProse(%q) expected error for disallowed characters, got nil", bad)
		}
	}
}
