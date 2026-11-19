package egm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// Size caps of the inspection of .git (R4d).
const (
	maxConfigSize  = 1 << 20
	maxHeadSize    = 4 << 10
	maxPointerSize = 4 << 10 // gitdir:, commondir and gitdir back-pointer files
)

// inspectedEntries cannot be a symlink, FIFO or device in <common> or
// <gitdir> (R4d); sharedindex.* is matched separately.
var inspectedEntries = []string{
	"objects", "objects/pack", "objects/info", "refs", "packed-refs", "info",
	"worktrees", "modules", "HEAD", "index", "config", "config.worktree",
	"shallow", "commondir", "gitdir", "info/exclude", "info/attributes",
	"info/sparse-checkout",
}

// Repo is a repository located and vetted by R4d. All its git calls go
// through the R4c shapes in git.go.
type Repo struct {
	Root     string    // real root: realpath of the dir containing .git
	GitDir   string    // realpath of the git dir (worktree gitdir or .git)
	Common   string    // realpath of the common git dir
	ID       string    // hex sha256 of Common (repo-id, R21)
	Warnings []Warning // e.g. git_config_neutralized (Message lists keys)
}

func hostile(format string, a ...any) *Error {
	return errorf(ExitError, "hostile_git_dir", format, a...)
}

// DiscoverRepo locates the repository of docPath through the filesystem,
// without asking git, starting from the document's directory (not the cwd),
// and vets its .git before any git call (R4d).
func DiscoverRepo(docPath string) (*Repo, error) {
	start, err := docDir(docPath)
	if err != nil {
		return nil, err
	}
	root, dotGit, err := findDotGit(start)
	if err != nil {
		return nil, err
	}
	r := &Repo{Root: root}
	if err := r.resolveGitDirs(dotGit); err != nil {
		return nil, err
	}
	if err := r.inspect(); err != nil {
		return nil, err
	}
	if err := r.checkConfig(context.Background()); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(r.Common))
	r.ID = hex.EncodeToString(sum[:])
	return r, nil
}

func docDir(docPath string) (string, error) {
	abs, err := filepath.Abs(docPath)
	if err != nil {
		return "", errorf(ExitError, "no_repository", "%s: %v", docPath, err)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", errorf(ExitError, "no_repository", "%s: %v", docPath, err)
	}
	return dir, nil
}

// realHome is where discovery stops: a versioned $HOME (dotfiles) must not
// adopt every downloaded document below it.
func realHome() string {
	h := os.Getenv("HOME")
	if h == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(h); err == nil {
		return resolved
	}
	return filepath.Clean(h)
}

// findDotGit walks up from start until a .git entry, stopping at $HOME
// (not inspected) and when the filesystem changes.
func findDotGit(start string) (root string, dotGit fs.FileInfo, err error) {
	home := realHome()
	cur := start
	dev, _ := deviceOf(cur)
	for {
		if home != "" && cur == home {
			return "", nil, errorf(ExitError, "no_repository", "no .git above %s up to $HOME", start)
		}
		fi, err := os.Lstat(filepath.Join(cur, ".git"))
		if err == nil {
			return cur, fi, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", nil, hostile("%s/.git: %v", cur, err)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", nil, errorf(ExitError, "no_repository", "no .git above %s", start)
		}
		if pdev, ok := deviceOf(parent); !ok || pdev != dev {
			return "", nil, errorf(ExitError, "no_repository", "no .git above %s on the same filesystem", start)
		}
		cur = parent
	}
}

// deviceOf returns st_dev through reflection, so this file builds on every
// GOOS without a build-tagged helper; where there is no Dev field (Windows)
// every path reports the same device and only $HOME and / stop the walk.
func deviceOf(p string) (uint64, bool) {
	fi, err := os.Lstat(p)
	if err != nil {
		return 0, false
	}
	v := reflect.ValueOf(fi.Sys())
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0, true
	}
	f := v.FieldByName("Dev")
	switch {
	case !f.IsValid():
		return 0, true
	case f.CanUint():
		return f.Uint(), true
	case f.CanInt():
		return uint64(f.Int()), true //nolint:gosec // device ids are compared, never used as sizes
	}
	return 0, true
}

func (r *Repo) resolveGitDirs(dotGit fs.FileInfo) error {
	p := filepath.Join(r.Root, ".git")
	m := dotGit.Mode()
	switch {
	case m&fs.ModeSymlink != 0:
		return hostile("%s is a symlink", p)
	case m.IsDir():
		if _, err := os.Lstat(filepath.Join(p, "commondir")); err == nil {
			return hostile("%s/commondir inside a .git directory", p)
		}
		r.GitDir, r.Common = p, p
		return nil
	case m.IsRegular():
		return r.resolveGitFile(p)
	}
	return hostile("%s is not a directory or regular file", p)
}

// resolveGitFile follows a "gitdir:" file: only a linked worktree of a
// repository whose gitdir points back to this .git is accepted.
func (r *Repo) resolveGitFile(dotGit string) error {
	target, err := readPointer(dotGit, r.Root, "gitdir: ")
	if err != nil {
		return err
	}
	if isSubmoduleGitDir(target) {
		return errorf(ExitError, "submodule_unsupported", "%s points to a submodule git dir", dotGit)
	}
	gitDir, err := filepath.EvalSymlinks(target)
	if err != nil {
		return hostile("%s: %v", dotGit, err)
	}
	if isSubmoduleGitDir(gitDir) {
		return errorf(ExitError, "submodule_unsupported", "%s points to a submodule git dir", dotGit)
	}
	commonRaw, err := readPointer(filepath.Join(gitDir, "commondir"), gitDir, "")
	if err != nil {
		return err
	}
	common, err := filepath.EvalSymlinks(commonRaw)
	if err != nil {
		return hostile("%s/commondir: %v", gitDir, err)
	}
	if filepath.Dir(gitDir) != filepath.Join(common, "worktrees") {
		return hostile("%s is not a worktree of %s", gitDir, common)
	}
	if err := checkBackPointer(gitDir, dotGit); err != nil {
		return err
	}
	r.GitDir, r.Common = gitDir, common
	return nil
}

func checkBackPointer(gitDir, dotGit string) error {
	back, err := readPointer(filepath.Join(gitDir, "gitdir"), gitDir, "")
	if err != nil {
		return hostile("%s has no back-pointer to %s: %v", gitDir, dotGit, err)
	}
	if resolved, err := filepath.EvalSymlinks(back); err != nil || resolved != dotGit {
		return hostile("%s/gitdir does not point back to %s", gitDir, dotGit)
	}
	return nil
}

// readPointer reads a one-line path file, strips prefix, and resolves a
// relative path against base.
func readPointer(p, base, prefix string) (string, error) {
	data, err := readSmallRegular(p, maxPointerSize)
	if err != nil {
		return "", hostile("%v", err)
	}
	line := strings.TrimRight(string(data), "\r\n")
	if !strings.HasPrefix(line, prefix) || strings.ContainsAny(line, "\x00\n") {
		return "", hostile("%s: unexpected content", p)
	}
	target := strings.TrimPrefix(line, prefix)
	if target == "" {
		return "", hostile("%s: empty path", p)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(base, target)
	}
	return filepath.Clean(target), nil
}

func isSubmoduleGitDir(p string) bool {
	return strings.Contains(filepath.ToSlash(filepath.Clean(p))+"/", "/.git/modules/")
}

// readSmallRegular reads a regular file of at most limit bytes without
// following links and without opening FIFOs or devices.
func readSmallRegular(p string, limit int64) ([]byte, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New(p + " is not a regular file")
	}
	if fi.Size() > limit {
		return nil, errors.New(p + " is too large")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	// Catch a swap between Lstat and Open.
	if ofi, err := f.Stat(); err != nil || !os.SameFile(fi, ofi) {
		return nil, errors.New(p + " changed while being read")
	}
	return io.ReadAll(io.LimitReader(f, limit+1))
}

// inspect vets <common> and <gitdir> directly in the filesystem before any
// git call (R4d).
func (r *Repo) inspect() error {
	dirs := []string{r.Common}
	if r.GitDir != r.Common {
		dirs = append(dirs, r.GitDir)
	}
	for _, d := range dirs {
		if err := inspectGitDir(d); err != nil {
			return err
		}
	}
	return nil
}

func inspectGitDir(dir string) error {
	names := append([]string{}, inspectedEntries...)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return hostile("%s: %v", dir, err)
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "sharedindex.") {
			names = append(names, e.Name())
		}
	}
	for _, n := range names {
		if err := checkEntry(filepath.Join(dir, filepath.FromSlash(n))); err != nil {
			return err
		}
	}
	if err := checkSizes(dir); err != nil {
		return err
	}
	for _, n := range []string{"objects/info/alternates", "objects/info/http-alternates"} {
		if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(n))); err == nil {
			return hostile("%s/%s exists", dir, n)
		}
	}
	return nil
}

const badEntryMode = fs.ModeSymlink | fs.ModeNamedPipe | fs.ModeDevice | fs.ModeCharDevice | fs.ModeSocket | fs.ModeIrregular

func checkEntry(p string) error {
	fi, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return hostile("%s: %v", p, err)
	}
	if fi.Mode()&badEntryMode != 0 {
		return hostile("%s is a %s", p, fi.Mode().Type())
	}
	return nil
}

func checkSizes(dir string) error {
	caps := map[string]int64{"config": maxConfigSize, "config.worktree": maxConfigSize, "HEAD": maxHeadSize}
	for n, limit := range caps {
		fi, err := os.Lstat(filepath.Join(dir, n))
		if err != nil {
			continue // absence is checked by git itself; type by checkEntry
		}
		if fi.Size() > limit {
			return hostile("%s/%s has %d bytes (max %d)", dir, n, fi.Size(), limit)
		}
	}
	return nil
}

// checkConfig classifies the keys of <common>/config and
// <gitdir>/config.worktree as read by git itself (no parser of our own).
func (r *Repo) checkConfig(ctx context.Context) error {
	var keys []string
	for _, f := range []string{filepath.Join(r.Common, "config"), filepath.Join(r.GitDir, "config.worktree")} {
		if _, err := os.Lstat(f); err != nil {
			continue
		}
		k, err := gitConfigList(ctx, f)
		if err != nil {
			return err
		}
		keys = append(keys, k...)
	}
	fatal, neutralized := classifyConfig(keys)
	if len(fatal) > 0 {
		return hostile("fatal git config keys: %s", strings.Join(fatal, ", "))
	}
	if len(neutralized) > 0 {
		r.Warnings = append(r.Warnings, Warning{Code: "git_config_neutralized", Message: strings.Join(neutralized, ", ")})
	}
	return nil
}
