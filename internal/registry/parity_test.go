package registry

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultHasCLIParityTools(t *testing.T) {
	want := []string{
		"tb_mem_add",
		"tb_mem_list",
		"tb_mem_show",
		"tb_mem_edit",
		"tb_mem_retire",
		"tb_mem_search",
		"tb_mem_check",
		"tb_mem_init",
		"tb_verify_run",
		"tb_review",
		"tb_drift",
		"tb_guard_check",
		"tb_stack",
		"tb_graph_impact",
		"tb_graph_explain",
		"tb_graph_path",
	}
	names := map[string]bool{}
	for _, tool := range Default().List() {
		names[tool.Name] = true
	}
	for _, n := range want {
		if !names[n] {
			t.Errorf("missing MCP tool %s (C3 CLI parity)", n)
		}
	}
}

func TestMemListTool(t *testing.T) {
	dir := t.TempDir()
	toolbelt := filepath.Join(dir, ".toolbelt")
	if err := os.MkdirAll(toolbelt, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Default().Call(context.Background(), "tb_mem_init", map[string]any{"cwd": dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_mem_init: %s", res.Content[0].Text)
	}
	res, err = Default().Call(context.Background(), "tb_mem_list", map[string]any{"cwd": dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_mem_list: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "anti_cheat") {
		t.Fatalf("esperava invariantes anti_cheat, got: %s", res.Content[0].Text)
	}
}

func TestMemAddShowEditSearchTool(t *testing.T) {
	dir := memToolDir(t)
	id := memToolAdd(t, dir, map[string]any{
		"rule":     "nunca tocar secrets/",
		"category": "security",
		"ref":      "docs/adr/0001.md",
	})
	res, err := Default().Call(context.Background(), "tb_mem_show", map[string]any{"cwd": dir, "id": id})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || !strings.Contains(res.Content[0].Text, "docs/adr/0001.md") {
		t.Fatalf("tb_mem_show: %v %s", res.IsError, res.Content[0].Text)
	}
	res, err = Default().Call(context.Background(), "tb_mem_edit", map[string]any{
		"cwd":  dir,
		"id":   id,
		"rule": "nunca tocar secrets/ nem .env",
		"ref":  "docs/decisions/secrets.md",
	})
	if err != nil || res.IsError {
		t.Fatalf("tb_mem_edit: %v %v", err, res)
	}
	res, err = Default().Call(context.Background(), "tb_mem_search", map[string]any{
		"cwd":   dir,
		"query": "decisions",
	})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].Text, id) {
		t.Fatalf("tb_mem_search: %v %v", err, res)
	}
}

func TestMemRetireAndCheckTool(t *testing.T) {
	dir := memToolDir(t)
	id := memToolAdd(t, dir, map[string]any{
		"rule":     "nunca tocar secrets/",
		"category": "security",
	})
	res, err := Default().Call(context.Background(), "tb_mem_retire", map[string]any{"cwd": dir, "id": id})
	if err != nil || res.IsError {
		t.Fatalf("tb_mem_retire: %v %v", err, res)
	}
	res, err = Default().Call(context.Background(), "tb_mem_list", map[string]any{"cwd": dir})
	if err != nil || res.IsError {
		t.Fatalf("tb_mem_list: %v %v", err, res)
	}
	if strings.Contains(res.Content[0].Text, id) {
		t.Fatalf("list default não deveria incluir retired: %s", res.Content[0].Text)
	}
	res, err = Default().Call(context.Background(), "tb_mem_check", map[string]any{"cwd": dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_mem_check: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "compact_safe") {
		t.Fatalf("esperava check report, got: %s", res.Content[0].Text)
	}
}

func TestMemAllBoolAndLegacyString(t *testing.T) {
	dir := memToolDir(t)
	id := memToolAdd(t, dir, map[string]any{
		"rule":     "nunca tocar secrets/",
		"category": "security",
	})
	res, err := Default().Call(context.Background(), "tb_mem_retire", map[string]any{"cwd": dir, "id": id})
	if err != nil || res.IsError {
		t.Fatalf("tb_mem_retire: %v %v", err, res)
	}
	res, err = Default().Call(context.Background(), "tb_mem_list", map[string]any{"cwd": dir, "all": true})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].Text, id) {
		t.Fatalf("tb_mem_list all=true: %v %v", err, res)
	}
	res, err = Default().Call(context.Background(), "tb_mem_list", map[string]any{"cwd": dir, "all": "true"})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].Text, id) {
		t.Fatalf("tb_mem_list all=\"true\" legado: %v %v", err, res)
	}
	res, err = Default().Call(context.Background(), "tb_mem_search", map[string]any{
		"cwd": dir, "query": "secrets", "all": true,
	})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].Text, id) {
		t.Fatalf("tb_mem_search all=true: %v %v", err, res)
	}
	res, err = Default().Call(context.Background(), "tb_mem_check", map[string]any{"cwd": dir, "all": "false"})
	if err != nil || res.IsError {
		t.Fatalf("tb_mem_check all=\"false\": %v %v", err, res)
	}
}

func TestVerifyRunTool(t *testing.T) {
	dir := gitRepo(t)
	res, err := Default().Call(context.Background(), "tb_verify_run", map[string]any{
		"cwd":      dir,
		"base":     "HEAD",
		"head":     "HEAD",
		"test_cmd": "true",
		"max_iter": "1",
		"timeout":  "30s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_verify_run: %s", res.Content[0].Text)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["verdict"] != "PASS" {
		t.Fatalf("verdict=%v", payload["verdict"])
	}
}

func TestReviewAndDriftTools(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "init")
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "src", "b.go"), "package src\n")
	writeFile(t, filepath.Join(dir, "rd.md"), `---
title: add b
base_commit: HEAD~1
scope:
  - src/
---
`)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "add b")

	res, err := Default().Call(context.Background(), "tb_review", map[string]any{
		"cwd":       dir,
		"ticket":    filepath.Join(dir, "rd.md"),
		"base":      "HEAD~1",
		"head":      "HEAD",
		"reviewers": "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_review: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "fresh_context") {
		t.Fatalf("esperava relatório de review, got: %s", res.Content[0].Text)
	}

	res, err = Default().Call(context.Background(), "tb_drift", map[string]any{
		"cwd":    dir,
		"ticket": filepath.Join(dir, "rd.md"),
		"base":   "HEAD~1",
		"head":   "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_drift: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "out_of_scope") &&
		!strings.Contains(res.Content[0].Text, "base_mismatch") {
		t.Fatalf("esperava drift report, got: %s", res.Content[0].Text)
	}
}

func TestStackTool(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "init")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	writeFile(t, filepath.Join(dir, "rd.md"), `---
title: add
scope:
  - a.go
  - rd.md
---
`)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "change")

	res, err := Default().Call(context.Background(), "tb_stack", map[string]any{
		"cwd":      dir,
		"base":     "HEAD~1",
		"head":     "HEAD",
		"ops":      []any{"guard", "drift"},
		"ticket":   filepath.Join(dir, "rd.md"),
		"test_cmd": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_stack: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, `"steps"`) ||
		!strings.Contains(res.Content[0].Text, `"total_duration_ms"`) {
		t.Fatalf("esperava JSON agregado: %s", res.Content[0].Text)
	}
}

func TestGuardCheckTool(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "ok.go"), "package ok\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "init")
	writeFile(t, filepath.Join(dir, "ok.go"), "package ok\nfunc X() {}\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "change")

	res, err := Default().Call(context.Background(), "tb_guard_check", map[string]any{
		"cwd":  dir,
		"base": "HEAD~1",
		"head": "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_guard_check: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "violations") &&
		!strings.Contains(res.Content[0].Text, "base_ref") {
		t.Fatalf("esperava check result, got: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, `"strict_fail": false`) {
		t.Fatalf("sem SeverityError, strict_fail deveria ser false: %s", res.Content[0].Text)
	}
}

func TestGuardCheckToolStrictFailDefault(t *testing.T) {
	dir := gitRepo(t)
	hidden := filepath.Join(dir, "tests", "hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(hidden, "sentinel.txt"), "ok\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(hidden, "sentinel.txt"), "changed\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "touch protected")

	res, err := Default().Call(context.Background(), "tb_guard_check", map[string]any{
		"cwd":  dir,
		"base": "HEAD~1",
		"head": "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_guard_check: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, `"strict_fail": true`) {
		t.Fatalf("SeverityError deveria marcar strict_fail por default: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "rules_checked") {
		t.Fatalf("esperava rules_checked: %s", res.Content[0].Text)
	}

	res, err = Default().Call(context.Background(), "tb_guard_check", map[string]any{
		"cwd":       dir,
		"base":      "HEAD~1",
		"head":      "HEAD",
		"warn_only": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_guard_check warn_only: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, `"strict_fail": false`) {
		t.Fatalf("warn_only deveria zerar strict_fail: %s", res.Content[0].Text)
	}
}

func TestGuardCheckToolRejectsOptionInjection(t *testing.T) {
	dir := gitRepo(t)
	git(t, dir, "commit", "--allow-empty", "-m", "two")
	outDir := t.TempDir()
	injected := filepath.Join(outDir, "pwned")
	res, err := Default().Call(context.Background(), "tb_guard_check", map[string]any{
		"cwd":  dir,
		"base": "--output=" + injected,
		"head": "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("MCP deveria rejeitar injeção: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "inválida") && !strings.Contains(res.Content[0].Text, "-") {
		t.Fatalf("mensagem: %s", res.Content[0].Text)
	}
	if _, statErr := os.Stat(injected + "..HEAD"); statErr == nil {
		t.Fatal("MCP não deveria criar arquivo via --output")
	}
}

func TestReviewToolRejectsMissingApprovedSHA(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "rd.md"), "---\ntitle: t\nscope:\n  - a.go\n---\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "head")
	res, err := Default().Call(context.Background(), "tb_review", map[string]any{
		"cwd":          dir,
		"base":         "HEAD~1",
		"head":         "HEAD",
		"ticket":       "rd.md",
		"approved_sha": "ffffffffffffffffffffffffffffffffffffffff",
		"reviewers":    1,
		"standards":    []any{"scope"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("approved_sha inexistente deveria falhar: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "inexistente") {
		t.Fatalf("erro claro esperado: %s", res.Content[0].Text)
	}
}

func TestMemImmutableRequiresForceMCP(t *testing.T) {
	dir := memToolDir(t)
	id := memToolAdd(t, dir, map[string]any{
		"rule":      "não alterar",
		"category":  "mandate",
		"immutable": true,
	})
	res, err := Default().Call(context.Background(), "tb_mem_edit", map[string]any{
		"cwd":  dir,
		"id":   id,
		"rule": "alterada",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("edit sem force deveria falhar")
	}
	res, err = Default().Call(context.Background(), "tb_mem_edit", map[string]any{
		"cwd":    dir,
		"id":     id,
		"rule":   "alterada",
		"force":  true,
		"reason": "revisão",
	})
	if err != nil || res.IsError {
		t.Fatalf("edit com force+reason: %v %v", err, res)
	}
}

func TestMemAddDefaultMutableMCP(t *testing.T) {
	dir := memToolDir(t)
	id := memToolAdd(t, dir, map[string]any{
		"rule":     "mutável por default",
		"category": "security",
	})
	res, err := Default().Call(context.Background(), "tb_mem_show", map[string]any{"cwd": dir, "id": id})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || !strings.Contains(res.Content[0].Text, `"immutable": false`) {
		t.Fatalf("esperava immutable false: %s", res.Content[0].Text)
	}
}

func TestMemCheckFailClosedMCP(t *testing.T) {
	dir := memToolDir(t)
	path := filepath.Join(dir, ".toolbelt", "invariants.yaml")
	if err := os.WriteFile(path, []byte("not: yaml: [["), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Default().Call(context.Background(), "tb_mem_check", map[string]any{"cwd": dir})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("check com yaml corrompido deveria falhar fechado")
	}
}

func TestMemRedactSecretsMCP(t *testing.T) {
	dir := memToolDir(t)
	id := memToolAdd(t, dir, map[string]any{
		"rule":     "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA AKIAIOSFODNN7EXAMPLE",
		"category": "security",
	})
	res, err := Default().Call(context.Background(), "tb_mem_show", map[string]any{"cwd": dir, "id": id})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_mem_show: %s", res.Content[0].Text)
	}
	text := res.Content[0].Text
	if strings.Contains(text, "ghp_AAAA") || strings.Contains(text, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("segredo vazou no MCP: %s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("esperava redação: %s", text)
	}
}

func memToolDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func memToolAdd(t *testing.T, dir string, args map[string]any) string {
	t.Helper()
	args["cwd"] = dir
	res, err := Default().Call(context.Background(), "tb_mem_add", args)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_mem_add: %s", res.Content[0].Text)
	}
	var added map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &added); err != nil {
		t.Fatal(err)
	}
	id, _ := added["id"].(string)
	if id == "" {
		t.Fatalf("id ausente: %s", res.Content[0].Text)
	}
	return id
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "config", "user.email", "t@t")
	git(t, dir, "config", "user.name", "t")
	git(t, dir, "commit", "--allow-empty", "-m", "root")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReviewDriftStackMissingTicket(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "init")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\nfunc X() {}\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "change")

	missing := filepath.Join(dir, "absent-ticket.md")
	for _, tool := range []string{"tb_review", "tb_drift"} {
		res, err := Default().Call(context.Background(), tool, map[string]any{
			"cwd":    dir,
			"ticket": missing,
			"base":   "HEAD~1",
			"head":   "HEAD",
		})
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		if !res.IsError {
			t.Fatalf("%s deveria retornar IsError com ticket ausente", tool)
		}
		msg := res.Content[0].Text
		if !strings.Contains(msg, "ticket não encontrado") {
			t.Fatalf("%s mensagem: %s", tool, msg)
		}
		if !strings.Contains(msg, missing) {
			t.Fatalf("%s deveria citar o caminho %q: %s", tool, missing, msg)
		}
	}

	res, err := Default().Call(context.Background(), "tb_stack", map[string]any{
		"cwd":    dir,
		"base":   "HEAD~1",
		"head":   "HEAD",
		"ops":    []any{"drift"},
		"ticket": missing,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_stack agrega erro na etapa, não no envelope: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, `"ok": false`) && !strings.Contains(res.Content[0].Text, `"ok":false`) {
		t.Fatalf("tb_stack deveria ter ok=false: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "ticket não encontrado") {
		t.Fatalf("tb_stack deveria citar ticket não encontrado: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, missing) {
		t.Fatalf("tb_stack deveria citar o caminho %q: %s", missing, res.Content[0].Text)
	}
}
