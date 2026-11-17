package router

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo cria um repositório temporário com branch explícita (independe de init.defaultBranch).
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("oi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".toolbelt/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func testCfg() config.RouterConfig {
	c := config.DefaultRouterConfig()
	c.Timeouts.PerTicket = "1h"
	c.Timeouts.Global = "1000h"
	return c
}

func sh(script string) []string { return []string{"sh", "-c", script} }

func newRouter(t *testing.T, root string, mut func(*Options)) *Router {
	t.Helper()
	o := Options{
		Root: root, Config: testCfg(), AgentCmd: sh("echo ok > out-$TB_TICKET_ID.txt"),
		AllowUnsandboxed: true, Tick: 15 * time.Millisecond,
	}
	if mut != nil {
		mut(&o)
	}
	r, err := New(&o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mk(t *testing.T, s *ticket.Store, title string, deps ...string) ticket.Ticket {
	t.Helper()
	tk, err := s.Create(&ticket.Ticket{Title: title, Dependencies: deps})
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func mkTimeout(t *testing.T, s *ticket.Store, title, timeout string) ticket.Ticket {
	t.Helper()
	tk, err := s.Create(&ticket.Ticket{Title: title, Acceptance: ticket.Acceptance{Timeout: timeout}})
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func get(t *testing.T, s *ticket.Store, id string) ticket.Ticket {
	t.Helper()
	tk, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func runWithTimeout(ctx context.Context, t *testing.T, r *Router) *Summary {
	t.Helper()
	type res struct {
		s   *Summary
		err error
	}
	ch := make(chan res, 1)
	go func() {
		s, err := r.Run(ctx)
		ch <- res{s, err}
	}()
	select {
	case x := <-ch:
		if x.err != nil {
			t.Fatalf("Run: %v", x.err)
		}
		return x.s
	case <-time.After(60 * time.Second):
		t.Fatal("router não terminou em 60s")
		return nil
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout esperando: %s", what)
}

func alertKinds(t *testing.T, root string) []string {
	t.Helper()
	as, err := ReadAlerts(root, AlertFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var k []string
	for _, a := range as {
		k = append(k, a.Kind)
	}
	return k
}

func countKind(kinds []string, kind string) int {
	n := 0
	for _, k := range kinds {
		if k == kind {
			n++
		}
	}
	return n
}

func index(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

func startedOrder(s *Summary) []string {
	var out []string
	for _, a := range s.Assignments {
		out = append(out, a.Ticket)
	}
	return out
}

func assertDoneWithCommit(t *testing.T, st *ticket.Store, ids ...string) {
	t.Helper()
	for _, id := range ids {
		tk := get(t, st, id)
		if tk.Status != ticket.StatusDone || tk.Result == nil || tk.Result.HeadCommit == "" || tk.Result.Branch != "tb-workspace/"+id {
			t.Fatalf("%s: %+v", id, tk)
		}
		if tk.Result.HeadCommit == tk.Result.BaseCommit {
			t.Fatalf("%s: agente escreveu arquivo mas não houve commit", id)
		}
	}
}

func TestDependencyOrdering(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	b := mk(t, st, "b")
	c := mk(t, st, "c", b.ID)
	a := mk(t, st, "a", c.ID)
	free := mk(t, st, "free")
	r := newRouter(t, root, func(o *Options) { o.Workers = 1 })
	sum := runWithTimeout(context.Background(), t, r)
	if sum.Reason != ReasonCompleted {
		t.Fatalf("reason = %s", sum.Reason)
	}
	order := startedOrder(sum)
	if len(order) != 4 || index(order, b.ID) > index(order, c.ID) || index(order, c.ID) > index(order, a.ID) {
		t.Fatalf("ordem inválida: %v", order)
	}
	assertDoneWithCommit(t, st, a.ID, b.ID, c.ID, free.ID)
	if out := runGit(t, root, "show", "--stat", "--format=%s", get(t, st, b.ID).Result.HeadCommit); !strings.Contains(out, "out-"+b.ID+".txt") || !strings.Contains(out, "tb: "+b.ID) {
		t.Fatalf("commit do worker: %s", out)
	}
	state, err := LoadStatus(root)
	if err != nil || state.Status != StateStopped || state.Stale {
		t.Fatalf("state final: %+v %v", state, err)
	}
}

func TestDirectDeliverySameWorkerAndBaseChaining(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a") // TKT-001
	x := mkTimeout(t, st, "x", "1h")
	bTk := mk(t, st, "b", a.ID) // depende de a
	_ = x
	script := `if [ "$TB_TICKET_ID" = "TKT-001" ]; then echo A > a.txt; sleep 0.1; fi
if [ "$TB_TICKET_ID" = "TKT-002" ]; then sleep 0.4; fi
if [ "$TB_TICKET_ID" = "TKT-003" ]; then cat a.txt > b.txt; fi`
	r := newRouter(t, root, func(o *Options) { o.Workers = 2; o.AgentCmd = sh(script) })
	sum := runWithTimeout(context.Background(), t, r)
	if len(sum.Done) != 3 {
		t.Fatalf("done = %v failed=%v", sum.Done, sum.Failed)
	}
	var asgA, asgB Assignment
	for _, as := range sum.Assignments {
		switch as.Ticket {
		case a.ID:
			asgA = as
		case bTk.ID:
			asgB = as
		}
	}
	if asgB.Via != ViaDirect || asgB.Worker != asgA.Worker {
		t.Fatalf("entrega direta esperada no mesmo worker: A=%+v B=%+v", asgA, asgB)
	}
	ta, tb := get(t, st, a.ID), get(t, st, bTk.ID)
	if tb.Result.BaseCommit != ta.Result.HeadCommit {
		t.Fatalf("base de B (%s) deveria ser HEAD de A (%s)", tb.Result.BaseCommit, ta.Result.HeadCommit)
	}
	if b, err := os.ReadFile(filepath.Join(tb.Result.Workspace, "b.txt")); err != nil || strings.TrimSpace(string(b)) != "A" {
		t.Fatalf("B deveria enxergar a.txt de A: %q %v", b, err)
	}
	if tb.Subscription != nil {
		t.Fatalf("assinatura deveria ser limpa ao iniciar: %+v", tb.Subscription)
	}
}

type fakeClock struct {
	mu  sync.Mutex
	off time.Duration
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Add(c.off)
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.off += d
	c.mu.Unlock()
}

func TestSubscriptionTTLFallsBackToBoard(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")         // TKT-001 quick -> w1
	x := mk(t, st, "x")         // TKT-002 médio -> w2
	b1 := mk(t, st, "b1", a.ID) // TKT-003 longo
	b2 := mk(t, st, "b2", a.ID) // TKT-004 reservado para w1 enquanto vivo
	clock := &fakeClock{}
	script := `case "$TB_TICKET_ID" in
TKT-002) sleep 0.4;;
TKT-003) sleep 2;;
esac
echo ok > f-$TB_TICKET_ID.txt`
	r := newRouter(t, root, func(o *Options) {
		o.Workers = 2
		o.Now = clock.Now
		o.AgentCmd = sh(script)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan *Summary, 1)
	go func() {
		s, err := r.Run(ctx)
		if err != nil {
			t.Error(err)
		}
		ch <- s
	}()

	waitFor(t, "b1 em execução e x concluído", func() bool {
		return get(t, st, b1.ID).Status == ticket.StatusRunning && get(t, st, x.ID).Status == ticket.StatusDone
	})
	// w2 está ocioso, mas b2 está reservado para w1 (assinatura viva)
	time.Sleep(150 * time.Millisecond)
	if g := get(t, st, b2.ID); g.Status != ticket.StatusPending || g.Subscription == nil || g.Subscription.Worker != "w1" {
		t.Fatalf("b2 deveria seguir pending e reservado para w1: %+v", g)
	}
	clock.Advance(31 * time.Minute) // expira o TTL (30m)
	waitFor(t, "b2 pego pelo board", func() bool { return get(t, st, b2.ID).Status != ticket.StatusPending })
	sum := <-ch
	var via, worker string
	for _, as := range sum.Assignments {
		if as.Ticket == b2.ID {
			via, worker = as.Via, as.Worker
		}
	}
	if via != ViaBoard || worker != "w2" {
		t.Fatalf("b2 deveria cair no board e ir para w2, veio via=%s worker=%s", via, worker)
	}
}

func TestSubscribeRespectsMaxPerWorker(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")
	mk(t, st, "b1", a.ID)
	mk(t, st, "b2", a.ID)
	mk(t, st, "b3", a.ID)
	cfg := testCfg()
	cfg.DirectDelivery.MaxSubscriptionsPerWorker = 2
	r := newRouter(t, root, func(o *Options) { o.Config = cfg })
	r.subscribeDependents(a.ID, "w1")
	n := 0
	for _, id := range []string{"TKT-002", "TKT-003", "TKT-004"} {
		if g := get(t, st, id); g.Subscription != nil && g.Subscription.Worker == "w1" && g.Subscription.DirectDelivery {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("assinaturas = %d, esperado 2 (teto)", n)
	}
	// desabilitar a entrega direta não reserva nada
	cfg.DirectDelivery.Enabled = false
	r2 := newRouter(t, root, func(o *Options) { o.Config = cfg })
	r2.clearSubscriptions("w1")
	r2.subscribeDependents(a.ID, "w1")
	for _, id := range []string{"TKT-002", "TKT-003", "TKT-004"} {
		if get(t, st, id).Subscription != nil {
			t.Fatalf("direct_delivery.enabled=false não deveria reservar %s", id)
		}
	}
}

func TestTimeoutStuckRetryThenFailed(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mkTimeout(t, st, "lento", "250ms")
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("sleep 30") })
	start := time.Now()
	sum := runWithTimeout(context.Background(), t, r)
	if time.Since(start) > 20*time.Second {
		t.Fatal("watchdog não matou o agente (sleep 30 sobreviveu)")
	}
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusFailed || got.Attempts != 2 {
		t.Fatalf("esperava failed após 2 tentativas: %+v", got)
	}
	if got.Result == nil || !strings.Contains(got.Result.Error, "worker_stuck") {
		t.Fatalf("result: %+v", got.Result)
	}
	if len(sum.Retried) != 1 || len(sum.Failed) != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	as, _ := ReadAlerts(root, AlertFilter{Kind: AlertWorkerStuck})
	if len(as) != 2 || as[0].Action != ActionRetry || as[1].Action != ActionEscalate || as[0].Severity != SevCritical || as[0].Ticket != tk.ID || as[0].Worker != "w1" {
		t.Fatalf("alertas stuck: %+v", as)
	}
	if countKind(alertKinds(t, root), AlertWorkerSlow) < 1 {
		t.Fatalf("esperava worker_slow antes do stuck: %v", alertKinds(t, root))
	}
}

func TestStuckWithFakeClock(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mkTimeout(t, st, "x", "10m")
	clock := &fakeClock{}
	cfg := testCfg()
	cfg.Retry.MaxAttempts = 1
	r := newRouter(t, root, func(o *Options) { o.Config = cfg; o.Now = clock.Now; o.AgentCmd = sh("sleep 30") })
	ch := make(chan *Summary, 1)
	go func() { s, _ := r.Run(context.Background()); ch <- s }()
	waitFor(t, "ticket running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	clock.Advance(8*time.Minute + 30*time.Second) // 85% => slow
	waitFor(t, "worker_slow", func() bool { return countKind(alertKinds(t, root), AlertWorkerSlow) == 1 })
	clock.Advance(2 * time.Minute) // > 10m => stuck
	<-ch
	if get(t, st, tk.ID).Status != ticket.StatusFailed {
		t.Fatal("esperava failed")
	}
	k := alertKinds(t, root)
	if countKind(k, AlertWorkerSlow) != 1 || countKind(k, AlertWorkerStuck) != 1 {
		t.Fatalf("alertas: %v", k)
	}
}

func TestCrashAlertAndRetry(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "quebra")
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("echo falhou; exit 3") })
	sum := runWithTimeout(context.Background(), t, r)
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusFailed || got.Attempts != 2 || got.Result.ExitCode != 3 {
		t.Fatalf("ticket: %+v result=%+v", got, got.Result)
	}
	as, _ := ReadAlerts(root, AlertFilter{Kind: AlertWorkerCrash})
	if len(as) != 2 || as[0].Action != ActionRetry || as[1].Action != ActionEscalate {
		t.Fatalf("alertas crash: %+v", as)
	}
	if len(sum.Failed) != 1 || len(sum.Assignments) != 2 {
		t.Fatalf("summary: %+v", sum)
	}
	logData, err := os.ReadFile(filepath.Join(root, got.Result.Log))
	if err != nil || strings.Count(string(logData), "falhou") != 2 {
		t.Fatalf("log: %q %v", logData, err)
	}
}

func TestCrashClearsWorkerSubscriptions(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")
	b := mk(t, st, "b", a.ID)
	if _, err := st.Update(b.ID, func(x *ticket.Ticket) error {
		x.Subscription = &ticket.Subscription{Worker: "w1", DirectDelivery: true, ExpiresAt: time.Now().Add(time.Hour)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, root, nil)
	r.clearSubscriptions("w1")
	if get(t, st, b.ID).Subscription != nil {
		t.Fatal("assinatura de w1 deveria ter sido limpa")
	}
}

func TestQueueGrowingAlertOnce(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	for i := 0; i < 5; i++ {
		mk(t, st, "t")
	}
	r := newRouter(t, root, func(o *Options) { o.Workers = 1; o.AgentCmd = sh("sleep 0.15") })
	runWithTimeout(context.Background(), t, r)
	if n := countKind(alertKinds(t, root), AlertQueueGrowing); n != 1 {
		t.Fatalf("queue_growing = %d, esperado 1 (uma vez por transição)", n)
	}
}

func TestKillControl(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "longo")
	cfg := testCfg()
	cfg.Retry.MaxAttempts = 1
	r := newRouter(t, root, func(o *Options) { o.Config = cfg; o.Workers = 1; o.AgentCmd = sh("sleep 30") })
	ch := make(chan *Summary, 1)
	go func() { s, _ := r.Run(context.Background()); ch <- s }()
	waitFor(t, "w1 working", func() bool {
		s, err := ReadState(root)
		return err == nil && len(s.Workers) == 1 && s.Workers[0].Status == "working" && s.Workers[0].Ticket == tk.ID
	})
	if _, err := RequestKill(root, "w9", "x", time.Now()); err != nil { // existe formato válido, worker inexistente é ignorado
		t.Fatal(err)
	}
	if _, err := RequestKill(root, "../etc", "x", time.Now()); err == nil {
		t.Fatal("worker inválido deveria falhar")
	}
	if _, err := RequestKill(root, "w1", "teste de kill", time.Now()); err != nil {
		t.Fatal(err)
	}
	sum := <-ch
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusFailed || !strings.Contains(got.Result.Error, "teste de kill") {
		t.Fatalf("ticket: %+v result=%+v", got, got.Result)
	}
	as, _ := ReadAlerts(root, AlertFilter{Kind: AlertWorkerKilled})
	if len(as) != 1 || as[0].Worker != "w1" || as[0].Action != ActionEscalate {
		t.Fatalf("alertas killed: %+v", as)
	}
	if len(sum.Failed) != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	if ents, _ := os.ReadDir(ControlDir(root)); len(ents) != 0 {
		t.Fatalf("pedidos de kill deveriam ser consumidos: %v", ents)
	}
}

func TestPauseThenResume(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")
	mk(t, st, "b")
	if _, err := ticket.Pause(root); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, root, nil)
	ch := make(chan *Summary, 1)
	go func() { s, _ := r.Run(context.Background()); ch <- s }()
	time.Sleep(200 * time.Millisecond)
	if get(t, st, a.ID).Status != ticket.StatusPending {
		t.Fatal("pausado: nada deveria ser atribuído")
	}
	select {
	case <-ch:
		t.Fatal("router pausado com trabalho pendente não deveria sair")
	default:
	}
	if s, err := ReadState(root); err != nil || !s.Paused {
		t.Fatalf("state deveria refletir pause: %+v %v", s, err)
	}
	if _, err := ticket.Resume(root); err != nil {
		t.Fatal(err)
	}
	sum := <-ch
	if len(sum.Done) != 2 || sum.Reason != ReasonCompleted {
		t.Fatalf("summary: %+v", sum)
	}
}

func TestDrainFinishesRunningOnly(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	for i := 0; i < 3; i++ {
		mk(t, st, "t")
	}
	r := newRouter(t, root, func(o *Options) { o.Workers = 1; o.AgentCmd = sh("sleep 0.5") })
	ch := make(chan *Summary, 1)
	go func() { s, _ := r.Run(context.Background()); ch <- s }()
	waitFor(t, "primeiro running", func() bool { return get(t, st, "TKT-001").Status == ticket.StatusRunning })
	if _, err := ticket.Drain(root); err != nil {
		t.Fatal(err)
	}
	sum := <-ch
	if sum.Reason != ReasonDrained || len(sum.Done) != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	if get(t, st, "TKT-002").Status != ticket.StatusPending || get(t, st, "TKT-003").Status != ticket.StatusPending {
		t.Fatal("drain não deveria iniciar novos tickets")
	}
}

func TestContextCancelRequeuesRunning(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "longo")
	ctx, cancel := context.WithCancel(context.Background())
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("sleep 30") })
	ch := make(chan *Summary, 1)
	go func() { s, _ := r.Run(ctx); ch <- s }()
	waitFor(t, "running", func() bool { return get(t, st, tk.ID).Status == ticket.StatusRunning })
	cancel()
	var sum *Summary
	select {
	case sum = <-ch:
	case <-time.After(20 * time.Second):
		t.Fatal("router não encerrou após cancelamento")
	}
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusPending || got.Attempts != 0 || got.Worker != "" {
		t.Fatalf("esperava pending sem tentativa consumida: %+v", got)
	}
	if sum.Reason != ReasonInterrupted || len(sum.Requeued) != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	if k := alertKinds(t, root); len(k) != 0 {
		t.Fatalf("interrupção não deveria gerar alertas: %v", k)
	}
	if s, _ := ReadState(root); s == nil || s.Status != StateStopped {
		t.Fatalf("state: %+v", s)
	}
}

func TestGlobalTimeoutStopsAndRequeues(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "longo")
	cfg := testCfg()
	cfg.Timeouts.Global = "300ms"
	r := newRouter(t, root, func(o *Options) { o.Config = cfg; o.AgentCmd = sh("sleep 30") })
	sum := runWithTimeout(context.Background(), t, r)
	if sum.Reason != ReasonGlobalTimeout || get(t, st, tk.ID).Status != ticket.StatusPending {
		t.Fatalf("summary: %+v ticket=%+v", sum, get(t, st, tk.ID))
	}
	if countKind(alertKinds(t, root), AlertGlobalTimeout) != 1 {
		t.Fatal("esperava alerta global_timeout")
	}
}

func TestOnceRunsSingleRound(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	for i := 0; i < 3; i++ {
		mk(t, st, "t")
	}
	r := newRouter(t, root, func(o *Options) { o.Workers = 2; o.Once = true })
	sum := runWithTimeout(context.Background(), t, r)
	if sum.Reason != ReasonOnce || len(sum.Done) != 2 {
		t.Fatalf("summary: %+v", sum)
	}
	if get(t, st, "TKT-003").Status != ticket.StatusPending {
		t.Fatal("--once não deveria iniciar o terceiro ticket")
	}
}

func TestFailedDependencyBlocksDependents(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")
	b := mk(t, st, "b", a.ID)
	cfg := testCfg()
	cfg.Retry.MaxAttempts = 1
	r := newRouter(t, root, func(o *Options) { o.Config = cfg; o.AgentCmd = sh("exit 1") })
	sum := runWithTimeout(context.Background(), t, r)
	if get(t, st, b.ID).Status != ticket.StatusPending || len(sum.Failed) != 1 || sum.Reason != ReasonStalled {
		t.Fatalf("summary: %+v", sum)
	}
	if len(sum.Unschedulable) != 1 || sum.Unschedulable[0] != b.ID {
		t.Fatalf("esperava %s em Unschedulable: %+v", b.ID, sum)
	}
}

func TestChildEnvFilteredAndInjected(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "env")
	r := newRouter(t, root, func(o *Options) {
		o.AgentCmd = sh("env")
		o.Environ = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "MY_API_TOKEN=supersecreto", "OUTRA=valor"}
	})
	runWithTimeout(context.Background(), t, r)
	got := get(t, st, tk.ID)
	data, err := os.ReadFile(filepath.Join(root, got.Result.Log))
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for _, want := range []string{"TB_TICKET_ID=" + tk.ID, "TB_WORKER_ID=w1", "TB_WORKSPACE=" + got.Result.Workspace, "TB_TICKET_FILE=" + filepath.Join(root, ".toolbelt", "tickets", tk.ID+".yaml")} {
		if !strings.Contains(log, want) {
			t.Errorf("log sem %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "supersecreto") || strings.Contains(log, "MY_API_TOKEN") || strings.Contains(log, "OUTRA=") {
		t.Fatalf("segredo/variável não permitida vazou para o filho:\n%s", log)
	}
}

func TestCommitIdentityFallbackWhenUnconfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "EMAIL"} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "-c", "user.name=setup", "-c", "user.email=setup@x", "commit", "-q", "-m", "init")

	st := ticket.Open(root)
	tk := mk(t, st, "sem identidade")
	r := newRouter(t, root, func(o *Options) { o.Environ = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home} })
	runWithTimeout(context.Background(), t, r)
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusDone {
		t.Fatalf("ticket: %+v err=%v", got, got.Result)
	}
	if who := runGit(t, root, "log", "-1", "--format=%an <%ae>", got.Result.HeadCommit); who != "tb <tb@localhost>" {
		t.Fatalf("who = %q", who)
	}
}

func TestRunRequiresAgentCmdAndHEAD(t *testing.T) {
	root := newRepo(t)
	r, err := New(&Options{Root: root, Config: testCfg(), AllowUnsandboxed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "agent_cmd") {
		t.Fatalf("esperava erro de agent_cmd: %v", err)
	}
	empty := t.TempDir()
	runGit(t, empty, "init", "-q")
	r2, _ := New(&Options{Root: empty, Config: testCfg(), AgentCmd: sh("true"), AllowUnsandboxed: true})
	if _, err := r2.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "commit") {
		t.Fatalf("esperava erro de repo sem commit: %v", err)
	}
	if _, err := New(&Options{Root: root, Workers: 2, Strategy: "xyz"}); err == nil {
		t.Fatal("strategy inválida deveria falhar")
	}
}

func TestRecoverOrphanRunning(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "orfao")
	if _, err := st.Transition(tk.ID, ticket.StatusRunning); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, root, nil)
	sum := runWithTimeout(context.Background(), t, r)
	if len(sum.Done) != 1 || get(t, st, tk.ID).Status != ticket.StatusDone {
		t.Fatalf("summary: %+v", sum)
	}
}

func assertPlanRounds(t *testing.T, p *Plan, a, b, c, d *ticket.Ticket) {
	t.Helper()
	if len(p.Rounds) != 2 {
		t.Fatalf("rounds: %+v", p.Rounds)
	}
	r1 := p.Rounds[0].Assignments
	if len(r1) != 2 || r1[0].Ticket != a.ID || r1[0].Worker != "w1" || r1[1].Ticket != b.ID || r1[1].Worker != "w2" {
		t.Fatalf("rodada 1: %+v", r1)
	}
	r2 := p.Rounds[1].Assignments
	if len(r2) != 2 || r2[0].Ticket != c.ID || r2[0].Via != ViaDirect || r2[0].Base != a.ID || r2[1].Ticket != d.ID || r2[1].Base != "HEAD" {
		t.Fatalf("rodada 2: %+v", r2)
	}
}

func TestBuildPlan(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "a")
	b := mk(t, st, "b")
	c := mk(t, st, "c", a.ID)
	d := mk(t, st, "d", a.ID, b.ID)
	f := mk(t, st, "f")
	if _, err := st.Transition(f.ID, ticket.StatusCancelled); err != nil {
		t.Fatal(err)
	}
	g := mk(t, st, "g", f.ID)
	board, _ := ticket.LoadBoard(st)
	p := BuildPlan(board, 2, ticket.StrategyDependency, true, 5)
	assertPlanRounds(t, &p, &a, &b, &c, &d)
	if len(p.Unschedulable) != 1 || p.Unschedulable[0].Ticket != g.ID {
		t.Fatalf("unschedulable: %+v", p.Unschedulable)
	}
	p2 := BuildPlan(board, 2, ticket.StrategyFIFO, false, 5)
	for _, rd := range p2.Rounds {
		for _, as := range rd.Assignments {
			if as.Via != ViaBoard {
				t.Fatalf("via direct sem direct_delivery: %+v", as)
			}
		}
	}
}

func TestParseCommand(t *testing.T) {
	cases := map[string][]string{
		`sh -c "echo hi > f"`:       {"sh", "-c", "echo hi > f"},
		`sh -c 'echo "a b" ; true'`: {"sh", "-c", `echo "a b" ; true`},
		`  a   b  `:                 {"a", "b"},
		`x\ y z`:                    {"x y", "z"},
		`""`:                        {""},
		`a "b \"c\" d"`:             {"a", `b "c" d`},
	}
	for in, want := range cases {
		got, err := ParseCommand(in)
		if err != nil || strings.Join(got, "\x00") != strings.Join(want, "\x00") || len(got) != len(want) {
			t.Errorf("ParseCommand(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{`a "b`, `a 'b`, `a\`} {
		if _, err := ParseCommand(bad); err == nil {
			t.Errorf("ParseCommand(%q) deveria falhar", bad)
		}
	}
}

func TestReadAlertsFilters(t *testing.T) {
	root := t.TempDir()
	l := &alertLog{path: AlertsPath(root)}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i, k := range []string{AlertWorkerSlow, AlertWorkerCrash, AlertWorkerSlow} {
		if err := l.append(&Alert{Time: base.Add(time.Duration(i) * time.Minute), Kind: k, Severity: SevWarn, Message: "m", Action: ActionNone}); err != nil {
			t.Fatal(err)
		}
	}
	if as, _ := ReadAlerts(root, AlertFilter{Kind: AlertWorkerSlow}); len(as) != 2 {
		t.Fatalf("kind: %d", len(as))
	}
	if as, _ := ReadAlerts(root, AlertFilter{Since: base.Add(30 * time.Second)}); len(as) != 2 {
		t.Fatalf("since: %d", len(as))
	}
	if as, _ := ReadAlerts(root, AlertFilter{Limit: 1}); len(as) != 1 || !as[0].Time.Equal(base.Add(2*time.Minute)) {
		t.Fatalf("limit: %+v", as)
	}
	if as, err := ReadAlerts(t.TempDir(), AlertFilter{}); err != nil || len(as) != 0 || as == nil {
		t.Fatalf("ausente deveria ser lista vazia: %v %v", as, err)
	}
}

func TestLoadStatusStale(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadStatus(root); err != ErrNoState {
		t.Fatalf("err = %v", err)
	}
	if _, err := writeState(root, &State{PID: 2147480000, Status: StateRunning, StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	rep, err := LoadStatus(root)
	if err != nil || !rep.Stale || rep.Alive {
		t.Fatalf("stale: %+v %v", rep, err)
	}
	if _, err := writeState(root, &State{PID: os.Getpid(), Status: StateRunning, StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if rep, _ := LoadStatus(root); !rep.Alive || rep.Stale {
		t.Fatalf("alive: %+v", rep)
	}
}
