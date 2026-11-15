package graph

import (
	"path/filepath"
	"strings"
)

func init() {
	RegisterIndexer(MDXIndexer{})
}

type MDXIndexer struct{}

func (MDXIndexer) Name() string     { return "mdx" }
func (MDXIndexer) Category() string { return CategoryFramework }

func (MDXIndexer) Match(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".mdx"
}

func (MDXIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	nodes, edges, err := indexMarkupDoc(rel, src, true)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	slash := filepath.ToSlash(rel)
	fileID := "file:" + slash
	nodes, edges = appendImportEdges(nodes, edges, fileID, extractImportPaths(src), 0.85)
	used := extractJSXComponents(src)
	edges = appendFileRenderEdges(edges, fileID, used, modName)
	return nodes, edges, nil
}
