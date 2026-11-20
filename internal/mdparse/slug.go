package mdparse

import (
	"strings"
	"unicode"
)

// Slugify lowercases s, keeps letters and digits, and collapses every other
// run of characters into one "-", trimmed at both ends. It returns "" when
// nothing is left; callers pick their own fallback.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
