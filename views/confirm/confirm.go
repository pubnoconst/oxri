// Package confirm renders the site's confirmation for destructive actions,
// in place of the browser's confirm() dialog.
//
// A destructive control is a link to a GET endpoint that returns a Prompt:
// with HTMX (HX-Request) it is swapped into a slot beside the control
// (Inline); without JavaScript the same URL serves a whole page (Page). Only
// the prompt's own button performs the action, and it carries confirmed=1,
// which the POST handler requires (Confirmed), so nothing can skip the
// question.
package confirm

import (
	"net/http"

	"github.com/oxri/oxri-go/views/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Field is the name and value the confirm button submits (e.g. action=delete_all).
type Field struct{ Name, Value string }

// Target is where the confirm button sends the action: its own POST form
// (PostTo) or an existing form on the page (SubmitForm). Sealed: only these two.
type Target interface{ target() }

// PostTo posts a form of the prompt's own to this URL.
type PostTo string

// SubmitForm submits the page's form with this id (the prompt sits inside or
// beside it, so it must not nest a form of its own).
type SubmitForm string

func (PostTo) target()     {}
func (SubmitForm) target() {}

// Prompt is one confirmation.
type Prompt struct {
	Question string // "Delete this journal entry permanently?"
	Confirm  string // the confirm button: "Delete entry"
	Danger   bool   // destructive styling for the confirm button
	Target   Target
	Submit   Field  // carried by the confirm button (optional)
	KeepHref string // where "Keep" goes on the whole-page prompt
}

// confirmedField is the proof that the person answered the prompt.
const confirmedField = "confirmed"

// Confirmed reports whether a POST came from a prompt's confirm button.
func Confirmed(r *http.Request) bool {
	return r.FormValue(confirmedField) == "1"
}

// IsFragmentRequest reports whether HTMX asked for the inline prompt.
func IsFragmentRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// Render writes the inline prompt for HTMX, or the whole page otherwise.
func Render(w http.ResponseWriter, r *http.Request, title string, p Prompt) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Vary", "HX-Request")
	w.Header().Set("Cache-Control", "no-store")
	if IsFragmentRequest(r) {
		_ = Inline(p).Render(w)
		return
	}
	_ = Page(title, p).Render(w)
}

func (p Prompt) confirmButton() g.Node {
	cls := "btn btn-neutral btn-sm"
	if p.Danger {
		cls = "btn btn-destructive btn-sm"
	}
	attrs := []g.Node{h.Type("submit"), h.Class(cls)}
	if f, ok := p.Target.(SubmitForm); ok {
		attrs = append(attrs, g.Attr("form", string(f)))
	}
	if p.Submit.Name != "" {
		attrs = append(attrs, h.Name(p.Submit.Name), h.Value(p.Submit.Value))
	}
	return h.Button(append(attrs, g.Text(p.Confirm))...)
}

// confirmControl is the confirm button with its confirmed=1 proof, wrapped in
// a POST form or bound to the page's form.
func (p Prompt) confirmControl() g.Node {
	switch t := p.Target.(type) {
	case PostTo:
		return h.Form(h.Class("confirm-form"), h.Method("POST"), h.Action(string(t)),
			h.Input(h.Type("hidden"), h.Name(confirmedField), h.Value("1")),
			p.confirmButton(),
		)
	case SubmitForm:
		return g.Group([]g.Node{
			h.Input(h.Type("hidden"), g.Attr("form", string(t)), h.Name(confirmedField), h.Value("1")),
			p.confirmButton(),
		})
	default:
		return nil
	}
}

// Inline is the prompt swapped in beside the control. "Keep" closes it in the
// browser (navigation-feedback.js), with no round trip; it takes focus, so a
// stray Enter never confirms.
func Inline(p Prompt) g.Node {
	return h.Div(h.Class("confirm-inline"), g.Attr("role", "group"), g.Attr("aria-label", "Confirm"),
		h.P(h.Class("confirm-question"), g.Text(p.Question)),
		h.Div(h.Class("confirm-actions"),
			p.confirmControl(),
			h.Button(h.Type("button"), h.Class("btn btn-neutral btn-sm"), g.Attr("data-confirm-keep", ""), g.Attr("autofocus", ""), g.Text("Keep")),
		),
	)
}

// Page is the whole-page prompt (no JavaScript).
func Page(title string, p Prompt) g.Node {
	keep := p.KeepHref
	if keep == "" {
		keep = "/"
	}
	return layout.Doc(title+" - RES IMPERII", "",
		h.Main(h.Class("confirm-page"),
			h.Div(h.Class("confirm-inline"),
				h.P(h.Class("confirm-question"), g.Text(p.Question)),
				h.Div(h.Class("confirm-actions"),
					p.confirmControl(),
					h.A(h.Class("btn btn-neutral btn-sm"), h.Href(keep), g.Text("Keep")),
				),
			),
		),
	)
}
