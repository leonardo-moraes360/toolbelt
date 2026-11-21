package gitref

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsDashPrefix(t *testing.T) {
	err := Validate("--output=/tmp/x")
	if err == nil {
		t.Fatal("esperava erro para ref começando com '-'")
	}
	if !strings.Contains(err.Error(), "-") {
		t.Fatalf("mensagem deveria citar o problema: %v", err)
	}
}

func TestValidateRejectsEmptyAndControls(t *testing.T) {
	if err := Validate(""); err == nil {
		t.Fatal("ref vazia")
	}
	if err := Validate("HEAD\n~1"); err == nil {
		t.Fatal("newline deveria ser rejeitado")
	}
	if err := Validate("HEAD"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRejectsMissingSHA(t *testing.T) {
	repo := initRepo(t)
	fake := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err := Resolve(repo, fake)
	if err == nil {
		t.Fatal("SHA de 40 hex inexistente deveria falhar")
	}
	if !strings.Contains(err.Error(), "missing or invalid") {
		t.Fatalf("erro deveria ser claro: %v", err)
	}
}

func TestResolveAcceptsHEAD(t *testing.T) {
	repo := initRepo(t)
	sha, err := Resolve(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(sha) != 40 {
		t.Fatalf("sha=%q", sha)
	}
	pairBase, pairHead, err := ResolvePair(repo, "HEAD", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if pairBase != sha || pairHead != sha {
		t.Fatalf("pair=%s/%s want %s", pairBase, pairHead, sha)
	}
}

func TestResolveRejectsOptionInjection(t *testing.T) {
	repo := initRepo(t)
	target := filepath.Join(t.TempDir(), "pwned")
	_, err := Resolve(repo, "--output="+target)
	if err == nil {
		t.Fatal("injeção de opção deveria falhar na validação")
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("não deveria criar arquivo via injeção")
	}
}

func TestResolvePairDefaultBaseOnRootCommit(t *testing.T) {
	repo := initRepo(t)
	head, err := Resolve(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	base, gotHead, err := ResolvePair(repo, DefaultBase, "HEAD")
	if err != nil {
		t.Fatalf("single-commit repo with default base should not fail: %v", err)
	}
	if base != head || gotHead != head {
		t.Fatalf("base=%s head=%s want both %s", base, gotHead, head)
	}
}

func TestResolvePairExplicitMissingBaseStillFails(t *testing.T) {
	repo := initRepo(t)
	_, _, err := ResolvePair(repo, "HEAD~2", "HEAD")
	if err == nil {
		t.Fatal("explicit missing base must fail")
	}
	if !strings.Contains(err.Error(), "history too short") {
		t.Fatalf("relative ref should get a history hint, not fetch origin: %v", err)
	}
	if strings.Contains(err.Error(), "git fetch origin") {
		t.Fatalf("no fetch-origin hint for relative refs: %v", err)
	}
}

func TestResolvePairOrphanNextToHistoryFailsClosed(t *testing.T) {
	repo := initRepo(t)
	for _, args := range [][]string{
		{"commit", "--allow-empty", "-m", "second"},
		{"checkout", "-q", "--orphan", "squashed"},
		{"commit", "-q", "-m", "orphan root"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if _, _, err := ResolvePair(repo, DefaultBase, "HEAD"); err == nil {
		t.Fatal("orphan root next to other history must not fall back to an empty diff")
	}
}

func TestResolvePairShallowCloneFailsClosed(t *testing.T) {
	src := initRepo(t)
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "second")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	dst := filepath.Join(t.TempDir(), "shallow")
	if out, err := exec.Command("git", "clone", "-q", "--depth", "1", "file://"+src, dst).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v %s", err, out)
	}
	if _, _, err := ResolvePair(dst, DefaultBase, "HEAD"); err == nil {
		t.Fatal("shallow clone must not fall back to an empty diff")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}
