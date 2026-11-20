package stack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStackVaultOp: the vault op runs tb vault check; a broken link fails
// the step, a fixed vault passes, and the op runs in parallel.
func TestStackVaultOp(t *testing.T) {
	dir := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt", "vault"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".toolbelt", "vault", "a.md"), "# A\n[[missing]]\n")
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-qm", "base")
	writeFile(t, filepath.Join(dir, "a.txt"), "b\n")
	runGit(t, dir, "commit", "-qam", "change")
	if !isParallelOp(OpVault) {
		t.Fatal("vault should run in parallel")
	}
	res, err := Run(context.Background(), &Options{RepoDir: dir, Base: "HEAD~1", Head: "HEAD", Ops: []Op{OpVault}})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || res.Steps[0].ExitCode != 1 || !strings.Contains(res.Steps[0].Reason, "vault") {
		t.Fatalf("broken link: %+v", res.Steps[0])
	}
	writeFile(t, filepath.Join(dir, ".toolbelt", "vault", "missing.md"), "# M\n")
	res, err = Run(context.Background(), &Options{RepoDir: dir, Base: "HEAD~1", Head: "HEAD", Ops: []Op{OpVault}})
	if err != nil || !res.OK {
		t.Fatalf("fixed: %+v %v", res, err)
	}
}
