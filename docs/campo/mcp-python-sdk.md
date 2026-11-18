🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/mcp-python-sdk.md)

# Field study: modelcontextprotocol/python-sdk

Study conducted on 2026-11-15 with `tb` built locally from this repository (`make build` → `toolbelt c8aae04`). The target was used only as a local clone (`/tmp/target` and copy `/tmp/python-sdk`); there was no push, issue, PR, or comment upstream.

## Repository

| Field | Value |
|-------|--------|
| Name | [modelcontextprotocol/python-sdk](https://github.com/modelcontextprotocol/python-sdk) |
| Language | Python 3.11+ (MCP SDK; package `mcp`) |
| Size | ~22 MiB working tree without `.venv` / ~292 MiB with `.venv`; shallow clone ~25 MiB before sync; ~419 `.py` under `src/`+`tests/`, ~3673 `.py` in the full post-deps tree |
| Clone base commit | `77001c9` (shallow `main`) |
| Existing AI instrumentation | `AGENTS.md` (166 lines, mandatory guidelines), `CLAUDE.md` (`@AGENTS.md`), `.claude/commands/review-pr.md`, `.claude/skills/test-quality/`, workflow `.github/workflows/claude.yml` |
| Toolchain | **Yes, partial.** Installed `uv 0.13.8`; `uv sync --frozen` OK with system Python 3.12. Ran: `pytest` (stdio client + interaction), `ruff check` on touched files. **Not run:** full `./scripts/test` (~100% coverage + matrix), full `pyright`, docs (`zensical`), full pre-commit, Windows/3.11–3.15. Reason: study time/scope; the full suite is expensive and AGENTS.md already notes ~23 s just for the default coverage script. |

## Chosen task

| Field | Value |
|-------|--------|
| What | Default of `StdioServerParameters.encoding_error_handler`: `"strict"` → `"replace"`, aligned with the server (PR #2302), + regression tests in `tests/client/test_stdio.py` |
| Why | P2 bug `ready for work` with a clear repro; minimal, testable, maintainer-plausible change; still open on `main` at study time |
| Issue | https://github.com/modelcontextprotocol/python-sdk/issues/2454 |
| Completed? | **Yes, locally** (local branch `campo-local-2454`, commit `960549b`). Not sent upstream (forbidden by study rules). |
| Native test/lint/build | `uv run --frozen pytest tests/client/test_stdio.py -q` → **37 passed**; `tests/interaction/transports/test_stdio.py` → **2 passed**; `ruff check` on changed files → OK. Global build/typecheck and `./scripts/test` **not** executed. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; created `.toolbelt/` + `config.yaml` (mode **0600**); `verify` defaults = `go test ./...` | hindered | Wall 0.008 s. Go defaults in a Python repo; agent must rewrite `verify.test_cmd` by hand. Config 0600 confirms known issue. |
| `tb version` | With `make build`: `c8aae04`. With `go build` without ldflags: `0.1.0-dev` / commit **unknown** | neutral | Both measured; “unknown” only on the binary without `-ldflags`. |
| `tb mem init/list/add/check` | OK; 5 invariants; `mem check` sha256 + retention 1.00 | helped (partial) | Positional `tb mem add "<regra>"` works. Docs in `docs/cli.md` describe nonexistent `--type/--title` → friction. `invariants.yaml` also 0600. |
| `tb graph rebuild/index` under `/tmp/target` | **0 nodes / 0 edges** in ~0.006 s | hindered | Root directory named `target` is in `SkipDirs`; `filepath.Walk` does `SkipDir` on the root itself. |
| `tb graph rebuild` under `/tmp/python-sdk` | **16550 nodes, 43746 edges**, 1.345 s | helped | Same code; only the cwd basename changes. |
| `tb graph search encoding_error_handler` | empty even with a full graph | hindered | Search only in title/path/package/id; does not index fields/attributes. |
| `tb graph impact --file src/mcp/client/stdio.py` | empty | neutral/hindered | Impact not very useful here (few `calls` edges: 100 in the whole repo). |
| `tb graph callers --function stdio_client` | empty | hindered | Python indexer extracts few calls. |
| `tb graph deps --file src/mcp/client/stdio.py` | lists imports (`anyio`, `mcp_types`, …) | helped | Only clearly useful graph subcommand for this task. |
| `tb graph impact` without `--file` | EXIT 1, “--file é obrigatório” | neutral | Confirms known UX issue. |
| Ticket `rd.md` + Scope | YAML with `---` works; markdown list without frontmatter **does not** parse multiline scope | hindered | Without `---`, drift marked `tests/client/test_stdio.py` out of scope even when listed. With frontmatter: clean drift. |
| `tb verify run` (pytest stdio) | PASS iter=1; **Hidden: disponível=false** | helped (partial) | Detects presence of `tests/hidden/` by path, not by configured `hidden_test_cmd`. Only ran the visible suite. |
| `tb verify` + syntax error in `examples/...` | PASS | hindered | Native `py_compile` failed; verify configured with only spot pytest passed. Confirms “partial checks”. |
| `tb verify --ruler visual` missing manifesto + `--skip-unavailable` | PASS, `"skipped": true` | neutral | Correct. |
| `tb verify --ruler visual` with empty `screens.yaml` + skip | PASS, **`"skipped": false`**, `screens: null` | hindered | Confirms visual ruler issue. |
| `tb review` (correct frontmatter ticket) | PASS, findings=[] | helped | Wall ~0.008 s; fresh_context true. |
| `tb review` + touch under `tests/hidden/` | FAIL critical invariant | hindered | False positive for anyone creating the hidden suite (default `mem init` invariant). |
| `tb drift` literal paths + frontmatter | OK | helped | |
| `tb drift` glob `src/**/*.py` | FAIL out_of_scope | hindered | `filepath.Match` does not handle `**`. Simple glob `src/mcp/client/*.py` **worked**. |
| `tb guard check` on the fix | OK, violations null | helped | |
| `tb guard` + `.toolbelt/guard.yaml` deny | **ignored** the YAML; still PASS | hindered | Confirmed: guard.yaml is not read. |
| `tb guard` + `.toolbelt/thresholds.yaml` in the diff | `protected_path` violation; `verify` FAIL iterations 0 | hindered | Hardcoded path aborts the verify flow. |
| `tb bench run --fake` | OK with `-t/-f` pointing at toolbelt fixtures | neutral | Target repo has no `bench/tasks`; config `default_agent` **does not** waive `--agent/--fake`. |
| `tb bench compare` | OK with `--fake`; **no** `--agent` flag | neutral | Help confirms absence of `--agent` on compare. |
| `tb mcp` | tools/list + tools/call `tb_version` OK; `tb_graph_search` → `null` under `/tmp/target` | helped (partial) | MCP starts; MCP graph inherits the `SkipDirs`/`target` bug. |
| Onboarding / AGENTS.md | `tb init` **does not** read or import `AGENTS.md` / `.claude/` | hindered | Architecture marks AGENTS.md as “pending”. |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare --help` without `--agent`; only `--fake` / JSONL. |
| `guard.yaml` ignored | **confirmed** | Deny in `.toolbelt/guard.yaml` for `stdio.py`; `guard check` → `violations: null`. |
| Outdated CLI docs | **confirmed** | `docs/cli.md` documents `tb ticket`, `tb mem add --type/--title`; real CLI: no `ticket`; `mem add <regra> --category`. |
| Version shows `unknown` | **confirmed (partial)** | Without ldflags: `commit: unknown` / `0.1.0-dev`. With `make build`: git commit. |
| Config files created 0600 | **confirmed** | `stat` → `config.yaml` and `invariants.yaml` mode 600; `graph.json` also 600. |
| Visual ruler reports `skipped=false` when skipping | **confirmed** | Empty manifesto + `--skip-unavailable` → `"skipped": false`, `screens: null`. Missing manifesto → `skipped: true` (OK). |
| `default_agent` ignored | **confirmed** | Config with `default_agent: cursor-agent ...`; `tb bench run` without flags → `--agent ou --fake é obrigatório`. |
| Guard aborts verify when diff touches hardcoded config path | **confirmed** | Diff only with `.toolbelt/thresholds.yaml` → verify FAIL, iterations 0. |
| Default fake agent needs `--fake-config` | **refuted (on the happy path)** | `tb bench run --fake` without `--fake-config` passed (100%). Flag exists to customize; not required. |
| `mem add` positional | **confirmed as docs/UX gap** | Positional works; docs suggest flags that do not exist (`--type`, `--title`, `--rule` → unknown flag). |
| Drift/Scope need literal paths (no globs) | **partially confirmed** | `*` in one segment works; `**` does not. Multiline scope without YAML frontmatter fails. |
| `graph impact` needs `--file` | **confirmed** | Without `--file` → EXIT 1. |
| Review false positives on tests/hidden invariant | **confirmed** | Adding `tests/hidden/test_x.py` in scope → critical finding from the default invariant. |
| Verify runs only partial checks / misses build errors | **confirmed** | Syntax error in `examples/mcpserver/unicode_example.py`: native `py_compile` fails; `tb verify` with spot pytest → PASS. Hidden suite “unavailable” even with `hidden_test_cmd` set if `tests/hidden` dir does not exist. |
| tb does not push agent toward completing whole task | **confirmed (qualitative in this study)** | Init Go defaults; does not import AGENTS.md; verify does not require project lint/typecheck/docs; ticket CLI absent; empty graph if cwd=`target`. Agent can “close” the tb loop with PASS and still omit native AGENTS.md checks (`pyright`, `./scripts/test`, encoding rules, etc.). |

## New problems

### P0 — `SkipDirs` swallows the root if the basename is `target`/`build`/`dist`/`bin`/…

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb graph rebuild --enable` → `0 nós`. Copy to `/tmp/python-sdk` → 16550 nodes.
- **Expected:** never skip the walk root; compare relative paths or only children.
- **Actual:** `skipDirs[info.Name()]` on the root itself → immediate `filepath.SkipDir`.

### P1 — `tb init` Go defaults + does not import repo instrumentation

- **Repro:** `tb init` on python-sdk → `verify.test_cmd: go test ./...`; empty vault; AGENTS.md ignored.
- **Expected:** detect ecosystem (pyproject/uv) and/or import AGENTS.md/.claude skills as initial context.
- **Actual:** generic template; onboarding does not point the agent at real checks.

### P1 — Multiline markdown scope without frontmatter is silently incomplete

- **Repro:** `rd.md` with `scope:\n  - a\n  - b` without `---`; drift reports a false positive.
- **Expected:** parse the YAML list or an explicit “unreadable scope” error.
- **Actual:** regex only reads `scope: ...` on the same line; list ignored.

### P2 — `graph search` does not find field symbols / `callers` almost empty in Python

- **Repro:** after indexing 16k nodes, `search encoding_error_handler` and `callers stdio_client` → empty; only 100 `calls` edges in the whole SDK.
- **Expected:** at least hits on indexed definitions/attributes; useful callers in Python.
- **Actual:** shallow search; Python call-graph insufficient for task navigation.

### P2 — Visual PASS with `skipped=false` and zero screens

- **Repro:** `.toolbelt/screens.yaml` with `screens: []` + `--skip-unavailable` → JSON `skipped: false`.
- **Expected:** `skipped: true` (or FAIL) when nothing was compared.
- **Actual:** PASS appearing as a real run.

### P3 — MCP `tb_graph_*` inherits empty graph / needs `TB_GRAPH=on` without rich feedback

- **Repro:** under `/tmp/target`, `tools/call tb_graph_search` → `"null"`.
- **Expected:** explicit error (“empty graph / root skipped / enable graph”).
- **Actual:** silent null result.

## What tb caught and what it let slip

**Caught**

- Scope drift/review when the ticket is in correct YAML frontmatter.
- Guard on hardcoded protected paths (`thresholds.yaml`, `tests/hidden/`).
- Verify running the configured `test_cmd` (pytest stdio) and failing when the package import breaks.
- Persistent invariants (`mem`) and fresh-context review with no findings on the clean fix.
- Fake bench + A/B compare (with toolbelt’s own fixtures).
- MCP tools/list and `tb_version`.

**Let slip / did not cover**

- Syntax error in a file outside `test_cmd` (example) → verify PASS.
- “Hidden” suite configured by command but marked unavailable without a `tests/hidden/` directory.
- Lint/typecheck/docs/`./scripts/test` required by AGENTS.md — tb neither suggests nor queues them on init.
- Project `guard.yaml`.
- AGENTS.md / Claude skills content (unread).
- Useless graph while the clone lives under `/tmp/target`.
- Task completeness beyond what the narrow ticket+test_cmd measures (e.g. updating SDK docs, conformance).

## Interaction with existing instrumentation / onboarding

python-sdk has mature AI instrumentation (dense `AGENTS.md`, `CLAUDE.md` alias, `test-quality` skill, `review-pr` command).

| Observed behavior | Detail |
|-------------------|--------|
| tb **ignores** AGENTS.md | No command read/imported; vault stays empty after init. |
| tb **partially duplicates** the idea of invariants | `mem init` creates generic anti-cheat (Go-ish: `tests/hidden`, `thresholds.yaml`) that **conflicts** with tb’s own model (create a hidden suite) and does not reflect AGENTS.md rules (uv-only, encoding=, 100% coverage, no `Test*` classes, etc.). |
| tb **does not replace** the native harness | AGENTS.md requires `uv run --frozen`, ruff, pyright, `./scripts/test`; default/`test_cmd`-only verify does not cover that. |
| What tb should import | (1) Read `AGENTS.md`/`CLAUDE.md` on `init` and materialize invariants/mandate; (2) map quality scripts (`scripts/test`, ruff, pyright) to `verify.test_cmd` / checklist; (3) index `.claude/skills/*` in the vault; (4) warn on conflicts (e.g. “never touch tests/hidden” invariant vs need to create the suite). |

## Improvement suggestions

1. **[P0][graph]** Do not apply `SkipDirs` to the walk root; treat `target`/`build` only as children.  
2. **[P0][init/verify]** Detect stack (Go/Python/Node) and fill `verify.*` with real commands; fail init if defaults are clearly wrong.  
3. **[P1][mem/ctx]** Import `AGENTS.md` on onboarding; generate invariants from it instead of the generic template.  
4. **[P1][drift/review]** Accept multiline YAML scope without requiring `---` **or** an explicit error; document frontmatter.  
5. **[P1][verify]** Treat configured `hidden_test_cmd` as available; do not depend only on `tests/hidden/` existing. Add optional `lint/build` ruler.  
6. **[P1][guard]** Load `.toolbelt/guard.yaml`; separate “touch thresholds” from aborting verify without an actionable message.  
7. **[P1][docs]** Sync `docs/cli.md` with real Cobra (remove/mark `ticket`, fix `mem add`).  
8. **[P2][bench]** Honor `bench.default_agent`; add `--agent` to `compare`.  
9. **[P2][verify/visual]** If zero screens evaluated → `skipped=true` (or FAIL), never silent PASS.  
10. **[P2][graph]** Improve Python index (attributes, callers) and message when rebuild yields 0 nodes.  
11. **[P2][graph]** Support `**` globs in scope (or document only `path/*` and `path/**`).  
12. **[P3][config]** Revisit 0600 on shared monorepos; 0644 + separate secrets.  
13. **[P3][mcp]** Structured errors when the graph is disabled/empty.

## Metrics

| metric | value |
|--------|--------|
| Summed wall time of timed commands in the log | **~35.4 s** (70 `WALL=` samples; excludes clone/`uv sync`/human reading) |
| Approximate study-session wall time (incl. sync, copies, exploration) | **~15–25 min** calendar |
| Timed tb invocations | **~55–60** `tb*` commands (rest = native/git) |
| Failures (EXIT≠0) among all timed samples | **25 / 70** (includes **intentional** bug-repro failures and negative native checks) |
| “Unexpected” tb failures on the happy fix path | few after correct frontmatter ticket: final verify/drift/review/guard PASS |
| Largest individual WALL | **9.96 s** (native `pytest` before the venv pointed at the right patch) |
| Useful graph rebuild | **1.345 s** / 16550 nodes (`/tmp/python-sdk`) |
| Final fix verify | **3.518 s**, PASS |
| MCP smoke | **<0.01 s** per request |
