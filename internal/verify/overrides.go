package verify

import (
	"fmt"
	"slices"

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
}

// MergeFlags is the one merge of file config and caller overrides into cfg:
// non-empty file commands replace cfg's defaults, then o applies. vc and o
// may be nil.
func MergeFlags(cfg *Config, vc *config.VerifyConfig, o *FlagOverrides) error {
	if vc != nil {
		mergeFileConfig(cfg, vc)
	}
	if o != nil {
		if err := applyOverrides(cfg, o); err != nil {
			return err
		}
	}
	return ValidateChecks(cfg.Checks)
}

func mergeFileConfig(cfg *Config, vc *config.VerifyConfig) {
	setIfNonEmpty(&cfg.VisibleTest, vc.TestCmd)
	setIfNonEmpty(&cfg.HiddenTest, vc.HiddenTestCmd)
	setIfNonEmpty(&cfg.BuildCmd, vc.BuildCmd)
	setIfNonEmpty(&cfg.LintCmd, vc.LintCmd)
	setIfNonEmpty(&cfg.TypecheckCmd, vc.TypecheckCmd)
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
	return applyCheckOverrides(cfg, o)
}

func applyCheckOverrides(cfg *Config, o *FlagOverrides) error {
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
