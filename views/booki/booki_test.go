package booki

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	html := RenderMarkdown()
	if !strings.Contains(html, "<h1>Book I</h1>") {
		t.Errorf("expected <h1>Book I</h1>, got:\n%s", html[:min(len(html), 500)])
	}
	if !strings.Contains(html, "<h2>On Cause</h2>") {
		t.Errorf("expected <h2>On Cause</h2>, got:\n%s", html[:min(len(html), 500)])
	}
}

func TestPage(t *testing.T) {
	var buf bytes.Buffer
	err := Page().Render(&buf)
	if err != nil {
		t.Fatalf("Page().Render failed: %v", err)
	}
	rendered := buf.String()
	if !strings.Contains(rendered, "<h1>Book I</h1>") {
		t.Errorf("rendered HTML missing <h1>Book I</h1>:\n%s", rendered[:min(len(rendered), 1000)])
	}
}
