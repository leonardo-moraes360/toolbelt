package config

import (
	"fmt"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// MergeConfig is the merge: section (docs/specs/merge-strategies.md, M23).
// Every key is user-only: the semantic and prompt strategies read it through
// LoadUser, and a repository .toolbelt/config.yaml cannot raise budgets.
type MergeConfig struct {
	Semantic MergeSemantic `yaml:"semantic"`
	Prompt   MergePrompt   `yaml:"prompt"`
}

// MergeSemantic bounds the deterministic resolver.
type MergeSemantic struct {
	MaxRegionLines int   `yaml:"max_region_lines"`
	MaxFileBytes   int64 `yaml:"max_file_bytes"`
}

// MergePrompt bounds the agent step. MaxNovelLines is a pointer so "unset"
// (report only) differs from 0 (the agent may only reuse existing lines).
type MergePrompt struct {
	MaxFiles       int    `yaml:"max_files"`
	ContextLines   int    `yaml:"context_lines"`
	MaxPromptBytes int64  `yaml:"max_prompt_bytes"`
	TimeoutPerFile string `yaml:"timeout_per_file"`
	MaxTotal       string `yaml:"max_total"`
	Attempts       int    `yaml:"attempts"`
	MaxStdoutBytes int64  `yaml:"max_stdout_bytes"`
	MaxNovelLines  *int   `yaml:"max_novel_lines,omitempty"`
}

// maxRegionLinesCap bounds the quadratic line diff of one region.
const maxRegionLinesCap = 10000

func DefaultMergeConfig() MergeConfig {
	return MergeConfig{
		Semantic: MergeSemantic{MaxRegionLines: 2000, MaxFileBytes: 1 << 20},
		Prompt: MergePrompt{
			MaxFiles: 10, ContextLines: 20, MaxPromptBytes: 256 << 10,
			TimeoutPerFile: "5m", MaxTotal: "20m", Attempts: 2, MaxStdoutBytes: 1 << 20,
		},
	}
}

// Validate checks the ranges of the merge section.
func (m *MergeConfig) Validate() error {
	for _, d := range []struct{ key, val string }{
		{"merge.prompt.timeout_per_file", m.Prompt.TimeoutPerFile},
		{"merge.prompt.max_total", m.Prompt.MaxTotal},
	} {
		if v, err := time.ParseDuration(d.val); err != nil || v <= 0 {
			return fmt.Errorf("%s must be a positive duration", d.key)
		}
	}
	checks := []struct {
		bad bool
		msg string
	}{
		{m.Semantic.MaxRegionLines < 1, "merge.semantic.max_region_lines must be >= 1"},
		{m.Semantic.MaxRegionLines > maxRegionLinesCap, "merge.semantic.max_region_lines must be <= 10000"},
		{m.Semantic.MaxFileBytes < 1, "merge.semantic.max_file_bytes must be >= 1"},
		{m.Prompt.MaxFiles < 1, "merge.prompt.max_files must be >= 1"},
		{m.Prompt.ContextLines < 0, "merge.prompt.context_lines must be >= 0"},
		{m.Prompt.MaxPromptBytes < 1, "merge.prompt.max_prompt_bytes must be >= 1"},
		{m.Prompt.Attempts < 1, "merge.prompt.attempts must be >= 1"},
		{m.Prompt.MaxStdoutBytes < 1, "merge.prompt.max_stdout_bytes must be >= 1"},
		{m.Prompt.MaxNovelLines != nil && *m.Prompt.MaxNovelLines < 0, "merge.prompt.max_novel_lines must be >= 0"},
	}
	for _, c := range checks {
		if c.bad {
			return fmt.Errorf("%s", c.msg)
		}
	}
	return nil
}

// egmUserOnlyKeys are the egm keys that choose the agent; the prompt strategy
// reads them from the user config only.
var egmUserOnlyKeys = []string{"agent_cmd", "path", "env", "home", "vendor", "output", "tools_disabled", "tools_check", "forbidden_flags"}

// RepoMergeKeys returns the sorted dotted keys set under merge, plus the egm
// agent keys, in <repoRoot>/.toolbelt/config.yaml (all ignored by merge), for
// config_untrusted warnings. A missing
// file is (nil, nil); an unsafe or unparsable one is an error (warn, ignore).
func RepoMergeKeys(repoRoot string) ([]string, error) {
	b, err := readRepoConfig(repoRoot)
	if err != nil || b == nil {
		return nil, err
	}
	var raw struct {
		Merge any            `yaml:"merge"`
		Egm   map[string]any `yaml:"egm"`
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parsing .toolbelt/config.yaml: %w", err)
	}
	seen := map[string]bool{}
	flattenKeys("merge", raw.Merge, seen)
	for _, k := range egmUserOnlyKeys {
		if v, ok := raw.Egm[k]; ok && v != nil {
			seen["egm."+k] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func flattenKeys(prefix string, v any, seen map[string]bool) {
	switch m := v.(type) {
	case nil:
	case map[string]any:
		if len(m) == 0 {
			seen[prefix] = true
		}
		for k, sub := range m {
			flattenKeys(prefix+"."+k, sub, seen)
		}
	default:
		seen[prefix] = true
	}
}
