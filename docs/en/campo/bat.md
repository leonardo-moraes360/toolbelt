[🇧🇷 Português](../../campo/bat.md) | 🇺🇸 English

# Field study: sharkdp/bat

Group: **without AI agent instrumentation**. Local clone at `/tmp/target/bat`. No interaction with upstream (no push/issue/PR/comment). `tb` built from this repo (`make build` → `toolbelt c8aae04`).

## Repository

| Field | Value |
|-------|-------|
| Name | [sharkdp/bat](https://github.com/sharkdp/bat) |
| Language | Rust (edition 2021, declared MSRV `1.88`) |
| Size | shallow clone ~15–17 MiB code; ~77 `.rs` files / ~21 515 LOC; local `target/` ~1.9 GiB after build |
| HEAD studied | `599c383` (merge #3837) + local fix commit `af23f13` (VM only) |
| Existing AI instrumentation | **None** (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot instructions: absent) |
| Toolchain | Rustup `stable` 1.100.0 (initially 1.83.0 — insufficient for MSRV); `cargo build --bins`, `cargo test` (integration), `cargo fmt`, `cargo clippy --bins` OK |
| Not configured | Submodules under `assets/syntaxes/*` and themes **not** initialized (checkout already brings precompiled assets `acknowledgements.bin` etc.; regeneration via `assets/create.sh` was neither needed nor run). Full syntax-tests / snapshots suite was not run due to time and asset dependencies. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Validate `--language` / `--map-syntax` even with highlighting off (`--color=never` / pipe), failing with `unknown syntax` and exit ≠ 0 |
| Why | Real bug, small-medium scope, maintainer-acceptable, with a clear integration test path |
| Issue | https://github.com/sharkdp/bat/issues/4120 |
| Completed? | **Yes** (locally): `src/controller.rs` + mirrored validation in `src/printer.rs`, 2 tests in `tests/integration_tests.rs`, entry in `CHANGELOG.md` |
| Native checks | `cargo build --bins` OK; `cargo fmt -- --check` OK; `cargo clippy --bins -- -D warnings` OK; tests `unknown_language_errors_with_color_never` and `unknown_map_syntax_errors_with_color_never` OK; regression `no_first_line_fallback_when_mapping_to_invalid_syntax` OK. Full integration suite (~290 tests) was **not** fully executed. |

Root cause: with `loop_through` (color never / pipe), `Controller` uses `SimplePrinter` and **skips** `get_syntax`, so invalid names are never rejected.

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb version` | OK (`c8aae04` with `make build`) | neutral | 0.006 s; without ldflags: `commit: unknown` / `built: unknown` |
| `tb init` | OK; creates `.toolbelt/` | hindered (partial) | `verify.test_cmd: go test ./...` in a Rust repo; no `AGENTS.md`; empty `vault/` and `bench/` dirs; `config.yaml` mode **600** |
| `tb mem init/list/add/check` | OK | helped (partial) | Go-centric default invariants (`tests/hidden/`, `thresholds.yaml`); `mem add` only accepts **positional** (`--rule` → unknown flag); files 600 |
| `tb graph rebuild/stats/search` | OK with `--enable` / `TB_GRAPH=on` | helped | 2971 nodes / 4323 edges in 0.26 s; `search get_syntax` found `src/assets.rs` |
| `tb graph impact` | requires `--file`; with file → empty output | hindered | without `--file`: exit 1; `--file src/printer.rs`: `(vazio)` — useless impact for guiding the change |
| `tb graph callers/deps` | OK only with correct flags | neutral | without `--function`/`--file` fails; callers of `get_syntax` empty; deps of `src/printer.rs` listed imports |
| ticket + Scope (`rd.md`) | manual (no `tb ticket`) | helped | frontmatter `base_commit`/`scope`/`forbidden`/`acceptance` consumed by drift/review |
| `tb drift` | OK / detects out of scope | helped | flagged `rd.md` outside scope; simple globs `src/*.rs` and `tests/**` work; `src/**/*.rs` **does not** |
| `tb review` | OK / findings | mixed | drift+acceptance; FP on `tests/hidden` invariant when the diff mentions the string; completeness warnings even with tests in the diff |
| `tb verify run` (tests) | PASS if `test_cmd` valid | mixed | ignores YAML `build_cmd`/`lint_cmd`; `hidden_test_cmd` only runs if `tests/hidden/` dir exists; with `test_cmd=true` PASS on a commit that **does not compile** |
| `tb verify --ruler visual` | PASS with skip | hindered | missing manifesto: `skipped=true`; with screens without reference: top-level **`skipped=false`** although screen `skipped=true` |
| `tb guard check` | OK | mixed | does not read `.toolbelt/guard.yaml`; hardcoded paths (`thresholds.yaml` etc.); aborts `verify` when touching protected path |
| `tb bench run --fake` | OK | neutral | 9 tasks, 33.3% success, ~4 s; YAML `default_agent` **ignored** (requires `--agent`/`--fake`) |
| `tb bench compare` | OK with `--fake` A/B | neutral | **no** `--agent` (`unknown flag`); A/B graph-off/on on mcp-smoke OK |
| `tb mcp` | OK (stdio JSON-RPC) | helped | `tools/list` exposed version/echo/env/run/graph_*; stderr: `servidor MCP iniciado` |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare --agent x …` → `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` listing `CHANGELOG.md`/`src/controller.rs`; `guard check` → `violations: null` |
| outdated CLI docs | **confirmed** | `docs/cli.md` documents `tb run`, `tb proc`, `tb md`, `tb ticket`; real `tb --help` does not list them |
| version shows `unknown` | **confirmed** (partial) | `go build` without ldflags: `commit: unknown`, `built: unknown` (version `0.1.0-dev`); with `make build` shows hash |
| config files created 0600 | **confirmed** | `stat`: `.toolbelt/config.yaml` and `invariants.yaml` and `graph.json` → mode **600** |
| visual ruler reports `skipped=false` when skipping | **confirmed** | with `screens.yaml` and missing ref: JSON top-level `"skipped": false` while screen has `"skipped": true` |
| `default_agent` ignored | **confirmed** | YAML `default_agent: echo-agent {prompt}`; `tb bench run` without flags → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | commit touching `.toolbelt/thresholds.yaml` → verify `FAIL` iterations 0, violation `protected_path` |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake … --task mcp-smoke` exit 0 **without** `--fake-config`; invalid path in `--fake-config` was also ignored successfully |
| `mem add` positional | **confirmed** | Usage: `tb mem add <regra>`; `--rule` → `unknown flag` |
| Drift/Scope need literal paths (no globs) | **partially refuted** | `src/*.rs`, `src/print*.rs`, `tests/**`, `CHANGELOG.*` OK; `src/**/*.rs` and `**/*.rs` **fail** (all `.rs` out of scope) |
| `graph impact` needs `--file` | **confirmed** | without flag: `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | **confirmed** | with `rd.md` in the diff containing `tests/hidden/`: critical finding on the invariant; without that string in the diff: FP disappears |
| verify runs only partial checks and misses build errors | **confirmed** | YAML `build_cmd`/`lint_cmd` not executed; commit with invalid Rust + `--test-cmd true` → **PASS**; native `cargo build` exit 101 |
| tb does not push the agent toward completing the whole task | **confirmed** (qualitative) | onboarding did not generate checklist/progress/`AGENTS.md` nor detect `cargo test`/`clippy`; `review` only warned completeness by substring in the diff; CHANGELOG/tests came from human CONTRIBUTING, not from tb |

## New problems

| sev | problem | repro | expected | actual |
|-----|---------|-------|----------|--------|
| P1 | `tb init` assumes Go in a Rust repo | `tb init` on bat | detect `Cargo.toml` → `cargo test` / `cargo clippy` | `verify.test_cmd: go test ./...` |
| P1 | CLI/config `hidden_test_cmd` ignored without `tests/hidden/` dir | `tb verify run --hidden-test-cmd 'cargo test … basic'` | run hidden command | `hidden_available: false`; never executes the cmd |
| P2 | empty `graph impact` in Rust | `tb graph impact --enable --file src/printer.rs` | callers/dependents | `(vazio)` |
| P2 | cobra flags UX without hint in short deps/callers error | `tb graph callers get_syntax` | accept positional or suggest `--function` | generic exit 1 |
| P2 | review completeness is fragile (substring) | acceptance criteria with `--map-syntax` / “unchanged” | evidence via tests/changelog | warnings even with tests added |
| P3 | schema `bench.default_agent` has no effect | set in YAML and omit CLI | use default | error requiring `--agent`/`--fake` |
| P3 | MCP `tools/list` includes graph tools even with graph off (docs say on demand) | `tb mcp` without `TB_GRAPH` | omit or annotate disabled | graph tools listed; search may fail later |

## What tb caught and what it let slip

**Caught**
- Drift when `rd.md` entered the commit outside Scope.
- Guard on hardcoded path `.toolbelt/thresholds.yaml` (and aborted verify).
- Review: warnings about acceptance criteria poorly evidenced in the diff text.
- Graph search located `get_syntax` / related files.

**Let slip**
- Rust build error with `verify` configured only with `test_cmd` (or `true`).
- Declared `build_cmd` / `lint_cmd` in config.
- User “protected” path via `guard.yaml`.
- Real impact of changing `printer.rs`/`controller.rs` (empty impact).
- Did not guide CHANGELOG (bat CONTRIBUTING requirement) nor full suite.

**Native vs tb:** native (`build`/`fmt`/`clippy`/focused tests) validated the fix; tb verify only echoed the `test_cmd` I configured manually after fixing the Go default.

## Interaction with existing instrumentation / onboarding

There was no prior AI instrumentation. `tb init` + `tb mem init` from scratch generated:

- `.toolbelt/config.yaml` (sandbox/graph/bench/verify defaults **Go**)
- `.toolbelt/invariants.yaml` (generic anti-cheat)
- empty dirs `vault/`, `bench/`, `logs/`
- later: `graph.json` (via rebuild), plus `guard.yaml`/`screens.yaml` created manually in the study

**Absent and missed:** repo `AGENTS.md` / skills; automatic detection of `cargo test`/`clippy`/`fmt`; progress log / feature list in harness style; ticket CLI (`tb ticket`); seed of local `bench/tasks`; short “how to operate in this repo” documentation.

Bootstrap quality: **low for Rust without AI files** — the agent needs to overwrite almost all of verify and ignore irrelevant invariants.

## Improvement suggestions

1. **[P0 · init/verify]** Detect toolchain (`Cargo.toml`, `go.mod`, `package.json`) and fill `test_cmd`/`build_cmd`/`lint_cmd`; fail loudly if Go default in a non-Go repo.  
2. **[P0 · verify]** Run `build_cmd` and `lint_cmd` in the loop (or at least in the final verdict); do not PASS on `test_cmd` alone.  
3. **[P1 · verify]** Honor `--hidden-test-cmd` / config even without a `tests/hidden/` directory.  
4. **[P1 · guard]** Load `.toolbelt/guard.yaml` (or document that it is dead and remove it).  
5. **[P1 · bench]** Use `bench.default_agent` when `--agent` is omitted; add `--agent` to `bench compare` A/B.  
6. **[P1 · onboarding]** Generate minimal `AGENTS.md` + progress/checklist from the ticket; push acceptance (tests + changelog if the repo requires it).  
7. **[P2 · review]** Match invariants by path/glob, not substring in the diff (avoids FP with `tests/hidden` in the ticket text).  
8. **[P2 · visual]** If any screen is skipped, `report.skipped=true` (or `partial_skip`).  
9. **[P2 · graph]** Useful impact/callers in Rust; accept positionals or actionable errors.  
10. **[P3 · config]** 0644 permissions for non-secret YAML; align `docs/cli.md` with real Cobra; consistent ldflags/`version` in the README.

## Metrics

| metric | value |
|--------|-------|
| Approximate study wall time | ~25–35 min (Rust upgrade + bat build + tb flow + focused tests); instrumented command sum ~22 s |
| Recorded `tb-*` invocations | **52** |
| `tb-*` failures (exit ≠ 0) | **13** (several intentional: missing flags, drift/review, guard abort, compare `--agent`) |
| Sum of `tb-*` times | **~7.0 s** |
| Initial bat build | ~30.6 s |
| `tb graph rebuild` | 0.26 s (2971 nodes) |
| `tb bench run --fake` (9 tasks) | ~4.0 s |
| MCP `tools/list` | ~0.006 s |

Local study artifacts (VM only): `/tmp/campo-bat/metrics.tsv`, verify/review/drift JSONs, clone `/tmp/target/bat` (not pushed).
