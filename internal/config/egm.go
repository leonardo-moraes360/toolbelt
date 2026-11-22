package config

import (
	"fmt"
	"path/filepath"
	"time"
)

// EgmConfig is the egm: section (docs/specs/egm-nativo.md, Configuration).
// Execution and secret keys only count from the user config (R34).
type EgmConfig struct {
	AgentCmd       []string `yaml:"agent_cmd,omitempty"`
	Path           string   `yaml:"path,omitempty"`
	Vendor         string   `yaml:"vendor,omitempty"`
	Output         string   `yaml:"output,omitempty"`
	ToolsDisabled  bool     `yaml:"tools_disabled,omitempty"`
	ToolsCheck     string   `yaml:"tools_check,omitempty"`
	ForbiddenFlags []string `yaml:"forbidden_flags,omitempty"`
	Env            struct {
		Pass []string `yaml:"pass,omitempty"`
	} `yaml:"env,omitempty"`
	Home struct {
		Copy     []string `yaml:"copy,omitempty"`
		ReadOnly []string `yaml:"read_only,omitempty"`
	} `yaml:"home,omitempty"`

	DocGlobs            []string          `yaml:"doc_globs,omitempty"`
	DocTypes            map[string]string `yaml:"doc_types,omitempty"`
	DetailHeadings      []string          `yaml:"detail_headings,omitempty"`
	RefsIgnore          []string          `yaml:"refs_ignore,omitempty"`
	Match               EgmMatch          `yaml:"match"`
	Attest              EgmAttest         `yaml:"attest"`
	RequireFullCoverage bool              `yaml:"require_full_coverage,omitempty"`
	MaxRounds           int               `yaml:"max_rounds"`
	CleanRounds         int               `yaml:"clean_rounds"`
	MaxAgentRuns        int               `yaml:"max_agent_runs"`
	MaxSourceBytes      int64             `yaml:"max_source_bytes"`
	MaxPacketBytes      int64             `yaml:"max_packet_bytes"`
	MaxParallel         int               `yaml:"max_parallel"`
	TimeoutPerReviewer  string            `yaml:"timeout_per_reviewer"`
	MaxStdoutBytes      int64             `yaml:"max_stdout_bytes"`
	CI                  EgmCI             `yaml:"ci"`
	Drift               EgmDrift          `yaml:"drift"`
	Bootstrap           EgmBootstrap      `yaml:"bootstrap"`
	Brainstorm          EgmBrainstorm     `yaml:"brainstorm"`
	Draft               EgmDraft          `yaml:"draft"`
	Receipt             EgmReceipt        `yaml:"receipt"`
}

// EgmBootstrap tunes tb egm bootstrap --engine llm (docs/specs/egm-drift-bootstrap.md, B3).
type EgmBootstrap struct {
	// MaxBytes bounds the model body. Limit key: the repository can only lower it.
	MaxBytes int64 `yaml:"max_bytes" json:"max_bytes"`
}

// EgmBrainstorm tunes tb egm brainstorm (docs/specs/egm-phase-c.md, R4).
type EgmBrainstorm struct {
	// MaxBytes bounds the concept brief body. Limit key: the repository can only lower it.
	MaxBytes int64 `yaml:"max_bytes" json:"max_bytes"`
}

// EgmDraft tunes tb egm draft (docs/specs/egm-phase-c.md, R9).
type EgmDraft struct {
	// MaxBytes bounds the draft body. Limit key: the repository can only lower it.
	MaxBytes int64 `yaml:"max_bytes" json:"max_bytes"`
}

// EgmReceipt tunes the receipt writer (B3). User-only: a CI pinned to an
// older tb must be upgraded before receipts move to version 2.
type EgmReceipt struct {
	WriteV2 bool `yaml:"write_v2,omitempty" json:"write_v2,omitempty"`
}

// EgmDrift tunes source drift (docs/specs/egm-drift-bootstrap.md, B1).
type EgmDrift struct {
	// CI is ci_source_drift's severity: warn (default) or fail. Policy key:
	// the repository config can only raise it to fail.
	CI string `yaml:"ci" json:"ci"`
	// MaxDocs bounds the drift goldfish of one drift-report --engine llm
	// (B3). Limit key: the repository can only lower it.
	MaxDocs int `yaml:"max_docs" json:"max_docs"`
}

// EgmCI tunes the CI mode of the egm gate (docs/specs/egm-trust.md, T-R6).
type EgmCI struct {
	MaxDocs int `yaml:"max_docs" json:"max_docs"`
}

// EgmMatch tunes R18; defaults calibrated in R0 (dogfood #31).
type EgmMatch struct {
	Sim  float64 `yaml:"sim" json:"sim"`
	Stem int     `yaml:"stem" json:"stem"`
	Gap  int     `yaml:"gap" json:"gap"`
}

// EgmAttest tunes R17.
type EgmAttest struct {
	MinQuotes int `yaml:"min_quotes" json:"min_quotes"`
}

// DefaultEgmConfig returns the documented defaults.
func DefaultEgmConfig() EgmConfig {
	return EgmConfig{
		Path:               "/usr/bin:/bin",
		Vendor:             "claude",
		ToolsCheck:         "auto",
		DocGlobs:           []string{"docs/specs/**", "docs/adr/**"},
		DocTypes:           map[string]string{"docs/adr/**": "adr"},
		DetailHeadings:     []string{"## Detailed Implementation Plan", "## Plano de Implementação Detalhado", "## Implementation Plan"},
		RefsIgnore:         []string{"refs/**"},
		Match:              EgmMatch{Sim: 0.23, Stem: 4, Gap: 3},
		Attest:             EgmAttest{MinQuotes: 3},
		MaxRounds:          8,
		CleanRounds:        2,
		MaxAgentRuns:       120,
		MaxSourceBytes:     65536,
		MaxPacketBytes:     409600,
		MaxParallel:        4,
		TimeoutPerReviewer: "5m",
		MaxStdoutBytes:     1 << 20,
		CI:                 EgmCI{MaxDocs: 5},
		Drift:              EgmDrift{CI: "warn", MaxDocs: 5},
		Bootstrap:          EgmBootstrap{MaxBytes: 32 << 10},
		Brainstorm:         EgmBrainstorm{MaxBytes: 16 << 10},
		Draft:              EgmDraft{MaxBytes: 32 << 10},
	}
}

// Validate checks the ranges of the Configuration section.
func (e *EgmConfig) Validate() error {
	checks := []struct {
		bad bool
		msg string
	}{
		{len(e.AgentCmd) > 0 && !filepath.IsAbs(e.AgentCmd[0]), "egm.agent_cmd[0] must be an absolute path"},
		{e.Vendor != "claude" && e.Vendor != "custom", "egm.vendor must be claude or custom"},
		{e.Output != "" && e.Output != "claude-json" && e.Output != "raw", "egm.output must be claude-json or raw"},
		{e.ToolsCheck != "auto" && e.ToolsCheck != "none", "egm.tools_check must be auto or none"},
		{!(e.Match.Sim > 0 && e.Match.Sim <= 1), "egm.match.sim must be in (0, 1]"}, // also rejects NaN
		{e.Match.Stem < 0, "egm.match.stem must be >= 0"},
		{e.Match.Gap < 0, "egm.match.gap must be >= 0"},
		{e.Attest.MinQuotes < 1, "egm.attest.min_quotes must be >= 1"},
		{e.MaxRounds < 1, "egm.max_rounds must be >= 1"},
		{e.CleanRounds < 1 || e.CleanRounds > e.MaxRounds, "egm.clean_rounds must be between 1 and max_rounds"},
		{e.MaxAgentRuns < 1, "egm.max_agent_runs must be >= 1"},
		{e.CI.MaxDocs < 1, "egm.ci.max_docs must be >= 1"},
		{e.Drift.MaxDocs < 1, "egm.drift.max_docs must be >= 1"},
		{e.Bootstrap.MaxBytes < 1, "egm.bootstrap.max_bytes must be >= 1"},
		{e.Brainstorm.MaxBytes < 1, "egm.brainstorm.max_bytes must be >= 1"},
		{e.Draft.MaxBytes < 1, "egm.draft.max_bytes must be >= 1"},
		{e.MaxParallel < 1, "egm.max_parallel must be >= 1"},
		{e.MaxSourceBytes < 1 || e.MaxPacketBytes < 1 || e.MaxStdoutBytes < 1, "egm.max_*_bytes must be >= 1"},
	}
	for _, c := range checks {
		if c.bad {
			return fmt.Errorf("%s", c.msg)
		}
	}
	return e.validateValues()
}

// validateValues checks the keys that are not plain bounds.
func (e *EgmConfig) validateValues() error {
	if v := e.Drift.CI; v != "" && v != "warn" && v != "fail" {
		return fmt.Errorf("egm.drift.ci must be warn or fail")
	}
	d, err := time.ParseDuration(e.TimeoutPerReviewer)
	if err != nil || d <= 0 {
		return fmt.Errorf("egm.timeout_per_reviewer must be a positive duration")
	}
	return nil
}
