package graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	gopath "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

func init() {
	RegisterIndexer(GoIndexer{})
}

type GoIndexer struct{}

func (GoIndexer) Name() string     { return "go" }
func (GoIndexer) Category() string { return CategoryLanguage }

func (GoIndexer) Match(path string) bool {
	return filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go")
}

func (GoIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSourceBytes(root, rel)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(root, rel), src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}

	pkgName := f.Name.Name
	pkgID := goPackageID(root, rel, pkgName)
	fileID := "file:" + filepath.ToSlash(rel)
	slash := filepath.ToSlash(rel)

	nodes := []Node{
		{ID: "pkg:" + pkgID, Type: NodePackage, Title: pkgID, Package: pkgID, Source: "extracted", Confidence: 1},
		{ID: fileID, Type: NodeFile, Path: slash, Title: filepath.Base(rel), Package: pkgID, Source: "extracted", Confidence: 1},
	}
	edges := []Edge{
		{SourceID: "pkg:" + pkgID, TargetID: fileID, Type: EdgeDefines, Source: "extracted", Confidence: 1},
	}

	type goFunc struct {
		id   string
		decl *ast.FuncDecl
		recv string
		self string
	}
	var funcs []goFunc
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		recvType, recvName := goReceiver(fn)
		title := fn.Name.Name
		qual := pkgID + "." + title
		if recvType != "" {
			title = recvType + "." + title
			qual = pkgID + "." + title
		}
		id := "func:" + qual
		funcs = append(funcs, goFunc{id: id, decl: fn, recv: recvType, self: recvName})
		nodes = append(nodes, Node{
			ID: id, Type: NodeFunction, Path: slash,
			Title: title, Package: pkgID, Source: "extracted", Confidence: 1,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: id, Type: EdgeDefines, Source: "extracted", Confidence: 1,
		})
	}

	imports := map[string]string{}
	for _, imp := range f.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		target := "pkg:" + impPath
		nodes = append(nodes, Node{
			ID: target, Type: NodePackage, Title: impPath, Package: impPath, Source: "extracted", Confidence: 1,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: target, Type: EdgeImports, Source: "extracted", Confidence: 1,
		})
		name := goImportName(imp, impPath)
		if name != "_" && name != "." {
			imports[name] = impPath
		}
	}

	for _, fn := range funcs {
		if fn.decl.Body == nil {
			continue
		}
		ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if targetID := goCallTarget(call, fn.recv, fn.self, pkgID, imports); targetID != "" {
				edges = append(edges, Edge{
					SourceID: fn.id, TargetID: targetID, Type: EdgeCalls,
					Source: "extracted", Confidence: 0.8,
				})
			}
			return true
		})
	}

	return nodes, edges, nil
}

// goCallTarget names the callee of call: a bare name is a function of the
// same package, pkg.F resolves through the file's imports, and recv.m inside a
// method resolves to the same receiver's method. Anything else (a call on a
// local variable) is emitted under goUnresolvedPrefix.
func goCallTarget(call *ast.CallExpr, recv, self, pkgID string, imports map[string]string) string {
	fn := call.Fun
	for {
		switch x := fn.(type) {
		case *ast.IndexExpr:
			fn = x.X
			continue
		case *ast.IndexListExpr:
			fn = x.X
			continue
		case *ast.ParenExpr:
			fn = x.X
			continue
		}
		break
	}
	switch fun := fn.(type) {
	case *ast.Ident:
		return "func:" + pkgID + "." + fun.Name
	case *ast.SelectorExpr:
		x, isIdent := fun.X.(*ast.Ident)
		switch {
		case !isIdent:
			return goUnresolvedPrefix + fun.Sel.Name
		case imports[x.Name] != "":
			return "func:" + imports[x.Name] + "." + fun.Sel.Name
		case recv != "" && x.Name == self:
			return "func:" + pkgID + "." + recv + "." + fun.Sel.Name
		}
		return goUnresolvedPrefix + x.Name + "." + fun.Sel.Name
	}
	return ""
}

// goUnresolvedPrefix marks a call on a value whose type is unknown. The "?"
// cannot occur in an import path or Go identifier, so the target never lands
// on a real node (a one-segment package ID such as `app` would otherwise make
// `app.Run()` on a local variable resolve to the function app.Run).
const goUnresolvedPrefix = "func:?"

// goReceiver returns the receiver's type name and the receiver variable name;
// both empty for a plain function.
func goReceiver(fn *ast.FuncDecl) (typeName, varName string) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", ""
	}
	if names := fn.Recv.List[0].Names; len(names) > 0 {
		varName = names[0].Name
	}
	return recvName(fn.Recv), varName
}

func goImportName(imp *ast.ImportSpec, importPath string) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	parts := strings.Split(importPath, "/")
	name := parts[len(parts)-1]
	if len(parts) > 1 && goMajorSuffix.MatchString(name) {
		name = parts[len(parts)-2]
	}
	if i := strings.LastIndex(name, ".v"); i > 0 && goMajorSuffix.MatchString(name[i+1:]) {
		name = name[:i]
	}
	return strings.TrimPrefix(name, "go-")
}

var goMajorSuffix = regexp.MustCompile(`^v\d+$`)

type goModEntry struct {
	mod     string
	modTime time.Time
	size    int64
}

// goModCache avoids re-reading go.mod for every file; the stat check keeps a
// long-lived process (the MCP server) from serving a renamed module.
var goModCache sync.Map

// goPackageID names a package by its import path, so two directories that
// both declare `package main` stay distinct. Without a go.mod the
// root-relative directory stands in; the root directory falls back to the
// package clause.
func goPackageID(root, rel, pkgName string) string {
	dir := filepath.ToSlash(filepath.Dir(rel))
	for d := dir; ; d = gopath.Dir(d) {
		if mod := goModulePath(root, gopath.Join(d, "go.mod")); mod != "" {
			sub := strings.TrimPrefix(strings.TrimPrefix(dir, d), "/")
			if d == "." {
				sub = dir
			}
			if sub == "" || sub == "." {
				return mod
			}
			return mod + "/" + sub
		}
		if d == "." || d == "/" || d == "" {
			break
		}
	}
	if dir == "." || dir == "" {
		return pkgName
	}
	return dir
}

func goModulePath(root, rel string) string {
	gomod := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Lstat(gomod)
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	if v, ok := goModCache.Load(gomod); ok {
		e := v.(goModEntry)
		if e.modTime.Equal(fi.ModTime()) && e.size == fi.Size() {
			return e.mod
		}
	}
	mod := ""
	if data, err := readSourceBytes(root, rel); err == nil {
		if m := goModuleLine.FindSubmatch(data); m != nil {
			mod = strings.Trim(string(m[1]), `"`)
		}
	}
	goModCache.Store(gomod, goModEntry{mod: mod, modTime: fi.ModTime(), size: fi.Size()})
	return mod
}

var goModuleLine = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)
