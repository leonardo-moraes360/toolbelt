package vecsearch

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// Warning texts shared by the CLI (stderr) and MCP (the warnings array).
const (
	WarnUnsandboxed  = "embed_unsandboxed: the embedding child ran without the full sandbox (allow_unsandboxed)"
	WarnStaleVectors = "vectors were built against an older graph index; run tb graph index --vectors"
)

// Hit flattens a node with the semantic/hybrid fields, so a hit reads like
// a lexical node plus extra keys.
type Hit struct {
	graph.Node
	Lines         string   `json:"lines,omitempty"`
	ScoreSemantic *float64 `json:"score_semantic,omitempty"`
	RankLexical   *int     `json:"rank_lexical,omitempty"`
	RankSemantic  *int     `json:"rank_semantic,omitempty"`
	ScoreFused    *float64 `json:"score_fused,omitempty"`
}

// SearchResult is a semantic or hybrid answer. StaleVectors means graph.vec
// predates the current graph index; the hits are still served.
type SearchResult struct {
	Hits         []Hit
	StaleVectors bool
}

// embedder obtains an embedder for repoRoot and appends embed_unsandboxed
// to warnings when the child lacks the full sandbox.
func embedder(ctx context.Context, st *Settings, repoRoot string, starter Starter, warnings *[]string) (embed.Embedder, func(), error) {
	o, err := st.StartOptions(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	emb, release, unsandboxed, err := starter.Embedder(ctx, o)
	if err != nil {
		return nil, nil, err
	}
	if unsandboxed {
		*warnings = append(*warnings, WarnUnsandboxed)
	}
	return emb, release, nil
}

// Build embeds idx's units into graph.vec next to s. The returned warnings
// are meaningful even when err is set.
func Build(ctx context.Context, s *graph.Store, idx *graph.Index, st *Settings, starter Starter) (*graph.VectorStats, []string, error) {
	root, tbDir := RepoRoot(s)
	warnings := st.warnings()
	if err := st.requireEnabled(); err != nil {
		return nil, warnings, err
	}
	emb, release, err := embedder(ctx, st, root, starter, &warnings)
	if err != nil {
		return nil, warnings, err
	}
	defer release()
	opts := st.Options(tbDir)
	stats, err := graph.BuildVectors(ctx, root, idx, emb, &opts)
	if err != nil {
		return nil, warnings, err
	}
	return &stats, warnings, nil
}

// Search answers a semantic or hybrid query from graph.vec; it never
// writes the store or the graph. The returned warnings are meaningful even
// when err is set.
func Search(ctx context.Context, s *graph.Store, mode, query string, limit int, st *Settings, starter Starter) (*SearchResult, []string, error) {
	warnings := st.warnings()
	if mode != ModeSemantic && mode != ModeHybrid {
		return nil, warnings, fmt.Errorf("vector search: mode %q is not semantic or hybrid", mode)
	}
	if err := st.requireEnabled(); err != nil {
		return nil, warnings, err
	}
	root, tbDir := RepoRoot(s)
	opts := st.Options(tbDir)
	store, err := graph.LoadVectorsOpts(opts.Path, &opts)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, warnings, fmt.Errorf("%w: no %s; run tb graph index --vectors", graph.ErrVectorsStale, opts.Path)
		}
		return nil, warnings, err
	}
	emb, release, err := embedder(ctx, st, root, starter, &warnings)
	if err != nil {
		return nil, warnings, err
	}
	defer release()
	if store.Stale(&opts, emb.Info()) {
		return nil, warnings, fmt.Errorf("%w: model, prefixes or chunk shape changed; run tb graph index --vectors", graph.ErrVectorsStale)
	}
	idx := s.Index()
	res := &SearchResult{StaleVectors: store.StaleGraph(idx)}
	var hits []graph.SearchHit
	if mode == ModeSemantic {
		hits, err = graph.SearchSemantic(ctx, idx, store, emb, query, limit)
	} else {
		hits, err = graph.SearchHybrid(ctx, idx, store, emb, query, limit)
	}
	if err != nil {
		return nil, warnings, err
	}
	if res.StaleVectors {
		warnings = append(warnings, WarnStaleVectors)
	}
	res.Hits = make([]Hit, len(hits))
	for i := range hits {
		h := &hits[i]
		res.Hits[i] = Hit{Node: h.Node, Lines: h.Lines, ScoreSemantic: h.ScoreSemantic,
			RankLexical: h.RankLexical, RankSemantic: h.RankSemantic, ScoreFused: h.ScoreFused}
	}
	return res, warnings, nil
}
