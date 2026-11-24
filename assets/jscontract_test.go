package assets

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// JavaScript has no compiler to catch a broken cross-file call, so this test
// checks the contract between the site's scripts. Scripts share code only
// through a few globals, window.ResImperii<Name> = { members }:
//
//   - every member another script uses must be exported (a removed or
//     renamed member fails here, not in a reader's browser);
//   - every exported member must be used by some other script (no dead
//     exports: a global is an API, kept as small as its callers).
//
// A use is window.ResImperiiX.member, or destructuring:
// const { a, b } = window.ResImperiiX.
func TestScriptGlobalsMatchTheirCallers(t *testing.T) {
	comments := regexp.MustCompile(`(?s)/\*.*?\*/|(?m)^\s*//[^\n]*|[^:]//[^\n]*`)
	export := regexp.MustCompile(`window\.(ResImperii\w+)\s*=\s*\{([^}]*)\}`)
	member := regexp.MustCompile(`window\.(ResImperii\w+)\.(\w+)`)
	destructure := regexp.MustCompile(`\{([^}]*)\}\s*=\s*window\.(ResImperii\w+)`)

	scripts := map[string]string{}
	_ = fs.WalkDir(files, "js", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".js") && !strings.HasSuffix(p, ".min.js") {
			b, _ := files.ReadFile(p)
			scripts[p] = comments.ReplaceAllString(string(b), "")
		}
		return nil
	})
	if len(scripts) < 5 {
		t.Fatalf("read only %d scripts", len(scripts))
	}

	exported := map[string]map[string]string{} // global -> member -> defining file
	for file, src := range scripts {
		for _, m := range export.FindAllStringSubmatch(src, -1) {
			if exported[m[1]] == nil {
				exported[m[1]] = map[string]string{}
			}
			for _, part := range strings.Split(m[2], ",") {
				if name := strings.TrimSpace(strings.SplitN(part, ":", 2)[0]); name != "" {
					exported[m[1]][name] = file
				}
			}
		}
	}
	if len(exported) == 0 {
		t.Fatal("found no window.ResImperii* globals; did the export pattern change?")
	}

	used := map[string]map[string]bool{} // global -> member used outside its defining file
	use := func(file, global, name string) {
		if _, ok := exported[global]; !ok {
			t.Errorf("%s uses window.%s, which no script defines", file, global)
			return
		}
		def, ok := exported[global][name]
		if !ok {
			t.Errorf("%s uses window.%s.%s, which %s does not export", file, global, name, definingFile(exported[global]))
			return
		}
		if def != file {
			if used[global] == nil {
				used[global] = map[string]bool{}
			}
			used[global][name] = true
		}
	}
	for file, src := range scripts {
		for _, m := range member.FindAllStringSubmatch(src, -1) {
			use(file, m[1], m[2])
		}
		for _, m := range destructure.FindAllStringSubmatch(src, -1) {
			for _, part := range strings.Split(m[1], ",") {
				if name := strings.TrimSpace(strings.SplitN(part, ":", 2)[0]); name != "" {
					use(file, m[2], name)
				}
			}
		}
	}

	for global, members := range exported {
		var dead []string
		for name := range members {
			if !used[global][name] {
				dead = append(dead, name)
			}
		}
		sort.Strings(dead)
		if len(dead) > 0 {
			t.Errorf("window.%s exports %v, which no other script uses; drop them from the global", global, dead)
		}
	}
}

func definingFile(members map[string]string) string {
	for _, f := range members {
		return f
	}
	return "its script"
}
