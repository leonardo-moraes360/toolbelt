package egm

import (
	"regexp"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

var redactPatterns = append(mem.SecretPatterns(),
	regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{32,}`),
	regexp.MustCompile(`(?i)\b[A-Za-z0-9_]*_(KEY|SECRET|PASSWORD)=[^\s"'&]+`),
)

// Redact replaces each secret with [REDACTED] followed by as many line breaks
// as the original excerpt had, so line numbers stay valid (R8). found reports
// whether anything was replaced (secret_like, R7).
func Redact(s string) (out string, found bool) {
	out = s
	for _, re := range redactPatterns {
		out = re.ReplaceAllStringFunc(out, func(m string) string {
			found = true
			return "[REDACTED]" + strings.Repeat("\n", strings.Count(m, "\n"))
		})
	}
	return out, found
}
