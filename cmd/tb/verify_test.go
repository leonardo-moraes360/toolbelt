package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

func TestVerifyShouldFail(t *testing.T) {
	if verifyShouldFail(verify.VerdictPass) || verifyShouldFail(verify.VerdictSkipped) {
		t.Fatal("PASS/SKIPPED não devem falhar o exit")
	}
	if !verifyShouldFail(verify.VerdictEscalate) || !verifyShouldFail(verify.VerdictFail) {
		t.Fatal("ESCALATE/FAIL devem falhar o exit (incl. --json)")
	}
}

func TestVerifyJSONEscalateExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := setupVerifyEscalateRepo(t)

	out, err := runVerifyErr(tbBin, dir, "--json", "--max-iter", "1")
	if err == nil {
		t.Fatalf("--json com ESCALATE deveria sair ≠ 0\n%s", out)
	}
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("exit code inesperado: %v\n%s", err, out)
	}
	assertVerifyVerdictJSON(t, out, verify.VerdictEscalate)

	out, err = runVerifyErr(tbBin, dir, "--max-iter", "1")
	if err == nil {
		t.Fatalf("sem --json com ESCALATE também deveria sair ≠ 0\n%s", out)
	}
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("exit code inesperado: %v\n%s", err, out)
	}
}

func setupVerifyEscalateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")

	cfgDir := filepath.Join(dir, ".toolbelt")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "verify:\n  test_cmd: \"true\"\n  build_cmd: \"false\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "head")
	return dir
}

func runVerifyErr(tbBin, dir string, flags ...string) ([]byte, error) {
	args := append([]string{"verify", "run"}, flags...)
	cmd := exec.Command(tbBin, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func assertVerifyVerdictJSON(t *testing.T, out []byte, want verify.Verdict) {
	t.Helper()
	marker := []byte(`"verdict"`)
	idx := bytes.LastIndex(out, marker)
	if idx < 0 {
		t.Fatalf("campo verdict não encontrado:\n%s", out)
	}
	start := idx
	for start > 0 && out[start] != '{' {
		start--
	}
	var res verify.Result
	if err := json.Unmarshal(out[start:], &res); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, out[start:])
	}
	if res.Verdict != want {
		t.Fatalf("verdict=%q want %q\n%s", res.Verdict, want, out)
	}
	if want == verify.VerdictEscalate && res.BuildPassed {
		t.Fatal("ESCALATE por build_cmd: false deveria ter build_passed=false")
	}
	if want == verify.VerdictEscalate && res.Reason == "" {
		t.Fatal("ESCALATE deve incluir reason no JSON")
	}
}

func setupVerifyFlagRepo(t *testing.T) (tbBin, dir, buildLog string) {
	t.Helper()
	tbBin = buildTB(t)
	dir = t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	cfgDir := filepath.Join(dir, ".toolbelt")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildLog = filepath.Join(dir, "cli-build.log")
	cfg := "verify:\n  test_cmd: \"true\"\n  build_cmd: \"echo CFG >> " + buildLog + "\"\n  lint_cmd: \"echo LINT >> " + buildLog + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "head")
	return tbBin, dir, buildLog
}

func TestVerifyCLINoBuildNoLint(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin, dir, buildLog := setupVerifyFlagRepo(t)
	out, err := runVerifyErr(tbBin, dir, "--json", "--max-iter", "1", "--test-cmd", "true", "--no-build", "--no-lint")
	if err != nil {
		t.Fatalf("no-build/no-lint: %v\n%s", err, out)
	}
	var res verify.Result
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if res.BuildRan || res.LintRan {
		t.Fatalf("flags devem pular build/lint: %+v", res)
	}
	if _, statErr := os.Stat(buildLog); !os.IsNotExist(statErr) {
		t.Fatal("config build/lint não devem executar com --no-build/--no-lint")
	}
}

func TestVerifyCLIBuildCmdOverride(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin, dir, _ := setupVerifyFlagRepo(t)
	marker := filepath.Join(dir, "custom.flag")
	out, err := runVerifyErr(tbBin, dir, "--json", "--max-iter", "1", "--build-cmd", "touch "+marker, "--lint-cmd", "true", "--test-cmd", "test -f "+marker)
	if err != nil {
		t.Fatalf("build-cmd override: %v\n%s", err, out)
	}
	var res verify.Result
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if !res.BuildRan || res.Verdict != verify.VerdictPass {
		t.Fatalf("override falhou: %+v", res)
	}
}

func TestVerifyCLIDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin, dir, _ := setupVerifyFlagRepo(t)
	dryFlag := filepath.Join(dir, "dry.flag")
	out, err := runVerifyErr(tbBin, dir, "--json", "--dry-run", "--build-cmd", "touch "+dryFlag, "--test-cmd", "false")
	if err != nil {
		t.Fatalf("dry-run: %v\n%s", err, out)
	}
	var res verify.Result
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("dry json: %v\n%s", err, out)
	}
	if !res.DryRun || res.Plan == nil || res.Verdict != verify.VerdictSkipped {
		t.Fatalf("dry-run payload: %+v", res)
	}
	if _, statErr := os.Stat(dryFlag); !os.IsNotExist(statErr) {
		t.Fatal("dry-run não deve executar")
	}
}
