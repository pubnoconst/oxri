package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidSafePath = errors.New("domain: invalid path identifier")
	safePathRegex      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)
)

// SafePath is an opaque domain type representing a single-segment path slug
// that is guaranteed safe from directory traversal, root escape, or injection.
type SafePath struct {
	slug string
}

// ParseSafePath parses a raw identifier into a SafePath.
// Must start with an alphanumeric character, contain only lower-case alphanumerics,
// underscores, or hyphens, and be between 1 and 80 characters long.
// Traversal sequences ('..', '/', '\', ':', null bytes) are structurally rejected.
func ParseSafePath(raw string) (SafePath, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return SafePath{}, fmt.Errorf("%w: path identifier cannot be empty", ErrInvalidSafePath)
	}

	if !safePathRegex.MatchString(trimmed) {
		return SafePath{}, fmt.Errorf("%w: '%s' must match pattern ^[a-z0-9][a-z0-9_-]{0,79}$", ErrInvalidSafePath, trimmed)
	}

	return SafePath{slug: trimmed}, nil
}

// String returns the safe path slug.
func (s SafePath) String() string {
	return s.slug
}

// IsZero reports whether SafePath is uninitialized.
func (s SafePath) IsZero() bool {
	return s.slug == ""
}
