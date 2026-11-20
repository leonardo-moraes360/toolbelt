package router

import (
	"fmt"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

type PlanAssignment struct {
	Worker string `json:"worker"`
	Ticket string `json:"ticket"`
	Title  string `json:"title"`
	Via    string `json:"via"`  // board | direct
	Base   string `json:"base"` // HEAD or parent ID (chained delivery)
}

type PlanRound struct {
	Round       int              `json:"round"`
	Assignments []PlanAssignment `json:"assignments"`
}

// PlanUnschedulable is a pending ticket that would never become ready.
type PlanUnschedulable struct {
	Ticket  string   `json:"ticket"`
	Reasons []string `json:"reasons"`
}

// Plan is the output of tb router run --dry-run (nothing is executed).
type Plan struct {
	Strategy       string              `json:"strategy"`
	Workers        int                 `json:"workers"`
	BusSlots       int                 `json:"bus_slots,omitempty"`
	DirectDelivery bool                `json:"direct_delivery"`
	Paused         bool                `json:"paused"`
	Draining       bool                `json:"draining"`
	Rounds         []PlanRound         `json:"rounds"`
	Unschedulable  []PlanUnschedulable `json:"unschedulable"`
	Note           string              `json:"note"`
}

// BuildPlan simulates scheduling assuming equal duration per ticket: each
// round every worker takes one ticket; with direct delivery, a worker prefers a
// dependent of the ticket it just finished (up to maxSubs per worker).
func BuildPlan(b *ticket.Board, workers int, strategy ticket.Strategy, direct bool, maxSubs int) Plan {
	return buildPlan(b, workers, 0, strategy, direct, maxSubs)
}

func buildPlan(b *ticket.Board, workers, busSlots int, strategy ticket.Strategy, direct bool, maxSubs int) Plan {
	p := Plan{
		Strategy: string(strategy), Workers: workers, BusSlots: busSlots, DirectDelivery: direct,
		Paused: b.Control.Paused, Draining: b.Control.Draining,
		Rounds: []PlanRound{}, Unschedulable: []PlanUnschedulable{},
		Note: "estimate assuming equal duration per ticket; the actual order depends on when each worker finishes",
	}
	pl := &planner{b: b, workers: workers, busSlots: busSlots, strategy: strategy, direct: direct, maxSubs: maxSubs,
		status: map[string]ticket.Status{}, byID: map[string]*ticket.Ticket{}}
	pending := 0
	for i := range b.Tickets {
		t := &b.Tickets[i]
		pl.status[t.ID] = t.Status
		pl.byID[t.ID] = t
		if t.Status == ticket.StatusPending {
			pending++
		}
	}
	lastWorker := map[string]string{} // ticket finished in the previous round -> worker
	for round := 1; round <= pending+1; round++ {
		cands := pl.ready()
		if len(cands) == 0 {
			break
		}
		assigns, ranThisRound := pl.assignRound(cands, lastWorker)
		for id := range ranThisRound {
			pl.status[id] = ticket.StatusDone
		}
		lastWorker = ranThisRound
		p.Rounds = append(p.Rounds, PlanRound{Round: round, Assignments: assigns})
	}
	p.Unschedulable = append(p.Unschedulable, pl.unschedulable()...)
	return p
}

type planner struct {
	b        *ticket.Board
	workers  int
	busSlots int
	strategy ticket.Strategy
	direct   bool
	maxSubs  int
	status   map[string]ticket.Status
	byID     map[string]*ticket.Ticket
}

func (pl *planner) ready() []ticket.Ticket {
	var out []ticket.Ticket
	for i := range pl.b.Tickets {
		t := &pl.b.Tickets[i]
		if pl.status[t.ID] != ticket.StatusPending {
			continue
		}
		ok := true
		for _, d := range t.Dependencies {
			if pl.status[d] != ticket.StatusDone {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, *t)
		}
	}
	ticket.SortTickets(out, pl.strategy, pl.b.Tickets)
	return out
}

// assignRound assigns one ticket per worker; returns the assignments and a
// ticket -> worker map.
func (pl *planner) assignRound(cands []ticket.Ticket, lastWorker map[string]string) (assigns []PlanAssignment, ranThisRound map[string]string) {
	taken := map[string]bool{}
	ranThisRound = map[string]string{}
	for _, w := range pl.slotIDs() {
		pick, via := pl.pickFor(w, cands, taken, lastWorker)
		if pick == nil {
			continue
		}
		taken[pick.ID] = true
		base := "HEAD"
		if len(pick.Dependencies) == 1 {
			base = pick.Dependencies[0]
		}
		assigns = append(assigns, PlanAssignment{Worker: w, Ticket: pick.ID, Title: pick.Title, Via: via, Base: base})
		ranThisRound[pick.ID] = w
	}
	return assigns, ranThisRound
}

// pickFor: with direct delivery, prefer a dependent of the ticket w just
// finished; otherwise the first free candidate.
func (pl *planner) pickFor(w string, cands []ticket.Ticket, taken map[string]bool, lastWorker map[string]string) (pick *ticket.Ticket, via string) {
	if pl.direct {
		for i := range cands {
			t := &cands[i]
			if taken[t.ID] || pl.maxSubs <= 0 {
				continue
			}
			for _, d := range t.Dependencies {
				if lastWorker[d] == w {
					return t, ViaDirect
				}
			}
		}
	}
	for i := range cands {
		if !taken[cands[i].ID] {
			return &cands[i], ViaBoard
		}
	}
	return nil, ViaBoard
}

// unschedulable lists pending tickets that never became ready, with reasons.
func (pl *planner) unschedulable() []PlanUnschedulable {
	var out []PlanUnschedulable
	for i := range pl.b.Tickets {
		t := &pl.b.Tickets[i]
		if pl.status[t.ID] != ticket.StatusPending {
			continue
		}
		var reasons []string
		for _, d := range t.Dependencies {
			if pl.status[d] == ticket.StatusDone {
				continue
			}
			if dep, ok := pl.byID[d]; ok {
				reasons = append(reasons, fmt.Sprintf("%s: %s", d, dep.Status))
			} else {
				reasons = append(reasons, d+": does not exist")
			}
		}
		out = append(out, PlanUnschedulable{Ticket: t.ID, Reasons: reasons})
	}
	return out
}

func (r *Router) Plan() (Plan, error) {
	b, err := ticket.LoadBoard(r.store)
	if err != nil {
		return Plan{}, err
	}
	return buildPlan(b, r.workers, len(r.slots)-r.workers, r.strategy, r.cfg.DirectDelivery.Enabled, r.cfg.DirectDelivery.MaxSubscriptionsPerWorker), nil
}

// slotIDs: local slots first, then bus slots, as the router fills them.
func (pl *planner) slotIDs() []string {
	ids := make([]string, 0, pl.workers+pl.busSlots)
	for i := 1; i <= pl.workers; i++ {
		ids = append(ids, fmt.Sprintf("w%d", i))
	}
	for i := 1; i <= pl.busSlots; i++ {
		ids = append(ids, fmt.Sprintf("b%d", i))
	}
	return ids
}
