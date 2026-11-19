package journals

import (
	"bytes"
	_ "embed"
	"fmt"
	"time"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/yuin/goldmark"
)

//go:embed style.css
var StyleCSS string

// RenderedEntry pairs a persisted Journal row with pre-rendered safe HTML and display metadata.
type RenderedEntry struct {
	Entry         db.Journal
	RenderedHTML  string
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

// RenderSingle prepares a single journal entry for HTML rendering.
func RenderSingle(j db.Journal) RenderedEntry {
	t := j.CreatedAt
	if t.IsZero() {
		t = time.UnixMilli(j.ID)
	}
	return RenderedEntry{
		Entry:         j,
		RenderedHTML:  RenderMarkdown(j.Body),
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
