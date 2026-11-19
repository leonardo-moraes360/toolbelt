package egm

import (
	"embed"
	"strings"
)

// The personas and standards ship inside the binary (R11) so a review never
// depends on files the reviewed repository could plant or change.
//
//go:embed personas/*.md standards/*.md
var contentFS embed.FS

// Persona is a reviewer lens (glossary) with its severity cap (R11).
type Persona struct {
	Name        string
	MaxSeverity string // blocker | major | nit | none (arbiter: emits a verdict, not findings)
	Text        string
}

// PersonaNames lists the embedded personas: the 8 of the preset plus the
// arbiter (R10, R29).
var PersonaNames = []string{
	"zoom-out", "inventive-catalyst", "comprehension", "design", "structural",
	"style", "implementation-readiness", "codebase-details", "arbiter",
}

// StandardNames lists the embedded standards (R10, R13).
var StandardNames = []string{"feature-spec", "adr", "implementation-readiness", "zoom-out"}

// LoadPersona returns an embedded persona. The name is checked against the
// closed list so a config value can never select an arbitrary file.
func LoadPersona(name string) (*Persona, error) {
	if !containsName(PersonaNames, name) {
		return nil, errorf(ExitError, "unknown_persona", "unknown persona %q", name)
	}
	b, err := contentFS.ReadFile("personas/" + name + ".md")
	if err != nil {
		return nil, errorf(ExitError, "unknown_persona", "persona %q: %v", name, err)
	}
	sev, body, ok := splitFrontMatter(string(b))
	if !ok {
		return nil, errorf(ExitError, "bad_persona", "persona %q: missing or invalid max_severity front matter", name)
	}
	return &Persona{Name: name, MaxSeverity: sev, Text: body}, nil
}

// LoadStandard returns the text of an embedded standard.
func LoadStandard(name string) (string, error) {
	if !containsName(StandardNames, name) {
		return "", errorf(ExitError, "unknown_standard", "unknown standard %q", name)
	}
	b, err := contentFS.ReadFile("standards/" + name + ".md")
	if err != nil {
		return "", errorf(ExitError, "unknown_standard", "standard %q: %v", name, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func containsName(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// splitFrontMatter parses the "---\nmax_severity: X\n---" header. It accepts
// only that key: anything else in the header is a packaging mistake.
func splitFrontMatter(s string) (sev, body string, ok bool) {
	rest, found := strings.CutPrefix(s, "---\n")
	if !found {
		return "", "", false
	}
	header, body, found := strings.Cut(rest, "\n---\n")
	if !found {
		return "", "", false
	}
	key, val, found := strings.Cut(strings.TrimSpace(header), ":")
	if !found || strings.TrimSpace(key) != "max_severity" {
		return "", "", false
	}
	sev = strings.TrimSpace(val)
	switch sev {
	case SevBlocker, SevMajor, SevNit, "none":
		return sev, strings.TrimSpace(body), true
	}
	return "", "", false
}
