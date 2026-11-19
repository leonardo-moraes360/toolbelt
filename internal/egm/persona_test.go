package egm

import (
	"strings"
	"testing"
)

func TestPersonaLoadAll(t *testing.T) {
	want := map[string]string{
		"zoom-out": SevBlocker, "inventive-catalyst": SevMajor, "comprehension": SevBlocker,
		"design": SevBlocker, "structural": SevBlocker, "style": SevMajor,
		"implementation-readiness": SevBlocker, "codebase-details": SevBlocker, "arbiter": "none",
	}
	if len(want) != len(PersonaNames) {
		t.Fatalf("persona list %v", PersonaNames)
	}
	for name, sev := range want {
		p, err := LoadPersona(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.MaxSeverity != sev || p.Name != name {
			t.Errorf("%s: max_severity %q, want %q", name, p.MaxSeverity, sev)
		}
		if p.Text == "" || strings.Contains(p.Text, "max_severity") {
			t.Errorf("%s: body must be non-empty and without front matter", name)
		}
	}
}

func TestPersonaPresetCovered(t *testing.T) {
	for _, r := range Preset {
		if _, err := LoadPersona(r.Persona); err != nil {
			t.Errorf("preset %s: %v", r.Name, err)
		}
		if r.Standard != "" && r.Standard != "structural" {
			if _, err := LoadStandard(r.Standard); err != nil {
				t.Errorf("preset %s: %v", r.Name, err)
			}
		}
	}
}

func TestPersonaUnknown(t *testing.T) {
	for _, n := range []string{"nope", "../standards/adr", "", "personas/style"} {
		if _, err := LoadPersona(n); err == nil {
			t.Errorf("LoadPersona(%q) must fail", n)
		}
		if _, err := LoadStandard(n); err == nil {
			t.Errorf("LoadStandard(%q) must fail", n)
		}
	}
}

func TestPersonaFrontMatter(t *testing.T) {
	cases := []struct {
		in, sev string
		ok      bool
	}{
		{"---\nmax_severity: major\n---\nbody\n", SevMajor, true},
		{"---\nmax_severity:  none \n---\nbody", "none", true},
		{"max_severity: major\nbody", "", false},
		{"---\nmax_severity: critical\n---\nbody", "", false},
		{"---\nseverity: major\n---\nbody", "", false},
		{"---\nmax_severity: major\nother: x\n---\nbody", "", false},
		{"---\nmax_severity: major\nbody", "", false},
	}
	for _, c := range cases {
		sev, body, ok := splitFrontMatter(c.in)
		if ok != c.ok || sev != c.sev || (ok && body != strings.TrimSpace(strings.SplitN(c.in, "---\n", 3)[2])) {
			t.Errorf("%q: got %q %q %v", c.in, sev, body, ok)
		}
	}
}

// TestStandardRequiredHeadings keeps the structural standards in sync with
// the required-sections checker (R13): both languages of every heading.
func TestStandardRequiredHeadings(t *testing.T) {
	for _, std := range []string{"feature-spec", "adr"} {
		text, err := LoadStandard(std)
		if err != nil {
			t.Fatal(err)
		}
		pairs := requiredHeadings[std]
		if len(pairs) != 5 {
			t.Fatalf("%s: %d required headings", std, len(pairs))
		}
		for _, pair := range pairs {
			for _, h := range pair {
				if !strings.Contains(text, "`"+h+"`") {
					t.Errorf("%s: missing required heading %q", std, h)
				}
			}
		}
	}
	for _, std := range StandardNames {
		if text, err := LoadStandard(std); err != nil || text == "" {
			t.Errorf("%s: %v", std, err)
		}
	}
}
