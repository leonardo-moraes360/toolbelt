🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/verify-rulers.md)

# Spec: additional rulers for `tb verify`

Author: toolbelt · Status: Implemented (round 2) (slice 1: R0–R3, R5, R6; slice 2: R4) · Date: 2026-11-22 · Decision record: [ADR-008](../adr/008-verificacao-adversarial.md) (pluggable rulers) · Related: [ADR-005](../adr/005-anti-trapaca.md) (anti-cheat), [verificacao-visual.md](../verificacao-visual.md) (the visual ruler)

## Overview

`tb verify run` judges a change with one of two rulers: `tests` (build, lint, visible tests, hidden tests) or `visual`. The field study found the gap that matters: **verify says PASS while the project's own gate fails**. It adds three things:

- **`typecheck` step** in the `tests` ruler: a type checker is its own gate, not a build or a lint;
- **named checks** in the `tests` ruler: extra gates the user declares (a vulnerability scan, `clippy`, a license check);
- **`coverage` ruler**: line coverage at or above a threshold the change cannot lower.

It also reads verify's commands from the base ref, so the change under test cannot rewrite its own gate. It makes rulers composable (`--ruler tests,coverage`), fixes an unknown `--ruler` silently running `tests`, and wires all of it into `tb stack` and MCP.

## Current state (2026-11-22)

| Item | State |
|------|-------|
| Ruler selection | `cmd/tb/verify.go:50` and `internal/registry/parity.go:490`: `visual` goes to `runVisualVerify`, **anything else runs `tests`** (an unknown ruler is not an error) |
| Ruler abstraction | None; each ruler returns its own payload; `emitVerifyResult` is the only shared seam |
| `tests` steps | build → lint (each once, cached; a failure is deterministic: `ESCALATE`) → visible tests → hidden tests, per iteration |
| Trust of verify's commands | `test_cmd`, `lint_cmd`, … come from the working tree's `.toolbelt/config.yaml` (`config.Load()`), which guard does not protect: the change under test can rewrite `test_cmd: true` |
| Verdicts | `PASS`, `FAIL`, `ESCALATE`, `SKIPPED`; only `PASS` and `SKIPPED` exit 0 |
| `Result.metrics` | `map[string]string`, never filled |
| `VerifyConfig` | `test_cmd`, `hidden_test_cmd`, `build_cmd`, `lint_cmd`; the file-config merge is copied in three places (`cmd/tb/verify.go`, `parity.go`, `stack.go`) |
| Exit codes | `main.go` exits 1 on any error; 127 for a missing command comes from the result, not from an error |
| Node `typecheck` script | Detected by `tb init` as `build_cmd` (`internal/config/detect.go:134`), so it is reported as a build |
| `.toolbelt/thresholds.yaml` | A guard protected path (`internal/guard/guard.go:73`) and the subject of an anti-cheat invariant; **nothing reads it** |
| `tb stack` verify op | Always the `tests` ruler; no ruler parameter |
| Execution | `sh -c` with the execenv allowlist, not sandboxed (unchanged here) |

## Demand (field study)

| Need | Where |
|------|-------|
| Typecheck as a gate: "vitest PASS + verify PASS; `tsc` fails" | [excalidraw.md](../campo/excalidraw.md) (P0), [pydantic-ai.md](../campo/pydantic-ai.md) (composite ruler, P2) |
| Verify PASS without the project's lint/build/typecheck | [SINTESE.md](../campo/SINTESE.md), [rodada2/SINTESE.md](../campo/rodada2/SINTESE.md) C9, click, mcp-python-sdk, ky, ollama, crush |
| Security scans the local gate did not see (`govulncheck`, `clippy`) | [paprika.md](../campo/paprika.md) |
| Coverage | weak (requests, express, mcp-python-sdk scripts); kept small for that reason |
| Mutation testing | no repository used it ([SINTESE.md](../campo/SINTESE.md)); out of scope |

## Goals

- G1: a type checker failure fails verify, reported as `typecheck`, not as build or lint.
- G2: the user can declare extra gates by name, and each is reported on its own.
- G3: a coverage ruler whose threshold comes from the **base** ref, so the change under test cannot lower it.
- G4: rulers compose: `--ruler tests,coverage` runs both and the verdict is the worst of them.
- G5: no behavior change without new config: `tb verify run` with today's config and flags gives the same verdict and the same JSON keys. Every new key is `omitempty` and appears only when its step ran. The one intended change: a change that edits `verify.*` in `.toolbelt/config.yaml` is caught (R0).
- G6: the gate cannot be rewritten by the change it judges: verify's commands and thresholds come from the base ref.

## Non-goals

- Mutation testing, flaky-test detection, performance regression, API-compatibility checks.
- Detecting tests without assertions (ADR-005 `artificial_coverage`): coverage here is a floor, not proof of testing.
- Sandboxing verify's commands (they run as today).
- Pinning what base-pinned commands read: they can still run files the change edits (Makefile targets, `package.json` scripts, test files). R0 pins which commands run, not what they read; hidden tests and guard protected paths are the defense for that (G6).
- Running the base ref's suite to compare coverage (a "no drop" mode); the threshold is a fixed floor.

## Glossary

- **Step:** one command inside the `tests` ruler (build, typecheck, lint, check, visible, hidden).
- **Gate step:** a step that runs once per verify run and is cached across iterations (build and lint today; typecheck and checks here). Its failure is deterministic for the agent's loop: verify ends with `ESCALATE`.
- **Base thresholds:** `.toolbelt/thresholds.yaml` as committed at the base ref.

## Design

### R0. Verify's configuration comes from the base ref

- With `-c`: only the `-c` file and the defaults, exactly as before rulers; the repository's `.toolbelt/config.yaml` is not read at all (neither base nor working tree).
- Without `-c`: defaults < global user config < the repository's `.toolbelt/config.yaml` **as committed at the base ref** (`git show <baseSHA>:.toolbelt/config.yaml`). The working-tree file is never read for verify. This covers the `verify:` section and the `env:` section (`allow`/`pass`), so a change cannot grant its own tests secrets. The same holds for `.toolbelt/thresholds.yaml` (R4). Flags and MCP arguments still override, as today: they come from the caller, not from the change.
- When the working tree's `verify:` or `env:` section differs from the base file's (uncommitted edits, a new uncommitted file, or edits committed on the branch), verify adds a warning naming the ignored keys: `ignored .toolbelt/config.yaml keys that differ from the base ref: verify.test_cmd (verify reads the base ref)`. The working-tree file compared is the one at the repository top level, also when verify runs from a subdirectory.
- A base ref that commits `.toolbelt/config.yaml` as a symlink (tree mode `120000`) is not followed: `config_source: "none"` and the warning `symlinked .toolbelt/config.yaml at the base ref is not followed; using the global config`. A malformed file at base aborts verify (fail-closed). With `-c` the repository file is not read, so neither applies.
- `overrides` (sorted, `omitempty`) lists every verify key whose effective value came from a flag or MCP argument and differs from what the trusted config (base/global/`-c`) set, plus skip toggles that disabled a configured step (`test_cmd`, `no_typecheck`, `skip_checks:govulncheck`, `no_checks`). Text mode (`tb verify run`, `tb stack`) prints `Warning: overrides applied: …` to stderr.
- `config_source` appears only outside the normal case: omitted when the base file was used (and with `-c`), `"none"` when the base ref has no file (global config and defaults only). So normal runs print the same JSON keys as before (G5).
- Guard compares the `verify:` section of `.toolbelt/config.yaml` at base and head. A difference is an error finding (`verify_config_changed`) that aborts verify like a protected-path hit, with the changed keys listed. With `-c` pointing to a file outside the repository top level (`filepath.Abs` + `EvalSymlinks`), the repository file is not read at all, so the change cannot affect the run: verify does not abort on `verify_config_changed` and a PR that migrates the verify config can reach PASS. The finding stays in guard's report and in the `guard` section of the result, and the `guard` step of `tb stack` still fails on it (a human must see it). A `-c` file inside the repository can be edited by the change itself, so verify aborts as without `-c`. `-c` is a persistent root flag, so an MCP server started with `tb -c <file> mcp` takes the same path (a human launched it); without `-c` MCP aborts. Other sections of the file stay unrestricted. A symlink at base or head is compared by its target text; any change is a finding listing `verify` (it cannot be parsed). A malformed file at base is the warning finding `verify_config_unreadable`, so guard and `-c` runs keep working; a malformed file at head stays an error finding.
- `--base` is chosen by the caller. An agent can commit a lowered threshold or a rewritten command and pass `--base HEAD`. The docs therefore say CI must pin the base (the PR's merge base), and verify warns (`base resolves to head`) when the base SHA equals the resolved head SHA. A `--base` (or MCP `base`) that points inside the change is not detected beyond base == head; CI must pin the merge base.
- A malformed global config (or `-c` file) aborts verify (fail-closed) instead of being skipped silently.
- This also closes the existing hole for `test_cmd`/`lint_cmd`/`build_cmd`/`hidden_test_cmd` and `env.pass`. It is a behavior change for runs whose change edits those keys, and for users who edited `verify:` or `env:` keys without committing them (now ignored, with the warning).

### R1. Ruler selection

- `--ruler` (MCP `ruler`) takes a comma-separated list of `tests`, `coverage`, `visual`. Default `tests`.
- An unknown name fails before anything runs: `invalid ruler "x": tests, coverage or visual`, exiting with the usage-error code 2 (main gains a typed usage error mapped to 2; other errors keep 1). Today it silently runs `tests`; this is a deliberate fix.
- `visual` cannot be combined with others yet (it ignores base/head and guard): `visual` with another ruler is an error.
- With several rulers, guard, ref resolution and the head worktree run **once**, in an orchestrator, and each ruler gets the same workdir (R6). Then `tests` runs, then `coverage`.
- Each ruler result has a `verdict` and, when `SKIPPED`, a `skip_kind`: `not_applicable` (the ruler has nothing to judge here, e.g. coverage with no threshold anywhere) or `unavailable` (it should judge but cannot, e.g. a browser missing). Composition: the worst of `ESCALATE` > `FAIL` > `SKIPPED(unavailable)` > `PASS`; a `not_applicable` ruler is ignored, and if every ruler is `not_applicable` the result is `SKIPPED(not_applicable)`. `--dry-run` gives each ruler's plan and the top-level `dry_run: true`, with no verdict merging.
- Exit code: 0 for `PASS` or `SKIPPED`; otherwise the first non-zero code in ruler order (127 for a missing command, 1 for the rest).

### R2. `typecheck` step (`tests` ruler)

- New config `verify.typecheck_cmd`, flags `--typecheck-cmd` and `--no-typecheck`, MCP `typecheck_cmd` / `no_typecheck`, and the same in `tb stack`.
- It is a gate step, run after build and before lint, once per run.
- JSON: `typecheck_ran` and `typecheck_passed`, both `omitempty` (unlike `lint_ran`, which is always present, so G5 holds), and the failure excerpt in `reason`, as lint does today.
- `tb init` detection (conservative, only when the tool resolves on `PATH` or through the package manager):
  - Node: a `typecheck` script becomes `typecheck_cmd` (`<pm> run typecheck`) instead of `build_cmd`; without the script, a `tsconfig.json` and a local `tsc` give the local `tsc` through the package manager (`npm exec -- tsc --noEmit`, `pnpm exec tsc --noEmit`, `yarn tsc --noEmit`, `bunx tsc --noEmit`).
  - Python: `mypy` or `pyright` configured in `pyproject.toml` (`[tool.mypy]`, `[tool.pyright]`) or their config files, and the tool on `PATH`. The current ruff → mypy lint fallback stops using mypy as lint.
  - Go and Rust: none (the build already type-checks).
- Existing configs keep working: a `build_cmd` that is a typecheck script stays a build until the user moves it. Re-running `tb init` changes what it writes for these repositories (`typecheck` moves from `build_cmd` to `typecheck_cmd`; mypy stops being the Python lint fallback). The affected tests are in `internal/config/detect_test.go` and `cmd/tb/init_json_test.go`, and the CHANGELOG says so.

### R3. Named checks (`tests` ruler)

- New config list:

  ```yaml
  verify:
    checks:
      - name: govulncheck          # [a-z0-9-]{1,32}, unique
        cmd: govulncheck ./...
      - name: clippy
        cmd: cargo clippy --all-targets -- -D warnings
  ```

- Each check is a gate step, run after lint, in order, once per run. Exit 0 passes; anything else fails the check, which ends verify with `ESCALATE` like any gate step.
- Flags: `--check name=cmd` (repeatable, adds or overrides one), `--skip-check name` (repeatable; renamed from `--no-check` in 1.0, which stays as a hidden deprecated alias until 2.0), `--no-checks`. MCP: `checks` (array of `{name, cmd}`), `skip_checks` (array of names) and `no_checks` (boolean, all). `tb stack` the same.
- JSON: `checks: [{name, passed, reason?}]`, `omitempty`.
- A missing executable fails fast with exit 127, like the other commands. A check cannot mark itself non-blocking or "unavailable" in Phase A: a scan that needs the network (govulncheck) belongs in CI, or is skipped locally with `--no-check` (Q1).
- `tb init` suggests, never adds silently: with `--write-checks`, it adds `govulncheck` (Go, if on `PATH`) and `clippy` (Rust, if `cargo clippy -V` works). Without the flag it prints the suggestion.

### R4. `coverage` ruler

- **Measurement:** the ruler runs `verify.coverage_cmd` in the head worktree, then reads the total line coverage:
  - Go (default when `go.mod` exists and `coverage_cmd` is empty): `go test -coverprofile=<tmp> ./...`, then the statement-weighted total from the profile, computed in Go (no `go tool cover` parsing).
    - Weighting: each block counts its statements once. Blocks are deduplicated by `file:startLine.startCol,endLine.endCol` (a block that appears several times, as with `-coverpkg`, is covered if any copy has a count > 0). Total = covered statements / all statements, the same as `go tool cover -func`'s total.
    - Packages without tests appear in the profile with 0% (Go ≥ 1.22) and count, as in `go tool cover`.
  - Generic: `coverage_cmd` plus `verify.coverage_pattern`, a regular expression with exactly one capture group matched against the command's output; the **last** match is the percentage, which must parse as a number in [0, 100]. Both are checked at config load. Examples: `TOTAL.*\s(\d+(?:\.\d+)?)%` for coverage.py (line coverage); `All files\s*\|\s*[\d.]+\s*\|\s*[\d.]+\s*\|\s*[\d.]+\s*\|\s*([\d.]+)` for istanbul's `% Lines` column. The metric is whatever the tool reports (lines for most, statements for Go), and the report names it.
  - No toolchain and no `coverage_cmd`: `SKIPPED(not_applicable)` when there is no threshold either; when a threshold exists (base file or `--min-coverage`), `FAIL` with `reason: "coverage: threshold set but no coverage_cmd for this repository"`, so a configured floor never passes silently.
- **Threshold:** `coverage.min_percent` (a number in [0, 100]) in **base thresholds** (`.toolbelt/thresholds.yaml` read from the base ref with `git show <base>:.toolbelt/thresholds.yaml`, like guard reads `guard.yaml`).
  - The worktree's copy is never read: a change that edits the file is already blocked by guard, and reading from base makes the edit pointless even with `--warn-only`.
  - `--min-coverage N` (MCP `min_coverage`) may only **raise** it: the effective threshold is `max(base, N)`. With no base file, `N` is the threshold.
  - No threshold at all: `SKIPPED(not_applicable)` with `reason: "coverage: no threshold"` (it does not demote a combined `PASS`, R1).
  - A base file that does not parse, or a `min_percent` outside [0, 100], is an error (verify aborts), like a malformed `guard.yaml`; it never falls back to "no threshold". Schema: `version: 1`, `coverage: {min_percent: <number>}`; unknown keys are an error.
- **Verdict:** `PASS` when coverage ≥ threshold, else `FAIL` (not `ESCALATE`: the agent can add tests). A failing `coverage_cmd` (tests fail) is `FAIL` with the excerpt.
- **JSON:** `coverage: {percent, threshold, threshold_source: "base"|"flag", cmd}` and `metrics["coverage_percent"]`.
- **Cheating rules:** `.toolbelt/thresholds.yaml` stays a protected path, and the `verify:` keys that choose the commands are compared by guard (R0); ADR-005's scenario "modify benchmark threshold" maps here. Coverage-padding without assertions is out of scope (non-goal).

### R5. `tb stack` and MCP

- `tb stack verify` gains `--ruler` (same list; `visual` refused there), `--typecheck-cmd`, `--no-typecheck`, `--check`, `--no-check(s)`, `--coverage-cmd`, `--min-coverage`; MCP `tb_stack` the same fields.
- The three copies of the file-config merge (`cmd/tb/verify.go`, `parity.go`, `stack.go`) become one function in `internal/verify`, so the new keys are read the same way everywhere.

### R6. Internal shape

- An orchestrator in `internal/verify` resolves refs, loads the base config (R0), runs guard and creates the head worktree once, then calls each ruler with that shared state: `Ruler` has `Name()` and `Run(ctx, *RunState) (*RulerResult, error)`, where `RunState` carries the config, the workdir and the refs. `tests`, `coverage` and `visual` implement it (`visual` keeps ignoring base/head, which is why it cannot be combined yet).
- `emitVerifyResult` prints one ruler's payload unchanged (G5) or, for several, `{verdict, rulers: {tests: …, coverage: …}}`.
- Gate steps share one runner (lint, typecheck and each check), so caching, `ESCALATE` and excerpts behave the same.

## Acceptance criteria

1. `tb verify run` with an existing config and no new flags gives the same verdict and JSON keys as before (golden test on a fixture repository).
2. `--ruler foo` exits 2 before running anything; `--ruler visual,tests` is refused.
3. A change that edits `verify.test_cmd` or `verify.coverage_cmd` in `.toolbelt/config.yaml` is reported by guard (`verify_config_changed`) and verify aborts, except with a `-c` file outside the repository, where the finding is still reported but verify proceeds; the base value is the one that would run. An uncommitted edit (also with base == head) is ignored with a warning naming the keys; with no base file the report says `config_source: "none"`.
4. A repository whose tests pass and whose `typecheck_cmd` fails ends `ESCALATE` with `typecheck_passed: false`; the typecheck runs once across iterations.
5. `tb init` on a Node fixture with a `typecheck` script writes it as `typecheck_cmd`, not `build_cmd`; with only `tsconfig.json` and a local `tsc`, it writes the package manager's form (`npm exec -- tsc --noEmit`, `pnpm exec tsc --noEmit`, `yarn tsc --noEmit`, `bunx tsc --noEmit`).
6. Two checks, one failing: `ESCALATE`, `checks` lists both with the failing one's excerpt; `--no-check` skips one; a missing executable exits 127.
7. `--ruler coverage` on a Go fixture computes the same total as `go tool cover -func` (within 0.1), also with `-coverpkg` duplicates; below the base threshold → `FAIL`; at or above → `PASS`.
8. A change that lowers `coverage.min_percent` in `.toolbelt/thresholds.yaml` does not lower the threshold used (base is read) and is reported by guard.
9. `--min-coverage` below the base threshold has no effect; above it raises the threshold (`threshold_source: "flag"`).
10. `--ruler tests,coverage`: worst verdict; with no threshold anywhere, a passing `tests` gives `PASS`; with a threshold but no coverage command, `FAIL`.
11. `tb stack verify --ruler tests,coverage` and MCP `tb_verify_run` / `tb_stack` with the new fields match the CLI.
12. The generic `coverage_pattern` takes the last match; an output with no match is `FAIL` with `reason: "coverage: pattern did not match"`; a pattern without exactly one group is a config error.
13. A malformed base `thresholds.yaml` aborts verify; it is not treated as "no threshold".
14. `--dry-run` lists typecheck, each check and the coverage command in the plan; the multi-ruler JSON shape has a golden test; the MCP schemas of `tb_verify_run` and `tb_stack` list the new fields (registry parity test).

## Docs to update with the implementation

README (roadmap item, `tb verify` row), `docs/cli.md` (verify flags, rulers table, init detection table, stack flags), ADR-008 (rulers list), CHANGELOG, and the pt-BR mirror of each.

## Open questions

- Q1: should a check be able to declare itself non-blocking (`warn: true`) or "unavailable" (no network) so a scan reports without failing? Proposed: no in Phase A; use `--no-check` locally and CI for the scan.
- Q2: branch coverage for Go (`-covermode=atomic` gives statements only). Proposed: statements only.
