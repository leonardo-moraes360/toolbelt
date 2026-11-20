package busrun

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

// blockingAdapter holds the wake until released, like a long agent turn.
type blockingAdapter struct {
	started chan struct{}
	release chan struct{}
	once    atomic.Bool
}

func (b *blockingAdapter) Wake(ctx context.Context, _ *Packet, _ *State) (Outcome, error) {
	if b.once.CompareAndSwap(false, true) {
		close(b.started)
	}
	select {
	case <-b.release:
	case <-ctx.Done():
	}
	return Outcome{Session: "s"}, nil
}

func lastSeen(t *testing.T, e *env) time.Time {
	t.Helper()
	st, err := e.store.Status("ch")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range st.Agents {
		if a.Name == "w" {
			return a.LastSeen
		}
	}
	t.Fatal("agent w missing")
	return time.Time{}
}

// AC9: heartbeats keep going while a wake is blocked.
func TestHarnessHeartbeatsDuringWake(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	ba := &blockingAdapter{started: make(chan struct{}), release: make(chan struct{})}
	h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Adapter: ba, Poll: 10 * time.Millisecond,
		Backoff: time.Millisecond, Attempts: 2, AllowUnsandboxed: true, HeartbeatEvery: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	if _, err := e.store.ClaimThread(e.worker, "T"); err != nil {
		t.Fatal(err)
	}
	e.send(t, bus.TypeQuery, `{"q":1}`)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.Run(ctx, false) }()
	<-ba.started
	before := lastSeen(t, e)
	time.Sleep(120 * time.Millisecond)
	if !lastSeen(t, e).After(before) {
		t.Fatal("no heartbeat while the wake was blocked")
	}
	close(ba.release)
	time.Sleep(80 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHarnessAuditsLeaseLost(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	h := e.open(t, &fakeAdapter{})
	if _, err := e.store.ClaimThread(e.worker, "T"); err != nil {
		t.Fatal(err)
	}
	// Mark the claim released as the watchdog would, without expiring w.
	h.store.SetClock(func() time.Time { return time.Now().Add(bus.DefaultLease + time.Minute) })
	if _, err := h.store.Watchdog("ch", false); err != nil {
		t.Fatal(err)
	}
	h.store.SetClock(time.Now)
	h.heartbeat()
	h.heartbeat()
	audit := readAudit(t, e)
	if strings.Count(audit, `"lease_lost"`) != 1 || !strings.Contains(audit, "thread T released") {
		t.Fatalf("audit:\n%s", audit)
	}
}

// A takeover by another agent is audited with the new owner, once, and a
// thread held again and lost again is audited again.
func TestHarnessAuditsTakeover(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	h := e.open(t, &fakeAdapter{})
	clock := time.Now()
	now := func() time.Time { return clock }
	e.store.SetClock(now)
	h.store.SetClock(now)
	claim := func(m bus.Member) {
		t.Helper()
		if _, err := e.store.ClaimThread(m, "T"); err != nil {
			t.Fatal(err)
		}
	}
	claim(e.worker)
	h.heartbeat()
	for range 2 {
		clock = clock.Add(bus.DefaultLease + time.Minute)
		claim(e.owner) // w is silent: takeover
		h.heartbeat()
		h.heartbeat()
		clock = clock.Add(bus.DefaultLease + time.Minute)
		claim(e.worker) // boss is silent: w takes it back
		h.heartbeat()
	}
	if n := strings.Count(readAudit(t, e), "taken over by agent:boss"); n != 2 {
		t.Fatalf("lease_lost audited %d times, want 2:\n%s", n, readAudit(t, e))
	}
}

func TestHarnessHeartbeatFailureAuditedOnce(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	h := e.open(t, &fakeAdapter{})
	if _, err := e.store.ExtendChannel("ch", time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	for range 3 {
		if d, stop := h.heartbeat(); stop || d != time.Minute {
			t.Fatalf("expired channel: %v %v", d, stop)
		}
	}
	if n := strings.Count(readAudit(t, e), "heartbeat_failed"); n != 1 {
		t.Fatalf("heartbeat_failed logged %d times", n)
	}
}
