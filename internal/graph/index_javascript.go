package graph

import (
	"os"
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
	reJSFuncDecl   = regexp.MustCompile(`(?m)(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(`)
	reJSFuncAssign = regexp.MustCompile(`(?m)(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:function\b|\()`)
	reJSCall       = regexp.MustCompile(`\b([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)?)\s*\(`)
	jsCallKeywords = map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"function": true, "return": true, "typeof": true, "new": true, "await": true,
		"import": true, "export": true, "class": true, "super": true,
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
	if strings.HasSuffix(strings.ToLower(path), ".d.ts") {
		return false
	}
	return !skipIndexedTestFile(path)
}

func (JSIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	full := filepath.Join(root, rel)
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, nil, err
	}
	raw := string(data)
	code := stripJSComments(raw)
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.9)

	funcIDs, fnNodes, fnEdges := extractJSFunctions(modName, filepath.ToSlash(rel), fileID, code)
	nodes = append(nodes, fnNodes...)
	edges = append(edges, fnEdges...)

	impNodes, impEdges := extractJSImports(fileID, raw)
	nodes = append(nodes, impNodes...)
	edges = append(edges, impEdges...)
	edges = append(edges, extractJSCalls(modName, fileID, funcIDs, code)...)

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

func extractJSImports(fileID, raw string) ([]Node, []Edge) {
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
		target := "pkg:" + imp
		nodes = append(nodes, Node{
			ID: target, Type: NodePackage, Title: imp, Source: "extracted", Confidence: 0.9,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: target, Type: EdgeImports, Source: "extracted", Confidence: 0.9,
		})
	}
	return nodes, edges
}

func extractJSCalls(modName, fileID string, funcIDs map[string]string, src string) []Edge {
	seen := map[string]bool{}
	var edges []Edge
	for _, m := range reJSCall.FindAllStringSubmatch(src, -1) {
		if len(m) < 2 {
			continue
		}
		callee := m[1]
		base := callee
		if i := strings.IndexByte(callee, '.'); i >= 0 {
			base = callee[:i]
		}
		if jsCallKeywords[base] || jsCallKeywords[callee] {
			continue
		}
		targetID := "func:" + modName + "." + callee
		if id, ok := funcIDs[callee]; ok {
			targetID = id
		} else if strings.Contains(callee, ".") {
			targetID = "func:" + callee
		}
		key := fileID + "->" + targetID
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: targetID, Type: EdgeCalls,
			Source: "extracted", Confidence: 0.55,
		})
	}
	return edges
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
