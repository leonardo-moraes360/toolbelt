🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/commander-js.md)

# Field study: commander.js

Group: **without AI agent instrumentation** (aside from human policy).  
Local target: `/tmp/target` (shallow clone of https://github.com/tj/commander.js).  
Binary: `tb` built with `make build` at `c8aae04` (`./bin/tb`).  
Date: 2026-11-15.  
Rule: no push/issue/PR/comment upstream; local clone only.

## Repository

| Field | Value |
|-------|--------|
| Name | `tj/commander.js` (npm: `commander` 15.1.0) |
| Language | JavaScript (ESM) + TypeScript typings (`typings/`), tests on Node test runner |
| Size | ~2.2 MiB shallow clone; ~217 versioned files; ~18 k LoC `.js` (incl. examples/tests); native suite **1430** tests (baseline) |
| Agent instrumentation | **Absent** as agent tooling: no `AGENTS.md`, `.cursor/`, `.cursorrules`, Copilot instructions, etc. There is `AI_POLICY.md` (human policy: AI ok with human in the loop; forbids autonomous/mass-generated PRs) + mention in `CONTRIBUTING.md` |
| Toolchain | **OK**: Node `v22.23.4` (≥22.12), npm `10.9.9`, `npm install` (214 pkgs), `npm test` and `npm run check` (type+lint+format) green at baseline. Did not configure extra browser/Chromium for visual ruler beyond Chrome already present at `/usr/local/bin/google-chrome`. |

## Chosen task

| Field | Value |
|-------|--------|
| What | Issue [#2683](https://github.com/tj/commander.js/issues/2683): (1) `Help.groupItems` from `Map` → record (`Object.hasOwn` + `Object.entries`); (2) getter `Command.alias()` typed/documented as possibly returning `undefined`; test covering getter without alias |
| Why | Small bug/compat, clear scope, maintainer-plausible, with typings + test; fits small-to-medium |
| Issue | https://github.com/tj/commander.js/issues/2683 (read-only) |
| Complete? | **Yes, locally** (commit `3383634` on local branch `campo-local-fix` of the clone; **not** sent upstream) |
| Native checks | After the fix: `npm test` → pass (1431 tests); `npm run check` → pass (type/lint/format). Side probes (lint-only / `tests/hidden`) were reverted with `git reset --hard` to the task commit |

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb version` (make build) | OK; version=`c8aae04` | neutral | `EXIT=0` ~6 ms; with Makefile ldflags |
| `go build` without ldflags → `version` | OK; `commit/built: unknown`, `0.1.0-dev` | hindered (DX) | `/tmp/tb-nolink version --json` → `"commit":"unknown","build_time":"unknown"` |
| `tb init` | OK; created `.toolbelt/` | neutral→hindered | `EXIT=0` 5 ms; `config.yaml` mode **600**; default `verify.test_cmd` **`go test ./...`** in JS repo; `default_agent: ""` later edited manually; no AGENTS.md / npm script detection |
| `tb mem init/list/check/add` | OK for real flow (`add <regra>`); doc flags fail | helped (invariants) / hindered (docs) | `mem add "…"` OK; `mem add --type fact …` → `unknown flag: --type`; `check` hash+retention 1.00; file mode **600** |
| Ticket / Scope (manual `rd.md`) | OK | helped | No `tb ticket` in binary; `rd.md` with `base_commit` + literal `scope:`; consumed by drift/review |
| `tb graph` without `--enable`/`TB_GRAPH` | Expected error | neutral | `grafo desabilitado (… ADR-011)` |
| `tb graph --enable rebuild` / `index .` | **0 nodes** | hindered | Clone at `/tmp/target`: `SkipDirs` includes name `"target"` → entire root in `SkipDir` (unit repro: dirs=1 skippedDir=1) |
| `tb graph --enable index lib` | 46 nodes / 631 edges | helped partially | Subdir indexing works; `search alias` / `callers --function alias` / `impact --file …` → `(vazio)` after useful index |
| `tb graph impact` without `--file` | Error | neutral | `--file é obrigatório` (EXIT=1) |
| `tb graph query` | Nonexistent | hindered (docs/expectation) | Falls through to `tb graph` help; subcommand does not exist |
| `tb verify run` (npm config) | PASS when `tests/hidden` absent | helped partially | With `verify.test_cmd: npm test` → PASS ~5.3 s; **`hidden_test_cmd` only runs if `tests/hidden/` dir exists** — otherwise `disponível: false` and ignores the configured command |
| `tb verify` + lint-only probe | Improper PASS | hindered | `npm run check:lint` FAIL (`__campoLintProbe` unused); `tb verify --test-cmd npm test` → **PASS** (does not run eslint) |
| `tb verify` + diff in `tests/hidden` | FAIL iter=0 | helped (anti-cheat) | Built-in guard aborts before tests: `Veredicto: FAIL (iterações: 0)` |
| `tb verify --ruler visual --json` | PASS skipped | neutral | `"verdict":"PASS","skipped":true,"skip_reason":"manifesto de telas ausente"`; Chrome detected |
| `tb review` (task diff) | OK, 0 findings | helped | Fresh=true, 3 goldfish, no drift on declared scope |
| `tb review` (`tests/hidden` probe) | Critical/error findings | helped | Detected out of scope + `tests/hidden/` invariant (true positive) |
| `tb drift` | OK in scope; detects outside | helped | Full scope → OK; `rd-narrow` (typings only) → 4 out of scope, EXIT=1 |
| `tb guard check` | Hardcoded `tests/hidden` yes; `guard.yaml` no | hindered | Custom `.toolbelt/guard.yaml` protecting `lib/help.js` **ignored**; `--manifest` **unknown flag**; touch `tests/hidden/sentinel.js` → `error protected_path …` |
| `tb bench run/list` on target | Fails without local tasks | neutral | `open bench/tasks: no such file`; `--agent` required even with `default_agent` in config |
| `tb bench run --fake` + toolbelt tasks | OK 33.3% | neutral | `-t /agent/repos/toolbelt/bench/tasks` ~3.3 s; **did not** need `--fake-config` |
| `tb bench compare` | A/B `--fake` OK; no `--agent` | neutral/hindered | Compare graph-off/on configs OK; `compare --agent …` → `unknown flag: --agent` |
| `tb mcp` | OK stdio JSON-RPC | helped partially | `tools/list` + `tools/call` `tb_version` responded; graph tools listed with `TB_GRAPH=on` |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `compare` help without `--agent`; `tb bench compare --agent "echo hi" …` → `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmed** | File with `protected_paths: lib/help.js` did not yield violation on diff that alters `lib/help.js`; only hardcoded paths (e.g. `tests/hidden`) |
| Outdated CLI docs | **confirmed** | `docs/cli.md`: `mem add --type/--title`, `tb serve`, `guard snapshot/protect`, manifesto globs; real CLI: `mem add <regra>`, `tb mcp`, only `guard check` |
| version shows `unknown` | **confirmed** (build without ldflags) | `/tmp/tb-nolink version` → commit/built `unknown`. With `make build`, shows hash (not “unknown”) |
| config files created 0600 | **confirmed** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → mode **600** |
| visual ruler reports `skipped=false` when skipping | **refuted** | JSON: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` |
| `default_agent` ignored | **confirmed** | Config with `default_agent: echo agent-default`; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** (for `tests/hidden`) | Diff with `tests/hidden/sentinel.js` → verify FAIL iter=0; `guard check --strict` EXIT=1 |
| default fake agent needs `--fake-config` | **refuted** (happy path) | `tb bench run --fake` with toolbelt tasks completed without `--fake-config`. Flag exists and `fake-agent --config` is optional |
| `mem add` positional | **confirmed** | Usage: `tb mem add <regra>`; docs with vault flags are wrong |
| Drift/Scope need literal paths (no globs) | **refuted** (simple globs) | Code uses `filepath.Match` and `/**`; `scope: lib/*.js` accepted `lib/help.js`/`lib/command.js` (only typings/test stayed out) |
| `graph impact` needs `--file` | **confirmed** | Without flag → `--file é obrigatório` |
| review false positives on tests/hidden invariant | **refuted** in this run | Review of task diff: `findings: []`. Only alerted when actually touching `tests/hidden/` |
| verify runs only partial checks / misses build errors | **confirmed** | (1) native lint-only FAIL + verify PASS with `npm test`; (2) `hidden_test_cmd` ignored without `tests/hidden/` dir; (3) init Go default in npm repo |
| tb does not push agent to complete whole task | **confirmed** (qualitative) | Init does not generate checklist/progress/AGENTS.md nor detect `npm run check`; verify can PASS without lint/format; no nudge of `rd.md` acceptance |

## New problems

1. **P0 — `SkipDirs` includes `target`, so `tb graph index .` / `rebuild` on clone `/tmp/target` indexes 0 nodes**  
   - Repro: `tb graph --enable index .` → `0 nós`; `index lib` → 46 nodes. Test: Walk at root counts `dirs=1 skippedDir=1` because `filepath.Base(/tmp/target)=="target"`.  
   - Expected: skip only Rust/Java build dirs *inside* the repo, not the workspace root.  
   - Actual: root basename in SkipDirs → silent empty graph.

2. **P1 — `hidden_test_cmd` only runs if `tests/hidden/` exists**  
   - Repro: config/`--hidden-test-cmd "npm run check"` with missing dir → `disponível: false`, command never runs, verify can PASS. Creating the dir, `hidden_test_cmd false` → ESCALATE and `HIDDEN_RAN` appears in the log.  
   - Expected: configured command always (or explicit skip warning).  
   - Actual: fixed-path gating hides configured “hidden” suite in JS monorepos.

3. **P2 — `tb graph rebuild` / `index .` silent failure (0 nodes) without error**  
   - Related to P0; UX: EXIT=0 with empty graph, easy to think it indexed.

4. **P2 — JS onboarding: Go defaults + no script detection**  
   - `tb init` writes `go test ./...` / `go test ./tests/hidden/...` in a repo with `package.json` scripts `test`/`check`.  
   - Expected: detect npm/pnpm and fill `verify.*` (and ideally remind about `npm run check`).

5. **P3 — `tb graph search/callers` of little use in JS of this repo**  
   - After indexing `lib`, searches for `alias`/`groupItems` → `(vazio)`; empty impact/deps for central files.  
   - Expected: find indexed methods/exports; actual: low ROI for location.

6. **P3 — Docs still mention `tb serve` / nonexistent guard subcommands**  
   - `tb serve` → `unknown command`; `guard` only has `check`.

## What tb caught and what it let slip

**Caught**
- Scope drift when `rd.md` was too narrow.
- Touch of `tests/hidden/` via `guard` + invariants in `review`.
- Verify running `npm test` end-to-end when configured and without guard violation.
- Graceful visual ruler skip with `skipped: true`.
- MCP responding to `tools/list` / `tb_version`.

**Let slip / did not help**
- Deliberately introduced lint error (`no-unused-vars`) with verify PASS (`npm test` only).
- Config `hidden_test_cmd: npm run check:lint` **not executed** without `tests/hidden/` folder.
- Graph at root of clone `/tmp/target` (0 nodes) — impact/search did not guide the change.
- Custom `guard.yaml` ignored (did not protect `lib/help.js`).
- Task completeness beyond unit tests: typings/README/examples were not demanded by any command beyond what I configured manually in `rd.md` (and review did not check narrative acceptance in a substantial way — 0 findings even for a real change).

**Native checks vs tb**
- Native (`npm test` + `npm run check`) validated the full fix.
- tb verify ≈ `npm test` (+ guard), not ≈ `npm run check`, except for a hack with `tests/hidden/` dir + hidden cmd.

## Interaction with existing instrumentation / onboarding

- Agent instrumentation: **none** of AGENTS.md/Cursor-style files; only `AI_POLICY.md` (policy). tb **neither read nor referenced** that policy.
- `tb init` generated: `.toolbelt/config.yaml`, empty dirs `bench/`, `logs/`, `vault/`.  
- `tb mem init` generated `invariants.yaml` (generic anti-cheat rules).  
- **Did not generate**: AGENTS.md, progress log, feature list JSON, `init.sh`, detection of `npm test`/`npm run check`, ticket template, effective `guard.yaml`.
- Bootstrap quality: enough to create a config folder; **insufficient** for an agent to start “from scratch” in a Node repo without editing `verify.test_cmd` and writing `rd.md` by hand.
- Clear gaps: should detect toolchain (npm scripts), generate AGENTS.md/`rd.md` skeleton, and not assume Go.

## Improvement suggestions

1. **[P0][graph]** Do not apply `SkipDirs` to the basename of the indexed *root*; treat `target`/`build`/`dist` only as children.  
2. **[P0][verify]** Run `hidden_test_cmd` when configured, regardless of `tests/hidden/`; or fail loudly if gating blocks.  
3. **[P1][init/onboarding]** Detect `package.json` scripts → fill `verify.test_cmd` / suggest `npm run check` as hidden or second check; optionally generate minimal `AGENTS.md`.  
4. **[P1][guard]** Honor `.toolbelt/guard.yaml` (or remove from docs); expose `--manifest` if that is the contract.  
5. **[P1][bench]** Respect `bench.default_agent`; add `--agent` to `bench compare` in the mode that runs configs.  
6. **[P2][docs]** Align `docs/cli.md` / README with real CLI (`mcp` not `serve`; positional `mem add`; guard only `check`).  
7. **[P2][verify]** Allow a list of checks (`test`+`lint`+`type`+`build`) to avoid partial PASS.  
8. **[P2][graph]** Warn when 0 nodes after index; improve JS method extraction for search/impact.  
9. **[P3][config]** Revisit umask 0600 if the vault is shared across processes/users on the same team.  
10. **[P3][workflow]** `tb init` / ticket skeleton with Scope+acceptance to push completeness (against the A/B finding of incomplete product).

## Metrics

| metric | value |
|--------|--------|
| Wall time (workflow on target, log `WALL_START`→end of probes) | ~2026-11-15T20:45:54Z → 20:51:41Z (**≈5 min 47 s**); `tb` build + clone/npm install immediately before (~35 s install + build ~6 s) |
| tb invocations recorded in evidence log | **70** `=== CMD ===` blocks parsed (+ MCP stdio and some graph outside the helper) |
| Failures (EXIT≠0) in those blocks | **23** (many intentional for issue repro: docs flags, graph disabled, expected drift, guard, compare `--agent`, bench without tasks) |
| Crashes (panic/segfault) | **0** observed |
| Useful `tb verify` (task PASS) | ~5.3–5.5 s per run with `npm test` |
| `tb bench run --fake` (9 toolbelt tasks) | ~3.3 s; A/B compare ~1.8 s |
| Raw evidence file | `/tmp/tb-campo-evidence.log` (VM; not committed) |
