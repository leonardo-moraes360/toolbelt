package stack

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

func TestParseOpUnknown(t *testing.T) {
	_, err := ParseOp("nope")
	if err == nil {
		t.Fatal("esperava erro para op desconhecida")
	}
	if !strings.Contains(err.Error(), "operação desconhecida") {
		t.Fatalf("mensagem: %v", err)
	}
	if strings.Contains(err.Error(), "Usage") {
		t.Fatalf("não deve citar Usage: %v", err)
	}
}

func TestParseOpsOrder(t *testing.T) {
	ops, err := ParseOps([]string{"guard", "verify", "drift", "review"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Op{OpGuard, OpVerify, OpDrift, OpReview}
	if len(ops) != len(want) {
		t.Fatalf("len=%d", len(ops))
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("ops[%d]=%s want %s", i, ops[i], want[i])
		}
	}
}

func TestParseOpsEmptyListsValidOps(t *testing.T) {
	_, err := ParseOps(nil)
	if err == nil {
		t.Fatal("esperava erro sem operações")
	}
	msg := err.Error()
	for _, op := range []string{"verify", "review", "drift", "guard"} {
		if !strings.Contains(msg, op) {
			t.Fatalf("mensagem deveria listar %q: %v", op, err)
		}
	}
	if strings.Contains(msg, "Usage") {
		t.Fatalf("não deve citar Usage: %v", err)
	}
}

func TestStackAggregateJSONAndOrder(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - a.go\n  - rd.md\n---\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "head")

	res, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    "HEAD~1",
		Head:    "HEAD",
		Ops:     []Op{OpGuard, OpDrift, OpReview},
		Ticket:  filepath.Join(dir, "rd.md"),
		TestCmd: "true",
		MaxIter: 1,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("esperava ok: %+v", res)
	}
	if len(res.Steps) != 3 {
		t.Fatalf("steps=%d", len(res.Steps))
	}
	if res.Steps[0].Op != "guard" || res.Steps[1].Op != "drift" || res.Steps[2].Op != "review" {
		t.Fatalf("ordem: %+v", res.Steps)
	}

	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ok", "steps", "total_duration_ms"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("faltando chave %s em %s", key, raw)
		}
	}
	steps, _ := payload["steps"].([]any)
	first, _ := steps[0].(map[string]any)
	for _, key := range []string{"op", "ok", "exit_code", "duration_ms", "result"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("step sem %s: %s", key, raw)
		}
	}
}

func failFastFixture(t *testing.T) string {
	t.Helper()
	dir := gitRepo(t)
	hidden := filepath.Join(dir, "tests", "hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(hidden, "sentinel.txt"), "ok\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(hidden, "sentinel.txt"), "changed\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - tests/\n---\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "touch protected")
	return dir
}

func TestStackFailFastSerial(t *testing.T) {
	dir := failFastFixture(t)
	res, err := Run(context.Background(), &Options{
		RepoDir: dir, Base: "HEAD~1", Head: "HEAD",
		Ops: []Op{OpGuard, OpDrift, OpReview}, FailFast: true, Jobs: 1,
		Ticket: filepath.Join(dir, "rd.md"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || len(res.Steps) != 1 || res.Steps[0].Op != "guard" || res.Steps[0].OK {
		t.Fatalf("fail-fast serial: %+v", res)
	}
	if ExitCode(res) == 0 {
		t.Fatal("exit code agregado deveria ser ≠ 0")
	}
}

func TestStackFailFastParallel(t *testing.T) {
	dir := failFastFixture(t)
	res, err := Run(context.Background(), &Options{
		RepoDir: dir, Base: "HEAD~1", Head: "HEAD",
		Ops: []Op{OpGuard, OpDrift, OpReview}, FailFast: true, Jobs: 4,
		Ticket: filepath.Join(dir, "rd.md"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("esperava falha no paralelo")
	}
	foundGuard := false
	for _, step := range res.Steps {
		if step.Op == "guard" {
			foundGuard = true
			if step.OK {
				t.Fatal("guard deveria falhar")
			}
		}
	}
	if !foundGuard || ExitCode(res) == 0 {
		t.Fatalf("fail-fast paralelo: %+v", res)
	}
}

func TestStackWithoutFailFastRunsAll(t *testing.T) {
	dir := failFastFixture(t)
	res, err := Run(context.Background(), &Options{
		RepoDir: dir, Base: "HEAD~1", Head: "HEAD",
		Ops: []Op{OpGuard, OpDrift}, FailFast: false,
		Ticket: filepath.Join(dir, "rd.md"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("sem fail-fast deveria rodar todas, got %d", len(res.Steps))
	}
}

func TestSharedDiffAndIndexOnce(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc Y() {}\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - a.go\n  - rd.md\n---\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "head")

	var diffCalls, indexCalls, memCalls, nameCalls int
	loaders := &Loaders{
		UnifiedDiff: func(repo, base, head string) (string, error) {
			diffCalls++
			return gitUnifiedDiff(repo, base, head)
		},
		NameStatus: func(repo, base, head string) (string, error) {
			nameCalls++
			return gitNameStatus(repo, base, head)
		},
		ChangedFiles: func(repo, base, head string) ([]string, error) {
			indexCalls++
			return gitChangedFiles(repo, base, head)
		},
		Invariants: func(repo string) ([]mem.Invariant, error) {
			memCalls++
			return loadInvariants(repo)
		},
		GraphIndex: func(repo string) (*graph.Index, error) {
			return &graph.Index{Root: repo}, nil
		},
	}

	res, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    "HEAD~1",
		Head:    "HEAD",
		Ops:     []Op{OpGuard, OpReview, OpDrift, OpVerify},
		Ticket:  filepath.Join(dir, "rd.md"),
		TestCmd: "true",
		MaxIter: 1,
		Timeout: 30 * time.Second,
		Loaders: loaders,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diffCalls != 1 {
		t.Fatalf("diff calculado %d vezes, want 1", diffCalls)
	}
	if indexCalls != 1 {
		t.Fatalf("índice (changed files) calculado %d vezes, want 1", indexCalls)
	}
	if nameCalls != 1 {
		t.Fatalf("name-status calculado %d vezes, want 1", nameCalls)
	}
	if memCalls != 1 {
		t.Fatalf("memória carregada %d vezes, want 1", memCalls)
	}
	if len(res.Steps) != 4 {
		t.Fatalf("steps=%d", len(res.Steps))
	}
}

func TestRunUnknownOp(t *testing.T) {
	_, err := Run(context.Background(), &Options{
		Ops: []Op{"bogus"},
	})
	if err == nil || !strings.Contains(err.Error(), "operação desconhecida") {
		t.Fatalf("err=%v", err)
	}
}

func TestStackParallelOrderAndJobs1(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - a.go\n  - rd.md\n---\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "head")

	res, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    "HEAD~1",
		Head:    "HEAD",
		Ops:     []Op{OpGuard, OpDrift, OpReview},
		Jobs:    4,
		Ticket:  filepath.Join(dir, "rd.md"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Steps) != 3 {
		t.Fatalf("steps=%d", len(res.Steps))
	}
	if res.Steps[0].Op != "guard" || res.Steps[1].Op != "drift" || res.Steps[2].Op != "review" {
		t.Fatalf("ordem JSON: %+v", res.Steps)
	}

	serial, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    "HEAD~1",
		Head:    "HEAD",
		Ops:     []Op{OpGuard, OpDrift, OpReview},
		Jobs:    1,
		Ticket:  filepath.Join(dir, "rd.md"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(serial.Steps) != 3 {
		t.Fatalf("jobs=1 steps=%d", len(serial.Steps))
	}
}

func TestResolveJobs(t *testing.T) {
	if got := ResolveJobs(1); got != 1 {
		t.Fatalf("jobs=1 -> %d", got)
	}
	if got := ResolveJobs(0); got < 1 {
		t.Fatalf("jobs=0 -> %d", got)
	}
	if got := ResolveJobs(999); got != maxJobs {
		t.Fatalf("jobs=999 -> %d want %d", got, maxJobs)
	}
}

func TestStackVerifyRunsOnHeadWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "go.mod"), "module demo\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "demo.go"), "package demo\nfunc Add(a, b int) int { return a + b }\n")
	writeFile(t, filepath.Join(dir, "demo_test.go"), "package demo\nimport \"testing\"\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 { t.Fatal() }\n}\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	baseOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseOut))

	writeFile(t, filepath.Join(dir, "marker_test.go"), "package demo\nimport \"testing\"\nfunc TestMarkerOnlyOnHead(t *testing.T) {\n\tt.Fatal(\"marker: only on head\")\n}\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "head-with-failing-marker")
	headOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headOut))
	git(t, dir, "checkout", "-q", base)

	res, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    base,
		Head:    head,
		Ops:     []Op{OpVerify},
		TestCmd: "go test ./...",
		MaxIter: 1,
		Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("stack verify must fail when head has a failing marker test absent from the current worktree")
	}
	if len(res.Steps) != 1 || res.Steps[0].Op != "verify" {
		t.Fatalf("steps=%+v", res.Steps)
	}
	raw, err := json.Marshal(res.Steps[0].Result)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["head_sha"] != head {
		t.Fatalf("head_sha=%v want %s", payload["head_sha"], head)
	}
	workdir, _ := payload["workdir"].(string)
	if workdir == "" || workdir == dir {
		t.Fatalf("expected detached workdir, got %q", workdir)
	}
}

func BenchmarkStackVsSequential(b *testing.B) {
	dir := gitRepoB(b)
	writeFileB(b, filepath.Join(dir, "a.go"), "package a\n")
	gitB(b, dir, "add", "-A")
	gitB(b, dir, "commit", "-m", "base")
	writeFileB(b, filepath.Join(dir, "a.go"), "package a\nfunc Z() {}\n")
	writeFileB(b, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - a.go\n  - rd.md\n---\n")
	gitB(b, dir, "add", "-A")
	gitB(b, dir, "commit", "-m", "head")

	opts := &Options{
		RepoDir: dir,
		Base:    "HEAD~1",
		Head:    "HEAD",
		Ops:     []Op{OpGuard, OpDrift, OpReview},
		Ticket:  filepath.Join(dir, "rd.md"),
	}

	b.Run("stack_parallel", func(b *testing.B) {
		o := *opts
		o.Jobs = 4
		for i := 0; i < b.N; i++ {
			if _, err := Run(context.Background(), &o); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("stack_jobs1", func(b *testing.B) {
		o := *opts
		o.Jobs = 1
		for i := 0; i < b.N; i++ {
			if _, err := Run(context.Background(), &o); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, op := range opts.Ops {
				if _, err := Run(context.Background(), &Options{
					RepoDir: opts.RepoDir,
					Base:    opts.Base,
					Head:    opts.Head,
					Ops:     []Op{op},
					Jobs:    1,
					Ticket:  opts.Ticket,
				}); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "config", "user.email", "t@t")
	git(t, dir, "config", "user.name", "t")
	git(t, dir, "commit", "--allow-empty", "-m", "root")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitRepoB(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	gitB(b, dir, "init")
	gitB(b, dir, "config", "user.email", "t@t")
	gitB(b, dir, "config", "user.name", "t")
	gitB(b, dir, "commit", "--allow-empty", "-m", "root")
	return dir
}

func gitB(b *testing.B, dir string, args ...string) {
	b.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func writeFileB(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}
