package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

func TestCLIGitMissingRefShowsStderrAndFetchHint(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := setupTwoCommitRepo(t)
	missing := "origin/no-such-ref-xyz"

	cases := []struct {
		name string
		args []string
	}{
		{name: "guard", args: []string{"guard", "check", "--base", missing, "--head", "HEAD"}},
		{name: "review", args: []string{"review", "--base", missing, "--head", "HEAD", "--ticket", "rd.md"}},
		{name: "drift", args: []string{"drift", "--base", missing, "--head", "HEAD", "--ticket", "rd.md"}},
		{name: "verify", args: []string{"verify", "run", "--base", missing, "--head", "HEAD", "--test-cmd", "true", "--max-iter", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runTBErr(tbBin, dir, tc.args...)
			if err == nil {
				t.Fatalf("esperava erro\n%s", out)
			}
			msg := string(out)
			if strings.TrimSpace(msg) == "Error: exit status 128" || msg == "exit status 128" {
				t.Fatalf("erro opaco exit status 128:\n%s", msg)
			}
			if !strings.Contains(msg, "stderr:") && !strings.Contains(msg, "unknown revision") && !strings.Contains(msg, "Needed a single revision") {
				t.Fatalf("deve incluir stderr do git:\n%s", msg)
			}
			if !strings.Contains(msg, "ref ausente:") {
				t.Fatalf("deve citar ref ausente:\n%s", msg)
			}
			if !strings.Contains(msg, "dica: git fetch origin") {
				t.Fatalf("deve sugerir fetch:\n%s", msg)
			}
		})
	}
}

func TestCLIApprovedSHAEqualsBaseClearMessage(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "base")
	base := gitRev(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "head")
	if err := os.WriteFile(filepath.Join(dir, "rd.md"), []byte("## Scope\n- f.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runTBErr(tbBin, dir, "review", "--base", base, "--head", "HEAD", "--approved-sha", base, "--ticket", "rd.md")
	if err == nil {
		t.Fatalf("esperava erro\n%s", out)
	}
	msg := string(out)
	if strings.Contains(strings.ToLower(msg), "usage: git range-diff") {
		t.Fatalf("não deve despejar usage:\n%s", msg)
	}
	if !strings.Contains(msg, "coincide com a base") && !strings.Contains(msg, "vazio") {
		t.Fatalf("mensagem clara esperada:\n%s", msg)
	}
}

func TestVerifyMissingCommandExit127JSONEnvelope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := setupTwoCommitRepo(t)

	out, err := runTBErr(tbBin, dir, "verify", "run", "--json", "--base", "HEAD~1", "--head", "HEAD",
		"--test-cmd", "tb-verify-missing-bin-xyz-9f3a", "--max-iter", "1")
	if err == nil {
		t.Fatalf("esperava exit ≠ 0\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("erro inesperado: %v\n%s", err, out)
	}
	if exitErr.ExitCode() != 127 {
		t.Fatalf("exit=%d want 127\n%s", exitErr.ExitCode(), out)
	}
	res := decodeVerifyResultJSON(t, out)
	if res.Error == "" {
		t.Fatalf("JSON deve incluir error:\n%s", out)
	}
	if !strings.Contains(res.Error, "não encontrado") {
		t.Fatalf("error=%q", res.Error)
	}
	if res.Verdict != verify.VerdictFail {
		t.Fatalf("verdict=%s want FAIL", res.Verdict)
	}
}

func TestVerifyMissingBuildCommandExit127JSON(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := setupTwoCommitRepo(t)
	cfgDir := filepath.Join(dir, ".toolbelt")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "verify:\n  test_cmd: \"true\"\n  build_cmd: \"tb-build-missing-xyz\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runTBErr(tbBin, dir, "verify", "run", "--json", "--base", "HEAD~1", "--head", "HEAD", "--max-iter", "1")
	if err == nil {
		t.Fatalf("esperava exit ≠ 0\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("erro inesperado: %v\n%s", err, out)
	}
	if exitErr.ExitCode() != 127 {
		t.Fatalf("exit=%d want 127\n%s", exitErr.ExitCode(), out)
	}
	res := decodeVerifyResultJSON(t, out)
	if !strings.Contains(res.Error, "verify.build_cmd") {
		t.Fatalf("error deve citar build_cmd: %q", res.Error)
	}
}

func setupTwoCommitRepo(t *testing.T) string {
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
	if err := os.WriteFile(filepath.Join(dir, "rd.md"), []byte("## Scope\n- f.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runTBErr(tbBin, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(tbBin, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func decodeVerifyResultJSON(t *testing.T, out []byte) verify.Result {
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
		t.Fatalf("deve emitir envelope JSON: %v\n%s", err, out[start:])
	}
	return res
}
