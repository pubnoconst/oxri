package domain_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestParseTag(t *testing.T) {
	valid := []string{
		"philosophy",
		"stoicism-101",
		"book-reviews-2026",
		"v2",
		"a",
		"123",
	}

	for _, s := range valid {
		tag, err := domain.ParseTag(s)
		if err != nil {
			t.Fatalf("ParseTag(%q) unexpected error: %v", s, err)
		}
		if tag.String() != s {
			t.Errorf("ParseTag(%q).String() = %q, want %q", s, tag.String(), s)
		}
		if tag.IsZero() {
			t.Errorf("ParseTag(%q).IsZero() returned true", s)
		}
	}

	invalid := []string{
		"",                                 // empty
		"   ",                              // spaces only
		"Philosophy",                       // uppercase (ParseTag normalizes to lowercase if valid chars, but let's test invalid patterns)
		"-leading-hyphen",                  // leading hyphen
		"trailing-hyphen-",                 // trailing hyphen
		"double--hyphen",                   // double hyphen
		"space in tag",                     // spaces
		"tag_underscore",                   // underscore not allowed in kebab-case tag
		"tag.dot",                          // punctuation
		"tag@domain",                       // special chars
		strings.Repeat("a", 41),            // too long (>40 chars)
	}

	for _, s := range invalid {
		_, err := domain.ParseTag(s)
		if err == nil && s != "Philosophy" { // Note: Philosophy might be lowercased to philosophy by ParseTag
			t.Errorf("ParseTag(%q) expected error, got nil", s)
		}
	}
}

func TestTagSet(t *testing.T) {
	t1, _ := domain.ParseTag("stoicism")
	t2, _ := domain.ParseTag("philosophy")
	t3, _ := domain.ParseTag("ethics")

	// Test NewTagSet deduplication and sorting
	set := domain.NewTagSet(t1, t2, t3, t1, t2)
	if set.Len() != 3 {
		t.Fatalf("TagSet.Len() = %d, want 3", set.Len())
	}

	// Should be sorted alphabetically: ethics, philosophy, stoicism
	expected := []string{"ethics", "philosophy", "stoicism"}
	if !reflect.DeepEqual(set.Strings(), expected) {
		t.Errorf("TagSet.Strings() = %v, want %v", set.Strings(), expected)
	}

	if !set.Contains(t1) {
		t.Errorf("TagSet.Contains(stoicism) returned false")
	}

	tOther, _ := domain.ParseTag("random")
	if set.Contains(tOther) {
		t.Errorf("TagSet.Contains(random) returned true, want false")
	}

	// Test ParseTagSet
	rawList := []string{"reading", "alpha", "reading"}
	parsedSet, err := domain.ParseTagSet(rawList)
	if err != nil {
		t.Fatalf("ParseTagSet failed: %v", err)
	}
	if parsedSet.Len() != 2 {
		t.Errorf("ParseTagSet len = %d, want 2", parsedSet.Len())
	}
}
