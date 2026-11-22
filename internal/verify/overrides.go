package verify

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

// CmdOverride is one command flag or MCP argument. Set distinguishes an
// explicit empty value (disables the step) from an absent one.
type CmdOverride struct {
	Cmd string
	Set bool
}

// FlagOverrides holds what the caller (CLI flags, MCP arguments, tb stack)
// layers over the file config. They come from the caller, not the change, so
// they apply after R0.
type FlagOverrides struct {
	// TestCmd replaces the visible test command only when non-empty: an
	// empty test command would pass vacuously.
	TestCmd CmdOverride
	// HiddenTestCmd replaces the hidden command whenever Set; empty restores
	// toolchain detection under tests/hidden.
	HiddenTestCmd CmdOverride
	BuildCmd      CmdOverride
	LintCmd       CmdOverride
	TypecheckCmd  CmdOverride
	NoBuild       bool
	NoLint        bool
	NoTypecheck   bool
	// Checks adds checks, or replaces the command of one with the same name.
	Checks     []config.VerifyCheck
	SkipChecks []string
	NoChecks   bool
	// CoverageCmd and CoveragePattern replace the file config whenever Set.
	// MinCoverage only raises the base threshold (the ruler takes the max).
	CoverageCmd     CmdOverride
	CoveragePattern CmdOverride
	MinCoverage     *float64
}

// MergeFlags is the one merge of file config and caller overrides into cfg:
// non-empty file commands replace cfg's defaults, then o applies. vc and o
// may be nil. cfg.AppliedOverrides is set to the overrides that changed the
// file config's effective values.
func MergeFlags(cfg *Config, vc *config.VerifyConfig, o *FlagOverrides) error {
	if vc != nil {
		mergeFileConfig(cfg, vc)
	}
	cfg.AppliedOverrides = nil
	if o != nil {
		before := *cfg
		before.Checks = slices.Clone(cfg.Checks)
		if err := applyOverrides(cfg, o); err != nil {
			return err
		}
		cfg.AppliedOverrides = appliedOverrides(&before, cfg, o)
	}
	if err := ValidateChecks(cfg.Checks); err != nil {
		return err
	}
	return ValidateCoveragePattern(cfg.CoveragePattern)
}

// appliedOverrides names each key whose value o changed from before, and
// each skip toggle that disabled a configured step, sorted.
func appliedOverrides(before, after *Config, o *FlagOverrides) []string {
	var out []string
	cmd := func(key string, b, a string) {
		if b != a {
			out = append(out, key)
		}
	}
	cmd("test_cmd", before.VisibleTest, after.VisibleTest)
	cmd("hidden_test_cmd", before.HiddenTest, after.HiddenTest)
	gate := func(key, toggle string, b, a string, disable bool) {
		switch {
		case b == a:
		case disable:
			out = append(out, toggle)
		default:
			out = append(out, key)
		}
	}
	cmd("coverage_cmd", before.CoverageCmd, after.CoverageCmd)
	cmd("coverage_pattern", before.CoveragePattern, after.CoveragePattern)
	if o.MinCoverage != nil {
		// Listed here; the run drops it once the base threshold shows the
		// flag does not raise it (dropNoopMinCoverage).
		out = append(out, "min_coverage")
	}
	gate("build_cmd", "no_build", before.BuildCmd, after.BuildCmd, o.NoBuild)
	gate("lint_cmd", "no_lint", before.LintCmd, after.LintCmd, o.NoLint)
	gate("typecheck_cmd", "no_typecheck", before.TypecheckCmd, after.TypecheckCmd, o.NoTypecheck)
	if o.NoChecks {
		if len(before.Checks) > 0 {
			out = append(out, "no_checks")
		}
	} else {
		for _, add := range o.Checks {
			i := slices.IndexFunc(before.Checks, func(c config.VerifyCheck) bool { return c.Name == add.Name })
			if i < 0 || before.Checks[i].Cmd != add.Cmd {
				out = append(out, "checks:"+add.Name)
			}
		}
		for _, name := range o.SkipChecks {
			if slices.ContainsFunc(before.Checks, func(c config.VerifyCheck) bool { return c.Name == name }) {
				out = append(out, "skip_checks:"+name)
			}
		}
	}
	sort.Strings(out)
	return slices.Compact(out)
}

func mergeFileConfig(cfg *Config, vc *config.VerifyConfig) {
	setIfNonEmpty(&cfg.VisibleTest, vc.TestCmd)
	setIfNonEmpty(&cfg.HiddenTest, vc.HiddenTestCmd)
	setIfNonEmpty(&cfg.BuildCmd, vc.BuildCmd)
	setIfNonEmpty(&cfg.LintCmd, vc.LintCmd)
	setIfNonEmpty(&cfg.TypecheckCmd, vc.TypecheckCmd)
	setIfNonEmpty(&cfg.CoverageCmd, vc.CoverageCmd)
	setIfNonEmpty(&cfg.CoveragePattern, vc.CoveragePattern)
	if len(vc.Checks) > 0 {
		cfg.Checks = append([]config.VerifyCheck(nil), vc.Checks...)
	}
}

func applyOverrides(cfg *Config, o *FlagOverrides) error {
	if o.TestCmd.Set {
		setIfNonEmpty(&cfg.VisibleTest, o.TestCmd.Cmd)
	}
	if o.HiddenTestCmd.Set {
		cfg.HiddenTest = o.HiddenTestCmd.Cmd
	}
	applyGateOverride(&cfg.BuildCmd, o.BuildCmd, o.NoBuild)
	applyGateOverride(&cfg.LintCmd, o.LintCmd, o.NoLint)
	applyGateOverride(&cfg.TypecheckCmd, o.TypecheckCmd, o.NoTypecheck)
	if err := applyCoverageOverrides(cfg, o); err != nil {
		return err
	}
	return applyCheckOverrides(cfg, o)
}

// applyCoverageOverrides reports caller mistakes (bad pattern, floor out of
// range) as usage errors, like a malformed --check.
func applyCoverageOverrides(cfg *Config, o *FlagOverrides) error {
	if o.CoverageCmd.Set {
		cfg.CoverageCmd = o.CoverageCmd.Cmd
	}
	if o.CoveragePattern.Set {
		if err := ValidateCoveragePattern(o.CoveragePattern.Cmd); err != nil {
			return &UsageError{Msg: "--coverage-pattern: " + strings.TrimPrefix(err.Error(), "verify.coverage_pattern: ")}
		}
		cfg.CoveragePattern = o.CoveragePattern.Cmd
	}
	if o.MinCoverage != nil {
		if !validPercent(*o.MinCoverage) {
			return &UsageError{Msg: fmt.Sprintf("--min-coverage %v: want a number in [0, 100]", *o.MinCoverage)}
		}
		v := *o.MinCoverage
		cfg.MinCoverage = &v
	}
	return nil
}

func applyCheckOverrides(cfg *Config, o *FlagOverrides) error {
	// Caller-supplied checks are a usage error when malformed, like a bad
	// --ruler; only file config errors stay plain errors.
	if err := ValidateChecks(o.Checks); err != nil {
		return &UsageError{Msg: "--check: " + strings.TrimPrefix(err.Error(), "verify.checks: ")}
	}
	if o.NoChecks {
		cfg.Checks = nil
		return nil
	}
	for _, add := range o.Checks {
		i := slices.IndexFunc(cfg.Checks, func(c config.VerifyCheck) bool { return c.Name == add.Name })
		if i >= 0 {
			cfg.Checks[i].Cmd = add.Cmd
			continue
		}
		cfg.Checks = append(cfg.Checks, add)
	}
	for _, name := range o.SkipChecks {
		i := slices.IndexFunc(cfg.Checks, func(c config.VerifyCheck) bool { return c.Name == name })
		if i < 0 {
			return &UsageError{Msg: fmt.Sprintf("--no-check %q: no such check", name)}
		}
		cfg.Checks = slices.Delete(cfg.Checks, i, i+1)
	}
	return nil
}

func applyGateOverride(dst *string, o CmdOverride, disable bool) {
	switch {
	case disable || (o.Set && o.Cmd == ""):
		*dst = ""
	case o.Set:
		*dst = o.Cmd
	}
}

func setIfNonEmpty(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}
