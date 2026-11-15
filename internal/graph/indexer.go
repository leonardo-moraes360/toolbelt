package graph

import (
	"os"
	"path/filepath"
	"strings"
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
	for _, idx := range others {
		if idx.Category() == CategoryDoc {
			out = append(out, idx)
		}
	}
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
	return idx, nil
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
		if skipDirs[info.Name()] {
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
	byTitle := map[string][]string{}
	for id, n := range nodes {
		switch n.Type {
		case NodeFunction, NodeClass, NodeFile, NodeSection, NodeComponent, NodeHook, NodeRoute:
			key := strings.ToLower(n.Title)
			byTitle[key] = append(byTitle[key], id)
		}
	}
	out := make([]Edge, 0, len(edges))
	for _, e := range edges {
		if !strings.HasPrefix(e.TargetID, "sym:") {
			out = append(out, e)
			continue
		}
		name := strings.TrimPrefix(e.TargetID, "sym:")
		cands := byTitle[strings.ToLower(name)]
		if len(cands) == 0 {
			if _, ok := nodes["file:"+name]; ok {
				e.TargetID = "file:" + name
				out = append(out, e)
			}
			continue
		}
		e.TargetID = cands[0]
		out = append(out, e)
	}
	return out
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
