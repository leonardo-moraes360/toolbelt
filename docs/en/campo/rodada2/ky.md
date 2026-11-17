[🇧🇷 Português](../../../campo/rodada2/ky.md) | 🇺🇸 English

# Field round 2: `tb` on sindresorhus/ky

Evidence collected on 2026-11-16 on a local clone at `/tmp/target` (no push, issue, PR, or comment upstream). `tb` built from this repo at `main@a46c295` (`make build` → `/tmp/tb`). Only product change in this study: this file. Work on ky stayed restricted to the VM.

## Repository

| Field | Value |
|-------|-------|
| Name | [sindresorhus/ky](https://github.com/sindresorhus/ky) |
| Language | TypeScript (ESM), HTTP client over Fetch |
| Size | ~109 files outside `node_modules`/`.git`/`distribution`/`.toolbelt`; ~1.7 MiB; ~4.9 kLOC in `source/**/*.ts`, ~30.8 kLOC in `test/**/*.ts` |
| AI instrumentation | `AGENTS.md` (`null` vs `undefined` convention); no `CLAUDE.md` / `.cursorrules` |
| Group | with existing AI-agent instrumentation |
| Toolchain | **Yes, with caveats.** Node v22.23.4 + npm 10.9.9; `npm install` + `npm run build` OK; `xo` on touched files OK; `ava test/validation.ts` 18 passed; `ava 'test/!(browser).ts'` → **1528 passed**. Did not run the full Playwright browser matrix / final `npm test` (costly). |

## Chosen task

| Field | Value |
|-------|-------|
| What | Reject `null` on enum-like RequestInit options (`credentials`, `mode`, `cache`, `redirect`) with a clear `TypeError` that names the option (`The \`<option>\` option must not be \`null\`. Use \`undefined\` to clear it.`), aligned with `AGENTS.md`; test in `test/validation.ts`. |
| Why | Before, those `null`s leaked opaque `Request constructor` errors (e.g. `null is not an accepted type. Expected one of omit, same-origin, include`). Maintainer-plausible and distinct from round 1. |
| Issue | No useful open issue (`gh issue list --state open` empty). Task derived from code + `AGENTS.md`. |
| Complete? | **Yes, locally** (commits only on the clone; not sent upstream). |
| Native checks | `npm run build` OK; `xo` on touched files OK; `ava test/validation.ts` 18 passed; `ava 'test/!(browser).ts'` 1528 passed; `tsc --project tsconfig.test.json --noEmit` OK via `hidden_test_cmd`. |
| Round 1 (comparison) | Reject invalid `retry.jitter` returns with `TypeError` + `cause`. **Not repeated** (upstream still silently falls back to delay without jitter — behavior documented in current tests). |

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph index --enable .` under `/tmp/target` → `589 nodes, 3802 edges` (WALL 0.246 s). In round 1 the same path gave 0 nodes. |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; config `hidden_test_cmd: npx tsc --project tsconfig.test.json --noEmit` → verify JSON `hidden_available: true`, `hidden_passed: true`. |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff touching `source/index.ts` (in `guard.yaml`): without `--strict` exit 0 with violations; with `--strict --json` **exit 1** + violation JSON. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `source/index.ts` → `protected_path` violation on that file. Hardcoded `.toolbelt/thresholds.yaml` still protected. |
| 5 | init toolchain detection | **worked** (partial) | `tb init` filled `verify.test_cmd: npm test` from `package.json`. **Did not** auto-fill `build_cmd`/`lint_cmd` (`build`/`xo` exist in scripts). |
| 6 | review `tests/hidden` matches by path | **worked** | Ticket `/tmp/rd-hidden-mention.md` mentions `tests/hidden` and `thresholds.yaml` only in prose; real diff does not touch those paths → `findings: []`. |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `bench.default_agent: echo AGENT {prompt}`; `tb bench run -r 1` without `--agent`/`--fake` ran (external agent). `tb bench compare ... --agent "echo AGENT {prompt}"` OK; compare without `--agent` also used default. |
| 8 | review parses `## Scope`/`## Escopo`, MD lists and `**` | **worked** | `rd.md` with `## Scope` + bullets `` `source/core/**/*.ts` `` etc. → `tb drift`/`tb review` `out_of_scope: null`. Narrow scope only `test/validation.ts` → `out_of_scope: [rd.md, source/core/Ky.ts]`. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Set manually; verify JSON: `build_ran/passed: true`, `lint_ran/passed: true`, `verdict: PASS` (WALL 5.809 s). |
| 10 | init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents` | **worked** (partial) | Init: `documentação de agentes encontrada sources=AGENTS.md` → `.toolbelt/agent_docs.yaml`. `hints: []` (AGENTS.md has no run tips). `--write-agents` **did not** overwrite existing AGENTS.md. Under `/tmp/empty-agents` without AGENTS.md, `--write-agents` seeded a minimal tb AGENTS.md. |
| 11 | graph Python `calls` edges | **not applicable** | TypeScript repo; JS/TS index emitted `edges_calls: 2875`. No Python code in ky to validate the Python indexer. |
| 12 | aggregated visual skipped + CLI docs aligned | **worked** | `tb verify run --ruler visual` → top-level `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. `docs/cli.md` documents hidden without requiring a dir, build/lint, Scope MD, guard.yaml, default_agent, visual skipped. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; detected npm; read AGENTS.md | helped | WALL 0.004 s; `test_cmd: npm test`; `agent_docs.yaml`; config mode `600` |
| `tb init --write-agents` | does not overwrite existing; seeds if absent | helped | ky: AGENTS.md intact; `/tmp/empty-agents`: AGENTS.md generated |
| `tb mem init/list/add/check` | OK; docs flags `--type/--title/--body` fail | neutral/hindered | `unknown flag: --type`; default invariants still Go-centric (`tests/hidden`) |
| `tb graph index --enable .` under `/tmp/target` | 589 nodes / 3802 edges | helped | SkipDirs at root fixed |
| `tb graph impact/deps/search` | impact with repo path OK if root=`.`; `#calculateDelay` missing | mixed | `source/core/Ky.ts` hit with root `.`; with root `source`, `--file source/core/Ky.ts` → empty / `--file core/Ky.ts` → hit |
| `tb verify run` (tests) | PASS with build+lint+visible+hidden | helped | Full JSON; hidden without dir |
| `tb verify run --ruler visual` | PASS skipped top-level | helped (vs R1) | `skipped: true` (before `skipped: false` with skipped screens) |
| ticket / Scope | Manual `rd.md`; no `tb ticket` | neutral | docs mark ticket as planned |
| `tb review` | empty findings on happy path; Scope MD OK | helped | exit 0; substring `tests/hidden` FP gone |
| `tb drift` | OK; `**` globs in Scope | helped | OOS detected with narrow scope |
| `tb guard check` | yaml + strict JSON OK; `base..head` API | helped / mixed UX | different from verify/review `--base/--head` |
| `tb bench run/compare` | default_agent and `--agent` OK | helped | fixtures copied from toolbelt (ky has no bench/) |
| `tb mcp` | stdio; tools/list + `tb_version` OK | neutral | still no verify/review/drift/guard/mem tools |
| `tb version` | with ldflags `a46c295`; without → `0.1.0-dev` + unknown | neutral | expected without `-ldflags` |

## Problems that persist

From round 1 (`docs/campo/ky.md`) that still showed up:

1. **Private `#foo` methods not indexed** — `graph search/callers` for `calculateDelay` → empty.
2. **Graph paths relative to indexed root** — indexing only `source` still requires `--file core/Ky.ts`, not `source/core/Ky.ts`.
3. **`review`/`drift` without `-o` shorthand** — `unknown shorthand flag: 'o'` (verify accepts `-o`).
4. **MCP without verify/review/drift/guard/mem** — only echo/env/run?/graph_*/version.
5. **`version` unknown without ldflags** — confirmed.
6. **Config files mode `0600`** — `config.yaml` and `agent_docs.yaml` at `600`.
7. **`mem add` positional only** — old docs flags (`--type/--title/--body`) nonexistent.
8. **No `tb ticket`/board/progress** — agent writes `rd.md` by hand; docs now warn “not implemented”.
9. **Go-centric default invariants** — `mem init` still creates `tests/hidden` / thresholds rules in a TS repo (no longer FP in review by substring, but still onboarding noise).
10. **Init does not import `AGENTS.md` rules as invariants** — only records path; `hints: []`.

## New problems

### P2 — `tb init` does not fill `build_cmd`/`lint_cmd` from `package.json`

- **Repro:** `tb init` in a repo with `build` scripts and lint via `xo`/`npm test` that already includes xo.
- **Expected:** suggest `build_cmd`/`lint_cmd` (or document that only `test_cmd` is auto).
- **Actual:** only `verify.test_cmd: npm test`; build/lint need manual yaml edits.

### P2 — `guard check` uses positional `base..head`; verify/review/drift use `--base`/`--head`

- **Repro:** `tb guard check --base A --head B` → `unknown flag: --base`. Correct form: `tb guard check A..B`.
- **Expected:** aligned flags across subcommands.
- **Actual:** UX inconsistency; friction in the field protocol.

### P3 — `agent_docs.yaml` with `hints: []` even with substantial `AGENTS.md`

- **Repro:** init on ky → sources=[AGENTS.md], hints=[].
- **Expected:** at least extract the null→TypeError convention as a tip/invariant, or document that only “how to run” blocks are extracted.
- **Actual:** nominal read with no operational value for this AGENTS.md.

### P3 — Bench on a target repo requires copying `bench/tasks` from toolbelt

- **Repro:** ky has no `bench/tasks`; `tb bench run` only worked after copying fixtures.
- **Expected:** clear error or minimal scaffold in init.
- **Actual:** depends on material outside the repo under test.

## What tb caught and what it let slip

**Caught**

- Real drift when Scope omitted `source/core/Ky.ts` and `rd.md`.
- Guard on `guard.yaml` `protected_paths` and on `.toolbelt/thresholds.yaml`.
- Verify would fail if build/lint/test/hidden broke (all ran and passed).
- Review without substring FP on `tests/hidden` from ticket mention.
- Graph indexable under `/tmp/target`; useful deps/impact with root `.`.
- Visual report with `skipped: true` when there is no manifest.

**Let slip / hindered**

- Did not extract the `AGENTS.md` rule into invariants (the task was chosen by reading AGENTS.md outside tb).
- Did not index `#calculateDelay` / internal callers.
- Did not suggest `build_cmd`/`lint_cmd` in init.
- MCP does not cover the verify/review loop.
- Did not push completeness beyond what the agent configured in `rd.md`/verify.

## Interaction with existing instrumentation / onboarding

- **`AGENTS.md`:** `tb init` **detects and records** it in `.toolbelt/agent_docs.yaml` (improvement vs round 1). Does not merge content into `mem` invariants. `hints: []`.
- **`--write-agents`:** respects existing AGENTS.md; useful for repos without instrumentation.
- **Residual duplication:** `mem init` still injects generic Go/toolbelt anti-cheat beside ky conventions.
- **Onboarding:** detects npm → `test_cmd`; does not import `build`/`xo` scripts; does not create a ticket. Workflow still depends on manual `rd.md` + edited yaml.

## Comparison with round 1

| Aspect | Round 1 | Round 2 | Verdict |
|--------|---------|---------|---------|
| SkipDirs `/tmp/target` | 0 nodes | 589 nodes | **better** |
| hidden_test_cmd | only with `tests/hidden/` dir | runs without the dir | **better** |
| build/lint in verify | absent | `build_ran`/`lint_ran` true | **better** |
| guard.yaml | ignored | read + strict JSON exit 1 | **better** |
| default_agent / compare --agent | broken | work | **better** |
| review FP `tests/hidden` | confirmed | refuted in this run | **better** |
| Scope `##` + `**` | fragile / partial | worked on this ticket | **better** |
| visual skipped top-level | `skipped: false` | `skipped: true` | **better** |
| AGENTS.md | ignored | read (empty hints) | **partially better** |
| `#private` in graph | missing | missing | **same** |
| MCP coverage | only graph/echo/version | same | **same** |
| `-o` on review/drift | missing | missing | **same** |
| Instrumented tb wall | ~40.7 s | ~6.2 s (fewer slow probes; verify 5.8 s) | **better** (different load) |

Overall: **clearly better** on the 12 applicable fixes; structural graph/MCP/ticket problems remain.

## Improvement suggestions

1. **[P1][init]** Auto-fill `build_cmd`/`lint_cmd` from `package.json` scripts (beyond `test_cmd`).  
2. **[P1][init/mem]** Extract convention bullets from `AGENTS.md` → invariants (not only path in `agent_docs.yaml`).  
3. **[P1][guard]** Accept `--base`/`--head` like verify/review/drift.  
4. **[P2][graph]** Index TS `#private` methods; normalize `--file` to the repo root.  
5. **[P2][cli]** `-o` shorthand on review/drift; scaffold `bench/tasks` in init or an actionable error.  
6. **[P2][mcp]** Expose verify/review/drift/guard/mem.  
7. **[P3][config]** `0644` for non-secret yaml (or document 0600).  
8. **[P3][mem]** Default invariants per detected toolchain (not always `tests/hidden`).

## Metrics

| metric | round 2 | round 1 (ky) |
|--------|---------|--------------|
| Wall time sum `*.time` (tb+native) | ~37.5 s | ~66.8 s |
| Wall time tb only (timed) | ~6.2 s | ~40.7 s |
| Timed tb invocations | 24 | ~61 |
| Intentional tb failures (probes) | several (guard strict, OOS drift, docs flags, graph off, `-o`) | 14 |
| Binary crashes | 0 | 0 |
| Native `ava 'test/!(browser).ts'` | 1528 passed / ~31.3 s | 1528 passed |
| `tb` version | `a46c295` | `c8aae04` |

Raw evidence files (VM): `/tmp/tb-evidence-r2/`.
