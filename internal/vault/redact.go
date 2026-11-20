package vault

import (
	"regexp"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

const redacted = "[REDACTED]"

// patterns is the vault redaction (R15): mem's patterns plus the vault's
// own, kept here so tb mem and tb egm output do not change (G6).
var patterns = append(mem.SecretPatterns(),
	regexp.MustCompile(`ghu_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`ghs_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`(?i)\b(?:password|passwd|secret)\s*[=:]\s*[^\s"'&]+`),
)

// Redact applies the vault redaction to the whole of s.
func Redact(s string) string {
	for _, re := range patterns {
		s = re.ReplaceAllString(s, redacted)
	}
	return s
}

func redactAll(list []string) []string {
	if list == nil {
		return nil
	}
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = Redact(s)
	}
	return out
}
