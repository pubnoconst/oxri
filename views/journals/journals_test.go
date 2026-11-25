package journals

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/paging"
)

func TestExcerpt280(t *testing.T) {
	shortHTML := RenderMarkdown("# Title\n\nShort paragraph with **bold** text.")
	gotShort := Excerpt280(shortHTML)
	if gotShort != "Title\n\nShort paragraph with bold text." {
		t.Errorf("unexpected short excerpt: %q", gotShort)
	}

	// The author's line breaks are kept; more than one blank line is not,
	// and spaces within a line collapse.
	lines := Excerpt280(RenderMarkdown("one  line\nnext line\n\n\n\n\nafter a gap<br>\n\n- item a\n- item b"))
	if want := "one line\nnext line\n\nafter a gap\n\nitem a\n\nitem b"; lines != want {
		t.Errorf("line breaks: got %q, want %q", lines, want)
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

// Search, tags and paging swap only the catalog in place (catalogSwap); the
// cards' own actions stay ordinary forms and links, so they are never
// swapped into the catalog by mistake.
func TestCatalogSwapsInPlaceOnlyFromFilterAndPager(t *testing.T) {
	var b strings.Builder
	_ = CatalogPage(true, CatalogView{
		Entries: RenderEntries([]db.Journal{{ID: 7, MediaURL: "https://example.org/a.png", Body: "An entry", Published: true}}),
		AllTags: []string{"Good"},
		Page:    paging.Page{Number: 2, Count: 3, Size: 12, Total: 30},
	}).Render(&b)
	out := b.String()

	if n := strings.Count(out, `hx-boost="true"`); n != 2 {
		t.Fatalf("hx-boost on %d elements, want 2 (the filter bar and the pager)", n)
	}
	for _, want := range []string{
		`class="journals-filter-bar" aria-label="Search and filter journals" hx-boost="true"`,
		`hx-swap="outerHTML show:none"`,
		`class="journals-pagination" aria-label="Journal pages" hx-boost="true"`,
		`hx-swap="outerHTML show:#journals-catalog-stage:top"`,
		`hx-select="#journals-catalog-stage"`,
		`id="journals-search"`,
		`<div id="journals-catalog-stage">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("catalog lacks %s", want)
		}
	}
	feed := out[strings.Index(out, `class="journals-feed`):]
	feed = feed[:strings.Index(feed, `class="journals-pagination"`)]
	if strings.Contains(feed, "hx-boost") {
		t.Error("the cards must not be boosted: their actions are ordinary forms")
	}
}

// The card's stretched link (on the date) covers the whole card: the date
// must not be dimmed by opacity, which would seal the overlay inside it and
// leave the blank space under an excerpt dead (a click there did nothing).
func TestCardLinkCoversTheWholeCard(t *testing.T) {
	var b strings.Builder
	_ = EntryCard(false, RenderedEntry{Entry: db.Journal{ID: 3, MediaURL: "https://example.org/a.png", Body: "x", Published: true}, Excerpt: "x"}, true).Render(&b)
	if !strings.Contains(b.String(), `class="entry-date entry-card-link"`) {
		t.Errorf("the card's date link changed: %s", b.String())
	}
	rule := regexp.MustCompile(`(?s)\.entry-date\s*\{[^}]*\}|\.entry-card-link\s*\{[^}]*\}`)
	for _, r := range rule.FindAllString(StyleCSS, -1) {
		if strings.Contains(r, "opacity") {
			t.Errorf("the card link is dimmed by opacity, which traps its overlay: %s", r)
		}
	}
}
