package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/review"
)

func TestDriftCLIExplicitBaseAndThreeDot(t *testing.T) {
	tbBin := buildTB(t)
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("root.txt", "root\n")
	run("add", ".")
	run("commit", "-m", "root")
	write("base.txt", "base\n")
	run("add", ".")
	run("commit", "-m", "base")
	fork := gitRevParseCLI(t, repo, "HEAD")

	run("checkout", "-b", "feature")
	write("pr.txt", "pr\n")
	run("add", ".")
	run("commit", "-m", "pr")

	run("checkout", "main")
	for _, n := range []string{"squash1.txt", "squash2.txt", "squash3.txt"} {
		write(n, "sq\n")
	}
	run("add", ".")
	run("commit", "-m", "squash")

	run("checkout", "feature")
	rd := "---\ntitle: pr\nbase_commit: " + fork + "\nscope:\n  - pr.txt\n  - rd.md\n---\n"
	write("rd.md", rd)
	run("add", ".")
	run("commit", "-m", "ticket")

	cmd := exec.Command(tbBin, "--json", "drift", "--ticket", "rd.md", "--base", "main", "--head", "HEAD")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tb drift: %v\n%s", err, out)
	}
	var drift review.DriftReport
	if err := json.Unmarshal(out, &drift); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	for _, f := range drift.OutOfScope {
		if strings.HasPrefix(f, "squash") {
			t.Fatalf("CLI --base main three-dot não deve listar squash: %v", drift.OutOfScope)
		}
	}
	if drift.BaseMismatch {
		t.Fatal("main ancestral não deve mismatch")
	}
	if drift.ActualBase == "" || drift.MergeBase == "" {
		t.Fatalf("actual_base/merge_base vazios: %#v", drift)
	}
	mainTip := gitRevParseCLI(t, repo, "main")
	if drift.ActualBase != mainTip {
		t.Fatalf("actual_base=%q want main %q ( --base deve prevalecer sobre ticket)", drift.ActualBase, mainTip)
	}
}

func gitRevParseCLI(t *testing.T, repo, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
