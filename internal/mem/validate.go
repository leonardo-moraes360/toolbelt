package mem

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

var ValidCategories = []string{"anti_cheat", "mandate", "security"}

const validRefHelp = "docs/adr/<file>.md, docs/decisions/<file>.md"

func ValidateCategory(category string) error {
	category = strings.TrimSpace(category)
	if category == "" {
		return fmt.Errorf("empty category; valid values: %s", strings.Join(ValidCategories, ", "))
	}
	for _, c := range ValidCategories {
		if category == c {
			return nil
		}
	}
	return fmt.Errorf("invalid category %q; valid values: %s", category, strings.Join(ValidCategories, ", "))
}

func ValidateRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if filepath.IsAbs(ref) {
		return fmt.Errorf("invalid ref %q; use a relative .md path (e.g. %s)", ref, validRefHelp)
	}
	clean := filepath.ToSlash(filepath.Clean(ref))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return fmt.Errorf("invalid ref %q; use a relative .md path (e.g. %s)", ref, validRefHelp)
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".md") {
		return fmt.Errorf("invalid ref %q; valid values: %s", ref, validRefHelp)
	}
	if !refLooksLikeADROrDecisions(clean) {
		return fmt.Errorf("invalid ref %q; valid values: %s", ref, validRefHelp)
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

func normalizeForSearch(s string) string { return mdparse.NormalizeForSearch(s) }
