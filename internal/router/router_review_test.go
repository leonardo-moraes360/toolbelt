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

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func startRun(ctx context.Context, r *Router) chan *Summary {
	ch := make(chan *Summary, 1)
	go func() { s, _ := r.Run(ctx); ch <- s }()
	return ch
}

func waitSummary(t *testing.T, ch chan *Summary, d time.Duration) *Summary {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(d):
		t.Fatal("router não encerrou a tempo")
		return nil
	}
}

// Finding 1: the agent swaps <ws>/.git for a repo with a malicious core.fsmonitor.
func TestWorkspaceGitReplacedByAgentIsNeverExecuted(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "malicioso")
	cfg := testCfg()
	cfg.Retry.MaxAttempts = 3
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "marker")
	hook := filepath.Join(tmp, "hook.sh")
	script := fmt.Sprintf(`printf '#!/bin/sh\ntouch %s\n' > %s; chmod +x %s; rm -rf .git; git init -q .; git config core.fsmonitor %s; echo x > f.txt`, marker, hook, hook, hook)
	r := newRouter(t, root, func(o *Options) { o.Config = cfg; o.AgentCmd = sh(script) })
	runWithTimeout(context.Background(), t, r)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("hook fsmonitor do agente foi executado pelo router")
	}
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusFailed || got.Attempts != 1 {
		t.Fatalf("esperava failed sem retry: %+v", got)
	}
	if got.Result == nil || !strings.Contains(got.Result.Error, "integridade") {
		t.Fatalf("erro sem explicação clara: %+v", got.Result)
	}
	if got.Result.HeadCommit != "" {
		t.Fatalf("não deveria ter commitado: %+v", got.Result)
	}
}

func TestWorkspaceGitSymlinkIsRefused(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "symlink")
	other := t.TempDir()
	script := fmt.Sprintf(`rm -rf .git; ln -s %s .git; echo x > f.txt`, other)
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh(script) })
	runWithTimeout(context.Background(), t, r)
	if get(t, st, tk.ID).Status != ticket.StatusFailed {
		t.Fatalf("esperava failed: %+v", get(t, st, tk.ID))
	}
}

// Finding 2: cancel during execution + shutdown does not resurrect the ticket.
func TestCanceledWhileRunningStaysCanceledOnShutdown(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "longo")
	ctx, cancel := context.WithCancel(context.Background())
	// huge tick: the watchdog does not act; only shutdown finds the canceled ticket
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("sleep 30"); o.Tick = time.Hour })
	ch := startRun(ctx, r)
	waitFor(t, "running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	if _, err := st.Cancel(tk.ID); err != nil {
		t.Fatal(err)
	}
	cancel()
	sum := waitSummary(t, ch, 20*time.Second)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusCanceled {
		t.Fatalf("ressuscitou: %+v", got)
	}
	if len(sum.Requeued) != 0 {
		t.Fatalf("summary: %+v", sum)
	}
}

func TestCancelWhileRunningKillsAgentPromptly(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "longo")
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("sleep 30") })
	ch := startRun(context.Background(), r)
	waitFor(t, "running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	if _, err := st.Cancel(tk.ID); err != nil {
		t.Fatal(err)
	}
	sum := waitSummary(t, ch, 15*time.Second) // sleep 30 only ends early if the agent is killed
	if got := get(t, st, tk.ID); got.Status != ticket.StatusCanceled {
		t.Fatalf("esperava canceled: %+v", got)
	}
	if countKind(alertKinds(t, root), AlertTicketCanceled) != 1 || len(sum.Failed)+len(sum.Retried)+len(sum.Done) != 0 {
		t.Fatalf("alertas=%v summary=%+v", alertKinds(t, root), sum)
	}
}

// Finding 4: a late reason does not drop the result of an agent that already exited 0.
func TestLateReasonDoesNotDiscardFinishedWork(t *testing.T) {
	for _, reason := range []string{reasonStuck, reasonKilled, reasonShutdown} {
		t.Run(reason, func(t *testing.T) {
			root := newRepo(t)
			st := ticket.Open(root)
			tk := mk(t, st, "x")
			clock := &fakeClock{}
			r := newRouter(t, root, func(o *Options) { o.Now = clock.Now })
			if _, err := st.Update(tk.ID, func(x *ticket.Ticket) error {
				x.Status = ticket.StatusRunning
				x.Worker = "w1"
				x.Attempts = 1
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			s := r.slots[0]
			*s = slot{id: "w1", ticket: tk.ID, started: clock.Now(), timeout: time.Minute, attempts: 1, reason: reason, cancel: func() {}}
			r.complete(&workerDone{slot: s, id: tk.ID, exit: 0, head: "abc123"})
			if got := get(t, st, tk.ID); got.Status != ticket.StatusDone || got.Result.HeadCommit != "abc123" {
				t.Fatalf("trabalho bom descartado: %+v", got)
			}
		})
	}
	// with a real cancellation, the reason still applies
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "x")
	r := newRouter(t, root, nil)
	_, _ = st.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusRunning
		x.Worker = "w1"
		x.Attempts = 1
		return nil
	})
	s := r.slots[0]
	*s = slot{id: "w1", ticket: tk.ID, timeout: time.Minute, attempts: 1, reason: reasonStuck, cancel: func() {}}
	r.complete(&workerDone{slot: s, id: tk.ID, exit: -1, canceled: true})
	if got := get(t, st, tk.ID); got.Status != ticket.StatusPending || !strings.Contains(got.Result.Error, "timeout") {
		t.Fatalf("stuck real deveria virar retry: %+v", got)
	}
}

// Finding 5: flock singleton.
func TestSecondRunFailsWhileFirstActive(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "longo")
	r1 := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("sleep 30") })
	ctx, cancel := context.WithCancel(context.Background())
	ch := startRun(ctx, r1)
	waitFor(t, "running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	r2 := newRouter(t, root, nil)
	_, err := r2.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "outro tb router run ativo") {
		t.Fatalf("esperava recusa: %v", err)
	}
	cancel()
	waitSummary(t, ch, 20*time.Second)
	// lock released on exit
	r3 := newRouter(t, root, nil)
	if _, err := r3.Run(context.Background()); err != nil {
		t.Fatalf("lock deveria ter sido liberado: %v", err)
	}
}

func TestStalePidInStateDoesNotBlock(t *testing.T) {
	root := newRepo(t)
	if _, err := writeState(root, &State{PID: 1, Status: StateRunning, StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, root, nil)
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("pid antigo em state.json não deve bloquear: %v", err)
	}
}

// Finding 6.
func TestMultiDepBaseAlert(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")
	b := mk(t, st, "b")
	c := mk(t, st, "c", a.ID, b.ID)
	r := newRouter(t, root, nil)
	runWithTimeout(context.Background(), t, r)
	if get(t, st, c.ID).Status != ticket.StatusDone {
		t.Fatal("c deveria concluir")
	}
	as, _ := ReadAlerts(root, AlertFilter{Kind: AlertMultiDepBase})
	if len(as) != 1 || as[0].Ticket != c.ID || as[0].Severity != SevWarn || as[0].Action != ActionNone ||
		!strings.Contains(as[0].Message, a.ID) || !strings.Contains(as[0].Message, b.ID) || !strings.Contains(as[0].Message, "tb merge") {
		t.Fatalf("alertas: %+v", as)
	}
}

func TestMultiDepBaseNoAlertWhenMerged(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")
	b := mk(t, st, "b")
	for _, id := range []string{a.ID, b.ID} {
		if _, err := st.Transition(id, ticket.StatusRunning); err != nil {
			t.Fatal(err)
		}
		if _, err := st.Update(id, func(x *ticket.Ticket) error {
			now := time.Now()
			x.Status = ticket.StatusDone
			x.MergedAt = &now
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	mk(t, st, "c", a.ID, b.ID)
	r := newRouter(t, root, nil)
	runWithTimeout(context.Background(), t, r)
	if n := countKind(alertKinds(t, root), AlertMultiDepBase); n != 0 {
		t.Fatalf("não deveria alertar com deps mergeadas: %d", n)
	}
}

// Finding 13: 0600 log.
func TestAgentLogIs0600(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "log")
	r := newRouter(t, root, nil)
	runWithTimeout(context.Background(), t, r)
	got := get(t, st, tk.ID)
	fi, err := os.Stat(filepath.Join(root, got.Result.Log))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("modo do log = %v", fi.Mode().Perm())
	}
}
