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

// TOTPConfirmPage renders the 30-second anti-bruteforce confirmation cooldown screen
// featuring the full-card sunlit water-drop ripple, a smooth progress bar, and "Confirming, wait.".
func TOTPConfirmPage(verifyID string, remainingMs, totalMs int64) g.Node {
	if totalMs <= 0 {
		totalMs = 30000
	}
	if remainingMs < 0 {
		remainingMs = 0
	}
	if remainingMs > totalMs {
		remainingMs = totalMs
	}
	startRatio := float64(totalMs-remainingMs) / float64(totalMs)
	barStyle := fmt.Sprintf("--cooldown-ms:%dms;--cooldown-start:%.4f;", remainingMs, startRatio)
	rippleStyle := "--ripple-x:50%;--ripple-y:46%;--ripple-diameter:150vmax;"

	return layout.Doc(
		"Confirming - RES IMPERII",
		StylePath+"?v=4",
		h.Div(h.Class("card-water-ripple is-pagewide"), g.Attr("style", rippleStyle), g.Attr("aria-hidden", "true"),
			h.Span(h.Class("card-water-drop")),
			h.Span(h.Class("card-water-wave wave-1")),
			h.Span(h.Class("card-water-wave wave-2")),
			h.Span(h.Class("card-water-wave wave-3")),
		),
		h.Main(h.Class("auth-page"),
			h.Header(h.Class("auth-header"),
				layout.BackLink("/login", "Back"),
				h.H1(g.Text("AUTHENTICATION")),
			),
			h.Section(h.Class("auth-card auth-confirm-card"), g.Attr("aria-live", "polite"),
				h.P(h.Class("auth-confirm-phrase"), g.Text("Confirming, wait.")),
				h.Div(h.Class("auth-cooldown-track"), g.Attr("role", "progressbar"), g.Attr("aria-label", "Authentication confirmation progress"),
					h.Div(h.Class("auth-cooldown-fill"), g.Attr("style", barStyle)),
				),
			),
			h.Div(
				h.ID("totp-confirm-poller"),
				g.Attr("hx-get", fmt.Sprintf("/auth/totp-status?id=%s", verifyID)),
				g.Attr("hx-trigger", "every 1s"),
				g.Attr("hx-swap", "none"),
			),
		),
	)
}

// TOTPPage renders the quiet 6-digit RFC 6238 TOTP login screen, with an optional
// collapsible loopback SSH fallback when SSH auth is also active locally.
func TOTPPage(nextURL, errMsg, sshToken, sshCommand, sshPort string) g.Node {
	var errNode g.Node
	if errMsg != "" {
		errNode = h.P(h.Class("auth-error"), g.Attr("role", "alert"), g.Text(errMsg))
	}

	var sshFallback g.Node
	if sshCommand != "" && sshToken != "" {
		copyScript := "navigator.clipboard.writeText(this.dataset.cmd).then(() => { this.innerText='Copied!'; setTimeout(() => { this.innerText='Copy Command'; }, 2000); });"
		sshFallback = g.Group([]g.Node{
			h.Details(h.Class("auth-alt-ssh"),
				h.Summary(h.Class("auth-alt-summary"), g.Text("Or verify via loopback SSH")),
				h.Div(h.Class("auth-alt-body"),
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
					h.Div(h.Class("auth-status-bar"),
						h.Span(h.Class("auth-pulse-dot")),
						h.Span(h.Class("auth-status-text"), g.Text(fmt.Sprintf("Listening on port %s · Waiting for SSH signature…", sshPort))),
					),
				),
			),
			h.Div(
				h.ID("auth-poller"),
				g.Attr("hx-get", fmt.Sprintf("/auth/status?token=%s&next=%s", sshToken, nextURL)),
				g.Attr("hx-trigger", "every 1s"),
				g.Attr("hx-swap", "none"),
			),
		})
	}

	return layout.Doc(
		"Authentication - RES IMPERII",
		StylePath+"?v=4",
		h.Main(h.Class("auth-page"),
			h.Header(h.Class("auth-header"),
				layout.BackLink("/", "Home"),
				h.H1(g.Text("AUTHENTICATION")),
				h.P(h.Class("auth-subtitle"), g.Text("Enter the 6-digit authenticator code for Res Imperii")),
			),
			h.Section(h.Class("auth-card"),
				errNode,
				h.Form(h.Class("auth-totp-form"), h.Method("POST"), h.Action("/login"),
					h.Input(h.Type("hidden"), h.Name("next"), h.Value(nextURL)),
					h.Label(h.For("totp-code"), h.Class("auth-totp-label"), g.Text("Authenticator Code")),
					h.Input(
						h.ID("totp-code"),
						h.Name("code"),
						h.Type("text"),
						h.Class("auth-totp-input"),
						g.Attr("inputmode", "numeric"),
						g.Attr("autocomplete", "one-time-code"),
						g.Attr("pattern", "[0-9]{6}"),
						g.Attr("maxlength", "6"),
						g.Attr("minlength", "6"),
						h.Placeholder("000000"),
						h.AutoFocus(),
						h.Required(),
					),
					h.Button(h.Type("submit"), h.Class("btn btn-neutral auth-totp-submit"), g.Text("Verify")),
				),
				sshFallback,
			),
		),
	)
}

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
