package registry

import (
	"context"
	"fmt"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func init() {
	registerTicketTools()
}

func ticketStoreFromArgs(args map[string]any) (*ticket.Store, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	return ticket.Open(ticket.FindRoot(cwd)), nil
}

func registerTicketTools() {
	cwdProp := Property{Type: "string", Description: "Diretório do repositório"}
	Register(&Tool{
		Name:        "tb_ticket_list",
		Description: "Lista tickets de .toolbelt/tickets (paridade com tb ticket list)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"status": {Type: "string", Description: "Filtro por status, separado por vírgula (pending,running,done,failed,cancelled)"},
				"deps":   {Type: "string", Description: "Só tickets que dependem deste ID"},
				"cwd":    cwdProp,
			},
		},
		Handler: handleTicketList,
	})
	Register(&Tool{
		Name:        "tb_ticket_show",
		Description: "Mostra um ticket (paridade com tb ticket show)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":  {Type: "string", Description: "ID do ticket (TKT-001)"},
				"cwd": cwdProp,
			},
			Required: []string{"id"},
		},
		Handler: handleTicketShow,
	})
	Register(&Tool{
		Name:        "tb_ticket_create",
		Description: "Cria ticket (paridade com tb ticket create; dependências desconhecidas e ciclos são rejeitados)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"title":       {Type: "string", Description: "Título do ticket"},
				"description": {Type: "string", Description: "Descrição"},
				"scope":       {Type: "array", Items: &Property{Type: "string"}, Description: "Paths permitidos"},
				"forbid":      {Type: "array", Items: &Property{Type: "string"}, Description: "Paths proibidos"},
				"accept":      {Type: "array", Items: &Property{Type: "string"}, Description: "Critérios de aceite"},
				"dep":         {Type: "array", Items: &Property{Type: "string"}, Description: "IDs de dependências"},
				"priority":    {Type: "integer", Description: "Prioridade (maior primeiro)", Default: 0},
				"timeout":     {Type: "string", Description: "Timeout (ex.: 10m)"},
				"dry_run":     {Type: "boolean", Description: "Valida sem gravar", Default: false},
				"cwd":         cwdProp,
			},
			Required: []string{"title"},
		},
		Handler: handleTicketCreate,
	})
	Register(&Tool{
		Name:        "tb_board",
		Description: "Mostra o board (RUNNING/READY/BLOCKED/DONE/FAILED) e controla pause/resume/drain (paridade com tb board)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"action":   {Type: "string", Description: "list (padrão)|pause|resume|drain", Default: "list"},
				"strategy": {Type: "string", Description: "fifo|priority|dependency", Default: "dependency"},
				"status":   {Type: "string", Description: "Filtro por status, separado por vírgula"},
				"worker":   {Type: "string", Description: "Filtra por worker"},
				"cwd":      cwdProp,
			},
		},
		Handler: handleBoard,
	})
}

func handleTicketList(_ context.Context, args map[string]any) (any, error) {
	s, err := ticketStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	sts, err := parseStatusCSV(StringArg(args, "status"))
	if err != nil {
		return nil, err
	}
	list, err := s.List(ticket.Filter{Statuses: sts, DependsOn: StringArg(args, "deps")})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []ticket.Ticket{}
	}
	return list, nil
}

func handleTicketShow(_ context.Context, args map[string]any) (any, error) {
	s, err := ticketStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	return s.Get(StringArg(args, "id"))
}

func handleTicketCreate(_ context.Context, args map[string]any) (any, error) {
	s, err := ticketStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	t := ticket.Ticket{
		Title:        StringArg(args, "title"),
		Description:  StringArg(args, "description"),
		Priority:     IntArg(args, "priority", 0),
		Dependencies: StringSliceArg(args, "dep", nil),
	}
	t.Mandate.Scope = StringSliceArg(args, "scope", nil)
	t.Mandate.Forbidden = StringSliceArg(args, "forbid", nil)
	t.Acceptance.Criteria = StringSliceArg(args, "accept", nil)
	t.Acceptance.Timeout = StringArg(args, "timeout")
	if BoolArg(args, "dry_run", false) {
		if err := s.CheckNew(t); err != nil {
			return nil, err
		}
		return t, nil
	}
	return s.Create(t)
}

func handleBoard(_ context.Context, args map[string]any) (any, error) {
	s, err := ticketStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	switch action := StringArg(args, "action"); action {
	case "", "list":
		st, err := ticket.ParseStrategy(StringArg(args, "strategy"))
		if err != nil {
			return nil, err
		}
		sts, err := parseStatusCSV(StringArg(args, "status"))
		if err != nil {
			return nil, err
		}
		b, err := ticket.LoadBoard(s)
		if err != nil {
			return nil, err
		}
		return b.View(st, sts, StringArg(args, "worker")), nil
	case "pause":
		return ticket.Pause(s.Root())
	case "resume":
		return ticket.Resume(s.Root())
	case "drain":
		return ticket.Drain(s.Root())
	default:
		return nil, fmt.Errorf("action inválida %q (use list, pause, resume ou drain)", action)
	}
}

func parseStatusCSV(csv string) ([]ticket.Status, error) {
	var out []ticket.Status
	for _, p := range splitCSVArg(csv) {
		st, err := ticket.ParseStatus(p)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}
