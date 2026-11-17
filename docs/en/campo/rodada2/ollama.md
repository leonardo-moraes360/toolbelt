[🇧🇷 Português](../../../campo/rodada2/ollama.md) | 🇺🇸 English

# Field study round 2: ollama/ollama

Study on 2026-11-16 with `tb` built from `main` (`make build` → `toolbelt a46c295`). Local shallow clone at `/tmp/target/ollama`. Group: **with existing AI instrumentation**. **No interaction** with upstream (no push, issue, PR, or comment).

## Repository

| Field | Value |
|-------|-------|
| Name | [ollama/ollama](https://github.com/ollama/ollama) |
| Language | Go (~1097 `.go`), TS UI, native C/C++ (llama/MLX) |
| Size | ~53 MiB shallow; 1657 files (excl. `.git`); `go 1.26.10` in `go.mod` |
| Existing AI instrumentation | `AGENTS.md` (cmake + `go build`/`go run`); `CLAUDE.md` points to `AGENTS.md`; integration docs under `docs/integrations/` |
| Toolchain | **Partial → better than round 1.** `GOTOOLCHAIN=auto` → Go 1.26.10; `go test ./format/`, `go vet ./format/`, and **`go build .` OK** (~51 s). CMake: `cmake` present, but configure failed (`Ninja` missing; `CMAKE_C_COMPILER`/`CXX` unset in the probe). `golangci-lint` missing. GPU backends not attempted. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `HumanNumber` so it does not emit `1000K` when rounding crosses the K→M boundary (`999500` → `1M`). Change in `format/format.go` + cases in `format/format_test.go`. |
| Why | Open bug, pure Go, small package with tests; different scope from round 1. |
| Issue | https://github.com/ollama/ollama/issues/19490 |
| Completed? | **Yes, locally** (commit `9ffcee3` only on the clone). Not sent upstream. |
| Native test/lint/build | `go test ./format/ -count=1` PASS; `go vet ./format/` PASS; `go build .` PASS. Full native CMake **not** configured (ninja). |
| Round 1 (comparison) | Missing `verbose` flag in `ollama launch` (issue #14654) under `cmd/`. |

## Verification of the 12 fixes

| # | fix | worked / did not work / not tested / not applicable | evidence |
|---|-----|-----------------------------------------------------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph --enable index /tmp/target` → `8925 nodes, 65764 edges` (root `target` does not zero the graph); rebuild under `/tmp/target/ollama` → `8923` nodes |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; `hidden_test_cmd='go test ./format/ -run HumanNumber -count=1'` → JSON `hidden_available: true`, `hidden_passed: true`, `verdict: PASS` |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff touching `tests/hidden/x.go`: `--strict --json` → EXIT **1** and JSON with `protected_path`; same for `.toolbelt/thresholds.yaml` |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `format/format.go` and `format/*` → violations on those files in the fix diff |
| 5 | init toolchain detection | **worked** | `tb init` in a repo with `go.mod` → `verify.test_cmd: go test ./...` (not a blind default without detection) |
| 6 | review `tests/hidden` by path | **worked** | Ticket only *mentions* `tests/hidden/**` without touching the dir → `findings: []` under `--standards invariants` |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `default_agent: echo AGENT_OK {prompt}` + `tb bench run -r 1` **without** `--agent`/`--fake` → EXIT 0; `bench compare ... --agent 'echo COMPARE {prompt}'` → EXIT 0 |
| 8 | review/drift `## Scope`/`## Escopo`, lists and `**` | **worked** | `## Scope` with bullets `` `format/**` `` → clean drift; `## Escopo` + `format/**` OK; scope `cmd/**` → `out_of_scope: [format/...]` EXIT 1; `format/**/*.go` and `format/*.go` match |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Config with `build_cmd=go build ./format/`, `lint_cmd=go vet ./format/` → `build_ran/passed: true`, `lint_ran/passed: true` on verify PASS |
| 10 | init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents` | **worked** | Init logged `sources=AGENTS.md,CLAUDE.md` and hints (`cmake`, `go build`, `go run . serve`); wrote `.toolbelt/agent_docs.yaml`; `--write-agents` did not overwrite existing `AGENTS.md` |
| 11 | graph Python `calls` | **not applicable** | Go repo; not exercised on `.py`. In Go, `callers --function HumanNumber` returned real callers (`cmd.showInfo`, `server.GetModelInfo`, …) |
| 12 | aggregated visual skipped + CLI docs aligned | **partial** | Visual: `skipped: true`, `skip_reason: manifesto de telas ausente`. Docs: `tb serve`/`mem add` better documented; still has example `tb graph index --path` and mentions `--manifest` for guard while `guard check` CLI only exposes `--strict` |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` | OK `a46c295` | neutral | build with ldflags |
| `tb init` / `--write-agents` | OK; Go toolchain + `agent_docs.yaml` | **helped** | hints from `AGENTS.md`; config `0600` |
| `tb mem init/list/add/check` | OK | helped | anti-cheat invariants + scope mandate |
| `tb graph index/rebuild/search/impact/callers` | OK at scale (~9k nodes) | **helped** | found `HumanNumber`; useful impact/callers; index `/tmp/target` OK |
| `tb drift` | OK with `## Scope` | **helped** | clean under the mandate; EXIT 1 with wrong scope |
| `tb review` | OK + completeness warnings | hindered (partial) | 3 acceptance warnings despite `999500→1M` test in the diff |
| `tb verify run` (tests + build/lint/hidden) | PASS | **helped** | `hidden_available: true` without a `tests/hidden/` folder |
| `tb verify --ruler visual` | PASS skipped | neutral | `skipped: true` |
| `tb guard check` + `guard.yaml` | detects custom + hardcoded | helped / hindered | custom works; with yaml protecting the task’s own files, **verify FAIL** `iterations: 0` |
| `tb bench run` (default_agent) / `--fake` / `compare --agent` | OK | helped | toolbelt fixtures (`bugfix-go`) |
| `tb mcp` `tools/list` | OK | helped | version, echo, env, run, graph_* |
| Native checks | Go test/vet/build OK | — | aligned with `verify` when `test_cmd`/`build_cmd`/`lint_cmd` point at `./format/` |

## Problems that persist

- **Noisy review completeness FPs**: acceptance criteria marked “little evidenced” even with the `999500` case and `TestHumanNumber` in the diff (same pattern as round 1).
- **Mode `0600` configs**: `config.yaml` / `agent_docs.yaml` still mode `600`.
- **CMake/ninja still outside the VM**: full native build incomplete; `tb` does not block “done” for that (AGENTS hints help, but are not a gate).
- **CLI docs still have leftovers**: example `graph index --path`; `--manifest` described for guard without a CLI flag.
- **Verify aborts early on protected path**: desired anti-cheat for hardcoded paths; now also via `guard.yaml` — easy to fail verification if the manifest lists the task’s own files.

## New problems

1. **P2 — `guard.yaml` + verify: protecting the task path becomes an immediate FAIL**  
   - Repro: `protected_paths: [format/format.go]`; `tb verify run` on the commit that changes that file → `verdict: FAIL`, `iterations: 0`, without running build/test.  
   - Expected: custom manifest distinguishes “do not cheat” vs “sensitive files allowed in the mandate”; or warn instead of aborting verify when the path is in Scope.  
   - Actual: any `protected_path` error aborts verify.

2. **P3 — Example `tb graph index --path` still in docs**  
   - Repro: `docs/cli.md` ~L284 shows `--path`; real CLI uses a positional argument.  
   - Expected: examples only with accepted flags/args (fix #12 incomplete).

3. **P3 — Docs cite `--manifest` for guard; CLI has no such flag**  
   - Repro: `tb guard check --help` only `--strict`; docs still mention `--manifest`.  
   - Expected: docs = CLI (loads `.toolbelt/guard.yaml` implicitly).

## What tb caught and what it let slip

**Caught**
- Scope drift with `## Scope`/`## Escopo` and `**` globs.
- Custom guard (`format/*`) and hardcoded (`tests/hidden`, `thresholds.yaml`) with `--strict --json` failing the exit code.
- Verify with configured `build_cmd`/`lint_cmd`/`hidden_test_cmd` → real PASS on the `format` package.
- Graph: search/impact/callers for `HumanNumber` after indexing `/tmp/target`.
- Onboarding: reading `AGENTS.md`/`CLAUDE.md` + build hints.

**Let slip / weakened**
- Noisy review completeness (does not confirm obvious acceptance from the test diff).
- Did not turn AGENTS hints (cmake/ninja) into a full native `verify.build_cmd` — only Go detected.
- With a poorly calibrated `guard.yaml`, verify itself blocks validating the task (false “anti-cheat” over the mandate).
- Visual ruler without a manifest → skip (expected); zero visual signal.

## Interaction with existing instrumentation / onboarding

- **`AGENTS.md` / `CLAUDE.md`**: `tb init` **read** them and recorded `.toolbelt/agent_docs.yaml` with hints (`cmake -B build .`, `cmake --build`, `go build .`, `go run . serve`). Clear improvement vs round 1 (before: no read).
- **`--write-agents`**: available; did not overwrite existing `AGENTS.md` (correct).
- **Duplication**: generic `mem` invariants vs AGENTS build rules still run in parallel; hints did not auto-fill cmake `build_cmd`.
- **Onboarding**: better bootstrap (Go toolchain + agent docs). Still no progress log / generated ticket; manual `rd.md`.

## Comparison with round 1

| Aspect | Round 1 | Round 2 | Verdict |
|--------|---------|---------|---------|
| SkipDirs `/tmp/target` | risk of 0 nodes | 8925 nodes indexing `/tmp/target` | **better** |
| `guard.yaml` | ignored | read and applied | **better** |
| `--strict --json` | exit 0 with violation | exit 1 | **better** |
| `default_agent` / `compare --agent` | broken | work | **better** |
| `## Scope` Markdown | fragile / YAML | headings + bullets + `**` OK | **better** |
| `hidden_test_cmd` | gated by dir | runs without dir | **better** |
| `build_cmd`/`lint_cmd` | inert | honored | **better** |
| AGENTS.md in init | ignored | read + hints | **better** |
| Review completeness | noise | similar noise | **same** |
| Native cmake build | failed | still incomplete (ninja); `go build .` now OK | **partially better** |
| Task | `cmd` verbose | `format` HumanNumber | different (as requested) |

Overall: **substantially better** on the 12 exercisable fixes in this repo; remaining gaps are review completeness, residual docs, and the guard.yaml↔verify footgun.

## Improvement suggestions

1. **[P1][guard/verify]** Do not abort verify for custom `protected_paths` that are in the ticket Scope (or warning severity + flag).  
2. **[P1][review]** Completeness: treat asserts/test cases in the diff as acceptance evidence.  
3. **[P2][init/verify]** Map `AGENTS.md` hints → suggest `build_cmd`/`lint_cmd` (cmake when detectable).  
4. **[P2][docs]** Remove orphan `graph index --path` and `--manifest`; only document what the CLI accepts.  
5. **[P3][config]** Less restrictive mode for non-secret `agent_docs.yaml`.

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Field session wall time | ~8–12 min (includes `go build .` ~51 s + toolchain download) | ~6–10 min |
| `tb` invocations (useful command blocks) | ~30 (excludes 10 EXIT 127 harness errors in the log) | 59 |
| Intentional tb failures (exit ≠ 0) | 3+ (2× guard `--strict`, 1× wrong-scope drift; verify FAIL with guard.yaml also exit 1) | 13 |
| Approx. sum of tb WALLs (without native `go build`) | ~5–6 s of tb commands + verify/bench ~2 s | ~9.7 s |
| Local commits on the ollama clone | 1 feature (`9ffcee3`) + discarded probes | feature + probes |
| Artifact in this repo | only `docs/campo/rodada2/ollama.md` | `docs/campo/ollama.md` |
