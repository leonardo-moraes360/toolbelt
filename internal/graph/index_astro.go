package graph

import (
	"path/filepath"
	"strings"
)

func init() {
	RegisterIndexer(AstroIndexer{})
}

type AstroIndexer struct{}

func (AstroIndexer) Name() string     { return "astro" }
func (AstroIndexer) Category() string { return CategoryFramework }

func (AstroIndexer) Match(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".astro"
}

func (AstroIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.9)

	fm, body := splitAstroFrontmatter(src)
	nodes, edges = appendImportEdges(nodes, edges, fileID, extractImportPaths(fm), 0.9)

	compName := filepath.Base(strings.TrimSuffix(rel, filepath.Ext(rel)))
	nodes, edges, compIDs := appendComponentNodes(nodes, edges, fileID, modName, rel, []string{compName}, 0.85)
	used := extractTemplateComponents(body)
	edges = appendRenderEdges(edges, compIDs, used, modName)
	return nodes, edges, nil
}

func splitAstroFrontmatter(src string) (frontmatter, body string) {
	trimmed := strings.TrimSpace(src)
	if !strings.HasPrefix(trimmed, "---") {
		return "", src
	}
	rest := trimmed[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", src
	}
	frontmatter = rest[:end]
	body = rest[end+4:]
	return frontmatter, body
}
