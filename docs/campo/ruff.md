🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/ruff.md)

# Field study: `tb` on astral-sh/ruff

Date: 2026-11-15. Local clone at `/tmp/target/ruff` (shallow). No interaction with the target remote. Code changes stayed only on the local clone; in this toolbelt repository only this file.

`tb` used: local `make build` → `./bin/tb` (embedded version `c8aae04`).

## Repository

| Field | Value |
|-------|-------|
| Name | [astral-sh/ruff](https://github.com/astral-sh/ruff) |
| Language | Rust (monorepo: Ruff + ty); Python in fixtures/scripts |
| Size | Shallow checkout ~145–186 MiB (without `target`); after local build `target/` ~2.3 GiB; ~2055 `.rs` files |
| Existing AI instrumentation | `AGENTS.md` (~20 KiB), `CLAUDE.md` (`@AGENTS.md`), `.agents/` (skills + threat models + Claude plugin), `.claude/settings.json` + hooks |
| Toolchain | **Partial.** Rust 1.99.0 (via `rust-toolchain.toml`), `cargo`, `cargo-nextest`, `uv` 0.13.8, `cargo-insta` via `uv run`. Compiled and tested the `ruff_linter` crate. **Did not** run: full workspace suite, `cargo clippy --workspace`, `prek` hooks, playground, release builds, file-watcher tests |

## Chosen task

| Field | Value |
|-------|-------|
| What | Prevent `FURB101`/`FURB103` from emitting a “safe” fix that rewrites `open(b"...")` → `Path(b"...").read_*()` / `write_*()`, which breaks at runtime (`TypeError` on `pathlib.Path`). Skip bytes-literal at the same point where file descriptors are already ignored (`helpers.rs`). Updated fixtures + snapshots. |
| Why | Real bug, small scope, with existing snapshot tests; blocks preview stabilization (`#27051`). |
| Issue | https://github.com/astral-sh/ruff/issues/26922 (comment asks for direction; local implementation followed option 3 aligned with the fd skip in `#27643`) |
| Completed? | **Yes on the local clone** (code + fixtures + snapshots; `cargo test -p ruff_linter --lib rules::refurb::tests` → 45 ok). Not sent upstream (forbidden by study rules). |
| Native checks | `cargo test -p ruff_linter --lib rules::refurb::tests` PASS (~87 s cold / ~0.06 s hot). `cargo check -p ruff_linter` PASS on the fix commit (~20 s). Workspace clippy / workspace nextest **not** completed. |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; created `.toolbelt/` + `config.yaml` | hindered (defaults) | Default `verify.test_cmd` `go test ./...` in a Rust repo; `config.yaml` mode `600` |
| `tb version` (make build) | `toolbelt c8aae04` | neutral | Makefile ldflags fill version |
| `tb version` (go build without ldflags) | `0.1.0-dev`, `commit: unknown`, `built: unknown` | hindered | `/tmp/tb-nolags version` |
| `tb mem init/list/add/check` | OK; 3 default invariants + 1 added | helped | `mem add` accepts positional rule; `check` reported `compactacao_segura: true` |
| `tb graph rebuild --enable` | 58926 nodes, 92578 edges in ~4.96 s | mixed | Fast indexing; but only **232** `calls` edges in the whole Rust monorepo |
| `tb graph search --enable read_whole_file` | Found `read_whole_file`, `ReadWholeFile`, etc. (~0.37 s) | helped | Located the rule files |
| `tb graph impact --enable --file …/helpers.rs` | empty / `null` | hindered | No useful impact even though the file exists in the graph |
| `tb graph impact` without `--file` | error `--file é obrigatório` | neutral | Confirms required flag |
| `tb graph callers --enable --function find_file_open` | empty | hindered | Weak Rust call graph |
| `tb graph deps --enable --file …/helpers.rs` | listed regex-ish pkgs (`pkg:crate::…`) | neutral | Superficial dependencies |
| ticket / Scope (manual `rd.md`) | `tb ticket` **does not exist** in the binary | hindered | Docs describe `tb ticket`; real CLI: unknown command. Scope via YAML frontmatter works |
| `tb drift --ticket rd.md` | Detected `rd.md` out of scope | helped | JSON with `out_of_scope: ["rd.md"]`; base matching `base_commit` |
| `tb drift` with globs | `/**` (suffix) and `dir/*.py` work; literal paths too | mixed | `rd-glob2.md` / `rd-glob3.md`; without glob, literals clean in `rd-literal.md` |
| `tb verify run --ruler tests` | PASS on the fix (~0.22 s hot) with `cargo test -p ruff_linter …` | mixed | `hidden_available: false` (no `tests/hidden/`); Go defaults useless without reconfig |
| `tb verify` + build error | With `--test-cmd true` → PASS; with `cargo check` → ESCALATE | hindered | Verify only runs the configured command; does not force build |
| `tb verify run --ruler visual` (no manifesto) | PASS, `skipped: true`, reason manifesto absent | neutral | Graceful skip in the top-level report |
| `tb verify` visual with manifesto without refs | PASS, **`skipped: false`**, screen `skipped: true` | hindered | Top-level report does not mark skip when only screens skip |
| `tb review` | Findings: scope `rd.md` + `tests/hidden` invariant FP | hindered | Ticket diff mentions `tests/hidden/` → critical false positive; weak token completeness |
| `tb guard check HEAD~1..HEAD` | No violations on the fix | neutral | Different UX (`base..head`, not `--base`) |
| `tb guard` + custom `.toolbelt/guard.yaml` | Custom **ignored**; hardcoded paths | hindered | Code: `NewChecker` fixes `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml` |
| `tb guard` touching `thresholds.yaml` | `protected_path` violation; `verify` returns FAIL without running tests | mixed | Expected abort; hardcoded path hinders if the agent edits tb config |
| `tb bench run` without `--agent`/`--fake` | Required error, ignores `default_agent` | hindered | Config with `default_agent: "echo-agent {prompt}"` ignored |
| `tb bench run --fake` | OK without `--fake-config` | helped | 9 tasks / 1 rep on the toolbelt repo; 33.3% success |
| `tb bench compare` | No `--agent` flag | neutral/confirmed | Only `--fake` / JSONL / A/B configs |
| `tb mcp` | `tools/list` + `tools/call` (`tb_version`, `tb_graph_search`) OK | helped | With `TB_GRAPH=on`, search returned `ReadWholeFile` |
| Docs `cli.md` vs binary | `ticket`, `board`, `run`, `md` documented, absent from CLI | hindered | `tb ticket` / `tb board` / `tb run` → unknown command |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare --help`: no `--agent`; only `--fake` for A/B |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` created; checker does not read it; docs still document it; only hardcoded paths in `internal/guard/guard.go` |
| outdated CLI docs | **confirmed** | `docs/cli.md` lists `tb ticket`, `tb board`, `tb run`, `tb md`; Phase 2 binary lacks them |
| version shows `unknown` | **partially confirmed** | Without ldflags: version `0.1.0-dev`, **commit/built = unknown**. With `make build`: version = short hash |
| config files created 0600 | **confirmed** | `stat`: `config.yaml` and `invariants.yaml` mode `600` |
| visual ruler reports `skipped=false` when skipping | **confirmed** | Manifesto present, refs missing: top-level `"skipped": false` with screen `"skipped": true` / `skip_reason: referência ausente` |
| `default_agent` ignored | **confirmed** | `bench.default_agent` set; `tb bench run` still requires `--agent` or `--fake` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | Commit on `.toolbelt/thresholds.yaml` → guard error → verify `FAIL` `iterations: 0` |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake` works without `--fake-config` (built-in fake agent) |
| `mem add` positional | **confirmed** | Usage: `tb mem add <regra>`; no `--rule`; join of positional args |
| Drift/Scope need literal paths (no globs) | **partially confirmed** | Literals OK. Suffix `/**` and `filepath.Match` (`dir/*.py`) OK. Docs/UX still push literals; mid-path/`**` globs limited |
| `graph impact` needs `--file` | **confirmed** | Without `--file`: `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | **confirmed** | Ticket commit with `forbidden: tests/hidden/` in the diff → critical finding even without touching `tests/hidden/` |
| verify runs only partial checks and misses build errors | **confirmed** | Go defaults; `hidden` only if dir exists; `--test-cmd true` PASS with broken Rust; only fails if `test_cmd` includes build |
| tb does not push agent toward completing the whole task | **confirmed (in this run)** | No `tb ticket`/`board`; init does not import `AGENTS.md`/skills; review completeness = weak token matching; FP/scope noise distracts; verify defaults do not guide to the repo checklist |

## New problems

1. **P1 — Useless call graph / impact in a Rust monorepo**  
   - Repro: `TB_GRAPH=on tb graph rebuild --enable` then `tb graph impact --enable --file crates/ruff_linter/src/rules/refurb/helpers.rs` → empty; `callers --function find_file_open` → empty. Stats: 31116 `.rs` nodes, but only 232 `calls` edges.  
   - Expected: impact/callers from Rust functions.  
   - Actual: symbol/file indexing OK; `calls` relations almost absent.

2. **P2 — Go-centric `verify` defaults without language detection**  
   - Repro: `tb init` on ruff → `test_cmd: go test ./...`.  
   - Expected: detect Cargo/`AGENTS.md` and suggest project commands.  
   - Actual: agent must reconfigure manually or verify “passes” without meaning.

3. **P2 — Inconsistent guard vs drift/review UX**  
   - Repro: `tb guard check --base HEAD~1` → unknown flag; needs `tb guard check HEAD~1..HEAD`.  
   - Expected: same `--base`/`--head` flags.  
   - Actual: two dialects.

4. **P3 — `graph callers`/`deps` require flags, not positionals**  
   - Repro: `tb graph callers --enable find_file_open` fails (`--function` required).  
   - Expected (by CLI habit): positional argument or a friendly message.  
   - Actual: usage error easy to trip over in an agent flow.

5. **P3 — Review completeness does not validate real acceptance**  
   - Repro: acceptance criteria in the ticket; review did not fail completeness despite behavioral criteria (“no longer triggers”).  
   - Expected: check tied to tests/`verify`.  
   - Actual: token heuristic on the diff (`reviewCompleteness`).

## What tb caught and what it let slip

**Caught**
- Drift of `rd.md` outside the declared scope.
- Base commit aligned with the ticket.
- Guard on protected path `.toolbelt/thresholds.yaml`.
- Verify with `cargo check` detected a broken build (ESCALATE after 3 iterations).
- Graph search found FURB101 symbols.
- MCP exposed the same basic tools.

**Let slip / hindered**
- Did not read `AGENTS.md` or skills (test commands, snapshot policy, AI policy).
- `go test` defaults would not have exercised the fix.
- Review: critical false positive on `tests/hidden` and scope noise on the ticket itself.
- Impact/callers did not help find side effects.
- With a weak `test_cmd`, verify PASS with invalid Rust code.
- Did not guide preview stabilization / issue design discussion (3 options in the comment).
- Did not suggest running workspace clippy/nextest.

## Interaction with existing instrumentation / onboarding

- **`tb` ignores** `AGENTS.md`, `CLAUDE.md`, `.agents/skills/*`, threat models, and `.claude/` hooks. No Go code references these paths (only research docs mention AGENTS.md as a future `tb ctx` idea).
- **`tb init`** creates **parallel** vault/invariants/config without importing repo rules (e.g. “never edit snapshots by hand”, `file_watching` filters, `INSTA_*`, AI policy).
- **Potential duplication**: generic invariants (`tests/hidden/`, `thresholds.yaml`) do not match Ruff’s real harness (mdtest/insta/nextest) and produce FPs.
- **Conflict**: Go defaults vs Cargo instructions in `AGENTS.md`.
- **What tb should import**: (1) “Running Tests” / Clippy blocks from `AGENTS.md` → `verify.test_cmd` / lint; (2) `.agents/skills` as optional playbooks; (3) manual-snapshot prohibitions → `mem` invariants; (4) labels/PR conventions only if there is a PR flow (not applicable here).

Observed onboarding: `tb init` + `tb mem init` in seconds, but **does not** onboard the agent into Ruff’s workflow; the human/AI agent still must read `AGENTS.md` outside tb.

## Improvement suggestions

1. **[graph/P1]** Rust indexer with real `calls`/`impact` (or document the limitation and do not advertise impact on Rust until it exists).  
2. **[verify/P1]** Detect toolchain on `init` (Cargo.toml → `cargo test -p …`; read `AGENTS.md` if present) and include `cargo check`/`clippy` in the preset.  
3. **[mem-ctx/P1]** `tb init --import-agents` (or automatic): extract invariants and test cmds from `AGENTS.md` / `.agents`.  
4. **[review/P1]** `tests/hidden` FP: only fire if the **changed** path is under `tests/hidden/`, not if the string appears in the ticket.  
5. **[guard/P2]** Honor `.toolbelt/guard.yaml`; align `--base`/`--head` flags.  
6. **[docs/P2]** Sync `docs/cli.md` with real commands; mark ticket/board/run/md as planned.  
7. **[bench/P2]** Honor `default_agent`; optionally `--agent` on `compare`.  
8. **[verify/P2]** Visual report: top-level `skipped=true` if every screen skipped.  
9. **[cli/P3]** Accept positionals on `graph callers`/`deps`/`impact` in addition to flags.  
10. **[router/ticket/P2]** Implement or remove docs for `tb ticket`/`board` — without that tb does not push end-to-end acceptance closure.

## Metrics

| metric | value |
|--------|-------|
| Study wall time (approx., after tb build) | ~6–15 min of tb+cargo use on the target; graph rebuild 4.96 s; cold cargo test refurb ~87 s; hot verify ~0.22 s; verify+broken cargo check ~19 s |
| tb invocations (command families / log markers) | ~25 command families; ~36 `===` sections in log `/tmp/tb-campo-ruff.log` |
| Observed tb failures / errors | `graph impact` without `--file`; `callers`/`deps` without flags; nonexistent `ticket`/`board`/`run`; `bench run` without agent; `drift`/`review` exit≠0 from findings; verify ESCALATE with broken build; guard unknown `--base` |
| tb crashes | No crash/segfault observed |
| MCP | `tools/list` + `tb_version` + `tb_graph_search` OK via stdio |

Raw local log (VM): `/tmp/tb-campo-ruff.log`. Study clone and commits were **not** sent to `astral-sh/ruff`.
