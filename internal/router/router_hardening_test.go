//go:build !windows

package router

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// repoWithGitlink creates a repo whose base commit contains a gitlink "sub".
func repoWithGitlink(t *testing.T) string {
	t.Helper()
	root := newRepo(t)
	head := runGit(t, root, "rev-parse", "HEAD")
	runGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+head+",sub")
	runGit(t, root, "commit", "-q", "-m", "gitlink")
	return root
}

// Finding 1: a .git nested inside a gitlink must not lead to filter execution.
func TestNestedGitInSubmoduleIsRefusedAndFilterNeverRuns(t *testing.T) {
	root := repoWithGitlink(t)
	st := ticket.Open(root)
	tk := mk(t, st, "submodulo")
	cfg := testCfg()
	cfg.Retry.MaxAttempts = 3
	marker := filepath.Join(t.TempDir(), "marker")
	script := fmt.Sprintf(`mkdir -p sub/.git/objects sub/.git/refs/heads
echo 'ref: refs/heads/main' > sub/.git/HEAD
printf '[core]\n\trepositoryformatversion = 0\n[filter "x"]\n\tclean = touch %s\n' > sub/.git/config
echo '* filter=x' > sub/.gitattributes
echo data > sub/file.txt
echo y > f.txt`, marker)
	r := newRouter(t, root, func(o *Options) { o.Config = cfg; o.AgentCmd = sh(script) })
	runWithTimeout(context.Background(), t, r)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("filtro clean do submódulo do agente foi executado pelo router")
	}
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusFailed || got.Attempts != 1 {
		t.Fatalf("esperava failed sem retry: %+v", got)
	}
	if got.Result == nil || !strings.Contains(got.Result.Error, "nested") || got.Result.HeadCommit != "" {
		t.Fatalf("resultado: %+v", got.Result)
	}
}

func TestNestedGitFileOrDirAnywhereIsRefused(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "nested")
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("mkdir -p a/b/c && echo 'gitdir: /x' > a/b/c/.git; echo y > f.txt") })
	runWithTimeout(context.Background(), t, r)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusFailed || !strings.Contains(got.Result.Error, "nested") {
		t.Fatalf("%+v %+v", got, got.Result)
	}
}

func TestStagedGitmodulesIsRefused(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "gitmodules")
	r := newRouter(t, root, func(o *Options) {
		o.AgentCmd = sh(`printf '[submodule "a"]\n\tpath = a\n\turl = /nope\n' > .gitmodules`)
	})
	runWithTimeout(context.Background(), t, r)
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusFailed || got.Attempts != 1 || !strings.Contains(got.Result.Error, "gitlink or .gitmodules") || got.Result.HeadCommit != "" {
		t.Fatalf("%+v %+v", got, got.Result)
	}
}

// Gitlink already in the worktree index (the agent can use git update-index):
// the commit is refused. Called directly because the agent's Landlock sandbox
// does not always let its git run.
func TestStagedGitlinkIsRefused(t *testing.T) {
	root := newRepo(t)
	ws := filepath.Join(t.TempDir(), "ws")
	runGit(t, root, "worktree", "add", "-q", "-b", "tb-workspace/x", ws)
	head := runGit(t, ws, "rev-parse", "HEAD")
	if err := os.Mkdir(filepath.Join(ws, "newsub"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, ws, "update-index", "--add", "--cacheinfo", "160000,"+head+",newsub")
	if err := os.WriteFile(filepath.Join(ws, "f.txt"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, root, nil)
	_, err := r.commitWorkspace(ws, &ticket.Ticket{ID: "TKT-001", Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "gitlink or .gitmodules") {
		t.Fatalf("esperava recusa: %v", err)
	}
	if got := runGit(t, ws, "rev-parse", "HEAD"); got != head {
		t.Fatal("commit foi criado")
	}
}

// Finding 2: a descendant that calls setsid escapes the group; it must be killed and the ticket fails without git.
func TestSetsidGrandchildIsKilledAndTicketFailsWithoutCommit(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "escape")
	cfg := testCfg()
	cfg.Retry.MaxAttempts = 3
	// pidfile inside the worktree (Landlock only allows writing there)
	script := `setsid sh -c 'echo $$ > pidfile; exec sleep 37' &
i=0; while [ ! -s pidfile ] && [ $i -lt 500 ]; do sleep 0.01; i=$((i+1)); done
echo x > f.txt`
	r := newRouter(t, root, func(o *Options) { o.Config = cfg; o.AgentCmd = sh(script) })
	runWithTimeout(context.Background(), t, r)
	pidGone(t, readPid(t, filepath.Join(root, ".toolbelt", "workspaces", tk.ID, "pidfile")), "escaped process")
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusFailed || got.Attempts != 1 {
		t.Fatalf("esperava failed sem retry: %+v", got)
	}
	if got.Result.HeadCommit != "" {
		t.Fatalf("não deveria ter commitado: %+v", got.Result)
	}
	if base := runGit(t, root, "rev-parse", "tb-workspace/"+tk.ID); base != got.Result.BaseCommit {
		t.Fatalf("branch do ticket avançou (%s != %s): git rodou no worktree", base, got.Result.BaseCommit)
	}
	kinds := alertKinds(t, root)
	if countKind(kinds, AlertAgentEscaped) != 1 || countKind(kinds, AlertWorkerCrash) != 0 {
		t.Fatalf("alertas: %v", kinds)
	}
	as, _ := ReadAlerts(root, AlertFilter{Kind: AlertAgentEscaped})
	if as[0].Severity != SevError {
		t.Fatalf("severidade: %+v", as[0])
	}
}

func TestBackgroundChildInSameGroupStillSucceeds(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "bg")
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("sleep 30 & echo ok > f.txt") })
	runWithTimeout(context.Background(), t, r)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusDone || got.Result.HeadCommit == "" {
		t.Fatalf("%+v %+v", got, got.Result)
	}
	if countKind(alertKinds(t, root), AlertAgentEscaped) != 0 {
		t.Fatal("falso positivo de agent_escaped")
	}
}

func noRetryCfg() config.RouterConfig {
	c := testCfg()
	c.Retry.MaxAttempts = 1
	return c
}

// Finding 3: kill requests from previous runs do not kill the first worker.
func TestStaleKillRequestBeforeRunIsCleared(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "a")
	if _, err := RequestKill(root, "w1", "velho", time.Now()); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, root, func(o *Options) {
		o.Config = noRetryCfg()
		o.Workers = 1
		o.AgentCmd = sh("sleep 0.3; echo ok > f.txt")
	})
	runWithTimeout(context.Background(), t, r)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusDone {
		t.Fatalf("kill obsoleto matou o worker: %+v %+v", got, got.Result)
	}
}

func TestKillRequestOlderThanRunIsIgnoredDuringRun(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "a")
	r := newRouter(t, root, func(o *Options) {
		o.Config = noRetryCfg()
		o.Workers = 1
		o.AgentCmd = sh("sleep 0.5; echo ok > f.txt")
	})
	ch := startRun(context.Background(), r)
	waitFor(t, "running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	if _, err := RequestKill(root, "w1", "velho", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	waitSummary(t, ch, 20*time.Second)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusDone {
		t.Fatalf("kill obsoleto foi honrado: %+v %+v", got, got.Result)
	}
}

// Finding 4: ticket canceled and returned to pending while the old slot still holds it.
func TestScheduleSkipsTicketStillHeldBySlot(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "a")
	r := newRouter(t, root, func(o *Options) { o.Workers = 2; o.AgentCmd = sh("sleep 30") })
	r.done = make(chan workerDone, 4)
	r.startedAt = time.Now()
	r.slots[0].ticket = tk.ID // old agent still shutting down
	t.Cleanup(func() {
		for _, s := range r.slots {
			if s.cancel != nil {
				s.cancel()
			}
		}
		time.Sleep(200 * time.Millisecond)
	})
	b, err := ticket.LoadBoard(st)
	if err != nil {
		t.Fatal(err)
	}
	r.schedule(b)
	if len(r.summary.Assignments) != 0 || r.slots[1].ticket != "" {
		t.Fatalf("ticket ainda preso ao slot foi reatribuído: %+v", r.summary.Assignments)
	}
	if got := get(t, st, tk.ID); got.Status != ticket.StatusPending {
		t.Fatalf("status mudou: %+v", got)
	}
}

// Finding 5: transiently invalid YAML does not bring down the router or running agents.
func TestTransientInvalidTicketKeepsWorkersRunning(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "a")
	flag := filepath.Join(t.TempDir(), "flag")
	r := newRouter(t, root, func(o *Options) {
		o.Workers = 1
		o.AgentCmd = sh(fmt.Sprintf("while [ ! -e %s ]; do sleep 0.02; done; echo ok > f.txt", flag))
	})
	r.maxBoardFail = 100000
	ch := startRun(context.Background(), r)
	waitFor(t, "running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	bad := filepath.Join(st.Dir(), "TKT-099.yaml")
	if err := os.WriteFile(bad, []byte("id: [oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "alerta board_unreadable", func() bool { return countKind(alertKinds(t, root), AlertBoardUnreadable) == 1 })
	time.Sleep(100 * time.Millisecond) // several ticks with an unreadable board
	if got := get(t, st, tk.ID); got.Status != ticket.StatusRunning {
		t.Fatalf("agente derrubado: %+v", got)
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flag, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := waitSummary(t, ch, 20*time.Second)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusDone || sum.Reason != ReasonCompleted {
		t.Fatalf("%+v reason=%s", got, sum.Reason)
	}
	as, _ := ReadAlerts(root, AlertFilter{Kind: AlertBoardUnreadable})
	if len(as) != 1 || as[0].Severity != SevWarn {
		t.Fatalf("alertas: %+v", as)
	}
}

func TestPersistentInvalidTicketGivesUpAsStalled(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "a")
	r := newRouter(t, root, func(o *Options) { o.Workers = 1; o.AgentCmd = sh("sleep 30") })
	r.maxBoardFail = 3
	type res struct {
		s   *Summary
		err error
	}
	ch := make(chan res, 1)
	go func() { s, err := r.Run(context.Background()); ch <- res{s, err} }()
	waitFor(t, "running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	if err := os.WriteFile(filepath.Join(st.Dir(), "TKT-099.yaml"), []byte("id: [oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case x := <-ch:
		if x.err == nil || x.s == nil || x.s.Reason != ReasonStalled {
			t.Fatalf("err=%v summary=%+v", x.err, x.s)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("router não desistiu")
	}
}

// Full exploit (REAL nested repo with a clean filter on a tracked, dirty
// file): the router's git must not recurse into the gitlink and run the filter.
// Calls commitWorkspace directly because the agent's git does not run under the
// test environment's Landlock.
func TestNestedRepoCleanFilterNeverRunsOnRouterGit(t *testing.T) {
	root := repoWithGitlink(t)
	ws := filepath.Join(t.TempDir(), "ws")
	runGit(t, root, "worktree", "add", "-q", "-b", "tb-workspace/x", ws)
	marker := filepath.Join(t.TempDir(), "marker")
	sub := filepath.Join(ws, "sub")
	runGit(t, sub, "init", "-q")
	runGit(t, sub, "config", "user.email", "a@a")
	runGit(t, sub, "config", "user.name", "a")
	runGit(t, sub, "config", "filter.x.clean", "touch "+marker+"; cat")
	if err := os.WriteFile(filepath.Join(sub, ".gitattributes"), []byte("* filter=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, sub, "add", "-A")
	runGit(t, sub, "commit", "-q", "-m", "x")
	_ = os.Remove(marker)
	// make the file "dirty" (mtime/content) so status has to re-filter
	if err := os.WriteFile(filepath.Join(sub, "f"), []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "g.txt"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, root, nil)
	if _, err := r.commitWorkspace(ws, &ticket.Ticket{ID: "TKT-001", Title: "x"}); err == nil {
		t.Fatal("esperava recusa por .git aninhado")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("filtro clean do repositório aninhado rodou no git do router")
	}
}
