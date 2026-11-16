[🇧🇷 Português](../../campo/express.md) | 🇺🇸 English

# Field study — expressjs/express

Group: without AI agent instrumentation in the target repository.
Local clone only (`/tmp/target`); no push/issue/PR/comment upstream.
`tb` built from this repo: `make build` → `bin/tb` (ldflags with commit `c8aae04`).

## Repository

| Field | Value |
|-------|-------|
| Name | [expressjs/express](https://github.com/expressjs/express) |
| Language | JavaScript (Node.js), CommonJS |
| Version on clone | 5.3.0 (shallow `HEAD` `43fca52`) |
| Size | ~2.0 MiB without `node_modules`; 217 source files; ~142 `.js`; ~22 kLOC JS |
| Existing AI instrumentation | None found (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, copilot instructions absent) |
| Toolchain | `npm install` OK (Node v22.23.4, npm 10.9.9). Scripts: `npm test` (mocha), `npm run lint` (eslint 8). No production `build` step. Coverage/`nyc` available via `test-ci`/`test-cov` — not required for the task. |
| Not configured | Real Redis for examples/sessions (devDependency `connect-redis`); not needed for the unit tests used. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `req.subdomains` for IPv6 hosts with dots (e.g. `[::ffff:127.0.0.1]`), which were split into “subdomains” because `net.isIP()` received the literal with brackets and returned `0`. |
| Why | Real bug, small scope (`lib/request.js` + tests), maintainer-plausible, with clear repro. |
| Issue | https://github.com/expressjs/express/issues/7532 |
| Completed? | Yes, locally (commit only on clone `/tmp/target`, no push). |
| Diff | Strip `[…]` before `isIP`; 2 new tests in `test/req.subdomains.js`. |
| Native checks | `npm test -- --grep subdomains` → 13 passing; full `npm test` → PASS (~2.5 s); `npm run lint` → PASS. Manual repro: before `["0","[::ffff:127"]`, after `[]`. |

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb version` (build with ldflags) | OK — `toolbelt c8aae04` | neutral | `EXIT=0 TIME=0.005s` |
| `tb version` (build without ldflags) | `0.1.0-dev`; `commit: unknown`; `built: unknown` | hindered | `/tmp/tb-nolflags version` |
| `tb init` | created `.toolbelt/config.yaml` (mode `600`); Go defaults (`verify.test_cmd: go test ./...`); no `AGENTS.md` | hindered (JS onboarding) | path `/tmp/target/.toolbelt`; no agent file generated |
| `tb mem init/add/list/check` | OK; 5 invariants; `compactacao_segura: true` | helped | `mem add` requires positional arg; files `0600` |
| `tb ticket …` | nonexistent command | hindered | `Error: unknown command "ticket"`; docs/cli.md still documents it |
| manual ticket `rd.md` (Scope) | parse OK by `drift`/`review` | helped | `scope: lib/request.js,test/req.subdomains.js` + `base_commit` |
| `tb graph index` / `index .` in `/tmp/target` | 0 nodes / 0 edges, EXIT=0 | hindered | root basename `target` ∈ `SkipDirs` → `filepath.SkipDir` at root |
| `tb graph --enable index lib` | 130 nodes, 426 edges | helped (partial) | need to pass subpath; `index` without `--enable` fails (ADR-011) |
| `tb graph index .` in `/tmp/express-src` | 452 nodes, 3051 edges | confirms cause | same code, dirname ≠ `target` |
| `tb graph search/callers/impact/deps` | search found `subdomains`; impact/deps on `lib/request.js` empty; weak callers | neutral/hindered | impact `(vazio)` even after indexing `lib` |
| `tb graph impact` without `--file` | required error | neutral | `Error: --file é obrigatório` |
| `tb verify run` (npm configured) | PASS; hidden `disponível: false` | helped (partial) | hidden only activates if `tests/hidden/` dir exists, ignores `--hidden-test-cmd` alone |
| `tb verify` Go defaults on JS repo | ESCALATE (3 iters) | hindered | `go test ./...` without Go module |
| `tb verify --test-cmd true` | PASS despite broken JS outside the check | hindered | demonstrates partial checks |
| `tb verify --ruler visual` without manifesto | PASS `skipped: true` | neutral | missing manifesto |
| `tb verify --ruler visual` with screens without refs | PASS report `skipped: false` but screens `skipped: true` | hindered | false “not skipped” in aggregated report |
| `tb review` (good diff) | EXIT=0; completeness warning on regression acceptance | neutral | soft FP: “Testes existentes… continuam passando” |
| `tb review -o` | fails | hindered | only `--output`, not `-o` (help lists `--output`) |
| `tb drift` (literal paths) | EXIT=0, no out_of_scope | helped | matched ticket scope |
| `tb drift` with `lib/*.js` | EXIT=0 (simple glob works) | neutral | `filepath.Match` |
| `tb drift` with `src/**/*.ts` | EXIT=1, real out_of_scope | neutral | mid-path `**` not supported |
| `tb guard check` on the fix | no violations | neutral | does not scan JS (only Go heuristics + paths) |
| `tb guard` + custom `.toolbelt/guard.yaml` | yaml ignored | hindered | custom protected `lib/request.js` did not yield violation |
| `tb guard` on `.toolbelt/thresholds.yaml` | `protected_path` | helped | hardcoded list |
| `tb verify` after touching thresholds | FAIL iters=0 (abort by guard) | hindered in verify flow | confirms abort when hardcoded path changes |
| `tb bench run` without `--agent`/`--fake` | error, ignores `bench.default_agent` | hindered | config had `echo DEFAULT_AGENT_RAN {prompt}` |
| `tb bench run --fake` | OK without `--fake-config` | helped | 9 tasks, ~3.8 s |
| `tb bench compare` | A/B with `--fake` OK; `--agent` flag nonexistent | hindered (real A/B) | `unknown flag: --agent` |
| `tb mcp` | tools/list + tools/call `tb_version` OK via stdio | helped | 8 tools exposed; graph tools need `TB_GRAPH=on` |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | confirmed | `tb bench compare --agent …` → `unknown flag: --agent`; help only has `--fake` |
| `guard.yaml` ignored | confirmed | `.toolbelt/guard.yaml` protecting `lib/request.js` → violations null on the same diff |
| outdated CLI docs | confirmed | `docs/cli.md` documents `tb ticket`, `tb run`, `tb proc`, `tb board`, etc.; top-level CLI has no `ticket`/`run`/`proc`/`board`/`merge`/`md`/`agent` |
| version shows `unknown` | confirmed (partial) | without ldflags: version `0.1.0-dev`, but `commit` and `built` = `unknown` |
| config files created 0600 | confirmed | `stat`: `600` on `config.yaml`, `invariants.yaml`, `graph.json` |
| visual ruler reports `skipped=false` when skipping | confirmed | manifesto present, refs missing: report `"skipped": false` + screen `"skipped": true`, verdict PASS |
| `default_agent` ignored | confirmed | `bench.default_agent` set; `tb bench run` still requires `--agent` or `--fake` |
| guard aborts verify when diff touches hardcoded config path | confirmed | diff on `.toolbelt/thresholds.yaml` → verify `FAIL (iterações: 0)` |
| default fake agent needs `--fake-config` | refuted | `tb bench run --fake` OK without flag; even `--fake-config` pointing at nonexistent file still ran |
| `mem add` positional | confirmed | `tb mem add --category security` → `requires at least 1 arg(s)`; only positional `<regra>` |
| Drift/Scope need literal paths (no globs) | refuted (partial) | `lib/*.js` works; `src/**/*.ts` does not; `/**` suffix is a special prefix in code |
| `graph impact` needs `--file` | confirmed | `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | refuted (in this case) | on clean diff, `tests/hidden` invariant did not fire; on cheat commit, fired correctly. There was a *completeness* FP, not that invariant |
| verify runs only partial checks and misses build errors | confirmed | Go defaults on JS repo; `--test-cmd true` gives PASS; hidden_cmd ignored without `tests/hidden` dir; init does not detect `npm test`/`eslint` |
| tb does not push agent toward completing whole task | confirmed (as product gap) | no `tb ticket`/board/progress; review only soft warning; init does not create progress harness or living checklist; agent must improvise `rd.md` |

## New problems

### P0 — `graph` does not index a repo whose basename ∈ `SkipDirs` (e.g. `target` directory)

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb graph --enable index .` → `0 nós`. `clone … /tmp/express-src && tb graph --enable index .` → `452 nós`. `SkipDirs` includes `"target"` (Rust/Java) and the walk does `SkipDir` when the root `info.Name()` is `target`.
- **Expected:** index the project tree; skip only ignorable *subdirectories*.
- **Actual:** empty graph with EXIT=0 (false success).

### P1 — `verify` only considers hidden suite if `tests/hidden/` exists, ignoring `--hidden-test-cmd` / config

- **Repro:** configure `hidden_test_cmd: npm test -- --grep IPv6-mapped` without creating the directory → `Oculto: false (disponível: false)`.
- **Expected:** configured hidden command runs (or explicit warning).
- **Actual:** flag/config silently inert.

### P1 — `tb init` assumes Go toolchain and does not detect npm/eslint/mocha

- **Repro:** `tb init` on express → `verify.test_cmd: go test ./...`.
- **Expected:** detect `package.json` scripts or ask.
- **Actual:** default verify unusable until manual edit.

### P2 — `graph impact` / `deps` return empty for central JS file after index

- **Repro:** index `lib`, `tb graph --enable impact --file lib/request.js` → `(vazio)`; `deps --file lib/request.js` → `(vazio)`.
- **Expected:** at least imports/`requires` and impacted files.
- **Actual:** no useful signal for change planning.

### P2 — visual report aggregates `skipped: false` with skipped screens and PASS

- Already listed as known; reinforced here with malformed manifesto/missing refs on Express (no UI).

### P3 — UX: `tb review -o` rejected; help does not show shorthand

- **Repro:** `tb review … -o /tmp/x.json` → `unknown shorthand flag: "o"`.
- **Expected:** `-o` alias of `--output` (as in `verify`/`bench`).
- **Actual:** only `--output`.

### P3 — `guard` does not analyze JS (skip/eq/etc.)

- **Repro:** mocha test changes do not trigger `test_skip`/`trivial_assert` (Go-only scanner).
- **Expected:** at least multi-language heuristics or a warning.
- **Actual:** only hardcoded `protected_path` + Go AST.

## What tb caught and what it let slip

**Caught**
- Scope drift when paths outside the ticket (`tests/hidden/cheat.js`).
- Invariant violation when touching `tests/hidden/`.
- Protected path `.toolbelt/thresholds.yaml` and corresponding verify abort.
- Review freshness + completeness warning (even if noisy).
- Graph search found `subdomains` after indexing `lib`.
- Fake bench and A/B compare (`--fake`) worked as a harness.

**Let slip / did not help**
- Did not guide bug discovery nor suggest checking `isIP`+brackets (empty graph impact/deps).
- Did not configure Express verify/lint/test on its own.
- Operator `guard.yaml` ignored — cannot protect project paths.
- Does not require (or track) full native suite; with narrow `--test-cmd` or `true`, passes.
- Review did not actually validate the IPv6 acceptance criteria (only generic regression text).
- No ticket/board to push “close acceptance / run full lint+test”.
- Default index on path `/tmp/target` failed silently (0 nodes).

Native comparison: `npm test` + `npm run lint` validated the fix end-to-end; tb verify, once pointed at `npm test -- --grep subdomains`, followed the visible test signal, but without real hidden and without dedicated lint/build.

## Interaction with existing instrumentation / onboarding

Express **does not** have agent instrumentation. tb onboarding from scratch:

| Artifact | Generated? | Quality |
|----------|------------|---------|
| `.toolbelt/config.yaml` | yes | generic Go; graph languages OK (includes javascript); verify/bench wrong for Node |
| `.toolbelt/invariants.yaml` via `mem init` | yes | useful anti-cheat, but centered on `tests/hidden` / thresholds (tb convention, not Express) |
| `AGENTS.md` / skills / progress log | no | gap vs research findings (AGENTS.md, long-horizon harness) |
| Automatic test/lint/build detection | no | had to edit YAML by hand |
| Ticket/Scope | no (missing command) | created `rd.md` manually; that worked with drift/review |
| Guard manifesto | docs cite `guard.yaml`; runtime ignores | confuses onboarding |
| MCP | starts easily (`tb mcp`) | useful; graph tools depend on enable/env |

Conclusion: tb **initializes a skeleton**, but **does not bootstrap** a JS repo “from nothing” into a verifiable flow. Missing toolchain detection, agent context generation (`AGENTS.md`), and a first-class mandate/ticket.

## Improvement suggestions

1. **[P0][graph]** Do not apply `SkipDirs` to the walk root directory; compare relative path, not only root `filepath.Base`. (area: graph)
2. **[P1][verify]** Activate hidden when `hidden_test_cmd`/config is set; or fail loudly if the dir is missing. Auto-suggest commands from `package.json` / Makefile. (verify / init)
3. **[P1][init]** Detect ecosystem (npm/go/cargo…) and fill `verify.test_cmd`, lint, and optionally generate `AGENTS.md` + `rd.md` template. (init / mem)
4. **[P1][guard]** Honor `.toolbelt/guard.yaml`; separate “abort verify” from “report violation” with a flag. (guard / verify)
5. **[P1][bench]** Respect `default_agent`; add `--agent` to `bench compare`. (bench)
6. **[P2][graph]** Make default `index` walk the repo even without args; JS impact/deps via `require`/`import`. (graph)
7. **[P2][docs]** Sync `docs/cli.md` / README with actually implemented commands; mark stubs. (docs)
8. **[P2][review]** Acceptance criteria: require test evidence (describe/it name in the diff) before warning; align `-o` with other commands. (review)
9. **[P2][verify/visual]** If all screens were skipped, report `skipped: true` (not silent PASS). (verify)
10. **[P3][mem/UX]** Accept `mem add --rule` in addition to positional; document `0600` file modes or use `0640`/`0644` for non-secret config. (mem / init)
11. **[P3][router/ticket]** Implement minimal `tb ticket` or remove from docs — without it tb does not push task closure. (ticket / router)

## Metrics

| metric | value |
|--------|-------|
| Wall time of instrumented session (STUDY_START→END) | ~147 s (~2.5 min) of sequentially timed commands; npm/clone/build setup separate (~15 s) |
| Sum of reported times in logged commands | ~20.1 s |
| Labeled invocations in the log (`===== … =====`) | 76 |
| `EXIT=0` / `EXIT=1` results in the log | 54 / 22 (many `EXIT=1` are expected negative tests) |
| Relevant unexpected tb failures | graph 0 nodes in `/tmp/target`; missing `ticket`; ignored `default_agent`; ignored `guard.yaml`; visual `skipped=false` |
| Local commits on target (unpublished) | IPv6 fix + experimental thresholds/hidden commits to prove guard |

Raw session log: `/tmp/tb-campo-express.log` (study VM; not versioned).
