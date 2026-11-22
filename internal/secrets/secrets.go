// Package secrets holds the secret redaction patterns and the path deny lists
// shared by everything that sends repository text out of process (egm prompts,
// graph vector embedding). It is a leaf package: stdlib only.
package secrets

import (
	"path"
	"regexp"
	"strings"
	"sync"
)

// The first group mirrors mem.SecretPatterns; mem is not imported so this
// package stays a leaf.
var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`gho_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`-----BEGIN[^-]+PRIVATE KEY-----[\s\S]*?-----END[^-]+PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-+=/]+`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]+`),
	regexp.MustCompile(`(?i)[A-Za-z0-9_]*_TOKEN=[^\s"'&]+`),
	regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{32,}`),
	regexp.MustCompile(`(?i)\b[A-Za-z0-9_]*_(KEY|SECRET|PASSWORD)=[^\s"'&]+`),
}

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

// Matches returns every secret-like excerpt of s, one entry per match.
func Matches(s string) []string {
	var out []string
	for _, re := range redactPatterns {
		out = append(out, re.FindAllString(s, -1)...)
	}
	return out
}

var (
	// DenyBaseGlob is matched against a path's last element.
	DenyBaseGlob = []string{
		".env*", "*.pem", "*.key", "*.p12", "id_*", "credentials*", "secrets*", "*.tfvars", "*.tfstate*",
		"kubeconfig*", ".npmrc", ".netrc", ".pypirc", "service-account*.json", "*.keystore", "*.jks",
	}
	// DenyPathGlob is matched against the whole repo-relative path.
	DenyPathGlob = []string{
		".toolbelt/**", "**/testdata/**", "internal/egm/personas/**", "internal/egm/standards/**",
		"internal/egm/injection_patterns.txt", "docs/pt-BR/**",
	}
)

// Denied reports whether rel (slash-separated, repo-relative) is on a deny
// list and must never leave the process.
func Denied(rel string) bool {
	return MatchAny(DenyPathGlob, rel) || MatchAny(DenyBaseGlob, path.Base(rel))
}

var (
	globMu    sync.Mutex
	globCache = map[string]*regexp.Regexp{}
)

// MatchGlob matches a repo-relative slash path against a glob where `**`
// spans directories (also mid-path), `*` and `?` stay inside one segment.
func MatchGlob(pattern, rel string) bool {
	globMu.Lock()
	re, ok := globCache[pattern]
	if !ok {
		re = regexp.MustCompile(globRegexp(pattern))
		globCache[pattern] = re
	}
	globMu.Unlock()
	return re.MatchString(rel)
}

func globRegexp(p string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '*' && i+1 < len(p) && p[i+1] == '*':
			i++
			if i+1 < len(p) && p[i+1] == '/' {
				i++
				b.WriteString("(?:.*/)?")
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return b.String()
}

// MatchAny reports whether rel matches any of the globs.
func MatchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if MatchGlob(g, rel) {
			return true
		}
	}
	return false
}
