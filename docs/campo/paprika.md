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
