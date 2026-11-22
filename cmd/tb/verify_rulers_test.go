package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

// runTBIsolated runs tb with an empty user config dir, so the developer's
// global config never leaks into verify's R0 merge. stdout and stderr are
// returned apart: text mode prints warnings on stderr.
func runTBIsolated(t *testing.T, tbBin, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(tbBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+t.TempDir())
	var outB, errB strings.Builder
	cmd.Stdout, cmd.Stderr = &outB, &errB
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("tb %v: %v", args, err)
	}
	return outB.String(), errB.String(), code
}

// rulersRepo makes base and head commits; baseCfg (if non-empty) is the
// .toolbelt/config.yaml committed at base.
func rulersRepo(t *testing.T, baseCfg string) string {
	t.Helper()
	dir := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init")
	gitRun("config", "user.email", "t@t")
	gitRun("config", "user.name", "t")
	if baseCfg != "" {
		writeRepoConfig(t, dir, baseCfg)
	}
	writeTestFile(t, filepath.Join(dir, "f.txt"), "a\n")
	gitRun("add", ".")
	gitRun("commit", "-m", "base")
	writeTestFile(t, filepath.Join(dir, "f.txt"), "b\n")
	writeTestFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: change\nscope:\n  - f.txt\n  - rd.md\n---\n")
	gitRun("add", ".")
	gitRun("commit", "-m", "head")
	return dir
}

func gitCommitAll(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", msg}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func writeRepoConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), content)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func decodeResult(t *testing.T, out string) verify.Result {
	t.Helper()
	var res verify.Result
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	return res
}

func TestVerifyCLIUnknownRulerExit2(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "")
	marker := filepath.Join(dir, "ran.flag")
	for _, ruler := range []string{"foo", "visual,tests", "coverage"} {
		_, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--ruler", ruler, "--test-cmd", "touch "+marker, "--max-iter", "1")
		if code != 2 {
			t.Fatalf("--ruler %s: exit=%d want 2\n%s", ruler, code, stderr)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("--ruler %s ran the test command", ruler)
		}
	}
	_, stderr, _ := runTBIsolated(t, tbBin, dir, "verify", "run", "--ruler", "foo")
	if !strings.Contains(stderr, `invalid ruler "foo"`) {
		t.Fatalf("stderr=%q", stderr)
	}
	// Other errors keep exit 1.
	if _, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--base", "no-such-ref", "--test-cmd", "true"); code != 1 {
		t.Fatalf("bad ref: exit=%d want 1\n%s", code, stderr)
	}
}

func TestVerifyCLITypecheckFailEscalates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "")
	out, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--json", "--ruler", "tests",
		"--typecheck-cmd", "echo type-error-xyz; false", "--test-cmd", "true", "--max-iter", "2")
	if code != 1 {
		t.Fatalf("exit=%d want 1\n%s%s", code, out, stderr)
	}
	res := decodeResult(t, out)
	if res.Verdict != verify.VerdictEscalate || !res.TypecheckRan || res.TypecheckPassed == nil || *res.TypecheckPassed {
		t.Fatalf("want ESCALATE with typecheck_passed=false: %+v", res)
	}
	if !strings.HasPrefix(res.Reason, "typecheck failed") || !strings.Contains(res.Reason, "type-error-xyz") {
		t.Fatalf("reason=%q", res.Reason)
	}
	out, _, code = runTBIsolated(t, tbBin, dir, "verify", "run", "--json", "--typecheck-cmd", "false", "--no-typecheck", "--test-cmd", "true", "--max-iter", "1")
	if res := decodeResult(t, out); code != 0 || res.Verdict != verify.VerdictPass || res.TypecheckRan {
		t.Fatalf("--no-typecheck: exit=%d %+v", code, res)
	}
}

func TestVerifyCLIChecksFlags(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "verify:\n  test_cmd: \"true\"\n  checks:\n    - name: cfg-bad\n      cmd: \"false\"\n")
	base := []string{"verify", "run", "--json", "--max-iter", "1"}

	out, _, code := runTBIsolated(t, tbBin, dir, append(base, "--check", "good=true", "--check", "bad=echo check-out-xyz; false")...)
	res := decodeResult(t, out)
	if code != 1 || res.Verdict != verify.VerdictEscalate || len(res.Checks) != 3 {
		t.Fatalf("exit=%d %+v", code, res)
	}
	if !res.Checks[1].Passed || res.Checks[2].Passed || !strings.Contains(res.Checks[2].Reason, "check-out-xyz") {
		t.Fatalf("checks=%+v", res.Checks)
	}

}

func TestVerifyCLIChecksOverrideAndSkip(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "verify:\n  test_cmd: \"true\"\n  checks:\n    - name: cfg-bad\n      cmd: \"false\"\n")
	base := []string{"verify", "run", "--json", "--max-iter", "1"}

	out, _, code := runTBIsolated(t, tbBin, dir, append(base, "--check", "cfg-bad=true")...)
	if res := decodeResult(t, out); code != 0 || res.Verdict != verify.VerdictPass || len(res.Checks) != 1 || !res.Checks[0].Passed {
		t.Fatalf("--check override: exit=%d %+v", code, res)
	}

	out, _, code = runTBIsolated(t, tbBin, dir, append(base, "--no-check", "cfg-bad")...)
	if res := decodeResult(t, out); code != 0 || res.Verdict != verify.VerdictPass || len(res.Checks) != 0 {
		t.Fatalf("--no-check: exit=%d %+v", code, res)
	}

	out, _, code = runTBIsolated(t, tbBin, dir, append(base, "--no-checks")...)
	if res := decodeResult(t, out); code != 0 || res.Verdict != verify.VerdictPass {
		t.Fatalf("--no-checks: exit=%d %+v", code, res)
	}

	for _, bad := range [][]string{{"--no-check", "nope"}, {"--check", "missing-equals"}} {
		if _, stderr, code := runTBIsolated(t, tbBin, dir, append(base, bad...)...); code != 2 {
			t.Fatalf("%v: exit=%d want 2\n%s", bad, code, stderr)
		}
	}
}

// Acceptance 3: a change editing verify.test_cmd is caught by guard and
// verify aborts; an uncommitted edit is ignored in favor of the base value.
func TestVerifyCLIConfigEditAbortsAndBaseWins(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "verify:\n  test_cmd: \"true\"\n")
	marker := filepath.Join(dir, "head-cmd.flag")

	writeRepoConfig(t, dir, "verify:\n  test_cmd: \"echo base-ignored\"\n")
	out, _, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--json", "--dry-run")
	res := decodeResult(t, out)
	if code != 0 || res.ConfigSource != verify.ConfigSourceBase || res.Plan == nil || res.Plan.Commands["test"] != "true" {
		t.Fatalf("worktree edit must not change the plan: exit=%d %+v", code, res)
	}

	writeRepoConfig(t, dir, "verify:\n  test_cmd: \"touch "+marker+"\"\n")
	gitCommitAll(t, dir, "rewrite gate")
	out, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--json", "--max-iter", "1", "--ticket", "rd.md")
	res = decodeResult(t, out)
	if code != 1 || res.Verdict == verify.VerdictPass || !strings.Contains(res.Reason, "verify_config_changed") {
		t.Fatalf("want guard abort: exit=%d %+v\n%s", code, res, stderr)
	}
	if !strings.Contains(res.Reason, "verify.test_cmd") {
		t.Fatalf("reason should name the key: %q", res.Reason)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the head's test_cmd must not run")
	}
}

func TestVerifyCLIBaseEqualsHeadDirtyWarns(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "verify:\n  test_cmd: \"true\"\n")
	writeRepoConfig(t, dir, "verify:\n  test_cmd: \"echo worktree-cmd\"\n")
	args := []string{"verify", "run", "--base", "HEAD", "--head", "HEAD", "--dry-run"}

	out, _, code := runTBIsolated(t, tbBin, dir, append(args, "--json")...)
	res := decodeResult(t, out)
	if code != 0 || res.ConfigSource != verify.ConfigSourceWorktree || res.Plan.Commands["test"] != "echo worktree-cmd" {
		t.Fatalf("exit=%d %+v", code, res)
	}
	if len(res.Warnings) < 2 || !strings.Contains(strings.Join(res.Warnings, "\n"), "base resolves to head") {
		t.Fatalf("warnings=%q", res.Warnings)
	}

	stdout, stderr, code := runTBIsolated(t, tbBin, dir, args...)
	if code != 0 || !strings.Contains(stderr, "Warning: base resolves to head") || strings.Contains(stdout, "Warning") {
		t.Fatalf("text mode warnings go to stderr: exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

func TestStackCLIVerifyRulerAndTypecheck(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "")
	stackArgs := []string{"stack", "verify", "--ticket", "rd.md", "--test-cmd", "true", "--max-iter", "1"}

	out, stderr, code := runTBIsolated(t, tbBin, dir, append(stackArgs, "--ruler", "tests", "--typecheck-cmd", "false")...)
	if code != 1 {
		t.Fatalf("exit=%d want 1\n%s%s", code, out, stderr)
	}
	var sr struct {
		OK    bool `json:"ok"`
		Steps []struct {
			ExitCode int           `json:"exit_code"`
			Reason   string        `json:"reason"`
			Result   verify.Result `json:"result"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(out), &sr); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if sr.OK || len(sr.Steps) != 1 || sr.Steps[0].Result.Verdict != verify.VerdictEscalate || !strings.HasPrefix(sr.Steps[0].Reason, "typecheck failed") {
		t.Fatalf("stack: %s", out)
	}

	out, _, code = runTBIsolated(t, tbBin, dir, append(stackArgs, "--check", "ok=true", "--no-typecheck")...)
	if code != 0 || !strings.Contains(out, `"name": "ok"`) {
		t.Fatalf("stack --check: exit=%d\n%s", code, out)
	}

	for _, ruler := range []string{"visual", "foo"} {
		if _, stderr, code := runTBIsolated(t, tbBin, dir, append(stackArgs, "--ruler", ruler)...); code != 2 {
			t.Fatalf("stack --ruler %s: exit=%d want 2\n%s", ruler, code, stderr)
		}
	}
}
