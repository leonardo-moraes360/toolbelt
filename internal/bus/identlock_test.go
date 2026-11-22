package bus

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestLockIdentityExclusiveAndNamesHolder(t *testing.T) {
	root := t.TempDir()
	rel, err := LockClient(root, "ch", "a", "tb mcp --bus-as")
	must(t, err)
	_, err = LockClient(root, "ch", "a", "tb bus run")
	var inUse *SessionInUseError
	if !errors.As(err, &inUse) || !errors.Is(err, ErrSessionInUse) {
		t.Fatalf("second holder: %v", err)
	}
	if inUse.PID != os.Getpid() || inUse.Kind != "tb mcp --bus-as" {
		t.Fatalf("holder = %+v", inUse)
	}
	if msg := err.Error(); !strings.Contains(msg, "session_in_use") || !strings.Contains(msg, "pid") {
		t.Fatalf("message: %s", msg)
	}
	// Another identity is independent.
	other, err := LockClient(root, "ch", "b", "x")
	must(t, err)
	other()
	rel()
	again, err := LockClient(root, "ch", "a", "x")
	must(t, err)
	again()
}

func TestLockIdentityConcurrentBindOneWinner(t *testing.T) {
	root := t.TempDir()
	const n = 16
	var wins, attempted atomic.Int32
	var wg sync.WaitGroup
	release := make(chan struct{})
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rel, err := LockClient(root, "ch", "a", "tb mcp --bus-as")
			attempted.Add(1)
			if err == nil {
				wins.Add(1)
				<-release
				rel()
				return
			}
			if !errors.Is(err, ErrSessionInUse) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	// Nobody releases before every goroutine has tried.
	for attempted.Load() < n {
		runtime.Gosched()
	}
	close(release)
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("wins = %d, want exactly 1", wins.Load())
	}
}

// A GUI client (client lock) and `tb bus run` (harness lock) coexist in
// notify mode; two of the same kind do not.
func TestClientAndHarnessLocksAreIndependent(t *testing.T) {
	root := t.TempDir()
	c, err := LockClient(root, "ch", "a", "tb mcp --bus-as")
	must(t, err)
	h, err := LockHarness(root, "ch", "a")
	must(t, err)
	if _, err := LockHarness(root, "ch", "a"); !errors.Is(err, ErrSessionInUse) {
		t.Fatalf("second harness: %v", err)
	}
	if err := ProbeClient(root, "ch", "a"); !errors.Is(err, ErrSessionInUse) {
		t.Fatalf("probe client: %v", err)
	}
	c()
	h()
	must(t, ProbeClient(root, "ch", "a"))
	must(t, ProbeHarness(root, "ch", "a"))
}

func TestJoinRefusedWhileIdentityHeld(t *testing.T) {
	s, _ := openTest(t)
	_, ownerInv, _ := s.CreateChannel("c", "boss", 0, 0)
	first, err := s.Join("c", ownerInv.Code, &AgentCard{Name: "boss"}, "")
	must(t, err)
	rel, err := LockClient(s.Root(), "c", "boss", "tb mcp --bus-as")
	must(t, err)
	re, _ := s.CreateInvite("c", RoleSupervisor, "boss", 0)
	if _, err := s.Join("c", re.Code, &AgentCard{Name: "boss"}, ""); !errors.Is(err, ErrSessionInUse) {
		t.Fatalf("re-join while held: %v", err)
	}
	if _, err := s.Auth("c", "boss", first.Token); err != nil {
		t.Fatalf("refused re-join must not rotate the token: %v", err)
	}
	rel()
	h, err := LockHarness(s.Root(), "c", "boss")
	must(t, err)
	if _, err := s.Join("c", re.Code, &AgentCard{Name: "boss"}, ""); !errors.Is(err, ErrSessionInUse) {
		t.Fatalf("re-join while a harness runs: %v", err)
	}
	h()
	if _, err := s.Join("c", re.Code, &AgentCard{Name: "boss"}, ""); err != nil {
		t.Fatalf("re-join after release (the refused attempt must not burn the invite): %v", err)
	}
}

// A holder killed without cleanup must not block the identity: the kernel
// drops the flock with the process.
func TestLockIdentityRecoversFromCrashedHolder(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(os.Environ(), "TB_LOCK_HELPER=1", "TB_LOCK_ROOT="+root, "TB_LOCK_STATE="+os.Getenv(EnvStateDir))
	out, err := cmd.StdoutPipe()
	must(t, err)
	must(t, cmd.Start())
	line, err := bufio.NewReader(out).ReadString('\n')
	must(t, err)
	if strings.TrimSpace(line) != "locked" {
		t.Fatalf("helper: %q", line)
	}
	_, err = LockClient(root, "ch", "a", "x")
	var inUse *SessionInUseError
	if !errors.As(err, &inUse) || inUse.PID != cmd.Process.Pid {
		t.Fatalf("while the helper lives: %v (helper pid %d)", err, cmd.Process.Pid)
	}
	must(t, cmd.Process.Kill())
	_ = cmd.Wait()
	rel, err := LockClient(root, "ch", "a", "x")
	must(t, err)
	rel()
}

func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("TB_LOCK_HELPER") != "1" {
		t.Skip("helper for TestLockIdentityRecoversFromCrashedHolder")
	}
	// TestMain gave this process its own state dir; share the parent's.
	if err := os.Setenv(EnvStateDir, os.Getenv("TB_LOCK_STATE")); err != nil {
		t.Fatal(err)
	}
	if _, err := LockClient(os.Getenv("TB_LOCK_ROOT"), "ch", "a", "helper"); err != nil {
		t.Fatal(err)
	}
	_, _ = os.Stdout.WriteString("locked\n")
	select {}
}
