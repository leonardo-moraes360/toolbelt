🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/click.md)

# Field study (round 2): pallets/click

Group: **without existing AI-agent instrumentation**.  
`tb` binary: `go build -o /tmp/tb ./cmd/tb` from `main` (`a46c295` local after pull).  
Target clone: `/tmp/click-campo2` (shallow from https://github.com/pallets/click).  
No interaction with the Click remote (no push/issue/PR/comment).  
Evidence: `/tmp/tb-campo2-click-evidence.log`.

## Repository

| Field | Value |
|-------|--------|
| Name | pallets/click |
| Language | Python (≥3.11), `click` package |
| Size | shallow ~12M without `.venv`; ~208M with deps; 76 `.py` under `src/`+`tests/`; ~12.8k LOC in `src/` |
| Existing AI instrumentation | None (`AGENTS.md`/`CLAUDE.md`/`.cursorrules`/copilot/aider absent in the clone) |
| Toolchain | **Yes, partial**: `uv` + venv, `uv sync` (groups `tests`/`typing`/`dev`), `pytest` 9.0.2, `ruff` 0.15.9, `mypy` 1.20.0; installed `less` (was missing). **Not configured**: Sphinx/docs, full tox matrix, local CI pre-commit hooks, full pyright suite. Baseline: `2626 passed, 24 skipped, 1 xfailed`; after the patch: `2627 passed`. |

## Chosen task

| Field | Value |
|-------|--------|
| What | Avoid eager open/close of `_LazyFile` on FIFOs (`stat.S_ISFIFO`) so the writer is not consumed before the lazy open |
| Why | Small bug, reproducible on Linux, one production file + one test; different from round 1 |
| Issue | https://github.com/pallets/click/issues/2645 |
| Completed? | **Yes, locally** (commit only under `/tmp/click-campo2`; not sent upstream) |
| Native checks | `pytest tests/test_utils/test_LazyFile.py`: 2 passed; suite: **2627 passed**; `ruff check` on touched files: OK; `mypy src/click/utils.py`: Success |
| Round 1 (comparison) | `get_error_hint` + `auto_envvar_prefix` / issue [#3914](https://github.com/pallets/click/issues/3914) |

Essential diff: in `src/click/utils.py`, `_LazyFile.__init__` only does early `open(...).close()` if the path is **not** a FIFO; test `test_lazyfile_skips_eager_open_for_fifo` in `tests/test_utils/test_LazyFile.py`. Ticket `rd.md` with `## Scope` / Markdown lists and globs.

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Real dir `/tmp/target` with `src/`+`tests/`: `1456 nodes`. In round 1, `/tmp/target` → 0 nodes. **Nuance**: `ln -s /tmp/click-campo2 /tmp/target` → 0 nodes (`filepath.Walk` does not descend a root symlink) — not a SkipDirs regression |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No dir; `hidden_test_cmd` in YAML → `hidden_available: true`, `hidden_passed: true`. With `--hidden-test-cmd false` → `hidden_passed: false`, verdict `ESCALATE` |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | With `protected_paths: [src/click/utils.py]` and a diff on the file: JSON with violation and `EXIT_STRICT:1`; without `--strict` exit 0 |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | Same manifest: `type=protected_path`, `file=src/click/utils.py`. Without custom YAML, only defaults; `core.py` in YAML blocked a probe |
| 5 | init toolchain detection | **worked** | On Click (with `uv.lock`): `verify.test_cmd: uv run pytest`. Fixture with only `pyproject.toml`: `pytest`. No longer `go test ./...` |
| 6 | review `tests/hidden` by path | **worked** | Ticket cites `tests/hidden/**` under Forbidden; clean review: `findings: []` (round 1: critical FP by substring) |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `default_agent: echo fake-agent` + `tb bench run -r 1` without `--agent` → OK. Without default → exit 1 requiring the flag. `compare --config-a/--config-b --agent 'echo hi'` → OK (~1.1s) |
| 8 | review/drift parse `## Scope`/`## Escopo`, lists, `**` | **worked** | `## Scope` + backticks: drift without `out_of_scope`. `## Escopo` + `src/click/**/*.py` / `tests/**/*.py`: only `rd.md` out (expected) |
| 9 | verify `build_cmd` / `lint_cmd` | **worked** (via YAML) | PASS with `build_ran/lint_ran/passed: true`. CLI flags `--build-cmd`/`--lint-cmd` **do not exist** (config only) |
| 10 | init reads AGENTS/CLAUDE and `--write-agents` | **worked** (partial hints) | `--write-agents` created `AGENTS.md` + `agent_docs.yaml`. Fixture with `CLAUDE.md`: `sources: [CLAUDE.md]`; `hints: []` despite “use pytest” text |
| 11 | graph Python `calls` | **worked** | Rebuild: `1956 nodes / 5584 edges`, `edges_calls: 2815`. `callers --function open_stream` and `format_filename` non-empty; `impact --file src/click/utils.py` listed types/tests |
| 12 | aggregated visual skipped + CLI docs | **worked** / **partial docs** | `--ruler visual` without manifest: `skipped: true`, `skip_reason: manifesto de telas ausente`. `docs/cli.md` aligned on mcp/mem/guard/bench; `tb ticket` remains “planejado Fase 3+” |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` | OK `0.1.0-dev`, commit/built `unknown` | neutral | No ldflags; ~0.006s |
| `tb init --write-agents` | `.toolbelt/` + `AGENTS.md` + `uv run pytest` | **helped** | Right toolchain; mode `600` on configs |
| `tb mem init/list/add/check` | OK; 4 invariants; retention 1.00 | helped | `mem add --type` still rejected (docs no longer sell it as implemented) |
| Ticket / Scope | Manual `rd.md`; `tb ticket` nonexistent | neutral | Docs mark Phase 3+; Markdown parser worked |
| `tb graph rebuild .` | 1956/5584; calls present | **helped** | Under `/tmp/click-campo2` (~0.21s). Search found `_LazyFile` |
| `tb graph` in dir `target` | real: nodes>0; symlink: 0 | helped / nuance | Root SkipDirs fixed; root symlink still blind |
| `tb graph impact/callers` | useful impact; mixed symbol callers | helped | `_LazyFile` callers empty; `open_stream`/`format_filename` OK |
| `tb verify run` | PASS with build+lint+visible+hidden | **helped** | After not protecting `utils.py` in the task’s guard.yaml |
| `tb verify --ruler visual` | PASS `skipped=true` | neutral | No screen manifest |
| `tb guard check A..B --strict --json` | exit 1 with violation | **helped** | Syntax is a range, not `--base` |
| `tb drift` / `tb review` | Scope OK; review without FP | **helped** | Literal `--base HEAD^` → false `base_mismatch`; full SHA OK |
| `tb bench run` / `compare --agent` | default_agent and `--agent` OK | helped partial | Needed to copy `bench/tasks` from toolbelt; init does not seed tasks |
| `tb mcp` | tools/list + `tb_version` + search OK | helped partial | Without `TB_GRAPH=on`, `tb_graph_search` `isError` |
| Native checks vs tb | suite/ruff/mypy OK | — | tb covered configured build/lint/test; did not replace mypy |

## Problems that persist

| problem (round 1 / synthesis) | status this round | evidence |
|-------------------------------|-------------------|----------|
| `version` → `unknown` without ldflags | persists | `commit: unknown`, `built: unknown` |
| Config mode `0600` | persists | `stat`: `600` on `config.yaml` and `AGENTS.md` |
| MCP graph requires `TB_GRAPH=on` | persists (by ADR-011 design) | without env → isError |
| No `tb ticket` | persists (now documented) | `docs/cli.md` “Não implementado” |
| Init does not create `bench/tasks` | persists | `bench list` fails until fixtures are copied |
| Review/guard still need a disciplined operator | partial | Guard.yaml protecting the task file aborts verify early (correct anti-cheat, but hinders flow if the manifest is too naive) |

## New problems

1. **P2 — `filepath.Walk` on symlink root → graph 0 nodes**  
   - Repro: `ln -s /tmp/click-campo2 /tmp/target && cd /tmp/target && TB_GRAPH=on tb graph --enable rebuild .` → `0 nodes`. Real dir `/tmp/target` with sources → >0.  
   - Expected: document or resolve the root symlink before the walk.  
   - Actual: silence with an empty index.

2. **P2 — `tb drift --base HEAD^` compares the string, not the resolved SHA**  
   - Repro: ticket with `base_commit: <sha>` + `--base HEAD^` → `base_mismatch: true` (`actual_base: "HEAD^"`).  
   - Expected: resolve the git ref before comparing.  
   - Actual: false mismatch; with expanded SHA, OK.

3. **P3 — `init` reads `CLAUDE.md` but does not extract command hints**  
   - Repro: `CLAUDE.md` with “use pytest -q” → `hints: []`.  
   - Expected: test/lint tips when the text is obvious, or document limits.  

4. **P3 — `build_cmd`/`lint_cmd` only in YAML, no CLI flags**  
   - Repro: `tb verify run --build-cmd …` → `unknown flag`.  
   - Expected: flags mirroring `test_cmd`/`hidden-test-cmd`, or explicit docs that only YAML counts.

5. **P3 — callers by class name `_LazyFile` empty**  
   - Repro: `graph callers --function _LazyFile` → `(vazio)` despite `calls` in the index and instances in code/tests.  
   - Expected: resolve constructor/`Class(` or a coverage message.

## What tb caught and what it let slip

**Caught**
- Protected path via `guard.yaml` (`utils.py` / `core.py`) and `--strict --json` exit code.
- Drift/review with `## Scope` + `**` globs; ticket `tests/hidden` FP **did not** return.
- Verify running build + lint + visible + hidden without a `tests/hidden/` folder.
- Python graph with thousands of `calls`; impact on `utils.py` pointed at consumers.
- Init with `uv run pytest` and `AGENTS.md` seed.
- Bench with `default_agent` and `compare --agent`.

**Let slip / did not help**
- Did not guide task choice or ticket writing (missing `tb ticket`).
- Mute graph if cwd is a symlink named `target`.
- False `base_mismatch` with relative refs (`HEAD^`).
- No mypy/typecheck in the default pipeline (only what I configured in lint/build).
- Empty CLAUDE.md `hints`.

## Interaction with existing instrumentation / onboarding

Repo **without** prior instrumentation. With `--write-agents`:

| Artifact | Quality |
|----------|---------|
| `.toolbelt/config.yaml` | Correct `test_cmd: uv run pytest`; I had to fill `build_cmd`/`lint_cmd`/`hidden`/`default_agent` by hand |
| `AGENTS.md` | Useful skeleton pointing at verify/guard/mem |
| `agent_docs.yaml` | Recorded `AGENTS.md`; empty hints |
| `bench/` | empty dirs; tasks **not** created |
| `guard.yaml` | Not generated by init; I created it manually and it was honored |

Onboarding much better than round 1 (no longer Go default), but still missing ticket/bench seed and hint extraction.

## Comparison with round 1

| Aspect | Round 1 | Round 2 | Verdict |
|--------|---------|---------|---------|
| Init toolchain | `go test ./...` | `uv run pytest` | **better** |
| Graph under `/tmp/target` | 0 nodes | real dir OK; symlink 0 | **better** (with nuance) |
| Python `calls` / impact | empty | 2815 calls; useful impact | **better** |
| guard.yaml | ignored | read + strict JSON | **better** |
| Review FP `tests/hidden` | critical FP | empty findings | **better** |
| Markdown Scope / `**` | fragile | `## Scope`/`## Escopo` OK | **better** |
| hidden without dir | did not run | runs / fails for real | **better** |
| build/lint in verify | inert | honored in YAML | **better** |
| bench default_agent / compare `--agent` | broken | works | **better** |
| AGENTS.md | neither read nor generated | `--write-agents` + read | **better** |
| `tb ticket` / version unknown / 0600 / TB_GRAPH | pain | same | **same** |
| Completing the task | despite tb | tb helped more (verify/graph/guard) | **better** |

## Improvement suggestions

1. **[P2][graph]** Resolve `EvalSymlinks` at the root before the walk (or warn if the root is a symlink).  
2. **[P2][drift]** Resolve git refs (`HEAD^`, tags) before comparing with `base_commit`.  
3. **[P2][init]** Seed a minimal `bench/tasks` or fail with “copy fixtures / run in the toolbelt repo”.  
4. **[P3][init]** Extract command hints from `AGENTS.md`/`CLAUDE.md` or document that only paths are recorded.  
5. **[P3][verify]** `--build-cmd` / `--lint-cmd` flags mirroring YAML.  
6. **[P3][graph]** Better caller resolution for Python classes / `__init__`.  
7. **[P3][init]** `rd.md` template with `## Scope` to close the ticket→drift→review loop.

## Metrics

| Metric | Round 2 | Round 1 (click) |
|--------|---------|-----------------|
| Session wall time (clone→report) | ~tens of minutes | similar |
| Instrumented exercise sections | 41 | — |
| Approx. session `tb` invocations | ~55–65 (init/mem/graph/verify/guard/drift/review/bench/mcp + retests) | 48 |
| exit ≠ 0 failures (several intentional) | ~13 markers + flag/docs errors | 19 |
| Sum of timed durations in the log | ~1.8s partial + verify/pytest ~seconds + bench compare ~1.4s | ~7.3s |
| Slowest | `bench compare --agent` ~1.1s; native pytest suite ~3.7–5s; `graph rebuild` ~0.21s | bench compare fake ~5.1s |
| Remaining relevant product failures | symlink-root→0 nodes; `HEAD^` base_mismatch; empty hints; no build/lint flags | SkipDirs; dead guard.yaml; Go init; review FP; no calls |

Local artifacts (not in toolbelt): `/tmp/tb-campo2-click-evidence.log`, clone `/tmp/click-campo2` with experimental commit `a3160c8` (not published upstream).
