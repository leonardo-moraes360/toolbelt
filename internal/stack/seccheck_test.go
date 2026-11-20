package stack

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/secplay"
)

func TestParseOpSecCheck(t *testing.T) {
	op, err := ParseOp("seccheck")
	if err != nil || op != OpSecCheck {
		t.Fatalf("ParseOp(seccheck) = %v, %v", op, err)
	}
	if !isParallelOp(OpSecCheck) {
		t.Error("seccheck should be a parallel (read-only) op")
	}
}

func writeSecCheckRepo(t *testing.T, severity secplay.Severity) string {
	t.Helper()
	dir := t.TempDir()
	tb := filepath.Join(dir, ".toolbelt")
	sc := &secplay.SecChecks{Version: 1, Checks: []secplay.Check{{
		ID:           "sqli-concat",
		Class:        "sql-injection",
		Description:  "string-built query",
		Pattern:      `query\(.*\+`,
		IncludeGlobs: []string{"*.go"},
		Severity:     severity,
	}}}
	data, err := yaml.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tb, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secplay.SecChecksPath(tb), data, 0o600); err != nil {
		t.Fatal(err)
	}
	vuln := "package x\nfunc f(id string){ db.query(\"SELECT \"+id) }\n"
	if err := os.WriteFile(filepath.Join(dir, "h.go"), []byte(vuln), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunSecCheckFailsOnError(t *testing.T) {
	dir := writeSecCheckRepo(t, secplay.SeverityError)
	step, err := runSecCheck(context.Background(), &Options{}, &Shared{RepoDir: dir, Changed: []string{"h.go"}})
	if err != nil {
		t.Fatalf("runSecCheck: %v", err)
	}
	if step.OK || step.ExitCode != 1 {
		t.Errorf("error-severity finding should fail: ok=%v exit=%d", step.OK, step.ExitCode)
	}
}

func TestRunSecCheckWarnOnly(t *testing.T) {
	dir := writeSecCheckRepo(t, secplay.SeverityError)
	step, err := runSecCheck(context.Background(), &Options{WarnOnly: true}, &Shared{RepoDir: dir, Changed: []string{"h.go"}})
	if err != nil {
		t.Fatalf("runSecCheck: %v", err)
	}
	if !step.OK || step.ExitCode != 0 {
		t.Errorf("--warn-only should not fail: ok=%v exit=%d", step.OK, step.ExitCode)
	}
}

func TestRunSecCheckWarningPasses(t *testing.T) {
	dir := writeSecCheckRepo(t, secplay.SeverityWarning)
	step, err := runSecCheck(context.Background(), &Options{}, &Shared{RepoDir: dir, Changed: []string{"h.go"}})
	if err != nil {
		t.Fatalf("runSecCheck: %v", err)
	}
	if !step.OK {
		t.Error("warning-severity findings should not fail the step")
	}
}
