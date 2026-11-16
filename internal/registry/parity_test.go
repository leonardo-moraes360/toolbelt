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
