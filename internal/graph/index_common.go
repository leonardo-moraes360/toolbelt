package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func moduleNameFromRel(rel string) string {
	slash := filepath.ToSlash(rel)
	dir := filepath.ToSlash(filepath.Dir(slash))
	if dir == "." || dir == "" {
		return strings.TrimSuffix(filepath.Base(slash), filepath.Ext(slash))
	}
	return dir
}

func uniqueMatches(re *regexp.Regexp, src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		if len(m) < 2 || m[1] == "" || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

func uniqueMatchesMulti(res []*regexp.Regexp, src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, re := range res {
		if re == nil {
			continue
		}
		for _, name := range uniqueMatches(re, src) {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func readSource(root, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func baseFileNodes(modName, rel string, conf float64) (nodes []Node, edges []Edge, pkgID, fileID string) {
	pkgID = "pkg:" + modName
	fileID = "file:" + filepath.ToSlash(rel)
	nodes = []Node{
		{ID: pkgID, Type: NodePackage, Title: modName, Package: modName, Source: "extracted", Confidence: conf},
		{ID: fileID, Type: NodeFile, Path: filepath.ToSlash(rel), Title: filepath.Base(rel), Package: modName, Source: "extracted", Confidence: conf},
	}
	edges = []Edge{
		{SourceID: pkgID, TargetID: fileID, Type: EdgeDefines, Source: "extracted", Confidence: conf},
	}
	return nodes, edges, pkgID, fileID
}

func appendNamedNodes(
	nodes []Node,
	edges []Edge,
	fileID, modName, relPath string,
	names []string,
	nodeType NodeType,
	idPrefix string,
	conf float64,
) ([]Node, []Edge) {
	slash := filepath.ToSlash(relPath)
	for _, name := range names {
		id := idPrefix + ":" + modName + "." + name
		nodes = append(nodes, Node{
			ID: id, Type: nodeType, Path: slash,
			Title: name, Package: modName, Source: "extracted", Confidence: conf,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: id, Type: EdgeDefines, Source: "extracted", Confidence: conf,
		})
	}
	return nodes, edges
}

// appendJSImportEdges is appendImportEdges for JS-family sources: relative,
// tsconfig-alias and workspace imports become file: edges, the rest pkg:.
func appendJSImportEdges(nodes []Node, edges []Edge, root, rel, fileID string, imports []string, conf float64) ([]Node, []Edge) {
	cfg := loadJSPathConfig(root)
	for _, imp := range imports {
		nodes, edges = appendResolvedJSImport(nodes, edges, root, rel, fileID, imp, cfg, conf)
	}
	return nodes, edges
}

func appendImportEdges(nodes []Node, edges []Edge, fileID string, imports []string, conf float64) ([]Node, []Edge) {
	for _, imp := range imports {
		target := "pkg:" + imp
		nodes = append(nodes, Node{
			ID: target, Type: NodePackage, Title: imp, Source: "extracted", Confidence: conf,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: target, Type: EdgeImports, Source: "extracted", Confidence: conf,
		})
	}
	return nodes, edges
}

type regexLang struct {
	name      string
	exts      map[string]bool
	funcRes   []*regexp.Regexp
	classRes  []*regexp.Regexp
	importRes []*regexp.Regexp
	skipTests bool
}

func (r *regexLang) Name() string     { return r.name }
func (r *regexLang) Category() string { return CategoryLanguage }

func (r *regexLang) Match(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if !r.exts[ext] {
		return false
	}
	if r.skipTests && skipIndexedTestFile(path) {
		return false
	}
	return true
}

func (r *regexLang) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.85)
	funcs := uniqueMatchesMulti(r.funcRes, src)
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, funcs, NodeFunction, "func", 0.8)
	classes := uniqueMatchesMulti(r.classRes, src)
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, classes, NodeClass, "class", 0.8)
	imports := uniqueMatchesMulti(r.importRes, src)
	nodes, edges = appendImportEdges(nodes, edges, fileID, imports, 0.85)
	return nodes, edges, nil
}

func extSet(exts ...string) map[string]bool {
	m := make(map[string]bool, len(exts))
	for _, e := range exts {
		m[e] = true
	}
	return m
}

func mustRE(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}
