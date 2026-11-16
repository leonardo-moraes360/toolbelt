package verify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if res.HeadSHA == "" {
		t.Fatal("expected head_sha in result")
	}
	if res.WorkDir != dir {
		t.Fatalf("workdir=%q want repo %q when head == HEAD", res.WorkDir, dir)
	}
}

func TestRunOnHeadWorktreeWhenHeadDiffers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module demo\ngo 1.22\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "demo.go"), []byte("package demo\nfunc Add(a, b int) int { return a + b }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "demo_test.go"), []byte("package demo\nimport \"testing\"\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 { t.Fatal() }\n}\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	base := gitRev(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "marker_test.go"), []byte("package demo\nimport \"testing\"\nfunc TestMarkerOnlyOnHead(t *testing.T) {\n\tt.Fatal(\"marker: only on head\")\n}\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "head-with-failing-marker")
	head := gitRev(t, dir)

	runGit(t, dir, "checkout", "-q", base)

	cfg := DefaultConfig(dir)
	cfg.BaseRef = base
	cfg.HeadRef = head
	cfg.Timeout = time.Minute
	cfg.MaxIterations = 1
	cfg.VisibleTest = "go test ./..."

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict == VerdictPass {
		t.Fatalf("verify must fail when head has a failing test absent from the current worktree; got PASS (workdir=%q head_sha=%q)", res.WorkDir, res.HeadSHA)
	}
	if res.HeadSHA != head {
		t.Fatalf("head_sha=%q want %q", res.HeadSHA, head)
	}
	if res.WorkDir == "" || res.WorkDir == dir {
		t.Fatalf("expected detached head workdir different from repo %q, got %q", dir, res.WorkDir)
	}
	if _, err := os.Stat(res.WorkDir); !os.IsNotExist(err) {
		t.Fatalf("detached worktree should be removed after verify, path=%q err=%v", res.WorkDir, err)
	}
}

func TestRunDirtyHeadWarnsButUsesRepo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")
	_ = os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("uncommitted\n"), 0o644)

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
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
	if res.WorkDir != dir {
		t.Fatalf("workdir=%q want %q", res.WorkDir, dir)
	}
	head := gitRev(t, dir)
	if res.HeadSHA != head {
		t.Fatalf("head_sha=%q want %q", res.HeadSHA, head)
	}
}

func TestDefaultHiddenTestCmd(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	if cfg.HiddenTest != "" {
		t.Fatalf("HiddenTest default = %q, want empty (opt-in / dir-gated)", cfg.HiddenTest)
	}
	if DefaultHiddenTestCmd != "go test ./tests/hidden/..." {
		t.Fatalf("DefaultHiddenTestCmd = %q", DefaultHiddenTestCmd)
	}
	if cfg.VisibleTest != "go test ./..." {
		t.Fatalf("VisibleTest default = %q", cfg.VisibleTest)
	}
}

func TestHiddenTestCmdRunsWithoutHiddenDir(t *testing.T) {
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
	cfg.HiddenTest = "false"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HiddenAvailable {
		t.Fatal("expected hidden suite available when hidden_test_cmd is set without tests/hidden/")
	}
	if res.HiddenPassed {
		t.Fatal("expected hidden cmd false to fail")
	}
	if res.Verdict == VerdictPass {
		t.Fatal("expected non-pass when configured hidden_test_cmd fails without tests/hidden/")
	}
}

func TestHiddenTestCmdPassWithoutHiddenDir(t *testing.T) {
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
	cfg.HiddenTest = "true"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HiddenAvailable {
		t.Fatal("expected hidden suite available when hidden_test_cmd is set without tests/hidden/")
	}
	if !res.HiddenPassed {
		t.Fatal("expected hidden cmd true to pass")
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func TestHiddenSkippedWithoutDirOrCmd(t *testing.T) {
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
	cfg.HiddenTest = ""

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.HiddenAvailable {
		t.Fatal("expected hidden suite unavailable without tests/hidden/ and without hidden_test_cmd")
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func TestHiddenDefaultGoCmdWhenDirExists(t *testing.T) {
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
	cfg.HiddenTest = ""

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.HiddenAvailable {
		t.Fatal("non-Go tests/hidden/ without applicable toolchain must be N/A")
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s want PASS when hidden suite is N/A", res.Verdict)
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
	if res.HiddenAvailable {
		t.Fatal("non-Go hidden dir must be marked not applicable")
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s want PASS", res.Verdict)
	}
}

func TestCustomProtectedPathDoesNotAbortVerify(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "format"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "format", "format.go"), []byte("package format\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	base := gitRev(t, dir)

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - format/format.go\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "format", "format.go"), []byte("package format\nfunc X(){}\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "edit mandate path")
	head := gitRev(t, dir)

	cfg := DefaultConfig(dir)
	cfg.BaseRef = base
	cfg.HeadRef = head
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.BuildCmd = "true"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations == 0 {
		t.Fatal("verify must not abort with 0 iterations on custom protected_paths")
	}
	if !res.BuildRan || !res.VisiblePassed {
		t.Fatalf("build/test must run: build_ran=%v visible=%v", res.BuildRan, res.VisiblePassed)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s want PASS", res.Verdict)
	}
	if res.Guard == nil || len(res.Guard.Violations) == 0 {
		t.Fatal("guard finding must still be reported")
	}
	found := false
	for _, v := range res.Guard.Violations {
		if v.Type == "protected_path" && v.File == "format/format.go" {
			found = true
			if v.Severity != "warning" {
				t.Fatalf("custom mandate path should be warning in verify, got %s", v.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected protected_path finding: %#v", res.Guard.Violations)
	}
}

func TestDefaultProtectedPathStillAbortsVerify(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "base")
	base := gitRev(t, dir)

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "thresholds.yaml"), []byte("min: 1\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "touch thresholds")
	head := gitRev(t, dir)

	cfg := DefaultConfig(dir)
	cfg.BaseRef = base
	cfg.HeadRef = head
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.BuildCmd = "true"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations != 0 {
		t.Fatalf("anti-cheat default must abort before iterations, got %d", res.Iterations)
	}
	if res.BuildRan {
		t.Fatal("build must not run when default protected_path aborts")
	}
	if res.Verdict != VerdictFail {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func TestScopedCustomProtectedPathContinuesVerify(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	base := gitRev(t, dir)

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - src/main.go\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "rd.md"), []byte("## Scope\n\n- src/main.go\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main\nfunc main(){}\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "edit in scope")
	head := gitRev(t, dir)

	cfg := DefaultConfig(dir)
	cfg.BaseRef = base
	cfg.HeadRef = head
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.TicketPath = filepath.Join(dir, "rd.md")

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations == 0 || res.Verdict != VerdictPass {
		t.Fatalf("in-scope custom must continue: iter=%d verdict=%s", res.Iterations, res.Verdict)
	}
}

func TestOutOfScopeCustomProtectedPathAbortsVerify(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "secrets.txt"), []byte("a\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	base := gitRev(t, dir)

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - secrets.txt\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "rd.md"), []byte("## Scope\n\n- src/main.go\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "secrets.txt"), []byte("b\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "touch secret")
	head := gitRev(t, dir)

	cfg := DefaultConfig(dir)
	cfg.BaseRef = base
	cfg.HeadRef = head
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.TicketPath = filepath.Join(dir, "rd.md")

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations != 0 || res.Verdict != VerdictFail {
		t.Fatalf("out-of-scope custom must abort: iter=%d verdict=%s", res.Iterations, res.Verdict)
	}
	if res.BuildRan {
		t.Fatal("build must not run on out-of-scope custom abort")
	}
}

func TestRunFailFastMissingCommand(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 3
	cfg.VisibleTest = "tb-verify-missing-bin-xyz-9f3a"

	res, err := Run(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error when test_cmd binary is missing")
	}
	if !strings.Contains(err.Error(), "não encontrado") {
		t.Fatalf("error should mention missing command, got %v", err)
	}
	if !strings.Contains(err.Error(), "verify.test_cmd") {
		t.Fatalf("error should mention verify.test_cmd, got %v", err)
	}
	if !strings.Contains(err.Error(), cfg.VisibleTest) {
		t.Fatalf("error should include command %q, got %v", cfg.VisibleTest, err)
	}
	if want := fmt.Sprintf(errMissingCmdFmt, cfg.VisibleTest, "verify.test_cmd"); err.Error() != want {
		t.Fatalf("error=%q want %q", err.Error(), want)
	}
	if res != nil && res.Iterations > 1 {
		t.Fatalf("missing command must fail-fast without retries, iterations=%d", res.Iterations)
	}
}

func TestRunFailFastNotExecutable(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	script := filepath.Join(dir, "not-exec.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 3
	cfg.VisibleTest = script

	_, err := Run(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error when test_cmd is not executable")
	}
	if !strings.Contains(err.Error(), "não encontrado") || !strings.Contains(err.Error(), "verify.test_cmd") {
		t.Fatalf("error should fail-fast for exit 126, got %v", err)
	}
}

func TestRunRetriesGenuineFailure(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 3
	cfg.VisibleTest = "false"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations != 3 {
		t.Fatalf("genuine failures must retry, iterations=%d want 3", res.Iterations)
	}
	if res.Verdict != VerdictEscalate {
		t.Fatalf("verdict=%s want ESCALATE", res.Verdict)
	}
	if res.Reason == "" || res.Error == "" {
		t.Fatalf("ESCALATE must expose reason/error in JSON, got reason=%q error=%q", res.Reason, res.Error)
	}
}

func TestHiddenSuiteNotApplicableWithoutToolchain(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests", "hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "tests", "hidden", "readme.md"), []byte("# no suite\n"), 0o644)
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
	if res.HiddenAvailable {
		t.Fatal("tests/hidden/ without applicable toolchain must be N/A")
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("verdict=%s want PASS", res.Verdict)
	}
}

func TestHiddenSuiteDetectsGoToolchain(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\ngo 1.22\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n"), 0o644)
	if err := os.MkdirAll(filepath.Join(dir, "tests", "hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "tests", "hidden", "hidden_test.go"), []byte("package hidden\nimport \"testing\"\nfunc TestHidden(t *testing.T) {}\n"), 0o644)
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

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HiddenAvailable {
		t.Fatal("Go hidden suite with go.mod must be available")
	}
	if !res.HiddenPassed || res.Verdict != VerdictPass {
		t.Fatalf("hidden_passed=%v verdict=%s", res.HiddenPassed, res.Verdict)
	}
}

func TestEscalateReasonInJSON(t *testing.T) {
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
	cfg.VisibleTest = "echo boom-visible >&2; false"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictEscalate {
		t.Fatalf("verdict=%s", res.Verdict)
	}
	raw, err := res.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"reason"`) {
		t.Fatalf("JSON missing reason: %s", raw)
	}
	if !strings.Contains(res.Reason, "verify.test_cmd") {
		t.Fatalf("reason=%q", res.Reason)
	}
	if res.Error == "" {
		t.Fatal("error field must be set")
	}
}

func TestBuildLintCachedAcrossIterations(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	buildLog := filepath.Join(dir, "build.count")
	lintLog := filepath.Join(dir, "lint.count")
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 3
	cfg.BuildCmd = "echo x >> " + buildLog
	cfg.LintCmd = "echo y >> " + lintLog
	cfg.VisibleTest = "false"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictEscalate || res.Iterations != 3 {
		t.Fatalf("verdict=%s iterations=%d", res.Verdict, res.Iterations)
	}
	buildN := countLines(t, buildLog)
	lintN := countLines(t, lintLog)
	if buildN != 1 {
		t.Fatalf("build ran %d times, want 1 (cache)", buildN)
	}
	if lintN != 1 {
		t.Fatalf("lint ran %d times, want 1 (cache)", lintN)
	}
}

func TestDeterministicBuildFailureNoRetry(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	buildLog := filepath.Join(dir, "buildfail.count")
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 3
	cfg.BuildCmd = "echo fail >> " + buildLog + "; false"
	cfg.VisibleTest = "true"

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations != 1 {
		t.Fatalf("deterministic build failure must not retry, iterations=%d", res.Iterations)
	}
	if res.Verdict != VerdictEscalate {
		t.Fatalf("verdict=%s", res.Verdict)
	}
	if countLines(t, buildLog) != 1 {
		t.Fatalf("build should run once, got %d", countLines(t, buildLog))
	}
	if !strings.Contains(res.Reason, "verify.build_cmd") {
		t.Fatalf("reason=%q", res.Reason)
	}
}

func TestDryRunPlanNoExecute(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	marker := filepath.Join(dir, "executed.flag")
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.BuildCmd = "touch " + marker
	cfg.LintCmd = "touch " + marker
	cfg.VisibleTest = "touch " + marker
	cfg.DryRun = true

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.Plan == nil {
		t.Fatalf("dry_run=%v plan=%v", res.DryRun, res.Plan)
	}
	if res.Plan.Dir != dir {
		t.Fatalf("plan.dir=%q want %q", res.Plan.Dir, dir)
	}
	if res.Plan.Commands["build"] != cfg.BuildCmd || res.Plan.Commands["test"] != cfg.VisibleTest {
		t.Fatalf("plan commands=%v", res.Plan.Commands)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("dry-run must not execute commands")
	}
	if res.Verdict != VerdictSkipped {
		t.Fatalf("verdict=%s want SKIPPED", res.Verdict)
	}
}

func TestApplyCmdOverridesNoBuildNoLint(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	cfg.BuildCmd = "go build ./..."
	cfg.LintCmd = "golangci-lint run"
	ApplyCmdOverrides(&cfg, "", false, true, "", false, true)
	if cfg.BuildCmd != "" || cfg.LintCmd != "" {
		t.Fatalf("no-build/no-lint must clear cmds, got build=%q lint=%q", cfg.BuildCmd, cfg.LintCmd)
	}
	cfg.BuildCmd = "keep"
	cfg.LintCmd = "keep"
	ApplyCmdOverrides(&cfg, "make build", true, false, "", true, false)
	if cfg.BuildCmd != "make build" || cfg.LintCmd != "" {
		t.Fatalf("explicit overrides failed: build=%q lint=%q", cfg.BuildCmd, cfg.LintCmd)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

func gitRev(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out[:len(out)-1])
}
