// Package layout provides the base HTML document frame for Res Imperii.
package layout

import (
	"github.com/oxri/oxri-go/assets"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Doc renders the full HTML document shell with typography, design tokens, and scripts.
func Doc(title string, componentCSS string, bodyChildren ...g.Node) g.Node {
	headElements := []g.Node{
		h.Meta(h.Charset("utf-8")),
		h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
		h.TitleEl(g.Text(title)),
		h.Link(h.Rel("icon"), h.Type("image/png"), h.Href(assets.Favicon.URL())),

		// Installable web app: manifest, home-screen icon (iOS), and the browser
		// chrome coloured like the page in each theme (theme-toggle.js keeps it
		// in step with a manual light/dark switch).
		h.Link(h.Rel("manifest"), h.Href("/manifest.webmanifest")),
		h.Link(h.Rel("apple-touch-icon"), h.Href(assets.AppleTouchIcon.URL())),
		h.Meta(h.Name("theme-color"), g.Attr("media", "(prefers-color-scheme: light)"), h.Content("#f5f4f0")),
		h.Meta(h.Name("theme-color"), g.Attr("media", "(prefers-color-scheme: dark)"), h.Content("#0f1113")),
		h.Meta(h.Name("apple-mobile-web-app-title"), h.Content("Res Imperii")),

		// EB Garamond Typeface
		h.Link(h.Rel("preconnect"), h.Href("https://fonts.googleapis.com")),
		h.Link(h.Rel("preconnect"), h.Href("https://fonts.gstatic.com"), g.Attr("crossorigin", "")),
		h.Link(h.Rel("stylesheet"), h.Href("https://fonts.googleapis.com/css2?family=EB+Garamond:wght@400;500;600;700&display=swap")),

		h.Link(h.Rel("stylesheet"), h.Href(assets.SiteCSS.URL())),

		// The dot-matrix loader (ResImperiiDots), in the head so every page script
		// at the end of <body> can rely on it. It only defines functions; starting
		// server-rendered matrices waits for DOMContentLoaded.
		h.Script(h.Src(assets.DotMatrixJS.URL())),
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
				h.Script(h.Src(assets.HTMXJS.URL())),
				h.Script(h.Src(assets.ThemeToggleJS.URL())),
				h.Script(h.Src(assets.NavigationFeedbackJS.URL())),
				h.Script(h.Src(assets.OfflineJS.URL())),
			),
		),
	)
}

// BackLink renders a quiet navigation return link with a pure-CSS circular chevron.
func BackLink(href, label string) g.Node {
	return h.A(h.Class("back-link link-quiet"), h.Href(href),
		h.Span(h.Class("back-icon"), g.Attr("aria-hidden", "true")),
		h.Span(h.Class("back-label"), g.Text(label)),
	)
}
