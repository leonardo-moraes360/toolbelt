package router

import (
	"fmt"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// ticketIDVar é a variável de ambiente (herdada do agente) que atribui um
// processo escapado ao seu ticket. Só ela é lida de /proc/<pid>/environ.
const ticketIDVar = "TB_TICKET_ID"

// pendingAlert é um alerta decidido fora da goroutine principal (worker): o
// estado de alertas pertence à goroutine de Run, que o emite em complete.
type pendingAlert struct {
	kind, severity, worker, ticket, msg, action string
}

func (r *Router) emit(p *pendingAlert) {
	r.alert(p.kind, p.severity, p.worker, p.ticket, p.msg, p.action)
}

// markActive/unmarkActive registram os tickets cujo AGENTE está executando
// (entre o início do sandbox e o fim do execute). São os suspeitos de um escape
// não atribuível; o worker em createWorkspace ainda não tem agente.
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

// takeEscaped consome a marca de escape de um ticket (ver collectEscapes).
func (r *Router) takeEscaped(id string) bool {
	r.escapedMu.Lock()
	defer r.escapedMu.Unlock()
	ok := r.escapedTkts[id]
	delete(r.escapedTkts, id)
	return ok
}

// collectEscapes varre os órfãos (já mortos pelo Registry) e decide a quem
// atribuí-los. Seguro para qualquer goroutine; devolve alertas a emitir.
//
// Atribuição (nunca "o ticket cujo execute varreu"):
//   - benigno: nasceu numa janela de spawn do git do workspace manager (daemon de
//     hook/fsmonitor do usuário), sem TB_TICKET_ID e sem ter invadido grupo de
//     agente: morto, alerta orphan_killed (warn), nenhum ticket falha.
//   - TB_TICKET_ID de um ticket com agente rodando: falha esse ticket.
//   - TB_TICKET_ID ausente (env -u), ilegível ou desconhecido (finalizado,
//     forjado): não atribuível. Conservador: falham TODOS os tickets com agente
//     rodando no momento da detecção (alerta agent_escaped com a lista, motivo
//     "não atribuível"). Sem nenhum agente rodando: orphan_killed (warn).
//
// Limitação: um agente que forja TB_TICKET_ID de outro ticket ATIVO faz o
// escape ser atribuído a este (o ambiente é a única pista após o reparenting).
func (r *Router) collectEscapes() []pendingAlert {
	return r.classifyEscapes(procs.Sweep())
}

// classifyEscapes aplica a política de atribuição de collectEscapes a órfãos já
// varridos (separado para ser testável com órfãos sintéticos).
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
		where := fmt.Sprintf("processo %d", o.PID)
		if o.ForeignGroup {
			where += fmt.Sprintf(" (infiltrado no grupo do agente de %s)", o.GroupOwner)
		}
		if o.InSpawnWindow && id == "" && !o.ForeignGroup {
			out = append(out, pendingAlert{AlertOrphanKilled, SevWarn, "", "",
				fmt.Sprintf("%s nasceu durante a criação de worktree (daemon de hook/fsmonitor do usuário?) e sem TB_TICKET_ID; encerrado, nenhum ticket foi falhado", where), ActionNone})
			continue
		}
		if worker, ok := r.active[id]; ok && id != "" {
			mark(id)
			out = append(out, pendingAlert{AlertAgentEscaped, SevError, worker, id,
				fmt.Sprintf("%s escapou do grupo do agente de %s (setsid/setpgid) e foi encerrado; worktree não foi tocado", where, id), ActionEscalate})
			continue
		}
		ids := make([]string, 0, len(r.active))
		for t := range r.active {
			ids = append(ids, t)
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			out = append(out, pendingAlert{AlertOrphanKilled, SevWarn, "", "",
				fmt.Sprintf("%s órfão sem agente em execução (TB_TICKET_ID %s); encerrado, nenhum ticket foi falhado", where, describeID(id)), ActionNone})
			continue
		}
		for _, t := range ids {
			mark(t)
		}
		out = append(out, pendingAlert{AlertAgentEscaped, SevError, "", "",
			fmt.Sprintf("%s escapou e foi encerrado, mas não atribuível (TB_TICKET_ID %s); falhando os tickets em execução: %s", where, describeID(id), strings.Join(ids, ", ")), ActionEscalate})
	}
	return out
}

func describeID(id string) string {
	if id == "" {
		return "ausente"
	}
	return "desconhecido: " + id
}

// sweepTick varre órfãos escapados durante a execução (watchdog, goroutine
// principal) e emite os alertas na hora.
func (r *Router) sweepTick() {
	alerts := r.collectEscapes()
	for i := range alerts {
		r.emit(&alerts[i])
	}
}
