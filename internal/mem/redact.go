package mem

import (
	"regexp"
)

const redactedPlaceholder = "[REDACTED]"

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`gho_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`-----BEGIN[^-]+PRIVATE KEY-----[\s\S]*?-----END[^-]+PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-+=/]+`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]+`),
	regexp.MustCompile(`(?i)[A-Za-z0-9_]*_TOKEN=[^\s"'&]+`),
}

// SecretPatterns returns a copy of the patterns RedactText uses, for callers
// that redact with their own replacement (egm keeps line numbering).
func SecretPatterns() []*regexp.Regexp {
	return append([]*regexp.Regexp(nil), secretPatterns...)
}

func RedactText(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, re := range secretPatterns {
		out = re.ReplaceAllString(out, redactedPlaceholder)
	}
	return out
}

func RedactInvariant(inv *Invariant) Invariant {
	if inv == nil {
		return Invariant{}
	}
	out := *inv
	out.Rule = RedactText(out.Rule)
	out.Ref = RedactText(out.Ref)
	return out
}

func RedactInvariants(list []Invariant) []Invariant {
	out := make([]Invariant, len(list))
	for i := range list {
		out[i] = RedactInvariant(&list[i])
	}
	return out
}
