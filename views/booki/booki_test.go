package booki

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	src := "# Book I\n\n## On Cause\n\n<script>alert(1)</script>\n\n[Bad](javascript:alert(1)) [Good](https://example.com)\n"
	html := RenderMarkdown(src)

	if !strings.Contains(html, "<h1>Book I</h1>") {
		t.Errorf("expected <h1>Book I</h1>, got:\n%s", html)
	}
	if !strings.Contains(html, "<h2>On Cause</h2>") {
		t.Errorf("expected <h2>On Cause</h2>, got:\n%s", html)
	}
	if strings.Contains(strings.ToLower(html), "<script") || strings.Contains(strings.ToLower(html), "javascript:") {
		t.Errorf("XSS payload leaked into rendered HTML:\n%s", html)
	}
	if !strings.Contains(html, `href="https://example.com"`) {
		t.Errorf("expected safe https link to be preserved:\n%s", html)
	}
}

func TestBookPageAndEditPage(t *testing.T) {
	var buf bytes.Buffer
	if err := BookPage("book-i", "Book I", "# Book I\n\nHello.", true).Render(&buf); err != nil {
		t.Fatalf("BookPage().Render failed: %v", err)
	}
	rendered := buf.String()
	if !strings.Contains(rendered, "<h1>Book I</h1>") || !strings.Contains(rendered, `href="/book-i/edit"`) {
		t.Errorf("rendered BookPage missing heading or edit link")
	}

	buf.Reset()
	if err := EditPage("book-ii", "Book II", "# Book II\n\nDraft.", "").Render(&buf); err != nil {
		t.Fatalf("EditPage().Render failed: %v", err)
	}
	editHTML := buf.String()
	if !strings.Contains(editHTML, `action="/book-ii"`) || !strings.Contains(editHTML, "Save Book II") {
		t.Errorf("rendered EditPage missing expected form action or submit button")
	}
}
