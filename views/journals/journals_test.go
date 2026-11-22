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

func TestRenderMarkdownSanitizesRunesAndSchemes(t *testing.T) {
	raw := "Hello 🚀 мир [bad](javascript:alert(1)) and [good](https://example.com)"
	rendered := RenderMarkdown(raw)
	if strings.Contains(rendered, "🚀") || strings.Contains(rendered, "мир") {
		t.Errorf("RenderMarkdown failed to strip disallowed runes: %q", rendered)
	}
	if strings.Contains(strings.ToLower(rendered), "javascript:") {
		t.Errorf("RenderMarkdown failed to neutralize javascript: scheme: %q", rendered)
	}
	if !strings.Contains(rendered, `href="https://example.com"`) {
		t.Errorf("RenderMarkdown should preserve valid https:// link: %q", rendered)
	}
}
