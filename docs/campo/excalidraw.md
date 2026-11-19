🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/excalidraw.md)

# Field study: excalidraw/excalidraw

Study conducted on 2026-11-15 with `tb` compiled from `toolbelt@c8aae04` (`make build` → `./bin/tb`). Local clone at `/tmp/target/excalidraw` (shallow). **No** interaction with the Excalidraw remote (no push, issue, PR, or comment). Raw evidence in `/tmp/campo-evidence/` on the agent VM.

## Repository

| Field | Value |
|-------|-------|
| Name | [excalidraw/excalidraw](https://github.com/excalidraw/excalidraw) |
| Language | TypeScript / React (Yarn workspaces monorepo) |
| Size | ~925 relevant source files; ~293k approximate LOC; shallow checkout ~93 MiB (with `node_modules` ~1.1 GiB) |
| Study base commit | `82190fb` (shallow `master`) |
| Existing AI instrumentation | `AGENTS.md` (4 lines: DOM/`ownerDocument`, prefer `Merge<>`); `CLAUDE.md` (monorepo structure, commands `yarn test:typecheck` / `yarn test:update` / `yarn fix`) |
| Toolchain | **Configured**: Node 22, Yarn 1.22.22, `yarn install --frozen-lockfile` OK, Vitest 3.0.6, `tsc`, ESLint. Headless Chrome present (`/usr/local/bin/google-chrome`). |
| Not configured / not run | Full app in browser (`yarn start` / Playwright UI E2E); production app build (`yarn build`) — outside math task scope; full `yarn test:all` suite (typecheck+eslint+prettier+app) — typecheck and eslint on touched files were run; Playwright browsers not installed beyond system Chrome. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `normalizeRadians` to return `0` (not `2π`) on negative multiples of `2π` and tiny negative angles; tests in `packages/math/tests/angle.test.ts`; impact on `cartesian2Polar` / `radiansBetweenAngles`. |
| Why | Small bug, isolated in `@excalidraw/math`, with clear repro and unit tests — suitable size for an E2E field study. |
| Issue | https://github.com/excalidraw/excalidraw/issues/12599 |
| Completed? | **Yes, locally** (local commit `16115e1` on the clone; **not** sent upstream). Fix + 6 tests passing. |
| Native checks | `yarn vitest run packages/math/tests` → 10 files / 64 tests PASS (~2.8 s). `yarn test:typecheck` on the task commit → PASS (~13.6 s). ESLint on touched files: 1 Prettier warning on the first test draft (`(-1e-17)`); after local adjustment → PASS. |

Fix applied in `packages/math/src/angle.ts`:

```ts
export const normalizeRadians = (angle: Radians): Radians => {
  const twoPi = 2 * Math.PI;
  const normalized = ((angle % twoPi) + twoPi) % twoPi;
  if (normalized === 0 || Math.abs(normalized - twoPi) < PRECISION) {
    return 0 as Radians;
  }
  return normalized as Radians;
};
```

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb version` (`make build` binary) | `toolbelt c8aae04`, commit/build filled | neutral | build with ldflags |
| `go run ./cmd/tb version` | `0.1.0-dev`, `commit: unknown`, `built: unknown` | hindered (useless version without ldflags) | `23-version-norflags.txt` |
| `tb init` | created `.toolbelt/` + `config.yaml` mode `600`; Go defaults (`verify.test_cmd: go test ./...`) | hindered on JS onboarding (wrong defaults for the repo) | wall 0.007 s; `stat` → `600 .toolbelt/config.yaml` |
| `tb mem init` / `add` / `list` / `check` | OK; `mem add <regra> --category`; `check` sha256 + retention 1.00 | helped (fast invariants) | `02`–`05`; `mem add --type/--title` (docs) → `unknown flag: --type` |
| Ticket / Scope (manual `rd.md`) | `tb ticket` **does not exist**; Scope via YAML frontmatter in `rd.md` | hindered (docs promise `tb ticket`; real flow is manual file) | `33-ticket-missing.txt`; `06-ticket.txt` |
| `TB_GRAPH=on tb graph rebuild --enable` | 4452 nodes, 18465 edges, ~1.05 s | helped | `07-graph-rebuild.txt` |
| `tb graph search "normalizeRadians"` | found `func:...normalizeRadians` + `rd.md` section | helped | `08-graph-search.txt` |
| `tb graph impact --file packages/math/src/angle.ts` | listed `angle.ts` and `point.ts` | helped partially (did not list consumers in `packages/element`) | `09-graph-impact.txt` |
| `tb graph impact` without `--file` | error `--file é obrigatório` | hindered | `09b-graph-impact-nofile.txt` |
| `tb graph callers --function normalizeRadians` | only returned the file itself | hindered (false negative for cross-package callers) | `11-graph-callers.txt` |
| `tb graph deps --file ...` | imports `./types`, `./utils` | neutral | `10-graph-deps.txt` |
| `tb graph stats` | counts OK | neutral | `12-graph-stats.txt` |
| `tb verify run` (vitest math) | `PASS` with `hidden_available: false` / `hidden_passed: false` | hindered (PASS without real hidden suite; Go defaults) | `17-verify.txt` wall 2.9 s |
| `tb verify` + pure type error | `PASS` (vitest ignores type error); `tsc` fails | hindered | `29g` PASS vs `29h` TS2322 |
| `tb verify --ruler visual` | without manifesto: `skipped:true` PASS; with manifesto without refs: **top-level `skipped:false`** and screen `skipped:true` | hindered (lying top-level flag) | `22`, `22c` |
| `tb review` | scope findings + **false positives** on `tests/hidden` and `thresholds.yaml` invariants | hindered | `19-review.txt` |
| `tb drift` | detected `.toolbelt/*` and `rd.md` out of scope; exit 1 | helped | `18-drift.txt` |
| `tb guard check` | no violations on task diff; with hardcoded paths → violations; **exit 0** even with violations (without `--strict`) | neutral/hindered | `20`, `21b` |
| `tb guard` + `.toolbelt/guard.yaml` | manifesto ignored (`angle.ts` path “protected” in yaml did not yield violation) | hindered | `20b-guard-yaml.txt` |
| `tb bench run` without `--agent`/`--fake` | required error; **ignores** config `bench.default_agent` | hindered | `24-bench-default-agent.txt` |
| `tb bench run --fake` | on Excalidraw fails (no `bench/tasks`); on toolbelt runs without `--fake-config` | neutral | `25`, `25b` |
| `tb bench compare --agent ...` | `unknown flag: --agent` | hindered | `26b` |
| `tb mcp` + `tools/list` | stdio server OK; tools graph/version/echo/env/run | helped (smoke) | `28-mcp-list.txt` |
| Parallel native checks | vitest/tsc/eslint as above | helped (real baseline) | `14`–`15`, `30`–`32` |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare --agent "echo hi"` → `unknown flag: --agent` (`26b`) |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` with `protected_paths: [packages/math/src/angle.ts]`; guard without violation on that path (`20b`) |
| outdated CLI docs | **confirmed** | `docs/cli.md` documents `tb mem add --type/--title` and `tb ticket`/`tb board`; real CLI: `mem add <regra>` and no `ticket`/`board` |
| version shows `unknown` | **confirmed** (without ldflags) | `go run ./cmd/tb version` → commit/built `unknown`; `make build` fills |
| config files created 0600 | **confirmed** | `config.yaml` and `invariants.yaml` mode `600` |
| visual ruler reports `skipped=false` when skipping | **confirmed** | with `screens.yaml` and missing ref: top-level `"skipped": false`, screen `"skipped": true` (`22c`) |
| `default_agent` ignored | **confirmed** | config with `default_agent: echo default-agent-used`; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | diff with `.toolbelt/thresholds.yaml` / `tests/hidden/*` → verify `FAIL` iterations 0 (`21c`) |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake` on toolbelt repo completed 9 tasks without `--fake-config` (`25b`) |
| `mem add` positional | **confirmed** | usage `tb mem add <regra>`; docs flags `--type` nonexistent |
| Drift/Scope need literal paths (no globs) | **partially refuted** | `packages/math/**` and `packages/math/tests/*` work in `inScope` (`18b`, `18c`); implementation accepts `/**` and `filepath.Match` |
| `graph impact` needs `--file` | **confirmed** | without flag → required error |
| review false positives on the tests/hidden invariant | **confirmed** | review critical on `tests/hidden` / `thresholds.yaml` invariant only because the text appears in the `invariants.yaml` diff (`19`) |
| verify runs only partial checks and misses build errors | **confirmed** | type error `number = "not-a-number"`: vitest PASS + verify PASS; `tsc` fails (`29g`/`29h`). Defaults are still `go test`. |
| tb does not push the agent toward completing the whole task | **confirmed** (observational) | init did not suggest typecheck/eslint/prettier from CLAUDE.md; verify PASS without hidden and without `tsc`; local commit included `.toolbelt/*` and `rd.md` outside Scope without tb blocking “done”; agent had to discover native checks alone |

## New problems

### P1 — `verify` gives `PASS` with `hidden_available: false`

- **Repro**: `tb verify run --test-cmd "yarn vitest run packages/math/tests" --hidden-test-cmd "yarn vitest run packages/math/src/angle.ts"` (or default Go hidden in JS repo).
- **Expected**: fail or mark incomplete if the hidden suite does not run.
- **Actual**: `"verdict":"PASS","hidden_passed":false,"hidden_available":false` (`17-verify.txt`).

### P1 — `guard check` exit 0 with violations

- **Repro**: alter `tests/hidden/*` or `.toolbelt/thresholds.yaml`; `tb guard check base..head --json`.
- **Expected**: exit ≠ 0 on `SeverityError` (or require `--strict` documented as the safe default).
- **Actual**: JSON with violations + exit 0 without `--strict` (`21b`).

### P2 — Weak `graph callers` / `impact` in TS monorepo

- **Repro**: index Excalidraw; `tb graph callers --function normalizeRadians` / `impact --file packages/math/src/angle.ts`.
- **Expected**: callers in `packages/element/src/resizeElements.ts` and `sizeHelpers.ts` (real imports).
- **Actual**: callers only the definition file; impact only `point.ts` (`11`, `09`).

### P2 — `tb init` does not adapt verify/bench to the repo ecosystem

- **Repro**: `tb init` in Yarn/TS monorepo.
- **Expected**: detect package manager / scripts (`test`, `typecheck`) or ask; read `AGENTS.md`/`CLAUDE.md`.
- **Actual**: `verify.test_cmd: go test ./...`; zero mention of existing instrumentation.

### P3 — Cobra error UX prints Usage after JSON

- **Repro**: `tb drift --json` / `tb review --json` with drift.
- **Expected**: only JSON + exit code under `--json`.
- **Actual**: valid JSON followed by `Error:` + Usage (`18`, `19`).

### P3 — `tb help` lists `router` as a command despite “not implemented”

- Neutral for the study; creates false expectation on onboarding.

## What tb caught and what it let slip

**Caught**

- Scope drift (`.toolbelt/config.yaml`, `.toolbelt/invariants.yaml`, `rd.md` outside Scope).
- Hardcoded protected paths (`tests/hidden/*`, `.toolbelt/thresholds.yaml`) and verify abort in that case.
- Function location via `graph search` / fast monorepo indexing.
- MCP smoke (`tools/list`).
- Review reported files out of scope (besides invariant FPs).

**Let slip**

- Pure TypeScript error when `test_cmd` is only Vitest (verify PASS).
- Absence of hidden suite (`hidden_available: false` + PASS).
- Prettier/ESLint warning on the new test (verify did not run eslint).
- Real consumers of `normalizeRadians` outside `packages/math`.
- Content of `AGENTS.md` / `CLAUDE.md` (not read on init/onboarding).
- Custom `.toolbelt/guard.yaml` manifesto.
- Task completeness beyond the configured `test_cmd` (app typecheck/lint/build).

## Interaction with existing instrumentation / onboarding

- **`tb init`**: creates only `.toolbelt/`; **does not read** `AGENTS.md` or `CLAUDE.md`. Internal docs (`achados.md` / architecture) list “AGENTS.md as local context” as **pending** (`tb ctx` still does not exist in the CLI).
- **Graph**: indexes `AGENTS.md` as `file`/`section` (search `"AGENTS"` finds the file), but that **does not** inject the rules into the agent’s mandate/context.
- **Duplication / conflict**: default `tb mem init` invariants (`tests/hidden`, `thresholds.yaml`) are generic Go/tb and **do not** mirror Excalidraw rules (`ownerDocument`, `Merge<>`, `yarn test:update`). Noise risk (already seen in review FPs) without local gain.
- **What tb should import**: (1) verification commands from `CLAUDE.md` / `package.json` scripts → `verify.test_cmd` + checklist (typecheck, eslint, prettier); (2) `AGENTS.md` bullets → immutable `mandate` invariants; (3) monorepo structure (workspaces) for graph/impact defaults; (4) neither overwrite nor silently ignore pre-existing instrumentation.

## Improvement suggestions

1. **[P0 · verify]** Do not emit `PASS` if `hidden_available=false` (or require `--allow-no-hidden`). Include `js/ts` profile with `test`+`typecheck`+`lint` from `package.json`/`CLAUDE.md`. *(area: verify)*
2. **[P0 · init / mem]** Onboarding that imports `AGENTS.md`/`CLAUDE.md`/`CONTRIBUTING.md` into invariants and `verify.*`. *(area: init, mem/ctx)*
3. **[P1 · guard]** Respect `.toolbelt/guard.yaml`; fail by default on error violations; document `--strict`. *(area: guard)*
4. **[P1 · review]** Match invariants by touched path / AST, not substring in the diff (eliminates `tests/hidden` FP). *(area: review)*
5. **[P1 · docs/cli]** Sync `docs/cli.md` and README with the real CLI (positional `mem add`; ticket/board as not implemented; bench compare flags). *(area: docs)*
6. **[P1 · visual]** Top-level `skipped=true` if all screens skip; do not report “clean” PASS with `skipped:false`. *(area: verify/visual)*
7. **[P2 · bench]** Honor `bench.default_agent`; accept `--agent` on `bench compare` (or document only `--fake`/JSONL). *(area: bench)*
8. **[P2 · graph]** Improve TS path-alias / workspace import resolution for cross-package callers/impact. *(area: graph)*
9. **[P2 · workflow]** “Done” checklist (clean Scope + verify + native typecheck/lint + clean drift) to push task completeness. *(area: router/verify — aligned with A/B completeness finding)*
10. **[P3 · ux]** Under `--json`, do not print Cobra Usage; less aggressive config permissions than `0600` when the vault has no secrets. *(area: cli/config)*

## Metrics

| metric | value |
|--------|-------|
| Summed wall time of timed commands (tb + native checks in the log) | ~49 s |
| tb invocations (approx., including known-issue probes) | ~45–55 |
| Observed tb failures / exits ≠ 0 | drift/review with drift; guard+verify on protected paths; bench without agent; compare `--agent`; poorly invoked impact/deps/callers; visual without skip; docs `mem add` flags; nonexistent `ticket` |
| “Product” vs usage failures | mostly expected from known bugs; incomplete graph callers and verify PASS without hidden/typecheck are the most serious for an agent |
| Graph rebuild | ~1.05 s / 4452 nodes / 18465 edges |
| Verify (vitest math, task OK) | ~2.9 s, PASS |
| Native vitest math | ~2.8 s, 64 tests PASS |
| `tsc` (task OK) | ~13.6 s, PASS |
| MCP `tools/list` | ~0.007 s, OK |
| Fake bench (on toolbelt repo, 9 tasks × 1) | ~4.0 s, 33.3% success (fake baseline, not Excalidraw) |

**Methodological note:** task “completeness” was reached by the agent consulting Excalidraw’s native instrumentation; tb alone (init → verify with Vitest `test_cmd`) would have declared PASS even with broken typecheck and without lint — aligned with the known problem of not pushing the agent to close the whole task.
