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
	cwdProp := Property{Type: "string", Description: "Repository directory"}
	Register(&Tool{
		Name:        "tb_ticket_list",
		Description: "Lists tickets from .toolbelt/tickets (parity with tb ticket list)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"status": {Type: "string", Description: "Status filter, comma-separated (pending,running,done,failed,canceled)"},
				"deps":   {Type: "string", Description: "Only tickets that depend on this ID"},
				"cwd":    cwdProp,
			},
		},
		Handler: handleTicketList,
	})
	Register(&Tool{
		Name:        "tb_ticket_show",
		Description: "Shows a ticket (parity with tb ticket show)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":  {Type: "string", Description: "Ticket ID (TKT-001)"},
				"cwd": cwdProp,
			},
			Required: []string{"id"},
		},
		Handler: handleTicketShow,
	})
	Register(&Tool{
		Name:        "tb_ticket_create",
		Description: "Creates a ticket (parity with tb ticket create; unknown dependencies and cycles are rejected)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"title":       {Type: "string", Description: "Ticket title"},
				"description": {Type: "string", Description: "Description"},
				"scope":       {Type: "array", Items: &Property{Type: "string"}, Description: "Allowed paths"},
				"forbid":      {Type: "array", Items: &Property{Type: "string"}, Description: "Forbidden paths"},
				"accept":      {Type: "array", Items: &Property{Type: "string"}, Description: "Acceptance criteria"},
				"dep":         {Type: "array", Items: &Property{Type: "string"}, Description: "Dependency IDs"},
				"priority":    {Type: "integer", Description: "Priority (highest first)", Default: 0},
				"timeout":     {Type: "string", Description: "Timeout (ex.: 10m)"},
				"dry_run":     {Type: "boolean", Description: "Validates without writing", Default: false},
				"cwd":         cwdProp,
			},
			Required: []string{"title"},
		},
		Handler: handleTicketCreate,
	})
	Register(&Tool{
		Name:        "tb_board",
		Description: "Shows the board (RUNNING/READY/BLOCKED/DONE/FAILED) and controls pause/resume/drain (parity with tb board)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"action":   {Type: "string", Description: "list (default)|pause|resume|drain", Default: "list"},
				"strategy": {Type: "string", Description: "fifo|priority|dependency", Default: "dependency"},
				"status":   {Type: "string", Description: "Status filter, comma-separated"},
				"worker":   {Type: "string", Description: "Filters by worker"},
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
		if err := s.CheckNew(&t); err != nil {
			return nil, err
		}
		return t, nil
	}
	return s.Create(&t)
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
		return nil, fmt.Errorf("invalid action %q (use list, pause, resume or drain)", action)
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
