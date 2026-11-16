package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemUnknownSubcommandExitsNonZero(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	cmd := exec.Command(tbBin, "mem", "naoexiste")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("unknown subcommand deveria sair ≠ 0\n%s", out)
	}
	if !strings.Contains(string(out), `unknown command "naoexiste"`) {
		t.Fatalf("esperava erro unknown command, got: %s", out)
	}
}

func TestMemShowAndEditCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := memFixtureDir(t)
	id := memAddJSON(t, tbBin, dir, "--category", "mandate", "--ref", "docs/adr/0001.md", "--json", "vincular ADR")

	out, err := runMem(tbBin, dir, "show", id)
	if err != nil {
		t.Fatalf("show: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "docs/adr/0001.md") {
		t.Fatalf("show sem ref: %s", out)
	}

	out, err = runMem(tbBin, dir, "edit", id, "--rule", "vincular ADR e decisions", "--ref", "docs/decisions/auth.md")
	if err != nil {
		t.Fatalf("edit: %v\n%s", err, out)
	}
	out, err = runMem(tbBin, dir, "search", "decisions")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), id) {
		t.Fatalf("search miss: %s", out)
	}
}

func TestMemRetireExcludedFromListCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := memFixtureDir(t)
	id := memAddJSON(t, tbBin, dir, "--json", "regra")

	if _, err := runMem(tbBin, dir, "retire", id); err != nil {
		t.Fatalf("retire: %v", err)
	}
	out, err := runMem(tbBin, dir, "list")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if strings.Contains(string(out), id) {
		t.Fatalf("list default inclui retired: %s", out)
	}
	out, err = runMem(tbBin, dir, "list", "--all")
	if err != nil {
		t.Fatalf("list --all: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), id) || !strings.Contains(string(out), "[retired]") {
		t.Fatalf("list --all sem retired: %s", out)
	}
}

func memFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func memAddJSON(t *testing.T, tbBin, dir string, args ...string) string {
	t.Helper()
	out, err := runMem(tbBin, dir, append([]string{"add"}, args...)...)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	var inv map[string]any
	if err := json.Unmarshal(out, &inv); err != nil {
		t.Fatal(err)
	}
	id, _ := inv["id"].(string)
	if id == "" {
		t.Fatalf("id ausente: %s", out)
	}
	return id
}

func runMem(tbBin, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(tbBin, append([]string{"mem"}, args...)...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
