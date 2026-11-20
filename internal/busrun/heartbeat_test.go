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
