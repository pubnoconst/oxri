package db

import (
	"strings"

	"github.com/oxri/oxri-go/internal/domain"
)

// JournalInput carries raw user input before parsing into a canonical domain.ValidatedJournalEntry.
type JournalInput struct {
	MediaURL   string
	Tags       []string
	Body       string
	Bookmarked bool
	Published  bool
}

// FormatTagTitleCase delegates tag normalization to domain.FormatTagTitleCase.
func FormatTagTitleCase(raw string) string {
	return domain.FormatTagTitleCase(raw)
}

// ParseTags splits a comma-separated tag string into trimmed, Title-Cased non-empty tags.
func ParseTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		formatted := FormatTagTitleCase(p)
		if formatted != "" {
			tags = append(tags, formatted)
		}
	}
	return tags
}

// ParseValidatedEntry converts a JournalInput into the canonical domain.ValidatedJournalEntry
// enforced by internal/domain.
func ParseValidatedEntry(in JournalInput) (domain.ValidatedJournalEntry, error) {
	return domain.ParseJournalEntry(in.MediaURL, in.Tags, in.Body, in.Bookmarked, in.Published)
}

// ValidateInput validates a JournalInput by parsing it through domain.ParseJournalEntry
// and returning the normalized JournalInput.
func ValidateInput(in JournalInput) (JournalInput, error) {
	entry, err := ParseValidatedEntry(in)
	if err != nil {
		return in, err
	}
	return JournalInput{
		MediaURL:   entry.MediaURL(),
		Tags:       entry.Tags(),
		Body:       entry.Body(),
		Bookmarked: entry.Bookmarked(),
		Published:  entry.Published(),
	}, nil
}
