// Package login provides the Gomponents authentication views for Res Imperii.
package login

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed login.css
var StyleCSS string

// StylePath is the public route for the login component CSS.
const StylePath = "/views/login/login.css"

// TOTPConfirmPage renders the 30-second anti-bruteforce confirmation cooldown screen
// featuring the pagewide sunlit water-drop ripple, a smooth progress bar, and "Confirming, wait.".
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

	// Inline SVG refraction filter (strict-CSP safe). Three chained displacement maps, one per
	// expanding wavefront, bend the submerged content radially along each ripple. The map images,
	// positions and strengths are driven per-frame by /assets/js/water-ripple.js from the CSS rings.
	// color-interpolation-filters=sRGB is essential so mid-grey (128) means zero displacement.
	var filterSVG strings.Builder
	filterSVG.WriteString(`<svg class="water-refract-svg" aria-hidden="true" width="0" height="0" focusable="false"><defs>`)
	filterSVG.WriteString(`<filter id="water-surface-refract" filterUnits="userSpaceOnUse" primitiveUnits="userSpaceOnUse" x="-20" y="-20" width="4000" height="3000" color-interpolation-filters="sRGB">`)
	filterSVG.WriteString(`<feFlood flood-color="rgb(128,128,128)" flood-opacity="1" result="neutral"/>`)
	for i := 1; i <= 3; i++ {
		in := "SourceGraphic"
		if i > 1 {
			in = fmt.Sprintf("d%d", i-1)
		}
		filterSVG.WriteString(fmt.Sprintf(`<feImage id="wr-img-%d" preserveAspectRatio="none" x="-10" y="-10" width="1" height="1" result="m%d"/>`, i, i))
		filterSVG.WriteString(fmt.Sprintf(`<feMerge result="map%d"><feMergeNode in="neutral"/><feMergeNode in="m%d"/></feMerge>`, i, i))
		filterSVG.WriteString(fmt.Sprintf(`<feDisplacementMap id="wr-disp-%d" in="%s" in2="map%d" scale="0" xChannelSelector="R" yChannelSelector="G" result="d%d"/>`, i, in, i, i))
	}
	filterSVG.WriteString(`</filter></defs></svg>`)

	return layout.Doc(
		"Confirming - RES IMPERII",
		StylePath+"?v=7",
		g.Raw(filterSVG.String()),
		h.Div(h.Class("auth-water-stage"), g.Attr("data-water-stage", ""),
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
		),
		h.Div(h.Class("card-water-ripple is-pagewide"), g.Attr("style", rippleStyle), g.Attr("aria-hidden", "true"),
			h.Span(h.Class("card-water-drop")),
			h.Span(h.Class("card-water-wave wave-1")),
			h.Span(h.Class("card-water-wave wave-2")),
			h.Span(h.Class("card-water-wave wave-3")),
		),
		h.Script(h.Src("/assets/js/water-ripple.js?v=2")),
	)
}

// TOTPPage renders the quiet authentication screen: one-click Passkey sign-in when enrolled
// for this domain, alongside Argon2id password + 6-digit TOTP recovery/bootstrap login.
func TOTPPage(nextURL, errMsg string, requirePassword, hasPasskeys, reauth bool) g.Node {
	var errNode g.Node
	if errMsg != "" {
		errNode = h.P(h.Class("auth-error"), g.Attr("role", "alert"), g.Text(errMsg))
	}

	subtitle := "Enter the 6-digit authenticator code for Res Imperii"
	if requirePassword {
		subtitle = "Enter your password and 6-digit authenticator code for Res Imperii"
	}
	if hasPasskeys {
		subtitle = "Sign in with your Passkey or fallback credentials"
	}
	if reauth {
		subtitle = "Re-verify your identity to manage security settings"
	}

	var passwordField g.Node
	codeAutoFocus := g.Node(h.AutoFocus())
	if hasPasskeys {
		codeAutoFocus = nil
	}
	if requirePassword {
		codeAutoFocus = nil
		var pwAutoFocus g.Node
		if !hasPasskeys {
			pwAutoFocus = h.AutoFocus()
		}
		passwordField = g.Group([]g.Node{
			h.Input(h.Type("hidden"), h.Name("username"), h.Value("pilot"), g.Attr("autocomplete", "username")),
			h.Label(h.For("admin-password"), h.Class("auth-totp-label"), g.Text("Password")),
			h.Input(
				h.ID("admin-password"),
				h.Name("password"),
				h.Type("password"),
				h.Class("auth-password-input"),
				g.Attr("autocomplete", "current-password"),
				pwAutoFocus,
				h.Required(),
			),
		})
	}

	totpForm := h.Form(h.Class("auth-totp-form"), h.Method("POST"), h.Action("/login"),
		h.Input(h.Type("hidden"), h.Name("next"), h.Value(nextURL)),
		passwordField,
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
			codeAutoFocus,
			h.Required(),
		),
		h.Button(h.Type("submit"), h.Class("btn btn-neutral auth-totp-submit"), g.Text("Verify")),
	)

	var cardBody g.Node
	var passkeyScript g.Node
	if hasPasskeys {
		var openAttr g.Node
		if errMsg != "" {
			openAttr = g.Attr("open", "")
		}
		cardBody = g.Group([]g.Node{
			h.Div(h.Class("auth-passkey-primary"),
				h.Button(
					h.Type("button"),
					h.ID("passkey-login-btn"),
					h.Class("btn btn-encouraged auth-passkey-btn"),
					g.Attr("data-next", nextURL),
					g.Text("Sign in with passkey"),
				),
				h.P(h.ID("passkey-login-status"), h.Class("auth-passkey-status"), g.Attr("role", "status"), g.Attr("hidden", "")),
			),
			h.Details(h.Class("auth-alt-fallback"), openAttr,
				h.Summary(h.Class("auth-alt-summary"), g.Text("Or sign in with Password & Authenticator")),
				h.Div(h.Class("auth-alt-body"),
					errNode,
					totpForm,
				),
			),
		})
		passkeyScript = h.Script(h.Src("/assets/js/passkey.js?v=1"))
	} else {
		cardBody = g.Group([]g.Node{
			errNode,
			totpForm,
		})
	}

	return layout.Doc(
		"Authentication - RES IMPERII",
		StylePath+"?v=9",
		h.Main(h.Class("auth-page"),
			h.Header(h.Class("auth-header"),
				layout.BackLink("/", "Home"),
				h.H1(g.Text("AUTHENTICATION")),
				h.P(h.Class("auth-subtitle"), g.Text(subtitle)),
			),
			h.Section(h.Class("auth-card"),
				cardBody,
			),
		),
		passkeyScript,
	)
}
