package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
	"github.com/leonardo-moraes360/toolbelt/internal/workspace"
)

func jsonIndent(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

// wsCreateMu serializa createWorkspace entre workers (git worktree add disputa
// locks de refs). NÃO é tomado por Sweep/settleGroup: o git do workspace manager
// é rastreado no Registry (SetRunner), então varreduras nunca o confundem com um
// órfão nem esperam por ele.
var wsCreateMu sync.Mutex

// procs rastreia todo filho direto do router (agentes e git). Varreduras
// (Sweep) matam e colhem qualquer filho direto não rastreado: descendentes de
// agentes que fizeram setsid/setpgid e foram reparentados ao router (Linux,
// PR_SET_CHILD_SUBREAPER). TB_TICKET_ID (herdado do ambiente do agente) atribui
// o escape ao ticket e é também o dono de cada grupo de agente (um processo que
// entra no grupo de outro agente via setpgid é detectado).
var procs = func() *sandbox.Registry {
	r := sandbox.NewRegistry(ticketIDVar)
	r.SetOwnerVar(ticketIDVar)
	return r
}()

// workspaceGitArgs neutralizam código do usuário/repositório no git do workspace
// manager (worktree add/remove): sem hooks (post-checkout), fsmonitor nem
// manutenção destacada, que viriam como daemons órfãos.
var workspaceGitArgs = []string{
	"-c", "core.hooksPath=" + os.DevNull,
	"-c", "core.fsmonitor=false",
	"-c", "maintenance.auto=false",
	"-c", "gc.auto=0",
	"-c", "gc.autoDetach=false",
}

// newWorkspaceManager cria o workspace.Manager do router: git rastreado no
// Registry (nunca confundido com um órfão, sem lock global durante o spawn) e sem
// hooks/fsmonitor/manutenção do usuário (workspaceGitArgs): worktrees criados
// pelo router nunca executam código do repositório/usuário.
func newWorkspaceManager(root string) (*workspace.Manager, error) {
	wsm, err := workspace.NewManager(root)
	if err != nil {
		return nil, err
	}
	wsm.SetRunner(runTracked)
	wsm.SetGitArgs(workspaceGitArgs...)
	return wsm, nil
}

// runTracked executa cmd registrando o pid em procs (CombinedOutput rastreado).
func runTracked(cmd *exec.Cmd) ([]byte, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := procs.Run(cmd, false)
	return buf.Bytes(), err
}

// gitOut roda git em dir e devolve a saída aparada.
func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // argumentos controlados pelo router
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := runTracked(cmd)
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// execute roda o agente de um ticket (goroutine do worker). Não toca no
// estado do router; devolve tudo por workerDone.
func (r *Router) execute(ctx context.Context, s *slot, worker string, t ticket.Ticket, base string) workerDone {
	ev := workerDone{slot: s, id: t.ID, base: base, branch: "tb-workspace/" + t.ID, exit: -1}
	logRel := filepath.Join(".toolbelt", "router", "logs", t.ID+".log")
	ev.log = filepath.ToSlash(logRel)

	ws, err := r.createWorkspace(t, base)
	if err != nil {
		ev.err = err
		return ev
	}
	ev.workspace = ws.Path

	ticketFile, _ := filepath.Abs(r.store.Path(t.ID))
	environ := r.opts.Environ
	if environ == nil {
		environ = os.Environ()
	}
	env := execenv.Filter(environ, r.opts.EnvOptions)
	env = withGitNoDetach(env)
	env = append(env,
		"TB_TICKET_ID="+t.ID,
		"TB_TICKET_FILE="+ticketFile,
		"TB_WORKER_ID="+worker,
		"TB_WORKSPACE="+ws.Path,
	)
	sb, err := sandbox.New(sandbox.Config{
		WorkDir:          ws.Path,
		AllowUnsandboxed: r.opts.AllowUnsandboxed,
		Env:              env,
		ProcessGroup:     true,
		Registry:         procs,
		GroupOwner:       t.ID,
	})
	if err != nil {
		ev.err = err
		return ev
	}
	r.takeEscaped(t.ID) // marca velha de tentativa anterior não pode envenenar esta
	r.markActive(t.ID, worker)
	defer r.unmarkActive(t.ID)
	res, err := sb.Run(ctx, r.argv[0], r.argv[1:]...)
	if res != nil {
		r.appendLog(ev.log, worker, t.ID, res.Output)
		ev.exit = res.ExitCode
		ev.cancelled = res.Canceled
		ev.timedOut = res.TimedOut
	}
	// Descendentes que escaparam do grupo (setsid/setpgid) já foram mortos pelo
	// Sweep. Só falhamos ESTE ticket se algum escape foi atribuído a ele (por
	// TB_TICKET_ID ou, sem atribuição possível, por estar rodando no momento da
	// detecção); escapes de outros tickets, varridos aqui, apenas os marcam.
	ev.alerts = r.collectEscapes()
	if r.takeEscaped(t.ID) {
		ev.err = fmt.Errorf("%w: processo(s) escaparam do grupo do agente (setsid/setpgid) e foram encerrados; agentes não podem se desacoplar", errAgentEscaped)
		return ev
	}
	if err != nil {
		ev.err = err
		return ev
	}
	// Não marcamos cancelled só porque ctx foi cancelado depois: se o agente já
	// terminou com 0 antes do cancelamento, o trabalho é válido (finding 4).
	if ev.exit != 0 || ev.cancelled || ev.timedOut {
		return ev
	}
	head, err := r.commitWorkspace(ws.Path, t)
	if err != nil {
		ev.err = err
		return ev
	}
	ev.head = head
	return ev
}

func (r *Router) appendLog(rel, worker, id, output string) {
	path := filepath.Join(r.opts.Root, rel)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // dentro de .toolbelt/router/logs
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Chmod(0o600) // logs antigos criados com 0644
	// A saída do agente já vem limitada (sandbox.MaxOutputBytes, só o tail);
	// reforça o teto caso o chamador mude.
	if len(output) > sandbox.MaxOutputBytes {
		output = output[len(output)-sandbox.MaxOutputBytes:]
	}
	fmt.Fprintf(f, "=== %s %s %s ===\n%s", id, worker, r.now().UTC().Format(time.RFC3339), output)
	if output != "" && !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(f)
	}
}

// createWorkspace cria o worktree do ticket em base, limpando resíduos de
// tentativas anteriores. Serializado: git worktree add disputa locks de refs.
func (r *Router) createWorkspace(t ticket.Ticket, base string) (*workspace.Workspace, error) {
	wsCreateMu.Lock()
	defer wsCreateMu.Unlock()
	// Janela de spawn: o git (e o que ele dispara) pode deixar daemons destacados
	// que não são de nenhum agente; vide Registry.BeginSpawnWindow.
	defer procs.BeginSpawnWindow()()
	path := filepath.Join(r.opts.Root, ".toolbelt", "workspaces", t.ID)
	if _, err := os.Stat(path); err == nil {
		_ = r.wsm.Remove(t.ID)
		_ = os.RemoveAll(path)
	}
	_, _ = gitOut(r.opts.Root, "worktree", "prune")
	_, _ = gitOut(r.opts.Root, "branch", "-D", "tb-workspace/"+t.ID)
	return r.wsm.Create(t.ID, base, workspace.Scope{
		AllowedPaths:   t.Mandate.Scope,
		ForbiddenPaths: t.Mandate.Forbidden,
	})
}

// errWorkspaceIntegrity indica que o worktree foi adulterado pelo agente (ou
// não corresponde ao registrado no repositório); nunca roda git nele e o ticket
// falha sem nova tentativa.
var errWorkspaceIntegrity = errors.New("integridade do worktree violada")

// errAgentEscaped: o agente deixou descendentes fora do seu grupo de processos.
// É uma violação de integridade (sem retry, sem git no worktree).
var errAgentEscaped = fmt.Errorf("%w: agent_escaped", errWorkspaceIntegrity)

// hardenedGitArgs são as opções que neutralizam execução de código via config:
// o router roda git FORA do sandbox, então nada que o agente consiga escrever
// pode virar comando (fsmonitor, hooks, ssh, diff externo, pager, assinatura).
var hardenedGitArgs = []string{
	"-c", "core.fsmonitor=false",
	"-c", "core.hooksPath=" + os.DevNull,
	"-c", "core.sshCommand=false",
	"-c", "diff.external=",
	"-c", "core.pager=cat",
	"-c", "commit.gpgsign=false",
	// Submódulos: o agente pode criar <gitlink>/.git com config própria; sem
	// estas opções status/diff/add recursariam nele e rodariam filtros dele.
	"-c", "diff.ignoreSubmodules=all",
	"-c", "status.submoduleSummary=false",
	"-c", "submodule.recurse=false",
	"-c", "fetch.recurseSubmodules=false",
	// git commit pode lançar `maintenance`/`gc --auto` destacado (setsid): viraria
	// um órfão que o Sweep tomaria por agente escapado.
	"-c", "maintenance.auto=false",
	"-c", "gc.auto=0",
	"-c", "gc.autoDetach=false",
}

// trustedGit descreve um worktree verificado: gitDir é o diretório
// <common>/worktrees/<nome> registrado pelo próprio git (fora do worktree, logo
// fora do alcance de escrita do agente).
type trustedGit struct {
	root, gitDir, ws string
}

// verifyWorkspace garante que <ws>/.git é um arquivo regular (não symlink, não
// diretório) cujo "gitdir:" aponta para um worktree registrado em
// <git-common-dir>/worktrees/<nome> do repositório da raiz, e que esse registro
// aponta de volta para <ws>/.git. O common-dir vem de `git rev-parse` com cwd
// na raiz (nunca no worktree).
func verifyWorkspace(root, ws string) (*trustedGit, error) {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{errWorkspaceIntegrity}, a...)...)
	}
	fi, err := os.Lstat(ws)
	if err != nil || !fi.IsDir() {
		return nil, bad("worktree %s ausente ou não é diretório", ws)
	}
	dotgit := filepath.Join(ws, ".git")
	fi, err = os.Lstat(dotgit)
	if err != nil {
		return nil, bad("%s ilegível: %v", dotgit, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, bad("%s foi substituído por %s (esperado arquivo regular gitdir:); commit recusado", dotgit, fi.Mode().Type())
	}
	if fi.Size() > 4096 {
		return nil, bad("%s grande demais", dotgit)
	}
	data, err := os.ReadFile(dotgit) //nolint:gosec // caminho do worktree do próprio ticket
	if err != nil {
		return nil, bad("%s: %v", dotgit, err)
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir:") || strings.ContainsAny(line, "\n\r") {
		return nil, bad("%s com conteúdo inesperado", dotgit)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(ws, gitDir)
	}
	gitDir = filepath.Clean(gitDir)

	out, err := gitOut(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	common, err := filepath.EvalSymlinks(out)
	if err != nil {
		return nil, bad("git-common-dir: %v", err)
	}
	realGitDir, err := filepath.EvalSymlinks(gitDir)
	if err != nil {
		return nil, bad("gitdir %s: %v", gitDir, err)
	}
	if filepath.Dir(realGitDir) != filepath.Join(common, "worktrees") {
		return nil, bad("gitdir %s não está em %s/worktrees", realGitDir, common)
	}
	// o registro do git (fora do worktree) deve apontar de volta para <ws>/.git
	back, err := os.ReadFile(filepath.Join(realGitDir, "gitdir")) //nolint:gosec // dentro do .git do repositório
	if err != nil {
		return nil, bad("registro do worktree ilegível: %v", err)
	}
	backReal, err1 := filepath.EvalSymlinks(strings.TrimSpace(string(back)))
	wantReal, err2 := filepath.EvalSymlinks(dotgit)
	if err1 != nil || err2 != nil || backReal != wantReal {
		return nil, bad("registro do worktree em %s não corresponde a %s", realGitDir, dotgit)
	}
	return &trustedGit{root: root, gitDir: realGitDir, ws: ws}, nil
}

// run executa git com --git-dir/--work-tree explícitos, cwd na raiz (nunca no
// worktree) e ambiente endurecido. Como --git-dir aponta para fora do worktree,
// o git nunca lê <ws>/.git nem o config dele.
//
// Por que .gitattributes no worktree é inofensivo: drivers de filtro/diff/merge
// (filter.*.clean, diff.*.textconv, merge.*.driver) só executam se definidos em
// config, e todo config lido (repo, worktree-config em gitDir, global) fica fora
// do alcance de escrita do agente. O agente pode escrever .gitattributes, mas
// sem driver configurado o atributo é ignorado.
func (g *trustedGit) run(args ...string) (string, error) {
	full := append([]string{"--git-dir=" + g.gitDir, "--work-tree=" + g.ws}, hardenedGitArgs...)
	full = append(full, args...)
	cmd := exec.Command("git", full...) //nolint:gosec // argumentos controlados pelo router
	cmd.Dir = g.root
	cmd.Env = append(cleanGitEnv(os.Environ()), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := runTracked(cmd)
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// cleanGitEnv remove variáveis que redirecionariam o repositório.
func cleanGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		switch k {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_NAMESPACE", "GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_EXTERNAL_DIFF", "GIT_PAGER", "GIT_SSH", "GIT_SSH_COMMAND":
			continue
		}
		out = append(out, e)
	}
	return out
}

// commitWorkspace comita mudanças do worktree e devolve o HEAD resultante.
// A identidade vem de identityArgs (lida da raiz com a config normal; fallback tb).
// Antes de qualquer git, verifica a integridade do worktree (finding 1).
func (r *Router) commitWorkspace(dir string, t ticket.Ticket) (string, error) {
	g, err := verifyWorkspace(r.opts.Root, dir)
	if err != nil {
		return "", err
	}
	if err := refuseNestedGit(dir); err != nil {
		return "", err
	}
	status, err := g.run("status", "--porcelain", "--ignore-submodules=all")
	if err != nil {
		return "", err
	}
	if status != "" {
		if _, err := g.run("add", "-A"); err != nil {
			return "", err
		}
		if err := refuseStagedSubmodules(g); err != nil {
			return "", err
		}
		args := append(r.identityArgs(), "commit", "--no-verify", "-q", "-m", fmt.Sprintf("tb: %s %s", t.ID, t.Title))
		if _, err := g.run(args...); err != nil {
			return "", err
		}
	}
	return g.run("rev-parse", "HEAD")
}

// refuseNestedGit recusa o worktree se houver qualquer entrada ".git" (arquivo,
// diretório ou symlink) além do ponteiro raiz: um repositório aninhado (p.ex.
// dentro de um gitlink do commit base) traria config/filtros controlados pelo
// agente para o git que o router roda FORA do sandbox. WalkDir não segue
// symlinks.
func refuseNestedGit(ws string) error {
	rootGit := filepath.Join(ws, ".git")
	var found string
	err := filepath.WalkDir(ws, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" && path != rootGit {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if found != "" {
		rel, _ := filepath.Rel(ws, found)
		return fmt.Errorf("%w: .git aninhado em %s (repositório/submódulo criado pelo agente); commit recusado", errWorkspaceIntegrity, rel)
	}
	if err != nil {
		return fmt.Errorf("%w: varredura do worktree: %v", errWorkspaceIntegrity, err)
	}
	return nil
}

// refuseStagedSubmodules recusa o commit se o índice adicionou/alterou um
// gitlink (modo 160000) ou .gitmodules. --ignore-submodules=none sobrepõe o
// diff.ignoreSubmodules=all endurecido.
//
// Nota: .gitattributes/.gitmodules no worktree não executam nada por si: drivers
// filter/diff/merge só resolvem contra config (repositório, worktree-config),
// todo fora do alcance de escrita do agente; core.attributesFile e include.path
// também vivem nesse config. Config global e de sistema ficam desligadas no git
// do router (trustedGit.run), então filtros do usuário (git-lfs) não rodam com o
// conteúdo do agente; um filtro definido na config do PRÓPRIO repositório ainda
// rodaria (escolha do dono do repositório).
func refuseStagedSubmodules(g *trustedGit) error {
	out, err := g.run("diff", "--cached", "--raw", "-z", "--no-renames", "--ignore-submodules=none")
	if err != nil {
		return err
	}
	toks := strings.Split(out, "\x00")
	for i := 0; i+1 < len(toks); i++ {
		hdr := toks[i]
		if !strings.HasPrefix(hdr, ":") {
			continue
		}
		f := strings.Fields(hdr)
		path := strings.TrimSpace(toks[i+1])
		if len(f) < 5 {
			continue
		}
		newMode, status := f[1], f[4]
		if newMode == "160000" || (path == ".gitmodules" && !strings.HasPrefix(status, "D")) {
			return fmt.Errorf("%w: commit adiciona/altera gitlink ou .gitmodules (%s); recusado", errWorkspaceIntegrity, path)
		}
	}
	return nil
}

// withGitNoDetach impede que o git DO AGENTE lance manutenção destacada
// (gc --auto faz setsid) que seria indistinguível de um processo escapado.
// Não sobrescreve GIT_CONFIG_COUNT já definido no ambiente.
func withGitNoDetach(env []string) []string {
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_CONFIG_COUNT=") {
			return env
		}
	}
	return append(env,
		"GIT_CONFIG_COUNT=3",
		"GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=gc.auto", "GIT_CONFIG_VALUE_1=0",
		"GIT_CONFIG_KEY_2=gc.autoDetach", "GIT_CONFIG_VALUE_2=false",
	)
}

// identityArgs devolve os "-c user.name=… -c user.email=…" do commit do router.
// Como o git confiável ignora a config global, a identidade é lida uma vez da
// raiz do repositório com a config normal (repo + global); sem ela, tb/tb@localhost.
func (r *Router) identityArgs() []string {
	r.identOnce.Do(func() {
		name, _ := gitOut(r.opts.Root, "config", "--get", "user.name")
		email, _ := gitOut(r.opts.Root, "config", "--get", "user.email")
		if name == "" {
			name = "tb"
		}
		if email == "" {
			email = "tb@localhost"
		}
		r.ident = []string{"-c", "user.name=" + name, "-c", "user.email=" + email}
	})
	return append([]string(nil), r.ident...)
}
