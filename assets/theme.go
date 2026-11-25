package assets

import (
	"crypto/sha256"
	"encoding/base64"
)

// ThemeInitJS is the one inline script on the site. It runs in <head>,
// before anything is drawn, and applies the reader's remembered light or
// dark choice (theme-toggle.js saves it), so a page never flashes the other
// theme first (HIG §1.7). Without a saved choice the page follows the
// device's setting (prefers-color-scheme in site.css).
const ThemeInitJS = `try{var t=localStorage.getItem("res-imperii-theme");if(t==="light"||t==="dark")document.documentElement.dataset.theme=t}catch(e){}`

// ThemeInitCSPSource allows exactly ThemeInitJS and no other inline script:
// the Content-Security-Policy names it by its SHA-256 (routes/middleware.go).
var ThemeInitCSPSource = func() string {
	sum := sha256.Sum256([]byte(ThemeInitJS))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}()
