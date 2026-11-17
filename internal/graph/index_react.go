package graph

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reReactFuncComp = mustRE(`(?m)(?:export\s+)?(?:default\s+)?function\s+([A-Z][A-Za-z0-9_]*)\s*(?:<[^(){]*?>)?\s*\(`)
	reReactConstFC  = mustRE(`(?m)(?:export\s+)?(?:const|let)\s+([A-Z][A-Za-z0-9_]*)\s*=\s*(?:React\.)?(?:memo|forwardRef)\s*\(`)
	reReactConstJSX = mustRE(`(?m)(?:export\s+)?(?:const|let)\s+([A-Z][A-Za-z0-9_]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*(?::\s*[^=]+)?=>`)
	reReactTypedFC  = mustRE(`(?m)(?:export\s+)?(?:const|let)\s+([A-Z][A-Za-z0-9_]*)\s*:\s*(?:React\.)?FC\b`)
	reReactHook     = mustRE(`(?m)(?:export\s+)?(?:default\s+)?(?:function\s+|const\s+|let\s+)(use[A-Z][A-Za-z0-9_]*)\b`)
)

func init() {
	RegisterIndexer(ReactIndexer{})
}

type ReactIndexer struct{}

func (ReactIndexer) Name() string     { return "react" }
func (ReactIndexer) Category() string { return CategoryFramework }

func (ReactIndexer) Match(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tsx" || ext == ".jsx"
}

func (ReactIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.9)
	nodes, edges = appendImportEdges(nodes, edges, fileID, extractImportPaths(src), 0.9)

	compNames := uniqueMatchesMulti([]*regexp.Regexp{
		reReactFuncComp, reReactConstFC, reReactConstJSX, reReactTypedFC,
	}, src)
	nodes, edges, compIDs := appendComponentNodes(nodes, edges, fileID, modName, rel, compNames, 0.9)

	hooks := uniqueMatches(reReactHook, src)
	nodes, edges = appendHookNodes(nodes, edges, fileID, modName, rel, hooks)

	used := extractJSXComponents(src)
	if len(compIDs) > 0 {
		edges = appendRenderEdges(edges, compIDs, used, modName)
	} else {
		edges = appendFileRenderEdges(edges, fileID, used, modName)
	}
	return nodes, edges, nil
}
