package graph

import (
	"path/filepath"
	"strings"
)

func init() {
	RegisterIndexer(RoutesIndexer{})
}

type RoutesIndexer struct{}

func (RoutesIndexer) Name() string     { return "routes" }
func (RoutesIndexer) Category() string { return CategoryFramework }

func (RoutesIndexer) Match(path string) bool {
	slash := filepath.ToSlash(strings.ToLower(path))
	base := filepath.Base(slash)
	ext := filepath.Ext(base)
	switch {
	case containsPathSeg(slash, "app"):
		return isAppRouterFile(base, ext)
	case containsPathSeg(slash, "pages"):
		return isPagesRouterFile(base, ext)
	case containsPathSeg(slash, "routes"):
		return isSvelteKitRouteFile(base, ext)
	default:
		return false
	}
}

func isAppRouterFile(base, ext string) bool {
	switch base {
	case "page" + ext, "route" + ext, "layout" + ext:
		return isWebScriptExt(ext) || ext == ".mdx"
	default:
		return false
	}
}

func isPagesRouterFile(base, ext string) bool {
	if strings.HasPrefix(base, "_") {
		return false
	}
	return isWebScriptExt(ext) || ext == ".vue"
}

func isSvelteKitRouteFile(base, ext string) bool {
	if !(strings.HasPrefix(base, "+page") || strings.HasPrefix(base, "+layout") || strings.HasPrefix(base, "+server")) {
		return false
	}
	return ext == ".svelte" || ext == ".ts" || ext == ".js"
}

func isWebScriptExt(ext string) bool {
	switch ext {
	case ".tsx", ".jsx", ".ts", ".js":
		return true
	default:
		return false
	}
}

func (RoutesIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	route := deriveRoutePath(rel)
	if route == "" {
		return nil, nil, nil
	}
	slash := filepath.ToSlash(rel)
	fileID := "file:" + slash
	routeID := routeNodeID(route)
	nodes := []Node{
		{ID: fileID, Type: NodeFile, Path: slash, Title: filepath.Base(rel), Source: "extracted", Confidence: 0.9},
		{ID: routeID, Type: NodeRoute, Path: slash, Title: route, Source: "extracted", Confidence: 0.9},
	}
	edges := []Edge{
		{SourceID: fileID, TargetID: routeID, Type: EdgeDefines, Source: "extracted", Confidence: 0.9},
	}
	return nodes, edges, nil
}

func deriveRoutePath(rel string) string {
	slash := filepath.ToSlash(rel)
	if rest, ok := afterPathSeg(slash, "app"); ok {
		return normalizeAppRouterPath(rest)
	}
	if rest, ok := afterPathSeg(slash, "pages"); ok {
		return normalizePagesRouterPath(rest)
	}
	if rest, ok := afterPathSeg(slash, "routes"); ok {
		return normalizeSvelteKitPath(rest)
	}
	return ""
}

func containsPathSeg(path, seg string) bool {
	_, ok := afterPathSeg(path, seg)
	return ok
}

func afterPathSeg(path, seg string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, p := range parts {
		if !strings.EqualFold(p, seg) {
			continue
		}
		if i+1 >= len(parts) {
			return "", true
		}
		return strings.Join(parts[i+1:], "/"), true
	}
	return "", false
}

func normalizeAppRouterPath(rest string) string {
	rest = strings.TrimSuffix(rest, filepath.Ext(rest))
	base := filepath.Base(rest)
	dir := filepath.ToSlash(filepath.Dir(rest))
	switch base {
	case "page", "route", "layout", "default", "loading", "error", "not-found":
		if dir == "." {
			return "/"
		}
		return "/" + stripRouteGroups(dir)
	default:
		return "/" + stripRouteGroups(rest)
	}
}

func normalizePagesRouterPath(rest string) string {
	rest = strings.TrimSuffix(rest, filepath.Ext(rest))
	rest = strings.ReplaceAll(rest, "\\", "/")
	if rest == "index" {
		return "/"
	}
	rest = strings.TrimSuffix(rest, "/index")
	rest = strings.ReplaceAll(rest, "[", ":")
	rest = strings.ReplaceAll(rest, "]", "")
	return "/" + strings.TrimPrefix(rest, "/")
}

func normalizeSvelteKitPath(rest string) string {
	return svelteKitRoute(strings.TrimSuffix(rest, filepath.Ext(rest)))
}

func svelteKitRoute(rest string) string {
	parts := strings.Split(filepath.ToSlash(rest), "/")
	var segs []string
	for _, p := range parts {
		if p == "" || strings.HasPrefix(p, "+") {
			continue
		}
		if strings.HasPrefix(p, "(") && strings.HasSuffix(p, ")") {
			continue
		}
		switch {
		case strings.HasPrefix(p, "[...") && strings.HasSuffix(p, "]"):
			segs = append(segs, "*"+strings.TrimSuffix(strings.TrimPrefix(p, "[..."), "]"))
		case strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]"):
			segs = append(segs, ":"+strings.TrimSuffix(strings.TrimPrefix(p, "["), "]"))
		default:
			segs = append(segs, p)
		}
	}
	if len(segs) == 0 {
		return "/"
	}
	return "/" + strings.Join(segs, "/")
}

func stripRouteGroups(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	var out []string
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if strings.HasPrefix(p, "(") && strings.HasSuffix(p, ")") {
			continue
		}
		if strings.HasPrefix(p, "@") {
			continue
		}
		switch {
		case strings.HasPrefix(p, "[[...") && strings.HasSuffix(p, "]]"):
			out = append(out, "*"+strings.TrimSuffix(strings.TrimPrefix(p, "[[..."), "]]"))
		case strings.HasPrefix(p, "[...") && strings.HasSuffix(p, "]"):
			out = append(out, "*"+strings.TrimSuffix(strings.TrimPrefix(p, "[..."), "]"))
		case strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]"):
			out = append(out, ":"+strings.TrimSuffix(strings.TrimPrefix(p, "["), "]"))
		default:
			out = append(out, p)
		}
	}
	return strings.Join(out, "/")
}
