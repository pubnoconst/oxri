package domain_test

import (
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestParseSafePath(t *testing.T) {
	valid := []string{
		"journal-2026-11-22",
		"book1-ch1",
		"refuge_notes",
		"a",
		"entry-123_test",
	}

	for _, s := range valid {
		sp, err := domain.ParseSafePath(s)
		if err != nil {
			t.Fatalf("ParseSafePath(%q) unexpected error: %v", s, err)
		}
		if sp.String() != s {
			t.Errorf("ParseSafePath(%q).String() = %q, want %q", s, sp.String(), s)
		}
		if sp.IsZero() {
			t.Errorf("ParseSafePath(%q).IsZero() returned true", s)
		}
	}

	invalid := []string{
		"",                      // empty
		"   ",                   // whitespace
		"../traversal",          // path traversal
		"..",                    // directory traversal
		"/root/path",            // unix root
		`C:\Windows\System32`,   // windows drive
		"sub/folder",            // subfolder slash
		`sub\folder`,            // backslash
		"-leading-dash",         // must start with alphanumeric
		"_leading-underscore",   // must start with alphanumeric
		"entry with spaces",     // spaces
		"file\x00null",          // null byte
		"entry.html",            // dot not in regex
		strings.Repeat("a", 81), // >80 characters
	}

	for _, s := range invalid {
		_, err := domain.ParseSafePath(s)
		if err == nil {
			t.Errorf("ParseSafePath(%q) expected error, got nil", s)
		}
	}
}
