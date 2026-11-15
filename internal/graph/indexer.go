package graph

import (
	"os"
	"path/filepath"
	"strings"
)

type LanguageIndexer interface {
	Name() string
	Match(path string) bool
	IndexFile(root, rel string) ([]Node, []Edge, error)
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

func indexerFor(path string) LanguageIndexer {
	for _, idx := range languageIndexers {
		if idx.Match(path) {
			return idx
		}
	}
	return nil
}

func BuildFromRoot(root string) (*Index, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	idx := &Index{Root: abs}
	nodes := map[string]Node{}
	var edges []Edge

	err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		return walkSourceFile(abs, path, info, err, nodes, &edges)
	})
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		idx.Nodes = append(idx.Nodes, n)
	}
	idx.Edges = edges
	return idx, nil
}

func walkSourceFile(root, path string, info os.FileInfo, err error, nodes map[string]Node, edges *[]Edge) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		base := info.Name()
		switch base {
		case ".git", "vendor", "node_modules", "bin", "dist", "build", ".next":
			return filepath.SkipDir
		default:
			return nil
		}
	}
	lang := indexerFor(path)
	if lang == nil {
		return nil
	}
	rel, _ := filepath.Rel(root, path)
	fileNodes, fileEdges, err := lang.IndexFile(root, rel)
	if err != nil {
		return nil
	}
	for _, n := range fileNodes {
		nodes[n.ID] = n
	}
	*edges = append(*edges, fileEdges...)
	return nil
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
