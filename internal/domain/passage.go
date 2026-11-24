package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// A Passage is the Bible passage an entry quotes in place of a media link.
// The author picks the verses in the editor; the browser fetches their text
// from the provider (internal/verse) and posts it, and the server fetches it
// again on save and stores the passage only if the two agree.
//
// It keeps which verses they are (so the editor can show the choice again)
// and their text, which is plain: one line, no markup.
type Passage struct {
	Translation string `json:"tr"`   // the provider's id, "eng_ylt"
	Book        string `json:"book"` // USFM id, "GEN"
	Chapter     int    `json:"ch"`
	From        int    `json:"from"`
	To          int    `json:"to"`
	Reference   string `json:"ref"` // "Genesis 12:3-4"
	Version     string `json:"ver"` // "YLT"
	Text        string `json:"text"`
}

// Limits on a passage.
const (
	MaxPassageTextLength      = 2000 // characters, the same as an entry's own words
	MaxPassageVerses          = 40
	MaxPassageReferenceLength = 80
	MaxPassageVersionLength   = 16
	// MaxPassageJSONLength caps the stored JSON (the column's CHECK).
	MaxPassageJSONLength = 12288
)

var (
	translationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_]{2,24}$`)
	bookIDPattern        = regexp.MustCompile(`^[1-4]?[A-Z]{2,3}$`)
)

// ValidatePassageRef checks the shape of which verses a passage is. Whether
// the book and translation exist is internal/verse's to say.
func ValidatePassageRef(translation, book string, chapter, from, to int) error {
	switch {
	case !translationIDPattern.MatchString(translation):
		return fmt.Errorf("%w: unknown translation", ErrInvalidJournalInput)
	case !bookIDPattern.MatchString(book):
		return fmt.Errorf("%w: unknown book", ErrInvalidJournalInput)
	case chapter < 1 || chapter > 150:
		return fmt.Errorf("%w: chapter must be 1–150", ErrInvalidJournalInput)
	case from < 1 || to < from || to > 176:
		return fmt.Errorf("%w: verses must run forwards, from verse 1 to at most 176", ErrInvalidJournalInput)
	case to-from+1 > MaxPassageVerses:
		return fmt.Errorf("%w: at most %d verses", ErrInvalidJournalInput, MaxPassageVerses)
	}
	return nil
}

// ValidatePassage checks a whole passage. Every text field must be plain:
// no control characters (line breaks included) and no angle brackets, so no
// markup can be stored.
func ValidatePassage(p Passage) error {
	if err := ValidatePassageRef(p.Translation, p.Book, p.Chapter, p.From, p.To); err != nil {
		return err
	}
	if strings.TrimSpace(p.Reference) == "" || utf8.RuneCountInString(p.Reference) > MaxPassageReferenceLength {
		return fmt.Errorf("%w: passage reference must be 1–%d characters", ErrInvalidJournalInput, MaxPassageReferenceLength)
	}
	if strings.TrimSpace(p.Version) == "" || utf8.RuneCountInString(p.Version) > MaxPassageVersionLength {
		return fmt.Errorf("%w: passage version must be 1–%d characters", ErrInvalidJournalInput, MaxPassageVersionLength)
	}
	if strings.TrimSpace(p.Text) == "" {
		return fmt.Errorf("%w: passage text is empty", ErrInvalidJournalInput)
	}
	if n := utf8.RuneCountInString(p.Text); n > MaxPassageTextLength {
		return fmt.Errorf("%w: the passage is %d characters; choose fewer verses (at most %d characters)", ErrInvalidJournalInput, n, MaxPassageTextLength)
	}
	for _, s := range []string{p.Reference, p.Version, p.Text} {
		if !utf8.ValidString(s) || strings.ContainsFunc(s, isNotPlainText) {
			return fmt.Errorf("%w: passage must be plain text (no control characters or markup)", ErrInvalidJournalInput)
		}
	}
	return nil
}

// isNotPlainText is a control or direction-override character, or an angle
// bracket (the start or end of markup).
func isNotPlainText(r rune) bool {
	return r < 0x20 || r == 0x7f || r == '<' || r == '>' ||
		(r >= 0x200b && r <= 0x200f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

// EncodePassage is the passage as stored. Its length is what the passage
// costs against the storage quota.
func EncodePassage(p Passage) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	if len(b) > MaxPassageJSONLength {
		return "", fmt.Errorf("%w: passage is too large to store", ErrInvalidJournalInput)
	}
	return string(b), nil
}

// DecodePassage reads a stored passage and re-checks it.
func DecodePassage(s string) (Passage, error) {
	var p Passage
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return Passage{}, err
	}
	if err := ValidatePassage(p); err != nil {
		return Passage{}, err
	}
	return p, nil
}
