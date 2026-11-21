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
