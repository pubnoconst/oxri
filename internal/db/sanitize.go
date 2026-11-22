package db

import (
	"strings"

	"github.com/oxri/oxri-go/internal/domain"
)

// ALL INPUTS ARE EVIL (Principia #3).
// All character-set whitelists are defined once in internal/domain and delegated here
// so there is a single canonical source of truth across domain modeling and persistence.

const (
	MaxProseRunes = domain.DefaultMaxProseLength
)

// IsAllowedMediaURLRune delegates to domain.IsAllowedMediaURLRune.
func IsAllowedMediaURLRune(r rune) bool {
	return domain.IsAllowedMediaURLRune(r)
}

// ValidateMediaURLChars delegates to domain.ValidateMediaURLChars.
func ValidateMediaURLChars(s string) (rune, bool) {
	return domain.ValidateMediaURLChars(s)
}

// IsPrivateOrLocalHost delegates to domain.IsPrivateOrLocalHost.
func IsPrivateOrLocalHost(hostname string) bool {
	return domain.IsPrivateOrLocalHost(hostname)
}

// IsAllowedTagRune delegates to domain.IsAllowedJournalTagRune.
func IsAllowedTagRune(r rune) bool {
	return domain.IsAllowedJournalTagRune(r)
}

// ValidateTagChars delegates to domain.ValidateJournalTagChars.
func ValidateTagChars(s string) (rune, bool) {
	return domain.ValidateJournalTagChars(s)
}

// IsAllowedProseRune delegates to domain.IsAllowedProseRune.
func IsAllowedProseRune(r rune) bool {
	return domain.IsAllowedProseRune(r)
}

// ValidateProseChars delegates to domain.ValidateProseChars.
func ValidateProseChars(s string) (rune, bool) {
	return domain.ValidateProseChars(s)
}

// SanitizeMediaURLInput strips any character outside printable ASCII (0x21..0x7E)
// and caps length at MaxMediaURLLength so invalid input is never echoed back to the DOM.
func SanitizeMediaURLInput(raw string) string {
	var b strings.Builder
	count := 0
	for _, r := range strings.TrimSpace(raw) {
		if count >= MaxMediaURLLength {
			break
		}
		if IsAllowedMediaURLRune(r) {
			b.WriteRune(r)
			count++
		}
	}
	return b.String()
}

// SanitizeTagsRawInput strips any character that is not an allowed tag rune, comma, or space.
func SanitizeTagsRawInput(raw string) string {
	var b strings.Builder
	count := 0
	for _, r := range strings.TrimSpace(raw) {
		if count >= 64 {
			break
		}
		if IsAllowedTagRune(r) || r == ',' || r == ' ' {
			b.WriteRune(r)
			count++
		}
	}
	return strings.TrimSpace(b.String())
}

// SanitizeProseInput strips any rune outside the allowed Latin/Greek/ASCII prose whitelist
// and caps length at MaxProseRunes.
func SanitizeProseInput(raw string) string {
	var b strings.Builder
	count := 0
	for _, r := range raw {
		if count >= MaxProseRunes {
			break
		}
		if IsAllowedProseRune(r) {
			b.WriteRune(r)
			count++
		}
	}
	return b.String()
}

// SanitizeSearchQuery strips any rune not allowed in prose/tags and caps length at 128 runes.
func SanitizeSearchQuery(q string) string {
	var b strings.Builder
	count := 0
	for _, r := range strings.TrimSpace(q) {
		if count >= 128 {
			break
		}
		if IsAllowedProseRune(r) {
			b.WriteRune(r)
			count++
		}
	}
	return strings.TrimSpace(b.String())
}
