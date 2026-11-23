package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrivilegedCallSites pins the places where the type system cannot help:
//
//   - Go cannot forbid the zero value domain.AdminCap{}. A zero cap is only
//     ever a failure result ("no authority"), so the literal may appear only
//     in the code that reports authentication failures. Anywhere else it would
//     be an attempt to forge authority (writes reject it at runtime too).
//   - Store.RestoreSnapshot replaces the whole database and is deliberately
//     uncapped (an offline operator tool holding raw database credentials).
//     It may be called only from cmd/backup.
//
// The scan parses non-test Go sources (comments and strings are ignored).
func TestPrivilegedCallSites(t *testing.T) {
	zeroCapAllowed := map[string]bool{
		"internal/auth/auth.go": true, // AuthenticateFresh: (zero, valid=false, fresh=false)
		"serverfns/auth.go":     true, // RequireAuth / RequireFreshAuth error returns
	}
	restoreAllowed := map[string]bool{
		"cmd/backup/main.go": true,
	}

	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "bin" || strings.HasPrefix(rel, ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		inDomain := strings.HasPrefix(rel, "internal/domain/")

		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if isAdminCapType(x.Type, inDomain) && !inDomain && !zeroCapAllowed[rel] {
					t.Errorf("%s: domain.AdminCap literal outside the allowed failure paths (forged authority?)", fset.Position(x.Pos()))
				}
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "RestoreSnapshot" && !restoreAllowed[rel] {
					t.Errorf("%s: RestoreSnapshot may only be called from cmd/backup", fset.Position(x.Pos()))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
}

func isAdminCapType(expr ast.Expr, inDomain bool) bool {
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		pkg, ok := x.X.(*ast.Ident)
		return ok && pkg.Name == "domain" && x.Sel.Name == "AdminCap"
	case *ast.Ident:
		return inDomain && x.Name == "AdminCap"
	}
	return false
}
