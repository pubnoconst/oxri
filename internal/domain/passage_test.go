package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func testPassage() Passage {
	return Passage{Translation: "eng_kjv", Book: "GEN", Chapter: 12, From: 3, To: 4, Reference: "Genesis 12:3-4", Version: "KJV",
		Text: "And I will bless them that bless thee, and curse him that curseth thee"}
}

func TestPassageRoundTrip(t *testing.T) {
	p := testPassage()
	enc, err := EncodePassage(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodePassage(enc)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Errorf("round trip = %+v, %v", got, err)
	}
}

func TestValidatePassage(t *testing.T) {
	if err := ValidatePassage(testPassage()); err != nil {
		t.Fatalf("valid passage rejected: %v", err)
	}
	bad := map[string]func(*Passage){
		"translation":  func(p *Passage) { p.Translation = "../x" },
		"book":         func(p *Passage) { p.Book = "gen" },
		"chapter":      func(p *Passage) { p.Chapter = 0 },
		"backwards":    func(p *Passage) { p.From, p.To = 4, 3 },
		"too many":     func(p *Passage) { p.From, p.To = 1, MaxPassageVerses+1 },
		"no reference": func(p *Passage) { p.Reference = " " },
		"no version":   func(p *Passage) { p.Version = "" },
		"no text":      func(p *Passage) { p.Text = " " },
		"too long":     func(p *Passage) { p.Text = strings.Repeat("a", MaxPassageTextLength+1) },
		"markup":       func(p *Passage) { p.Text = "And <b>I</b>" },
		"markup ref":   func(p *Passage) { p.Reference = "<i>Genesis</i> 12:3" },
		"control":      func(p *Passage) { p.Text = "a\u202eb" },
		"newline":      func(p *Passage) { p.Text = "a\nb" },
	}
	for name, mutate := range bad {
		p := testPassage()
		mutate(&p)
		if err := ValidatePassage(p); !errors.Is(err, ErrInvalidJournalInput) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestWithPassageCountsBytes(t *testing.T) {
	e, err := ParseJournalEntry("https://bible.helloao.org/api/eng_kjv/GEN/12.json", nil, "body", false, true)
	if err != nil {
		t.Fatal(err)
	}
	before := e.EstimatedBytes(0)
	e, err = e.WithPassage(testPassage())
	if err != nil {
		t.Fatal(err)
	}
	if got := e.EstimatedBytes(0) - before; got != int64(len(e.PassageJSON())) || got == 0 {
		t.Errorf("passage adds %d bytes, want %d", got, len(e.PassageJSON()))
	}
}

func TestPassageSharesTheJournalLimit(t *testing.T) {
	p := testPassage()
	quoted := len([]rune(p.Text))
	entry := func(bodyLen int) ValidatedJournalEntry {
		e, err := ParseJournalEntry("https://bible.helloao.org/api/eng_kjv/GEN/12.json", nil, strings.Repeat("λ", bodyLen), false, true)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	// Exactly at the limit together (counted in characters: λ is two bytes).
	if _, err := entry(MaxJournalBodyLength - quoted).WithPassage(p); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	// One over.
	if _, err := entry(MaxJournalBodyLength - quoted + 1).WithPassage(p); !errors.Is(err, ErrInvalidJournalInput) {
		t.Errorf("one over the limit: err = %v", err)
	}
}
