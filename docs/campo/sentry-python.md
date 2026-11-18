🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/sentry-python.md)

# Field: getsentry/sentry-python

Field study of `tb` (local `make build`, version with ldflags `c8aae04`) using the agent flow on a local clone of `https://github.com/getsentry/sentry-python`. No push, issues, PRs, or comments upstream. Evidence under `/tmp/tb-campo-evidence/` on this VM.

## Repository

| Field | Value |
|-------|-------|
| Name | getsentry/sentry-python |
| Language | Python (SDK) |
| Size | ~19 MiB shallow clone; ~429 MiB after `.venv`/`.tox`; ~523 `.py` outside venv/tox; GitHub `size` ≈ 119 MiB; ~2220 stars |
| Existing AI instrumentation | `AGENTS.md` (source), `CLAUDE.md` → symlink to `AGENTS.md`, `agents.toml` (getsentry skills: code-review, find-bugs, skill-scanner, security-review) |
| Toolchain | **Partially configured**: `uv` installed; `uv sync` (dev: tox, tox-uv, ruff, pre-commit); `uv pip install -e .`; tox env `py3.12-common` functional; `uv run --group typing mypy` ok for `sentry_sdk/scrubber.py`; ruff check/format ok on touched files |
| Not configured | full tox matrix (Django/Flask/AWS/MCP/agents integrations etc.), Python 3.6–3.11/3.13+/free-threaded, gevent/eventlet, pre-commit hooks installed on the target git, Sphinx docs |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `EventScrubber` to normalize hyphen/underscore on denylist keys and remove `x_forwarded_for` from `DEFAULT_DENYLIST` (stays only in PII). Update the affected legacy WSGI test. |
| Why | Real bug, small/medium scope, with a clear test path; maintainer would plausibly accept after product discussion (label Waiting for: Product Owner). |
| Issue | https://github.com/getsentry/sentry-python/issues/8272 (read-only; no interaction) |
| Completed? | **Yes on the local clone** (local commits `b372f0e`, `799876a` on branch `local/campo-scrubber-8272`; **not** sent upstream) |
| Native result | `TESTPATH=tests/test_scrubber.py uv run tox -e py3.12-common`: **15 passed**. Legacy WSGI after adjustment: **2 passed**. `ruff check`/`ruff format --check` on touched files: ok. `mypy sentry_sdk/scrubber.py`: ok. First run without `TESTPATH` (whole common suite) failed **1** test (`test_request_headers_legacy_no_pii_redacts_sensitive`) — expected from the issue itself; fixed afterward |

Local files changed: `sentry_sdk/scrubber.py`, `tests/test_scrubber.py`, `tests/integrations/wsgi/test_wsgi.py`.

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; created `.toolbelt/` + `config.yaml` | hindered | Defaults `verify.test_cmd: go test ./...` / `hidden_test_cmd: go test ./tests/hidden/...` in a Python repo; **did not** read `AGENTS.md`/`agents.toml`; config permissions `0600` |
| `tb mem init/list/add/check` | OK; 3 default invariants + 1 added | neutral | `mem add` only positional; generic invariants (`tests/hidden`, Go-ish) poorly aligned with Sentry’s AGENTS.md |
| Ticket / Scope (`rd.md`) | Manual (no `tb ticket`) | helped (when used) | Narrow scope later detected WSGI out of scope via drift/review |
| `tb graph --enable rebuild` (cwd `/tmp/target`) | **0 nodes / 0 edges** in ~8 ms | hindered | Root dir named `target` ∈ `SkipDirs` → `SkipDir` at root |
| `tb graph --enable rebuild sentry_sdk` | 2800–2801 nodes, ~4735 edges (~0.33 s) | helped (partial) | Python indexing produces only `defines`/`imports` |
| `tb graph search EventScrubber` | Found `class:scrubber.EventScrubber` after scoped index | helped | Useful to locate the symbol |
| `tb graph impact --file …` | `(vazio)` | hindered | 0 `calls` edges in the Python graph → useless impact |
| `tb graph callers --function scrub_dict` | `(vazio)` | hindered | Same (no `calls`) |
| `tb graph deps --file scrubber.py` | 3 packages (`_types`, `utils`, `typing`) | neutral | Imports only |
| `tb drift` (scope without WSGI) | exit 1; `out_of_scope: tests/integrations/wsgi/test_wsgi.py` | helped | Caught a necessary file outside the mandate |
| `tb drift` (full scope) | exit 0 | helped | Base commit matched |
| `tb review` | Findings only from scope/invariant heuristics; no semantic Python review | neutral/hindered | Goldfish did not assess fix quality; FP if the diff mentions `tests/hidden` |
| `tb guard check` | No violations on the real diff; ignore `guard.yaml` | hindered (config) | Hardcoded paths; `tox.ini` in `guard.yaml` **not** flagged |
| `tb verify run` (real tests) | PASS in ~5.5 s; `hidden_available: false` | hindered (partial) | `hidden_test_cmd` **only runs** if `tests/hidden/` dir exists; ruff passed as hidden **did not execute** |
| `tb verify run --ruler visual` | PASS with skip; top-level `skipped:false` with skipped screens | hindered (UX) | See known problems |
| `tb bench run --fake` | Runs; feature-small 0% without solution files | neutral | `--fake` without `--fake-config` does not apply patches |
| `tb bench compare --fake` (A/B graph) | OK (~0.12 s) | neutral | No `--agent` flag |
| `tb mcp` | `tools/list` OK (version/echo/env/run/graph_*) | neutral | No mem/verify/review/drift/guard tools |
| Native checks vs tb | tox/ruff/mypy caught WSGI regression and syntax; tb verify with `test_cmd=true` gave PASS with SyntaxError in code | hindered | See “What tb caught…” |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare --agent …` → `unknown flag: --agent`; help only has `--fake` / A/B configs / JSONL |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` listed `tox.ini`; diff touching `tox.ini` + thresholds + `tests/hidden` only reported **hardcoded** paths (`thresholds.yaml`, `tests/hidden/*`), not `tox.ini` |
| Outdated CLI docs | **confirmed** | `docs/cli.md` documents `tb run`, `tb proc`, `tb md`, `tb ticket`, `tb board`; `tb ticket` → `unknown command`; real help does not list them |
| Version shows `unknown` | **confirmed** (build without ldflags) | `go build -o /tmp/tb-nolink` → `commit: unknown`, `built: unknown` (version `0.1.0-dev`). With `make build`: `c8aae04` |
| Config files created `0600` | **confirmed** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` all `600` |
| Visual ruler reports `skipped=false` when skipping | **confirmed** | With example manifesto and missing refs: top-level `"skipped": false` while each screen has `"skipped": true`, `passed: true`, verdict PASS |
| `default_agent` ignored | **confirmed** | `config.yaml` with `bench.default_agent: echo-agent {prompt}`; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| Guard aborts verify when diff touches hardcoded config path | **confirmed** | Diff with `.toolbelt/thresholds.yaml` / `tests/hidden/…` → verify exit 1, `iterations: 0`, verdict FAIL without running tests |
| Default fake agent needs `--fake-config` | **confirmed** (to pass tasks with code) | Without config: `feature-small` passed=false (does not create/modify files). Default `ShouldPass:true` only emits JSON. With task `mcp-smoke` (trivial check) passed=true without config |
| `mem add` positional | **confirmed** | `tb mem add --rule foo` → `unknown flag: --rule`; usage `tb mem add <regra>` |
| Drift/Scope need literal paths (no globs) | **partially refuted** | `tests/**` works (special case); `tests/test_*.py` via `filepath.Match` works in the same segment; docs/UX still push literal paths |
| `graph impact` needs `--file` | **confirmed** | Without flag: `--file é obrigatório` |
| Review false positives on `tests/hidden` invariant | **confirmed** | Diff comment containing the string `tests/hidden` (without touching the dir) → critical finding `possível violação de invariante: NUNCA modifique arquivos em tests/hidden/` (agreement 3) |
| Verify runs only partial checks / misses build errors | **confirmed** | (1) hidden cmd ignored without `tests/hidden/`; (2) `test_cmd=true` + SyntaxError in `scrubber.py` → verify PASS; native import fails |
| tb does not push completing the whole task | **confirmed (observational, N=1)** | After the scrubber fix, the common suite still failed on WSGI; tb verify configured only on `test_scrubber.py` gave PASS; scope drift (and the issue) pointed at the gap — not a completion checklist/harness. No controlled A/B in this repo |

## New problems

### P0 — `SkipDirs` includes `target` and zeros the graph if the clone lives under `…/target`

- **Repro**: `git clone … /tmp/target && cd /tmp/target && tb init && tb graph --enable rebuild .` → `0 nós`. Control: dir `/tmp/not-target-name` indexes; `/tmp/tb-graph-bug-demo/target` → 0 nodes.
- **Expected**: skip only subdirs named `target` (e.g. Rust), not the walk root.
- **Actual**: `walkSourceFile` does `SkipDir` when `info.Name()` ∈ SkipDirs, including on the absolute root.

### P1 — Python indexer does not emit `calls` edges

- **Repro**: after `tb graph --enable rebuild sentry_sdk`, edge counts: `defines`/`imports` only; `calls=0`. `tb graph impact --file scrubber.py` and `callers --function scrub_dict` → `(vazio)`.
- **Expected**: useful impact/callers in Python (group repo).
- **Actual**: commands “work” but are always empty → false sense of coverage.

### P1 — `tb init` does not detect ecosystem / does not import AGENTS.md

- **Repro**: init on sentry-python leaves `verify.test_cmd: go test ./...`.
- **Expected**: defaults from `pyproject.toml`/`tox.ini`/`AGENTS.md` (tox via uv, ruff, mypy).
- **Actual**: an agent that trusts the default runs Go in a Python repo.

### P2 — `hidden_test_cmd` only runs if `tests/hidden/` exists

- **Repro**: verify with `--hidden-test-cmd 'uv run ruff check …'` in a repo without that dir → JSON `hidden_available: false`, ruff never ran, verdict still PASS if visible passed.
- **Expected**: configured hidden command always runs (or an explicit warning).
- **Actual**: flag silently no-op.

### P2 — Flag UX vs docs (`graph index --path`, `mem add --rule`)

- **Repro**: `tb graph index --path …` → unknown flag (positional `[path]`); `mem add --rule` → unknown flag.
- **Expected**: documented flags or errors that show the correct form without looking like a bug.

### P3 — MCP does not expose mem/verify/review/drift/guard

- **Repro**: `tools/list` → only `tb_version`, `tb_echo`, `tb_env`, `tb_run`, graph_*.
- **Expected**: parity with the CLI used in the field flow (group with agent instrumentation).

## What tb caught and what it let slip

**Caught**
- Drift/review: `tests/integrations/wsgi/test_wsgi.py` outside the initial scope.
- Guard: change to `.toolbelt/thresholds.yaml` and `tests/hidden/*`.
- Graph search: located `EventScrubber` after indexing `sentry_sdk`.
- Verify: failed (ESCALATE) when the real `test_cmd` imported an SDK with SyntaxError.

**Let slip / did not help**
- WSGI test regression while verify was limited to `tests/test_scrubber.py` (misleading PASS for the full task).
- SyntaxError with `test_cmd=true` → PASS.
- `hidden_test_cmd` (ruff) not executed.
- Empty impact/callers in Python.
- Review did not critique patch design; only scope/invariant heuristics.
- Go init/defaults; zero use of AGENTS.md.
- Useless full graph rebuild under `/tmp/target`.

## Interaction with existing instrumentation / onboarding

| Repo artifact | What tb did |
|---------------|-------------|
| `AGENTS.md` / `CLAUDE.md` | **Ignored**. Did not import rules (tox via uv, ruff, mypy, do not edit generated `tox.ini`, integration/test guidelines). |
| `agents.toml` (Sentry skills) | **Ignored**. No bridge to MCP/CLI skills. |
| `tb init` onboarding | Created empty vault/logs/bench + generic config. **Did not** create a harness-style progress log / feature list; **did not** generate or merge AGENTS.md. |
| Conflict | Default `tb mem` invariants talk about `tests/hidden/` and Go-centric thresholds; AGENTS.md talks about tox/ruff/mypy and generated files — parallel signals without merge. |
| What tb should import | Verify commands (tox common, ruff, mypy); ban on editing generated `tox.ini`/workflows; preference for e2e/`capture_events` tests; `uv run` stack; skill names from `agents.toml` as optional review hooks |

Research finding (`docs/pesquisa/achados.md`: “AGENTS.md as local context”) remains **pending** in practice.

## Improvement suggestions

1. **[P0][graph]** Do not apply `SkipDirs` to the walk root directory; rename skip `target` → more specific patterns (`**/target/release`, Cargo) — area `graph`.
2. **[P1][graph]** Extract `calls` edges (or impact fallback via `imports`/references) in the Python indexer — area `graph`.
3. **[P1][init/mem]** `tb init` detect Python and populate `verify.*` + optionally import `AGENTS.md` bullets into `invariants.yaml` / ticket template — areas `init`, `mem/ctx`.
4. **[P1][guard]** Actually load `.toolbelt/guard.yaml`; document the current divergence — area `guard`.
5. **[P1][verify]** Always run configured `hidden_test_cmd`; multi-command defaults (test + lint + typecheck); fail if only `true`/`go test` in a non-Go repo — area `verify`.
6. **[P2][verify/visual]** Propagate `skipped: true` in the report when aggregating skipped screens; do not mark `passed: true` without measurement — area `verify`.
7. **[P2][bench]** Honor `bench.default_agent`; accept `--agent` on A/B `compare`; document that `--fake` without `--fake-config` does not apply diffs — area `bench`.
8. **[P2][docs/cli]** Sync `docs/cli.md` with real commands or mark “planned”; align flags (`index [path]`, positional `mem add`) — area `docs`.
9. **[P2][review]** Invariant heuristic by touched path, not substring in the diff — area `review`.
10. **[P3][mcp]** Expose drift/review/verify/mem on MCP for repos already instrumented with agents — area `mcp`.
11. **[P3][router/harness]** Completion checklist (ticket acceptance ↔ files/tests) to push the whole task — area `verify`/`ticket`.

## Metrics

| Metric | Value |
|--------|-------|
| Wall time (timed evidence + tox) | ~10–12 min active session; evidence-file span ≈ **577 s**; largest cost = full common tox **~269 s** (1 WSGI fail) + focused re-runs |
| Recorded `tb` invocations | **38** lines in `summary.tsv` (+ builds/`mcp`/`version` nolink outside the TSV) |
| exit≠0 failures | **13** in the log (mostly expected probes: scope drift, invalid flags, verify abort, visual without manifest, default_agent, etc.); **0 crashes** (panic/segfault) observed |
| `tb` build | `make build` OK (~8 s incl. module download) |
| MCP | `tools/list` OK; 8 tools |
| Local commits on target | 2 product (+ reverted probes); **0** push |
| This report’s branch | `campo/sentry-python` (only `docs/campo/sentry-python.md`) |
