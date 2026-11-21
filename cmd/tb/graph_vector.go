package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/embed"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// Search modes of tb graph search (docs/specs/vector-index.md, CLI).
const (
	modeLexical  = "lexical"
	modeSemantic = "semantic"
	modeHybrid   = "hybrid"
)

// vectorSettings is the vector config read only from the user layer: the
// keys choose what native code loads, so a repository cannot set them.
type vectorSettings struct {
	vector   config.GraphVectorConfig
	embed    config.GraphEmbedConfig
	warnings []string
}

func loadVectorSettings(repoRoot string) (*vectorSettings, error) {
	user, err := config.LoadUser(cfgFile)
	if err != nil {
		return nil, err
	}
	vs := &vectorSettings{vector: user.Graph.Vector, embed: user.Graph.Embed}
	keys, err := config.RepoGraphVectorKeys(repoRoot)
	switch {
	case err != nil:
		vs.warnings = append(vs.warnings, "config_untrusted: .toolbelt/config.yaml: "+err.Error())
	case len(keys) > 0:
		vs.warnings = append(vs.warnings, "config_untrusted: ignored repository keys "+strings.Join(keys, ", ")+" (set them in the user config)")
	}
	return vs, nil
}

func (vs *vectorSettings) options(toolbeltDir string) graph.VectorOptions {
	opts := graphBuildOptions()
	return graph.VectorOptions{
		MaxChunks:    vs.vector.MaxChunks,
		ChunkLines:   vs.vector.ChunkLines,
		ChunkBytes:   vs.vector.ChunkBytes,
		OverlapLines: vs.vector.OverlapLines,
		MaxFileBytes: opts.MaxFileBytes,
		QueryPrefix:  vs.embed.QueryPrefix,
		DocPrefix:    vs.embed.DocPrefix,
		Path:         graph.VectorPath(toolbeltDir),
	}
}

func (vs *vectorSettings) startEmbedder(ctx context.Context, repoRoot string) (*embed.Client, error) {
	if vs.embed.Model == "" || vs.embed.ModelSHA256 == "" {
		return nil, fmt.Errorf("%w: graph.embed.model and graph.embed.model_sha256 must be set in the user config", embed.ErrModelPath)
	}
	c, err := embed.Start(ctx, &embed.StartOptions{
		Model:            vs.embed.Model,
		SHA256:           vs.embed.ModelSHA256,
		Threads:          vs.embed.Threads,
		Timeout:          vs.embed.Timeout,
		AllowUnsandboxed: vs.embed.AllowUnsandboxed,
		QueryPrefix:      vs.embed.QueryPrefix,
		DocPrefix:        vs.embed.DocPrefix,
		RepoRoot:         repoRoot,
	})
	if err != nil {
		return nil, err
	}
	if c.Unsandboxed() {
		vs.warnings = append(vs.warnings, "embed_unsandboxed: the embedding child ran without the full sandbox (allow_unsandboxed)")
	}
	return c, nil
}

func (vs *vectorSettings) printWarnings() {
	for _, w := range vs.warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
}

// graphRepoRoot is the directory that holds .toolbelt (the graph's root).
func graphRepoRoot(s *graph.Store) (root, toolbeltDir string) {
	toolbeltDir = filepath.Dir(s.Path())
	return filepath.Dir(toolbeltDir), toolbeltDir
}

func buildGraphVectors(ctx context.Context, s *graph.Store, idx *graph.Index) (*graph.VectorStats, []string, error) {
	root, tbDir := graphRepoRoot(s)
	vs, err := loadVectorSettings(root)
	if err != nil {
		return nil, nil, err
	}
	if !vs.vector.Enabled {
		return nil, vs.warnings, fmt.Errorf("%w: set graph.vector.enabled: true in the user config", graph.ErrVectorsDisabled)
	}
	c, err := vs.startEmbedder(ctx, root)
	if err != nil {
		return nil, vs.warnings, err
	}
	defer c.Close()
	opts := vs.options(tbDir)
	st, err := graph.BuildVectors(ctx, root, idx, c, &opts)
	if err != nil {
		return nil, vs.warnings, err
	}
	return &st, vs.warnings, nil
}

// vectorHit flattens a node with the semantic/hybrid fields, so a hit reads
// like a lexical node plus extra keys.
type vectorHit struct {
	graph.Node
	Lines         string   `json:"lines,omitempty"`
	ScoreSemantic *float64 `json:"score_semantic,omitempty"`
	RankLexical   *int     `json:"rank_lexical,omitempty"`
	RankSemantic  *int     `json:"rank_semantic,omitempty"`
	ScoreFused    *float64 `json:"score_fused,omitempty"`
}

func runVectorSearch(cmd *cobra.Command, s *graph.Store, mode, query string, limit int) error {
	root, tbDir := graphRepoRoot(s)
	vs, err := loadVectorSettings(root)
	if err != nil {
		return err
	}
	defer vs.printWarnings()
	if !vs.vector.Enabled {
		return fmt.Errorf("%w: set graph.vector.enabled: true in the user config", graph.ErrVectorsDisabled)
	}
	opts := vs.options(tbDir)
	store, err := graph.LoadVectorsOpts(opts.Path, &opts)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: no %s; run tb graph index --vectors", graph.ErrVectorsStale, opts.Path)
		}
		return err
	}
	ctx := cmd.Context()
	c, err := vs.startEmbedder(ctx, root)
	if err != nil {
		return err
	}
	defer c.Close()
	if store.Stale(&opts, c.Info()) {
		return fmt.Errorf("%w: model, prefixes or chunk shape changed; run tb graph index --vectors", graph.ErrVectorsStale)
	}
	idx := s.Index()
	staleVectors := store.StaleGraph(idx)
	var hits []graph.SearchHit
	if mode == modeSemantic {
		hits, err = graph.SearchSemantic(ctx, idx, store, c, query, limit)
	} else {
		hits, err = graph.SearchHybrid(ctx, idx, store, c, query, limit)
	}
	if err != nil {
		return err
	}
	if staleVectors {
		vs.warnings = append(vs.warnings, "vectors were built against an older graph index; run tb graph index --vectors")
	}
	out := make([]vectorHit, len(hits))
	for i := range hits {
		h := &hits[i]
		out[i] = vectorHit{Node: h.Node, Lines: h.Lines, ScoreSemantic: h.ScoreSemantic,
			RankLexical: h.RankLexical, RankSemantic: h.RankSemantic, ScoreFused: h.ScoreFused}
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"stale":         s.Stale(),
			"mode":          mode,
			"stale_vectors": staleVectors,
			"nodes":         out,
		})
	}
	if len(out) == 0 {
		fmt.Println("(empty)")
		return nil
	}
	for i := range out {
		h := &out[i]
		path := h.Path
		if h.Lines != "" {
			path += ":" + h.Lines
		}
		fmt.Printf("%-28s %-10s %s\n", h.ID, h.Type, path)
	}
	return nil
}

func validSearchMode(mode string) error {
	switch mode {
	case modeLexical, modeSemantic, modeHybrid:
		return nil
	}
	return fmt.Errorf("invalid --mode %q: lexical, semantic or hybrid", mode)
}
