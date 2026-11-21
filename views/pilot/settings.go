// Package pilot provides experimental administrative views and user profile settings.
package pilot

import (
	_ "embed"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed settings.css
var StyleCSS string

// StylePath is the public route for the pilot settings component CSS.
const StylePath = "/views/pilot/settings.css"

// SettingsPage renders the administrative security and profile settings view.
func SettingsPage(keyType string) g.Node {
	return layout.Doc(
		"Settings - RES IMPERII",
		StylePath,
		h.Main(h.Class("settings-page"),
			h.Header(h.Class("settings-header"),
				layout.BackLink("/", "Home"),
				h.Div(h.Class("settings-heading"),
					h.P(h.Class("settings-eyebrow"), g.Text("PILOT / EXPERIMENTAL · SETTINGS")),
					h.H1(g.Text("Security & Authentication")),
					h.P(h.Class("settings-subtitle"), g.Text("Manage administrative credentials, hardware security keys, and access controls in the experimental pilot environment.")),
				),
			),
			h.Div(h.Class("settings-stack"),
				// Session Status Banner with Log out action
				h.Section(h.Class("settings-card status-card"),
					h.Div(h.Class("status-indicator"),
						h.Span(h.Class("status-dot")),
						h.Div(h.Class("status-content"),
							h.Span(h.Class("status-title"), g.Text("Administrative Session Active")),
							h.Span(h.Class("status-desc"), g.Text("Authenticated via Loopback SSH Gateway · Session is active and valid")),
						),
					),
					h.Form(h.Method("POST"), h.Action("/logout"),
						h.Button(h.Type("submit"), h.Class("btn btn-neutral btn-sm"), g.Text("Log out")),
					),
				),

				// Authentication Methods
				h.Section(h.Class("settings-card"),
					h.Div(h.Class("card-header"),
						h.H2(g.Text("Authentication Methods")),
						h.P(h.Class("card-desc"), g.Text("Configured credentials and keys used to sign in to Res Imperii.")),
					),
					h.Div(h.Class("methods-list"),
						// Method 1: Loopback SSH Key
						h.Div(h.Class("method-card"),
							h.Div(h.Class("method-info"),
								h.Div(h.Class("method-title-row"),
									h.H3(h.Class("method-name"), g.Text("Loopback SSH Key")),
									h.Span(h.Class("badge badge-active"), g.Text("Active")),
								),
								h.P(h.Class("method-description"), g.Text("Terminal-based cryptographic handshake via OpenSSH or local key agent.")),
								h.Div(h.Class("method-meta"),
									h.Span(h.Class("meta-label"), g.Text("Key type:")),
									h.Code(h.Class("meta-code"), g.Text(keyType)),
									h.Span(h.Class("meta-sep"), g.Text("·")),
									h.Span(h.Class("meta-label"), g.Text("Source:")),
									h.Span(h.Class("meta-val"), g.Text("Environment (ADMIN_SSH_PUBKEY)")),
								),
							),
							h.Div(h.Class("method-actions"),
								h.Button(h.Type("button"), h.Class("btn btn-destructive"), g.Attr("disabled", ""), g.Attr("aria-disabled", "true"), g.Attr("title", "Primary loopback key is configured via environment"), g.Text("Remove")),
							),
						),

						// Method 2: Passkeys / WebAuthn
						h.Div(h.Class("method-card"),
							h.Div(h.Class("method-info"),
								h.Div(h.Class("method-title-row"),
									h.H3(h.Class("method-name"), g.Text("Passkeys (FIDO2 / WebAuthn)")),
									h.Span(h.Class("badge badge-pending"), g.Text("Pending HTTPS")),
								),
								h.P(h.Class("method-description"), g.Text("Hardware security keys (YubiKey), platform biometrics (Windows Hello, Touch ID), and passkey managers.")),
								h.Div(h.Class("method-meta"),
									h.Span(h.Class("meta-label"), g.Text("Enrolled:")),
									h.Span(h.Class("meta-val"), g.Text("0 credentials")),
									h.Span(h.Class("meta-sep"), g.Text("·")),
									h.Span(h.Class("meta-label"), g.Text("Standard:")),
									h.Span(h.Class("meta-val"), g.Text("W3C WebAuthn Level 3")),
								),
							),
							h.Div(h.Class("method-actions"),
								h.Button(h.Type("button"), h.Class("btn btn-destructive"), g.Attr("disabled", ""), g.Attr("aria-disabled", "true"), g.Attr("title", "No registered passkey to remove"), g.Text("Remove")),
							),
						),
					),

					// Footer Controls & Notice
					h.Div(h.Class("methods-footer"),
						h.Div(h.Class("action-buttons"),
							h.Button(h.Type("button"), h.Class("btn btn-encouraged"), g.Attr("disabled", ""), g.Attr("aria-disabled", "true"), g.Attr("title", "Passkey registration requires HTTPS and a registered domain"), g.Text("Add Passkey")),
							h.Button(h.Type("button"), h.Class("btn btn-neutral"), g.Attr("disabled", ""), g.Attr("aria-disabled", "true"), g.Attr("title", "Requires HTTPS and a registered domain"), g.Text("Add SSH Key")),
						),
						h.Div(h.Class("https-notice"), g.Attr("role", "note"),
							h.Div(h.Class("notice-icon"), g.Text("ℹ")),
							h.Div(h.Class("notice-body"),
								h.Span(h.Class("notice-title"), g.Text("Passkey management is locked in development")),
								h.P(g.Text("Adding and removing FIDO2/WebAuthn passkeys requires a registered public domain origin (rp_id) and a secure HTTPS transport. These controls will be enabled once deployed to production.")),
							),
						),
					),
				),
			),
		),
	)
}
