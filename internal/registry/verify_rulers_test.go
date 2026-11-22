package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRulersSchemaFields(t *testing.T) {
	want := map[string]string{
		"typecheck_cmd":    "string",
		"no_typecheck":     "boolean",
		"checks":           "array",
		"skip_checks":      "array",
		"no_checks":        "boolean",
		"ruler":            "string",
		"coverage_cmd":     "string",
		"coverage_pattern": "string",
		"min_coverage":     "number",
	}
	for _, name := range []string{"tb_verify_run", "tb_stack"} {
		tool, ok := Default().Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		for prop, typ := range want {
			if got := tool.InputSchema.Properties[prop].Type; got != typ {
				t.Fatalf("%s.%s type=%q want %q", name, prop, got, typ)
			}
		}
		if tool.InputSchema.Properties["checks"].Items.Type != "object" {
			t.Fatalf("%s.checks items must be objects", name)
		}
	}
}

func callJSON(t *testing.T, tool string, args map[string]any) (payload map[string]any, text string, isErr bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	res, err := Default().Call(context.Background(), tool, args)
	if err != nil {
		t.Fatal(err)
	}
	text = res.Content[0].Text
	if res.IsError {
		return nil, text, true
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, text)
	}
	return payload, text, false
}

func TestVerifyRunToolChecksAndTypecheck(t *testing.T) {
	dir := gitRepo(t)
	base := map[string]any{"cwd": dir, "base": "HEAD", "head": "HEAD", "test_cmd": "true", "max_iter": 1, "timeout": "30s"}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	payload, text, isErr := callJSON(t, "tb_verify_run", with(map[string]any{
		"checks": []any{map[string]any{"name": "good", "cmd": "true"}, map[string]any{"name": "bad", "cmd": "false"}},
	}))
	if isErr || payload["verdict"] != "ESCALATE" || !strings.Contains(text, `"name": "bad"`) {
		t.Fatalf("checks: %s", text)
	}

	payload, text, isErr = callJSON(t, "tb_verify_run", with(map[string]any{
		"checks":      []any{map[string]any{"name": "bad", "cmd": "false"}},
		"skip_checks": []any{"bad"},
	}))
	if isErr || payload["verdict"] != "PASS" {
		t.Fatalf("skip_checks: %s", text)
	}

	payload, text, isErr = callJSON(t, "tb_verify_run", with(map[string]any{"typecheck_cmd": "false"}))
	if isErr || payload["verdict"] != "ESCALATE" || payload["typecheck_passed"] != false {
		t.Fatalf("typecheck: %s", text)
	}

	payload, text, isErr = callJSON(t, "tb_verify_run", with(map[string]any{"typecheck_cmd": "false", "no_typecheck": true}))
	if isErr || payload["verdict"] != "PASS" {
		t.Fatalf("no_typecheck: %s", text)
	}

	for _, bad := range []map[string]any{
		{"ruler": "foo"},
		{"skip_checks": []any{"nope"}},
		{"checks": []any{"not-an-object"}},
		{"checks": []any{map[string]any{"name": "Bad Name", "cmd": "true"}}},
		{"checks": []any{map[string]any{"name": "dup", "cmd": "true"}, map[string]any{"name": "dup", "cmd": "false"}}},
	} {
		if _, text, isErr := callJSON(t, "tb_verify_run", with(bad)); !isErr {
			t.Fatalf("%v should be a tool error: %s", bad, text)
		}
	}
}

func TestStackToolVerifyRulerAndChecks(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: x\nscope:\n  - rd.md\n---\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "ticket")
	args := map[string]any{
		"cwd": dir, "ops": []any{"verify"}, "ticket": filepath.Join(dir, "rd.md"),
		"test_cmd": "true", "max_iter": 1, "ruler": "tests",
		"checks": []any{map[string]any{"name": "bad", "cmd": "false"}},
	}
	payload, text, isErr := callJSON(t, "tb_stack", args)
	if isErr || payload["ok"] != false || !strings.Contains(text, "check bad failed") {
		t.Fatalf("stack checks: %s", text)
	}
	args["no_checks"] = true
	if payload, text, isErr = callJSON(t, "tb_stack", args); isErr || payload["ok"] != true {
		t.Fatalf("stack no_checks: %s", text)
	}
	args["checks"] = []any{map[string]any{"name": "dup", "cmd": "true"}, map[string]any{"name": "dup", "cmd": "true"}}
	if _, text, isErr = callJSON(t, "tb_stack", args); !isErr || !strings.Contains(text, "duplicate") {
		t.Fatalf("stack duplicate check must be a tool error: %s", text)
	}
	args["ruler"] = "visual"
	if _, text, isErr = callJSON(t, "tb_stack", args); !isErr || !strings.Contains(text, "visual") {
		t.Fatalf("stack visual must be refused: %s", text)
	}
}

// MCP arguments that change the trusted config are listed in overrides.
func TestVerifyRunToolReportsOverrides(t *testing.T) {
	dir := gitRepo(t)
	payload, text, isErr := callJSON(t, "tb_verify_run", map[string]any{
		"cwd": dir, "base": "HEAD", "head": "HEAD", "test_cmd": "true", "typecheck_cmd": "true",
		"max_iter": 1, "timeout": "30s",
	})
	if isErr {
		t.Fatal(text)
	}
	got, _ := payload["overrides"].([]any)
	if len(got) != 2 || got[0] != "test_cmd" || got[1] != "typecheck_cmd" {
		t.Fatalf("overrides=%v\n%s", payload["overrides"], text)
	}
}

// coverageRepo commits a base thresholds file with min_percent 70.
func coverageRepo(t *testing.T) string {
	t.Helper()
	dir := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".toolbelt", "thresholds.yaml"), "version: 1\ncoverage:\n  min_percent: 70\n")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: x\nscope:\n  - rd.md\n---\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "thresholds")
	return dir
}

func coverageOf(t *testing.T, rulers any) map[string]any {
	t.Helper()
	m, _ := rulers.(map[string]any)
	cov, _ := m["coverage"].(map[string]any)
	if cov == nil {
		t.Fatalf("no coverage ruler in %v", rulers)
	}
	return cov
}

func mergedArgs(base, extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func coverageBlock(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	cov, _ := coverageOf(t, payload["rulers"])["coverage"].(map[string]any)
	if cov == nil {
		t.Fatalf("no coverage block: %v", payload)
	}
	return cov
}

// tb_verify_run takes the coverage fields and matches the CLI.
func TestVerifyRunToolCoverage(t *testing.T) {
	dir := coverageRepo(t)
	args := map[string]any{
		"cwd": dir, "base": "HEAD", "head": "HEAD", "test_cmd": "true", "max_iter": 1, "timeout": "30s",
		"ruler": "tests,coverage", "coverage_cmd": "echo 'TOTAL 75%'", "coverage_pattern": `TOTAL (\d+)%`,
	}
	payload, text, isErr := callJSON(t, "tb_verify_run", args)
	if isErr || payload["verdict"] != "PASS" {
		t.Fatalf("verify coverage: %s", text)
	}
	cov := coverageBlock(t, payload)
	if cov["threshold"] != 70.0 || cov["threshold_source"] != "base" || cov["percent"] != 75.0 {
		t.Fatalf("coverage=%v", cov)
	}
	if o := fmt.Sprint(payload["overrides"]); !strings.Contains(o, "coverage_cmd") || !strings.Contains(o, "coverage_pattern") {
		t.Fatalf("overrides=%v", o)
	}
	payload, text, isErr = callJSON(t, "tb_verify_run", mergedArgs(args, map[string]any{"min_coverage": 80}))
	if isErr || payload["verdict"] != "FAIL" || coverageBlock(t, payload)["threshold_source"] != "flag" {
		t.Fatalf("verify min_coverage: %s", text)
	}
	assertCoverageArgErrors(t, "tb_verify_run", args)
}

// tb_stack takes the same coverage fields as tb stack verify.
func TestStackToolCoverage(t *testing.T) {
	dir := coverageRepo(t)
	args := map[string]any{
		"cwd": dir, "ops": []any{"verify"}, "ticket": filepath.Join(dir, "rd.md"), "base": "HEAD", "head": "HEAD",
		"test_cmd": "true", "max_iter": 1, "ruler": "tests,coverage",
		"coverage_cmd": "echo 'TOTAL 75%'", "coverage_pattern": `TOTAL (\d+)%`,
	}
	payload, text, isErr := callJSON(t, "tb_stack", mergedArgs(args, map[string]any{"min_coverage": 80}))
	if isErr || payload["ok"] != false || !strings.Contains(text, "below the threshold 80% (flag)") {
		t.Fatalf("stack min_coverage: %s", text)
	}
	if payload, text, isErr = callJSON(t, "tb_stack", args); isErr || payload["ok"] != true {
		t.Fatalf("stack coverage: %s", text)
	}
	assertCoverageArgErrors(t, "tb_stack", args)
}

func assertCoverageArgErrors(t *testing.T, tool string, args map[string]any) {
	t.Helper()
	for _, bad := range []map[string]any{{"coverage_pattern": `(a)(b)`}, {"min_coverage": 101}, {"min_coverage": "lots"}} {
		if _, text, isErr := callJSON(t, tool, mergedArgs(args, bad)); !isErr {
			t.Fatalf("%s %v should be a tool error: %s", tool, bad, text)
		}
	}
}
