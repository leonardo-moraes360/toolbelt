🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/paprika.md)

# Field study: Paprika (TypeScript/pnpm monorepo from scratch)

Study done on 2026-11-21 with `tb` built from this repository (`make build` → `toolbelt 4ed7d1f`, go1.27.4, linux/amd64). The target was a **new** repository, created from scratch during the study: Paprika, a local design tool with a built-in MCP server. `tb` was the harness from start to finish (bootstrap, invariants, graph, verify, guard, review, drift, stack). Nothing was sent upstream to toolbelt.

## Repository

| Field | Value |
|-------|-------|
| Name | Paprika (local, no remote) |
| Language | TypeScript 5.9, Node 22, React 19, Vite 8, Vitest 4, Playwright 1.63 |
| Layout | pnpm monorepo: `packages/core`, `packages/server`, `apps/web` (49 tracked files) |
| Commits assessed | `d5e5af7..`, 5 commits (base = first commit) |
| Prior AI instrumentation | None: `AGENTS.md` seeded by `tb init --write-agents`, then rewritten |
| Toolchain | Yes, complete: build, typecheck, 25 unit/integration tests and 6 E2E tests in Chromium |

## Task

| Field | Value |
|-------|-------|
| What | Implement Paprika v0.1: zod node model, atomic transactions with undo/redo, MCP (stdio and HTTP, 16 tools), live React canvas over WebSocket, WCAG contrast lint, screenshot and React export |
| Complete? | Yes. `tb stack verify guard review drift --base d5e5af7 --ticket rd.md` → `ok: true`, verify PASS on the 1st iteration (15.4 s) |

## tb usage by command

| Command | Result | Helped / hindered / neutral | Evidence |
|---------|--------|-----------------------------|----------|
| `tb init --write-agents` | OK in 0.02 s; detected `pnpm test` and `pnpm run build` from the lockfile | helped | No Go defaults (the python-sdk study problem did not repeat). `config.yaml` is now 0644 (dogfood #7 fixed). |
| `tb init` (generated config) | ~40-line `egm` block and empty `.toolbelt/bench` and `.toolbelt/vault` directories in a repo with no specs or bench | neutral | Reading noise for someone opening the config for the first time. `graph.languages` does not list `typescript` (it sits under `javascript`), which is confusing. |
| `tb init` (seeded AGENTS.md) | Generic text ("may use toolbelt") that goes stale when `verify.*` changes | neutral | I changed `test_cmd` and `lint_cmd`, and AGENTS.md kept quoting the old values. |
| `tb mem init` | 3 default invariants | hindered (slightly) | Seeds "NEVER change thresholds in `.toolbelt/thresholds.yaml`", but the file does not exist and `init` does not create it. |
| `tb mem add` (×5) + `mem check` | OK; sha256 and retention 1.00 | helped | Clear output. There is no category for architecture rules (only `anti_cheat`, `mandate` and `security`); I used `mandate`. |
| `tb verify run --json` (1-commit repo) | Error: `base: ref missing or invalid "HEAD~1"` | hindered | The hint suggests `git fetch origin HEAD~1^{commit}`, but the repo has no remote. In a new repo, the first `verify` always fails. |
| `tb verify run --json` (with Playwright) | ESCALATE after 3 iterations (13 s); the same test passes in the shell | hindered (high) | The env allowlist removed `PLAYWRIGHT_BROWSERS_PATH` and Chromium was not found. Nothing in the output says which variables were filtered. Fixed with `env.allow: [PLAYWRIGHT_BROWSERS_PATH, PAPRIKA_CHROMIUM]`. |
| `tb verify run --json` (failure output) | `reason` with ANSI codes (`\u001b[31m…`) and truncated from the start | hindered | The `FAIL` lines only show in the DEBUG log (dogfood #10 still open). The deterministic failure ran 3 times (dogfood #3 still open). |
| `tb verify run` (after the env fix) | PASS, iter=1; build + lint (`pnpm typecheck`) + tests + E2E in 15.6 s | helped | `test_cmd: pnpm test && pnpm test:e2e` works (the command runs through the shell). |
| `TB_GRAPH=on tb graph index` | 236 nodes and 506 edges in 0.07 s | helped | Fast. Indexed `.tsx` and React components. |
| `tb graph search applyOps` | 1st result correct | helped | The next results are other functions from the same file (acceptable noise). |
| `tb graph callers --function nodeStyle` | Only `render.ts`; **does not find** `apps/web/src/Canvas.tsx` | hindered | The workspace import `@paprika/core` becomes the external node `pkg:@paprika/core`. In a pnpm/TS monorepo, `callers` and `impact` do not cross packages. That was exactly the invariant I wanted to check ("nodeStyle is the only rendering source"). |
| `tb graph deps --file apps/web/src/Canvas.tsx` | Lists `@paprika/core` and `react` | neutral | The relative import `./Canvas` (from `App.tsx`) shows up as `pkg:./Canvas`. |
| `tb guard` (via stack) | 4 rules, 0 violations, 57 ms | neutral | The 5 `mandate`/`security` invariants I added are text only: guard and review do not check them. |
| `tb review --ticket rd.md` (via stack) | 5 warnings, 22 ms | neutral | 4 `quality-large-func` on React components (App 142, Inspector 138, Canvas 262 and the hook 80 lines). The Canvas one is legitimate; the others are normal component shape. 1 `accept-3`: the criterion "build, typecheck, tests and E2E pass" comes out "without evidence", even though `verify` in the **same stack** gave PASS. Findings carry no line. |
| `tb drift --ticket rd.md` (via stack) | OK; no out-of-scope items; base matches | helped | YAML frontmatter with `packages/**` globs worked. |
| `tb stack verify guard review drift` | `ok: true` in 15.4 s; one aggregated JSON | helped | Good "final gate" format for the agent. |
| `tb mcp` | Not exercised | — | I used the CLI. The `tb_*` MCP tools were not tested in this study. |

## New problems

### P1: the env filter silently breaks Playwright
`verify` removes `PLAYWRIGHT_BROWSERS_PATH`, so the E2E suite fails only inside `tb`. Proposal: (a) `tb init` detects `playwright`/`@playwright/test` in `package.json` and fills `env.allow` with `PLAYWRIGHT_*`; (b) on FAIL, the JSON includes `filtered_env: [...]` with the removed names (no values).

### P1: `verify` does not work in a single-commit repo
The default `--base HEAD~1` fails and the hint says to run `git fetch origin`. Proposal: without `HEAD~1`, use the empty tree (`4b825dc…`) as the base, or analyze the working tree; change the hint when there is no remote.

### P2: graph blind across workspace packages
Imports of packages declared in `pnpm-workspace.yaml`, npm/yarn `workspaces` or tsconfig `paths` should resolve to the local package files (through `package.json` `exports`/`main`), not become an external `pkg:`. Relative imports (`./X`) should not become `pkg:` nodes.

### P2: `verify` failure output does not serve agents well
Strip ANSI (or run the child with `NO_COLOR=1 FORCE_COLOR=0`), put lines with `FAIL`/`Error`/`✗` first in the `reason` excerpt, and do not rerun a deterministic suite when nothing changed between iterations.

### P3: review does not use the verify result of the same stack
Acceptance criteria like "tests/build pass" should count the `verdict: PASS` of `verify` in the same `tb stack` run.

### P3: `quality-large-func` on React components
For `.tsx`/`.jsx`, measure JSX size apart from logic, or use a separate limit for components. Include the line in the finding.

### P3: seeded invariant points at a file that does not exist
`mem init` mentions `.toolbelt/thresholds.yaml`, which nothing creates. Suggestion: seed it only when the file exists, or create the file along with it.

## What tb caught and what it missed

- **Caught:** the Playwright environment problem (indirectly, through the mismatch between the shell and `tb verify`) and the scope of the work (drift came out clean, with evidence).
- **Missed:** two real UI bugs that only E2E caught (`blur` rewrote the field after Enter and duplicated the history entry; the selection of a newly created node was cleared before the server confirmed it). And CSS injection through `fontFamily`, found in manual review. None of these is `tb`'s job, but `review` flagged nothing security-related about inline `style`.

## Metrics

| Metric | Value |
|--------|-------|
| `tb init` | 0.02 s |
| `tb graph index` | 0.07 s (236 nodes / 506 edges) |
| `tb verify run` (PASS) | 15.6 s, 1 iteration |
| `tb verify run` (env failure) | 13.5 s wasted on 3 identical iterations |
| `tb stack verify guard review drift` | 15.4 s total; guard 57 ms, review 22 ms, drift 12 ms |
| Invariants | 8 (3 seeded + 5 project), retention 1.00 |

## Status

The seven problems above were fixed in [#152](https://github.com/leonardo-moraes360/toolbelt/pull/152) (merged 2026-11-21). Two differences from the proposals: `tb init` does not fill `env.allow`; instead, `PLAYWRIGHT_BROWSERS_PATH`, `PUPPETEER_CACHE_DIR` and similar join the default allowlist by exact name. And the `--base` fallback applies only to a repository with a single commit that is not a shallow clone; there the base becomes `HEAD` itself (empty diff), not the empty tree.

## Addendum 2026-11-21: Go server with SQLite (`server-go/`)

Second task in the same repo, now with Go alongside TypeScript. `tb` v0.0.9 installed with `go install …/cmd/tb@latest` (the binary reports commit `unknown`). `test_cmd` now also runs `go test`, the parity suite and E2E against the Go binary.

| Command | Result |
|---------|--------|
| `tb verify run` | PASS in 1 iteration after one fix (below); build + lint + TS tests, Go, parity and E2E |
| `tb stack verify guard review drift --base origin/main --ticket rd.md` | ok; guard 4 rules with no violation, drift clean (0 out of scope, 0 forbidden), review with 9 `quality-large-func` |
| `tb mem add` | Worked, but only after creating an ADR (see P2) |

### P1: `go build` fails inside `verify` because of VCS stamping
In `verify`'s filtered environment, `git` cannot report status and `go build` fails with `error obtaining VCS status: exit status 128 … Use -buildvcs=false`. In a normal shell the same command passes. Worked around with `go build -buildvcs=false`. Proposal: `tb init` suggests `-buildvcs=false` when it detects `go.mod`, or the filter keeps the variables git needs (`HOME`, non-secret `GIT_*`); and `reason` shows the end of the output (the error line was cut off and only appeared with `-v`).

### P2: `tb mem add --ref` only accepts `docs/adr` or `docs/decisions`
I tried to reference `server-go/README.md` and it was rejected. Good for forcing a recorded decision, but the message could suggest creating the ADR.

### P2: `tb` is not on PATH in cloud sessions
I needed `go install` and `PATH=$PATH:~/go/bin`. `go install` produces a version without a commit (`unknown`), which makes it hard to report a bug with the exact version. Proposal: `-ldflags` via `debug.ReadBuildInfo` (`go install` fills in the module version).

### P3: `review`/`drift` require commits
With everything in the working tree, you cannot review before committing. Proposal: `--worktree` comparing the current tree with the base.

### P3: `quality-large-func` on declarative tables
The 9 warnings are schema definitions and route/tool registrations (long lists, no branching). Proposal: measure complexity (branches) in addition to lines, or ignore bodies that are a single composite literal.

### Metrics
| Metric | Value |
|--------|-------|
| `tb verify run` inside `tb stack` (PASS) | 32.8 s (TS + Go build, typecheck, vet, TS tests, TS E2E, go test, parity and E2E against Go) |
| `tb stack …` | 33 s total; guard 136 ms, review 173 ms, drift 32 ms |

## Addendum 2026-11-21: "Rust/WASM canvas" thread (cloud container)

| Situation | Result | Helped / hindered | Proposal |
|-----------|--------|-------------------|----------|
| `tb` in the cloud container | Not installed; the toolbelt clone is outside the session's repository scope, so `tb mem list` (AGENTS.md rule 1), `verify` and `stack` did not run | hindered | Publish binaries in releases (`go install …@latest` or tarball) and document a fallback in AGENTS.md: read `.toolbelt/invariants.yaml` directly when `tb` is missing. That is what I did; the YAML file was enough to check the invariants |
| Invariants as text | Read `invariants.yaml` by hand and found a real conflict: "nodeStyle is the only rendering source" versus a second renderer (WASM). Recorded in ADR 0001 | helped | Have `tb review` flag when a diff creates a path parallel to one named in a `mandate` invariant (e.g. a new file that draws nodes without importing `nodeStyle`) |
| Polyglot project (Rust in `experiments/`) | `verify.test_cmd` only covers pnpm; `cargo test` was left out of the gate | neutral | `tb init` detects `Cargo.toml` in subfolders and suggests a composite `test_cmd`, or `verify` accepts a list of suites per directory |

## Addendum 2026-11-21: "Paprika CI" thread (cloud container, PR #5)

`tb` built from the public clone (`go build -buildvcs=false ./cmd/tb`, in ~1 min); `tb mem list` and `tb verify run` ran (PASS in 1 iteration, 32.5 s).

| Situation | Result | Helped / hindered | Proposal |
|-----------|--------|-------------------|----------|
| Local gate vs CI gate | `verify` passed, but CI caught 2 failures the local gate could not see: clippy from the newer Rust stable (1.100, lint `chunks_exact_to_as_chunks`) and `govulncheck` | hindered | `verify` accepts suites per directory (Rust and Go lint) and warns when the local toolchain is behind CI's; a `tb ci plan` reading the same area map as CI would avoid two sources of truth |
| `lint_cmd` without a real Go linter | `lint_cmd` only has `go vet`; golangci-lint found 40 items (2 dead code, 1 ineffectual assignment) | neutral | `tb init` suggests `golangci-lint run` when there is a `go.mod` and the binary exists |
| Change-based selection | I wrote `ci-plan.mjs` and `go-affected.sh` by hand; `tb`'s graph (`graph`) already knows Go imports | neutral | `tb graph affected --base <sha>` returning affected packages/test files would serve both CI and `verify` (run only what changed) |
| `verify` output | Shows only the verdict; no per-step timing | neutral | Show build/lint/test duration to decide what to parallelize |

## Addendum 2026-11-21: migration to toolbelt v0.0.10 (user's machine, PR #11)

`tb` at `~/go/bin/tb`, `toolbelt 0.0.10`, commit f376165 (go1.27.4); the binary now reports the commit, which resolves the P2 about the `unknown` version. Bootstrap done in a worktree from `origin/main` (dae178d) with `tb init --json`. Result: `agent_docs.yaml` with new hints (`pnpm check`, `pnpm test:render`); `config.yaml` untouched by `init`, new keys `egm.drift` and `egm.bootstrap` added by hand; `verify.*`, `env.allow` and the 9 invariants unchanged (`tb mem check` retention 1.00). `tb stack verify guard`: ok, verify PASS in 1 iteration (19.5 s), guard 48 ms.

| Command | Expected | Got | Severity |
|---------|----------|-----|----------|
| `tb bootstrap` / `tb help bootstrap` | A bootstrap or migration command for v10 | Does not exist ("Unknown help topic"); bootstrap is `tb init`. "bootstrap" in the docs is only egm phase B2 | medium |
| `tb init` with an existing `config.yaml` | Migrate or add the new keys, or at least warn which are missing | Silent no-op; I found `egm.drift`/`egm.bootstrap` by comparing with a `config.yaml` generated in an empty repo | medium |
| `tb init --dry-run` | See the plan before writing | No dry-run | low |
| `tb init` on `agent_docs.yaml` | "never overwrites" (help) | Rewrote the hints (correct content, but contradicts the help) | low |
| `tb init` in a worktree | Worktree-local ignore, or a warning | Writes to `.git/info/exclude` of the common dir, also affecting the main checkout, without warning | low |
| `tb verify` with a deterministic failure | ESCALATE on the 2nd repeat of the same named failure (CHANGELOG v0.0.10) | 3 identical iterations (~25 s); the friction from the first task's metrics remains | medium |
| `verify` `reason` | Excerpt with the decisive line (CHANGELOG v0.0.10) | Cut `Executable doesn't exist at …chromium_headless_shell-1243`, which only appears in the DEBUG log | medium |
| `tb doctor` | Check the environment (Playwright Chromium, `test_cmd` binaries) before `verify` | Does not exist; you only find out from the `verify` failure | low |

Resolved in v10, relative to the earlier addenda: `PLAYWRIGHT_BROWSERS_PATH`/`PUPPETEER_*` in the default allowlist (env filter P1), `filtered_env` in non-PASS results, `--base` fallback in a 1-commit repo, graph resolves pnpm workspace packages, `tb mem init` no longer seeds the `thresholds.yaml` invariant (Paprika's remains because it was seeded earlier; I did not remove it, it is `immutable`).

Main proposal: `tb init --migrate` (or `tb config migrate`) that merges missing default keys, with `--dry-run` showing the diff, and a warning in `tb init` when the existing `config.yaml` is behind the binary's version (e.g. a `version:` field in the config).

## Addendum 2026-11-21: multi-vendor local bus (user's machine, PR #12)

`tb bus` (toolbelt 0.0.10, commit f376165c) in the Paprika checkout. A smoke test on a throwaway channel passed end to end (join, assign, recv, claim, second claim rejected naming the owner, complete with epoch, log, show); `tb mcp` lists 54 tools, including the `tb_bus_*` ones. Real channel `paprika` created with `--lease 5m`, supervisor `agent:leonardo`, a 24h worker invite for the external agent. No versioned file had to change: `.toolbelt/bus/` was already in `.gitignore` and `.git/info/exclude`.

| Command | Expected | Got | Severity |
|---------|----------|-----|----------|
| `tb bus invite --ttl 24h` | Expiry with a full date | `expires 16:28:07Z` with no date: looks like it expires now; the real date only by reading `bus.db` | medium |
| Time outputs | A single time zone | `channel extend` and the `tb mcp` log in local time (`-03:00`); create, invite, list and show in `Z` | low |
| `channel create --ttl 15m` | Owner invite with the same validity, or a warning | Owner invite with 10m (the `--invite-ttl` default), without saying it is a different value | low |
| Channel lease | Choose/change it later | The 2m default only shows after create; there is no command to change the lease of an existing channel | low |
| `channel list` with an expired channel | Mark it `expired` | Listed the same as active ones; only by comparing the date | low |
| Closing a channel | `channel close`/`delete` | Does not exist; I used `channel extend --ttl 1s`, which is counterintuitive | medium |
| Pending invites | `invite list` / `invite revoke` | Do not exist; a leaked 24h invite has no revocation other than waiting | medium |
| `tb bus discover` with Codex installed | Read `~/.codex/config.toml` | `exists (not parsed)`; does not say whether `tb mcp` is configured there | low |
| Cloud agent threads | Join the same bus | Impossible: `bus.db` is local and Claude cloud threads cannot reach the machine; coordination between cloud and bus is by convention only (path lanes) | medium (design limit, not a bug) |
| Splitting work | Claim protecting files | Claim protects the *thread*, not paths; I had to invent `paths` in the payload and have the supervisor check by hand | medium |

Proposals: print `expires` as full RFC 3339 and always in UTC; `tb bus invite list|revoke`; `tb bus channel close`; `channel list` with a `state` column; `discover` parsing Codex's TOML; an optional `paths` field on `claim` (or `tb bus lock <glob>`) that rejects overlap with open claims, which would make lane splitting verifiable instead of a convention. For cloud agents, an optional transport mode (e.g. mirroring the board into a versioned file or a `bus/board` branch) would let remote threads at least read the claims.

## Addendum 2026-11-21: "first backlog batch" thread, item 1 (cloud container)

`tb guard check 1a5e5c0..HEAD` and `tb verify run --base 1a5e5c0 --head HEAD` (toolbelt v0.0.10) on the item 1 branch (groups, pages, multi-select). Both passed: guard 4 rules/0 violations, verify PASS in 1 iteration (build, lint and visible tests).

| Command | Expected | Got | Severity |
|---------|----------|-----|----------|
| `tb guard check` on a branch with 2+ commits | Base = merge-base with the default branch | Default `HEAD~1`: would check only the last commit; I had to pass the range by hand | medium |
| `tb verify run` with skipped tests | Count and show `skipped` (or fail with `--no-skip`) | PASS without saying that 4 WASM E2E tests were skipped (`wasm32-unknown-unknown` was missing); I only found out by reading the vitest log | medium |
| `verify` summary | How many tests ran per command | Only `Visible: true`; no count | low |
| Path lanes (bus, PR #12) | See another thread's lane before editing | The lane lives only in `docs/agent-bus.md`, with no check; the conflict warning arrived through a message between threads | low (see the bus addendum) |

Proposals: `guard check` with default base = `merge-base origin/HEAD`; `verify` extracts `skipped` from vitest/go test output (as it already extracts failures) and shows it in the verdict.

### Handshake with the first external agent (`agent:iq3`, 2026-11-21 16:42Z)

| Command | Expected | Got | Severity |
|---------|----------|-----|----------|
| `tb bus log paprika --limit 30` before the query | Show messages seq 1–2 (or say why they are hidden) | "no messages in the channel", but the next query got seq=3; afterwards the log showed only #3 | medium |
| Idle MCP agent | Know whether it will wake up | iq3 went `alive=false` 5 min after the last heartbeat and did not read the query; nothing in `channel show` gives the Agent Card's `wake` (manual/notify/harness), so the supervisor cannot tell whether to nudge the human | medium |
| Agent card | See vendor/model/capabilities of whoever joined | `channel show` shows only name, role and alive; there is no `tb bus agent show <name>` | low |
| Agent Card vs wake file | Fix the card's `wake` without changing the token | `--check` warns `wake_mismatch (card says manual, wake file says notify)`; fixing it needs a new invite + re-join (changes the token, restarts the harness). Proposal: `tb bus card update` for informational fields | low |
| Agent reading the tb docs | Consistent `send` examples | iq3's handoff used `send --json '{…}'` as the payload; in v10 `--json` is the output format and the payload is `--payload`. A sign that the flag confuses agents | low |

### Backlog item 3 (paste SVG/HTML, SVG export), 2026-11-21

`tb guard check 63a90b0..HEAD` (4 rules, 0 violations) and `tb verify run --base 63a90b0 --head HEAD` (PASS in 1 iteration) on the stacked item 3 branch.

| Command | Expected | Got | Severity |
|---------|----------|-----|----------|
| `tb guard --base <branch>` / `tb verify --base` | Accept the short form, or suggest the subcommand | "unknown flag: --base" and the generic help; the flag only exists on `guard check`/`verify run`. Proposal: "did you mean `tb verify run --base`?" | low |
| `verify` with stacked branches | Base = the branch below in the stack | I had to pass the item 2 commit SHA by hand again (same friction as item 1) | medium |
| `verify` vs Go server parity/E2E | Also run the suites that depend on an external binary | `test_cmd` covers only the visible TS tests; parity and E2E against Go ran outside tb (`pnpm test:parity:go`, `test:e2e:go`) and the verdict does not say so | low |
| `tb bus send` outside the repo root | Find the bus by channel or warn that there is no `.toolbelt/` in the cwd | From /tmp: `no session for "leonardo" on channel "paprika": run tb bus join first` (the bus is resolved from the cwd; the message suggests a join that would replace the session). Proposal: say which root was used and suggest `--root`/cd | medium |

## Addendum 2026-11-21: `tb` guaranteed in cloud sessions (SessionStart hook, ephemeral container)

Goal: cloud threads skipped `tb verify` because the container starts without `tb`. Solution in Paprika: a versioned `SessionStart` hook (`.claude/hooks/session-start.sh`) that runs `go install github.com/leonardo-moraes360/toolbelt/cmd/tb@v0.0.10` with sumdb enforced and checks the module hash in the binary (`go version -m`, `h1:q1ojb4SU…`). Container with go1.24.7; first install ~28–34 s (includes downloading the go1.27.4 toolchain), later runs ~20 ms. Afterwards: `tb mem list` (9 invariants, same as the repo), `tb stack verify guard` ok: verify PASS in 1 iteration (22.6 s) and guard 4 rules/0 violations.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Install in an ephemeral environment | Release binary with checksum (`SHA256SUMS`) or a pinnable install script | The only path is `go install`: needs Go in the container, downloads the toolchain and compiles (~30 s per new container). The GitHub releases API is not even reachable through the session proxy; `proxy.golang.org`/`sum.golang.org` are | medium |
| `tb version` after `go install …@v0.0.10` | `toolbelt 0.0.10` + commit | `toolbelt dev`, `commit: unknown`, `built: unknown`. The P2 was only fixed for `make build`; anyone installing via the documented path is back to not knowing the version. Proposal: fall back to `debug.ReadBuildInfo().Main.Version` (and `vcs.revision` when present) | medium |
| Check programmatically "is this the pinned tb?" | `tb version --json` with version and module hash | I had to use `go version -m` and `awk` on the `mod` line; without Go on PATH there would be no way to check | low |
| `go 1.27.4` in toolbelt's `go.mod` | Minimum by minor (`go 1.27`) | An exact patch forces a toolchain download on any Go < 1.27.4 (the container had 1.24.7); works with `GOTOOLCHAIN=auto`, but breaks where `GOTOOLCHAIN=local` | low |
| Bootstrap in a fresh clone | Not needing `tb init` | Helped: versioned `config.yaml`, `invariants.yaml` and `agent_docs.yaml` were enough; `tb mem list` and `stack` ran without init or state in `$XDG_STATE_HOME` | helped |
| `env.allow` in the container | Pass through the image's Chromium | Helped: `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium` (written by the hook into `CLAUDE_ENV_FILE`) reached E2E without touching the config | helped |
| `tb stack` output | Short summary for agents | One large JSON (~hundreds of lines); I needed a script to read the verdict and iterations. Proposal: `--summary` with one line per step (`verify PASS 1 it 22.6s · guard 0/4`) | low |
| Harness integration guidance | toolbelt docs on cloud agents | None; every repo reinvents the hook. Proposal: `tb init --claude-hook` (or an example in docs) generating the `SessionStart` with pinned version and hash | medium |

Main proposals: publish binaries with a signed `SHA256SUMS` in releases, fix the version for `go install` builds, `tb version --json`, and an official generator for the session hook.

## Addendum 2026-11-21: backlog items 4 and 5 (path editing, theme modes), cloud container

`tb guard check <item4>..HEAD` (4 rules, 0 violations) and `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium tb verify run --base <item4> --head HEAD` (PASS in 1 iteration: build, lint, visible tests; hidden n/a) on the item 5 branch (PR #26). Before the PR, `main` (landing page + CI) was merged into the 5 branches of the stack; checks ran on each one.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `verify` with Playwright pointing at a Chromium that does not exist | Detect the missing browser before running and say how to point at another one | The image's Playwright asks for `chromium_headless_shell-1255`, which is not installed; without `PAPRIKA_CHROMIUM` the screenshot tests fail with "Executable doesn't exist" mid-suite. Proposal: a configurable pre-check (`requires: [chromium]`) with the fix in the message | medium |
| `guard` × conditional skip | Not count `it.skipIf(!chromium)` as a skipped test | `test_skip` warns on the Go test that calls `t.Skip` when there is no Chromium (a legitimate environment skip). Proposal: allow annotating the skip (`// tb:env-skip`) or tell unconditional from conditional skips | low |
| `verify` FAIL verdict | See which step failed and the error excerpt | The final output shows `Build/Lint/Visible: true/false`, but not the command nor the last lines of the error; I had to rerun the step outside tb. Proposal: print the command and the last ~20 lines of the failing step | medium |
| `verify` on a stack after merging `main` | Base = the branch below, only the item's diff | Worked by passing the branch name (`--base claude/...-4-edicao-path`) instead of a SHA; helped. The item 1/3 friction (SHA by hand) goes away once the lower branch is on the remote | helped |

## Addendum 2026-11-21: backlog item 6 (agent layer), cloud container

`tb guard check origin/<item4>..HEAD` (4 rules, 0 violations) and `tb verify run --base origin/<item4> --head HEAD` (PASS in 1 iteration) on the item 6 branch (PR #28). The change touched 56 files (translating the messages in the TS core and the Go port).

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Large mechanical change (translating ~150 messages in TS and Go) | `guard` pointing out strings that diverge between the implementations | The goldens (`go test ./internal/core`) caught the divergences, not tb; `guard` only looks at diff rules. Proposal: an optional "string parity" rule fed by a map declared in the repo | low |
| `verify` × language-dependent tests | Know that the verdict covered parity and E2E against Go | PASS, but the output does not list the suites that ran; I checked by running `test:parity:go` and `test:e2e:go` separately (same friction as item 3) | medium |
| Stacked PRs merged into the wrong base | Some warning that the stack did not reach `main` | tb does not know about the stack; I found out on GitHub that #17/#20/#22/#26 went into their base branches after #16 was already on `main`. Proposal: `tb stack status` comparing the declared stack with what is on the default branch | medium |
| `tb stack verify guard` after a pull (user's machine) | PASS on the 1st run | 1st failed in 13s (2 screenshot tests: Chromium `chromium-1228` no longer exists; only 1243/1255); 2nd with `PAPRIKA_CHROMIUM=…1255` PASS in 26s, guard 4 rules with no violation. Same `tb doctor` gap already noted | low |

## Addendum 2026-11-21: CodeQL fixes (#29) and rename in place (#31)

`tb guard check` and `tb verify run` (PASS in 1 iteration) on the item 4 branch (CodeQL fixes), on the merge into item 6, and on the new `-7-renomear` branch (PR #31, web + E2E only).

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `tb guard check origin/main..HEAD` with `main` ahead of the branch | Evaluate only the branch's commits (`git log A..B` semantics) | Reported `test_deletion` on files that only exist on `main` (fixtures in `.github/scripts/test/`): the range seems to become `git diff A B` (tree against tree). Worked around with `$(git merge-base origin/main HEAD)..HEAD`. Proposal: use the merge-base by default (or `A...B`) and say in the output which base was used | medium |
| UI-only change covered by E2E | The verdict saying whether E2E ran | Output shows `Visible: true`; I had to run `test:e2e` and `test:e2e:go` separately to be sure (same friction as items 3 and 6) | medium |
| `tb guard check origin/main...HEAD` (three dots, trying merge-base semantics) | Accept `A...B` as git does | Failed with `hint: git fetch origin .HEAD`: the parser splits at the first `..` and `.HEAD` is left over. Proposal: accept `...` (merge-base) or reject it with a clear message | low |
| Red CI on the stack because of an old `ci.yml` on the branch (no `fetch-depth: 2`, already fixed on `main`) | tb warning that the branch is behind `main` in CI files | Only found out by reading the job log (`no merge base`); fixed by merging `main`. Proposal: `tb stack status` listing `.github/` files that diverge from the default branch | low |

## Addendum 2026-11-21: workspaces page (branch `claude/project-thread-qmgfw0`)

`tb stack verify guard review drift --ticket rd.md` in the cloud session (tb v0.0.10 installed by the `SessionStart` hook). 1st round FAIL, 2nd PASS in ~3 min (verify, guard, review with 3 reviewers, drift with nothing out of scope).

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `verify` in the cloud without `PAPRIKA_CHROMIUM` | Browser pre-check before the suite | Again 3 screenshot tests failed mid-suite (Playwright asks for a missing headless shell); PASS after exporting `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium`. Same friction as items 3–6: `tb doctor`/a pre-check is still what would save the most | medium |
| `tb stack ...` output redirected to a file | A single JSON for `jq`/`json.load` | stdout carried the JSON and the `DBG` log lines were interleaved on stderr; with `2>&1` the JSON cannot be parsed. Separating them works, but with `--json` the default output could go to stdout only | low |
| Change that adds MCP tools (regenerated contract) | drift knowing that `server-go/contract/*` is derived | `drift` accepted it (the ticket mentioned MCP); worked well with `rd.md` describing the scope | helped |

## Addendum 2026-11-21: diagrams (branch `claude/project-thread-2x0kue`, PR #44)

`tb stack verify guard` in the cloud session (tb installed in `~/go/bin`, outside the default PATH). 1st round ESCALATE, 2nd PASS in ~3 min.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `verify` without `PAPRIKA_CHROMIUM` exported | Browser pre-check before the suite | Again: 3 screenshot tests failed and the verdict was `ESCALATE` after 3 iterations (~3 min lost). PASS after exporting `PAPRIKA_CHROMIUM`. Still the no. 1 friction in the cloud; an `env.require` in the config that fails in 1 s would fix it | medium |
| Reading the `tb stack` result | A top-level field with the failure reason | The JSON has `steps[].result.reason` with the log tail; useful, but `reason` repeats the same excerpt in `error`. One would be enough | low |
| `tb` installed by `go install` | Being on PATH | `~/go/bin` is not on the session's PATH; I had to `export PATH=$PATH:~/go/bin` in every command. The hook could export it or link into `/usr/local/bin` | low |

## Addendum 2026-11-21: removing the TypeScript server (branch `claude/project-thread-tfkzb6`)

`tb stack verify guard review drift --ticket rd.md` in the cloud session (tb v0.0.10 installed with `go install …@latest` in `~/go/bin`, reports `toolbelt dev`). The change deletes `packages/server`, moves the black-box suites to `packages/e2e` and takes the TS server out of CI (68 files). Final result: `verify` PASS in 1 iteration (with `--base HEAD`, see below), `review` 0 findings, `drift` nothing out of scope; `guard` FAIL with 5 `test_deletion` explained in the PR.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Tests **moved** with `git mv` (`packages/server/test/** → packages/e2e/test/**`, git shows `R`) | `guard` following renames (`git diff -M`) and not reporting a removal | 4 `test_deletion` on files that only changed folder; `verify` aborts ("guard aborted verify (protected path)") before build and tests. Proposal: detect renames and report them as `test_moved` (info) | high |
| **Intentional** removal of tests for deleted code (`server.test.ts` tested the removed server) | An auditable way to accept the removal (e.g. `allow_test_deletion:` in the ticket with a reason, or `--accept test_deletion:<file>`) | No mechanism: `--ticket` only opens an exception for `protected_paths`. I ran `verify` with `--base HEAD --head HEAD` to get build/lint/tests and left the violation explicit in the PR. It works, but it is a workaround | high |
| `review`/`drift` with a dirty worktree and HEAD = a merge commit | Base = merge-base with `origin/main` (= HEAD) | Used `759a056` (the stale **local** `main`) and flagged as out of scope files in `experiments/` I never touched. Fixed with `git branch -f main origin/main`. Proposal: prefer `origin/<default>` and warn when the local `main` is behind | medium |
| `rd.md` in Markdown with a prose "Escopo" section | Scope read from the section | 68/68 files "out of scope": only the YAML frontmatter with `scope:` (globs) works reliably. Proposal: document the format in `tb review --help` or warn "empty/unrecognized scope" instead of flagging everything as an error | medium |
| `verify` without `PAPRIKA_CHROMIUM` | Browser pre-check | Same friction as the previous items: E2E failed because `chromium_headless_shell-1255` was missing; PASS after exporting the variable | medium |
| `tb mem edit <id> --rule …` | Edit the invariant that mentioned the TS server | OK, `mem check` with retention 1.00; short, clear output | helped |

## Addendum 2026-11-21: renaming and deleting workspaces (branch `claude/project-thread-puuktn`, PR #46)

`tb stack verify guard review drift --ticket rd.md` in the cloud session (tb v0.0.10 via `go install`, in `~/go/bin`, reports `toolbelt dev`). Final round PASS: verify 42 s, guard ok, review 0 findings, drift none; `tb guard check <merge-base>..HEAD` ok.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `review`/`drift` run **before the commit** (changes only in the worktree) | Review the worktree diff, or warn "nothing committed since the base" | Reviewed the default range `HEAD~1..HEAD`, which was the merge of PR #45, and flagged `styles.css` as out of scope for a change that was not mine. I only understood it by reading `base_ref`/`head_ref` in the JSON. Proposal: with a dirty worktree, warn and review `HEAD` + worktree, or fail with "commit first" | medium |
| `rd.md` with a `## Escopo (arquivos)` section and paths in backticks | Recognize the file list | 22/22 files "out of scope" (worse than without the section, when only 1 was flagged). It worked with `## Scope` and paths without backticks, a format found only by reading `git_errors_test.go` in the module. Reinforces the previous thread's item: document the format in `--help` and warn "scope not recognized" | medium |
| `verify` with `PAPRIKA_CHROMIUM` exported | PASS | PASS on the first run, including TS and Go E2E. Exporting the variable before running is still the recipe | helped |
| The container's `golangci-lint` | Run the Go lint locally | The binary was built with go1.25 and the module asks for 1.26, so it does not run. Not tb's fault, but `lint_cmd` (`go vet`) does not cover golangci; CI does | low |

## Addendum 2026-11-21: flicker when dragging elements (branch `claude/project-thread-l72mwk`, PR #48)

`tb stack verify guard review drift --ticket rd.md` in the cloud session (tb v0.0.10 via `go install`, in `~/go/bin`, outside PATH; `PAPRIKA_CHROMIUM` exported). verify PASS and guard ok on the first run (~58 s total); review/drift FAIL on the 1st round only because of the `rd.md` format; on the 2nd, review with 3 warnings and drift with nothing out of scope.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `rd.md` with an inline scope: `Escopo: a.tsx e b.test.ts.` | Recognize both paths, or warn that the line was not understood | Only the 1st path entered the scope; the test file came out as "out of scope" and `drift` failed. It worked with `## Escopo` and a `- path` list. Third thread in a row with the same friction: a `tb ticket lint rd.md` (or the parsed scope printed at the start of `review`) would fix it | medium |
| `review` with `- [ ]` criteria in the ticket | Link the criterion to the test that covers it | 2 "acceptance criterion without evidence" warnings for criteria covered by `drag.e2e.test.ts` (test names in pt-BR, criterion in other words). The heuristic seems to match words only; accepting `(test: <name>)` in the criterion would help | low |
| `review` on an E2E test file | Ignore the size of a `describe` | Warning "added function is too large (195 lines +)" for the vitest `describe` callback. Proposal: do not count `describe`/`it` callbacks as functions | low |
| `tb stack` with `2>&1` into a file | Readable JSON | The JSON came after log lines; I had to look for the first `{` to parse it (same item as previous threads) | low |
| `verify` with `PAPRIKA_CHROMIUM` exported | PASS | PASS on the first run, with TS and Go E2E | helped |

## Addendum 2026-11-21: migrating the app's CSS to Tailwind (branch `claude/project-thread-ol0wza`)

`tb stack verify guard` in the cloud session (tb v0.0.10 installed by the `session-start.sh` hook with `CLAUDE_CODE_REMOTE=true`, in `~/go/bin`, outside PATH; `PAPRIKA_CHROMIUM` exported). verify PASS on the first run (~58 s, build + typecheck + `go vet` + unit + TS and Go E2E); `tb guard check <merge-base>..HEAD` ok. The main validation (18 before/after screenshots compared pixel by pixel) was a separate script: tb has no step for that.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `tb stack verify guard` with the change still **uncommitted** | `guard` looking at the worktree, or warning "nothing committed since the base" | `verify` ran on the worktree (correct), but `guard` checked `7c86a70..d817b6c` (the merge of PR #45, not mine) and answered ok with 0 violations. An "ok" on the wrong range gives the impression the change was checked. Same friction as the `review`/`drift` item in the workspaces thread: with a dirty worktree, warn or include the worktree | medium |
| Visual refactor that must stay **identical** | A `verify` step (or a hook) for visual regression: run a screenshot script on the base and on HEAD and compare | Nothing in tb; I wrote `shots.mjs` + `diff.py` in the scratchpad (base build copied by hand). Proposal: `visual_cmd` in `config.yaml`, run on the base and on HEAD in a temporary worktree, with the pixel diff in the `stack` JSON | medium |
| Session hook without `CLAUDE_CODE_REMOTE` in this thread's environment | `tb` available when the session opens | `tb` was not installed; I ran `CLAUDE_CODE_REMOTE=true bash .claude/hooks/session-start.sh` and it installed with the hash check. The hook works, but it did not fire on its own in this session (not tb's fault; noted because it is tb's entry point) | low |
| CSS classes used as selectors by E2E | Something warning that removing `is-active`/`layer-name` breaks tests | Found out through E2E (5 cascading failures, one of them a 5 s timeout). Not `guard`'s job today; a `protected_patterns` (e.g. strings mentioned in tests) would be a cheap warning. Recorded in AGENTS.md: the plain classes are hooks | low |
| `verify` with `PAPRIKA_CHROMIUM` exported | PASS | PASS on the first run, with TS and Go E2E | helped |
| (cover, local branch stacked on #46) `tb stack verify guard review drift` after the commit, with `## Scope` as a plain list | PASS on the first run | PASS (verify 26 s, review 0, drift none). With the right `rd.md` format and the commit made first, the flow is fast; the two frictions of the previous round did not repeat | helped |

## Addendum 2026-11-21: Dockerfile to run Paprika (branch `claude/project-thread-mv4yl5`)

`tb stack verify guard review drift --ticket rd.md` in the cloud session (tb v0.0.10 via `go install`, in `~/go/bin`, outside PATH; `PAPRIKA_CHROMIUM` exported), after the commit and with `git branch -f main origin/main`. Everything PASS on the first run: verify 88 s, guard 4 rules with no violation, review 0 findings, drift none (scope in the `scope:` frontmatter). The main validation (image build, HTTP smoke of the canvas, `/mcp`, screenshot and persistence) was a separate script, `.github/scripts/docker-smoke.sh`.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Change whose product is a **Docker image** | `verify` having a step for artifacts beyond build/lint/test (e.g. `docker build` + smoke) | `verify` ran `pnpm build` + the app tests, which did not change, and gave PASS without ever building the image. The PASS is true but says nothing about what was delivered. Proposal: `extra_cmds` (or `artifact_cmd`) in `config.yaml`, selectable by a glob of touched files (`Dockerfile` → `docker build && smoke`), with the result in the `stack` JSON | medium |
| `verify` on an infra/docs-only change | Skip what was not affected, as CI does with `ci-plan.mjs` | Ran the full suite (88 s) for a change that touches no app code. Proposal: `verify --changed` reading a selector from the repository (`ci-plan.mjs` already exists) | low |
| `guard` with a new `Dockerfile` and a changed workflow | Some warning about a sensitive surface (CI and runtime image) | 0 violations; the 4 rules do not look at CI/Docker. A "CI/infra file changed: check pins and permissions" warning would be cheap. The repo's `check-actions-pinned.mjs` covered the pins | low |
| Scope in the `scope:` frontmatter with exact paths, commit before `review` | PASS with no false "out of scope" | PASS on the first run; with the previous threads' recipe (commit first, local `main` up to date, scope in YAML) the flow had no friction | helped |
| `tb mem list` at the start | Relevant invariants | Brought the one about binding to `127.0.0.1` + Host/Origin validation, which drove the decision: the image listens on `0.0.0.0` only inside the container, the docs publish on the host loopback, and the smoke tests that an odd `Host` still gets 403 | helped |

## Addendum 2026-11-21: Lucide icons + scrollbar (PRs #51 and #53)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session hook | `tb` on PATH when the session opens | It was not; `go install …/cmd/tb@v0.0.10` worked (downloaded the go1.27.4 toolchain on its own) and the binary landed in `~/go/bin`, outside PATH | low |
| `tb stack verify guard review drift` with a **dirty** worktree on a HEAD that is a merge (`7d489e6`, merge of #48) | Worktree diff against `main`, or a clear refusal | `guard`/`review`/`drift` used `HEAD^1..HEAD` (`8241be8..7d489e6`) and flagged `drag.e2e.test.ts` (from PR #48) as out of scope; the only warning was `WRN dirty worktree` on stderr. Third thread with the same friction: include the worktree or fail with "nothing committed since the base" | medium |
| Same command after the commit | PASS | PASS (verify 44 s with E2E 29/29 and API 19/19 against Go) | helped |
| Stacked branch (scrollbar on top of Lucide) | Base = `main` or the declared base | Base = `HEAD^` (the Lucide commit), which happens to be right for a stacked PR; but the JSON does not say where the base came from. Proposal: a `base_source` field (`merge-base main`/`HEAD^`/flag) | low |
| CSS-only change (scrollbar) | Visual validation | `verify` cannot see it; manual screenshot in Chromium without `--hide-scrollbars` (headless hides the bar by default, which would make any `visual_cmd` blind to this kind of change) | low |

## Addendum 2026-11-21: agent page in the presence list (branch `claude/project-thread-4plm47`)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session hook | `tb` on PATH | Outside PATH again (`/root/go/bin`); the hook installed it, but the session does not see it. Fourth thread with this | low |
| `tb stack verify guard` without `PAPRIKA_CHROMIUM` exported | A clear prerequisite warning before running ~30 s of suite | `verify` failed 2 iterations with "Executable doesn't exist … chromium_headless_shell-1255" and only `DBG` said why. Proposal: `verify.require_env` in `config.yaml` (e.g. `PAPRIKA_CHROMIUM`) checked before the build | medium |
| `tb guard check origin/main..HEAD` after the commit | ok | ok (4 rules, 0 violations); committing before `stack` avoided the wrong-range friction | helped |
| Same `stack` with the env exported | PASS | PASS (28 s, E2E 29/29) | helped |

## Addendum 2026-11-21: workspace card: one click opens, no flicker, full corners (branch `claude/project-thread-ioa23i`)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | Not installed; `go install …/cmd/tb@v0.0.10` and `PATH+=/root/go/bin`. Fifth thread with this | low |
| `tb stack verify guard` after the commit, with `PAPRIKA_CHROMIUM` exported | PASS | PASS on the first run (41 s; build, lint, tests; guard 4 rules with no violation). The "commit first + env exported" recipe keeps working without friction | helped |
| Subpixel visual bug (border eaten by the cover at the corners, at 1x) | Some step that catches a visual regression | Nothing in tb; I reproduced it with Playwright at 1x/2x and a zoomed crop with ffmpeg, before and after. Reinforces the `visual_cmd` proposal (base vs HEAD), with a configurable DPR: the defect barely shows at 2x | medium |
| Interaction change (one click opens) that contradicts the behavior of an earlier PR (#46) | Warning that an existing test encodes the old behavior | E2E caught it (the test asserted `aria-pressed`); `guard` has no way of knowing. Fine for that to be the test's job | low |

## Addendum 2026-11-21: TanStack Query in the overview (PR #59)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Sixth thread with this | low |
| `tb stack verify guard --ticket rd.md` with a dirty worktree (everything staged, before the commit) | Worktree diff against `main` | PASS (47 s), but `head_ref` in the JSON is the commit of the updated `main` and `base_ref` the clone's old `main`: the report does not make clear that it evaluated the worktree. After the commit, `tb guard check merge-base..HEAD` ok | low |
| Behavior regression (GET count per burst of writes) | Some way to prove "fails before, passes after" | Done by hand: `git stash` of `apps/web`, rebuild, run the new E2E (20 GETs on `main`), `stash pop`. Proposal: `tb verify --against <ref>` that runs a selected test on the base and on HEAD and requires failure → success | medium |
| `bundle` grew 36 kB (+10.5 kB gzip) with the new dependency | Bundle budget warning | Nothing in tb; measured by hand with two builds. Proposal: an optional `size_cmd` with a limit in `thresholds.yaml` | low |
| Intermittent E2E on CI after merging main into the branch (dialog reopened empty, ~1 in 3) | `verify` helping tell a flake from a bug | One `verify` passes or fails by luck; I found the cause by running the suite 10x in a loop with markers. Proposal: `verify --repeat N` (or `test_repeat` in `config.yaml`) that runs the suite N times and reports the failure rate per test in the JSON | medium |

## Addendum 2026-11-21: WASM (CPU) layer flicker when resizing panels (branch `claude/project-thread-bzqa63`)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Seventh thread with this | low |
| `tb --version` | `v0.0.10` + commit | `toolbelt dev / commit: unknown` when installed by `go install`: `debug.ReadBuildInfo()` (Main.Version) could be read as a fallback | low |
| `tb stack verify guard review drift --ticket rd.md` before the commit | review/drift evaluating the change | `verify` PASS (42 s), but `review`/`drift` looked at `base..HEAD` with HEAD = `main` (empty diff) and only reported "rd.md has no declared scope". Nobody warned that the dirty worktree was left out. Proposal: a `worktree_dirty: true` warning in the JSON when the range does not include the local diff | medium |
| `rd.md` format | The format documented at hand (`tb review --help`) | Only by reading `internal/review/review.go` did I find the YAML frontmatter (`title`, `scope`, `forbidden`, `acceptance`). Proposal: `tb init` generating a sample `rd.md`, or the `scope-missing` finding citing the format | medium |
| After the commit, with frontmatter | guard/review/drift clean | ok, 0 findings (scope = 2 files, forbidden = server-go/core) | helped |
| Proving "fails before, passes after" on the new E2E | `tb verify --against <ref>` | By hand again: `git stash` of the file, `vite build`, test fails (8/8 empty frames per panel), `stash pop`, passes 3x in a row. Reinforces the TanStack thread's proposal | medium |
| Timing bug (frame painted empty) | Some support for per-frame visual regression | Nothing in tb; the test uses its own ResizeObserver to read the canvas before painting. Reinforces `visual_cmd` | low |

## Addendum 2026-11-21: humanized README + setup prompt (branch `claude/project-thread-dxksae`)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@v0.0.10` also downloaded go1.27.4 (the toolchain the module requires), ~1 extra min. Eighth thread with this | low |
| `tb guard check "$(git merge-base origin/main HEAD)"..HEAD` before `git fetch` | Range with only my commit | The clone's `origin/main` was stale: guard reported `test_deletion` on `packages/server/test/*` (removal of the TS server, PR #47, by someone else). After `git fetch origin main`, `guard: ok (4 rules, 0 violations)`. Proposal: guard warning when the range has commits already on the remote (or suggesting `fetch`), and AGENTS telling agents to `fetch` first | medium |
| Docs-only change (README, docs/, list of site pages) | A "docs" mode with proportional checks | None; the full `verify` (build + E2E) would be waste. I ran by hand: relative link check (script), `site:build` (32 pages), screenshots in both languages and `ci-plan` (no job). Proposal: `verify --profile docs` with `docs_cmd` (links + site build) | medium |
| Consistency between languages (README.md × README.pt-BR.md, docs/ × docs/pt-BR/) | Something that flags a section missing in one of the mirrors | Nothing in tb; I compared the headings by hand. Proposal: a `mirror` invariant/guard rule comparing the list of `##` between configured pairs of files | low |

## Addendum 2026-11-21: design-only thread over MCP (user's machine)

Task: draw Paprika's brand and interfaces (9 boards, ~1070 nodes) in a fresh workspace of the local server, only through Paprika's MCP. No code change in the repo except the agent feedback log. `tb` touched the work only at the start.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `SessionStart` hook on the user's machine | Accept the local `tb` or explain what differs | `tb on PATH (/home/leonardo/go/bin/tb, module version v0.0.10) is not the pinned v0.0.10.` The version matches; what fails is the module hash: `go version -m` shows `mod … v0.0.10` with no `h1:` sum (binary built from a local checkout). The message names the version, not the hash, so it reads as a contradiction | medium |
| `tb mem list` at the start (AGENTS.md rule 1) | The invariants, fast | Helped: 9 invariants in milliseconds; the "original tool, never copy Paper" and "every mutation through applyOps" rules framed the work (draw only through MCP, no direct DB edits) | helped |
| A task with no code (design through MCP) | Some way to record "verify not applicable" | `tb stack verify guard` has nothing to check when only docs change and the deliverable lives in SQLite; there is no `tb` notion of an external deliverable (screenshots, an MCP document) to attach to the ticket | low |

Proposals: the Paprika hook (or a `tb version --json` with `sum`) should say "hash missing/different" instead of repeating the version; `tb ticket` could accept artifacts (paths to PNGs, a workspace id) as evidence for non-code work.

## Addendum 2026-11-22: Dependabot PR #55, Node 26 in the Dockerfile without corepack

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | Installed by `.claude/hooks/session-start.sh` with `CLAUDE_CODE_REMOTE=true`, but in `/root/go/bin`, outside PATH. Ninth thread with this | low |
| `tb stack verify guard` without `PAPRIKA_CHROMIUM` | Prerequisite warning before the suite | Failed 2 iterations with "chromium_headless_shell-1255 doesn't exist" (~87 s lost). Reinforces `verify.require_env` | medium |
| Same command with the env exported | PASS | PASS (build, lint, tests; E2E 30 ok + 7 skip, API 19/19 against Go). Guard: 1 `t.Skip()` warning in `server-go/internal/team/helpers_test.go`, which came from main (merge), not from the PR | helped |
| HEAD = merge of main into a third-party branch (Dependabot) | Base = merge-base with `main` | `base_ref` = Dependabot's old head, so guard evaluated the whole main merge and flagged someone else's `t.Skip()`. Proposal: with a merge HEAD, use `merge-base origin/main HEAD` as the base, or say in the JSON that the base includes commits from another branch | medium |
| Dockerfile change (build stage) | Some step that exercises the Dockerfile | `verify` does not touch Docker; a local `docker build` fails behind the container's proxy (rustup/go mod without the CA). I validated the changed stage in a `docker run node:26-bookworm-slim` with the CA mounted. Proposal: an optional `docker_cmd` with a configurable `--build-arg`/CA secret | low |

## Addendum 2026-11-22: WCAG 2.1 AA accessibility round (branch `claude/project-thread-gui1x7`, PR #70)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@v0.0.10` (downloaded go1.27.4 again) + `PATH+=~/go/bin`. Tenth thread with this | low |
| `tb mem list` | Project invariants | ok, 10 invariants; the design-first `mandate` was there and drove the decision not to touch colors in this PR | helped |
| `tb stack verify guard review drift --ticket rd.md` with a prose `rd.md` ("Escopo: … apps/web … apps/site build") | Scope turned into globs | `review`/`drift` only looked at `HEAD~1` (the site commit) and flagged the 3 site files as out of scope. With a `## Escopo` list of globs and `--base $(git merge-base origin/main HEAD)`, both passed with no findings. Proposal: `stack` using the merge-base as the default base when HEAD is on a branch, and the `scope` finding showing the scope it understood | medium |
| `verify` with `PAPRIKA_CHROMIUM` exported | PASS | PASS (build, typecheck + go vet, core/Go tests, API and E2E including the new a11y test) | helped |
| Accessibility change | An a11y step in `verify` | None; I ran axe-core separately on 16 scenarios (script in /mnt/project-files/paprika/a11y/scripts/audit.mjs). Proposal: an optional `a11y_cmd` in the config, with the result (violations per rule) in the `verify` JSON | medium |
| Proving "fails before, passes after" | `tb verify --against <ref>` | By hand again: `git stash` of apps/web/src, build, 5/5 fail, `stash pop`, 5/5 pass. Reinforces the proposal already recorded | medium |
| Quality warning "source code changed without test changes" | Consider evidence outside tests | Fired on the site commit (no suite); the evidence was axe. Proposal: accept `evidence:` in rd.md to silence it with a justification | low |

## Addendum 2026-11-22: backlog batch 2, 6 items on stacked branches (branches `claude/project-thread-lqkofu[-N-*]`)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Eleventh thread with this | low |
| `tb guard check "$(git merge-base origin/main HEAD)"..HEAD` at the top of a 6-branch stack | Findings per branch/PR | A single report for the whole range (2 `test_skip` warnings in screenshot_test.go, expected). Proposal: `tb guard check --stack <branch…>` evaluating each step against the previous one and saying which PR each finding belongs to | medium |
| Main merge (PR #70 from another thread) going down the stack | Help propagating the merge | By hand: merge on each step, one conflict (AgentCursors.tsx). Proposal: `tb stack sync` that merges in cascade and stops at the first conflict | medium |
| Change in the Rust sidecar + Go supervisor + UI (item 4) | `verify` covering `cargo test` | `verify` does not run `cargo` (crates/sync); I ran `cargo fmt`, `clippy -D warnings` and `cargo test` separately, and the UI validation with 3 real instances (Playwright script). Proposal: an optional `rust_cmd` in the config | medium |
| Drawing in the user's local Paprika from the cloud | A planned path | Needed Remote Control + an MCP script written to the user's folder and deleted afterwards. Not tb's, but the design-first `mandate` could say what to do when 127.0.0.1:4777 is not reachable | low |

## Addendum 2026-11-22: importing formats and drag and drop on the canvas (branch `claude/project-thread-nsgbq0`, PR #79)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Twelfth thread with this | low |
| `tb --version` after `go install …@v0.0.10` | `v0.0.10` | `toolbelt dev`, `commit: unknown`: the version is not recorded in a binary installed by `go install`. Proposal: read `debug.ReadBuildInfo()` as a fallback | low |
| `tb init` in an already initialized repo | Not touch versioned files | Rewrote `.toolbelt/agent_docs.yaml` with 7 new hints taken from AGENTS.md (diff in the versioned file); undone with `git checkout`. Proposal: only rewrite with `--refresh-docs`, or warn that a versioned file changed | medium |
| `tb mem list` | Invariants | ok; the originality `mandate` drove recording `.fig` as "needs a decision" (reverse engineering) instead of promising the import | helped |
| `tb guard check <merge-base>..HEAD` on a docs-only change | Fast ok | `ok (4 rules, 0 violations)` | helped |
| Second PR on the same branch after #79 merged (i18n, PR #81) | `tb guard check` only on the new commit | ok (0 violations) against the merge-base with the updated `main`; no friction | helped |

## Addendum 2026-11-22: evaluating the A2UI protocol (thread "Evaluate the A2UI protocol in Paprika", no branch or PR)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@latest` + `PATH+=~/go/bin`. Thirteenth thread with this | low |
| `tb mem search original` / `tb mem search unknown` | Invariants that weigh on the decision | ok, found the originality `mandate` and the "unknown fields are rejected" one; both went into the risks (A2UI only by import, with a report of what was ignored) | helped |
| Evaluation-only task (external spec + throwaway spike outside the repo) | A "study" mode in tb | Nothing applies: `verify`/`guard`/`drift` need a diff. `tb egm` would review the report, but it requires the document inside the repo and the report went to a Claude Doc. Proposal: `tb egm review --file <path outside the repo>` in deterministic mode with only `refs-exist` and anti-injection | low |
| Spike calling Paprika's MCP over HTTP | `radius: "$token"` accepted as `fontSize` | Rejected ("expected number, received string"). Friction in Paprika's MCP, not in tb; recorded in the report as its own item | medium |

## Addendum 2026-11-22: planning a batch and publishing it on the bus (user's machine)

Task: read Paprika's backlog, plan batch 3 (17 tickets, lanes A/B) and publish it on the local bus as the supervisor (`--as leonardo`). No code change.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Same `SessionStart` warning as the previous addendum | — | Still `module version v0.0.10 is not the pinned v0.0.10` | medium |
| `tb bus send --payload @file` in a loop, with `--idempotency-key` | Batch publish, safe to rerun | Helped: 17 sends, one line each (`sent <ulid> seq=N`), the key makes a rerun harmless | helped |
| Knowing the next free ticket id (`PAP-B-<nnn>`) | `tb bus log --thread-prefix` or a list of threads | Had to `tb bus log paprika --limit 200 | grep -o 'PAP-[A-Z]-[0-9]*' | sort -u`; payloads are redacted in the log but thread ids are not, which is what made it work. Proposal: `tb bus threads <channel>` with state (assigned, claimed, completed) | medium |
| Lane A tickets (cloud threads, not on the bus) | A message type to reserve paths without assigning | No `task.reserve`/`announce`; I used `task.status` to `topic:board`, which a worker may read as progress. Proposal: an `announce` type or a `reserved_paths` field that `claim` checks for overlap | medium |
| Dependencies between tickets (`deps` in the payload) | `claim` refuses or warns while a dep is not completed | `deps` is free-form JSON the bus ignores; a worker can claim PAP-B-023 before PAP-B-022 is done | medium |
| Path overlap between open tickets | Supervisor-side check before send | Manual (I compared `paths` by hand); `tb bus channel show` lists claims, not the paths of assigned-but-unclaimed tickets | low |

Proposals: `tb bus threads`, a reserve/announce message for lane A, `deps` honored by `claim`, and an overlap check over assigned `paths` at send time.

## Addendum 2026-11-22: drawing icons over MCP after a directory move (user's machine)

Task: redraw six flat vegetable icons in the local Paprika over MCP and log the friction. No code change in the repo except the agent feedback log. The checkout was on `chore/house-cleaning`, whose last commit moved `server-go/` to `server/` and `docs/` to `docs/en-US/`.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `tb mem list` at the start | The invariants, fast | Helped: 10 invariants instantly; "never copy Paper" and "every mutation through applyOps" framed the work (draw only through MCP) | helped |
| `tb stack guard` with the default base `HEAD~1` | Pass, or flag real test changes | `ok: false` with 14 errors (11 `test_deletion`, 3 `test_skip`) in 0.3 s. `git show -M --stat` shows the test files as pure renames (`{server-go => server}/…_test.go`, 0 lines changed). Guard reads a rename as a deletion; the agent would have to rule out 14 false errors by hand before calling the work done | high |
| Invariant text after the move | Invariants that name paths stay true | The `server-go is the only server` invariant (and AGENTS.md) still names `server-go/`, which no longer exists; `tb mem check` does not look at paths | medium |
| A task with no code (design through MCP) | A way to say "verify not applicable" | Same as the 2026-11-21 design-only addendum: nothing in `tb` takes screenshots or a workspace id as evidence | low |

Proposals: guard should run with rename detection (`git diff -M`) and treat a rename with ≥90 % similarity as a move, not `test_deletion`; `tb mem check` could warn when an invariant names a path that does not exist in the tree.

## Addendum 2026-11-22: migration to toolbelt v1.0.0-rc.3 (user's machine, isolated worktree)

Task: move Paprika from `tb` v0.0.10 to v1.0.0-rc.3 (config, `SessionStart` hook, AGENTS.md and docs) from a worktree off `origin/main`, without touching the live `paprika` bus. The `server-go/` → `server/` move is still only on the unpushed `chore/house-cleaning` branch, so `main` keeps `server-go/` and the invariant stays correct there.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Finding what to change after an upgrade | A migration note per release, or `tb init --migrate` / `tb doctor` | Only the CHANGELOG (60 KB, rc.1 alone is ~40 bullets). Worked by running `tb init --json` in a scratch copy of the repo and diffing its `config.yaml` against the committed one. `tb init` never migrates an existing file | high |
| Every `tb` command on the old config | The removed keys reported once | Nine `warning: .toolbelt/config.yaml: unknown config key …` lines on stderr for **every** command (`tb mem list`, `tb guard`, `tb bus …`), before any output. Clear about what is wrong, noisy until someone migrates. A `tb config check` (or the warning naming the release that removed the key) would make it actionable | medium |
| PR that edits `verify:` (here: add `typecheck_cmd`, change `lint_cmd`) | Guard flags the change; verify still runs with the trusted base config | `verify_config_changed` is an error **and aborts verify**, also with `-c <file>`. A PR that migrates the verify config can never get a `PASS` from `tb verify`/`tb stack verify`; I ran the gates by hand (`pnpm go:vet && pnpm check`). The security reason is sound, but there is no path for a reviewed config change: proposal: run verify with the base config and report the finding, or a `--accept-config-change <sha>` that only a human (not MCP) can pass | high |
| `tb verify run --base origin/main` in a worktree | Base = the up-to-date main | `git fetch origin main` only moved `FETCH_HEAD`, so `origin/main` was stale and verify's guard ran against an old base (dozens of unrelated `test_deletion`s). Not a tb bug, but verify could warn when the base is not an ancestor-or-merge-base of head's upstream, or print the base's date | low |
| Rename false positive (reported in the previous addendum) | Fixed in rc.3 | Still there: a scratch repo with `git mv server-go server` gives 20 `test_deletion` errors plus `test_skip` warnings for skips that were already in the files. Cause: `internal/guard/guard.go` `parseDiff` marks the source of every `R*`/`C*` entry `deleted: true`. Proposal: for `R`, only count a deletion when the destination is not a test file (and do not re-report unchanged skips in `R100` files) | high |
| `go install …@v1.0.0-rc.3` in the cloud hook | Same binary as the release | Works and the `h1:` hash is on sum.golang.org. `tb version` now prints the version (fixed since rc.1) but `commit: unknown` and `embed: unavailable (built without embednative)`: semantic/hybrid graph search does not exist in cloud sessions that install through `go install`. Worth one line in the install docs | low |
| Bus during the upgrade | Know whether the live channel is safe | The store was already at schema v4 (`task_links` table) when I looked (read-only, `mode=ro`), so another `tb` had migrated it. Three `tb mcp` processes were still running a deleted binary (an older `tb`) against that v4 store. Nothing in `tb bus` tells the supervisor that a client runs an older version than the store; proposal: agents report their `tb` version on `heartbeat` and `channel show` flags mismatches | medium |
| `tb mem list` / `tb guard` speed | Fast | Helped: both instant; guard made the rename bug and the config gate visible in under a second | helped |

Main proposals: a migration section per release (or `tb init --migrate`), a reviewed path for `verify:` changes, the `parseDiff` rename fix, and version reporting on the bus.

## Addendum 2026-11-23: closing a channel and opening a new one for three Cursor sessions (user's machine)

Task: retire channel `paprika` (no live agents, no open claims), open `paprika-2`, and issue one distinct worker identity per Cursor session, after two sessions sharing one identity had collided in one worktree (PAP-B-013).

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Checking it is safe to close | Live agents and open claims in one view | Helped: `tb bus channel show paprika` listed four agents (all `alive=false`) and four claims (all `released outcome=expired`) in one call | helped |
| Closing the old channel | `tb bus channel close` (or `delete`/`archive`) | No such command. Used `tb bus channel extend paprika --ttl 1s` to make it expire, which works and is reversible (`extend --ttl 0`), but reads as a hack and `channel list` keeps showing it next to old smoke channels. Proposal: `channel close <ch>` that refuses while a claim is held or an agent is alive, and `list --open` | medium |
| `tb bus invite --name cursor-N` | The output names the agent the invite is bound to | Output is `invite: XEP9-D4ZF (channel paprika-2, role worker, single-use, expires …)`, without the bound name. With three invites issued in a loop, the mapping invite → name comes only from the order of the loop; there is no `invite list` to check it without spending the invite | medium |
| One Cursor window, several chats | One bus identity per chat | `tb mcp` binds the identity to the MCP connection, and a client that runs one MCP server process for several chats would give all of them the same identity: the same collision as PAP-B-013. Used the CLI join (`--as <name>` on every command) and one Cursor window per worktree instead. Proposal: the docs should say this outright, and `join` could refuse a second identity on the same connection with a clear error | high |
| Supervisor invite with `--invite-ttl 24h` | Time to hand it over | Helped: `channel create` takes `--invite-ttl`, and `invite --ttl 24h` covers a user who pastes the invites hours later | helped |
| Nine `unknown config key` warnings | Once | Still on every command (see the rc.3 migration addendum) | low |

Proposals: `channel close`, the bound name in the `invite` output (and an `invite list` for the human), and an explicit "one identity per MCP connection" warning.

## Addendum 2026-11-24: opening `paprika-3` after closing `paprika-2` (user's machine, tb v1.0.0-rc.4)

Task: open a fresh channel for the same three Cursor workers (`cursor-1/2/3`) and the supervisor `leonardo`, with no tickets yet (the focus is stabilizing v0.1.0). Same recipe as the 2026-11-23 addendum.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `SessionStart` hook | Accept the local `tb` or name what differs | `tb on PATH … module version v1.0.0-rc.4 is not the pinned v0.0.10`: the repo's hook still pins v0.0.10 while the project moved to rc.x. Paprika-side drift, but `tb` has no way to declare "this repo needs tb ≥ X" that the hook could read | low |
| Creating the channel | One call with lease and a long owner invite | Helped: `channel create paprika-3 --owner leonardo --ttl 0 --lease 5m --invite-ttl 24h` | helped |
| Issuing three bound invites | The output names the bound agent | Still `invite: 1SV8-XMNH (channel paprika-3, role worker, …)` without the name; mapping comes from loop order (same as 2026-11-23) | medium |
| Wake files for the new channel | `tb bus run` init/copy from the previous channel | No command; copied `wake/paprika-2/cursor-N.yaml` to `wake/paprika-3/` with `sed` on the comment and `chmod 600`. Proposal: `tb bus run --init --from <channel>` or a `wake` template per agent | low |
| One-command start for a worker | Join and wait together | Helped: `tb bus run --invite <code>` joins when there is no session yet, so each Cursor window needs one line for the harness | helped |
| Old channels in `channel list` | Closed ones hidden | `paprika` and `paprika-2` (expired via `extend --ttl`) still listed next to smoke channels; still no `channel close` or `list --open` | low |
| `unknown config key` warnings | Gone after migration | Gone: no warning on any command this session | helped |
| Recovering payloads of cancelled tickets from a closed channel | `tb bus log --thread X --payload` for the supervisor, or re-send from a thread | I assumed `log` still redacts payloads (as `docs/agent-bus.md` in Paprika says) and read `messages.payload` from `bus.db` with Python in `mode=ro`. Later `tb bus log paprika-3` printed full payloads in rc.4, so the workaround was not needed; the consumer docs are what is stale. Proposal: say in the CHANGELOG/help of `log` whether payloads are shown, and offer `send --from-thread <channel>/<thread>` to re-publish | low |
| Re-publishing 10 tickets with `deps` on a new channel | The bus says what it does with `deps` | Helped (new in rc.4): every send warns that the broker does not enforce `deps` and points at `tb router` or sending the next task on completion. Clear, but printed 10 times in a loop; once per command run would do | helped |
| Worker joined without `--wake` | The `run` harness picks up the wake file, or `join` warns that a wake file exists for this name | `join` defaults to `--wake manual`; `run` then shows `manual` on the card plus a `wake_mismatch` warning, and fixing it needs a fresh invite and a second join (the first invite is spent). The supervisor's instructions (mine) left the flag out. Proposal: `join` reads the wake file's adapter when one exists, or `run` can update the declared wake mode without a new invite | medium |

## Addendum 2026-11-24: evaluating canvas libraries (thread "Evaluate canvas libs for the editor", no branch or PR)

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Session start | `tb` on PATH | It was not; `go install …/cmd/tb@latest` (~1 min). Fourteenth thread with this; `tb --version` still `toolbelt dev`, `commit: unknown` | low |
| `tb mem list` | Invariants that weigh on the decision | ok, and decisive: "nodeStyle is the single rendering source" and "every mutation goes through applyOps" rule out Konva/Fabric/Pixi/tldraw/Excalidraw without a spike | helped |
| Evaluating an external dependency (license, fit) | Some tb support for "evaluate a dependency" | Nothing applies: `verify`/`guard`/`drift` need a diff. Proposal: `tb mem check-dep <package> <license>` checking the license against an allowlist in invariants.yaml (e.g. MIT/Apache/BSD only), to block licenses like the tldraw SDK's early | low |

## Addendum 2026-11-24: fix for PR #166 (cloud session, tb v1.0.0-rc.3)

One-commit change limited to `server-go/cmd/paprika`. Validated with `go test ./...` and `golangci-lint` directly, without the full `verify`.

| Situation | Expected | Got | Severity |
|-----------|----------|-----|----------|
| Install in a new session | `tb` ready when the session opens | `go install …/cmd/tb@v1.0.0-rc.3` requires go ≥ 1.27.4; the toolchain auto-switch downloaded 1.27.4 without error, but it cost ~1 min. Proposal: a prebuilt binary in the hook, or a clear message when the local toolchain is below the required one | low |
| `tb guard check origin/main..HEAD` on a one-commit PR | Fast ok | `ok (4 rules, 0 violations)`, fast and quiet | helped |
| `tb mem list` | Readable invariants | ok, nothing to adjust | helped |
| `tb verify run` | Full gate | Not run: the change was limited to one Go package; `go test ./...` and `golangci-lint` ran outside `tb` | neutral |

## Addendum 2026-11-25: brand concepts (thread "Refine the Paprika brand concept")

| Command | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `tb` in the cloud session | Already on PATH through the SessionStart hook (TB_VERSION v1.0.0-rc.3) | Missing; needed `go install …/cmd/tb@v1.0.0-rc.3` → ~/go/bin (outside PATH) | medium |
| `tb guard` | Run the default diff check | Prints the help; the check is `tb guard check` | low |
| `tb guard check` / `tb mem check` | ok on a docs-only change | `guard: ok (4 rules, 0 violations)`; `invariants: 10, retention 1.00` | ok |

## Addendum 2026-11-25: GitHub Actions time with act (thread "Reduce GitHub Actions time", branch `claude/project-thread-0puwlr`, no PR)

| Command | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `tb` in the cloud session | On PATH | Missing; `go install …/cmd/tb@latest` → ~/go/bin; `tb --version` = `toolbelt dev`, `commit: unknown` | low |
| `tb guard check` (commit of the Rust sidecar fix) | ok | `guard: ok (4 rules, 0 violations)` | ok |
| CI investigation (per-job/step times via `gh api`, local `act`) | Some support for measuring the pipeline | Nothing applies; I aggregated the times with my own script. Proposal: `tb bench ci <run-ids>` aggregating median/p90 per job and step and pointing out the critical path | low |
| Test with a fixed wait (sidecar killed after 5 s in every test) | tb flagging a test that is too slow | Nothing; found by measuring by hand. Proposal: a `guard`/`verify` rule flagging a test whose duration ≈ an exact multiple of a timeout in the code | medium |

## Addendum 2026-11-25: house cleaning (thread "Do house cleaning in the repository", branch `claude/project-thread-rkbjtz`)

| Command | Expected | Got | Severity |
|-----------|----------|-----|----------|
| `tb` in the cloud session | On PATH through the SessionStart hook | Missing; I ran the hook by hand (`CLAUDE_CODE_REMOTE=true bash .claude/hooks/session-start.sh`), which installed the pinned v1.0.0-rc.3 in ~/go/bin (outside PATH) | low |
| `tb guard check <merge-base>..HEAD` with `git mv server-go server` | Renames recognized | 60 `test_deletion` (error) on the old paths of the moved `_test.go` files; rc.3's known false positive scales badly on a whole-directory rename. Proposal: follow renames with `git diff -M` and report `test_moved` (info) | high |
| `tb guard check` with an intentional removal of tests (bench/) | A way to declare the removal in the ticket | Only `test_deletion` (error), no way to accept it; I had to prove it by hand by counting `func Test`/`it(` before and after. Proposal: `allow_test_deletion: [glob]` in the rd.md frontmatter | medium |
| `tb guard check` after stripping comments | Nothing about lines whose meaning did not change | `test_skip` (warning) on every `t.Skip()` in a touched file, even when the line did not change. Proposal: only flag a `t.Skip` added in the diff | medium |
| `tb stack verify guard --ticket rd.md` | Check the branch's range | The stack's guard used `HEAD~1` as the base (only the last commit); the full range needs a separate `tb guard check`. Proposal: default base = merge-base with `main` | medium |
| `bench:` in `.toolbelt/config.yaml` pointing at a removed directory | A warning or an error | I removed the section and `tb mem list` kept working; no command complained before, even with the directory missing | low |
| `tb stack verify` (build, typecheck, go:vet, tests) | ok | ok, with `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium` (the `chromium-1243/...` path trips the revision check 1243 ≠ 1255) | ok |
