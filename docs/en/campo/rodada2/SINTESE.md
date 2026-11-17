[🇧🇷 Português](../../../campo/rodada2/SINTESE.md) | 🇺🇸 English

# Field study synthesis for `tb` — round 2 (20 repositories)

> Date: 2026-11-16 · Branch: `campo2/sintese` · Typical agent binary: `toolbelt@a46c295`  
> Raw reports: [`docs/campo/rodada2/<slug>.md`](.) · Template: [`PROMPT.md`](PROMPT.md) · Round 1: [`../SINTESE.md`](../SINTESE.md)

This document aggregates the 20 round-2 reports (revalidation of the **12 fixes** merged after round 1), without reimplementing code. Counts and verdicts come **only** from the tables and sections of the cited reports.

---

## (a) Matrix of the 12 fixes × 20 repositories

### Legend

| Symbol | Meaning (normalized from the report status column) |
|--------|-----------------------------------------------------|
| ✓ | worked |
| ~ | partial / worked with caveat |
| ✗ | did not work |
| N/A | not applicable |
| NT | not tested |

### Fixes (same order as [`PROMPT.md`](PROMPT.md))

1. graph SkipDirs at walk root  
2. verify `hidden_test_cmd` without `tests/hidden/`  
3. guard `--strict --json` exit ≠ 0  
4. guard reads `protected_paths` from `guard.yaml`  
5. init toolchain detection  
6. review `tests/hidden` matches by path  
7. bench honors `default_agent` and `compare --agent`  
8. review/drift parses `## Scope`/`## Escopo`, MD lists, and `**` globs  
9. verify `build_cmd` and `lint_cmd`  
10. init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents`  
11. graph `calls` edges in Python  
12. verify aggregated visual skipped + CLI docs aligned  

### Matrix

Column abbreviations = slug (study order).

| # | pm | ol | k6 | ex | ky | mcp | cr | se | ru | pa | co | gi | rg | ba | ty | cl | re | xp | cm | jq | ✓ | ~ | ✗ | N/A |
|---|----|----|----|----|----|-----|----|----|----|----|----|----|----|----|----|----|----|----|----|----|---|---|---|-----|
| 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 2 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 3 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 4 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **19** | **1** | 0 | 0 |
| 5 | ✓ | ✓ | ✓ | ~ | ~ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | **16** | **4** | 0 | 0 |
| 6 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **19** | **1** | 0 | 0 |
| 7 | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **19** | **1** | 0 | 0 |
| 8 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **18** | **2** | 0 | 0 |
| 9 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 10 | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ~ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | **16** | **4** | 0 | 0 |
| 11 | N/A | N/A | N/A | N/A | N/A | ✓ | N/A | ✓ | ✓ | ~ | ✓ | N/A | N/A | N/A | ✓ | ✓ | ✓ | N/A | N/A | N/A | **7** | **1** | 0 | **12** |
| 12 | ~ | ~ | ~ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ~ | ✓ | ✓ | ~ | ✓ | **13** | **7** | 0 | 0 |

Slugs: pm=[playwright-mcp](playwright-mcp.md), ol=[ollama](ollama.md), k6=[k6](k6.md), ex=[excalidraw](excalidraw.md), ky=[ky](ky.md), mcp=[mcp-python-sdk](mcp-python-sdk.md), cr=[crush](crush.md), se=[sentry-python](sentry-python.md), ru=[ruff](ruff.md), pa=[pydantic-ai](pydantic-ai.md), co=[cobra](cobra.md), gi=[gin](gin.md), rg=[ripgrep](ripgrep.md), ba=[bat](bat.md), ty=[typer](typer.md), cl=[click](click.md), re=[requests](requests.md), xp=[express](express.md), cm=[commander-js](commander-js.md), jq=[jq](jq.md).

### Totals by fix (summary)

| # | Fix | ✓ | ~ | ✗ | N/A | Aggregate reading |
|---|-----|---|---|---|-----|-------------------|
| 1 | SkipDirs at root | 20 | 0 | 0 | 0 | Unanimous consensus; root-symlink nuance shows up as a *new* problem in [k6](k6.md)/[click](click.md), not as a failure of the fix |
| 2 | `hidden_test_cmd` without dir | 20 | 0 | 0 | 0 | Configured held-out suite now runs |
| 3 | `--strict --json` exit ≠ 0 | 20 | 0 | 0 | 0 | CI no longer swallows a violation under JSON |
| 4 | `guard.yaml` `protected_paths` | 19 | 1 | 0 | 0 | ~ in [gin](gin.md): `secrets/` (trailing slash only) does not match children via `filepath.Match` |
| 5 | init toolchain | 16 | 4 | 0 | 0 | Detects `npm`/`cargo`/`pytest`/`uv`/`make`/`go`; ~ = does not fill `build_cmd`/`lint_cmd` or picks an incomplete harness ([excalidraw](excalidraw.md), [ky](ky.md), [sentry-python](sentry-python.md), [typer](typer.md)) |
| 6 | review path `tests/hidden` | 19 | 1 | 0 | 0 | ~ in [pydantic-ai](pydantic-ai.md): no substring FP, but `forbidden_hit` is null when touching the dir |
| 7 | `default_agent` / `compare --agent` | 19 | 1 | 0 | 0 | ~ in [playwright-mcp](playwright-mcp.md): default agent OK, but absolute `tasks_dir` from YAML is ignored |
| 8 | Scope MD + `**` | 18 | 2 | 0 | 0 | ~ in [pydantic-ai](pydantic-ai.md) (`Forbidden **`) and [gin](gin.md) (YAML frontmatter without `scope:` ignores `## Escopo` in the body) |
| 9 | `build_cmd`/`lint_cmd` | 20 | 0 | 0 | 0 | Honored when set; residual gap is **auto-fill** in init |
| 10 | AGENTS/`--write-agents` | 16 | 4 | 0 | 0 | ~ = reads but empty/noisy hints or incomplete seed ([ky](ky.md), [crush](crush.md), [ruff](ruff.md), [click](click.md)) |
| 11 | Python `calls` | 7 | 1 | 0 | 12 | N/A outside Python (12); ✓ on Python repos (+ probes in [ruff](ruff.md)/[cobra](cobra.md)); ~ in [pydantic-ai](pydantic-ai.md) (empty `callers redact_secrets`) |
| 12 | visual skipped + docs | 13 | 7 | 0 | 0 | “All screens skipped” aggregation OK on most; ~ for `screens: []` still `skipped=false` and/or residual CLI docs |

**No report marked ✗ (did not work) or NT (not tested) on the 12 fixes.** Partials cluster around deep onboarding (5, 10), scope/forbidden edge cases (6, 8), bench paths (7), guard globs (4), Python callers (11), and visual/docs (12).

---

## (b) Comparison with round 1

### Comparison method

- **R1**: [`docs/campo/SINTESE.md`](../SINTESE.md) + [`docs/campo/<slug>.md`](../) · typical binary `c8aae04`.  
- **R2**: this directory · typical binary `a46c295`.  
- Per-repo verdict = **“Comparison with round 1”** section of each R2 report (not invented here).  
- R2 failures are largely **intentional probes** (OOS drift, guard strict, dead flags); in R1 many failures were **product bugs** among the 12 fixes.

### Summary table

| Slug | Task R1 → R2 | Complete R1 | Complete R2 | tb failures R1 → R2 (report) | Wall / session (report) | R2 verdict |
|------|--------------|-------------|-------------|------------------------------|-------------------------|------------|
| [playwright-mcp](playwright-mcp.md) | env docs → AGENTS+cli tests | yes | yes | 14 → 18 (probes) | ~3.4 min → ~2.5 min cmds | **better** |
| [ollama](ollama.md) | verbose launch → HumanNumber | yes | yes | 13 → 3+ intentional | ~6–10 min → ~8–12 min | **better** |
| [k6](k6.md) | TagSet partial → WithField | **partial** | **yes** | 12 → probes; 0 crashes | ~70 s log → ~4.5 min harness | **better** |
| [excalidraw](excalidraw.md) | normalizeRadians → new task | yes | yes | product bugs → expected ESCALATE lint | ~49 s → ~103 s (more checks) | **better** |
| [ky](ky.md) | retry.jitter → new task | yes | yes | 14 → probes | tb ~40.7 s → ~6.2 s | **better** |
| [mcp-python-sdk](mcp-python-sdk.md) | encoding handler → display_name | yes | yes | 25/70 → 9 | ~35.4 s → ~24.7 s | **better** |
| [crush](crush.md) | slog capitalization → Capitalize | yes | yes | 10 → 14 probes | ~3 min → ~2.5 min | **better** |
| [sentry-python](sentry-python.md) | EventScrubber → streaming | yes | yes | 13 → 17 probes | ~10–12 min → session + ~28 s timed | **better** |
| [ruff](ruff.md) | FURB open bytes → N801 | yes | yes | product bugs → empty Rust impact/callers | ~6–15 min → ~15–25 min | **better** |
| [pydantic-ai](pydantic-ai.md) | last_modified → redact/detectors | yes | yes | 26 → 12 | ~57 s → ~74 s invoc. | **better** |
| [cobra](cobra.md) | SetErrPrefix → ExactArgs | yes | yes | 19 → 9 | ~3.6 min → ~2.8 min evid. | **better** |
| [gin](gin.md) | BindUnmarshaler → ValidateStruct | yes | yes | 11 → 7 | ~5.2 min → ~3.8 min | **better** (with new frontmatter gap) |
| [ripgrep](ripgrep.md) | multiline `-U` → hyperlink | yes | yes | ~11–16 → ~8–12 | ~15–20 min → ~8–12 min | **better** |
| [bat](bat.md) | language/map-syntax → offset | yes | yes | 13 → ~12–14 | ~25–35 min → ~25–40 min | **better** |
| [typer](typer.md) | flaky TMPDIR → ZDOTDIR | yes | yes | 20 → 5 intentional | ~15–20 min → ~15–25 min | **better** |
| [click](click.md) | get_error_hint → lazy file | yes | yes | 19 → ~13 | similar (tens of minutes) | **better** |
| [requests](requests.md) | digest qop → IPv6 proxies | yes | yes | ≥15 → ~8 | ~25–30 min → ~25–35 min | **better** |
| [express](express.md) | subdomains IPv6 → DataView ETag | yes | yes | 22 → 6 | ~20 s cmds → ~19 s | **better** |
| [commander-js](commander-js.md) | Help.groupItems → parse args | yes | yes | 23 → 19 | ~5 min 47 s → ~3 min 46 s | **better** |
| [jq](jq.md) | literals `23.` → slice assign | yes | yes | 18 → several expected | ~25–30 min → ~20–25 min | **better** |

### Aggregate reading R1 → R2

- **Local conclusion**: R1 19/20 (k6 partial) → R2 **20/20** yes.  
- **Per-repo verdict**: **20/20 “better”** on the 12 exercisable fixes; gaps that remain “the same” are mainly incomplete MCP, review completeness, mode `0600`, missing `tb ticket`, weak Rust/C graph, and not pushing task completeness.  
- **tb failures**: raw counts sometimes rise (more probes of the 12 fixes); the *kind* changes — from *broken product* to *expected probe failure*. Happy paths (final verify/review/drift) pass where R1 failed.  
- **Graph `/tmp/target`**: from 0 nodes (several R1) to thousands (R2), e.g. [ky](ky.md) 589, [mcp-python-sdk](mcp-python-sdk.md) ~16k, [requests](requests.md) 853, [express](express.md) 452.  
- **Init**: from nearly universal `go test ./...` default → `npm test` / `cargo test` / `pytest` / `uv run pytest` / `make check` per toolchain.

---

## (c) Problems that persist and new problems (deduplicated)

Severity = highest cited across reports for the theme; count = how many R2 reports list the item under “persist” / “new” / suggestions with evidence.

### Persist from round 1 (still observed)

| # | Problem | Repos (n) | Sev. | Evidence |
|---|---------|-----------|------|----------|
| C1 | Init detects `test_cmd` but **does not fill** `build_cmd`/`lint_cmd` (nor applies AGENTS/package.json/Makefile hints) | ≥15 (e.g. [excalidraw](excalidraw.md), [ky](ky.md), [mcp-python-sdk](mcp-python-sdk.md), [sentry-python](sentry-python.md), [crush](crush.md), [requests](requests.md), [express](express.md), [commander-js](commander-js.md), [cobra](cobra.md), [bat](bat.md), [typer](typer.md), [ruff](ruff.md), …) | P1–P2 | Post-init only `test_cmd`; manual lint/build |
| C2 | `.toolbelt/*` configs (and sometimes `AGENTS.md` seed) in mode **0600** | ≥12 | P3 | R1 checklist; reconfirmed in several R2 |
| C3 | MCP without CLI parity (`mem`/`verify`/`review`/`drift`/`guard`) | ≥10 ([ky](ky.md), [sentry-python](sentry-python.md), [typer](typer.md), [requests](requests.md), [gin](gin.md), [ripgrep](ripgrep.md), …) | P2–P3 | `tools/list` only graph/version/echo/… |
| C4 | Noisy review **completeness** (accept with test in the diff still warns) | ≥8 ([ollama](ollama.md), [crush](crush.md), [express](express.md), [cobra](cobra.md), [bat](bat.md), [jq](jq.md), …) | P2–P3 | warning “criterion little evidenced” |
| C5 | `version` → `unknown` without ldflags | common | P3 | expected without `-ldflags` |
| C6 | No first-class `tb ticket` / mandate | common | P3 | docs mark it as planned |
| C7 | Rust/C graph: empty or useless `callers`/`impact` | [ruff](ruff.md), [ripgrep](ripgrep.md), [bat](bat.md), [jq](jq.md) (+ mentions) | P1–P2 | impact `(vazio)` despite index |
| C8 | Graph off without `TB_GRAPH`/`--enable` (MCP included) | common; [k6](k6.md) | P3 | aligned with ADR-011; still friction |
| C9 | Verify only as good as `test_cmd` (PASS without lint if `lint_cmd` empty) | [commander-js](commander-js.md), [express](express.md), [requests](requests.md), … | P2 | documented opt-in; false PASS if agent does not set |
| C10 | tb **does not push** completing the task on its own | qualitative, several | P2–P3 | no progress/checklist harness |
| C11 | CLI docs with residual examples/flags (`mem add --type`, `graph index --path`, etc.) | [ollama](ollama.md), [k6](k6.md), [commander-js](commander-js.md), [playwright-mcp](playwright-mcp.md), … | P3 | CLI rejects; docs still cite |
| C12 | Does not read `AI_POLICY.md` / `.agents/` skills | [ripgrep](ripgrep.md), [commander-js](commander-js.md), [ruff](ruff.md), [typer](typer.md), [sentry-python](sentry-python.md) (`agents.toml`) | P3 | outside init’s radar |

### New or reframed in round 2

| # | Problem | Repos (n) | Sev. | Evidence |
|---|---------|-----------|------|----------|
| N1 | `LoadTicket`: YAML frontmatter **without** `scope:` **ignores** `## Scope`/`## Escopo` in the body | 1 ([gin](gin.md)) | **P1** | review `scope-missing` with frontmatter+body MD |
| N2 | YAML `bench.tasks_dir` / `fixtures_dir` / `output_dir` **ignored** (relative default / `-t` wins) | ≥6 ([playwright-mcp](playwright-mcp.md), [crush](crush.md), [excalidraw](excalidraw.md), [sentry-python](sentry-python.md), [gin](gin.md), [bat](bat.md)) | P2 | `open bench/tasks: no such file` with absolute path in YAML |
| N3 | `guard.yaml` protecting the **task’s own path** → `tb verify` FAIL with **0 iterations** (does not run build/test) | ≥4 ([ollama](ollama.md), [k6](k6.md), [jq](jq.md), [bat](bat.md)) | P2 | verify aborts on custom `protected_path` |
| N4 | Root **symlink** named `target` → graph **0 nodes** (`filepath.Walk` does not descend) | ≥2 ([k6](k6.md), [click](click.md)) | P2 | `ln -s … /tmp/target` → 0; real dir OK |
| N5 | `screens: []` → top-level `skipped: false` / `screens: null` / PASS | ≥2 ([playwright-mcp](playwright-mcp.md), [mcp-python-sdk](mcp-python-sdk.md)) | P2 | distinct from “all screens skipped” |
| N6 | Visual `skipped: true` still with **verdict PASS** | ≥1 ([ripgrep](ripgrep.md)) | P2 | missing manifest → skipped true + PASS |
| N7 | `Forbidden` / `**` globs **do not** populate `forbidden_hit` (only `out_of_scope`) | ≥1 ([pydantic-ai](pydantic-ai.md)) | P2 | cheat under `tests/hidden/` without `forbidden_hit` |
| N8 | `guard check` UX: `--base`/`--head` **unknown**; requires positional `A..B` | ≥5 ([playwright-mcp](playwright-mcp.md), [ky](ky.md), [pydantic-ai](pydantic-ai.md), [gin](gin.md), [requests](requests.md)) | P2–P3 | inconsistent with verify/review |
| N9 | Noisy AGENTS hints (Go fences) or **empty** despite useful text | ≥4 ([crush](crush.md), [ky](ky.md), [click](click.md), [cobra](cobra.md)) | P2–P3 | `agent_docs.yaml` |
| N10 | Python `calls` edges with **spurious targets** / incomplete name-based `callers` | ≥3 ([requests](requests.md), [pydantic-ai](pydantic-ai.md), [click](click.md)) | P2 | e.g. `setup.py→tests.setup`; empty `redact_secrets` |
| N11 | Weak **JS** callers/impact despite populated index | ≥2 ([commander-js](commander-js.md), [ky](ky.md) `#private`) | P2 | `_prepareUserArgs` → empty |
| N12 | `tb init` chooses `uv run pytest` **without** `uv` on PATH | 1 ([typer](typer.md)) | P2 | `uv: command not found` |
| N13 | Bench marks `passed: true` with agent `exit_code: 1` | 1 ([jq](jq.md)) | P2 | Go fixture + agent `false` |
| N14 | `protected_paths: secrets/` (dir) **does not** protect children | 1 ([gin](gin.md)) | P2 | `secrets/*` works; `secrets/` null |
| N15 | Global `-c`/`--config` flag **does not** feed `config.Load()` in bench | 1 ([ripgrep](ripgrep.md)) | P2 | still requires `--agent` |
| N16 | `tb drift` with `HEAD^` → `base_mismatch` (compares string, not SHA) | 1 ([click](click.md)) | P2 | expanded SHA OK |
| N17 | Generic `AGENTS.md` seed / mode 0600; does not cite detected toolchain | ≥3 ([express](express.md), [jq](jq.md), [ripgrep](ripgrep.md)) | P2–P3 | harness stub |
| N18 | verify `ESCALATE`/`FAIL` can exit **0** with `--json` (failed-build scenario) | 1 ([jq](jq.md)) | P3 | `build_cmd: false` |
| N19 | `rd.md` outside Scope → drift/review error if not listed | ≥3 ([playwright-mcp](playwright-mcp.md), [cobra](cobra.md), …) | P3 | ticket path OOS |
| N20 | Docs/CLI: missing `graph query` (EXIT 0 + help); `review -o` only `--output` | [commander-js](commander-js.md), [k6](k6.md) | P3 | inconsistent UX |

---

## (d) Next improvements (1 PR each, merge order)

Order avoids conflicts (review → init/config → verify/guard → bench → graph → mcp → docs) and delivers value early. Files = **likely** packages (not edited in this synthesis).

| # | Suggested PR title | One line | Pri. | Likely files | Depends on |
|---|--------------------|----------|------|--------------|------------|
| 1 | `fix(review): merge ## Scope from body when frontmatter has no scope` | Closes the P1 from [gin](gin.md): YAML frontmatter without `scope:` must not void MD headings. | P0/P1 | `internal/review` (ticket parser) | — |
| 2 | `fix(review): populate forbidden_hit for Forbidden/** and hidden invariant` | Aligns drift/review with real touches of forbidden paths ([pydantic-ai](pydantic-ai.md)). | P1 | `internal/review` | 1 (same parser; merge after) |
| 3 | `feat(init): fill/suggest build_cmd and lint_cmd from toolchain` | Closes C1 (≥15 repos): `package.json` scripts, `pyproject`/Makefile/`golangci`, short AGENTS hints. | P1 | `internal/config`, `cmd/tb` init | — |
| 4 | `fix(init): filter AGENTS hints and validate binaries on PATH` | Hints without long fences ([crush](crush.md)); fallback if `uv` missing ([typer](typer.md)); optional invariant extraction. | P1 | init / light `internal/mem` | 3 |
| 5 | `fix(bench): honor tasks_dir/fixtures_dir/output_dir from config.yaml` | Absolute YAML paths stop being ignored (N2). | P1 | `cmd/tb/bench.go`, bench config | — |
| 6 | `fix(bench): do not mark passed when agent exit ≠ 0` | Fixes [jq](jq.md); optional exit ≠ 0 if `default_agent` is 127. | P1 | `internal/bench` / `cmd/tb/bench.go` | 5 (same package; after) |
| 7 | `fix(verify): empty screens and skipped must not give misleading PASS` | `screens: []` → `skipped=true` ([playwright-mcp](playwright-mcp.md), [mcp-python-sdk](mcp-python-sdk.md)); skipped ≠ PASS ([ripgrep](ripgrep.md)). | P1 | `internal/verify` (visual) | — |
| 8 | `fix(verify/guard): do not abort verify only for mandate protected_paths` | Dev mode / exception for paths in Scope (N3: [ollama](ollama.md), [k6](k6.md), [jq](jq.md), [bat](bat.md)). | P1 | `internal/verify`, `internal/guard` | — |
| 9 | `feat(guard): directory globs and --base/--head aliases` | `secrets/` → child prefix ([gin](gin.md)); UX aligned with verify/review (N8). | P1 | `internal/guard`, `cmd/tb/guard.go` | 8 (same package; after) |
| 10 | `fix(graph): EvalSymlinks / warn on symlink root` | Avoids 0 nodes under linked `/tmp/target` ([k6](k6.md), [click](click.md)). | P2 | `internal/graph` | — |
| 11 | `fix(graph): Python callers by suffix + confidence filter` | Cuts noise ([requests](requests.md)) and name holes ([pydantic-ai](pydantic-ai.md)). | P2 | `internal/graph` (Python index) | 10 |
| 12 | `feat(graph): minimal Rust calls/impact or explicit error` | Closes C7 ([ruff](ruff.md), [ripgrep](ripgrep.md), [bat](bat.md)); optional JS class methods in same cycle or sibling PR. | P2 | `internal/graph` | 10 |
| 13 | `feat(mcp): expose mem/verify/review/drift/guard` | Parity with the field loop (C3). | P2 | MCP server / tools | — |
| 14 | `fix(cli): drift resolves git refs; exit ≠ 0 on ESCALATE JSON` | `HEAD^` ([click](click.md)); verify exit code ([jq](jq.md)). | P2 | `internal/review` drift, `cmd/tb/verify` | — |
| 15 | `docs(cli): remove dead examples + AGENTS seed with toolchain` | `mem add --type`/`--category`, `graph index --path`, `graph query`; seed cites detected `test_cmd` (C11, N17, N20). | P3 | `docs/cli.md`, init seed | after 3–4 |

Maximum 15 PRs; item 12 can split into `feat(graph): Rust calls` + `fix(graph): callers JS class methods` if review prefers smaller diffs. Item 15 can be docs-only micros in parallel.

### Explicitly **not** prioritized now

| Item | Why (R2 field) |
|------|----------------|
| Turn on `TB_GRAPH=on` by default | Still aligned with ADR-011; pain is MCP/warning, not making it mandatory |
| Remove mode `0600` from secret configs | Still good hygiene; only public `AGENTS.md`/YAML seeds want 0644 |
| Rewrite graph with full tree-sitter | High cost; incremental gain from callers/filter/minimal Rust attacks the pain |
| Router/A2A/workers | Zero evidence in these 20 single-agent tasks |

---

## Next step

Use this synthesis as **input for the next PR queue**, starting with items 1–9 (P0/P1). Raw reports remain in `docs/campo/rodada2/<slug>.md`. Round 1 synthesis: [`docs/campo/SINTESE.md`](../SINTESE.md).
