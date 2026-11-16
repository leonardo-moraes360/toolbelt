package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	rePyFunc       = mustRE(`(?m)^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	rePyClass      = mustRE(`(?m)^\s*class\s+([A-Za-z_]\w*)\s*[:(]`)
	rePyImportName = mustRE(`(?m)^\s*import\s+([A-Za-z_][\w.]*)`)
	rePyImportFrom = mustRE(`(?m)^\s*from\s+([A-Za-z_][\w.]*)\s+import\s+`)
	rePyCall       = mustRE(`\b([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)\s*\(`)
	rePyDefHeader  = mustRE(`(?m)^([ \t]*)(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pyCallSkip     = map[string]bool{
		"if": true, "elif": true, "for": true, "while": true, "with": true,
		"except": true, "assert": true, "return": true, "yield": true, "raise": true,
		"del": true, "lambda": true, "await": true, "async": true,
		"def": true, "class": true, "not": true, "and": true, "or": true,
		"in": true, "is": true, "match": true, "case": true,
		"print": true, "len": true, "str": true, "int": true, "float": true,
		"bool": true, "list": true, "dict": true, "set": true, "tuple": true,
		"type": true, "isinstance": true, "issubclass": true, "super": true,
		"property": true, "classmethod": true, "staticmethod": true,
		"enumerate": true, "range": true, "iter": true, "next": true,
		"open": true, "getattr": true, "setattr": true, "hasattr": true,
		"callable": true, "vars": true, "dir": true, "min": true, "max": true,
		"sum": true, "abs": true, "round": true, "sorted": true, "reversed": true,
		"zip": true, "map": true, "filter": true, "any": true, "all": true,
		"format": true, "repr": true, "hash": true, "id": true, "input": true,
		"bytes": true, "bytearray": true, "frozenset": true, "object": true,
		"Exception": true, "ValueError": true, "TypeError": true, "KeyError": true,
	}
)

func init() {
	RegisterIndexer(PythonIndexer{})
}

type PythonIndexer struct{}

func (PythonIndexer) Name() string     { return "python" }
func (PythonIndexer) Category() string { return CategoryLanguage }

func (PythonIndexer) Match(path string) bool {
	if strings.ToLower(filepath.Ext(path)) != ".py" {
		return false
	}
	return !skipIndexedTestFile(path)
}

func (PythonIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	full := filepath.Join(root, rel)
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, nil, err
	}
	raw := string(data)
	code := stripPyNoise(raw)
	modName := pythonModuleName(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.85)

	funcIDs := map[string]string{}
	funcs := uniqueMatches(rePyFunc, code)
	for _, name := range funcs {
		funcIDs[name] = "func:" + modName + "." + name
	}
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, funcs, NodeFunction, "func", 0.8)

	classes := uniqueMatches(rePyClass, code)
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, classes, NodeClass, "class", 0.8)

	imports := uniqueMatchesMulti([]*regexp.Regexp{rePyImportName, rePyImportFrom}, raw)
	nodes, edges = appendImportEdges(nodes, edges, fileID, imports, 0.85)
	edges = append(edges, extractPyCalls(fileID, funcIDs, code)...)

	return nodes, edges, nil
}

func extractPyCalls(fileID string, funcIDs map[string]string, src string) []Edge {
	spans := pyFunctionSpans(src, funcIDs)
	seen := map[string]bool{}
	var edges []Edge
	for _, m := range rePyCall.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 4 {
			continue
		}
		raw := src[m[2]:m[3]]
		callee := pyCalleeName(raw)
		if callee == "" || pyCallSkip[callee] {
			continue
		}
		base := raw
		if i := strings.IndexByte(raw, '.'); i >= 0 {
			base = raw[:i]
		}
		if pyCallSkip[base] && !strings.Contains(raw, ".") {
			continue
		}
		at := m[0]
		if pyIsDefSite(spans, at, callee) {
			continue
		}
		sourceID := pyCallerAt(spans, fileID, at)
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

type pySpan struct {
	name      string
	id        string
	start     int
	bodyStart int
	end       int
}

func pyCallerAt(spans []pySpan, fileID string, at int) string {
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

func pyIsDefSite(spans []pySpan, at int, callee string) bool {
	for _, sp := range spans {
		if at >= sp.start && at < sp.bodyStart && sp.name == callee {
			return true
		}
	}
	return false
}

func pyFunctionSpans(src string, funcIDs map[string]string) []pySpan {
	type pending struct {
		name      string
		id        string
		indent    int
		start     int
		bodyStart int
	}
	var stack []pending
	var spans []pySpan
	offset := 0
	lines := strings.Split(src, "\n")
	for li, line := range lines {
		lineStart := offset
		indent := pyLeadingIndent(line)
		trimmed := strings.TrimSpace(line)
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				break
			}
			if indent <= top.indent {
				spans = append(spans, pySpan{
					name: top.name, id: top.id,
					start: top.start, bodyStart: top.bodyStart, end: lineStart,
				})
				stack = stack[:len(stack)-1]
				continue
			}
			break
		}
		if m := rePyDefHeader.FindStringSubmatchIndex(line); m != nil {
			name := line[m[4]:m[5]]
			id, ok := funcIDs[name]
			if !ok {
				id = "func:" + name
			}
			bodyStart := lineStart + len(line)
			if li+1 < len(lines) {
				bodyStart++
			}
			stack = append(stack, pending{
				name: name, id: id, indent: pyLeadingIndent(line),
				start: lineStart, bodyStart: bodyStart,
			})
		}
		offset += len(line)
		if li+1 < len(lines) {
			offset++
		}
	}
	end := len(src)
	for i := len(stack) - 1; i >= 0; i-- {
		top := stack[i]
		spans = append(spans, pySpan{
			name: top.name, id: top.id,
			start: top.start, bodyStart: top.bodyStart, end: end,
		})
	}
	return spans
}

func pyLeadingIndent(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' {
			n++
			continue
		}
		if r == '\t' {
			n += 4
			continue
		}
		break
	}
	return n
}

func pyCalleeName(raw string) string {
	if i := strings.LastIndexByte(raw, '.'); i >= 0 {
		return raw[i+1:]
	}
	return raw
}

func pythonModuleName(rel string) string {
	slash := filepath.ToSlash(rel)
	mod := strings.TrimSuffix(slash, filepath.Ext(slash))
	mod = strings.ReplaceAll(mod, "/", ".")
	if strings.HasSuffix(mod, ".__init__") {
		mod = strings.TrimSuffix(mod, ".__init__")
	}
	if mod == "" || mod == "." {
		return strings.TrimSuffix(filepath.Base(slash), filepath.Ext(slash))
	}
	return mod
}

func stripPyNoise(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	i := 0
	for i < len(src) {
		switch {
		case src[i] == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '"' || src[i] == '\'':
			i = writePyStringPlaceholder(&b, src, i)
		default:
			b.WriteByte(src[i])
			i++
		}
	}
	return b.String()
}

func writePyStringPlaceholder(b *strings.Builder, src string, i int) int {
	quote := src[i]
	if i+2 < len(src) && src[i+1] == quote && src[i+2] == quote {
		b.WriteString("   ")
		i += 3
		for i+2 < len(src) {
			if src[i] == quote && src[i+1] == quote && src[i+2] == quote {
				b.WriteString("   ")
				return i + 3
			}
			writePyPlaceholderByte(b, src[i])
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
		writePyPlaceholderByte(b, src[i])
		i++
	}
	return i
}

func writePyPlaceholderByte(b *strings.Builder, c byte) {
	if c == '\n' {
		b.WriteByte('\n')
		return
	}
	b.WriteByte(' ')
}
