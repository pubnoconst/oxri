// Package landing provides the Gomponents view for the Res Imperii landing page.
package landing

import (
	_ "embed"
	"github.com/oxri/oxri-go/assets"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed landing.css
var StyleCSS string

// StylePath is the public CSS route for the landing component styles.
const StylePath = "/views/landing/landing.css"

// Style is the content-hashed handle for StyleCSS (URL and route handler).
var Style = assets.Component(StylePath, StyleCSS)

// Page renders the minimal landing page view inside the base layout shell.
func Page(isAuth bool) g.Node {
	authNodes := []g.Node{}
	if isAuth {
		authNodes = append(authNodes,
			h.A(h.Class("landing-auth link-quiet"), h.Href("/settings"), g.Text("Settings")),
			h.Form(h.Class("landing-auth-form"), h.Method("POST"), h.Action("/logout"),
				h.Button(h.Type("submit"), h.Class("landing-auth link-quiet"), g.Text("Log out")),
			),
		)
	} else {
		authNodes = append(authNodes,
			h.A(h.Class("landing-auth link-quiet"), h.Href("/login"), g.Text("Log in")),
		)
	}

	return layout.Doc("RES IMPERII", Style.URL(),
		h.Main(h.Class("landing"),
			h.Nav(h.Class("landing-nav"),
				h.H1(h.Class("landing-title"), g.Text("RES·IMPERII")),
				h.Ul(h.Class("landing-books"),
					h.Li(h.A(h.Class("link-plain"), h.Href("/book-i"), g.Text("Book I"))),
					h.Li(h.A(h.Class("link-plain"), h.Href("/book-ii"), g.Text("Book II"))),
					h.Li(h.A(h.Class("link-plain"), h.Href("/book-iii"), g.Text("Book III"))),
				),
				h.Hr(h.Class("landing-rule")),
				h.A(h.Class("landing-journals link-plain"), h.Href("/journals"), g.Text("Journals")),
				h.Div(h.Class("landing-auth-wrapper"),
					g.Group(authNodes),
				),
			),
		),
	)
}
