package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrProseTooLong      = errors.New("domain: prose exceeds maximum allowed length")
	ErrProseNullBytes    = errors.New("domain: prose contains forbidden null bytes")
	ErrProseDisallowedCh = errors.New("domain: prose contains disallowed character (only Latin, Greek, numbers, and standard punctuation permitted; no emojis)")
)

const (
	DefaultMaxProseLength = 100_000 // 100KB characters max
)

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
	if r == 0x037E || r == 0x0384 || r == 0x0385 || r == 0x0387 ||
		(r >= 0x1FBD && r <= 0x1FC1) || (r >= 0x1FCD && r <= 0x1FCF) ||
		(r >= 0x1FDD && r <= 0x1FDF) || (r >= 0x1FED && r <= 0x1FEF) ||
		r == 0x1FFD || r == 0x1FFE {
		return true
	}
	switch r {
	case 0x00A1, 0x00A7, 0x00AB, 0x00B0, 0x00B7, 0x00BB, 0x00BF,
		0x2013, 0x2014, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2026:
		return true
	}
	return false
}

// Prose is an opaque type representing safe, sanitized text content.
type Prose struct {
	content string
}

// ParseProse validates text against length constraints and strict Latin/Greek/ASCII character whitelisting.
func ParseProse(raw string, maxLen int) (Prose, error) {
	if maxLen <= 0 {
		maxLen = DefaultMaxProseLength
	}

	if strings.ContainsRune(raw, '\x00') {
		return Prose{}, ErrProseNullBytes
	}

	for _, r := range raw {
		if !IsAllowedProseRune(r) {
			return Prose{}, fmt.Errorf("%w: U+%04X", ErrProseDisallowedCh, r)
		}
	}

	charCount := utf8.RuneCountInString(raw)
	if charCount > maxLen {
		return Prose{}, fmt.Errorf("%w: length is %d, maximum allowed is %d", ErrProseTooLong, charCount, maxLen)
	}

	// Clean trailing spaces and normalize carriage returns
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.TrimRight(normalized, " \t\n")

	return Prose{content: normalized}, nil
}

// String returns the underlying text.
func (p Prose) String() string {
	return p.content
}

// Len returns the rune length of the prose.
func (p Prose) Len() int {
	return utf8.RuneCountInString(p.content)
}

// IsZero reports whether the Prose is empty or uninitialized.
func (p Prose) IsZero() bool {
	return p.content == ""
}
