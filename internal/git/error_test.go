package git

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestWrapIncludesStderrMissingRefAndFetchHint(t *testing.T) {
	err := errors.New("exit status 128")
	stderr := []byte("fatal: ambiguous argument 'origin/missing-branch..HEAD': unknown revision or path not in the working tree.\n")
	got := Wrap(err, stderr, "diff", "--name-status", "origin/missing-branch..HEAD")
	if got == nil {
		t.Fatal("expected error")
	}
	msg := got.Error()
	if !strings.Contains(msg, "unknown revision") {
		t.Fatalf("deve incluir stderr do git:\n%s", msg)
	}
	if !strings.Contains(msg, "ref missing: origin/missing-branch") {
		t.Fatalf("deve citar a ref ausente:\n%s", msg)
	}
	if !strings.Contains(msg, "hint: git fetch origin origin/missing-branch") {
		t.Fatalf("deve sugerir fetch:\n%s", msg)
	}
	if strings.Contains(msg, "Usage:") {
		t.Fatalf("não deve incluir Usage:\n%s", msg)
	}
}

func TestWrapRangeDiffStripsUsage(t *testing.T) {
	err := errors.New("exit status 129")
	stderr := []byte("fatal: need two commit ranges\n\nusage: git range-diff [<options>] <old-base>..<old-tip> <new-base>..<new-tip>\n   or: git range-diff [<options>] <old-tip>...<new-tip>\n")
	got := Wrap(err, stderr, "range-diff", "abc..abc", "abc..def")
	if got == nil {
		t.Fatal("expected error")
	}
	msg := got.Error()
	if strings.Contains(strings.ToLower(msg), "usage: git range-diff") {
		t.Fatalf("não deve despejar usage do range-diff:\n%s", msg)
	}
	if !strings.Contains(msg, "need two commit ranges") {
		t.Fatalf("deve manter mensagem clara:\n%s", msg)
	}
	if !strings.Contains(msg, "base..approved looks empty") {
		t.Fatalf("deve explicar intervalo vazio:\n%s", msg)
	}
}

func TestOutputMissingRef(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Fatal(err)
	}
	_, err := Output(dir, "rev-parse", "--verify", "no-such-ref-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ref missing: no-such-ref-xyz") {
		t.Fatalf("deve citar ref:\n%s", msg)
	}
	if !strings.Contains(msg, "hint: git fetch origin no-such-ref-xyz") {
		t.Fatalf("deve sugerir fetch:\n%s", msg)
	}
	if !strings.Contains(msg, "stderr:") {
		t.Fatalf("deve incluir stderr:\n%s", msg)
	}
}

func TestDetectMissingRefsFromRevParse(t *testing.T) {
	refs := detectMissingRefs("fatal: Needed a single revision\n", []string{"rev-parse", "--verify", "feature/x"})
	if len(refs) != 1 || refs[0] != "feature/x" {
		t.Fatalf("refs=%v", refs)
	}
}

func TestEmptyRange(t *testing.T) {
	if !emptyRange("abc..abc") {
		t.Fatal("esperado vazio")
	}
	if emptyRange("abc..def") {
		t.Fatal("não deveria ser vazio")
	}
}
