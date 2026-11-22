// Package layout provides the base HTML document frame for Res Imperii.
package layout

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Doc renders the full HTML document shell with typography, design tokens, and scripts.
func Doc(title string, componentCSS string, bodyChildren ...g.Node) g.Node {
	headElements := []g.Node{
		h.Meta(h.Charset("utf-8")),
		h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
		h.TitleEl(g.Text(title)),
		h.Link(h.Rel("icon"), h.Type("image/png"), h.Href("/assets/favicon.png?v=1")),

		// EB Garamond Typeface
		h.Link(h.Rel("preconnect"), h.Href("https://fonts.googleapis.com")),
		h.Link(h.Rel("preconnect"), h.Href("https://fonts.gstatic.com"), g.Attr("crossorigin", "")),
		h.Link(h.Rel("stylesheet"), h.Href("https://fonts.googleapis.com/css2?family=EB+Garamond:wght@400;500;600;700&display=swap")),

		// Global Site CSS
		h.Link(h.Rel("stylesheet"), h.Href("/assets/css/site.css?v=7")),
	}

	if componentCSS != "" {
		headElements = append(headElements, h.Link(h.Rel("stylesheet"), h.Href(componentCSS)))
	}

	return h.Doctype(
		h.HTML(h.Lang("en"),
			h.Head(headElements...),
			h.Body(
				g.Group(bodyChildren),
				// Vendored HTMX, Theme Toggle, and Page Transition Feedback (No External CDN scripts)
				h.Script(h.Src("/assets/js/htmx.min.js")),
				h.Script(h.Src("/assets/js/theme-toggle.js?v=2")),
				h.Script(h.Src("/assets/js/navigation-feedback.js?v=1")),
			),
		),
	)
}

// BackLink renders a quiet navigation return link with a pure-CSS circular chevron.
func BackLink(href, label string) g.Node {
	return h.A(h.Class("back-link"), h.Href(href),
		h.Span(h.Class("back-icon"), g.Attr("aria-hidden", "true")),
		h.Span(h.Class("back-label"), g.Text(label)),
	)
}
