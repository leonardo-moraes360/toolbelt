package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/vecsearch"
)

// Search modes of tb graph search (docs/specs/vector-index.md, CLI).
const (
	modeLexical  = vecsearch.ModeLexical
	modeSemantic = vecsearch.ModeSemantic
	modeHybrid   = vecsearch.ModeHybrid
)

func printVectorWarnings(warnings []string) {
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
}

func buildGraphVectors(ctx context.Context, s *graph.Store, idx *graph.Index) (*graph.VectorStats, []string, error) {
	root, _ := vecsearch.RepoRoot(s)
	st, err := vecsearch.LoadSettings(cfgFile, root)
	if err != nil {
		return nil, nil, err
	}
	return vecsearch.Build(ctx, s, idx, st, vecsearch.OneShot{})
}

func runVectorSearch(cmd *cobra.Command, s *graph.Store, mode, query string, limit int) error {
	root, _ := vecsearch.RepoRoot(s)
	st, err := vecsearch.LoadSettings(cfgFile, root)
	if err != nil {
		return err
	}
	res, warnings, err := vecsearch.Search(cmd.Context(), s, mode, query, limit, st, vecsearch.OneShot{})
	// After the results, as before the vecsearch move.
	defer printVectorWarnings(warnings)
	if err != nil {
		return err
	}
	if jsonOutput {
		return emitJSON(envelope.GraphSearch, map[string]any{
			"stale":         s.Stale(),
			"mode":          mode,
			"stale_vectors": res.StaleVectors,
			"nodes":         res.Hits,
		})
	}
	if len(res.Hits) == 0 {
		fmt.Println("(empty)")
		return nil
	}
	for i := range res.Hits {
		h := &res.Hits[i]
		path := h.Path
		if h.Lines != "" {
			path += ":" + h.Lines
		}
		fmt.Printf("%-28s %-10s %s\n", h.ID, h.Type, path)
	}
	return nil
}

func validSearchMode(mode string) error {
	return vecsearch.CheckMode(mode, "--mode")
}
