package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestGuardDefaultExitOnError(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir, base, head := setupProtectedDiff(t)
	rangeArg := base + ".." + head

	out, err := runGuardErr(tbBin, dir, rangeArg)
	if err == nil {
		t.Fatalf("default com SeverityError deveria sair ≠ 0\n%s", out)
	}
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("exit code inesperado: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "protected_path") {
		t.Fatalf("esperava violação no stdout:\n%s", out)
	}

	out, err = runGuardErr(tbBin, dir, rangeArg, "--json")
	if err == nil {
		t.Fatalf("default --json com SeverityError deveria sair ≠ 0\n%s", out)
	}
	assertGuardJSON(t, out, true)

	out, err = runGuardErr(tbBin, dir, rangeArg, "--warn-only")
	if err != nil {
		t.Fatalf("--warn-only deveria sair 0: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "protected_path") {
		t.Fatalf("--warn-only deveria ainda listar violações:\n%s", out)
	}

	out, err = runGuardErr(tbBin, dir, rangeArg, "--warn-only", "--json")
	if err != nil {
		t.Fatalf("--warn-only --json deveria sair 0: %v\n%s", err, out)
	}
	assertGuardJSON(t, out, true)

	out, err = runGuardErr(tbBin, dir, rangeArg, "--strict", "--json")
	if err == nil {
		t.Fatalf("--strict --json com SeverityError deveria sair ≠ 0\n%s", out)
	}
	assertGuardJSON(t, out, true)
}

func TestGuardTextOKLine(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
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
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "ok.go")
	git("commit", "-m", "base")
	base := gitRev(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package ok\nfunc X() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "ok.go")
	git("commit", "-m", "change")
	head := gitRev(t, dir)

	out := runGuard(t, tbBin, dir, base+".."+head)
	got := strings.TrimSpace(string(out))
	if !strings.HasPrefix(got, "guard: ok (") || !strings.Contains(got, "0 violações") {
		t.Fatalf("esperava linha OK no modo texto, got: %q", got)
	}
	if strings.Contains(got, "error ") || strings.Contains(got, "warning ") {
		t.Fatalf("não esperava violações: %q", got)
	}
}

func TestGuardCheckBaseHeadFlags(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir, base, head := setupProtectedDiff(t)

	out, err := runGuardFlags(tbBin, dir, "--base", base, "--head", head, "--json")
	if err == nil {
		t.Fatalf("--base/--head com SeverityError deveria sair ≠ 0\n%s", out)
	}
	assertGuardJSON(t, out, true)

	out, err = runGuardFlags(tbBin, dir, "--base", base, "--head", head, "--warn-only", "--json")
	if err != nil {
		t.Fatalf("--base/--head --warn-only --json deveria sair 0: %v\n%s", err, out)
	}
	assertGuardJSON(t, out, true)

	out, err = runGuardFlags(tbBin, dir, "--base", base, "--head", head, "--strict", "--json")
	if err == nil {
		t.Fatalf("--base/--head --strict --json deveria sair ≠ 0\n%s", out)
	}
	assertGuardJSON(t, out, true)
}

func TestResolveGuardRange(t *testing.T) {
	cmd := guardCheckCmd
	_ = cmd.Flags().Set("base", "HEAD~1")
	_ = cmd.Flags().Set("head", "HEAD")

	base, head, err := resolveGuardRange(cmd, []string{"abc..def"})
	if err != nil || base != "abc" || head != "def" {
		t.Fatalf("range A..B: got %q %q err=%v", base, head, err)
	}

	_ = cmd.Flags().Set("base", "b1")
	_ = cmd.Flags().Set("head", "h1")
	base, head, err = resolveGuardRange(cmd, nil)
	if err != nil || base != "b1" || head != "h1" {
		t.Fatalf("flags: got %q %q err=%v", base, head, err)
	}

	if _, _, err := resolveGuardRange(cmd, []string{"sem-range"}); err == nil {
		t.Fatal("range inválido deveria falhar")
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

func runGuardFlags(tbBin, dir string, flags ...string) ([]byte, error) {
	args := append([]string{"guard", "check"}, flags...)
	cmd := exec.Command(tbBin, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func assertGuardJSON(t *testing.T, out []byte, wantViolations bool) {
	t.Helper()
	raw := extractJSONObject(out)
	var res guard.CheckResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, out)
	}
	if wantViolations && len(res.Violations) == 0 {
		t.Fatal("esperava violações no JSON")
	}
	if res.RulesChecked <= 0 {
		t.Fatal("esperava rules_checked > 0")
	}
}

func extractJSONObject(out []byte) []byte {
	s := string(out)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return out
	}
	return []byte(s[start : end+1])
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
