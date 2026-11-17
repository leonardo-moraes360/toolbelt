package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesGitExclude(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README")
	git("commit", "-m", "init")

	cmd := exec.Command(tbBin, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	exclude := filepath.Join(dir, ".git", "info", "exclude")
	data, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, ".toolbelt/graph.json") || !strings.Contains(text, ".toolbelt/logs/") {
		t.Fatalf("exclude incompleto:\n%s", text)
	}

	if err := os.WriteFile(filepath.Join(dir, ".toolbelt", "graph.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".toolbelt", "invariants.yaml"), []byte("version: 1\ninvariants: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check := exec.Command("git", "check-ignore", "-v", ".toolbelt/graph.json")
	check.Dir = dir
	cout, _ := check.CombinedOutput()
	if !strings.Contains(string(cout), "graph.json") {
		t.Fatalf("graph não ignorado: %s", cout)
	}
	check2 := exec.Command("git", "check-ignore", "-v", ".toolbelt/invariants.yaml")
	check2.Dir = dir
	cout2, err2 := check2.CombinedOutput()
	if err2 == nil && strings.Contains(string(cout2), "invariants.yaml") {
		t.Fatalf("invariants não deveria ser ignorado: %s", cout2)
	}
}
