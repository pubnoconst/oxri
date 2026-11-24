package editor

import (
	"strconv"
	"strings"

	"github.com/oxri/oxri-go/internal/verse"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// mediaField is the entry's media: a link (a video, image, post, …) or a
// passage of scripture, chosen from the first dropdown. Choosing Scripture,
// then Bible, shows the passage's fields: Book* and From* are required, To
// and Translation are not. scripture-picker.js shows and hides the groups
// and previews the passage; without it the server still accepts the fields.
func mediaField(data FormData) g.Node {
	s := data.Scripture
	isScripture := s.Kind == "scripture"
	return h.Div(h.Class("editor-field editor-media-field"),
		h.Div(h.Class("field-label-row"),
			h.Label(h.For("journal-media-kind"), g.Text("Media")),
			h.Span(h.ID("media-char-count"), h.Class("field-counter"), g.If(isScripture, h.Hidden("")), g.Text("0 / 128")),
		),
		h.Select(h.ID("journal-media-kind"), h.Name("media_kind"), h.Class("editor-select"),
			option("link", "Link", !isScripture),
			option("scripture", "Scripture", isScripture),
		),

		h.Div(h.ID("media-link-group"), h.Class("media-group"), g.If(isScripture, h.Hidden("")),
			h.Input(
				h.ID("journal-media-link"),
				h.Name("media_url"),
				h.Type("url"),
				// Not 128: a longer paste must arrive whole so the editor can
				// say it is too long, rather than being cut into a broken link.
				h.MaxLength("2048"),
				h.Placeholder("https://..."),
				h.AutoComplete("url"),
				g.Attr("aria-label", "Media link"),
				h.Value(data.MediaURL),
			),
			h.P(h.Class("field-hint"), g.Text("Required (1–128 chars). Video, audio, image, or X / Twitter post link.")),
			h.P(h.ID("media-error"), h.Class("field-error"), g.Attr("aria-live", "polite")),
		),

		h.Div(h.ID("media-scripture-group"), h.Class("media-group"), g.If(!isScripture, h.Hidden("")),
			h.Div(h.Class("field-label-row"),
				h.Label(h.For("journal-scripture"), g.Text("Scripture"), requiredMark()),
			),
			h.Select(h.ID("journal-scripture"), h.Name("scripture"), h.Class("editor-select"),
				option("", "Choose a scripture", s.Scripture == ""),
				option("bible", "Bible", s.Scripture == "bible"),
			),
			h.Div(h.ID("bible-fields"), h.Class("scripture-fields"), g.If(s.Scripture != "bible", h.Hidden("")),
				h.Div(h.Class("scripture-cell scripture-book"),
					h.Label(h.For("verse-book-search"), g.Text("Book"), requiredMark()),
					bookSearch(s.Book),
				),
				h.Div(h.Class("scripture-cell"),
					h.Label(h.For("verse-from"), g.Text("From"), requiredMark()),
					h.Input(h.ID("verse-from"), h.Name("verse_from"), h.Type("text"), h.Placeholder("12:3"),
						h.MaxLength("7"), h.AutoComplete("off"), g.Attr("aria-required", "true"), g.Attr("aria-describedby", "scripture-hint"),
						h.Value(s.From)),
				),
				h.Div(h.Class("scripture-cell"),
					h.Label(h.For("verse-to"), g.Text("To")),
					h.Input(h.ID("verse-to"), h.Name("verse_to"), h.Type("text"), h.Placeholder("4"),
						h.MaxLength("7"), h.AutoComplete("off"), g.Attr("aria-describedby", "scripture-hint"),
						h.Value(s.To)),
				),
				h.Div(h.Class("scripture-cell scripture-translation"),
					h.Label(h.For("verse-tr-choice"), g.Text("Translation")),
					h.Select(h.ID("verse-tr-choice"), h.Name("verse_tr_choice"), h.Class("editor-select"),
						translationOptions(s.TranslationChoice),
					),
				),
				h.P(h.ID("scripture-hint"), h.Class("field-hint scripture-hint"),
					g.Text("From is chapter:verse, like 12:3. To is the last verse, like 4, in the same chapter; leave it empty for one verse.")),
			),
			h.P(h.ID("scripture-error"), h.Class("field-error"), g.Attr("aria-live", "polite")),
		),
		// Which translation the preview used, and the text it showed: the
		// server checks the text against the provider on save.
		h.Input(h.Type("hidden"), h.ID("verse-tr"), h.Name("verse_tr"), h.Value(s.Translation)),
		h.Input(h.Type("hidden"), h.ID("verse-text"), h.Name("verse_text"), h.Value(s.Text)),
	)
}

// requiredMark is the star after a required field's label. The field itself
// carries aria-required, so the star is not read out.
func requiredMark() g.Node {
	return h.Span(h.Class("field-required"), g.Attr("aria-hidden", "true"), g.Text(" *"))
}

func option(value, label string, selected bool) g.Node {
	return h.Option(h.Value(value), g.If(selected, h.Selected()), g.Text(label))
}

// bookSearch is the Book field: a search box over the 66 books (a
// combobox; scripture-picker.js filters the list as the author types) and a
// hidden field with the chosen book's USFM id, which is what is posted.
func bookSearch(selected string) g.Node {
	name := ""
	items := make([]g.Node, 0, len(verse.Books))
	for _, b := range verse.Books {
		if b.ID == selected {
			name = b.Name
		}
		// tabindex -1: a click or tap moves focus into the list rather than
		// nowhere, so the search box knows the list is still in use.
		items = append(items, h.Li(h.ID("book-"+b.ID), g.Attr("role", "option"), h.TabIndex("-1"), g.Attr("aria-selected", strconv.FormatBool(b.ID == selected)),
			g.Attr("data-id", b.ID), g.Attr("data-chapters", strconv.Itoa(b.Chapters)), g.Text(b.Name)))
	}
	return h.Div(h.Class("book-search"),
		h.Input(h.ID("verse-book-search"), h.Type("text"), g.Attr("role", "combobox"),
			g.Attr("aria-autocomplete", "list"), g.Attr("aria-expanded", "false"), g.Attr("aria-controls", "verse-book-list"),
			g.Attr("aria-required", "true"), h.AutoComplete("off"), g.Attr("spellcheck", "false"),
			h.Placeholder("Search, like gen or 1 john"), h.Value(name)),
		h.Ul(h.ID("verse-book-list"), h.Class("book-list"), g.Attr("role", "listbox"), g.Attr("aria-label", "Books"), h.Hidden(""),
			g.Group(items)),
		h.Input(h.Type("hidden"), h.ID("verse-book"), h.Name("verse_book"), h.Value(selected)),
	)
}

// translationOptions lists "auto" first: the editor tries verse.Preferred in
// order (Young's Literal, then the King James) and uses the first that has
// the passage.
func translationOptions(selected string) g.Node {
	labels := make([]string, 0, len(verse.Preferred))
	for _, id := range verse.Preferred {
		if t, ok := verse.LookupTranslation(id); ok {
			labels = append(labels, t.Label)
		}
	}
	auto := strings.Join(labels, ", or ") + " where missing"
	nodes := []g.Node{h.Option(h.Value("auto"), g.Attr("data-order", strings.Join(verse.Preferred, " ")),
		g.If(selected == "" || selected == "auto", h.Selected()), g.Text(auto))}
	for _, t := range verse.Translations {
		nodes = append(nodes, h.Option(h.Value(t.ID), g.Attr("data-label", t.Label),
			g.If(t.ID == selected, h.Selected()), g.Text(t.Name)))
	}
	return g.Group(nodes)
}
