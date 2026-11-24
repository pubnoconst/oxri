// Package verse quotes Bible passages from the Free Use Bible API
// (bible.helloao.org, HelloAO): free, no key, no rate limit, openly licensed
// translations including Young's Literal Translation, one static JSON file
// per chapter. It is an API: nothing is scraped.
//
// The author picks a passage in the editor (book, from, to, translation).
// The browser fetches the chapter, builds the passage's text and posts it
// (scripture-picker.js); on save the server fetches the chapter, builds the
// text again and stores the passage only if the two agree (Source.Verify).
// Both sides build the text by one rule, Assemble.
package verse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/domain"
)

// APIBase is the provider. Its chapter files are served with CORS open, so
// the editor reads them directly.
const APIBase = "https://bible.helloao.org/api"

// A Translation the editor offers.
type Translation struct {
	ID    string // the provider's id
	Label string // on the clipping, "YLT"
	Name  string // in the editor
}

// Translations are the English translations the editor offers, word-for-word
// ones first. All are public domain or openly licensed.
var Translations = []Translation{
	{"eng_ylt", "YLT", "Young's Literal Translation"},
	{"eng_kjv", "KJV", "King James Version"},
	{"eng_lsv", "LSV", "Literal Standard Version"},
	{"eng_asv", "ASV", "American Standard Version (1901)"},
	{"eng_dby", "Darby", "Darby Translation"},
	{"ENGWEBP", "WEB", "World English Bible"},
	{"BSB", "BSB", "Berean Standard Bible"},
}

// Preferred is the order the editor tries when the author leaves the
// translation to it: Young's Literal, then the King James.
var Preferred = []string{"eng_ylt", "eng_kjv"}

// LookupTranslation finds a translation the editor offers.
func LookupTranslation(id string) (Translation, bool) {
	for _, t := range Translations {
		if t.ID == id {
			return t, true
		}
	}
	return Translation{}, false
}

// Choice is the passage an author picked: a range of verses in one chapter,
// in one translation. Only ParseChoice makes one.
type Choice struct {
	Translation Translation
	Book        Book
	Chapter     int
	From, To    int
}

// ChapterURL is the provider's file for the choice's chapter. It is also the
// entry's media link.
func (c Choice) ChapterURL() string {
	return APIBase + "/" + c.Translation.ID + "/" + c.Book.ID + "/" + strconv.Itoa(c.Chapter) + ".json"
}

// Reference is how the passage is cited: "Genesis 12:3" or "Genesis 12:3-4".
func (c Choice) Reference() string {
	ref := c.Book.Name + " " + strconv.Itoa(c.Chapter) + ":" + strconv.Itoa(c.From)
	if c.To > c.From {
		ref += "-" + strconv.Itoa(c.To)
	}
	return ref
}

var (
	fromPattern = regexp.MustCompile(`^([0-9]{1,3}):([0-9]{1,3})$`)
	toPattern   = regexp.MustCompile(`^(?:([0-9]{1,3}):)?([0-9]{1,3})$`)
)

// ParseChoice reads the editor's fields: a book id, From as chapter:verse
// ("12:3"), To as a verse or chapter:verse in the same chapter ("4", "12:4",
// or empty for one verse), and a translation id.
func ParseChoice(book, from, to, translation string) (Choice, error) {
	invalid := func(msg string) (Choice, error) {
		return Choice{}, fmt.Errorf("%w: %s", domain.ErrInvalidJournalInput, msg)
	}
	tr, ok := LookupTranslation(translation)
	if !ok {
		return invalid("choose a translation")
	}
	b, ok := LookupBook(book)
	if !ok {
		return invalid("choose a book")
	}
	m := fromPattern.FindStringSubmatch(strings.TrimSpace(from))
	if m == nil {
		return invalid("From is chapter:verse, like 12:3")
	}
	ch, _ := strconv.Atoi(m[1])
	first, _ := strconv.Atoi(m[2])
	if ch < 1 || ch > b.Chapters {
		return invalid(fmt.Sprintf("%s has %d chapters", b.Name, b.Chapters))
	}
	last := first
	if to = strings.TrimSpace(to); to != "" {
		n := toPattern.FindStringSubmatch(to)
		if n == nil {
			return invalid("To is a verse, like 4, or chapter:verse in the same chapter")
		}
		if n[1] != "" {
			if c, _ := strconv.Atoi(n[1]); c != ch {
				return invalid("From and To must be in the same chapter")
			}
		}
		last, _ = strconv.Atoi(n[2])
	}
	if err := domain.ValidatePassageRef(tr.ID, b.ID, ch, first, last); err != nil {
		return Choice{}, err
	}
	return Choice{Translation: tr, Book: b, Chapter: ch, From: first, To: last}, nil
}

// chapterFile is the part of a provider chapter file Assemble reads.
type chapterFile struct {
	Book struct {
		ID string `json:"id"`
	} `json:"book"`
	Chapter struct {
		Number  int            `json:"number"`
		Content []chapterBlock `json:"content"`
	} `json:"chapter"`
}

type chapterBlock struct {
	Type    string            `json:"type"`
	Number  int               `json:"number"`
	Content []json.RawMessage `json:"content"`
}

var (
	// spaces is every kind of white space, the same set as
	// scripture-picker.js (Go's \s is ASCII only, JavaScript's is not).
	spaces = regexp.MustCompile(`[\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]+`)
	// spaceBeforeClose is a space the joining put before closing punctuation.
	spaceBeforeClose = regexp.MustCompile(` ([,.;:!?)\]’”»])`)
)

// Normalize collapses white space to single spaces, trims the ends, and
// removes a space before closing punctuation.
func Normalize(s string) string {
	s = strings.TrimSpace(spaces.ReplaceAllString(s, " "))
	return spaceBeforeClose.ReplaceAllString(s, "$1")
}

// ErrNoVerses: the chapter does not have the verses (in this translation).
var ErrNoVerses = errors.New("verse: the chapter has no such verses")

// Assemble builds the text of verses from..to of a chapter file: every piece
// of text in each verse, in order (plain strings, and the text of poetry
// lines, words of Jesus and superscriptions), joined by spaces, then
// Normalize; footnote markers, line breaks and headings are left out. Both
// ends of the range must exist.
func Assemble(file []byte, from, to int) (string, error) {
	var ch chapterFile
	if err := json.Unmarshal(file, &ch); err != nil {
		return "", fmt.Errorf("verse: chapter file: %w", err)
	}
	var verses []string
	haveFrom, haveTo := false, false
	for _, b := range ch.Chapter.Content {
		if b.Type != "verse" || b.Number < from || b.Number > to {
			continue
		}
		haveFrom = haveFrom || b.Number == from
		haveTo = haveTo || b.Number == to
		for _, raw := range b.Content {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				verses = append(verses, s)
				continue
			}
			var part struct {
				Text *string `json:"text"`
			}
			if json.Unmarshal(raw, &part) == nil && part.Text != nil {
				verses = append(verses, *part.Text)
			}
		}
	}
	text := Normalize(strings.Join(verses, " "))
	if !haveFrom || !haveTo || text == "" {
		return "", ErrNoVerses
	}
	return text, nil
}

// Errors from fetching.
var (
	// ErrUnavailable: the provider has no such chapter in the translation.
	ErrUnavailable = errors.New("verse: no such chapter")
	// ErrMismatch: a posted text is not what the provider serves.
	ErrMismatch = errors.New("verse: text does not match the provider's")
)

// maxChapter caps how much of a chapter file is read (Psalm 119, the longest
// chapter, is well under this).
const maxChapter = 2 << 20

// fetchChapter reads one chapter file from the provider.
func fetchChapter(ctx context.Context, client *http.Client, c Choice) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ChapterURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, ErrUnavailable
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("verse: provider returned %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxChapter+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxChapter {
		return nil, errors.New("verse: chapter file too large")
	}
	return body, nil
}

// Source fetches chapters and remembers recent ones. A nil Source (tests, or
// no network) has no passages.
type Source struct {
	client *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	file []byte
	at   time.Time
}

const (
	cacheSize = 32
	cacheTTL  = 6 * time.Hour
	timeout   = 8 * time.Second
)

// NewSource fetches with client.
func NewSource(client *http.Client) *Source {
	return &Source{client: client, cache: map[string]cached{}}
}

// Verify builds the passage c names from the provider and checks that
// posted, the text the author's browser built for it, is the same text.
func (s *Source) Verify(ctx context.Context, c Choice, posted string) (domain.Passage, error) {
	file, err := s.chapter(ctx, c)
	if err != nil {
		return domain.Passage{}, err
	}
	text, err := Assemble(file, c.From, c.To)
	if err != nil {
		return domain.Passage{}, err
	}
	if Normalize(posted) != text {
		return domain.Passage{}, ErrMismatch
	}
	p := domain.Passage{
		Translation: c.Translation.ID, Book: c.Book.ID, Chapter: c.Chapter, From: c.From, To: c.To,
		Reference: c.Reference(), Version: c.Translation.Label, Text: text,
	}
	if err := domain.ValidatePassage(p); err != nil {
		return domain.Passage{}, err
	}
	return p, nil
}

func (s *Source) chapter(ctx context.Context, c Choice) ([]byte, error) {
	if s == nil {
		return nil, errors.New("verse: no source")
	}
	key := c.ChapterURL()
	s.mu.Lock()
	if e, ok := s.cache[key]; ok && time.Since(e.at) < cacheTTL {
		s.mu.Unlock()
		return e.file, nil
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	file, err := fetchChapter(ctx, s.client, c)
	if err != nil {
		return nil, err
	}
	var check chapterFile
	if json.Unmarshal(file, &check) != nil || check.Book.ID != c.Book.ID || check.Chapter.Number != c.Chapter {
		return nil, ErrUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cache) >= cacheSize {
		oldest := ""
		for k, e := range s.cache {
			if oldest == "" || e.at.Before(s.cache[oldest].at) {
				oldest = k
			}
		}
		delete(s.cache, oldest)
	}
	s.cache[key] = cached{file: file, at: time.Now()}
	return file, nil
}
