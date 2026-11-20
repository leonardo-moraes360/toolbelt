package lab

import (
	"os"
	"testing"
)

func TestScaffold(t *testing.T) {
	root := tempRepo(t)
	path, err := Scaffold(root, "demo", "sql-injection", "")
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	// What was written must load back and resolve to the sandbox tier.
	l, err := Load(root, "demo")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tier, _ := ResolveTier(l, ""); tier != TierSandbox {
		t.Errorf("scaffolded lab resolves to %q, want sandbox", tier)
	}
}

func TestScaffoldRefusesOverwrite(t *testing.T) {
	root := tempRepo(t)
	if _, err := Scaffold(root, "demo", "sql-injection", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, "demo", "sql-injection", ""); err == nil {
		t.Error("Scaffold should refuse to overwrite an existing lab")
	}
}

func TestScaffoldRejectsBadInput(t *testing.T) {
	root := tempRepo(t)
	if _, err := Scaffold(root, "Bad Name", "sql-injection", ""); err == nil {
		t.Error("bad name should error")
	}
	if _, err := Scaffold(root, "ok", "", ""); err == nil {
		t.Error("empty class should error")
	}
}
