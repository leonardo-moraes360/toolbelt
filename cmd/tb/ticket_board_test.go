package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func runTB(t *testing.T, bin, dir string, args ...string) (string, error) {
	t.Helper()
	c := exec.Command(bin, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	return string(out), err
}

func TestTicketBoardCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	dir := t.TempDir()
	must := func(args ...string) string {
		t.Helper()
		out, err := runTB(t, bin, dir, args...)
		if err != nil {
			t.Fatalf("tb %v: %v\n%s", args, err, out)
		}
		return out
	}
	must("ticket", "create", "--title", "base", "--priority", "1", "--timeout", "5m")
	must("ticket", "create", "--title", "dependent", "--dep", "TKT-001")
	must("ticket", "create", "--title", "livre", "--priority", "5")

	if out, err := runTB(t, bin, dir, "ticket", "create", "--title", "x", "--dep", "TKT-042"); err == nil || !strings.Contains(out, "unknown") {
		t.Fatalf("dep desconhecida deveria falhar: %v %s", err, out)
	}
	must("ticket", "create", "--title", "seco", "--dry-run")

	var list []map[string]any
	if err := json.Unmarshal([]byte(must("ticket", "list", "--json")), &list); err != nil || len(list) != 3 {
		t.Fatalf("list: %v %d", err, len(list))
	}
	if out := must("ticket", "show", "TKT-002"); !strings.Contains(out, "TKT-001") {
		t.Fatalf("show: %s", out)
	}

	var view map[string]any
	if err := json.Unmarshal([]byte(must("board", "list", "--json", "--strategy", "priority")), &view); err != nil {
		t.Fatal(err)
	}
	ready := view["ready"].([]any)
	if len(ready) != 2 || ready[0].(map[string]any)["id"] != "TKT-003" {
		t.Fatalf("ready = %v", ready)
	}
	if len(view["blocked"].([]any)) != 1 {
		t.Fatalf("blocked = %v", view["blocked"])
	}

	must("board", "pause")
	if out := must("board", "list"); !strings.Contains(out, "paused") {
		t.Fatalf("board: %s", out)
	}
	must("board", "resume")
	must("board", "drain")
	must("board", "resume")

	must("ticket", "cancel", "TKT-003")
	if out, err := runTB(t, bin, dir, "ticket", "cancel", "TKT-003"); err == nil {
		t.Fatalf("cancelar de novo deveria falhar: %s", out)
	}
	must("ticket", "retry", "TKT-003")
}
