[🇧🇷 Português](../../../campo/rodada2/playwright-mcp.md) | 🇺🇸 English

# Field study round 2: microsoft/playwright-mcp

Evidence collected on 2026-11-16 in the VM. Local clone at `/tmp/target/playwright-mcp` (shallow). No interaction with the target remote. `tb` binary built from current `main` (`go build -o /tmp/tb ./cmd/tb` → `toolbelt 0.1.0-dev`, `commit: unknown` without ldflags). Raw log: `/tmp/tb-campo2-playwright-mcp.log`. Group: **with existing AI instrumentation**.

## Repository

| Field | Value |
|-------|-------|
| Name | [microsoft/playwright-mcp](https://github.com/microsoft/playwright-mcp) (`@playwright/mcp` 0.0.85) |
| Language | JavaScript/TypeScript (npm wrapper; real MCP server in `microsoft/playwright` / `playwright-core`) |
| Size | ~50 files outside `node_modules`/`.git`/`.toolbelt`; ~564 KiB tree (du); ~4.9 kLOC in `.js`/`.ts`/`.md`/`.json` counted in the clone |
| Existing AI instrumentation | `AGENTS.md`; `.claude/skills/release.md`; workflows `.github/workflows/agent-{triage,review,task,roll,release}.yml` |
| Toolchain | **OK:** Node 22, `npm ci`, system Chrome + Playwright Chromium. **Not exercised:** WebKit, multi-project Firefox, `dtest`/Docker, roll from `../playwright`. `npm run build` is a no-op (`echo OK`). |

## Chosen task

| Field | Value |
|-------|-------|
| What | Align `AGENTS.md` with the real projects (`--project=chrome`) and add regressions in `tests/cli.spec.ts` for `cli.js --version` / `--help` |
| Why | Local bug in this package (does not need the upstream monorepo); the `--project=chromium` example in `AGENTS.md` conflicts with `playwright.config.ts` (only `chrome` + optional `chromium-docker`) and already caused ESCALATE in round 1 |
| Issue | No dedicated issue; contrast with [#1861](https://github.com/microsoft/playwright-mcp/issues/1861) (README env vars) and [#1864](https://github.com/microsoft/playwright-mcp/issues/1864) (upstream `browser_close`) |
| Completed locally? | **Yes** on the clone (local commit `c921ce1`); **not** sent upstream |
| Native result | `npx playwright test tests/cli.spec.ts --project=chrome`: **3 passed**. `npm run lint`: **OK**. `npm run build`: **OK** |
| Round 1 (comparison) | Fix of `update-readme.js` + README regeneration + env-var regression (issue #1861). This round used a **different** task. |

## Verification of the 12 fixes

| # | fix | status | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Clone/copy with basename `target` (`/tmp/skipdirs-probe/target`): `tb graph --enable rebuild` → **130 nodes / 408 edges** (not 0). Also `cwd=/tmp/target`: **126 nodes** |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | Config `hidden_test_cmd: echo HIDDEN_RAN && exit 0` without a `tests/hidden/` folder; `tb verify run --ruler tests --json` → `"hidden_available": true`, `"hidden_passed": true` |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | With `package.json` violation: `tb guard check 'HEAD~1..HEAD' --strict --json` → **EXIT=1** + JSON with violations. Without violation: **EXIT=0** |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `package.json`; diff touching only `package.json` → `violations: [{type:protected_path,file:package.json}]`. Clean diff → `violations: null` |
| 5 | init toolchain detection | **worked** | `tb init` in a repo with `package.json` → `verify.test_cmd: npm test` (no longer `go test ./...`) |
| 6 | review `tests/hidden` by path | **worked** | Commit that only **mentions** `// note: do not touch tests/hidden` in `tests/cli.spec.ts` → `tb review` **findings: []**, EXIT=0 (in R1 it was a critical FP) |
| 7 | bench `default_agent` and `compare --agent` | **worked** (with caveat) | `bench.default_agent: echo AGENT_OK {prompt}` + `-t/-f` → run without `--agent/--fake` EXIT=0; without default → clear error. `tb bench compare --config-a/--config-b --agent 'echo CMP {prompt}'` EXIT=0. **Caveat:** absolute `bench.tasks_dir` in YAML is **not** applied by the CLI (still `bench/tasks`) |
| 8 | review/drift `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | Ticket with `## Escopo` + bullets + `` `**/*.spec.ts` ``: in-scope drift/review EXIT=0; out-of-scope `index.js` change → `out_of_scope: [index.js]`; glob `**/*.spec.ts` accepts `tests/cli.spec.ts`; `**/*.md` rejects the `.spec.ts` |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Config with `build_cmd: npm run build`, `lint_cmd: npm run lint`; verify JSON → `build_ran/passed` and `lint_ran/passed` true, verdict PASS |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** | Init recorded `sources: AGENTS.md` + hints (`npm ci`, `npm run lint`, …) in `.toolbelt/agent_docs.yaml`; `--write-agents` **did not** overwrite existing AGENTS.md. In an empty dir: seeded a minimal AGENTS.md pointing at tb |
| 11 | graph Python `calls` | **not applicable** | Repo without `.py`; JS indexed `edges_calls: 208` (JS indexer already existed) |
| 12 | verify visual skipped + CLI docs | **partial** | Missing manifest → `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` (**ok**). `screens: []` → still `"skipped": false`, `"screens": null`, PASS (**not ok**; `aggregateVisualSkipped` returns early if `len(screens)==0`). `docs/cli.md` more aligned with the real CLI than in R1 |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK (~5 ms); detects npm; reads AGENTS.md | helped | `test_cmd: npm test`; extracted hints; still suggests “multi-project” `npm test` copied from the old AGENTS.md (before the fix) |
| `tb init --write-agents` | OK; does not overwrite | helped | With AGENTS.md present: preserves; empty probe: creates Toolbelt stub |
| `tb mem init/list/add/check` | OK | neutral→helped | Local mandate on package ownership; `mem check` OK |
| Ticket / Scope (`rd.md`) | OK via Markdown `## Escopo` | helped | Lists + `**/*.spec.ts` worked; `rd.md` out of scope if not listed (expected) |
| `tb graph --enable rebuild/stats/deps/impact` | rebuild 130 nodes / 408 edges | helped | SkipDirs OK on root `target`; `deps --file cli.js` listed `playwright-core`; `search install-browser` → `null`; `impact --file cli.js` → `null` |
| `tb verify run` (tests) | PASS with build+lint+hidden | helped | Runs `build_cmd`/`lint_cmd`/`test_cmd`/`hidden_test_cmd`; 3 green chrome tests |
| `tb verify run` (visual) | PASS skipped / PASS not-skipped | neutral→hindered | Missing: correct skip; `screens: []`: `skipped=false` |
| `tb review` | PASS on the fix with full scope; substring FP gone | helped | Mentioning `tests/hidden` does not create a finding; Markdown scope OK |
| `tb drift` | OK in scope; flags OOS and globs | helped | `index.js` out; `**/*.spec.ts` vs `**/*.md` discriminate |
| `tb guard check` | OK; reads `guard.yaml`; `--strict --json` exit 1 | helped | `base..head` syntax (not `--base`); UX still confuses people coming from R1 |
| `tb bench run/list/compare` | OK with `-t/-f` + default_agent / `--agent` | neutral→helped | No local tasks on the target, `-t` required; YAML `tasks_dir` ignored by CLI |
| `tb mcp` | tools/list OK | helped | JSON-RPC: `tb_version`, `tb_graph_*`, `tb_echo`, … |
| `tb version` | `0.1.0-dev` / commit unknown | neutral | `go build` without ldflags |

## Problems that persist

| problem (R1 / synthesis) | status this round | evidence |
|--------------------------|-------------------|----------|
| `version` → `unknown` without ldflags | persists | `tb version` → `commit: unknown` |
| Config files mode `0600` | not deeply re-tested | init created `.toolbelt/`; `stat` not measured this round |
| Visual `skipped=false` with empty manifest | **persists** | `screens: []` → skipped false |
| Hardcoded guard list still aborts verify on thresholds | not re-tested | focus on custom `guard.yaml` |
| `graph impact`/`search` sometimes empty | persists (partial) | `impact`/`search install-browser` → null despite useful deps |
| tb does not push completing the task alone | persists (observational) | No `tb ticket`/board; agent still writes `rd.md` by hand |
| Residual docs/CLI drift | partial | Guard uses `base..head`; trying `--base` fails (`unknown flag`) |
| Init hints echo outdated AGENTS.md | persists until a human fixes AGENTS | Before the fix, hints included `--project=chromium` |

## New problems

| ID | sev | repro | expected | actual |
|----|-----|-------|----------|--------|
| N1 | P2 | In `.toolbelt/config.yaml` set absolute `bench.tasks_dir` / `fixtures_dir` pointing at the toolbelt harness; run `tb bench run --task doc-function` without `-t` | CLI uses YAML paths | Log shows `tasks_dir: bench/tasks` and fails `open bench/tasks/...`; only `default_agent` seems read from config |
| N2 | P2 | `tb verify run --ruler visual` with `.toolbelt/screens.yaml` = `screens: []` | `skipped=true` + reason “no screens” (aggregation) | `skipped=false`, `screens: null`, PASS (`aggregateVisualSkipped` returns if len==0) |
| N3 | P3 | `tb guard check --base HEAD~1 --head HEAD` (review/drift-style flags) | Either accept the flags, or cross-link docs/help | `unknown flag: --base`; requires `HEAD~1..HEAD` |
| N4 | P3 | Ticket with `## Escopo` that omits `rd.md`, commit that adds the ticket | Review/drift do not treat the ticket as noise, or document exclusion | `rd.md` always `out_of_scope` if not listed |

## What tb caught and what it let slip

**Caught**
- Scope drift/review with `## Escopo` + `**` globs.
- Guard via `guard.yaml` on `package.json`.
- Verify with build+lint+test+hidden configured (PASS only when the whole chain passes).
- Init npm toolchain + import of hints from `AGENTS.md`.
- Graph deps of `cli.js`; SkipDirs does not zero the index on root `target`.
- Review without substring FP on `tests/hidden`.

**Let slip**
- AGENTS.md ↔ `playwright.config.ts` inconsistency **before** the fix (tb imported wrong hints instead of validating them against real config).
- Empty visual still “PASS” without skipped.
- YAML `bench.tasks_dir` (agent may think the harness is configured and then fails).
- Missing CLI `--version`/`--help` coverage before the new tests (no docs↔binary ruler beyond what a human configured).

**Native vs tb:** native (`cli.spec.ts` chrome + lint) validated the fix; tb verify mirrored it well once `build_cmd`/`lint_cmd`/`test_cmd` were filled — clear improvement vs R1 (only `test_cmd`).

## Interaction with existing instrumentation / onboarding

- `tb init` **now reads** `AGENTS.md` and writes `agent_docs.yaml` + command hints (improvement vs R1, which ignored it).
- Still **does not validate** hints against the real toolchain: echoed `--project=chromium` / multi-browser list from AGENTS.md while only project `chrome` exists.
- `--write-agents` respects an existing file (correct for this “with instrumentation” group).
- Skills `.claude/skills/release.md` and `agent-*` workflows remain outside vault/mem.
- R1 conflict “AGENTS.md says chromium” was mitigated in the task (we edited AGENTS.md); tb alone would not have fixed it.

## Comparison with round 1

| dimension | R1 | R2 | verdict |
|-----------|----|----|---------|
| Init Go defaults | hindered | detects `npm test` | **better** |
| AGENTS.md | ignored | read + hints | **better** (still echoes file errors) |
| Verify build/lint | did not exist | `build_ran`/`lint_ran` | **better** |
| hidden without dir | did not run | runs if `hidden_test_cmd` set | **better** |
| guard.yaml | ignored | real violations | **better** |
| guard `--strict --json` | exit 0 with violation | exit 1 | **better** |
| review FP `tests/hidden` | confirmed | refuted | **better** |
| Scope `## Escopo` + `**` | fragile / glob failed | worked | **better** |
| SkipDirs root `target` | 0 nodes (other reports) | 130 nodes | **better** |
| default_agent / compare `--agent` | broken | works | **better** |
| Visual empty screens | skipped=false | skipped=false | **same** |
| Completeness / ticket CLI | absent | absent | **same** |
| Local task | completed (#1861) | completed (AGENTS+cli tests) | **same** (success) |

Overall: **clearly better** for the init→scope→verify→guard→review loop; serious R1 regressions among the 12 fixes did not reappear, except the visual `screens: []` hole and YAML `bench.tasks_dir`.

## Improvement suggestions

1. **[P1][bench]** Honor `bench.tasks_dir` / `fixtures_dir` / `output_dir` from `config.yaml` (today only `default_agent` is read reliably).  
2. **[P1][verify/visual]** Treat a manifest with `screens: []` as `skipped=true` (adjust `aggregateVisualSkipped` for len==0).  
3. **[P2][init]** Validate AGENTS.md hints against `playwright.config.*` / real scripts (warn if `--project=X` does not exist).  
4. **[P2][guard/docs]** Accept `--base`/`--head` as aliases of `base..head`, or document only the positional form in every example.  
5. **[P2][review/drift]** Auto-include the `--ticket` path in scope, or an explicit warning.  
6. **[P3][graph]** Improve textual search in JS (`install-browser` string in `cli.js` did not index).  
7. **[P3][mem/onboarding]** Import `.claude/skills/*` as optional playbooks.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Wall time (log timestamps) | ~12:36:12–12:38:43 UTC (~2.5 min of timed commands; excludes earlier `npm ci`/browser install) | ~3.4 min timed |
| Sum of `tb-*` durations in the log | ~3.5 s (dominated by verify tests ~3.1 s) | ~41 s |
| `tb` invocations in the log | 49 | 43 |
| `tb` failures (exit ≠ 0) | 18 (includes expected probes: UX guard flags, OOS drift, bench without `-t`, review without rd in scope, etc.) | 14 |
| Local task | completed (3 chrome tests + lint) | completed (2 tests + lint) |
