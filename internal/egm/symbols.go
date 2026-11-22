package egm

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// Symbol citations (docs/specs/egm-phase-c.md, C4): a Go source may be cited
// as path.go#Name or path.go#Recv.Name. The symbol is identified by the file
// plus the name inside it, never by a graph ID, and verified on the
// snapshot's bytes with a pure parser: no graph database, no LLM.

var (
	reSymFrag    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)
	reLineAnchor = regexp.MustCompile(`^L\d+(-L\d+)?$`)
)

// symbolFrag reports whether f is a symbol fragment: an identifier (or
// Recv.Name) that is not a line anchor such as L10 or L10-L20.
func symbolFrag(f string) bool { return reSymFrag.MatchString(f) && !reLineAnchor.MatchString(f) }

// isGoSymbolSpan: a backtick span like `internal/x/x.go#Name`, which the
// extension rule of SpanCandidates would otherwise reject.
func isGoSymbolSpan(s string) bool {
	i := strings.LastIndex(s, "#")
	return i > 0 && strings.HasSuffix(s[:i], ".go") && symbolFrag(s[i+1:])
}

// SymbolCite is one symbol citation of a document.
type SymbolCite struct {
	Path string
	Name string
	Line int
}

// Key is the citation's identity in receipts and rounds.
func (c SymbolCite) Key() string { return c.Path + "#" + c.Name }

// DocCites lists the symbol citations of a document (links relative to the
// document, spans relative to the root), first line of each, and the .go
// paths it cites plainly (with no symbol fragment): those are file-scoped.
func DocCites(docRel, text string) (cites []SymbolCite, plain map[string]bool) {
	plain = map[string]bool{}
	seen := map[string]bool{}
	visit := func(base, raw string, line int) {
		raw, frag, _ := strings.Cut(raw, "#")
		rel, why := cleanRel(base, raw)
		if why != "" || rel == docRel || !strings.HasSuffix(rel, ".go") {
			return
		}
		if !symbolFrag(frag) {
			plain[rel] = true
			return
		}
		c := SymbolCite{Path: rel, Name: frag, Line: line}
		if !seen[c.Key()] {
			seen[c.Key()] = true
			cites = append(cites, c)
		}
	}
	for i, l := range strings.Split(text, "\n") {
		for _, h := range graph.CollectMDLinks(l) {
			visit(path.Dir(docRel), h, i+1)
		}
		for _, s := range SpanCandidates(l) {
			if !strings.HasSuffix(s, "/") {
				visit("", s, i+1)
			}
		}
	}
	return cites, plain
}

// citedSymbolKeys is the set of "path#Name" the document cites, valid or not.
func citedSymbolKeys(docRel, text string) map[string]bool {
	cites, _ := DocCites(docRel, text)
	out := map[string]bool{}
	for _, c := range cites {
		out[c.Key()] = true
	}
	return out
}

// symbolLookup is the outcome of resolving a name in a file's symbols.
type symbolLookup int

const (
	symFound symbolLookup = iota
	symAbsent
	symAmbiguous
)

func findSymbol(syms []graph.Symbol, name string) (graph.Symbol, symbolLookup) {
	var hit graph.Symbol
	n := 0
	for _, s := range syms {
		if s.Name == name {
			hit, n = s, n+1
			if s.Ambiguous {
				return s, symAmbiguous
			}
		}
	}
	if n == 0 {
		return hit, symAbsent
	}
	return hit, symFound
}

// checkSymbols is R16 on the snapshot's sources: each symbol citation whose
// file is an included source must name one declaration of that file. It adds
// its findings to res (so the refs checker's lineages cover them) and
// returns the hashes of the resolved, unambiguous citations (R17).
func checkSymbols(d *Doc, docRel, text string, sources []Source, res *CheckResult) map[string]string {
	out := map[string]string{}
	cites, _ := DocCites(docRel, text)
	raw := map[string][]byte{}
	for _, s := range sources {
		if s.Raw != nil {
			raw[s.Path] = s.Raw
		}
	}
	parsed := map[string][]graph.Symbol{}
	broken := map[string]bool{}
	for _, c := range cites {
		b, ok := raw[c.Path]
		if !ok {
			continue
		}
		syms, done := parsed[c.Path]
		if !done && !broken[c.Path] {
			var err error
			if syms, err = graph.GoSymbols(b); err != nil {
				broken[c.Path] = true
				res.Findings = append(res.Findings, symbolFinding(d, docRel, c, SevNit, "symbol_unparsed:"+c.Path,
					fmt.Sprintf("%#q does not parse, so its symbol citations cannot be verified", c.Path)))
				res.inputs = append(res.inputs, "symbol\t"+c.Path+"\tunparsed")
				continue
			}
			parsed[c.Path] = syms
		}
		if broken[c.Path] {
			continue
		}
		sym, got := findSymbol(syms, c.Name)
		switch got {
		case symAbsent:
			res.Findings = append(res.Findings, symbolFinding(d, docRel, c, SevMajor, "refs:symbol:"+c.Key(),
				fmt.Sprintf("symbol %#q not found in %#q", c.Name, c.Path)))
			res.inputs = append(res.inputs, "symbol\t"+c.Key()+"\tmissing")
		case symAmbiguous:
			res.Findings = append(res.Findings, symbolFinding(d, docRel, c, SevMajor, "symbol_ambiguous:"+c.Key(),
				fmt.Sprintf("symbol_ambiguous: %#q is declared more than once in %#q (or is _ or init), so it cannot be cited", c.Name, c.Path)))
			res.inputs = append(res.inputs, "symbol\t"+c.Key()+"\tambiguous")
		default:
			out[c.Key()] = sym.SHA256
			res.inputs = append(res.inputs, "symbol\t"+c.Key()+"\t"+sym.SHA256)
		}
	}
	res.Findings = dedupeByKey(res.Findings)
	return out
}

func symbolFinding(d *Doc, docRel string, c SymbolCite, sev, key, msg string) Finding {
	return Finding{
		Reviewer: checkRefs, Severity: sev,
		Scope:    Scope{Heading: d.HeadingAt(c.Line), Lines: []int{c.Line, c.Line}},
		Message:  msg,
		Evidence: []Evidence{{Path: docRel, Line: c.Line}},
		Key:      key,
	}
}
