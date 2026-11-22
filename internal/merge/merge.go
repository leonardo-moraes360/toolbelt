// Package merge integrates the work of several tickets (worktree branches)
// into a single target, detecting overlap and conflicts (Phase 3).
//
// Integration happens in a temporary detached worktree; refs only change at
// the end, atomically (ff-only or compare-and-swap via update-ref).
package merge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// ErrConflict is returned (wrapped) when the fail strategy hits a conflict.
var ErrConflict = errors.New("merge conflict")

type Options struct {
	Repo     string // any directory inside the git repository
	Store    *ticket.Store
	IDs      []string // positional IDs (normalizable forms)
	Strategy Strategy // "" = fail
	DryRun   bool     // integrate, report and discard
	Output   string   // write the target..integrated diff to this file, without moving refs
	Into     string   // default: current branch
	Force    bool     // allow re-merging already merged tickets

	// AllowSensitive allows applying tickets that change sensitive paths
	// (hooks, .toolbelt, workflows, .gitmodules, gitlinks, .gitattributes...).
	AllowSensitive bool

	// Resolutions is a receipt ID to replay (prompt only).
	Resolutions string
	// Config is the root -c user config override for the semantic and prompt
	// settings (empty: the global config).
	Config string
}

type File struct {
	Path    string   `json:"path"`
	Tickets []string `json:"tickets"`
	Status  string   `json:"status"` // merge | conflict | last_wins | semantic | prompt
}

type Conflict struct {
	File       string   `json:"file"`
	Ticket     string   `json:"ticket"`
	Commit     string   `json:"commit"`
	Others     []string `json:"others,omitempty"`
	Resolution string   `json:"resolution"` // "" (unresolved), last_wins, semantic or prompt
	// Reason is why the file stayed unresolved (semantic and prompt only).
	Reason string `json:"reason,omitempty"`
	// Hunks lists the conflict regions with their rule or reason.
	Hunks []Hunk `json:"hunks,omitempty"`
}

// Warning is a non-fatal notice, e.g. config_untrusted.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

type TicketInfo struct {
	ID      string   `json:"id"`
	Branch  string   `json:"branch,omitempty"`
	Commits int      `json:"commits"`
	Files   []string `json:"files"`
}

type Result struct {
	Strategy  Strategy     `json:"strategy"`
	Target    string       `json:"target"`
	StartSHA  string       `json:"start_sha"`
	ResultSHA string       `json:"result_sha,omitempty"`
	Tickets   []TicketInfo `json:"tickets"`
	Files     []File       `json:"files"`
	Conflicts []Conflict   `json:"conflicts"`
	DryRun    bool         `json:"dry_run"`
	Applied   bool         `json:"applied"`
	Output    string       `json:"output,omitempty"`
	NoChanges bool         `json:"no_changes"`
	Sensitive []Sensitive  `json:"sensitive"`

	// Fields below only appear with the semantic and prompt strategies.
	Resolutions []Resolution `json:"resolutions,omitempty"`
	ResultTree  string       `json:"result_tree,omitempty"`
	Receipt     *ReceiptRef  `json:"receipt,omitempty"`
	Warnings    []Warning    `json:"warnings,omitempty"`
}

var (
	gitOnce sync.Once
	gitBin  string
	gitErr  error
)

// gitPath resolves git to an absolute path once (avoids a PATH lookup on every call).
func gitPath() (string, error) {
	gitOnce.Do(func() {
		p, err := exec.LookPath("git")
		if err == nil {
			p, err = filepath.Abs(p)
		}
		if err != nil {
			gitErr = fmt.Errorf("git not found in PATH: %w", err)
			return
		}
		gitBin = p
	})
	return gitBin, gitErr
}

type runner struct {
	repo string
	cfg  []string // -c user.name/email fallback
}

// command builds a hardened git invocation (shared by git and gitStatus).
func (r runner) command(dir string, args ...string) (*exec.Cmd, error) {
	full := append(append(append([]string{}, hardenedGitArgs...), r.cfg...), args...)
	bin, err := gitPath()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_EDITOR=true")
	return cmd, nil
}

func (r runner) git(dir string, args ...string) (string, error) {
	cmd, err := r.command(dir, args...)
	if err != nil {
		return "", err
	}
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return stdout.String(), nil
}

// wtGit is kept for readability: git always runs hardened (no hooks,
// fsmonitor, external diff, pager or GPG signing).
func (r runner) wtGit(dir string, args ...string) (string, error) {
	return r.git(dir, args...)
}

func (r runner) out(dir string, args ...string) (string, error) {
	s, err := r.git(dir, args...)
	return strings.TrimSpace(s), err
}

type prepared struct {
	t       ticket.Ticket
	base    string
	head    string
	commits []string
	files   []string
}

// session is the state of one Run after validation and ticket preparation.
type session struct {
	r                runner
	top              string
	opts             *Options
	strategy         Strategy
	target, startSHA string
	// proposal: a prompt run that writes a receipt and never moves a ref.
	proposal bool
	rc       *Receipt // receipt being replayed (prompt apply)
	prep     []prepared
	res      *Result
	byFile   map[string][]string
	rv       *resolver
}

func newSession(opts *Options) (*session, error) {
	strategy, err := validateOptions(opts)
	if err != nil {
		return nil, err
	}
	r, top, err := newRunner(opts)
	if err != nil {
		return nil, err
	}
	s := &session{r: r, top: top, opts: opts, strategy: strategy}
	if s.target, s.startSHA, err = resolveTarget(r, opts.Into); err != nil {
		return nil, err
	}
	// A prompt proposal never moves a ref, whatever --dry-run says.
	s.proposal = strategy == StrategyPrompt && opts.Resolutions == ""
	if s.canApply() && s.target == "" {
		return nil, fmt.Errorf("detached HEAD: provide --into <branch> (or use --dry-run/--output)")
	}
	if opts.Resolutions != "" {
		if s.rc, err = loadReceipt(top, opts.Resolutions); err != nil {
			return nil, err
		}
	}
	if s.prep, err = prepare(r, opts); err != nil {
		return nil, err
	}
	if s.rc != nil {
		if err := s.rc.checkStale(s.target, s.startSHA, s.prep); err != nil {
			return nil, err
		}
	}
	s.res = &Result{Strategy: strategy, Target: s.target, StartSHA: s.startSHA, DryRun: opts.DryRun, Conflicts: []Conflict{}}
	s.byFile = indexTickets(s.res, s.prep)
	return s, nil
}

func (s *session) canApply() bool {
	return !s.opts.DryRun && s.opts.Output == "" && !s.proposal
}

func Run(opts *Options) (*Result, error) {
	s, err := newSession(opts)
	if err != nil {
		return nil, err
	}
	// Trust gate: paths that lead to code execution outside the sandbox.
	if err := checkSensitive(s.r, s.prep, s.res, s.canApply() && !opts.AllowSensitive); err != nil {
		if errors.Is(err, ErrSensitive) {
			return s.res, err
		}
		return nil, err
	}
	if s.strategy.resolving() {
		var closeRV func()
		if s.rv, closeRV, err = prepareSemantic(s.r, opts, s.strategy, s.prep, s.res, s.rc); err != nil {
			return nil, err
		}
		defer closeRV()
	}
	wt, cleanup, err := addTempWorktree(s.r, s.top, s.startSHA)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if s.rv != nil {
		s.rv.wt = wt
	}
	return s.integrate(wt)
}

// integrate cherry-picks every ticket into wt, then reports, records the
// proposal receipt or advances the target.
func (s *session) integrate(wt string) (*Result, error) {
	status := map[string]string{}
	failed, err := pickAll(s.r, wt, s.prep, s.strategy, s.byFile, s.res, status, s.rv)
	if err != nil {
		return nil, err
	}
	if s.rv != nil {
		s.res.Resolutions = s.rv.out
	}
	s.res.Files = fileEntries(s.byFile, status)
	if s.proposal {
		if err := writeProposal(s.r, s.top, wt, s.target, s.startSHA, s.prep, s.res, s.rv, failed); err != nil {
			return nil, err
		}
	}
	if failed {
		return s.res, fmt.Errorf("%w: %d file(s) in conflict; no ref was changed (%s)", ErrConflict, len(s.res.Conflicts), conflictHint(s.strategy))
	}
	if s.rc != nil {
		if err := checkReplay(s.r, wt, s.rc, s.rv); err != nil {
			return nil, err
		}
	}
	return finalize(s.r, s.top, wt, s.opts, s.target, s.startSHA, s.prep, s.res, s.proposal)
}

// writeProposal records the prompt proposal in a receipt (also when files
// stayed unresolved, for audit; such a receipt cannot be applied).
func writeProposal(r runner, top, wt, target, startSHA string, prep []prepared, res *Result, rv *resolver, failed bool) error {
	tree := ""
	if !failed {
		var err error
		if tree, err = r.out(wt, "rev-parse", "HEAD^{tree}"); err != nil {
			return err
		}
	}
	ref, err := writeReceipt(top, newReceipt(target, startSHA, prep, res, rv.agent, tree))
	if err != nil {
		return err
	}
	res.Receipt = ref
	return nil
}

// checkReplay confirms the replayed receipt produced exactly the reviewed tree.
func checkReplay(r runner, wt string, rc *Receipt, rv *resolver) error {
	if err := rv.replay.checkAllUsed(); err != nil {
		return err
	}
	tree, err := r.out(wt, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return err
	}
	if tree != rc.ResultTree {
		return reasonErr("receipt_mismatch", "the integrated tree %.12s differs from the receipt's %.12s", tree, rc.ResultTree)
	}
	return nil
}

// conflictHint is the tail of the ErrConflict message.
func conflictHint(s Strategy) string {
	switch s {
	case StrategySemantic:
		return "use --strategy prompt or --strategy last_wins to resolve"
	case StrategyPrompt:
		return "see the hunk reasons; --strategy last_wins keeps the later ticket's file"
	}
	return "use --strategy last_wins to resolve"
}

// finalize records the resulting SHA and then, depending on options, writes the
// patch (--output), only reports (--dry-run) or advances the target and marks
// the tickets.
func finalize(r runner, top, wt string, opts *Options, target, startSHA string, prep []prepared, res *Result, proposal bool) (*Result, error) {
	sha, err := r.out(wt, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	res.ResultSHA = sha
	res.NoChanges = sha == startSHA
	if res.Strategy.resolving() {
		if res.ResultTree, err = r.out(wt, "rev-parse", "HEAD^{tree}"); err != nil {
			return nil, err
		}
	}

	if opts.Output != "" {
		return writeOutputPatch(r, top, opts.Output, startSHA, res)
	}
	if opts.DryRun || proposal {
		return res, nil
	}
	return applyAndMark(r, top, opts, target, startSHA, prep, res)
}

// checkSensitive fills res.Sensitive and, when enforce is set, refuses the
// integration if sensitive paths exist (the error wraps ErrSensitive; res is
// preserved).
func checkSensitive(r runner, prep []prepared, res *Result, enforce bool) (err error) {
	res.Sensitive, err = detectSensitive(r, prep)
	if err != nil {
		return err
	}
	if len(res.Sensitive) > 0 && enforce {
		var b strings.Builder
		for _, s := range res.Sensitive {
			fmt.Fprintf(&b, "\n  %s: %s (%s)", s.Ticket, s.Path, s.Reason)
		}
		return fmt.Errorf("%w; no ref was changed (review and use --allow-sensitive):%s", ErrSensitive, b.String())
	}
	return nil
}

func validateOptions(opts *Options) (Strategy, error) {
	strategy := opts.Strategy
	if strategy == "" {
		strategy = StrategyFail
	}
	switch strategy {
	case StrategyFail, StrategyLastWins, StrategySemantic, StrategyPrompt:
	default:
		return "", fmt.Errorf("invalid strategy %q (use %s)", strategy, strategyChoices)
	}
	if opts.Resolutions != "" && strategy != StrategyPrompt {
		return "", fmt.Errorf("--resolutions only applies to --strategy prompt")
	}
	if opts.Store == nil {
		return "", fmt.Errorf("ticket store missing")
	}
	if len(opts.IDs) == 0 {
		return "", fmt.Errorf("provide at least one ticket ID")
	}
	return strategy, nil
}

// newRunner resolves the repository toplevel and the git identity fallback.
func newRunner(opts *Options) (r runner, top string, err error) {
	repoDir := opts.Repo
	if repoDir == "" {
		repoDir = "."
	}
	r = runner{repo: repoDir}
	top, err = r.out(repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return runner{}, "", fmt.Errorf("not a git repository: %w", err)
	}
	r.repo = top
	if email, _ := r.out(top, "config", "user.email"); email == "" {
		r.cfg = []string{"-c", "user.name=toolbelt", "-c", "user.email=toolbelt@localhost"}
	}
	return r, top, nil
}

// indexTickets fills res.Tickets and returns file -> tickets that touch it.
func indexTickets(res *Result, prep []prepared) map[string][]string {
	byFile := map[string][]string{}
	for i := range prep {
		p := &prep[i]
		res.Tickets = append(res.Tickets, TicketInfo{ID: p.t.ID, Branch: p.t.Result.Branch, Commits: len(p.commits), Files: nonNil(p.files)})
		for _, f := range p.files {
			byFile[f] = append(byFile[f], p.t.ID)
		}
	}
	return byFile
}

func addTempWorktree(r runner, top, startSHA string) (wt string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "tb-merge-")
	if err != nil {
		return "", nil, err
	}
	wt = filepath.Join(tmp, "wt")
	if _, err := r.git(top, "worktree", "add", "--detach", "--quiet", wt, startSHA); err != nil {
		_ = os.RemoveAll(tmp)
		return "", nil, err
	}
	cleanup = func() {
		_, _ = r.git(top, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(tmp)
		_, _ = r.git(top, "worktree", "prune")
	}
	return wt, cleanup, nil
}

// pickAll accumulates conflicts in res and per-file status. failed=true when
// the fail strategy hit a conflict (it stops at the first one).
func pickAll(r runner, wt string, prep []prepared, strategy Strategy, byFile map[string][]string, res *Result, status map[string]string, rv *resolver) (failed bool, err error) {
	for i := range prep {
		p := &prep[i]
		for _, c := range p.commits {
			conflicts, err := pick(r, wt, c, p, strategy, byFile, rv)
			if err != nil {
				return false, err
			}
			unresolved := false
			for i := range conflicts {
				cf := &conflicts[i]
				res.Conflicts = append(res.Conflicts, *cf)
				switch {
				case cf.Resolution == "":
					status[cf.File] = "conflict"
					unresolved = true
				case status[cf.File] == "conflict" || (status[cf.File] == string(StrategyPrompt) && cf.Resolution == string(StrategySemantic)):
				default:
					status[cf.File] = cf.Resolution
				}
			}
			if strategy == StrategyFail && len(conflicts) > 0 || strategy.resolving() && unresolved {
				return true, nil
			}
		}
	}
	return false, nil
}

func fileEntries(byFile map[string][]string, status map[string]string) []File {
	paths := make([]string, 0, len(byFile))
	for f := range byFile {
		paths = append(paths, f)
	}
	sort.Strings(paths)
	files := []File{}
	for _, f := range paths {
		st := status[f]
		if st == "" {
			st = "merge"
		}
		files = append(files, File{Path: f, Tickets: byFile[f], Status: st})
	}
	return files
}

// writeOutputPatch writes the target..integrated diff without moving refs.
func writeOutputPatch(r runner, top, output, startSHA string, res *Result) (*Result, error) {
	diff, err := r.git(top, "diff", "--no-ext-diff", "--binary", startSHA, res.ResultSHA)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(output, []byte(diff), 0o600); err != nil {
		return nil, err
	}
	res.Output = output
	return res, nil
}

func applyAndMark(r runner, top string, opts *Options, target, startSHA string, prep []prepared, res *Result) (*Result, error) {
	if !res.NoChanges {
		if err := advance(r, top, target, startSHA, res.ResultSHA); err != nil {
			return nil, err
		}
	}
	res.Applied = true
	now := time.Now().UTC()
	for i := range prep {
		id := prep[i].t.ID
		if _, err := opts.Store.Update(id, func(t *ticket.Ticket) error {
			t.MergedAt = &now
			return nil
		}); err != nil {
			return res, fmt.Errorf("refs updated, but failed to mark %s as merged: %w", id, err)
		}
	}
	return res, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// resolveTarget returns (branch, sha); an empty branch means detached HEAD.
func resolveTarget(r runner, into string) (branch, sha string, err error) {
	if into != "" {
		if err := gitref.Validate(into); err != nil {
			return "", "", err
		}
		into = strings.TrimPrefix(into, "refs/heads/")
		sha, err = r.out(r.repo, "rev-parse", "--verify", "--end-of-options", "refs/heads/"+into+"^{commit}")
		if err != nil {
			return "", "", fmt.Errorf("target branch %q does not exist", into)
		}
		return into, sha, nil
	}
	sha, err = r.out(r.repo, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("repository has no commits: %w", err)
	}
	br, err := r.out(r.repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", sha, nil
	}
	return br, sha, nil
}

func prepare(r runner, opts *Options) ([]prepared, error) {
	seen := map[string]bool{}
	var ids []string
	for _, raw := range opts.IDs {
		id, err := ticket.NormalizeID(raw)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	all, err := opts.Store.List(ticket.Filter{})
	if err != nil {
		return nil, err
	}
	depth := ticket.Depths(all)
	var prep []prepared
	for _, id := range ids {
		p, err := prepareOne(r, opts, id)
		if err != nil {
			return nil, err
		}
		prep = append(prep, p)
	}
	sort.SliceStable(prep, func(i, j int) bool { return depth[prep[i].t.ID] < depth[prep[j].t.ID] })
	return prep, nil
}

// prepareOne requires a done ticket and resolves base/head, commits and files.
func prepareOne(r runner, opts *Options, id string) (prepared, error) {
	t, err := opts.Store.Get(id)
	if err != nil {
		return prepared{}, err
	}
	if t.Status != ticket.StatusDone {
		return prepared{}, fmt.Errorf("ticket %s is not done (status %s)", id, t.Status)
	}
	if t.Result == nil || t.Result.HeadCommit == "" || t.Result.BaseCommit == "" {
		return prepared{}, fmt.Errorf("ticket %s has no result (base/head commit) to merge", id)
	}
	if t.MergedAt != nil && !opts.Force {
		return prepared{}, fmt.Errorf("ticket %s was already merged at %s (use --force to redo)", id, t.MergedAt.Format(time.RFC3339))
	}
	base, head := t.Result.BaseCommit, t.Result.HeadCommit
	if base, err = gitref.Resolve(r.repo, base); err != nil {
		return prepared{}, fmt.Errorf("ticket %s base: %w", id, err)
	}
	if head, err = gitref.Resolve(r.repo, head); err != nil {
		return prepared{}, fmt.Errorf("ticket %s head: %w", id, err)
	}
	list, err := r.out(r.repo, "rev-list", "--reverse", "--topo-order", "--no-merges", base+".."+head)
	if err != nil {
		return prepared{}, err
	}
	files, err := r.git(r.repo, "diff", "--no-ext-diff", "--name-only", "--no-renames", "-z", base, head)
	if err != nil {
		return prepared{}, err
	}
	p := prepared{t: t, base: base, head: head}
	p.commits = append(p.commits, strings.Fields(list)...)
	for _, f := range strings.Split(files, "\x00") {
		if f != "" {
			p.files = append(p.files, f)
		}
	}
	sort.Strings(p.files)
	return p, nil
}

// pick returns the conflicts found, resolved or not depending on the strategy.
//
// last_wins: on conflict, the WHOLE file from the later ticket's commit
// replaces the accumulated one (no per-hunk merge); earlier tickets' changes
// to that same file are discarded.
func pick(r runner, wt, commit string, p *prepared, strategy Strategy, byFile map[string][]string, rv *resolver) ([]Conflict, error) {
	_, err := r.wtGit(wt, "cherry-pick", "--allow-empty", "--keep-redundant-commits", commit)
	if err == nil {
		return nil, nil
	}
	unmerged, uerr := r.out(wt, "diff", "--no-ext-diff", "--name-only", "-z", "--diff-filter=U")
	if uerr != nil {
		_, _ = r.git(wt, "cherry-pick", "--abort")
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(unmerged, "\x00") {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		_, _ = r.git(wt, "cherry-pick", "--abort")
		return nil, err
	}
	var out []Conflict
	for _, f := range files {
		var others []string
		for _, id := range byFile[f] {
			if id != p.t.ID {
				others = append(others, id)
			}
		}
		out = append(out, Conflict{File: f, Ticket: p.t.ID, Commit: commit, Others: others})
	}
	if strategy == StrategyFail {
		_, _ = r.git(wt, "cherry-pick", "--abort")
		return out, nil
	}
	if rv != nil {
		return pickResolving(r, wt, rv, &pickCtx{commit: commit, ticket: p.t.ID}, files, out)
	}
	for i, f := range files {
		if err := resolveTheirs(r, wt, commit, f); err != nil {
			_, _ = r.git(wt, "cherry-pick", "--abort")
			return nil, err
		}
		out[i].Resolution = string(StrategyLastWins)
	}
	if err := finishPick(r, wt); err != nil {
		_, _ = r.git(wt, "cherry-pick", "--abort")
		return nil, err
	}
	return out, nil
}

// pickResolving runs the semantic resolver on the unmerged files; the pick is
// aborted (nothing kept) unless every file resolved.
func pickResolving(r runner, wt string, rv *resolver, ctx *pickCtx, files []string, out []Conflict) ([]Conflict, error) {
	ok, err := rv.resolvePick(ctx, files, out)
	if err != nil {
		_, _ = r.git(wt, "cherry-pick", "--abort")
		return nil, err
	}
	if !ok {
		_, _ = r.git(wt, "cherry-pick", "--abort")
		return out, nil
	}
	if err := finishPick(r, wt); err != nil {
		_, _ = r.git(wt, "cherry-pick", "--abort")
		return nil, err
	}
	return out, nil
}

// resolveTheirs resolves f in favor of the ticket's commit (last_wins):
// checkout --theirs (fallback: the commit's content) + add, or rm when the
// file does not exist in the commit.
func resolveTheirs(r runner, wt, commit, f string) error {
	if _, e := r.git(wt, "cat-file", "-e", commit+":"+f); e != nil {
		_, e := r.git(wt, "rm", "-f", "--quiet", "--", f)
		return e
	}
	if _, e := r.git(wt, "checkout", "--theirs", "--", f); e != nil {
		// fallback: the commit's content
		if _, e2 := r.git(wt, "checkout", commit, "--", f); e2 != nil {
			return e2
		}
	}
	_, e := r.git(wt, "add", "--", f)
	return e
}

// finishPick only skips when the pick became empty after resolution (nothing
// staged against HEAD). Any other continue failure is returned (the caller
// aborts without losing a commit).
func finishPick(r runner, wt string) error {
	if _, e := r.git(wt, "diff", "--no-ext-diff", "--cached", "--quiet"); e == nil {
		_, e2 := r.wtGit(wt, "cherry-pick", "--skip")
		return e2
	}
	_, e := r.wtGit(wt, "cherry-pick", "--continue")
	return e
}

// advance moves the target from old to new atomically.
func advance(r runner, top, branch, oldSHA, newSHA string) error {
	cur, err := r.out(top, "rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		return err
	}
	if cur != oldSHA {
		return fmt.Errorf("branch %s changed during integration (%s != %s); redo the merge", branch, short(cur), short(oldSHA))
	}
	if dir := checkedOutAt(r, top, branch); dir != "" {
		dirty, err := r.out(dir, "status", "--porcelain", "--untracked-files=no")
		if err != nil {
			return err
		}
		if dirty != "" {
			return fmt.Errorf("working tree of %s is dirty; commit/stash before merging into %s", dir, branch)
		}
		if _, err := r.git(dir, "merge", "--ff-only", "--quiet", newSHA); err != nil {
			return err
		}
		return nil
	}
	if _, err := r.git(top, "update-ref", "-m", "tb merge", "refs/heads/"+branch, newSHA, oldSHA); err != nil {
		return fmt.Errorf("branch %s changed during integration: %w", branch, err)
	}
	return nil
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func checkedOutAt(r runner, top, branch string) string {
	out, err := r.git(top, "worktree", "list", "--porcelain")
	if err != nil {
		return ""
	}
	dir := ""
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			dir = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+branch:
			return dir
		}
	}
	return ""
}
