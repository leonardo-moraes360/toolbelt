package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRepoConfig(t *testing.T, dir, content string) {
	t.Helper()
	p := filepath.Join(dir, ".toolbelt", "config.yaml")
	if content == "" {
		_ = os.Remove(p)
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func verifyConfigFindings(t *testing.T, baseCfg, headCfg string) []Violation {
	t.Helper()
	dir := setupRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRepoConfig(t, dir, baseCfg)
	base := commitAll(t, dir, "base")
	writeRepoConfig(t, dir, headCfg)
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	head := commitAll(t, dir, "head")
	res, err := NewChecker(dir).Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	var out []Violation
	for _, v := range res.Violations {
		if v.Type == ViolationVerifyConfigChanged {
			out = append(out, v)
		}
	}
	return out
}

const baseVerifyCfg = "version: 1\nverify:\n  test_cmd: go test ./...\n  lint_cmd: go vet ./...\n"

func TestVerifyConfigChanged(t *testing.T) {
	cases := []struct {
		name     string
		base     string
		head     string
		wantKeys string // empty: no finding expected
	}{
		{"unchanged", baseVerifyCfg, baseVerifyCfg, ""},
		{"reordered keys", baseVerifyCfg, "verify:\n  lint_cmd: go vet ./...\n  test_cmd: go test ./...\nversion: 1\n", ""},
		{"test_cmd changed", baseVerifyCfg, "version: 1\nverify:\n  test_cmd: \"true\"\n  lint_cmd: go vet ./...\n", "verify.test_cmd"},
		{"checks added", baseVerifyCfg, baseVerifyCfg + "  checks:\n    - name: vuln\n      cmd: govulncheck ./...\n", "verify.checks"},
		{"two keys", baseVerifyCfg, "version: 1\nverify:\n  test_cmd: x\n  lint_cmd: y\n", "verify.lint_cmd, verify.test_cmd"},
		{"section removed", baseVerifyCfg, "version: 1\n", "verify.lint_cmd, verify.test_cmd"},
		{"file removed", baseVerifyCfg, "", "verify.lint_cmd, verify.test_cmd"},
		{"section added", "version: 1\n", baseVerifyCfg, "verify.lint_cmd, verify.test_cmd"},
		{"empty section added", "version: 1\n", "version: 1\nverify: {}\n", "verify"},
		{"other section changed", baseVerifyCfg + "graph:\n  enabled: false\n", baseVerifyCfg + "graph:\n  enabled: true\n", ""},
		{"absent at both", "", "", ""},
		{"no verify section at both", "version: 1\n", "version: 2\n", ""},
		{"malformed head", baseVerifyCfg, "verify: [unclosed\n", "verify"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := verifyConfigFindings(t, tc.base, tc.head)
			if tc.wantKeys == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected findings: %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want 1 finding, got %+v", got)
			}
			v := got[0]
			if v.Severity != SeverityError || v.File != ".toolbelt/config.yaml" {
				t.Fatalf("bad finding: %+v", v)
			}
			if v.Context != tc.wantKeys || !strings.Contains(v.Message, tc.wantKeys) {
				t.Fatalf("keys: got context %q message %q, want %q", v.Context, v.Message, tc.wantKeys)
			}
		})
	}
}

// A malformed base file is a warning finding, not a guard error, so guard
// and verify -c keep working.
func TestVerifyConfigMalformedBaseWarns(t *testing.T) {
	dir := setupRepo(t)
	writeRepoConfig(t, dir, "verify: [unclosed\n")
	base := commitAll(t, dir, "base")
	writeRepoConfig(t, dir, baseVerifyCfg)
	head := commitAll(t, dir, "head")
	res, err := NewChecker(dir).Check(base, head)
	if err != nil {
		t.Fatalf("malformed base must not be a guard error: %v", err)
	}
	var found bool
	for _, v := range res.Violations {
		if v.Type == ViolationVerifyConfigChanged {
			t.Fatalf("unexpected verify_config_changed: %+v", v)
		}
		if v.Type == ViolationVerifyConfigUnreadable {
			found = true
			if v.Severity != SeverityWarning || v.File != ".toolbelt/config.yaml" {
				t.Fatalf("bad finding: %+v", v)
			}
		}
	}
	if !found {
		t.Fatalf("want verify_config_unreadable, got %+v", res.Violations)
	}
}

// writeRepoConfigLink replaces the config with a symlink to target ("" removes
// it).
func writeRepoConfigLink(t *testing.T, dir, target string) {
	t.Helper()
	p := filepath.Join(dir, ".toolbelt", "config.yaml")
	_ = os.Remove(p)
	if target == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyConfigSymlink(t *testing.T) {
	type side struct{ link, file string }
	cases := []struct {
		name       string
		base, head side
		want       bool
	}{
		{"same target", side{link: "../cfg-a.yaml"}, side{link: "../cfg-a.yaml"}, false},
		{"target changed", side{link: "../cfg-a.yaml"}, side{link: "../cfg-b.yaml"}, true},
		{"link added", side{file: baseVerifyCfg}, side{link: "../cfg-a.yaml"}, true},
		{"link removed", side{link: "../cfg-a.yaml"}, side{file: baseVerifyCfg}, true},
		{"link added over nothing", side{}, side{link: "../cfg-a.yaml"}, true},
	}
	apply := func(t *testing.T, dir string, s side) {
		if s.link != "" {
			writeRepoConfigLink(t, dir, s.link)
			return
		}
		writeRepoConfigLink(t, dir, "")
		writeRepoConfig(t, dir, s.file)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupRepo(t)
			for _, n := range []string{"cfg-a.yaml", "cfg-b.yaml"} {
				if err := os.WriteFile(filepath.Join(dir, n), []byte(baseVerifyCfg), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			apply(t, dir, tc.base)
			base := commitAll(t, dir, "base")
			apply(t, dir, tc.head)
			if err := os.WriteFile(filepath.Join(dir, "README"), []byte("y\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			head := commitAll(t, dir, "head")
			res, err := NewChecker(dir).Check(base, head)
			if err != nil {
				t.Fatal(err)
			}
			var got []Violation
			for _, v := range res.Violations {
				if v.Type == ViolationVerifyConfigChanged {
					got = append(got, v)
				}
			}
			if !tc.want {
				if len(got) != 0 {
					t.Fatalf("unexpected findings: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Context != "verify" || got[0].Severity != SeverityError ||
				!strings.Contains(got[0].Message, "cannot parse") {
				t.Fatalf("want one verify finding, got %+v", got)
			}
		})
	}
}

// A caller-supplied name-status diff that omits the config file must not hide
// the change: the rule reads both blobs itself.
func TestVerifyConfigIgnoresSuppliedDiff(t *testing.T) {
	dir := setupRepo(t)
	writeRepoConfig(t, dir, baseVerifyCfg)
	base := commitAll(t, dir, "base")
	writeRepoConfig(t, dir, "verify:\n  test_cmd: \"true\"\n")
	head := commitAll(t, dir, "head")
	res, err := NewChecker(dir).CheckDiff(base, head, "M\tREADME\n")
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationVerifyConfigChanged, ".toolbelt/config.yaml") {
		t.Fatalf("want verify_config_changed, got %+v", res.Violations)
	}
}
