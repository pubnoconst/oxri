// Package journals provides the Gomponents catalog, single-entry permalink,
// and Make Room (vacate-space) views for Res Imperii.
package journals

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/oxri/oxri-go/assets"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/domain"
	"github.com/oxri/oxri-go/internal/paging"
	"github.com/oxri/oxri-go/internal/tweet"
	"github.com/oxri/oxri-go/views/layout"
	passageview "github.com/oxri/oxri-go/views/passage"
	tweetview "github.com/oxri/oxri-go/views/tweet"
	"github.com/yuin/goldmark"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed journals.css
var StyleCSS string

// StylePath is the public route for the journals component CSS.
const StylePath = "/views/journals/journals.css"

// Style is the content-hashed handle for StyleCSS (URL and route handler).
// The clipping stylesheets (views/tweet, then views/passage, which builds on
// it) are folded in: journal cards are where clippings appear.
var Style = assets.Component(StylePath, StyleCSS+"\n"+tweetview.StyleCSS+"\n"+passageview.StyleCSS)

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
	// Tweet is the site's own drawing of the linked post on X, when its
	// snapshot exists (routes attach it); nil leaves the slot to X's embed.
	Tweet *tweet.Snapshot
	// Passage is the Bible passage the entry quotes, drawn by the server
	// (views/passage); nil for any other link.
	Passage *domain.Passage
}

// CatalogPageSize is how many entries one catalog page shows. Most entries
// embed a tweet (a cross-origin iframe each), so a page is kept small enough
// for budget phones; 12 fills 1, 2 or 3 grid columns evenly.
const CatalogPageSize = 12

// CatalogView holds one page of rendered entries alongside search query and
// active tag filters.
type CatalogView struct {
	Entries      []RenderedEntry // this page's entries only
	AllTags      []string
	Query        string
	SelectedTags []string
	Page         paging.Page
}

// filterValues is the current search and tag selection as query values.
func (v CatalogView) filterValues() url.Values {
	vals := url.Values{}
	if q := strings.TrimSpace(v.Query); q != "" {
		vals.Set("q", q)
	}
	for _, t := range v.SelectedTags {
		if norm := db.FormatTagTitleCase(t); norm != "" {
			vals.Add("tag", norm)
		}
	}
	return vals
}

// PageURL returns the /journals URL of page n with the current search and tag
// filters kept. Page 1 has no page parameter.
func (v CatalogView) PageURL(n int) string {
	vals := v.filterValues()
	if n > 1 {
		vals.Set("page", strconv.Itoa(n))
	}
	if encoded := vals.Encode(); encoded != "" {
		return "/journals?" + encoded
	}
	return "/journals"
}

// CatalogTagLimit is how many tags the catalog shows before "More tags": the
// most used ones (AllTags is ordered by use, db.TagsByUse).
const CatalogTagLimit = 12

// TagGroups splits AllTags into the ones shown at once (the most used, plus
// any selected tag, so a filter in effect is always visible and removable)
// and the rest, alphabetical, behind "More tags".
func (v CatalogView) TagGroups() (shown, more []string) {
	for i, tag := range v.AllTags {
		if i < CatalogTagLimit || v.IsTagSelected(tag) {
			shown = append(shown, tag)
		} else {
			more = append(more, tag)
		}
	}
	sort.Strings(more)
	return shown, more
}

func (v CatalogView) tagPill(tag string) g.Node {
	cls := "tag-filter-pill"
	if v.IsTagSelected(tag) {
		cls += " is-active"
	}
	return h.A(
		h.Href(v.ToggleTagURL(tag)),
		h.Class(cls),
		g.Attr("aria-pressed", fmt.Sprintf("%t", v.IsTagSelected(tag))),
		g.Text(tag),
	)
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
	var passage *domain.Passage
	if p, ok := j.Passage(); ok {
		passage = &p
	}
	return RenderedEntry{
		Passage:        passage,
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

// CatalogContentFragment renders the filter bar, batch action toolbar, and
// collage cards, inside /journals (CatalogPage).
func CatalogContentFragment(isAuth bool, view CatalogView) g.Node {
	var filterBar g.Node
	if len(view.Entries) > 0 || len(view.AllTags) > 0 || view.HasFilters() {
		hiddenTagInputs := make([]g.Node, 0, len(view.SelectedTags))
		for _, t := range view.SelectedTags {
			hiddenTagInputs = append(hiddenTagInputs, h.Input(h.Type("hidden"), h.Name("tag"), h.Value(t)))
		}

		var clearLink g.Node
		if view.HasFilters() {
			clearLink = h.A(h.Href("/journals"), h.Class("filter-clear-link link-quiet"), g.Text("Clear"))
		}

		var tagPills g.Node
		if len(view.AllTags) > 0 {
			shown, more := view.TagGroups()
			pills := make([]g.Node, 0, len(shown)+2)
			pills = append(pills, h.Span(h.Class("tag-filter-label"), g.Text("Tags:")))
			for _, tag := range shown {
				pills = append(pills, view.tagPill(tag))
			}
			if len(more) > 0 {
				morePills := make([]g.Node, 0, len(more))
				for _, tag := range more {
					morePills = append(morePills, view.tagPill(tag))
				}
				pills = append(pills, h.Details(h.Class("tag-filter-more"),
					h.Summary(h.Class("link-quiet"), g.Text(fmt.Sprintf("More tags (%d)", len(more)))),
					h.Div(h.Class("tag-filter-more-list"), g.Group(morePills)),
				))
			}
			tagPills = h.Div(h.Class("journals-tag-filters"), g.Attr("role", "group"), g.Attr("aria-label", "Filter by tag"), g.Group(pills))
		}

		filterBar = h.Section(h.Class("journals-filter-bar"), g.Attr("aria-label", "Search and filter journals"),
			catalogSwap("none"),
			h.Form(h.Class("journals-search-form"), h.Method("GET"), h.Action("/journals"),
				g.Group(hiddenTagInputs),
				h.Input(
					h.ID("journals-search"), // htmx gives focus back to this id after a swap
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
					h.Span(g.Text(selectAllLabel(view.Page))),
				),
				h.Span(h.ID("selected-count"), h.Class("batch-count"), g.Text("0 selected")),
			),
			h.Div(h.Class("batch-buttons"),
				h.Button(h.Type("submit"), h.Name("action"), h.Value("publish_selected"), h.Class("btn btn-neutral btn-sm"), g.Attr("data-requires-selection", "true"), h.Disabled(), g.Text("Publish selected")),
				h.Button(h.Type("submit"), h.Name("action"), h.Value("unpublish_selected"), h.Class("btn btn-neutral btn-sm"), g.Attr("data-requires-selection", "true"), h.Disabled(), g.Text("Unpublish selected")),
				batchConfirmButton("delete_selected", "btn btn-destructive btn-sm", "Delete selected", true),
				h.Span(h.Class("batch-divider"), g.Attr("aria-hidden", "true")),
				batchConfirmButton("unpublish_all", "btn btn-neutral btn-sm", "Unpublish all", false),
				batchConfirmButton("delete_all", "btn btn-destructive btn-sm", "Delete all", false),
			),
			h.Div(h.ID(batchConfirmSlot), h.Class("confirm-slot")),
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
		paginationNav(view),
	})
}

// selectAllLabel says what "Select all" covers once the catalog has pages.
func selectAllLabel(p paging.Page) string {
	if p.IsPaged() {
		return "Select all on this page"
	}
	return "Select all"
}

// catalogSwap makes the links and forms inside an element (the filter bar,
// the pager) replace only the catalog, in place, rather than load a new page
// (HIG §6). The response is the same /journals page; htmx takes its
// #journals-catalog-stage and swaps it for the current one in a single
// frame, and puts the URL in the address bar (Back and shared links work as
// before). The header, the search box (focus and cursor, by its id) and the
// scroll position stay where they are; the current results stay visible
// until the new ones replace them (journals.css dims them only if the
// response is slow). show is where to scroll after the swap: "none" for
// search and tags, the top of the results for paging. Without JavaScript
// the links and form load the page as before.
func catalogSwap(show string) g.Node {
	return g.Group([]g.Node{
		g.Attr("hx-boost", "true"),
		g.Attr("hx-target", "#journals-catalog-stage"),
		g.Attr("hx-select", "#journals-catalog-stage"),
		g.Attr("hx-swap", "outerHTML show:"+show),
		g.Attr("hx-indicator", "#journals-catalog-stage"),
	})
}

// paginationNav renders Newer / Page n of m / Older below the feed, only when
// there is more than one page. Entries are newest first, so the previous page
// is "Newer".
func paginationNav(view CatalogView) g.Node {
	p := view.Page
	if !p.IsPaged() {
		return nil
	}
	link := func(enabled bool, n int, rel, label string) g.Node {
		if !enabled {
			return h.Span(h.Class("btn btn-neutral btn-sm journals-page-link is-disabled"), g.Attr("aria-disabled", "true"), g.Text(label))
		}
		return h.A(h.Href(view.PageURL(n)), h.Rel(rel), h.Class("btn btn-neutral btn-sm journals-page-link"), g.Text(label))
	}
	return h.Nav(h.Class("journals-pagination"), g.Attr("aria-label", "Journal pages"),
		catalogSwap("#journals-catalog-stage:top"),
		link(p.HasPrev(), p.Number-1, "prev", "← Newer"),
		h.Span(h.Class("journals-page-status"), g.Attr("aria-current", "page"),
			g.Textf("Page %d of %d", p.Number, p.Count)),
		link(p.HasNext(), p.Number+1, "next", "Older →"),
	)
}

func renderJournalsHeader(isAuth bool) g.Node {
	// Signed out, a reader is offered only "Author sign in": never an action
	// they can't take.
	var newEntry, authLink g.Node
	if isAuth {
		newEntry = h.A(h.Class("new-entry-link btn btn-neutral"), h.Href("/journals/new"), g.Text("Add new entry"))
		authLink = h.Form(h.Class("auth-action-form"), h.Method("POST"), h.Action("/logout"),
			h.Button(h.Type("submit"), h.Class("auth-action-link link-quiet"), g.Text("Sign out")),
		)
	} else {
		authLink = h.A(h.Class("auth-action-link link-quiet"), h.Href("/login?next=/journals"), g.Text("Author sign in"))
	}

	return h.Header(h.Class("journals-header"),
		layout.BackLink("/", "Back"),
		h.Div(h.Class("journals-title-row"),
			h.H1(g.Text("Journals")),
			h.Div(h.Class("journals-actions"),
				newEntry,
				authLink,
			),
		),
	)
}

// CatalogPage renders /journals with its catalog in the same document: the
// header, then the filter bar and this page of cards (CatalogContentFragment).
func CatalogPage(isAuth bool, view CatalogView) g.Node {
	var catalogScript g.Node
	if isAuth {
		catalogScript = h.Script(h.Src(assets.JournalsCatalogJS.URL()))
	}

	return layout.Doc(
		"Journals - RES IMPERII",
		Style.URL(),
		h.Main(h.Class("journals-page"),
			renderJournalsHeader(isAuth),
			h.Div(h.ID("journals-catalog-stage"), CatalogContentFragment(isAuth, view)),
		),
		h.Script(h.Src(assets.JournalMediaJS.URL())),
		catalogScript,
	)
}

// EntryPage renders the single-entry permalink reading view (/journals/{id}).
func EntryPage(isAuth bool, item RenderedEntry) g.Node {
	return layout.Doc(
		"Journal Entry - RES IMPERII",
		Style.URL(),
		h.Main(h.Class("journals-page journals-single-page"),
			h.Header(h.Class("journals-header"),
				layout.BackLink("/journals", "Journals"),
			),
			h.Div(h.Class("journals-feed journals-single"),
				EntryCard(isAuth, item, false),
			),
		),
		h.Script(h.Src(assets.JournalMediaJS.URL())),
	)
}

// mediaSlot holds the entry's media. A tweet with a snapshot is drawn here by
// the server (views/tweet) and marked mounted, so journal-media.js leaves it;
// anything else is filled in the browser from data-media-url.
func mediaSlot(item RenderedEntry) g.Node {
	if item.Passage != nil {
		return h.Div(h.Class("journal-media-slot journal-preview-media"),
			g.Attr("data-media-url", item.Entry.MediaURL), g.Attr("data-media-mounted", item.Entry.MediaURL),
			passageview.Box(*item.Passage),
		)
	}
	if item.Tweet != nil {
		return h.Div(h.Class("journal-media-slot journal-preview-media"),
			g.Attr("data-media-url", item.Entry.MediaURL), g.Attr("data-media-mounted", item.Entry.MediaURL),
			tweetview.Box(*item.Tweet),
		)
	}
	// Everything else is built in the browser (journal-media.js); its slot is
	// given the right space now, so nothing below moves when it arrives (HIG
	// §1.7). A plain link needs no script: its card is drawn here.
	switch kind := classifyMedia(item.Entry.MediaURL); kind {
	case mediaLink:
		return h.Div(h.Class("journal-media-slot journal-preview-media is-link"),
			g.Attr("data-media-url", item.Entry.MediaURL), g.Attr("data-media-mounted", item.Entry.MediaURL),
			linkCard(item.Entry.MediaURL),
		)
	default:
		return h.Div(h.Class("journal-media-slot journal-preview-media is-"+string(kind)), g.Attr("data-media-url", item.Entry.MediaURL))
	}
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
		dateNode = h.A(h.Class("entry-date entry-card-link link-quiet"), h.Href(EntryURL(item.Entry.ID)), g.Text(item.FormattedDate))
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
			confirmTrigger(ActionURL(item.Entry.ID, "delete"), confirmSlotID(item.Entry.ID), "Delete"),
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
		mediaSlot(item),
		tagsBlock,
		contentBlock,
		adminFooter,
		g.If(isAuth, h.Div(h.ID(confirmSlotID(item.Entry.ID)), h.Class("confirm-slot"))),
	)
}

// batchConfirmSlot is where a batch action's prompt appears.
const batchConfirmSlot = "batch-confirm"

// batchConfirmButton asks before a batch action: it fetches the prompt (with
// the selected ids) into the batch bar; the prompt's button submits the form.
func batchConfirmButton(action, cls, label string, needsSelection bool) g.Node {
	return h.Button(h.Type("button"), h.Class(cls),
		g.Attr("hx-get", "/journals/batch/confirm"),
		g.Attr("hx-vals", fmt.Sprintf(`{"action":%q}`, action)),
		g.Attr("hx-include", ".entry-select-cb:checked"),
		g.Attr("hx-target", "#"+batchConfirmSlot),
		g.Attr("hx-swap", "innerHTML"),
		g.Attr("aria-controls", batchConfirmSlot),
		g.If(needsSelection, g.Group([]g.Node{g.Attr("data-requires-selection", "true"), h.Disabled()})),
		g.Text(label),
	)
}

// confirmSlotID is where an entry's delete prompt appears (views/confirm).
func confirmSlotID(id int64) string { return fmt.Sprintf("confirm-entry-%d", id) }

// confirmTrigger is a destructive control that asks first: with HTMX it
// fetches the prompt into slotID; without JavaScript the same URL is a page.
func confirmTrigger(href, slotID, label string) g.Node {
	return h.A(h.Class("btn btn-destructive btn-sm"), h.Href(href),
		g.Attr("hx-get", href), g.Attr("hx-target", "#"+slotID), g.Attr("hx-swap", "innerHTML"),
		g.Attr("aria-controls", slotID),
		g.Text(label),
	)
}
