package graph

import (
	"path/filepath"
	"strings"
)

var reVueName = mustRE(`(?m)^\s*name\s*:\s*['"]([A-Za-z_][\w-]*)['"]`)

func init() {
	RegisterIndexer(VueIndexer{})
}

type VueIndexer struct{}

func (VueIndexer) Name() string     { return "vue" }
func (VueIndexer) Category() string { return CategoryFramework }

func (VueIndexer) Match(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".vue"
}

func (VueIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.9)

	script := firstMatchingBlock(src, []string{"<script setup", "<script"}, "</script>")
	template := firstMatchingBlock(src, []string{"<template"}, "</template>")
	nodes, edges = appendImportEdges(nodes, edges, fileID, extractImportPaths(script), 0.9)

	compName := filepath.Base(strings.TrimSuffix(rel, filepath.Ext(rel)))
	if names := uniqueMatches(reVueName, script); len(names) > 0 {
		compName = names[0]
	}
	nodes, edges, compIDs := appendComponentNodes(nodes, edges, fileID, modName, rel, []string{compName}, 0.9)
	used := extractTemplateComponents(template)
	edges = appendRenderEdges(edges, compIDs, used, modName)
	return nodes, edges, nil
}
