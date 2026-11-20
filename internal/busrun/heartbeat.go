package busrun

import (
	"context"
	"time"
)

// runHeartbeats renews the agent's lease at the channel's interval until
// ctx ends (docs/bus-lease.md R6) and audits each newly lost thread once
// (thread and new owner, never a payload). A failed heartbeat is retried
// sooner; the harness never claims or completes on its own.
func (h *Harness) runHeartbeats(ctx context.Context) {
	for {
		every := h.heartbeat()
		if h.opts.HeartbeatEvery > 0 {
			every = h.opts.HeartbeatEvery
		}
		t := time.NewTimer(every)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// heartbeat sends one heartbeat and returns when to send the next.
func (h *Harness) heartbeat() time.Duration {
	hb, err := h.store.Heartbeat(h.member)
	if err != nil {
		h.audit(&Event{Event: "heartbeat_failed", Detail: err.Error()})
		return 5 * time.Second
	}
	if h.lost == nil {
		h.lost = map[string]bool{}
	}
	for _, l := range hb.Lost {
		if h.lost[l.Thread] {
			continue
		}
		h.lost[l.Thread] = true
		detail := "thread " + l.Thread + " released after the lease expired"
		if l.NewOwner != "" {
			detail = "thread " + l.Thread + " taken over by " + l.NewOwner
		}
		h.audit(&Event{Event: "lease_lost", Detail: detail})
	}
	d, err := time.ParseDuration(hb.HeartbeatInterval)
	if err != nil || d <= 0 {
		return 10 * time.Second
	}
	return d
}
