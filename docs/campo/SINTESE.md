🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/SINTESE.md)

# Synthesis of the `tb` field study (20 repositories)

> Date: 2026-11-15 · Branch: `campo/sintese` · Typical agent binary: `toolbelt@c8aae04`  
> Raw reports: [`docs/campo/<slug>.md`](.) · Research: [`docs/pesquisa/`](../pesquisa/) · ADRs: [`docs/adr/`](../adr/)

This document aggregates the 20 field reports, checks the strongest claims against the current `tb` code, cross-references research in `docs/pesquisa/`, and selects ranked improvements for focused PRs (without implementing code in this delivery).

---

## (a) Method and table of the 20 repositories

### Method

1. **Sample**: 20 local clones of public repositories (10 with existing AI agent instrumentation; 10 without).
2. **Protocol**: each cloud agent used `tb` on the clone, chose a small/medium maintainer-plausible task, ran the command battery (`init`, `mem`, `graph`, `drift`, `review`, `verify`, `guard`, `bench`), and published **one** report in `docs/campo/<slug>.md` on branch `campo/<slug>`.
3. **Synthesis**: this branch brings the 20 files **unchanged**, re-reads research/ADRs/README, and **revalidates in code** the recurring claims (list in the verification section).
4. **Limitations**: tasks and native tooling vary; some native checks were partial due to cost; the clone under `/tmp/target` biased discovery of the `SkipDirs` bug; interaction with upstream was forbidden.

### Groups

| Group | Slugs |
|-------|-------|
| **With** AI instrumentation (`AGENTS.md` / `CLAUDE.md` / skills / etc.) | playwright-mcp, ollama, k6, excalidraw, ky, mcp-python-sdk, crush, sentry-python, ruff, pydantic-ai |
| **Without** AI instrumentation | cobra, gin, ripgrep, bat, typer, click, requests, express, commander-js, jq |

### Summary table

| Slug | Language | Group | Task (summary) | Complete? | Native checks (result) |
|------|----------|-------|----------------|-----------|------------------------|
| [playwright-mcp](playwright-mcp.md) | JS/TS | with | Env var docs in `update-readme.js` + CLI regression | Yes (local) | `playwright test` 2 passed; `npm run lint` OK |
| [ollama](ollama.md) | Go (+TS/C++) | with | Missing `verbose` flag in `ollama launch` | Yes (local) | `go test ./cmd/` OK; native cmake build failed on VM |
| [k6](k6.md) | Go | with | `TagSet.Len` / `Iterate` | Partial | `go test ./metrics/` + build + vet OK; lint/`-race` not run |
| [excalidraw](excalidraw.md) | TS/React | with | `normalizeRadians` → `0` on multiples of 2π | Yes (local) | vitest math PASS; typecheck PASS |
| [ky](ky.md) | TS | with | Reject invalid `retry.jitter` | Yes (local) | `npm run build` OK; retry tests OK |
| [mcp-python-sdk](mcp-python-sdk.md) | Python | with | Default `encoding_error_handler=replace` | Yes (local) | pytest stdio 37+2 passed; ruff OK |
| [crush](crush.md) | Go | with | Capitalization lint for `slog.*Context` | Yes (local) | scripts + `task lint:log` + `go build` OK |
| [sentry-python](sentry-python.md) | Python | with | `EventScrubber` hyphen/underscore + denylist | Yes (local) | tox scrubber 15 passed; ruff OK |
| [ruff](ruff.md) | Rust | with | FURB101/103 should not “fix” `open(b"...")` | Yes (local) | `cargo test -p ruff_linter …refurb` PASS |
| [pydantic-ai](pydantic-ai.md) | Python | with | `MessagesBuilder.last_modified` | Yes (local) | ruff/pyright OK; target pytest PASS |
| [cobra](cobra.md) | Go | without | `SetErrPrefix("")` | Yes (local) | `go test ./...` PASS; lint with pre-existing issues |
| [gin](gin.md) | Go | without | Field name in BindUnmarshaler errors | Yes (local) | `go test`/`build` PASS; pre-existing lint in package |
| [ripgrep](ripgrep.md) | Rust | without | Wrong lines with `-U`+`-r` multiline | Yes (local) | printer tests + target suite PASS; fmt OK |
| [bat](bat.md) | Rust | without | Validate `--language`/`--map-syntax` with color never | Yes (local) | build/fmt/clippy + target integration tests OK |
| [typer](typer.md) | Python | without | Flaky `test_file_error` with long `TMPDIR` | Yes (local) | file pytest PASS; ruff OK |
| [click](click.md) | Python | without | `get_error_hint` + `auto_envvar_prefix` | Yes (local) | 2627 passed; ruff/mypy on touched files OK |
| [requests](requests.md) | Python | without | Digest `qop` with spaces after comma | Yes (local) | pytest digest 4 passed; ruff OK |
| [express](express.md) | JS | without | `req.subdomains` with mapped IPv6 | Yes (local) | `npm test` + lint PASS |
| [commander-js](commander-js.md) | JS/TS | without | `Help.groupItems` Map→record + typed alias | Yes (local) | `npm test` + `npm run check` PASS |
| [jq](jq.md) | C | without | Reject literals `23.` / `-23.` / `0.` | Yes (local; 1st attempt failed) | `make check` 9/9 PASS |

**Local completion rate**: 19/20 yes; 1/20 partial (k6). No task was submitted upstream (study protocol).

---

## Verification of claims in the current code

Each claim below was checked against the source on `main`/`campo/sintese` (packages cited).

| # | Claim | Verdict | Evidence in code |
|---|-------|---------|------------------|
| 1 | `SkipDirs` contains `target` and applies to the walk **root basename** → indexing `/tmp/target` yields 0 nodes | **Verified** | `internal/graph/indexer.go`: `SkipDirs` includes `"target"`; `walkSourceFile` does `if skipDirs[info.Name()] { return filepath.SkipDir }` without excluding the root |
| 2 | `tb init` writes `verify.test_cmd: go test ./...` by default | **Verified** | `internal/config/config.go` `DefaultConfig().Verify` |
| 3 | `.toolbelt/guard.yaml` is ignored | **Verified** | `internal/guard/guard.go` `NewChecker` only hardcodes `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`; no YAML loader |
| 4 | `bench.default_agent` is ignored | **Verified** | field exists in `BenchConfig`, but `parseBenchRunFlags` in `cmd/tb/bench.go` requires `--agent`/`--fake` and does not read the YAML |
| 5 | `tb verify` does not run build/lint (only `test_cmd` + hidden) | **Verified** | `internal/verify/verify.go` `Run` only calls `VisibleTest` / `HiddenTest` |
| 6 | `hidden_test_cmd` only runs if `tests/hidden/` exists | **Verified** | `verify.go`: `HiddenAvailable` via `os.Stat(.../tests/hidden)`; hidden only runs if true |
| 7 | `tb review` FP on literal substring `tests/hidden` | **Verified** | `suggestsViolation` in `internal/review/review.go` does `strings.Contains` on rule+diff |
| 8 | `tb guard check --strict --json` can exit 0 with a violation | **Verified** | `cmd/tb/guard.go`: with `jsonOutput` returns **before** the `strict`/`os.Exit(1)` block |
| 9 | Graph tools in MCP/CLI require `TB_GRAPH=on` / `--enable` | **Verified** | `cmd/tb/graph.go`, `internal/graph/graph.go` `EnvEnable = "TB_GRAPH"`; ADR-011 |
| 10 | Python/Rust almost no `calls` edges | **Verified** | `regexLang.IndexFile` in `index_common.go` only emits `defines`/`imports`; `EdgeCalls` only in `index_go.go` / `index_javascript.go` |
| 11 | `init`/`mem` neither read nor generate `AGENTS.md`/`CLAUDE.md` | **Verified** | no match in Go code; `EnsureDefaultAntiCheat` only writes 3 fixed invariants |
| 12 | Ticket Markdown parser: `## Scope` + bullets fragile; `**` globs partial | **Verified** | `parseTicketMarkdown` only matches `scope:`/`escopo:` on the same line; `inScope` handles `/**` suffix and `filepath.Match`, not Markdown headings nor general `src/**/*.rs` |
| 13 | Config files created as `0600` | **Verified** | `config.Write` / saves use `0o600` |
| 14 | Visual ruler: `skipped=false` at top with skipped screens | **Partial** | `skipVisual` (missing manifesto/browser) sets `Skipped=true`; if the manifesto exists and only references are missing, screens get `skipped=true` but the report **does not** propagate `report.Skipped=true` |

Field checklist claims that the code **does not** support as bugs:

| Claim | Verdict | Note |
|-------|---------|------|
| Drift/Scope only accept literal paths (no globs) | **Refuted in code** (and in most reports) | `inScope` implements simple globs |
| `--fake` requires `--fake-config` | **Refuted** | happy path works without the flag |
| Graph always on / no flag | **Refuted by design** | ADR-011: on demand |

---

## (b) Frequency matrix (issues × repositories)

Counts from the “known problems” tables of the 20 reports, normalized. Format: **confirmed / refuted / partial / not tested**.

### Shared checklist issues

| Issue | With AI (n=10) | Without AI (n=10) | Total confirmed | Code |
|-------|----------------|-------------------|-----------------|------|
| `bench compare` without `--agent` | 10/0/0/0 | 10/0/0/0 | 20 | confirmed (CLI) |
| `guard.yaml` ignored | 10/0/0/0 | 10/0/0/0 | 20 | **verified** |
| Outdated CLI docs | 10/0/0/0 | 10/0/0/0 | 20 | docs vs cobra |
| `version` → `unknown` without ldflags | 10/0/0/0 | 10/0/0/0 | 20 | expected without `-ldflags` |
| Config files mode `0600` | 10/0/0/0 | 10/0/0/0 | 20 | **verified** |
| Visual `skipped=false` when skipping | 8/2/0/0 | 5/5/0/0 | 13 | **partial** (references scenario) |
| `default_agent` ignored | 10/0/0/0 | 10/0/0/0 | 20 | **verified** |
| Guard aborts verify on hardcoded path | 10/0/0/0 | 10/0/0/0 | 20 | **verified** (anti-cheat) |
| Fake needs `--fake-config` | 2/8/0/0 | 1/8/1/0 | 3 | **refuted** in practice |
| `mem add` positional only | 10/0/0/0 | 10/0/0/0 | 20 | outdated docs |
| Drift requires literal paths | 5/5/0/0 | 0/10/0/0 | 5 | **refuted** in code; confusion with `## Scope` |
| `graph impact` requires `--file` | 10/0/0/0 | 10/0/0/0 | 20 | UX, not a bug |
| Review FP on `tests/hidden` | 8/1/0/1 | 8/2/0/0 | 16 | **verified** |
| Verify partial checks / misses build | 10/0/0/0 | 10/0/0/0 | 20 | **verified** |
| tb does not push completing the task | 9/0/0/1* | 8/0/0/2 | ~17 | qualitative; aligned with harness gap |

\*values rounded from the tables; some reports marked N/A.

### New issues (emerged in the field)

| Issue | With AI | Without AI | Field notes | Code |
|-------|---------|------------|-------------|------|
| `SkipDirs` + root `target` → 0 nodes | ~4 confirmed (where clone was `/tmp/target`) | ~5 | Reproducible; indexing a subdir works | **verified** |
| `hidden_test_cmd` gated by dir | ~9 | ~9 | Almost universal | **verified** |
| Init Go default in non-Go repo | ~9 | 10 | More painful in **without** AI group | **verified** |
| `## Scope` / bullets parser | ~2–4 | ~1–3 | Strong in Go (cobra/crush) | **verified** |
| `guard --strict --json` exit 0 | few exercised | few | Clear bug when tested | **verified** |
| Few `calls` edges (Py/Rust) | Py/Rust in with group | Py/Rust/C in without group | empty callers/impact | **verified** |
| Does not read/generate `AGENTS.md` | 10 (file exists and tb ignores) | 10 (does not generate) | **With** group loses human skills | **verified** |
| Graph off without `TB_GRAPH`/`--enable` | common | common | Aligned with ADR-011 | **verified** (desired) |

**With vs without** instrumentation difference: product bugs are almost identical. The **without** group suffers more on onboarding (`go test` default + absence of `AGENTS.md`). The **with** group shows that tb **does not leverage** already-present instrumentation (skills, documented lint commands).

---

## (c) What tb caught vs what it let slip

### Caught (actually helped)

- **Anti-cheat guard on hardcoded paths**: touch `tests/hidden/` or `.toolbelt/thresholds.yaml` → verify FAIL early (several repos).
- **Drift / scope review** when the ticket uses `scope:` on the same line or YAML frontmatter: flagged `rd.md` and files outside the mandate.
- **Mem invariants**: anti-cheat baseline + retention `mem check`.
- **Graph search** in Go/JS when the index was not empty: located symbols (cobra `SetErrPrefix`, express `subdomains`, etc.).
- **Verify** as a runner for the manually configured `test_cmd` (mocha/pytest/go test/cargo subset).

### Let slip / hindered

- **Verify PASS without lint/build/typecheck** (commander, express, bat with `test_cmd=true`, etc.).
- **Hidden suite configured but never executed** without a `tests/hidden/` folder.
- **Mute graph** on `/tmp/target` clones and **empty callers/impact** in Python/Rust/C/shell.
- **Review FP** when the ticket itself mentions `tests/hidden`.
- **Useless custom guard.yaml**; loose exit code with `--json`.
- **Go-centric init** in 18+ non-Go or mixed repos; does not read `AGENTS.md`/`CLAUDE.md`/`package.json`/`Makefile`.
- **Does not push completeness** (k6 stayed partial; jq needed a 2nd attempt without a harness nudge).

### Aggregated pattern

`tb` today is a **good set of point rulers** (hardcoded guard, literal drift, one-command verify) and a **bad bootstrapper / bad consumer of human context**. In repos with instrumentation, the gap is *integration*; without, it is *discovery*.

---

## (d) Comparison with `docs/pesquisa`

### Field findings **supported** by research sources/ideas

| Field finding | Source / idea | Link |
|---------------|---------------|------|
| Fragile / easy-to-bypass hidden suite (`hidden` gated) | SpecBench, ImpossibleBench, finding #2 | Held-out only counts if it actually runs and stays invisible to the generator |
| Review/guard with FPs and fragile patterns | TRACE, ImpossibleBench, ADR-005/008 | Needs better signals than substring |
| Init without progress/AGENTS/features | Anthropic *Effective harnesses*; `AGENTS.md` spec; table findings | Incomplete bootstrap = agent without rails |
| Graph sometimes useless/expensive; off by default OK | *How Should Agents See Code?*, Cost-Inefficient Behaviors, ADR-011 | Field confirms: search helps; callers/impact weak; forcing graph does not |
| Human skills > automatic search | Cost-Inefficient Behaviors (−41% tokens) | Repos **with** AI had skills/`AGENTS.md` that tb ignored |
| Invariants outside compaction already exist (`tb mem`) | Compact-Bench, arXiv 2610.31778, ADR-010 | Field used `mem`; still need to wire to review without FPs |
| Scope declared in the ticket | Claim Plane, CAID, finding #5 | Drift helped when the parser understood the scope |
| Judges need an external signal | Huang et al.; MT-Bench | Verify without lint = false PASS; LLM judges alone are not enough |

### Research ideas the field shows as **urgent**

1. **Long-horizon harness / `tb init` progress+features style** — onboarding failed in almost all repos without (and also with) instrumentation.
2. **Real held-out in verify** — gating by `tests/hidden/` nullifies the reward-hacking KPI.
3. **Consume `AGENTS.md` / human skills** — field evidence + cost paper.
4. **Monitor beyond the diff** — guard only looks at paths/Go AST; `__eq__` patterns etc. barely showed up in the multi-language scanner.
5. **Robust mandate/scope parser** — without it, Claim Plane on paper does not become enforcement.

### Research ideas **without support** (or without evidence) in this field study

| Idea | Why the field does not support it (yet) |
|------|-----------------------------------------|
| Router / A2A / parallel workers / waste watchdog | No task used multi-agent; no evidence of scope collision in this study |
| ACP (Zed) / TUI UI `tb md` | Outside the 20-agent protocol |
| Design2Code / CLIP visual metrics | Visual ruler almost always skipped; no useful signal |
| Vector index / deep Graphify integration | Regex graph is already the bottleneck; vector was not the observed failure |
| Network sandbox proxy / Landlock | Little exercised in unit bugfix tasks |
| `tb bench --impossible` | Bench only ran `--fake` on toolbelt; did not measure cheat rate on the target |
| Test mutation (ACH) | No repo used mutants |
| Hybrid compaction in production | `mem check` exists, but there were no long sessions with LLM compaction |

---

## (e) Elected improvements (P0→P3) and proposed PRs

### Ranking

#### P0-1 — Do not skip the walk root when the basename ∈ `SkipDirs`

- **Problem**: `target`/`build`/`dist` as the root folder name zeros the graph with no error.
- **Evidence**: confirmed on `/tmp/target` clones — [click](click.md), [commander-js](commander-js.md), [express](express.md), [ky](ky.md), [mcp-python-sdk](mcp-python-sdk.md), [requests](requests.md), [sentry-python](sentry-python.md), [typer](typer.md), [k6](k6.md) (≈9).
- **Source**: ADR-011 (useful on-demand graph); code-map papers.
- **Change**: in `walkSourceFile`, do not apply `SkipDir` when `path == root`.
- **Package**: `internal/graph` (+ test).
- **Acceptance**: `BuildFromRoot("/tmp/target")` indexes children; child `node_modules`/`target` still skipped; regression test.
- **Size**: S

#### P0-2 — Run `hidden_test_cmd` when configured (without requiring the dir)

- **Problem**: hidden command in YAML/`--hidden-test-cmd` is ignored if `tests/hidden/` does not exist → false PASS.
- **Evidence**: ~18 repos; explicit in [commander-js](commander-js.md), [express](express.md), [excalidraw](excalidraw.md), [bat](bat.md), etc.
- **Source**: SpecBench / ImpossibleBench / finding #2.
- **Change**: `HiddenAvailable = cfg.HiddenTest != ""` **or** dir exists; if cmd is set and fails to run, reflect in the verdict; document.
- **Package**: `internal/verify`, `cmd/tb/verify`.
- **Acceptance**: without dir + cmd=`false` → not PASS; with empty dir + default go hidden → documented behavior.
- **Size**: S

#### P0-3 — Exit code of `guard check --strict` with `--json`

- **Problem**: JSON early-return prevents `os.Exit(1)` → green CI with a violation.
- **Evidence**: [jq](jq.md), [excalidraw](excalidraw.md), [commander-js](commander-js.md) (when exercised).
- **Source**: ADR-005; AI Control / monitors.
- **Change**: apply `strict` **after** JSON encode (or return `error`/`ExitError`).
- **Package**: `cmd/tb/guard.go`.
- **Acceptance**: `--strict --json` with `SeverityError` → exit ≠ 0; stdout remains valid JSON.
- **Size**: XS

#### P0-4 — Honor `guard.yaml` (configurable protected paths)

- **Problem**: only hardcoded; custom ignored in 20/20.
- **Evidence**: 20 confirmed.
- **Source**: ADR-005; ImpossibleBench (protect scoring).
- **Change**: load `.toolbelt/guard.yaml` (`protected_paths`) and merge with defaults.
- **Package**: `internal/guard`, docs.
- **Acceptance**: protecting `lib/help.js` / `src/click/core.py` via YAML yields `protected_path`.
- **Size**: S

#### P1-1 — `tb init` detects toolchain (do not assume Go)

- **Problem**: `go test ./...` in Python/JS/Rust/C.
- **Evidence**: ~19 repos; worse in without-AI group.
- **Source**: *Effective harnesses*; AGENTS.md.
- **Change**: detect `go.mod` / `package.json` / `pyproject.toml` / `Cargo.toml` / `Makefile` and fill `verify.test_cmd` (and suggest hidden/lint).
- **Package**: `internal/config`, `cmd/tb` init.
- **Acceptance**: init on npm fixture → `npm test`; python → `pytest`; go unchanged.
- **Size**: M

#### P1-2 — Review: end FP on substring `tests/hidden`

- **Problem**: ticket/diff that only *mentions* the rule triggers a finding.
- **Evidence**: 16 confirmed ([jq](jq.md), [bat](bat.md), [crush](crush.md), …); some refuted when the diff did not cite the string.
- **Source**: ADR-010; Compact-Bench (rules matter, but clean signal).
- **Change**: match changed path via `inScope`/`hitForbidden`, not `Contains` on the diff text.
- **Package**: `internal/review`.
- **Acceptance**: `rd.md` with `forbidden: tests/hidden/**` without touching the dir → no critical finding; touching a file → finding.
- **Size**: S

#### P1-3 — Use `bench.default_agent` when `--agent` is omitted

- **Problem**: dead config field; 20/20.
- **Evidence**: unanimous checklist.
- **Source**: DX / harness.
- **Change**: `parseBenchRunFlags` falls back to `cfg.Bench.DefaultAgent`.
- **Package**: `cmd/tb/bench.go`.
- **Acceptance**: YAML with `default_agent: echo hi` + `tb bench run -r 1` does not require the flag.
- **Size**: XS

#### P1-4 — Scope parser: headings + Markdown lists

- **Problem**: `## Scope` + `- path` fails or only gets the 1st item.
- **Evidence**: [cobra](cobra.md), [crush](crush.md), “literal” drift reports misinterpreted.
- **Source**: Claim Plane / CAID (mandate with files).
- **Change**: parse block after `## Scope`/`## Escopo`; accept bullets; keep current globs.
- **Package**: `internal/review` (`parseTicketMarkdown`).
- **Acceptance**: fixture `rd.md` with heading+bullets → complete `Scope`; `src/**` and `src/*.go` regression OK.
- **Size**: S

#### P1-5 — Verify: optional build/lint pipeline (or reject inert YAML)

- **Problem**: `lint_cmd`/`build_cmd` in YAML are silent; PASS without compiling.
- **Evidence**: 20 confirmed “partial checks”.
- **Source**: judges with external signal; SWE-agent (lint on edit).
- **Change**: either (A) honor `verify.build_cmd`/`lint_cmd` if present, or (B) error/warn on unknown keys; document.
- **Package**: `internal/config`, `internal/verify`.
- **Acceptance**: repo with failing lint does not PASS if lint is configured.
- **Size**: M

#### P2-1 — Read/`seed` `AGENTS.md` in init/mem/ctx

- **Problem**: with-AI group: files ignored; without group: nothing generated.
- **Evidence**: 19+; crush/excalidraw/ky explicitly.
- **Source**: agents.md; Anthropic skills; Cost-Inefficient Behaviors.
- **Change**: if present, index/warn on init; if not, `--write-agents` option for a minimal stub pointing to test/lint/tb.
- **Package**: `cmd/tb` init, `internal/mem` (light).
- **Acceptance**: init on repo with `AGENTS.md` records path; flag generates skeleton without overwriting.
- **Size**: M

#### P2-2 — Minimal `calls` edges for Python (then Rust)

- **Problem**: empty `graph callers`/`impact` where the task was Py/Rust.
- **Evidence**: click, requests, mcp-python-sdk, pydantic-ai, bat, ripgrep, ruff.
- **Source**: LocAgent / RepoGraph; ADR-011 (useful when locating is the bottleneck).
- **Change**: conservative regex extractor of `name(` calls resolving against indexed funcs in the same module; precision metric on fixture.
- **Package**: `internal/graph`.
- **Acceptance**: Python fixture with A→B yields ≥1 `EdgeCalls`; callers not empty.
- **Size**: M

#### P2-3 — Visual report: `skipped=true` if all screens were skipped

- **Problem**: top-level lies; 13 confirmed / 7 refuted (different scenarios).
- **Evidence**: bat, crush, excalidraw vs click/typer (only missing manifesto).
- **Source**: Design2Code (honest metrics).
- **Change**: after `runVisualScreens`, if len(screens)>0 and all skipped → `report.Skipped=true`.
- **Package**: `internal/verify/visual.go`.
- **Acceptance**: manifesto without references → JSON `skipped: true`.
- **Size**: XS

#### P3-1 — Sync `docs/cli.md` with the real CLI

- **Problem**: `mem add --type`, `tb ticket`, `tb serve`, guard snapshot, etc.
- **Evidence**: 20/20.
- **Source**: Writing effective tools (stable names).
- **Change**: docs-only aligning existing commands; mark Phase 3 as planned.
- **Package**: `docs/cli.md` (+ maybe README status).
- **Acceptance**: no documented flag that the CLI rejects in “implemented” examples.
- **Size**: S

### Separate PRs (recommended merge order)

Order avoids conflicts (graph → verify/guard → review/config → docs) and delivers value early.

| # | PR title | One line | Priority | Depends on |
|---|----------|----------|----------|------------|
| 1 | `fix(graph): do not apply SkipDirs on walk root` | Fixes empty graph in dirs named `target`/`build`/`dist`. | P0 | — |
| 2 | `fix(verify): run hidden_test_cmd without requiring tests/hidden/` | Configured held-out actually executes. | P0 | — |
| 3 | `fix(guard): exit ≠ 0 with --strict --json` | CI stops swallowing violations in JSON. | P0 | — |
| 4 | `feat(guard): load protected_paths from guard.yaml` | Custom paths take effect. | P0 | 3 (same cmd package; merge after exit code) |
| 5 | `feat(init): detect toolchain and fill verify.test_cmd` | Stops assuming Go on npm/cargo/pytest. | P1 | — |
| 6 | `fix(review): tests/hidden invariant FP by path, not substring` | Ticket that cites the rule stops being a finding. | P1 | — |
| 7 | `fix(bench): honor bench.default_agent` | Config stops being a dead field. | P1 | — |
| 8 | `fix(review): parse ## Scope and Markdown lists` | Human Markdown mandate works in drift. | P1 | 6 (same parser) |
| 9 | `feat(verify): honor optional build_cmd/lint_cmd` | False PASS without lint/build becomes fixable opt-in. | P1 | 2 |
| 10 | `feat(init): seed/read AGENTS.md` | Connects tb to the repo’s human skills. | P2 | 5 |
| 11 | `feat(graph): basic calls edges for Python` | Useful callers/impact outside Go/JS. | P2 | 1 |
| 12 | `fix(verify): aggregated visual skipped + aligned docs(cli)` | Visual ruler honesty + docs; can be 1 docs+visual PR or 2 micros. | P2/P3 | — |

Maximum 12 PRs; item 12 can split into `fix(verify): visual skipped` + `docs(cli): align with CLI` if review prefers even smaller diffs.

---

## (f) Explicitly **not** recommended (now)

| Item | Why |
|------|-----|
| Remove mode `0600` from configs | Confirmed 20×, but it is secret hygiene; not a product bug. |
| Turn `TB_GRAPH=on` by default | ADR-011 + cost papers; field shows utility **on demand**, not obligation. |
| Require `--fake-config` on the fake agent | Claim **refuted** in most repos. |
| Restrict drift to literal paths | Simple globs **already work**; the real hole is the heading parser. |
| Implement router/A2A/workers in this cycle | Strong research, **zero** field evidence in this single-agent bugfix study. |
| Test mutation / impossible bench / network proxy | No field exercise; keep on Phase 3+ roadmap, not the immediate queue. |
| Rewrite graph with full tree-sitter now | High cost; incremental gain with Python regex `calls` already attacks the observed pain. |
| Treat “tb does not push completing the task” with prompt only | Needs artifacts (progress/features/verify gate), not more text. |

---

## Next step

Use this synthesis as **input for the next implementation phase**, starting with the P0 PRs in table order. Raw reports remain in `docs/campo/<slug>.md` for evidence tracing.

**Round 2** (field revalidation of the 12 fixes): synthesis in [`docs/campo/rodada2/SINTESE.md`](rodada2/SINTESE.md); reports in [`docs/campo/rodada2/<slug>.md`](rodada2/).
