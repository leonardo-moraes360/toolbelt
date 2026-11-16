package mem

import (
	"path/filepath"
	"testing"
)

func TestAddListCheck(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("anti_cheat", "NUNCA apague testes", "")
	if err != nil || inv.ID == "" {
		t.Fatal(err)
	}
	list, err := s.List("", false)
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	rep, err := s.Check("", []string{inv.ID}, false)
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
	_, _ = s.Add("security", "regra", "")
	rep, err := s.Check("", []string{"inv-missing"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Missing) != 1 {
		t.Fatal("esperava missing")
	}
}

func TestGetAndEditWithRef(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("mandate", "usar ADR", "docs/adr/0001.md")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ref != "docs/adr/0001.md" || got.Rule != "usar ADR" {
		t.Fatalf("%+v", got)
	}

	rule := "usar ADR e decisions.md"
	cat := "security"
	ref := "docs/decisions/auth.md"
	edited, err := s.Edit(inv.ID, EditFields{Rule: &rule, Category: &cat, Ref: &ref})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Rule != rule || edited.Category != cat || edited.Ref != ref {
		t.Fatalf("%+v", edited)
	}

	hits, err := s.Search("decisions", false)
	if err != nil || len(hits) != 1 {
		t.Fatalf("search: %v %+v", err, hits)
	}
}

func TestRetireExcludedFromList(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("mandate", "usar ADR", "docs/adr/0001.md")
	if err != nil {
		t.Fatal(err)
	}
	retired, err := s.Retire(inv.ID)
	if err != nil || !retired.Retired || retired.RetiredAt == nil {
		t.Fatalf("retire: %+v err=%v", retired, err)
	}
	list, err := s.List("", false)
	if err != nil || len(list) != 0 {
		t.Fatalf("list default deveria omitir retired: %v %+v", err, list)
	}
	all, err := s.List("", true)
	if err != nil || len(all) != 1 {
		t.Fatalf("list --all: %v %+v", err, all)
	}
}

func TestRetireExcludedFromSearchAndCheck(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("mandate", "usar ADR", "docs/adr/0001.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Retire(inv.ID); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search("ADR", false)
	if err != nil || len(hits) != 0 {
		t.Fatalf("search default omite retired: %v %+v", err, hits)
	}
	hits, err = s.Search("ADR", true)
	if err != nil || len(hits) != 1 {
		t.Fatalf("search --all: %v %+v", err, hits)
	}
	rep, err := s.Check("", nil, false)
	if err != nil || rep.Count != 0 {
		t.Fatalf("check default: %+v err=%v", rep, err)
	}
	rep, err = s.Check("", nil, true)
	if err != nil || rep.Count != 1 {
		t.Fatalf("check --all: %+v err=%v", rep, err)
	}
}

func TestEditRequiresFields(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("anti_cheat", "x", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Edit(inv.ID, EditFields{}); err == nil {
		t.Fatal("edit sem campos deveria falhar")
	}
	if _, err := s.Get("missing"); err == nil {
		t.Fatal("get missing deveria falhar")
	}
}
