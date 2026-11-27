package stack

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// branchRepo builds main with two commits and a feature branch with two
// more, so HEAD~1 and the merge-base with main differ.
func branchRepo(t *testing.T) (dir, mainTip string) {
	t.Helper()
	dir = gitRepo(t)
	runGit(t, dir, "checkout", "-q", "-B", "main")
	// git init's branch depends on init.defaultBranch (master on CI runners):
	// leave main as the only default-branch candidate.
	for _, b := range defaultBranchCandidates {
		if b != "main" && !strings.Contains(b, "/") {
			_ = exec.Command("git", "-C", dir, "branch", "-q", "-D", b).Run()
		}
	}
	for _, c := range []string{"m1", "m2"} {
		writeFile(t, filepath.Join(dir, c+".txt"), c+"\n")
		runGit(t, dir, "add", "-A")
		runGit(t, dir, "commit", "-q", "-m", c)
	}
	mainTip = revParse(t, dir, "HEAD")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	for _, c := range []string{"f1", "f2"} {
		writeFile(t, filepath.Join(dir, c+".txt"), c+"\n")
		runGit(t, dir, "add", "-A")
		runGit(t, dir, "commit", "-q", "-m", c)
	}
	return dir, mainTip
}

func prepareBase(t *testing.T, opts *Options) *Shared {
	t.Helper()
	shared, err := Prepare(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return shared
}

func TestStackBaseDefaultsToMergeBase(t *testing.T) {
	dir, mainTip := branchRepo(t)
	shared := prepareBase(t, &Options{RepoDir: dir})
	if shared.Base != mainTip || shared.BaseSource != BaseSourceMergeBase {
		t.Fatalf("base=%s source=%s, want merge-base %s", shared.Base, shared.BaseSource, mainTip)
	}
	if len(shared.Changed) != 2 {
		t.Fatalf("changed=%v, want both branch commits' files", shared.Changed)
	}
}

func TestStackBasePrefersOriginHEAD(t *testing.T) {
	dir, _ := branchRepo(t)
	// origin/main sits at m1, local main at m2: origin/HEAD wins.
	m1 := revParse(t, dir, "main~1")
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", m1)
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	shared := prepareBase(t, &Options{RepoDir: dir})
	if shared.Base != m1 || shared.BaseSource != BaseSourceMergeBase {
		t.Fatalf("base=%s source=%s, want origin/HEAD merge-base %s", shared.Base, shared.BaseSource, m1)
	}
}

func TestStackBaseOnDefaultBranchFallsBackToParent(t *testing.T) {
	dir, mainTip := branchRepo(t)
	runGit(t, dir, "checkout", "-q", "main")
	shared := prepareBase(t, &Options{RepoDir: dir})
	want := revParse(t, dir, mainTip+"~1")
	if shared.Base != want || shared.BaseSource != BaseSourceParent {
		t.Fatalf("base=%s source=%s, want HEAD~1 %s", shared.Base, shared.BaseSource, want)
	}
}

func TestStackBaseWithoutDefaultBranchFallsBackToParent(t *testing.T) {
	dir, _ := branchRepo(t)
	runGit(t, dir, "branch", "-q", "-m", "main", "not-a-default-branch")
	shared := prepareBase(t, &Options{RepoDir: dir})
	if want := revParse(t, dir, "HEAD~1"); shared.Base != want || shared.BaseSource != BaseSourceParent {
		t.Fatalf("base=%s source=%s, want HEAD~1 %s", shared.Base, shared.BaseSource, want)
	}
}

func TestStackBaseExplicitRefs(t *testing.T) {
	dir, mainTip := branchRepo(t)
	shared := prepareBase(t, &Options{RepoDir: dir, Base: "HEAD~1"})
	if want := revParse(t, dir, "HEAD~1"); shared.Base != want || shared.BaseSource != BaseSourceParent {
		t.Fatalf("explicit HEAD~1: base=%s source=%s, want %s", shared.Base, shared.BaseSource, want)
	}
	shared = prepareBase(t, &Options{RepoDir: dir, Base: "main~1"})
	if want := revParse(t, dir, mainTip+"~1"); shared.Base != want || shared.BaseSource != BaseSourceFlag {
		t.Fatalf("explicit ref: base=%s source=%s, want %s", shared.Base, shared.BaseSource, want)
	}
}

func TestStackBaseTicketBaseCommitWins(t *testing.T) {
	dir, mainTip := branchRepo(t)
	m1 := revParse(t, dir, mainTip+"~1")
	ticket := filepath.Join(t.TempDir(), "rd.md")
	if err := os.WriteFile(ticket, []byte("---\nbase_commit: "+m1+"\n---\n# t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"", "HEAD~1"} {
		shared := prepareBase(t, &Options{RepoDir: dir, Base: base, Ticket: ticket})
		if shared.Base != m1 || shared.BaseSource != BaseSourceTicket {
			t.Fatalf("base %q: got %s source=%s, want ticket %s", base, shared.Base, shared.BaseSource, m1)
		}
	}
}

func TestStackResultReportsBase(t *testing.T) {
	dir, mainTip := branchRepo(t)
	res, err := Run(context.Background(), &Options{RepoDir: dir, Ops: []Op{OpGuard}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Base != mainTip || res.BaseSource != BaseSourceMergeBase {
		t.Fatalf("result base=%s source=%s, want %s merge-base", res.Base, res.BaseSource, mainTip)
	}
}
