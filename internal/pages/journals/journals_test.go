package journals

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExcerpt280(t *testing.T) {
	shortMD := "# Heading\n\nSome **bold** text and a [link](https://example.com)."
	gotShort := Excerpt280(RenderMarkdown(shortMD))
	expectedShort := "Heading Some bold text and a link."
	if gotShort != expectedShort {
		t.Errorf("expected %q, got %q", expectedShort, gotShort)
	}

	longMD := "# Long Entry\n\n" + strings.Repeat("word ", 80)
	gotLong := Excerpt280(RenderMarkdown(longMD))
	runeCount := utf8.RuneCountInString(gotLong)
	if runeCount > MaxExcerptRunes {
		t.Errorf("excerpt rune count %d exceeded max %d", runeCount, MaxExcerptRunes)
	}
	if !strings.HasSuffix(gotLong, "…") {
		t.Errorf("expected truncated excerpt to end with ellipsis, got %q", gotLong)
	}
}
