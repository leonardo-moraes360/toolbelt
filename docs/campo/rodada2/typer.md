🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/typer.md)

# Field study round 2: fastapi/typer

Group: without existing AI-agent instrumentation (no AI agent instrumentation for contributing to the repository itself).  
Binary: `/tmp/tb` (build `go build -o /tmp/tb ./cmd/tb` from `main` @ `a46c295`).  
Working clone: `/tmp/typer-r2`.  
Date: 2026-11-16.

## Repository

| Field | Value |
|-------|-------|
| Name | [fastapi/typer](https://github.com/fastapi/typer) |
| Language | Python (≥3.10), CLI over vendored Click |
| Size | shallow clone ~5.1 MiB; **643** `.py` files; **211** `tests/test_*.py` |
| AI instrumentation | **No** `AGENTS.md`, `CLAUDE.md`, or `.cursorrules` at the root. There is `typer/.agents/skills/typer/SKILL.md` (skill for Typer *consumers*, not contribution onboarding). |
| Toolchain | **Partially configured.** `pip install -e ".[standard]"` + `pytest`, `ruff`, `mypy`, `pytest-xdist`, `coverage`. `uv` was **not** on PATH (there is `uv.lock`). MkDocs docs / `prek` / full CI suite **not** installed. Smoke: ZDOTDIR tests + zsh completion subset OK; `ruff check` on touched files OK. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Make `install_zsh` respect `ZDOTDIR` (when valid) when writing `.zshrc`, instead of always using `$HOME/.zshrc`. |
| Why | Real reported bug; small patch with isolated unit tests (`tmp_path`), different from round 1’s `TMPDIR` flakiness. |
| Link | https://github.com/fastapi/typer/discussions/1757 |
| Complete? | **Yes**, locally: logic in `typer/_completion_shared.py` + 2 tests in `tests/test_completion/test_completion_install.py`. |
| Native checks | `pytest` of the 2 new tests → PASS; `ruff check` on touched files → PASS; `tests/test_completion/` subset (zsh/install) → 11 passed, 1 skipped. **No** push/PR/issue upstream. |
| Round 1 (comparison) | Flaky `tests/test_types_file.py::test_file_error` with long `TMPDIR` (discussion #1984). |

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Partial clone under `/tmp/target` + `TB_GRAPH=on tb graph rebuild` → **626 nodes / 1492 edges** (round 1: 0 nodes). |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | `tests/hidden` nonexistent; `tb verify run --json` → `hidden_available: true`, `hidden_passed: true` with `hidden_test_cmd: pytest …test_completion_source_zsh`. |
| 3 | guard `--strict --json` exit ≠0 | **worked** | With violation on `typer/colors.py`: without `--strict` exit 0; with `--strict --json` **exit 1** + violations JSON. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `typer/colors.py`; diff touching the file → `protected_path` violation. |
| 5 | init toolchain detection | **worked** (with caveat) | `tb init` on typer → `verify.test_cmd: uv run pytest` (detected `uv.lock`/`pyproject`). **Caveat:** `uv` missing from PATH → broken default until manual fix. |
| 6 | review `tests/hidden` by path | **worked** | Comment `# probe tests/hidden…` only in the diff → `findings: []`. Real touch of `tests/hidden/secret_test.py` → **critical** invariant finding. |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Without `--agent`/`--fake`, `bench run` used `echo-agent-SHOULD-BE-USED` (`agent_output`: `not found`, exit 127). `bench compare --agent echo-compare-agent …` ran A/B. |
| 8 | review/drift `## Scope`/`## Escopo`, lists, `**` | **worked** | Ticket with `## Escopo` + bullets + `tests/test_completion/**/*.py` kept files in scope; ticket only with `typer/main.py` → drift; `tests/**/*.py` + `typer/**/*.py` kept code, flagged `rd.md`. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | JSON: `build_ran/passed: true`, `lint_ran/passed: true` before tests. |
| 10 | init reads AGENTS/CLAUDE and `--write-agents` | **worked** | Without files: log “nenhum AGENTS.md/CLAUDE.md…”. With synthetic AGENTS.md: `agent_docs.yaml` + hints. `--write-agents` seeds AGENTS.md; does not overwrite existing. |
| 11 | graph Python `calls` | **worked** | Stats: `edges_calls: 3827`. `graph callers --function install_zsh` returned tests + file; `impact --file typer/_completion_shared.py` non-empty. |
| 12 | verify visual skipped + CLI docs | **worked** | `--ruler visual` → `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. `docs/cli.md` and `docs/verificacao-visual.md` describe top-level aggregation. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; detects `uv run pytest`; warns missing AGENTS | mixed | WALL 0.005 s; default depends on missing `uv` |
| `tb init --write-agents` | Seeds minimal AGENTS.md | helped | Probe under `/tmp/typer-agents-probe` |
| `tb mem init/add/list/check` | OK; 3 defaults + 1 rule | helped (light) | `mem check` retention 1.00 |
| `tb graph rebuild/stats/search/callers/impact` | 3054 nodes / 9821 edges; useful callers/impact | **helped** | Strong contrast with round 1 (empty callers/impact) |
| ticket `rd.md` | Manual (`tb ticket` nonexistent) | hindered (light) | Same gap as r1; Markdown parser OK |
| `tb verify run` | PASS with build+lint+test+hidden | helped | After manual cmd config |
| `tb verify --ruler visual` | Graceful skip `skipped=true` | neutral | No UI manifest in Typer |
| `tb review` | Clean fix → `findings: []`; substring FP fixed | helped | Path vs comment probe |
| `tb drift` | Markdown/`**` scope OK; wrong scope detects | helped | EXIT 1 on bad ticket |
| `tb guard check` | YAML + `--strict --json` correct | helped | Exit 1 with violation |
| `tb bench` | Honors `default_agent`; `compare --agent` exists | mixed | Needs copied `bench/tasks`; nonexistent agent → 0% |
| `tb mcp` | stdio OK; tools version/echo/env/run/graph_* | neutral | No mem/verify/review/drift/guard |
| `tb version` | `0.1.0-dev`, commit/built `unknown` | neutral | Build without ldflags |

## Problems that persist

| problem (round 1) | status in round 2 | evidence |
|-------------------|-------------------|----------|
| `version` → `unknown` without ldflags | persists | `tb version --json` |
| configs mode `0600` | persists | `stat` on `.toolbelt/config.yaml` |
| MCP without mem/verify/review/drift/guard | persists | `tools/list` |
| Missing `tb ticket` / manual ticket | persists | `unknown command` in r1; r2 wrote `rd.md` by hand |
| Init does not fill `lint_cmd`/`build_cmd` alone | persists (partial) | Only auto `test_cmd`; lint/build needed manual YAML |
| Bench without tasks on the target | persists | Needed to copy `bench/tasks` from toolbelt |
| Fake/`doc-function` pass@1=0% without FilesToCreate | persists | `--fake` success 0.0% |
| Consumer `.agents` skill ignored | persists | Init does not read `typer/.agents/skills/...` |

## New problems

### P2 — `tb init` chooses `uv run pytest` without `uv` on PATH

- **Repro:** `tb init` on a clone with `uv.lock` where `uv` is not installed; inspect `verify.test_cmd`.
- **Expected:** fallback to `pytest` / `python -m pytest` if `uv` does not exist.
- **Actual:** `test_cmd: uv run pytest` → default verify would fail (`uv: command not found`).

### P3 — AGENTS.md hints do not become `lint_cmd`/`build_cmd`

- **Repro:** `AGENTS.md` with “Lint with ruff…” / “Build: python -m build”; `tb init`.
- **Expected:** at least suggest or fill `lint_cmd`/`build_cmd`.
- **Actual:** hints in `agent_docs.yaml`; `config.yaml` only changes `test_cmd`.

### P3 — Invalid `default_agent` fails opaquely (exit 127, pass@1=0%)

- **Repro:** `bench.default_agent: echo-agent-SHOULD-BE-USED`; `tb bench run -r 1 --task doc-function`.
- **Expected:** clear “agent not found” error / bench command exit ≠0.
- **Actual:** bench exit 0, 0% success, `agent_output` with `not found` in the JSONL.

## What tb caught and what it let slip

**Caught**
- Drift when the ticket scope did not cover the changed files.
- Guard on a custom path via `guard.yaml` + exit code with `--strict --json`.
- Review: `tests/hidden` invariant only on touched path; ZDOTDIR patch without findings.
- Python graph: located `install_zsh`, callers of the new tests, module impact.
- Verify with configured build/lint/test/hidden: PASS aligned with native pytest/ruff.

**Let slip / did not cover**
- Did not install/validate presence of `uv` when suggesting `uv run pytest`.
- Did not configure lint/build automatically.
- Did not read the skill under `typer/.agents/skills/`.
- MCP does not expose the full field flow.
- Does not block an “incomplete task” (ticket/progress) alone.

**vs native:** pytest+ruff confirmed the fix; `tb verify` mirrored after manual config. Post-`init` default (`uv run pytest`) would be worse than direct `pytest` on this VM.

## Interaction with existing instrumentation / onboarding

- Group **without** contribution instrumentation: no AGENTS/CLAUDE at the root.
- Consumer skill under `typer/.agents/...` remains irrelevant to the harness.
- `tb init` now: detects Python/uv, warns missing AGENTS, optional `--write-agents`.
- Still need to assemble ticket `rd.md`, adjust `lint_cmd`/`build_cmd`/`hidden_test_cmd`, and (in this environment) swap `uv run` for `pytest`.
- Better than round 1, but bootstrap is still not “zero-touch” for Python without `uv`.

## Comparison with round 1

| Aspect | Round 1 | Round 2 | Verdict |
|--------|---------|---------|---------|
| SkipDirs `/tmp/target` | 0 nodes | 626 nodes | **better** |
| Python `calls` / callers | empty for patch symbol | 3827 calls; `install_zsh` callers OK | **better** |
| hidden without dir | `hidden_available: false` | `true` + command ran | **better** |
| guard.yaml / `--strict --json` | ignored / exit 0 | reads YAML / exit 1 | **better** |
| init toolchain | `go test ./...` | `uv run pytest` | **better** (with uv P2) |
| review FP `tests/hidden` | critical on comment | only on path | **better** |
| bench default_agent / compare `--agent` | required flags; compare without `--agent` | honors config; flag exists | **better** |
| Markdown Scope / `**` | fragile / partial | `## Escopo` + `**` OK | **better** |
| build/lint in verify | nonexistent | run when configured | **better** |
| AGENTS / `--write-agents` | did not read/generate | reads + seeds | **better** |
| visual skipped aggregation | ambiguous in some reports | `skipped: true` + aligned docs | **better** |
| MCP coverage | only graph/version/… | same | **same** |
| Completing the task alone | weak | still weak | **same** |

Overall: **clearly better** on the 12 fixes; remaining gaps are fine onboarding (uv), MCP, and completeness harness.

## Improvement suggestions

1. **[P2][init]** When detecting `uv.lock`, check whether `uv` is on PATH; otherwise fall back to `pytest` / `python -m pytest`.  
2. **[P2][init]** Fill or suggest `lint_cmd`/`build_cmd` from `pyproject`/hints (e.g. `ruff check`).  
3. **[P2][mcp]** Expose mem/verify/review/drift/guard tools.  
4. **[P3][bench]** Fail the command (exit ≠0) if the `default_agent` binary does not exist.  
5. **[P3][init]** Optionally read skills under `.agents/` / `**/SKILL.md`, not only root AGENTS/CLAUDE.  
6. **[P3][cli]** `tb ticket init` template generating `rd.md` with `## Escopo`.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Summed wall time of logged cmds | ~8.1 s (37 timed invocations) + ~6.3 s extra native pytest | ~8.9 s (79 invocations) |
| Approx. session wall time | ~15–25 min (build, clone, toolchain, ZDOTDIR fix, probes of the 12 fixes) | ~15–20 min |
| Logged `tb` invocations | 37 (+ auxiliary init probes) | 79 |
| Non-zero exits | 5 intentional (review drift, bad drift, guard strict, bench without tasks) | 20 |
| Relevant unexpected failures | default `uv run` without `uv`; bench agent 127 | empty `/tmp/target` graph; hidden off; etc. |
| MCP | smoke OK | smoke OK |
| Commits on target | 1 local (`fix: respect ZDOTDIR…`); **no** push/PR/issue on fastapi/typer | 1 local; no push |
