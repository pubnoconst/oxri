package domain

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrInvalidTag = errors.New("domain: invalid tag format")
	tagRegex      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

const (
	MinTagLength = 1
	MaxTagLength = 40
)

// Tag is an opaque type representing a validated, canonical lowercase kebab-case tag.
type Tag struct {
	val string
}

// ParseTag parses and validates a raw tag string.
// Tags must be 1-40 characters, lowercase alphanumeric with single hyphens,
// no leading or trailing hyphens, and no special characters.
func ParseTag(raw string) (Tag, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if len(trimmed) < MinTagLength || len(trimmed) > MaxTagLength {
		return Tag{}, fmt.Errorf("%w: length must be between %d and %d characters", ErrInvalidTag, MinTagLength, MaxTagLength)
	}

	if !tagRegex.MatchString(trimmed) {
		return Tag{}, fmt.Errorf("%w: '%s' does not match lowercase kebab-case", ErrInvalidTag, trimmed)
	}

	return Tag{val: trimmed}, nil
}

// String returns the tag's canonical string representation.
func (t Tag) String() string {
	return t.val
}

// IsZero reports whether the Tag is uninitialized.
func (t Tag) IsZero() bool {
	return t.val == ""
}

// TagSet is an immutable collection of sorted, deduplicated Tags.
type TagSet struct {
	tags []Tag
}

// NewTagSet constructs a TagSet from already-validated Tags, sorting and deduplicating them.
func NewTagSet(tags ...Tag) TagSet {
	if len(tags) == 0 {
		return TagSet{tags: nil}
	}

	seen := make(map[string]struct{}, len(tags))
	var unique []Tag
	for _, t := range tags {
		if t.IsZero() {
			continue
		}
		if _, exists := seen[t.val]; !exists {
			seen[t.val] = struct{}{}
			unique = append(unique, t)
		}
	}

	sort.Slice(unique, func(i, j int) bool {
		return unique[i].val < unique[j].val
	})

	return TagSet{tags: unique}
}

// ParseTagSet parses a list of raw tag strings, rejecting the entire set if any tag is invalid.
func ParseTagSet(raws []string) (TagSet, error) {
	tags := make([]Tag, 0, len(raws))
	for _, raw := range raws {
		tag, err := ParseTag(raw)
		if err != nil {
			return TagSet{}, err
		}
		tags = append(tags, tag)
	}
	return NewTagSet(tags...), nil
}

// Slice returns a copy of the tags slice.
func (ts TagSet) Slice() []Tag {
	if len(ts.tags) == 0 {
		return nil
	}
	res := make([]Tag, len(ts.tags))
	copy(res, ts.tags)
	return res
}

// Strings returns a slice of string representations.
func (ts TagSet) Strings() []string {
	if len(ts.tags) == 0 {
		return nil
	}
	res := make([]string, len(ts.tags))
	for i, t := range ts.tags {
		res[i] = t.val
	}
	return res
}

// Contains checks if the TagSet contains the given Tag.
func (ts TagSet) Contains(t Tag) bool {
	if t.IsZero() || len(ts.tags) == 0 {
		return false
	}
	idx := sort.Search(len(ts.tags), func(i int) bool {
		return ts.tags[i].val >= t.val
	})
	return idx < len(ts.tags) && ts.tags[idx].val == t.val
}

// Len returns the count of tags in the set.
func (ts TagSet) Len() int {
	return len(ts.tags)
}
