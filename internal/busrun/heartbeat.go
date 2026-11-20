package busrun

import (
	"context"
	"errors"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

// runHeartbeats renews the agent's lease at the channel's interval until
// ctx ends (docs/bus-lease.md R6) and audits each newly lost thread once
// (thread and new owner, never a payload). The harness never claims or
// completes on its own.
func (h *Harness) runHeartbeats(ctx context.Context) {
	for {
		every, stop := h.heartbeat()
		if stop {
			return
		}
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

// heartbeat sends one heartbeat and returns when to send the next; stop
// is true when the session is no longer valid (the main loop stops too).
func (h *Harness) heartbeat() (next time.Duration, stop bool) {
	hb, err := h.store.Heartbeat(h.member)
	if err != nil {
		return h.heartbeatFailed(err)
	}
	h.hbErr = ""
	h.auditLost(&hb)
	d, err := time.ParseDuration(hb.HeartbeatInterval)
	if err != nil || d <= 0 {
		return 10 * time.Second, false
	}
	return d, false
}

// auditLost logs lease_lost once per loss: a thread held again, or taken
// by another owner, makes the next loss new.
func (h *Harness) auditLost(hb *bus.HeartbeatResult) {
	if h.lost == nil {
		h.lost = map[string]string{}
	}
	for _, t := range hb.Held {
		delete(h.lost, t.Thread)
	}
	for _, l := range hb.Lost {
		if prev, ok := h.lost[l.Thread]; ok && prev == l.NewOwner {
			continue
		}
		h.lost[l.Thread] = l.NewOwner
		detail := "thread " + l.Thread + " released after the lease expired"
		if l.NewOwner != "" {
			detail = "thread " + l.Thread + " taken over by " + l.NewOwner
		}
		h.audit(&Event{Event: "lease_lost", Detail: detail})
	}
}

// heartbeatFailed audits each distinct error once. A transient error
// (SQLITE_BUSY) retries soon so a short lease survives it; an expired
// channel backs off; an invalid session stops the heartbeats.
func (h *Harness) heartbeatFailed(err error) (time.Duration, bool) {
	if msg := err.Error(); msg != h.hbErr {
		h.hbErr = msg
		h.audit(&Event{Event: "heartbeat_failed", Detail: msg})
	}
	switch {
	case errors.Is(err, bus.ErrUnauthorized):
		return 0, true
	case errors.Is(err, bus.ErrChannelExpired):
		return time.Minute, false
	}
	return 5 * time.Second, false
}
