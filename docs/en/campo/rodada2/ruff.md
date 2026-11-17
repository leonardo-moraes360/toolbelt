[🇧🇷 Português](../../../campo/rodada2/ruff.md) | 🇺🇸 English

# Field study round 2: `tb` on astral-sh/ruff

Date: 2026-11-16. Local clone at `/tmp/target/ruff` (shallow). No interaction with the target remote. Code changes stayed only on the local clone; in this toolbelt repository only this file.

`tb` used: local `make build` → `./bin/tb` (embedded version `a46c295`, main with the 12 fixes).

Group: **with existing AI-agent instrumentation**.

## Repository

| Field | Value |
|-------|-------|
| Name | [astral-sh/ruff](https://github.com/astral-sh/ruff) |
| Language | Rust (Ruff + ty monorepo); Python in fixtures/scripts/resources |
| Size | Shallow checkout ~145–187 MiB (without full `target` in use); ~2057 `.rs` files |
| Existing AI instrumentation | `AGENTS.md` (~20 KiB), `CLAUDE.md` (`@AGENTS.md`), `.agents/` (skills + threat models), Claude hooks/settings |
| Toolchain | **Partial.** Rust 1.99.0 (`rust-toolchain.toml`), `cargo`, `cargo-nextest` 0.9.152, `uv` 0.13.8. Built/tested `ruff_linter` (pep8_naming) and `cargo run -p ruff -- check` on the probe. **Did not** run: full workspace suite, `cargo clippy --workspace`, `prek` hooks, playground, file-watcher |

## Chosen task

| Field | Value |
|-------|-------|
| What | `N801` must not flag CapWords names with a **trailing** underscore (`class Fine_`, `class _Fine_`), aligned with the leading-underscore skip and pep8-naming/PEP 8. Still flags mid-name underscores (`Bad_Class_`). Fixture + snapshot updated. |
| Why | Small real bug (#30230), distinct from round 1, with an existing snapshot harness. |
| Issue | https://github.com/astral-sh/ruff/issues/30230 |
| Complete? | **Yes on the local clone** (`trim_matches('_')` in `invalid_class_name.rs`; pep8_naming tests 49 ok; `ruff check --select N801` on the probe: only `Bad_Class_` and `bad`). Not sent upstream. |
| Native checks | `cargo test -p ruff_linter --lib rules::pep8_naming::tests` PASS (~84 s cold / ~0.04 s warm). `cargo check -p ruff_linter` PASS. Workspace clippy/nextest **not** completed. |
| Round 1 (comparison) | FURB101/103: do not emit a “safe” fix for `open(b"...")` → `Path(...).read_*/write_*` |

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Useful clone under `/tmp/target/ruff` → 58926 nodes / 96890 edges. Dedicated probe `/tmp/skipdirs-probe/target` (basename=`target`) → 6+ nodes after rebuild (no longer zeros). |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; with `hidden_test_cmd: cargo check -p ruff_linter` → `hidden_available: true`, `hidden_passed: true` in verify JSON. |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | With a violation: `--json` alone → EXIT 0; `--strict --json` → EXIT 1 (same JSON payload with `violations`). |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | Custom path `custom_protected_marker.txt` in `.toolbelt/guard.yaml` → `protected_path` violation on the commit that touched it; union with defaults documented in `docs/cli.md`. |
| 5 | init toolchain detection | **worked** | `tb init` on ruff → `verify.test_cmd: cargo test` (no longer `go test ./...`). |
| 6 | review `tests/hidden` by path | **worked** | Ticket mentions `tests/hidden/` in the text; `tb review --ticket rd.md --json` → `findings: []` (no critical FP). |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `bench.default_agent: "echo-agent {prompt}"`; `tb bench run -r 1` without `--agent` → `agent=external` and finished. `tb bench compare --help` lists `-a/--agent`; A/B with `--config-a/b --agent "echo-agent {prompt}" --task bugfix-go` produced a Markdown table. |
| 8 | review parses `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | Ticket with `## Escopo` + bullets + `pep8_naming/**` / `snapshots/**`: drift/review without `out_of_scope` on the fix; noise commit `lib_noise_campo.txt` → `out_of_scope` detected. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Config with `build_cmd: cargo check -p ruff_linter`, `lint_cmd: true` → JSON `build_ran/passed` and `lint_ran/passed` true; verify PASS ~19.5 s. |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** (partial UX) | On ruff: log `documentação de agentes encontrada` + `.toolbelt/agent_docs.yaml` with `cargo`/`uv`/`nextest` hints. `--write-agents` in an empty repo seeded only `AGENTS.md` (did not create `CLAUDE.md`); hints **do not** become `test_cmd` automatically beyond Cargo detection. |
| 11 | graph Python `calls` edges | **worked** | Stats: `edges_calls: 4526` (round 1: 232). Heuristic on `graph.json`: ~4294 calls linked to `.py` / fixtures; probe `mod.py` → 1 call and `tb graph callers --function helper` returned `mod.main`. |
| 12 | aggregated visual skipped + CLI docs aligned | **worked** | Without manifest: top-level `skipped: true`. With manifest and missing ref: `skipped: true`, `skip_reason: todas as telas foram skipped`. `docs/cli.md` documents build/lint, visual skipped, `protected_paths`, `default_agent`, and marks `tb ticket` as not implemented. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; `.toolbelt/` + `config.yaml` + `agent_docs.yaml` | **helped** | Detected Cargo; read AGENTS/CLAUDE; still mode `600` on YAMLs |
| `tb init --write-agents` | OK on ruff (docs already existed); empty probe seeded AGENTS.md | mixed | Does not seed CLAUDE.md; does not import `.agents/` skills |
| `tb mem init/list/add/check` | OK; 3 default + 1 insta rule | helped | `compactacao_segura: true` |
| `tb graph rebuild --enable` | 58926 nodes / 96890 edges / 4526 calls ~4.57 s | helped | Fast indexing; root SkipDirs OK |
| `tb graph search --enable invalid_class_name` | Found function/file/class | helped | ~0.38 s |
| `tb graph impact --enable --file …/invalid_class_name.rs` | `null` | hindered | Rust call graph still weak (~232 “other” calls) |
| `tb graph callers --enable --function invalid_class_name` | `null` | hindered | Same |
| `tb graph deps --enable --file …` | pkgs `crate::…` / `ruff_python_ast` | neutral | Shallow imports |
| ticket (manual `rd.md`) | `tb ticket` nonexistent (docs say planned) | neutral | Scope via `## Escopo` worked with drift/review |
| `tb drift --ticket rd.md` | Clean on the fix; flagged noise outside the glob | **helped** | `**` globs + Markdown heading OK |
| `tb verify run` | PASS with build+lint+test+hidden | **helped** | Cargo defaults; hidden without dir; ~19.5 s |
| `tb verify run --ruler visual` | PASS skipped (manifest/ref) | neutral | Correct top-level aggregation |
| `tb review --ticket rd.md` | `findings: []`, clean drift | **helped** | No `tests/hidden` FP |
| `tb guard check` + `guard.yaml` | Reads YAML; `--strict --json` EXIT 1 | **helped** | Custom path and fix path detected |
| `tb bench run` (default_agent) | Ran without `--agent`/`--fake` | helped | 9 tasks / 11.1% success with echo-agent |
| `tb bench compare --agent` | A/B graph-off vs graph-on OK | helped | `--agent` flag present and working |
| `tb mcp` | `tools/list` + `tb_version` + `tb_graph_search` OK | helped | With `TB_GRAPH=on`, search found `invalid_class_name` |
| Native checks (outside tb) | pep8_naming + ruff check N801 OK | helped | Confirmed Fine_/Bad_Class_ behavior |

## Problems that persist

Round 1 problems (`docs/campo/ruff.md`) that **still** appear in this run:

| round 1 problem | round 2 status |
|-----------------|----------------|
| Useless call graph / impact in a **Rust** monorepo | **persists** — `impact`/`callers` on `invalid_class_name` → `null`; ~232 non-Python calls |
| Config files mode `0600` | **persists** — `stat` → `600` on `config.yaml` / `invariants.yaml` |
| Missing `tb ticket` / board (only manual rd.md) | **persists** — `unknown command "ticket"`; docs now mark planned |
| Completeness/review does not truly validate behavioral acceptance | **persists** (partial) — review passed without linking to `verify`/N801 test |
| Init does not import `.agents/` skills / snapshot policy as invariants | **persists** — only hints in `agent_docs.yaml`; `mem` still generic |
| Verify defaults still too generic without refining crate/package | **partially mitigated** — `cargo test` instead of `go test`, but not the AGENTS.md filter `-p ruff_linter …` |
| Guard UX `base..head` vs `--base` | **not deeply revalidated** — `HEAD~1..HEAD` flow used successfully |

## New problems

1. **P2 — `--write-agents` seeds only `AGENTS.md`, not `CLAUDE.md`**  
   - Repro: `tb init --write-agents` in an empty repo → creates `AGENTS.md`; `CLAUDE.md` absent.  
   - Expected (per fix 10 wording): AGENTS/CLAUDE seed.  
   - Actual: `SeedAgentsMD` only writes AGENTS.md.

2. **P2 — AGENTS.md hints do not become structured `verify.*_cmd`**  
   - Repro: init on ruff extracts `cargo nextest …`, `uv run … prek`, etc., but `test_cmd` stays only `cargo test`.  
   - Expected: map suggested lint/test/build from AGENTS.md.  
   - Actual: informative record in `agent_docs.yaml` without applying to verify.

3. **P3 — `lint_cmd: true` / echo agents mask quality**  
   - Repro: verify PASS with `lint_cmd: true`; bench default_agent echo “passes” tasks without editing code (11.1%).  
   - Expected: fail or warn when lint/test are no-ops.  
   - Actual: harness accepts and reports PASS/low success without a weak-config alert.

4. **P3 — Skills `.agents/skills/*` still outside tb**  
   - Repro: directory with ty/review skills present; no tb command lists/imports them.  
   - Expected (“with instrumentation” group): at least discovery.  
   - Actual: parallel to human instrumentation.

## What tb caught and what it let slip

**Caught**
- Drift outside `## Escopo` / `**` globs.
- Review without `tests/hidden` FP despite the ticket mention.
- Guard with custom `protected_paths` + strict+json exit.
- Verify with configured build + hidden without a `tests/hidden/` folder.
- Graph search of the N801 symbol; Python calls in the index; SkipDirs on a dir named `target`.
- Cargo init + AGENTS/CLAUDE read.

**Let slip / hindered**
- Rust impact/callers did not help find who calls `invalid_class_name`.
- Did not suggest the exact AGENTS.md command (`nextest -p …`, `INSTA_*` policy).
- Did not push clippy/workspace; agent had to choose the subset.
- Bench with echo-agent does not validate the N801 task (only toolbelt fake/external harness).
- Without `tb ticket`, the acceptance flow stays manual.

## Interaction with existing instrumentation / onboarding

- **Improvement vs round 1:** `tb init` **reads** `AGENTS.md`/`CLAUDE.md`, writes `agent_docs.yaml`, and lists command hints; `--write-agents` documented and functional for a minimal seed.
- **Still parallel:** default invariants (`tests/hidden/`, `thresholds.yaml`) do not reflect the Ruff harness (insta/mdtest/nextest). Skills under `.agents/skills/` and threat models **do not** enter vault/`mem`.
- **Residual conflict:** `test_cmd: cargo test` is better than Go, but AGENTS.md mandates `cargo nextest` + `INSTA_*` vars / `file_watching` exclusion — the agent still must read AGENTS.md outside tb for the correct workflow.
- Onboarding: seconds for init/mem; *real* Ruff onboarding still depends on a human/AI opening AGENTS.md.

## Comparison with round 1

| aspect | round 1 (`c8aae04`) | round 2 (`a46c295`) |
|--------|---------------------|---------------------|
| Init / verify defaults | `go test ./...` | `cargo test` + optional build/lint/hidden |
| AGENTS.md | ignored | read + hints; skills still out |
| Review FP `tests/hidden` | critical FP | absent |
| Markdown Scope / `**` | fragile / partial | `## Escopo` + globs OK in this run |
| guard.yaml / strict+json | ignored / exit 0 | honored / EXIT 1 |
| Graph under `/tmp/target…` | SkipDirs risk | root OK; Python calls ↑; Rust impact equally weak |
| Bench default_agent / compare --agent | broken | works |
| Visual skipped top-level | failed with missing refs | aggregated `skipped: true` |
| Conclusion | tb hindered Rust bootstrap | **better** harness; **same** on Rust call-graph and skill-onboarding depth |

Overall: **better** in concrete ways on the 12 testable fixes; the structural Rust monorepo pain (impact/callers) and fine use of AI instrumentation **remain**.

## Improvement suggestions

1. **[graph/P1]** Emit useful Rust `calls`/`impact` (or document the limitation and degrade empty-impact UX).  
2. **[init/P1]** Apply AGENTS.md hints to `verify.test_cmd` / `lint_cmd` / `build_cmd` (e.g. prefer `cargo nextest` when cited).  
3. **[init/P2]** `--write-agents` also seed or link `CLAUDE.md`; optional `--import-skills` from `.agents/skills`.  
4. **[mem/P2]** Import AGENTS.md prohibitions (“do not edit snapshots by hand”) as invariants.  
5. **[verify/P2]** Warn when `lint_cmd`/`test_cmd` are no-ops (`true`, `echo`).  
6. **[bench/P3]** On a target without `bench/tasks`, fail with a clear message instead of depending on an absolute toolbelt path.  
7. **[cli/P3]** Keep `ticket`/`board` docs clearly “planned” (already improved) and point at an `rd.md` template with `## Escopo`.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Wall time (approx., after tb build) | ~15–25 min tb+cargo on the target (incl. ~84 s cold pep8_naming test; graph ~4.6 s; verify ~19.5 s; ruff binary build for N801 probe ~1–2 min) | ~6–15 min; graph 4.96 s; cold cargo test refurb ~87 s |
| tb invocations (`===` / family marks) | ~25 sections in log `/tmp/tb-campo-ruff-r2.log` + extra probes (guard/bench/MCP/write-agents) | ~25 families / ~36 sections |
| Observed tb failures / errors | empty Rust `graph impact`/`callers`; `tb ticket` unknown; drift EXIT≠0 on noise probe (expected); guard EXIT 1 with `--strict` (expected) | similar + several bugs now fixed |
| tb crashes | None | None |
| MCP | `tools/list` + `tb_version` + `tb_graph_search` OK | OK |

Local raw log (VM): `/tmp/tb-campo-ruff-r2.log`. Study clone and commits were **not** sent to `astral-sh/ruff`.
