package egm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

const maxRepoConfigBytes = 64 << 10

// Settings is the effective egm config for one run (R34).
type Settings struct {
	config.EgmConfig
	// GateDocGlobs is DocGlobs plus the repository's extra items, which only
	// apply to the gate selection (R33), never to the source exclusion (R7).
	GateDocGlobs []string
	Warnings     []Warning
}

// LoadUserConfig reads the user config: the -c file when set, else
// config.GlobalConfigPath(); a missing default file means the defaults (R31).
// It never reads the merged config nor the repository's file.
func LoadUserConfig(override string) (config.EgmConfig, error) {
	path := override
	if path == "" {
		path = config.GlobalConfigPath()
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		if override == "" && errors.Is(err, os.ErrNotExist) {
			return config.DefaultEgmConfig(), nil
		}
		return config.EgmConfig{}, fmt.Errorf("loading user config %s: %w", path, err)
	}
	if err := cfg.Egm.Validate(); err != nil {
		return config.EgmConfig{}, err
	}
	return cfg.Egm, nil
}

// repoEgm holds the keys of the repository's egm: section; nil means absent.
type repoEgm struct {
	AgentCmd       any `yaml:"agent_cmd"`
	Path           any `yaml:"path"`
	Env            any `yaml:"env"`
	Home           any `yaml:"home"`
	Vendor         any `yaml:"vendor"`
	Output         any `yaml:"output"`
	ToolsDisabled  any `yaml:"tools_disabled"`
	ToolsCheck     any `yaml:"tools_check"`
	ForbiddenFlags any `yaml:"forbidden_flags"`

	DocTypes       any `yaml:"doc_types"`
	RefsIgnore     any `yaml:"refs_ignore"`
	DetailHeadings any `yaml:"detail_headings"`

	DocGlobs            []string `yaml:"doc_globs"`
	RequireFullCoverage *bool    `yaml:"require_full_coverage"`
	CleanRounds         *int     `yaml:"clean_rounds"`
	Attest              *struct {
		MinQuotes *int `yaml:"min_quotes"`
	} `yaml:"attest"`
	Match *struct {
		Sim  *float64 `yaml:"sim"`
		Stem *int     `yaml:"stem"`
		Gap  *int     `yaml:"gap"`
	} `yaml:"match"`

	MaxAgentRuns       *int    `yaml:"max_agent_runs"`
	MaxRounds          *int    `yaml:"max_rounds"`
	MaxParallel        *int    `yaml:"max_parallel"`
	TimeoutPerReviewer *string `yaml:"timeout_per_reviewer"`
	MaxSourceBytes     *int64  `yaml:"max_source_bytes"`
	MaxPacketBytes     *int64  `yaml:"max_packet_bytes"`
	MaxStdoutBytes     *int64  `yaml:"max_stdout_bytes"`
}

// ApplyRepoConfig overlays the repository's egm: section on the user config
// under R34: execution keys ignored, limits only go down, policy only
// hardens. raw is the file's bytes (nil when absent).
func ApplyRepoConfig(user *config.EgmConfig, raw []byte) Settings {
	s := Settings{EgmConfig: *user, GateDocGlobs: append([]string(nil), user.DocGlobs...)}
	if raw == nil {
		return s
	}
	var doc struct {
		Egm *repoEgm `yaml:"egm"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		s.untrusted("unparsable .toolbelt/config.yaml ignored")
		return s
	}
	if doc.Egm == nil {
		return s
	}
	r := doc.Egm
	s.applyUserOnly(r)
	s.applyLimits(r)
	s.applyPolicy(r)
	return s
}

func (s *Settings) untrusted(msg string) {
	s.Warnings = append(s.Warnings, Warning{Code: "config_untrusted", Message: msg, Path: ".toolbelt/config.yaml"})
}

func (s *Settings) applyUserOnly(r *repoEgm) {
	keys := map[string]any{
		"agent_cmd": r.AgentCmd, "path": r.Path, "env": r.Env, "home": r.Home, "vendor": r.Vendor,
		"output": r.Output, "tools_disabled": r.ToolsDisabled, "tools_check": r.ToolsCheck,
		"forbidden_flags": r.ForbiddenFlags, "doc_types": r.DocTypes, "refs_ignore": r.RefsIgnore,
		"detail_headings": r.DetailHeadings,
	}
	var present []string
	for k, v := range keys {
		if v != nil {
			present = append(present, k)
		}
	}
	sort.Strings(present)
	for _, k := range present {
		s.untrusted(fmt.Sprintf("egm.%s only counts from the user config; ignored", k))
	}
}

func lowerInt(s *Settings, name string, cur, v *int) {
	if v == nil {
		return
	}
	if *v < 1 || *v > *cur {
		s.untrusted(fmt.Sprintf("egm.%s=%d can only lower %d; capped", name, *v, *cur))
		return
	}
	*cur = *v
}

func lowerInt64(s *Settings, name string, cur, v *int64) {
	if v == nil {
		return
	}
	if *v < 1 || *v > *cur {
		s.untrusted(fmt.Sprintf("egm.%s=%d can only lower %d; capped", name, *v, *cur))
		return
	}
	*cur = *v
}

func (s *Settings) applyLimits(r *repoEgm) {
	lowerInt(s, "max_agent_runs", &s.MaxAgentRuns, r.MaxAgentRuns)
	lowerInt(s, "max_rounds", &s.MaxRounds, r.MaxRounds)
	lowerInt(s, "max_parallel", &s.MaxParallel, r.MaxParallel)
	lowerInt64(s, "max_source_bytes", &s.MaxSourceBytes, r.MaxSourceBytes)
	lowerInt64(s, "max_packet_bytes", &s.MaxPacketBytes, r.MaxPacketBytes)
	lowerInt64(s, "max_stdout_bytes", &s.MaxStdoutBytes, r.MaxStdoutBytes)
	if r.TimeoutPerReviewer != nil {
		cur, _ := time.ParseDuration(s.TimeoutPerReviewer)
		v, err := time.ParseDuration(*r.TimeoutPerReviewer)
		if err != nil || v <= 0 || v > cur {
			s.untrusted("egm.timeout_per_reviewer can only lower the user value; capped")
		} else {
			s.TimeoutPerReviewer = *r.TimeoutPerReviewer
		}
	}
	if s.CleanRounds > s.MaxRounds {
		s.untrusted("egm.max_rounds below clean_rounds; clean_rounds kept, max_rounds raised back")
		s.MaxRounds = s.CleanRounds
	}
}

// stemStrength orders match.stem by strictness: 0 (whole words) is the
// strictest.
func stemStrength(v int) int {
	if v == 0 {
		return 1 << 30
	}
	return v
}

func (s *Settings) applyPolicy(r *repoEgm) {
	if r.CleanRounds != nil {
		if *r.CleanRounds < s.CleanRounds || *r.CleanRounds > s.MaxRounds {
			s.untrusted("egm.clean_rounds can only go up (up to max_rounds); ignored")
		} else {
			s.CleanRounds = *r.CleanRounds
		}
	}
	if r.RequireFullCoverage != nil {
		if *r.RequireFullCoverage {
			s.RequireFullCoverage = true
		} else if s.RequireFullCoverage {
			s.untrusted("egm.require_full_coverage can only become true; ignored")
		}
	}
	if r.Attest != nil && r.Attest.MinQuotes != nil {
		if *r.Attest.MinQuotes < s.Attest.MinQuotes {
			s.untrusted("egm.attest.min_quotes can only go up; ignored")
		} else {
			s.Attest.MinQuotes = *r.Attest.MinQuotes
		}
	}
	s.applyMatch(r)
	s.GateDocGlobs = appendNew(s.GateDocGlobs, r.DocGlobs)
}

func (s *Settings) applyMatch(r *repoEgm) {
	if r.Match == nil {
		return
	}
	m := r.Match
	if m.Sim != nil {
		if !(*m.Sim >= s.Match.Sim && *m.Sim <= 1) { // also rejects NaN
			s.untrusted("egm.match.sim can only go up (max 1); ignored")
		} else {
			s.Match.Sim = *m.Sim
		}
	}
	if m.Stem != nil {
		if *m.Stem < 0 || stemStrength(*m.Stem) < stemStrength(s.Match.Stem) {
			s.untrusted("egm.match.stem can only go up (0 = whole words is the strictest); ignored")
		} else {
			s.Match.Stem = *m.Stem
		}
	}
	if m.Gap != nil {
		if *m.Gap < 0 || *m.Gap > s.Match.Gap {
			s.untrusted("egm.match.gap can only go down; ignored")
		} else {
			s.Match.Gap = *m.Gap
		}
	}
}

func appendNew(list, extra []string) []string {
	for _, x := range extra {
		if !containsStr(list, x) {
			list = append(list, x)
		}
	}
	return list
}

// readRepoConfig reads <root>/.toolbelt/config.yaml with Lstat per component,
// a regular file and at most 64 KiB (R34). A missing file is (nil, nil); an
// unsafe one is (nil, warning).
func readRepoConfig(root string) ([]byte, *Warning) {
	dir := filepath.Join(root, ".toolbelt")
	for _, p := range []string{dir, filepath.Join(dir, "config.yaml")} {
		fi, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, &Warning{Code: "config_untrusted", Message: err.Error(), Path: ".toolbelt/config.yaml"}
		}
		want := fi.IsDir()
		if p != dir {
			want = fi.Mode().IsRegular()
		}
		if !want {
			return nil, &Warning{Code: "config_untrusted", Message: "not a regular file/directory: ignored", Path: ".toolbelt/config.yaml"}
		}
	}
	f, err := os.Open(filepath.Join(dir, "config.yaml")) //nolint:gosec // path checked above
	if err != nil {
		return nil, &Warning{Code: "config_untrusted", Message: err.Error(), Path: ".toolbelt/config.yaml"}
	}
	defer f.Close()
	// The file opened must be the regular file inspected, not a swap.
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return nil, &Warning{Code: "config_untrusted", Message: "not a regular file: ignored", Path: ".toolbelt/config.yaml"}
	}
	b, err := io.ReadAll(io.LimitReader(f, maxRepoConfigBytes+1))
	if err != nil || len(b) > maxRepoConfigBytes {
		return nil, &Warning{Code: "config_untrusted", Message: "larger than 64 KiB or unreadable: ignored", Path: ".toolbelt/config.yaml"}
	}
	return b, nil
}
