// Package login provides the Gomponents authentication view for Res Imperii.
package login

import (
	_ "embed"
	"fmt"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed login.css
var StyleCSS string

// StylePath is the public route for the login component CSS.
const StylePath = "/views/login/login.css"

// Page renders the cryptographic SSH authentication challenge screen.
func Page(token, sshCommand, nextURL, sshPort string) g.Node {
	copyScript := "navigator.clipboard.writeText(this.dataset.cmd).then(() => { this.innerText='Copied!'; setTimeout(() => { this.innerText='Copy Command'; }, 2000); });"

	return layout.Doc(
		"Authentication - RES IMPERII",
		StylePath,
		h.Main(h.Class("auth-page"),
			h.Header(h.Class("auth-header"),
				layout.BackLink("/", "Home"),
				h.H1(g.Text("AUTHENTICATION")),
				h.P(h.Class("auth-subtitle"), g.Text("Loopback SSH verification for Res Imperii")),
			),
			h.Section(h.Class("auth-card"),
				h.Ol(h.Class("auth-steps"),
					h.Li(
						h.Span(h.Class("step-num"), g.Text("1")),
						h.Div(h.Class("step-content"),
							h.P(h.Class("step-title"), g.Text("Open your terminal")),
							h.P(h.Class("step-desc"), g.Text("Use PowerShell, Windows Terminal, or your preferred shell where your SSH key is accessible.")),
						),
					),
					h.Li(
						h.Span(h.Class("step-num"), g.Text("2")),
						h.Div(h.Class("step-content"),
							h.P(h.Class("step-title"), g.Text("Run the authentication command")),
							h.Div(h.Class("auth-command-box"),
								h.Code(h.Class("auth-command"), g.Text(sshCommand)),
								h.Button(
									h.Type("button"),
									h.Class("auth-copy-button btn btn-neutral"),
									g.Attr("data-cmd", sshCommand),
									g.Attr("onclick", copyScript),
									g.Text("Copy Command"),
								),
							),
						),
					),
					h.Li(
						h.Span(h.Class("step-num"), g.Text("3")),
						h.Div(h.Class("step-content"),
							h.P(h.Class("step-title"), g.Text("Identity verification")),
							h.P(h.Class("step-desc"), g.Text("Your private key signs the loopback challenge. Upon verification, this page automatically unlocks and returns you to your destination.")),
						),
					),
				),
				h.Div(h.Class("auth-status-bar"),
					h.Span(h.Class("auth-pulse-dot")),
					h.Span(h.Class("auth-status-text"), g.Text(fmt.Sprintf("Listening on port %s · Waiting for SSH signature...", sshPort))),
				),
			),
			// HTMX Poller: checks /auth/status every 1s; receives HX-Redirect when approved
			h.Div(
				h.ID("auth-poller"),
				g.Attr("hx-get", fmt.Sprintf("/auth/status?token=%s&next=%s", token, nextURL)),
				g.Attr("hx-trigger", "every 1s"),
				g.Attr("hx-swap", "none"),
			),
		),
	)
}
