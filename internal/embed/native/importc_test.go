package native

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOnlyNativeImportsC is ADR-018's import rule: no Go file outside
// internal/embed/native may import "C" (golangci's depguard cannot see the
// cgo pseudo-package).
func TestOnlyNativeImportsC(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	self, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "third_party", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			return nil // not ours to judge here; go vet reports it
		}
		for _, imp := range f.Imports {
			if imp.Path.Value != `"C"` {
				continue
			}
			abs, _ := filepath.Abs(filepath.Dir(p))
			if abs != self {
				t.Errorf("%s imports \"C\" outside internal/embed/native", p)
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
