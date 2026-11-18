🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/typer.md)

# Field study: fastapi/typer

Group: without AI-agent instrumentation for contributing to the repository itself.  
Binary: `/agent/repos/toolbelt/bin/tb` (`make build`, version `c8aae04`).  
Working clone: `/tmp/typer-repo` (workaround; see P0).  
Date: 2026-11-15.

## Repository

| Field | Value |
|-------|-------|
| Name | [fastapi/typer](https://github.com/fastapi/typer) |
| Language | Python (≥3.10), CLI over vendored Click |
| Size | shallow clone ~5.5–8.1 MiB; **643** `.py` files; **211** `tests/test_*.py` |
| AI instrumentation | **None** of `AGENTS.md`, `.cursorrules`, `CLAUDE.md`, or Cursor rules for contributors. There is `typer/.agents/skills/typer/SKILL.md` (official skill for *consumers* writing Typer apps — not onboarding an agent on the Typer codebase). Also `scripts/agent_banner.py` / `includes/agent-banner.md` (docs banner). |
| Toolchain | **Partially configured.** `pip install -e .` + test deps (`pytest`, `ruff`, `mypy`, `pytest-xdist`, …). `uv` was **not** on PATH; sync via `uv.lock` was not used. MkDocs / `prek` / full CI suite **not** installed. Smoke: `pytest tests/test_types_file.py` and `tests/test_core.py` OK; `ruff check` OK on touched files. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix flakiness of `tests/test_types_file.py::test_file_error` when `TMPDIR` is long (Rich wraps the path and the assert `"cannot-open.txt" in output` fails). |
| Why | Real test bug reported in a discussion; minimal patch, tests only, maintainer-acceptable. |
| Link | https://github.com/fastapi/typer/discussions/1984 |
| Completed? | **Yes**, locally: `monkeypatch.chdir(tmp_path)` + `Path("cannot-open.txt")` (same approach as the commit cited in the discussion, `2c40f62`, which was not on shallow `main`). |
| Native checks | `pytest tests/test_types_file.py` → 24 passed, 2 skipped; with artificially long `TMPDIR` → `test_file_error` PASS; `ruff check tests/test_types_file.py` PASS; `import typer` OK. **Did not** open a PR/issue upstream. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; creates `.toolbelt/` + `config.yaml` (mode **0600**); Go defaults (`verify.test_cmd: go test ./...`); **does not** generate `AGENTS.md` | hindered (Python onboarding) | WALL 0.007 s; config without pytest/ruff detection |
| `tb version` | With `make build`: `c8aae04`. Without ldflags: `0.1.0-dev`, `commit: unknown`, `built: unknown` | neutral | `/tmp/tb-nolink version` |
| `tb mem init/list/add/check` | OK; 3 default invariants + 1 added; positional `mem add <regra>` | helped (light) | `mem add --rule` fails (nonexistent flag); CLI docs describe another API |
| `tb graph rebuild/stats/search/impact/callers` | Under `/tmp/target` (dir name = `target`): **0 nodes** (P0). Under `/tmp/typer-repo`: 3056 nodes / 6093 edges; `search test_file_error` found the function; `impact`/`callers format_filename` → `(vazio)` | mixed | SkipDirs includes `target`; impact not very useful for this patch |
| ticket `rd.md` (Scope) | Manual; `tb ticket` **does not exist** | hindered | `unknown command "ticket"`; YAML ticket used by drift/review |
| `tb verify run` | PASS on configured tests; **no** `lint_cmd`/`build_cmd` in code; hidden suite only if `tests/hidden/` dir exists; touch `.toolbelt/thresholds.yaml` → FAIL iter 0 (guard) | mixed | JSON: `hidden_available` false without the dir; visual skip PASS |
| `tb review` | In-scope diff → findings `[]`; mention of `tests/hidden` in a comment → **critical** false positive | hindered (FP) | agreement 3/3 on goldfish |
| `tb drift` | Literal scope and glob `tests/*.py` OK; wrong scope detects out_of_scope | helped | `filepath.Match` implemented |
| `tb guard check` | `base..head` syntax (not `--base`); hardcoded paths; `guard.yaml` ignored | hindered | custom yaml path → `violations: null`; thresholds → protected_path |
| `tb bench` | Without `bench/tasks` on Typer fails; with tasks copied from toolbelt, `--fake` runs; config `default_agent` **ignored**; `compare` without `--agent` | neutral/hindered | `--agent ou --fake é obrigatório` despite `default_agent` set |
| `tb mcp` | stdio OK; `tools/list` exposes version/echo/env/run/graph_* | helped (smoke) | initialize + tools/list in 0.007 s |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `--help` only has `--fake` to run A/B configs; no external agent flag |
| `guard.yaml` ignored | **confirmed** | `NewChecker` hardcodes `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`; custom yaml path did not produce a violation |
| outdated CLI docs | **confirmed** | docs: `tb mem add --type/--title`, `tb ticket`, `tb run`, `tb guard check --type…`; real CLI diverges |
| `version` shows `unknown` | **confirmed** (without ldflags) | raw `go build` → `commit: unknown` / `built: unknown` |
| configs created 0600 | **confirmed** | `stat` → `600 .toolbelt/config.yaml` and `invariants.yaml` / `graph.json` |
| visual ruler reports `skipped=false` when skipping | **refuted** (on this build) | JSON `"skipped": true`; stdout “Skip gracioso…” |
| `default_agent` ignored | **confirmed** | config with `default_agent: echo-agent-SHOULD-BE-USED`; `tb bench run` → `--agent ou --fake é obrigatório` |
| guard aborts verify if diff touches hardcoded path | **confirmed** | thresholds in the diff → verify FAIL iterations 0 |
| fake default needs `--fake-config` | **partial** | `--fake` runs without the flag; without config with `FilesToCreate`, pass@1=0% on task `api-handler` |
| `mem add` positional | **confirmed** (docs vs CLI) | CLI: `tb mem add <regra>`; docs still show `--type/--title` flags |
| Drift/Scope need literal paths (no globs) | **refuted** | `tests/*.py` kept the file in scope via `filepath.Match` |
| `graph impact` needs `--file` | **confirmed** | without flag → `Error: --file é obrigatório` |
| review FP on tests/hidden invariant | **confirmed** | comment `# probe tests/hidden…` → critical |
| verify only partial checks / miss build | **confirmed** (partial) | only `test_cmd`/`hidden_test_cmd`; no lint/build; hidden only with `tests/hidden/` dir |
| tb does not push completing the whole task | **observed** | init does not generate checklist/progress/AGENTS; agent must assemble ticket and cmds by hand |

## New problems

### P0 — `SkipDirs` includes basename `target` and zeros the graph

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb init && TB_GRAPH=on tb graph rebuild` → `0 nós, 0 arestas`. Same repo under `/tmp/typer-repo` → 3056 nodes.
- **Expected:** skip only build subdirs named `target/` (e.g. Rust), not the project root when cwd is named `target`.
- **Actual:** `skipDirs[info.Name()]` on the walk applies to the root.

### P1 — Onboarding assumes Go and does not detect Python toolchain

- **Repro:** `tb init` in a Python repo.
- **Expected:** detect `pyproject.toml` / pytest / ruff and fill `verify.*`.
- **Actual:** `test_cmd: go test ./...`, `hidden_test_cmd: go test ./tests/hidden/...`; no `AGENTS.md`.

### P1 — Verify hidden suite tied to existence of `tests/hidden/`

- **Repro:** `--hidden-test-cmd 'pytest …'` without creating the directory → `hidden_available: false` and the configured command **does not run**.
- **Expected:** honor `hidden_test_cmd` whenever it is set.
- **Actual:** `Stat(tests/hidden)` decides availability.

### P2 — Empty `graph impact` / `callers` for a Python symbol relevant to the patch

- **Repro:** after rebuild, `tb graph impact --file tests/test_types_file.py` and `callers --function format_filename` → `(vazio)`.
- **Expected:** useful edges to guide the test change.
- **Actual:** search finds the function; impact/callers do not help in this case.

### P2 — `guard check` UX vs review docs/flags

- **Repro:** `tb guard check --base HEAD~1 --head HEAD` → `unknown flag: --base`.
- **Expected:** same ergonomics as drift/review **or** docs aligned to `base..head`.
- **Actual:** only `tb guard check <base>..<head>`.

### P3 — MCP without mem/verify/review/drift/guard tools

- **Repro:** `tools/list` after initialize.
- **Expected:** parity with the field flow.
- **Actual:** only version/echo/env/run/graph_*.

## What tb caught and what it let slip

**Caught**
- Scope drift when the ticket pointed at the wrong file.
- Guard on `.toolbelt/thresholds.yaml` (and verify abort).
- “Fresh” review with no findings on the legitimate patch.
- Graph (after path workaround): located `test_file_error`.
- Verify with pytest `test_cmd`: PASS on the fix; FAIL when `__init__.py` had a SyntaxError (via import in conftest).

**Let slip / did not cover**
- Did not suggest or configure pytest/ruff on its own.
- Did not validate lint/build as separate stages.
- Ignored custom `guard.yaml`.
- False positive on `tests/hidden` invariant from a test comment.
- Impact/callers did not guide the patch.
- Did not push a “complete task” checklist (ticket, tests, lint, docs).

**Compared with native:** pytest+ruff confirmed the fix; tb verify, after manual config, mirrored pytest. Without manual config, the Go default would be useless/a false negative in this repo.

## Interaction with existing instrumentation / onboarding

- Existing instrumentation limited to a Typer *user* skill (`.agents/skills/...`), irrelevant to the `tb` contribution flow.
- `tb init` **did not** read the skill, **did not** generate `AGENTS.md`, **did not** detect test/lint/build commands, **did not** create a harness-style ticket/progress log.
- Default memory is generic (Go-oriented anti-cheat: `tests/hidden`, `thresholds.yaml`).
- Conclusion: “from nothing” bootstrap is weak for Python; the human/AI agent must rewrite verify and invent `rd.md`.

## Improvement suggestions

1. **[P0][graph]** Do not apply `SkipDirs` to the walk root directory; compare relative path, not only `filepath.Base` of the root.  
2. **[P0][init/verify]** Detect ecosystem (`pyproject.toml`, `package.json`, `go.mod`) and fill `test_cmd`/`lint`/`build`; optionally emit `AGENTS.md`.  
3. **[P1][verify]** Treat `hidden_test_cmd` as source of truth; `tests/hidden/` only as a default convention.  
4. **[P1][guard]** Load `.toolbelt/guard.yaml`; unify flags with drift/review.  
5. **[P1][docs]** Sync `docs/cli.md` with real Cobra (`mem`, `guard`, absence of `ticket`/`run`).  
6. **[P2][review]** Match invariants by touched path / hunk, not a loose substring in the diff.  
7. **[P2][bench]** Honor `bench.default_agent`; add `--agent` to `bench compare`.  
8. **[P2][mcp]** Expose mem/verify/review/drift/guard.  
9. **[P3][init]** `rd.md` template + progress log to push task closure.

## Metrics

| metric | value |
|--------|-------|
| Wall time of logged `tb`/check commands (sum) | ~8.9 s (79 timed invocations) |
| Approximate field-session wall time | ~15–20 min (build, clone, toolchain, implementation, probes) |
| Logged invocations | 79 |
| Nonzero exits | 20 (many intentional: bad flags, drift, negative probes) |
| Relevant unexpected failures | empty graph under `/tmp/target`; bench without tasks on Typer; hidden suite silently off |
| MCP | smoke OK (1 session) |
| Commits on target | 1 local only (`test: keep file error path short for long TMPDIR`); **no** push/PR/issue on fastapi/typer |
