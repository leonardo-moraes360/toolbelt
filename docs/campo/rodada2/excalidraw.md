🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/excalidraw.md)

# Field round 2: excalidraw/excalidraw

Study on 2026-11-16 with `tb` from `toolbelt@a46c295` (`make build` → `./bin/tb`). Local clone at `/tmp/target/excalidraw` (shallow). Group: **with existing AI-agent instrumentation**. **No** interaction with the Excalidraw remote. Evidence under `/tmp/campo2-evidence/`.

## Repository

| Field | Value |
|-------|-------|
| Name | [excalidraw/excalidraw](https://github.com/excalidraw/excalidraw) |
| Language | TypeScript / React (Yarn workspaces monorepo) |
| Size | ~1375 TS/JS files (without `node_modules`); shallow checkout ~93 MiB; with `node_modules` ~1.1 GiB |
| Base commit | `82190fb` (`master` shallow) |
| AI instrumentation | `AGENTS.md` (DOM/`ownerDocument`, `Merge<>`); `CLAUDE.md` (monorepo structure, `yarn test:typecheck` / `yarn test:update` / `yarn fix`) |
| Toolchain | **Configured**: Node 22, Yarn 1.22.22, `yarn install --frozen-lockfile` OK, Vitest 3.0.6, `tsc`, ESLint, headless Chrome (`/usr/local/bin/google-chrome`) |
| Not configured | App in the browser / Playwright UI E2E; production `yarn build`; full `yarn test:all` suite (typecheck+eslint on task files and vitest math were run) |

## Chosen task

| Field | Value |
|-------|-------|
| What | Slice 1 of [#9297](https://github.com/excalidraw/excalidraw/issues/9297): add types `ViewportPoint` and `GenericPoint` in `@excalidraw/math`, helper `viewportPointFrom`, tests in `packages/math/tests/point.test.ts`. Full migration of `extends GlobalPoint \| LocalPoint` → `GenericPoint` **deferred** (monorepo typecheck breaks when `ViewportPoint` enters existing constraints). |
| Why | Small/medium feature, isolated in the math package, with a real issue and unit tests; different from round 1. |
| Issue | https://github.com/excalidraw/excalidraw/issues/9297 |
| Completed? | **Yes, locally** (commit `619250f` on the clone; **not** sent upstream). A broad attempt to swap constraints produced ~29 `tsc` errors (utils/element/excalidraw); the typed slice+helper passed. |
| Native checks | `yarn vitest run packages/math/tests` → 9 files / 60 tests PASS (~2.7–3.0 s). `yarn test:typecheck` → PASS (~14–15 s). ESLint on touched files: 1 Prettier warning on the 1st draft; after `--fix` → PASS. |
| Round 1 (comparison) | Round 1: bugfix `normalizeRadians` → `0` on multiples of 2π (issue #12599). Round 2: types `ViewportPoint`/`GenericPoint` (#9297). |

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Clone under `/tmp/target/excalidraw`; `TB_GRAPH=on tb graph rebuild --enable` → 4453–4454 nodes / ~18471 edges (~1.47 s), not an empty graph (`09-graph-rebuild.txt`) |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; `--hidden-test-cmd "yarn vitest run …point.test.ts"` → `hidden_available: true`, `hidden_passed: true`, verdict PASS (`31b-hidden-no-dir.txt`). With `false` → `hidden_available: true`, `hidden_passed: false`, ESCALATE (`31c`) |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | With `.toolbelt/guard.yaml` protecting `types.ts` (changed in the diff): without `--strict` exit 0 + violations; with `--strict --json` exit **1** + valid JSON (`29b`/`30b`) |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | YAML with `packages/math/src/types.ts` → `protected_path` violation on that file; without YAML → `violations: null` on the same diff (`28`/`29`) |
| 5 | init toolchain detection | **worked** (useful partial) | `tb init` → `verify.test_cmd: yarn test` (no longer `go test`); did not fill `build_cmd`/`lint_cmd` alone from scripts/`CLAUDE.md` (`02-init.txt`) |
| 6 | review `tests/hidden` by path | **worked** | `rd.md` mentions `tests/hidden/**` under Forbidden; task diff does not touch that path → `findings: []` (`27-review.txt`, `40-review-no-fp-hidden.txt`) |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `bench.default_agent: echo default-agent-used` + `tb bench run -r 1 --task bugfix-go` (without `--agent`) started `agent=external` and finished (`33b`). `tb bench compare … --agent "echo compare-agent"` accepted the flag and ran A/B (`35b`). Help documents `-a/--agent` |
| 8 | review/drift `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | Ticket with `## Escopo` + bullets + `packages/math/tests/**` and `packages/math/src/**/*.ts`; clean drift/review on the task commit; `CONTRIBUTING.md` commit → `out_of_scope: ["CONTRIBUTING.md"]` (`26`, `39b`) |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Config with `build_cmd: yarn test:typecheck` and `lint_cmd: eslint…`; 1st verify → lint failed (Prettier), `lint_ran/passed`, `build_ran/passed`, verdict ESCALATE (`23`). After fix → PASS with build+lint+visible+hidden (`25`) |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** | Init logged sources + hints; wrote `.toolbelt/agent_docs.yaml`. `--write-agents` on Excalidraw **did not** overwrite `AGENTS.md`. In an empty repo seeded a minimal `AGENTS.md` (`02`, `02c`, `38`) |
| 11 | graph Python `calls` | **not applicable** | TS/React repo; JS `calls` edges present in stats (`edges_calls: 8871`). Python indexer not exercised |
| 12 | aggregated visual skipped + CLI docs aligned | **worked** | Manifest without refs → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"` (`32-visual-skipped.txt`). `docs/cli.md` documents hidden without dir, build/lint, `default_agent`, ticket as planned, visual skipped (`41`) |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` (`make build`) | `toolbelt a46c295` with commit/built | neutral | `01-version.txt` |
| `go run ./cmd/tb version` | `0.1.0-dev`, commit/built `unknown` | hindered (without ldflags) | `48-version-norflags.txt` |
| `tb init` | `.toolbelt/` + `yarn test`; read AGENTS/CLAUDE; mode `600` | helped (onboarding much better than round 1) | `02-init.txt` WALL 0.006 s |
| `tb init --write-agents` | does not overwrite existing; seeds if absent | helped | `02c`, `38` |
| `tb mem init/add/list/check` | OK; invariants + sha256 | helped | `03`–`06` |
| Ticket (manual `rd.md`) | `tb ticket` still nonexistent (docs: Phase 3+) | neutral (manual flow documented) | `41`, `08-rd.md` |
| `TB_GRAPH=on tb graph rebuild` | ~4454 nodes / ~18474 edges | helped | `09`, `42` |
| `tb graph search viewportPointFrom` | found the new function | helped | `43` |
| `tb graph callers --function pointFrom` | callers inside `packages/math` (curve/point/segment); little cross-package | hindered partially | `45` |
| `tb graph impact --file point.ts` | impact limited to the math package | hindered partially | `46` |
| `tb verify run` (build+lint+test+hidden) | FAIL→PASS; caught Prettier; hidden without dir | helped | `23`, `25`, `31b` |
| `tb verify --ruler visual` | honest top-level skipped | helped | `32` |
| `tb review rd.md` | no findings; no `tests/hidden` FP | helped | `27`, `40` |
| `tb drift rd.md` | clean on the task; detected OOS; still prints Usage after JSON | helped / UX hinders | `26`, `39b` |
| `tb guard check` + `guard.yaml` + `--strict --json` | YAML honored; strict exit 1 | helped | `28`–`30b` |
| `tb bench run` via `default_agent` | works without `--agent` | helped | `33b` |
| `tb bench compare --agent` | flag accepted; A/B OK | helped | `35b` |
| `tb mcp` + `tools/list` | initialize + tools (env/run/graph/…) | helped | `37b-mcp.txt` |
| Native checks | vitest/tsc/eslint as above | helped (baseline) | `17`–`22`, `49`–`50` |

## Problems that persist

- **Weak cross-package callers/impact in the TS monorepo** (round 1 P2): `pointFrom` / `types.ts` still do not list consumers in `packages/element` / `excalidraw` usefully (`45`, `46`, and search for `GlobalPoint` only found substring matches in function names — `11`).
- **`version` → `unknown` without ldflags** (`48`).
- **Configs mode `0600`** (`02-init.txt` stat).
- **Cobra Usage after JSON** on `tb drift` with drift (`39b`: JSON + `Error:` + Usage).
- **Init does not promote `CLAUDE.md` hints to `build_cmd`/`lint_cmd`**: only logs hints and sets `test_cmd: yarn test`; typecheck/eslint still need manual YAML edits (residual onboarding gap).
- **`tb ticket` not implemented** (docs now mark it as planned — better expectation, command still missing).

## New problems

### P2 — AGENTS/CLAUDE hints do not become `verify.build_cmd`/`lint_cmd`

- **Repro**: `tb init` on Excalidraw (`yarn test:typecheck` / `yarn fix` in `CLAUDE.md`).
- **Expected**: at least suggest/fill build/lint beyond `yarn test`.
- **Actual**: `agent_docs.yaml` with hints; `config.yaml` only with `test_cmd: yarn test` (`02-init.txt`).

### P3 — Absolute YAML `bench.tasks_dir` may be ignored depending on cwd

- **Repro**: under `/tmp/bench-probe`, config with `tasks_dir: /agent/repos/toolbelt/bench/tasks`; `tb bench run -r 1`.
- **Expected**: open the absolute path.
- **Actual**: log `tasks_dir=bench/tasks` relative and fails `open bench/tasks` (`33-bench-default.txt`). In the toolbelt repo (right cwd) default_agent works.

### P3 — Expectation regression: verify “PASS” still does not mean “maintainer-complete task”

- Not a code regression of the 12 fixes (build/lint now work when configured), but the post-init default still passes without Excalidraw typecheck/eslint. The agent must remember to mirror `CLAUDE.md` into YAML.

No regression observed on the 12 listed fixes when exercised on purpose.

## What tb caught and what it let slip

**Caught**

- Prettier warning on the new helper via `lint_cmd` (verify ESCALATE until fixed).
- Drift of `CONTRIBUTING.md` outside `## Escopo`.
- Protected path via `guard.yaml` + exit 1 with `--strict --json`.
- Configured hidden suite without a `tests/hidden/` directory.
- Visual with all screens skipped → top-level `skipped: true`.
- Monorepo indexing under `/tmp/target` and locating `viewportPointFrom`.
- Reading `AGENTS.md`/`CLAUDE.md` in init; no review FP from mentioning `tests/hidden`.

**Let slip**

- Cross-package consumers of point APIs (graph).
- Auto-fill of typecheck/eslint in config (hints only).
- “Maintainer” completeness if the agent accepts only the post-init default `yarn test`.
- The failed attempt to migrate all generics to `GenericPoint` (task/monorepo typecheck limitation — tb does not analyze that API risk).

## Interaction with existing instrumentation / onboarding

- **Clear improvement vs round 1**: `tb init` detects Yarn, writes `agent_docs.yaml`, extracts commands from `CLAUDE.md`, and `--write-agents` respects an existing file.
- **Still incomplete**: hints do not become `build_cmd`/`lint_cmd`; default `mem init` invariants stay generic (`tests/hidden`, `thresholds.yaml`) and do not import `AGENTS.md` bullets (`ownerDocument`, `Merge<>`).
- **No destructive conflict**: did not overwrite `AGENTS.md`/`CLAUDE.md`.
- **What would still need importing**: (1) map hints → `verify.*`; (2) `AGENTS.md` bullets → `tb mem` mandate; (3) Yarn workspaces for graph/impact defaults.

## Comparison with round 1

| Aspect | Round 1 (`c8aae04`) | Round 2 (`a46c295`) |
|--------|---------------------|---------------------|
| init / toolchain | `go test` in a Yarn repo | `yarn test` + AGENTS/CLAUDE read |
| verify | only test_cmd; hidden dead without dir; PASS without tsc | build/lint/hidden honored; caught Prettier |
| guard.yaml / strict JSON | ignored; strict+json exit 0 | honored; exit 1 |
| review FP `tests/hidden` | confirmed | absent on this ticket |
| Markdown Scope | fragile / frontmatter | `## Escopo` + globs OK |
| bench default_agent / compare `--agent` | broken | work |
| visual skipped top-level | lying | `skipped: true` |
| graph under `/tmp/target` | SkipDirs risk (other repos) | indexed normally |
| TS cross-package callers | weak | still weak |
| CLI docs | outdated | aligned on checked points |

**Verdict:** clearly **better** on the 12 prioritized fixes; structural TS monorepo graph pain and “pushing completeness” from native instrumentation **remain**.

## Improvement suggestions

1. **[P1 · init/verify]** Apply `CLAUDE.md`/`package.json` hints to `build_cmd`/`lint_cmd` (or an interactive checklist), not only log + `test_cmd`.  
2. **[P1 · mem/init]** Import `AGENTS.md` bullets as `mandate` invariants.  
3. **[P2 · graph]** Resolve cross-package callers/impact with path aliases / Yarn workspaces (TS).  
4. **[P2 · bench]** Honor absolute `tasks_dir`/`fixtures_dir` in config regardless of cwd.  
5. **[P3 · cli]** With `--json`, do not print Cobra Usage after an error (`drift`/`review`).  
6. **[P3 · verify]** `js-monorepo` profile that combines test+typecheck+lint from known scripts.

## Metrics

| metric | round 2 | round 1 (ref) |
|--------|---------|---------------|
| Summed wall time (timed tb + native sample in the log) | ~103 s (includes failed typecheck of the broad attempt + verify×2) | ~49 s |
| Approx. tb invocations (with probes of the 12 fixes) | ~55–70 | ~45–55 |
| Evidence files under `/tmp/campo2-evidence` | 68 | — |
| Observed tb failures | verify ESCALATE (Prettier lint) until fix; expected guard strict exit 1; OOS drift; bench cwd/tasks; compare without before/after | drift/review/guard/bench product bugs |
| Graph rebuild | ~1.47 s / 4453 nodes / 18471 edges | ~1.05 s / 4452 / 18465 |
| Final verify PASS (build+lint+test+hidden) | ~20.2 s | ~2.9 s (vitest only) |
| Vitest math | ~3.0 s, 60 tests PASS | ~2.8 s, 64 tests |
| `tsc` (OK slice) | ~14.8 s PASS | ~13.6 s PASS |
| MCP `tools/list` | OK | OK |
| Bench fake (toolbelt, 9 tasks) | 33.3% success (fake baseline) | same |

**Note:** the task was closed with typecheck+eslint+vitest; tb only “closed” the loop after we configured `build_cmd`/`lint_cmd`/`hidden_test_cmd` manually — real progress vs round 1, still dependent on the agent mirroring existing instrumentation.
