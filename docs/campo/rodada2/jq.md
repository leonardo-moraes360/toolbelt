🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/jq.md)

# Field study (round 2): jqlang/jq

Group: without existing AI-agent instrumentation.  
Tool: `tb` built from this repo on `main` (`make build` → `bin/tb`, ldflags `a46c295`).  
Target clone: `/tmp/target` (local only; no push/issue/PR/comment upstream).  
Date: 2026-11-16.

## Repository

| Field | Value |
|-------|-------|
| Name | [jqlang/jq](https://github.com/jqlang/jq) |
| Language | C (autotools + embedded oniguruma) |
| Size | shallow + submodules ≈ 55–56 MiB on disk; ~27 kLOC under `src/` (`.c`/`.h`); 45 `.c`/`.h` files under `src/` |
| AI instrumentation | None on entry (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot, Aider absent). Only CI under `.github/workflows/` |
| Toolchain | **Yes, complete**: `autoconf`/`automake`/`libtool`/`flex`/`bison` installed in the session; `git submodule update --init`; `autoreconf -i`; `./configure --with-oniguruma=builtin --disable-docs`; `make -j`; baseline `make check` 9/9 PASS. HTML docs not generated. No Valgrind/scan-build this session |

## Chosen task

| Field | Value |
|-------|-------|
| What | Make assignment to an array slice beyond the current length pad with `null` (like `.[10] |= 2`), in `jv_set` / `parse_slice` (`src/jv_aux.c`), with regressions in `tests/jq.test` |
| Why | Maintainer-filed bug, small scope, different from round 1, verifiable with `make check` and one-liners |
| Issue | https://github.com/jqlang/jq/issues/2813 |
| Complete? | **Yes** (locally): `jq -ncM '[1] \| .[10:11] \|= [2]'` → `[1,null,…,null,2]` (same as `.[10] \|= 2`); `make check` 9/9 PASS |
| Native checks | `make -j` OK; `make check` 9/9 PASS after the fix (~8 s). No dedicated C linter in the repo beyond what CI/scanbuild would do (not run) |
| Round 1 (comparison) | Reject JSON literals with a trailing decimal point (`23.`) in `src/jv_parse.c` / issue #2414 |

Ticket: `rd.md` with `## Scope` / `## Escopo`, Markdown lists, `**` globs, `base_commit`, and `forbidden` (mention of `tests/hidden/**`).

## Verification of the 12 fixes

| # | fix | worked / did not work / not tested / not applicable | evidence |
|---|-----|-----------------------------------------------------|----------|
| 1 | graph SkipDirs at walk root | **worked** | Clone under `/tmp/target` (basename `target`): `tb graph --enable index .` → **1064 nodes / 1530 edges** (no longer zeros) |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; with `hidden_test_cmd: echo HIDDEN_RAN && exit 0` → `hidden_available: true`, `hidden_passed: true`, verdict PASS |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff on `.toolbelt/thresholds.yaml` → JSON with `protected_path` and **exit 1** with `--strict --json` |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `src/jv_aux.c`; `tb guard check BASE..HEAD --strict --json` → violation on that file, exit 1 |
| 5 | init toolchain detection | **worked** | `tb init` on jq → `verify.test_cmd: make check` (via `Makefile` / `check` target) |
| 6 | review `tests/hidden` matches by path | **worked** | Ticket mentions `forbidden: tests/hidden/**` / anti-cheat invariant; review **did not** produce an invariant FP from substring alone — only drift of `rd.md` out of scope |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Without `--agent`/`--fake`, with `bench.default_agent` set → runs (`agent_output` contains the agent echo). `tb bench compare ... --agent 'echo AGENT {prompt}'` accepted (flag exists and runs A/B) |
| 8 | review/drift `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | Ticket with only `## Scope` + bullets: `src/jv_aux.c`/`tests/jq.test` in scope; `rd.md` out. Glob `src/**/*.c` + `tests/**` also matches `src/jv_aux.c` (only `rd.md` out_of_scope) |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | With `build_cmd: make -j2`, `lint_cmd: true` → `build_ran/passed` and `lint_ran/passed` true on PASS. With `build_cmd: false` → `build_passed: false`, verdict `ESCALATE` |
| 10 | init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents` | **worked** | Repo without instrumentation: `tb init --write-agents` seeded `AGENTS.md` + `.toolbelt/agent_docs.yaml` (`sources: [AGENTS.md]`). Without the flag, log suggests `--write-agents` |
| 11 | graph Python `calls` edges | **not applicable** (C repo); fix **confirmed** in a probe | On jq: `edges_calls: 19` (shallow C). Probe `/tmp/py-calls-probe/m.py` (`a` calls `b`): `edges_calls: 1`; `callers --function b` → `func:m.a` |
| 12 | aggregated visual skipped + CLI docs | **worked** | Manifest with missing ref: top-level `"skipped": true`, `"skip_reason": "todas as telas foram skipped"`. `docs/cli.md` marks `tb ticket` as **not implemented** |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` | OK (`a46c295` with ldflags) | neutral | Build via Makefile |
| `tb init` / `--write-agents` | OK; `make check`; seed `AGENTS.md` | helped | Correct C/Make defaults; minimal onboarding generated |
| `tb mem init/list/add/check` | OK; 3+1 invariants; sha256 check | helped / hindered (docs) | `mem add --type` still `unknown flag`; positional OK |
| `tb graph --enable index/stats/search` | 1064 nodes; search found `jv_set` / `parse_slice` | helped | Root SkipDirs fixed; useful for locating symbols |
| `tb graph callers/deps/impact` | callers/`impact` empty for useful C; deps = includes | hindered (C) | `callers --function jv_array_set` → `(vazio)`; `impact --file src/jv_aux.c` → `(vazio)` |
| Ticket (`rd.md`) | Manual; `tb ticket` absent | neutral | Docs now say “planned”; drift/review flow consumes `rd.md` |
| `tb drift` | Detected `rd.md` outside `## Scope` | helped | Exit 1 + JSON; still dumps Usage on stderr |
| `tb review` | Drift of `rd.md`; no `tests/hidden` FP | helped | Markdown Scope OK; Usage on stderr remains |
| `tb verify run --ruler tests` | PASS with build/lint/test/hidden | helped | ~7–8 s with `make check`; hidden runs without a folder |
| `tb verify` + `guard.yaml` on the edited file | FAIL early (`protected_path`) | hindered | Protecting the task path aborts verify before tests |
| `tb verify --ruler visual` | PASS aggregated skipped | neutral | Chrome present; missing refs → correct top-level skip |
| `tb guard check` | Reads `guard.yaml`; `--strict --json` exit 1 | helped | Hardcoded + YAML |
| `tb bench run` / `compare` | `default_agent` and `--agent` OK; `--fake` OK | helped / partial | Tasks linked from toolbelt; `passed: true` even with agent `exit_code: 1` if the Go fixture passes |
| `tb mcp` | `initialize` + `tools/list` OK | helped | graph tools + version/echo/env/run |
| Native checks vs tb | `make check` aligned with configured verify | — | Verify without `make check` (e.g. `test_cmd: true`) can still misleadingly PASS |

## Problems that persist

- **`graph callers` / `impact` barely useful in C** (shallow index; few `calls` edges) — same as round 1.
- **Missing `tb ticket`** (now documented as planned; command still absent).
- **`mem add` positional only** (`--type`/`--title` fail).
- **Config mode `0600`** (`config.yaml`, seeded `AGENTS.md`, etc.).
- **Cobra Usage on stderr** after useful JSON on drift/review with exit ≠ 0.
- **Completeness / acceptance**: review does not truly validate acceptance checkboxes; operator still must run well-configured `make check`.
- **Verify can PASS with a weak `test_cmd`** if the human/config does not point at the real suite (partly mitigated by `build_cmd`, not eliminated).

## New problems

### P2 — `guard.yaml` on the task’s own path aborts `tb verify`

- **Repro**: `protected_paths: [src/jv_aux.c]` + a diff that edits that file; `tb verify run --ruler tests`.
- **Expected**: guard reports; verify can still run tests **or** distinguish “sensitive path” vs “mandate file”.
- **Actual**: `verdict: FAIL`, `iterations: 0`, only `protected_path` violation — hidden/build/test do not even run.

### P2 — `tb bench` marks `passed: true` with agent `exit_code: 1`

- **Repro**: `default_agent: 'printf … && false'`; `tb bench run -r 1 --task bugfix-go`.
- **Expected**: agent failure (or at least `passed: false` if exit ≠ 0).
- **Actual**: `agent_output` shows the printf, `exit_code: 1`, but `passed: true` because the fixture’s Go verification passed.

### P3 — Generic `AGENTS.md` seed does not cite `make check`

- **Repro**: `tb init --write-agents` on jq.
- **Expected**: tips aligned with the detected `test_cmd` (`make check`).
- **Actual**: generic harness text; `agent_docs.yaml` with `hints: []` (Makefile does not become a narrative hint).

### P3 — `verify` with a failing `build_cmd` can exit 0 with verdict `ESCALATE`

- **Repro**: `build_cmd: false`; `tb verify run --json`.
- **Expected**: exit ≠ 0 when verdict ≠ PASS.
- **Actual**: JSON `ESCALATE` / `build_passed: false` with exit 0 observed this session.

## What tb caught and what it let slip

**Caught**

- Drift of `rd.md` outside Markdown scope.
- Guard YAML + hardcoded (`thresholds.yaml`) with strict exit code.
- Verify with build + lint + `make check` + hidden without a folder.
- Init with Make toolchain and `AGENTS.md` seed.
- Graph search of touched symbols; root SkipDirs under `/tmp/target` fixed.
- Aggregated visual skipped when refs are missing.

**Let slip / did not help**

- Impact/callers did not guide the C patch.
- With `test_cmd: true`, verify PASS without exercising the jq suite (operator still decides).
- Bench “success” despite agent exit failure.
- Review did not semantically check acceptance criteria beyond heuristics/drift.
- Protecting the task file in `guard.yaml` blocks a useful verify loop.

## Interaction with existing instrumentation / onboarding

Repo **without** instrumentation on entry (prompt group). Observed onboarding:

| Artifact | Quality |
|----------|---------|
| `.toolbelt/config.yaml` via `tb init` | Adequate `test_cmd: make check`; `hidden_test_cmd` came empty on a clean init (better than round 1’s `go test`) |
| `AGENTS.md` via `--write-agents` | Created; generic toolbelt; mode 0600 |
| `.toolbelt/agent_docs.yaml` | Records sources; empty hints |
| `tb mem init` | 3 generic anti-cheat invariants |
| Reading `CLAUDE.md` | N/A (absent); code path exercised via seed |

Conclusion: bootstrap **improved** vs round 1 (toolchain + AGENTS), but the seeded AGENTS still does not document the native jq workflow (`autoreconf`/`configure`/`make check`).

## Comparison with round 1

| Aspect | Round 1 | Round 2 | Verdict |
|--------|---------|---------|---------|
| SkipDirs `/tmp/target` | 0 nodes (other repos) / jq used a subdir | 1064 nodes at root `target` | **better** |
| Init defaults | `go test ./...` | `make check` | **better** |
| AGENTS / `--write-agents` | nonexistent | seed OK | **better** |
| Scope `##` + `**` | fragile / `scope:` line | headings + `src/**/*.c` OK | **better** |
| Review FP `tests/hidden` | confirmed | absent in this run | **better** |
| guard.yaml / `--strict --json` | ignored / exit 0 | violation + exit 1 | **better** |
| hidden without folder | did not run | runs | **better** |
| build/lint in verify | absent | ran | **better** |
| visual skipped top-level | lied | `skipped: true` | **better** |
| C graph callers/impact | empty | still empty | **same** |
| Pushing full acceptance | weak | still weak | **same** |
| Task | #2414 numeric parse | #2813 slice assign | different (ok) |

Overall: **better** in concrete ways on the 12 fixes; C graph and completeness-harness bottlenecks remain.

## Improvement suggestions

1. **[verify/guard] P1** — Do not abort the test loop only because a mandate path is in `protected_paths`; or require opt-in / warning severity for paths declared in scope.  
2. **[bench] P1** — Fail the task (or expose `agent_ok`) when the agent process returns exit ≠ 0, regardless of the fixture.  
3. **[verify] P1** — Exit code ≠ 0 for `FAIL`/`ESCALATE` verdicts (even with `--json`).  
4. **[init/agents] P2** — Include the detected `verify.test_cmd` in the seeded `AGENTS.md` (`make check`, etc.).  
5. **[graph] P2** — Improve C `calls`/impact (`#include` + non-static symbols) — still the main hole in this repo.  
6. **[cli] P3** — With `--json`, silence Cobra Usage on domain errors (drift/review).  
7. **[docs/mem] P3** — Align `mem add` docs↔CLI or accept named flags.

## Metrics

| Metric | Round 2 | Round 1 (jq) |
|--------|---------|--------------|
| Session wall time (clone+toolchain+fix+tb) | ≈ 20–25 min | ≈ 25–30 min |
| Wall time of timed `tb` invocations only (1st batch) | ≈ 7.6 s summed; verify `make check` ≈ 7.3 s | ≈ 3.75 min span |
| Recorded `tb` invocations | ~30+ (21 in 1st batch @8b5a499 + retest @a46c295 + mcp/bench) | 45 |
| `tb` failures (exit ≠ 0) | several expected (drift, guard strict, init `--write-agents` on old binary, invalid flags) | 18 |
| `make check` baseline / post-fix | 9/9 PASS / 9/9 PASS | 9/9 / 9/9 (#2414) |
| Final evidence binary for the 12 fixes | `toolbelt@a46c295` | `toolbelt@c8aae04` |

Local artifacts (not committed in toolbelt or jq upstream): `/tmp/campo-jq-r2/evidence/`, clone `/tmp/target` with local commit `6dafa26` (fix #2813) — **not** sent to jq’s GitHub.
