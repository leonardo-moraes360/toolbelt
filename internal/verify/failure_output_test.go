package verify

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func newCommittedRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")
	return dir
}

func TestRunSingleCommitDefaultBase(t *testing.T) {
	dir := newCommittedRepo(t)
	cfg := DefaultConfig(dir) // BaseRef = HEAD~1, missing in a one-commit repo
	cfg.Timeout = 30 * time.Second
	cfg.VisibleTest = "true"
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("first verify in a new repo must not fail on HEAD~1: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("verdict=%s reason=%s", res.Verdict, res.Reason)
	}
}

func TestFailReasonStripsANSIAndKeepsFailLines(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("\x1b[32m✓\x1b[0m test ok number padding padding padding\n")
	}
	b.WriteString("\x1b[31m FAIL \x1b[0m src/canvas.test.ts > selects new node\n")
	for i := 0; i < 200; i++ {
		b.WriteString("noise line after failure padding padding padding\n")
	}
	b.WriteString("Test Files  1 failed | 4 passed\n")
	got := failReason("verify.test_cmd", b.String())
	if strings.Contains(got, "\x1b") {
		t.Fatalf("ANSI left in reason: %q", got)
	}
	if !strings.Contains(got, "FAIL  src/canvas.test.ts > selects new node") {
		t.Fatalf("FAIL line missing from reason:\n%s", got)
	}
	if !strings.Contains(got, "Test Files  1 failed") {
		t.Fatalf("summary tail missing from reason:\n%s", got)
	}
	if len(got) > failReasonMax+64 {
		t.Fatalf("reason too long: %d", len(got))
	}
}

func TestDeterministicTestFailureStopsEarly(t *testing.T) {
	dir := newCommittedRepo(t)
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 3
	// Same named failure every run; the duration differs and must be ignored.
	cfg.VisibleTest = `echo "--- FAIL: TestX ($(date +%N)s)"; echo "Error: browserType.launch: Executable doesn't exist"; exit 1`
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictEscalate || res.Iterations != 2 {
		t.Fatalf("verdict=%s iterations=%d want ESCALATE after 2", res.Verdict, res.Iterations)
	}
	if !strings.Contains(res.Reason, "deterministic failure") {
		t.Fatalf("reason should say deterministic: %s", res.Reason)
	}
}

func TestChangingFailureKeepsRetrying(t *testing.T) {
	dir := newCommittedRepo(t)
	counter := filepath.Join(t.TempDir(), "n")
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 3
	cfg.VisibleTest = `echo x >> ` + counter + `; echo "FAIL case-$(wc -l < ` + counter + `)"; exit 1`
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations != 3 {
		t.Fatalf("different failures must keep retrying, iterations=%d", res.Iterations)
	}
}

func TestFilteredEnvNamesOnFail(t *testing.T) {
	t.Setenv("PAPRIKA_CHROMIUM", "/opt/chromium")
	dir := newCommittedRepo(t)
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "false"
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.FilteredEnv, "PAPRIKA_CHROMIUM") {
		t.Fatalf("filtered_env should name PAPRIKA_CHROMIUM: %v", res.FilteredEnv)
	}
	for _, n := range res.FilteredEnv {
		if strings.Contains(n, "/opt/chromium") {
			t.Fatal("value leaked into filtered_env")
		}
	}

	cfg = DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.VisibleTest = "true"
	res, err = Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilteredEnv != nil {
		t.Fatalf("PASS should not carry filtered_env: %v", res.FilteredEnv)
	}
}

func TestChildGetsNoColor(t *testing.T) {
	env := withNoColor([]string{"PATH=/bin"})
	if !slices.Contains(env, "NO_COLOR=1") || !slices.Contains(env, "FORCE_COLOR=0") {
		t.Fatalf("env=%v", env)
	}
	env = withNoColor([]string{"FORCE_COLOR=1"})
	if slices.Contains(env, "NO_COLOR=1") {
		t.Fatalf("operator choice must win: %v", env)
	}
}

// dogfood #10: with many packages the tail of `go test ./...` is all `ok` and
// `[no test files]`; the failing package printed early must still reach the
// reason.
func TestExcerptKeepsFailLinesBeyondTheTail(t *testing.T) {
	var b strings.Builder
	b.WriteString("--- FAIL: TestEarly (0.00s)\n    early_test.go:12: boom\nFAIL\tmod/early\t0.01s\n")
	for i := 0; i < 400; i++ {
		b.WriteString("ok  \tmod/pkg" + strings.Repeat("x", 20) + "\t0.01s\n?   \tmod/none\t[no test files]\n")
	}
	got := excerpt(b.String(), 600)
	if len(got) > 700 {
		t.Fatalf("excerpt too long: %d", len(got))
	}
	for _, want := range []string{"--- FAIL: TestEarly", "FAIL\tmod/early"} {
		if !strings.Contains(got, want) {
			t.Errorf("excerpt lost %q:\n%s", want, got)
		}
	}
}
