package db

import (
	"strings"
	"unicode"
)

// ALL INPUTS ARE EVIL.
// Strict character-set whitelisting enforces that only English alphanumeric,
// Latin, and Greek characters (plus context-appropriate ASCII/typographic
// punctuation) are accepted. Emojis, control characters, zero-width/bidi
// spoofing characters, and non-whitelisted scripts are rejected.

// IsAllowedMediaURLRune permits only printable ASCII characters (0x21..0x7E) in URLs.
func IsAllowedMediaURLRune(r rune) bool {
	return r >= 0x21 && r <= 0x7E
}

// ValidateMediaURLChars checks that every rune in a media URL is printable ASCII.
func ValidateMediaURLChars(s string) (rune, bool) {
	for _, r := range s {
		if !IsAllowedMediaURLRune(r) {
			return r, false
		}
	}
	return 0, true
}

// IsAllowedTagRune permits only ASCII digits (0-9) and Latin or Greek letters
// (plus combining diacritical marks for accented Latin/Greek).
func IsAllowedTagRune(r rune) bool {
	if r >= '0' && r <= '9' {
		return true
	}
	if unicode.IsLetter(r) && unicode.In(r, unicode.Latin, unicode.Greek) {
		return true
	}
	if r >= 0x0300 && r <= 0x036F {
		return true
	}
	return false
}

// ValidateTagChars verifies that a tag contains only Latin/Greek letters and ASCII digits.
func ValidateTagChars(s string) (rune, bool) {
	for _, r := range s {
		if !IsAllowedTagRune(r) {
			return r, false
		}
	}
	return 0, true
}

// IsAllowedProseRune permits:
//   - Standard whitespace: space, \n, \r, \t
//   - Printable ASCII (0x21..0x7E): English alphanumeric + Markdown/prose punctuation
//   - Latin and Greek letters (unicode.Latin, unicode.Greek)
//   - Combining diacritics (0x0300..0x036F) and classical Greek polytonic marks
//   - Standard typographic punctuation (en/em-dash, curly quotes, ellipsis, middle dot, guillemets)
//
// Emojis, variation selectors, zero-width/bidi characters, control characters,
// and non-Latin/non-Greek scripts return false.
func IsAllowedProseRune(r rune) bool {
	if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
		return true
	}
	if r >= 0x21 && r <= 0x7E {
		return true
	}
	if unicode.IsLetter(r) && unicode.In(r, unicode.Latin, unicode.Greek) {
		return true
	}
	if r >= 0x0300 && r <= 0x036F {
		return true
	}
	// Classical Greek punctuation & polytonic breathings/accents
	if r == 0x037E || r == 0x0384 || r == 0x0385 || r == 0x0387 ||
		(r >= 0x1FBD && r <= 0x1FC1) || (r >= 0x1FCD && r <= 0x1FCF) ||
		(r >= 0x1FDD && r <= 0x1FDF) || (r >= 0x1FED && r <= 0x1FEF) ||
		r == 0x1FFD || r == 0x1FFE {
		return true
	}
	// Standard typographic prose punctuation
	switch r {
	case 0x00A1, 0x00A7, 0x00AB, 0x00B0, 0x00B7, 0x00BB, 0x00BF,
		0x2013, 0x2014, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2026:
		return true
	}
	return false
}

// ValidateProseChars checks that every rune in a Markdown body is within the
// English/Latin/Greek/punctuation whitelist (no emojis or disallowed scripts).
func ValidateProseChars(s string) (rune, bool) {
	for _, r := range s {
		if !IsAllowedProseRune(r) {
			return r, false
		}
	}
	return 0, true
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
