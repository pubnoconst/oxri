// Package journals provides the Gomponents catalog, single-entry permalink,
// and Make Room (vacate-space) views for Res Imperii.
package journals

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/views/layout"
	"github.com/yuin/goldmark"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed journals.css
var StyleCSS string

// StylePath is the public route for the journals component CSS.
const StylePath = "/views/journals/journals.css"

const MaxExcerptRunes = 280

var (
	blockEndRe   = regexp.MustCompile(`(?i)</(?:p|h[1-6]|li|blockquote|pre|div)>|<br\s*/?>`)
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
	attrSchemeRe = regexp.MustCompile(`(?i)\b(href|src)\s*=\s*"([^"]*)"`)
)

// RenderedEntry pairs a persisted Journal row with pre-rendered safe HTML, excerpt, and metadata.
type RenderedEntry struct {
	Entry          db.Journal
	RenderedHTML   string
	Excerpt        string
	FormattedDate  string
	Tags           []string
	EstimatedBytes int64
	FormattedSize  string
}

// CatalogView holds rendered entries alongside search query and active tag filters.
type CatalogView struct {
	Entries      []RenderedEntry
	AllTags      []string
	Query        string
	SelectedTags []string
}

func (v CatalogView) HasFilters() bool {
	return strings.TrimSpace(v.Query) != "" || len(v.SelectedTags) > 0
}

func (v CatalogView) IsTagSelected(tag string) bool {
	formatted := db.FormatTagTitleCase(tag)
	for _, t := range v.SelectedTags {
		if db.FormatTagTitleCase(t) == formatted {
			return true
		}
	}
	return false
}

// ToggleTagURL returns a /journals URL that toggles `tag` in the selected filter set
// while preserving the current search query `q`.
func (v CatalogView) ToggleTagURL(tag string) string {
	target := db.FormatTagTitleCase(tag)
	vals := url.Values{}
	if strings.TrimSpace(v.Query) != "" {
		vals.Set("q", strings.TrimSpace(v.Query))
	}
	found := false
	for _, existing := range v.SelectedTags {
		norm := db.FormatTagTitleCase(existing)
		if norm == target {
			found = true
			continue
		}
		if norm != "" {
			vals.Add("tag", norm)
		}
	}
	if !found && target != "" {
		vals.Add("tag", target)
	}
	encoded := vals.Encode()
	if encoded == "" {
		return "/journals"
	}
	return "/journals?" + encoded
}

// RenderMarkdown converts Markdown prose into sanitized HTML (raw HTML omitted by goldmark,
// input prose runes strictly whitelisted, and link schemes restricted to http/https/relative).
func RenderMarkdown(src string) string {
	cleanSrc := db.SanitizeProseInput(src)
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(cleanSrc), &buf); err != nil {
		return ""
	}
	return attrSchemeRe.ReplaceAllStringFunc(buf.String(), func(match string) string {
		sub := attrSchemeRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		attrName := sub[1]
		val := strings.TrimSpace(html.UnescapeString(sub[2]))
		lower := strings.ToLower(val)
		if strings.HasPrefix(lower, "https://") ||
			strings.HasPrefix(lower, "http://") ||
			(strings.HasPrefix(lower, "/") && !strings.HasPrefix(lower, "//")) ||
			strings.HasPrefix(lower, "#") {
			return match
		}
		return fmt.Sprintf(`%s="#"`, attrName)
	})
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
	estBytes := j.EstimatedBytes()
	return RenderedEntry{
		Entry:          j,
		RenderedHTML:   rendered,
		Excerpt:        Excerpt280(rendered),
		FormattedDate:  t.Local().Format("Jan 2, 2006"),
		Tags:           j.Tags(),
		EstimatedBytes: estBytes,
		FormattedSize:  FormatBytes(estBytes),
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

// Page renders a simple catalog view for a slice of rendered entries.
func Page(isAuth bool, entries []RenderedEntry) g.Node {
	return CatalogPage(isAuth, CatalogView{
		Entries: entries,
	})
}

// CatalogContentFragment renders the filter bar, batch action toolbar, and collage cards
// so they can be populated asynchronously via HTMX (/journals/content) after the page shell loads.
func CatalogContentFragment(isAuth bool, view CatalogView) g.Node {
	var filterBar g.Node
	if len(view.Entries) > 0 || len(view.AllTags) > 0 || view.HasFilters() {
		hiddenTagInputs := make([]g.Node, 0, len(view.SelectedTags))
		for _, t := range view.SelectedTags {
			hiddenTagInputs = append(hiddenTagInputs, h.Input(h.Type("hidden"), h.Name("tag"), h.Value(t)))
		}

		var clearLink g.Node
		if view.HasFilters() {
			clearLink = h.A(h.Href("/journals"), h.Class("filter-clear-link"), g.Text("Clear"))
		}

		var tagPills g.Node
		if len(view.AllTags) > 0 {
			pills := make([]g.Node, 0, len(view.AllTags)+1)
			pills = append(pills, h.Span(h.Class("tag-filter-label"), g.Text("Tags:")))
			for _, tag := range view.AllTags {
				cls := "tag-filter-pill"
				if view.IsTagSelected(tag) {
					cls += " is-active"
				}
				pills = append(pills, h.A(
					h.Href(view.ToggleTagURL(tag)),
					h.Class(cls),
					g.Attr("aria-pressed", fmt.Sprintf("%t", view.IsTagSelected(tag))),
					g.Text(tag),
				))
			}
			tagPills = h.Div(h.Class("journals-tag-filters"), g.Attr("role", "group"), g.Attr("aria-label", "Filter by tag"), g.Group(pills))
		}

		filterBar = h.Section(h.Class("journals-filter-bar"), g.Attr("aria-label", "Search and filter journals"),
			h.Form(h.Class("journals-search-form"), h.Method("GET"), h.Action("/journals"),
				g.Group(hiddenTagInputs),
				h.Input(
					h.Type("search"),
					h.Name("q"),
					h.Class("journals-search-input"),
					h.Placeholder("Search entries…"),
					h.Value(view.Query),
					g.Attr("aria-label", "Search journal entries"),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-neutral btn-sm"), g.Text("Search")),
				clearLink,
			),
			tagPills,
		)
	}

	var batchBar g.Node
	if isAuth && len(view.Entries) > 0 {
		batchBar = h.Form(h.ID("batch-form"), h.Class("journals-batch-bar"), h.Method("POST"), h.Action("/journals/batch"),
			h.Div(h.Class("batch-select-group"),
				h.Label(h.Class("batch-select-all"),
					h.Input(h.Type("checkbox"), h.ID("select-all-entries")),
					h.Span(g.Text("Select all")),
				),
				h.Span(h.ID("selected-count"), h.Class("batch-count"), g.Text("0 selected")),
			),
			h.Div(h.Class("batch-buttons"),
				h.Button(h.Type("submit"), h.Name("action"), h.Value("publish_selected"), h.Class("btn btn-neutral btn-sm"), g.Attr("data-requires-selection", "true"), h.Disabled(), g.Text("Publish selected")),
				h.Button(h.Type("submit"), h.Name("action"), h.Value("unpublish_selected"), h.Class("btn btn-neutral btn-sm"), g.Attr("data-requires-selection", "true"), h.Disabled(), g.Text("Unpublish selected")),
				h.Button(h.Type("submit"), h.Name("action"), h.Value("delete_selected"), h.Class("btn btn-destructive btn-sm"), g.Attr("data-requires-selection", "true"), h.Disabled(), g.Text("Delete selected")),
				h.Span(h.Class("batch-divider"), g.Attr("aria-hidden", "true")),
				h.Button(h.Type("submit"), h.Name("action"), h.Value("unpublish_all"), h.Class("btn btn-neutral btn-sm"), g.Text("Unpublish all")),
				h.Button(h.Type("submit"), h.Name("action"), h.Value("delete_all"), h.Class("btn btn-destructive btn-sm"), g.Text("Delete all")),
			),
		)
	}

	var feedContent g.Node
	if len(view.Entries) == 0 {
		msg := "No entries yet."
		if view.HasFilters() {
			msg = "No matching entries found."
		}
		feedContent = h.P(h.Class("journals-empty"), g.Text(msg))
	} else {
		cards := make([]g.Node, 0, len(view.Entries))
		for _, item := range view.Entries {
			cards = append(cards, EntryCard(isAuth, item, true))
		}
		feedContent = h.Div(h.Class("journals-feed is-collage-grid"), g.Group(cards))
	}

	return g.Group([]g.Node{
		filterBar,
		batchBar,
		feedContent,
	})
}

func renderJournalsHeader(isAuth bool) g.Node {
	var authLink g.Node
	if isAuth {
		authLink = h.Form(h.Class("auth-action-form"), h.Method("POST"), h.Action("/logout"),
			h.Button(h.Type("submit"), h.Class("auth-action-link"), g.Text("Sign out")),
		)
	} else {
		authLink = h.A(h.Class("auth-action-link"), h.Href("/login?next=/journals"), g.Text("Author sign in"))
	}

	return h.Header(h.Class("journals-header"),
		layout.BackLink("/", "Back"),
		h.Div(h.Class("journals-title-row"),
			h.H1(g.Text("Journals")),
			h.Div(h.Class("journals-actions"),
				h.A(h.Class("new-entry-link btn btn-neutral"), h.Href("/journals/new"), g.Text("Add new entry")),
				authLink,
			),
		),
	)
}

// CatalogShellPage renders the /journals document frame in 0ms independent of the database.
// Once mounted in the browser, HTMX fetches contentURL (/journals/content) from the remote DB
// and populates #journals-catalog-stage asynchronously.
func CatalogShellPage(isAuth bool, contentURL string) g.Node {
	if contentURL == "" {
		contentURL = "/journals/content"
	}

	var catalogScript g.Node
	if isAuth {
		catalogScript = h.Script(h.Src("/assets/js/journals-catalog.js?v=2"))
	}

	rippleStyle := "--ripple-x:50%;--ripple-y:38%;--ripple-diameter:150vmax;"

	return layout.Doc(
		"Journals - RES IMPERII",
		StylePath+"?v=5",
		h.Div(h.Class("water-stage"), g.Attr("data-water-stage", ""),
			h.Main(h.Class("journals-page"),
				renderJournalsHeader(isAuth),
				h.Div(
					h.ID("journals-catalog-stage"),
					g.Attr("hx-get", contentURL),
					g.Attr("hx-trigger", "load"),
					g.Attr("hx-swap", "innerHTML swap:120ms"),
					h.P(h.Class("journals-empty journals-loading-placeholder"), g.Attr("aria-live", "polite"), g.Text("Loading entries…")),
				),
			),
		),
		h.Div(
			h.Class("card-water-ripple is-pagewide"),
			g.Attr("style", rippleStyle),
			g.Attr("data-water-dismiss-on-swap", "journals-catalog-stage"),
			g.Attr("aria-hidden", "true"),
			h.Span(h.Class("card-water-drop")),
			h.Span(h.Class("card-water-wave wave-1")),
			h.Span(h.Class("card-water-wave wave-2")),
			h.Span(h.Class("card-water-wave wave-3")),
		),
		h.Script(h.Src("/assets/js/water-ripple.js?v=2")),
		h.Script(h.Src("/assets/js/journal-media.js?v=8")),
		catalogScript,
	)
}

// CatalogPage renders the main /journals collage grid with search, tag filters, and batch management.
func CatalogPage(isAuth bool, view CatalogView) g.Node {
	var catalogScript g.Node
	if isAuth {
		catalogScript = h.Script(h.Src("/assets/js/journals-catalog.js?v=2"))
	}

	return layout.Doc(
		"Journals - RES IMPERII",
		StylePath+"?v=5",
		h.Main(h.Class("journals-page"),
			renderJournalsHeader(isAuth),
			h.Div(h.ID("journals-catalog-stage"),
				CatalogContentFragment(isAuth, view),
			),
		),
		h.Script(h.Src("/assets/js/water-ripple.js?v=2")),
		h.Script(h.Src("/assets/js/journal-media.js?v=8")),
		catalogScript,
	)
}

// EntryPage renders the single-entry permalink reading view (/journals/{id}).
func EntryPage(isAuth bool, item RenderedEntry) g.Node {
	return layout.Doc(
		"Journal Entry - RES IMPERII",
		StylePath+"?v=5",
		h.Main(h.Class("journals-page journals-single-page"),
			h.Header(h.Class("journals-header"),
				layout.BackLink("/journals", "Journals"),
			),
			h.Div(h.Class("journals-feed journals-single"),
				EntryCard(isAuth, item, false),
			),
		),
		h.Script(h.Src("/assets/js/water-ripple.js?v=2")),
		h.Script(h.Src("/assets/js/journal-media.js?v=8")),
	)
}

// EntryCard renders either a compact collage card or a full permalink card.
func EntryCard(isAuth bool, item RenderedEntry, isCollage bool) g.Node {
	cls := "journal-card"
	if isCollage {
		cls += " is-collage"
	} else {
		cls += " is-full"
	}
	if !item.Entry.Published {
		cls += " is-unpublished"
	}

	var selectCb g.Node
	if isAuth && isCollage {
		selectCb = h.Input(
			h.Type("checkbox"),
			h.Class("entry-select-cb"),
			g.Attr("form", "batch-form"),
			h.Name("ids"),
			h.Value(fmt.Sprintf("%d", item.Entry.ID)),
			g.Attr("aria-label", "Select journal entry"),
		)
	}

	var dateNode g.Node
	if isCollage {
		dateNode = h.A(h.Class("entry-date entry-card-link"), h.Href(EntryURL(item.Entry.ID)), g.Text(item.FormattedDate))
	} else {
		dateNode = h.Span(h.Class("entry-date"), g.Text(item.FormattedDate))
	}

	var draftBadge g.Node
	if !item.Entry.Published {
		draftBadge = h.Span(h.Class("entry-badge is-draft"), g.Text("Unpublished"))
	}
	var bookmarkBadge g.Node
	if item.Entry.Bookmarked {
		bookmarkBadge = h.Span(h.Class("entry-badge is-bookmarked"), g.Text("★"))
	}

	var tagsBlock g.Node
	if len(item.Tags) > 0 {
		chips := make([]g.Node, 0, len(item.Tags))
		for _, tag := range item.Tags {
			chips = append(chips, h.Span(h.Class("journal-tag-chip"), g.Text(tag)))
		}
		tagsBlock = h.Div(h.Class("journal-entry-tags"), g.Group(chips))
	}

	var contentBlock g.Node
	if isCollage {
		contentBlock = h.P(h.Class("journal-entry-excerpt"), g.Text(item.Excerpt))
	} else {
		contentBlock = h.Div(h.Class("journal-entry-body journal-rendered-content"), g.Raw(item.RenderedHTML))
	}

	var adminFooter g.Node
	if isAuth {
		bmText := "Bookmark"
		if item.Entry.Bookmarked {
			bmText = "Unbookmark"
		}
		var pubForm g.Node
		if item.Entry.Published {
			pubForm = h.Form(h.Method("POST"), h.Action(ActionURL(item.Entry.ID, "unpublish")),
				h.Button(h.Type("submit"), h.Class("btn btn-neutral btn-sm"), g.Text("Unpublish")),
			)
		} else {
			pubForm = h.Form(h.Method("POST"), h.Action(ActionURL(item.Entry.ID, "publish")),
				h.Button(h.Type("submit"), h.Class("btn btn-neutral btn-sm"), g.Text("Publish")),
			)
		}

		adminFooter = h.Footer(h.Class("journal-card-actions"),
			h.A(h.Class("btn btn-neutral btn-sm"), h.Href(EditURL(item.Entry.ID)), g.Text("Edit")),
			h.Form(h.Method("POST"), h.Action(ActionURL(item.Entry.ID, "bookmark")),
				h.Button(h.Type("submit"), h.Class("btn btn-neutral btn-sm"), g.Text(bmText)),
			),
			pubForm,
			h.Form(
				h.Method("POST"),
				h.Action(ActionURL(item.Entry.ID, "delete")),
				g.Attr("data-confirm", "Delete this journal entry permanently?"),
				h.Button(h.Type("submit"), h.Class("btn btn-destructive btn-sm"), g.Text("Delete")),
			),
		)
	}

	return h.Article(h.Class(cls),
		h.Header(h.Class("journal-card-meta"),
			h.Div(h.Class("journal-meta-left"),
				selectCb,
				dateNode,
			),
			h.Div(h.Class("journal-meta-badges"),
				draftBadge,
				bookmarkBadge,
			),
		),
		h.Div(h.Class("journal-media-slot journal-preview-media"), g.Attr("data-media-url", item.Entry.MediaURL)),
		tagsBlock,
		contentBlock,
		adminFooter,
	)
}
