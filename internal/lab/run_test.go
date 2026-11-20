package lab

import (
	"runtime"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/lab/scenario"
)

func sqliLab() *Lab {
	return &Lab{
		Name: "sqli", Class: "sql-injection", Tier: TierSandbox,
		Nodes: Nodes{Target: Node{Build: "builtin:sql-injection"}},
	}
}

// TestRunSandboxSQLi is A4: the seed SQLi lab reproduces the attack and the
// paired blue check detects it.
func TestRunSandboxSQLi(t *testing.T) {
	if a := ProbeSandbox(); !a.Available {
		t.Skipf("sandbox tier unavailable here (%s)", a.Remediation)
	}
	v, err := RunSandbox(sqliLab())
	if err != nil {
		t.Fatalf("RunSandbox: %v", err)
	}
	if v.Attack != scenario.AttackReproduced {
		t.Errorf("attack = %q, want reproduced", v.Attack)
	}
	if v.Defense != DefenseDetected {
		t.Errorf("defense = %q, want detected", v.Defense)
	}
	if v.MatchedCheck == "" {
		t.Error("a detected verdict must name the matching check")
	}
}

func TestRunSandboxUnknownClass(t *testing.T) {
	if a := ProbeSandbox(); !a.Available {
		t.Skipf("sandbox tier unavailable here (%s)", a.Remediation)
	}
	l := sqliLab()
	l.Class = "no-such-class"
	if _, err := RunSandbox(l); err == nil {
		t.Error("RunSandbox should error on a class with no scenario")
	}
}

func TestRunSandboxNonLinuxFailsClosed(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("sandbox tier is available on Linux")
	}
	if _, err := RunSandbox(sqliLab()); err == nil {
		t.Error("RunSandbox must fail closed where the sandbox tier is unavailable")
	}
}
