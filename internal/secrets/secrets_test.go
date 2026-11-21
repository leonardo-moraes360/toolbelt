package secrets

import (
	"strings"
	"testing"
)

func TestRedactPEMKeepsLines(t *testing.T) {
	in := "a\n-----BEGIN RSA PRIVATE KEY-----\nMIIEow\nAAAA\n-----END RSA PRIVATE KEY-----\nb\n"
	out, found := Redact(in)
	if !found {
		t.Fatal("PEM block not found")
	}
	if strings.Contains(out, "MIIEow") || !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("not redacted: %q", out)
	}
	if strings.Count(out, "\n") != strings.Count(in, "\n") {
		t.Fatalf("line count changed: %q", out)
	}
	if !strings.HasSuffix(out, "\nb\n") || !strings.HasPrefix(out, "a\n") {
		t.Fatalf("context lost: %q", out)
	}
}

func TestRedactPatterns(t *testing.T) {
	for _, s := range []string{
		"token ghp_" + strings.Repeat("a", 24),
		"github_pat_" + strings.Repeat("b", 24),
		"AKIA" + strings.Repeat("A", 16),
		"Authorization: Bearer abc.def",
		"xoxb-123-abc",
		"GITHUB_TOKEN=xyz",
		"key sk-ant-" + strings.Repeat("c", 24),
		"sk-" + strings.Repeat("d", 32),
		"DB_PASSWORD=hunter2",
		"aws_secret=x",
	} {
		out, found := Redact(s)
		if !found || !strings.Contains(out, "[REDACTED]") {
			t.Errorf("Redact(%q) = %q, %v", s, out, found)
		}
	}
	if out, found := Redact("plain text, no secrets\n"); found || out != "plain text, no secrets\n" {
		t.Errorf("clean text changed: %q %v", out, found)
	}
}

func TestDenied(t *testing.T) {
	cases := map[string]bool{
		".env":                                true,
		"config/.env.local":                   true,
		"certs/server.pem":                    true,
		"deploy/prod.tfvars":                  true,
		"home/id_ed25519":                     true,
		"svc/service-account-ci.json":         true,
		".toolbelt/graph.json":                true,
		"internal/x/testdata/a.go":            true,
		"testdata/a.go":                       true,
		"docs/pt-BR/README.md":                true,
		"internal/egm/personas/a.md":          true,
		"internal/egm/injection_patterns.txt": true,
		"main.go":                             false,
		"docs/specs/vector-index.md":          false,
		"internal/secrets/secrets.go":         true,
		"keys/README.md":                      false,
		"service-account.yaml":                false,
	}
	for rel, want := range cases {
		if got := Denied(rel); got != want {
			t.Errorf("Denied(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		g, p string
		ok   bool
	}{
		{"docs/**", "docs/a/b.md", true},
		{"**/testdata/**", "a/testdata/x", true},
		{"*.md", "a/b.md", false},
		{"a?c", "abc", true},
		{"a?c", "a/c", false},
		{"a.b", "axb", false},
	}
	for _, c := range cases {
		if MatchGlob(c.g, c.p) != c.ok {
			t.Errorf("MatchGlob(%q, %q) != %v", c.g, c.p, c.ok)
		}
	}
	if !MatchAny([]string{"x", "*.go"}, "a.go") || MatchAny(nil, "a.go") {
		t.Error("MatchAny")
	}
}
