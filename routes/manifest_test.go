package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oxri/oxri-go/assets"
)

// The site is installable: a valid manifest with a stable id, standalone
// display, and icons that resolve to PNGs of the sizes it claims (including a
// maskable one), linked from every page along with the Apple icon and colours.
func TestWebAppManifestAndIcons(t *testing.T) {
	router := NewRouter(Config{})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	rec := get("/manifest.webmanifest")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("manifest: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var m webManifest
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if m.ID != "/" || m.StartURL != "/" || m.Display != "standalone" || m.Name == "" || m.ShortName == "" {
		t.Errorf("manifest fields: %+v", m)
	}

	sawMaskable := false
	for _, ic := range m.Icons {
		res := get(ic.Src)
		if res.Code != http.StatusOK {
			t.Errorf("icon %s: %d", ic.Src, res.Code)
			continue
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(res.Body.Bytes()))
		if err != nil {
			t.Errorf("icon %s is not a PNG: %v", ic.Src, err)
			continue
		}
		if got := fmt.Sprintf("%dx%d", cfg.Width, cfg.Height); got != ic.Sizes {
			t.Errorf("icon %s is %s, manifest says %s", ic.Src, got, ic.Sizes)
		}
		if ic.Purpose == "maskable" {
			sawMaskable = true
		}
	}
	if !sawMaskable {
		t.Errorf("manifest needs a maskable icon for Android adaptive icons")
	}

	page := get("/").Body.String()
	for _, want := range []string{
		`rel="manifest" href="/manifest.webmanifest"`,
		`rel="apple-touch-icon" href="` + assets.AppleTouchIcon.URL() + `"`,
		`name="theme-color" media="(prefers-color-scheme: light)"`,
		`name="theme-color" media="(prefers-color-scheme: dark)"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("landing page head is missing %q", want)
		}
	}
}
