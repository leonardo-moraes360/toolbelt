package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	DefaultIndexFile     = "graph.json"
	EnvEnable            = "TB_GRAPH"
	FlagEnv              = "TB_FLAG_GRAPH"
	MinCallsConfidence   = 0.6
	SoftResolveConfidence = 0.75
)

type NodeType string

const (
	NodeFile      NodeType = "file"
	NodeFunction  NodeType = "function"
	NodePackage   NodeType = "package"
	NodeClass     NodeType = "class"
	NodeSection   NodeType = "section"
	NodeComponent NodeType = "component"
	NodeHook      NodeType = "hook"
	NodeRoute     NodeType = "route"
)

type EdgeType string

const (
	EdgeImports  EdgeType = "imports"
	EdgeCalls    EdgeType = "calls"
	EdgeDefines  EdgeType = "defines"
	EdgeLinks    EdgeType = "links"
	EdgeMentions EdgeType = "mentions"
	EdgeRenders  EdgeType = "renders"
	EdgeTemplate EdgeType = "template"
)

type Node struct {
	ID         string   `json:"id"`
	Type       NodeType `json:"type"`
	Path       string   `json:"path,omitempty"`
	Title      string   `json:"title"`
	Package    string   `json:"package,omitempty"`
	Source     string   `json:"source"`
	Confidence float64  `json:"confidence"`
}

type Edge struct {
	SourceID   string   `json:"source_id"`
	TargetID   string   `json:"target_id"`
	Type       EdgeType `json:"type"`
	Source     string   `json:"source"`
	Confidence float64  `json:"confidence"`
}

type Index struct {
	Root  string `json:"root"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	index *Index
}

func Enabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvEnable)))
	if v == "" {
		v = strings.ToLower(strings.TrimSpace(os.Getenv(FlagEnv)))
	}
	switch v {
	case "1", "true", "on", "yes", "enabled":
		return true
	default:
		return false
	}
}

func DefaultPath(toolbeltDir string) string {
	return filepath.Join(toolbeltDir, DefaultIndexFile)
}

func NewStore(indexPath string) *Store {
	return &Store{path: indexPath, index: &Index{}}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.index = &Index{}
		return nil
	}
	if err != nil {
		return err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return err
	}
	s.index = &idx
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *Store) Rebuild(root string) (*Index, error) {
	opts := DefaultBuildOptions()
	return s.RebuildOpts(root, &opts)
}

func (s *Store) RebuildOpts(root string, opts *BuildOptions) (*Index, error) {
	idx, err := BuildFromRootOpts(root, opts)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.index = idx
	s.mu.Unlock()
	if err := s.Save(); err != nil {
		return nil, err
	}
	return idx, nil
}

func (s *Store) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := map[string]int{
		"nodes": len(s.index.Nodes),
		"edges": len(s.index.Edges),
	}
	for _, n := range s.index.Nodes {
		stats["nodes_"+string(n.Type)]++
	}
	for _, e := range s.index.Edges {
		stats["edges_"+string(e.Type)]++
	}
	return stats
}

func (s *Store) Search(query string, limit int) []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || limit <= 0 {
		return nil
	}
	var out []Node
	for _, n := range s.index.Nodes {
		hay := strings.ToLower(n.Title + " " + n.Path + " " + n.Package + " " + n.ID)
		if strings.Contains(hay, q) {
			out = append(out, n)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func (s *Store) Callers(function string, depth int) []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if depth <= 0 {
		depth = 1
	}
	targets := s.findFunctionIDs(function)
	seen := map[string]bool{}
	var out []Node
	frontier := targets
	for d := 0; d < depth; d++ {
		var next []string
		for _, tid := range frontier {
			for _, e := range s.index.Edges {
				if e.Type != EdgeCalls || e.TargetID != tid {
					continue
				}
				if e.Confidence < MinCallsConfidence {
					continue
				}
				if seen[e.SourceID] {
					continue
				}
				seen[e.SourceID] = true
				if n, ok := s.nodeByID(e.SourceID); ok {
					out = append(out, n)
					next = append(next, e.SourceID)
				}
			}
		}
		frontier = next
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Deps(moduleOrFile string, transitive bool) []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	roots := s.findPackageOrFileIDs(moduleOrFile)
	seen := map[string]bool{}
	var out []Node
	frontier := roots
	for len(frontier) > 0 {
		var next []string
		for _, rid := range frontier {
			for _, e := range s.index.Edges {
				if e.Type != EdgeImports || e.SourceID != rid {
					continue
				}
				if seen[e.TargetID] {
					continue
				}
				seen[e.TargetID] = true
				if n, ok := s.nodeByID(e.TargetID); ok {
					out = append(out, n)
					if transitive {
						next = append(next, e.TargetID)
					}
				}
			}
		}
		if !transitive {
			break
		}
		frontier = next
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Impact(file string, depth int) []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if depth <= 0 {
		depth = 2
	}
	ids := s.findPackageOrFileIDs(file)
	funcs := map[string]bool{}
	for _, id := range ids {
		for _, e := range s.index.Edges {
			if e.Type == EdgeDefines && e.SourceID == id {
				funcs[e.TargetID] = true
			}
		}
	}
	seen := map[string]bool{}
	var out []Node
	frontier := keys(funcs)
	for d := 0; d < depth; d++ {
		var next []string
		for _, tid := range frontier {
			for _, e := range s.index.Edges {
				if e.Type != EdgeCalls || e.TargetID != tid {
					continue
				}
				if e.Confidence < MinCallsConfidence {
					continue
				}
				if seen[e.SourceID] {
					continue
				}
				seen[e.SourceID] = true
				if n, ok := s.nodeByID(e.SourceID); ok {
					out = append(out, n)
					next = append(next, e.SourceID)
				}
			}
		}
		frontier = next
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) RequireEnabled() error {
	if !Enabled() {
		return fmt.Errorf("grafo desabilitado (defina %s=on para usar sob demanda, ADR-011)", EnvEnable)
	}
	return nil
}

func (s *Store) nodeByID(id string) (Node, bool) {
	for _, n := range s.index.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

func (s *Store) findFunctionIDs(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	for _, n := range s.index.Nodes {
		if n.Type != NodeFunction {
			continue
		}
		if !functionQueryMatches(n, name) {
			continue
		}
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		ids = append(ids, n.ID)
	}
	return ids
}

func functionQueryMatches(n Node, query string) bool {
	if n.Title == query || strings.HasSuffix(n.Title, "."+query) {
		return true
	}
	id := n.ID
	if strings.HasSuffix(id, "."+query) || strings.HasSuffix(id, "/"+query) || strings.HasSuffix(id, ":"+query) {
		return true
	}
	idDot := strings.ReplaceAll(id, "/", ".")
	return strings.HasSuffix(idDot, "."+query) || strings.HasSuffix(idDot, ":"+query)
}

func (s *Store) findPackageOrFileIDs(q string) []string {
	q = strings.TrimSpace(q)
	var ids []string
	for _, n := range s.index.Nodes {
		if n.Type != NodePackage && n.Type != NodeFile {
			continue
		}
		if n.Title == q || n.Path == q || strings.Contains(n.Path, q) || strings.HasSuffix(n.ID, q) {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
