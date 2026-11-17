package router

import (
	"fmt"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// PlanAssignment é uma atribuição simulada.
type PlanAssignment struct {
	Worker string `json:"worker"`
	Ticket string `json:"ticket"`
	Title  string `json:"title"`
	Via    string `json:"via"`  // board | direct
	Base   string `json:"base"` // HEAD ou ID do pai (entrega encadeada)
}

// PlanRound agrupa o que roda "em paralelo" numa rodada.
type PlanRound struct {
	Round       int              `json:"round"`
	Assignments []PlanAssignment `json:"assignments"`
}

// PlanUnschedulable é um ticket pending que nunca ficaria pronto.
type PlanUnschedulable struct {
	Ticket  string   `json:"ticket"`
	Reasons []string `json:"reasons"`
}

// Plan é a saída de tb router run --dry-run (nada é executado).
type Plan struct {
	Strategy       string              `json:"strategy"`
	Workers        int                 `json:"workers"`
	DirectDelivery bool                `json:"direct_delivery"`
	Paused         bool                `json:"paused"`
	Draining       bool                `json:"draining"`
	Rounds         []PlanRound         `json:"rounds"`
	Unschedulable  []PlanUnschedulable `json:"unschedulable"`
	Note           string              `json:"note"`
}

func planReady(b *ticket.Board, status map[string]ticket.Status, strategy ticket.Strategy) []ticket.Ticket {
	var out []ticket.Ticket
	for i := range b.Tickets {
		t := &b.Tickets[i]
		if status[t.ID] != ticket.StatusPending {
			continue
		}
		ok := true
		for _, d := range t.Dependencies {
			if status[d] != ticket.StatusDone {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, *t)
		}
	}
	ticket.SortTickets(out, strategy, b.Tickets)
	return out
}

func planPickDirect(cands []ticket.Ticket, taken map[string]bool, lastWorker map[string]string, w string, maxSubs int) (pick *ticket.Ticket, via string) {
	held := 0
	for i := range cands {
		t := &cands[i]
		if taken[t.ID] || held >= maxSubs {
			continue
		}
		for _, d := range t.Dependencies {
			if lastWorker[d] == w {
				return t, ViaDirect
			}
		}
	}
	return nil, ViaBoard
}

func planPickBoard(cands []ticket.Ticket, taken map[string]bool) *ticket.Ticket {
	for i := range cands {
		if !taken[cands[i].ID] {
			return &cands[i]
		}
	}
	return nil
}

func planUnschedulable(b *ticket.Board, status map[string]ticket.Status, byID map[string]ticket.Ticket) []PlanUnschedulable {
	var out []PlanUnschedulable
	for i := range b.Tickets {
		t := &b.Tickets[i]
		if status[t.ID] != ticket.StatusPending {
			continue
		}
		var reasons []string
		for _, d := range t.Dependencies {
			if status[d] != ticket.StatusDone {
				if dep, ok := byID[d]; ok {
					reasons = append(reasons, fmt.Sprintf("%s: %s", d, dep.Status))
				} else {
					reasons = append(reasons, d+": inexistente")
				}
			}
		}
		out = append(out, PlanUnschedulable{Ticket: t.ID, Reasons: reasons})
	}
	return out
}

// BuildPlan simula o agendamento assumindo duração igual por ticket: a cada
// rodada cada worker pega um ticket; com entrega direta, o worker prefere um
// filho do que acabou de concluir (até maxSubs por worker).
func BuildPlan(b *ticket.Board, workers int, strategy ticket.Strategy, direct bool, maxSubs int) Plan {
	p := Plan{
		Strategy: string(strategy), Workers: workers, DirectDelivery: direct,
		Paused: b.Control.Paused, Draining: b.Control.Draining,
		Rounds: []PlanRound{}, Unschedulable: []PlanUnschedulable{},
		Note: "estimativa com duração igual por ticket; a ordem real depende de quando cada worker termina",
	}
	status := map[string]ticket.Status{}
	byID := map[string]ticket.Ticket{}
	pending := 0
	for i := range b.Tickets {
		t := &b.Tickets[i]
		status[t.ID] = t.Status
		byID[t.ID] = *t
		if t.Status == ticket.StatusPending {
			pending++
		}
	}
	lastWorker := map[string]string{}
	for round := 1; round <= pending+1; round++ {
		cands := planReady(b, status, strategy)
		if len(cands) == 0 {
			break
		}
		taken := map[string]bool{}
		var assigns []PlanAssignment
		ranThisRound := map[string]string{}
		for i := 1; i <= workers; i++ {
			w := fmt.Sprintf("w%d", i)
			var pick *ticket.Ticket
			via := ViaBoard
			if direct {
				pick, via = planPickDirect(cands, taken, lastWorker, w, maxSubs)
			}
			if pick == nil {
				pick = planPickBoard(cands, taken)
				via = ViaBoard
			}
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
		for id := range ranThisRound {
			status[id] = ticket.StatusDone
		}
		lastWorker = ranThisRound
		p.Rounds = append(p.Rounds, PlanRound{Round: round, Assignments: assigns})
	}
	p.Unschedulable = planUnschedulable(b, status, byID)
	return p
}

// Plan monta o plano a partir do board atual deste router.
func (r *Router) Plan() (Plan, error) {
	b, err := ticket.LoadBoard(r.store)
	if err != nil {
		return Plan{}, err
	}
	return BuildPlan(b, r.workers, r.strategy, r.cfg.DirectDelivery.Enabled, r.cfg.DirectDelivery.MaxSubscriptionsPerWorker), nil
}
