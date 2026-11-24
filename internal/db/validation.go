package db

import (
	"fmt"
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
	// PassageJSON, when set, is a stored passage to keep (a backup restore);
	// it is decoded and validated again.
	PassageJSON string
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
	entry, err := domain.ParseJournalEntry(in.MediaURL, in.Tags, in.Body, in.Bookmarked, in.Published)
	if err != nil || in.PassageJSON == "" {
		return entry, err
	}
	p, err := domain.DecodePassage(in.PassageJSON)
	if err != nil {
		return domain.ValidatedJournalEntry{}, fmt.Errorf("%w: stored passage: %v", domain.ErrInvalidJournalInput, err)
	}
	return entry.WithPassage(p)
}

// ScriptureInput is the editor's scripture fields, raw, as posted: the
// media is a Bible passage instead of a link when Kind is "scripture".
// routes parses them with internal/verse and checks Text against the
// provider before anything is stored.
type ScriptureInput struct {
	Kind              string // media_kind: "link" or "scripture"
	Scripture         string // scripture: "bible"
	Book              string // verse_book: USFM id
	From              string // verse_from: chapter:verse
	To                string // verse_to: verse, or chapter:verse, or ""
	TranslationChoice string // verse_tr_choice: a translation id, or "auto"
	Translation       string // verse_tr: the translation the preview used
	Text              string // verse_text: the text the preview showed
}

// IsBible reports whether the media is a Bible passage.
func (s ScriptureInput) IsBible() bool {
	return s.Kind == "scripture" && s.Scripture == "bible"
}
