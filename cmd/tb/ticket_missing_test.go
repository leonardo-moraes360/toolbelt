package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDriftReviewStackMissingTicketExit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "head")

	missing := "no-such-ticket.md"
	cases := []struct {
		name string
		args []string
	}{
		{name: "drift", args: []string{"drift", "--ticket", missing, "--base", "HEAD~1", "--head", "HEAD"}},
		{name: "review", args: []string{"review", "--ticket", missing, "--base", "HEAD~1", "--head", "HEAD", "--standards", "scope", "--reviewers", "1"}},
		{name: "stack", args: []string{"stack", "drift", "--ticket", missing, "--base", "HEAD~1", "--head", "HEAD"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, code := runTBCapture(t, tbBin, dir, tc.args...)
			if code == 0 {
				t.Fatalf("ticket ausente deveria sair ≠ 0\n%s", out)
			}
			if !strings.Contains(out, "ticket not found") {
				t.Fatalf("saída deveria citar ticket não encontrado:\n%s", out)
			}
			if !strings.Contains(out, missing) {
				t.Fatalf("saída deveria citar o caminho procurado %q:\n%s", missing, out)
			}
		})
	}
}
