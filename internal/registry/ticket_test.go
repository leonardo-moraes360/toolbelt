package registry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func callTool(t *testing.T, name string, args map[string]any) (result ToolResult, text string) {
	t.Helper()
	res, err := Default().Call(context.Background(), name, args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(res.Content) > 0 {
		text = res.Content[0].Text
	}
	return *res, text
}

func TestTicketToolsRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, tool := range Default().List() {
		names[tool.Name] = true
	}
	for _, n := range []string{"tb_ticket_list", "tb_ticket_show", "tb_ticket_create", "tb_board"} {
		if !names[n] {
			t.Errorf("tool %s não registrada", n)
		}
	}
}

func TestTicketToolsFlow(t *testing.T) {
	dir := t.TempDir()
	with := func(m map[string]any) map[string]any {
		m["cwd"] = dir
		return m
	}

	res, text := callTool(t, "tb_ticket_create", with(map[string]any{"title": "um", "priority": 2, "scope": []any{"internal/**"}}))
	if res.IsError {
		t.Fatal(text)
	}
	var a ticket.Ticket
	if err := json.Unmarshal([]byte(text), &a); err != nil || a.ID != "TKT-001" || a.Priority != 2 {
		t.Fatalf("create: %v %s", err, text)
	}
	res, text = callTool(t, "tb_ticket_create", with(map[string]any{"title": "dois", "dep": []any{"TKT-001"}}))
	if res.IsError {
		t.Fatal(text)
	}
	res, text = callTool(t, "tb_ticket_create", with(map[string]any{"title": "ruim", "dep": []any{"TKT-099"}}))
	if !res.IsError || !strings.Contains(text, "unknown dependency") {
		t.Fatalf("dep desconhecida deveria falhar: %v %s", res.IsError, text)
	}
	res, text = callTool(t, "tb_ticket_create", with(map[string]any{"title": "dry", "dry_run": true}))
	if res.IsError {
		t.Fatal(text)
	}
	checkTicketReads(t, dir)
}

// checkTicketReads checks list/show/board and the board control.
func checkTicketReads(t *testing.T, dir string) {
	t.Helper()
	cwd := map[string]any{"cwd": dir}
	with := func(m map[string]any) map[string]any {
		m["cwd"] = dir
		return m
	}
	_, text := callTool(t, "tb_ticket_list", cwd)
	var list []ticket.Ticket
	if err := json.Unmarshal([]byte(text), &list); err != nil || len(list) != 2 {
		t.Fatalf("list: %v %s", err, text)
	}
	_, text = callTool(t, "tb_ticket_show", with(map[string]any{"id": "TKT-002"}))
	var b ticket.Ticket
	if err := json.Unmarshal([]byte(text), &b); err != nil || b.Dependencies[0] != "TKT-001" {
		t.Fatalf("show: %v %s", err, text)
	}

	_, text = callTool(t, "tb_board", cwd)
	var v ticket.View
	if err := json.Unmarshal([]byte(text), &v); err != nil || len(v.Ready) != 1 || len(v.Blocked) != 1 {
		t.Fatalf("board: %v %s", err, text)
	}
	_, text = callTool(t, "tb_board", with(map[string]any{"action": "pause"}))
	var c ticket.Control
	if err := json.Unmarshal([]byte(text), &c); err != nil || !c.Paused {
		t.Fatalf("pause: %v %s", err, text)
	}
	res, _ := callTool(t, "tb_board", with(map[string]any{"action": "bogus"}))
	if !res.IsError {
		t.Fatal("action inválida deveria falhar")
	}
}
