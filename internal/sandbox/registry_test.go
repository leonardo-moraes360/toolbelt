//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func registrySandbox(t *testing.T, dir string, reg *Registry) *Sandbox {
	t.Helper()
	sb, err := New(Config{WorkDir: dir, ProcessGroup: true, AllowUnsandboxed: true, Registry: reg})
	if err != nil {
		t.Fatal(err)
	}
	return sb
}

func TestRegistrySweepKillsSetsidDescendant(t *testing.T) {
	release, ok := AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	dir := t.TempDir()
	reg := NewRegistry()
	script := `setsid sh -c 'echo $$ > pidfile; exec sleep 37' &
i=0; while [ ! -s pidfile ] && [ $i -lt 500 ]; do sleep 0.01; i=$((i+1)); done
sleep 30 &`
	res, err := registrySandbox(t, dir, reg).Run(context.Background(), "sh", "-c", script)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "pidfile"))
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	esc := reg.Sweep()
	if len(esc) != 1 || esc[0].PID != pid {
		t.Fatalf("escapes=%+v pid=%d", esc, pid)
	}
	if syscall.Kill(pid, 0) == nil {
		t.Fatal("processo escapado ainda vivo")
	}
	if again := reg.Sweep(); len(again) != 0 {
		t.Fatalf("segunda varredura: %+v", again)
	}
}

func TestRegistrySameGroupBackgroundIsNotAnEscape(t *testing.T) {
	release, ok := AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	reg := NewRegistry()
	start := time.Now()
	res, err := registrySandbox(t, t.TempDir(), reg).Run(context.Background(), "sh", "-c", "sleep 30 & sleep 30 & exit 0")
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if esc := reg.Sweep(); len(esc) != 0 {
		t.Fatalf("falso positivo: %+v", esc)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("Run demorou demais")
	}
}

func ownedSandbox(t *testing.T, dir string, reg *Registry, owner string) *Sandbox {
	t.Helper()
	sb, err := New(Config{
		WorkDir: dir, ProcessGroup: true, AllowUnsandboxed: true, Registry: reg, GroupOwner: owner,
		Env: []string{"PATH=" + os.Getenv("PATH"), "OWNER=" + owner},
	})
	if err != nil {
		t.Fatal(err)
	}
	return sb
}

func ownedRegistry() *Registry {
	reg := NewRegistry("OWNER")
	reg.SetOwnerVar("OWNER")
	return reg
}

func waitFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout esperando %s", path)
	return 0
}

// Finding 3: agent A joins agent B's pgid (setpgid) to hide from the sweep.
// Before: an orphan in a live agent group was always ignored.
func TestRegistryOrphanJoinedForeignLiveGroupIsEscape(t *testing.T) {
	release, ok := AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	reg := ownedRegistry()
	shared := t.TempDir()
	bPid := filepath.Join(shared, "b.pid")
	ctx, cancel := context.WithCancel(context.Background())
	bDone := make(chan struct{})
	go func() {
		defer close(bDone)
		_, _ = ownedSandbox(t, shared, reg, "B").Run(ctx, "sh", "-c", "echo $$ > "+bPid+"; exec sleep 30")
	}()
	bGroup := waitFile(t, bPid)
	escPid := filepath.Join(shared, "a-esc.pid")
	script := `( perl -e 'setpgrp(0, ` + strconv.Itoa(bGroup) + `) or die; open(F, ">` + escPid + `"); print F $$; close F; exec "sleep", "31"' & )
i=0; while [ ! -s ` + escPid + ` ] && [ $i -lt 500 ]; do sleep 0.01; i=$((i+1)); done`
	res, err := ownedSandbox(t, shared, reg, "A").Run(context.Background(), "sh", "-c", script)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	pid := waitFile(t, escPid)
	esc := reg.Sweep()
	cancel()
	<-bDone
	if len(esc) != 1 || esc[0].PID != pid {
		t.Fatalf("escapes=%+v pid=%d", esc, pid)
	}
	o := esc[0]
	if !o.ForeignGroup || o.GroupOwner != "B" || o.PGID != bGroup || o.Vars["OWNER"] != "A" {
		t.Fatalf("classificação: %+v", o)
	}
	if syscall.Kill(pid, 0) == nil {
		t.Fatal("infiltrado ainda vivo")
	}
}

// A legitimate descendant of the agent itself (same owner) reparented to the
// router while the leader is alive is NOT an escape; without TB_TICKET_ID (env -u) it is suspect.
func TestRegistryOwnGroupOrphanOwnerCheck(t *testing.T) {
	release, ok := AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	reg := ownedRegistry()
	// sweep in the MIDDLE of execution (like the watchdog tick): the leader is still alive
	midRun := func(script string) []Orphan {
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = ownedSandbox(t, t.TempDir(), reg, "A").Run(context.Background(), "sh", "-c", script)
		}()
		time.Sleep(700 * time.Millisecond)
		esc := reg.Sweep()
		<-done
		return esc
	}
	if esc := midRun("(sleep 30 &); sleep 1.2"); len(esc) != 0 {
		t.Fatalf("falso positivo para descendente do próprio grupo: %+v", esc)
	}
	esc := midRun("(env -u OWNER sleep 30 &); sleep 1.2")
	if len(esc) != 1 || !esc[0].ForeignGroup || esc[0].Vars["OWNER"] != "" || esc[0].GroupOwner != "A" {
		t.Fatalf("env -u em grupo próprio deveria ser escape não atribuível: %+v", esc)
	}
}

// Finding 4: an orphan born inside a spawn window is flagged; outside it, not.
func TestRegistrySpawnWindowMarksOrphan(t *testing.T) {
	release, ok := AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	reg := NewRegistry("TB_TICKET_ID")
	daemon := func() {
		t.Helper()
		cmd := exec.Command("sh", "-c", "setsid sleep 33 >/dev/null 2>&1 </dev/null &")
		if err := reg.Run(cmd, false); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	end := reg.BeginSpawnWindow()
	daemon()
	end()
	in := reg.Sweep()
	time.Sleep(100 * time.Millisecond)
	daemon() // outside the window
	out := reg.Sweep()
	if len(in) != 1 || !in[0].InSpawnWindow || in[0].Start == 0 {
		t.Fatalf("dentro da janela: %+v", in)
	}
	if len(out) != 1 || out[0].InSpawnWindow {
		t.Fatalf("fora da janela: %+v", out)
	}
}

// Finding 5: Sweep never waits for a slow tracked child (no global lock).
func TestRegistrySweepDoesNotBlockOnSlowTrackedChild(t *testing.T) {
	reg := NewRegistry()
	done := make(chan error, 1)
	go func() { done <- reg.Run(exec.Command("sleep", "1"), false) }()
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	if esc := reg.Sweep(); len(esc) != 0 {
		t.Fatalf("filho rastreado tomado por órfão: %+v", esc)
	}
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("Sweep bloqueou %v durante filho rastreado", d)
	}
	if err := <-done; err != nil {
		t.Fatalf("filho rastreado foi morto: %v", err)
	}
}

// Finding 6: the group registry does not accumulate entries.
func TestRegistryGroupsArePruned(t *testing.T) {
	release, ok := AcquireSubreaper()
	if !ok {
		t.Skip("PR_SET_CHILD_SUBREAPER indisponível")
	}
	defer release()
	reg := ownedRegistry()
	for i := 0; i < 3; i++ {
		if _, err := ownedSandbox(t, t.TempDir(), reg, "A").Run(context.Background(), "sh", "-c", "sleep 30 & exit 0"); err != nil {
			t.Fatal(err)
		}
	}
	reg.Sweep()
	reg.mu.Lock()
	n := len(reg.groups)
	reg.mu.Unlock()
	if n != 0 {
		t.Fatalf("groups não podado: %d entradas", n)
	}
}

func TestRegistryGroupKeyedByStartTime(t *testing.T) {
	reg := ownedRegistry()
	k := groupKey{pid: os.Getpid(), start: 1}
	reg.groups[k] = &groupInfo{owner: "old", finished: true}
	reg.groups[groupKey{pid: os.Getpid(), start: 2}] = &groupInfo{owner: "new"}
	if g := reg.findGroupLocked(os.Getpid()); g == nil || g.owner != "new" {
		t.Fatalf("deveria preferir o líder mais novo: %+v", g)
	}
}
