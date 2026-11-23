// Package settings provides administrative security and credential management views.
package settings

import (
	_ "embed"
	"fmt"
	"github.com/oxri/oxri-go/assets"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed settings.css
var StyleCSS string

// StylePath is the public route for the settings component CSS.
const StylePath = "/views/settings/settings.css"

// Style is the content-hashed handle for StyleCSS (URL and route handler).
var Style = assets.Component(StylePath, StyleCSS)

// PasskeyItem is a display-ready enrolled Passkey for the current RP ID.
type PasskeyItem struct {
	ID         string
	Name       string
	CreatedAt  string
	LastUsedAt string
}

// SettingsData carries the live authentication and passkey state for /settings.
type SettingsData struct {
	MethodLabel      string
	RPID             string
	PasskeySupported bool
	FreshAuth        bool
	Passkeys         []PasskeyItem
}

// SettingsPage renders the administrative security and profile settings view.
func SettingsPage(data SettingsData) g.Node {
	if data.MethodLabel == "" {
		data.MethodLabel = "TOTP (RFC 6238)"
	}

	enrolledLabel := fmt.Sprintf("%d credentials", len(data.Passkeys))
	if len(data.Passkeys) == 1 {
		enrolledLabel = "1 credential"
	}

	passkeyBadge := h.Span(h.Class("badge badge-pending"), g.Text("Not Enrolled"))
	if len(data.Passkeys) > 0 {
		passkeyBadge = h.Span(h.Class("badge badge-active"), g.Text("Active"))
	} else if !data.PasskeySupported {
		passkeyBadge = h.Span(h.Class("badge badge-pending"), g.Text("Requires HTTPS / localhost"))
	}

	var passkeyList g.Node
	if len(data.Passkeys) > 0 {
		items := make([]g.Node, 0, len(data.Passkeys))
		for _, pk := range data.Passkeys {
			metaText := "Added " + pk.CreatedAt
			if pk.LastUsedAt != "" {
				metaText += " · Last used " + pk.LastUsedAt
			}
			var removeControl g.Node
			if data.PasskeySupported && !data.FreshAuth {
				removeControl = h.Button(h.Type("button"), h.Class("btn btn-destructive btn-sm"), g.Attr("disabled", ""), g.Attr("aria-disabled", "true"), g.Text("Remove"))
			} else {
				removeControl = h.Form(
					h.Method("POST"),
					h.Action("/settings/passkeys/"+pk.ID+"/delete"),
					g.Attr("data-confirm", "Remove this passkey?"),
					h.Button(h.Type("submit"), h.Class("btn btn-destructive btn-sm"), g.Text("Remove")),
				)
			}
			items = append(items, h.Li(h.Class("passkey-row"),
				h.Div(h.Class("passkey-row-info"),
					h.Span(h.Class("passkey-row-name"), g.Text(pk.Name)),
					h.Span(h.Class("passkey-row-meta"), g.Text(metaText)),
				),
				removeControl,
			))
		}
		passkeyList = h.Ul(h.Class("passkey-list"), g.Group(items))
	}

	var footerControls g.Node
	switch {
	case data.PasskeySupported && data.FreshAuth:
		footerControls = h.Div(h.Class("methods-footer"),
			h.Div(h.Class("passkey-add-row"),
				h.Input(
					h.ID("passkey-label-input"),
					h.Type("text"),
					h.Class("passkey-label-input"),
					h.Placeholder("Passkey label (e.g. Proton Pass, Laptop, Phone)"),
					g.Attr("maxlength", "32"),
					g.Attr("aria-label", "Passkey label"),
				),
				h.Button(h.Type("button"), h.ID("add-passkey-btn"), h.Class("btn btn-encouraged"), g.Text("Add passkey")),
			),
			h.P(h.ID("passkey-register-status"), h.Class("passkey-status"), g.Attr("role", "status"), g.Attr("hidden", "")),
		)
	case data.PasskeySupported && !data.FreshAuth:
		footerControls = h.Div(h.Class("methods-footer"),
			h.Div(h.Class("passkey-add-row"),
				h.Input(
					h.ID("passkey-label-input"),
					h.Type("text"),
					h.Class("passkey-label-input"),
					h.Placeholder("Passkey label (e.g. Proton Pass, Laptop, Phone)"),
					g.Attr("maxlength", "32"),
					g.Attr("disabled", ""),
					g.Attr("aria-label", "Passkey label"),
				),
				h.Button(h.Type("button"), h.Class("btn btn-encouraged"), g.Attr("disabled", ""), g.Attr("aria-disabled", "true"), g.Text("Add passkey")),
			),
			h.Div(h.Class("https-notice"), g.Attr("role", "note"),
				h.Div(h.Class("notice-icon"), g.Text("ℹ")),
				h.Div(h.Class("notice-body"),
					h.Span(h.Class("notice-title"), g.Text("Recent verification required to manage passkeys")),
					h.P(g.Text("For security, adding or removing passkeys requires signing in within the last 5 minutes.")),
					h.A(h.Class("btn btn-neutral btn-sm reauth-link"), h.Href("/login?next=/settings&reauth=1"), g.Text("Re-verify identity")),
				),
			),
		)
	default:
		footerControls = h.Div(h.Class("methods-footer"),
			h.Div(h.Class("action-buttons"),
				h.Button(h.Type("button"), h.Class("btn btn-encouraged"), g.Attr("disabled", ""), g.Attr("aria-disabled", "true"), g.Text("Add passkey")),
			),
			h.Div(h.Class("https-notice"), g.Attr("role", "note"),
				h.Div(h.Class("notice-icon"), g.Text("ℹ")),
				h.Div(h.Class("notice-body"),
					h.Span(h.Class("notice-title"), g.Text("Passkey registration requires HTTPS or localhost")),
					h.P(g.Text("Open this page over HTTPS (e.g. your Render URL) or http://localhost:8080 (not 127.0.0.1) to enroll FIDO2/WebAuthn passkeys.")),
				),
			),
		)
	}

	var rpMeta g.Node
	if data.RPID != "" {
		rpMeta = g.Group([]g.Node{
			h.Span(h.Class("meta-sep"), g.Text("·")),
			h.Span(h.Class("meta-label"), g.Text("Domain:")),
			h.Code(h.Class("meta-code"), g.Text(data.RPID)),
		})
	}

	return layout.Doc(
		"Settings - RES IMPERII",
		Style.URL(),
		h.Main(h.Class("settings-page"),
			h.Header(h.Class("settings-header"),
				layout.BackLink("/", "Home"),
				h.Div(h.Class("settings-heading"),
					h.P(h.Class("settings-eyebrow"), g.Text("SETTINGS")),
					h.H1(g.Text("Security & Authentication")),
					h.P(h.Class("settings-subtitle"), g.Text("Manage administrative credentials, passkeys, and access controls for Res Imperii.")),
				),
			),
			h.Div(h.Class("settings-stack"),
				// Session Status Banner with Log out action
				h.Section(h.Class("settings-card status-card"),
					h.Div(h.Class("status-indicator"),
						h.Span(h.Class("status-dot")),
						h.Div(h.Class("status-content"),
							h.Span(h.Class("status-title"), g.Text("Administrative Session Active")),
							h.Span(h.Class("status-desc"), g.Text("Authenticated session is active and valid")),
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
						h.P(h.Class("card-desc"), g.Text("Configured credentials and passkeys used to sign in to Res Imperii.")),
					),
					h.Div(h.Class("methods-list"),
						// Method 1: Passkeys / WebAuthn
						h.Div(h.Class("method-card method-card-stack"),
							h.Div(h.Class("method-info"),
								h.Div(h.Class("method-title-row"),
									h.H3(h.Class("method-name"), g.Text("Passkeys (FIDO2 / WebAuthn)")),
									passkeyBadge,
								),
								h.P(h.Class("method-description"), g.Text("Phishing-resistant sign-in via Proton Pass, Windows Hello, Touch ID, phone passkeys, or hardware security keys.")),
								h.Div(h.Class("method-meta"),
									h.Span(h.Class("meta-label"), g.Text("Enrolled:")),
									h.Span(h.Class("meta-val"), g.Text(enrolledLabel)),
									rpMeta,
								),
							),
							passkeyList,
						),

						// Method 2: Password + Authenticator (TOTP)
						h.Div(h.Class("method-card"),
							h.Div(h.Class("method-info"),
								h.Div(h.Class("method-title-row"),
									h.H3(h.Class("method-name"), g.Text("Authenticator (TOTP)")),
									h.Span(h.Class("badge badge-active"), g.Text("Active")),
								),
								h.P(h.Class("method-description"), g.Text("6-digit time-based one-time codes. Instant on success; short, growing waits after wrong attempts; each code works only once.")),
								h.Div(h.Class("method-meta"),
									h.Span(h.Class("meta-label"), g.Text("Method:")),
									h.Code(h.Class("meta-code"), g.Text(data.MethodLabel)),
								),
							),
						),
					),
					footerControls,
				),
			),
		),
		h.Script(h.Src(assets.PasskeyJS.URL())),
	)
}
