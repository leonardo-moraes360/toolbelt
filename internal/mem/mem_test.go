package mem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddListCheck(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("anti_cheat", "NUNCA apague testes", "", false)
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
	_, _ = s.Add("security", "regra", "", false)
	rep, err := s.Check("", []string{"inv-missing"}, false)
	if err == nil {
		t.Fatal("check com id ausente deveria falhar fechado")
	}
	if rep == nil || len(rep.Missing) != 1 {
		t.Fatalf("esperava missing no report: %+v", rep)
	}
}

func TestGetAndEditWithRef(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("mandate", "usar ADR", "docs/adr/0001.md", false)
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
	edited, err := s.Edit(inv.ID, EditFields{Rule: &rule, Category: &cat, Ref: &ref}, MutateOpts{})
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
	inv, err := s.Add("mandate", "usar ADR", "docs/adr/0001.md", false)
	if err != nil {
		t.Fatal(err)
	}
	retired, err := s.Retire(inv.ID, MutateOpts{})
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
	inv, err := s.Add("mandate", "usar ADR", "docs/adr/0001.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Retire(inv.ID, MutateOpts{}); err != nil {
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
	inv, err := s.Add("anti_cheat", "x", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Edit(inv.ID, EditFields{}, MutateOpts{}); err == nil {
		t.Fatal("edit sem campos deveria falhar")
	}
	if _, err := s.Get("missing"); err == nil {
		t.Fatal("get missing deveria falhar")
	}
}

func TestImmutableRequiresForceAndReason(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("mandate", "regra imutável", "", true)
	if err != nil {
		t.Fatal(err)
	}
	rule := "alterada"
	if _, err := s.Edit(inv.ID, EditFields{Rule: &rule}, MutateOpts{}); err == nil {
		t.Fatal("edit sem --force deveria falhar")
	}
	if _, err := s.Edit(inv.ID, EditFields{Rule: &rule}, MutateOpts{Force: true}); err == nil {
		t.Fatal("edit com --force sem --reason deveria falhar")
	}
	if _, err := s.Retire(inv.ID, MutateOpts{}); err == nil {
		t.Fatal("retire sem --force deveria falhar")
	}
	edited, err := s.Edit(inv.ID, EditFields{Rule: &rule}, MutateOpts{Force: true, Reason: "incidente", Who: "tester"})
	if err != nil || edited.Rule != rule {
		t.Fatalf("edit forçado: %+v err=%v", edited, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, DefaultHistoryFile))
	if err != nil {
		t.Fatal(err)
	}
	var entry HistoryEntry
	if err := json.Unmarshal(data[:len(data)-1], &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Action != "edit" || entry.Who != "tester" || entry.Reason != "incidente" || entry.OldValue == nil || entry.OldValue.Rule != "regra imutável" {
		t.Fatalf("histórico: %+v", entry)
	}
	if _, err := s.Retire(inv.ID, MutateOpts{Force: true, Reason: "aposentar"}); err != nil {
		t.Fatal(err)
	}
}

func TestAntiCheatRequiresForce(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("anti_cheat", "não trapacear", "", false)
	if err != nil {
		t.Fatal(err)
	}
	rule := "trapacear"
	if _, err := s.Edit(inv.ID, EditFields{Rule: &rule}, MutateOpts{}); err == nil {
		t.Fatal("anti_cheat deveria exigir --force")
	}
}

func TestAddDefaultMutable(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("security", "regra nova", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Immutable {
		t.Fatal("nova invariante deve nascer mutável por default")
	}
	flagged, err := s.Add("security", "fixada", "", true)
	if err != nil || !flagged.Immutable {
		t.Fatalf("com immutable=true deveria marcar: %+v err=%v", flagged, err)
	}
}

func TestCheckFailClosedCorruptAndUnevaluable(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := os.WriteFile(s.Path(), []byte("not: yaml: [["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Check("", nil, false); err == nil {
		t.Fatal("yaml corrompido deveria falhar fechado")
	}

	missing := NewStore(filepath.Join(dir, "missing-dir"))
	if _, err := missing.Check("", nil, false); err == nil {
		t.Fatal("arquivo ausente deveria falhar fechado")
	}

	good := NewStore(filepath.Join(dir, "good"))
	if err := os.MkdirAll(filepath.Join(dir, "good"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("version: 1\ninvariants:\n  - id: inv-bad\n    category: anti_cheat\n    rule: \"\"\n    immutable: true\n")
	if err := os.WriteFile(good.Path(), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := good.Check("", nil, false)
	if err == nil {
		t.Fatal("regra vazia deveria falhar fechado")
	}
	if rep == nil || len(rep.Errors) == 0 {
		t.Fatalf("esperava errors no report: %+v", rep)
	}
}

func TestCheckFailClosedWrongSHA(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	inv, err := s.Add("security", "ok", "", false)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := s.Check("deadbeef", []string{inv.ID}, false)
	if err == nil {
		t.Fatal("sha divergente deveria falhar fechado")
	}
	if rep == nil || rep.CompactSafe {
		t.Fatalf("esperava compact_safe=false: %+v", rep)
	}
}

func TestRedactSecrets(t *testing.T) {
	in := "token ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA aws AKIAIOSFODNN7EXAMPLE Bearer abc.def xoxb-123-secret API_TOKEN=supersecret"
	out := RedactText(in)
	for _, leak := range []string{
		"ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"AKIAIOSFODNN7EXAMPLE",
		"Bearer abc.def",
		"xoxb-123-secret",
		"API_TOKEN=supersecret",
	} {
		if strings.Contains(out, leak) {
			t.Fatalf("vazou %q em %q", leak, out)
		}
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("esperava placeholder: %q", out)
	}
	pem := "-----BEGIN RSA PRIVATE KEY-----\nABC\n-----END RSA PRIVATE KEY-----"
	if RedactText(pem) == pem {
		t.Fatal("PEM deveria ser redigido")
	}
	src := Invariant{Rule: "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
	inv := RedactInvariant(&src)
	if strings.Contains(inv.Rule, "ghp_AAAA") {
		t.Fatalf("RedactInvariant: %q", inv.Rule)
	}
}
