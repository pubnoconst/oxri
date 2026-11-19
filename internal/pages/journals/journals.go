package journals

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/yuin/goldmark"
)

//go:embed style.css
var StyleCSS string

const MaxExcerptRunes = 280

var (
	blockEndRe = regexp.MustCompile(`(?i)</(?:p|h[1-6]|li|blockquote|pre|div)>|<br\s*/?>`)
	htmlTagRe  = regexp.MustCompile(`<[^>]+>`)
)

// RenderedEntry pairs a persisted Journal row with pre-rendered safe HTML, excerpt, and metadata.
type RenderedEntry struct {
	Entry         db.Journal
	RenderedHTML  string
	Excerpt       string
	FormattedDate string
	Tags          []string
}

// RenderMarkdown converts Markdown prose into sanitized HTML (raw HTML omitted by goldmark).
func RenderMarkdown(src string) string {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return buf.String()
}

// Excerpt280 extracts plain text from Markdown-rendered HTML and caps it at 280 characters.
func Excerpt280(renderedHTML string) string {
	spaced := blockEndRe.ReplaceAllString(renderedHTML, " ")
	stripped := htmlTagRe.ReplaceAllString(spaced, "")
	unescaped := html.UnescapeString(stripped)
	collapsed := strings.Join(strings.Fields(unescaped), " ")

	if utf8.RuneCountInString(collapsed) <= MaxExcerptRunes {
		return collapsed
	}
	runes := []rune(collapsed)
	return strings.TrimRight(string(runes[:MaxExcerptRunes-1]), " ") + "…"
}

// RenderSingle prepares a single journal entry for HTML rendering.
func RenderSingle(j db.Journal) RenderedEntry {
	t := j.CreatedAt
	if t.IsZero() {
		t = time.UnixMilli(j.ID)
	}
	rendered := RenderMarkdown(j.Body)
	return RenderedEntry{
		Entry:         j,
		RenderedHTML:  rendered,
		Excerpt:       Excerpt280(rendered),
		FormattedDate: t.Local().Format("Jan 2, 2006"),
		Tags:          j.Tags(),
	}
}

// RenderEntries prepares a slice of journal entries for catalog rendering.
func RenderEntries(entries []db.Journal) []RenderedEntry {
	out := make([]RenderedEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, RenderSingle(e))
	}
	return out
}

func EntryURL(id int64) string {
	return fmt.Sprintf("/journals/%d", id)
}

func EditURL(id int64) string {
	return fmt.Sprintf("/journals/%d/edit", id)
}

func ActionURL(id int64, action string) string {
	return fmt.Sprintf("/journals/%d/%s", id, action)
}
