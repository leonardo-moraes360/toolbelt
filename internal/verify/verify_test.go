package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRunPass(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\ngo 1.22\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nfunc Add(a,b int) int { return a+b }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\nimport \"testing\"\nfunc TestAdd(t *testing.T){ if Add(1,2)!=3 { t.Fatal() } }\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = time.Minute
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func TestDefaultHiddenTestCmd(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	if cfg.HiddenTest != "go test ./tests/hidden/..." {
		t.Fatalf("HiddenTest default = %q", cfg.HiddenTest)
	}
	if cfg.VisibleTest != "go test ./..." {
		t.Fatalf("VisibleTest default = %q", cfg.VisibleTest)
	}
}

func TestHiddenTestCmdConfigurableNonGo(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests", "hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "tests", "hidden", "check.mjs"), []byte("process.exit(0)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"demo"}`+"\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = time.Minute
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.HiddenTest = "node tests/hidden/check.mjs"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HiddenAvailable {
		t.Fatal("expected hidden suite available")
	}
	if !res.HiddenPassed {
		t.Fatal("expected hidden suite to pass with custom cmd")
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func TestAbsentBuildLintCmdKeepsCurrentBehavior(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.BuildRan || res.LintRan {
		t.Fatalf("expected no build/lint without cmds, got build_ran=%v lint_ran=%v", res.BuildRan, res.LintRan)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func TestBuildCmdFailureFailsVerify(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.BuildCmd = "false"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.BuildRan {
		t.Fatal("expected build_cmd to run")
	}
	if res.BuildPassed {
		t.Fatal("expected build_cmd failure")
	}
	if res.VisiblePassed {
		t.Fatal("tests should not run after build failure")
	}
	if res.Verdict == VerdictPass {
		t.Fatal("verify must not PASS when build_cmd fails")
	}
}

func TestLintCmdFailureFailsVerify(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.LintCmd = "false"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.LintRan {
		t.Fatal("expected lint_cmd to run")
	}
	if res.LintPassed {
		t.Fatal("expected lint_cmd failure")
	}
	if res.VisiblePassed {
		t.Fatal("tests should not run after lint failure")
	}
	if res.Verdict == VerdictPass {
		t.Fatal("verify must not PASS when lint_cmd fails")
	}
}

func TestBuildCmdRunsBeforeTests(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.BuildCmd = "touch built.flag"
	cfg.VisibleTest = "test -f built.flag"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.BuildRan || !res.BuildPassed {
		t.Fatalf("build_ran=%v build_passed=%v", res.BuildRan, res.BuildPassed)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s (build must run before tests)", res.Verdict)
	}
}

func TestBuildAndLintPassWithTests(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.BuildCmd = "true"
	cfg.LintCmd = "true"
	cfg.VisibleTest = "true"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.BuildRan || !res.BuildPassed || !res.LintRan || !res.LintPassed {
		t.Fatalf("build/lint flags: ran=%v/%v passed=%v/%v", res.BuildRan, res.LintRan, res.BuildPassed, res.LintPassed)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func TestHiddenTestCmdDefaultFailsNonGoSuite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests", "hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "tests", "hidden", "noop.txt"), []byte("not go\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HiddenAvailable {
		t.Fatal("expected hidden suite available")
	}
	if res.HiddenPassed {
		t.Fatal("default go hidden cmd should fail on non-Go suite")
	}
	if res.Verdict == VerdictPass {
		t.Fatal("expected non-pass without custom hidden_test_cmd")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}
