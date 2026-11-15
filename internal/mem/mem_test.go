package mem

import (
	"path/filepath"
	"testing"
)

func TestAddListCheck(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("anti_cheat", "NUNCA apague testes")
	if err != nil || inv.ID == "" {
		t.Fatal(err)
	}
	list, err := s.List("")
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	rep, err := s.Check("", []string{inv.ID})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Count != 1 || !rep.CompactSafe {
		t.Fatalf("%+v", rep)
	}
	if filepath.Base(s.Path()) != DefaultInvariantsFile {
		t.Fatal("path errado")
	}
}

func TestCheckMissingID(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	_, _ = s.Add("security", "regra")
	rep, err := s.Check("", []string{"inv-missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Missing) != 1 {
		t.Fatal("esperava missing")
	}
}
