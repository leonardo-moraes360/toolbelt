package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/guard"
)

func TestHasStrictError(t *testing.T) {
	if hasStrictError(nil) {
		t.Fatal("nil não deve falhar")
	}
	if hasStrictError([]guard.Violation{{Severity: guard.SeverityWarning}}) {
		t.Fatal("warning não deve falhar")
	}
	if !hasStrictError([]guard.Violation{{Severity: guard.SeverityError}}) {
		t.Fatal("error deve falhar")
	}
}

func TestGuardStrictJSONExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir, base, head := setupProtectedDiff(t)
	rangeArg := base + ".." + head

	out := runGuard(t, tbBin, dir, rangeArg, "--json")
	assertGuardJSON(t, out, true)

	out, err := runGuardErr(tbBin, dir, rangeArg, "--strict", "--json")
	if err == nil {
		t.Fatalf("--strict --json com SeverityError deveria sair ≠ 0\n%s", out)
	}
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("exit code inesperado: %v\n%s", err, out)
	}
	assertGuardJSON(t, out, true)

	if _, err := runGuardErr(tbBin, dir, rangeArg, "--strict"); err == nil {
		t.Fatal("--strict sem --json também deveria sair ≠ 0")
	}
}

func buildTB(t *testing.T) string {
	t.Helper()
	root := findModuleRoot(t)
	tbBin := filepath.Join(t.TempDir(), "tb")
	build := exec.Command("go", "build", "-o", tbBin, "./cmd/tb")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build tb: %v\n%s", err, out)
	}
	return tbBin
}

func setupProtectedDiff(t *testing.T) (dir, base, head string) {
	t.Helper()
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

	hidden := filepath.Join(dir, "tests", "hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "sentinel.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "tests/hidden/sentinel.txt")
	git("commit", "-m", "base")
	base = gitRev(t, dir)

	if err := os.WriteFile(filepath.Join(hidden, "sentinel.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "tests/hidden/sentinel.txt")
	git("commit", "-m", "touch protected")
	head = gitRev(t, dir)
	return dir, base, head
}

func gitRev(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out[:len(out)-1])
}

func runGuard(t *testing.T, tbBin, dir, rangeArg string, flags ...string) []byte {
	t.Helper()
	out, err := runGuardErr(tbBin, dir, rangeArg, flags...)
	if err != nil {
		t.Fatalf("guard %v: %v\n%s", flags, err, out)
	}
	return out
}

func runGuardErr(tbBin, dir, rangeArg string, flags ...string) ([]byte, error) {
	args := append([]string{"guard", "check", rangeArg}, flags...)
	cmd := exec.Command(tbBin, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func assertGuardJSON(t *testing.T, out []byte, wantViolations bool) {
	t.Helper()
	var res guard.CheckResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, out)
	}
	if wantViolations && len(res.Violations) == 0 {
		t.Fatal("esperava violações no JSON")
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod não encontrado")
		}
		dir = parent
	}
}
