🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/bat.md)

# Field study round 2: sharkdp/bat

Group: **without existing AI-agent instrumentation**. Local clone at `/tmp/target/bat`. No interaction with upstream. `tb` built from this repo on `main` (`make build` → `toolbelt a46c295`, with the 12 fixes merged).

## Repository

| Field | Value |
|-------|-------|
| Name | [sharkdp/bat](https://github.com/sharkdp/bat) |
| Language | Rust (edition 2021, MSRV `1.88`) |
| Size | shallow clone ~17 MiB code; ~67 `.rs` files / ~19 632 LOC; local `target/` after build |
| Studied HEAD | `fbd9a41` (upstream) + local commit `6a2710c` (VM only) |
| Existing AI instrumentation | **None** before the study (`AGENTS.md`/`CLAUDE.md`/`.cursorrules` absent). `tb init --write-agents` seeded a minimal `AGENTS.md` + `.toolbelt/agent_docs.yaml` |
| Toolchain | Rustup `stable` 1.100.0 (upgraded from 1.83.0 — insufficient for MSRV); `cargo build --bins`, `cargo test --test integration_tests …`, `cargo fmt`, `cargo clippy --bins` OK |
| Not configured | `assets/syntaxes/*` submodules / themes not initialized (precompiled assets were enough). Full integration suite (~290 tests) and syntax-tests/snapshots not run for time |

## Chosen task

| Field | Value |
|-------|-------|
| What | Avoid `capacity overflow` panic in `--line-range` with a huge offset-from-end (`:-N` / `-N:` with N ≈ `usize::MAX`) |
| Why | Real open bug, small–medium scope, clear integration-test path; **different from round 1** |
| Issue | https://github.com/sharkdp/bat/issues/3845 (also #4039) |
| Completed? | **Yes** (locally): `src/controller.rs` (`saturating_add` + `VecDeque::new`/`try_reserve` instead of `with_capacity`), 2 tests in `tests/integration_tests.rs`, entry in `CHANGELOG.md` |
| Native checks | `cargo build --bins` OK; `cargo fmt -- --check` OK; `cargo clippy --bins -- -D warnings` OK; tests `line_range_huge_offset_from_end_does_not_panic` and `line_range_huge_offset_from_start_of_end_does_not_panic` OK |
| Round 1 (comparison) | Validate `--language`/`--map-syntax` with color never (issue #4120) — distinct task |

Root cause: `print_file_ranges` did `largest_offset_from_end() + 1` and `VecDeque::with_capacity(buffer_size)`, which aborts with `capacity overflow` for huge offsets.

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Index a directory literally named `/tmp/campo-bat/target` → `3 nodes, 2 edges` (not 0). Under `/tmp/target/bat` rebuild indexed `2981` nodes |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | `ls tests/hidden` missing; `tb verify run` with `hidden_test_cmd: cargo test …` → `hidden_available: true`, `hidden_passed: true`, verdict PASS |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | With violations in `guard.yaml`: `--strict --json` → exit **1** + JSON with `protected_path` |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | YAML with `CHANGELOG.md` + `src/controller.rs` → violations on those paths in the fix diff |
| 5 | init toolchain detection | **worked** | `tb init` on bat → `verify.test_cmd: cargo test` (no longer `go test ./...`) |
| 6 | review `tests/hidden` by path | **worked** | Ticket mentioning `tests/hidden/` only in prose, without touching the path → findings without invariant FP; only scope drift |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Without flags: uses `echo AGENT_OK {prompt}` (`agent: external`, 100 % on mcp-smoke). Without default → error asking for `--agent`/`--fake`/`default_agent`. `compare --agent` accepted and ran A/B |
| 8 | review `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | Ticket with `## Escopo` + bullets + `` `tests/**/*.rs` `` → `drift`/`review` without out-of-scope on fix files; `src/**/*.rs` also matched `controller.rs` |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | PASS with both `*_ran: true`. `build_cmd: false` → ESCALATE `build_passed: false` without running lint. `lint_cmd: false` → ESCALATE `lint_passed: false` |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** | Without files: `--write-agents` seeded `AGENTS.md` and wrote `.toolbelt/agent_docs.yaml` (`sources: AGENTS.md`). Flag documented in `tb init --help` |
| 11 | graph Python `calls` edges | **not applicable** | Rust repo; no `.py`. (Note: Rust index still leaves useful `graph callers`/`impact` empty) |
| 12 | aggregated visual skipped + CLI docs aligned | **worked** | Missing manifest: top-level `skipped: true`. Screens with missing refs: `skipped: true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` mentions `build_cmd`, `## Scope`, `guard.yaml`, `default_agent`, visual skip |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` | OK (`a46c295`) | neutral | 0.006 s |
| `tb init` / `--write-agents` | OK; `cargo test` + `AGENTS.md` | helped | Rust toolchain detected; minimal onboarding created; still does not fill `build_cmd`/`lint_cmd` |
| `tb mem init/list/add/check` | OK | helped (partial) | Default invariants still Go-centric (`tests/hidden/`); positional `mem add` OK |
| `tb graph rebuild/stats/search` | OK with `--enable` | helped | 2981 nodes / 4554 edges / 0.37 s; search found `largest_offset_from_end` in `line_range.rs` |
| `tb graph impact/callers` | empty output on Rust | hindered | `--file src/controller.rs` and `--function largest_offset_from_end` → `(vazio)` |
| `tb graph deps` | OK | neutral | Listed imports of `controller.rs` |
| ticket `rd.md` (`## Escopo`) | manual | helped | Markdown parser + `**` globs consumed by drift/review |
| `tb drift` | OK | helped | Fix scope clean; globs `src/**/*.rs` / `tests/**/*.rs` OK (improvement vs round 1) |
| `tb review` | OK / 1 completeness warning | mixed | No `tests/hidden` FP; still warns on acceptance “Existing line-range tests still pass” despite tests in the diff |
| `tb verify run` | PASS with build+lint+test+hidden | helped | `hidden` without dir; effective `build_cmd`/`lint_cmd`; aborts if `guard.yaml` protects the fix’s own files |
| `tb verify --ruler visual` | PASS skipped | helped | Top-level aggregation `skipped: true` (fixed vs round 1) |
| `tb guard check` | OK + reads YAML | helped | Custom `protected_paths` + defaults; `--strict --json` exit 1 |
| `tb bench run` (`default_agent`) | OK with absolute `-t` | mixed | Honors `default_agent`; **ignores** YAML `bench.tasks_dir` (always CLI default `bench/tasks` without `-t`) |
| `tb bench compare --agent` / `--fake` | OK | helped | `--agent` flag present; A/B mcp-smoke 100 %/100 % |
| `tb mcp` | OK (stdio) | helped | `tools/list` with version/echo/env/run/graph_*; stderr `servidor MCP iniciado` |

## Problems that persist

From round 1 on bat, still observable:

- **Empty Rust `graph impact` / `callers`** — search finds the symbol; impact/callers do not guide the change.
- **Fragile review completeness** — acceptance warning even with tests added in the diff.
- **Go-centric default invariants** in `mem init` (`tests/hidden/`, `thresholds.yaml`) on a Rust repo.
- **Config files mode `0600`** — `.toolbelt/config.yaml`, generated `AGENTS.md`, etc.
- **Inert `bench.tasks_dir` from config** without CLI `-t` (related to empty local `bench/tasks` onboarding) — in practice the agent must pass `-t` pointing at external fixtures.
- **Verify aborts immediately** if `guard.yaml` protects paths the fix itself must touch (intentional guard behavior, but hinders the verify↔implement loop if the manifest is too broad).

## New problems

| sev | problem | repro | expected | actual |
|-----|---------|-------|----------|--------|
| P2 | YAML `bench.tasks_dir` / `fixtures_dir` in `.toolbelt/config.yaml` ignored | Set absolute paths in YAML; run `tb bench run -r 1 --task mcp-smoke` without `-t` | use config dirs | log shows `tasks_dir: bench/tasks` and fails `open bench/tasks/...` |
| P2 | `tb init` detects `cargo test` but does not suggest Rust `build_cmd`/`lint_cmd` | `tb init` on bat | fill `cargo build` / `cargo fmt` or `clippy` | only `test_cmd: cargo test`; `hidden_test_cmd: ""`; build/lint empty until manual edit |
| P3 | `graph callers` without useful Rust `calls` edges (search≠callers) | `tb graph callers --enable --function largest_offset_from_end` | callers from `controller.rs` | `(vazio)` despite 994 aggregated `edges_calls` in stats |
| P3 | Review completeness still via weak tokens | Acceptance “Existing line-range tests still pass” with tests in the diff | no warning or evidence via test names | `accept-1` warning |

No clear regression of the 12 fixes in this repo: items that failed in round 1 and were in scope of the fixes passed the tests above.

## What tb caught and what it let slip

**Caught**
- Drift/scope with `## Escopo` + `**` globs (fix files inside scope).
- Guard via `guard.yaml` (CHANGELOG/controller) and correct exit code with `--strict --json`.
- Verify failed correctly when `build_cmd` or `lint_cmd` broke; passed with focused build+fmt+tests+hidden without a `tests/hidden/` dir.
- Graph search located `largest_offset_from_end`.
- Aggregated visual skipped in the top-level report.

**Let slip**
- Real impact of changing `controller.rs` (empty impact/callers).
- Did not guide filling `build_cmd`/`lint_cmd` or CHANGELOG alone (still came from human CONTRIBUTING).
- Review completeness did not recognize new tests as acceptance evidence.

**Native vs tb:** native (`build`/`fmt`/`clippy`/focused tests) validated the fix; `tb verify` echoed the configured cmds (including hidden without a directory) and now also build/lint — clear improvement vs round 1.

## Interaction with existing instrumentation / onboarding

There was no prior AI instrumentation. With current tb:

- `tb init --write-agents` created a minimal (toolbelt) `AGENTS.md` + `agent_docs.yaml`.
- Detected `Cargo.toml` → `cargo test`.
- **Did not** generate `CLAUDE.md`, progress/checklist, local `bench/tasks` seed, or cargo `build_cmd`/`lint_cmd`.
- Bootstrap quality: **better than round 1** (no longer Go default; there is AGENTS.md), still **incomplete for Rust** (empty lint/build and local bench).

## Comparison with round 1

| area | round 1 | round 2 | verdict |
|------|---------|---------|---------|
| init toolchain | `go test ./...` | `cargo test` | **better** |
| AGENTS.md | absent | `--write-agents` seeds | **better** |
| verify build/lint | ignored | run and fail the verdict | **better** |
| hidden without dir | never ran | runs `hidden_test_cmd` | **better** |
| guard.yaml | ignored | read + strict/json OK | **better** |
| Scope `**` / `## Escopo` | fragile / `src/**/*.rs` failed | matched fix files | **better** |
| review FP `tests/hidden` | confirmed | did not reproduce | **better** |
| visual skipped top-level | `skipped: false` with skipped screens | `skipped: true` | **better** |
| bench default_agent / compare `--agent` | broken | works | **better** |
| Rust graph impact | empty | still empty | **same** |
| review completeness | fragile | still fragile | **same** |

Overall: **better** in concrete ways on the 12 applicable fixes; structural Rust graph and lint/build onboarding gaps remain.

## Improvement suggestions

1. **[P1 · init/verify]** Beyond `test_cmd`, fill `build_cmd`/`lint_cmd` by toolchain (`cargo build --bins`, `cargo fmt -- --check` or `clippy`).  
2. **[P1 · bench]** Honor config `bench.tasks_dir` / `fixtures_dir` when CLI flags are at defaults.  
3. **[P2 · graph]** Emit useful Rust `calls`/`impact` (today search helps; callers/impact do not).  
4. **[P2 · review]** Completeness: match test/file names in the diff to acceptance criteria, not only substring.  
5. **[P3 · mem]** Default invariants by toolchain (not always Go `tests/hidden/`).  
6. **[P3 · config]** 0644 permissions for non-secret YAML/AGENTS.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Approx. study wall time | ~25–40 min (Rust upgrade + bat build + tb flow + focused tests) | ~25–35 min |
| Recorded `tb` invocations | **~45** (includes intentional failure probes) | 52 |
| `tb` failures (exit ≠ 0) | **~12–14** (several intentional: strict guard, lint/build fail, bench without `-t`, probe review drift) | 13 |
| Approx. sum of `tb` times | **~10 s** short cmds + ~2.3 s verify PASS + ~4.4 s bench fake | ~7.0 s |
| `tb graph rebuild` (bat) | 0.37 s (2981 nodes) | 0.26 s (2971 nodes) |
| `tb verify run` PASS | ~2.3 s | (not comparable — build/lint now run) |
| `tb bench run --fake` (9 tasks) | ~4.4 s · 33.3 % | ~4.0 s · 33.3 % |
| tb binary | `a46c295` | `c8aae04` |

Local study artifacts (VM only): `/tmp/campo-bat/metrics.tsv`, `/tmp/campo-bat/out/*.txt`, clone `/tmp/target/bat` (not pushed upstream).
