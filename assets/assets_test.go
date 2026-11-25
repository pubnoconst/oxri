package assets

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A file added to assets/ must get a typed handle, so nothing can reference
// it by a hand-written URL.
func TestEveryFileHasAHandle(t *testing.T) {
	_ = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !declared[p] {
			t.Errorf("embedded file %q has no typed handle in assets.go", p)
		}
		return nil
	})
}

func TestURLCarriesContentHash(t *testing.T) {
	b, _ := files.ReadFile("js/journal-media.js")
	if got, want := JournalMediaJS.URL(), "/assets/js/journal-media.js?v="+contentHash(b); got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	a := Component("/views/x/x.css", "a{}")
	b2 := Component("/views/x/x.css", "a{color:red}")
	if a.URL() == b2.URL() {
		t.Fatalf("different content must give different URLs")
	}
}

func TestCachingFollowsTheHash(t *testing.T) {
	notFound := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }
	h := FileHandler(notFound)

	get := func(url string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		return rec
	}

	if rec := get(SiteCSS.URL()); rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("current hash: expected 200 + immutable, got %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	for _, stale := range []string{SiteCSS.Path(), SiteCSS.Path() + "?v=15"} {
		if rec := get(stale); rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: expected 200 + no-cache, got %d %q", stale, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
	for _, bad := range []string{"/assets/", "/assets/js/", "/assets/js/../css/site.css", "/assets/nope.js", "/assets/js//journal-media.js"} {
		if rec := get(bad); rec.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", bad, rec.Code)
		}
	}

	comp := Component("/views/x/x.css", "a{}")
	rec := httptest.NewRecorder()
	comp.Handler()(rec, httptest.NewRequest(http.MethodGet, comp.URL(), nil))
	if rec.Body.String() != "a{}" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Errorf("component: unexpected response %q %q %q", rec.Body.String(), rec.Header().Get("Cache-Control"), rec.Header().Get("Content-Type"))
	}
}

// No Go source may hand-write an asset URL or a version number again:
// views must use the typed handles. Test files are exempt (they probe routes).
func TestNoHandWrittenAssetURLs(t *testing.T) {
	forbidden := regexp.MustCompile(`^/assets/|\?v=\d`)
	root := ".."
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			if rel == "bin" || strings.HasPrefix(rel, ".") && rel != ".." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || rel == "assets/assets.go" || rel == "assets/fonts.go" {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if v, uerr := strconv.Unquote(lit.Value); uerr == nil && forbidden.MatchString(v) {
				t.Errorf("%s: hand-written asset URL %q; use a typed handle from package assets", fset.Position(lit.Pos()), v)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
}
