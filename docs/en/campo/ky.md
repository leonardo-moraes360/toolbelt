[🇧🇷 Português](../../campo/ky.md) | 🇺🇸 English

# Field study: `tb` on sindresorhus/ky

Evidence collected on 2026-11-15 on a local clone at `/tmp/target` (no push, issue, PR, or comment upstream). `tb` compiled from this repo (`make build` → `bin/tb`, version `c8aae04`). Product change only in this file; work on ky stayed restricted to the VM.

## Repository

| Field | Value |
|-------|-------|
| Name | [sindresorhus/ky](https://github.com/sindresorhus/ky) |
| Language | TypeScript (ESM), HTTP client over Fetch |
| Size | ~116 files outside `node_modules`/`.git`/`distribution`; ~1.9 MiB; ~4.9 kLOC in `source/**/*.ts`, ~30.9 kLOC in `test/**/*.ts` |
| AI instrumentation | `AGENTS.md` (`null` vs `undefined` convention); no `CLAUDE.md` / `.cursorrules`; CI in `.github/workflows/main.yml` |
| Toolchain | **Yes, with caveats.** Node v22.23.4 + npm 10.9.9; `npm install` + `npm run build` OK; Playwright Chromium/Firefox/WebKit installed; `xo` OK (pre-existing warnings); non-browser `ava` suite 1528 passed after the change. Chromium sample of `test/browser.ts`: 23 passed. Did not run the full firefox+webkit+all-browsers matrix on the final `npm test` (costly); on the first `npm test` before installing all browsers there were 41 failures from missing Playwright binaries. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Reject invalid returns from `retry.jitter` (function) with `TypeError` (`must return a non-negative finite number`), chaining the original error as `cause`; update Absolute jitter examples to `Math.max(0, ...)`; tests in `test/retry.ts` and `test/retry-decisions.ts`. |
| Why | `retry.delay` already rejects NaN/negative; `retry.jitter` silently fell back to the delay without jitter (hides misconfiguration). Aligns with `AGENTS.md` (clear error instead of fallback). Maintainer-plausible. |
| Issue | No useful open issue at collection time (`gh issue list --state open` empty). Task derived from code + local convention, not from an issue. |
| Complete? | **Yes, locally** (commit `056a1f0` only on the clone; not sent upstream). |
| Native checks | `npm run build` OK; `xo` on touched files: 2 pre-existing warnings; `tsc --project tsconfig.test.json --noEmit` OK; `ava 'test/!(browser).ts'` → **1528 passed**; Chromium sample OK. |

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb init` | OK; creates `.toolbelt/` + `config.yaml` | neutral | 0.007 s; config born `0600` |
| `tb mem init/list/add/check` | OK with positional; docs flags `--type/--title/--body` fail | helped (invariants) / hindered (docs) | `mem add "…"` OK; `unknown flag: --type` |
| `tb graph index` without `--enable` | ADR-011 error | neutral | `grafo desabilitado` |
| `tb graph index --enable` in `/tmp/target` (cwd) | **0 nodes** | hindered | SkipDirs includes `target`; clone basename |
| `tb graph index --enable source` | 180 nodes / 631 edges | helped | useful TS indexing |
| `tb graph index --enable .` in `/tmp/kyrepo` | 590 nodes / 3804 edges | helped | confirms `target` name bug |
| `tb graph impact --file` | requires `--file`; path relative to indexed root | hindered UX | `source/core/Ky.ts` → empty if root=`source`; `core/Ky.ts` → hit |
| `tb graph search/callers/deps` | search by partial title OK; private `#calculateDelay` not indexed; callers empty | neutral/hindered | private `#` methods missing from the graph |
| tickets / Scope | **no** `tb ticket`; manual `rd.md` with YAML frontmatter | hindered | docs list `tb ticket`; CLI: `unknown command` |
| `tb verify` (tests) | PASS on configured tests; hidden unavailable | helped partial | `visible_passed=true`, `hidden_available=false` |
| `tb verify` (visual) | PASS with screens `skipped=true`, report `skipped=false` | hindered | JSON in `/tmp/tb-evidence/verify-visual.json` |
| `tb review` | exit 1 with invariant false positives | hindered | flags `tests/hidden` and `thresholds.yaml` in the ticket diff |
| `tb drift` | OK with literal paths; shallow globs work | helped | final drift `out_of_scope: null` |
| `tb guard check` | hardcoded OK; `guard.yaml` ignored | hindered config | `source/index.ts` in custom yaml → no violation; `.toolbelt/thresholds.yaml` → violation |
| `tb bench run/compare` | `--fake` OK; `default_agent` ignored; compare without `--agent` | neutral/hindered | `--agent ou --fake é obrigatório` even with `default_agent` in yaml |
| `tb mcp` | starts stdio; `tools/list` and `tb_version` OK | helped | 8 tools (`tb_echo`, `tb_env`, `tb_run`, graph_*, `tb_version`) |
| `tb version` | with ldflags: `c8aae04`; without: `0.1.0-dev` + `commit/built: unknown` | neutral | `/tmp/tb-nolink version` |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | help only has `--fake` / JSONL / A/B configs; no `--agent` |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` with `source/index.ts`; diff on that path → `violations: null` |
| outdated CLI docs | **confirmed** | docs: `tb ticket/run/proc/md/agent`, `mem add --type`; real CLI without those commands/flags |
| version shows `unknown` | **confirmed** (without ldflags) | plain `go build` → `commit: unknown`, `built: unknown` (version `0.1.0-dev`) |
| config files created 0600 | **confirmed** | `tb init` → `.toolbelt/config.yaml` mode `600` |
| visual ruler `skipped=false` when skipping | **confirmed** | report `"skipped": false` with screens `"skipped": true` / `referência ausente` |
| `default_agent` ignored | **confirmed** | yaml with `default_agent: "echo AGENT {prompt}"`; `bench run` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | diff on `.toolbelt/thresholds.yaml` → verify `FAIL`, `iterations: 0`, violation `protected_path` |
| default fake agent needs `--fake-config` | **partially refuted** | `--fake` without config runs (0% success); `--fake-config` optional |
| `mem add` positional | **confirmed** | `tb mem add <regra>`; docs flags `--type/--title/--body` nonexistent |
| Drift/Scope need literal paths (no globs) | **partially refuted** | `test/*.ts` and `source/**/*.ts` (1 level) match via `filepath.Match`; real recursive `**` does not; literals more predictable |
| `graph impact` needs `--file` | **confirmed** | `--file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmed** | critical findings even without touching `tests/hidden/` (text in the diff/`rd.md`) |
| verify runs only partial checks / misses build errors | **confirmed** | TS error in `source/index.ts`: `npm run build` fails; `tb verify` with only `ava` → PASS; default Go hidden path `hidden_available=false` |
| tb does not push agent toward completing whole task | **confirmed** (qualitative) | no `tb ticket`/board/progress; agent builds `rd.md`/verify by hand; review/guard create friction without a completion checklist |

## New problems

### P0 — `SkipDirs` treats the root basename as a directory to skip

- **Repro:** clone into `/tmp/target` (or any folder named `target`/`build`/`dist`/`bin`/…) and run `tb graph index --enable .` → `0 nós`. Same tree in `/tmp/kyrepo` → hundreds of nodes.
- **Expected:** skip only *subdirectories* with those names, never the walk root.
- **Actual:** `info.Name()=="target"` at root → `filepath.SkipDir` on everything.

### P1 — Graph paths are relative to the indexed root, without normalizing the repo cwd

- **Repro:** `tb graph index --enable source` then `tb graph impact --enable --file source/core/Ky.ts` → `(vazio)`; `--file core/Ky.ts` → hit.
- **Expected:** accept repo path or document/echo the root used.
- **Actual:** silence/`(vazio)`.

### P2 — JS indexer does not capture private `#foo` methods

- **Repro:** search/`callers` of `#calculateDelay` / `calculateDelay` after indexing `source` → empty (function exists in `Ky.ts`).
- **Expected:** at least index the name without `#` or document the limitation.
- **Actual:** missing symbol; impact/`callers` useless for modern internal TS API.

### P2 — `tb review`/`drift` only accept `--output`, not `-o` (verify accepts `-o`)

- **Repro:** `tb review … -o out.json` → `unknown shorthand flag: 'o'`.
- **Expected:** consistent short flag or clear docs.
- **Actual:** inconsistency between subcommands.

### P3 — MCP `tools/list` does not mirror verify/review/drift/guard/mem

- **Repro:** `tools/list` → only echo/env/run/graph_*/version.
- **Expected** (current phase): at least tools aligned with the field workflow, or docs stating the cut.
- **Actual:** MCP agent cannot operate the verify/review loop via the protocol.

## What tb caught and what it let slip

**Caught**

- Real drift when `rd.md` was outside Scope.
- Guard on hardcoded path `.toolbelt/thresholds.yaml` and verify abort in that case.
- Review correctly signaled out-of-scope change when the ticket omitted `rd.md`.
- Graph deps of `core/Ky.ts` listed imports after correct indexing.
- Verify (tests) passed the focused jitter suite when the code was right.

**Let slip / hindered**

- Did not read `AGENTS.md` nor enforce the `null` convention.
- Did not index the repo on the default path `/tmp/target` (SkipDirs bug).
- Did not see a TypeScript build error if `test_cmd` ≠ `npm run build` / full `npm test`.
- Default hidden tests (`go test ./tests/hidden/...`) irrelevant to ky → `hidden_available=false` without failing the verdict.
- Review marked `tests/hidden` and `thresholds.yaml` violation only because the text appeared in the ticket diff (false positive).
- Project `guard.yaml` was ignored.
- Visual “passed” without useful pixel comparison (per-screen `skipped`, `skipped=false` in the report).
- Did not push task closure (second Infinity test in `retry-decisions.ts` only appeared in full native `ava`).

## Interaction with existing instrumentation / onboarding

- **`AGENTS.md`:** zero references in tb Go code; only mentioned in research docs (`tb ctx` / pending). tb **ignores** the file.
- **Duplication:** `tb mem init` creates generic invariants (Go/`tests/hidden`/thresholds) that do **not** reflect ky rules and still produce review false positives.
- **Conflict:** default invariants assume Go/toolbelt layout; ky is TS/ava/xo.
- **Onboarding:** `tb init` does not detect npm stack, does not suggest `test_cmd` from `package.json`, does not import `AGENTS.md`, does not create ticket/progress. Default verify config remains `go test ./...`.
- **What tb should import:** `AGENTS.md` content as invariants/`mandate`; `package.json` scripts (`test`/`build`) as `verify.test_cmd` / build step; eventually `xo` as lint ruler.

## Improvement suggestions

1. **[P0][graph]** Do not apply `SkipDirs` to the walk root directory; only to children.  
2. **[P0][verify]** Run (or require) the detected stack’s build command; do not PASS on partial tests alone.  
3. **[P1][mem/onboarding]** On `tb init`, detect Node/Go/etc., fill `verify.*` and import `AGENTS.md` → invariants.  
4. **[P1][guard]** Read `.toolbelt/guard.yaml`; stop hardcoding only three paths.  
5. **[P1][review]** Match invariants by touched path, not by substring in the diff/ticket.  
6. **[P1][docs/cli]** Sync `docs/cli.md` with the real tree; remove or mark “not implemented”.  
7. **[P1][bench]** Honor `bench.default_agent`; on `compare`, accept `--agent` (not only `--fake`).  
8. **[P2][verify/visual]** If any screen skips, `report.skipped=true` (or a distinct SKIP verdict).  
9. **[P2][graph]** Normalize `--file` to the repo root; index `#private` methods.  
10. **[P2][workflow]** Minimal ticket/progress command (even if it only creates `rd.md` + checklist) to push completeness.  
11. **[P3][config]** `0644` for non-secret yaml (or document 0600).  
12. **[P3][mcp]** Expose verify/review/drift/guard/mem in the MCP catalog.

## Metrics

| metric | value |
|--------|-------|
| Wall time (instrumented sum of invocations in the log) | ~66.8 s |
| Wall time `tb*` invocations only (approx.) | ~40.7 s |
| Recorded invocations (tb + native + probes) | 70 |
| tb invocations (approx., excl. native/docs) | 61 |
| tb failures in the log | 14 (almost all intentional probes: docs flags, graph off, blocking drift/review, default_agent, thresholds) |
| Binary crashes | 0 |
| End-to-end study time (clone, npm, playwright, implementation, tb) | ~15–20 min session (not only CLI) |

Raw evidence files (VM): `/tmp/tb-evidence/`.
