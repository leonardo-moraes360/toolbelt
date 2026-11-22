package graph

import "testing"

const symSrc = `package p

// Doc comment.
func Top() int { return 1 }

type T[K any] struct{ k K }

func (t *T[K]) M() {}

func (T[K]) V() {}

type (
	A int
	B string
)

const C = 1

const (
	D, E = 1, 2
)

var _ = 0

func init() {}

func Dup() {}
func Dup() {}
`

func TestGoSymbols(t *testing.T) {
	syms, err := GoSymbols([]byte(symSrc))
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Symbol{}
	for _, s := range syms {
		if _, ok := by[s.Name]; !ok {
			by[s.Name] = s
		}
	}
	for _, c := range []struct {
		name, kind string
		start, end int
		amb        bool
	}{
		{"Top", SymFunc, 4, 4, false}, {"T", SymType, 6, 6, false}, {"T.M", SymMethod, 8, 8, false},
		{"T.V", SymMethod, 10, 10, false}, {"A", SymType, 13, 13, false}, {"B", SymType, 14, 14, false},
		{"C", SymConst, 17, 17, false}, {"D", SymConst, 20, 20, false}, {"E", SymConst, 20, 20, false},
		{"_", SymVar, 23, 23, true}, {"init", SymFunc, 25, 25, true}, {"Dup", SymFunc, 27, 27, true},
	} {
		g, ok := by[c.name]
		if !ok || g.Kind != c.kind || g.StartLine != c.start || g.EndLine != c.end || g.Ambiguous != c.amb || len(g.SHA256) != 64 {
			t.Errorf("%s: got %+v ok=%v", c.name, g, ok)
		}
	}
}

func TestGoSymbolsHashIgnoresDocAndOutside(t *testing.T) {
	a, _ := GoSymbols([]byte("package p\n\n// one\nfunc F() { x() }\n\nfunc G() {}\n"))
	b, _ := GoSymbols([]byte("package p\n\n// two, longer\n// comment\nfunc F() { x() }\n\nfunc G() { y() }\n"))
	if a[0].SHA256 != b[0].SHA256 {
		t.Error("a doc-comment edit changed the hash")
	}
	if a[1].SHA256 == b[1].SHA256 {
		t.Error("a body edit kept the hash")
	}
}

func TestGoSymbolsParseError(t *testing.T) {
	if s, err := GoSymbols([]byte("package p\nfunc (")); err == nil || s != nil {
		t.Errorf("want error and no symbols, got %v %v", s, err)
	}
}

func TestGoSymbolsLineDirective(t *testing.T) {
	s, err := GoSymbols([]byte("package p\n//line x.go:100\nfunc F() {}\n"))
	if err != nil || s[0].StartLine != 3 {
		t.Errorf("got %+v %v", s, err)
	}
}
