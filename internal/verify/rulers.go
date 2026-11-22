package verify

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
	"github.com/leonardo-moraes360/toolbelt/internal/guard"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

const (
	RulerTests  = "tests"
	RulerVisual = "visual"
)

// SkipKind qualifies a SKIPPED verdict for composition (R1).
type SkipKind string

const (
	// SkipNotApplicable: the ruler has nothing to judge; ignored when merged.
	SkipNotApplicable SkipKind = "not_applicable"
	// SkipUnavailable: the ruler should judge but cannot; ranks above PASS.
	SkipUnavailable SkipKind = "unavailable"
)

// UsageError is a caller mistake (bad --ruler, unknown --no-check); the CLI
// maps it to exit code 2.
type UsageError struct {
	Msg string
}

func (e *UsageError) Error() string { return e.Msg }

func (e *UsageError) ExitCode() int { return 2 }

func IsUsageError(err error) bool {
	var e *UsageError
	return errors.As(err, &e)
}

// RunState is what the orchestrator resolves once and every ruler shares.
type RunState struct {
	Config       *Config
	BaseSHA      string
	HeadSHA      string
	WorkDir      string
	Guard        *guard.CheckResult
	GuardAborted bool
	GuardReason  string
	ConfigSource string
	Warnings     []string
	Overrides    []string
	// BaseThresholds is loaded in prepareRun when coverage is selected, so a
	// malformed file aborts before any ruler runs.
	BaseThresholds *BaseThresholds
}

// RulerResult is one ruler's outcome. Payload is what that ruler prints on
// its own (*Result for tests, *VisualReport for visual).
type RulerResult struct {
	Name     string
	Verdict  Verdict
	SkipKind SkipKind
	// ExitCode is this ruler's process exit code: 0 for PASS/SKIPPED, 127
	// for a missing command, 1 otherwise.
	ExitCode int
	Payload  any
}

type Ruler interface {
	Name() string
	Run(ctx context.Context, st *RunState) (*RulerResult, error)
}

// MultiResult is the composed outcome; Payload gives the JSON to print.
type MultiResult struct {
	Verdict      Verdict        `json:"verdict"`
	SkipKind     SkipKind       `json:"skip_kind,omitempty"`
	DryRun       bool           `json:"dry_run,omitempty"`
	ConfigSource string         `json:"config_source,omitempty"`
	Warnings     []string       `json:"warnings,omitempty"`
	Overrides    []string       `json:"overrides,omitempty"`
	Rulers       map[string]any `json:"rulers"`
	Results      []*RulerResult `json:"-"`
}

// Payload is the single ruler's payload unchanged (G5), or the composed
// {verdict, rulers: {...}} shape for several.
func (m *MultiResult) Payload() any {
	if len(m.Results) == 1 {
		return m.Results[0].Payload
	}
	return m
}

// FirstReason is the reason of the first ruler, in ruler order, that did
// not pass; "" when none has one.
func (m *MultiResult) FirstReason() string {
	for _, r := range m.Results {
		if r.Verdict == VerdictPass || (r.Verdict == VerdictSkipped && r.SkipKind == SkipNotApplicable) {
			continue
		}
		switch p := r.Payload.(type) {
		case *Result:
			if p.Reason != "" {
				return p.Reason
			}
		case *CoverageReport:
			if p.Reason != "" {
				return p.Reason
			}
		}
	}
	return ""
}

func (m *MultiResult) ExitCode() int {
	return ExitCode(m.Verdict, m.Results)
}

// ExitCode: 0 for PASS or SKIPPED, else the first non-zero ruler code in
// ruler order (127 missing command, 1 the rest).
func ExitCode(verdict Verdict, results []*RulerResult) int {
	if verdict == VerdictPass || verdict == VerdictSkipped {
		return 0
	}
	for _, r := range results {
		if r.ExitCode != 0 {
			return r.ExitCode
		}
	}
	return 1
}

// ParseRulers parses --ruler: a comma list of tests|coverage|visual,
// deduplicated and run in that order (tests before coverage); empty means
// tests; visual cannot be combined yet.
func ParseRulers(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return []string{RulerTests}, nil
	}
	var names []string
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		if name != RulerTests && name != RulerCoverage && name != RulerVisual {
			return nil, &UsageError{Msg: fmt.Sprintf("invalid ruler %q: tests, coverage or visual", name)}
		}
		if !containsString(names, name) {
			names = append(names, name)
		}
	}
	if len(names) > 1 && containsString(names, RulerVisual) {
		return nil, &UsageError{Msg: "ruler visual cannot be combined with other rulers"}
	}
	// R1 fixes the order: tests, then coverage.
	slices.SortStableFunc(names, func(a, b string) int { return rulerOrder[a] - rulerOrder[b] })
	return names, nil
}

var rulerOrder = map[string]int{RulerTests: 0, RulerCoverage: 1, RulerVisual: 2}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var verdictRank = map[Verdict]int{VerdictPass: 1, VerdictSkipped: 2, VerdictFail: 3, VerdictEscalate: 4}

// MergeVerdicts composes ruler verdicts (R1): ESCALATE > FAIL >
// SKIPPED(unavailable) > PASS; not_applicable rulers are ignored, and only
// those gives SKIPPED(not_applicable).
func MergeVerdicts(results []*RulerResult) (Verdict, SkipKind) {
	best, kind := Verdict(""), SkipKind("")
	for _, r := range results {
		if r.Verdict == VerdictSkipped && r.SkipKind == SkipNotApplicable {
			continue
		}
		if best == "" || verdictRank[r.Verdict] > verdictRank[best] {
			best = r.Verdict
		}
	}
	switch best {
	case "":
		return VerdictSkipped, SkipNotApplicable
	case VerdictSkipped:
		kind = SkipUnavailable
	}
	return best, kind
}

func rulerFor(name string) Ruler {
	switch name {
	case RulerVisual:
		return visualRuler{}
	case RulerCoverage:
		return coverageRuler{}
	}
	return testsRuler{}
}

// RunRulers resolves refs, loads the base config (R0), runs guard and
// attaches the head worktree once, then runs each ruler in order on that
// shared state. names must come from ParseRulers.
func RunRulers(ctx context.Context, cfg *Config, names []string) (*MultiResult, error) {
	if len(names) == 0 {
		names = []string{RulerTests}
	}
	st := &RunState{Config: cfg, WorkDir: cfg.RepoDir}
	cleanup := func() {}
	if !(len(names) == 1 && names[0] == RulerVisual) {
		var err error
		ctx, st, cleanup, err = prepareRun(ctx, cfg, containsString(names, RulerCoverage))
		if err != nil {
			return nil, err
		}
	}
	defer cleanup()
	multi := &MultiResult{Rulers: map[string]any{}, DryRun: cfg.DryRun, ConfigSource: st.ConfigSource, Warnings: st.Warnings, Overrides: st.Overrides}
	for _, name := range names {
		rr, err := rulerFor(name).Run(ctx, st)
		if err != nil {
			return nil, err
		}
		multi.Results = append(multi.Results, rr)
		multi.Rulers[name] = rr.Payload
	}
	if cfg.DryRun {
		multi.Verdict = VerdictSkipped
		return multi, nil
	}
	multi.Verdict, multi.SkipKind = MergeVerdicts(multi.Results)
	return multi, nil
}

// prepareRun is the shared setup: refs, R0, guard, head worktree. On a guard
// abort no worktree is attached and WorkDir stays the repository.
func prepareRun(ctx context.Context, cfg *Config, wantCoverage bool) (context.Context, *RunState, func(), error) {
	noop := func() {}
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 3
	}
	base, head, err := gitref.ResolvePair(cfg.RepoDir, cfg.BaseRef, cfg.HeadRef)
	if err != nil {
		return ctx, nil, noop, err
	}
	cfg.BaseRef, cfg.HeadRef = base, head
	st := &RunState{Config: cfg, BaseSHA: base, HeadSHA: head, WorkDir: cfg.RepoDir}
	if err := applyRunConfig(cfg, st); err != nil {
		return ctx, nil, noop, err
	}
	st.Guard, st.GuardAborted, st.GuardReason, err = applyGuard(cfg)
	if err != nil {
		return ctx, nil, noop, err
	}
	if st.GuardAborted {
		return ctx, st, noop, nil
	}
	if wantCoverage {
		th, err := LoadBaseThresholds(cfg.RepoDir, base)
		if err != nil {
			return ctx, nil, noop, err
		}
		st.BaseThresholds = &th
		dropNoopMinCoverage(cfg, st, th)
	}
	hw, err := attachHeadWorkDir(cfg.RepoDir, head)
	if err != nil {
		return ctx, nil, noop, err
	}
	st.WorkDir, st.HeadSHA = hw.WorkDir, hw.HeadSHA
	cleanup := hw.Cleanup
	if hw.Temporary {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		cleanup = func() { stop(); hw.Cleanup() }
	}
	if hw.DirtyWarn {
		log.Warn("dirty worktree with head equal to HEAD; build/lint/test may include uncommitted changes",
			"workdir", hw.WorkDir, "head_sha", hw.HeadSHA)
	}
	return ctx, st, cleanup, nil
}

// applyRunConfig applies R0 when the caller supplied a trusted config (the
// verify and env sections; -c skips the repository file), then the caller's
// overrides. Refs are already resolved to SHAs.
func applyRunConfig(cfg *Config, st *RunState) error {
	if cfg.TrustedConfig == nil {
		if cfg.Overrides == nil {
			return nil
		}
		err := MergeFlags(cfg, nil, cfg.Overrides)
		st.Overrides = cfg.AppliedOverrides
		return err
	}
	rc, err := LoadVerifyConfig(cfg.RepoDir, cfg.BaseRef, cfg.HeadRef, cfg.TrustedConfig, config.File() != "")
	if err != nil {
		return err
	}
	cfg.EnvAllow = append(cfg.EnvAllow, rc.Env.Allow...)
	cfg.EnvPass = append(cfg.EnvPass, rc.Env.Pass...)
	st.ConfigSource, st.Warnings = rc.Source, rc.Warnings
	err = MergeFlags(cfg, &rc.Verify, cfg.Overrides)
	st.Overrides = cfg.AppliedOverrides
	return err
}

type testsRuler struct{}

func (testsRuler) Name() string { return RulerTests }

func (testsRuler) Run(ctx context.Context, st *RunState) (*RulerResult, error) {
	res, err := runTests(ctx, st)
	rr := &RulerResult{Name: RulerTests}
	switch {
	case err != nil && IsMissingCommand(err) && res != nil:
		rr.ExitCode = 127
	case err != nil:
		return nil, err
	}
	rr.Verdict, rr.SkipKind, rr.Payload = res.Verdict, res.SkipKind, res
	if rr.ExitCode == 0 && rr.Verdict != VerdictPass && rr.Verdict != VerdictSkipped {
		rr.ExitCode = 1
	}
	return rr, nil
}

type visualRuler struct{}

func (visualRuler) Name() string { return RulerVisual }

func (visualRuler) Run(ctx context.Context, st *RunState) (*RulerResult, error) {
	vc := VisualConfig{Threshold: DefaultSimilarityThreshold, SkipOK: true}
	if st.Config.Visual != nil {
		vc = *st.Config.Visual
	}
	rep, err := RunVisual(ctx, st.Config.RepoDir, vc)
	if err != nil {
		return nil, err
	}
	rr := &RulerResult{Name: RulerVisual, Verdict: rep.Verdict, Payload: rep}
	switch rep.Verdict {
	case VerdictSkipped:
		rr.SkipKind = SkipUnavailable
	case VerdictPass:
	default:
		rr.ExitCode = 1
	}
	return rr, nil
}

// dropNoopMinCoverage unlists min_coverage when the flag does not raise the
// threshold above the base value: it changed nothing.
func dropNoopMinCoverage(cfg *Config, st *RunState, th BaseThresholds) {
	if cfg.MinCoverage == nil || th.MinCoverage == nil || *cfg.MinCoverage > *th.MinCoverage {
		return
	}
	cfg.AppliedOverrides = slices.DeleteFunc(slices.Clone(cfg.AppliedOverrides), func(k string) bool { return k == "min_coverage" })
	st.Overrides = cfg.AppliedOverrides
}
