package mem

import (
	"strings"
	"testing"
)

func TestValidateCategory(t *testing.T) {
	for _, cat := range ValidCategories {
		if err := ValidateCategory(cat); err != nil {
			t.Fatalf("%s: %v", cat, err)
		}
	}
	err := ValidateCategory("bogus")
	if err == nil {
		t.Fatal("esperava erro")
	}
	msg := err.Error()
	for _, part := range []string{"categoria inválida", "anti_cheat", "mandate", "security"} {
		if !strings.Contains(msg, part) {
			t.Fatalf("erro sem %q: %v", part, err)
		}
	}
}

func TestValidateRef(t *testing.T) {
	valid := []string{
		"",
		"docs/adr/0001.md",
		"docs/decisions/auth.md",
		"adr/foo.md",
		"path/to/decisions.md",
	}
	for _, ref := range valid {
		if err := ValidateRef(ref); err != nil {
			t.Fatalf("%q: %v", ref, err)
		}
	}
	invalid := []string{
		"/etc/passwd",
		"../secrets.md",
		"docs/readme.md",
		"docs/adr/note.txt",
	}
	for _, ref := range invalid {
		err := ValidateRef(ref)
		if err == nil {
			t.Fatalf("%q deveria falhar", ref)
		}
		msg := err.Error()
		if !strings.Contains(msg, "ref inválido") || !strings.Contains(msg, "docs/adr/") || !strings.Contains(msg, "docs/decisions/") {
			t.Fatalf("%q erro sem lista: %v", ref, err)
		}
	}
}

func TestNormalizeForSearchAccentInsensitive(t *testing.T) {
	if normalizeForSearch("Autenticação") != normalizeForSearch("autenticacao") {
		t.Fatalf("%q vs %q", normalizeForSearch("Autenticação"), normalizeForSearch("autenticacao"))
	}
	if normalizeForSearch("Não") != normalizeForSearch("nao") {
		t.Fatal("Não/nao")
	}
}
