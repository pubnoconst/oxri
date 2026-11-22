package journals

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExcerpt280(t *testing.T) {
	shortHTML := RenderMarkdown("# Title\n\nShort paragraph with **bold** text.")
	gotShort := Excerpt280(shortHTML)
	if gotShort != "Title Short paragraph with bold text." {
		t.Errorf("unexpected short excerpt: %q", gotShort)
	}

	longProse := strings.Repeat("Res Imperii journal prose word ", 25)
	longHTML := RenderMarkdown(longProse)
	gotLong := Excerpt280(longHTML)
	if utf8.RuneCountInString(gotLong) > MaxExcerptRunes {
		t.Errorf("excerpt exceeded %d runes: got %d", MaxExcerptRunes, utf8.RuneCountInString(gotLong))
	}
	if !strings.HasSuffix(gotLong, "…") {
		t.Errorf("expected truncated excerpt to end with ellipsis, got %q", gotLong)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := FormatBytes(512); got != "512 B" {
		t.Errorf("FormatBytes(512) = %q", got)
	}
	if got := FormatBytes(2048); got != "2.0 KB" {
		t.Errorf("FormatBytes(2048) = %q", got)
	}
}
