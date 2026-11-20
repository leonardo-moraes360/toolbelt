//go:build unix

package router

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/busrun"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

type busFixture struct {
	root          string
	store         *bus.Store
	tickets       *ticket.Store
	owner, worker bus.Member
}

func newBusFixture(t *testing.T) *busFixture {
	t.Helper()
	root := newRepo(t)
	s, err := bus.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	_, oi, err := s.CreateChannel("ch", "boss", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Join("ch", oi.Code, &bus.AgentCard{Name: "boss"}, "")
	if err != nil {
		t.Fatal(err)
	}
	ri, _ := s.CreateInvite("ch", bus.RoleRouter, "router", 0)
	r, err := s.Join("ch", ri.Code, &bus.AgentCard{Name: "router"}, bus.RoleRouter)
	if err != nil {
		t.Fatal(err)
	}
	if err := bus.SaveSession(root, &r); err != nil {
		t.Fatal(err)
	}
	wi, _ := s.CreateInvite("ch", bus.RoleWorker, "", 0)
	w, err := s.Join("ch", wi.Code, &bus.AgentCard{Name: "w"}, bus.RoleWorker)
	if err != nil {
		t.Fatal(err)
	}
	return &busFixture{root: root, store: s, tickets: ticket.Open(root), owner: o.Member, worker: w.Member}
}

func (f *busFixture) router(t *testing.T, mut func(*Options)) *Router {
	t.Helper()
	cfg := testCfg()
	cfg.MaxWorkers = 0
	cfg.Bus.Channel, cfg.Bus.As, cfg.Bus.ClaimTimeout = "ch", "router", "3s"
	return newRouter(t, f.root, func(o *Options) {
		o.Config, o.AgentCmd, o.AllowUnsandboxed = cfg, nil, false
		if mut != nil {
			mut(o)
		}
	})
}

// fakeWorker plays a B4a harness: claims confined, runs work in the
// workspace and completes with the verdict it returns. handled counts tasks.
type fakeWorker struct {
	f       *busFixture
	m       bus.Member
	work    func(task *busrun.RouterTask, n int) (verdict string, complete bool)
	claim   func(m bus.Member, thread string) (bus.Claim, error)
	mu      sync.Mutex
	handled int
	threads []string
}

func (w *fakeWorker) run(ctx context.Context) {
	seen := map[string]bool{}
	for ctx.Err() == nil {
		msgs, _ := w.f.store.Pending(w.m, 0, 0, []bus.Type{bus.TypeTaskAssign}, true, 100)
		for i := range msgs {
			if seen[msgs[i].ID] {
				continue
			}
			seen[msgs[i].ID] = true
			var task busrun.RouterTask
			if json.Unmarshal(msgs[i].Payload, &task) != nil {
				continue
			}
			w.mu.Lock()
			w.handled++
			n := w.handled
			w.threads = append(w.threads, task.Thread)
			w.mu.Unlock()
			claim := w.claim
			if claim == nil {
				claim = w.f.store.ClaimConfined
			}
			c, err := claim(w.m, task.Thread)
			if err != nil {
				continue
			}
			verdict, complete := w.work(&task, n)
			if complete {
				b, _ := json.Marshal(busrun.Verdict{Verdict: verdict, Summary: "\x1b[31mzebra\x1b[0m token=ghp_" + strings.Repeat("x", 36)})
				_, _ = w.f.store.Complete(w.m, task.Thread, c.Epoch, b)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (w *fakeWorker) start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); w.run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
}

func edit(task *busrun.RouterTask) {
	_ = os.WriteFile(filepath.Join(task.Workspace, "bus-"+task.Ticket+".txt"), []byte("work\n"), 0o644)
}

// AC15: bus only; the worker edits and passes; the router commits.
func TestBusHappyPath(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "bus ticket")
	w := &fakeWorker{f: f, m: f.worker, work: func(task *busrun.RouterTask, _ int) (string, bool) {
		edit(task)
		return "PASS", true
	}}
	w.start(t)
	sum := runWithTimeout(context.Background(), t, f.router(t, nil))
	if len(sum.Done) != 1 {
		t.Fatalf("summary %+v", sum)
	}
	requireDoneWithCommit(t, f.tickets, tk.ID)
	got := get(t, f.tickets, tk.ID)
	if got.Result.BusAgent != "agent:w" || got.Result.BusEpoch != 1 || got.BusThread != "" || got.Worker != "b1" {
		t.Fatalf("bus fields: %+v / %+v", got, got.Result)
	}
	parents := runGit(t, f.root, "rev-list", "--parents", "-n", "1", got.Result.HeadCommit)
	if f := strings.Fields(parents); len(f) != 2 || f[1] != got.Result.BaseCommit {
		t.Fatalf("parents: %s", parents)
	}
	if !strings.HasPrefix(w.threads[0], tk.ID+"/") || !strings.HasSuffix(w.threads[0], ".1") {
		t.Fatalf("thread %q", w.threads[0])
	}
	// AC31: the summary only reaches the router log, stripped and redacted.
	logb, _ := os.ReadFile(filepath.Join(f.root, got.Result.Log))
	y, _ := os.ReadFile(f.tickets.Path(tk.ID))
	if strings.Contains(string(logb), "\x1b") || strings.Contains(string(logb), "ghp_x") || !strings.Contains(string(logb), "zebra") || strings.Contains(string(y), "zebra") {
		t.Fatalf("summary handling:\nlog %q\nticket %s", logb, y)
	}
}

// AC17: FAIL retries on a new thread with a reset worktree.
func TestBusFailRetries(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "retry me")
	w := &fakeWorker{f: f, m: f.worker, work: func(task *busrun.RouterTask, n int) (string, bool) {
		if _, err := os.Stat(filepath.Join(task.Workspace, "bus-"+task.Ticket+".txt")); err == nil {
			return "FAIL", true // the reset did not happen
		}
		edit(task)
		if n == 1 {
			return "FAIL", true
		}
		return "PASS", true
	}}
	w.start(t)
	runWithTimeout(context.Background(), t, f.router(t, nil))
	requireDoneWithCommit(t, f.tickets, tk.ID)
	if len(w.threads) != 2 || w.threads[0] == w.threads[1] || !strings.HasSuffix(w.threads[1], ".2") {
		t.Fatalf("threads %v", w.threads)
	}
	if countKind(alertKinds(t, f.root), AlertBusVerdictFail) != 1 {
		t.Fatalf("alerts %v", alertKinds(t, f.root))
	}
}

// AC18: an agent that commits by itself escalates with no retry.
func TestBusAgentCommitEscalates(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "sneaky")
	w := &fakeWorker{f: f, m: f.worker, work: func(task *busrun.RouterTask, _ int) (string, bool) {
		edit(task)
		runGit(t, task.Workspace, "add", "-A")
		runGit(t, task.Workspace, "commit", "-q", "-m", "agent commit")
		return "PASS", true
	}}
	w.start(t)
	sum := runWithTimeout(context.Background(), t, f.router(t, nil))
	got := get(t, f.tickets, tk.ID)
	if got.Status != ticket.StatusFailed || len(sum.Retried) != 0 || !strings.Contains(got.Result.Error, "integrity") {
		t.Fatalf("ticket %+v result %+v", got, got.Result)
	}
}

// AC19: a claim without the marker is revoked and the task reassigned.
func TestBusClaimRejected(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "picky")
	w := &fakeWorker{f: f, m: f.worker, work: func(task *busrun.RouterTask, _ int) (string, bool) {
		edit(task)
		return "PASS", true
	}}
	w.claim = func(m bus.Member, thread string) (bus.Claim, error) {
		if len(w.threads) == 1 {
			return f.store.ClaimThread(m, thread) // unconfined: no marker
		}
		return f.store.ClaimConfined(m, thread)
	}
	w.start(t)
	runWithTimeout(context.Background(), t, f.router(t, nil))
	requireDoneWithCommit(t, f.tickets, tk.ID)
	if countKind(alertKinds(t, f.root), AlertBusClaimRejected) != 1 || len(w.threads) != 2 {
		t.Fatalf("alerts %v threads %v", alertKinds(t, f.root), w.threads)
	}
	if _, err := f.store.ClaimConfined(f.worker, w.threads[0]); err == nil {
		t.Fatal("the rejected thread was not revoked")
	}
}

// AC19 (unclaimed): with nobody claiming, the attempt fails as unclaimed
// and the thread is closed for good.
func TestBusUnclaimed(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "lonely")
	r := f.router(t, func(o *Options) {
		o.Config.Bus.ClaimTimeout = "150ms"
		o.Config.Retry.MaxAttempts = 1
	})
	runWithTimeout(context.Background(), t, r)
	got := get(t, f.tickets, tk.ID)
	if got.Status != ticket.StatusFailed || !strings.Contains(got.Result.Error, "unclaimed") {
		t.Fatalf("ticket %+v", got.Result)
	}
	msgs, _ := f.store.Log("ch", "", 50)
	for i := 0; i < len(msgs); {
		if msgs[i].Type != bus.TypeTaskAssign {
			msgs = append(msgs[:i], msgs[i+1:]...)
			continue
		}
		i++
	}
	if len(msgs) == 0 {
		t.Fatal("no assign was sent")
	}
	if _, err := f.store.ClaimConfined(f.worker, msgs[0].Thread); err == nil {
		t.Fatal("a late claim on a withdrawn thread succeeded")
	}
}

// AC20: a revoke by someone else (lease lost) fails the attempt.
func TestBusLeaseLost(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "lost")
	w := &fakeWorker{f: f, m: f.worker, work: func(task *busrun.RouterTask, _ int) (string, bool) {
		_, _ = f.store.Revoke(f.owner, task.Thread, "canceled")
		return "", false
	}}
	w.start(t)
	r := f.router(t, func(o *Options) { o.Config.Retry.MaxAttempts = 1 })
	runWithTimeout(context.Background(), t, r)
	got := get(t, f.tickets, tk.ID)
	if got.Status != ticket.StatusFailed || !strings.Contains(got.Result.Error, "lease") ||
		countKind(alertKinds(t, f.root), AlertBusLeaseLost) != 1 {
		t.Fatalf("ticket %+v alerts %v", got.Result, alertKinds(t, f.root))
	}
}

// AC21: a claimed attempt past its timeout is revoked (worker_stuck).
func TestBusTimeout(t *testing.T) {
	f := newBusFixture(t)
	tk := mkTimeout(t, f.tickets, "slow", "300ms")
	w := &fakeWorker{f: f, m: f.worker, work: func(*busrun.RouterTask, int) (string, bool) { return "", false }}
	w.start(t)
	r := f.router(t, func(o *Options) { o.Config.Retry.MaxAttempts = 1 })
	runWithTimeout(context.Background(), t, r)
	got := get(t, f.tickets, tk.ID)
	kinds := alertKinds(t, f.root)
	if got.Status != ticket.StatusFailed || countKind(kinds, AlertWorkerStuck) != 1 {
		t.Fatalf("ticket %+v alerts %v", got.Result, kinds)
	}
	c, err := f.store.Claim("ch", w.threads[0])
	if err != nil || c.RevokedAt == nil {
		t.Fatalf("thread not revoked: %+v %v", c, err)
	}
}

// AC29: bus only needs no agent_cmd nor Landlock; --check refuses a second
// router-role member.
func TestBusCheck(t *testing.T) {
	f := newBusFixture(t)
	r := f.router(t, nil)
	if err := r.CheckBus(); err != nil {
		t.Fatalf("check: %v", err)
	}
	r.closeBus()
	inv, _ := f.store.CreateInvite("ch", bus.RoleRouter, "router2", 0)
	if _, err := f.store.Join("ch", inv.Code, &bus.AgentCard{Name: "router2"}, bus.RoleRouter); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckBus(); err == nil || !strings.Contains(err.Error(), "router-role") {
		t.Fatalf("second router accepted: %v", err)
	}
	cfg := testCfg()
	cfg.MaxWorkers = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("max_workers 0 without bus.channel accepted")
	}
}

// AC33: the dry run shows bus slots.
func TestBusDryRun(t *testing.T) {
	f := newBusFixture(t)
	mk(t, f.tickets, "planned")
	plan, err := f.router(t, nil).Plan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.BusSlots != 1 || len(plan.Rounds) == 0 || plan.Rounds[0].Assignments[0].Worker != "b1" {
		t.Fatalf("plan %+v", plan)
	}
}

// AC24: shutdown with a busy bus slot ends without hanging, revokes the
// thread and requeues without spending the attempt.
func TestBusShutdown(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "interrupted")
	claimed := make(chan string, 1)
	w := &fakeWorker{f: f, m: f.worker, work: func(task *busrun.RouterTask, _ int) (string, bool) {
		claimed <- task.Thread
		return "", false
	}}
	w.start(t)
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *Summary, 1)
	go func() { s, _ := f.router(t, nil).Run(ctx); ch <- s }()
	thread := <-claimed
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown hung")
	}
	got := get(t, f.tickets, tk.ID)
	c, _ := f.store.Claim("ch", thread)
	if got.Status != ticket.StatusPending || got.Attempts != 0 || c.RevokedAt == nil {
		t.Fatalf("ticket %+v claim %+v", got, c)
	}
}

// AC23: a restart revokes the open thread a previous run left behind.
func TestBusRecoverRevokes(t *testing.T) {
	f := newBusFixture(t)
	tk := mk(t, f.tickets, "orphan")
	thread := tk.ID + "/old000.1"
	if _, err := f.store.ClaimConfined(f.worker, thread); err != nil {
		t.Fatal(err)
	}
	if _, err := f.tickets.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status, x.Worker, x.BusThread, x.Attempts = ticket.StatusRunning, "b1", thread, 1
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r := f.router(t, func(o *Options) { o.Once = true })
	runWithTimeout(context.Background(), t, r)
	c, _ := f.store.Claim("ch", thread)
	if c.RevokedAt == nil {
		t.Fatalf("old thread not revoked: %+v", c)
	}
}
