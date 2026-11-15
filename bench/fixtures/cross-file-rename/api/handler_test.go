package api

import (
	"cross-file-rename/models"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestGreeting(t *testing.T) {
	u := models.User{ID: 1, Email: "a@b.c"}
	got := Greeting(u)
	if got != "hello a@b.c" {
		t.Fatalf("Greeting = %q", got)
	}
}

func TestNoCustomerType(t *testing.T) {
	root := ".."
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if ok && id.Name == "Customer" {
				t.Errorf("%s ainda referencia Customer", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
