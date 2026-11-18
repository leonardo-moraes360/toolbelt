package router

import (
	"fmt"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// ticketIDVar is the environment variable (inherited from the agent) that
// attributes an escaped process to its ticket. It is the only one read from
// /proc/<pid>/environ.
const ticketIDVar = "TB_TICKET_ID"

// pendingAlert is decided off the main goroutine (in a worker): alert state
// belongs to the Run goroutine, which emits it in complete.
type pendingAlert struct {
	kind, severity, worker, ticket, msg, action string
}

func (r *Router) emit(p *pendingAlert) {
	r.alert(p.kind, p.severity, p.worker, p.ticket, p.msg, p.action)
}

// markActive/unmarkActive track tickets whose AGENT is running (from sandbox
// start to the end of execute). They are the suspects of an unattributable
// escape; a worker still in createWorkspace has no agent yet.
func (r *Router) markActive(id, worker string) {
	r.escapedMu.Lock()
	if r.active == nil {
		r.active = map[string]string{}
	}
	r.active[id] = worker
	r.escapedMu.Unlock()
}

func (r *Router) unmarkActive(id string) {
	r.escapedMu.Lock()
	delete(r.active, id)
	r.escapedMu.Unlock()
}

// takeEscaped consumes a ticket's escape mark (see collectEscapes).
func (r *Router) takeEscaped(id string) bool {
	r.escapedMu.Lock()
	defer r.escapedMu.Unlock()
	ok := r.escapedTkts[id]
	delete(r.escapedTkts, id)
	return ok
}

// collectEscapes sweeps orphans (already killed by the Registry) and decides
// whom to attribute them to. Safe from any goroutine; returns the alerts to emit.
//
// Attribution (never "the ticket whose execute ran the sweep"):
//   - benign: born inside a spawn window of the workspace manager's git (user
//     hook/fsmonitor daemon), with no TB_TICKET_ID and without having entered an
//     agent group: killed, orphan_killed alert (warn), no ticket fails.
//   - TB_TICKET_ID of a ticket with a running agent: that ticket fails.
//   - TB_TICKET_ID missing (env -u), unreadable or unknown (finished, forged):
//     unattributable. Conservative: ALL tickets with an agent running at
//     detection time fail (agent_escaped alert listing them, reason
//     "não atribuível"). With no agent running: orphan_killed (warn).
//
// Limitation: an agent that forges the TB_TICKET_ID of another ACTIVE ticket gets
// the escape attributed to that ticket (the environment is the only clue left
// after reparenting).
func (r *Router) collectEscapes() []pendingAlert {
	return r.classifyEscapes(procs.Sweep())
}

// classifyEscapes applies collectEscapes' attribution policy to orphans that
// were already swept (split out to be testable with synthetic orphans).
func (r *Router) classifyEscapes(orphans []sandbox.Orphan) []pendingAlert {
	if len(orphans) == 0 {
		return nil
	}
	var out []pendingAlert
	r.escapedMu.Lock()
	defer r.escapedMu.Unlock()
	mark := func(id string) {
		if r.escapedTkts == nil {
			r.escapedTkts = map[string]bool{}
		}
		r.escapedTkts[id] = true
	}
	for _, o := range orphans {
		id := o.Vars[ticketIDVar]
		where := fmt.Sprintf("process %d", o.PID)
		if o.ForeignGroup {
			where += fmt.Sprintf(" (infiltrated into the agent group of %s)", o.GroupOwner)
		}
		if o.InSpawnWindow && id == "" && !o.ForeignGroup {
			out = append(out, pendingAlert{AlertOrphanKilled, SevWarn, "", "",
				fmt.Sprintf("%s was spawned during worktree creation (user hook/fsmonitor daemon?) without TB_TICKET_ID; terminated, no ticket was failed", where), ActionNone})
			continue
		}
		if worker, ok := r.active[id]; ok && id != "" {
			mark(id)
			out = append(out, pendingAlert{AlertAgentEscaped, SevError, worker, id,
				fmt.Sprintf("%s escaped the agent group of %s (setsid/setpgid) and was terminated; worktree was not touched", where, id), ActionEscalate})
			continue
		}
		ids := make([]string, 0, len(r.active))
		for t := range r.active {
			ids = append(ids, t)
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			out = append(out, pendingAlert{AlertOrphanKilled, SevWarn, "", "",
				fmt.Sprintf("%s orphaned with no running agent (TB_TICKET_ID %s); terminated, no ticket was failed", where, describeID(id)), ActionNone})
			continue
		}
		for _, t := range ids {
			mark(t)
		}
		out = append(out, pendingAlert{AlertAgentEscaped, SevError, "", "",
			fmt.Sprintf("%s escaped and was terminated, but is unattributable (TB_TICKET_ID %s); failing the running tickets: %s", where, describeID(id), strings.Join(ids, ", ")), ActionEscalate})
	}
	return out
}

func describeID(id string) string {
	if id == "" {
		return "missing"
	}
	return "unknown: " + id
}

// sweepTick sweeps orphans that escaped during execution (watchdog, main
// goroutine) and emits the alerts immediately.
func (r *Router) sweepTick() {
	alerts := r.collectEscapes()
	for i := range alerts {
		r.emit(&alerts[i])
	}
}
