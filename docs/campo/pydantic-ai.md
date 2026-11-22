🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/pydantic-ai.md)

# Field: pydantic/pydantic-ai

Field study of `tb` (local `make build`, injected version `c8aae04`) against a local clone of [pydantic/pydantic-ai](https://github.com/pydantic/pydantic-ai) at `/tmp/target/pydantic-ai`. No push, issues, PRs, or comments on the target.

## Repository

| Field | Value |
|-------|-------|
| Name | `pydantic/pydantic-ai` |
| Language | Python ≥3.11 (`uv` workspace: `pydantic-ai-slim`, `pydantic-graph`, `pydantic-evals`, `pydantic-ai-harness`, `clai`, `pydantic-clai2`) |
| Size (shallow clone) | ~473 MiB; ~6023 files in the tree; ~2507 `.py` (before `.venv` / `.toolbelt`) |
| Study base commit | `f93bcff0` (main at clone time) |
| Existing AI instrumentation | `AGENTS.md` + `CLAUDE.md` symlink; `AGENTS.md`/`CLAUDE.md` in several subpackages (`docs/`, `tests/`, `pydantic_ai_slim/…`, harness, clai2); `.agents/skills/` and `.claude/skills/` (branch-context, pushing-commits, initialize-worktree, etc.); `.claude/settings.json`; `.gemini/config.yaml`; `agent_docs/`; `PLAN.md`; `THREAT_MODEL.md` |
| Toolchain | **Yes, partial.** `uv` 0.13.8 installed; `make install` (sync + pre-commit) ok; `make lint` ok; `uv run pytest` ok on target tests; spot `pyright` on the changed file ok. **Not run:** full `make test` / `make` / multi-Python matrix (`install-all-python`), for cost; LLM provider keys not configured (irrelevant to the chosen task). |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `MessagesBuilder.last_modified` to return the **most recent** message of the requested type since the checkpoint (use `reversed(_modified_since(...))`), with a regression test |
| Why | Small bug, reproducible without an LLM, one file + test; maintainer already described the one-line fix |
| Issue | https://github.com/pydantic/pydantic-ai/issues/13535 |
| Completed? | **Yes, locally.** Local commit `1ddd0d1` on branch `campo/fix-last-modified` (never pushed). Diff: `_messages_builder.py` + `tests/test_ui.py` |
| Native checks | `ruff format/check` on the files: pass; `pyright` on the file: 0 errors; `pytest tests/test_ui.py -k messages_builder`: 3 passed. Full suite / global typecheck: **not run** |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK (0.007 s); created `.toolbelt/` with Go defaults | hindered (defaults) | `verify.test_cmd: go test ./...` in a Python repo; `config.yaml` mode `600` |
| `tb mem init/add/list/check` | OK; 4 invariants | neutral/helped | `mem add` only positional (`tb mem add <regra>`); `invariants.yaml` mode `600`; check sha256 ok |
| `tb graph --enable rebuild` | OK (5.5 s); 70906 nodes / 135068 edges | helped (map) | Indexed Python/markdown; `graph.json` ~53 MiB mode `600` |
| `tb graph search last_modified` | Found `func:…ui.last_modified` | helped | Located the symbol quickly |
| `tb graph impact` without `--file` | Error `--file é obrigatório` | hindered | UX requires the flag; does not infer from the diff |
| `tb graph impact --file …/_messages_builder.py` | OK but output `(vazio)` | hindered | No useful impact despite the index |
| `tb graph callers` / `deps` | Positional rejected; with flags OK | hindered (UX) | `callers --function last_modified` → empty; `deps --file` listed pkgs |
| Ticket `rd.md` (Scope) | Clean drift with literal paths | helped | Frontmatter `base_commit` + `scope` + `forbidden` |
| `tb verify run` (pytest) | Visible PASS; hidden `disponível: false` | partial | Only runs hidden if `tests/hidden/` dir exists; ignores `--hidden-test-cmd` without that dir |
| `tb verify` + thresholds in the diff | FAIL iter 0, without running tests | hindered | Guard `protected_path` on `.toolbelt/thresholds.yaml` aborts verify |
| `tb verify --ruler visual` | PASS with skip | neutral | JSON: `"skipped": true`, reason manifesto absent |
| `tb review` | OK; 1 completeness warning | neutral | Criterion “existing … tests still pass” “weakly evidenced” despite the new test |
| `tb drift` | OK on literal scope | helped | No out_of_scope on the real ticket |
| `tb drift` with `**/*.py` | Marked in-scope files as out | hindered | `filepath.Match` does not treat recursive `**` |
| `tb guard check` | OK on clean diff; violates `thresholds.yaml` | helped (anti-cheat) | Custom `guard.yaml` **ignored** (deny of the fix file → `violations: null`) |
| `tb bench run --fake` | OK on `bugfix-go` (toolbelt repo) | neutral | Target has no `bench/tasks`; harness uses toolbelt fixtures |
| `tb bench compare` | Fake A/B OK; `--agent` flag nonexistent | hindered | `unknown flag: --agent` |
| `tb mcp` | tools/list + `tb_version` OK | neutral | graph/mem/verify tools **not** mirrored beyond graph_* + echo/env/run/version |
| Onboarding vs AGENTS.md | tb **does not read** AGENTS.md/skills | hindered | `tb ctx`/import AGENTS.md only in research docs (“pending”); init does not reference repo skills |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare … --agent` → `unknown flag: --agent`; help without the flag |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` with deny of the fix path; `guard check` → `violations: null` |
| outdated CLI docs | **confirmed** | `docs/cli.md` documents `tb ticket`/`tb board`/`tb run`/`tb proc`; real CLI: `ticket`/`board` nonexistent; `router` “não implementado” |
| version shows `unknown` | **confirmed (without ldflags)** | `go build` without `-ldflags`: `commit: unknown` / `built: unknown`; `make build` injects hash (`c8aae04`) |
| config files created 0600 | **confirmed** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → mode `600` |
| visual ruler reports `skipped=false` when skipping | **refuted** | `/tmp/verify-visual.json`: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` |
| `default_agent` ignored | **confirmed** | `bench.default_agent` in YAML; `tb bench run` without `--agent`/`--fake` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | Diff with `.toolbelt/thresholds.yaml` → `protected_path` violation; verify FAIL iterations 0 |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake --task bugfix-go` success without `--fake-config`; embedded `FakeAgentCommand("")` |
| `mem add` positional | **confirmed** | Usage `tb mem add <regra>`; `--rule` → unknown flag |
| Drift/Scope need literal paths (no globs) | **confirmed (partial)** | dir prefix and `foo/**` ok; `pydantic_ai_slim/**/*.py` **does not** match (both files out_of_scope); single-segment `ui/*` via `filepath.Match` ok |
| `graph impact` needs `--file` | **confirmed** | Without flag: exit 1 `--file é obrigatório` |
| review false positives on the tests/hidden invariant | **refuted (in this flow)** | Clean fix review: only completeness warning; `tests/hidden` invariant only fired when actually editing `tests/hidden/` |
| verify runs only partial checks and misses build errors | **confirmed** | Syntax error in `scripts/_tb_campo_broken.py` + spot pytest `test-cmd` → verify **PASS**; native ruff fails. Go `go test` defaults in a Python repo → ESCALATE without validating Python |
| tb does not push the agent toward completing the whole task | **confirmed (observational)** | AGENTS.md requires skills, docs, broad typecheck, PR template, etc.; tb neither imports nor enforces that — only diff/scope/test_cmd. Agent can “pass” tb with an incomplete task vs the project standard |

## New problems

| ID | sev | repro | expected | actual |
|----|-----|-------|----------|--------|
| N1 | P1 | `tb init` in a Python repo; `tb verify run` without overriding cmds | Detect ecosystem or fail with clear guidance | Defaults `go test ./...`; opaque ESCALATE — **fixed in `e5ab8bf`** (re-checked 2026-11-22) |
| N2 | P1 | `tb verify … --hidden-test-cmd 'pytest …'` without `tests/hidden/` dir | Honor the passed command | `hidden_available: false`; command ignored |
| N3 | P2 | `tb graph impact --file <py with symbols>` after rebuild | List impacted callers/deps | `(vazio)` — **fixed in `d788b1f`** (re-checked 2026-11-22) |
| N4 | P2 | `tb graph callers last_modified` (positional, like search) | Accept positional or a single message | Requires `--function`; UX inconsistent with `search` — **still open** (re-checked 2026-11-22): `tb graph callers` still requires `--function`; a positional alias would be additive and is left to after 1.0, since flags are frozen as they are |
| N5 | P2 | MCP `tools/list` after using CLI mem/verify/review | CLI↔MCP parity | Only `tb_graph_*`, `tb_version`, `tb_echo`, `tb_env`, `tb_run` |
| N6 | P3 | Scope `pydantic_ai_slim/pydantic_ai/ui/*` vs file in the dir | Document that `*` is single-segment / prefer `dir/` or `dir/**` | Mixed behavior confuses (“globs”) |
| N7 | P2 | Repo with rich AGENTS.md; `tb init` | Import/summarize project rules into vault/invariants | Ignores entirely; risk of conflict with generic Go-oriented invariants (`tests/hidden/`) |

## What tb caught and what it let slip

**Caught**
- Scope drift when the ticket listed correct literal paths.
- Edits to `tests/hidden/` (forbidden + invariants) and `.toolbelt/thresholds.yaml` (guard).
- Visible test failure when `test_cmd` itself imported a file with broken syntax.
- Visual skip with missing manifesto (`skipped: true`).

**Let slip**
- Syntax error in a file **outside** `test_cmd` (verify PASS vs ruff FAIL).
- Go defaults in a Python project (agent must reconfigure by hand).
- Soft completeness: warning for a textual acceptance criterion, but no check whether AGENTS.md docs/skills/THREAT_MODEL were updated (they did not need to here — the point is tb does not know).
- Empty `graph impact` → did not suggest `tests/test_ui.py` as a file to touch.
- Operator `guard.yaml` not applied.
- Did not require project lint/typecheck; only whatever was in `test_cmd`.

## Interaction with existing instrumentation / onboarding

- **tb does not read** `AGENTS.md`, `.agents/`/`.claude/` skills, or `agent_docs/`. Search in toolbelt Go/docs: AGENTS.md appears only as a future idea (`tb ctx`, architecture “pending”).
- **Potential duplication:** default invariants (`tests/hidden/`, `thresholds.yaml`) are from tb’s Go harness; pydantic-ai already has anti-patterns and workflow in AGENTS.md/skills — distinct sets, no merge.
- **Conflict:** `tb init` onboarding suggests a workflow centered on `rd.md` + `.toolbelt/`, while the repo requires skills (`initialize-worktree`, `branch-context`, `pushing-commits-to-the-repo`) and not pushing to main (pre-commit `no-commit-to-branch`). An agent that only follows tb ignores the real process.
- **What tb should import:** (1) hierarchical AGENTS.md paths; (2) `Makefile`/`docs/contributing.md` commands → `verify.test_cmd` / lint / typecheck; (3) skill scope restrictions (narrow bugfix); (4) project quality invariants (type-safety, no bot `Co-Authored-By`, etc.) for `tb mem`.

## Improvement suggestions

1. **[P0 · verify/init]** Auto-detect Python/`uv`/`make` and fill `verify.test_cmd` (and lint) in `tb init`; never default `go test` outside Go repos.  
2. **[P0 · verify]** If `--hidden-test-cmd` is passed, always run it; do not tie it to existence of `tests/hidden/`.  
3. **[P0 · mem/ctx]** `tb init`/`tb ctx`: read AGENTS.md (and skills index) → invariants + project completion checklist.  
4. **[P1 · guard]** Honor `.toolbelt/guard.yaml` (or document that it is dead and remove it).  
5. **[P1 · bench]** Use `bench.default_agent` when `--agent` is omitted; add `--agent` to `bench compare` (or document only `--fake`/configs).  
6. **[P1 · drift/review]** Really support `**` globs; docs with `dir/`, `dir/**`, literal path examples.  
7. **[P1 · graph]** `impact` from diff `base..head` without `--file`; populate Python `calls` edges (today impact is empty).  
8. **[P2 · mcp]** Expose mem/verify/review/drift/guard on MCP with CLI parity.  
9. **[P2 · docs]** Mark unimplemented commands in `docs/cli.md` / README; align “planned” tree vs `tb --help`.  
10. **[P2 · verify]** Composite ruler: `test_cmd` + `lint_cmd` + `typecheck_cmd` + optional build, so partial checks cannot “pass”.  
11. **[P3 · ux]** Uniform positionals (`graph search` vs `callers --function`); `.toolbelt` file permissions aligned to the repo umask (do not force 0600 if the team shares the worktree).

## Metrics

| metric | value |
|--------|-------|
| Wall time (sum of timed commands in the log) | ~57 s of invocation CPU-wall; study session ~25–30 min including clone/`make install`/reading |
| tb invocations (+ native checks in the evidence harness) | ~65 log sections; ~53 commands with parseable EXIT/TIME on the first structured pass |
| tb failures (exit ≠ 0) in the structured log | 26 in the main sample (includes expected negatives: missing flags, deliberate drift, initial shallow HEAD~1, etc.) |
| Crash/segfault | none observed |
| Task delivered on target | yes (local only); evidence: MessagesBuilder pytest 3/3 pass; 2-file diff |
