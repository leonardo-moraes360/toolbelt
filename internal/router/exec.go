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

// wsCreateMu serializes createWorkspace across workers (git worktree add
// contends for ref locks). Sweep/settleGroup do NOT take it: the workspace
// manager's git is tracked in the Registry (SetRunner), so sweeps never mistake
// it for an orphan nor wait for it.
var wsCreateMu sync.Mutex

// procs tracks every direct child of the router (agents and git). Sweeps kill
// and reap any untracked direct child: agent descendants that called
// setsid/setpgid and were reparented to the router (Linux,
// PR_SET_CHILD_SUBREAPER). TB_TICKET_ID (inherited from the agent's environment)
// attributes the escape to a ticket and also names the owner of each agent group
// (a process that joins another agent's group via setpgid is detected).
var procs = func() *sandbox.Registry {
	r := sandbox.NewRegistry(ticketIDVar)
	r.SetOwnerVar(ticketIDVar)
	return r
}()

// workspaceGitArgs neutralize user/repository code in the workspace manager's
// git (worktree add/remove): no hooks (post-checkout), fsmonitor or detached
// maintenance, which would show up as orphan daemons.
var workspaceGitArgs = []string{
	"-c", "core.hooksPath=" + os.DevNull,
	"-c", "core.fsmonitor=false",
	"-c", "maintenance.auto=false",
	"-c", "gc.auto=0",
	"-c", "gc.autoDetach=false",
}

// newWorkspaceManager tracks its git in the Registry (never mistaken for an
// orphan, no global lock during spawn) and disables user hooks/fsmonitor/
// maintenance (workspaceGitArgs): router-created worktrees never run
// repository/user code.
func newWorkspaceManager(root string) (*workspace.Manager, error) {
	wsm, err := workspace.NewManager(root)
	if err != nil {
		return nil, err
	}
	wsm.SetRunner(runTracked)
	wsm.SetGitArgs(workspaceGitArgs...)
	return wsm, nil
}

// runTracked registers the pid in procs (a tracked CombinedOutput).
func runTracked(cmd *exec.Cmd) ([]byte, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := procs.Run(cmd, false)
	return buf.Bytes(), err
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // args controlled by the router
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := runTracked(cmd)
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// execute runs in the worker goroutine. It never touches router state;
// everything is returned through workerDone.
func (r *Router) execute(ctx context.Context, s *slot, worker string, t *ticket.Ticket, base string) workerDone {
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
	r.takeEscaped(t.ID) // a stale mark from a previous attempt must not poison this one
	r.markActive(t.ID, worker)
	defer r.unmarkActive(t.ID)
	res, err := sb.Run(ctx, r.argv[0], r.argv[1:]...)
	if res != nil {
		r.appendLog(ev.log, worker, t.ID, res.Output)
		ev.exit = res.ExitCode
		ev.canceled = res.Canceled
		ev.timedOut = res.TimedOut
	}
	// Descendants that escaped the group (setsid/setpgid) were already killed by
	// Sweep. Fail THIS ticket only if an escape was attributed to it (via
	// TB_TICKET_ID or, when attribution is impossible, because it was running at
	// detection time); escapes of other tickets swept here only mark them.
	ev.alerts = r.collectEscapes()
	if r.takeEscaped(t.ID) {
		ev.err = fmt.Errorf("%w: processo(s) escaparam do grupo do agente (setsid/setpgid) e foram encerrados; agentes não podem se desacoplar", errAgentEscaped)
		return ev
	}
	if err != nil {
		ev.err = err
		return ev
	}
	// Do not mark canceled just because ctx was canceled afterwards: if the agent
	// already exited 0 before the cancellation, the work is valid (finding 4).
	if ev.exit != 0 || ev.canceled || ev.timedOut {
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
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // inside .toolbelt/router/logs
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Chmod(0o600) // old logs were created with 0644
	// Agent output is already capped (sandbox.MaxOutputBytes, tail only);
	// enforce the cap again in case the caller changes.
	if len(output) > sandbox.MaxOutputBytes {
		output = output[len(output)-sandbox.MaxOutputBytes:]
	}
	fmt.Fprintf(f, "=== %s %s %s ===\n%s", id, worker, r.now().UTC().Format(time.RFC3339), output)
	if output != "" && !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(f)
	}
}

// createWorkspace cleans up leftovers from previous attempts. Serialized: git
// worktree add contends for ref locks.
func (r *Router) createWorkspace(t *ticket.Ticket, base string) (*workspace.Workspace, error) {
	wsCreateMu.Lock()
	defer wsCreateMu.Unlock()
	// Spawn window: git (and whatever it triggers) may leave detached daemons
	// that belong to no agent; see Registry.BeginSpawnWindow.
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

// errWorkspaceIntegrity means the agent tampered with the worktree (or it does
// not match the one registered in the repository); git never runs in it and the
// ticket fails without retry.
var errWorkspaceIntegrity = errors.New("integridade do worktree violada")

// errAgentEscaped: the agent left descendants outside its process group. It is
// an integrity violation (no retry, no git in the worktree).
var errAgentEscaped = fmt.Errorf("%w: agent_escaped", errWorkspaceIntegrity)

// hardenedGitArgs neutralize code execution via config: the router runs git
// OUTSIDE the sandbox, so nothing the agent can write may become a command
// (fsmonitor, hooks, ssh, external diff, pager, signing).
var hardenedGitArgs = []string{
	"-c", "core.fsmonitor=false",
	"-c", "core.hooksPath=" + os.DevNull,
	"-c", "core.sshCommand=false",
	"-c", "diff.external=",
	"-c", "core.pager=cat",
	"-c", "commit.gpgsign=false",
	// Submodules: the agent can create <gitlink>/.git with its own config; without
	// these options status/diff/add would recurse into it and run its filters.
	"-c", "diff.ignoreSubmodules=all",
	"-c", "status.submoduleSummary=false",
	"-c", "submodule.recurse=false",
	"-c", "fetch.recurseSubmodules=false",
	// git commit may launch a detached `maintenance`/`gc --auto` (setsid): it would
	// become an orphan that Sweep mistakes for an escaped agent.
	"-c", "maintenance.auto=false",
	"-c", "gc.auto=0",
	"-c", "gc.autoDetach=false",
}

// trustedGit is a verified worktree: gitDir is the <common>/worktrees/<name>
// directory registered by git itself (outside the worktree, hence outside the
// agent's write reach).
type trustedGit struct {
	root, gitDir, ws string
}

// verifyWorkspace ensures <ws>/.git is a regular file (not a symlink, not a
// directory) whose "gitdir:" points to a worktree registered in
// <git-common-dir>/worktrees/<name> of the root repository, and that this
// registration points back to <ws>/.git. The common dir comes from
// `git rev-parse` with cwd at the root (never in the worktree).
func verifyWorkspace(root, ws string) (*trustedGit, error) {
	dotgit := filepath.Join(ws, ".git")
	gitDir, err := readGitdirPointer(ws, dotgit)
	if err != nil {
		return nil, err
	}
	realGitDir, err := checkRegisteredWorktree(root, dotgit, gitDir)
	if err != nil {
		return nil, err
	}
	return &trustedGit{root: root, gitDir: realGitDir, ws: ws}, nil
}

func integrityErr(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{errWorkspaceIntegrity}, a...)...)
}

// readGitdirPointer requires <ws>/.git to be a small regular file of the form
// "gitdir: <path>" and returns the path (absolute and cleaned).
func readGitdirPointer(ws, dotgit string) (string, error) {
	fi, err := os.Lstat(ws)
	if err != nil || !fi.IsDir() {
		return "", integrityErr("worktree %s ausente ou não é diretório", ws)
	}
	fi, err = os.Lstat(dotgit)
	if err != nil {
		return "", integrityErr("%s ilegível: %v", dotgit, err)
	}
	if !fi.Mode().IsRegular() {
		return "", integrityErr("%s foi substituído por %s (esperado arquivo regular gitdir:); commit recusado", dotgit, fi.Mode().Type())
	}
	if fi.Size() > 4096 {
		return "", integrityErr("%s grande demais", dotgit)
	}
	data, err := os.ReadFile(dotgit) //nolint:gosec // path inside the ticket's own worktree
	if err != nil {
		return "", integrityErr("%s: %v", dotgit, err)
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir:") || strings.ContainsAny(line, "\n\r") {
		return "", integrityErr("%s com conteúdo inesperado", dotgit)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(ws, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

// checkRegisteredWorktree confirms gitDir is registered under
// <git-common-dir>/worktrees and points back to dotgit; it returns the real
// gitDir (symlinks resolved).
func checkRegisteredWorktree(root, dotgit, gitDir string) (string, error) {
	out, err := gitOut(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	common, err := filepath.EvalSymlinks(out)
	if err != nil {
		return "", integrityErr("git-common-dir: %v", err)
	}
	realGitDir, err := filepath.EvalSymlinks(gitDir)
	if err != nil {
		return "", integrityErr("gitdir %s: %v", gitDir, err)
	}
	if filepath.Dir(realGitDir) != filepath.Join(common, "worktrees") {
		return "", integrityErr("gitdir %s não está em %s/worktrees", realGitDir, common)
	}
	// git's registration (outside the worktree) must point back to <ws>/.git
	back, err := os.ReadFile(filepath.Join(realGitDir, "gitdir")) //nolint:gosec // inside the repository's .git
	if err != nil {
		return "", integrityErr("registro do worktree ilegível: %v", err)
	}
	backReal, err1 := filepath.EvalSymlinks(strings.TrimSpace(string(back)))
	wantReal, err2 := filepath.EvalSymlinks(dotgit)
	if err1 != nil || err2 != nil || backReal != wantReal {
		return "", integrityErr("registro do worktree em %s não corresponde a %s", realGitDir, dotgit)
	}
	return realGitDir, nil
}

// run executes git with explicit --git-dir/--work-tree, cwd at the root (never
// in the worktree) and a hardened environment. Since --git-dir points outside
// the worktree, git never reads <ws>/.git or its config.
//
// Why .gitattributes in the worktree is harmless: filter/diff/merge drivers
// (filter.*.clean, diff.*.textconv, merge.*.driver) only run if defined in
// config, and all config read (repo, worktree config in gitDir, global) is out
// of the agent's write reach. The agent can write .gitattributes, but without a
// configured driver the attribute is ignored.
func (g *trustedGit) run(args ...string) (string, error) {
	full := append([]string{"--git-dir=" + g.gitDir, "--work-tree=" + g.ws}, hardenedGitArgs...)
	full = append(full, args...)
	cmd := exec.Command("git", full...) //nolint:gosec // args controlled by the router
	cmd.Dir = g.root
	cmd.Env = append(cleanGitEnv(os.Environ()), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := runTracked(cmd)
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// cleanGitEnv drops variables that would redirect the repository.
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

// commitWorkspace returns the resulting HEAD. The identity comes from
// identityArgs (read from the root with normal config; tb fallback). Worktree
// integrity is verified before any git call (finding 1).
func (r *Router) commitWorkspace(dir string, t *ticket.Ticket) (string, error) {
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

// refuseNestedGit rejects the worktree if it has any ".git" entry (file,
// directory or symlink) besides the root pointer: a nested repository (e.g.
// inside a gitlink of the base commit) would bring agent-controlled
// config/filters into the git the router runs OUTSIDE the sandbox. WalkDir does
// not follow symlinks.
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

// refuseStagedSubmodules rejects the commit if the index added/changed a
// gitlink (mode 160000) or .gitmodules. --ignore-submodules=none overrides the
// hardened diff.ignoreSubmodules=all.
//
// Note: .gitattributes/.gitmodules in the worktree execute nothing by
// themselves: filter/diff/merge drivers only resolve against config (repository,
// worktree config), all out of the agent's write reach; core.attributesFile and
// include.path live in that config too. Global and system config are disabled in
// the router's git (trustedGit.run), so user filters (git-lfs) never run on agent
// content; a filter defined in the repository's OWN config would still run (the
// repository owner's choice).
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

// withGitNoDetach stops the AGENT's git from launching detached maintenance
// (gc --auto calls setsid), which would be indistinguishable from an escaped
// process. It does not override a GIT_CONFIG_COUNT already set in the
// environment.
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

// identityArgs returns the "-c user.name=… -c user.email=…" for router
// commits. Since the trusted git ignores global config, the identity is read
// once from the repository root with normal config (repo + global); without
// it, tb/tb@localhost.
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
