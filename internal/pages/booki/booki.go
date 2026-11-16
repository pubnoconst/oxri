package booki

import (
	"bytes"
	_ "embed"
	"sync"

	"github.com/oxri/oxri-go/rawcontent"
	"github.com/yuin/goldmark"
)

//go:embed style.css
var StyleCSS string

var (
	renderedHTML string
	renderOnce   sync.Once
)

func RenderMarkdown() string {
	renderOnce.Do(func() {
		var buf bytes.Buffer
		// goldmark by default excludes raw HTML, matching the pulldown-cmark filter
		if err := goldmark.Convert([]byte(rawcontent.BookIMarkdown), &buf); err == nil {
			renderedHTML = buf.String()
		}
	})
	return renderedHTML
}
