// Package merge integra o trabalho de vários tickets (branches de worktrees)
// num único alvo, detectando sobreposição e conflitos (Fase 3).
//
// A integração acontece num worktree temporário destacado; refs só mudam no
// final, de forma atômica (ff-only ou compare-and-swap via update-ref).
package merge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// Strategy define o que fazer diante de conflito.
type Strategy string

const (
	StrategyFail     Strategy = "fail"
	StrategyLastWins Strategy = "last_wins"
)

// ParseStrategy valida a estratégia ("" = fail).
func ParseStrategy(s string) (Strategy, error) {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "", "fail":
		return StrategyFail, nil
	case "last_wins", "last-wins":
		return StrategyLastWins, nil
	case "prompt", "semantic":
		return "", fmt.Errorf("estratégia %q não implementado (use fail ou last_wins)", v)
	default:
		return "", fmt.Errorf("estratégia inválida %q (use fail ou last_wins)", s)
	}
}

// ErrConflict é devolvido (embrulhado) quando a estratégia fail encontra conflito.
var ErrConflict = errors.New("conflito de merge")

// Options configura Run.
type Options struct {
	Repo     string        // diretório dentro do repositório git
	Store    *ticket.Store // store de tickets
	IDs      []string      // IDs posicionais (formas normalizáveis)
	Strategy Strategy      // "" = fail
	DryRun   bool          // integra, reporta e descarta
	Output   string        // escreve diff alvo..integrado neste arquivo, sem mover refs
	Into     string        // branch alvo (default: branch atual)
	Force    bool          // permite re-merge de tickets já mergeados

	// AllowSensitive permite aplicar tickets que alteram caminhos sensíveis
	// (hooks, .toolbelt, workflows, .gitmodules, gitlinks, .gitattributes...).
	AllowSensitive bool
}

// File descreve um arquivo tocado por tickets.
type File struct {
	Path    string   `json:"path"`
	Tickets []string `json:"tickets"`
	Status  string   `json:"status"` // merge | conflict | last_wins
}

// Conflict descreve um conflito encontrado.
type Conflict struct {
	File       string   `json:"file"`
	Ticket     string   `json:"ticket"`
	Commit     string   `json:"commit"`
	Others     []string `json:"others,omitempty"`
	Resolution string   `json:"resolution"` // "" (não resolvido) ou last_wins
}

// TicketInfo resume um ticket integrado.
type TicketInfo struct {
	ID      string   `json:"id"`
	Branch  string   `json:"branch,omitempty"`
	Commits int      `json:"commits"`
	Files   []string `json:"files"`
}

// Result é o relatório de uma integração.
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
}

type runner struct {
	repo string
	cfg  []string // -c user.name/email fallback
}

func (r runner) git(dir string, args ...string) (string, error) {
	full := append(append(append([]string{}, hardenedGitArgs...), r.cfg...), args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_EDITOR=true")
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

// wtGit é mantido por legibilidade: git já roda sempre endurecido (sem hooks,
// fsmonitor, diff externo, pager e assinatura GPG).
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

func validateRunOpts(opts *Options) (Strategy, error) {
	strategy := opts.Strategy
	if strategy == "" {
		strategy = StrategyFail
	}
	if strategy != StrategyFail && strategy != StrategyLastWins {
		return "", fmt.Errorf("estratégia %q não implementado", strategy)
	}
	if opts.Store == nil {
		return "", fmt.Errorf("store de tickets ausente")
	}
	if len(opts.IDs) == 0 {
		return "", fmt.Errorf("informe ao menos um ID de ticket")
	}
	return strategy, nil
}

func openMergeRepo(opts *Options) (runner, string, error) {
	repoDir := opts.Repo
	if repoDir == "" {
		repoDir = "."
	}
	r := runner{repo: repoDir}
	top, err := r.out(repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return r, "", fmt.Errorf("não é um repositório git: %w", err)
	}
	r.repo = top
	if email, _ := r.out(top, "config", "user.email"); email == "" {
		r.cfg = []string{"-c", "user.name=toolbelt", "-c", "user.email=toolbelt@localhost"}
	}
	return r, top, nil
}

func indexPrepared(res *Result, prep []prepared) (byFile map[string][]string, status map[string]string) {
	byFile = map[string][]string{}
	status = map[string]string{}
	for i := range prep {
		p := &prep[i]
		res.Tickets = append(res.Tickets, TicketInfo{ID: p.t.ID, Branch: p.t.Result.Branch, Commits: len(p.commits), Files: nonNil(p.files)})
		for _, f := range p.files {
			byFile[f] = append(byFile[f], p.t.ID)
		}
	}
	return byFile, status
}

func sensitiveBlocked(res *Result, canApply, allow bool) error {
	if len(res.Sensitive) == 0 || !canApply || allow {
		return nil
	}
	var b strings.Builder
	for _, s := range res.Sensitive {
		fmt.Fprintf(&b, "\n  %s: %s (%s)", s.Ticket, s.Path, s.Reason)
	}
	return fmt.Errorf("%w; nenhuma ref foi alterada (revise e use --allow-sensitive):%s", ErrSensitive, b.String())
}

func recordConflicts(res *Result, status map[string]string, conflicts []Conflict) {
	for _, cf := range conflicts {
		res.Conflicts = append(res.Conflicts, cf)
		if cf.Resolution == "" {
			status[cf.File] = "conflict"
		} else if status[cf.File] != "conflict" {
			status[cf.File] = "last_wins"
		}
	}
}

func cherryPickAll(r runner, wt string, prep []prepared, strategy Strategy, byFile map[string][]string, res *Result, status map[string]string) (failed bool, err error) {
	for i := range prep {
		p := &prep[i]
		for _, c := range p.commits {
			conflicts, perr := pick(r, wt, c, p, strategy, byFile)
			if perr != nil {
				return false, perr
			}
			recordConflicts(res, status, conflicts)
			if strategy == StrategyFail && len(conflicts) > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func fillFileReport(res *Result, byFile map[string][]string, status map[string]string) {
	paths := make([]string, 0, len(byFile))
	for f := range byFile {
		paths = append(paths, f)
	}
	sort.Strings(paths)
	for _, f := range paths {
		st := status[f]
		if st == "" {
			st = "merge"
		}
		res.Files = append(res.Files, File{Path: f, Tickets: byFile[f], Status: st})
	}
	if res.Files == nil {
		res.Files = []File{}
	}
}

func publishResult(r runner, top string, opts *Options, prep []prepared, res *Result, startSHA, sha string) error {
	if opts.Output != "" {
		diff, err := r.git(top, "diff", "--no-ext-diff", "--binary", startSHA, sha)
		if err != nil {
			return err
		}
		if err := os.WriteFile(opts.Output, []byte(diff), 0o600); err != nil {
			return err
		}
		res.Output = opts.Output
		return nil
	}
	if opts.DryRun {
		return nil
	}
	if !res.NoChanges {
		if err := advance(r, top, res.Target, startSHA, sha); err != nil {
			return err
		}
	}
	res.Applied = true
	now := time.Now().UTC()
	for i := range prep {
		p := &prep[i]
		if _, err := opts.Store.Update(p.t.ID, func(t *ticket.Ticket) error {
			t.MergedAt = &now
			return nil
		}); err != nil {
			return fmt.Errorf("refs atualizadas, mas falhou marcar %s como mergeado: %w", p.t.ID, err)
		}
	}
	return nil
}

func withMergeWorktree(r runner, top, startSHA string, fn func(wt string) error) error {
	tmp, err := os.MkdirTemp("", "tb-merge-")
	if err != nil {
		return err
	}
	wt := filepath.Join(tmp, "wt")
	if _, err := r.git(top, "worktree", "add", "--detach", "--quiet", wt, startSHA); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	defer func() {
		_, _ = r.git(top, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(tmp)
		_, _ = r.git(top, "worktree", "prune")
	}()
	return fn(wt)
}

func integratePrep(r runner, top string, opts *Options, prep []prepared, strategy Strategy, res *Result, byFile map[string][]string, status map[string]string, startSHA string) error {
	var failed bool
	err := withMergeWorktree(r, top, startSHA, func(wt string) error {
		var perr error
		failed, perr = cherryPickAll(r, wt, prep, strategy, byFile, res, status)
		if perr != nil {
			return perr
		}
		fillFileReport(res, byFile, status)
		if failed {
			return nil
		}
		sha, perr := r.out(wt, "rev-parse", "HEAD")
		if perr != nil {
			return perr
		}
		res.ResultSHA = sha
		res.NoChanges = sha == startSHA
		return publishResult(r, top, opts, prep, res, startSHA, sha)
	})
	if err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("%w: %d arquivo(s) em conflito; nenhuma ref foi alterada (use --strategy last_wins para resolver)", ErrConflict, len(res.Conflicts))
	}
	return nil
}

// Run executa a integração.
func Run(opts *Options) (*Result, error) {
	strategy, err := validateRunOpts(opts)
	if err != nil {
		return nil, err
	}
	r, top, err := openMergeRepo(opts)
	if err != nil {
		return nil, err
	}
	target, startSHA, err := resolveTarget(r, opts.Into)
	if err != nil {
		return nil, err
	}
	canApply := !opts.DryRun && opts.Output == ""
	if canApply && target == "" {
		return nil, fmt.Errorf("HEAD destacado: informe --into <branch> (ou use --dry-run/--output)")
	}
	prep, err := prepare(r, opts)
	if err != nil {
		return nil, err
	}
	res := &Result{Strategy: strategy, Target: target, StartSHA: startSHA, DryRun: opts.DryRun, Conflicts: []Conflict{}}
	byFile, status := indexPrepared(res, prep)
	res.Sensitive, err = detectSensitive(r, prep)
	if err != nil {
		return nil, err
	}
	if err := sensitiveBlocked(res, canApply, opts.AllowSensitive); err != nil {
		return res, err
	}
	if err := integratePrep(r, top, opts, prep, strategy, res, byFile, status, startSHA); err != nil {
		if res.Applied || (len(res.Conflicts) > 0 && errors.Is(err, ErrConflict)) {
			return res, err
		}
		return nil, err
	}
	return res, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// resolveTarget devolve (branch, sha). branch vazio = HEAD destacado.
func resolveTarget(r runner, into string) (branch, sha string, err error) {
	if into != "" {
		if err := gitref.Validate(into); err != nil {
			return "", "", err
		}
		into = strings.TrimPrefix(into, "refs/heads/")
		sha, err = r.out(r.repo, "rev-parse", "--verify", "--end-of-options", "refs/heads/"+into+"^{commit}")
		if err != nil {
			return "", "", fmt.Errorf("branch alvo %q não existe", into)
		}
		return into, sha, nil
	}
	sha, err = r.out(r.repo, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("repositório sem commits: %w", err)
	}
	br, err := r.out(r.repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", sha, nil
	}
	return br, sha, nil
}

func normalizeMergeIDs(raws []string) ([]string, error) {
	seen := map[string]bool{}
	var ids []string
	for _, raw := range raws {
		id, err := ticket.NormalizeID(raw)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func prepareOne(r runner, opts *Options, id string) (prepared, error) {
	t, err := opts.Store.Get(id)
	if err != nil {
		return prepared{}, err
	}
	if t.Status != ticket.StatusDone {
		return prepared{}, fmt.Errorf("ticket %s não está done (status %s)", id, t.Status)
	}
	if t.Result == nil || t.Result.HeadCommit == "" || t.Result.BaseCommit == "" {
		return prepared{}, fmt.Errorf("ticket %s sem resultado (base/head commit) para mergear", id)
	}
	if t.MergedAt != nil && !opts.Force {
		return prepared{}, fmt.Errorf("ticket %s já foi mergeado em %s (use --force para refazer)", id, t.MergedAt.Format(time.RFC3339))
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

func prepare(r runner, opts *Options) ([]prepared, error) {
	ids, err := normalizeMergeIDs(opts.IDs)
	if err != nil {
		return nil, err
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

func abortPick(r runner, wt string) {
	_, _ = r.git(wt, "cherry-pick", "--abort")
}

func resolveLastWinsFile(r runner, wt, commit, f string) error {
	if _, e := r.git(wt, "cat-file", "-e", commit+":"+f); e != nil {
		if _, e := r.git(wt, "rm", "-f", "--quiet", "--", f); e != nil {
			abortPick(r, wt)
			return e
		}
		return nil
	}
	if _, e := r.git(wt, "checkout", "--theirs", "--", f); e != nil {
		if _, e2 := r.git(wt, "checkout", commit, "--", f); e2 != nil {
			abortPick(r, wt)
			return e2
		}
	}
	if _, e := r.git(wt, "add", "--", f); e != nil {
		abortPick(r, wt)
		return e
	}
	return nil
}

func finishLastWinsPick(r runner, wt string, out []Conflict) ([]Conflict, error) {
	if _, e := r.git(wt, "diff", "--no-ext-diff", "--cached", "--quiet"); e == nil {
		if _, e2 := r.wtGit(wt, "cherry-pick", "--skip"); e2 != nil {
			abortPick(r, wt)
			return nil, e2
		}
		return out, nil
	}
	if _, e := r.wtGit(wt, "cherry-pick", "--continue"); e != nil {
		abortPick(r, wt)
		return nil, e
	}
	return out, nil
}

// pick aplica um commit no worktree e devolve os conflitos encontrados
// (resolvidos ou não, conforme a estratégia).
//
// last_wins: em conflito, a versão INTEIRA do arquivo no commit do ticket
// posterior substitui a acumulada (não há merge por hunk); alterações de
// tickets anteriores nesse mesmo arquivo são descartadas.
func pick(r runner, wt, commit string, p *prepared, strategy Strategy, byFile map[string][]string) ([]Conflict, error) {
	_, err := r.wtGit(wt, "cherry-pick", "--allow-empty", "--keep-redundant-commits", commit)
	if err == nil {
		return nil, nil
	}
	unmerged, uerr := r.out(wt, "diff", "--no-ext-diff", "--name-only", "-z", "--diff-filter=U")
	if uerr != nil {
		abortPick(r, wt)
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(unmerged, "\x00") {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		abortPick(r, wt)
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
		abortPick(r, wt)
		return out, nil
	}
	for i, f := range files {
		if err := resolveLastWinsFile(r, wt, commit, f); err != nil {
			return nil, err
		}
		out[i].Resolution = string(StrategyLastWins)
	}
	return finishLastWinsPick(r, wt, out)
}

// advance move o alvo de oldSHA para newSHA de forma atômica.
func advance(r runner, top, branch, oldSHA, newSHA string) error {
	cur, err := r.out(top, "rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		return err
	}
	if cur != oldSHA {
		return fmt.Errorf("branch %s mudou durante a integração (%s != %s); refaça o merge", branch, short(cur), short(oldSHA))
	}
	if dir := checkedOutAt(r, top, branch); dir != "" {
		dirty, err := r.out(dir, "status", "--porcelain", "--untracked-files=no")
		if err != nil {
			return err
		}
		if dirty != "" {
			return fmt.Errorf("árvore de trabalho de %s está suja; faça commit/stash antes de mergear em %s", dir, branch)
		}
		if _, err := r.git(dir, "merge", "--ff-only", "--quiet", newSHA); err != nil {
			return err
		}
		return nil
	}
	if _, err := r.git(top, "update-ref", "-m", "tb merge", "refs/heads/"+branch, newSHA, oldSHA); err != nil {
		return fmt.Errorf("branch %s mudou durante a integração: %w", branch, err)
	}
	return nil
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// checkedOutAt devolve o diretório do worktree que tem branch em checkout.
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
