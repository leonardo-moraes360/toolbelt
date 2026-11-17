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
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - a.go\n  - rd.md\n---\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "head")

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

func TestStackFailFast(t *testing.T) {
	dir := gitRepo(t)
	hidden := filepath.Join(dir, "tests", "hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(hidden, "sentinel.txt"), "ok\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(hidden, "sentinel.txt"), "changed\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - tests/\n---\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "touch protected")

	res, err := Run(context.Background(), &Options{
		RepoDir:  dir,
		Base:     "HEAD~1",
		Head:     "HEAD",
		Ops:      []Op{OpGuard, OpDrift, OpReview},
		FailFast: true,
		Ticket:   filepath.Join(dir, "rd.md"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("esperava falha")
	}
	if len(res.Steps) != 1 {
		t.Fatalf("fail-fast deveria parar após 1 step, got %d", len(res.Steps))
	}
	if res.Steps[0].Op != "guard" || res.Steps[0].OK {
		t.Fatalf("primeiro step: %+v", res.Steps[0])
	}
	if ExitCode(res) == 0 {
		t.Fatal("exit code agregado deveria ser ≠ 0")
	}

	resAll, err := Run(context.Background(), &Options{
		RepoDir:  dir,
		Base:     "HEAD~1",
		Head:     "HEAD",
		Ops:      []Op{OpGuard, OpDrift},
		FailFast: false,
		Ticket:   filepath.Join(dir, "rd.md"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resAll.Steps) != 2 {
		t.Fatalf("sem fail-fast deveria rodar todas, got %d", len(resAll.Steps))
	}
}

func TestSharedDiffAndIndexOnce(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc Y() {}\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - a.go\n  - rd.md\n---\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "head")

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

func TestStackPrepareErrorInsideStep(t *testing.T) {
	dir := gitRepo(t)
	res, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    "DOES-NOT-EXIST",
		Head:    "HEAD",
		Ops:     []Op{OpGuard, OpDrift},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("esperava falha")
	}
	if len(res.Steps) != 1 {
		t.Fatalf("steps=%d want 1", len(res.Steps))
	}
	if res.Steps[0].Error == "" {
		t.Fatalf("error deveria estar no step: %+v", res.Steps[0])
	}
	if res.Steps[0].Op != "guard" {
		t.Fatalf("op=%s", res.Steps[0].Op)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	steps, _ := payload["steps"].([]any)
	first, _ := steps[0].(map[string]any)
	if first["error"] == nil || first["error"] == "" {
		t.Fatalf("JSON sem error no step: %s", raw)
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
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	baseOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseOut))

	writeFile(t, filepath.Join(dir, "marker_test.go"), "package demo\nimport \"testing\"\nfunc TestMarkerOnlyOnHead(t *testing.T) {\n\tt.Fatal(\"marker: only on head\")\n}\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "head-with-failing-marker")
	headOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headOut))
	runGit(t, dir, "checkout", "-q", base)

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

func TestStackDurationMsIsFractionalFloat(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "head")

	res, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    "HEAD~1",
		Head:    "HEAD",
		Ops:     []Op{OpGuard},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	total, ok := payload["total_duration_ms"].(float64)
	if !ok {
		t.Fatalf("total_duration_ms type %T in %s", payload["total_duration_ms"], raw)
	}
	if total <= 0 {
		t.Fatalf("total_duration_ms=%v should keep sub-ms fraction (>0)", total)
	}
	steps, _ := payload["steps"].([]any)
	first, _ := steps[0].(map[string]any)
	stepDur, ok := first["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms type %T", first["duration_ms"])
	}
	if stepDur <= 0 {
		t.Fatalf("step duration_ms=%v want >0", stepDur)
	}
	if durationMillis(500*time.Microsecond) == 0 {
		t.Fatal("durationMillis should not truncate sub-millisecond to 0")
	}
	if got := durationMillis(1500 * time.Microsecond); got < 1.4 || got > 1.6 {
		t.Fatalf("durationMillis(1.5ms)=%v", got)
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

	b.Run("stack", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := Run(context.Background(), opts); err != nil {
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
					Ticket:  opts.Ticket,
				}); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

func TestPreparePinsHeadSHAOnce(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "head")
	wantHead := revParse(t, dir, "HEAD")
	wantBase := revParse(t, dir, "HEAD~1")

	shared, err := Prepare(&Options{RepoDir: dir, Base: "HEAD~1", Head: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if shared.Head != wantHead {
		t.Fatalf("Head pinado=%q want %q", shared.Head, wantHead)
	}
	if shared.Base != wantBase {
		t.Fatalf("Base pinado=%q want %q", shared.Base, wantBase)
	}

	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc Y() {}\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "moved-head")
	if shared.Head == revParse(t, dir, "HEAD") {
		t.Fatal("Prepare não deve acompanhar HEAD após o pin")
	}
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "t@t")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "root")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
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

func TestStackMissingTicketFails(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "head")

	missing := filepath.Join(dir, "no-such-ticket.md")
	res, err := Run(context.Background(), &Options{
		RepoDir: dir,
		Base:    "HEAD~1",
		Head:    "HEAD",
		Ops:     []Op{OpDrift, OpReview},
		Ticket:  missing,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("ticket ausente deveria falhar o stack")
	}
	if ExitCode(res) == 0 {
		t.Fatal("exit code agregado deveria ser ≠ 0")
	}
	for _, step := range res.Steps {
		if step.OK {
			t.Fatalf("step %s deveria falhar: %+v", step.Op, step)
		}
		if !strings.Contains(step.Error, "ticket não encontrado") {
			t.Fatalf("step %s deveria citar ticket não encontrado: %+v", step.Op, step)
		}
		if !strings.Contains(step.Error, missing) {
			t.Fatalf("step %s deveria citar o caminho %q: %+v", step.Op, missing, step)
		}
	}
}
