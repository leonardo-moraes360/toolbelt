🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/pydantic-ai.md)

# Field round 2: pydantic/pydantic-ai

Field study of `tb` (local `make build`, version `a46c295`) against a local clone of [pydantic/pydantic-ai](https://github.com/pydantic/pydantic-ai) at `/tmp/campo2-pydantic-ai/repo`. No push, issues, PRs, or comments on the target. Group: **with existing AI-agent instrumentation**.

> Method note: local toolbelt `main` was at `8b5a499` (missing PRs #33–#40). After `git fetch origin main` + rebuild, the binary used for all evidence below is `toolbelt a46c295`.

## Repository

| Field | Value |
|-------|-------|
| Name | `pydantic/pydantic-ai` |
| Language | Python ≥3.11 (`uv` workspace: `pydantic-ai-slim`, `pydantic-graph`, `pydantic-evals`, `pydantic-ai-harness`, `clai`, …) |
| Size (shallow clone) | ~508 MiB excluding `.venv`; ~6996 tree entries (excl. `.venv`/`.git`/`.toolbelt`); ~2528 `.py` |
| Study base commit | `c7586d7` (main on the clone) |
| Existing AI instrumentation | `AGENTS.md` + `CLAUDE.md` symlink; `AGENTS.md`/`CLAUDE.md` in subpackages; `.agents/skills/` (13 skills) and `.claude/`; `.gemini/config.yaml`; `agent_docs/`; etc. |
| Toolchain | **Yes, partial.** `uv` 0.13.8; `make install` ok; `make lint` ok; `uv run pytest` on the target file ok; `pyright` on the changed file: 0 errors. **Not run:** full `make test` suite / multi-Python matrix; LLM provider keys (irrelevant to the task). |

## Chosen task

| Field | Value |
|-------|-------|
| What | Extend the `google_api_key` pattern in `pydantic_ai_harness.guardrails.detectors` to redact Gemini keys with prefix `AQ.` (besides legacy `AIza`), with regression tests |
| Why | Small bug, no LLM, one module + tests; MRE in the issue; different from round 1 |
| Issue | https://github.com/pydantic/pydantic-ai/issues/13550 (label `needs-maintainer-discussion`; bot recommends adding `AQ.` to defaults) |
| Complete? | **Yes, locally.** Local branch `campo/fix-aq-prefix` @ `307b3c8` (never pushed). Diff: `detectors.py` + `tests/harness/guardrails/test_detectors.py` + `rd.md` |
| Native checks | Guardrails-file `pytest`: pass (incl. 2 new); `make lint` pass; `pyright` on the file: 0 errors. Full suite: **not** |
| Round 1 (comparison) | `MessagesBuilder.last_modified` (issue #13535) — distinct task |

## Verification of the 12 fixes

| # | fix | status | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Under `/tmp/target` (real dir with `pkg/mod.py`): `tb graph --enable rebuild` → **4 nodes / 4 edges**, `search hello` found `func:pkg.hello`. Before the fix, basename `target` zeroed the graph. |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; `hidden_test_cmd` in YAML + `--hidden-test-cmd …`: JSON with `hidden_available: true`, `hidden_passed: true`, verdict `PASS` (~14 s). |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | With `protected_paths` pointing at the fix file: `--json` alone exit **0** (violations present); `--strict --json` exit **1** + valid JSON with `protected_path`. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with the fix path → violation; switching to `secrets/never/**` → `violations: null`, `--strict --json` exit 0. Hardcoded `tests/hidden/*` also caught `tests/hidden/cheat.py`. |
| 5 | init toolchain detection | **worked** | `tb init` → `verify.test_cmd: uv run pytest` (no longer `go test ./...`). `hidden_test_cmd` came `""` (better than Go default). |
| 6 | review `tests/hidden` by path | **worked** (partial) | Ticket that only *mentions* `tests/hidden/**` under Forbidden: no invariant finding from substring. Real touch of `tests/hidden/cheat.py`: guard catches; review/drift mark `out_of_scope` — but `forbidden_hit` stayed `null` and there was no explicit finding from mem invariant `tests/hidden/`. |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Under `/agent/repos/toolbelt` with `bench.default_agent: "./bin/fake-agent"`: `tb bench run --task bugfix-go -r 1` **without** `--agent/--fake` → 100% success. `tb bench compare … --agent ./bin/fake-agent` exit 0. Help documents `--agent` on compare. |
| 8 | review/drift parses `## Scope`/`## Escopo`, lists and `**` | **worked** (partial) | `## Scope` + bullets: clean drift when scope covers the 3 files. `## Escopo` parsed. Glob `**/*detectors*` kept the `.py` in-scope (only `rd.md` out). But `Forbidden: tests/hidden/**` **did not** populate `forbidden_hit` when touching the dir. |
| 9 | verify `build_cmd` / `lint_cmd` | **worked** | PASS with `build_ran/lint_ran/passed: true`. With `lint_cmd: false` → `lint_passed: false`, verdict `ESCALATE`, exit 1 (3 iterations). |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** | Init logged `documentação de agentes encontrada` + `agent_docs.yaml` (sources + hints: make/pytest/ruff/uv…). `--write-agents` **did not** overwrite existing `AGENTS.md` (exit 0). |
| 11 | graph Python `calls` edges | **worked** (partial) | `edges_calls: 201764` (round 1: empty impact/callers). `impact --file …/detectors.py` listed `_capability.py`, `test_detectors.py`, funcs. `callers --function secret_data` listed tests. `callers --function redact_secrets` still `(vazio)` (incomplete name/resolution). |
| 12 | aggregated visual skipped + CLI docs | **worked** | Missing manifest: `skipped: true`. Manifest with missing ref: `skipped: true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` marks `tb ticket`/`board` as planned and documents `## Scope` / `hidden_test_cmd`. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; Python/`uv` detected; read AGENTS.md | **helped** | `test_cmd: uv run pytest`; `agent_docs.yaml` with Makefile hints |
| `tb init --write-agents` | OK; does not overwrite | neutral | Exit 0; AGENTS.md intact (already existed) |
| `tb mem init/add/list/check` | OK | neutral | 3 default anti-cheat + 1 task rule; `mem check` ok |
| `tb graph --enable rebuild` | OK (~11–12 s); 71000 nodes / 336596 edges | **helped** | Includes mass `calls` |
| `tb graph search` | Found `redact_secrets` | helped | Quick location |
| `tb graph impact --file …/detectors.py` | Lists related files/funcs | **helped** | Included `tests/harness/guardrails/test_detectors.py` (in round 1 impact was empty) |
| `tb graph callers --function redact_secrets` | `(vazio)` | hindered | Despite existing `calls`; `secret_data` works |
| `tb graph` under `/tmp/target` | 4 nodes | helped | Root SkipDirs fixed |
| Ticket `rd.md` (`## Scope` + `**`) | Clean drift/review after including `rd.md` in scope | helped | First version flagged `rd.md` out_of_scope (parser ok) |
| `tb verify run` | PASS with build+lint+visible+hidden | **helped** | Full JSON; hidden without `tests/hidden/` dir |
| `tb verify --ruler visual` | PASS skipped | neutral | Correct `skipped: true` aggregation |
| `tb review` | Clean on the fix; catches scope drift | helped | No FP from mentioning `tests/hidden` in the ticket |
| `tb drift` | OK in scope; fails if a file is out | helped | `forbidden_hit` still fragile with `**` |
| `tb guard check A..B` | Reads `guard.yaml`; `--strict --json` exit 1 | **helped** | UX: positional `base..head` range (not `--base`) |
| `tb bench run` / `compare --agent` | OK on the toolbelt repo with default_agent | neutral | Target has no `bench/tasks`; harness uses toolbelt fixtures |
| `tb mcp` | tools/list + `tb_version` OK | neutral | Tools: graph_* + echo/env/run/version — no mem/verify/review/drift/guard |

## Problems that persist

From round 1 (`docs/campo/pydantic-ai.md`), what **still** shows up:

| round 1 problem | round 2 status |
|-----------------|----------------|
| Init Go defaults | **fixed** (`uv run pytest`) |
| `hidden_test_cmd` gated by dir | **fixed** |
| Ignored `guard.yaml` | **fixed** |
| `default_agent` / compare without `--agent` | **fixed** |
| Does not read AGENTS.md | **fixed** (read + hints; does not import skills) |
| Python without `calls` / empty impact | **improved** (useful calls + impact); `callers redact_secrets` still empty |
| Verify without build/lint | **fixed** when configured |
| Outdated CLI docs (`ticket`/`board`) | **improved** (marked planned); tree still lists Phase 3 commands |
| Config mode `0600` | **persists** (`config.yaml` mode 600) |
| MCP without CLI parity | **persists** |
| tb does not import skills / does not push AGENTS.md checklist | **persists** (only superficial command hints) |
| `graph impact` requires `--file` | **persists** (UX) |

## New problems

| ID | sev | repro | expected | actual |
|----|-----|-------|----------|--------|
| N1 | P2 | Ticket with `## Forbidden` / `- tests/hidden/**`; commit that adds `tests/hidden/cheat.py`; `tb drift`/`tb review` | `forbidden_hit` filled (and/or mem invariant finding) | Only `out_of_scope`; `forbidden_hit: null`; no finding from invariant `tests/hidden/` |
| N2 | P2 | `tb graph callers --function redact_secrets` after rebuild with 200k+ `calls` | Callers or a disambiguation message | `(vazio)`; `secret_data` in the same file works |
| N3 | P3 | `tb guard check --base HEAD~1 --head HEAD` (verify-style flags) | Accept or clear error pointing at `base..head` | `unknown flag: --base` |
| N4 | P3 | `tb init` in a repo with rich AGENTS.md | Actionable hints wired to `verify.lint_cmd`/`typecheck` | Hints in `agent_docs.yaml`, but `lint_cmd`/`build_cmd` not filled; agent still configures by hand |
| N5 | P3 | Shallow clone without a local commit; `tb verify run` without `--base/--head` | Clear message | Opaque `Error: exit status 128` (git) |

## What tb caught and what it let slip

**Caught**
- Drift when `rd.md` or `tests/hidden/cheat.py` were outside `## Scope`.
- `guard.yaml` protected_paths and hardcoded `tests/hidden/*`.
- `lint_cmd` failure → verify does not PASS.
- Configured hidden suite without a `tests/hidden/` folder.
- Impact suggesting the test file to touch.
- Honest visual skip (`skipped: true`).

**Let slip**
- `forbidden_hit` for glob `tests/hidden/**` in the ticket.
- Mem invariant finding when editing `tests/hidden/` (only scope drift).
- Soft completeness: clean review without checking whether maintainer discussion / guardrails docs were updated (here the local fix follows the MRE; the point is tb does not know the `needs-maintainer-discussion` label).
- Did not demand global `make typecheck` — only what was in `lint_cmd`/`test_cmd`.

## Interaction with existing instrumentation / onboarding

- **tb reads** `AGENTS.md`/`CLAUDE.md` in init → `.toolbelt/agent_docs.yaml` (sources + `make`/`pytest`/`ruff`/`uv` hints).
- **tb does not import** `.agents/skills/*`, nor AGENTS.md narrow-scope / type-safety / PR-template rules into `mem` or the review checklist.
- **`--write-agents`**: correct not to overwrite; useful only when instrumentation is absent.
- **Residual conflict:** tb default invariants (`tests/hidden/`, thresholds) vs real workflow (skills `initialize-worktree`, `branch-context`, pre-commit `no-commit-to-branch`). They coexist without merge.
- Onboarding better than round 1 (toolchain + agents docs), but the agent still must read AGENTS.md outside tb for the project standard.

## Comparison with round 1

| dimension | round 1 | round 2 | verdict |
|-----------|---------|---------|---------|
| Init / verify defaults | `go test ./...` | `uv run pytest` | **better** |
| Hidden tests | ignored without dir | run if cmd set | **better** |
| Custom guard | dead YAML | `protected_paths` + strict JSON | **better** |
| AGENTS.md | ignored | read + hints | **better** |
| Python graph | empty impact/callers | useful calls + impact | **better** |
| Bench flags | `--agent` missing on compare; dead default | both ok | **better** |
| Markdown Scope | fragile | `## Scope`/`## Escopo` + `**` on code paths | **better** |
| Forbidden `**` / hidden invariant | — | `forbidden_hit` null | **new/residual hole** |
| MCP parity | incomplete | same | **same** |
| Local task | complete | complete (other issue) | **same** |

Overall: **clearly better** than round 1 in this repo; the 12 fixes confirm in practice, with caveats on name-based callers, forbidden globs, and depth of AGENTS.md import.

## Improvement suggestions

1. **[P1 · drift/review]** Make `## Forbidden` with `**` populate `forbidden_hit`; align mem invariant `tests/hidden/` with the same path matcher as guard.  
2. **[P1 · graph]** Resolve `callers --function` by suffix/title when there is a single Python symbol (`redact_secrets`), or list candidates.  
3. **[P1 · init]** Map AGENTS.md/`Makefile` hints → `verify.lint_cmd` / typecheck (not only `test_cmd`).  
4. **[P2 · guard UX]** Accept `--base`/`--head` besides `A..B`, mirroring verify/review.  
5. **[P2 · verify]** Translate git `exit status 128` into “create a commit or pass --base/--head”.  
6. **[P2 · mcp]** Expose mem/verify/review/drift/guard.  
7. **[P3 · mem]** Import 3–5 textual invariants from AGENTS.md (project anti-patterns) in `tb init`.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Wall time (sum of EXIT/TIME in structured log) | ~74 s of timed invocations; session ~40–50 min incl. fetch/rebuild/clone/`make install` | ~57 s invocations; ~25–30 min |
| tb invocations (+ natives in the log) | ~54 sections; ~48 with EXIT/TIME | ~53 parseable |
| tb failures (exit ≠ 0) in the log | 12 (includes expected negatives: intentional drift, guard strict, wrong flags, lint fail) | 26 |
| Crash/segfault | none | none |
| Binary | `a46c295` | `c8aae04` |
| Task delivered on target | yes (local only); pytest AQ./AIza 2/2 + guardrails file pass | yes (local only) |
