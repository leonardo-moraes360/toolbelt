package mem

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var ValidCategories = []string{"anti_cheat", "mandate", "security"}

const validRefHelp = "docs/adr/<arquivo>.md, docs/decisions/<arquivo>.md"

func ValidateCategory(category string) error {
	category = strings.TrimSpace(category)
	if category == "" {
		return fmt.Errorf("categoria vazia; valores válidos: %s", strings.Join(ValidCategories, ", "))
	}
	for _, c := range ValidCategories {
		if category == c {
			return nil
		}
	}
	return fmt.Errorf("categoria inválida %q; valores válidos: %s", category, strings.Join(ValidCategories, ", "))
}

func ValidateRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if filepath.IsAbs(ref) {
		return fmt.Errorf("ref inválido %q; use caminho relativo .md (ex: %s)", ref, validRefHelp)
	}
	clean := filepath.ToSlash(filepath.Clean(ref))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return fmt.Errorf("ref inválido %q; use caminho relativo .md (ex: %s)", ref, validRefHelp)
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".md") {
		return fmt.Errorf("ref inválido %q; valores válidos: %s", ref, validRefHelp)
	}
	if !refLooksLikeADROrDecisions(clean) {
		return fmt.Errorf("ref inválido %q; valores válidos: %s", ref, validRefHelp)
	}
	return nil
}

func refLooksLikeADROrDecisions(clean string) bool {
	base := strings.ToLower(filepath.Base(clean))
	if base == "decisions.md" {
		return true
	}
	parts := strings.Split(strings.ToLower(clean), "/")
	for _, p := range parts[:len(parts)-1] {
		if p == "adr" || p == "decisions" {
			return true
		}
	}
	return false
}

func normalizeForSearch(s string) string {
	s = norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
