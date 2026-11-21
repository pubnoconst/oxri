package domain

import (
	"errors"
	"time"
)

var (
	ErrZeroPath = errors.New("domain: entry path cannot be zero value")
	ErrZeroBody = errors.New("domain: entry body cannot be zero value")
)

// ValidatedEntry represents a complete, immutable journal or document entry.
// Because all fields are unexported and all member types are opaque,
// a ValidatedEntry can never exist in an unvalidated or corrupt state.
type ValidatedEntry struct {
	path      SafePath
	tags      TagSet
	body      Prose
	media     []MediaURL
	createdAt time.Time
}

// NewValidatedEntry constructs a ValidatedEntry from validated domain types.
func NewValidatedEntry(path SafePath, tags TagSet, body Prose, media []MediaURL, createdAt time.Time) (ValidatedEntry, error) {
	if path.IsZero() {
		return ValidatedEntry{}, ErrZeroPath
	}
	if body.IsZero() {
		return ValidatedEntry{}, ErrZeroBody
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	var safeMedia []MediaURL
	for _, m := range media {
		if !m.IsZero() {
			safeMedia = append(safeMedia, m)
		}
	}

	return ValidatedEntry{
		path:      path,
		tags:      tags,
		body:      body,
		media:     safeMedia,
		createdAt: createdAt.UTC(),
	}, nil
}

// Path returns the entry's safe path identifier.
func (e ValidatedEntry) Path() SafePath {
	return e.path
}

// Tags returns the entry's tag set.
func (e ValidatedEntry) Tags() TagSet {
	return e.tags
}

// Body returns the entry's validated prose text.
func (e ValidatedEntry) Body() Prose {
	return e.body
}

// Media returns a copy of the validated media URLs.
func (e ValidatedEntry) Media() []MediaURL {
	if len(e.media) == 0 {
		return nil
	}
	res := make([]MediaURL, len(e.media))
	copy(res, e.media)
	return res
}

// CreatedAt returns the timestamp in UTC.
func (e ValidatedEntry) CreatedAt() time.Time {
	return e.createdAt
}
