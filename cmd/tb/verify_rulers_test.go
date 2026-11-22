package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

// runTBIsolated runs tb with an empty user config dir, so the developer's
// global config never leaks into verify's R0 merge. stdout and stderr are
// returned apart: text mode prints warnings on stderr.
func runTBIsolated(t *testing.T, tbBin, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runTBWithEnv(t, tbBin, dir, []string{"XDG_CONFIG_HOME=" + t.TempDir()}, args...)
}

// runTBWithEnv runs tb with env appended to the process environment.
func runTBWithEnv(t *testing.T, tbBin, dir string, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(tbBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
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
	for _, ruler := range []string{"foo", "visual,tests", "coverage,visual"} {
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

	for _, bad := range [][]string{
		{"--no-check", "nope"}, {"--check", "missing-equals"},
		{"--check", "Bad_Name=true"}, {"--check", "dup=true", "--check", "dup=false"},
	} {
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
	if code != 0 || res.ConfigSource != "" || res.Plan == nil || res.Plan.Commands["test"] != "true" {
		t.Fatalf("worktree edit must not change the plan: exit=%d %+v", code, res)
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "ignored .toolbelt/config.yaml keys that differ from the base ref: verify.test_cmd") {
		t.Fatalf("warnings=%q", res.Warnings)
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

// R0: with base == HEAD, a dirty edit of the committed file is ignored too.
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
	if code != 0 || res.ConfigSource != "" || res.Plan.Commands["test"] != "true" {
		t.Fatalf("exit=%d %+v", code, res)
	}
	joined := strings.Join(res.Warnings, "\n")
	if len(res.Warnings) != 2 || !strings.Contains(joined, "base resolves to head") || !strings.Contains(joined, "base ref: verify.test_cmd") {
		t.Fatalf("warnings=%q", res.Warnings)
	}

	stdout, stderr, code := runTBIsolated(t, tbBin, dir, args...)
	if code != 0 || !strings.Contains(stderr, "Warning: base resolves to head") || strings.Contains(stdout, "Warning") {
		t.Fatalf("text mode warnings go to stderr: exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

// pre-rulers JSON keys of a tests-ruler run with build and lint, captured
// from an origin/main build (before verify read the base ref) on the same
// fixture.
var preRulersRunKeys = []string{
	"build_passed", "build_ran", "guard", "head_sha", "hidden_available", "hidden_passed",
	"iterations", "lint_passed", "lint_ran", "verdict", "visible_passed", "workdir",
}

func topLevelKeys(t *testing.T, out string) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// G5: no -c, a committed repo config and no new flags give the same JSON
// keys and verdict as before this slice.
func TestVerifyCLIGoldenCommittedConfigUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "verify:\n  test_cmd: \"true\"\n  build_cmd: \"true\"\n  lint_cmd: \"true\"\n")
	out, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--json")
	if code != 0 {
		t.Fatalf("exit=%d\n%s%s", code, out, stderr)
	}
	if got := topLevelKeys(t, out); !slices.Equal(got, preRulersRunKeys) {
		t.Fatalf("keys=%v\nwant %v", got, preRulersRunKeys)
	}
	if res := decodeResult(t, out); res.Verdict != verify.VerdictPass {
		t.Fatalf("verdict=%s", res.Verdict)
	}
}

// R0 with -c: only the -c file and the defaults, the repository config is
// not read (neither base nor working tree), as before this slice.
func TestVerifyCLIExplicitConfigIgnoresRepo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "verify:\n  test_cmd: \"echo from-base\"\n")
	writeRepoConfig(t, dir, "verify:\n  test_cmd: \"echo from-worktree\"\n")
	cfgFile := filepath.Join(t.TempDir(), "c.yaml")
	writeTestFile(t, cfgFile, "verify:\n  test_cmd: \"echo from-c\"\n")
	out, stderr, code := runTBIsolated(t, tbBin, dir, "-c", cfgFile, "verify", "run", "--json", "--dry-run")
	res := decodeResult(t, out)
	if code != 0 || res.Plan == nil || res.Plan.Commands["test"] != "echo from-c" {
		t.Fatalf("exit=%d %+v\n%s", code, res, stderr)
	}
	if res.ConfigSource != "" || len(res.Warnings) != 0 {
		t.Fatalf("-c must not report repo config: source=%q warnings=%q", res.ConfigSource, res.Warnings)
	}
}

// Review repro: no base file, global test_cmd fails, the working tree sets
// test_cmd to true. The edit must not turn the run into a PASS.
func TestVerifyCLIUncommittedTestCmdIgnored(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "")
	writeRepoConfig(t, dir, "verify:\n  test_cmd: \"true\"\n")
	xdg := t.TempDir()
	if err := os.MkdirAll(filepath.Join(xdg, "toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(xdg, "toolbelt", "config.yaml"), "verify:\n  test_cmd: \"exit 1\"\n")
	out, stderr, code := runTBWithEnv(t, tbBin, dir, []string{"XDG_CONFIG_HOME=" + xdg}, "verify", "run", "--json", "--max-iter", "1")
	res := decodeResult(t, out)
	if code != 1 || (res.Verdict != verify.VerdictFail && res.Verdict != verify.VerdictEscalate) {
		t.Fatalf("want FAIL/ESCALATE: exit=%d %+v\n%s", code, res, stderr)
	}
	if res.ConfigSource != verify.ConfigSourceNone {
		t.Fatalf("config_source=%q want none", res.ConfigSource)
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "ignored .toolbelt/config.yaml keys that differ from the base ref: verify.test_cmd") {
		t.Fatalf("warnings=%q", res.Warnings)
	}
}

// Finding 6: env.pass added in the working tree does not reach the tests;
// the same key committed at base does.
func TestVerifyCLIWorktreeEnvPassIgnored(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	const probe = "TB_R0_SECRET_PROBE"
	testCmd := "verify:\n  test_cmd: 'test -z \"$" + probe + "\"'\n"
	env := func() []string { return []string{"XDG_CONFIG_HOME=" + t.TempDir(), probe + "=leak"} }
	args := []string{"verify", "run", "--json", "--max-iter", "1"}

	dir := rulersRepo(t, testCmd)
	writeRepoConfig(t, dir, testCmd+"env:\n  pass: ["+probe+"]\n")
	out, stderr, code := runTBWithEnv(t, tbBin, dir, env(), args...)
	res := decodeResult(t, out)
	if code != 0 || res.Verdict != verify.VerdictPass {
		t.Fatalf("worktree env.pass must be ignored: exit=%d %+v\n%s", code, res, stderr)
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "base ref: env.pass") {
		t.Fatalf("warnings=%q", res.Warnings)
	}

	dir = rulersRepo(t, testCmd+"env:\n  pass: ["+probe+"]\n")
	out, _, code = runTBWithEnv(t, tbBin, dir, env(), args...)
	if res := decodeResult(t, out); code != 1 || res.Verdict == verify.VerdictPass {
		t.Fatalf("base env.pass must reach the tests: exit=%d %+v", code, res)
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
	if _, stderr, code := runTBIsolated(t, tbBin, dir, append(stackArgs, "--check", "Bad_Name=true")...); code != 2 {
		t.Fatalf("stack --check Bad_Name: exit=%d want 2\n%s", code, stderr)
	}
}

// Overrides of the base config are reported in JSON and as a text-mode
// warning, by tb verify run and by tb stack's verify step; tb stack also
// prints the R0 warnings on stderr.
func TestVerifyCLIReportsOverrides(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "verify:\n  test_cmd: \"false\"\n  typecheck_cmd: \"true\"\n")
	out, _, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--json", "--dry-run", "--test-cmd", "true", "--no-typecheck")
	res := decodeResult(t, out)
	if code != 0 || !slices.Equal(res.Overrides, []string{"no_typecheck", "test_cmd"}) {
		t.Fatalf("exit=%d overrides=%v", code, res.Overrides)
	}
	_, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--dry-run", "--test-cmd", "true", "--no-typecheck")
	if code != 0 || !strings.Contains(stderr, "Warning: overrides applied: no_typecheck, test_cmd") {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}

	writeRepoConfig(t, dir, "verify:\n  test_cmd: \"echo wt\"\n  typecheck_cmd: \"true\"\n")
	stdout, stderr, code := runTBIsolated(t, tbBin, dir, "stack", "verify", "--dry-run", "--test-cmd", "true", "--base", "HEAD~1", "--head", "HEAD")
	if code != 0 {
		t.Fatalf("stack exit=%d\n%s\n%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"Warning: ignored .toolbelt/config.yaml keys that differ from the base ref: verify.test_cmd (verify reads the base ref)",
		"Warning: overrides applied: test_cmd",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stack stderr missing %q:\n%s", want, stderr)
		}
	}
	if !strings.Contains(stdout, `"overrides"`) {
		t.Fatalf("stack json lacks overrides:\n%s", stdout)
	}
	_, stderr, _ = runTBIsolated(t, tbBin, dir, "--json", "stack", "verify", "--dry-run", "--test-cmd", "true", "--base", "HEAD~1", "--head", "HEAD")
	if strings.Contains(stderr, "Warning:") {
		t.Fatalf("--json stack must not print warnings:\n%s", stderr)
	}
}

// A symlinked config at base is not followed: the global config applies.
func TestVerifyCLISymlinkedBaseConfig(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "")
	writeTestFile(t, filepath.Join(dir, "real.yaml"), "verify:\n  test_cmd: \"echo via-link\"\n")
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../real.yaml", filepath.Join(dir, ".toolbelt", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	gitCommitAll(t, dir, "link")
	out, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--json", "--dry-run", "--base", "HEAD", "--head", "HEAD")
	res := decodeResult(t, out)
	if code != 0 || res.ConfigSource != verify.ConfigSourceNone || res.Plan == nil || res.Plan.Commands["test"] == "echo via-link" {
		t.Fatalf("exit=%d %+v\n%s", code, res, stderr)
	}
	if !slices.Contains(res.Warnings, verify.SymlinkConfigWarning) {
		t.Fatalf("warnings=%v", res.Warnings)
	}
}

// coverageMultiJSON is the --ruler tests,coverage payload.
type coverageMultiJSON struct {
	Verdict verify.Verdict `json:"verdict"`
	Rulers  struct {
		Tests    verify.Result         `json:"tests"`
		Coverage verify.CoverageReport `json:"coverage"`
	} `json:"rulers"`
	Overrides []string `json:"overrides"`
}

func runCoverageJSON(t *testing.T, tbBin, dir string, args ...string) (m coverageMultiJSON, code int) {
	t.Helper()
	out, stderr, code := runTBIsolated(t, tbBin, dir, append([]string{"verify", "run", "--json"}, args...)...)
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json: %v\n%s\n%s", err, out, stderr)
	}
	if m.Rulers.Coverage.Coverage == nil {
		t.Fatalf("no coverage block:\n%s", out)
	}
	return m, code
}

// R4 through the CLI: tb verify run and tb stack verify with --ruler
// tests,coverage, the coverage flags and the base-pinned threshold.
func TestVerifyCLICoverageRuler(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := rulersRepo(t, "")
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, ".toolbelt", "thresholds.yaml"), "version: 1\ncoverage:\n  min_percent: 70\n")
	gitCommitAll(t, dir, "thresholds")
	args := []string{"--base", "HEAD", "--head", "HEAD", "--ruler", "tests,coverage", "--test-cmd", "true", "--max-iter", "1",
		"--coverage-cmd", "echo 'TOTAL 75%'", "--coverage-pattern", `TOTAL (\d+)%`}
	checkVerifyCLICoverageFlags(t, tbBin, dir, args)
	checkStackCLICoverage(t, tbBin, dir, args)
	checkCLICoverageLoweredFloor(t, tbBin, dir)
}

func checkVerifyCLICoverageFlags(t *testing.T, tbBin, dir string, args []string) {
	t.Helper()
	m, code := runCoverageJSON(t, tbBin, dir, args...)
	if code != 0 || m.Verdict != verify.VerdictPass || m.Rulers.Coverage.Coverage.Threshold != 70 {
		t.Fatalf("exit=%d %+v", code, m)
	}
	if !slices.Contains(m.Overrides, "coverage_cmd") || !slices.Contains(m.Overrides, "coverage_pattern") {
		t.Fatalf("overrides=%v", m.Overrides)
	}
	m, code = runCoverageJSON(t, tbBin, dir, append(args, "--min-coverage", "60")...)
	if code != 0 || m.Rulers.Coverage.Coverage.ThresholdSource != verify.ThresholdSourceBase {
		t.Fatalf("below base: exit=%d %+v", code, m.Rulers.Coverage)
	}
	m, code = runCoverageJSON(t, tbBin, dir, append(args, "--min-coverage", "80")...)
	if code != 1 || m.Verdict != verify.VerdictFail || m.Rulers.Coverage.Coverage.ThresholdSource != verify.ThresholdSourceFlag {
		t.Fatalf("above base: exit=%d %+v", code, m.Rulers.Coverage)
	}
	if !slices.Contains(m.Overrides, "min_coverage") {
		t.Fatalf("overrides=%v", m.Overrides)
	}
	if _, stderr, code := runTBIsolated(t, tbBin, dir, append([]string{"verify", "run"}, append(args, "--coverage-pattern", "nogroup")...)...); code != 2 {
		t.Fatalf("bad pattern: exit=%d want 2\n%s", code, stderr)
	}
	out, stderr, code := runTBIsolated(t, tbBin, dir, append([]string{"verify", "run", "--json", "--dry-run"}, args...)...)
	if code != 0 || !strings.Contains(out, `"coverage": "echo 'TOTAL 75%'"`) {
		t.Fatalf("dry-run: exit=%d\n%s\n%s", code, out, stderr)
	}
}

func checkStackCLICoverage(t *testing.T, tbBin, dir string, args []string) {
	t.Helper()
	stackArgs := append([]string{"stack", "verify", "--ticket", "rd.md"}, args...)
	out, stderr, code := runTBIsolated(t, tbBin, dir, append(stackArgs, "--min-coverage", "80")...)
	if code != 1 || !strings.Contains(out, "below the threshold 80% (flag)") {
		t.Fatalf("stack: exit=%d\n%s\n%s", code, out, stderr)
	}
	if out, stderr, code = runTBIsolated(t, tbBin, dir, stackArgs...); code != 0 {
		t.Fatalf("stack pass: exit=%d\n%s\n%s", code, out, stderr)
	}
}

// A change that lowers the floor is caught by guard; the base value still
// applies.
func checkCLICoverageLoweredFloor(t *testing.T, tbBin, dir string) {
	t.Helper()
	writeTestFile(t, filepath.Join(dir, ".toolbelt", "thresholds.yaml"), "version: 1\ncoverage:\n  min_percent: 1\n")
	gitCommitAll(t, dir, "lower floor")
	out, stderr, code := runTBIsolated(t, tbBin, dir, "verify", "run", "--json", "--base", "HEAD~1", "--head", "HEAD",
		"--ruler", "coverage", "--coverage-cmd", "echo 'TOTAL 50%'", "--coverage-pattern", `TOTAL (\d+)%`)
	if code != 1 || !strings.Contains(out, "guard aborted verify") || !strings.Contains(out, ".toolbelt/thresholds.yaml") {
		t.Fatalf("lowered floor: exit=%d\n%s\n%s", code, out, stderr)
	}
}
