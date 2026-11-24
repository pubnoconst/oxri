package routes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/domain"
	"github.com/oxri/oxri-go/internal/verse"
)

// scriptureChoice reads the editor's scripture fields. When the media is a
// Bible passage it returns the choice, and the entry's media link becomes
// the passage's chapter on the provider (whatever the link field held).
func scriptureChoice(in *db.JournalInput, s db.ScriptureInput) (verse.Choice, bool, error) {
	if s.Kind == "scripture" && !s.IsBible() {
		return verse.Choice{}, false, fmt.Errorf("%w: choose a scripture", domain.ErrInvalidJournalInput)
	}
	if !s.IsBible() {
		return verse.Choice{}, false, nil
	}
	c, err := verse.ParseChoice(s.Book, s.From, s.To, s.Translation)
	if err != nil {
		return verse.Choice{}, true, err
	}
	in.MediaURL = c.ChapterURL()
	return c, true, nil
}

// quotePassage gives the entry the passage c names, after checking that
// posted, the text the editor's preview showed, is what the provider serves.
// An edit that keeps the same verses and text keeps the stored passage
// without asking the provider again.
func quotePassage(ctx context.Context, src *verse.Source, entry domain.ValidatedJournalEntry, c verse.Choice, posted string, prev *db.Journal) (domain.ValidatedJournalEntry, error) {
	if verse.Normalize(posted) == "" {
		return domain.ValidatedJournalEntry{}, fmt.Errorf("%w: wait for the verses to load in the preview before saving", domain.ErrInvalidJournalInput)
	}
	if prev != nil {
		if p, ok := prev.Passage(); ok && p.Translation == c.Translation.ID && p.Book == c.Book.ID &&
			p.Chapter == c.Chapter && p.From == c.From && p.To == c.To && p.Text == verse.Normalize(posted) {
			return entry.WithPassage(p)
		}
	}
	p, err := src.Verify(ctx, c, posted)
	if err != nil {
		log.Printf("[VERSE] verify %s %d-%d failed: %v", c.ChapterURL(), c.From, c.To, err)
		return domain.ValidatedJournalEntry{}, passageError(c, err)
	}
	return entry.WithPassage(p)
}

// passageError is what the author is told when a passage can't be saved. It
// wraps ErrInvalidJournalInput, so the form is shown again with the message.
func passageError(c verse.Choice, err error) error {
	switch {
	case errors.Is(err, verse.ErrMismatch):
		return fmt.Errorf("%w: the verse text doesn't match the provider's; let the preview load again and save", domain.ErrInvalidJournalInput)
	case errors.Is(err, verse.ErrNoVerses), errors.Is(err, verse.ErrUnavailable):
		return fmt.Errorf("%w: %s is not in the %s", domain.ErrInvalidJournalInput, c.Reference(), c.Translation.Name)
	case errors.Is(err, domain.ErrInvalidJournalInput):
		return err // too long, and the like: already says why
	default:
		return fmt.Errorf("%w: couldn't reach the Bible provider to check the verses; try again", domain.ErrInvalidJournalInput)
	}
}

// scriptureFromPassage fills the editor's scripture fields from a stored
// passage, for editing it.
func scriptureFromPassage(p domain.Passage) db.ScriptureInput {
	s := db.ScriptureInput{
		Kind: "scripture", Scripture: "bible", Book: p.Book,
		From:              strconv.Itoa(p.Chapter) + ":" + strconv.Itoa(p.From),
		TranslationChoice: p.Translation, Translation: p.Translation, Text: p.Text,
	}
	if p.To > p.From {
		s.To = strconv.Itoa(p.To)
	}
	return s
}

// sanitizeScriptureForEcho caps the scripture fields for showing the form
// again (or holding a save on the Make Room page): ids and numbers keep only
// what they can hold, and the text only plain characters. Everything is
// checked again when it is posted.
func sanitizeScriptureForEcho(s db.ScriptureInput) db.ScriptureInput {
	keep := func(v string, n int, ok func(rune) bool) string {
		out := make([]rune, 0, len(v))
		for _, r := range v {
			if len(out) >= n {
				break
			}
			if ok(r) {
				out = append(out, r)
			}
		}
		return string(out)
	}
	word := func(r rune) bool {
		return r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
	}
	ref := func(r rune) bool { return r == ':' || (r >= '0' && r <= '9') }
	return db.ScriptureInput{
		Kind:              keep(s.Kind, 16, word),
		Scripture:         keep(s.Scripture, 16, word),
		Book:              keep(s.Book, 4, word),
		From:              keep(s.From, 8, ref),
		To:                keep(s.To, 8, ref),
		TranslationChoice: keep(s.TranslationChoice, 24, word),
		Translation:       keep(s.Translation, 24, word),
		Text: keep(s.Text, domain.MaxPassageTextLength, func(r rune) bool {
			return r >= 0x20 && r != 0x7f && r != '<' && r != '>'
		}),
	}
}
