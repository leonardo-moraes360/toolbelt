package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
)

// Symbol kinds of GoSymbols.
const (
	SymFunc   = "func"
	SymMethod = "method"
	SymType   = "type"
	SymConst  = "const"
	SymVar    = "var"
)

// Symbol is one top-level Go declaration of a file. Lines are 1-based and
// inclusive; SHA256 covers the raw source bytes of the range, so it does not
// depend on go/printer or on the Go version of the build.
type Symbol struct {
	Name      string
	Kind      string
	StartLine int
	EndLine   int
	SHA256    string
	// Ambiguous marks a name a citation cannot pin: "_", "init", or a name
	// declared more than once in the file.
	Ambiguous bool
}

// GoSymbols parses one Go file's bytes and returns its top-level funcs,
// methods (named Recv.Name, pointer and type parameters stripped), types,
// consts and vars, ordered by position. It is pure: no file system, no
// object resolution. A syntax error returns the parser's error and no
// symbols, so a half-parsed file never yields a hash.
//
// The range starts at the declaration keyword (the doc comment stays out, so
// editing a comment is not a change). Inside a parenthesized group only the
// spec is hashed; an ungrouped type, const or var includes its keyword.
func GoSymbols(src []byte) ([]Symbol, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var out []Symbol
	add := func(name, kind string, from, to token.Pos) {
		// PositionFor(.., false) ignores //line directives: ranges are
		// positions in these bytes.
		a, b := fset.PositionFor(from, false), fset.PositionFor(to-1, false)
		if a.Offset < 0 || b.Offset+1 > len(src) || b.Offset < a.Offset {
			return
		}
		sum := sha256.Sum256(src[a.Offset : b.Offset+1])
		out = append(out, Symbol{Name: name, Kind: kind, StartLine: a.Line, EndLine: b.Line, SHA256: hex.EncodeToString(sum[:])})
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				add(d.Name.Name, SymFunc, d.Pos(), d.End())
				continue
			}
			if r := recvName(d.Recv); r != "" {
				add(r+"."+d.Name.Name, SymMethod, d.Pos(), d.End())
			}
		case *ast.GenDecl:
			genSymbols(d, add)
		}
	}
	count := map[string]int{}
	for i := range out {
		count[out[i].Name]++
	}
	for i := range out {
		n := out[i].Name
		out[i].Ambiguous = count[n] > 1 || n == "_" || n == "init"
	}
	return out, nil
}

// genSymbols adds the specs of one type, const or var declaration.
func genSymbols(d *ast.GenDecl, add func(name, kind string, from, to token.Pos)) {
	grouped := d.Lparen.IsValid()
	for _, sp := range d.Specs {
		from := d.Pos()
		if grouped {
			from = sp.Pos()
		}
		switch sp := sp.(type) {
		case *ast.TypeSpec:
			add(sp.Name.Name, SymType, from, sp.End())
		case *ast.ValueSpec:
			kind := SymVar
			if d.Tok == token.CONST {
				kind = SymConst
			}
			for _, n := range sp.Names {
				add(n.Name, kind, from, sp.End())
			}
		}
	}
}

// recvName is the receiver's base type name: pointers, parentheses and
// type parameter lists stripped.
func recvName(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) != 1 {
		return ""
	}
	e := fl.List[0].Type
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}
