//go:build !windows

package router

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func alertsOf(t *testing.T, root, kind string) []Alert {
	t.Helper()
	as, err := ReadAlerts(root, AlertFilter{Kind: kind})
	if err != nil {
		t.Fatal(err)
	}
	return as
}

// gitShim instala um "git" na frente do PATH que, para `worktree add`, roda
// pre antes e post depois do git real.
func gitShim(t *testing.T, pre, post string) {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh

case "$*" in *"worktree add"*) IS_ADD=1 ;; esac
if [ -n "$IS_ADD" ]; then
:
%s
fi
%s "$@"
rc=$?
if [ -n "$IS_ADD" ]; then
:
%s
fi
exit $rc
`, pre, gitBin, post)
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil { //nolint:gosec // teste
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Finding 2: o escape do ticket B, varrido pelo execute do ticket A (que termina
// primeiro), era atribuído a A. Agora falha B (por TB_TICKET_ID) e A conclui.
func TestEscapeSweptByOtherTicketIsAttributedToCulprit(t *testing.T) {
	release, ok := sandbox.AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	root := newRepo(t)
	st := ticket.Open(root)
	a := mk(t, st, "inocente")
	b := mk(t, st, "culpado")
	// o Landlock só deixa B escrever no próprio worktree; A só precisa ler o pidfile
	pidf := filepath.Join(root, ".toolbelt", "workspaces", b.ID, "pidfile")
	script := fmt.Sprintf(`if [ "$TB_TICKET_ID" = %q ]; then
  ( setsid sh -c 'echo $$ > pidfile; exec sleep 37' & )
  i=0; while [ ! -s %s ] && [ $i -lt 500 ]; do sleep 0.01; i=$((i+1)); done
  sleep 1.5
else
  i=0; while [ ! -s %s ] && [ $i -lt 500 ]; do sleep 0.01; i=$((i+1)); done
fi
echo x > f.txt`, b.ID, pidf, pidf)
	cfg := testCfg()
	cfg.Retry.MaxAttempts = 3
	// tick longo: só os Sweep do fim de cada execute observam o escape
	r := newRouter(t, root, func(o *Options) {
		o.Config = cfg
		o.Workers = 2
		o.Tick = 30 * time.Second
		o.AgentCmd = sh(script)
	})
	runWithTimeout(context.Background(), t, r)
	gotA, gotB := get(t, st, a.ID), get(t, st, b.ID)
	if gotA.Status != ticket.StatusDone {
		t.Fatalf("ticket inocente falhou: %+v %+v", gotA, gotA.Result)
	}
	if gotB.Status != ticket.StatusFailed || gotB.Attempts != 1 || !strings.Contains(gotB.Result.Error, "agent_escaped") {
		t.Fatalf("culpado deveria falhar sem retry: %+v %+v", gotB, gotB.Result)
	}
	data, _ := os.ReadFile(pidf)
	if pid, _ := strconv.Atoi(strings.TrimSpace(string(data))); pid <= 0 || syscall.Kill(pid, 0) == nil {
		t.Fatalf("escapado %d ainda vivo", pid)
	}
	as := alertsOf(t, root, AlertAgentEscaped)
	if len(as) != 1 || as[0].Ticket != b.ID {
		t.Fatalf("alertas: %+v", as)
	}
}

func newBareRouter(t *testing.T) *Router {
	t.Helper()
	return newRouter(t, newRepo(t), nil)
}

// Finding 2: atribuição por TB_TICKET_ID; ausente/desconhecido => conservador.
func assertUnattributableEscape(t *testing.T, r *Router, name string, o sandbox.Orphan) {
	t.Helper()
	al := r.classifyEscapes([]sandbox.Orphan{o})
	if len(al) != 1 || al[0].kind != AlertAgentEscaped || al[0].ticket != "" ||
		!strings.Contains(al[0].msg, "não atribuível") || !strings.Contains(al[0].msg, "TKT-001, TKT-002") {
		t.Fatalf("%s: %+v", name, al)
	}
	if !r.takeEscaped("TKT-001") || !r.takeEscaped("TKT-002") {
		t.Fatalf("%s: deveria marcar todos os tickets em execução", name)
	}
}

func TestCollectEscapesAttribution(t *testing.T) {
	r := newBareRouter(t)
	r.markActive("TKT-001", "w1")
	r.markActive("TKT-002", "w2")
	tkt := func(id string) map[string]string { return map[string]string{ticketIDVar: id} }
	al := r.classifyEscapes([]sandbox.Orphan{{PID: 1, Vars: tkt("TKT-002")}})
	if len(al) != 1 || al[0].ticket != "TKT-002" || r.takeEscaped("TKT-001") || !r.takeEscaped("TKT-002") {
		t.Fatalf("atribuído: %+v", al)
	}
	assertUnattributableEscape(t, r, "ausente", sandbox.Orphan{PID: 2})
	assertUnattributableEscape(t, r, "env -u", sandbox.Orphan{PID: 3, Vars: map[string]string{}})
	assertUnattributableEscape(t, r, "desconhecido", sandbox.Orphan{PID: 4, Vars: tkt("TKT-999")})
	assertUnattributableEscape(t, r, "foreign-miss", sandbox.Orphan{PID: 5, ForeignGroup: true, GroupOwner: "TKT-001"})
	al = r.classifyEscapes([]sandbox.Orphan{{PID: 6, ForeignGroup: true, GroupOwner: "TKT-001", Vars: tkt("TKT-002")}})
	if len(al) != 1 || al[0].ticket != "TKT-002" || r.takeEscaped("TKT-001") || !r.takeEscaped("TKT-002") {
		t.Fatalf("infiltrado: %+v", al)
	}
	r.unmarkActive("TKT-001")
	r.unmarkActive("TKT-002")
	al = r.classifyEscapes([]sandbox.Orphan{{PID: 7}})
	if len(al) != 1 || al[0].kind != AlertOrphanKilled || r.takeEscaped("TKT-001") {
		t.Fatalf("sem ativos: %+v", al)
	}
}

// Finding 4: órfão da janela de spawn sem TB_TICKET_ID não falha ninguém.
func TestSpawnWindowOrphanIsNotAttributed(t *testing.T) {
	r := newBareRouter(t)
	r.markActive("TKT-001", "w1")
	al := r.classifyEscapes([]sandbox.Orphan{{PID: 1, InSpawnWindow: true}})
	if len(al) != 1 || al[0].kind != AlertOrphanKilled || al[0].severity != SevWarn || r.takeEscaped("TKT-001") {
		t.Fatalf("%+v", al)
	}
	// com TB_TICKET_ID, ou infiltrado em grupo de agente, a janela não absolve
	al = r.classifyEscapes([]sandbox.Orphan{{PID: 2, InSpawnWindow: true, Vars: map[string]string{ticketIDVar: "TKT-001"}}})
	if len(al) != 1 || al[0].kind != AlertAgentEscaped || !r.takeEscaped("TKT-001") {
		t.Fatalf("%+v", al)
	}
	al = r.classifyEscapes([]sandbox.Orphan{{PID: 3, InSpawnWindow: true, ForeignGroup: true, GroupOwner: "TKT-009"}})
	if len(al) != 1 || al[0].kind != AlertAgentEscaped {
		t.Fatalf("%+v", al)
	}
}

// Finding 2: a marca de escape é limpa em complete (inclusive no descarte) para
// não envenenar a nova tentativa do mesmo ticket.
func TestCompleteClearsEscapedMarkOnDiscard(t *testing.T) {
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "x") // pending: complete cai no caminho de descarte
	r := newRouter(t, root, nil)
	r.escapedTkts = map[string]bool{tk.ID: true}
	r.complete(&workerDone{slot: &slot{id: "w1", ticket: tk.ID}, id: tk.ID})
	if r.escapedTkts[tk.ID] {
		t.Fatal("marca de escape sobreviveu ao complete (envenena a retentativa)")
	}
}

// Finding 4: um daemon destacado disparado pelo git (hook/fsmonitor) durante a
// criação do worktree não falha o ticket nem é atribuído a ele.
func TestDaemonFromWorktreeCreationDoesNotFailTicket(t *testing.T) {
	release, ok := sandbox.AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	pidf := filepath.Join(t.TempDir(), "daemon.pid")
	gitShim(t, "", fmt.Sprintf("setsid sh -c 'echo $$ > %s; exec sleep 37' </dev/null >/dev/null 2>&1 &", pidf))
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "daemon do usuário")
	r := newRouter(t, root, func(o *Options) { o.AgentCmd = sh("sleep 0.5; echo ok > f.txt") })
	runWithTimeout(context.Background(), t, r)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusDone {
		t.Fatalf("ticket falhou por daemon alheio: %+v %+v", got, got.Result)
	}
	kinds := alertKinds(t, root)
	if countKind(kinds, AlertAgentEscaped) != 0 || countKind(kinds, AlertOrphanKilled) != 1 {
		all, _ := ReadAlerts(root, AlertFilter{})
		t.Fatalf("alertas: %v %+v", kinds, all)
	}
	if as := alertsOf(t, root, AlertOrphanKilled); as[0].Severity != SevWarn || as[0].Ticket != "" {
		t.Fatalf("%+v", as[0])
	}
	data, _ := os.ReadFile(pidf)
	if pid, _ := strconv.Atoi(strings.TrimSpace(string(data))); pid <= 0 || syscall.Kill(pid, 0) == nil {
		t.Fatalf("daemon %d ainda vivo", pid)
	}
}

// Finding 4: o git do workspace manager não roda hooks do usuário.
func TestWorktreeCreationDoesNotRunUserHooks(t *testing.T) {
	root := newRepo(t)
	marker := filepath.Join(t.TempDir(), "hook-ran")
	hook := filepath.Join(root, ".git", "hooks", "post-checkout")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch "+marker+"\nsetsid sleep 37 </dev/null >/dev/null 2>&1 &\n"), 0o755); err != nil { //nolint:gosec // teste
		t.Fatal(err)
	}
	st := ticket.Open(root)
	tk := mk(t, st, "hook")
	r := newRouter(t, root, nil)
	runWithTimeout(context.Background(), t, r)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("post-checkout do usuário foi executado pelo router")
	}
	if got := get(t, st, tk.ID); got.Status != ticket.StatusDone {
		t.Fatalf("%+v %+v", got, got.Result)
	}
}

// Finding 5: Sweep não espera por um `git worktree add` lento (antes, wsSpawnMu
// ficava preso durante todo o git e parava o loop do router).
func TestSweepNotBlockedBySlowWorktreeAdd(t *testing.T) {
	release, ok := sandbox.AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	gitShim(t, "sleep 1.2", "")
	root := newRepo(t)
	r := newRouter(t, root, nil)
	wsm, err := newWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	r.wsm = wsm
	st := ticket.Open(root)
	tk := mk(t, st, "lento")
	head := runGit(t, root, "rev-parse", "HEAD")
	done := make(chan error, 1)
	go func() { _, err := r.createWorkspace(&tk, head); done <- err }()
	time.Sleep(400 * time.Millisecond) // dentro do git lento
	start := time.Now()
	if esc := procs.Sweep(); len(esc) != 0 {
		t.Fatalf("git rastreado tomado por órfão: %+v", esc)
	}
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("Sweep bloqueado %v pelo git do workspace", d)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("createWorkspace: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("createWorkspace não terminou")
	}
}

// Finding 7: sem Landlock e sem opt-in explícito, o router não toca em nada.
func TestRouterRunFailsClosedWithoutLandlock(t *testing.T) {
	restore := sandbox.OverrideLandlockProbe(func() (bool, int, error) { return false, 0, errors.New("simulado") })
	defer restore()
	root := newRepo(t)
	st := ticket.Open(root)
	tk := mk(t, st, "x")
	r := newRouter(t, root, func(o *Options) { o.AllowUnsandboxed = false })
	if _, err := r.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "--allow-unsandboxed") {
		t.Fatalf("Run deveria recusar sem Landlock/opt-in: %v", err)
	}
	if got := get(t, st, tk.ID); got.Status != ticket.StatusPending {
		t.Fatalf("ticket tocado: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".toolbelt", "workspaces", tk.ID)); err == nil {
		t.Fatal("worktree criado sem sandbox")
	}
	r2 := newRouter(t, root, nil) // AllowUnsandboxed: true
	runWithTimeout(context.Background(), t, r2)
	if got := get(t, st, tk.ID); got.Status != ticket.StatusDone {
		t.Fatalf("com opt-in explícito deveria rodar: %+v %+v", got, got.Result)
	}
}

// Finding 8: config global do usuário (filtros git-lfs etc.) não roda sobre o
// conteúdo do agente; identidade global ainda é usada.
func TestGlobalGitConfigFilterNeverRunsAndIdentityIsKept(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(t.TempDir(), "filter-ran")
	attrs := filepath.Join(home, "attrs")
	if err := os.WriteFile(attrs, []byte("*.txt filter=spy\n"), 0o644); err != nil { //nolint:gosec // teste
		t.Fatal(err)
	}
	gc := fmt.Sprintf("[user]\n\tname = Global Dev\n\temail = global@example.com\n[core]\n\tattributesFile = %s\n[filter \"spy\"]\n\tclean = \"sh -c 'touch %s; cat'\"\n", attrs, marker)
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gc), 0o644); err != nil { //nolint:gosec // teste
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := newRepo(t)
	runGit(t, root, "config", "--unset", "user.name")
	runGit(t, root, "config", "--unset", "user.email")
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	st := ticket.Open(root)
	tk := mk(t, st, "lfs")
	r := newRouter(t, root, func(o *Options) {
		o.AgentCmd = sh("echo conteudo > f.txt")
		o.Environ = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home}
	})
	runWithTimeout(context.Background(), t, r)
	got := get(t, st, tk.ID)
	if got.Status != ticket.StatusDone {
		t.Fatalf("%+v %+v", got, got.Result)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("filtro da config global do usuário rodou sobre o conteúdo do agente")
	}
	if who := runGit(t, root, "log", "-1", "--format=%an <%ae>", got.Result.HeadCommit); who != "Global Dev <global@example.com>" {
		t.Fatalf("who = %q", who)
	}
}
