// Package login provides the Gomponents authentication views for Res Imperii.
package login

import (
	_ "embed"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed login.css
var StyleCSS string

// StylePath is the public route for the login component CSS.
const StylePath = "/views/login/login.css"

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
		passkeyScript = h.Script(h.Src("/assets/js/passkey.js?v=2"))
	} else {
		cardBody = g.Group([]g.Node{
			errNode,
			totpForm,
		})
	}

	return layout.Doc(
		"Authentication - RES IMPERII",
		StylePath+"?v=11",
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
