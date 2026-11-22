package merge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo creates a repo with a main branch, a base commit and .toolbelt ignored.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.name", "t")
	git(t, dir, "config", "user.email", "t@t")
	write(t, dir, ".gitignore", ".toolbelt/\n")
	write(t, dir, "shared.txt", "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\n")
	write(t, dir, "a.txt", "a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	return dir
}

// addTicket branches off main with edits and registers the ticket as done.
func addTicket(t *testing.T, dir string, deps []string, edits map[string]string) ticket.Ticket {
	t.Helper()
	return addTicketForce(t, dir, deps, edits, nil)
}

// addTicketForce is addTicket with `git add -f` on the listed (ignored) paths.
func addTicketForce(t *testing.T, dir string, deps []string, edits map[string]string, force []string) ticket.Ticket {
	t.Helper()
	s := ticket.Open(dir)
	tk, err := s.Create(&ticket.Ticket{Title: "t", Dependencies: deps})
	if err != nil {
		t.Fatal(err)
	}
	base := git(t, dir, "rev-parse", "main")
	br := "work/" + strings.ToLower(tk.ID)
	git(t, dir, "checkout", "-q", "-b", br, base)
	for f, c := range edits {
		write(t, dir, f, c)
		if strings.HasPrefix(f, ".githooks/") {
			_ = os.Chmod(filepath.Join(dir, f), 0o755)
		}
	}
	git(t, dir, "add", "-A")
	for _, f := range force {
		git(t, dir, "add", "-f", "--", f)
	}
	git(t, dir, "commit", "-q", "-m", tk.ID)
	head := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", "main")
	if _, err := s.Transition(tk.ID, ticket.StatusRunning); err != nil {
		t.Fatal(err)
	}
	out, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusDone
		x.Result = &ticket.Result{Branch: br, BaseCommit: base, HeadCommit: head}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertNoTempWorktree(t *testing.T, dir string) {
	t.Helper()
	wl := git(t, dir, "worktree", "list", "--porcelain")
	if n := strings.Count(wl, "worktree "); n != 1 {
		t.Fatalf("worktree residual:\n%s", wl)
	}
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDisjointMergeAdvancesBranch(t *testing.T) {
	dir := newRepo(t)
	a := addTicket(t, dir, nil, map[string]string{"x.txt": "x\n"})
	b := addTicket(t, dir, nil, map[string]string{"y.txt": "y\n"})
	start := git(t, dir, "rev-parse", "main")
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{a.ID, "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied || res.ResultSHA == start || git(t, dir, "rev-parse", "main") != res.ResultSHA {
		t.Fatalf("main não avançou: %+v", res)
	}
	if readFile(t, dir, "x.txt") != "x\n" || readFile(t, dir, "y.txt") != "y\n" {
		t.Fatal("working tree não atualizada")
	}
	for _, f := range res.Files {
		if f.Status != "merge" {
			t.Errorf("status %+v", f)
		}
	}
	s := ticket.Open(dir)
	for _, id := range []string{a.ID, b.ID} {
		if tk, _ := s.Get(id); tk.MergedAt == nil {
			t.Errorf("%s sem MergedAt", id)
		}
	}
	assertNoTempWorktree(t, dir)
	// already merged requires --force
	if _, err := Run(&Options{Repo: dir, Store: s, IDs: []string{a.ID}}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("esperava erro de já mergeado: %v", err)
	}
	if _, err := Run(&Options{Repo: dir, Store: s, IDs: []string{a.ID}, Force: true}); err != nil {
		t.Fatalf("force: %v", err)
	}
}

func TestOverlapNonConflicting(t *testing.T) {
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"shared.txt": "L1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nL9\n"})
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || len(res.Files[0].Tickets) != 2 || res.Files[0].Status != "merge" {
		t.Fatalf("overlap: %+v", res.Files)
	}
	if got := readFile(t, dir, "shared.txt"); !strings.HasPrefix(got, "L1\n") || !strings.HasSuffix(got, "L9\n") {
		t.Fatalf("conteúdo: %q", got)
	}
}

func conflictRepo(t *testing.T) string {
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"shared.txt": "one\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "two\n"})
	return dir
}

func TestConflictFailChangesNothing(t *testing.T) {
	dir := conflictRepo(t)
	start := git(t, dir, "rev-parse", "main")
	s := ticket.Open(dir)
	res, err := Run(&Options{Repo: dir, Store: s, IDs: []string{"TKT-001", "TKT-002"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("esperava ErrConflict: %v", err)
	}
	if res == nil || len(res.Conflicts) != 1 || res.Conflicts[0].File != "shared.txt" || res.Conflicts[0].Ticket != "TKT-002" {
		t.Fatalf("conflitos: %+v", res)
	}
	if res.Files[0].Status != "conflict" {
		t.Fatalf("status: %+v", res.Files)
	}
	if git(t, dir, "rev-parse", "main") != start {
		t.Fatal("ref mudou")
	}
	if tk, _ := s.Get("TKT-001"); tk.MergedAt != nil {
		t.Fatal("MergedAt marcado")
	}
	if st := git(t, dir, "status", "--porcelain"); st != "" {
		t.Fatalf("árvore suja: %s", st)
	}
	assertNoTempWorktree(t, dir)
}

func TestConflictLastWins(t *testing.T) {
	dir := conflictRepo(t)
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategyLastWins})
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, dir, "shared.txt") != "two\n" {
		t.Fatalf("last_wins: %q", readFile(t, dir, "shared.txt"))
	}
	if res.Files[0].Status != "last_wins" || len(res.Conflicts) != 1 || res.Conflicts[0].Resolution != "last_wins" {
		t.Fatalf("relatório: %+v", res)
	}
	assertNoTempWorktree(t, dir)
}

func TestDryRunAndOutput(t *testing.T) {
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"x.txt": "x\n"})
	start := git(t, dir, "rev-parse", "main")
	s := ticket.Open(dir)
	res, err := Run(&Options{Repo: dir, Store: s, IDs: []string{"1"}, DryRun: true})
	if err != nil || res.Applied || res.ResultSHA == "" {
		t.Fatalf("dry: %+v %v", res, err)
	}
	if git(t, dir, "rev-parse", "main") != start {
		t.Fatal("dry-run moveu ref")
	}
	if tk, _ := s.Get("TKT-001"); tk.MergedAt != nil {
		t.Fatal("dry-run marcou MergedAt")
	}
	patch := filepath.Join(t.TempDir(), "out.patch")
	if _, err := Run(&Options{Repo: dir, Store: s, IDs: []string{"1"}, Output: patch}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(patch); !strings.Contains(string(b), "+++ b/x.txt") {
		t.Fatalf("patch: %s", b)
	}
	if git(t, dir, "rev-parse", "main") != start {
		t.Fatal("--output moveu ref")
	}
	assertNoTempWorktree(t, dir)
	git(t, dir, "apply", "--check", patch)
}

func TestIntoNonCheckedOutBranchCAS(t *testing.T) {
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"x.txt": "x\n"})
	git(t, dir, "branch", "integ", "main")
	mainSHA := git(t, dir, "rev-parse", "main")
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"1"}, Into: "integ"})
	if err != nil {
		t.Fatal(err)
	}
	if git(t, dir, "rev-parse", "integ") != res.ResultSHA || git(t, dir, "rev-parse", "main") != mainSHA {
		t.Fatal("CAS errado")
	}
	if _, err := os.Stat(filepath.Join(dir, "x.txt")); err == nil {
		t.Fatal("working tree do main não deveria mudar")
	}
	assertNoTempWorktree(t, dir)
}

func TestDirtyCheckedOutTargetRefused(t *testing.T) {
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"x.txt": "x\n"})
	write(t, dir, "a.txt", "sujo\n")
	start := git(t, dir, "rev-parse", "main")
	_, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"1"}})
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("esperava recusa: %v", err)
	}
	if git(t, dir, "rev-parse", "main") != start {
		t.Fatal("ref mudou")
	}
	assertNoTempWorktree(t, dir)
}

func TestNotDoneRejected(t *testing.T) {
	dir := newRepo(t)
	s := ticket.Open(dir)
	if _, err := s.Create(&ticket.Ticket{Title: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(&Options{Repo: dir, Store: s, IDs: []string{"1"}}); err == nil || !strings.Contains(err.Error(), "is not done") {
		t.Fatalf("err: %v", err)
	}
}

func TestDependencyOrder(t *testing.T) {
	dir := newRepo(t)
	// TKT-001 depends on TKT-002 -> 002 must come first even when passed last.
	b := addTicket(t, dir, nil, map[string]string{"shared.txt": "base-dep\n"})
	_ = b
	s := ticket.Open(dir)
	tk, err := s.Create(&ticket.Ticket{Title: "filho", Dependencies: []string{"TKT-001"}})
	if err != nil {
		t.Fatal(err)
	}
	base := git(t, dir, "rev-parse", "main")
	git(t, dir, "checkout", "-q", "-b", "work/child", base)
	write(t, dir, "shared.txt", "child\n")
	git(t, dir, "commit", "-q", "-am", "child")
	head := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", "main")
	s.Transition(tk.ID, ticket.StatusRunning)
	if _, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusDone
		x.Result = &ticket.Result{BaseCommit: base, HeadCommit: head}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	res, err := Run(&Options{Repo: dir, Store: s, IDs: []string{tk.ID, "TKT-001"}, Strategy: StrategyLastWins})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tickets[0].ID != "TKT-001" || res.Tickets[1].ID != tk.ID {
		t.Fatalf("ordem: %+v", res.Tickets)
	}
	if readFile(t, dir, "shared.txt") != "child\n" {
		t.Fatal("filho deveria vencer")
	}
}

func TestParseStrategyAndRefs(t *testing.T) {
	for in, want := range map[string]Strategy{"": StrategyFail, "prompt": StrategyPrompt, " Semantic ": StrategySemantic, "last-wins": StrategyLastWins} {
		if got, err := ParseStrategy(in); err != nil || got != want {
			t.Errorf("%q: %v %v", in, got, err)
		}
	}
	if _, err := ParseStrategy("xyz"); err == nil || !strings.Contains(err.Error(), "semantic") {
		t.Errorf("xyz deveria falhar listando as estratégias: %v", err)
	}
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"x.txt": "x\n"})
	if _, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"1"}, Into: "--help"}); err == nil {
		t.Fatal("into com '-' deveria falhar")
	}
	if _, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"1"}, Into: "nao-existe"}); err == nil {
		t.Fatal("branch inexistente deveria falhar")
	}
}

func TestOverlapDetectedAcrossRename(t *testing.T) {
	dir := newRepo(t)
	// ticket A renames shared.txt -> moved.txt
	s := ticket.Open(dir)
	tk, err := s.Create(&ticket.Ticket{Title: "rename"})
	if err != nil {
		t.Fatal(err)
	}
	base := git(t, dir, "rev-parse", "main")
	git(t, dir, "checkout", "-q", "-b", "work/rename", base)
	git(t, dir, "mv", "shared.txt", "moved.txt")
	git(t, dir, "commit", "-q", "-m", "rename")
	head := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", "main")
	if _, err := s.Transition(tk.ID, ticket.StatusRunning); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusDone
		x.Result = &ticket.Result{Branch: "work/rename", BaseCommit: base, HeadCommit: head}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// ticket B edits the old path
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nL9\n"})
	res, err := Run(&Options{Repo: dir, Store: s, IDs: []string{"TKT-001", "TKT-002"}, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range res.Files {
		if f.Path == "shared.txt" {
			found = true
			if len(f.Tickets) != 2 {
				t.Fatalf("overlap em shared.txt deveria listar 2 tickets: %+v", f)
			}
		}
	}
	if !found {
		t.Fatalf("shared.txt (caminho antigo do rename) ausente: %+v", res.Files)
	}
}

func TestLastWinsSurvivesGPGSignAndHooks(t *testing.T) {
	dir := conflictRepo(t)
	// gpgsign without a key + a hook that always fails: the internal worktree must ignore both.
	git(t, dir, "config", "commit.gpgsign", "true")
	git(t, dir, "config", "gpg.program", "false")
	hooks := filepath.Join(dir, ".git", "hooks")
	for _, h := range []string{"commit-msg", "pre-commit", "prepare-commit-msg", "pre-merge-commit"} {
		write(t, hooks, h, "#!/bin/sh\nexit 1\n")
		if err := os.Chmod(filepath.Join(hooks, h), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	start := git(t, dir, "rev-parse", "main")
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategyLastWins, DryRun: true})
	if err != nil {
		t.Fatalf("last_wins deveria ignorar gpgsign/hooks: %v", err)
	}
	if res.Applied || len(res.Conflicts) != 1 || res.Conflicts[0].Resolution != "last_wins" {
		t.Fatalf("relatório: %+v", res)
	}
	if git(t, dir, "rev-parse", "main") != start {
		t.Fatal("dry-run moveu ref")
	}
	// ticket 2's commit must not be lost: the target..result diff contains "two"
	if got := git(t, dir, "show", res.ResultSHA+":shared.txt"); got != "two" {
		t.Fatalf("conteúdo do resultado: %q", got)
	}
	if n := git(t, dir, "rev-list", "--count", start+".."+res.ResultSHA); n != "2" {
		t.Fatalf("esperava 2 commits integrados, got %s", n)
	}
}

// Hook that only fails on the post-conflict commit: before the fix, --skip
// swallowed the ticket's commit and the result lacked "two".
func TestLastWinsContinueFailureDoesNotLoseCommit(t *testing.T) {
	dir := conflictRepo(t)
	hooks := filepath.Join(dir, ".git", "hooks")
	write(t, hooks, "commit-msg", "#!/bin/sh\nif grep -q 'Conflicts' \"$1\"; then exit 1; fi\nexit 0\n")
	if err := os.Chmod(filepath.Join(hooks, "commit-msg"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategyLastWins, DryRun: true})
	if err != nil {
		t.Fatalf("hook ignorado no worktree interno: %v", err)
	}
	if got := git(t, dir, "show", res.ResultSHA+":shared.txt"); got != "two" {
		t.Fatalf("commit do TKT-002 perdido: %q", got)
	}
}

func markerHook(marker string) string {
	return "#!/bin/sh\necho pwned > " + marker + "\n"
}

// hooksRepo: core.hooksPath=.githooks with tracked hooks; the ticket swaps the hook.
func hooksRepo(t *testing.T, hook string, trapped bool) (dir, marker string) {
	t.Helper()
	dir = newRepo(t)
	marker = filepath.Join(t.TempDir(), "marker")
	body := "#!/bin/sh\nexit 0\n"
	if trapped {
		body = markerHook(marker)
	}
	write(t, dir, ".githooks/"+hook, body)
	chmodHooks(t, dir)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "hooks")
	git(t, dir, "config", "core.hooksPath", ".githooks")
	_ = os.Remove(marker) // the commit above may have fired the hook
	return dir, marker
}

func chmodHooks(t *testing.T, dir string) {
	t.Helper()
	ents, _ := os.ReadDir(filepath.Join(dir, ".githooks"))
	for _, e := range ents {
		_ = os.Chmod(filepath.Join(dir, ".githooks", e.Name()), 0o755)
	}
}

func TestSensitiveHooksRefusedAndHardenedFF(t *testing.T) {
	dir, marker := hooksRepo(t, "post-merge", false)
	addTicket(t, dir, nil, map[string]string{".githooks/post-merge": markerHook(marker)})
	start := git(t, dir, "rev-parse", "main")
	s := ticket.Open(dir)

	res, err := Run(&Options{Repo: dir, Store: s, IDs: []string{"TKT-001"}})
	if !errors.Is(err, ErrSensitive) {
		t.Fatalf("esperava ErrSensitive: %v", err)
	}
	if res == nil || len(res.Sensitive) != 1 || res.Sensitive[0].Ticket != "TKT-001" || res.Sensitive[0].Path != ".githooks/post-merge" {
		t.Fatalf("sensitive: %+v", res)
	}
	if git(t, dir, "rev-parse", "main") != start {
		t.Fatal("ref mudou sem --allow-sensitive")
	}
	if tk, _ := s.Get("TKT-001"); tk.MergedAt != nil {
		t.Fatal("MergedAt marcado")
	}

	// dry-run only reports
	res, err = Run(&Options{Repo: dir, Store: s, IDs: []string{"TKT-001"}, DryRun: true})
	if err != nil || len(res.Sensitive) != 1 {
		t.Fatalf("dry-run: %v %+v", err, res)
	}

	_, err = Run(&Options{Repo: dir, Store: s, IDs: []string{"TKT-001"}, AllowSensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	if git(t, dir, "rev-parse", "main") == start {
		t.Fatal("ff não aconteceu")
	}
	chmodHooks(t, dir)
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatal("post-merge do agente executou")
	}
}

func TestLastWinsDoesNotRunTrackedPostCheckout(t *testing.T) {
	dir, marker := hooksRepo(t, "post-checkout", true)
	addTicket(t, dir, nil, map[string]string{"shared.txt": "one\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "two\n"})
	_ = os.Remove(marker) // setup checkouts fired the hook
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategyLastWins, AllowSensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 {
		t.Fatalf("conflitos: %+v", res.Conflicts)
	}
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatal("post-checkout do agente executou")
	}
}

func TestSensitiveToolbeltConfigRefused(t *testing.T) {
	dir := newRepo(t)
	addTicketForce(t, dir, nil, map[string]string{".toolbelt/config.yaml": "agent_cmd: evil\n"}, []string{".toolbelt/config.yaml"})
	start := git(t, dir, "rev-parse", "main")
	_, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001"}})
	if !errors.Is(err, ErrSensitive) {
		t.Fatalf("esperava ErrSensitive: %v", err)
	}
	if git(t, dir, "rev-parse", "main") != start {
		t.Fatal("ref mudou")
	}
}

func TestSensitiveCustomHooksPathAndWorkflows(t *testing.T) {
	dir := newRepo(t)
	git(t, dir, "config", "core.hooksPath", "tools/hooks")
	addTicket(t, dir, nil, map[string]string{"tools/hooks/pre-commit": "x\n", ".github/workflows/ci.yml": "y\n", "sub/.gitattributes": "* filter=x\n"})
	res, _ := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001"}, DryRun: true})
	if res == nil || len(res.Sensitive) != 3 {
		t.Fatalf("sensitive: %+v", res)
	}
}
