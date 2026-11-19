package egm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// hostileConfig wires every neutralized key to the canary script.
func hostileConfig(script, otherWorktree string) string {
	return strings.NewReplacer("CANARY", script, "OTHER", otherWorktree).Replace(`[core]
	fsmonitor = CANARY
	sshCommand = CANARY
	pager = CANARY
	editor = CANARY
	askPass = CANARY
	alternateRefsCommand = CANARY
	hooksPath = HOOKS
	worktree = OTHER
[diff]
	external = CANARY
[diff "x"]
	textconv = CANARY
	command = CANARY
[filter "x"]
	clean = CANARY
	smudge = CANARY
	process = CANARY
	required = true
[merge "x"]
	driver = CANARY
[credential]
	helper = !CANARY
[pager]
	diff = CANARY
`)
}

// newHostileRepo returns a repository with two commits, an untracked file,
// a modified file, .gitattributes "* filter=x diff=x" and every neutralized
// key pointing at a canary.
func newHostileRepo(t *testing.T) (dir, other, sentinel string) {
	t.Helper()
	dir = newRepo(t)
	writeFile(t, filepath.Join(dir, ".gitattributes"), "* filter=x diff=x merge=x\n")
	writeFile(t, filepath.Join(dir, ".gitignore"), "*.log\n")
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "# A\nchanged\n")
	writeFile(t, filepath.Join(dir, "docs", "b.md"), "# B\n")
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-qm", "second")
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "# A\nworktree edit\n")
	writeFile(t, filepath.Join(dir, "docs", "new.md"), "# new\n")
	writeFile(t, filepath.Join(dir, "docs", "x.log"), "log\n")

	script, sentinel := canary(t)
	hooks := filepath.Join(t.TempDir(), "hooks")
	for _, h := range []string{"pre-commit", "post-checkout", "post-index-change", "reference-transaction", "fsmonitor-watchman"} {
		data, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(hooks, h), string(data))
		if err := os.Chmod(filepath.Join(hooks, h), 0o755); err != nil { //nolint:gosec // hook canary must be executable
			t.Fatal(err)
		}
	}
	other = t.TempDir()
	writeFile(t, filepath.Join(other, "docs", "elsewhere.md"), "# elsewhere\n")
	cfg := strings.ReplaceAll(hostileConfig(script, other), "HOOKS", hooks)
	appendConfig(t, filepath.Join(dir, ".git", "config"), cfg)
	return dir, other, sentinel
}

func TestGitHostileConfigExecutesNothing(t *testing.T) {
	dir, _, sentinel := newHostileRepo(t)
	r, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Code != "git_config_neutralized" {
		t.Fatalf("warnings %+v", r.Warnings)
	}
	exerciseAllCalls(t, r)
	assertNoSentinel(t, sentinel)

	// Discrimination: the same repository does run the canary under a plain
	// git, so a missing sentinel above is evidence, not a broken fixture.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = dir
	cmd.Env = testGitEnv()
	_ = cmd.Run()
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("canary fixture is not discriminating: plain git diff did not run it")
	}
}

func exerciseAllCalls(t *testing.T, r *Repo) {
	t.Helper()
	ctx := context.Background()
	steps := map[string]func() error{
		"ls-files -s":     func() error { _, err := r.LsFilesStage(ctx, "docs/a.md", "docs/b.md"); return err },
		"ls-files others": func() error { _, err := r.LsFilesOthers(ctx, "docs"); return err },
		"ls-tree":         func() error { _, err := r.LsTree(ctx, "HEAD", "docs/a.md"); return err },
		"ls-tree -r":      func() error { _, err := r.LsTreeNames(ctx, "HEAD", "docs", 100); return err },
		"hash-object":     func() error { _, err := r.HashObject(ctx, "docs/a.md"); return err },
		"cat-file blob":   func() error { _, err := r.CatFileBlob(ctx, "HEAD", "docs/a.md", 1<<20); return err },
		"merge-base":      func() error { _, err := r.MergeBase(ctx, "HEAD~1", "HEAD"); return err },
		"diff":            func() error { _, err := r.DiffNameStatus(ctx, "HEAD~1", "HEAD"); return err },
		"check-ignore":    func() error { _, err := r.CheckIgnore(ctx, "docs/x.log", "docs/a.md"); return err },
		"diff --no-index": func() error { return diffNoIndexIn(ctx, r.Root) },
	}
	for name, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// diffNoIndexIn diffs two files with .gitattributes "* diff=x" next to them.
func diffNoIndexIn(ctx context.Context, root string) error {
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(state, ".gitattributes"), []byte("* diff=x\n"), 0o644)
	_ = os.WriteFile(filepath.Join(state, "a"), []byte("1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(state, "b"), []byte("2\n"), 0o644)
	_, err := DiffNoIndex(ctx, state, "a", "b")
	return err
}

func TestGitExplicitWorkTreeBeatsCoreWorktree(t *testing.T) {
	dir, _, _ := newHostileRepo(t)
	r, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	others, err := r.LsFilesOthers(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(others, ",") != "docs/new.md" {
		t.Fatalf("others %v (core.worktree must have no effect)", others)
	}
}

func TestGitCalls(t *testing.T) {
	dir, _, _ := newHostileRepo(t)
	r, err := DiscoverRepo(filepath.Join(dir, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t.Run("ls-files -s", func(t *testing.T) { checkLsFiles(ctx, t, r) })
	t.Run("ls-tree", func(t *testing.T) { checkLsTree(ctx, t, r) })
	t.Run("hash-object", func(t *testing.T) { checkHashObject(ctx, t, r) })
	t.Run("cat-file", func(t *testing.T) { checkCatFile(ctx, t, r) })
	t.Run("revs", func(t *testing.T) { checkRevs(ctx, t, r) })
	t.Run("check-ignore", func(t *testing.T) { checkCheckIgnore(ctx, t, r) })
	t.Run("diff --no-index", func(t *testing.T) { checkDiffNoIndex(ctx, t) })
}

func checkLsFiles(ctx context.Context, t *testing.T, r *Repo) {
	got, err := r.LsFilesStage(ctx, "docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	want := gitT(t, r.Root, "rev-parse", ":docs/a.md")
	if len(got) != 1 || got[0].Path != "docs/a.md" || got[0].Mode != "100644" || got[0].Object != want {
		t.Fatalf("got %+v want object %s", got, want)
	}
}

func checkLsTree(ctx context.Context, t *testing.T, r *Repo) {
	got, err := r.LsTree(ctx, "HEAD", "docs/b.md")
	if err != nil {
		t.Fatal(err)
	}
	want := gitT(t, r.Root, "rev-parse", "HEAD:docs/b.md")
	if len(got) != 1 || got[0].Object != want || got[0].Mode != "100644" {
		t.Fatalf("got %+v", got)
	}
	names, err := r.LsTreeNames(ctx, "HEAD", "docs", 2)
	if err != nil || strings.Join(names, ",") != "docs/a.md,docs/b.md" {
		t.Fatalf("names %v err %v", names, err)
	}
	_, err = r.LsTreeNames(ctx, "HEAD", "docs", 1)
	wantReason(t, err, "too_many_entries")
}

func checkHashObject(ctx context.Context, t *testing.T, r *Repo) {
	got, err := r.HashObject(ctx, "docs/a.md", "docs/new.md")
	if err != nil {
		t.Fatal(err)
	}
	// The raw bytes' hash: the filter=x clean canary must not have applied.
	want := gitT(t, r.Root, "hash-object", "--no-filters", "docs/a.md")
	if len(got) != 2 || got[0] != want {
		t.Fatalf("got %v want %s", got, want)
	}
}

func checkCatFile(ctx context.Context, t *testing.T, r *Repo) {
	n, err := r.CatFileSize(ctx, "HEAD", "docs/a.md")
	if err != nil || n != int64(len("# A\nchanged\n")) {
		t.Fatalf("size %d err %v", n, err)
	}
	b, err := r.CatFileBlob(ctx, "HEAD", "docs/a.md", 100)
	if err != nil || string(b) != "# A\nchanged\n" {
		t.Fatalf("blob %q err %v", b, err)
	}
	_, err = r.CatFileBlob(ctx, "HEAD", "docs/a.md", 3)
	wantReason(t, err, "blob_too_large")
	_, err = r.CatFileSize(ctx, "HEAD", "./docs/a.md")
	wantReason(t, err, "git_forbidden")
}

func checkRevs(ctx context.Context, t *testing.T, r *Repo) {
	head, err := r.RevParse(ctx, "HEAD")
	if err != nil || head != gitT(t, r.Root, "rev-parse", "HEAD") {
		t.Fatalf("head %s err %v", head, err)
	}
	base, err := r.MergeBase(ctx, "HEAD~1", "HEAD")
	if err != nil || base != gitT(t, r.Root, "rev-parse", "HEAD~1") {
		t.Fatalf("base %s err %v", base, err)
	}
	ns, err := r.DiffNameStatus(ctx, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range ns {
		got[e.Path] = e.Status
	}
	if got["docs/a.md"] != "M" || got["docs/b.md"] != "A" || got[".gitattributes"] != "A" {
		t.Fatalf("name-status %+v", ns)
	}
	for _, bad := range []string{"", "-n", "--output=/tmp/x", "HEAD\nx", "nope-not-a-rev"} {
		_, err := r.RevParse(ctx, bad)
		wantReason(t, err, "bad_revision")
	}
}

func checkCheckIgnore(ctx context.Context, t *testing.T, r *Repo) {
	got, err := r.CheckIgnore(ctx, "docs/x.log", "docs/a.md", "docs/we\"ird.log", ":(top)z.log", "docs/é.log")
	if err != nil {
		t.Fatal(err)
	}
	if !got["docs/x.log"] || got["docs/a.md"] || !got["docs/we\"ird.log"] || !got[":(top)z.log"] || !got["docs/é.log"] {
		t.Fatalf("got %v", got)
	}
	got, err = r.CheckIgnore(ctx, "docs/a.md")
	if err != nil || got["docs/a.md"] {
		t.Fatalf("none ignored: %v err %v", got, err)
	}
}

func checkDiffNoIndex(ctx context.Context, t *testing.T) {
	state := t.TempDir()
	writeFile(t, filepath.Join(state, "a"), "1\n2\n")
	writeFile(t, filepath.Join(state, "b"), "1\n3\n")
	out, err := DiffNoIndex(ctx, state, "a", "b")
	if err != nil || !strings.Contains(string(out), "@@ -2 +2 @@") {
		t.Fatalf("out %q err %v", out, err)
	}
	out, err = DiffNoIndex(ctx, state, "a", "a")
	if err != nil || len(out) != 0 {
		t.Fatalf("identical: %q err %v", out, err)
	}
}

func TestGitRejectsUnsafeArgs(t *testing.T) {
	r := &Repo{Root: "/nonexistent", GitDir: "/nonexistent/.git"}
	ctx := context.Background()
	_, err := r.LsFilesStage(ctx)
	wantReason(t, err, "git_forbidden")
	for _, p := range []string{"/etc/passwd", "../x", "a/../b", "a//b", "a/", "", "a\x00b", `a\b`} {
		_, err := r.LsFilesStage(ctx, p)
		wantReason(t, err, "git_forbidden")
	}
	_, err = r.HashObject(ctx)
	wantReason(t, err, "git_forbidden")
	_, err = r.CheckIgnore(ctx)
	wantReason(t, err, "git_forbidden")
}

func TestGitEnvStripsInheritedGit(t *testing.T) {
	t.Setenv("GIT_DIR", "/hostile")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.fsmonitor=/hostile'")
	t.Setenv("GIT_EXTERNAL_DIFF", "/hostile")
	env := gitEnv("GIT_DIR=/good")
	seen := map[string]bool{}
	for _, kv := range env {
		if strings.Contains(kv, "/hostile") {
			t.Fatalf("inherited %s", kv)
		}
		seen[kv] = true
	}
	for _, kv := range append(append([]string{}, gitFixedEnv...), "GIT_DIR=/good") {
		if !seen[kv] {
			t.Fatalf("missing %s", kv)
		}
	}
	for _, kv := range []string{"GIT_LITERAL_PATHSPECS=1", "GIT_NO_LAZY_FETCH=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_ASKPASS="} {
		if !seen[kv] {
			t.Fatalf("missing %s", kv)
		}
	}
}

func TestGitArgvShape(t *testing.T) {
	r := &Repo{Root: "/root", GitDir: "/root/.git"}
	argv, err := r.call(opDiffNameStatus, "--end-of-options", "a", "b", "--").argv()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"-c core.fsmonitor=false", "-c core.hooksPath=/dev/null", "-c diff.external=", "-c protocol.allow=never", "-c safe.directory=/root", "diff --name-status --no-renames -z --no-ext-diff --no-textconv --end-of-options a b --"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q lacks %q", joined, want)
		}
	}
}

func TestGitClosedList(t *testing.T) {
	allowed := map[string]bool{"ls-files": true, "ls-tree": true, "hash-object": true, "cat-file": true, "rev-parse": true, "merge-base": true, "diff": true, "check-ignore": true, "config": true}
	if len(gitOps) != 13 {
		t.Fatalf("gitOps has %d shapes, the R4c table has 13", len(gitOps))
	}
	for op, prefix := range gitOps {
		if !allowed[prefix[0]] {
			t.Fatalf("op %d runs %q, not in R4c", op, prefix[0])
		}
	}
	_, err := runGit(context.Background(), &gitCall{op: gitOp(999)})
	wantReason(t, err, "git_forbidden")
}

func TestGitTimeout(t *testing.T) {
	requireUnixGit(t)
	fake := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil { //nolint:gosec // fake git must be executable
		t.Fatal(err)
	}
	oldPath, oldTimeout := gitPath, gitTimeout
	t.Cleanup(func() { gitPath, gitTimeout = oldPath, oldTimeout })
	gitPath = func() (string, error) { return fake, nil }
	gitTimeout = 200 * time.Millisecond
	dir := t.TempDir()
	r := &Repo{Root: dir, GitDir: filepath.Join(dir, ".git")}
	start := time.Now()
	_, err := r.RevParse(context.Background(), "HEAD")
	// RevParse maps failures to bad_revision; check the raw call too.
	_, rawErr := runGit(context.Background(), r.call(opRevParse, "--end-of-options", "HEAD"))
	wantReason(t, rawErr, "git_timeout")
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("hung git not bounded: err %v after %s", err, time.Since(start))
	}
}

func TestGitResolveBinaryIsAbsolute(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p, err := resolveGit()
	if err != nil {
		t.Skip("no git in trusted locations")
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("git path %s", p)
	}
}

// TestGitOnlyInGitGo is the static guard of R4c: no other non-test file of
// the package may execute git, so the closed list in git.go is the only way.
func TestGitOnlyInGitGo(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`(?i)exec\.Command(Context)?\([^)]*git`),
		regexp.MustCompile(`"(/usr/bin/|/bin/|/usr/local/bin/)?git"`),
		regexp.MustCompile(`\brunGit\(`),
		regexp.MustCompile(`\bgitPath\(`),
	}
	for _, f := range files {
		if f == "git.go" || strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, re := range forbidden {
			if loc := re.Find(data); loc != nil {
				t.Errorf("%s runs git outside git.go: %q", f, loc)
			}
		}
	}
}
