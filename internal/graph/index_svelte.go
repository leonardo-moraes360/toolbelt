package graph

import (
	"path/filepath"
	"strings"
)

func init() {
	RegisterIndexer(SvelteIndexer{})
}

type SvelteIndexer struct{}

func (SvelteIndexer) Name() string     { return "svelte" }
func (SvelteIndexer) Category() string { return CategoryFramework }

func (SvelteIndexer) Match(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".svelte"
}

func (SvelteIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.9)

	script := firstMatchingBlock(src, []string{"<script"}, "</script>")
	nodes, edges = appendImportEdges(nodes, edges, fileID, extractImportPaths(script), 0.9)

	compName := filepath.Base(strings.TrimSuffix(rel, filepath.Ext(rel)))
	nodes, edges, compIDs := appendComponentNodes(nodes, edges, fileID, modName, rel, []string{compName}, 0.9)

	markup := src
	if i := strings.Index(strings.ToLower(src), "</script>"); i >= 0 {
		markup = src[i+len("</script>"):]
	}
	used := extractTemplateComponents(markup)
	edges = appendRenderEdges(edges, compIDs, used, modName)
	return nodes, edges, nil
}
