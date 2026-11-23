package journals

import (
	"fmt"
	"github.com/oxri/oxri-go/assets"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/views/layout"
	"github.com/oxri/oxri-go/views/loading"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// VacateSpaceView holds all state required to render /journals/vacate-space/.
type VacateSpaceView struct {
	Entries           []RenderedEntry
	RequiredBytes     int64
	PendingEntryBytes int64
	UsedBytes         int64
	LimitBytes        int64
	AutoPlan          db.AutoVacatePlan
	PendingSub        *db.PendingSubmission
	AutoResubmitReady bool
}

// HasPendingSubmission returns true when a stashed journal form is ready to be resubmitted.
func (v VacateSpaceView) HasPendingSubmission() bool {
	return v.PendingSub != nil
}

// ResubmitActionURL returns the form action URL (/journals or /journals/{id}) for resubmitting the pending entry.
func (v VacateSpaceView) ResubmitActionURL() string {
	if v.PendingSub != nil && v.PendingSub.IsEdit && v.PendingSub.EntryID > 0 {
		return fmt.Sprintf("/journals/%d", v.PendingSub.EntryID)
	}
	return "/journals"
}

// InitialProgressPercent returns 100 when RequiredBytes <= 0, or 0 initially before manual selection.
func (v VacateSpaceView) InitialProgressPercent() int {
	if v.RequiredBytes <= 0 {
		return 100
	}
	return 0
}

// FormatBytes formats a byte count into a concise human-readable unit (B, KB, MB, GB).
func FormatBytes(b int64) string {
	if b < 0 {
		b = 0
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func progressMax(req int64) int64 {
	if req <= 0 {
		return 1
	}
	return req
}

func progressInitialVal(req int64) int64 {
	if req <= 0 {
		return 1
	}
	return 0
}

// VacateSpacePage renders the /journals/vacate-space/ view with progress tracking,
// prioritized auto-vacate, manual entry eviction, and awaiting-confirmation overlay.
func VacateSpacePage(view VacateSpaceView) g.Node {
	readoutText := fmt.Sprintf("0 B of %s (0%%)", FormatBytes(view.RequiredBytes))
	if view.RequiredBytes <= 0 {
		readoutText = "Enough space freed (100%)"
	}

	fillClass := "vacate-progress-fill"
	if view.RequiredBytes <= 0 {
		fillClass += " is-complete"
	}

	var resubmitForm g.Node
	if view.HasPendingSubmission() {
		var pubHidden, bmHidden g.Node
		if view.PendingSub.Published {
			pubHidden = h.Input(h.Type("hidden"), h.Name("published"), h.Value("1"))
		}
		if view.PendingSub.Bookmarked {
			bmHidden = h.Input(h.Type("hidden"), h.Name("bookmarked"), h.Value("1"))
		}

		hintText := "Your new entry is ready and will be saved automatically once you make room."
		if view.RequiredBytes <= 0 {
			hintText = "Enough space is available — saving your entry now…"
		}

		btnAttrs := []g.Node{
			h.Type("submit"),
			h.ID("resubmit-entry-btn"),
			h.Class("btn btn-encouraged btn-sm"),
		}
		if view.RequiredBytes > 0 {
			btnAttrs = append(btnAttrs, h.Disabled())
		}
		btnAttrs = append(btnAttrs, g.Text("Save entry now"))

		resubmitForm = h.Form(
			h.ID("resubmit-journal-form"),
			h.Class("vacate-resubmit-form"),
			h.Method("POST"),
			h.Action(view.ResubmitActionURL()),
			g.Attr("data-confirm-loading", "true"),
			g.Attr("data-auto-resubmit", fmt.Sprintf("%t", view.AutoResubmitReady)),
			h.Input(h.Type("hidden"), h.Name("media_url"), h.Value(view.PendingSub.MediaURL)),
			h.Input(h.Type("hidden"), h.Name("tags"), h.Value(view.PendingSub.TagsRaw)),
			h.Input(h.Type("hidden"), h.Name("body"), h.Value(view.PendingSub.Body)),
			pubHidden,
			bmHidden,
			h.Div(h.Class("vacate-resubmit-bar"),
				h.Span(h.Class("vacate-resubmit-hint"), g.Text(hintText)),
				h.Button(btnAttrs...),
			),
		)
	}

	var autoPlanNode g.Node
	if len(view.AutoPlan.CandidateIDs) > 0 {
		autoPlanNode = h.P(h.Class("vacate-auto-plan"),
			g.Text("This will remove "),
			h.Strong(g.Text(fmt.Sprintf("%d unpublished", view.AutoPlan.UnpublishedCount))),
			g.Text(" and "),
			h.Strong(g.Text(fmt.Sprintf("%d un-bookmarked", view.AutoPlan.UnbookmarkedCount))),
			g.Text(" entries, freeing "),
			h.Strong(g.Text(FormatBytes(view.AutoPlan.TotalFreedBytes))),
			g.Text("."),
		)
	} else {
		autoPlanNode = h.P(h.Class("vacate-auto-plan"),
			g.Text("No unpublished or un-bookmarked entries are available to remove automatically."),
		)
	}

	autoBtnAttrs := []g.Node{
		h.Type("submit"),
		h.ID("auto-vacate-btn"),
		h.Class("btn btn-destructive"),
	}
	if len(view.AutoPlan.CandidateIDs) == 0 {
		autoBtnAttrs = append(autoBtnAttrs, h.Disabled())
	}
	autoBtnAttrs = append(autoBtnAttrs, g.Text("Clear space automatically"))

	manualBtnAttrs := []g.Node{
		h.Type("submit"),
		h.ID("vacate-selected-btn"),
		h.Class("btn btn-destructive"),
	}
	if view.RequiredBytes > 0 {
		manualBtnAttrs = append(manualBtnAttrs, h.Disabled())
	}
	manualBtnAttrs = append(manualBtnAttrs, g.Text("Remove selected & save"))

	var manualEntriesNode g.Node
	if len(view.Entries) == 0 {
		manualEntriesNode = h.P(h.Class("journals-empty"), g.Text("No existing journals found."))
	} else {
		rows := make([]g.Node, 0, len(view.Entries))
		for _, item := range view.Entries {
			rowCls := "vacate-entry-row"
			if !item.Entry.Published {
				rowCls += " is-unpublished"
			}

			var pubBadge g.Node
			if !item.Entry.Published {
				pubBadge = h.Span(h.Class("entry-badge is-draft"), g.Text("Unpublished"))
			} else {
				pubBadge = h.Span(h.Class("entry-badge"), g.Text("Published"))
			}

			var bmBadge g.Node
			if item.Entry.Bookmarked {
				bmBadge = h.Span(h.Class("entry-badge is-bookmarked"), g.Text("★ Bookmarked"))
			} else {
				bmBadge = h.Span(h.Class("entry-badge"), g.Text("Un-bookmarked"))
			}

			tagChips := make([]g.Node, 0, len(item.Tags))
			for _, tag := range item.Tags {
				tagChips = append(tagChips, h.Span(h.Class("journal-tag-chip"), g.Text(tag)))
			}

			rows = append(rows, h.Label(h.Class(rowCls),
				h.Input(
					h.Type("checkbox"),
					h.Class("vacate-entry-cb"),
					h.Name("ids"),
					h.Value(fmt.Sprintf("%d", item.Entry.ID)),
					g.Attr("data-bytes", fmt.Sprintf("%d", item.EstimatedBytes)),
					g.Attr("data-published", fmt.Sprintf("%t", item.Entry.Published)),
					g.Attr("data-bookmarked", fmt.Sprintf("%t", item.Entry.Bookmarked)),
				),
				h.Div(h.Class("vacate-entry-main"),
					h.Div(h.Class("vacate-entry-top"),
						h.Span(h.Class("entry-date"), g.Text(item.FormattedDate)),
						h.Div(h.Class("journal-meta-badges"),
							pubBadge,
							bmBadge,
							g.Group(tagChips),
						),
					),
					h.P(h.Class("vacate-entry-excerpt"), g.Text(item.Excerpt)),
				),
				h.Div(h.Class("vacate-entry-size-col"),
					h.Span(
						h.Class("vacate-entry-size"),
						g.Attr("data-bytes", fmt.Sprintf("%d", item.EstimatedBytes)),
						g.Text(item.FormattedSize),
					),
				),
			))
		}
		manualEntriesNode = h.Div(h.Class("vacate-entries-list"), g.Group(rows))
	}

	return layout.Doc(
		"Make Room - RES IMPERII",
		Style.URL(),
		h.Main(h.Class("journals-page vacate-space-page"),
			h.Header(h.Class("journals-header"),
				layout.BackLink("/journals", "Journals"),
				h.Div(h.Class("journals-title-row"),
					h.H1(g.Text("Make Room")),
				),
			),
			// 1. Summary & Progress Card
			h.Section(
				h.ID("vacate-progress-section"),
				h.Class("vacate-card vacate-summary-card"),
				g.Attr("aria-label", "Space needed"),
				g.Attr("data-required-bytes", fmt.Sprintf("%d", view.RequiredBytes)),
				g.Attr("data-entry-bytes", fmt.Sprintf("%d", view.PendingEntryBytes)),
				h.P(h.Class("vacate-lead"),
					g.Text("Your journal archive is full. Remove a few older entries to free up at least "),
					h.Strong(g.Text(FormatBytes(view.RequiredBytes))),
					g.Text(", and your new entry will be saved automatically."),
				),
				h.Div(h.Class("vacate-metrics"),
					h.Div(h.Class("vacate-metric-item"),
						h.Span(h.Class("vacate-metric-label"), g.Text("This entry")),
						h.Span(h.Class("vacate-metric-value"), h.ID("vacate-entry-bytes"), g.Text(FormatBytes(view.PendingEntryBytes))),
					),
					h.Div(h.Class("vacate-metric-item"),
						h.Span(h.Class("vacate-metric-label"), g.Text("Space needed")),
						h.Span(h.Class("vacate-metric-value"), h.ID("vacate-required-bytes"), g.Text(FormatBytes(view.RequiredBytes))),
					),
					h.Div(h.Class("vacate-metric-item"),
						h.Span(h.Class("vacate-metric-label"), g.Text("Archive used")),
						h.Span(h.Class("vacate-metric-value"), g.Text(fmt.Sprintf("%s of %s", FormatBytes(view.UsedBytes), FormatBytes(view.LimitBytes)))),
					),
				),
				h.Div(h.Class("vacate-progress-wrapper"),
					h.Div(h.Class("vacate-progress-header"),
						h.Span(h.Class("vacate-progress-title"), g.Text("Space selected")),
						h.Span(h.ID("vacate-progress-readout"), h.Class("vacate-progress-readout"), g.Text(readoutText)),
					),
					h.Progress(
						h.ID("vacate-progress-bar"),
						h.Class("vacate-progress-native"),
						h.Max(fmt.Sprintf("%d", progressMax(view.RequiredBytes))),
						h.Value(fmt.Sprintf("%d", progressInitialVal(view.RequiredBytes))),
					),
					h.Div(
						h.Class("vacate-progress-track"),
						g.Attr("role", "progressbar"),
						g.Attr("aria-valuemin", "0"),
						g.Attr("aria-valuemax", fmt.Sprintf("%d", progressMax(view.RequiredBytes))),
						g.Attr("aria-valuenow", fmt.Sprintf("%d", progressInitialVal(view.RequiredBytes))),
						h.Div(
							h.ID("vacate-progress-fill"),
							h.Class(fillClass),
							h.Style(fmt.Sprintf("width: %d%%;", view.InitialProgressPercent())),
						),
					),
				),
				resubmitForm,
			),
			// 2. Make Room Automatically Card
			h.Section(h.Class("vacate-card vacate-auto-card"), g.Attr("aria-label", "Make room automatically"),
				h.Div(h.Class("vacate-auto-info"),
					h.H2(g.Text("Make Room Automatically")),
					h.P(h.Class("vacate-auto-desc"),
						g.Text("Remove older "),
						h.Strong(g.Text("unpublished")),
						g.Text(" drafts first, and then older entries you haven't "),
						h.Strong(g.Text("bookmarked")),
						g.Text(", until there is enough room."),
					),
					autoPlanNode,
				),
				h.Form(h.Method("POST"), h.Action("/journals/vacate-space/"), h.Class("vacate-auto-form"), g.Attr("data-confirm-loading", "true"),
					h.Input(h.Type("hidden"), h.Name("action"), h.Value("auto_vacate")),
					h.Input(h.Type("hidden"), h.Name("required_bytes"), h.Value(fmt.Sprintf("%d", view.RequiredBytes))),
					h.Button(autoBtnAttrs...),
				),
			),
			// 3. Choose Entries to Remove Card
			h.Section(h.Class("vacate-card vacate-manual-card"), g.Attr("aria-label", "Choose entries to remove"),
				h.Form(h.ID("vacate-manual-form"), h.Method("POST"), h.Action("/journals/vacate-space/"), g.Attr("data-confirm-loading", "true"),
					h.Input(h.Type("hidden"), h.Name("action"), h.Value("delete_selected")),
					h.Div(h.Class("vacate-manual-header"),
						h.Div(
							h.H2(g.Text("Choose Entries to Remove")),
							h.P(h.Class("vacate-manual-subtitle"), g.Text("Select entries below until the bar reaches 100%.")),
						),
						h.Button(manualBtnAttrs...),
					),
					manualEntriesNode,
				),
			),
		),
		loading.Overlay(loading.DefaultPhrase, view.AutoResubmitReady),
		h.Script(h.Src(assets.WaterRippleJS.URL())),
		h.Script(h.Src(assets.LoadingTransitionJS.URL())),
		h.Script(h.Src(assets.VacateSpaceJS.URL())),
	)
}
