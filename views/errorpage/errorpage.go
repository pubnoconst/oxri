package errorpage

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"fmt"
	"github.com/oxri/oxri-go/assets"
	"net/http"

	"github.com/oxri/oxri-go/views/layout"
	qrcode "github.com/skip2/go-qrcode"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed errorpage.css
var StyleCSS string

// StylePath is the public route for the error page component CSS.
const StylePath = "/views/errorpage/errorpage.css"

// Style is the content-hashed handle for StyleCSS (URL and route handler).
var Style = assets.Component(StylePath, StyleCSS)

// MessageForStatus returns a secure, client-safe message for an HTTP status code.
// Internal error details or stack traces are never exposed.
func MessageForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "The request could not be understood or processed."
	case http.StatusUnauthorized:
		return "Authentication is required to proceed."
	case http.StatusForbidden:
		return "You do not have permission to view this page."
	case http.StatusNotFound:
		return "This page could not be found."
	case http.StatusMethodNotAllowed:
		return "The requested method is not supported for this resource."
	case http.StatusRequestTimeout:
		return "The request took too long. Please try again."
	case http.StatusRequestEntityTooLarge:
		return "The submitted payload exceeds the permitted size limit."
	case http.StatusGatewayTimeout:
		return "The service took too long to respond. Please try again."
	default:
		if status >= 400 && status < 500 {
			return "The request could not be completed."
		}
		return "Something went wrong. Please try again later."
	}
}

// NewReference generates a cryptographically random UUID v4 reference code.
func NewReference() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// QRDataURI generates a base64-encoded PNG data URI of a QR code containing the reference string.
func QRDataURI(reference string) string {
	if reference == "" {
		return ""
	}
	png, err := qrcode.Encode(reference, qrcode.Medium, 160)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// Page renders the error page markup using Gomponents.
func Page(status int, message, reference, qrDataURI string) g.Node {
	title := fmt.Sprintf("Error %d - RES IMPERII", status)
	return layout.Doc(
		title,
		Style.URL(),
		h.Main(h.Class("error-page"),
			h.Section(h.Class("error-card"),
				h.P(h.Class("error-wordmark"), g.Text("RES IMPERII")),
				h.P(h.Class("error-code"), g.Text(fmt.Sprintf("%d", status))),
				h.H1(h.Class("error-title"), g.Text(message)),
				h.P(h.Class("error-reference-label"), g.Text("Reference code")),
				h.P(h.Class("error-reference"), g.Text(reference)),
				g.If(qrDataURI != "",
					h.Div(h.Class("error-qr"),
						h.Img(h.Src(qrDataURI), h.Alt("QR code containing the reference code")),
						h.P(g.Text("Scan for the reference code")),
					),
				),
				h.A(h.Class("error-home-link link-plain"), h.Href("/"), g.Text("Return home")),
			),
		),
	)
}

// Render writes the HTTP status header and renders the complete error page.
func Render(w http.ResponseWriter, status int, userMsg string) {
	RenderWithReference(w, status, userMsg, NewReference())
}

// RenderWithReference renders the error page using a specific incident reference code.
func RenderWithReference(w http.ResponseWriter, status int, userMsg, ref string) {
	if userMsg == "" {
		userMsg = MessageForStatus(status)
	}
	if ref == "" {
		ref = NewReference()
	}
	qr := QRDataURI(ref)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := Page(status, userMsg, ref, qr).Render(w); err != nil {
		// Minimal emergency fallback
		_, _ = fmt.Fprintf(w, "<!doctype html><html><head><title>Error</title></head><body><h1>%d</h1><p>%s</p><p>Ref: %s</p><a href=\"/\">Home</a></body></html>",
			status, userMsg, ref)
	}
}
