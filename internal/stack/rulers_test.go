package stack

import (
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

func TestValidateOptionsRuler(t *testing.T) {
	for ruler, wantUsage := range map[string]bool{"": false, "tests": false, "visual": true, "foo": true} {
		err := validateOptions(&Options{Ops: []Op{OpVerify}, Ruler: ruler})
		if got := verify.IsUsageError(err); got != wantUsage {
			t.Fatalf("ruler %q: usage error=%v want %v (%v)", ruler, got, wantUsage, err)
		}
	}
}

func TestVerifyOverridesFromOptions(t *testing.T) {
	o := verifyOverrides(&Options{
		TestCmd: "t", TypecheckCmd: "", TypecheckCmdSet: true, NoLint: true,
		Checks: []config.VerifyCheck{{Name: "a", Cmd: "true"}}, SkipChecks: []string{"b"}, NoChecks: true,
	})
	if !o.TestCmd.Set || o.HiddenTestCmd.Set || !o.TypecheckCmd.Set || o.TypecheckCmd.Cmd != "" || !o.NoLint ||
		len(o.Checks) != 1 || len(o.SkipChecks) != 1 || !o.NoChecks {
		t.Fatalf("overrides=%+v", o)
	}
}

func TestCoverageOptions(t *testing.T) {
	v := 90.0
	o := verifyOverrides(&Options{CoverageCmd: "cov", CoverageCmdSet: true, CoveragePattern: `(\d+)`, CoveragePatternSet: true, MinCoverage: &v})
	if o.CoverageCmd != (verify.CmdOverride{Cmd: "cov", Set: true}) || !o.CoveragePattern.Set || o.MinCoverage == nil || *o.MinCoverage != 90 {
		t.Fatalf("overrides=%+v", o)
	}
	if err := validateOptions(&Options{Ops: []Op{OpVerify}, Ruler: "tests,coverage"}); err != nil {
		t.Fatal(err)
	}
	bad := 101.0
	for _, opts := range []*Options{
		{Ops: []Op{OpVerify}, CoveragePattern: "nogroup", CoveragePatternSet: true},
		{Ops: []Op{OpVerify}, MinCoverage: &bad},
	} {
		if err := validateOptions(opts); !verify.IsUsageError(err) {
			t.Fatalf("opts=%+v: err=%v, want usage error", opts, err)
		}
	}
}
