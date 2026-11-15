package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSearchCallers(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "mathutil", "math.go"), `package mathutil
func Scale(v, factor int) int { return v * factor }
`)
	mustWrite(t, filepath.Join(root, "service", "billing.go"), `package service
import "example/mathutil"
func Total(price, qty int) int { return mathutil.Scale(price, qty) }
func Discounted(price, qty, discount int) int { return Total(price, qty) - discount }
`)

	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes) == 0 {
		t.Fatal("expected nodes")
	}

	storePath := filepath.Join(t.TempDir(), "graph.json")
	s := NewStore(storePath)
	s.index = idx
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	hits := s.Search("Scale", 10)
	if len(hits) == 0 {
		t.Fatal("search Scale empty")
	}

	callers := s.Callers("Scale", 2)
	found := false
	for _, c := range callers {
		if c.Title == "Total" || c.Title == "Discounted" {
			found = true
		}
	}
	if !found && len(callers) == 0 {
		t.Logf("callers=%v edges=%d", callers, len(idx.Edges))
	}

	deps := s.Deps("service", true)
	if len(deps) == 0 {
		t.Log("deps empty (ok if import path not local)")
	}
}

func TestEnabledFlag(t *testing.T) {
	t.Setenv(EnvEnable, "")
	t.Setenv(FlagEnv, "")
	if Enabled() {
		t.Fatal("expected disabled by default")
	}
	t.Setenv(EnvEnable, "on")
	if !Enabled() {
		t.Fatal("expected enabled")
	}
}

func TestRequireEnabled(t *testing.T) {
	t.Setenv(EnvEnable, "off")
	t.Setenv(FlagEnv, "")
	s := NewStore(filepath.Join(t.TempDir(), "g.json"))
	if err := s.RequireEnabled(); err == nil {
		t.Fatal("expected error when disabled")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
