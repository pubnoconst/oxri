package domain

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxJournalMediaURLLength = 128
	MaxJournalTagCount       = 3
	MaxJournalTagLength      = 8
	// MaxJournalBodyLength caps a journal entry's body, in characters (Unicode code
	// points, so accented Latin and Greek count as one each). Books keep the
	// shared long-form DefaultMaxProseLength.
	MaxJournalBodyLength = 2000
)

var (
	ErrInvalidJournalInput = errors.New("invalid journal input")
)

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

// IsPrivateOrLocalHost reports whether a URL hostname points to localhost,
// loopback, link-local, or private internal IP space (anti-SSRF).
func IsPrivateOrLocalHost(hostname string) bool {
	lower := strings.ToLower(strings.TrimSpace(hostname))
	if lower == "" || lower == "localhost" || strings.HasSuffix(lower, ".localhost") || strings.HasSuffix(lower, ".local") {
		return true
	}
	if ip := net.ParseIP(lower); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}

// IsAllowedJournalTagRune permits only ASCII letters (A-Z, a-z) and ASCII digits (0-9).
func IsAllowedJournalTagRune(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// ValidateJournalTagChars verifies that a tag contains only ASCII letters and digits.
func ValidateJournalTagChars(s string) (rune, bool) {
	for _, r := range s {
		if !IsAllowedJournalTagRune(r) {
			return r, false
		}
	}
	return 0, true
}

// FormatTagTitleCase normalizes a tag string to Title Case (first rune uppercase, rest lowercase).
func FormatTagTitleCase(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	first := string(unicode.ToUpper(runes[0]))
	if len(runes) == 1 {
		return first
	}
	return first + strings.ToLower(string(runes[1:]))
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

// ValidatedJournalEntry is an opaque domain entity representing a journal entry
// that has passed all character-set, SSRF, tag-uniqueness, and length invariants.
// Persistence operations in internal/db require a ValidatedJournalEntry.
type ValidatedJournalEntry struct {
	mediaURL   string
	tags       []string
	body       string
	bookmarked bool
	published  bool
	passage    string // encoded Passage, or "" (WithPassage)
}

// ParseJournalEntry parses and validates raw journal fields into a ValidatedJournalEntry.
func ParseJournalEntry(rawMediaURL string, rawTags []string, rawBody string, bookmarked, published bool) (ValidatedJournalEntry, error) {
	mediaURL := strings.TrimSpace(rawMediaURL)
	if mediaURL == "" {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: media link is required", ErrInvalidJournalInput)
	}
	if utf8.RuneCountInString(mediaURL) > MaxJournalMediaURLLength {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: media link must be at most %d characters", ErrInvalidJournalInput, MaxJournalMediaURLLength)
	}
	if badRune, ok := ValidateMediaURLChars(mediaURL); !ok {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: media link contains disallowed character U+%04X (only printable ASCII URL characters permitted; no emojis)", ErrInvalidJournalInput, badRune)
	}
	u, err := url.ParseRequestURI(mediaURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: media link must be a valid http or https URL", ErrInvalidJournalInput)
	}
	if IsPrivateOrLocalHost(u.Hostname()) {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: media link host must not be localhost or a private/internal IP", ErrInvalidJournalInput)
	}

	if len(rawTags) > MaxJournalTagCount {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: at most %d tags are allowed", ErrInvalidJournalInput, MaxJournalTagCount)
	}
	cleanedTags := make([]string, 0, len(rawTags))
	seen := make(map[string]bool, len(rawTags))
	for _, tag := range rawTags {
		t := FormatTagTitleCase(tag)
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > MaxJournalTagLength {
			return ValidatedJournalEntry{}, fmt.Errorf("%w: tag %q exceeds %d characters", ErrInvalidJournalInput, t, MaxJournalTagLength)
		}
		if badRune, ok := ValidateJournalTagChars(t); !ok {
			return ValidatedJournalEntry{}, fmt.Errorf("%w: tag contains disallowed character U+%04X (only ASCII letters and digits permitted)", ErrInvalidJournalInput, badRune)
		}
		if seen[t] {
			return ValidatedJournalEntry{}, fmt.Errorf("%w: duplicate tag %q (tags must be unique)", ErrInvalidJournalInput, t)
		}
		seen[t] = true
		cleanedTags = append(cleanedTags, t)
	}

	if strings.TrimSpace(rawBody) == "" {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: journal content cannot be empty", ErrInvalidJournalInput)
	}
	if n := utf8.RuneCountInString(rawBody); n > MaxJournalBodyLength {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: journal content is %d characters; the maximum is %d", ErrInvalidJournalInput, n, MaxJournalBodyLength)
	}
	if badRune, ok := ValidateProseChars(rawBody); !ok {
		return ValidatedJournalEntry{}, fmt.Errorf("%w: journal content contains disallowed character U+%04X (only English, Latin, Greek, digits, and standard punctuation permitted; no emojis)", ErrInvalidJournalInput, badRune)
	}

	return ValidatedJournalEntry{
		mediaURL:   mediaURL,
		tags:       cleanedTags,
		body:       rawBody,
		bookmarked: bookmarked,
		published:  published,
	}, nil
}

// MediaURL returns the validated media URL string.
func (e ValidatedJournalEntry) MediaURL() string {
	return e.mediaURL
}

// Tags returns a copy of the validated Title-Cased tags (0..3).
func (e ValidatedJournalEntry) Tags() []string {
	if len(e.tags) == 0 {
		return nil
	}
	cp := make([]string, len(e.tags))
	copy(cp, e.tags)
	return cp
}

// TagAt returns the tag at index idx (0..2) or "" if absent.
func (e ValidatedJournalEntry) TagAt(idx int) string {
	if idx >= 0 && idx < len(e.tags) {
		return e.tags[idx]
	}
	return ""
}

// Body returns the validated Markdown prose body.
func (e ValidatedJournalEntry) Body() string {
	return e.body
}

// Bookmarked reports whether the entry is bookmarked.
func (e ValidatedJournalEntry) Bookmarked() bool {
	return e.bookmarked
}

// Published reports whether the entry is published.
func (e ValidatedJournalEntry) Published() bool {
	return e.published
}

// WithPassage returns the entry quoting p, the Bible passage its media link
// points to. p is validated (marks included) and stored encoded.
func (e ValidatedJournalEntry) WithPassage(p Passage) (ValidatedJournalEntry, error) {
	if err := ValidatePassage(p); err != nil {
		return ValidatedJournalEntry{}, err
	}
	enc, err := EncodePassage(p)
	if err != nil {
		return ValidatedJournalEntry{}, err
	}
	e.passage = enc
	return e, nil
}

// PassageJSON is the encoded passage, or "" when the entry quotes none.
func (e ValidatedJournalEntry) PassageJSON() string {
	return e.passage
}

// EstimatedBytes returns the exact UTF-8 byte footprint of the entry plus rowOverhead.
func (e ValidatedJournalEntry) EstimatedBytes(rowOverhead int64) int64 {
	total := rowOverhead + int64(len(e.mediaURL)) + int64(len(e.body)) + int64(len(e.passage))
	for _, t := range e.tags {
		total += int64(len(t))
	}
	return total
}
