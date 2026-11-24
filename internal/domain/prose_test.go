package domain_test

import (
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

// The prose character policy (journal bodies and books): Latin, Greek,
// numbers and standard punctuation; never emoji, other scripts, or invisible
// zero-width and bidi characters.
func TestProseCharacterPolicy(t *testing.T) {
	allowed := func(s string) bool {
		for _, r := range s {
			if !domain.IsAllowedProseRune(r) {
				return false
			}
		}
		return true
	}
	if !allowed("Rōma æterna, Λόγος 123 — RES·IMPERII.\n\t«ἀρχή» … ‘quoted’") {
		t.Error("valid Latin, Greek, numbers and punctuation were refused")
	}
	for _, bad := range []string{"Hello 🚀", "Hello мир", "Zero\u200bWidth", "bidi\u202etext", "nul\x00byte"} {
		if allowed(bad) {
			t.Errorf("%q was allowed", bad)
		}
	}
}
