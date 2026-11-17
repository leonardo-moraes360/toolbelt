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
	"sync"
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

var (
	gitOnce sync.Once
	gitBin  string
	gitErr  error
)

// gitPath resolve o git uma única vez para caminho absoluto (evita busca em PATH a cada chamada).
func gitPath() (string, error) {
	gitOnce.Do(func() {
		p, err := exec.LookPath("git")
		if err == nil {
			p, err = filepath.Abs(p)
		}
		if err != nil {
			gitErr = fmt.Errorf("git não encontrado no PATH: %w", err)
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

func (r runner) git(dir string, args ...string) (string, error) {
	full := append(append(append([]string{}, hardenedGitArgs...), r.cfg...), args...)
	bin, err := gitPath()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(bin, full...)
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

// Run executa a integração.
func Run(opts *Options) (*Result, error) {
	strategy, err := validateOptions(opts)
	if err != nil {
		return nil, err
	}
	r, top, err := newRunner(opts)
	if err != nil {
		return nil, err
	}

	// Alvo.
	target, startSHA, err := resolveTarget(r, opts.Into)
	if err != nil {
		return nil, err
	}
	canApply := !opts.DryRun && opts.Output == ""
	if canApply && target == "" {
		return nil, fmt.Errorf("HEAD destacado: informe --into <branch> (ou use --dry-run/--output)")
	}

	// Tickets.
	prep, err := prepare(r, opts)
	if err != nil {
		return nil, err
	}

	res := &Result{Strategy: strategy, Target: target, StartSHA: startSHA, DryRun: opts.DryRun, Conflicts: []Conflict{}}
	byFile := indexTickets(res, prep)

	// Gate de confiança: caminhos que viram execução de código fora do sandbox.
	if err := checkSensitive(r, prep, res, canApply && !opts.AllowSensitive); err != nil {
		if errors.Is(err, ErrSensitive) {
			return res, err
		}
		return nil, err
	}

	// Worktree temporário destacado.
	wt, cleanup, err := addTempWorktree(r, top, startSHA)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	status := map[string]string{}
	failed, err := pickAll(r, wt, prep, strategy, byFile, res, status)
	if err != nil {
		return nil, err
	}
	res.Files = fileEntries(byFile, status)
	if failed {
		return res, fmt.Errorf("%w: %d arquivo(s) em conflito; nenhuma ref foi alterada (use --strategy last_wins para resolver)", ErrConflict, len(res.Conflicts))
	}

	return finalize(r, top, wt, opts, target, startSHA, prep, res)
}

// finalize registra o SHA resultante e, conforme as opções, grava o patch
// (--output), só reporta (--dry-run) ou avança o alvo e marca os tickets.
func finalize(r runner, top, wt string, opts *Options, target, startSHA string, prep []prepared, res *Result) (*Result, error) {
	sha, err := r.out(wt, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	res.ResultSHA = sha
	res.NoChanges = sha == startSHA

	if opts.Output != "" {
		return writeOutputPatch(r, top, opts.Output, startSHA, res)
	}
	if opts.DryRun {
		return res, nil
	}
	return applyAndMark(r, top, opts, target, startSHA, prep, res)
}

// checkSensitive preenche res.Sensitive e, quando enforce, recusa a integração
// se houver caminhos sensíveis (erro embrulha ErrSensitive; res é preservado).
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
		return fmt.Errorf("%w; nenhuma ref foi alterada (revise e use --allow-sensitive):%s", ErrSensitive, b.String())
	}
	return nil
}

// validateOptions valida as opções e devolve a estratégia efetiva.
func validateOptions(opts *Options) (Strategy, error) {
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

// newRunner resolve o toplevel do repositório e o fallback de identidade git.
func newRunner(opts *Options) (r runner, top string, err error) {
	repoDir := opts.Repo
	if repoDir == "" {
		repoDir = "."
	}
	r = runner{repo: repoDir}
	top, err = r.out(repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return runner{}, "", fmt.Errorf("não é um repositório git: %w", err)
	}
	r.repo = top
	if email, _ := r.out(top, "config", "user.email"); email == "" {
		r.cfg = []string{"-c", "user.name=toolbelt", "-c", "user.email=toolbelt@localhost"}
	}
	return r, top, nil
}

// indexTickets preenche res.Tickets e devolve arquivo -> tickets que o tocam.
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

// addTempWorktree cria o worktree destacado temporário; cleanup o remove.
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

// pickAll aplica os commits de todos os tickets no worktree, acumulando
// conflitos em res e status por arquivo. failed=true quando a estratégia fail
// encontrou conflito (interrompe no primeiro).
func pickAll(r runner, wt string, prep []prepared, strategy Strategy, byFile map[string][]string, res *Result, status map[string]string) (failed bool, err error) {
	for i := range prep {
		p := &prep[i]
		for _, c := range p.commits {
			conflicts, err := pick(r, wt, c, p, strategy, byFile)
			if err != nil {
				return false, err
			}
			for _, cf := range conflicts {
				res.Conflicts = append(res.Conflicts, cf)
				if cf.Resolution == "" {
					status[cf.File] = "conflict"
				} else if status[cf.File] != "conflict" {
					status[cf.File] = "last_wins"
				}
			}
			if strategy == StrategyFail && len(conflicts) > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

// fileEntries monta a lista ordenada de arquivos tocados com seu status.
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

// writeOutputPatch grava o diff alvo..integrado em output, sem mover refs.
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

// applyAndMark avança o alvo e marca os tickets como mergeados.
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
			return res, fmt.Errorf("refs atualizadas, mas falhou marcar %s como mergeado: %w", id, err)
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

// prepareOne valida um ticket done e resolve base/head, commits e arquivos.
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

// resolveTheirs resolve o arquivo f em favor do commit do ticket (last_wins):
// checkout --theirs (fallback: conteúdo do commit) + add, ou rm quando o
// arquivo não existe no commit.
func resolveTheirs(r runner, wt, commit, f string) error {
	if _, e := r.git(wt, "cat-file", "-e", commit+":"+f); e != nil {
		_, e := r.git(wt, "rm", "-f", "--quiet", "--", f)
		return e
	}
	if _, e := r.git(wt, "checkout", "--theirs", "--", f); e != nil {
		// fallback: conteúdo do commit
		if _, e2 := r.git(wt, "checkout", commit, "--", f); e2 != nil {
			return e2
		}
	}
	_, e := r.git(wt, "add", "--", f)
	return e
}

// finishPick conclui o cherry-pick resolvido. Só vira skip quando o pick ficou
// vazio após a resolução (nada staged contra HEAD). Qualquer outra falha do
// continue é devolvida (o chamador aborta, sem perder commit).
func finishPick(r runner, wt string) error {
	if _, e := r.git(wt, "diff", "--no-ext-diff", "--cached", "--quiet"); e == nil {
		_, e2 := r.wtGit(wt, "cherry-pick", "--skip")
		return e2
	}
	_, e := r.wtGit(wt, "cherry-pick", "--continue")
	return e
}

// advance move o alvo de old para new de forma atômica.
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
