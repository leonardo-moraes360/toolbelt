package main

import (
	"strings"
	"testing"
)

// TestExitCodeTable has one case per row of the stable exit-code table in
// docs/cli.md (item J2, J5, J6).
func TestExitCodeTable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	repo := envelopeRepo(t)

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"success", []string{"version"}, 0},
		{"unknown flag", []string{"drift", "--not-a-real-flag"}, 2},
		{"bad argument count", []string{"workspace", "remove"}, 2},
		{"unknown subcommand", []string{"mem", "no-such-subcommand"}, 2},
		{"unknown top-level command", []string{"no-such-command"}, 2},
		{"invalid ruler", []string{"verify", "run", "--ruler", "bogus", "--dry-run"}, 2},
		{"required flag missing", []string{"lab", "new", "x"}, 2},
		{"runtime error", []string{"mem", "show", "nope"}, 1},
		{"verify gate fails", []string{"verify", "run", "--test-cmd", "false", "--max-iter", "1", "--no-build", "--no-lint"}, 1},
		{"verify command not found", []string{"verify", "run", "--test-cmd", "tb-no-such-binary-xyz", "--no-build", "--no-lint"}, 127},
		{"sandbox timeout", []string{"sandbox", "run", "--allow-unsandboxed", "--timeout", "300ms", "--", "sleep", "5"}, 124},
		{"sandbox timeout json", []string{"--json", "sandbox", "run", "--allow-unsandboxed", "--timeout", "300ms", "--", "sleep", "5"}, 124},
		{"sandbox child code", []string{"sandbox", "run", "--allow-unsandboxed", "--", "sh", "-c", "exit 7"}, 7},
		{"sandbox child code json", []string{"--json", "sandbox", "run", "--allow-unsandboxed", "--", "sh", "-c", "exit 7"}, 7},
		{"sandbox command not found", []string{"sandbox", "run", "--allow-unsandboxed", "tb-no-such-binary-xyz"}, 127},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, code := runTBJSON(t, bin, repo, nil, tc.args...)
			if code != tc.want {
				t.Fatalf("tb %s: exit %d, want %d\n%s", strings.Join(tc.args, " "), code, tc.want, out)
			}
		})
	}
}

func TestSandboxJSONTimedOut(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	out, code := runTBJSON(t, bin, t.TempDir(), nil, "--json", "sandbox", "run", "--allow-unsandboxed", "--timeout", "300ms", "--", "sleep", "5")
	if code != 124 {
		t.Fatalf("exit %d, want 124", code)
	}
	if !strings.Contains(string(out), `"timed_out":true`) {
		t.Fatalf("timed_out not true: %s", out)
	}
}
