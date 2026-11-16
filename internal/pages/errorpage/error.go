package errorpage

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"fmt"
	"net/http"

	qrcode "github.com/skip2/go-qrcode"
)

//go:embed style.css
var StyleCSS string

func MessageForStatus(status int) string {
	switch status {
	case http.StatusForbidden:
		return "You do not have permission to view this page."
	case http.StatusNotFound:
		return "This page could not be found."
	case http.StatusRequestTimeout:
		return "The request took too long. Please try again."
	case http.StatusGatewayTimeout:
		return "The service took too long to respond. Please try again."
	default:
		if status >= 400 && status < 500 {
			return "The request could not be completed."
		}
		return "Something went wrong. Please try again later."
	}
}

func QRDataURI(reference string) string {
	png, err := qrcode.Encode(reference, qrcode.Medium, 160)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func NewReference() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func FallbackPage(status int) string {
	return fmt.Sprintf("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>Error - RES IMPERII</title></head><body><main><h1>%d</h1><p>%s</p><a href=\"/\">Return home</a></main></body></html>",
		status, MessageForStatus(status))
}
