package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/vecsearch"
)

// vectorPool is the server's one warm embedding child (spec, MCP V2). It
// is created on first use so processes that never search pay nothing.
var (
	vectorPoolMu sync.Mutex
	vectorPool   *vecsearch.Pool
	// newVectorPool is replaced by tests with a fake starter.
	newVectorPool = func() *vecsearch.Pool { return vecsearch.NewPool(vecsearch.StartClient, vecsearch.DefaultIdle) }
)

func vectorStarter() vecsearch.Starter {
	vectorPoolMu.Lock()
	defer vectorPoolMu.Unlock()
	if vectorPool == nil {
		vectorPool = newVectorPool()
	}
	return vectorPool
}

// Shutdown releases what the tools keep between calls (the warm embedding
// child). tb mcp calls it when the server stops; a later call starts anew.
func Shutdown() {
	vectorPoolMu.Lock()
	p := vectorPool
	vectorPool = nil
	vectorPoolMu.Unlock()
	if p != nil {
		_ = p.Close()
	}
}

func loadVectorSettings(s *graph.Store) (*vecsearch.Settings, error) {
	root, _ := vecsearch.RepoRoot(s)
	return vecsearch.LoadSettings(config.File(), root)
}

// withWarnings keeps the warnings of a failed call in its error text: an
// MCP error carries no other field, and config_untrusted explains why a
// repository's vector keys had no effect.
func withWarnings(err error, warnings []string) error {
	if len(warnings) == 0 {
		return err
	}
	return fmt.Errorf("%w (warnings: %s)", err, strings.Join(warnings, "; "))
}

func nonNil(warnings []string) []string {
	if warnings == nil {
		return []string{}
	}
	return warnings
}

// graphVectorSearch is tb graph search --mode semantic|hybrid --json, with
// the warnings the CLI prints on stderr in a warnings array.
func graphVectorSearch(ctx context.Context, s *graph.Store, mode, query string, limit int) (any, error) {
	st, err := loadVectorSettings(s)
	if err != nil {
		return nil, err
	}
	res, warnings, err := vecsearch.Search(ctx, s, mode, query, limit, st, vectorStarter())
	if err != nil {
		return nil, withWarnings(err, warnings)
	}
	return graphPayload(s, map[string]any{
		"mode":          mode,
		"stale_vectors": res.StaleVectors,
		"nodes":         res.Hits,
		"warnings":      nonNil(warnings),
	}), nil
}

// graphVectorIndex is tb graph index --vectors: it adds the vectors stats
// and warnings to the index payload.
func graphVectorIndex(ctx context.Context, s *graph.Store, idx *graph.Index, payload map[string]any) error {
	st, err := loadVectorSettings(s)
	if err != nil {
		return err
	}
	stats, warnings, err := vecsearch.Build(ctx, s, idx, st, vectorStarter())
	if err != nil {
		return withWarnings(err, warnings)
	}
	payload["vectors"] = stats
	payload["warnings"] = nonNil(warnings)
	return nil
}
