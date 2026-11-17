[🇧🇷 Português](../../campo/playwright-mcp.md) | 🇺🇸 English

# Field study: microsoft/playwright-mcp

Evidence collected on 2026-11-15 on the local VM. Clone at `/tmp/target/playwright-mcp` (shallow). No interaction with the target remote (no push/issue/PR/comment). `tb` binary built from this repo (`make build` → `toolbelt c8aae04`). Raw log: `/tmp/tb-campo-evidence.log`.

## Repository

| Field | Value |
|-------|-------|
| Name | [microsoft/playwright-mcp](https://github.com/microsoft/playwright-mcp) (`@playwright/mcp` 0.0.85) |
| Language | JavaScript/TypeScript (npm wrapper; real MCP server lives in `microsoft/playwright` / `playwright-core`) |
| Size | ~49 files outside `node_modules`/`.git`; ~552 KiB tree; ~3.5 kLOC counted in `.js`/`.ts`/`.md` |
| Existing AI instrumentation | `AGENTS.md`; `.claude/skills/release.md`; workflows `.github/workflows/agent-{triage,review,task,roll,release}.yml` |
| Toolchain | **OK:** Node 22, `npm ci`, system Chrome + Playwright Chromium/Chrome, Firefox downloaded. **Not exercised end to end:** WebKit, multi-browser `npm test`, `dtest`/Docker, roll from `../playwright` checkout. `npm run build` is a no-op (`echo OK`). |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix env-var docs generation in `update-readme.js` + regenerate `README.md` + regression in `tests/cli.spec.ts` |
| Why | Real bug local to this repo (does not need the upstream monorepo); maintainer-plausible; has an open issue and a clear test path |
| Issue | https://github.com/microsoft/playwright-mcp/issues/1861 |
| Completed locally? | **Yes** on the clone (local commit `80d3280`); **not** sent upstream (forbidden by study rules) |
| Native result | `npx playwright test tests/cli.spec.ts --project=chrome`: **2 passed**. `npm run lint`: **OK** after the fix. With a syntax error introduced only in `update-readme.js`, `node --check` and `npm run lint` fail, but the README test still passes |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK (~7 ms); creates `.toolbelt/config.yaml` | hindered (defaults) | Defaults `verify.test_cmd: go test ./...` and `hidden_test_cmd: go test ./tests/hidden/...` in a JS repo; does not read `AGENTS.md` |
| `tb mem init/list/add/check` | OK | neutral→helped | `mem add --type fact` fails (`unknown flag`); `mem add "<regra>" --category mandate` works; `check` SHA256 OK |
| Ticket / Scope (manual `rd.md`) | OK via file; `tb ticket` **nonexistent** | hindered | `tb ticket` → `unknown command`; Scope in YAML frontmatter worked with literal paths |
| `tb graph --enable rebuild/stats/search/deps/impact/index` | rebuild 132 nodes / 413 edges; search found `optionEnvName` | helped | `impact` without `--file` exit 1 (`--file é obrigatório`); with `--file update-readme.js` OK; deps listed `fs`/`path`/`playwright-core` |
| `tb verify run` (tests) | PASS with `--project=chrome`; ESCALATE with `--project=chromium` (nonexistent project) | hindered if misconfigured | Only runs `test_cmd`; does not include `npm run lint`/`node --check`; hidden only if `tests/hidden/` exists |
| `tb verify run` (visual) | PASS | hindered (UX) | Missing manifesto: `skipped=true`. `screens: []`: `skipped=false` with `screens: null` and PASS |
| `tb review` | PASS on clean fix; FAIL (false positive) if diff mentions `tests/hidden` | hindered | Comment `// note: do not touch tests/hidden` produced a critical finding identical to actually touching the path |
| `tb drift` | OK with literal scope; FAIL with `**/*.js` | helped (literal) / hindered (`**` glob) | Literal scope: no drift. `**/*.js`/`**/*.md`: README.md and update-readme.js “out of scope” |
| `tb guard check` | OK on the fix; detects `.toolbelt/thresholds.yaml`; **ignores** `.toolbelt/guard.yaml` | hindered | `package.json` in `guard.yaml` protected_paths → `violations: null`; hardcoded thresholds → violation + immediate verify FAIL |
| `tb bench run/list/compare` | list/run --fake OK; compare A/B OK; `--agent` on compare rejected | neutral | Config `default_agent` **ignored** (`--agent ou --fake é obrigatório`). `--fake` **without** `--fake-config` worked |
| `tb mcp` | tools/list OK | helped | JSON-RPC response with `tb_version`, `tb_run`, `tb_graph_*`, `tb_echo`, `tb_env` |
| `tb version` | `c8aae04` via `make build`; `commit: unknown` via `go build` without ldflags | neutral | Confirms version ambiguity depending on build mode |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare ... --agent 'echo hi'` → `unknown flag: --agent`; help only lists `--fake` |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` with `package.json`; diff only in `package.json` → guard `violations: null`. Docs (`docs/cli.md`, `docs/arquitetura.md`) still promise `--manifest`/guard.yaml; real CLI has no `--manifest` |
| outdated CLI docs | **confirmed** | `docs/cli.md` documents `tb mem add --type/--title`; CLI requires `tb mem add <regra>`. Guard docs diverge from code |
| version shows `unknown` | **confirmed** (without ldflags) | `go build -o /tmp/tb-nolflags ./cmd/tb` → `commit: unknown` / `built: unknown`. With `make build`, version = git describe |
| config files created 0600 | **confirmed** | `stat`: `.toolbelt/config.yaml` and `invariants.yaml` → mode `600` |
| visual ruler `skipped=false` when skipping | **confirmed** | `screens.yaml` with `screens: []` → `"skipped": false`, `"screens": null`, `"verdict": "PASS"` |
| `default_agent` ignored | **confirmed** | Config with `default_agent: "echo AGENT_OK {prompt}"`; `tb bench run` without flags → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | Diff on `.toolbelt/thresholds.yaml` → guard error + `tb verify` verdict `FAIL` iterations 0 (does not run tests) |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake` without `--fake-config` completed 9 tasks; `FakeAgentCommand("")` omits `--config` on purpose |
| `mem add` positional | **confirmed** | Help: `tb mem add <regra>`; docs flags `--type/--title` nonexistent |
| Drift/Scope need literal paths (no globs) | **partially confirmed** | simple `filepath.Match` (`update-readme.*`) works; `**/*.js` **does not** match (README/update-readme out of scope) |
| `graph impact` needs `--file` | **confirmed** | Without flag: `Error: --file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmed** | Diff that only **mentions** the string `tests/hidden` in a comment → critical finding “possível violação” |
| verify runs only partial checks / misses build errors | **confirmed** | With syntactically broken `update-readme.js`: `node --check` and `npm run lint` fail; `tb verify` with only `tests/cli.spec.ts` → **PASS** |
| tb does not push agent toward completing whole task | **confirmed** (observational) | No `tb ticket`/board/progress; init does not import `AGENTS.md` (which warns most bugs are upstream); green verify without lint/build |

## New problems

| ID | sev | repro | expected | actual |
|----|-----|-------|----------|--------|
| N1 | P1 | `tb init` in a Node repo; run `tb verify run` without editing config | Defaults aligned to the detected toolchain, or onboarding that fails early with a clear message | Go defaults; ESCALATE 3× if the agent copies `chromium` from AGENTS.md while `playwright.config.ts` only defines project `chrome` |
| N2 | P2 | `tb drift` with scope `**/*.js` | Either `**` support, or an explicit “glob not supported” error | Treated as silent non-match → false drift |
| N3 | P2 | `tb review` + default `tests/hidden` invariant + diff with the substring in content | Heuristic by touched path, not substring in the diff | Blocking false positive |
| N4 | P2 | Create `.toolbelt/guard.yaml` and change a listed path | Guard loads the manifesto (as docs promise) | Manifesto ignored; only hardcoded list in `internal/guard/guard.go` |
| N5 | P3 | `tb verify --ruler visual` with empty manifesto `screens: []` | `skipped=true` + reason “nenhuma tela” | `skipped=false`, PASS |
| N6 | P3 | `tb mem add --type fact ...` following `docs/cli.md` | Docs command works or docs updated | `unknown flag: --type` |
| N7 | P1 | Agent uses only `tb verify` configured with a partial test after editing the README generator | Verify requires (or suggests) `npm run lint` / generator syntax check too | PASS with a broken generator |

## What tb caught and what it let slip

**Caught**
- Scope/drift: `package.json` change outside the mandate (forbidden).
- Guard: change to `.toolbelt/thresholds.yaml` (hardcoded path).
- Graph: located `optionEnvName` and dependencies of `update-readme.js`.
- Review on the clean fix diff: no findings.
- MCP: tools listable via stdio.

**Let slip**
- Fabricated env vars in the README **before** the fix (tb has no docs↔code ruler).
- Custom `guard.yaml` (package.json “protected” in user config).
- Syntax error in `update-readme.js` when `test_cmd` only read the README.
- Absence of `npm run lint` in the verify loop (lint is the native check that regenerates/validates README).
- Default Go-centric invariants (`tests/hidden/`, thresholds) unrelated to this repo.
- Critical guidance from `AGENTS.md` (“most bugs/features are upstream”) — tb did not import it.

**Native vs tb:** native (`lint` + `cli.spec.ts` on chrome) validates the full fix; tb only mirrors whatever is in `test_cmd`. With wrong config (`chromium`), tb failed where adjusting the project name fixes it.

## Interaction with existing instrumentation / onboarding

- `tb init` **ignores** `AGENTS.md`, `.claude/skills/`, and `agent-*` workflows. Vault stays empty; invariants are the generic anti-cheat template.
- Potential conflict: `AGENTS.md` instructs `npx playwright test ... --project=chromium`, but this checkout only declares project `chrome` → an agent that follows AGENTS.md + `tb verify` copies the command and gets ESCALATE.
- Duplication: mem/invariants and AGENTS.md both talk about process, but in different vocabularies; tb does not link them.
- What tb should import: (1) build/test snippets from `AGENTS.md` → `verify.test_cmd` / checklist; (2) “fix here only what this repo owns” warning → mandate; (3) commit/branch convention; (4) `.claude/skills/*` skills as optional vault playbooks.

## Improvement suggestions

1. **[P0][verify/init]** Detect toolchain (package.json scripts) in `tb init` and fill `test_cmd`/`lint_cmd`/`build_cmd`; verify should run configurable lint+test+build.  
2. **[P0][guard]** Actually load `.toolbelt/guard.yaml` (or remove it from docs); unify with the hardcoded list.  
3. **[P1][docs/cli]** Sync `docs/cli.md` with the real CLI (`mem add`, guard flags, bench compare).  
4. **[P1][mem/onboarding]** Import/summarize `AGENTS.md` (and skills) into the vault on init; `--import-agents` flag.  
5. **[P1][review]** Match invariants by touched path / AST, not substring in the diff.  
6. **[P2][drift]** Explicit `**` support or clear rejection of unsupported globs.  
7. **[P2][bench]** Honor `default_agent`; optionally `--agent` on A/B `bench compare`.  
8. **[P2][verify/visual]** Mark `skipped=true` when there are no screens / refs.  
9. **[P3][init]** Config permissions 0644 (or document 0600).  
10. **[P3][ticket]** Ship a minimal `tb ticket` (already used via ad-hoc `rd.md`) to close the Scope→verify→review loop.

## Metrics

| metric | value |
|--------|-------|
| Wall time (evidence log) | ~20:43:07–20:46:30 UTC (~3.4 min of timed commands; `tb` build ~6 s before) |
| Sum of timed durations | ~41.2 s |
| `tb-*` invocations in the log | 43 |
| `tb-*` failures (exit ≠ 0) | 14 (includes expected probe failures: impact without file, compare --agent, default_agent, globs, review false positive, misconfigured verify, etc.) |
| Total logged invocations (tb+native) | 56 |
| Local task | completed (fix #1861) with green chrome tests + lint |
