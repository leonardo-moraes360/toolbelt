package graph

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reFWImportFrom = mustRE(`(?m)(?:import|export)(?:[\s\w\{\}\*,]*)\s+from\s+['"]([^'"]+)['"]`)
	reFWImportSide = mustRE(`(?m)^\s*import\s+['"]([^'"]+)['"]`)
	reFWJSXTag     = mustRE(`<([A-Z][A-Za-z0-9_]*)\b`)
	reFWKebabTag   = mustRE(`<([a-z][a-z0-9]*(?:-[a-z0-9]+)+)\b`)
)

func extractImportPaths(src string) []string {
	return uniqueMatchesMulti([]*regexp.Regexp{reFWImportFrom, reFWImportSide}, src)
}

func extractJSXComponents(src string) []string {
	return uniqueMatches(reFWJSXTag, src)
}

func kebabToPascal(name string) string {
	parts := strings.Split(name, "-")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

func extractTemplateComponents(src string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, name := range extractJSXComponents(src) {
		add(name)
	}
	for _, kebab := range uniqueMatches(reFWKebabTag, src) {
		add(kebabToPascal(kebab))
	}
	return out
}

func componentNodeID(modName, name string) string {
	return "component:" + modName + "." + name
}

func hookNodeID(modName, name string) string {
	return "hook:" + modName + "." + name
}

func routeNodeID(routePath string) string {
	return "route:" + routePath
}

func appendComponentNodes(
	nodes []Node,
	edges []Edge,
	fileID, modName, rel string,
	names []string,
	conf float64,
) ([]Node, []Edge, map[string]string) {
	ids := map[string]string{}
	slash := filepath.ToSlash(rel)
	for _, name := range names {
		id := componentNodeID(modName, name)
		ids[name] = id
		nodes = append(nodes, Node{
			ID: id, Type: NodeComponent, Path: slash,
			Title: name, Package: modName, Source: "extracted", Confidence: conf,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: id, Type: EdgeDefines, Source: "extracted", Confidence: conf,
		})
	}
	return nodes, edges, ids
}

func appendHookNodes(
	nodes []Node,
	edges []Edge,
	fileID, modName, rel string,
	names []string,
) ([]Node, []Edge) {
	slash := filepath.ToSlash(rel)
	for _, name := range names {
		id := hookNodeID(modName, name)
		nodes = append(nodes, Node{
			ID: id, Type: NodeHook, Path: slash,
			Title: name, Package: modName, Source: "extracted", Confidence: 0.85,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: id, Type: EdgeDefines, Source: "extracted", Confidence: 0.85,
		})
	}
	return nodes, edges
}

func appendRenderEdges(edges []Edge, fromIDs map[string]string, used []string, modName string) []Edge {
	seen := map[string]bool{}
	for _, fromID := range fromIDs {
		for _, name := range used {
			target := componentNodeID(modName, name)
			if id, ok := fromIDs[name]; ok {
				target = id
			}
			key := fromID + "->" + target
			if seen[key] || fromID == target {
				continue
			}
			seen[key] = true
			edges = append(edges, Edge{
				SourceID: fromID, TargetID: target, Type: EdgeRenders,
				Source: "extracted", Confidence: 0.7,
			})
		}
	}
	return edges
}

func appendFileRenderEdges(edges []Edge, fileID string, used []string, modName string) []Edge {
	seen := map[string]bool{}
	for _, name := range used {
		target := componentNodeID(modName, name)
		key := fileID + "->" + target
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: target, Type: EdgeRenders,
			Source: "extracted", Confidence: 0.65,
		})
	}
	return edges
}

func sliceBetween(src, startTag, endTag string) string {
	start := strings.Index(strings.ToLower(src), strings.ToLower(startTag))
	if start < 0 {
		return ""
	}
	rest := src[start:]
	end := strings.Index(strings.ToLower(rest), strings.ToLower(endTag))
	if end < 0 {
		return rest
	}
	openEnd := strings.Index(rest, ">")
	if openEnd < 0 || openEnd > end {
		return rest[:end]
	}
	return rest[openEnd+1 : end]
}

func firstMatchingBlock(src string, openPatterns []string, closeTag string) string {
	lower := strings.ToLower(src)
	best := -1
	var open string
	for _, p := range openPatterns {
		i := strings.Index(lower, strings.ToLower(p))
		if i >= 0 && (best < 0 || i < best) {
			best = i
			open = p
		}
	}
	if best < 0 {
		return ""
	}
	return sliceBetween(src[best:], open, closeTag)
}
