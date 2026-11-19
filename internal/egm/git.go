package egm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// gitTimeout bounds every git call (R4c). A package var so tests can shorten it.
var gitTimeout = 30 * time.Second

// gitWaitDelay bounds how long we wait for git's pipes after killing it, so a
// grandchild holding stdout cannot make a timed-out call hang.
const gitWaitDelay = 2 * time.Second

// gitMaxOutput caps stdout of calls whose output size is not otherwise bounded.
const gitMaxOutput = 64 << 20

// gitCandidates are the only places the git binary is taken from: PATH may be
// controlled by the environment the user runs tb in, so it is not consulted.
var gitCandidates = []string{"/usr/bin/git", "/bin/git", "/usr/local/bin/git"}

// gitPath resolves the git binary once. A package var so tests can install a
// fake git (e.g. one that hangs).
var gitPath = sync.OnceValues(resolveGit)

func resolveGit() (string, error) {
	for _, p := range gitCandidates {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", errorf(ExitError, "git_not_found", "git not found in %s", strings.Join(gitCandidates, ", "))
}

// gitOp is one shape of the closed list of git subcommands allowed in egm
// (R4c). Nothing outside this table can be run by runGit.
type gitOp int

const (
	opLsFilesStage gitOp = iota
	opLsFilesOthers
	opLsTree
	opLsTreeNames
	opHashObject
	opCatFileSize
	opCatFileBlob
	opRevParse
	opMergeBase
	opDiffNameStatus
	opDiffNoIndex
	opCheckIgnore
	opConfigList
)

// gitOps holds the fixed argv prefix of each allowed shape; callers only add
// validated tails (--end-of-options, verified object ids, "--" and paths).
var gitOps = map[gitOp][]string{
	opLsFilesStage:   {"ls-files", "-s", "-z"},
	opLsFilesOthers:  {"ls-files", "-z", "--others", "--exclude-standard"},
	opLsTree:         {"ls-tree", "-z"},
	opLsTreeNames:    {"ls-tree", "-r", "-z", "--name-only"},
	opHashObject:     {"hash-object", "--no-filters"},
	opCatFileSize:    {"cat-file", "-s"},
	opCatFileBlob:    {"cat-file", "blob"},
	opRevParse:       {"rev-parse", "--verify", "--quiet"},
	opMergeBase:      {"merge-base"},
	opDiffNameStatus: {"diff", "--name-status", "--no-renames", "-z", "--no-ext-diff", "--no-textconv"},
	opDiffNoIndex:    {"diff", "--no-index", "-U0", "--no-ext-diff", "--no-textconv", "--no-color"},
	opCheckIgnore:    {"check-ignore", "--no-index"},
	opConfigList:     {"config", "--no-includes", "--null", "--list", "--file"},
}

// gitFixedOptions neutralize every config key that could make an allowed
// subcommand execute something or reach the network (R4c).
var gitFixedOptions = []string{
	"-c", "core.fsmonitor=false",
	"-c", "core.hooksPath=/dev/null",
	"-c", "core.sshCommand=false",
	"-c", "diff.external=",
	"-c", "core.pager=cat",
	"-c", "core.untrackedCache=false",
	"-c", "submodule.recurse=false",
	"-c", "diff.ignoreSubmodules=all",
	"-c", "core.excludesFile=/dev/null",
	"-c", "core.attributesFile=/dev/null",
	"-c", "protocol.allow=never",
	"-c", "credential.helper=",
	"-c", "core.askPass=",
	"-c", "core.editor=true",
	"-c", "core.alternateRefsCommand=",
}

// gitFixedEnv is appended after every inherited GIT_* variable is dropped.
var gitFixedEnv = []string{
	"GIT_CONFIG_NOSYSTEM=1",
	"GIT_CONFIG_GLOBAL=/dev/null",
	"GIT_PAGER=cat",
	"GIT_TERMINAL_PROMPT=0",
	"GIT_NO_LAZY_FETCH=1",
	"GIT_LITERAL_PATHSPECS=1",
	"GIT_ASKPASS=",
}

// gitCall is one invocation: an allowed shape plus its validated tail.
type gitCall struct {
	op        gitOp
	tail      []string
	dir       string   // cwd
	safeDir   string   // real root for safe.directory; empty outside a repo
	extraEnv  []string // GIT_DIR/GIT_WORK_TREE or GIT_CEILING_DIRECTORIES
	okExit1   bool     // exit 1 is a valid answer (no differences, none ignored)
	maxOutput int64
}

// gitEnv builds the environment: os.Environ() minus every GIT_* plus the
// fixed values, so nothing from the user's shell can redirect git.
func gitEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	env = append(env, gitFixedEnv...)
	return append(env, extra...)
}

func (c *gitCall) argv() ([]string, error) {
	prefix, ok := gitOps[c.op]
	if !ok {
		return nil, errorf(ExitError, "git_forbidden", "programming error: git op %d is not in the R4c list", int(c.op))
	}
	args := append([]string{}, gitFixedOptions...)
	if c.safeDir != "" {
		args = append(args, "-c", "safe.directory="+c.safeDir)
	}
	args = append(args, prefix...)
	return append(args, c.tail...), nil
}

// limitedBuffer fails the call instead of buffering unbounded output.
type limitedBuffer struct {
	buf      bytes.Buffer
	max      int64
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if int64(b.buf.Len()+len(p)) > b.max {
		b.overflow = true
		return 0, errors.New("output limit exceeded")
	}
	return b.buf.Write(p)
}

// runGit is the only place in the egm package that executes git.
func runGit(ctx context.Context, c *gitCall) ([]byte, error) {
	args, err := c.argv()
	if err != nil {
		return nil, err
	}
	bin, err := gitPath()
	if err != nil {
		return nil, err
	}
	tctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	maxOut := c.maxOutput
	if maxOut <= 0 {
		maxOut = gitMaxOutput
	}
	stdout := &limitedBuffer{max: maxOut}
	stderr := &limitedBuffer{max: 64 << 10}
	cmd := exec.CommandContext(tctx, bin, args...)
	cmd.Dir = c.dir
	cmd.Env = gitEnv(c.extraEnv...)
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = gitWaitDelay
	runErr := cmd.Run()
	if errors.Is(tctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		return nil, errorf(ExitError, "git_timeout", "git %s exceeded %s", gitOps[c.op][0], gitTimeout)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return stdout.buf.Bytes(), c.checkExit(runErr, stdout, stderr)
}

func (c *gitCall) checkExit(runErr error, stdout, stderr *limitedBuffer) error {
	if stdout.overflow {
		return errorf(ExitError, "git_output_too_large", "git %s: output above %d bytes", gitOps[c.op][0], stdout.max)
	}
	if runErr == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) && ee.ExitCode() == 1 && c.okExit1 {
		return nil
	}
	msg := strings.TrimSpace(stderr.buf.String())
	if len(msg) > 512 {
		msg = msg[:512]
	}
	return errorf(ExitError, "git_failed", "git %s: %v: %s", gitOps[c.op][0], runErr, msg)
}

// validRelPath accepts only clean repo-relative slash paths, so no argument
// can be read as an option or reach outside the root.
func validRelPath(p string) error {
	switch {
	case p == "":
		return errors.New("empty path")
	case strings.ContainsRune(p, 0):
		return errors.New("NUL in path")
	case strings.HasPrefix(p, "/"), strings.Contains(p, "\\"):
		return fmt.Errorf("path %q is not a repo-relative slash path", p)
	case p == ".":
		return nil
	case path.Clean(p) != p, p == "..", strings.HasPrefix(p, "../"):
		return fmt.Errorf("path %q is not clean or leaves the root", p)
	}
	return nil
}

func validPaths(paths []string) error {
	if len(paths) == 0 {
		// An empty pathspec would sweep the whole repository (R4c forbids it).
		return errorf(ExitError, "git_forbidden", "programming error: git call without paths")
	}
	for _, p := range paths {
		if err := validRelPath(p); err != nil {
			return errorf(ExitError, "git_forbidden", "programming error: %v", err)
		}
	}
	return nil
}

// validRevSyntax rejects what could be read as an option or break argv
// before the rev even reaches rev-parse.
func validRevSyntax(rev string) error {
	if rev == "" || strings.HasPrefix(rev, "-") || strings.ContainsAny(rev, "\x00\n") {
		return errorf(ExitError, "bad_revision", "invalid revision %q", rev)
	}
	return nil
}

func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func (r *Repo) call(op gitOp, tail ...string) *gitCall {
	return &gitCall{
		op:       op,
		tail:     tail,
		dir:      r.Root,
		safeDir:  r.Root,
		extraEnv: []string{"GIT_DIR=" + r.GitDir, "GIT_WORK_TREE=" + r.Root},
	}
}

// IndexEntry is one entry of ls-files -s or ls-tree.
type IndexEntry struct {
	Mode, Object, Path string
}

// NameStatus is one entry of diff --name-status.
type NameStatus struct {
	Status, Path string
}

// LsFilesStage runs ls-files -s -z -- <paths>.
func (r *Repo) LsFilesStage(ctx context.Context, paths ...string) ([]IndexEntry, error) {
	if err := validPaths(paths); err != nil {
		return nil, err
	}
	out, err := runGit(ctx, r.call(opLsFilesStage, append([]string{"--"}, paths...)...))
	if err != nil {
		return nil, err
	}
	return parseEntries(out, 3)
}

// LsFilesOthers runs ls-files -z --others --exclude-standard -- <dirs>.
func (r *Repo) LsFilesOthers(ctx context.Context, dirs ...string) ([]string, error) {
	if err := validPaths(dirs); err != nil {
		return nil, err
	}
	out, err := runGit(ctx, r.call(opLsFilesOthers, append([]string{"--"}, dirs...)...))
	if err != nil {
		return nil, err
	}
	return splitNUL(out), nil
}

// LsTree runs ls-tree -z <rev> -- <paths>.
func (r *Repo) LsTree(ctx context.Context, rev string, paths ...string) ([]IndexEntry, error) {
	if err := validPaths(paths); err != nil {
		return nil, err
	}
	oid, err := r.RevParse(ctx, rev)
	if err != nil {
		return nil, err
	}
	return r.lsTreeOID(ctx, oid, paths...)
}

// lsTreeOID is LsTree for an oid this package already got from RevParse,
// without resolving it again (gate mode runs it once per checked path).
func (r *Repo) lsTreeOID(ctx context.Context, oid string, paths ...string) ([]IndexEntry, error) {
	if err := validPaths(paths); err != nil {
		return nil, err
	}
	if !isObjectID(oid) {
		return nil, errorf(ExitError, "git_forbidden", "programming error: ls-tree of %q, not an object id", oid)
	}
	tail := append([]string{"--end-of-options", oid, "--"}, paths...)
	out, err := runGit(ctx, r.call(opLsTree, tail...))
	if err != nil {
		return nil, err
	}
	return parseEntries(out, 3)
}

// LsTreeNames runs ls-tree -r -z --name-only <rev> -- <dir> and fails when
// there are more than max names (R13).
func (r *Repo) LsTreeNames(ctx context.Context, rev, dir string, limit int) ([]string, error) {
	if err := validPaths([]string{dir}); err != nil {
		return nil, err
	}
	oid, err := r.RevParse(ctx, rev)
	if err != nil {
		return nil, err
	}
	out, err := runGit(ctx, r.call(opLsTreeNames, "--end-of-options", oid, "--", dir))
	if err != nil {
		return nil, err
	}
	names := splitNUL(out)
	if len(names) > limit {
		return nil, errorf(ExitError, "too_many_entries", "%s has more than %d files at %s", dir, limit, rev)
	}
	return names, nil
}

// HashObject runs hash-object --no-filters -- <paths>; --no-filters keeps
// clean/smudge filters from the hostile config out of the way.
func (r *Repo) HashObject(ctx context.Context, paths ...string) ([]string, error) {
	if err := validPaths(paths); err != nil {
		return nil, err
	}
	out, err := runGit(ctx, r.call(opHashObject, append([]string{"--"}, paths...)...))
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(string(out))
	if len(ids) != len(paths) {
		return nil, errorf(ExitError, "git_failed", "hash-object returned %d ids for %d paths", len(ids), len(paths))
	}
	return ids, nil
}

// objectAt builds <oid>:<path> from a verified rev; the path must be clean,
// because "./" would be resolved against the cwd.
func (r *Repo) objectAt(ctx context.Context, rev, p string) (string, error) {
	if err := validPaths([]string{p}); err != nil {
		return "", err
	}
	if p == "." {
		return "", errorf(ExitError, "git_forbidden", "programming error: cat-file of the root tree")
	}
	oid, err := r.RevParse(ctx, rev)
	if err != nil {
		return "", err
	}
	return oid + ":" + p, nil
}

// CatFileSize runs cat-file -s <rev>:<path>.
func (r *Repo) CatFileSize(ctx context.Context, rev, p string) (int64, error) {
	obj, err := r.objectAt(ctx, rev, p)
	if err != nil {
		return 0, err
	}
	out, err := runGit(ctx, r.call(opCatFileSize, "--end-of-options", obj))
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, errorf(ExitError, "git_failed", "cat-file -s: %v", err)
	}
	return n, nil
}

// CatFileBlob runs cat-file blob <rev>:<path> after checking the size, so a
// huge blob is refused without being read.
func (r *Repo) CatFileBlob(ctx context.Context, rev, p string, limit int64) ([]byte, error) {
	size, err := r.CatFileSize(ctx, rev, p)
	if err != nil {
		return nil, err
	}
	if size > limit {
		return nil, errorf(ExitError, "blob_too_large", "%s at %s has %d bytes (max %d)", p, rev, size, limit)
	}
	obj, err := r.objectAt(ctx, rev, p)
	if err != nil {
		return nil, err
	}
	c := r.call(opCatFileBlob, "--end-of-options", obj)
	c.maxOutput = size + 1
	return runGit(ctx, c)
}

// RevParse runs rev-parse --verify --quiet --end-of-options <rev> and returns
// the object id. Every other rev-taking call goes through it first.
func (r *Repo) RevParse(ctx context.Context, rev string) (string, error) {
	if err := validRevSyntax(rev); err != nil {
		return "", err
	}
	out, err := runGit(ctx, r.call(opRevParse, "--end-of-options", rev))
	if err != nil {
		return "", errorf(ExitError, "bad_revision", "unknown revision %q", rev)
	}
	oid := strings.TrimSpace(string(out))
	if !isObjectID(oid) {
		return "", errorf(ExitError, "bad_revision", "revision %q did not resolve to an object id", rev)
	}
	return oid, nil
}

func (r *Repo) revPair(ctx context.Context, a, b string) (oa, ob string, err error) {
	if oa, err = r.RevParse(ctx, a); err != nil {
		return "", "", err
	}
	if ob, err = r.RevParse(ctx, b); err != nil {
		return "", "", err
	}
	return oa, ob, nil
}

// MergeBase runs merge-base <a> <b>.
func (r *Repo) MergeBase(ctx context.Context, a, b string) (string, error) {
	oa, ob, err := r.revPair(ctx, a, b)
	if err != nil {
		return "", err
	}
	out, err := runGit(ctx, r.call(opMergeBase, "--end-of-options", oa, ob))
	if err != nil {
		return "", err
	}
	base := strings.TrimSpace(string(out))
	if !isObjectID(base) {
		return "", errorf(ExitError, "git_failed", "no merge base between %s and %s", a, b)
	}
	return base, nil
}

// DiffNameStatus runs diff --name-status --no-renames -z <a> <b>.
func (r *Repo) DiffNameStatus(ctx context.Context, a, b string) ([]NameStatus, error) {
	oa, ob, err := r.revPair(ctx, a, b)
	if err != nil {
		return nil, err
	}
	out, err := runGit(ctx, r.call(opDiffNameStatus, "--end-of-options", oa, ob, "--"))
	if err != nil {
		return nil, err
	}
	fields := splitNUL(out)
	if len(fields)%2 != 0 {
		return nil, errorf(ExitError, "git_failed", "diff --name-status: odd number of fields")
	}
	res := make([]NameStatus, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		res = append(res, NameStatus{Status: fields[i], Path: fields[i+1]})
	}
	return res, nil
}

// CheckIgnore runs check-ignore --no-index -- <paths>. Exit 1 (nothing
// ignored) is a valid answer.
func (r *Repo) CheckIgnore(ctx context.Context, paths ...string) (map[string]bool, error) {
	if err := validPaths(paths); err != nil {
		return nil, err
	}
	// check-ignore refuses the literal pathspec magic, so for this call
	// GIT_LITERAL_PATHSPECS is off and a leading ':' (the only way to spell
	// magic) is neutralized with "./"; the output echoes our argument.
	args := make([]string, 0, len(paths)+1)
	back := make(map[string]string, len(paths))
	args = append(args, "--")
	for _, p := range paths {
		arg := p
		if strings.HasPrefix(p, ":") {
			arg = "./" + p
		}
		back[arg] = p
		args = append(args, arg)
	}
	c := r.call(opCheckIgnore, args...)
	c.extraEnv = append(c.extraEnv, "GIT_LITERAL_PATHSPECS=0")
	c.okExit1 = true
	out, err := runGit(ctx, c)
	if err != nil {
		return nil, err
	}
	return parseCheckIgnore(out, paths, back), nil
}

func parseCheckIgnore(out []byte, paths []string, back map[string]string) map[string]bool {
	res := make(map[string]bool, len(paths))
	for _, p := range paths {
		res[p] = false
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		// check-ignore has no -z without --stdin, so special paths come C-quoted.
		if strings.HasPrefix(line, `"`) {
			if uq, err := strconv.Unquote(line); err == nil {
				line = uq
			}
		}
		if p, ok := back[line]; ok {
			res[p] = true
		}
	}
	return res
}

// DiffNoIndex runs diff --no-index -U0 between two files outside any
// repository: no GIT_DIR/GIT_WORK_TREE, cwd and GIT_CEILING_DIRECTORIES in
// stateDir. Exit 1 (differences) is not an error.
func DiffNoIndex(ctx context.Context, stateDir, a, b string) ([]byte, error) {
	for _, p := range []string{stateDir, a, b} {
		if p == "" || strings.ContainsRune(p, 0) {
			return nil, errorf(ExitError, "git_forbidden", "programming error: invalid diff --no-index argument %q", p)
		}
	}
	c := &gitCall{
		op:       opDiffNoIndex,
		tail:     []string{"--", a, b},
		dir:      stateDir,
		extraEnv: []string{"GIT_CEILING_DIRECTORIES=" + stateDir},
		okExit1:  true,
	}
	return runGit(ctx, c)
}

// gitConfigList runs config --file <path> --no-includes --null --list outside
// any repository (cwd and ceiling in an empty dir), so a hostile repository
// format cannot influence how the file itself is read.
func gitConfigList(ctx context.Context, file string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "tb-egm-git-")
	if err != nil {
		return nil, errorf(ExitError, "git_failed", "temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	c := &gitCall{
		op:       opConfigList,
		tail:     []string{file},
		dir:      tmp,
		extraEnv: []string{"GIT_CEILING_DIRECTORIES=" + tmp},
	}
	out, err := runGit(ctx, c)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, e := range splitNUL(out) {
		key, _, _ := strings.Cut(e, "\n")
		keys = append(keys, key)
	}
	return keys, nil
}

func splitNUL(out []byte) []string {
	s := strings.TrimSuffix(string(out), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

// parseEntries parses "<mode> <x> <y>\t<path>" records (ls-files -s: object,
// stage; ls-tree: type, object). nFields is the count before the tab.
func parseEntries(out []byte, nFields int) ([]IndexEntry, error) {
	var res []IndexEntry
	for _, rec := range splitNUL(out) {
		meta, p, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != nFields {
			return nil, errorf(ExitError, "git_failed", "unexpected entry %q", rec)
		}
		obj := f[1]
		if !isObjectID(obj) {
			obj = f[2] // ls-tree: <mode> <type> <object>
		}
		res = append(res, IndexEntry{Mode: f[0], Object: obj, Path: p})
	}
	return res, nil
}
