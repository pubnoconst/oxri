package journals

import (
	"fmt"
	"sort"
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

// A signed-out reader is never offered "Add new entry"; the author is.
func TestNewEntryLinkOnlyForAuthor(t *testing.T) {
	render := func(isAuth bool) string {
		var b strings.Builder
		_ = CatalogPage(isAuth, CatalogView{}).Render(&b)
		return b.String()
	}
	if out := render(false); strings.Contains(out, "Add new entry") || strings.Contains(out, `href="/journals/new"`) {
		t.Errorf("signed-out catalog offers a new entry")
	}
	if out := render(false); !strings.Contains(out, "Author sign in") {
		t.Errorf("signed-out catalog lost its sign-in link")
	}
	if out := render(true); !strings.Contains(out, "Add new entry") {
		t.Errorf("author's catalog lost its new-entry link")
	}
}

// The catalog leads with the most used tags; a selected tag is always shown;
// the rest wait, alphabetical, behind "More tags".
func TestCatalogTagGroups(t *testing.T) {
	var byUse []string
	for i := 0; i < CatalogTagLimit+4; i++ {
		byUse = append(byUse, fmt.Sprintf("T%02d", CatalogTagLimit+4-i)) // reverse-alphabetical, by use
	}
	view := CatalogView{AllTags: byUse, SelectedTags: []string{byUse[len(byUse)-1]}}
	shown, more := view.TagGroups()
	if len(shown) != CatalogTagLimit+1 || shown[0] != byUse[0] || shown[len(shown)-1] != byUse[len(byUse)-1] {
		t.Errorf("shown: %v", shown)
	}
	if len(more) != 3 || !sort.StringsAreSorted(more) {
		t.Errorf("more: %v (want 3, alphabetical)", more)
	}

	var b strings.Builder
	_ = CatalogContentFragment(false, view).Render(&b)
	out := b.String()
	if !strings.Contains(out, `<details class="tag-filter-more"><summary class="link-quiet">More tags (3)</summary>`) {
		t.Errorf("missing More tags disclosure")
	}
	small := CatalogView{AllTags: byUse[:3]}
	b.Reset()
	_ = CatalogContentFragment(false, small).Render(&b)
	if strings.Contains(b.String(), "tag-filter-more") {
		t.Errorf("no disclosure when every tag fits")
	}
}
