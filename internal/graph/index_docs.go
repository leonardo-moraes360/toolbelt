package graph

import (
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

var (
	reMDHeading   = mustRE(`(?m)^(#{1,6})\s+(.+?)\s*$`)
	reMDLink      = mustRE(`\[([^\]]*)\]\(([^)]+)\)`)
	reMDAutolink  = mustRE(`(?m)<([^>]+\.(?:md|mdx|html?|txt|go|py|rs|java|ts|tsx|js|mjs))>`)
	reHTMLHeading = mustRE(`(?is)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
	reHTMLAnchor  = mustRE(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>`)
	reBacktick    = mustRE("`([^`\n]{1,120})`")
	rePathLike    = mustRE(`(?m)(?:^|[\s(])((?:[\w.-]+/)*[\w.-]+\.(?:go|py|rs|java|kt|cs|php|rb|swift|ts|tsx|js|mjs|c|h|cpp|hpp|md|mdx|html|txt|sh))\b`)
	reSymLike     = mustRE(`\b([A-Z][A-Za-z0-9_]{2,}|[a-z][a-zA-Z0-9_]{2,})\b`)
	reFileExt     = mustRE(`\.\w{1,5}$`)
)

func init() {
	RegisterIndexer(MarkdownIndexer{})
	RegisterIndexer(HTMLIndexer{})
	RegisterIndexer(TXTIndexer{})
}

type MarkdownIndexer struct{}

func (MarkdownIndexer) Name() string     { return "markdown" }
func (MarkdownIndexer) Category() string { return CategoryDoc }
func (MarkdownIndexer) Match(file string) bool {
	return strings.ToLower(filepath.Ext(file)) == ".md"
}

func (MarkdownIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	return indexMarkupDoc(rel, src, true)
}

type HTMLIndexer struct{}

func (HTMLIndexer) Name() string     { return "html" }
func (HTMLIndexer) Category() string { return CategoryDoc }
func (HTMLIndexer) Match(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == ".html" || ext == ".htm"
}

func (HTMLIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	return indexHTMLDoc(rel, src)
}

type TXTIndexer struct{}

func (TXTIndexer) Name() string     { return "txt" }
func (TXTIndexer) Category() string { return CategoryDoc }
func (TXTIndexer) Match(file string) bool {
	return strings.ToLower(filepath.Ext(file)) == ".txt"
}

func (TXTIndexer) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	return indexTXTDoc(rel, src)
}

func indexMarkupDoc(rel, src string, markdown bool) ([]Node, []Edge, error) {
	slash := filepath.ToSlash(rel)
	docID := "file:" + slash
	nodes := []Node{{
		ID: docID, Type: NodeFile, Path: slash, Title: filepath.Base(rel),
		Source: "extracted", Confidence: 0.9,
	}}
	var edges []Edge

	for _, m := range reMDHeading.FindAllStringSubmatch(src, -1) {
		title := strings.TrimSpace(m[2])
		slug := slugify(title)
		secID := "section:" + slash + "#" + slug
		nodes = append(nodes, Node{
			ID: secID, Type: NodeSection, Path: slash, Title: title,
			Source: "extracted", Confidence: 0.9,
		})
		edges = append(edges, Edge{
			SourceID: docID, TargetID: secID, Type: EdgeDefines, Source: "extracted", Confidence: 0.9,
		})
	}

	if markdown {
		edges = append(edges, docLinkEdges(docID, slash, collectMDLinks(src))...)
		edges = append(edges, docMentionEdges(docID, src)...)
	}
	return nodes, edges, nil
}

func indexHTMLDoc(rel, src string) ([]Node, []Edge, error) {
	slash := filepath.ToSlash(rel)
	docID := "file:" + slash
	nodes := []Node{{
		ID: docID, Type: NodeFile, Path: slash, Title: filepath.Base(rel),
		Source: "extracted", Confidence: 0.9,
	}}
	var edges []Edge

	for _, m := range reHTMLHeading.FindAllStringSubmatch(src, -1) {
		title := strings.TrimSpace(stripTags(m[2]))
		if title == "" {
			continue
		}
		slug := slugify(title)
		secID := "section:" + slash + "#" + slug
		nodes = append(nodes, Node{
			ID: secID, Type: NodeSection, Path: slash, Title: title,
			Source: "extracted", Confidence: 0.85,
		})
		edges = append(edges, Edge{
			SourceID: docID, TargetID: secID, Type: EdgeDefines, Source: "extracted", Confidence: 0.85,
		})
	}

	var hrefs []string
	for _, m := range reHTMLAnchor.FindAllStringSubmatch(src, -1) {
		if len(m) >= 2 {
			hrefs = append(hrefs, m[1])
		}
	}
	edges = append(edges, docLinkEdges(docID, slash, hrefs)...)
	edges = append(edges, docMentionEdges(docID, stripTags(src))...)
	return nodes, edges, nil
}

func indexTXTDoc(rel, src string) ([]Node, []Edge, error) {
	slash := filepath.ToSlash(rel)
	docID := "file:" + slash
	nodes := []Node{{
		ID: docID, Type: NodeFile, Path: slash, Title: filepath.Base(rel),
		Source: "extracted", Confidence: 0.8,
	}}
	edges := docMentionEdges(docID, src)
	edges = append(edges, docLinkEdges(docID, slash, collectPathLikes(src))...)
	return nodes, edges, nil
}

func collectMDLinks(src string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		out = append(out, raw)
	}
	for _, m := range reMDLink.FindAllStringSubmatch(src, -1) {
		add(m[2])
	}
	for _, m := range reMDAutolink.FindAllStringSubmatch(src, -1) {
		add(m[1])
	}
	return out
}

func collectPathLikes(src string) []string {
	return uniqueMatches(rePathLike, src)
}

func docLinkEdges(docID, docPath string, hrefs []string) []Edge {
	var edges []Edge
	seen := map[string]bool{}
	for _, href := range hrefs {
		target := resolveDocHref(docPath, href)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		edges = append(edges, Edge{
			SourceID: docID, TargetID: target, Type: EdgeLinks,
			Source: "extracted", Confidence: 0.75,
		})
	}
	return edges
}

func resolveDocHref(fromPath, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "mailto:") {
		return ""
	}
	frag := ""
	if i := strings.IndexByte(href, '#'); i >= 0 {
		frag = href[i+1:]
		href = href[:i]
	}
	if q := strings.IndexByte(href, '?'); q >= 0 {
		href = href[:q]
	}
	if q := strings.IndexByte(frag, '?'); q >= 0 {
		frag = frag[:q]
	}
	if href == "" {
		if frag == "" {
			return ""
		}
		return "section:" + fromPath + "#" + slugify(frag)
	}
	resolved := normalizeRelPath(fromPath, href)
	if resolved == "" {
		return ""
	}
	if frag != "" {
		return "section:" + resolved + "#" + slugify(frag)
	}
	return "file:" + resolved
}

func normalizeRelPath(fromPath, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "/") {
		return strings.TrimPrefix(filepath.ToSlash(href), "/")
	}
	dir := path.Dir(filepath.ToSlash(fromPath))
	return path.Clean(path.Join(dir, filepath.ToSlash(href)))
}

func docMentionEdges(docID, src string) []Edge {
	var edges []Edge
	seen := map[string]bool{}
	add := func(target string, conf float64) {
		if target == "" || seen[target] || target == docID {
			return
		}
		seen[target] = true
		edges = append(edges, Edge{
			SourceID: docID, TargetID: target, Type: EdgeMentions,
			Source: "extracted", Confidence: conf,
		})
	}
	for _, m := range reBacktick.FindAllStringSubmatch(src, -1) {
		inner := strings.TrimSpace(m[1])
		if inner == "" {
			continue
		}
		if looksLikePath(inner) {
			add("file:"+filepath.ToSlash(inner), 0.7)
			continue
		}
		if looksLikeSymbol(inner) {
			add("sym:"+inner, 0.55)
		}
	}
	for _, p := range collectPathLikes(src) {
		add("file:"+filepath.ToSlash(p), 0.65)
	}
	return edges
}

func looksLikePath(s string) bool {
	return strings.Contains(s, "/") || (strings.Contains(s, ".") && reFileExt.MatchString(s))
}

func looksLikeSymbol(s string) bool {
	if strings.ContainsAny(s, " \t\n/\\") {
		return false
	}
	return reSymLike.MatchString(s)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "section"
	}
	return out
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			in = true
		case '>':
			in = false
		default:
			if !in {
				b.WriteByte(s[i])
			}
		}
	}
	return b.String()
}
