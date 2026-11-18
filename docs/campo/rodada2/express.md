🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/express.md)

# Field round 2 — expressjs/express

Group: without existing AI-agent instrumentation.
Local clone only (`/tmp/express-r2` and `/tmp/target` for SkipDirs regression); no push/issue/PR/comment upstream.
`tb` built from this repo on `main` @ `a46c295` (`make build` → `bin/tb`).

## Repository

| Field | Value |
|-------|-------|
| Name | [expressjs/express](https://github.com/expressjs/express) |
| Language | JavaScript (Node.js), CommonJS |
| Version in clone | 5.3.0 (shallow `HEAD` `43fca52`) |
| Size | ~2–3 MiB excluding `node_modules` cleanly; ~68 MiB with deps; 142 `.js` outside `node_modules`; ~18.7 kLOC in `lib/`+`test/*.js` |
| Existing AI instrumentation | None (`AGENTS.md`/`CLAUDE.md`/`.cursorrules`/copilot instructions absent before `tb init --write-agents`) |
| Toolchain | `npm install` OK (Node v22.23.4, npm 10.9.9). Scripts: `npm test` (mocha), `npm run lint` (eslint 8). No production build step. |
| Not configured | Real Redis (`connect-redis`); not needed for the task’s unit tests. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `res.send(DataView)` which recognized the view via `ArrayBuffer.isView()` but converted with `Buffer.from(dataView)` → empty Buffer and `Content-Length: 0`. |
| Why | Real bug, small scope (`lib/response.js` + `test/res.send.js`), maintainer-plausible, with a clear repro and open issue. |
| Issue | https://github.com/expressjs/express/issues/7510 |
| Completed? | Yes, locally (commit only on clone `/tmp/express-r2`, no push). |
| Diff | `isDataView` → `Buffer.from(view.buffer, byteOffset, byteLength)`; 2 new tests (with/without ETag). |
| Native checks | `npm test -- --grep DataView` → 2 passing; full `npm test` → **1292 passing**; `npm run lint` → PASS. Manual repro: before `len 0`; after `len 3 hex 010203`. |
| Round 1 (comparison) | Round 1 used `req.subdomains` with mapped IPv6 host (`[::ffff:127.0.0.1]`, issue #7532). This round is another surface (`res.send` / DataView). |

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph --enable index .` under `/tmp/target` → **452 nodes, 3051 edges** (round 1: 0 nodes). Under `/tmp/express-r2`: 455 nodes / 3054 edges. |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; config/`--hidden-test-cmd` → `hidden_available: true`, `hidden_passed: true`. With cmd `false` → `hidden_passed: false`, verdict `ESCALATE` (3 iters). |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | With `protected_paths: [lib/response.js]`: `--strict --json` printed violation JSON and `EXIT=1`. Without `--strict` + JSON: `EXIT=0` (expected). |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` protecting `lib/response.js` → `protected_path` on the fix diff. Protecting only untouched `lib/request.js` → `violations: null`. |
| 5 | init toolchain detection | **worked** | `tb init` on Express → `verify.test_cmd: npm test` (no longer `go test ./...`). |
| 6 | review `tests/hidden` by path | **worked** | Ticket `rd.md` mentions `tests/hidden/**` under Forbidden; `--standards invariants` → `findings: []` (no substring FP). |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `tb bench run -r 1` without `--agent` used `default_agent: echo DEFAULT_AGENT_RAN {prompt}` (EXIT=0). `tb bench compare … --agent 'echo COMPARE_AGENT {prompt}'` EXIT=0. |
| 8 | review/drift `## Scope` / lists / `**` | **worked** | Ticket with `## Scope` + bullets + `test/**/*.js`; fix drift without `out_of_scope`. Glob only `test/**/*.js`+`lib/**/*.js` also stayed in-scope. `lib/**` suffix OK. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Verify JSON: `build_ran/passed: true`, `lint_ran/passed: true` with `lint_cmd: npm run lint` and `build_cmd: node -e "require('.')"` . |
| 10 | init reads AGENTS/CLAUDE and `--write-agents` | **worked** | Without files: log “use --write-agents”. With flag: seeded `AGENTS.md`, wrote `.toolbelt/agent_docs.yaml` (`sources: [AGENTS.md]`). No prior CLAUDE.md to read. |
| 11 | graph Python `calls` edges | **not applicable** | JS repo; did not exercise the Python indexer. In JS the graph emitted `edges_calls: 2191`. |
| 12 | aggregated visual skipped + CLI docs | **worked** | Manifest with missing ref → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"` (round 1: `skipped: false`). `docs/cli.md` documents hidden without dir, `## Scope`, `guard.yaml`, `default_agent`, visual skipped. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` (ldflags) | OK — `toolbelt a46c295` | neutral | `EXIT=0 TIME=0.006s` |
| `tb init` | `.toolbelt/`; `test_cmd: npm test`; warns missing AGENTS | **helped** | JS toolchain detected; mode `600` on config |
| `tb init --write-agents` | seeded `AGENTS.md` + `agent_docs.yaml` | **helped** | minimal seed pointing at verify/guard/mem |
| `tb mem init/add/list/check` | 4 invariants; `compactacao_segura: true` | helped | `mem add` still positional only |
| Manual ticket `rd.md` (`## Scope`) | parse OK in drift/review | **helped** | no `tb ticket` (docs: planned Phase 3+) |
| `tb graph index` without `--enable` | ADR-011 error | neutral | expected |
| `tb graph --enable index .` | 455 nodes / 3054 edges | **helped** | also OK under `/tmp/target` (SkipDirs fix) |
| `tb graph search/impact/deps/callers` | search found `res.send*`; impact/deps of `lib/response.js` populated; callers `--function send` → `lib/response.js` | **helped** (better than R1) | R1: empty impact/deps on `lib/request.js` |
| `tb verify run` (npm+lint+build+hidden) | `PASS`; hidden available without dir | **helped** | iters=1; build/lint/visible/hidden true |
| `tb verify --ruler visual` | honest aggregated skip | helped | `skipped: true` with skipped screens |
| `tb review` | EXIT=0; 3 soft completeness warnings | neutral | no `tests/hidden` FP; generic acceptances loosely tied to the diff |
| `tb drift` | EXIT=0 on the fix; EXIT=1 with `Readme.extra.md` out of scope | **helped** | Markdown mandate worked |
| `tb guard check` | clean on the fix; custom yaml and hardcoded thresholds OK | **helped** | `--strict --json` exit 1 with violation |
| `tb bench run` / `compare` | default_agent and `--agent` OK; `--fake` OK | helped | 9 tasks; ~33% success on echo/fake (expected) |
| `tb mcp` | tools/list + `tb_version` OK | helped | 8 tools; graph tools need enable/env |

## Problems that persist

| problem (R1 / checklist) | status this round | evidence |
|--------------------------|-------------------|----------|
| Config files mode `0600` | persists | `config.yaml` and even seeded `AGENTS.md` stayed `600` |
| Missing `tb ticket` | persists | docs/cli marks Phase 3+; flow still needs manual `rd.md` |
| Noisy review completeness | persists | 3 warnings (“etag…”, “Uint8Array…”, “lint…”) without looking at real test evidence |
| Guard does not analyze JS (skip/assert) | persists | only protected paths + Go heuristics; mocha diff does not yield `test_skip` |
| Graph off by default | persists (by ADR-011 design) | `index` without `--enable` fails |
| `mem add` positional only | persists | help/Args require positional `<regra>` |
| Docs still list stubs (`ticket`, `run`, `proc`…) | partial | better aligned (marked planned), but large tree still mixes Phase 3 |

## New problems

### P2 — Seeded `AGENTS.md` with mode `0600`

- **Repro:** `tb init --write-agents` in a repo without AGENTS → `stat` shows `600` on `AGENTS.md`.
- **Expected:** team-readable documentation file (`0644`) or document the choice.
- **Actual:** same permission as sensitive configs; friction on shared clones.

### P3 — Review completeness does not tie acceptance to test names in the diff

- **Repro:** acceptances about ETag/Uint8Array/lint with tests and lint really in the diff → still `warning` “pouco evidenciado”.
- **Expected:** detect `it('…DataView…')` / lint mention in ticket+verify as evidence.
- **Actual:** weak token matching; noise on good PRs.

### P3 — Bench default_agent `echo` marks ~33% success

- **Repro:** `default_agent: echo …` + `tb bench run -r 1` → 33.3% success.
- **Expected:** OK for wiring smoke; but easy to confuse with harness regression if the operator does not notice the agent is a noop.
- **Actual:** “green” harness with a low rate without failing the command.

## What tb caught and what it let slip

**Caught**
- npm toolchain in init; `AGENTS.md` seed when requested.
- Configured hidden suite without a `tests/hidden/` folder.
- Build/lint in the verify pipeline (not only test).
- Drift/scope with Markdown heading + `**` globs.
- Custom guard via `guard.yaml` + strict exit code with JSON.
- SkipDirs at root `/tmp/target` (R1 regression fixed).
- Honest aggregated visual skipped.
- Useful graph impact/deps/callers on `lib/response.js` (JS `calls` present).

**Let slip / did not help**
- Did not discover the bug alone (search for “DataView” empty before the fix; the clue came from the issue).
- Review did not truly validate the DataView acceptances (only soft warnings).
- No `tb ticket`/board to push closing acceptance.
- Guard does not inspect JS patch content (path only).
- Multi-byte typed arrays (`Uint16Array`, issue #7538) stayed out of scope — tb did not suggest widening.

Native comparison: `npm test` (1292) + `eslint` validated the fix end to end; `tb verify` aligned to those commands followed the signal (including hidden grep and lint/build).

## Interaction with existing instrumentation / onboarding

Express **did not** have agent instrumentation. With the round fixes:

| Artifact | Generated/read? | Quality |
|----------|-----------------|---------|
| `.toolbelt/config.yaml` | yes | Correct `test_cmd: npm test`; lint/build/hidden/default_agent edited by hand for the study |
| `AGENTS.md` via `--write-agents` | yes | useful skeleton; mode `600`; empty hints (`hints: []`) |
| `.toolbelt/agent_docs.yaml` | yes | records `sources: [AGENTS.md]` |
| `CLAUDE.md` | n/a | nonexistent; init does not force creation |
| Ticket/Scope | manual `rd.md` | new parser consumed `## Scope` + lists + `**` |
| Guard manifest | yes, honored | effective `.toolbelt/guard.yaml` |
| MCP | yes | stdio OK |

Conclusion: onboarding **improved a lot** vs R1 (no longer assumes Go; can seed AGENTS). Still missing auto lint/build from `package.json` scripts and a first-class mandate/ticket.

## Comparison with round 1

| Dimension | R1 | R2 | Verdict |
|-----------|----|----|---------|
| Init JS | `go test ./...` | `npm test` | **better** |
| AGENTS | did not generate | `--write-agents` works | **better** |
| Graph under `/tmp/target` | 0 nodes | 452 nodes | **better** |
| JS impact/deps | empty (request) | populated (response) | **better** |
| Hidden without dir | ignored | runs | **better** |
| build/lint in verify | inert | honored | **better** |
| guard.yaml / strict JSON | broken | OK | **better** |
| Markdown Scope / `**` | fragile | OK in this run | **better** |
| Visual skipped | lying `false` | aggregated `true` | **better** |
| default_agent / compare `--agent` | broken | OK | **better** |
| Completeness review / ticket CLI / mode 0600 | noisy / absent / 0600 | same | **same** |
| Push closing the task | weak | still weak | **same** |

Overall: **clearly better** on the 12 exercisable fixes; product gaps (ticket, smart completeness, JS analysis in guard) remain.

## Improvement suggestions

1. **[P1][init]** Prefill `lint_cmd`/`build_cmd` from `package.json` scripts (`lint`, `check`) when present — area: init/config.  
2. **[P1][review]** Acceptances: consider `it`/`describe` names in the diff and last verify status as evidence — area: review.  
3. **[P2][init]** Seed `AGENTS.md` with `0644`; secret configs stay `0600` — area: init.  
4. **[P2][ticket]** Minimal `tb ticket` (create/show from issue URL) or `rd.md` template in init — area: ticket/onboarding.  
5. **[P2][guard]** Light multi-language heuristics (skip/assert in JS) or a “Go-only scanner” warning — area: guard.  
6. **[P3][bench]** When `default_agent` is noop/`echo`, warn that the success rate does not measure the target repo — area: bench UX.  
7. **[P3][graph]** Search for issue APIs (`DataView`) before the patch still empty — index comments/identifiers more broadly — area: graph.

## Metrics

| metric | round 2 | round 1 (express.md) |
|--------|---------|----------------------|
| Wall time of timed commands (sum) | ~19.0 s (46 labeled invocations in the log) | ~20.1 s (76 invocations) |
| Approx. instrumented session wall | ~4–5 min incl. npm install, task, tb battery | ~2.5 min cmds + ~15 s setup |
| `EXIT=0` / `EXIT≠0` in R2 log | 40 / 6 (expected negatives: OOS drift, guard strict, graph off, hidden fail) | 54 / 22 |
| Unexpected tb failures | none of the 12 fixes failed where applicable | SkipDirs, hidden gate, Go default, guard.yaml, default_agent, visual skipped |
| Local commits on target (unpublished) | 1 DataView fix (+ experimental commits reverted for drift/guard) | IPv6 fix + experimentals |
| tb binary | `a46c295` | `c8aae04` |

Raw session log: `/tmp/tb-campo2-express.log` (study VM; not versioned).
