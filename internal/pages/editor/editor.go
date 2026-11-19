package editor

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed style.css
var StyleCSS string

// DefaultBody provides starter Markdown when composing a new journal entry.
const DefaultBody = `# A journal entry

Write here and see it take shape in the preview.

## A section

Add **emphasis**, lists, links, or ` + "`inline code`" + `.

- First thought
- Another thought`

// FormData carries field state for creating or editing a journal entry.
type FormData struct {
	IsEdit       bool
	EntryID      int64
	MediaURL     string
	Tags         string
	Body         string
	Bookmarked   bool
	Published    bool
	ErrorMsg     string
	ExistingTags []string
}

func (d FormData) ActionURL() string {
	if d.IsEdit {
		return fmt.Sprintf("/journals/%d", d.EntryID)
	}
	return "/journals"
}

func (d FormData) PageTitle() string {
	if d.IsEdit {
		return "Edit Entry - RES IMPERII"
	}
	return "New Journal Entry - RES IMPERII"
}

func (d FormData) ExistingTagsCSV() string {
	return strings.Join(d.ExistingTags, ",")
}
