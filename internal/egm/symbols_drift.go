package egm

import (
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// symbolKeys returns m's keys sorted.
func symbolKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// splitSymbolKey splits "path#Name".
func splitSymbolKey(k string) (p, name string, ok bool) {
	i := strings.Index(k, "#")
	if i <= 0 || !symbolFrag(k[i+1:]) {
		return "", "", false
	}
	return k[:i], k[i+1:], true
}

// symScopes maps a symbol-scoped source path to its baseline symbols (name →
// sha256). A path is symbol-scoped when the document cites it only through
// symbol citations and the baseline holds every one of them; anything else
// (a plain citation, a v1/v2 receipt, a round with no symbol baseline) is
// file-scoped, as before C5.
type symScopes map[string]map[string]string

func symbolScopes(docRel, text string, set *Settings, syms map[string]string) symScopes {
	out := symScopes{}
	if len(syms) == 0 {
		return out
	}
	cites, plain := docCitesWith(docRel, text, set)
	byPath := map[string][]SymbolCite{}
	for _, c := range cites {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	for p, cs := range byPath {
		if plain[p] {
			continue
		}
		m := map[string]string{}
		for _, c := range cs {
			h, ok := syms[c.Key()]
			if !ok {
				m = nil // a cited symbol without a baseline: file-scoped
				break
			}
			m[c.Name] = h
		}
		if m != nil {
			out[p] = m
		}
	}
	return out
}

// compareSymbols is R20 for one symbol-scoped file whose bytes (b, nil when
// the blob is over the drift read bound) differ from the baseline's: each
// baseline symbol is extracted and its hash compared. A changed file whose
// cited symbols are all unchanged is listed in OutsideSymbols, not drift.
func compareSymbols(p string, b []byte, maxSource int64, want map[string]string, dd *DriftDoc) {
	names := symbolKeys(want)
	missing := func(reason string) {
		for _, n := range names {
			dd.SymbolsMissing = append(dd.SymbolsMissing, MissingSymbol{Path: p, Symbol: n, Reason: reason})
		}
	}
	if b == nil || int64(len(b)) > maxSource {
		missing("too_large")
		return
	}
	syms, err := graph.GoSymbols(b)
	if err != nil {
		missing("unparsed")
		return
	}
	clean := true
	for _, n := range names {
		sym, got := findSymbol(syms, n)
		switch {
		case got == symAbsent:
			dd.SymbolsMissing = append(dd.SymbolsMissing, MissingSymbol{Path: p, Symbol: n, Reason: "deleted"})
			clean = false
		case got == symAmbiguous:
			dd.SymbolsMissing = append(dd.SymbolsMissing, MissingSymbol{Path: p, Symbol: n, Reason: "ambiguous"})
			clean = false
		case sym.SHA256 != want[n]:
			dd.SymbolsChanged = append(dd.SymbolsChanged, SymbolRef{Path: p, Symbol: n})
			clean = false
		}
	}
	if clean {
		dd.OutsideSymbols = append(dd.OutsideSymbols, p)
	}
}
