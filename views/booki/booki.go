// Package booki provides the Gomponents reader and Markdown editor views
// for Book I, Book II, and Book III, backed by Turso.
package booki

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/oxri/oxri-go/assets"
	"html"
	"regexp"
	"strings"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/views/layout"
	"github.com/yuin/goldmark"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

//go:embed booki.css
var StyleCSS string

// StylePath is the public route for the Book reader & editor component CSS.
const StylePath = "/views/booki/booki.css"

// Style is the content-hashed handle for StyleCSS (URL and route handler).
var Style = assets.Component(StylePath, StyleCSS)

var attrSchemeRe = regexp.MustCompile(`(?i)\b(href|src)\s*=\s*"([^"]*)"`)

// RenderMarkdown converts Book Markdown into strictly sanitized HTML:
// 1. Input prose runes are whitelisted (Latin, Greek, ASCII digits, standard punctuation).
// 2. Goldmark omits raw HTML tags (<script>, <iframe>, <svg>, etc.) by default.
// 3. All href/src attributes are restricted to https://, http://, /, or # schemes.
func RenderMarkdown(src string) string {
	cleanSrc := db.SanitizeProseInput(src)
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(cleanSrc), &buf); err != nil {
		return ""
	}
	return attrSchemeRe.ReplaceAllStringFunc(buf.String(), func(match string) string {
		sub := attrSchemeRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		attrName := sub[1]
		val := strings.TrimSpace(html.UnescapeString(sub[2]))
		lower := strings.ToLower(val)
		if strings.HasPrefix(lower, "https://") ||
			strings.HasPrefix(lower, "http://") ||
			(strings.HasPrefix(lower, "/") && !strings.HasPrefix(lower, "//")) ||
			strings.HasPrefix(lower, "#") {
			return match
		}
		return fmt.Sprintf(`%s="#"`, attrName)
	})
}

// BookPage renders a Book reading view (Book I, II, or III) from persisted Markdown.
// When isAuth is true, a quiet Edit action is displayed in the header bar.
func BookPage(slug, title, markdownBody string, isAuth bool) g.Node {
	contentHTML := RenderMarkdown(markdownBody)

	var editAction g.Node
	if isAuth {
		editAction = h.A(h.Class("btn btn-neutral btn-sm book-edit-link"), h.Href("/"+slug+"/edit"), g.Text("Edit"))
	}

	return layout.Doc(
		title+" - RES IMPERII",
		Style.URL(),
		h.Main(h.Class("article-page"),
			h.Article(h.Class("book-article"),
				h.Div(h.Class("book-top-bar"),
					layout.BackLink("/", "Back"),
					editAction,
				),
				h.Section(h.Class("book-content"), g.Raw(contentHTML)),
				layout.BackLink("/", "Back"),
			),
		),
	)
}

// EditPage renders the authenticated 2-column Markdown editor + live sanitized preview for a Book.
func EditPage(slug, title, markdownBody, errMsg string) g.Node {
	var errNode g.Node
	if errMsg != "" {
		errNode = h.P(h.Class("book-editor-error"), g.Attr("role", "alert"), g.Text(errMsg))
	}

	return layout.Doc(
		"Edit "+title+" - RES IMPERII",
		Style.URL(),
		h.Main(h.Class("book-editor-page"),
			h.Header(h.Class("book-editor-header"),
				layout.BackLink("/"+slug, title),
				h.H1(g.Text("Edit "+title)),
			),
			errNode,
			h.Form(h.Class("book-editor-form"), h.Method("POST"), h.Action("/"+slug),
				h.Div(h.Class("book-editor-grid"),
					h.Textarea(
						h.ID("book-md-input"),
						h.Name("body"),
						h.Class("book-md-textarea"),
						g.Attr("aria-label", title),
						g.Attr("aria-describedby", "book-length-error"),
						// The limit is in bytes (db.MaxBookBodyBytes); book-editor.js
						// shows an error past it. No maxlength: it counts characters
						// and would silently cut a long paste.
						g.Attr("data-max-bytes", fmt.Sprintf("%d", db.MaxBookBodyBytes)),
						g.Attr("spellcheck", "true"),
						h.Required(),
						g.Text(markdownBody),
					),
					h.Div(h.ID("book-md-preview"), h.Class("book-content book-preview-box"), g.Attr("aria-label", "How it will look"), g.Raw(RenderMarkdown(markdownBody))),
				),
				h.P(h.ID("book-length-error"), h.Class("book-editor-error"), g.Attr("role", "alert"), g.Attr("hidden", "")),
				h.Div(h.Class("book-editor-actions"),
					h.Button(h.Type("submit"), h.Class("btn btn-encouraged"), g.Text("Save changes")),
					h.A(h.Class("btn btn-neutral"), h.Href("/"+slug), g.Text("Cancel")),
				),
			),
		),
		h.Script(h.Src(assets.MarkdownItJS.URL())),
		h.Script(h.Src(assets.PurifyJS.URL())),
		h.Script(h.Src(assets.BookEditorJS.URL())),
	)
}
