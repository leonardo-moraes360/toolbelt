package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// GraphVectorConfig is graph.vector (docs/specs/vector-index.md,
// Configuration). It is user-only: graph reads it through LoadUser.
type GraphVectorConfig struct {
	Enabled      bool `yaml:"enabled"`
	MaxChunks    int  `yaml:"max_chunks"`
	ChunkLines   int  `yaml:"chunk_lines"`
	ChunkBytes   int  `yaml:"chunk_bytes"`
	OverlapLines int  `yaml:"overlap_lines"`
}

// GraphEmbedConfig is graph.embed. It chooses what native code loads, so it
// is user-only like graph.vector.
type GraphEmbedConfig struct {
	Model            string        `yaml:"model"`
	ModelSHA256      string        `yaml:"model_sha256"`
	QueryPrefix      string        `yaml:"query_prefix"`
	DocPrefix        string        `yaml:"doc_prefix"`
	Threads          int           `yaml:"threads"`
	Timeout          time.Duration `yaml:"timeout"`
	AllowUnsandboxed bool          `yaml:"allow_unsandboxed"`
}

// maxRepoConfigBytes matches egm's cap on the repository config (R34).
const maxRepoConfigBytes = 64 << 10

var reSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func DefaultGraphVectorConfig() GraphVectorConfig {
	return GraphVectorConfig{MaxChunks: 50000, ChunkLines: 40, ChunkBytes: 1536, OverlapLines: 8}
}

func DefaultGraphEmbedConfig() GraphEmbedConfig {
	return GraphEmbedConfig{Threads: min(4, runtime.NumCPU()), Timeout: 2 * time.Minute}
}

// MaxChunkBytes keeps one chunk well inside the child's 1 MiB request.
const MaxChunkBytes = 512 << 10

func (v *GraphVectorConfig) Validate() error {
	switch {
	case v.MaxChunks < 1:
		return fmt.Errorf("graph.vector.max_chunks must be >= 1")
	case v.ChunkLines < 1:
		return fmt.Errorf("graph.vector.chunk_lines must be >= 1")
	case v.ChunkBytes < 4:
		// A cut at a UTF-8 boundary needs room for one 4-byte rune.
		return fmt.Errorf("graph.vector.chunk_bytes must be >= 4")
	case v.ChunkBytes > MaxChunkBytes:
		// One chunk must fit in one child request (1 MiB) with its prefix.
		return fmt.Errorf("graph.vector.chunk_bytes must be <= %d", MaxChunkBytes)
	case v.OverlapLines < 0 || v.OverlapLines >= v.ChunkLines:
		// overlap >= chunk_lines would never advance past a chunk.
		return fmt.Errorf("graph.vector.overlap_lines must be >= 0 and < chunk_lines")
	}
	return nil
}

func (e *GraphEmbedConfig) Validate() error {
	switch {
	case e.Model != "" && !filepath.IsAbs(e.Model):
		return fmt.Errorf("graph.embed.model must be an absolute path")
	case e.ModelSHA256 != "" && !reSHA256.MatchString(e.ModelSHA256):
		return fmt.Errorf("graph.embed.model_sha256 must be 64 lowercase hex characters")
	case e.Threads < 1:
		return fmt.Errorf("graph.embed.threads must be >= 1")
	case e.Timeout <= 0:
		return fmt.Errorf("graph.embed.timeout must be a positive duration")
	}
	return nil
}

// LoadUser reads the user config: the -c file when override is set, else
// GlobalConfigPath(); a missing default file means the defaults. It never
// reads the repository's file, so graph.vector and graph.embed must be read
// only from here. Only those two sections are validated: other callers
// (egm) validate their own section.
func LoadUser(override string) (*Config, error) {
	path := override
	if path == "" {
		path = GlobalConfigPath()
	}
	cfg := DefaultConfig()
	cfg.Merge = DefaultMergeConfig()
	if err := loadFromFile(path, cfg); err != nil {
		if override == "" && errors.Is(err, os.ErrNotExist) {
			d := DefaultConfig()
			d.Merge = DefaultMergeConfig()
			return d, nil
		}
		return nil, fmt.Errorf("loading user config %s: %w", path, err)
	}
	if err := cfg.Graph.Vector.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Graph.Embed.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Merge.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// RepoGraphVectorKeys returns the sorted dotted keys set under graph.vector
// and graph.embed in <repoRoot>/.toolbelt/config.yaml, for config_untrusted
// warnings. A missing file is (nil, nil). A symlinked, non-regular, oversized
// or unparsable file is an error: the caller warns and ignores it.
func RepoGraphVectorKeys(repoRoot string) ([]string, error) {
	b, err := readRepoConfig(repoRoot)
	if err != nil || b == nil {
		return nil, err
	}
	var raw struct {
		Graph struct {
			Vector any `yaml:"vector"`
			Embed  any `yaml:"embed"`
		} `yaml:"graph"`
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parsing .toolbelt/config.yaml: %w", err)
	}
	seen := map[string]bool{}
	for _, s := range []struct {
		name string
		v    any
	}{{"graph.vector", raw.Graph.Vector}, {"graph.embed", raw.Graph.Embed}} {
		switch m := s.v.(type) {
		case nil:
		case map[string]any:
			for k := range m {
				seen[s.name+"."+k] = true
			}
		default:
			// A scalar or list still tries to set the section.
			seen[s.name] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// readRepoConfig reads <root>/.toolbelt/config.yaml with Lstat per
// component, a regular file and at most maxRepoConfigBytes, like egm's
// reader. A missing file is (nil, nil).
func readRepoConfig(root string) ([]byte, error) {
	dir := filepath.Join(root, DefaultConfigDir)
	file := filepath.Join(dir, DefaultConfigFile)
	for _, p := range []string{dir, file} {
		fi, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if (p == dir && !fi.IsDir()) || (p == file && !fi.Mode().IsRegular()) {
			return nil, fmt.Errorf("%s is not a regular file or directory", p)
		}
	}
	f, err := os.Open(file) //nolint:gosec // path checked above
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// The file opened must be the regular file inspected, not a swap.
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", file)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxRepoConfigBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxRepoConfigBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", file, maxRepoConfigBytes)
	}
	return b, nil
}
