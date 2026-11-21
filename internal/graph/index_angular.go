package graph

import (
	"path/filepath"
	"strings"
)

var (
	reNgComponent  = mustRE(`@Component\s*\(`)
	reNgInjectable = mustRE(`@Injectable\s*\(`)
	reNgModule     = mustRE(`@NgModule\s*\(`)
	reNgSelector   = mustRE(`selector\s*:\s*['"]([^'"]+)['"]`)
	reNgClass      = mustRE(`(?m)export\s+class\s+([A-Za-z_]\w*)`)
	reNgTemplate   = mustRE(`templateUrl\s*:\s*['"]([^'"]+)['"]`)
)

func init() {
	RegisterIndexer(AngularIndexer{})
}

type AngularIndexer struct{}

func (AngularIndexer) Name() string     { return "angular" }
func (AngularIndexer) Category() string { return CategoryFramework }

func (AngularIndexer) Match(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(path))
	ext := filepath.Ext(base)
	switch ext {
	case ".ts", ".js":
		return strings.Contains(base, ".component.") ||
			strings.Contains(base, ".service.") ||
			strings.Contains(base, ".module.") ||
			strings.Contains(base, ".pipe.") ||
			strings.Contains(base, ".directive.")
	case ".html", ".htm":
		return strings.Contains(lower, ".component.html")
	default:
		return false
	}
}

func (AngularIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.9)
	ext := strings.ToLower(filepath.Ext(rel))

	if ext == ".html" || ext == ".htm" {
		comp := angularComponentNameFromHTML(rel)
		if comp != "" {
			target := componentNodeID(modName, comp)
			edges = append(edges, Edge{
				SourceID: fileID, TargetID: target, Type: EdgeTemplate,
				Source: "extracted", Confidence: 0.8,
			})
		}
		used := extractTemplateComponents(src)
		edges = appendFileRenderEdges(edges, fileID, used, modName)
		return nodes, edges, nil
	}

	nodes, edges = appendJSImportEdges(nodes, edges, root, rel, fileID, extractImportPaths(src), 0.9)
	classes := uniqueMatches(reNgClass, src)
	isComp := reNgComponent.MatchString(src)
	isInj := reNgInjectable.MatchString(src)
	isMod := reNgModule.MatchString(src)

	if isComp {
		name := firstOr(classes, fallbackAngularName(rel, "Component"))
		nodes, edges, _ = appendComponentNodes(nodes, edges, fileID, modName, rel, []string{name}, 0.9)
		if sel := uniqueMatches(reNgSelector, src); len(sel) > 0 {
			nodes = append(nodes, Node{
				ID: "component:" + modName + ".selector." + sel[0], Type: NodeComponent,
				Path: filepath.ToSlash(rel), Title: sel[0], Package: modName,
				Source: "extracted", Confidence: 0.85,
			})
		}
		for _, tpl := range uniqueMatches(reNgTemplate, src) {
			tplPath := normalizeRelPath(filepath.ToSlash(rel), tpl)
			edges = append(edges, Edge{
				SourceID: componentNodeID(modName, name), TargetID: "file:" + tplPath,
				Type: EdgeTemplate, Source: "extracted", Confidence: 0.85,
			})
		}
	}
	if isInj || isMod {
		for _, cls := range classes {
			nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, []string{cls}, NodeClass, "class", 0.85)
		}
	}
	if !isComp && !isInj && !isMod {
		nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, classes, NodeClass, "class", 0.8)
	}
	return nodes, edges, nil
}

func angularComponentNameFromHTML(rel string) string {
	base := filepath.Base(rel)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSuffix(base, ".component")
	if base == "" {
		return ""
	}
	parts := strings.Split(base, "-")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	b.WriteString("Component")
	return b.String()
}

func fallbackAngularName(rel, suffix string) string {
	base := filepath.Base(rel)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	for _, cut := range []string{".component", ".service", ".module", ".pipe", ".directive"} {
		base = strings.TrimSuffix(base, cut)
	}
	parts := strings.Split(base, "-")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	name := b.String()
	if name == "" {
		return suffix
	}
	if !strings.HasSuffix(name, suffix) {
		name += suffix
	}
	return name
}

func firstOr(list []string, fallback string) string {
	if len(list) > 0 {
		return list[0]
	}
	return fallback
}
