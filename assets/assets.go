// Package assets embeds the site's static files and gives every one of them a
// content-hashed URL.
//
// Views never write an asset URL or a version number by hand: they refer to a
// typed handle (assets.SiteCSS, assets.JournalMediaJS, …) whose URL carries a
// hash of the exact bytes being served (…?v=<hash>). Edit a file and its URL
// changes on the next build, so browsers can never keep a stale copy, and a
// mistyped or deleted file fails at process start (and in every test) instead
// of shipping a broken reference.
//
// Component stylesheets owned by view packages (embedded as strings there) use
// Component() to get the same treatment.
package assets

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed css js icons favicon.png
var files embed.FS

// Asset is a static file with a content-hashed URL. Its fields are unexported:
// handles are declared only in this package (files) or minted by Component.
type Asset struct {
	path        string // URL path, e.g. /assets/js/journal-media.js
	hash        string
	contentType string
	body        []byte // only for Component assets (served by Handler)
}

// URL is the cache-busting URL to put in HTML: path?v=<content hash>.
func (a Asset) URL() string { return a.path + "?v=" + a.hash }

// Path is the route the asset is served at (without the version).
func (a Asset) Path() string { return a.path }

// Embedded files. Each is verified to exist at init.
var (
	Favicon = file("favicon.png")

	// App icons (cmd/icongen draws them; `mage icons`).
	Icon192         = file("icons/icon-192.png")
	Icon512         = file("icons/icon-512.png")
	IconMaskable512 = file("icons/icon-maskable-512.png")
	AppleTouchIcon  = file("icons/apple-touch-icon.png")
	SiteCSS         = file("css/site.css")

	HTMXJS               = file("js/htmx.min.js")
	MarkdownItJS         = file("js/markdown-it.min.js")
	PurifyJS             = file("js/purify.min.js")
	ThemeToggleJS        = file("js/theme-toggle.js")
	NavigationFeedbackJS = file("js/navigation-feedback.js")
	JournalMediaJS       = file("js/journal-media.js")
	DotMatrixJS          = file("js/dot-matrix.js")
	JournalEditorJS      = file("js/journal-editor.js")
	JournalsCatalogJS    = file("js/journals-catalog.js")
	LoadingTransitionJS  = file("js/loading-transition.js")
	VacateSpaceJS        = file("js/vacate-space.js")
	PasskeyJS            = file("js/passkey.js")
	BookEditorJS         = file("js/book-editor.js")
	OfflineJS            = file("js/offline.js")
)

// declared records every embedded-file handle (see TestEveryFileHasAHandle).
var declared = map[string]bool{}

func file(rel string) Asset {
	b, err := files.ReadFile(rel)
	if err != nil {
		panic("assets: missing embedded file " + rel + ": " + err.Error())
	}
	declared[rel] = true
	return Asset{path: "/assets/" + rel, hash: contentHash(b)}
}

// Component returns a content-hashed handle for a stylesheet a view package
// embeds itself (served at urlPath by Handler).
func Component(urlPath, css string) Asset {
	return Asset{path: urlPath, hash: contentHash([]byte(css)), contentType: "text/css; charset=utf-8", body: []byte(css)}
}

// Handler serves a Component asset.
func (a Asset) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCaching(w, r, a.hash)
		w.Header().Set("Content-Type", a.contentType)
		_, _ = w.Write(a.body)
	}
}

func contentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:12]
}

// setCaching: a request carrying the current hash can be cached forever (the
// URL changes whenever the content does); anything else must revalidate.
func setCaching(w http.ResponseWriter, r *http.Request, hash string) {
	if r.URL.Query().Get("v") == hash {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

// RoutePattern is the router pattern FileHandler must be mounted at.
const RoutePattern = "/assets/*"

// FileHandler serves the embedded files under /assets/. Directories, missing
// files and traversal attempts go to notFound (no directory listings).
func FileHandler(notFound http.HandlerFunc) http.Handler {
	hashes := map[string]string{}
	_ = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, rerr := files.ReadFile(p); rerr == nil {
				hashes[p] = contentHash(b)
			}
		}
		return nil
	})
	server := http.StripPrefix("/assets/", http.FileServerFS(files))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/assets/")
		if rel == "" || strings.HasSuffix(r.URL.Path, "/") || strings.Contains(rel, "..") || path.Clean(rel) != rel {
			notFound(w, r)
			return
		}
		hash, ok := hashes[rel]
		if !ok {
			notFound(w, r)
			return
		}
		setCaching(w, r, hash)
		server.ServeHTTP(w, r)
	})
}
