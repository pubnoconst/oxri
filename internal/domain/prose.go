package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidProse   = errors.New("domain: invalid prose text")
	ErrProseTooLong   = errors.New("domain: prose exceeds maximum allowed length")
	ErrProseNullBytes = errors.New("domain: prose contains forbidden null bytes")
)

const (
	DefaultMaxProseLength = 100_000 // 100KB characters max
)

// Prose is an opaque type representing safe, sanitized text content.
type Prose struct {
	content string
}

// ParseProse validates text against length constraints and forbidden characters (null bytes).
func ParseProse(raw string, maxLen int) (Prose, error) {
	if maxLen <= 0 {
		maxLen = DefaultMaxProseLength
	}

	if strings.ContainsRune(raw, '\x00') {
		return Prose{}, ErrProseNullBytes
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
