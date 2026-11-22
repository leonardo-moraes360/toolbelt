package graph

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	jsExts = map[string]bool{
		".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	}
	reJSImportFrom = regexp.MustCompile(`(?m)(?:import|export)(?:[\s\w\{\}\*,]*)\s+from\s+['"]([^'"]+)['"]`)
	reJSImportSide = regexp.MustCompile(`(?m)^\s*import\s+['"]([^'"]+)['"]`)
	reJSRequire    = regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`)
	reJSFuncDecl   = regexp.MustCompile(`(?m)(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*(?:<[^(){]*?>)?\s*\(`)
	reJSFuncAssign = regexp.MustCompile(`(?m)(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::\s*[^=]+)?=\s*(?:async\s*)?(?:function\b|\(|<[A-Za-z_$])`)
	reJSCall       = regexp.MustCompile(`\b([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)?)\s*\(`)
	reJSXCallTag   = regexp.MustCompile(`<([A-Z][A-Za-z0-9_]*)\b`)
	jsCallKeywords = map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"function": true, "return": true, "typeof": true, "new": true, "await": true,
		"import": true, "export": true, "class": true, "super": true, "void": true,
		"delete": true, "throw": true, "yield": true, "instanceof": true,
	}
)

func init() {
	RegisterIndexer(JSIndexer{})
}

type JSIndexer struct{}

func (JSIndexer) Name() string     { return "javascript" }
func (JSIndexer) Category() string { return CategoryLanguage }

func (JSIndexer) Match(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if !jsExts[ext] {
		return false
	}
	return !strings.HasSuffix(strings.ToLower(path), ".d.ts")
}

func (JSIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	data, err := readSourceBytes(root, rel)
	if err != nil {
		return nil, nil, err
	}
	raw := string(data)
	code := stripJSComments(raw)
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.9)
	cfg := loadJSPathConfig(root)

	funcIDs, fnNodes, fnEdges := extractJSFunctions(modName, filepath.ToSlash(rel), fileID, code)
	nodes = append(nodes, fnNodes...)
	edges = append(edges, fnEdges...)

	impNodes, impEdges := extractJSImports(root, rel, fileID, raw, cfg)
	nodes = append(nodes, impNodes...)
	edges = append(edges, impEdges...)
	bindings := extractJSBindings(root, rel, raw, cfg)
	edges = append(edges, extractJSCalls(fileID, funcIDs, code, bindings)...)

	return nodes, edges, nil
}

func extractJSFunctions(modName, relPath, fileID, src string) (map[string]string, []Node, []Edge) {
	funcIDs := map[string]string{}
	var nodes []Node
	var edges []Edge
	add := func(name string, conf float64) {
		if _, ok := funcIDs[name]; ok {
			return
		}
		id := "func:" + modName + "." + name
		funcIDs[name] = id
		nodes = append(nodes, Node{
			ID: id, Type: NodeFunction, Path: relPath,
			Title: name, Package: modName, Source: "extracted", Confidence: conf,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: id, Type: EdgeDefines, Source: "extracted", Confidence: conf,
		})
	}
	for _, name := range uniqueMatches(reJSFuncDecl, src) {
		add(name, 0.85)
	}
	for _, name := range uniqueMatches(reJSFuncAssign, src) {
		add(name, 0.8)
	}
	return funcIDs, nodes, edges
}

func extractJSImports(root, rel, fileID, raw string, cfg *jsPathConfig) ([]Node, []Edge) {
	imports := map[string]bool{}
	for _, re := range []*regexp.Regexp{reJSImportFrom, reJSImportSide, reJSRequire} {
		for _, m := range re.FindAllStringSubmatch(raw, -1) {
			if len(m) >= 2 && m[1] != "" {
				imports[m[1]] = true
			}
		}
	}
	var nodes []Node
	var edges []Edge
	for imp := range imports {
		nodes, edges = appendResolvedJSImport(nodes, edges, root, rel, fileID, imp, cfg, 0.9)
	}
	return nodes, edges
}

func appendResolvedJSImport(nodes []Node, edges []Edge, root, rel, fileID, imp string, cfg *jsPathConfig, conf float64) ([]Node, []Edge) {
	if resolved, ok := resolveJSImport(root, rel, imp, cfg); ok {
		target := "file:" + resolved
		nodes = append(nodes, Node{
			ID: target, Type: NodeFile, Path: resolved, Title: filepath.Base(resolved),
			Package: moduleNameFromRel(resolved), Source: "extracted", Confidence: conf,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: target, Type: EdgeImports, Source: "extracted", Confidence: conf,
		})
		return nodes, edges
	}
	target := "pkg:" + imp
	nodes = append(nodes, Node{
		ID: target, Type: NodePackage, Title: imp, Source: "extracted", Confidence: conf,
	})
	edges = append(edges, Edge{
		SourceID: fileID, TargetID: target, Type: EdgeImports, Source: "extracted", Confidence: conf,
	})
	return nodes, edges
}

type jsCallSink struct {
	seen  map[string]bool
	edges []Edge
}

func (s *jsCallSink) add(sourceID, targetID string, conf float64) {
	if sourceID == targetID {
		return
	}
	key := sourceID + "->" + targetID
	if s.seen[key] {
		return
	}
	s.seen[key] = true
	s.edges = append(s.edges, Edge{
		SourceID: sourceID, TargetID: targetID, Type: EdgeCalls,
		Source: "extracted", Confidence: conf,
	})
}

func (s *jsCallSink) addCallee(sourceID, callee string, funcIDs map[string]string) {
	if id, ok := funcIDs[callee]; ok {
		s.add(sourceID, id, 0.7)
		return
	}
	s.add(sourceID, "sym:"+callee, 0.55)
}

func (s *jsCallSink) addBoundCallee(sourceID, raw string, funcIDs map[string]string, bindings []jsBinding) {
	if exportName, module, ok := resolveJSBoundCallee(raw, bindings); ok {
		if id, ok := funcIDs[exportName]; ok {
			s.add(sourceID, id, 0.7)
			return
		}
		sym := "sym:" + exportName
		if module != "" {
			sym = "sym:" + exportName + "@" + module
		}
		s.add(sourceID, sym, 0.55)
		return
	}
	s.addCallee(sourceID, jsCalleeName(raw), funcIDs)
}

func extractJSCalls(fileID string, funcIDs map[string]string, src string, bindings []jsBinding) []Edge {
	spans := jsFunctionSpans(src, funcIDs)
	sink := &jsCallSink{seen: map[string]bool{}}
	appendJSCallExprs(sink, fileID, funcIDs, src, spans, bindings)
	appendJSXCallEdges(sink, fileID, funcIDs, src, spans, bindings)
	return sink.edges
}

func appendJSCallExprs(sink *jsCallSink, fileID string, funcIDs map[string]string, src string, spans []jsSpan, bindings []jsBinding) {
	for _, m := range reJSCall.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 4 {
			continue
		}
		raw := src[m[2]:m[3]]
		callee := jsCalleeName(raw)
		if !jsCalleeAllowed(raw, callee) {
			continue
		}
		at := m[0]
		if jsIsDefSite(spans, at, callee) {
			continue
		}
		sink.addBoundCallee(jsCallerAt(spans, fileID, at), raw, funcIDs, bindings)
	}
}

func jsCalleeAllowed(raw, callee string) bool {
	if callee == "" || jsCallKeywords[callee] {
		return false
	}
	base := raw
	if i := strings.IndexByte(raw, '.'); i >= 0 {
		base = raw[:i]
	}
	if jsCallKeywords[base] && !strings.Contains(raw, ".") {
		return false
	}
	return true
}

func appendJSXCallEdges(sink *jsCallSink, fileID string, funcIDs map[string]string, src string, spans []jsSpan, bindings []jsBinding) {
	for _, m := range reJSXCallTag.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 4 {
			continue
		}
		name := src[m[2]:m[3]]
		if name == "" {
			continue
		}
		sink.addBoundCallee(jsCallerAt(spans, fileID, m[0]), name, funcIDs, bindings)
	}
}

type jsSpan struct {
	name      string
	id        string
	start     int
	bodyStart int
	end       int
}

func jsCallerAt(spans []jsSpan, fileID string, at int) string {
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

func jsIsDefSite(spans []jsSpan, at int, callee string) bool {
	for _, sp := range spans {
		if at >= sp.start && at < sp.bodyStart && sp.name == callee {
			return true
		}
	}
	return false
}

func jsFunctionSpans(src string, funcIDs map[string]string) []jsSpan {
	var spans []jsSpan
	for _, re := range []*regexp.Regexp{reJSFuncDecl, reJSFuncAssign} {
		for _, m := range re.FindAllStringSubmatchIndex(src, -1) {
			if len(m) < 4 {
				continue
			}
			name := src[m[2]:m[3]]
			id, ok := funcIDs[name]
			if !ok {
				continue
			}
			bodyStart, end, ok := jsFindBody(src, m[1])
			if !ok {
				continue
			}
			spans = append(spans, jsSpan{
				name: name, id: id,
				start: m[0], bodyStart: bodyStart, end: end,
			})
		}
	}
	return spans
}

func jsFindBody(src string, afterSig int) (bodyStart, end int, ok bool) {
	i := afterSig
	if afterSig > 0 && src[afterSig-1] == '(' {
		endParen, okParen := jsMatchParen(src, afterSig-1)
		if !okParen {
			return 0, 0, false
		}
		i = endParen + 1
	}
	for i < len(src) {
		c := src[i]
		switch {
		case c == '(':
			endParen, okParen := jsMatchParen(src, i)
			if !okParen {
				return 0, 0, false
			}
			i = endParen + 1
		case c == ':':
			i = jsSkipTSType(src, i+1)
		case c == '{':
			bodyStart = i + 1
			end, ok = jsMatchBrace(src, i)
			return bodyStart, end, ok
		case c == '=' && i+1 < len(src) && src[i+1] == '>':
			return jsArrowBody(src, i+2)
		case c == ';':
			return 0, 0, false
		default:
			i++
		}
	}
	return 0, 0, false
}

func jsArrowBody(src string, i int) (bodyStart, end int, ok bool) {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i++
	}
	if i < len(src) && src[i] == '{' {
		bodyStart = i + 1
		end, ok = jsMatchBrace(src, i)
		return bodyStart, end, ok
	}
	bodyStart = i
	end = jsExprEnd(src, i)
	return bodyStart, end, end > bodyStart
}

type jsTypeDepth struct {
	angle, brace, paren, bracket int
}

func (d *jsTypeDepth) zero() bool {
	return d.angle|d.brace|d.paren|d.bracket == 0
}

func (d *jsTypeDepth) feed(c byte) {
	switch c {
	case '<':
		d.angle++
	case '>':
		if d.angle > 0 {
			d.angle--
		}
	case '{':
		d.brace++
	case '}':
		if d.brace > 0 {
			d.brace--
		}
	case '(':
		d.paren++
	case ')':
		if d.paren > 0 {
			d.paren--
		}
	case '[':
		d.bracket++
	case ']':
		if d.bracket > 0 {
			d.bracket--
		}
	}
}

func jsSkipTSType(src string, i int) int {
	var depth jsTypeDepth
	for i < len(src) {
		if depth.zero() && jsTypeBodyStart(src, i) {
			return i
		}
		depth.feed(src[i])
		i++
	}
	return i
}

func jsTypeBodyStart(src string, i int) bool {
	if src[i] == '{' {
		return true
	}
	return src[i] == '=' && i+1 < len(src) && src[i+1] == '>'
}

func jsMatchParen(src string, open int) (int, bool) {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func jsMatchBrace(src string, open int) (int, bool) {
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

func jsExprEnd(src string, start int) int {
	depthParen, depthBrace, depthBracket := 0, 0, 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		case '[':
			depthBracket++
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
		case ',', ';', '\n':
			if depthParen == 0 && depthBrace == 0 && depthBracket == 0 {
				return i
			}
		}
	}
	return len(src)
}

func jsCalleeName(raw string) string {
	if i := strings.LastIndexByte(raw, '.'); i >= 0 {
		return raw[i+1:]
	}
	return raw
}

func stripJSComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	i := 0
	for i < len(src) {
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			if i+1 < len(src) {
				i += 2
			}
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}
