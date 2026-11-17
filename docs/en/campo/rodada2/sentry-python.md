[🇧🇷 Português](../../../campo/rodada2/sentry-python.md) | 🇺🇸 English

# Field round 2: getsentry/sentry-python

ROUND 2 field study of `tb` (local `make build`, ldflags `a46c295`) on a local clone of `https://github.com/getsentry/sentry-python`. Group: **with existing AI instrumentation**. No push, issues, PRs, or comments upstream. Evidence under `/tmp/tb-campo2-evidence/` on this VM.

## Repository

| Field | Value |
|-------|-------|
| Name | getsentry/sentry-python |
| Language | Python (SDK) |
| Size | ~19 MiB shallow clone; ~472 MiB with `.venv`/`.tox`; ~523 `.py` outside venv/tox; GitHub `size` ≈ 123 MiB; ~2220 stars |
| Existing AI instrumentation | `AGENTS.md` (source), `CLAUDE.md` → symlink to `AGENTS.md`, `agents.toml` (getsentry skills) |
| Toolchain | **Partially configured**: `uv` 0.13.8; `uv sync` (dev: tox, tox-uv, ruff, pre-commit); `uv pip install -e .`; tox env `py3.12-google_genai-v1.29.0` functional; `uv run ruff` ok; `uv run --group typing mypy` ok on the touched file |
| Not configured | full tox matrix (Django/Flask/AWS/MCP/agents etc.), Python 3.6–3.11/3.13+/free-threaded, gevent/eventlet, pre-commit hooks on the target git, Sphinx docs |

## Chosen task

| Field | Value |
|-------|-------|
| What | In `accumulate_streaming_response` (google_genai), populate `response_id` / `model` from `chunk.response_id` and `chunk.model_version`; assertions in `test_streaming_generate_content` for `GEN_AI_RESPONSE_ID` / `GEN_AI_RESPONSE_MODEL`. |
| Why | Real bug, small scope, clear test path; distinct from round 1. |
| Issue | https://github.com/getsentry/sentry-python/issues/5812 (read-only; no interaction) |
| Completed? | **Yes on the local clone** (commit `e8b58b5` on branch `local/campo2-google-genai-5812`; **not** sent upstream) |
| Native result | `TESTPATH=…/test_google_genai.py uv run tox -e py3.12-google_genai-v1.29.0 -- -k 'test_streaming_generate_content and not data_collection and not tools and not message'`: **4 passed**. `ruff check` / `ruff format --check` on touched files: ok. `mypy` on `streaming.py`: ok. |
| Round 1 (comparison) | Round 1: `EventScrubber` hyphen/underscore + denylist (`issues/8272`). This round: google_genai streaming `response_id`/`model_version` (`issues/5812`). |

Local files changed: `sentry_sdk/integrations/google_genai/streaming.py`, `tests/integrations/google_genai/test_google_genai.py`.

## Verification of the 12 fixes

| # | fix | status | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph --enable rebuild .` under `/tmp/target` → `8053 nodes, 25307 edges` (before: 0 nodes). Full rebuild on the working clone → 8072 nodes / 26373 edges. |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; `tb verify run` with `hidden_test_cmd=uv run ruff check …` → `hidden_available: true`, `hidden_passed: true`. |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff touching `tox.ini`: `tb guard check --json --strict BASE..HEAD` → exit **1** + JSON with `violations`. Without `--strict`: exit 0 with the same violations. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `tox.ini`; `protected_path` violation on `tox.ini` reported. Defaults (`thresholds.yaml`) remain active. |
| 5 | init toolchain detection | **worked** (partial) | `tb init` → `verify.test_cmd: uv run pytest` (no longer `go test`). Still **does not** choose `uv run tox` preferred by `AGENTS.md`. |
| 6 | review `tests/hidden` by path | **worked** | Ticket citing `tests/hidden/` without touching the dir → `findings: []` (no substring FP). |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Without `--agent`/`--fake`, with `bench.default_agent: "true"` in `.toolbelt/config.yaml` → run OK (`agent=external`). Without default → error asking for `--agent`/`--fake`/`default_agent`. `tb bench compare` accepts `-a/--agent` + `--config-a/--config-b`. |
| 8 | review/drift `## Scope`/`## Escopo`, lists, `**` | **worked** | Ticket with `## Escopo` + bullets + `tests/integrations/google_genai/**/*.py` → drift OK. Scope with only `streaming.py` → flagged the test (`out_of_scope`). |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | JSON: `build_ran: true`, `build_passed: true`, `lint_ran: true`, `lint_passed: true` + visible/hidden. Failed build (missing google-genai import in root `.venv`) aborted before tests (`step: build_or_lint`) — desired behavior. |
| 10 | init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents` | **worked** | Created `.toolbelt/agent_docs.yaml` (`sources: AGENTS.md, CLAUDE.md` + tox/ruff/mypy hints). `tb init --write-agents` in an empty dir generates a skeleton; on the existing repo **does not** overwrite `AGENTS.md` (empty diff). |
| 11 | graph Python `calls` edges | **worked** | Stats: `edges_calls: 12793` (full) / `4882` (`sentry_sdk` only). `tb graph callers --function accumulate_streaming_response` → `new_iterator` / `new_async_iterator`. `impact --file …/streaming.py` lists wrappers. |
| 12 | aggregated visual skipped + CLI docs | **worked** | Visual without manifesto: `skipped: true`. With screens and missing refs: top-level `skipped: true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` marks `tb ticket`/`tb run`/etc. as planned and documents Scope/hidden/guard.yaml/default_agent. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; `.toolbelt/` + `config.yaml` + `agent_docs.yaml` | helped | Detected Python/`uv`; read AGENTS/CLAUDE; `test_cmd=uv run pytest` (better than Go; still suboptimal vs tox) |
| `tb init --write-agents` | OK | helped | Seeds AGENTS in an empty repo; does not overwrite on sentry-python |
| `tb mem init/list/add/check` | OK | neutral | 3 default invariants + 1 (`tox.ini`); still weakly tied to the native flow beyond manual add |
| Ticket / `## Escopo` (`rd.md`) | Manual | helped | Markdown+globs parser worked in drift/review |
| `tb graph --enable rebuild .` under `/tmp/target` | 8k+ nodes | helped | Root SkipDirs fixed |
| `tb graph search accumulate_streaming_response` | Found symbol | helped | Fast location |
| `tb graph callers --function …` | 2 callers | helped | Useful Python `calls` (previously empty) |
| `tb graph impact --file …` | Impact list | helped (partial) | Empty if the index was built with a cwd/`path` whose file prefix does not match `--file` (e.g. rebuild `sentry_sdk` + path with `sentry_sdk/…`) |
| `tb graph deps --file …` | 9 packages | helped | After rebuild aligned to the path |
| `tb drift` | exit 0 on full scope; exit 1 on narrow | helped | Caught test outside the mandate |
| `tb review` | `findings: []` on clean patch | neutral | No semantic Python review; scope/invariant heuristics OK |
| `tb guard check … --json --strict` | exit 1 with `tox.ini` / thresholds | helped | `guard.yaml` + defaults |
| `tb verify run` | PASS with build/lint/test/hidden | helped | Ran streaming tox + ruff; failed early with invalid build_cmd |
| `tb verify run --ruler visual` | PASS skipped | neutral | Correct `skipped` aggregation; no product signal in this SDK |
| `tb bench run` (default_agent) | OK with toolbelt tasks | neutral | No `bench/tasks` on the target; needs external `-t` or copied fixtures |
| `tb bench compare --agent` | OK A/B graph-off/on | neutral | `--agent` flag present and usable |
| `tb mcp` | `tools/list` OK | neutral | version/echo/env/run/graph_*; no mem/verify/review/drift/guard |
| Native checks vs tb | tox 4 passed; ruff/mypy ok | helped | tb verify mirrored the native result when `test_cmd` pointed at tox |

## Problems that persist

From round 1 (`docs/campo/sentry-python.md`) that **still** appear:

| round 1 problem | round 2 status |
|-----------------|----------------|
| Init/defaults poorly aligned with `AGENTS.md` (tox via uv) | **persists** (improved: no longer `go test`; still `uv run pytest` instead of tox) |
| `agents.toml` ignored | **persists** |
| MCP without mem/verify/review/drift/guard | **persists** |
| Review without semantic analysis of the Python patch | **persists** (scope/invariants only) |
| Config files mode `0600` | **persists** (`config.yaml`/`agent_docs.yaml`/`graph.json` 600) |
| Version `unknown` without ldflags | **not retested** with raw `go build`; with `make build` ok (`a46c295`) |
| Fake/`bench` without tasks on the target repo | **persists** (must point at external fixtures) |
| tb does not “push” task completeness alone | **partly mitigated** by scope drift; still no acceptance-checklist harness |

Round 1 problems **fixed** in this repo: SkipDirs/`/tmp/target`, Python `calls`, gated `hidden_test_cmd`, ignored `guard.yaml`, `--strict --json` exit 0, dead `default_agent`, `## Scope` parser, substring FP `tests/hidden`, verify without build/lint, visual `skipped=false`, outdated CLI docs (planned marked), init not reading AGENTS.

## New problems

### P2 — `graph impact`/`deps` depend on the path exactly as indexed

- **Repro**: `tb graph --enable rebuild sentry_sdk` then `impact --file sentry_sdk/integrations/google_genai/streaming.py` → `(vazio)`. With `--file integrations/google_genai/streaming.py` (or rebuild `.` + full path) → useful result.
- **Expected**: tolerant resolution of package prefix / cwd.
- **Actual**: literal path match in the index → false sense of “no impact”.

### P2 — Detected `verify.test_cmd` does not follow the harness documented in AGENTS.md

- **Repro**: `tb init` on sentry-python → `uv run pytest`; AGENTS.md says `uv run tox -e …`.
- **Expected**: prefer tox when `tox.ini` + AGENTS text mention tox.
- **Actual**: pyproject/uv heuristic → pytest; hints go to `agent_docs.yaml` but do not become `test_cmd`.

### P3 — Absolute `bench.tasks_dir` in YAML can be ignored / relative defaults

- **Repro**: write `bench.tasks_dir: /agent/repos/toolbelt/bench/tasks` in the target config; `tb bench run -r 1` still tried relative `bench/tasks` until `-t` was passed on the CLI.
- **Expected**: honor absolute YAML paths.
- **Actual**: agent must discover the `-t` flag (docs help, but YAML alone failed in this setup).

### P3 — MCP still without parity with the field flow

- **Repro**: `tools/list` after init — no mem/verify/review/drift/guard tools.
- **Expected**: at least verify/drift/guard for already-instrumented repos.
- **Actual**: only utilities + graph_*.

No P0 regression observed on the 12 fixes in this exercise.

## What tb caught and what it let slip

**Caught**
- Drift: test outside the narrow `## Escopo`.
- Guard: change to `tox.ini` via `guard.yaml` and to `.toolbelt/thresholds.yaml`.
- Graph: search + callers of `accumulate_streaming_response`; impact after aligned path.
- Verify: build/lint/hidden/test; early failure when `build_cmd` imported the integration without `google-genai` in the root venv.
- Review: no FP when mentioning `tests/hidden` in the ticket.

**Let slip / did not help**
- Did not suggest the “right” tox `test_cmd` from AGENTS.md (agent had to configure).
- Review did not critique patch design/completeness beyond scope.
- Without bench tasks on the target, benchmark only works with external fixtures.
- `agents.toml` skills did not enter the flow.

## Interaction with existing instrumentation / onboarding

| Repo artifact | What tb did |
|---------------|-------------|
| `AGENTS.md` / `CLAUDE.md` | **Read** → `.toolbelt/agent_docs.yaml` with sources + hints (tox, ruff, mypy, examples). **Did not** promote hints to `verify.test_cmd` automatically. `--write-agents` does not overwrite. |
| `agents.toml` | **Ignored**. |
| `tb init` onboarding | Created vault/logs/bench + config with Python/`uv` toolchain. Better than round 1; tox vs pytest gap remains. |
| Conflict | Default invariants still talk about `tests/hidden`/thresholds; AGENTS talks about tox/ruff/mypy — they now coexist with recorded hints, but without automatic merge into `verify.*`. |

## Comparison with round 1

**Better**, in concrete ways:

| Area | Round 1 | Round 2 |
|------|---------|---------|
| Graph under `/tmp/target` | 0 nodes | thousands of nodes |
| Python `calls` / callers | empty | useful callers and impact |
| `hidden_test_cmd` | no-op without `tests/hidden/` | runs (ruff ran) |
| `guard.yaml` | ignored | `tox.ini` flagged |
| `--strict --json` | exit 0 with violation | exit 1 |
| Init | `go test` + AGENTS ignored | `uv run pytest` + `agent_docs.yaml` |
| Markdown Scope | fragile / literal paths | `## Escopo` + `**` OK |
| Verify build/lint | effectively absent | `build_ran`/`lint_ran` |
| Visual skipped | top-level `skipped=false` | aggregated `skipped=true` |
| CLI docs | phantom commands | marked as planned |

Same level: incomplete MCP; weakly semantic review; bench depends on external fixtures; `agents.toml` off the radar.

## Improvement suggestions

1. **[P1][init/verify]** Map `AGENTS.md`/`tox.ini` hints → `verify.test_cmd`/`lint_cmd` (e.g. if tox.ini exists and AGENTS cites `uv run tox`, prefer that over `uv run pytest`) — areas `init`, `verify`.
2. **[P2][graph]** Resolve `--file` with flexible suffix/prefix (accept `sentry_sdk/…` and `…` relative to the indexed subdir) — area `graph`.
3. **[P2][bench]** Honor absolute YAML `tasks_dir`/`fixtures_dir`; clear error if dir missing — area `bench`.
4. **[P2][init/mem]** Optional: import AGENTS bullets into invariants (beyond `agent_docs.yaml`) — areas `init`, `mem`.
5. **[P3][mcp]** Expose drift/review/verify/guard/mem on MCP — area `mcp`.
6. **[P3][init]** Read `agents.toml` (skill names) as optional review hooks — area `init`/`review`.

## Metrics

| Metric | Round 2 | Round 1 (reference) |
|--------|---------|---------------------|
| Wall time (sum of timed commands in `summary.tsv`) | ~28 s of recorded `tb`/probes; larger total session (~includes tox ~6 s/run + mypy ~10 s + uv/tox setup) | ~10–12 min session; evidence span ≈ 577 s |
| Recorded invocations | **60** lines in `summary.tsv` (+ mcp/version outside) | **38** |
| exit≠0 failures | **17** (expected probes: narrow drift, guard strict, bad flags, bench without tasks/agent, ruff format before fix) | **13** |
| Crashes (panic/segfault) | **0** | **0** |
| `tb` build | `make build` OK (`a46c295`) | `make build` OK (`c8aae04`) |
| MCP tools | 8 (version/echo/env/run/graph_*) | 8 |
| Local commits on target | 1 product (+ probes on disposable branch); **0** push | 2; **0** push |
| This report’s branch | `campo2/sentry-python` (only `docs/campo/rodada2/sentry-python.md`) | `campo/sentry-python` |
