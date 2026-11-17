[🇧🇷 Português](../../../campo/rodada2/mcp-python-sdk.md) | 🇺🇸 English

# Field study round 2: modelcontextprotocol/python-sdk

Study conducted on 2026-11-16 with `tb` built from current `main` (`make build` → `toolbelt a46c295`). Target only on a local clone (`/tmp/python-sdk` and a copy `/tmp/target`); no push, issue, PR, or comment upstream.

## Repository

| Field | Value |
|-------|--------|
| Name | [modelcontextprotocol/python-sdk](https://github.com/modelcontextprotocol/python-sdk) |
| Language | Python 3.11+ (MCP SDK; `mcp` package) |
| Size | ~25 MiB shallow clone; ~303 MiB with `.venv`; ~420 `.py` under `src/`+`tests/` |
| Clone base commit | `77001c9` (`main` shallow) |
| Group | With existing AI instrumentation |
| AI instrumentation | `AGENTS.md` (166 lines), `CLAUDE.md` (`@AGENTS.md`), `.claude/commands/`, `.claude/skills/test-quality/`, Claude workflow |
| Toolchain | **Yes, partial.** `uv 0.13.8` + `uv sync --frozen` (Python 3.12). Run: targeted pytest, `ruff check` on touched files, spot `compileall`. **Not run:** full `./scripts/test`, full `pyright`, docs/`zensical`, matrix 3.11–3.15. |

## Chosen task

| Field | Value |
|-------|--------|
| What | In `get_display_name` (`src/mcp/shared/metadata_utils.py`), treat blank `title` (and `annotations.title`) as absent, falling through to the next level / `name`; tests in `tests/shared/test_metadata_utils.py` |
| Why | Small client bug/UX, testable, off the transport hot path; maintainer-plausible; different exercise from round 1 |
| Issue | None (local change; no upstream interaction) |
| Completed? | **Yes, locally** (branch `campo2-local-display-name`, commit `036fec8`). Not sent upstream. |
| Native test/lint/build | `uv run --frozen pytest tests/shared/test_metadata_utils.py tests/server/mcpserver/test_title.py -q` → **10 passed**; `ruff check` on touched files → OK; module `compileall` → OK. Full suite / pyright **not** run. |
| Round 1 (comparison) | Default `StdioServerParameters.encoding_error_handler`: `"strict"` → `"replace"` (issue #2454) |

## Verification of the 12 fixes

| # | Fix | Result | Evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph --enable rebuild` under `/tmp/target` → **16521 nodes / 63000 edges** (~2.1 s). In round 1 the same path gave 0 nodes. |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir, with `hidden_test_cmd: uv run --frozen ruff check …` → `"hidden_available": true`, `"hidden_passed": true`, verdict PASS. |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Touch of a protected path + `--strict --json` → JSON with violation and **EXIT 1**. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `src/mcp/shared/metadata_utils.py` → `protected_path` + EXIT 1 under `--strict --json`. |
| 5 | init toolchain detection | **worked** | `tb init` → `verify.test_cmd: uv run pytest` (no longer `go test`). `hidden_test_cmd` came empty. |
| 6 | review `tests/hidden` by path | **worked** | Ticket only citing `tests/hidden/**` → findings=[]; real commit under `tests/hidden/probe.py` → critical invariant finding. |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `bench.default_agent: true {prompt}` → `tb bench run` without `--agent/--fake` OK; without default → required error; `tb bench compare … --agent 'true {prompt}'` OK. |
| 8 | review `## Scope`/`## Escopo`, MD lists and `**` | **worked** | Ticket `## Escopo` + bullets → clean drift/review (no “no scope declared”). Frontmatter `src/mcp/shared/**/*.py` also accepted. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Final verify: `build_ran/passed` and `lint_ran/passed` true before tests. |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** | Init logged sources + hints (`uv run --frozen …`); wrote `.toolbelt/agent_docs.yaml`. In an empty dir, `--write-agents` seeded `AGENTS.md` without clobber. |
| 11 | graph Python `calls` | **worked** | Stats: **19400** `edges_calls` (round 1: 100). `callers --function get_display_name` and `impact --file …/metadata_utils.py` listed real callers. |
| 12 | aggregated visual skipped + CLI docs | **worked (partial)** | Missing manifest → `skipped: true`. Screens with missing refs → top-level `skipped: true` + `skip_reason: todas as telas foram skipped`. **Still:** `screens: []` → `skipped: false`, `screens: null`. Docs: `hidden_test_cmd` aligned; `tb ticket` marked “Não implementado”, but long `tb ticket`/`mem add --type` examples still appear. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; detected uv/pytest; read AGENTS/CLAUDE | helped | Wall ~0.006 s. `test_cmd=uv run pytest` (without `--frozen` required by AGENTS). Config mode 0600. |
| `tb init --write-agents` | OK in a repo without AGENTS | helped | Seeded `/tmp/noagents/AGENTS.md`; on python-sdk did not overwrite. |
| `tb version` | `a46c295` with `make build` | neutral | Without ldflags still shows `unknown` (not retested this cycle). |
| `tb mem init/list/add/check` | OK; 3 anti-cheat defaults | neutral | Does not import AGENTS.md rules into the vault; only a generic template. |
| `tb graph rebuild` `/tmp/target` | 16521 nodes | helped | Confirms SkipDirs fix. |
| `tb graph rebuild` `/tmp/python-sdk` | 16541 nodes / 63030 edges | helped | ~2.0 s. |
| `tb graph search/callers/impact/deps` | useful search + callers + impact; deps = `mcp_types` | helped | Python calls enable navigating the task. |
| Ticket `## Escopo` + `**` | drift/review OK | helped | Different from round 1 (broken parser). |
| `tb verify run` (build+lint+test+hidden) | PASS; hidden ran without dir | helped | ~1.7 s on the happy path. |
| `tb verify --ruler visual` | aggregated skip OK; `screens: []` still `skipped:false` | neutral/hindered | Empty case still lying. |
| `tb review` / `tb drift` | PASS on the fix with frontmatter or `## Escopo` | helped | Fresh_context true; findings=[]. |
| `tb guard check` | OK on the fix; honors `guard.yaml` | helped | Strict+JSON exit 1 with violation. |
| `tb bench run/compare` | default_agent + `--agent` on compare | helped | Toolbelt fixtures; target repo without `bench/tasks`. |
| `tb mcp` | tools/list + `tb_version` + `tb_graph_search` OK | helped | With `TB_GRAPH=on`, search found `get_display_name`. |

## Problems that persist

| problem (round 1) | status this round | evidence |
|-------------------|-------------------|----------|
| Init does not use AGENTS `--frozen` / checks | **persists (partial)** | Reads hints in `agent_docs.yaml`, but `verify.test_cmd` stays `uv run pytest` without `--frozen`; does not fill lint/build/pyright/`./scripts/test`. |
| Generic mem defaults vs AGENTS | **persists** | `mem init` only Go-ish anti-cheat; does not materialize uv-only / encoding= / coverage rules. |
| Partial verify checks / miss outside cmd | **persists** | Syntax error in `examples/_campo_broken.py` + narrow test_cmd → verify PASS. |
| Config files mode 0600 | **persists** | `stat` → `config.yaml` and `agent_docs.yaml` mode 600. |
| CLI docs with invented surfaces | **persists (reduced)** | `tb ticket` now “Não implementado”, but example blocks and `mem add --type/--title` still present. |
| Misleading visual PASS | **persists (residual case)** | `screens: []` → top-level `skipped: false`. |
| Graph off without `TB_GRAPH`/`--enable` | **persists (by design)** | ADR-011; MCP search needs the env. |

Round 1 problems **resolved** in this binary: SkipDirs/`/tmp/target`, `guard.yaml`, `--strict --json`, hidden without dir, Markdown Scope, default_agent, Python callers, AGENTS.md read, build/lint cmds, substring FP on `tests/hidden`.

## New problems

### P2 — `screens: []` still reports `skipped: false`

- **Repro:** `.toolbelt/screens.yaml` with `screens: []` + `tb verify run --ruler visual --skip-unavailable --json`
- **Expected:** `skipped: true` (nothing to evaluate)
- **Actual:** `"skipped": false`, `"screens": null`, verdict PASS

### P2 — Init reads AGENTS.md but does not apply it to the harness

- **Repro:** `tb init` on python-sdk → `agent_docs.yaml` with `uv run --frozen pytest` / ruff / pyright; `config.yaml` with `test_cmd: uv run pytest` and empty lint/build
- **Expected:** map hints to `verify.test_cmd` / `lint_cmd` / checklist or warn on mismatch
- **Actual:** tips only in the log/auxiliary YAML; agent still must rewrite verify by hand

### P3 — Empty default `hidden_test_cmd` after Python init

- **Repro:** init in a repo with `uv.lock` → `hidden_test_cmd: ""`
- **Expected:** useful default (e.g. ruff) or document that hidden is explicit opt-in
- **Actual:** empty field; hidden only runs if the agent configures it

### P3 — Impact lists symbol callers, not “affected files” in an obvious way

- **Repro:** `tb graph impact --file src/mcp/shared/metadata_utils.py` → functions that call `get_display_name` (useful), without suggested risk/test ranking
- **Expected:** (improvement) point at priority tests/examples from impact
- **Actual:** raw list; the agent still decides coverage

## What tb caught and what it let slip

**Caught**

- Scope via `## Escopo` / frontmatter and `**` globs
- Guard via `guard.yaml` + hardcoded + strict/JSON exit code
- Build + lint + test + hidden (without a `tests/hidden/` folder)
- Python callers/impact for `get_display_name`
- Onboarding: discovery of AGENTS/CLAUDE and `--write-agents` seed
- Bench with `default_agent` and `compare --agent`
- MCP graph search with graph enabled

**Let slip / did not cover**

- `uv run` without `--frozen` in the init default vs the AGENTS.md mandate
- pyright / `./scripts/test` / 100% coverage — not queued
- Syntax error outside configured cmds → PASS
- Empty visual manifest (`screens: []`) looks like execution
- Dense AGENTS.md rules do not become `mem` invariants

## Interaction with existing instrumentation / onboarding

| Behavior | Detail |
|----------|--------|
| tb **reads** AGENTS.md/CLAUDE.md | `agent_docs.yaml` + hints log (`uv run --frozen …`, ruff, pyright, coverage). |
| tb **does not apply** automatically | `verify.*` does not inherit `--frozen` or lint/typecheck; `mem` vault ignores the content. |
| `--write-agents` | Only seeds if absent; on python-sdk preserved the mature AGENTS. |
| Overlap | `mem init` anti-cheat `tests/hidden` is still generic; skill `.claude/skills/test-quality/` is not indexed in the vault. |
| What onboarding still lacks | (1) promote hints → `verify.test_cmd`/`lint_cmd`/`build_cmd`; (2) invariants from AGENTS (uv-only, encoding=); (3) point at `scripts/test` and the quality skill. |

## Comparison with round 1

| Aspect | Round 1 (`c8aae04`) | Round 2 (`a46c295`) |
|--------|---------------------|---------------------|
| Graph under `/tmp/target` | 0 nodes | ~16k nodes |
| Python calls | ~100; empty callers | ~19k; useful callers/impact |
| Init | `go test`; ignores AGENTS | `uv run pytest`; reads AGENTS + `--write-agents` |
| Markdown Scope | broken without frontmatter | `## Escopo` + `**` OK |
| Hidden / guard.yaml / strict JSON / default_agent | failed | work |
| Visual skipped | `skipped:false` with skipped refs | aggregation OK; residual `screens: []` |
| Conclusion | tb hindered Python onboarding | **better in concrete ways** on the 12 fixes; main gap now is **apply** AGENTS to the harness, not only read it |

## Improvement suggestions

1. **[P1][init/verify]** When detecting `uv run --frozen …` / ruff / pyright hints, fill `verify.test_cmd`/`lint_cmd` (or fail init with expected diff vs AGENTS).  
2. **[P1][mem]** Optional: extract invariants from AGENTS.md “FORBIDDEN”/mandate sections.  
3. **[P2][verify/visual]** `screens: []` or zero evaluable screens → `skipped: true`.  
4. **[P2][docs]** Remove or isolate `tb ticket` / `mem add --type` examples behind “planned”, without looking like current CLI.  
5. **[P2][init]** Default Python `hidden_test_cmd` (e.g. `uv run --frozen ruff check .`) or omit the field with clear docs.  
6. **[P3][config]** Review 0600 in monorepos; 0644 for non-secret YAML.  
7. **[P3][graph]** `impact` suggest touched tests / caller examples as a verify checklist.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Binary | `toolbelt a46c295` | `toolbelt c8aae04` |
| Summed wall (timed samples, logs 1+2) | **~24.7 s** / 72 samples | ~35.4 s / 70 |
| Canonical retest wall (log 2, current main) | **~9.7 s** / 25 samples | — |
| EXIT≠0 failures (all logs) | **9** (probes: old write-agents, bench flags, intentional guard, intermediate-binary `**/*.py` drift, etc.) | 25/70 |
| Failures on the fix happy path (a46c295) | **0** (final verify/review/drift/guard PASS) | few after frontmatter |
| Largest individual WALL | **3.37 s** (initial native pytest) / graph rebuild ~2.1 s | 9.96 s |
| Final verify | **~1.7 s**, PASS, hidden+build+lint | 3.52 s |
| Useful graph | 16541 nodes, 19400 calls | 16550 nodes, 100 calls |
| MCP smoke | tools/list + search OK | tools/list OK; null search under `/tmp/target` |
