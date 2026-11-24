package passage

import (
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestBox(t *testing.T) {
	var b strings.Builder
	if err := Box(domain.Passage{Reference: "John 3:16", Version: "KJV", Text: "For God so loved the world & all"}).Render(&b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{"<span class=\"passage-version\">KJV</span>", "<p>For God so loved the world &amp; all</p>", "John 3:16"} {
		if !strings.Contains(got, want) {
			t.Errorf("box lacks %q: %s", want, got)
		}
	}
}

func TestBoxEscapes(t *testing.T) {
	var b strings.Builder
	_ = Box(domain.Passage{Reference: "x", Version: "y", Text: `"><script>alert(1)</script>`}).Render(&b)
	if strings.Contains(b.String(), "<script>") {
		t.Errorf("not escaped: %s", b.String())
	}
}
