🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/commander-js.md)

# Field study ROUND 2: commander.js

Group: **without existing AI-agent instrumentation** (no AI agent tooling beyond human policy).  
Local target: `/tmp/commander-js` (shallow clone of https://github.com/tj/commander.js); SkipDirs probe at `/tmp/target`.  
Binary: `tb` built with `make build` at `a46c295` (`./bin/tb`).  
Date: 2026-11-16.  
Rule: no push/issue/PR/comment upstream; local clone only.

## Repository

| Field | Value |
|-------|--------|
| Name | `tj/commander.js` (npm: `commander` 15.1.0) |
| Language | JavaScript (ESM) + TypeScript typings (`typings/`); tests with the Node test runner |
| Size | ~2.9 MiB in the clone (with `node_modules`); **220** versioned files; ~18 k LoC versioned `.js` |
| Agent instrumentation | **Absent** as agent tooling: no `AGENTS.md`/`CLAUDE.md`/`.cursor`/Copilot instructions upstream. There is `AI_POLICY.md` (human policy). After `tb init --write-agents`, the harness generated a local `AGENTS.md` (not sent upstream) |
| Toolchain | **OK**: Node `v22.23.4`, npm `10.9.9`, `npm install` (214 pkgs), baseline `npm test` (**1430** pass) and green `npm run check`. Chrome at `/usr/local/bin/google-chrome` (visual ruler) |

## Chosen task

| Field | Value |
|-------|--------|
| What | Issue [#2603](https://github.com/tj/commander.js/issues/2603): with `process.versions.electron` **and** `ELECTRON_RUN_AS_NODE` set, `parse()` must use the Node convention (`argv.slice(2)`), not Electron packaged (`argv.slice(1)`). Patch in `lib/command.js` + test in `tests/command.parse.test.js` |
| Why | Real bug with minimal repro and patch; maintainer-plausible; clear scope with tests |
| Issue | https://github.com/tj/commander.js/issues/2603 (read-only; *pending release* label upstream — fix still **not** on the shallow-cloned `master`) |
| Complete? | **Yes, locally** (commit `485c4f5` on local branch `campo2-local-fix` of the clone; **not** sent upstream) |
| Native checks | After the fix: `npm test` → pass; `npm run check` → pass. Side probes (lint/`tests/hidden`/`lib/help.js`) were reverted with `git reset --hard` |
| Round 1 (comparison) | Round 1 used [#2683](https://github.com/tj/commander.js/issues/2683) (`Help.groupItems` Map→record + `alias()` typing). This round uses a **different** task (#2603) |

## Verification of the 12 fixes

| # | fix | worked / did not work / not tested / not applicable | evidence |
|---|-----|-----------------------------------------------------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Clone under `/tmp/target`: `tb graph --enable rebuild` → **426 nodes / 2047 edges** (EXIT=0). In round 1 the same path gave 0 nodes |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | Config `hidden_test_cmd: npm run check:lint` without the dir: JSON `"hidden_available": true`, `"hidden_passed": true` and text `Oculto: true (disponível: true)` |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff touching `lib/help.js` (via `guard.yaml`): JSON with `protected_path` + **EXIT=1** |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `lib/help.js`; probe commit → `{"type":"protected_path","file":"lib/help.js",...}` EXIT=1. Hardcoded path `tests/hidden/sentinel.js` also violated |
| 5 | init toolchain detection | **worked** | `tb init` in an npm repo → `verify.test_cmd: npm test` (not `go test`). `hidden_test_cmd` stayed `""` (better than round 1’s Go default) |
| 6 | review `tests/hidden` by path | **worked** | Ticket `rd-mention-hidden.md` cites `tests/hidden`/`forbidden` without touching the dir: finding was only **out of scope** (`tests/command.parse.test.js`), no critical invariant from substring. When **touching** `tests/hidden/sentinel.js`: `[critical] (invariants) … tests/hidden/` |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Config `default_agent: echo agent-default`; `tb bench run -r 1 -t …/bench/tasks` **without** `--agent` → EXIT=0, `agent=external`. `tb bench compare --agent "echo hi" --fake …` → EXIT=0 (~120 ms). `compare` help lists `--agent` |
| 8 | review parses `## Scope`/`## Escopo`, MD lists and `**` | **worked** | `rd.md` with `## Escopo` + bullets `` `lib/command.js` `` / `` `tests/**/*.js` ``: review **without** “no scope declared”; `tests/command.parse.test.js` in scope. `rd-narrow.md` only `typings/**/*.ts` → drift EXIT=1 with `lib/command.js` and `tests/command.parse.test.js` out |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | With both set: `Build: true` / `Lint: true`. Lint probe on `lib/option.js` + `lint_cmd`: verify fails at `build_or_lint` (eslint unused). Without `lint_cmd`: same native lint FAIL + verify **PASS** (opt-out false negative) |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** | Without files: `tb init --write-agents` created `AGENTS.md` (EXIT=0). Re-`init` with AGENTS present: `documentação de agentes encontrada sources=AGENTS.md` + `.toolbelt/agent_docs.yaml`. Flag nonexistent in round 1 |
| 11 | graph Python `calls` edges | **not applicable** | JS repo; JS indexer already emits `calls` (stats: `edges_calls: 1132`). Did not exercise a Python fixture |
| 12 | aggregated visual skipped + CLI docs aligned | **worked** (partial on docs) | `tb verify run --ruler visual --json` → `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. `docs/cli.md` documents `tb mcp` (not `serve`) and `## Scope`; residual examples still show `mem add --category` and `tb mem add --type` is still rejected by the CLI |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `make build` / `tb version` | OK; `a46c295` | neutral | JSON with commit/build_time; ~5 ms |
| `tb init` | OK; `test_cmd: npm test` | **helped** | npm detection; `hidden_test_cmd: ""` |
| `tb init --write-agents` | OK; generated `AGENTS.md` | **helped** | File mode 600; minimal toolbelt text |
| `tb mem init/add/list/check` | OK | helped | `mem check` retention; `mem add --type` still unknown flag |
| Ticket `rd.md` (`## Escopo`) | OK | **helped** | Drift/review consumed heading+bullets+`**` |
| `tb graph` without enable | ADR-011 error | neutral | Expected |
| `tb graph --enable rebuild` (and under `/tmp/target`) | 419–426 nodes | **helped** | Root SkipDirs fixed; useful indexing |
| `tb graph search _prepareUserArgs` | section/ticket hits | neutral/partial | Low ROI for finding the method under `lib/` |
| `tb graph callers --function _prepareUserArgs` / `parse` / `emit` | `(vazio)` | hindered | Despite `edges_calls: 1132`, named callers for the task stayed empty |
| `tb graph query` | Nonexistent (falls to help) | hindered (docs/expectation) | EXIT=0 with help; missing subcommand |
| `tb verify run` (build+lint+test+hidden) | PASS ~22 s | **helped** | JSON: build/lint/visible/hidden true |
| `tb verify` without `lint_cmd` + broken lint | undeserved PASS | hindered (if misconfigured) | Confirms lint is opt-in |
| `tb verify --ruler visual --json` | PASS skipped | neutral | Missing manifest; Chrome detected |
| `tb review --ticket rd.md` | 0 findings on the task | helped | Fresh=true; scope OK |
| `tb review` + touch `tests/hidden` | critical invariant | helped | True positive by path |
| `tb drift` (full / narrow scope) | OK / EXIT=1 | **helped** | Narrow listed 2 files out |
| `tb guard check HEAD~1..HEAD` | OK on the task; FAIL on probes | **helped** | `guard.yaml` + hardcoded |
| `tb guard check --strict --json` | EXIT=1 with violation | **helped** | Valid JSON + exit ≠ 0 |
| `tb bench run` without local tasks | FAIL open bench/tasks | neutral | Expected on the target |
| `tb bench run` with `default_agent` + toolbelt `-t` | OK 11.1% | helped (DX) | Did not require `--agent` |
| `tb bench run --fake` / `compare --agent` | OK | neutral | Fake ~63 ms; compare ~120 ms |
| `tb mcp` (stdio) | OK | helped partial | `tools/list` + `tb_version`; graph tools with `TB_GRAPH=on` |

## Problems that persist

From round 1 (`docs/campo/commander-js.md`), still observed in this run:

| round 1 problem | status now | evidence |
|-----------------|------------|----------|
| Missing `tb graph query` | **persists** | Falls through to `tb graph` help |
| `graph callers`/search little useful for the JS task | **persists** (partial) | Large index, but `callers --function _prepareUserArgs` → `(vazio)` |
| Config mode `0600` | **persists** | `stat`: `config.yaml` and `AGENTS.md` → **600** |
| Old-docs `mem add` flags | **persists** (partial) | `unknown flag: --type`; docs still cite `--category` in examples |
| Verify PASS without lint if `lint_cmd` empty | **persists** (by opt-in design) | Native lint FAIL probe + verify PASS |
| `tb bench list` without tasks on the target | **persists** | `open bench/tasks: no such file` |
| Init does not auto-fill npm lint/build/hidden | **persists** (partial) | Detects `npm test`, but not `npm run check` / type / lint |
| Graph off without `--enable`/`TB_GRAPH` | **persists** (desired ADR-011) | Explicit error |

Round 1 items **fixed** in this binary: SkipDirs+`/tmp/target`, gated `hidden_test_cmd`, ignored `guard.yaml`, `--strict --json` exit 0, Go-default init, dead `default_agent`, parser without `## Scope`, review substring FP `tests/hidden`.

## New problems

1. **P2 — JS `callers`/`impact` still weak for guiding the change despite a populated index**  
   - Repro: after `rebuild`, `tb graph --enable callers --function _prepareUserArgs` → `(vazio)`; `impact --file lib/command.js` only echoes the file itself.  
   - Expected: callers of `parse`/`parseAsync` → `_prepareUserArgs` or useful impact.  
   - Actual: high `edges_calls`, but name resolution for the task fails — low ROI for locating (already noted in R1; remains post-fixes).

2. **P3 — `tb graph query` still suggested by field habits/docs and missing**  
   - Repro: `tb graph --enable query '…'` prints help and EXIT=0.  
   - Expected: subcommand or clear “unknown command” error (EXIT≠0).  
   - Actual: low useful signal.

3. **P3 — Init does not suggest `npm run check` / `build_cmd` / `lint_cmd` from `package.json` scripts**  
   - Repro: `tb init` → only `test_cmd: npm test`; `hidden_test_cmd: ""`.  
   - Expected (post P1-1): at least a hint or fill of lint/check.  
   - Actual: agent still must edit YAML by hand to avoid a false PASS (confirmed in the probe without `lint_cmd`).

4. **P3 — Examples in `docs/cli.md` still show `mem add --category`**  
   - Repro: grep in `docs/cli.md` (~L348, ~L895); real CLI rejects flags like `--type`.  
   - Expected: examples only in the implemented form.  
   - Actual: partial #12 alignment.

5. **P3 — Drift report prints `Base: <declared> → <actualBase>` with the same SHA even when head diverges**  
   - Repro: `tb drift --ticket rd.md` after the task commit → `Base: a5a7dc… → a5a7dc…` while review shows `a5a7dc…..HEAD` and narrow lists commit files.  
   - Expected: show base→head of the analyzed range.  
   - Actual: confuses human reading (file comparison seems to work).

## What tb caught and what it let slip

**Caught**
- Scope drift with `## Escopo` + `**` globs (narrow).
- Custom `guard.yaml` (`lib/help.js`) and hardcoded `tests/hidden/` path.
- Verify with configured `build_cmd`/`lint_cmd`/`hidden_test_cmd` (full-task PASS; FAIL on lint probe).
- Review invariant by real path under `tests/hidden/`.
- SkipDirs at root `/tmp/target` (non-empty graph).
- Bench `default_agent`; MCP `tb_version` / graph tools.

**Let slip / did not help**
- Lint introduced on purpose when `lint_cmd` is not in YAML (verify PASS).
- Locating the function via `callers`/`search` for `_prepareUserArgs` (empty).
- Narrative completeness of ticket acceptance: review 0 findings on the correct change (does not validate Electron behavior).
- Upstream `AI_POLICY.md` was not read/indexed (only `AGENTS.md` generated by tb itself).

**Native checks vs tb**
- Native (`npm test` + `npm run check`) validated the fix.
- tb verify ≈ type + lint + test + lint-as-hidden **when** YAML is filled manually; init alone ≈ only `npm test`.

## Interaction with existing instrumentation / onboarding

- Upstream: **without** AGENTS/Cursor; only `AI_POLICY.md`. tb **did not** reference the policy.
- `tb init`: `.toolbelt/config.yaml` with `npm test`; `bench/`/`logs/`/`vault/` dirs.
- `tb init --write-agents`: created a minimal `AGENTS.md` pointing at verify/guard/mem.
- Re-init: wrote `agent_docs.yaml` with `sources: [AGENTS.md]`.
- `tb mem init`: default anti-cheat invariants.
- Bootstrap: **better than round 1** (toolchain + AGENTS seed). Still requires manual edits of `lint_cmd`/`build_cmd`/`hidden_test_cmd` and ticket `rd.md` for a full adversarial flow.

## Comparison with round 1

| Aspect | Round 1 | Round 2 | Verdict |
|--------|---------|---------|---------|
| SkipDirs `/tmp/target` | 0 nodes | 426 nodes | **better** |
| `hidden_test_cmd` | ignored without dir | runs | **better** |
| `guard.yaml` | ignored | honored + strict/json exit | **better** |
| Init toolchain | `go test` | `npm test` | **better** |
| Markdown Scope | fragile / manual `scope:` | `## Escopo` + `**` OK | **better** |
| Review FP `tests/hidden` | mitigated in practice in R1; code now path-based | confirmed path-based | **better** |
| Bench `default_agent` / `compare --agent` | broken | work | **better** |
| build/lint in verify | absent | honored when set | **better** |
| AGENTS seed/read | absent | `--write-agents` + re-read | **better** |
| JS callers/search for the task | weak | still weak | **same** |
| False PASS without lint in YAML | yes | yes (opt-in) | **same** (mitigable) |
| Workflow wall time | ~5 min 47 s | ~3 min 46 s (12:39:37→12:43:23Z) + prior npm setup | **better** (less rework on P0 bugs) |

Overall: **clearly better** on the 12 fixes; remaining bottlenecks are deep onboarding (auto lint/check), JS callers graph, and acceptance completeness.

## Improvement suggestions

1. **[P1][init]** Fill or suggest `lint_cmd`/`build_cmd`/`hidden_test_cmd` from `package.json` scripts (`check`, `lint`, `typecheck`).  
2. **[P1][graph]** Improve `callers`/`search` resolution for JS class methods (e.g. `_prepareUserArgs`).  
3. **[P2][graph]** Remove or implement `tb graph query`; if absent, EXIT≠0.  
4. **[P2][drift]** Print real `base..head` of the range (not only identical declared→actualBase).  
5. **[P2][docs]** Remove remaining `mem add --category` / dead-flag examples in `docs/cli.md`.  
6. **[P3][verify]** Loud warning when only `test_cmd` is set but `package.json` has `lint`/`check`.  
7. **[P3][init]** Optionally index/warn about `AI_POLICY.md` / CONTRIBUTING when there is no AGENTS.  
8. **[P3][config]** Review mode `0600` if vault/AGENTS are shared on the team.

## Metrics

| metric | value | round 1 |
|--------|-------|---------|
| Wall time (evidenced workflow) | **≈3 min 46 s** (12:39:37Z→12:43:23Z) after tb rebuild + re-init; npm install/baseline beforehand ~47 s | ≈5 min 47 s |
| tb invocations (`=== CMD ===` blocks) | **54** (+ MCP stdio and some stray graph calls) | 70 |
| EXIT≠0 failures in those blocks | **19** (many on purpose: graph off, narrow drift, guard probes, lint, docs flags, bench without tasks) | 23 |
| Crashes | **0** | 0 |
| Useful `tb verify` (full-task PASS) | ~21–23 s with type+lint+test+hidden | ~5.3 s (`npm test` only) |
| `tb bench run --fake` (9 tasks) | ~63 ms (1 rep); compare ~120 ms | ~3.3 s / ~1.8 s |
| Raw evidence file | `/tmp/tb-campo2-commander-evidence.log` (VM; not committed) | `/tmp/tb-campo-evidence.log` |
