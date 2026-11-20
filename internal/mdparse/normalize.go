package mdparse

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeForSearch lowercases s and strips combining marks after NFD, so
// "Ação" matches "acao" (R13).
func NormalizeForSearch(s string) string {
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
