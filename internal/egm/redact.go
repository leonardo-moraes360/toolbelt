package egm

import "github.com/leonardo-moraes360/toolbelt/internal/secrets"

// Redact replaces each secret with [REDACTED] plus the excerpt's line breaks
// so line numbers stay valid (R8); found feeds secret_like (R7).
func Redact(s string) (out string, found bool) { return secrets.Redact(s) }
