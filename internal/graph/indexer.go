package graph

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

const (
	CategoryLanguage          = "language"
	CategoryDoc               = "doc"
	CategoryFramework         = "framework"
	DefaultMaxFileBytes int64 = 512 * 1024
)

type LanguageIndexer interface {
	Name() string
	Category() string
	Match(path string) bool
	IndexFile(root, rel string) ([]Node, []Edge, error)
}

type BuildOptions struct {
	Languages    []string
	Docs         []string
	Frameworks   []string
	MaxFileBytes int64
	SkipDirs     []string
}

func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		Languages: []string{
			"go", "javascript", "python", "java", "kotlin", "csharp",
			"cpp", "rust", "php", "ruby", "swift", "shell",
		},
		Docs: []string{"markdown", "html", "txt"},
		Frameworks: []string{
			"react", "vue", "svelte", "astro", "angular", "routes", "mdx",
		},
		MaxFileBytes: DefaultMaxFileBytes,
		SkipDirs: []string{
			".git", "vendor", "node_modules", "bin", "dist", "build",
			".next", "target", "__pycache__", ".venv", "venv", ".tox",
			"coverage", ".idea", ".vscode",
		},
	}
}

func normalizeBuildOptions(opts *BuildOptions) {
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = DefaultMaxFileBytes
	}
	if len(opts.SkipDirs) == 0 {
		opts.SkipDirs = DefaultBuildOptions().SkipDirs
	}
	if len(opts.Languages) == 0 {
		opts.Languages = DefaultBuildOptions().Languages
	}
	if len(opts.Docs) == 0 {
		opts.Docs = DefaultBuildOptions().Docs
	}
	if len(opts.Frameworks) == 0 {
		opts.Frameworks = DefaultBuildOptions().Frameworks
	}
}

func allowsIndexer(opts *BuildOptions, idx LanguageIndexer) bool {
	name := strings.ToLower(idx.Name())
	switch idx.Category() {
	case CategoryDoc:
		return containsFold(opts.Docs, name)
	case CategoryFramework:
		return containsFold(opts.Frameworks, name)
	default:
		return containsFold(opts.Languages, name)
	}
}

func containsFold(list []string, want string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return true
		}
	}
	return false
}

var languageIndexers []LanguageIndexer

func RegisterIndexer(idx LanguageIndexer) {
	if idx == nil {
		return
	}
	languageIndexers = append(languageIndexers, idx)
}

func Indexers() []LanguageIndexer {
	out := make([]LanguageIndexer, len(languageIndexers))
	copy(out, languageIndexers)
	return out
}

func indexersFor(path string, opts *BuildOptions) []LanguageIndexer {
	var frameworks, others []LanguageIndexer
	for _, idx := range languageIndexers {
		if !allowsIndexer(opts, idx) || !idx.Match(path) {
			continue
		}
		if idx.Category() == CategoryFramework {
			frameworks = append(frameworks, idx)
			continue
		}
		others = append(others, idx)
	}
	if len(frameworks) == 0 {
		return others
	}
	out := append([]LanguageIndexer{}, frameworks...)
	out = append(out, others...)
	return out
}

func BuildFromRoot(root string) (*Index, error) {
	opts := DefaultBuildOptions()
	return BuildFromRootOpts(root, &opts)
}

func BuildFromRootOpts(root string, opts *BuildOptions) (*Index, error) {
	if opts == nil {
		def := DefaultBuildOptions()
		opts = &def
	}
	normalizeBuildOptions(opts)
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs, err = resolveIndexRoot(abs)
	if err != nil {
		return nil, err
	}
	idx := &Index{Root: abs}
	nodes := map[string]Node{}
	var edges []Edge
	skip := map[string]bool{}
	for _, d := range opts.SkipDirs {
		skip[d] = true
	}

	err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		return walkSourceFile(abs, path, info, err, nodes, &edges, opts, skip)
	})
	if err != nil {
		return nil, err
	}

	edges = resolveSoftMentions(nodes, edges)

	for _, n := range nodes {
		idx.Nodes = append(idx.Nodes, n)
	}
	idx.Edges = edges
	stampIndexFingerprint(idx, abs, opts)
	return idx, nil
}

func resolveIndexRoot(abs string) (string, error) {
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	if resolved != abs {
		log.Warn("graph root is a symlink; using real path before walk", "from", abs, "to", resolved)
	}
	return resolved, nil
}

func walkSourceFile(
	root, path string,
	info os.FileInfo,
	err error,
	nodes map[string]Node,
	edges *[]Edge,
	opts *BuildOptions,
	skipDirs map[string]bool,
) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		if path != root && skipDirs[info.Name()] {
			return filepath.SkipDir
		}
		return nil
	}
	if info.Size() > opts.MaxFileBytes {
		return nil
	}
	idxs := indexersFor(path, opts)
	if len(idxs) == 0 {
		return nil
	}
	rel, _ := filepath.Rel(root, path)
	for _, lang := range idxs {
		fileNodes, fileEdges, err := lang.IndexFile(root, rel)
		if err != nil {
			continue
		}
		for _, n := range fileNodes {
			nodes[n.ID] = n
		}
		*edges = append(*edges, fileEdges...)
	}
	return nil
}

func resolveSoftMentions(nodes map[string]Node, edges []Edge) []Edge {
	byTitle := indexNodesByTitle(nodes)
	importsOf := buildFileImports(edges)
	out := make([]Edge, 0, len(edges))
	for _, e := range edges {
		if !strings.HasPrefix(e.TargetID, "sym:") {
			out = append(out, e)
			continue
		}
		target, conf, ok := resolveSymTarget(nodes, byTitle, importsOf, e)
		if !ok {
			continue
		}
		e.TargetID = target
		e.Confidence = conf
		out = append(out, e)
	}
	return out
}

func indexNodesByTitle(nodes map[string]Node) map[string][]string {
	byTitle := map[string][]string{}
	for id, n := range nodes {
		switch n.Type {
		case NodeFunction, NodeClass, NodeFile, NodeSection, NodeComponent, NodeHook, NodeRoute:
			key := strings.ToLower(n.Title)
			byTitle[key] = append(byTitle[key], id)
		}
	}
	return byTitle
}

func buildFileImports(edges []Edge) map[string][]string {
	m := map[string][]string{}
	for _, e := range edges {
		if e.Type != EdgeImports {
			continue
		}
		switch {
		case strings.HasPrefix(e.TargetID, "pkg:"):
			m[e.SourceID] = append(m[e.SourceID], strings.TrimPrefix(e.TargetID, "pkg:"))
		case strings.HasPrefix(e.TargetID, "file:"):
			m[e.SourceID] = append(m[e.SourceID], strings.TrimPrefix(e.TargetID, "file:"))
		}
	}
	return m
}

func resolveSymTarget(
	nodes map[string]Node,
	byTitle map[string][]string,
	importsOf map[string][]string,
	e Edge,
) (target string, confidence float64, ok bool) {
	name, moduleHint := splitSymTarget(strings.TrimPrefix(e.TargetID, "sym:"))
	cands := byTitle[strings.ToLower(name)]
	if len(cands) == 0 {
		if _, exists := nodes["file:"+name]; exists {
			return "file:" + name, e.Confidence, true
		}
		return "", 0, false
	}
	if e.Type == EdgeCalls {
		return resolveCallSym(nodes, importsOf, e.SourceID, cands, e.Confidence, moduleHint)
	}
	return cands[0], e.Confidence, true
}

func splitSymTarget(raw string) (name, module string) {
	if i := strings.IndexByte(raw, '@'); i >= 0 {
		return raw[:i], raw[i+1:]
	}
	return raw, ""
}

func resolveCallSym(
	nodes map[string]Node,
	importsOf map[string][]string,
	sourceID string,
	cands []string,
	conf float64,
	moduleHint string,
) (target string, confidence float64, ok bool) {
	funcs := functionCandidates(nodes, cands)
	if len(funcs) == 0 {
		return "", 0, false
	}
	src := nodes[sourceID]
	fileID := sourceFileID(sourceID, &src)
	imported := importsOf[fileID]
	if moduleHint != "" {
		imported = append([]string{moduleHint}, imported...)
	}
	matched := filterCallCandidates(nodes, funcs, &src, imported)
	if moduleHint != "" {
		matched = filterByModuleHint(nodes, matched, moduleHint)
	}
	if len(matched) != 1 {
		return "", 0, false
	}
	if conf < SoftResolveConfidence {
		conf = SoftResolveConfidence
	}
	return matched[0], conf, true
}

func filterByModuleHint(nodes map[string]Node, ids []string, moduleHint string) []string {
	hint := stripJSImportExt(filepath.ToSlash(moduleHint))
	var out []string
	for _, id := range ids {
		n := nodes[id]
		if n.Path == "" {
			continue
		}
		path := filepath.ToSlash(n.Path)
		base := stripJSImportExt(strings.TrimSuffix(path, filepath.Ext(path)))
		if path == moduleHint || base == hint || strings.HasPrefix(base, hint+"/") {
			out = append(out, id)
		}
	}
	if len(out) > 0 {
		return out
	}
	return ids
}

func functionCandidates(nodes map[string]Node, cands []string) []string {
	var funcs, comps []string
	for _, id := range cands {
		n, exists := nodes[id]
		if !exists {
			continue
		}
		switch n.Type {
		case NodeFunction:
			funcs = append(funcs, id)
		case NodeComponent, NodeHook:
			comps = append(comps, id)
		}
	}
	if len(funcs) > 0 {
		return funcs
	}
	return comps
}

func sourceFileID(sourceID string, src *Node) string {
	if strings.HasPrefix(sourceID, "file:") {
		return sourceID
	}
	if src.Path != "" {
		return "file:" + filepath.ToSlash(src.Path)
	}
	return sourceID
}

func filterCallCandidates(nodes map[string]Node, funcs []string, src *Node, imported []string) []string {
	var matched []string
	for _, id := range funcs {
		n := nodes[id]
		if sameCallPackage(src, &n) || importCovers(imported, src.Path, &n) {
			matched = append(matched, id)
		}
	}
	return matched
}

func sameCallPackage(src, tgt *Node) bool {
	return src.Package != "" && src.Package == tgt.Package
}

func importCovers(imported []string, srcPath string, n *Node) bool {
	pkg := n.Package
	qual := callSymbolQual(n)
	for _, imp := range imported {
		if imp == "" {
			continue
		}
		if imp == pkg || strings.HasPrefix(pkg, imp+".") {
			return true
		}
		if strings.HasPrefix(qual, imp+".") {
			return true
		}
		if fileImportCovers(imp, n) {
			return true
		}
		if relativeImportCovers(imp, srcPath, n) {
			return true
		}
	}
	return false
}

func fileImportCovers(imp string, n *Node) bool {
	if n.Path == "" {
		return false
	}
	path := filepath.ToSlash(n.Path)
	impSlash := filepath.ToSlash(imp)
	if path == impSlash {
		return true
	}
	base := stripJSImportExt(strings.TrimSuffix(path, filepath.Ext(path)))
	hint := stripJSImportExt(impSlash)
	return base == hint || strings.HasPrefix(base, hint+"/")
}

func callSymbolQual(n *Node) string {
	qual := n.ID
	for _, prefix := range []string{"func:", "component:", "hook:"} {
		if strings.HasPrefix(qual, prefix) {
			qual = strings.TrimPrefix(qual, prefix)
			break
		}
	}
	return strings.ReplaceAll(qual, "/", ".")
}

func relativeImportCovers(imp, srcPath string, n *Node) bool {
	if srcPath == "" || !strings.HasPrefix(imp, ".") {
		return false
	}
	resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(srcPath), imp)))
	resolved = stripJSImportExt(resolved)
	if n.Path != "" {
		base := stripJSImportExt(filepath.ToSlash(strings.TrimSuffix(n.Path, filepath.Ext(n.Path))))
		if base == resolved || strings.HasPrefix(base, resolved+"/") {
			return true
		}
	}
	if n.Package != "" && (resolved == n.Package || strings.HasPrefix(resolved, n.Package+"/")) {
		return true
	}
	return false
}

func stripJSImportExt(path string) string {
	lower := strings.ToLower(path)
	for _, ext := range []string{".tsx", ".ts", ".jsx", ".js", ".mjs", ".cjs"} {
		if strings.HasSuffix(lower, ext) {
			return path[:len(path)-len(ext)]
		}
	}
	return path
}

func skipIndexedTestFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if strings.HasSuffix(name, "_test") {
		return true
	}
	lower := strings.ToLower(base)
	if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		return true
	}
	return false
}
