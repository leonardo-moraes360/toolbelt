package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	reRustFunc   = mustRE(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+([A-Za-z_]\w*)\s*[<(]`)
	reRustType   = mustRE(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait|type)\s+([A-Za-z_]\w*)`)
	reRustMod    = mustRE(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?mod\s+([A-Za-z_]\w*)`)
	reRustUse    = mustRE(`(?m)^\s*use\s+([A-Za-z_][\w:]*(?:::\*)?)`)
	reRustCall   = mustRE(`\b([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)\s*\(`)
	rustCallSkip = map[string]bool{
		"if": true, "while": true, "for": true, "loop": true, "match": true,
		"return": true, "break": true, "continue": true, "async": true, "await": true,
		"unsafe": true, "fn": true, "struct": true, "enum": true, "trait": true,
		"impl": true, "mod": true, "use": true, "type": true, "const": true,
		"static": true, "let": true, "mut": true, "ref": true, "where": true,
		"as": true, "in": true, "move": true, "box": true, "pub": true,
		"crate": true, "super": true, "self": true, "Self": true,
		"true": true, "false": true, "Some": true, "None": true, "Ok": true, "Err": true,
		"vec": true, "Vec": true, "String": true, "str": true, "format": true,
		"print": true, "println": true, "eprint": true, "eprintln": true, "dbg": true,
		"panic": true, "todo": true, "unimplemented": true, "unreachable": true,
		"assert": true, "assert_eq": true, "assert_ne": true, "debug_assert": true,
		"drop": true, "clone": true, "into": true, "from": true, "default": true,
		"new": true, "len": true, "is_empty": true, "unwrap": true, "expect": true,
		"ok": true, "err": true, "map": true, "and_then": true, "or_else": true,
	}
)

func init() {
	RegisterIndexer(RustIndexer{})
}

type RustIndexer struct{}

func (RustIndexer) Name() string     { return "rust" }
func (RustIndexer) Category() string { return CategoryLanguage }

func (RustIndexer) Match(path string) bool {
	if strings.ToLower(filepath.Ext(path)) != ".rs" {
		return false
	}
	return !skipIndexedTestFile(path)
}

func (RustIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	full := filepath.Join(root, rel)
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, nil, err
	}
	raw := string(data)
	code := stripRustNoise(raw)
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.85)

	funcIDs := map[string]string{}
	funcs := uniqueMatches(reRustFunc, code)
	for _, name := range funcs {
		funcIDs[name] = "func:" + modName + "." + name
	}
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, funcs, NodeFunction, "func", 0.8)

	types := uniqueMatchesMulti([]*regexp.Regexp{reRustType, reRustMod}, code)
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, types, NodeClass, "class", 0.8)

	imports := uniqueMatches(reRustUse, raw)
	nodes, edges = appendImportEdges(nodes, edges, fileID, imports, 0.85)
	edges = append(edges, extractRustCalls(fileID, funcIDs, code)...)

	return nodes, edges, nil
}

func extractRustCalls(fileID string, funcIDs map[string]string, src string) []Edge {
	spans := rustFunctionSpans(src, funcIDs)
	seen := map[string]bool{}
	var edges []Edge
	for _, m := range reRustCall.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 4 {
			continue
		}
		raw := src[m[2]:m[3]]
		callee := rustCalleeName(raw)
		if callee == "" || rustCallSkip[callee] {
			continue
		}
		base := raw
		if i := strings.Index(raw, "::"); i >= 0 {
			base = raw[:i]
		}
		if rustCallSkip[base] && !strings.Contains(raw, "::") && !strings.Contains(raw, ".") {
			continue
		}
		at := m[0]
		if rustIsDefSite(spans, at, callee) {
			continue
		}
		sourceID := rustCallerAt(spans, fileID, at)
		targetID := ""
		conf := 0.7
		if id, ok := funcIDs[callee]; ok {
			if id == sourceID {
				continue
			}
			targetID = id
		} else {
			targetID = "sym:" + callee
			conf = 0.55
		}
		key := sourceID + "->" + targetID
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, Edge{
			SourceID: sourceID, TargetID: targetID, Type: EdgeCalls,
			Source: "extracted", Confidence: conf,
		})
	}
	return edges
}

type rustSpan struct {
	name      string
	id        string
	start     int
	bodyStart int
	end       int
}

func rustCallerAt(spans []rustSpan, fileID string, at int) string {
	caller := fileID
	bestStart := -1
	for _, sp := range spans {
		if at >= sp.bodyStart && at < sp.end && sp.start >= bestStart {
			bestStart = sp.start
			caller = sp.id
		}
	}
	return caller
}

func rustIsDefSite(spans []rustSpan, at int, callee string) bool {
	for _, sp := range spans {
		if at >= sp.start && at < sp.bodyStart && sp.name == callee {
			return true
		}
	}
	return false
}

func rustFunctionSpans(src string, funcIDs map[string]string) []rustSpan {
	var spans []rustSpan
	for _, m := range reRustFunc.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 4 {
			continue
		}
		name := src[m[2]:m[3]]
		id, ok := funcIDs[name]
		if !ok {
			id = "func:" + name
		}
		sigStart := m[0]
		bodyStart, end, ok := rustFindBody(src, m[1])
		if !ok {
			continue
		}
		spans = append(spans, rustSpan{
			name: name, id: id,
			start: sigStart, bodyStart: bodyStart, end: end,
		})
	}
	return spans
}

func rustFindBody(src string, afterSig int) (bodyStart, end int, ok bool) {
	i := afterSig
	for i < len(src) {
		c := src[i]
		if c == '{' {
			bodyStart = i + 1
			end, ok = rustMatchBrace(src, i)
			return bodyStart, end, ok
		}
		if c == ';' {
			return 0, 0, false
		}
		i++
	}
	return 0, 0, false
}

func rustMatchBrace(src string, open int) (int, bool) {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func rustCalleeName(raw string) string {
	if i := strings.LastIndex(raw, "::"); i >= 0 {
		return raw[i+2:]
	}
	if i := strings.LastIndexByte(raw, '.'); i >= 0 {
		return raw[i+1:]
	}
	return raw
}

func stripRustNoise(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	i := 0
	for i < len(src) {
		switch {
		case rustStartsLineComment(src, i):
			i = rustSkipLineComment(src, i)
		case rustStartsBlockComment(src, i):
			i = rustSkipBlockComment(&b, src, i)
		case src[i] == '"' || src[i] == '\'':
			i = writeRustStringPlaceholder(&b, src, i)
		default:
			b.WriteByte(src[i])
			i++
		}
	}
	return b.String()
}

func rustStartsLineComment(src string, i int) bool {
	return i+1 < len(src) && src[i] == '/' && src[i+1] == '/'
}

func rustStartsBlockComment(src string, i int) bool {
	return i+1 < len(src) && src[i] == '/' && src[i+1] == '*'
}

func rustSkipLineComment(src string, i int) int {
	for i < len(src) && src[i] != '\n' {
		i++
	}
	return i
}

func rustSkipBlockComment(b *strings.Builder, src string, i int) int {
	i += 2
	for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
		writeRustPlaceholderByte(b, src[i])
		i++
	}
	if i+1 < len(src) {
		i += 2
	}
	return i
}

func writeRustStringPlaceholder(b *strings.Builder, src string, i int) int {
	quote := src[i]
	if quote == '"' && i+1 < len(src) && src[i+1] == '"' && i+2 < len(src) && src[i+2] == '"' {
		b.WriteString("   ")
		i += 3
		for i+2 < len(src) {
			if src[i] == '"' && src[i+1] == '"' && src[i+2] == '"' {
				b.WriteString("   ")
				return i + 3
			}
			writeRustPlaceholderByte(b, src[i])
			i++
		}
		return i
	}
	b.WriteByte(' ')
	i++
	for i < len(src) {
		if src[i] == '\\' && i+1 < len(src) {
			b.WriteString("  ")
			i += 2
			continue
		}
		if src[i] == quote {
			b.WriteByte(' ')
			return i + 1
		}
		writeRustPlaceholderByte(b, src[i])
		i++
	}
	return i
}

func writeRustPlaceholderByte(b *strings.Builder, c byte) {
	if c == '\n' {
		b.WriteByte('\n')
		return
	}
	if unicode.IsSpace(rune(c)) {
		b.WriteByte(c)
		return
	}
	b.WriteByte(' ')
}
