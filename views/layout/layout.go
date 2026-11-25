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
		// The reader's remembered light or dark choice, applied before the
		// first paint so no page flashes the other theme (assets.ThemeInitJS;
		// the only inline script, allowed by its hash in the CSP).
		h.Script(g.Raw(assets.ThemeInitJS)),
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

		// Typefaces, self-hosted (assets/fonts.go), each fetched when the page
		// has text that needs it. Not preloaded: see assets/fonts.go.
		h.Link(h.Rel("stylesheet"), h.Href(assets.FontsCSS.URL())),

		h.Link(h.Rel("stylesheet"), h.Href(assets.SiteCSS.URL())),

		// The dot-matrix loader (ResImperiiDots). Deferred: fetched at once but
		// run after the page is parsed, so it no longer holds back the first
		// paint (PROFILING.md). Deferred scripts run before DOMContentLoaded,
		// and every caller uses ResImperiiDots at DOMContentLoaded or later (a
		// click, a slot coming into view) or checks for it first.
		h.Script(h.Src(assets.DotMatrixJS.URL()), h.Defer()),
	}

	if componentCSS != "" {
		headElements = append(headElements, h.Link(h.Rel("stylesheet"), h.Href(componentCSS)))
	}

	return h.Doctype(
		h.HTML(h.Lang("en"),
			h.Head(headElements...),
			h.Body(
				ThemeToggle(),
				g.Group(bodyChildren),
				// Vendored HTMX, Theme Toggle, and Page Transition Feedback (No External CDN scripts)
				h.Script(h.Src(assets.HTMXJS.URL())),
				h.Script(h.Src(assets.ThemeToggleJS.URL())),
				h.Script(h.Src(assets.NavigationFeedbackJS.URL())),
				h.Script(h.Src(assets.OfflineJS.URL())),
				// Field performance: Core Web Vitals from this view, sent as it
				// is left (vitals.js, PROFILING.md).
				h.Script(h.Src(assets.WebVitalsJS.URL())),
				h.Script(h.Src(assets.VitalsJS.URL())),
			),
		),
	)
}

// ThemeToggle is the light/dark switch at the top right of every page
// (theme-toggle.js). It shows the moon in light mode and the sun in dark
// mode: the theme a tap switches to. The first visit follows the device;
// a tap is remembered (localStorage) and applied before the first paint on
// every later page (assets.ThemeInitJS).
func ThemeToggle() g.Node {
	svg := func(class string, paths ...g.Node) g.Node {
		return g.El("svg", g.Attr("class", "theme-icon "+class), g.Attr("viewBox", "0 0 24 24"), g.Attr("width", "20"), g.Attr("height", "20"),
			g.Attr("fill", "none"), g.Attr("stroke", "currentColor"), g.Attr("stroke-width", "1.6"), g.Attr("stroke-linecap", "round"),
			g.Attr("stroke-linejoin", "round"), g.Attr("aria-hidden", "true"), g.Attr("focusable", "false"), g.Group(paths))
	}
	path := func(d string) g.Node { return g.El("path", g.Attr("d", d)) }
	return h.Button(h.Type("button"), h.Class("theme-toggle"), g.Attr("data-theme-toggle", ""),
		g.Attr("aria-label", "Switch between light and dark"), h.Title("Switch between light and dark"),
		svg("theme-icon-moon", path("M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5Z")),
		svg("theme-icon-sun",
			g.El("circle", g.Attr("cx", "12"), g.Attr("cy", "12"), g.Attr("r", "4")),
			path("M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4"),
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
