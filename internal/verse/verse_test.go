package verse

import (
	"errors"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestParseChoice(t *testing.T) {
	cases := []struct {
		book, from, to, tr string
		ok                 bool
		ref, url           string
	}{
		{"GEN", "12:3", "4", "eng_ylt", true, "Genesis 12:3-4", "https://bible.helloao.org/api/eng_ylt/GEN/12.json"},
		{"GEN", "12:3", "12:4", "eng_kjv", true, "Genesis 12:3-4", "https://bible.helloao.org/api/eng_kjv/GEN/12.json"},
		{"PSA", " 23:1 ", "", "eng_kjv", true, "Psalms 23:1", "https://bible.helloao.org/api/eng_kjv/PSA/23.json"},
		{"1JN", "4:8", "8", "eng_lsv", true, "1 John 4:8", "https://bible.helloao.org/api/eng_lsv/1JN/4.json"},
		{"GEN", "12:3", "13:1", "eng_kjv", false, "", ""}, // across chapters
		{"GEN", "12:4", "3", "eng_kjv", false, "", ""},    // backwards
		{"GEN", "51:1", "", "eng_kjv", false, "", ""},     // Genesis has 50 chapters
		{"GEN", "12", "", "eng_kjv", false, "", ""},       // no verse
		{"GEN", "12:1", "45", "eng_kjv", false, "", ""},   // too many verses
		{"XYZ", "1:1", "", "eng_kjv", false, "", ""},
		{"GEN", "1:1", "", "auto", false, "", ""}, // the browser resolves "auto"
		{"GEN", "1:1", "", "../x", false, "", ""},
	}
	for _, c := range cases {
		got, err := ParseChoice(c.book, c.from, c.to, c.tr)
		if (err == nil) != c.ok {
			t.Errorf("ParseChoice(%q, %q, %q, %q) err = %v, want ok %v", c.book, c.from, c.to, c.tr, err, c.ok)
			continue
		}
		if !c.ok {
			if !errors.Is(err, domain.ErrInvalidJournalInput) {
				t.Errorf("error %v is not ErrInvalidJournalInput", err)
			}
			continue
		}
		if got.Reference() != c.ref || got.ChapterURL() != c.url {
			t.Errorf("got %q %q, want %q %q", got.Reference(), got.ChapterURL(), c.ref, c.url)
		}
	}
}

// psalm23 is the shape of a provider chapter file: poetry lines, a
// superscription, footnote markers and a line break (BSB, public domain).
const psalm23 = `{
 "book": {"id": "PSA", "commonName": "Psalms"},
 "chapter": {"number": 23, "content": [
  {"type": "heading", "content": ["The LORD Is My Shepherd"]},
  {"type": "verse", "number": 1, "content": [{"text": "A Psalm of David.", "descriptive": true}, {"text": "The LORD is my shepherd;", "poem": 1}, {"noteId": 50}, {"text": "I shall not want.", "poem": 2}]},
  {"type": "line_break"},
  {"type": "verse", "number": 2, "content": [{"text": "He makes me lie down in green pastures;", "poem": 1}, {"lineBreak": true}, {"text": "He leads me beside quiet waters.", "poem": 2}]},
  {"type": "verse", "number": 3, "content": ["He restores my soul", {"noteId": 51}, ", and guides me\u00a0 in paths ", {"text": "of righteousness", "wordsOfJesus": true}, "."]}
 ]}
}`

func TestAssemble(t *testing.T) {
	cases := []struct {
		from, to int
		want     string
	}{
		{1, 1, "A Psalm of David. The LORD is my shepherd; I shall not want."},
		{2, 3, "He makes me lie down in green pastures; He leads me beside quiet waters. He restores my soul, and guides me in paths of righteousness."},
	}
	for _, c := range cases {
		got, err := Assemble([]byte(psalm23), c.from, c.to)
		if err != nil || got != c.want {
			t.Errorf("Assemble(%d, %d) = %q, %v\nwant %q", c.from, c.to, got, err, c.want)
		}
	}
	for _, r := range [][2]int{{4, 4}, {3, 9}} {
		if _, err := Assemble([]byte(psalm23), r[0], r[1]); !errors.Is(err, ErrNoVerses) {
			t.Errorf("Assemble(%d, %d) err = %v, want ErrNoVerses", r[0], r[1], err)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"  a \n\t b  ": "a b",
		"word , next":  "word, next",
		"say ” .":      "say”.",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranslations(t *testing.T) {
	for _, id := range Preferred {
		if _, ok := LookupTranslation(id); !ok {
			t.Errorf("preferred %s is not offered", id)
		}
	}
	if Translations[0].ID != Preferred[0] {
		t.Errorf("the editor's first translation should be the preferred one")
	}
	if len(Books) != 66 || Books[0].ID != "GEN" || Books[65].ID != "REV" {
		t.Errorf("books: %d", len(Books))
	}
}

func TestNilSource(t *testing.T) {
	var s *Source
	c, _ := ParseChoice("GEN", "1:1", "", "eng_kjv")
	if _, err := s.Verify(t.Context(), c, "x"); err == nil {
		t.Error("nil source verified a passage")
	}
}
