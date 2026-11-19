package egm

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitT runs the test's own git (not egm's) to build fixtures, isolated from
// the developer's config.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	base := []string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	cmd.Env = testGitEnv()
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

func testGitEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
}

func requireUnixGit(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hostile repo fixtures need a POSIX filesystem")
	}
	if _, err := gitPath(); err != nil {
		t.Skip("git not installed in a trusted location")
	}
}

func realpath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo creates a committed repository with docs/a.md and isolates HOME
// in a sibling temp dir, so discovery never reaches the developer's HOME.
func newRepo(t *testing.T) string {
	t.Helper()
	requireUnixGit(t)
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "init", "-q")
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "# A\n")
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-qm", "init")
	return realpath(t, dir)
}

func appendConfig(t *testing.T, file, text string) {
	t.Helper()
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func wantReason(t *testing.T, err error, reason string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want *Error %s, got %v", reason, err)
	}
	if e.Reason != reason || e.Code != ExitError {
		t.Fatalf("want reason %s code 2, got %s code %d (%s)", reason, e.Reason, e.Code, e.Msg)
	}
}

func TestRepoDiscoverPlain(t *testing.T) {
	dir := newRepo(t)
	r, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	gd := filepath.Join(dir, ".git")
	if r.Root != dir || r.GitDir != gd || r.Common != gd {
		t.Fatalf("got %+v", r)
	}
	sum := sha256.Sum256([]byte(gd))
	if r.ID != hex.EncodeToString(sum[:]) {
		t.Fatalf("ID %s", r.ID)
	}
	if len(r.Warnings) != 0 {
		t.Fatalf("warnings %+v", r.Warnings)
	}
}

func TestRepoDiscoverFromDocNotCwd(t *testing.T) {
	dir := newRepo(t)
	other := newRepo(t)
	t.Chdir(other)
	r, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Root != dir {
		t.Fatalf("root %s, want %s", r.Root, dir)
	}
}

func TestRepoDiscoverFreshInitNoIndex(t *testing.T) {
	requireUnixGit(t)
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	gitT(t, dir, "init", "-q")
	if _, err := os.Lstat(filepath.Join(dir, ".git", "index")); err == nil {
		t.Fatal("fresh init should have no index")
	}
	if _, err := DiscoverRepo(filepath.Join(dir, "doc.md")); err != nil {
		t.Fatal(err)
	}
}

func TestRepoReftableAccepted(t *testing.T) {
	requireUnixGit(t)
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "--ref-format=reftable")
	r, err := DiscoverRepo(filepath.Join(dir, "doc.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 0 {
		t.Fatalf("warnings %+v", r.Warnings)
	}
}

func TestRepoNoRepositoryUpToHome(t *testing.T) {
	requireUnixGit(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	// A versioned $HOME (dotfiles) must not adopt a downloaded document.
	gitT(t, home, "init", "-q")
	doc := filepath.Join(home, "Downloads", "x", "doc.md")
	writeFile(t, doc, "# x\n")
	_, err := DiscoverRepo(doc)
	wantReason(t, err, "no_repository")
}

func TestRepoNoRepositoryAtAll(t *testing.T) {
	requireUnixGit(t)
	t.Setenv("HOME", "/nonexistent-home")
	doc := filepath.Join(t.TempDir(), "doc.md")
	_, err := DiscoverRepo(doc)
	wantReason(t, err, "no_repository")
}

func TestRepoDotGitSymlinkRefused(t *testing.T) {
	dir := newRepo(t)
	moved := filepath.Join(t.TempDir(), "real.git")
	if err := os.Rename(filepath.Join(dir, ".git"), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	_, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	wantReason(t, err, "hostile_git_dir")
}

func TestRepoCommondirInGitDirRefused(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, filepath.Join(dir, ".git", "commondir"), "/elsewhere\n")
	_, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	wantReason(t, err, "hostile_git_dir")
}

func addWorktree(t *testing.T, dir string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "wt")
	gitT(t, dir, "worktree", "add", "-q", wt)
	return realpath(t, wt)
}

func TestRepoWorktreeAccepted(t *testing.T) {
	dir := newRepo(t)
	wt := addWorktree(t, dir)
	r, err := DiscoverRepo(filepath.Join(wt, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	main, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Root != wt || r.Common != main.Common || r.ID != main.ID {
		t.Fatalf("worktree %+v main %+v", r, main)
	}
	if filepath.Dir(r.GitDir) != filepath.Join(main.Common, "worktrees") {
		t.Fatalf("gitdir %s", r.GitDir)
	}
}

func TestRepoGitdirWithoutBackPointerRefused(t *testing.T) {
	dir := newRepo(t)
	wt := addWorktree(t, dir)
	r, err := DiscoverRepo(filepath.Join(wt, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(r.GitDir, "gitdir")); err != nil {
		t.Fatal(err)
	}
	_, err = DiscoverRepo(filepath.Join(wt, "docs", "a.md"))
	wantReason(t, err, "hostile_git_dir")
}

func TestRepoGitdirPointingElsewhereRefused(t *testing.T) {
	dir := newRepo(t)
	wt := addWorktree(t, dir)
	// A copy of the worktree's .git file in another directory: the
	// back-pointer names the original worktree, not this one.
	copyDir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(wt, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(copyDir, ".git"), string(data))
	_, err = DiscoverRepo(filepath.Join(copyDir, "doc.md"))
	wantReason(t, err, "hostile_git_dir")

	// gitdir: straight to another repository's .git (no commondir).
	other := newRepo(t)
	writeFile(t, filepath.Join(copyDir, ".git"), "gitdir: "+filepath.Join(other, ".git")+"\n")
	_, err = DiscoverRepo(filepath.Join(copyDir, "doc.md"))
	wantReason(t, err, "hostile_git_dir")
}

func TestRepoSubmoduleUnsupported(t *testing.T) {
	super := newRepo(t)
	modDir := filepath.Join(super, ".git", "modules", "sub")
	gitT(t, super, "init", "-q", "--bare", modDir)
	sub := filepath.Join(super, "sub")
	writeFile(t, filepath.Join(sub, ".git"), "gitdir: ../.git/modules/sub\n")
	_, err := DiscoverRepo(filepath.Join(sub, "doc.md"))
	wantReason(t, err, "submodule_unsupported")
}

func mkfifo(t *testing.T, p string) {
	t.Helper()
	if err := exec.Command("mkfifo", p).Run(); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
}

func TestRepoHostileEntriesRefused(t *testing.T) {
	cases := map[string]func(t *testing.T, gd string){
		"objects symlink": func(t *testing.T, gd string) { replaceWithSymlink(t, filepath.Join(gd, "objects")) },
		"refs symlink":    func(t *testing.T, gd string) { replaceWithSymlink(t, filepath.Join(gd, "refs")) },
		"index symlink":   func(t *testing.T, gd string) { replaceWithSymlink(t, filepath.Join(gd, "index")) },
		"packed-refs fifo": func(t *testing.T, gd string) {
			_ = os.Remove(filepath.Join(gd, "packed-refs"))
			mkfifo(t, filepath.Join(gd, "packed-refs"))
		},
		"sharedindex fifo": func(t *testing.T, gd string) { mkfifo(t, filepath.Join(gd, "sharedindex.abc")) },
		"info/exclude symlink": func(t *testing.T, gd string) {
			replaceWithSymlink(t, filepath.Join(gd, "info", "exclude"))
		},
		"HEAD too big": func(t *testing.T, gd string) {
			writeFile(t, filepath.Join(gd, "HEAD"), "ref: refs/heads/main\n"+strings.Repeat(" ", maxHeadSize))
		},
		"config too big": func(t *testing.T, gd string) {
			appendConfig(t, filepath.Join(gd, "config"), "#"+strings.Repeat("x", maxConfigSize)+"\n")
		},
		"alternates": func(t *testing.T, gd string) {
			writeFile(t, filepath.Join(gd, "objects", "info", "alternates"), "/elsewhere/objects\n")
		},
		"http-alternates": func(t *testing.T, gd string) {
			writeFile(t, filepath.Join(gd, "objects", "info", "http-alternates"), "http://x/objects\n")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t)
			mutate(t, filepath.Join(dir, ".git"))
			_, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
			wantReason(t, err, "hostile_git_dir")
		})
	}
}

func replaceWithSymlink(t *testing.T, p string) {
	t.Helper()
	moved := filepath.Join(t.TempDir(), filepath.Base(p))
	if err := os.Rename(p, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, p); err != nil {
		t.Fatal(err)
	}
}

// canary returns a script that records its execution in a sentinel file
// outside the repository, and that sentinel's path.
func canary(t *testing.T) (script, sentinel string) {
	t.Helper()
	d := t.TempDir()
	sentinel = filepath.Join(d, "SENTINEL")
	script = filepath.Join(d, "canary.sh")
	body := "#!/bin/sh\necho \"$0 $*\" >> '" + sentinel + "'\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil { //nolint:gosec // test canary must be executable
		t.Fatal(err)
	}
	return script, sentinel
}

func assertNoSentinel(t *testing.T, sentinel string) {
	t.Helper()
	if data, err := os.ReadFile(sentinel); err == nil {
		t.Fatalf("hostile config executed: %s", data)
	}
}

func TestRepoFatalConfigRefused(t *testing.T) {
	cases := map[string]string{
		"include.path":     "[include]\n\tpath = /etc/hostile\n",
		"includeIf":        "[includeIf \"gitdir:/\"]\n\tpath = /etc/hostile\n",
		"partialClone":     "[extensions]\n\tpartialClone = origin\n[remote \"origin\"]\n\turl = file:///nonexistent\n\tpromisor = true\n\tuploadpack = CANARY\n",
		"promisor alone":   "[remote \"origin\"]\n\tpromisor = true\n",
		"unknown ext":      "[extensions]\n\tfrobnicate = true\n",
		"case-insensitive": "[EXTENSIONS]\n\tPartialClone = origin\n",
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t)
			script, sentinel := canary(t)
			appendConfig(t, filepath.Join(dir, ".git", "config"), strings.ReplaceAll(cfg, "CANARY", script))
			_, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
			wantReason(t, err, "hostile_git_dir")
			assertNoSentinel(t, sentinel)
		})
	}
}

func TestRepoFatalConfigWorktreeFile(t *testing.T) {
	dir := newRepo(t)
	gitT(t, dir, "config", "extensions.worktreeConfig", "true")
	writeFile(t, filepath.Join(dir, ".git", "config.worktree"), "[include]\n\tpath = /etc/hostile\n")
	_, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	wantReason(t, err, "hostile_git_dir")
}

func TestRepoRealWorldConfigAccepted(t *testing.T) {
	cases := []struct {
		name, cfg, warn string
	}{
		{"husky", "[core]\n\thooksPath = .husky/_\n", "core.hookspath"},
		{"git-lfs", "[filter \"lfs\"]\n\tclean = git-lfs clean -- %f\n\tsmudge = git-lfs smudge -- %f\n\tprocess = git-lfs filter-process\n\trequired = true\n", "filter.lfs.clean"},
		{"vscode", "[branch \"main\"]\n\tvscode-merge-base = origin/main\n", ""},
		{"scalar", "[core]\n\tfsmonitor = true\n\tuntrackedCache = true\n", "core.fsmonitor"},
		{"common keys", "[core]\n\tsparseCheckout = true\n[index]\n\tversion = 4\n[pack]\n\tthreads = 1\n[extensions]\n\tworktreeConfig = true\n", ""},
		{"extraheader", "[http \"https://example.com/\"]\n\textraheader = AUTHORIZATION: x\n", "http.https://example.com/.extraheader"},
		{"npm merge driver", "[merge \"npm-merge-driver\"]\n\tname = npm\n\tdriver = npx npm-merge-driver merge %A %O %B %P\n", "merge.npm-merge-driver.driver"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := newRepo(t)
			appendConfig(t, filepath.Join(dir, ".git", "config"), c.cfg)
			r, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
			if err != nil {
				t.Fatal(err)
			}
			if c.warn == "" {
				if len(r.Warnings) != 0 {
					t.Fatalf("unexpected warnings %+v", r.Warnings)
				}
				return
			}
			if len(r.Warnings) != 1 || r.Warnings[0].Code != "git_config_neutralized" || !strings.Contains(r.Warnings[0].Message, c.warn) {
				t.Fatalf("want git_config_neutralized with %s, got %+v", c.warn, r.Warnings)
			}
		})
	}
}
