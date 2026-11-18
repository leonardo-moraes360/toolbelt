🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/requests.md)

# Field round 2: psf/requests

Group: **without existing AI-agent instrumentation**.  
Tool: `tb` built from `main` (`make build`, ldflags `a46c295`).  
Local clone: `/tmp/target` (basename `target` on purpose to revalidate SkipDirs).  
No interaction with the remote `psf/requests` (no push/issue/PR/comment).

## Repository

| Field | Value |
|-------|-------|
| Name | [psf/requests](https://github.com/psf/requests) |
| Language | Python ≥3.10 |
| Size | ~34 `.py` under `src/`+`tests/`; ~6396 LOC in `src/requests`; working tree ~5.8 MiB (without `.git`/`.venv`/`.toolbelt`); shallow clone depth 50 |
| Base commit | `3c7baaa` |
| Existing AI instrumentation | **No** `AGENTS.md`, `CLAUDE.md`, `.cursorrules`. There is `.github/AI_POLICY.md` (human policy). |
| Toolchain | **Partial.** `python3.12` + venv + `requirements-dev.txt` + editable install + `ruff` + `pytest`. Patch target suite: **13 passed**. **Not configured:** multi-version `tox`, `pyright`, Sphinx docs, full pre-commit. Full suite not re-run (environment with known network timeouts from round 1). |

## Chosen task

| Field | Value |
|-------|-------|
| What | In `should_bypass_proxies`, normalize **bracketed** IPv6 entries in `no_proxy` (e.g. `[2001:db8::1]`) to match `urlparse().hostname` (without brackets). Parametrized test in `tests/test_utils.py`. |
| Why | Clear bug, local repro without network, small scope, distinct from round 1. |
| Issue | No open issue consulted (protocol: no network beyond clone/deps). Motivated by local failure: `no_proxy='[2001:db8::1]'` did not bypass `http://[2001:db8::1]/`. |
| Completed? | **Yes on the local clone.** Diff: 2 lines in `utils.py` + 4 test cases. No `HISTORY.md` / upstream PR (forbidden). |
| Native checks | `pytest tests/test_utils.py -k 'bypass_proxies_no_proxy_ipv6 or bypass_proxies_no_proxy_domain'`: **13 passed**. `ruff check` on touched files: **ok**. |
| Round 1 (comparison) | Digest `qop` with spaces after commas (`auth.py` + `test_lowlevel.py`, issue #7648). |

## Verification of the 12 fixes

| # | fix | worked / did not work / not tested / not applicable | evidence |
|---|-----|-----------------------------------------------------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph index --enable .` under `/tmp/target` → **853 nodes, 2349 edges** (round 1: 0 nodes on the same path) |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; config with domain pytest `hidden_test_cmd` → JSON `hidden_available: true`, `hidden_passed: true`, verdict PASS |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff touching a protected path → JSON with `severity:error` and **EXIT=1** |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `src/requests/adapters.py`; probe commit → `protected_path` on that file |
| 5 | init toolchain detection | **worked** | `tb init` → `verify.test_cmd: pytest` (not `go test`); probe `/tmp/agents-probe` same |
| 6 | review `tests/hidden` matches by path | **worked** | Ticket mentions `tests/hidden/**` only as forbidden/prose; clean review (`findings: []`, no critical FP) |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Without `--agent` with `default_agent: echo…` → `agent=external`, EXIT=0; without default → EXIT=1; `compare --help` lists `-a/--agent`; A/B with `--agent` ran |
| 8 | review `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | Ticket with only `## Scope` + bullets → review PASS; frontmatter `src/requests/**/*.py` + `tests/**` covered `utils.py`/`test_utils.py` |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | JSON: `build_ran/passed: true`, `lint_ran/passed: true` before tests |
| 10 | init reads AGENTS/CLAUDE and `--write-agents` | **worked** | Flag exists; in a repo without docs, `tb init --write-agents` seeded `AGENTS.md` + `.toolbelt/agent_docs.yaml`. Under `/tmp/target` (no instrumentation) init without the flag does not create AGENTS (expected) |
| 11 | graph Python `calls` | **worked** | Stats: `edges_calls: 1170`. `callers --function should_bypass_proxies` listed tests + `get_environ_proxies`; `impact --file src/requests/utils.py` non-empty |
| 12 | aggregated visual skipped + CLI docs aligned | **worked** | Manifesto with missing refs → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` documents `tb mem add <regra>` (no phantom `--type`); CLI rejects `--type` (EXIT=1) consistently with current docs |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; `test_cmd: pytest` | **helped** | Correct Python onboarding vs round 1 (`go test`) |
| `tb init --write-agents` | Seeds `AGENTS.md` | **helped** | Probe `/tmp/agents-probe`; mode 0600 |
| `tb mem init/list/check/add` | OK | neutral/helped | Anti-cheat invariants; `mem add --type` still errors (docs ok) |
| `tb graph index` under `/tmp/target` | 853 nodes / 2349 edges | **helped** | Root SkipDirs fixed |
| `tb graph search/callers/impact` | useful callers/impact in Python | **helped** | Unlike round 1 (empty) |
| Ticket `rd.md` + `## Scope` | Parse OK | **helped** | Heading+bullets and `**` globs |
| `tb verify run` | PASS with build/lint/hidden | **helped** | No `tests/hidden/` folder |
| `tb verify --ruler visual` | aggregated PASS skipped | neutral | No real UI in the project |
| `tb review` | Clean with correct scope | **helped** | No `tests/hidden` FP |
| `tb drift` | No out-of-scope on the fix | helped | Aligned with review |
| `tb guard check --strict --json` | EXIT=1 with violation | **helped** | `guard.yaml` honored |
| `tb bench run` / `compare --agent` | OK with toolbelt fixtures | neutral | Target repo has no own `bench/tasks` |
| `tb mcp` tools/list | JSON-RPC subset | neutral | echo/env/run/graph_*/version; no mem/verify/review |
| `tb version` | `a46c295` | neutral | With ldflags |

## Problems that persist

- **Incomplete MCP** vs CLI (no mem/verify/review/drift/guard) — same as round 1.
- **Verify only as broad as `test_cmd`**: with `-k ipv6` filter, errors in other files do not enter the verdict (same pattern as round 1).
- **Init does not fill `lint_cmd`/`build_cmd`** automatically (only `test_cmd`); still manual YAML editing.
- **`graph impact` requires `--file`** (rigid UX unchanged).
- **Configs mode `0600`** (confirmed on `config.yaml` / seeded `AGENTS.md`).
- **Target repo without bench tasks**: must point `-t/-f` at the toolbelt tree.
- Architecture/`cli.md` docs still mention `--manifest`/extra promises in places; the effective `guard check <base>..<head>` contract requires a positional range (easy UX mistake).

## New problems

### P2 — Python `calls` edges with spurious targets

- **Repro:** After `tb graph index --enable .`, inspect `calls` edges (e.g. `file:setup.py` → `func:tests.setup`, confidence 0.55).
- **Expected:** Conservative resolution within the module / real imports.
- **Actual:** Heuristic yields useful callers/impact, but also low-precision pairs that can confuse the agent.

### P2 — Onboarding still needs manual YAML for lint/build/hidden

- **Repro:** `tb init` on requests → `lint_cmd`/`build_cmd` absent; `hidden_test_cmd: ""`.
- **Expected:** Suggest `ruff`/`pytest` subset from `pyproject.toml`/`Makefile`.
- **Actual:** Only `test_cmd: pytest`; build/lint/hidden configured by hand in this study.

### P3 — Visual manifesto format not obvious

- **Repro:** YAML with singular `reference:` key does not aggregate like the `references: {light: …}` schema; with the right schema, aggregated skipped works.
- **Expected:** Schema error/warning or accept an alias.
- **Actual:** Silence / different behavior depending on YAML shape.

## What tb caught and what it let slip

**Caught**

- Indexing under `/tmp/target` (SkipDirs regression gone).
- Callers of `should_bypass_proxies` tests via `calls` edges.
- Drift/review with `## Scope` and `**` globs.
- Build+lint+hidden in verify without a `tests/hidden/` folder.
- `guard.yaml` violation on `adapters.py` with `--strict --json` (exit 1).
- `AGENTS.md` seed via `--write-agents` in a repo without instrumentation.

**Let slip**

- Contribution completeness (`HISTORY.md` / CONTRIBUTING checklist) — no nudge.
- Coverage beyond the narrow `test_cmd`.
- Quality/precision of some `calls` edges.
- Policy in `.github/AI_POLICY.md` (not imported into mem/invariants).

**Native vs tb:** pytest+ruff validated the fix; `tb verify` with the same commands also PASS. tb did not replace tox/pyright/full suite.

## Interaction with existing instrumentation / onboarding

Requests **has no** agent harness; only `AI_POLICY.md`.

`tb init` + `mem init` created `.toolbelt/` (config, vault, logs, bench, invariants). With `--write-agents` (probe), generated a minimal `AGENTS.md` pointing at verify/guard/mem.

**Improvement vs round 1:** correct Python defaults + AGENTS seed option.  
**Still missing:** import lint/build hints from `Makefile`/`pyproject`; reference `AI_POLICY.md`; `rd.md` template.

## Comparison with round 1

| Aspect | Round 1 | Round 2 | Verdict |
|--------|---------|---------|---------|
| SkipDirs `/tmp/target` | 0 nodes | 853 nodes | **better** |
| init `test_cmd` | `go test ./...` | `pytest` | **better** |
| graph callers/impact Python | empty | populated (`calls`) | **better** |
| hidden without dir | ignored | runs | **better** |
| guard.yaml | ignored | read | **better** |
| `--strict --json` | loose (when tested) | EXIT=1 | **better** |
| review FP `tests/hidden` | yes | no | **better** |
| `## Scope` + `**` | fragile | OK | **better** |
| build/lint in verify | inert | honored | **better** |
| default_agent / compare `--agent` | broken | OK | **better** |
| AGENTS / `--write-agents` | absent | works | **better** |
| aggregated visual skipped | partial | OK in all-skipped scenario | **better** |
| MCP subset / task completeness | gaps | gaps | **same** |
| `calls` precision | N/A | new noise | **mixed** |

In aggregate: **clearly better** than round 1 for the end-to-end flow in this repo; the 12 listed fixes **worked** under `tb@a46c295`.

## Improvement suggestions

1. **[P1 · graph]** Filter / rank Python `calls` by confidence and module scope; hide pairs &lt;0.6 in default `callers`.  
2. **[P1 · init]** Fill `lint_cmd`/`build_cmd` from `pyproject`/`Makefile` (e.g. `ruff check`, `python -m build`).  
3. **[P2 · verify]** Warn when `test_cmd` is a subset (`-k` / single path) without a broad `hidden_test_cmd`.  
4. **[P2 · guard]** Accept `--base/--head` flags in addition to `base..head` (UX aligned with review/drift).  
5. **[P2 · visual]** Validate manifesto schema and `reference`→`references` alias.  
6. **[P2 · mcp]** Expose mem/verify/review/guard on MCP.  
7. **[P3 · onboarding]** When `.github/AI_POLICY.md` is detected, record a pointer in `agent_docs.yaml` / mem.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Wall time (active tb study + patch + checks) | ~25–35 min (includes rebuild after `main` fast-forward) | ~25–30 min |
| Approx. `tb` invocations | ~45 (post-`a46c295` revalidation included) | ~32 |
| tb failures (expected exit ≠ 0 / probes) | ~8 (stale-binary old doc flags, guard without range, bench without tasks, mem `--type`, probes) | ≥15 |
| Crashes | None | None |
| Final binary | `toolbelt a46c295` | `c8aae04` |
| Target clone | Local only; zero `psf/requests` GitHub interaction | same |
| Artifact | Only `docs/campo/rodada2/requests.md` on branch `campo2/requests` | `docs/campo/requests.md` |
