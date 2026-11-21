// Package vecsearch runs the vector side of tb graph (docs/specs/vector-index.md)
// for both the CLI and the MCP server: it reads the user-only settings,
// obtains an embedder through a Starter and builds or queries graph.vec.
package vecsearch

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/embed"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// Search modes (spec, CLI).
const (
	ModeLexical  = "lexical"
	ModeSemantic = "semantic"
	ModeHybrid   = "hybrid"
)

// CheckMode rejects an unknown mode; name is how the caller spells the
// option (--mode on the CLI, mode over MCP).
func CheckMode(mode, name string) error {
	switch mode {
	case ModeLexical, ModeSemantic, ModeHybrid:
		return nil
	}
	return fmt.Errorf("invalid %s %q: lexical, semantic or hybrid", name, mode)
}

// Settings is the vector config read only from the user layer: the keys
// choose what native code loads, so a repository cannot set them.
type Settings struct {
	Vector config.GraphVectorConfig
	Embed  config.GraphEmbedConfig
	// MaxFileBytes mirrors graph.max_file_bytes from the merged config, the
	// same cap the graph indexer applies.
	MaxFileBytes int64
	// Warnings are config_untrusted notices about ignored repository keys.
	Warnings []string
}

// LoadSettings reads graph.vector and graph.embed from cfgFile (or the
// global config) and warns about any the repository's config tries to set.
func LoadSettings(cfgFile, repoRoot string) (*Settings, error) {
	user, err := config.LoadUser(cfgFile)
	if err != nil {
		return nil, err
	}
	st := &Settings{Vector: user.Graph.Vector, Embed: user.Graph.Embed, MaxFileBytes: graph.DefaultBuildOptions().MaxFileBytes}
	if cfg, err := config.Load(); err == nil && cfg.Graph.MaxFileBytes > 0 {
		st.MaxFileBytes = cfg.Graph.MaxFileBytes
	}
	keys, err := config.RepoGraphVectorKeys(repoRoot)
	switch {
	case err != nil:
		st.Warnings = append(st.Warnings, "config_untrusted: .toolbelt/config.yaml: "+err.Error())
	case len(keys) > 0:
		st.Warnings = append(st.Warnings, "config_untrusted: ignored repository keys "+strings.Join(keys, ", ")+" (set them in the user config)")
	}
	return st, nil
}

// Options is the chunk shape and store path for a .toolbelt directory.
func (st *Settings) Options(toolbeltDir string) graph.VectorOptions {
	return graph.VectorOptions{
		MaxChunks:    st.Vector.MaxChunks,
		ChunkLines:   st.Vector.ChunkLines,
		ChunkBytes:   st.Vector.ChunkBytes,
		OverlapLines: st.Vector.OverlapLines,
		MaxFileBytes: st.MaxFileBytes,
		QueryPrefix:  st.Embed.QueryPrefix,
		DocPrefix:    st.Embed.DocPrefix,
		Path:         graph.VectorPath(toolbeltDir),
	}
}

// StartOptions is how the child is started for repoRoot; it fails with
// model_path when the model keys are unset.
func (st *Settings) StartOptions(repoRoot string) (*embed.StartOptions, error) {
	if st.Embed.Model == "" || st.Embed.ModelSHA256 == "" {
		return nil, fmt.Errorf("%w: graph.embed.model and graph.embed.model_sha256 must be set in the user config", embed.ErrModelPath)
	}
	return &embed.StartOptions{
		Model:            st.Embed.Model,
		SHA256:           st.Embed.ModelSHA256,
		Threads:          st.Embed.Threads,
		Timeout:          st.Embed.Timeout,
		AllowUnsandboxed: st.Embed.AllowUnsandboxed,
		QueryPrefix:      st.Embed.QueryPrefix,
		DocPrefix:        st.Embed.DocPrefix,
		RepoRoot:         repoRoot,
	}, nil
}

func (st *Settings) requireEnabled() error {
	if !st.Vector.Enabled {
		return fmt.Errorf("%w: set graph.vector.enabled: true in the user config", graph.ErrVectorsDisabled)
	}
	return nil
}

func (st *Settings) warnings() []string { return slices.Clone(st.Warnings) }

// RepoRoot is the directory that holds .toolbelt (the graph's root).
func RepoRoot(s *graph.Store) (root, toolbeltDir string) {
	toolbeltDir = filepath.Dir(s.Path())
	return filepath.Dir(toolbeltDir), toolbeltDir
}

// CheckVectorRoot refuses vectors for an index built from a path other than
// the repository root: units are read relative to the root, so a subtree
// index would embed the wrong files.
func CheckVectorRoot(s *graph.Store, indexPath string) error {
	root, _ := RepoRoot(s)
	want, err1 := filepath.Abs(root)
	got, err2 := filepath.Abs(indexPath)
	if err1 != nil || err2 != nil {
		return errors.Join(err1, err2)
	}
	if r, err := filepath.EvalSymlinks(want); err == nil {
		want = r
	}
	if r, err := filepath.EvalSymlinks(got); err == nil {
		got = r
	}
	if got != want {
		return fmt.Errorf("vectors_root: vectors index the repository root (%s), not %s; drop the path", want, got)
	}
	return nil
}
