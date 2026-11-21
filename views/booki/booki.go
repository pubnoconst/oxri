// Package booki provides the Gomponents reader view for Book I.
package booki

import (
	"bytes"
	_ "embed"
	"sync"

	"github.com/oxri/oxri-go/rawcontent"
	"github.com/oxri/oxri-go/views/layout"
	"github.com/yuin/goldmark"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed booki.css
var StyleCSS string

// StylePath is the public route for the Book I component CSS.
const StylePath = "/views/booki/booki.css"

var (
	renderedHTML string
	renderOnce   sync.Once
)

// RenderMarkdown safely renders the canonical Book I markdown into HTML.
// Goldmark excludes raw HTML by default, strictly preventing script injection.
func RenderMarkdown() string {
	renderOnce.Do(func() {
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(rawcontent.BookIMarkdown), &buf); err == nil {
			renderedHTML = buf.String()
		}
	})
	return renderedHTML
}

// Page renders the full Book I reading view inside the base layout shell.
func Page() g.Node {
	contentHTML := RenderMarkdown()

	return layout.Doc(
		"Book I - RES IMPERII",
		StylePath,
		h.Main(h.Class("article-page"),
			h.Article(h.Class("book-article"),
				layout.BackLink("/", "Back"),
				h.Section(h.Class("book-content"), g.Raw(contentHTML)),
				layout.BackLink("/", "Back"),
			),
		),
	)
}
