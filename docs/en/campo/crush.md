[🇧🇷 Português](../../campo/crush.md) | 🇺🇸 English

# Field: charmbracelet/crush

Field study of `tb` (toolbelt @ `c8aae04`) using the public third-party repository **charmbracelet/crush** only as a local clone (`/tmp/target/crush`). No push, issues, PRs, or comments upstream.

## Repository

| Field | Value |
|-------|-------|
| Name | [charmbracelet/crush](https://github.com/charmbracelet/crush) |
| Language | Go (`go 1.27.4` in `go.mod`) |
| Size (shallow clone) | ~25 MiB disk; **904** `.go` files; ~**204 480** Go LOC |
| Existing AI instrumentation | `AGENTS.md` (root); `internal/ui/AGENTS.md`; `internal/cmd/stats/AGENTS.md`; `internal/oauth/callback/AGENTS.md`; `.agents/skills/` (`builtin-skills`, `shell-builtins`); `crush.json`; skills/hooks schema/docs |
| Toolchain | **Partially configured** |

Toolchain on this VM:

- `GOTOOLCHAIN=go1.27.4` downloaded and compiled the binary (`go build .` OK, ~1 m26 s the first time).
- `go test ./internal/filepathext` OK; `go test ./internal/skills` OK.
- `task` installed via `go install`; `task lint:log` OK after the fix.
- **Not installed**: `golangci-lint`, `gofumpt` (full lint `task lint` / `task fmt` not run).
- Full suite `go test ./...` / `task test` was **not** run end-to-end (high cost; outside the minimum scope of the chosen task).

## Chosen task

| Field | Value |
|-------|-------|
| What | Close a hole in `scripts/check_log_capitalization.sh`: the grep only matched `slog.Info/Error/...("msg")` and **not** `slog.*Context(ctx, "msg")`, even though `AGENTS.md` requires capitalization and `task lint:log` depends on that script. |
| Why | Real and small gap, with local repro and self-test; aligned with the repo’s AI instrumentation; maintainer-plausible (strengthens already-documented lint). |
| Issue | No dedicated issue; motivated by `AGENTS.md` § Build/Test/Lint + existing script. Issues read (#3366, #3389, #254, etc.) were too large or Windows-specific. |
| Completed? | **Yes**, on the local clone (branch `campo-local-log-lint`, commit `1f2974c`). **Not** sent upstream (forbidden). |
| Native checks | `./scripts/check_log_capitalization.sh` → exit 0; `./scripts/check_log_capitalization_test.sh` → OK; `task lint:log` → exit 0; `go build .` → exit 0. `golangci-lint` / `task test` not run. |

Implementation detail: regex with two alternatives — classic `slog.(Error\|Info\|Warn\|Debug\|…)\(["'][a-z]` and Context `slog.(Error\|Info\|Warn\|Debug)Context\([^,]+,[[:space:]]*["'][a-z]`. First attempt (only adding `*Context` to the name list) **failed** because the message is not the 1st argument; second attempt produced false positives; third passed the self-test.

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb version` (make build) | OK; `toolbelt c8aae04` | neutral | `2026-11-15T20:47:28Z`; commit/built filled via ldflags |
| `tb version` (without ldflags) | `0.1.0-dev`, commit/built `unknown` | hindered | `/tmp/tb-nolflags version` |
| `tb init` | created `.toolbelt/` + `config.yaml` mode **0600** | helped (bootstrap) / hindered (perms) | `stat` → `600 .toolbelt/config.yaml` |
| `tb mem init` | `invariants.yaml` 0600 + 3 anti-cheat rules | helped | includes `NUNCA modifique arquivos em tests/hidden/` |
| `tb mem add <regra>` | OK positional | helped | `inv-…` added |
| `tb mem add --type/--title` (docs) | **unknown flag: --type** | hindered | docs/cli.md vs CLI divergence |
| `tb mem list` / `check` | OK; retention 1.00 | neutral | 4 invariants at the end |
| `tb graph --enable index .` | 5136 nodes, 38724 edges, ~0.21 s | helped | indexed Go + markdown (AGENTS.md) |
| `tb graph stats/search` | OK; found scripts and AGENTS.md sections | helped | search `AGENTS`, `check_log` |
| `tb graph impact` without `--file` | exit 1 `--file é obrigatório` | hindered | UX without default on current diff |
| `tb graph impact --file scripts/check_log_capitalization.sh` | `(vazio)` | neutral | shell script without useful edges |
| `tb graph deps --file …` | `(vazio)` | neutral | same |
| Ticket/`rd.md` + Scope | YAML frontmatter works; `## Scope` bullets **not** parsed | hindered (UX) | `scope-missing` on review with markdown headings |
| `tb drift` (literal YAML scope) | OK, no out_of_scope | helped | `rd.md` frontmatter |
| `tb drift` with `scripts/*.sh` (YAML) | OK (glob via `filepath.Match`) | helped | globs **work** in the matcher |
| `tb review` | empty findings with correct YAML ticket | helped | fresh_context true, 3 goldfish |
| `tb review` on `tests/hidden/` commit | critical on hidden invariant | hindered (FP in the “create hidden” case) | exit 1 + guard protected_path |
| `tb guard check` on fix diff | violations null | helped | clean JSON |
| `tb guard` + custom `.toolbelt/guard.yaml` | **ignored** | hindered | we protected `scripts/check_log_capitalization_test.sh` in yaml; change not flagged |
| `tb guard` on `.toolbelt/thresholds.yaml` | protected_path error (hardcoded) | helped (anti-cheat) / hindered (inflexible) | |
| `tb verify run` (test_cmd=self-test) | PASS, 1 iter, ~0.03 s | helped | visible_passed true; hidden_available false |
| `tb verify` after touching thresholds | FAIL immediately; **tests did not run** | hindered | iterations=0 |
| `tb verify --ruler visual` (no manifesto) | PASS, `skipped: true` | neutral | skip_reason missing manifesto |
| `tb verify --ruler visual` (manifesto without refs) | PASS, **`skipped: false`**, screen `skipped: true` | hindered | false “not skipped” in root report |
| `verify.build_cmd` / `lint_cmd` in config | **not consumed** by `tb verify` | hindered | only `TestCmd`/`HiddenTestCmd` in code |
| `tb bench run` without `--agent` with `default_agent` set | exit 1 requires `--agent` or `--fake` | hindered | default_agent ignored |
| `tb bench run --fake` | OK 33.3% on toolbelt fixtures | helped | **did not** require `--fake-config` |
| `tb bench compare --fake` A/B | OK | helped | |
| `tb bench compare --agent …` | unknown flag `--agent` | hindered | confirmed |
| `tb mcp` tools/list + tools/call | OK; `tb_version` responded | helped | stdio JSON-RPC |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `Error: unknown flag: --agent`; help without the flag |
| `guard.yaml` ignored | **confirmed** | yaml protected the test script; `guard check` on fix diff → `violations: null`; only hardcoded paths in `internal/guard/guard.go` |
| outdated CLI docs | **confirmed** | `docs/cli.md` shows `tb mem add --type/--title`; real CLI is `tb mem add <regra>`; README still says “Fase 0” while roadmap lists Fase 2 |
| version shows `unknown` | **confirmed (partial)** | without ldflags: version `0.1.0-dev`, **commit/built = unknown**; with `make build`: ok |
| config files created 0600 | **confirmed** | `config.yaml` and `invariants.yaml` → mode 600 |
| visual ruler reports `skipped=false` when skipping | **confirmed** | with `screens.yaml` and missing ref: top-level `"skipped": false` but screen `"skipped": true` |
| `default_agent` ignored | **confirmed** | config `default_agent: echo AGENT-RAN {prompt}` + `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | diff on `.toolbelt/thresholds.yaml` → verify FAIL, iterations 0, tests not executed |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake` without `--fake-config` completed successfully; flag only changes command if provided |
| `mem add` positional | **confirmed** | usage `tb mem add <regra>`; `--type` unknown |
| Drift/Scope need literal paths (no globs) | **refuted (matcher); confirmed (markdown UX)** | YAML `scripts/*.sh` matches via `filepath.Match`; bullets under `## Scope` are **not** read (only same-line `Scope:` or YAML frontmatter) |
| `graph impact` needs `--file` | **confirmed** | `--file é obrigatório` |
| review false positives on the tests/hidden invariant | **confirmed** | commit only with `tests/hidden/ok.go` → critical finding + exit 1 |
| verify runs only partial checks and misses build errors | **confirmed** | config mentally accepts `build_cmd`/`lint_cmd` but `cmd/tb/verify.go` only applies `TestCmd`/`HiddenTestCmd`; broken build would not be exercised by default verify |
| tb does not push agent toward completing the whole task | **confirmed (in this study)** | 1st regex “fix” did not detect Context (self-test failed / context_exit=0); tb did not block the flow until I deliberately ran the self-test; review/drift do not demand acceptance checkboxes |

## New problems

### P1 — Scope parser ignores markdown lists under heading

- **Repro**: `rd.md` with `## Scope` + bullets `- path`; `tb review` / `tb drift`.
- **Expected**: scope filled from the list.
- **Actual**: `scope-missing` / empty scope; drift does not mark out_of_scope (because `len(scope)==0` disables the check).

### P1 — Visual report: `skipped=false` with all screens skipped

- **Repro**: `.toolbelt/screens.yaml` without `references`; `tb verify run --ruler visual --skip-unavailable --json`.
- **Expected**: `skipped: true` in the root report (or a distinct verdict).
- **Actual**: `"skipped": false`, `"verdict": "PASS"`, screens with `skipped: true`.

### P2 — First Context lint iteration easy to “pass” verify if test_cmd is weak

- **Repro**: regex that only adds `*Context` to the name group; `test_cmd` that only runs the script on a clean tree.
- **Expected**: verify require a regression that fails with lowercase Context.
- **Actual**: clean tree passes; hole only appears with a negative fixture (hence the self-test was essential — tb does not require it).

### P2 — `graph impact`/`deps` useless for shell scripts

- **Repro**: index crush; `impact --file scripts/check_log_capitalization.sh`.
- **Expected**: at least useful doc/mention edges or a clear “no nodes” message.
- **Actual**: `(vazio)` without diagnosis.

### P3 — user-created `guard.yaml` is 0644 while tb configs are 0600

- **Repro**: `cat > .toolbelt/guard.yaml`; compare with `config.yaml`.
- **Expected**: documented / uniform permission policy.
- **Actual**: mix of 0600 (tb) and 0644 (manual); yaml still ignored.

### P3 — MCP `tb_graph_*` listed without making clear in tools/list that the graph is off by default

- Tools appear; CLI requires `--enable`/`TB_GRAPH=on`. Call with graph off not exercised beyond listing.

## What tb caught and what it let slip

**Caught**

- Guard on hardcoded path `.toolbelt/thresholds.yaml` and `tests/hidden/*`.
- Scope drift/review when the YAML ticket is correct.
- Verify PASS on the lint self-test after the fix.
- Graph indexed `AGENTS.md` and located the touched scripts.
- Visual skip when there is no manifesto (`skipped: true`).

**Let slip / did not cover**

- Did not read `AGENTS.md` at `init`/onboarding time to suggest real `test_cmd`/`lint_cmd` (`task test`, `task lint:log`).
- Did not execute `build_cmd`/`lint_cmd` even when present in the edited yaml.
- Did not prevent the 1st incomplete regex fix (missing mandatory negative fixture).
- Custom `guard.yaml` ignored → false sense of configurable policy.
- Empty impact for `.sh` change.
- Root visual report lied with `skipped=false` when screens skipped.
- No `tb ticket` (Phase 3): “tickets with Scope” = manual `rd.md` file.

Compared with native: the shell self-test and `task lint:log` were the source of truth; tb verify was only as good as the `test_cmd` I configured.

## Interaction with existing instrumentation / onboarding

| Aspect | Observation |
|--------|-------------|
| `tb init` | Creates parallel `.toolbelt/`; **does not** import `AGENTS.md`, `.agents/skills`, `Taskfile.yaml`, or `crush.json`. |
| Duplication | Generic `mem init` invariants vs rules already in `AGENTS.md` (log capitalization, CGO, tests). Content not synchronized. |
| Conflict | tb `tests/hidden/` invariant vs natural flow of adding a hidden suite in tb’s own harness — review/guard punish what the product asks for. |
| Ignore | Rich crush rules (gofumpt, `task lint:log`, golden `-update`, mock providers) stay only in AGENTS; agent with tb is not pushed to read them. |
| Graph | **Reads indirectly**: indexes `AGENTS.md` and nested AGENTS as docs (sections/headings). Useful for search, not for normative onboarding. |
| What tb should import | (1) Build/Test/Lint commands from AGENTS/Taskfile → `verify.test_cmd` / lint; (2) explicit prohibitions (“NUNCA…”, octal file perms, capitalization) → `invariants.yaml`; (3) context file paths already listed by crush (`AGENTS.md`, `CRUSH.md`, …) → `mem`/`ctx`; (4) do not overwrite `.agents/` skills — reference them. |

## Improvement suggestions

1. **[P0][verify]** Honor `build_cmd` and `lint_cmd` (or document that they do not exist) and fail if the config declares ignored keys.  
2. **[P0][docs]** Align `docs/cli.md` / README (`mem add`, project phase, guard.yaml) with the real CLI.  
3. **[P0][guard]** Load `.toolbelt/guard.yaml` or remove the mention from docs; today it is a trap.  
4. **[P1][review/drift]** Parse markdown lists under `## Scope` / `## Escopo`; warn when scope is empty (instead of disabling drift).  
5. **[P1][verify/visual]** Propagate `skipped=true` in the root report if all screens were skipped.  
6. **[P1][bench]** Respect `bench.default_agent`; add `--agent` to `bench compare` or document only `--fake`/JSONL.  
7. **[P1][init/mem]** Onboarding: detect `AGENTS.md` / Taskfile and prefill verify + invariants (“already has AI instrumentation” group).  
8. **[P2][graph]** Explicit message when impact/deps are empty; optional script indexing as “task” linked to docs that cite them.  
9. **[P2][verify]** Require or generate a negative fixture for lint rulers (avoids “PASS” with residual hole).  
10. **[P2][version]** `go build` without ldflags should not report commit `unknown` without warning; or embed via `debug.ReadBuildInfo`.  
11. **[P3][config]** Revisit 0600 vs collaboration in `.toolbelt/` (gitignored?).

## Metrics

| Metric | Value |
|--------|-------|
| Wall time (instrumented tb session, evidence log) | ~**3 min** (`20:47:28Z` → `20:50:33Z`), excluding initial Go 1.27 download / crush build (~1 m26 s before) |
| Named tb invocations in the log | **18** CMD tags + several ad-hoc (init/mem/bench/mcp/version) ≈ **~30+** |
| Observed tb failures (exit 1) | **10** occurrences in evidence (includes expected probes: impact without file, compare --agent, thresholds, review-hidden, etc.) |
| tb crashes | **0** |
| MCP | tools/list + `tb_version` OK |
| Changes in toolbelt repo | **only** this file `docs/campo/crush.md` |
| Changes in crush | only local in `/tmp/target/crush` (no push) |
