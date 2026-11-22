package embed

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestOnlyChildReachesNative pins the ADR-019 invariant that native code
// runs only in tb __graph-embed: its kernels may use instructions every
// other command must not execute. native.Open is the first native call, it
// is called only by Serve and Smoke, and those only by the child command.
func TestOnlyChildReachesNative(t *testing.T) {
	root := moduleRoot(t)
	allowed := map[string][]string{
		"native.Open": {"internal/embed/serve.go", "internal/embed/smoke.go"},
		"embed.Serve": {"cmd/tb/graph_embed.go"},
		"embed.Smoke": {"cmd/tb/graph_embed.go"},
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			switch d.Name() {
			case ".git", "third_party", "testdata", "node_modules", "fixtures":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		for _, call := range qualifiedCalls(t, path) {
			files, watched := allowed[call]
			if watched && !slices.Contains(files, rel) {
				t.Errorf("%s calls %s; only %v may (ADR-019)", rel, call, files)
			}
		}
		// Inside package embed the entry points are unqualified.
		if strings.HasPrefix(rel, "internal/embed/") && !strings.Contains(strings.TrimPrefix(rel, "internal/embed/"), "/") {
			for _, name := range unqualifiedCalls(t, path) {
				if name == "Serve" || name == "Smoke" {
					t.Errorf("%s calls %s; only cmd/tb/graph_embed.go may (ADR-019)", rel, name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func parseFile(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return f
}

// qualifiedCalls lists pkg.Func calls, also as method values (pkg.Func
// passed without a call).
func qualifiedCalls(t *testing.T, path string) []string {
	var out []string
	ast.Inspect(parseFile(t, path), func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				out = append(out, id.Name+"."+sel.Sel.Name)
			}
		}
		return true
	})
	return out
}

func unqualifiedCalls(t *testing.T, path string) []string {
	var out []string
	ast.Inspect(parseFile(t, path), func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok {
				out = append(out, id.Name)
			}
		}
		return true
	})
	return out
}
