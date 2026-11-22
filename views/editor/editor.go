// Package editor provides the Gomponents 2-column compose & preview view for journals.
package editor

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/oxri/oxri-go/views/layout"
	"github.com/oxri/oxri-go/views/loading"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed editor.css
var StyleCSS string

// StylePath is the public route for the journal editor component CSS.
const StylePath = "/views/editor/editor.css"

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

// Page renders a fresh journal editor with default starter content.
func Page(isAuth bool) g.Node {
	return FormPage(isAuth, FormData{
		Published: true,
		Body:      DefaultBody,
	})
}

// FormPage renders the journal compose and live preview view.
func FormPage(isAuth bool, data FormData) g.Node {
	eyebrow := "JOURNALS / NEW ENTRY"
	heading := "A new entry"
	if data.IsEdit {
		eyebrow = "JOURNALS / EDIT ENTRY"
		heading = "Edit entry"
	}

	var authNotice g.Node
	if isAuth {
		authNotice = h.P(h.Class("editor-notice"), g.Text("Signed in"))
	} else {
		authNotice = h.P(h.Class("editor-notice"),
			g.Text("Draft preview · "),
			h.A(h.Href("/login?next=/journals/new"), h.Style("color: inherit; text-decoration: underline;"), g.Text("Sign in")),
		)
	}

	var errorBanner g.Node
	if data.ErrorMsg != "" {
		errorBanner = h.Div(h.Class("editor-server-error"), g.Attr("role", "alert"), g.Text(data.ErrorMsg))
	}

	var optionsBlock g.Node
	if isAuth {
		pubInputAttrs := []g.Node{h.Type("checkbox"), h.Name("published"), h.Value("1")}
		if data.Published {
			pubInputAttrs = append(pubInputAttrs, h.Checked())
		}
		bmInputAttrs := []g.Node{h.Type("checkbox"), h.Name("bookmarked"), h.Value("1")}
		if data.Bookmarked {
			bmInputAttrs = append(bmInputAttrs, h.Checked())
		}

		optionsBlock = h.Div(h.Class("editor-options"),
			h.Label(h.Class("checkbox-label"),
				h.Input(pubInputAttrs...),
				h.Span(g.Text("Published (visible on public catalog)")),
			),
			h.Label(h.Class("checkbox-label"),
				h.Input(bmInputAttrs...),
				h.Span(g.Text("Bookmarked")),
			),
		)
	}

	var actionsBlock g.Node
	if isAuth {
		btnText := "Publish entry"
		if data.IsEdit {
			btnText = "Save changes"
		}
		actionsBlock = h.Div(h.Class("journal-editor-actions"),
			h.Button(
				h.Type("submit"),
				h.ID("publish-submit-btn"),
				g.Attr("data-is-auth", "true"),
				h.Class("btn btn-encouraged"),
				h.Disabled(),
				g.Text(btnText),
			),
			h.A(h.Href("/journals"), h.Class("btn btn-neutral"), g.Text("Cancel")),
		)
	} else {
		actionsBlock = h.Div(h.Class("journal-editor-actions"),
			h.P(h.Class("publish-note"), g.Text("Sign in to publish an entry.")),
			h.Button(
				h.Type("button"),
				h.ID("publish-submit-btn"),
				g.Attr("data-is-auth", "false"),
				h.Class("btn btn-encouraged"),
				h.Disabled(),
				g.Text("Publish entry"),
			),
		)
	}

	return layout.Doc(
		data.PageTitle(),
		StylePath+"?v=1",
		h.Main(h.Class("journal-editor-page"),
			h.Header(h.Class("journal-editor-header"),
				layout.BackLink("/journals", "Journals"),
				h.Div(h.Class("editor-heading"),
					h.Div(
						h.P(h.Class("editor-eyebrow"), g.Text(eyebrow)),
						h.H1(g.Text(heading)),
					),
					authNotice,
				),
			),
			errorBanner,
			h.Form(
				h.ID("journal-editor-form"),
				h.Method("POST"),
				h.Action(data.ActionURL()),
				g.Attr("data-confirm-loading", "true"),
				g.Attr("novalidate", ""),
				h.Div(h.Class("journal-editor-grid"),
					// Left pane: Compose
					h.Section(h.Class("editor-pane editor-compose-card"),
						h.Div(h.Class("editor-section-heading"),
							h.H2(g.Text("Compose")),
							h.Span(g.Text("01")),
						),
						h.Div(h.Class("editor-field"),
							h.Div(h.Class("field-label-row"),
								h.Label(h.For("journal-media-link"), g.Text("Media link")),
								h.Span(h.ID("media-char-count"), h.Class("field-counter"), g.Text("0 / 128")),
							),
							h.Input(
								h.ID("journal-media-link"),
								h.Name("media_url"),
								h.Type("url"),
								h.MaxLength("128"),
								h.Required(),
								h.Placeholder("https://..."),
								h.AutoComplete("url"),
								h.Value(data.MediaURL),
							),
							h.P(h.Class("field-hint"), g.Text("Required (1–128 chars). Video, audio, image, or X / Twitter post link.")),
							h.P(h.ID("media-error"), h.Class("field-error"), g.Attr("aria-live", "polite")),
						),
						h.Div(h.Class("editor-field"),
							h.Div(h.Class("field-label-row"),
								h.Label(h.For("journal-tags"), g.Text("Tags")),
								h.Span(h.ID("tags-count"), h.Class("field-counter"), g.Text("0 / 3")),
							),
							h.Input(
								h.ID("journal-tags"),
								h.Name("tags"),
								h.Type("text"),
								h.AutoComplete("off"),
								h.Placeholder("Log, Dev, Note"),
								h.Value(data.Tags),
								g.Attr("data-existing-tags", data.ExistingTagsCSV()),
							),
							h.Div(h.ID("tag-suggestions"), h.Class("tag-suggestions"), g.Attr("role", "listbox"), g.Attr("aria-label", "Tag suggestions")),
							h.P(h.Class("field-hint"), g.Text("Up to 3 unique tags, up to 4 characters each (letters and numbers only; no emojis).")),
							h.P(h.ID("tags-error"), h.Class("field-error"), g.Attr("aria-live", "polite")),
						),
						h.Div(h.Class("editor-field editor-content-field"),
							h.Label(h.For("journal-content"), g.Text("Entry content")),
							h.Textarea(
								h.ID("journal-content"),
								h.Name("body"),
								h.Rows("16"),
								h.Required(),
								g.Text(data.Body),
							),
							h.P(h.ID("body-error"), h.Class("field-error"), g.Attr("aria-live", "polite")),
						),
						optionsBlock,
					),
					// Right pane: Preview
					h.Section(h.Class("editor-pane editor-preview-card"),
						h.Div(h.Class("editor-section-heading"),
							h.H2(g.Text("Preview")),
							h.Span(g.Text("02")),
						),
						h.Article(h.ID("journal-preview"), h.Class("journal-preview"),
							h.Div(h.ID("journal-preview-media"), h.Class("journal-preview-media"), h.Title("Media preview")),
							h.Div(h.Class("journal-preview-body"),
								h.Div(h.ID("journal-preview-tags"), h.Class("journal-preview-tags")),
								h.Div(h.ID("journal-rendered-content"), h.Class("journal-rendered-content")),
							),
						),
					),
				),
				actionsBlock,
			),
		),
		loading.Overlay(loading.DefaultPhrase, false),
		h.Script(h.Src("/assets/js/markdown-it.min.js?v=1")),
		h.Script(h.Src("/assets/js/purify.min.js?v=1")),
		h.Script(h.Src("https://platform.twitter.com/widgets.js"), h.Charset("utf-8"), h.Async()),
		h.Script(h.Src("/assets/js/journal-media.js?v=1")),
		h.Script(h.Src("/assets/js/journal-editor.js?v=9")),
		h.Script(h.Src("/assets/js/loading-transition.js?v=1")),
	)
}
