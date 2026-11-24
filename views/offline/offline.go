// Package offline renders the page a service worker shows when a navigation
// fails because the device has no connection.
//
// The page is self-contained (inline style, no scripts, no external CSS) on
// purpose: the service worker caches only this one document, so anything it
// linked to would itself be unreachable offline.
package offline

import (
	"bytes"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Path is where the page is served and what the service worker caches.
const Path = "/offline"

// style mirrors the site's palette (site.css --page / --page-ink) for both
// system themes. The font falls back to Georgia if EB Garamond isn't cached.
const style = `
:root{color-scheme:light dark;--page:#f5f4f0;--ink:#34332f}
@media (prefers-color-scheme:dark){:root{--page:#0f1113;--ink:#d6d6d3}}
html,body{margin:0;background:var(--page);color:var(--ink)}
body{display:grid;min-height:100vh;place-items:center;padding:1.5rem;box-sizing:border-box;
font-family:"EB Garamond",Georgia,"Times New Roman",serif;text-align:center}
main{max-width:26rem}
.mark{font-size:.75rem;letter-spacing:.14em;text-transform:uppercase;opacity:.65;margin:0 0 2.5rem}
h1{font-weight:400;font-size:1.9rem;margin:0 0 .75rem}
p{font-size:1.1rem;line-height:1.5;margin:0 0 2rem}
a{color:inherit;text-underline-offset:.2em}
`

// Page is the offline document. "Try again" links to the current address
// (the service worker serves this page under the address that failed), so it
// simply retries it.
func Page() g.Node {
	return h.Doctype(
		h.HTML(h.Lang("en"),
			h.Head(
				h.Meta(h.Charset("utf-8")),
				h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
				h.Meta(h.Name("robots"), h.Content("noindex")),
				h.Meta(h.Name("theme-color"), g.Attr("media", "(prefers-color-scheme: light)"), h.Content("#f5f4f0")),
				h.Meta(h.Name("theme-color"), g.Attr("media", "(prefers-color-scheme: dark)"), h.Content("#0f1113")),
				h.TitleEl(g.Text("Offline - RES IMPERII")),
				h.StyleEl(g.Raw(style)),
			),
			h.Body(
				h.Main(
					h.P(h.Class("mark"), g.Text("RES IMPERII")),
					h.H1(g.Text("You're offline")),
					h.P(g.Text("Reconnect to the internet, then try again.")),
					h.A(h.Href(""), g.Text("Try again")),
				),
			),
		),
	)
}

// HTML is the rendered page, built once.
var HTML = func() []byte {
	var b bytes.Buffer
	if err := Page().Render(&b); err != nil {
		panic("offline: render: " + err.Error())
	}
	return b.Bytes()
}()
