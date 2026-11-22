package registry

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRulersSchemaFields(t *testing.T) {
	want := map[string]string{
		"typecheck_cmd": "string",
		"no_typecheck":  "boolean",
		"checks":        "array",
		"skip_checks":   "array",
		"no_checks":     "boolean",
		"ruler":         "string",
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
