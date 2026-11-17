[🇧🇷 Português](../../campo/ollama.md) | 🇺🇸 English

# Field study: ollama/ollama

Study conducted on 2026-11-15 with `tb` built from this repo (`make build` → `toolbelt c8aae04`). Local shallow clone at `/tmp/target/ollama`. **No interaction** with upstream (no push, issue, PR, or comment).

## Repository

| Field | Value |
|-------|-------|
| Name | [ollama/ollama](https://github.com/ollama/ollama) |
| Language | Go (~1097 `.go`), TS UI (~50), native C/C++ (llama/MLX) |
| Size | ~80 MiB shallow clone; ~1657 files (excl. `.git`); Go 1.26.10 in `go.mod` |
| Existing AI instrumentation | `AGENTS.md` (build: cmake + `go build`/`go run`); `CLAUDE.md` (only points at `AGENTS.md`); Claude/Copilot integration docs under `docs/integrations/` (product, not an agent harness) |
| Toolchain | **Partial.** `GOTOOLCHAIN=auto` downloaded Go 1.26.10; `go test ./api/` and `go test ./cmd/` OK. Configured CMake failed: linker missing `libstdc++` (`cannot find -lstdc++`); `ninja` absent. Full native build (`cmake -B build && cmake --build`) **not** possible. `golangci-lint` absent. GPU backends not attempted. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Avoid failure `flag accessed but not defined: verbose` when running a model via `ollama launch` (cobra context without the flag). Helpers `verboseEnabled` / `ensureVerboseFlag`; `/set verbose` and `/set quiet` register the flag on demand; 3 unit tests. |
| Why | Real open bug, reproducible, small scope in a pure Go package with tests; maintainer already saw a similar unmerged PR. |
| Issue | https://github.com/ollama/ollama/issues/14654 (related unmerged PR: https://github.com/ollama/ollama/pull/14701) |
| Completed? | **Yes, locally** at commit `a36cdca` (local clone only). Not sent upstream. |
| Native test/lint/build | `go test ./cmd/ -count=1` → **ok**. Project lint: no local script + no golangci. Native cmake build: **failed** (libstdc++/ninja). Main binary `go build` not required for the fix; package `cmd` compiles via the tests. |

Files changed on the clone: `cmd/cmd.go`, `cmd/interactive.go`, `cmd/cmd_test.go`. Ticket `rd.md` with YAML frontmatter (`scope`, `base_commit`, `acceptance`).

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` | OK (`c8aae04` with `make build`; `0.1.0-dev` + `commit: unknown` without ldflags) | neutral | 0.005 s; `go build` without ldflags → unknown |
| `tb init` | OK; created `.toolbelt/` | helped | config + vault + logs + bench; **did not** read `AGENTS.md` |
| `tb mem init/list/add/check` | OK | helped | 5 invariants; positional `mem add <regra>` works; docs `--type` fails |
| Ticket/`rd.md` + Scope | OK after YAML frontmatter | helped / hindered | Markdown `## Scope` is not enough; needs YAML as in `LoadTicket` |
| `tb graph --enable index cmd/` | OK (1138 nodes) | helped | `index --path` (docs) is an unknown flag; positional works |
| `tb graph rebuild` | OK (8924 nodes, ~1.05 s) | helped | large repo is indexable |
| `tb graph search/impact` | OK with `--json` / after index | neutral / hindered | `impact` requires `--file`; text sometimes `(vazio)` before rebuild; search does not find the symbol until reindex |
| `tb graph callers --file` | Error | hindered | docs ask for `--file`; CLI rejects (`unknown flag: --file`) |
| `tb verify run` (test_cmd=`go test ./cmd/`) | PASS | helped | 1.2 s; `hidden_available: false` still PASS |
| `tb verify --ruler visual` | PASS + skip | neutral | JSON `skipped: true`, reason manifesto absent; Chrome present |
| `tb drift` | OK on literal scope | helped | 0.01 s; `cmd/**` also works |
| `tb review` | OK with warnings | hindered | 3 completeness warnings even with tests in the diff (“weakly evidenced” acceptance) |
| `tb guard check` | OK hardcoded | helped / hindered | detects `tests/hidden` and `.toolbelt/thresholds.yaml`; **ignores** custom `.toolbelt/guard.yaml` |
| `tb bench run --fake` | OK (9 tasks, 33.3%) | helped | used toolbelt’s own tasks; ollama has no fixtures |
| `tb bench compare` A/B `--fake` | OK | helped | graph-off vs graph-on |
| `tb bench compare --agent` | Error `unknown flag` | hindered | confirmed |
| `tb mcp` + `tools/list` | OK | helped | tools: version, echo, env, run, graph_* |
| Native checks vs tb | `go test ./cmd/` OK | — | tb verify aligned when `test_cmd` points at the right package; with `test_cmd=./format/` and broken `cmd` → false PASS |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `--help` without the flag; `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmed** | manifesto with protected `cmd/cmd.go` → `violations: null` on a diff that changes that file; `NewChecker` uses hardcoded paths |
| outdated CLI docs | **confirmed** | `docs/cli.md`: `tb serve`, `tb mem add --type`, `graph index --path`, `graph callers --file` ≠ real CLI (`tb mcp`, positional add, positional index, callers without `--file`) |
| version shows `unknown` | **confirmed (partial)** | without ldflags: `commit: unknown`, `built: unknown`; with `make build`: commit hash |
| config files created 0600 | **confirmed** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → `600` |
| visual ruler reports `skipped=false` when skipping | **refuted** | JSON: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`; text: “Skip gracioso…” |
| `default_agent` ignored | **confirmed** | config `default_agent: echo-agent {prompt}` but `tb bench run` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | commit with `.toolbelt/thresholds.yaml` → verify `FAIL`, `iterations: 0`, protected_path violation |
| default fake agent needs `--fake-config` | **refuted (in this use)** | `tb bench run --fake` without `--fake-config` finished 9 tasks |
| `mem add` positional | **confirmed** | CLI: `tb mem add <regra>`; docs still show `--type/--title` flags |
| Drift/Scope need literal paths (no globs) | **partially refuted** | `cmd/**`, `cmd/`, `cmd/*` OK; generic recursive glob `**/*_test.go` only matches 1 level (Go `filepath.Match`) |
| `graph impact` needs `--file` | **confirmed** | without flag: `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | **not reproduced on this diff** | `--standards invariants,scope` → `findings: []`; false positives observed on **completeness**/acceptance, not hidden |
| verify runs only partial checks and misses build errors | **confirmed (conditional)** | with `test_cmd=go test ./format/` and a syntax error in `cmd/cmd.go` → verify **PASS**; with `./cmd/` the same error → **ESCALATE** |
| tb does not push agent toward completing the whole task | **confirmed (observational)** | tb did not point at `AGENTS.md`/cmake, did not require native build, did not suggest registering the flag in `LaunchCmd`; review asked for acceptance “in the diff” instead of guiding closure; the whole workflow was driven manually |

## New problems

1. **P1 — `tb guard check --strict --json` does not fail the exit code**  
   - Repro: diff that touches `tests/hidden/*`; `tb guard check BASE..HEAD --strict` → exit 1; same with `--json` → exit **0** (JSON still lists violations).  
   - Expected: `--strict` fails regardless of `--json`.  
   - Actual: early `return` on the JSON branch before evaluating `--strict` (`cmd/tb/guard.go`).

2. **P2 — Text output of `graph impact`/`search` can show `(vazio)` with a populated graph (or pre-rebuild)**  
   - Repro: after only `index cmd/`, text `impact --file cmd/cmd.go` → `(vazio)`; with `--json` or after `rebuild` there are nodes.  
   - Expected: same useful information in text and JSON.

3. **P2 — `graph search` does not reflect new code until reindex; result path can drop the directory**  
   - Repro: after the fix, without reindex → search `verboseEnabled` empty; after `index cmd/` finds `func:cmd.verboseEnabled` with path `cmd.go` (not `cmd/cmd.go`).  
   - Expected: canonical path and guidance to reindex after changes.

4. **P2 — Docs vs CLI in several places beyond those already listed**  
   - `tb graph callers` docs with `--file`; real help only `--function`/`--depth`.  
   - `tb serve` documented; binary exposes `tb mcp`.

5. **P3 — `tb verify` PASS with `hidden_available: false` and `hidden_passed: false`**  
   - Repro: config without a real hidden suite → verdict PASS.  
   - Expected: explicit warning or degraded verdict when hidden is unavailable.

6. **P3 — user-created `guard.yaml` is 0644 while `tb` configs are 0600**  
   - Permission inconsistency; manifesto is still ignored (known P0).

## What tb caught and what it let slip

**Caught**
- Scope drift when the ticket pointed at wrong paths (`format/**` vs changes in `cmd/`).
- Hardcoded protected paths (`tests/hidden/*`, `.toolbelt/thresholds.yaml`) and verify abort in those cases.
- Compile/test failure when `test_cmd` covers the broken package.
- Graph indexing at scale (8k+ nodes) and useful `impact`/`deps` after rebuild.
- Operational MCP `tools/list`.

**Let slip / weakened**
- Custom `guard.yaml` (protect `cmd/cmd.go`) completely ignored.
- Syntax error in `cmd` with verify pointing only at `./format/` → PASS.
- Review completeness: marked acceptance criteria as “weakly evidenced” despite helpers + tests in the diff (noise).
- Did not read `AGENTS.md` (cmake/libstdc++/ninja) — an agent without tb would also need to read it, but tb neither imports nor summarizes that in onboarding.
- No separate native lint/build ruler; only whatever is in `test_cmd`.
- `bench compare` without `--agent` blocks A/B with a real agent via compare alone.

## Interaction with existing instrumentation / onboarding

- **`AGENTS.md` / `CLAUDE.md`**: tb **does not read, import, conflict with, or mirror** them. No references in toolbelt `cmd/` or `internal/`. `tb init` only creates `.toolbelt/{config,vault,logs,bench}` + then `mem init` → `invariants.yaml`.
- **Duplication**: tb anti-cheat invariants (`tests/hidden`, thresholds) are generic and **do not** cover ollama build rules (parallel cmake, `go run . serve`, CGO).
- **Onboarding**: does not generate a harness-style progress log / feature list; does not create a ticket from issues; does not suggest `verify.test_cmd` from `AGENTS.md` or Makefile.
- **What tb should import**: contents of `AGENTS.md`/`CLAUDE.md` (and links to `docs/development.md`) as build facts/mandate; detect `go test`/`cmake` commands; optionally map issue labels → ticket with Scope.

## Improvement suggestions

1. **[P0][guard]** Honor `.toolbelt/guard.yaml` (merge with hardcoded defaults) — area `guard`.  
2. **[P0][docs]** Align `docs/cli.md` / README with the real CLI (`mcp`, `mem add`, graph flags) — area `docs`.  
3. **[P1][guard]** Apply `--strict` on the `--json` branch too — area `guard` CLI.  
4. **[P1][bench]** Accept `--agent` on `bench compare` (or inherit `default_agent`) — area `bench`.  
5. **[P1][bench]** Honor `bench.default_agent` when `--agent`/`--fake` are omitted — area `bench`/`config`.  
6. **[P1][verify]** “Repo checks” mode: beyond `test_cmd`, optional `build_cmd`/`lint_cmd`; fail or warn if hidden is unavailable — area `verify`.  
7. **[P1][mem/ctx]** `tb init` / `tb ctx`: import `AGENTS.md`/`CLAUDE.md` into the vault + suggest `verify.*` — area `mem`/`init`.  
8. **[P2][review]** Completeness: treat test/symbol names in the diff as acceptance evidence — area `review`.  
9. **[P2][graph]** UX: “empty graph — run index” message; canonical paths; callers with `--file` or corrected docs — area `graph`/`docs`.  
10. **[P2][config]** 0640/0644 permissions for non-secret configs; document 0600 — area `config`.  
11. **[P2][router/ux]** Task checklist (native build, lint, acceptance) to push full closure — area `onboarding`/`ticket`.

## Metrics

| metric | value |
|--------|-------|
| Study wall time | ~237 s of instrumentation + ~4 min of Go tests/deps on the first run (order of **~6–10 min** total field session) |
| Recorded `tb` invocations | **59** command blocks in the evidence log |
| tb failures (exit ≠ 0) | **13** (mostly intentional probes: nonexistent flags, forced drift, guard strict, compare --agent, graph disabled, etc.) |
| Sum of measured tb times | ~**9.7 s** (excludes native `go test` and module download) |
| Local commits on the ollama clone | feature `a36cdca` + discarded commits for guard/verify repro |
| Artifact in this repo | only this file at `docs/campo/ollama.md` |
