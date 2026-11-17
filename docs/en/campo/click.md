[🇧🇷 Português](../../campo/click.md) | 🇺🇸 English

# Field study: pallets/click

Group: without AI agent instrumentation.  
`tb` binary: local `make build` at `c8aae04` (`./bin/tb`).  
Target clone: `/tmp/target` (shallow of https://github.com/pallets/click).  
No interaction with the Click remote (no push/issue/PR/comment).

## Repository

| Field | Value |
|-------|--------|
| Name | pallets/click |
| Language | Python (≥3.11), package `click` |
| Size | shallow clone ~2.9M (without `.venv`); with deps ~9.3M; ~77 `.py` under `src/`+`tests/`; ~12.8k LOC in `src/` |
| Existing AI instrumentation | None found (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, copilot/aider absent) |
| Toolchain | **Yes, partial**: `uv` + venv, `uv sync` (groups `tests`/`typing`/`dev`), `pytest` 9.0.2, `ruff` 0.15.9, `mypy` 1.20.0. Installed `less` (was missing; 24 pager tests failed). **Not configured**: Sphinx docs, full tox matrix, local CI pre-commit hooks, pyright on full suite. Baseline after `less`: `2626 passed, 24 skipped, 1 xfailed`. |

## Chosen task

| Field | Value |
|-------|--------|
| What | Fix `Option.get_error_hint` to resolve `auto_envvar_prefix` when `show_envvar=True` (mirroring `get_help_extra`) |
| Why | Small bug, clear reproduction, one production file + one regression test; maintainer would likely accept |
| Issue | https://github.com/pallets/click/issues/3914 |
| Completed? | **Yes, locally** (commits only in `/tmp/target`; not sent upstream) |
| Native checks | `pytest -k "show_envvar or missing_envvar"`: 17 passed; full suite: **2627 passed**; `ruff check` on touched files: OK after E501 wrap; `mypy src/click/core.py`: exit 0 |

Essential diff: in `src/click/core.py`, `get_error_hint` now derives `MYAPP_PORT` via `ctx.auto_envvar_prefix`; test `test_show_envvar_auto_prefix_in_error_hint` in `tests/test_options.py`. Manual ticket `rd.md` with literal Scope.

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb version` | OK; version=`c8aae04` (Make ldflags) | neutral | `0.005s`. Without ldflags: `0.1.0-dev`, `commit/built: unknown` |
| `tb init` | Created `.toolbelt/` + `config.yaml` mode **600** | hindered (Go defaults) | `verify.test_cmd: go test ./...` in a Python repo; no pytest/ruff detection; no `AGENTS.md` |
| `tb mem init/list/add/check` | OK | helped (anti-cheat baseline) | positional `mem add <regra>` works; `docs/cli.md` flags (`--type`, `--title`) **fail** (`unknown flag: --type`) |
| Ticket / Scope | Manual (`rd.md`); `tb ticket` **does not exist** | hindered | CLI: `unknown command "ticket"`; Had to write Scope/base/forbidden by hand |
| `tb graph rebuild .` in `/tmp/target` | **0 nodes** | hindered | SkipDirs includes name `target` → root `/tmp/target` is skipped entirely |
| `tb graph index src` | 486 nodes / 662 edges | helped | `search get_error_hint` found `func:click.get_error_hint`; `0.040s` |
| `tb graph impact --file …` | empty (with and without `src/`) | hindered | Without `--file` → exit 1 (`--file é obrigatório`). With indexed file still `(vazio)` |
| `tb graph callers` | empty for `get_error_hint` | neutral/hindered | Python indexing did not build useful `calls` edges here |
| `tb verify run` (pytest cmds) | PASS (visible OK; hidden `available=false`) | helped partially | After manual config. Default `go test` → useless ESCALATE |
| `tb verify --ruler visual` | PASS `skipped=true` | neutral | Reason: `manifesto de telas ausente`; `skipped` correct in this case |
| `tb guard check` | Hardcoded paths only | hindered | `.toolbelt/guard.yaml` protecting `src/click/core.py` **ignored**; core diff without violation |
| `tb guard` + verify on `thresholds.yaml` | verify FAIL / aborts | helped (anti-cheat) | Violation `protected_path` on `.toolbelt/thresholds.yaml` |
| `tb drift` | Detects out-of-scope / base_mismatch | helped partially | Globs `src/click/*.py`, `tests/**`, `src/**/*.py` worked; ticket with globs still requires listing extra paths (e.g. `rd.md`) |
| `tb review` | Critical findings on clean review | hindered | FP: mentioning `tests/hidden` / `thresholds.yaml` in the `rd.md` itself in the diff triggers invariants |
| `tb bench` on Click | Fails without `bench/tasks` | hindered | On toolbelt repo, `bench compare --fake` OK (~5s); `compare --agent` → `unknown flag` |
| `tb mcp` | tools/list + `tb_version` OK | helped partially | Without `TB_GRAPH=on` in the process, `tb_graph_search` errors; with env, search/index OK |
| Native checks vs tb | ruff E501 only native | — | verify does not run lint/build beyond `test_cmd` |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare … --agent "echo hi"` → `unknown flag: --agent`; help only lists `--fake` |
| `guard.yaml` ignored | **confirmed** | Manifesto protecting `src/click/core.py`; `guard check` on the fix commit → `violations: null`. Code: hardcoded paths in `NewChecker` |
| Outdated CLI docs | **confirmed** | `docs/cli.md`: `tb serve`, `mem add --type/--title`, `graph index --path`; real: `tb mcp`, `mem add <regra>`, `graph index [path]` |
| version shows `unknown` | **confirmed (partial)** | Without `-ldflags`: `commit: unknown`, `built: unknown`. With Make: version=short SHA, not semver |
| config files created 0600 | **confirmed** | `stat`: `600 .toolbelt/config.yaml` (also graph/invariants) |
| visual ruler `skipped=false` when skipping | **refuted** (in this scenario) | `--ruler visual` without manifesto → `"skipped": true` |
| `default_agent` ignored | **confirmed** | `bench.default_agent: echo-agent-placeholder` in config; `tb bench run` without `--agent` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | Commit on `.toolbelt/thresholds.yaml` → verify FAIL, `iterations: 0`, `protected_path` |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake --task bugfix-go` exit 0 without `--fake-config` (on toolbelt repo) |
| `mem add` positional | **confirmed** | Help: `tb mem add <regra>`; old docs with vault flags |
| Drift/Scope need literal paths (no globs) | **refuted (partial)** | `src/click/*.py`, `tests/**`, `src/**/*.py` matched via `filepath.Match` / `/**` prefix |
| `graph impact` needs `--file` | **confirmed** | Without flag → exit 1 `--file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmed** | Review of clean fix (`core.py`+`test_options.py`+`rd.md`) → critical for text `tests/hidden` / `thresholds.yaml` **inside rd.md** |
| verify runs only partial checks / misses build errors | **confirmed (partial)** | Go defaults on Python; no ruff/mypy/compileall. Syntax error in a module imported by pytest **was** caught; lint E501 **was not** |
| tb does not push agent toward completing whole task | **confirmed (qualitative)** | No checklist/progress/AGENTS; wrong defaults; review FP and empty graph divert; completed the task **despite** tb, not because of its guidance |

## New problems

1. **P0 — `graph` SkipDirs swallows the clone if the root directory is named `target` (or another skip)**  
   - Repro: `cd /tmp/target && TB_GRAPH=on tb graph --enable rebuild .` → `0 nós`. Same repo under `/tmp/click-repo` → `1956 nós`.  
   - Expected: do not apply SkipDir to the walk root.  
   - Actual: `info.Name()=="target"` → `filepath.SkipDir` at the root.

2. **P1 — `tb init` does not detect non-Go toolchain**  
   - Repro: `tb init` on Click.  
   - Expected: suggest/fill `pytest`/`ruff` (or ask).  
   - Actual: `go test ./...` and `go test ./tests/hidden/...`.

3. **P2 — `graph impact`/`callers` useless after Python index**  
   - Repro: index `src`, `impact --file src/click/core.py` or `click/core.py` → `(vazio)`; `callers --function get_error_hint` → empty.  
   - Expected: impacted files/functions or a clear coverage message.  
   - Actual: silence `(vazio)`.

4. **P2 — Review treats mention of a forbidden path in the ticket as a violation**  
   - Repro: `rd.md` with `forbidden: tests/hidden/` committed; review of the same range.  
   - Expected: only flag changes to those paths.  
   - Actual: `suggestsViolation` does substring on the whole diff.

5. **P3 — MCP graph inherits process env; CLI `--enable` does not “stick”**  
   - Repro: `tb mcp` without `TB_GRAPH=on` → `tb_graph_search` isError; with env OK.  
   - Expected: document or accept enable via tool arg/config.

6. **P3 — user-created `guard.yaml` is never read (docs promise it)**  
   - Already listed as known; here: `199B` file in `.toolbelt/guard.yaml` mode 644, zero effect.

## What tb caught and what it let slip

**Caught**
- Hardcoded protected path `.toolbelt/thresholds.yaml` (blocked verify).
- Scope drift when `rd.md` outside Scope.
- `base_mismatch` when `--base` ≠ ticket `base_commit`.
- After manual config, verify PASS on regression `show_envvar_auto_prefix_in_error_hint`.
- Graph search found `get_error_hint` (with index on `src`).
- Graceful visual ruler skip without manifesto.

**Let slip / did not help**
- Go verify defaults (did not guide to pytest).
- Custom `guard.yaml` ignored (did not protect `core.py` via manifesto).
- Ruff E501 on the first patch version (only native `ruff`).
- Clean review with invariant FPs → noise, not signal.
- Graph rebuild in `target` folder = empty graph (almost “blind”).
- Impact/callers with no value for locating consumer tests.
- Absence of `tb ticket` / progress → did not push complete acceptance (test+lint+docs).

## Interaction with existing instrumentation / onboarding

Repo **without** AGENTS.md / agent rules. tb onboarding:

| Generated by `tb init` / `mem init` | Quality |
|-------------------------------------|---------|
| `.toolbelt/config.yaml` | Generic Go; `default_agent: ""`; graph languages OK in the list, but wrong verify |
| dirs `bench/`, `logs/`, `vault/` | Empty / placeholders; `bench/tasks` **not** created → `bench` breaks on target |
| `invariants.yaml` (via `mem init`) | 3 useful anti-cheat rules as seed |
| **Does not generate** | `AGENTS.md`, effective `guard.yaml`, `pytest`/`ruff`/`mypy` detection, ticket template, `init.sh` scripts, progress log |

Conclusion: the bootstrap **does not** prepare an agent to work well on a clean Python project; the operator must edit verify by hand and invent `rd.md`. Missing auto-detect of test/lint/build commands and a minimal `AGENTS.md` pointing to `tb mem`/`verify`/`graph`.

## Improvement suggestions

1. **[P0][graph]** Do not skip the indexing root directory; treat SkipDirs only for descendants (`indexer.go` walk).  
2. **[P0][init/verify]** Detect ecosystem (pyproject/pytest/ruff/go.mod/…) and fill `verify.test_cmd` / suggest hidden suite.  
3. **[P1][guard]** Load `.toolbelt/guard.yaml` or remove the docs promise; today only hardcoded.  
4. **[P1][review]** Match invariants by changed path, not substring in the diff/ticket.  
5. **[P1][docs]** Align `docs/cli.md` with the real CLI (`mcp`, `mem add`, graph flags).  
6. **[P1][init]** Generate `AGENTS.md` + `rd.md` template + “configure verify” note.  
7. **[P2][bench]** `bench compare --agent`; honor `default_agent`; minimal task seed or clear skip on `init`.  
8. **[P2][graph]** Impact accept path with/without `src/` prefix; warn when there are no edges.  
9. **[P2][verify]** Optional lint/build ruler (`ruff`/`compileall`/`go build`) beyond test_cmd.  
10. **[P3][config]** Reconsider umask 0600 vs 0644 in shared monorepos; document.  
11. **[P3][mcp]** Propagate graph enable via config/tool arg, not only host env.

## Metrics

| Metric | Value |
|--------|--------|
| Study wall time (first→last tb log) | ~174 s of instrumented commands; full session (clone+deps+fix+report) ~tens of minutes |
| Recorded `tb` invocations | 48 |
| Failures (exit ≠ 0) among them | 19 (several intentional: drift/review/guard/docs mismatch) |
| Sum of tb command durations | ~7.3 s |
| Slowest commands | `bench compare --fake` ~5.1 s; `verify` with pytest ~0.2–0.5 s; `graph index src` ~0.04 s |
| Relevant “product” failures | graph 0 nodes in `/tmp/target`; review FP; inert guard.yaml; init Go-default; compare without `--agent`; nonexistent `ticket` |

Local artifacts (not in toolbelt): `/tmp/tb-campo-click-evidence.log`, clone `/tmp/target` with experiment commits (incl. syntax/thresholds/hidden reverts).
