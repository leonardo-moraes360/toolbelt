🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/egm-nativo.md)

# Spec: native EGM, Phase A, core (`tb egm review|loop|resolve|status`)

Author: toolbelt · Status: R18–R27 revised with the R0 data and simplified after the re-measurement (not reviewed); R0 open only for item 8; implementation slices 1 (deterministic engine, interactive mode), 2 (`llm` engine, arbiter) and 3 (gate mode, `tb stack` op, MCP) done · Date: 2026-11-17 · Decision: [ADR-015](../adr/015-egm-nativo.md) · Companion: [egm trust](egm-trust.md)

## Overview

`tb egm` reviews **design documents** (specs and ADRs) with goldfish (reviewers in a fresh context) and repeats the review loop until it converges.

This spec is the **core** of Phase A:
- isolation of the reviewers;
- review, loop, resolve and arbiter;
- defenses against **third-party content** (cloned repository, downloaded file, reviewed document).

The protections against active fraud by the author itself, who runs as the same user, live in the [egm trust](egm-trust.md) spec: hash chain, anchor, pins, TTY confirmation and gate by trust ref.

## Assumed Threat Model

- **Hostile content (covered):** the repository, the downloaded file, its `.git/` and the reviewed document may be hostile. `tb egm` must not execute anything from that content nor leak any user secret to the provider.
- **Honest but sloppy elephant (covered):** whoever writes the document may try accidental or lazy shortcuts, such as marking as fixed without fixing, contesting until it passes or downgrading severity. The loop does not let these shortcuts become `PASS`.
- **Active fraud by the same user (out of scope):** a process with a shell as the same user can edit the config or the state, swap the agent binary or forge inputs. This is left to the [egm trust](egm-trust.md) spec, and the core promises nothing against this case.

## Glossary

| Term | Definition |
|------|------------|
| elephant | Whoever wrote the document and calls `tb egm`: the main agent or a human. |
| goldfish | One-shot reviewer in an isolated process. Sees only its own prompt. |
| reviewer | A slot of the preset (R10), for example `style-feature`. Each reviewer uses one persona; a persona can serve several reviewers. |
| owning reviewer | The reviewer responsible for a lineage and the only one that re-evaluates it (R19, R20). |
| arbiter | Goldfish that judges a contest (R29). |
| persona | The reviewer's lens: what to look for and how to judge (`personas/<name>.md`). |
| standard | Quality criterion (`standards/<name>.md`). The structural ones list required headings (R13). |
| document type | `spec` or `adr` (R6). Determines the structural standard; the detailed plan determines the required reviewers. |
| gated document | A document that matches `egm.doc_globs` (R33), except a directory index named `README.md` (case-insensitive), which lists documents instead of being one. |
| heading | An ATX line (`#` to `######` followed by a space) outside a fenced code block. Comparisons use the whole line, with the `#`, trimmed at both ends. |
| `consolidated` preset | A round with the required reviewers (R10) in parallel, without checkpoints. |
| user config | The `egm:` section of `config.GlobalConfigPath()`. |
| `<state>` | `$XDG_STATE_HOME/tb` or, without that variable, `~/.local/state/tb`. On macOS, `os.UserConfigDir()/toolbelt/state`. |
| state directory (`<state-dir>`) | `<state>/egm/<repo-id>/<slug>/`. Lives outside any worktree and is shared by the worktrees of the same repository (R21). |
| packet | Read-only directory inside the state directory, with what was handed to a goldfish. It is also its workdir (R1). |
| nonce | A random 128-bit value per run. Delimits the data blocks and must be echoed in the output. Proves the output is from this run, **not** that the model resisted injection. |
| attestation | The `checked` list of literal excerpts from the document that the reviewer read, checked by tb (R17). |
| lineage's cited sources | The document plus the union of the **verified** `evidence[].path` of the occurrences that went through `resolveSource` in the round (`cited`, R21). |
| `origin_round` / `opened_round` | Round in which the lineage was born / last round in which it became `open` or `reopened` (a reopening updates `opened_round`). |
| blocking lineage | A blocker or major lineage in `open`, `reopened`, `contest_pending`, `escalated` or unconfirmed `fixed`. |
| policy | `policy_sha`: the sha256 of the canonical JSON with the effective required reviewers (the full set), `clean_rounds`, `match`, `attest`, `detail_headings`, `doc_types`, `refs_ignore`, `max_source_bytes` and `max_packet_bytes` (R21). The engine and the profile are not included; they are recorded in the rounds, and their effects are already downgrades (R24). |
| content | `content`: the sha256 of the path-sorted list of `(path, sha256 of the bytes)` pairs of the document and of all sources resolved by R7 from the document, before per-reviewer packaging and regardless of who ran, plus `checks_sha` (the sha256 of the checkers' inputs: each checked path with type and existence). The bytes come from the worktree in interactive mode and from the blob in gate mode (R7a). |
| convergence record | A record that ties a `content` to a convergence (R23). |
| counted rounds | The `egm.clean_rounds` consecutive clean rounds, over the same `content`, that produced a convergence record. |
| cycle | Rounds since the last convergence, or since `review`/`--restart`. Cap and budget count per cycle (R23, R27). |
| slug | Identifier of the document (R14). |
| FeatureRange / DetailedRange | Lines before the detailed-plan heading and from it onward (R6). Without that heading, FeatureRange is the whole document and DetailedRange is empty. |
| lineage `F-<k>` | Stable ID of a finding across rounds (`k` sequential per slug). |
| occurrence `<reviewer>-r<round>-<i>` | ID of a finding in one round. |
| reviewer status | `completed` (valid output), `not_run` (failed, R16, or was interrupted, R3/R27), `unavailable` (LLM persona without an agent) or `skipped` (no applicable range, R6/R17). |
| vendor profile | Per-CLI recipe: argv, no-tools flags, authentication, output envelope and tool-use detection (R4, R5, R15). |

## Problem

`tb review` reviews diffs with heuristics. It does not review documents, has no LLM reviewer and does not converge. The upstream EGM does this in prose and leaves isolation, state and approval up to the harness and the model. See [ADR-015](../adr/015-egm-nativo.md#context).

## Goals

- G1: isolate each reviewer by process, sandbox, disposable HOME, minimal env and absence of tools, and verify that absence at run time.
- G2: produce a structured finding (`egm.Finding`) that `tb stack` and MCP can consume.
- G3: leave convergence up to tb, persisted and tied to an exact content. A sloppy shortcut (previous section) does not reach `PASS`.
- G4: run `tb egm` over a third-party repository or a downloaded file without executing any binary and without exposing a secret chosen by that content, whether via config, symlink or git config.
- G5: have a useful offline mode, with declared coverage, that does not pass a strict gate.
- G6: keep the CLI and JSON of `tb review` and `tb drift`, and the behavior of `tb merge` and the router, unchanged.

## Non-Goals

- tb asks the user nothing and does not edit the reviewed document.
- tb does not change the severity given by a reviewer. An output above the persona's cap is invalid (R11).
- tb does not talk to the network nor call an LLM API.
- Gate in CI: the gate is **local** (R33a).
- Protection against active fraud by the same user: left to [egm trust](egm-trust.md).
- Hardening the loaders that `tb stack` already runs before any op: the `git diff base...head` and the `config.Load()` of `Prepare`, which today run without the R4c and R34 protections. In an untrusted repository, use `tb egm status --at <rev>` directly, which is hardened.
- Out of this phase:
  - `staged`/`feature`/`implementation` presets;
  - personas by path;
  - `intermediate-zoom-out`;
  - `codex` profile;
  - per-reviewer agent override;
  - symbol verification in `tb graph`;
  - churn;
  - `review.Finding` → `egm.Finding` adapter;
  - drift-report, bootstrap, draft and brainstorm.
- The upstream personas' text is not copied.

## Critical User Journeys (CUJs)

- **CUJ-1.** *Given* `docs/specs/x.md` (versioned or not) and the `claude` profile configured, *when* I run `tb egm review docs/specs/x.md`, *then* the required reviewers run isolated and I get `round-1/report.{md,json}` with the verdict `IN_PROGRESS`. <!-- tb:egm-example -->
- **CUJ-2.** *Given* there is no agent, *when* I run the same command, *then* the deterministic checkers run, the LLM personas are `unavailable`, tb explains why it did not use `llm` and the best possible verdict is `PASS_PARTIAL`.
- **CUJ-3.** *Given* round 1 has findings, *when* I edit the pointed lines (or the cited code), run `tb egm resolve docs/specs/x.md --fix F-1,F-2` and then `tb egm loop next docs/specs/x.md` twice, *then* the owning reviewer re-evaluates each lineage and the last two rounds, clean and over the same content, produce a convergence record. <!-- tb:egm-example -->
- **CUJ-4.** *Given* the blocker `F-3`, which cites `internal/x.go`, *when* I run `tb egm resolve docs/specs/x.md --contest F-3 --evidence internal/x.go:40`, *then* F-3 becomes `contest_pending` and the next `loop next` runs the arbiter first. A second contest of F-3 is refused. <!-- tb:egm-example -->
- **CUJ-5.** *Given* a cloned repository whose `.toolbelt/config.yaml` defines `egm.agent_cmd` and `egm.env.pass`, *when* I run `tb egm review`, *then* those values are ignored, tb emits `config_untrusted` and nothing from the repository is executed.
- **CUJ-6.** *Given* a downloaded file with hostile `.git/config` and `.gitattributes` (filters, fsmonitor, `core.worktree`, partial clone), a forged `.git/index` and a versioned directory that is a symlink to `~/.config`, *when* I run `tb egm review`, *then* nothing is executed and nothing outside the repository and its common git directory is read.
- **CUJ-7.** *Given* I reviewed a spec in a feature worktree, merged it and removed the worktree, *when* I run `status` in another worktree with the same content, *then* the verdict is the same.
- **CUJ-8.** *Given* I converge on version A and commit version B, *when* I run `tb stack egm`, *then* the gate evaluates B (the `head`), finds no record and answers `STALE`.

## Technical Plan

```
elephant ──tb egm review|loop next|resolve|status──┐
                                                    ▼
┌─────────────────────────── tb ───────────────────────────┐
│ phase 1 (no goldfish): config → real repo root →         │
│   snapshot → sources (real path) → redact → scan         │
│ phase 2: goldfish in sandbox.New(RO packet, Tmp, Home,   │
│   minimal env, Stdin, Stdout), no tools; checkers        │
│ phase 3 (no goldfish): validate (schema, cap, nonce,     │
│   attestation, tool-use, evidence) → match → lineage     │
│   → state.json (atomic) → convergence → verdict          │
└──────────────────────────────────────────────────────────┘
```

tb only calls git in phases 1 and 3, never while a goldfish is running (R3).

### Requirements

**R0: spike.** This requirement blocks R1, R2, R3, R4, R4b and R5, and the move of ADR-015 to `accepted`. R4c and R4d do not depend on the spike. R18 to R27 were revised with the R0 data ([dogfood #31–#33](../campo/dogfood.md)). Convergence was measured twice and neither run converged within its budget; Phase A accepts that (see Open Questions), so only item 8 still blocks ADR-015 from `accepted`.

A prototype in `bench/egm-spike/` must prove that `claude -p`:
1. runs in `sandbox.New` with the packet as a read-only workdir, private `TMPDIR`, disposable HOME, minimal env (R4b) and minimal `PATH`;
2. receives the prompt via stdin and returns JSON on stdout, separate from stderr;
3. runs **with no tools at all**, and the envelope makes it possible to detect when a tool is used (R15a);
4. does not load `~/.claude/CLAUDE.md`, the user's memories or settings;
5. produces a valid attestation (R17);
6. leaves no orphan child process without `TB_EGM_OWNER` (R3);
7. cannot read `tb`'s `/proc/<ppid>/environ` (Landlock restricts ptrace to processes outside the domain), nor `/dev/pts/*` and `/dev/shm/*` (R2, `MinimalDev`). R1 is adjusted according to the result;
8. with explicit `GIT_DIR` and a repository owned by another uid, git accepts the read with `-c safe.directory`, or does not even do the owner check.

The spike must also:
- **Authentication:** define the order of preference: (1) API key via `egm.env.pass`; (2) the vendor's config dir in the disposable HOME, for example `CLAUDE_CONFIG_DIR`; (3) copying a credential file, only as a last resort.
- **Token refresh:** test whether a refresh in the disposable HOME invalidates the real credential.
- **Runtime:** define the allowed runtime directories (R4).
- **macOS:** record the Keychain as a limitation.
- **Matching:** collect at least 30 paraphrased pairs and 30 distinct pairs in the same region, and calibrate R18–R20 for wrong merges ≤ 5% and missed matches ≤ 20%. **Outcome:** not met across documents (R18); Phase A accepts it, because a missed match only duplicates a lineage.
- **Convergence:** measure the median number of rounds until 2 clean ones on 2 or 3 real documents. If it exceeds 4, `max_rounds` and the clean-round rule are revised before implementing. **Outcome:** it exceeded 4 twice; `max_rounds` went to 8 and the clean-round rule was revised and then simplified (R20, R23, Open Questions).

The result goes in `docs/campo/dogfood.md` (PT and EN).

**Sandbox and execution**

- R1: In each round, tb writes `<state-dir>/round-<n>/target.md`, the redacted snapshot of the document, and `round-<n>/sources/<path>`, the redacted snapshot of each source sent to some goldfish (used by `--fix`, R28). For each LLM goldfish, it also writes `round-<n>/packets/<reviewer>/` with:
  - `prompt.txt`;
  - `sources.json`: path, `sha256` and the included lines;
  - `meta.json`: persona, standard, range, nonce and `argv_sha256`.

  Arbiters (R29) live in `<state-dir>/arbiter-<m>/<F-k>/`, where `<m>` is a sequential counter of arbiter steps, with owner `egm:<slug>:a<m>:<F-k>`.

  The content comes from the **worktree** in interactive mode and from the evaluated commit in gate mode (R33). The goldfish's workdir is the packet, outside the repository.

  Landlock allows reading the packet, the R4 paths and the system rules that already exist in `addSystemReadRules`: `/bin`, `/sbin`, `/lib*`, `/usr/bin`, `/usr/sbin`, `/usr/lib*`, `/usr/libexec`, `/usr/share`, `/etc`, `/dev`, `/proc` and `/sys`. Reading another process's `/proc/<pid>/environ` goes through the ptrace check, which Landlock restricts for processes outside the domain; R0 confirms this. The absence of tools (R5, verified by R15a) is a second barrier, not the only one.
- R2: `sandbox.Config` gains the fields below. With them zeroed, nothing changes.
  - `ReadOnlyWorkDir bool`: the workdir rule, read-write today, becomes read-only.
  - `PrivateTmp string`: writable directory that replaces the `os.TempDir()` rule, exported as `TMPDIR`.
  - `Home string`: writable directory, exported as `HOME`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME` and `XDG_DATA_HOME`.
  - `Path string`: exported as `PATH`.
  - `Stdin io.Reader`.
  - `MinimalDev bool`: the rule that reads all of `/dev` is replaced by rules only for `/dev/null`, `/dev/zero`, `/dev/random` and `/dev/urandom`, without `/dev/pts` and without `/dev/shm`.
  - `Stdout io.Writer` and `MaxStdoutBytes int`: stdout goes only to this writer. If the limit overflows, tb keeps reading and discarding the rest (the child does not block on the pipe) until the process ends or hits the timeout, and `Result` carries `StdoutOverflow: true`. stderr stays in the `tailBuffer`.

  `Home` and `Path` override `Config.Env`. Change in `addRule`: for a regular file, the mask becomes `accessMask(readOnly, abi)` without the directory-only rights (`ReadDir`, `Remove*`, `Make*`, `Refer`), as `deviceFileMask` already does.
- R3: the `tb egm` process creates **one** `sandbox.Registry` with `NewRegistry("TB_EGM_OWNER")` and `SetOwnerVar("TB_EGM_OWNER")`, shared by all goldfish and arbiters, and calls `sandbox.AcquireSubreaper()` once. Without a subreaper (`ok=false`), an explicit `--engine llm` exits 2; with `auto`, R12 applies.
  - **Each goldfish:** uses `sandbox.New` with `ProcessGroup: true` and that Registry, `GroupOwner = egm:<slug>:r<n>:<reviewer>` (for the arbiter, `egm:<slug>:a<m>:<F-k>`, R1) and the same string in `TB_EGM_OWNER`.
  - **Git:** tb does not call git while goldfish are running. Git calls (R4c) happen only in phases 1 and 3.
  - **Sweep:** each `Sweep()` attributes orphans by `Orphan.Vars["TB_EGM_OWNER"]` and by `GroupOwner`.
    - Orphan of a goldfish of the round, including a `ForeignGroup` with that owner: that goldfish becomes `not_run` with `reason: escaped_process`.
    - Attribution uses `Orphan.Vars["TB_EGM_OWNER"]` first, which is the owner the process itself declares, and then `Orphan.GroupOwner`.
    - Orphan with no owner during phase 2: tb kills the groups of every goldfish still running, marks them all `not_run` with `reason: unattributed_orphan`, and the round is not clean. The round is not aborted.
  - **When it runs:** `Sweep()` runs after each goldfish finishes and at the end of phase 2.
  - **Timeout:** `context.WithTimeout(egm.timeout_per_reviewer)`.
  - **Platform:** the `llm` engine requires Linux with Landlock.
- R4: for the `claude` profile, tb:
  1. creates, **per goldfish**, the HOME and the `PrivateTmp` in `os.TempDir()/tb-egm-<rand>/`, with permission `0700` and an `owner.pid` file with `tb`'s pid;
  2. authenticates in the way defined in R0;
  3. allows read-only access to the resolved binary's directory, to the directories in `egm.home.read_only` and, as a file rule, to the realpath of `/etc/resolv.conf` when it lives outside `/etc` (systemd-resolved points it into `/run`; without it DNS fails, an R0 finding);
  4. deletes everything at the end (with defer, including on timeout) and sweeps the `tb-egm-*` older than 1h whose `owner.pid` is no longer alive.

  `egm.agent_cmd[0]` must be absolute. The process runs with `PATH = egm.path` (default `/usr/bin:/bin`).
- R4b (minimal env): the goldfish's env is built from scratch, without `os.Environ()`, and contains only:
  - `HOME`, `TMPDIR`, `XDG_*` and `PATH` (R2);
  - `LANG=C.UTF-8`;
  - `TB_EGM_OWNER` and `TB_EGM_ATTEMPT` (`1` or `2`);
  - the variables in `egm.env.pass`.

  tb refuses with exit 2 names in `egm.env.pass` that start with `NODE_`, `PYTHON`, `LD_`, `DYLD_`, `RUBY`, `PERL5` or `GIT_`, as well as `BASH_ENV`, `ENV` and `PROMPT_COMMAND`.
- R4c (egm's git): the `egm` package has its own git options and env in `internal/egm/git.go`. It does **not** change `internal/router` nor `internal/merge`.
  - **Options:**
    - `-c core.fsmonitor=false`, `-c core.hooksPath=/dev/null`, `-c core.sshCommand=false`;
    - `-c diff.external=`, `-c core.pager=cat`, `-c core.untrackedCache=false`;
    - `-c submodule.recurse=false`, `-c diff.ignoreSubmodules=all`;
    - `-c core.excludesFile=/dev/null`, `-c core.attributesFile=/dev/null`;
    - `-c protocol.allow=never`, `-c credential.helper=`, `-c core.askPass=`, `-c core.editor=true`, `-c core.alternateRefsCommand=`;
    - `--end-of-options` before any `<rev>`, `<a>` or `<b>`, which first go through `rev-parse --verify --end-of-options`;
    - `-c safe.directory=<real root>`, for repositories owned by another uid (file extracted with sudo, shared mounts);
    - `--no-ext-diff --no-textconv`, where the subcommand accepts them.
  - **Env:** starts from `os.Environ()` and removes everything that starts with `GIT_`. Then it adds `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_PAGER=cat`, `GIT_TERMINAL_PROMPT=0`, `GIT_NO_LAZY_FETCH=1` (ignored by old versions, which are already covered by R4d), `GIT_LITERAL_PATHSPECS=1`, `GIT_ASKPASS=` and explicit `GIT_DIR`/`GIT_WORK_TREE`, coming from R4d. The exception is `diff --no-index`, which runs without `GIT_DIR`/`GIT_WORK_TREE`, with cwd and `GIT_CEILING_DIRECTORIES` in the state directory.
  - **Timeout:** each call has a fixed 30 s timeout. If it expires, exit 2 with `reason: git_timeout`.
  - **Allowed subcommands (closed list):**

    | Subcommand | Use |
    |------------|-----|
    | `ls-files -s -z -- <paths>` | Git mode and blob in the index: candidate sources; prefix directories of `doc_globs` (R33); prefix directory of a plan `*` (R13) |
    | `ls-files -z --others --exclude-standard -- <dirs>` | Unversioned gated documents, only in the prefix directories of `doc_globs` (R33) |
    | `ls-tree -z <rev> -- <paths>` | Git mode and blob at a commit: candidate sources, the document and `.toolbelt/config.yaml` (gate mode) |
    | `ls-tree -r -z --name-only <rev> -- <dir>` | Files under the prefix directory of a plan `*`, in gate mode (R13), with at most 1000 entries |
    | `hash-object --no-filters -- <paths>` | Hash of the worktree bytes, only for sources already validated by R7 |
    | `cat-file -s <rev>:<path>`, `cat-file blob <rev>:<path>` | Size and content at a commit (gate, arbiter); the size is checked before reading |
    | `rev-parse --verify`, `merge-base <a> <b>` | Refs and three-dot base (R33) |
    | `diff --name-status --no-renames -z <a> <b>` | Tree diff between commits (R33) |
    | `diff --no-index -U0` | Diff between snapshots, outside any repository (env above) |
    | `check-ignore --no-index -- <paths>` | Exclusion of ignored files (R7) |
    | `config --file <path> --no-includes --null --list` | Reading `config` and `config.worktree` for the allowlist (R4d); executes nothing |

    Any other git call in the package is a programming error, covered by a static test. `git status` is not used, and tb never sweeps all versioned files.
- R4d (real root and trusted `.git`): tb locates the repository through the filesystem, without asking git.
  - **Discovery:** it walks up from the directory (realpath) of the **document**, not from the cwd, until it finds a `.git` entry. The search stops on reaching the realpath of `$HOME` (without inspecting it) and on crossing a filesystem (`st_dev`). With no `.git`, tb exits 2 with `reason: no_repository`. A downloaded document without `.git` inside a user repository is then reviewed as part of that repository; this is expected, and the sources stay limited by R7.
  - **Git directories:**
    - `.git` must be a directory or a regular `gitdir:` file; a symlink is refused;
    - if `.git` is a directory, `GIT_DIR` and `<common>` are that directory, and a `commondir` file inside it is refused;
    - if `.git` is a `gitdir:` file (worktree), `<common>` is the realpath of `<gitdir>/commondir`, `<gitdir>` must be `<common>/worktrees/<name>`, and `<gitdir>/gitdir` must point back to this `.git`. A `gitdir:` pointing to `<super>/.git/modules/…` (submodule) gives exit 2 with `reason: submodule_unsupported`;
    - `GIT_WORK_TREE` is always the directory that contains `.git`; a `core.worktree` has no effect, because the explicit env takes precedence;
    - the realpath of that directory is the **real root**.
  - **Inspection of `.git`**, before any git call, directly in the filesystem and without following links:
    - in `<common>` and `<gitdir>`, the entries `objects`, `objects/pack`, `objects/info`, `refs`, `packed-refs`, `info`, `worktrees`, `modules`, `HEAD`, `index`, `config`, `config.worktree`, `shallow`, `commondir`, `gitdir`, `sharedindex.*`, `info/exclude`, `info/attributes` and `info/sparse-checkout` cannot be a symlink, FIFO or device;
    - `config` and `config.worktree` are at most 1 MiB; `HEAD` is at most 4 KiB; `index` is optional (a fresh `git init` has none);
    - neither `objects/info/alternates` nor `objects/info/http-alternates` may exist.
  - **Git config, with a list of fatal keys:** tb reads `<common>/config` and `<gitdir>/config.worktree` with `git config --file <path> --no-includes --null --list` (R4c), with no parser of its own. The comparison is case-insensitive, and `<n>` is any subsection. Each key falls into one of three groups:
    1. **Fatal:** the keys that the R4c `-c` options cannot neutralize:
       - `include.*` and `includeif.*`;
       - `extensions.partialclone` and `remote.<n>.promisor`;
       - any `extensions.*` other than `objectformat`, `worktreeconfig`, `refstorage` and `preciousobjects`.

       The `objects/info/alternates` and `objects/info/http-alternates` files are also fatal (inspection above).
    2. **Neutralized**, accepted with the `git_config_neutralized` warning: the ones the R4c `-c` options override, or that no allowed subcommand reads. For example `core.hookspath`, `core.fsmonitor`, `core.sshcommand`, `core.pager`, `core.editor`, `core.askpass`, `core.excludesfile`, `core.attributesfile`, `core.worktree`, `core.alternaterefscommand`, `filter.*`, `diff.*`, `merge.<n>.driver`, `mergetool.*`, `difftool.*`, `credential.*`, `protocol.*`, `url.*`, `http.*`, `pager.*` and `submodule.recurse`.
    3. **Other keys:** accepted with no warning.

    A fatal key gives exit 2 with `reason: hostile_git_dir`. The message lists the keys.

    The security of this design does **not** depend on enumerating dangerous keys. It depends on three things: the closed list of R4c subcommands, the R4c `-c` options and env, and the sentinel tests (Verification Plan) that prove none of these subcommands executes anything with a hostile config. If a new git version starts reading a new key in an allowed subcommand, the sentinel tests are what detect it.
- R5: the goldfish's argv is `egm.agent_cmd` plus the profile flags that turn off **all** tools; all content goes in the prompt. tb refuses with exit 2:
  - argv with `bypassPermissions`, `--dangerously-skip-permissions` or anything from `egm.forbidden_flags`;
  - a `custom` profile without `egm.tools_disabled: true`.
- R6 (ranges and type):
  - **Cut:** tb splits the document at the **first heading** whose whole line equals an item of `egm.detail_headings`.
  - **Type:** `egm.doc_types` maps globs to types. The default is `docs/adr/**` → `adr`; the rest is `spec`. The type only determines the structural standard (R13).
  - **Required reviewers:**
    - with DetailedRange: the 9 of R10;
    - without DetailedRange: the 6 R10 reviewers whose range is not the DetailedRange.
  - **Coverage:** in a `spec` without DetailedRange, the 3 detail reviewers are `skipped` (`reason: no_detailed_section`), which is partial coverage (R24). In an `adr` without DetailedRange, they are not required.

**Sources, redaction and prompt**

- R7 (sources): the reviewed document and the sources go through these rules.
  - **Reviewed document:**
    - it is read by its realpath;
    - it must be a regular file inside the real root (R4d); otherwise, exit 2;
    - it may be unversioned (CUJ-1);
    - it is always a source;
    - if it exceeds `egm.max_packet_bytes`, exit 2.
  - **Source candidates:**
    - (a) relative markdown links (`CollectMDLinks`), resolved relative to the document's directory;
    - (b) single-backtick spans, extracted by egm's own regex in `internal/egm/sources.go`, resolved relative to the root.

    A span is only a candidate if it:
    - has no space, `<`, `>`, `$`, `~`, `(`, `*`, `{`, `…` nor `:`;
    - ends in an extension from egm's list (those of `rePathLike` plus `json`, `yaml`, `yml` and `toml`) or ends in `/`. `tb graph`'s `rePathLike` does not change.
  - **Resolution** (`egm.resolveSource`):
    1. strips fragment and query;
    2. refuses an absolute path;
    3. joins with the base and applies `path.Clean`;
    4. refuses if it leaves the root;
    5. applies `Lstat` to **each component** starting from the root; any component that is a symlink excludes the source (`symlink`);
    6. requires the last component to be a regular file;
    7. checks `ls-files -s`: it must be in the index with mode `100644` or `100755`.
  - **Exclusions, recorded in `skipped_sources` with the reason:**
    - outside the root, and URLs;
    - unversioned or ignored;
    - `.env*`, `*.pem`, `*.key`, `*.p12`, `id_*`, `credentials*`, `secrets*`, `*.tfvars`, `*.tfstate*`, `kubeconfig*`, `.npmrc`, `.netrc`, `.pypirc`, `service-account*.json`, `*.keystore` and `*.jks`;
    - any source in which redaction (R8) finds some secret (`secret_like`); the reviewed document is redacted, but not excluded;
    - `.toolbelt/` and `.git/`;
    - `**/testdata/**`, `internal/egm/personas/`, `internal/egm/standards/` and `internal/egm/injection_patterns.txt`;
    - **other gated documents and files under `docs/pt-BR/`**, because they are not code and are reviewed as documents;
    - above `egm.max_source_bytes`;
    - whatever exceeds `egm.max_packet_bytes`.
  - **Order of the source rules:**
    1. deny-list, `.toolbelt/`, `.git/`, `testdata` and ignored files (`check-ignore`): silent exclusion;
    2. a candidate that exists in the worktree, is not in the index and is not ignored: `unstaged_source` warning, and the round is **not clean**, with the message `faça git add das fontes citadas antes de convergir` ("git add the cited sources before converging");
    3. a versioned source whose worktree bytes differ from the index blob (partial staging): also `unstaged_source`.

    That way, the interactive mode's `content` is the same one the gate will see after the commit.
  - **Effects:**
    - an exclusion by `max_*` in a source of a reviewer with code sources makes coverage partial;
    - only reviewers with code sources (R10) and the arbiter receive sources besides the document.
- R7a (gate mode): with `--at <rev>`, resolution is the same as in R7, but over the commit's tree:
  - the mode comes from `ls-tree` and must be `100644` or `100755`; an intermediate component that is not a tree (for example, a `120000` symlink) excludes the source;
  - the bytes come from `cat-file blob`;
  - the exclusions are the same as in R7.

  If a source uses a `.gitattributes` filter, the worktree and blob bytes differ, and tb emits `filtered_source`. In that case, the interactive mode's `content` does not match the gate's. This is a declared limitation.
- R8: `internal/egm/redact.go` uses the `mem.RedactText` patterns, obtained through a new getter `mem.SecretPatterns()` that returns a copy without changing the behavior of `mem`, plus the patterns below, and replaces each match with `[REDACTED]` followed by the same number of line breaks as the original excerpt, preserving the numbering. `RedactText` itself does not change.
  - `\bsk-ant-[A-Za-z0-9_-]{20,}`
  - `\bsk-[A-Za-z0-9]{32,}`
  - `(?i)\b[A-Za-z0-9_]*_(KEY|SECRET|PASSWORD)=[^\s"'&]+`
- R9 (prompt): `prompt.txt` contains, in this order:
  1. persona and standard;
  2. `REVIEWER: <name>`;
  3. `RANGE: <a>-<b>`;
  4. `NONCE: <nonce>`;
  5. the output contract;
  6. `PRIOR:` followed by a `<<<DATA-<nonce> path=PRIOR>>> … <<<END-<nonce>>>>` block;
  7. the `<<<DATA-<nonce> path=…>>> … <<<END-<nonce>>>>` blocks, with the original line numbering.

  `PRIOR` lists the lineages the reviewer owns that are in `open` or `reopened`, in unconfirmed `fixed` or in `fixed` confirmed in the current cycle. Of each one, it carries only these fields: `lineage`, `severity`, `status`, `scope`, `message`, `rationale` and `evidence`. No text written by the elephant (reasons) goes into the prompt. Every reviewer receives the whole document in the data blocks; the `RANGE` only indicates the focus. A finding whose `scope` falls outside the range is accepted and counts normally, but gets `out_of_range: true` in the report. The prompt declares that **all** content of the blocks is data, never instruction.

**Personas, standards and engines**

- R10: reviewers of the `consolidated` preset, run in parallel up to `egm.max_parallel`.

  | Reviewer | Persona | Standard | Range | Code sources |
  |----------|---------|----------|-------|--------------|
  | zoom-out | zoom-out | zoom-out | whole doc | no |
  | inventive-catalyst | inventive-catalyst | (none) | whole doc | no |
  | comprehension | comprehension | (none) | whole doc | no |
  | design | design | type's structural | FeatureRange | no |
  | structural | structural | type's structural | whole doc | no |
  | style-feature | style | (none) | FeatureRange | no |
  | implementation-readiness | implementation-readiness | implementation-readiness | DetailedRange | yes |
  | codebase-details | codebase-details | implementation-readiness | whole doc | yes |
  | style-detailed | style | (none) | DetailedRange | no |

  The structural standard is `feature-spec` for `spec` and `adr` for `adr`. `--reviewers a,b` restricts the list and makes coverage partial.
- R11: the personas, the `arbiter` and the standards are embedded (`go:embed`) in `internal/egm/personas/` and `internal/egm/standards/`, with original text. Each persona declares `max_severity`: `major` for `inventive-catalyst` and `style`, and `blocker` for the others. A finding above the cap makes the output invalid.
- R12: `--engine auto|llm|deterministic`, default `auto`.
  - `auto` uses `llm` if `egm.agent_cmd` is set in the user config. Otherwise, it uses `deterministic` and prints `agent_cmd ausente na config de usuário` ("agent_cmd missing from user config").
  - If `llm` cannot run (outside Linux, without Landlock or without a subreaper), `auto` uses `deterministic` and records the reason in `engine_reason`.
  - There is no inheritance from `router.agent_cmd`.
  - `--engine llm` without an agent is an error (exit 2).
- R13: the checkers below and the `injection-scan` (R17a) run **in both engines** and do not count as reviewers for coverage.

  | Checker | Partially covers (`deterministic` engine) | Severity | Checks |
  |---------|-------------------------------------------|----------|--------|
  | `refs-exist` | codebase-details | `major` | Each R7 candidate (b) that contains `/` goes through steps 1 to 5 of `resolveSource`. Those that leave the root or cross a symlink are ignored, without saying whether they exist. The rest must exist **in the index** (interactive mode) or **in the `<rev>` tree** (gate mode), as a file or as a directory that contains versioned files. Ignores: paths under `.toolbelt/` or `.git/`; the plan's `Create` paths, after expanding `{a,b}`, and the directories that contain them (on a line with a glob, the prefix before the first `*`); whatever matches `egm.refs_ignore` (default `refs/**`); lines with `<!-- tb:egm-example -->`. Each exemption (marker or `Create`) is listed in `report.json` under `exemptions`. |
  | `required-sections` | structural | `major` | Headings required by the structural standard; either of the two languages is enough |
  | `plan-files-listed` | implementation-readiness | `major` | The parsing described below; only with DetailedRange |

  Headings required by `required-sections`:

  | Standard | Required headings |
  |----------|-------------------|
  | `feature-spec` | `## Visão geral` ou `## Overview`; `## Problema` ou `## Problem`; `## Objetivos` ou `## Goals`; `## Não-objetivos` ou `## Non-Goals`; `## Plano técnico` ou `## Technical Plan` |
  | `adr` | `## Status`; `## Contexto` ou `## Context`; `## Decisão` ou `## Decision`; `## Consequências` ou `## Consequences`; `## Alternativas Consideradas` ou `## Alternatives Considered` |

  `plan-files-listed` parsing:
  1. Uses the first table of the DetailedRange that has an `Arquivo`/`File` column and an `Ação`/`Action` column.
  2. Reads only the `Arquivo`/`File` column, and only the single-backtick spans.
  3. The text `(+ docs/pt-BR/)` outside backticks, in a cell, is a directive: it generates the `docs/pt-BR/…` mirror of each `docs/…` path in that cell.
  4. Every path in the column, before and after the expansions, goes through steps 1 to 5 of `resolveSource`. An absolute path or one outside the root becomes a `major` finding, without querying the filesystem. The per-component `Lstat` stops at the first nonexistent component, which is accepted. `{a,b}` expands to at most 64 paths per cell, and `**` is not accepted. With `Modify`, a `*` must match some versioned file under the prefix directory before the `*`, with `ls-files` in interactive mode and `ls-tree -r` in gate mode (R4c), with at most 1000 entries. With `Create`, the path cannot already exist in the index (or in the tree, in gate mode); if it does, it becomes a `nit` finding.
  5. The action must be `Create`, `Modify` or `Delete`, and every `Modify` or `Delete` must exist.

  Checker lineages can be contested at the arbiter. In the `deterministic` engine, the LLM personas are `unavailable`, there is no arbiter and `--contest` is refused. A major `injection-scan` lineage in that engine leaves when the re-run does not reproduce it (R20), through `--fix` or through `--force-settle`, which is a lineage downgrade. This is consistent with the best offline verdict being `PASS_PARTIAL`.

**Output contract and validation**

- R14: the slug is the lowercase relative path, with `/` → `--` and `[^a-z0-9-]` → `-`, plus `-` and the first 8 hex of the path's sha256. A renamed document is a new document.
- R15: stdout, without the profile's envelope, is validated by a hand-written validator in `internal/egm/validate.go`, with no new dependency, that follows `internal/egm/schema/finding.schema.json`.
  - Envelope: `claude-json` uses the `result` field and is the default for `claude`; `raw` is the default for `custom`.
  - Format: `{"reviewer","nonce","checked":[{"heading","quote"}],"findings":[{"severity","scope":{"heading","lines":[a,b]},"message","rationale","evidence":[{"path","line"}]}],"prior":[{"lineage","still_holds":bool}]}`.
- R15a (tool use): the profile defines how to detect tool use in the envelope.
  - In `claude-json`, these count as use: `num_turns > 1`, `tool_use` entries or a non-empty `permission_denials`.
  - If use is detected, the goldfish becomes `not_run` with `reason: tools_used` and the report carries the `tools_used` warning.
  - In Phase A, the `custom` profile always has `tools_check: none`, and therefore the best verdict with it is `PASS_PARTIAL`.
- R16 (failures): `not_run` is never equivalent to zero findings.
  - With retry (one more attempt and then `not_run` with `reason`): missing output, invalid schema or cap exceeded, wrong nonce, invalid attestation, incomplete `prior`, `StdoutOverflow` and timeout.
  - Without retry: escaped process (R3), tool use (R15a) and budget (R27).
- R16a: a finding gets `unverified: true` when `evidence.path:line` is not in the packet's sources (the document counts) or when `scope.heading` does not exist in the snapshot.
  - An `unverified` finding counts for R23 like any other and can be contested.
  - Its evidence does not go into `cited`.
  - It appears marked in the report.
- R17 (attestation): let `H` be the number of headings in the range and `Q = egm.attest.min_quotes` (default 3).
  - **Minimum:** each LLM reviewer returns at least `Q` entries in `checked`, covering at least `min(H, max(1, ceil(H/3)))` distinct headings. If there is quotable text, at least one entry must not be heading text.
  - **Validity of each `quote`:** it must appear **literally** in the snapshot, under the declared heading and inside the range, after normalizing whitespace and removing `|`. It must be 20 to 200 characters long, except when it is a heading's text.
  - **Special cases:**
    - range with fewer than `Q × 20` characters of text: the minimum drops to 1;
    - range with text and no heading: zero headings required and empty `heading`;
    - range with no heading and no text: the reviewer is `skipped` (`reason: range_too_small`).
- R17a (anti-injection scan): before each round, tb sweeps the **whole** snapshot (prose, code and quotes), each reviewer's `PRIOR` block and the sources sent.
  - **Patterns:** they live in `internal/egm/injection_patterns.txt` (RE2, one per line, case-insensitive). The initial content is **exactly**:
    - `ignore (all |the )?(previous|prior|above) instructions`
    - `do not report (any|this|these|the) (findings?|issues?|problems?)`
    - `return (no|empty|zero) findings`
    - `you are (approved|pre-approved)`
    - `como revisor, (não|aprove)`
    - `output only (json|\{)`
    - `responda apenas (com|o) json`
  - **Finding:** the matches under the same heading, or from the same source file, form **one** `major` finding of `injection-scan`.
    - `scope` is the union of the lines.
    - It also carries `context` and `match_sha` (the sha256 of the matched text). `context` is `code` (fenced block or inline code), `quote` (line starting with `>`), `source` (source), `prior` (`PRIOR` block) or `prose` (the rest).
    - A match in `PRIOR` produces no lineage. The matched excerpt is replaced with `[PADRÃO-REMOVIDO]` in that round's `PRIOR`, and tb produces the `prior_injection` warning.
  - **Contest** (R29):
    - the `scope` lines go to the arbiter quoted and escaped, in a `TRECHO-SINALIZADO` block;
    - the contest is refused if there is a term directed at the arbiter (`arbiter`, `árbitro`, `withdraw`, `uphold`, `verdict`, `veredito`) within 80 characters of any match;
    - the scan runs over the evidence excerpt, exempting the `scope` lines of `injection-scan` lineages in `open`, `reopened`, `contest_pending` or `settled` with an equal `match_sha`; any other match refuses the contest.
  - **Reopening:** an `injection-scan` lineage in `settled` **reopens** if the `match_sha` changes.
  - **Goldens:**
    - the `internal/egm/testdata/injection/*.md` fixtures cover each pattern in prose, code, quote and source, documents that discuss patterns without a directive, and an arbiter term near and far from the match;
    - as a regression, this spec produces no `injection-scan` lineage;
    - ADR-015 produces a single lineage, whose only match is the second example cited in its §3. The contest with the document as evidence is accepted, and the arbiter's withdrawal takes the lineage to `settled`.

**Matching, lineage and state**

- R18 (comparison functions): `normalize(msg)` lowercases, strips accents and punctuation, collapses whitespace, removes the stopwords in `internal/egm/stopwords.txt` and cuts each token to its first `egm.match.stem` runes (default 4; 0 keeps whole words).
  - `sim(a,b)` is the Jaccard of the `normalize` tokens.
  - `region(a,b)` holds when both have the same `heading` and, if both have `lines`, the gap between the ranges, `max(0, max(a₁,b₁) − min(a₂,b₂))`, is at most `egm.match.gap` (default 3). The scope of an existing lineage is mapped to the current snapshot first (R20).
  - Defaults calibrated in R0 over `internal/egm/testdata/paraphrases.json` (dogfood #31): `sim` 0.23 with `stem` 4 gives about 3% wrong merges and 24% missed matches on the corpus (20% from the wording alone; 4 of the 87 paraphrased pairs sit under different headings, which `region` never merges, dogfood #34), and, tuned on one document and tested on the other, 22–24% missed and 4–7% wrong merges, both above the targets. A line-overlap threshold had no effect and was dropped. Missed matches show up as duplicate lineages, which cost rounds but never hide a finding.
  - A link declared by the reviewer (`same_as`) and a list of other reviewers' lineages in the prompt were tried in R0 and dropped: reviewers never used them (dogfood #33).
- R19 (merge): lineages of another owner with a lower severity than the finding are not candidates. A new finding is an occurrence of the first candidate, in `F-<k>` order, with `sim ≥ egm.match.sim` and `region`; otherwise it merges with new findings of the same round that meet the same conditions; otherwise it is new.

  A merge keeps the highest severity and the `reviewers[]` list. The owner is the reviewer of the **highest-severity** occurrence. Ties are broken in this order: reviewer with code sources, then the R10 order, and checkers after reviewers.
- R20 (re-evaluation): between rounds, the one who re-evaluates is the **owning reviewer**. The R9 set goes into its `PRIOR`, and it answers `prior[].still_holds` for each lineage. Omitting a lineage from that set invalidates the output.

  | Status | `still_holds: true` | `still_holds: false` |
  |--------|---------------------|----------------------|
  | open / reopened | stays | → fixed (`by: reviewer`, confirmed) |
  | fixed | → reopened | stays fixed (confirmed) |

  - **A fix needs a change:** `still_holds: false` on an `open` or `reopened` lineage is applied as follows:
    - a change since `opened_round` touches the mapped `scope` lines or the `evidence[].line ± 5` lines of a `cited` source (the R28 test): the lineage becomes `fixed`, confirmed;
    - the `content` changed, but no change touches those lines: the lineage becomes `fixed`, confirmed, and the report carries `fixed_elsewhere`, so a fix that only the owner vouches for is visible;
    - the `content` did not change: the answer is ignored, the lineage stays, and the report carries `fixed_without_edit`.

    In R0 the owners never kept a lineage open (0 of 267 re-evaluations over two measurements, dogfood #32 and #33); `fixed_elsewhere` is how that leniency shows up in the report.
  - **Checker lineages:** re-evaluated by re-running the checker.
    - Not reproduced: becomes `fixed`, confirmed.
    - Reproduced: `fixed` becomes `reopened`.
  - **Fixed owner:** a lineage's owner never changes in Phase A. If the owner does not run in the round (`skipped`, `unavailable` or removed by `--reviewers`), the lineage does not change. A blocking lineage whose owner cannot run makes R26 refuse the round.
  - **Line mapping:** the scopes of existing lineages are mapped to the current snapshot by `git diff --no-index -U0` between the snapshots; the same diffs, for the document and for the `cited` sources, give the changed lines that the fix test uses.
  - **New findings:** they go through R19.
    - Matched a `fixed` lineage: it reopens.
    - Matched an `open` or `reopened` lineage: it records the occurrence; if the new severity is higher, the lineage rises to it.
    - Matched `settled`, `declined`, `contest_pending` or `escalated` and the new severity **is not higher**: it only records the occurrence, except for R17a.
    - Matched one of these, but the new severity is higher: it becomes a new `F-<k>`.
    - Did not match: it becomes a new `F-<k>`.
- R21 (state): the state directory contains:
  - `state.json` (`0600`), written to a temporary file followed by `rename` and an `fsync` of the directory;
  - `round-<n>/`;
  - `lock`, a `flock` that the kernel releases if the process dies. Commands that change the state take `LOCK_EX|LOCK_NB` and, if the lock is busy, exit with exit 2 and `reason: locked`. `status` reads without a lock, which is safe thanks to the atomic write.

  `state.json` stores:
  - **Rounds:** `n`, `cycle`, `content`, `policy_sha`, `doc_sha256`, `head_commit`, `sources` (path → sha256), engine, required and executed reviewers with status, and `argv_sha256`.
  - **Map** from occurrence to lineage.
  - **Lineages:** `severity`, `owner`, `persona`, `status ∈ open|fixed|reopened|contest_pending|settled|escalated|declined`, `confirmed`, `contests`, `origin_round`, `opened_round`, `cited`, `match_sha` (in `injection-scan`) and `downgrade` (R24).
  - **Transitions:** command, round, reason and `by ∈ elephant|reviewer|rerun|arbiter`.
  - **Convergence records:** `content`, `policy_sha`, the counted rounds and the cycle downgrade flags.

  The `repo-id` is the sha256 of the realpath of the common git directory (R4d's `<common>`). Worktrees of the same repository share the `repo-id`; clones and forks have their own.
- R22: a `settled` or `declined` lineage that reappears only records the occurrence, except for R17a and a higher severity (R20).

**Convergence and verdict**

- R23 (clean round and convergence): a round is clean when, **after** matching and re-evaluation, all three conditions hold:
  - no blocker or major lineage is in `open`, `reopened`, `contest_pending` or `escalated`;
  - none is `fixed` without confirmation;
  - no required reviewer ended up `not_run`.

  There is convergence with `egm.clean_rounds` (default 2) consecutive clean rounds **over the same `content`**. A clean round with a `content` different from the previous one resets the count to 1. Convergence writes a record for the `content` and closes the cycle; the next round opens a new cycle.

  Cycle cap: `capped` when round `egm.max_rounds` (default 8; R0 had no clean round in 4 effective rounds per document, dogfood #32) of the cycle ends without converging. From then on, `loop next` is refused with exit 1 (`FAIL`). The way out is `--restart`, which requires a new `content`.
- R24 (downgrades): there are two kinds, and all are recorded.
  - **Lineage downgrades**, which apply while the lineage stays in the status: `forced` (`--force-settle`) and `declined` of a `major`. A `forced` of a **blocker** is marked `forced_blocker`, and `tb stack` gives `ok: false` with it even without `--strict` (R33). They persist across cycles. They only go away through `--fix` from `settled` or `declined` (R28), followed by the owner's confirmation (`still_holds: false`).
  - **Cycle downgrades**, which apply to that cycle's convergence record:
    - `--restart`;
    - some required reviewer that did not end up `completed` in a counted round, for any reason other than structural `skipped`, including having been removed by `--reviewers`. The `policy_sha` always uses the full required set, never the `--reviewers` subset;
    - `max_*` exclusion;
    - required reviewer `unavailable`, or `skipped` for a non-structural reason, or a document of type `spec` without DetailedRange (`no_detailed_section`);
    - profile without `tools_check`.
- R25 (verdict): the verdict is computed over the **target content**, which is the worktree in interactive mode or the commit in gate mode (R33). The rules are evaluated **in this order**, and the first one that matches wins:

  | # | Verdict | Condition |
  |---|---------|-----------|
  | 1 | `NO_STATE` | There is no state. |
  | 2 | `PASS` | There is a convergence record for the target content, with `policy_sha` equal to the current one, no cycle downgrade, no lineage with a lineage downgrade and no blocking lineage at all. |
  | 3 | `PASS_PARTIAL` | Like row 2, but with some downgrade. |
  | 4 | `FAIL` | The current cycle is `capped`. |
  | 5 | `STALE` | There is a record for another content, or for this content with a different `policy_sha`, or for this content with some blocking lineage. |
  | 6 | `IN_PROGRESS` | None of the above. |

  The report also carries `loop_state ∈ running|capped|converged`. A coverage smaller than that of the cycle's first round is not refused: it becomes a cycle downgrade (R24). Structural `skipped` (`no_detailed_section`, `range_too_small`) does not count as a reduction.
- R26: in `loop next`, the arbiters of the `contest_pending` lineages (R29) run first. Then, if the `content` has not changed since the previous round **and** there is some blocking lineage whose status does not change without an edit or `resolve` (`open`, `reopened`, `escalated`, or `contest_pending` with no eligible arbiter), **and** there is no unconfirmed `fixed` lineage, which can always change in a new round, tb does not open a new round and shows `documento e fontes não mudaram desde a rodada N; há findings bloqueantes abertos` ("document and sources have not changed since round N; there are open blocking findings"). It exits 0 if some arbiter ran and 2 if none ran. Otherwise, the round is opened. An eligible arbiter means the `llm` engine with an agent, remaining budget and fewer than 2 consecutive `not_run` on that lineage. If some blocking lineage has an owner that cannot run in the effective engine, because it was removed by `--reviewers`, is `unavailable` or because the engine fell back to `deterministic`, tb does not open the round, exits 2 with `reason: owner_unavailable`, and the attempt does not count toward the cap. With `auto` and LLM-owned lineages in the state, falling back to `deterministic` requires an explicit `--engine deterministic`.
- R27 (budget): tb counts the cycle's agent runs (reviewers, retries and arbiters) and stops at `egm.max_agent_runs` (default 120). Whatever is left becomes `not_run` with `reason: budget`.
  - The worst case of a round is `reviewers × 2 + pending contests × 2`, and `--dry-run` prints that value and the balance.
  - A round whose worst case exceeds the balance is refused with exit 2.

**Resolve and arbiter**

- R28: `tb egm resolve <doc>`. A transition to the current status is a successful no-op. An invalid transition gives exit 2 and changes nothing.

  | Flag | From | To | Restriction |
  |------|------|----|-------------|
  | `--fix F-…` | open, reopened, escalated, contest_pending, settled, declined | fixed (unconfirmed) | Since the **reference round** (the one of the lineage's last status transition: the `opened_round`, or the round in which it went to `settled` or `declined`), there was a diff with some changed line that, ignoring whitespace, touched the mapped `scope` lines or the `evidence[].line ± 5` lines of a source in `cited`. `resolve` compares a new, redacted snapshot of the worktree (the document and the `cited` sources, resolved by `resolveSource`, written to `<state-dir>/resolve-<n>/`) against the reference round's snapshot. The whitespace normalization is done in Go over the hunks. From `contest_pending`, it cancels the contest (`contests` stays 1). From `settled`/`declined`, the lineage downgrade only goes away with the owner's confirmation. A fix elsewhere closes through the owner's re-evaluation, without `--fix`. |
  | `--contest F-x --evidence path:line` | open, reopened | contest_pending | Effective engine with an agent; `contests == 0`; the path is in `cited` and goes through `resolveSource` again; the persona is not comprehension. If the path is the document, the evidence comes from the current round's snapshot. If it is another source, it must be versioned, and the sha256 of the worktree bytes, that of the blob bytes at `HEAD` and the one recorded at the `opened_round` must be equal. Recording the contest sets `contests = 1`. |
  | `--decline F-…` | open, reopened | declined | `contests == 0`. Applies only to `nit` and `major`; a declined `major` gets a lineage downgrade. Never applies to `blocker`. |
  | `--force-settle F-x --reason "…"` | open, reopened, contest_pending, escalated | settled | `forced: true`, lineage downgrade |

- R29 (arbiter): the arbiter receives, in the R9 blocks:
  - the original `message` and `rationale`;
  - the whole `scope`, or the `TRECHO-SINALIZADO`;
  - the evidence (±20 lines), redacted and checked by R17a.

  The output follows `internal/egm/schema/arbiter.schema.json` (`{"nonce","checked":[{"heading","quote"}],"verdict":"withdraw|uphold","rationale"}`) and goes through the R17 attestation (over the `scope` and the evidence), R15a and R16.

  Outcomes:
  - `withdraw` of a `nit`, a checker or `injection-scan`: `settled` (`by: arbiter`).
  - `withdraw` of a `blocker` or `major` with an LLM owner: `fixed` without confirmation. The owner re-evaluates in the next round: `still_holds: false` confirms; `true` reopens (`contests` stays 1).
  - `uphold`: `open` (`contests` is already 1).
  - `not_run`: stays `contest_pending`. After 2 consecutive `not_run` of the arbiter, the lineage goes to `escalated`.

  After `uphold`, the lineage only leaves through `--fix`, the owner's `still_holds: false` or `--force-settle`.
- R30: `escalated` is a status set only by tb (R17a, R29), with no entry flag. A blocker or major lineage in `escalated` only leaves through `--fix` or `--force-settle`.

**Integration**

- R31 (CLI): every command accepts `--json`.

  | Command | Flags |
  |---------|-------|
  | `tb egm review <doc>` | `--engine`, `--reviewers`, `--dry-run`, `--restart` |
  | `tb egm loop next <doc>` | `--engine`, `--reviewers`, `--dry-run` |
  | `tb egm resolve <doc>` | those of R28 |
  | `tb egm status <doc>` | `--at <rev>`, `--strict` |

  - **`review`:** creates round 1 of the first cycle. If state already exists, it suggests `loop next` or `--restart`.
  - **`--restart`:** opens a new cycle, keeping the lineages, with cap and budget reset, and downgrades that cycle. It always requires the `content` to have changed since the last round, so each restart costs an edit. The budget stays bounded per content version.
  - **Config:** the `tb egm` commands do not go through the root's `config.Load()` (`cmd/tb/main.go`). The test is by the command chain (`cmd.CommandPath()` starts with `tb egm`), not by the leaf name, which collides with `tb review` and others. The user config is loaded with `config.LoadFile(config.GlobalConfigPath())`, or with the `-c` file. If the file does not exist, the defaults apply, and the `auto` engine resolves to `deterministic`. The user config never comes from the merged config nor from `Shared.FileConfig`. On top of it, only the repository config is applied, under the R34 rules. `tb stack` keeps going through the root's `config.Load()` (Non-Goals).
  - **MCP:** in Phase A, only `tb_egm_status` (arguments `cwd`, `doc`, `at` and `strict`) and `tb_egm_resolve` (arguments `cwd`, `doc` and the R28 flags) live in `internal/registry/parity.go` and in the `parity_test.go` list. `review` and `loop next` take minutes and stay CLI-only. `cwd` must be absolute, and the R4d discovery starts from `cwd/doc`. `tb mcp` reloads the user config on every call.
- R32 (exit codes):

  | Command | 0 | 1 | 2 | 3 |
  |---------|---|---|---|---|
  | `review`, `loop next` | Round or arbiter step completed, and the verdict is not `FAIL` | `FAIL` | Usage, execution or env error; budget; `loop next` with no change and no arbiter (R26) | — |
  | `resolve` | Success or no-op | — | Invalid transition; contest refused; `locked`; `hostile_git_dir`, `no_repository`, `submodule_unsupported`, `git_timeout` | — |
  | `status` | `PASS`, or `PASS_PARTIAL` without `--strict` and without `egm.require_full_coverage: true` | `PASS_PARTIAL` with `--strict` or `egm.require_full_coverage: true`; `FAIL` | Usage error; `hostile_git_dir`, `no_repository`, `submodule_unsupported`, `git_timeout` | `IN_PROGRESS`, `STALE`, `NO_STATE` |
- R33 (gate in `tb stack`): `tb stack` gains the `egm` op, the `--strict` and `--egm-doc <path>` (repeatable) flags, the `EgmDocs []string` and `Strict` fields in `stack.Options`, the `egm_docs` and `strict` arguments in the `tb_stack` schema (with the `ops` description including `egm`) and the additive `warn` field in `StepResult` and `Result`.
  - **Selection:** the op picks the gated documents (default `docs/specs/**` and `docs/adr/**`) added or modified between `merge-base(base, head)` and `head` (the stack's same three-dot semantics), through R4c's `diff --name-status --no-renames`. A document with state whose sources changed in the range, but that was not touched, is not selected: it only produces the `sources_changed` warning (drift detection belongs to Phase B). Those from `--egm-doc` are added. Deleted documents are left out.
  - **Evaluation:** each document is evaluated with `status --at head`. The document, the sources and the `checks_sha` are recomputed from the document at `head`, through R7a and R13 in gate mode, never from the recorded paths.
  - **Empty selection:** `ok: true, warn: true`, with the `no_gated_docs` warning, or `ok: false` with `--strict`. If, in addition, the worktree has modified or unversioned gated documents, the result is `ok: false` with `uncommitted_gated_docs`. This check uses `ls-files -s` and `ls-files --others` only in the prefix directories of `doc_globs` (the part before the first `*`), with the glob filter applied in Go, and `hash-object --no-filters` on the files found.

  | Verdict | Result in the stack |
  |---------|---------------------|
  | `PASS` | `ok: true` |
  | `PASS_PARTIAL` | `ok: true, warn: true` (`ok: false` with `--strict` or `require_full_coverage`) |
  | others | `ok: false` |

  `Result.warn` is the OR of the steps' `warn`. The `egm` op is not parallel. If it appears before other ops, the stack moves it to the end, and the description of `ops` in the `tb_stack` schema says so. A document with `forced_blocker` gives `ok: false` even without `--strict`. With `warn`, the stack lists the downgrades, and the `ExitCode` stays 0. In Phase A, choosing the range is up to whoever calls the stack. Selection by trust ref is in [egm trust](egm-trust.md).
- R33a (CI): the gate is local. In a CI checkout there is no state, so the result is `NO_STATE` and `ok: false`. CI can run the loop with its own credentials or not include the `egm` op.
- R33b (report): `report.json` follows `internal/egm/schema/report.schema.json`, with the fields:
  - `slug`, `doc`, `doc_type`, `cycle`, `round`, `verdict`, `loop_state`, `content` and `target` (`worktree` or `<rev>`);
  - `coverage`, `engine` and `engine_reason`;
  - `findings` per lineage;
  - `skipped_sources`;
  - `warnings` (`config_untrusted`, `tools_used`, `prior_injection`, `fixed_elsewhere`, `fixed_without_edit`, `unstaged_source`, `filtered_source`, `secret_like`…); the `no_gated_docs` warning belongs to `tb stack`, not to the report;
  - `downgrades` and `budget`.

  The ordering is stable and the body has no timestamps. `report.md` is generated from the JSON.
- R34 (config and compatibility): `review.Finding`, `tb review`, `tb drift`, `tb merge` and the router do not change.

  **Execution and secret keys:** `agent_cmd`, `path`, `env`, `home`, `vendor`, `output`, `tools_disabled`, `tools_check`, `forbidden_flags`. They come **only** from the user config. Any key that loosens a protection goes into this class.
  - One of these keys present in the repository's `.toolbelt/config.yaml` (in the worktree or, in gate mode, in the `head` blob) is ignored, and tb emits `config_untrusted`.
  - With `-c/--config`, these keys are read from the given file, because the user chose the file themselves.

  **Limit keys:** `max_agent_runs`, `max_rounds`, `max_parallel`, `timeout_per_reviewer`, `max_source_bytes`, `max_packet_bytes` and `max_stdout_bytes`. The repository can only **lower** these values. A repository value above the user config value (or the default) is capped to it, and tb emits `config_untrusted`.

  **Policy keys:** the repository can only **harden**.
  - `clean_rounds`, `attest.min_quotes`, `match.sim`, and `match.stem` (with 0, whole words, counting as above any other value) only go up; `match.gap` only goes down.
  - `require_full_coverage` only becomes `true`.
  - `doc_globs` only gains items, and the repository's items apply only to the gate selection (R33), not to the source exclusion (R7).
  - `doc_types`, `refs_ignore` and `detail_headings` come only from the user config.
  - Values that loosen are ignored with `config_untrusted`.
  - In gate mode, the repository config comes from the `head` blob, under this same rule.

  **Other keys:** come from the user config, overlaid by the repository config.

  **Reading the repository config:** `tb egm` reads the `egm:` section of `.toolbelt/config.yaml` with its own reader. From the real root, each path component goes through `Lstat`, the file must be regular and the limit is 64 KiB. Otherwise, the file is ignored with `config_untrusted`. The `config.Load` used by the rest of tb does not change.

### Configuration

```yaml
egm:
  # Execution and secret (R34): only in the user config.
  agent_cmd: []              # [0] absolute; empty → deterministic
  path: /usr/bin:/bin        # goldfish PATH
  vendor: claude             # claude | custom
  output: claude-json        # claude-json | raw (custom default: raw)
  tools_disabled: false      # must be true with vendor: custom
  tools_check: auto          # auto (from profile) | none (downgrades)
  forbidden_flags: []
  env:
    pass: []                 # e.g. [ANTHROPIC_API_KEY]; prefixes forbidden in R4b
  home:
    copy: []                 # only if R0 picks copying: paths relative to the real HOME, copied into the disposable HOME with 0600
    read_only: []            # vendor runtime directories
  # Limits (R34): the repository can only lower max_* and timeout_per_reviewer.
  # Policy (R34): the repository only hardens; doc_types, refs_ignore and detail_headings only from the user.
  doc_globs: ["docs/specs/**", "docs/adr/**"]
  doc_types: {"docs/adr/**": adr}   # rest → spec
  detail_headings: ["## Detailed Implementation Plan", "## Plano de Implementação Detalhado", "## Implementation Plan"]
  refs_ignore: ["refs/**"]
  match: {sim: 0.23, stem: 4, gap: 3}   # 0 < sim ≤ 1; stem ≥ 0; gap ≥ 0 (lines)
  attest: {min_quotes: 3}    # ≥ 1
  require_full_coverage: false
  max_rounds: 8              # ≥ 1, per cycle
  clean_rounds: 2            # 1 ≤ clean_rounds ≤ max_rounds
  max_agent_runs: 120        # ≥ 1, per cycle
  max_source_bytes: 65536
  max_packet_bytes: 409600
  max_parallel: 4            # ≥ 1
  timeout_per_reviewer: 5m   # time.ParseDuration, > 0
  max_stdout_bytes: 1048576
```

## Threat Model (core)

| Threat | Mitigation |
|--------|------------|
| Third-party repository picks the binary or forwards secrets | R34 (execution and secret only from the user config), R4b |
| Downloaded file with hostile `.git/config`/`.gitattributes` (filters, fsmonitor, drivers, submodules, `core.worktree`, `include`, `alternates`, `excludesFile`, `commondir`, `gitdir:` to another repository, partial clone with lazy fetch) | R4c (closed list, no `git status`, no full sweep, timeout), R4d (real root via filesystem starting from the document, explicit `GIT_WORK_TREE`, prior inspection of `.git`, config **allowlist**), `GIT_NO_LAZY_FETCH` and `protocol.allow=never` |
| Downloaded document without `.git`, in a directory with no repository above it (up to `$HOME`) | R4d (`no_repository`; the search stops at the realpath of `$HOME`) |
| Repository config that raises limits (cost, timeout), or that is a FIFO or symlink | R34 (limits only go down; reading with `Lstat`, regular file, 64 KiB) |
| Contest evidence pointing to a secret or symlink | R28 (`resolveSource` again), glossary (`cited` only with verified paths) |
| `tb stack` loaders running git without hardening | **Out of scope** (Non-Goals); use `tb egm status --at` |
| Versioned symlink, including in a parent directory, and forged index | R7 (`Lstat` per component, mode in the index), R4d |
| Code injection into the agent via env | R4b |
| Vendor update turns tools back on | R15a |
| Document, source or `PRIOR` instructs the reviewers | R17a, R17, R9 (`PRIOR` with fixed fields, no elephant text). Residual risk: a subtle injection the scan misses, obeyed by all, with a valid attestation. |
| Using or persuading the arbiter | R17a, R29 |
| Goldfish reads `/proc/<pid>/environ` or the repository | R1 (Landlock's ptrace restriction, confirmed in R0; workdir outside the repository), R5 and R15a (no tools). |
| Global `CLAUDE.md` | R4, R0 |
| Secret in the content | R7, R8 |
| Leaked or rotated credential | R0, R4 |
| Fake output on stderr, truncated JSON | R2 |
| Sloppy shortcut (`--fix` without fixing, contesting until it passes, downgrading severity, `--decline`/`--force-settle` laundered by an edit, blocker absorbed by a declined nit, reverting to a content that already converged with an open blocker) | R28, R29, R20 (higher severity becomes a new lineage), R23, R24, R25 (row 2 requires zero open blocking lineages), R26 |
| The owner closes a lineage without a change, or a new finding is absorbed by another owner's milder lineage | R20 (`fixed_without_edit`, `fixed_elsewhere`), R19 (no merge into another owner's lower-severity lineage). Residual risk: an owner that vouches for a fix elsewhere is believed, and only the warning shows it |
| Convergence over different texts, swapping the content after review | R23, R25, R33 (`status --at head`) |
| Crash in the middle of a write, or two processes at the same time | R21 (atomic write, non-blocking `flock`) |
| Repository config loosening the policy | R34 (the repository only hardens) |
| `refs-exist` querying paths outside the repository | R13 (`resolveSource` before existence) |
| Unbounded cost (repeated `loop next`, `--restart`) | R26, R27, R31 (`--restart` requires a new `content`) |
| Active fraud by the same user | **Out of scope**: see [egm trust](egm-trust.md) |

## Non-Functional Requirements (NFRs)

- **Determinism:** in the `deterministic` engine, the same inputs produce the same `report.json` byte for byte.
- **Cost:** about 9 runs per round with DetailedRange and 6 without, limited by R27. In R0, a round of 9 `claude` reviewers plus the author goldfish cost about US$ 2–2.5 (dogfood #32).

## Alternatives

See [ADR-015](../adr/015-egm-nativo.md#alternatives-considered).

## Open Questions

- R0 defined authentication (copy of `claudeAiOauth` into the disposable HOME with `CLAUDE_CONFIG_DIR`, only while the access token is fresh), the no-tools flags and tool-use detection, the runtime directories (binary directory, `resolv.conf` realpath) and the matching and convergence data ([dogfood #22–#33](../campo/dogfood.md)). R18–R27 were revised with it. Still open from R0: item 8 (`safe.directory` with another uid).
- R4 must check the access token's `expiresAt` before phase 2 and stop with a clear error, since a copy cannot refresh an expired token (dogfood #29); `egm-spike` already does this.
- **Convergence in Phase A** ([dogfood #32 and #33](../campo/dogfood.md)): two measurements, 9 reviewers and an author goldfish, did not converge within US$ 16 and US$ 20 (4–6 rounds). In rounds 2 and later, 41 of 42 new unlinked blocking findings were on lines the author had just changed: new problems come from the edits. The more mature `vault.md` stopped producing new blocking findings after round 3; the draft `egm-trust.md` grew from 73 to 193 lines and kept producing them. Phase A therefore targets documents past the draft stage, keeps `max_rounds: 8`, and reports `capped` instead of promising convergence on a growing draft. `late` lineages and `same_as` links were tried and dropped (no measured effect). If capped loops turn out common in use, the next candidates are: only the owners of the cycle's lineages and the reviewers whose range touches the changed lines run after round 1, or `clean_rounds: 1`.
- Findings from the last review round that were left until after R0:
  - code reviewers see the whole document, but `implementation-readiness` stays only on the DetailedRange;
  - offline coverage: an `injection-scan` lineage without an agent only leaves through a re-run, `--fix` or `--force-settle`;
  - owner transfer, `supersedes` and rounds beyond the cap were left out of v1 and may come back with data.

## Detailed Implementation Plan

| File | Action | Reason |
|------|--------|--------|
| `internal/sandbox/sandbox.go`, `internal/sandbox/landlock_linux.go`, `internal/sandbox/landlock_other.go` | Modify | New fields and file mask (R2) |
| `bench/egm-spike/` | Create | Spike (R0) |
| `docs/campo/dogfood.md` (+ docs/pt-BR/) | Modify | R0 result |
| `internal/egm/vendor.go` | Create | Profiles, authentication, flags, envelope, tool detection, bypass refusal (R4, R5, R15, R15a) |
| `internal/egm/env.go` | Create | Minimal env (R4b) |
| `internal/egm/run.go` | Create | Single Registry, subreaper, orphan attribution, timeout, retry (R3, R16) |
| `internal/egm/git.go` | Create | egm's git: options, env and allowed subcommands (R4c) |
| `internal/egm/repo.go` | Create | Real repository root, `.git` inspection and `repo-id` (R4d, R21) |
| `internal/egm/glob.go` | Create | Globs with `**` for `doc_globs`, `doc_types` and `refs_ignore` |
| `cmd/tb/main.go` | Modify | Skip the root's `config.Load()` for the `tb egm` commands (R31) |
| `internal/egm/packet.go`, `internal/egm/sources.go` | Create | Snapshot, worktree or commit content, `resolveSource`, exclusions, prompt (R1, R7, R9) |
| `internal/graph/index_docs.go` | Modify | Export `CollectMDLinks` (R7) |
| `internal/mem/redact.go` | Modify | `SecretPatterns()` getter, no behavior change (R8) |
| `internal/egm/gitconfig.go` | Create | Classification of the `git config --list` keys (R4d) |
| `internal/egm/redact.go` | Create | egm redaction with line preservation (R8) |
| `internal/egm/personas/*.md`, `internal/egm/standards/*.md` | Create | 8 personas and `arbiter`; standards `feature-spec`, `adr`, `implementation-readiness`, `zoom-out` (R10, R11, R13, R29) |
| `internal/egm/checkers.go`, `internal/egm/checker_refs.go`, `internal/egm/checker_plan.go` | Create | Checkers (R13); in the `egm` package, since a separate checkers subpackage would import `Repo` and `Doc` from `egm` and form a cycle |
| `internal/egm/injection.go`, `internal/egm/injection_patterns.txt` | Create | Scan (R17a) |
| `internal/egm/attest.go` | Create | Attestation (R17) |
| `internal/egm/stopwords.txt` | Create | Stopwords (R18) |
| `internal/egm/finding.go`, `internal/egm/validate.go`, `internal/egm/schema/finding.schema.json`, `internal/egm/schema/arbiter.schema.json` | Create | Types, validator, schemas, `unverified` (R11, R15–R16a) |
| `internal/egm/match.go` | Create | Matching, owner, owner transfer, lineage (R18–R20) |
| `internal/egm/state.go` | Create | Atomic state, `flock`, cycles, convergence per content, downgrades, verdicts, budget (R21–R27) |
| `internal/egm/slug.go` | Create | Slug (R14) |
| `internal/egm/report.go`, `internal/egm/schema/report.schema.json` | Create | Report (R33b) |
| `internal/egm/resolve.go` | Create | Transitions and arbiter (R28–R30) |
| `internal/egm/review.go` | Create | Phased orchestration, types, ranges, arbiter step, `--restart` (R6, R10, R12, R26, R31) |
| `internal/egm/config.go` | Create | Repository config reader, key classes, limits and `config_untrusted` (R34) |
| `internal/egm/testdata/paraphrases.json` | Create | R0 corpus |
| `internal/egm/testdata/injection/*.md` | Create | Anti-injection scan fixtures (R17a) |
| `internal/config/config.go` | Modify | `EgmConfig` with Validate (R34) |
| `cmd/tb/egm.go` | Create | Cobra group (R31, R32) |
| `internal/stack/stack.go`, `cmd/tb/stack.go` | Modify | `egm` op, `EgmDocs`, `Strict`, `--strict`, `--egm-doc`, `warn` (R33) |
| `internal/registry/parity.go`, `internal/registry/parity_test.go` | Modify | `tb_egm_*` tools, `tb_stack` schema (R31, R33) |
| `internal/bench/fake_agent_egm.go`, `cmd/tb/fake_agent_egm.go` | Create | egm modes and the `__fake-agent-egm` subcommand, only with the `egmfake` build tag (see the note below) |
| `docs/cli.md`, `docs/arquitetura.md` (+ docs/pt-BR/) | Modify | Document `tb egm` (in section 8.5 of `docs/arquitetura.md`); replace the fictitious example `tb review pr:42` |
| `CHANGELOG.md` | Modify | Unreleased entry |

Note on the fake-agent:
- **Build:** the egm modes live in `internal/bench/fake_agent_egm.go`, with the `egmfake` build tag. The e2e tests compile a separate binary (`go build -tags egmfake -o <tmp>/fake-agent ./cmd/tb`) and configure it as `egm.agent_cmd` (`[<tmp>/fake-agent, __fake-agent-egm, <ro-dir>/fake.json]`) with the `claude` profile. `<ro-dir>` is a directory listed in `egm.home.read_only`.
- **Input:** `__fake-agent-egm` is a new hidden subcommand, registered only by the tagged file, with `DisableFlagParsing: true`. The first argument is the config file; the rest (the `claude` profile flags) is ignored. The workdir is the cwd, which is the packet. It reads the prompt from stdin and extracts `REVIEWER:`, `NONCE:`, `PRIOR:` and the blocks.
- **Output:** it returns the `claude-json` envelope. In it, `num_turns` and `tool_use` are controlled by the mode.
- **Configuration:** a JSON `{"<reviewer or arbiter>": ["<mode attempt 1>", "<mode attempt 2>"]}`, with the attempt coming from `TB_EGM_ATTEMPT`. Any read or format error is a hard failure. The current `__fake-agent` does not change.
- **Modes** (each one emits the `claude-json` envelope):
  - `findings`: one `major` finding with evidence in the document and a valid attestation;
  - `clean`: zero findings, a valid attestation and `prior` with `still_holds: false` for everything;
  - `invalid`, `no-nonce`, `over-severity`, `omit-prior`, `bad-attest`: the failure in the name;
  - `stderr-json`: fake JSON on stderr and valid output on stdout;
  - `overflow`: stdout above the limit;
  - `sleep`: sleeps past the timeout;
  - `probe-fs`: tries to read the real `$HOME`, the repository, `/proc/<ppid>/environ`, `/dev/pts/*`, `/dev/shm` and another packet, and reports each result in a `nit` finding;
  - `escape`: leaves a `setsid` process without `TB_EGM_OWNER`;
  - `tool-use`: `num_turns: 2`;
  - `withdraw` and `uphold`: arbiter output.

### Verification Plan

- **R0:** script in `bench/egm-spike/`. The result goes in `docs/campo/dogfood.md` (PT and EN), and the corpus and convergence rate in `internal/egm/testdata/paraphrases.json`.
- **Sandbox:**
  - packet as RO workdir and repository inaccessible;
  - `PrivateTmp`;
  - `Home` and `Path` precedence;
  - separate stdout and overflow;
  - file rule without EINVAL;
  - `probe-fs` and `escape`, including in parallel;
  - orphan with no owner;
  - subreaper unavailable.

  A skip is only legitimate without Landlock.
- **Hostile content (CUJ-5, CUJ-6):**
  - `.git/config` with `core.fsmonitor`, `diff.external`, `filter.x.clean`, `include.path`, `core.sshCommand` and `core.worktree`;
  - `.gitattributes` with `* filter=x`;
  - submodule with its own config;
  - `.git` as a `gitdir:` file pointing outside;
  - forged index with a parent directory symlinked outside;
  - versioned symlink to `/dev/zero` and a versioned FIFO;
  - repository with `egm.agent_cmd` and `egm.env.pass`;
  - `.toolbelt/config.yaml` as a FIFO, as a symlink to `/dev/zero` and with limits above the user's;
  - `[include]`, `alternates`, `core.excludesFile` pointing to a FIFO, `.git` as a symlink and `gitdir:` without a back-pointer;
  - hung git (timeout);
  - `extensions.partialClone` with a canary `remote.origin.uploadpack` and a missing object: nothing is executed (`hostile_git_dir`);
  - fatal keys (`include`, `partialclone`, `promisor`, unknown `extensions.*`) refused; neutralized ones accepted with a warning; common keys (sparse-checkout, `index.*`, `pack.*`, `http.<url>.extraheader`, npm merge driver) accepted;
  - real config from husky (`core.hooksPath`), git-lfs (`filter.lfs.*`), VS Code (`branch.<n>.vscode-merge-base`), Scalar (`core.fsmonitor`) and reftable: accepted, with `git_config_neutralized` when applicable;
  - repository owned by another uid (`safe.directory`); magic pathspec (`:(top)`, `*`) in a link;
  - `commondir` in a `.git` directory, `objects` or `refs` as a symlink, `packed-refs` as a FIFO;
  - downloaded document without `.git` under a versioned `$HOME`: exit 2 (`no_repository`);
  - plan with an absolute path, `..`, nested `{…}` and a broad `*`: no query outside the root and no hang;
  - unstaged source: round not clean; after `git add` and commit, `status --at HEAD` gives `PASS`;
  - a gate where only a cited source changed does not select the document and emits `sources_changed`; an empty selection with an uncommitted gated document gives `ok: false`.

  Method for "nothing outside the root is read": sentinels outside the root. A sentinel FIFO would make the test hang, and sentinel files have unique tokens that cannot appear in any packet, prompt or report.

  In every case, `review`, `status --at` and `resolve` execute nothing, read nothing outside the root and do not hang. A static test checks that the `egm` package only calls the R4c subcommands. The `tb merge` and router tests keep passing unchanged.
- **Env and tools:**
  - `NODE_OPTIONS` and the like do not reach the goldfish;
  - a forbidden prefix in `egm.env.pass` is refused;
  - `tool-use` mode gives `not_run` with `tools_used`.
- **Shortcuts (tables):**
  - `--fix` with no change, or whitespace-only;
  - second `--contest`;
  - evidence outside `cited`, changed or with injection;
  - arbiter term near the match;
  - changed `match_sha` reopens the lineage;
  - `--decline` of a blocker refused;
  - declined nit followed by a blocker that matches it: new lineage, no convergence;
  - reverting to a converged content with an open blocker gives `STALE`;
  - offline `--contest` refused; `--fix` from `contest_pending`; two arbiter `not_run` lead to `escalated`;
  - `--force-settle`, then `--fix` with a real fix and the owner's confirmation: back to `PASS`;
  - repeated `loop next` with an `escalated` lineage and no edit is refused;
  - `--restart` resets cap and budget, downgrades the cycle and always requires a new `content`;
  - the arbiter withdraws a blocker, then `loop next` without an edit: the round opens and the owner confirms;
  - `--reviewers zoom-out` from round 1 gives at most `PASS_PARTIAL`;
  - `--fix` right after editing, with no round in between, is accepted;
  - a repository that loosens `clean_rounds`, `match`, `doc_types` or `refs_ignore` is ignored;
  - `refs-exist` with `..` and with a symlink: ignored, without reporting existence;
  - extended deny-list and `secret_like`;
  - busy lock: exit 2; `status` reads during a round;
  - the `.git` of a freshly created repository, without `index`, and a large `index` are accepted;
  - contest evidence with a symlink or `.env` is refused;
  - changed `policy_sha` gives `STALE`;
  - spec without a plan gives `PASS_PARTIAL`;
  - `capped` gives `FAIL`, and `--restart` with a new `content` opens a cycle;
  - `--force-settle` of a blocker gives `ok: false` in the stack even without `--strict`;
  - owner removed by `--reviewers` or engine fallen back to `deterministic` with a blocking lineage gives `owner_unavailable`;
  - a cited ignored file is silently excluded; partial staging gives `unstaged_source`;
  - `MinimalDev`: `/dev/pts` and `/dev/shm` unreadable;
  - `--force-settle`, edit and new convergence: stays `PASS_PARTIAL`;
  - injection in prose, code, quote, source and `PRIOR` (fixtures);
  - injection regression: this spec with no lineage, ADR-015 with one;
  - invalid attestation, headings-only, valid in this spec's DetailedRange, and `range_too_small`;
  - CUJ-7 and CUJ-8;
  - two clean rounds over different contents do not converge;
  - `still_holds: false` with no change since `opened_round` gives `fixed_without_edit`; with a change elsewhere, `fixed` and `fixed_elsewhere`;
  - a blocker that matches another owner's major opens a new lineage;
  - `withdraw` of a blocker reopened by the owner;
  - coverage narrowing;
  - `max_*` exclusion;
  - `skipped` owner transfers the lineage;
  - gate with a changed document without `PASS`, deleted document, empty selection and rename.
- **Match:**
  - R0 corpus: with the defaults, wrong merges and missed matches over `paraphrases.json` stay within the values recorded in R18;
  - shifted lines;
  - renamed heading;
  - owner by severity;
  - `style-feature` × `style-detailed`;
  - closing by rerun.
- **State:**
  - R28 transitions and idempotence;
  - verdict precedence (R25);
  - cycles;
  - `capped` and `--restart`;
  - R26 with and without arbiters;
  - arbiter `withdraw`, `uphold` and `not_run`;
  - budget;
  - process killed while holding the lock (the lock is released);
  - kill between the temporary file and the `rename` (the previous state stays valid);
  - `NO_STATE` in a clean checkout.
- **Document types:**
  - an ADR without a plan reaches `PASS`;
  - a spec without a plan reaches `PASS_PARTIAL`;
  - an ADR with a plan requires the 9 reviewers and keeps the `adr` standard.
- **Golden:** the `report.json` of this spec in the `deterministic` engine, including the parsing of its own plan (R13, with the mirror directive) and `refs-exist` with no false positives.
- **Regression:**
  - `tb review` and `tb drift` JSON;
  - `tb mem` and `tb_mem_*` unchanged (R8);
  - `go test ./...` and `TestDocsTranslationMirrorComplete`.

*Based on the EGM pattern*
