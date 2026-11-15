package graph

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

func init() {
	RegisterIndexer(GoIndexer{})
}

type GoIndexer struct{}

func (GoIndexer) Name() string { return "go" }

func (GoIndexer) Match(path string) bool {
	return filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go")
}

func (GoIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	full := filepath.Join(root, rel)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, full, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}

	pkgName := f.Name.Name
	pkgID := "pkg:" + pkgName
	fileID := "file:" + filepath.ToSlash(rel)

	nodes := []Node{
		{ID: pkgID, Type: NodePackage, Title: pkgName, Package: pkgName, Source: "extracted", Confidence: 1},
		{ID: fileID, Type: NodeFile, Path: filepath.ToSlash(rel), Title: filepath.Base(rel), Package: pkgName, Source: "extracted", Confidence: 1},
	}
	edges := []Edge{
		{SourceID: pkgID, TargetID: fileID, Type: EdgeDefines, Source: "extracted", Confidence: 1},
	}

	funcIDs := map[string]string{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		name := fn.Name.Name
		id := fmt.Sprintf("func:%s.%s", pkgName, name)
		funcIDs[name] = id
		nodes = append(nodes, Node{
			ID: id, Type: NodeFunction, Path: filepath.ToSlash(rel),
			Title: name, Package: pkgName, Source: "extracted", Confidence: 1,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: id, Type: EdgeDefines, Source: "extracted", Confidence: 1,
		})
	}

	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		target := "pkg:" + path
		nodes = append(nodes, Node{
			ID: target, Type: NodePackage, Title: path, Source: "extracted", Confidence: 1,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: target, Type: EdgeImports, Source: "extracted", Confidence: 1,
		})
	}

	for _, callerID := range funcIDs {
		parts := strings.Split(callerID, ".")
		fn := findFunc(f, parts[len(parts)-1])
		if fn == nil || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := callName(call)
			if callee == "" {
				return true
			}
			targetID := "func:" + pkgName + "." + callee
			if id, ok := funcIDs[callee]; ok {
				targetID = id
			} else if strings.Contains(callee, ".") {
				targetID = "func:" + callee
			}
			edges = append(edges, Edge{
				SourceID: callerID, TargetID: targetID, Type: EdgeCalls,
				Source: "extracted", Confidence: 0.8,
			})
			return true
		})
	}

	return nodes, edges, nil
}

func findFunc(f *ast.File, name string) *ast.FuncDecl {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name != nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func callName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if pkg, ok := fun.X.(*ast.Ident); ok {
			return pkg.Name + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	default:
		return ""
	}
}
