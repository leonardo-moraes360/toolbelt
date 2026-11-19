package stack

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEgmLast(t *testing.T) {
	got := egmLast([]Op{OpEgm, OpVerify, OpGuard, OpEgm, OpReview})
	want := []Op{OpVerify, OpGuard, OpReview, OpEgm}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("egmLast = %v, want %v", got, want)
	}
	if got := egmLast([]Op{OpVerify}); !reflect.DeepEqual(got, []Op{OpVerify}) {
		t.Fatalf("no egm: %v", got)
	}
	if _, err := ParseOp("egm"); err != nil {
		t.Fatal(err)
	}
	if isParallelOp(OpEgm) {
		t.Fatal("egm must not be parallel")
	}
}

// TestStackEgmOp runs the egm op end to end: moved to the end, an empty
// selection passes with warn (Result.warn is the OR), a touched spec without
// state fails with NO_STATE.
func TestStackEgmOp(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir := gitRepo(t)
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "a.txt"), "b\n")
	runGit(t, dir, "commit", "-qam", "no docs")

	res, err := Run(context.Background(), &Options{RepoDir: dir, Base: "HEAD~1", Head: "HEAD", Ops: []Op{OpEgm}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.Warn || len(res.Steps) != 1 || !res.Steps[0].Warn {
		t.Fatalf("empty selection: %+v", res)
	}
	res, err = Run(context.Background(), &Options{RepoDir: dir, Base: "HEAD~1", Head: "HEAD", Ops: []Op{OpEgm}, Strict: true})
	if err != nil || res.OK || res.Warn {
		t.Fatalf("strict empty: %+v %v", res, err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "docs", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "docs", "adr", "001-x.md"), "# ADR\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-qm", "adr")
	res, err = Run(context.Background(), &Options{RepoDir: dir, Base: "HEAD~1", Head: "HEAD", Ops: []Op{OpEgm, OpGuard}})
	if err != nil {
		t.Fatal(err)
	}
	last := res.Steps[len(res.Steps)-1]
	if res.OK || last.Op != string(OpEgm) || last.ExitCode != 1 || !strings.Contains(last.Reason, "NO_STATE") {
		t.Fatalf("adr without state: %+v", res)
	}
}
