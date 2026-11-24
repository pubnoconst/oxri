package routes

import (
	"encoding/json"
	"net/http"

	"github.com/oxri/oxri-go/assets"
)

// Colours for the installed app. The splash background is the favicon's cream
// (#fefaee), so the "RI" icon blends into it; the default chrome colour is the
// site's light --page (pages override it per theme with theme-color metas).
const (
	iconCream = "#fefaee"
	pageLight = "#f5f4f0"
)

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

type webManifest struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Icons           []manifestIcon `json:"icons"`
}

// manifestJSON is built once: the icon URLs carry content hashes (assets), so
// a redrawn icon is picked up without anyone editing a URL by hand. The id is
// fixed so a changed manifest never reads to the browser as a different app.
var manifestJSON = func() []byte {
	b, err := json.MarshalIndent(webManifest{
		ID:              "/",
		Name:            "Res Imperii",
		ShortName:       "Res Imperii",
		Description:     "Journals and Books I–III.",
		StartURL:        "/",
		Scope:           "/",
		Display:         "standalone",
		BackgroundColor: iconCream,
		ThemeColor:      pageLight,
		Icons: []manifestIcon{
			{Src: assets.Icon192.URL(), Sizes: "192x192", Type: "image/png", Purpose: "any"},
			{Src: assets.Icon512.URL(), Sizes: "512x512", Type: "image/png", Purpose: "any"},
			{Src: assets.IconMaskable512.URL(), Sizes: "512x512", Type: "image/png", Purpose: "maskable"},
		},
	}, "", "  ")
	if err != nil {
		panic("routes: manifest: " + err.Error())
	}
	return b
}()

// handleManifest serves the web app manifest (installability). It must
// revalidate: browsers read it to decide name, icons and colours.
func handleManifest(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(manifestJSON)
}
