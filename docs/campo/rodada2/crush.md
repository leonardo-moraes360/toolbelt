🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/crush.md)

# Field round 2: charmbracelet/crush

Field study ROUND 2 of `tb` (toolbelt @ `a46c295`) on the third-party repository **charmbracelet/crush**, group **with existing AI-agent instrumentation**. Local clone only (`/tmp/target/crush`). No push, issues, PRs, or interaction with upstream.

## Repository

| Field | Value |
|-------|-------|
| Name | [charmbracelet/crush](https://github.com/charmbracelet/crush) |
| Language | Go (`go 1.27.4` in `go.mod`) |
| Size (shallow clone) | ~25 MiB disk; **904** `.go` files; ~**204 515** Go LOC |
| Existing AI instrumentation | `AGENTS.md` (root); `internal/ui/AGENTS.md`; `internal/cmd/stats/AGENTS.md`; `internal/oauth/callback/AGENTS.md`; `.agents/skills/` (`builtin-skills`, `shell-builtins`); `crush.json`; skills/hooks schema/docs |
| Toolchain | **Partially configured** |

Toolchain on this VM:

- `GOTOOLCHAIN=go1.27.4`: `go build .` OK (~1 m16 s); `go test ./internal/stringext/` OK; `task` via `go install`; `task lint:log` OK.
- **Not installed**: `golangci-lint`, `gofumpt` (full `task lint` / `task fmt` not run).
- Full suite `go test ./...` / `task test` **not** run (cost; outside task scope).

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `stringext.Capitalize`: replace `cases.Title` (title-case every word, which turns `"API"` → `"Api"`) with uppercase of only the first rune, preserving the rest (acronyms and internal casing). Include unit tests. |
| Why | Real, small bug with a local repro (`Title("API")=="Api"`); call sites use Capitalize for error titles and UI labels (`internal/agent`, `internal/ui/...`). Maintainer-plausible. |
| Issue | No dedicated issue; motivated by inspecting `internal/stringext` + call sites. Large open issues (#4193 timeout, #4242 TMPDIR, etc.) left out. |
| Completed? | **Yes**, on the local clone (branch `campo2-local-capitalize`, commit `ca9566c`). **Not** sent upstream (forbidden). |
| Native checks | `go test ./internal/stringext/` → PASS; `./scripts/check_log_capitalization.sh` → exit 0; `task lint:log` → exit 0; `go build .` → exit 0. `golangci-lint` / `task test` not run. |
| Round 1 (comparison) | Capitalization lint for `slog.*Context` in `scripts/check_log_capitalization.sh` — **different task** from this round. |

## Verification of the 12 fixes

| # | fix | result | evidence |
|---|-----|--------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph --enable index /tmp/target` (basename=`target`) → **5183** nodes / 39091 edges (previously zeroed). Index under `/tmp/target/crush` → 5157 nodes. |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No dir: `hidden_test_cmd=false` + `test_cmd=true` → `hidden_available: true`, `hidden_passed: false`, verdict `ESCALATE`, exit 1. With a real cmd on the fix → `hidden_passed: true`. Init default is now `hidden_test_cmd: ""`. |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff touching a protected path: `--json` exit 0; `--strict --json` exit **1** with valid JSON on stdout. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `internal/stringext/string_test.go` → `protected_path` on the fix diff. Hardcoded `.toolbelt/thresholds.yaml` and `tests/hidden/*` continue. |
| 5 | init toolchain detection | **worked** | `go.mod` → `verify.test_cmd: go test ./...` (not a wrong generic default). |
| 6 | review `tests/hidden` by path | **worked** | Ticket only *mentioning* `tests/hidden/**` → no critical invariant finding. Real commit under `tests/hidden/ok.go` → `critical` + guard `protected_path`. |
| 7 | bench `default_agent` and `compare --agent` | **worked** | Without `--agent`, used `default_agent: echo AGENT-RAN {prompt}` (JSONL with `AGENT-RAN …`). `compare --agent` exists in help; `compare --config-a/--config-b --agent 'echo …'` exit 0. |
| 8 | review `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | `## Escopo` + bullets + `internal/stringext/**/*.go` → clean drift. Scope only `string.go` → `out_of_scope: string_test.go`. `**/*_test.go` kept the test in-scope and marked `string.go` out. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Verify PASS with `build_ran/passed` and `lint_ran/passed` true (`go test -c` + `check_log_capitalization.sh`). |
| 10 | init reads AGENTS.md/CLAUDE.md and `--write-agents` | **worked** (with caveat) | With `AGENTS.md`: wrote `.toolbelt/agent_docs.yaml` + hints; `--write-agents` **did not** overwrite. In a dir without AGENTS: `--write-agents` seeded a skeleton. Caveat: hints extracted large Go blocks as “commands”. |
| 11 | graph Python `calls` | **not applicable** | Go repo; Go `calls` edges OK (29566). Python extractor not exercised here. |
| 12 | aggregated visual skipped + CLI docs | **worked** | `screens.yaml` without refs → `"skipped": true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` for `mem add` aligned to positional (no longer `--type/--title`). |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb version` (make build) | OK `a46c295` | neutral | ldflags filled |
| `tb init` | `.toolbelt/` + detect Go + reads AGENTS.md | **helped** | `agent_docs.yaml`; test_cmd `go test ./...` |
| `tb init --write-agents` (with AGENTS) | does not overwrite | helped | empty `diff` on AGENTS.md |
| `tb init --write-agents` (without AGENTS) | seeds minimal | helped | `/tmp/noagents/AGENTS.md` created |
| `tb mem init/add/list/check` | OK; retention 1.00 | helped | 4 invariants |
| `tb mem add --type` | unknown flag | neutral | CLI docs already fixed; old `--type` flag only trips people who memorized round 1 |
| `tb graph --enable index` | 5157–5183 nodes | **helped** | Root SkipDirs fixed |
| `tb graph search Capitalize` | found `func:stringext.Capitalize` | helped | |
| `tb graph impact --file string.go` | lists UI/agent callers | helped | |
| `tb graph impact` without `--file` | exit 1 required | hindered (UX) | same as round 1 |
| `tb graph callers --function Capitalize` | 5 callers | helped | |
| Ticket `## Escopo` + bullets/`**` | parse OK | **helped** | contrasted with round 1 |
| `tb drift` / `tb review` | scope + completeness | helped / neutral | acceptance warning “pouco evidenciado” even with tests in the diff |
| `tb verify run` (build/lint/test/hidden) | PASS, 1 iter, ~0.84 s | **helped** | |
| `tb verify --ruler visual` | aggregated skipped=true | helped | |
| `tb guard` + `guard.yaml` | custom violations | **helped** | |
| `tb guard --strict --json` | exit 1 | helped | |
| `tb bench run` without `-t` | fails `bench/tasks` | hindered | YAML `tasks_dir` **not** applied; needs `-t`/`-f` |
| `tb bench run -t …` without `--agent` | uses default_agent | helped | |
| `tb bench compare --agent` | OK with `--config-a/b` | helped | |
| `tb mcp` tools/list + `tb_version` | OK | helped | stdio JSON-RPC; graph tools document `TB_GRAPH=on` |

## Problems that persist

| problem (round 1 / checklist) | status | evidence |
|-------------------------------|--------|----------|
| `graph impact` requires `--file` | persists | exit 1 `--file é obrigatório` |
| Config `.toolbelt/*` mode **0600** | persists | `config.yaml` and `agent_docs.yaml` → 600 |
| Manual `guard.yaml` **0644** vs tb configs 0600 | persists | `stat` → 644 |
| Docs/`mem add --type` in agent memory | mitigated | CLI/docs OK; `--type` still unknown (expected) |
| Verify only as good as configured `test_cmd` | persists (by design) | I had to set focused build/lint/test manually |
| Init does not prefill lint/build from Taskfile/AGENTS | partial | reads hints, but **does not** write `lint_cmd`/`build_cmd` into YAML |
| Full native suite / golangci not on VM | persists | not installed |

Round 1 problems **resolved** in this run: root SkipDirs; ignored guard.yaml; `--strict --json` exit 0; `default_agent` / `compare --agent`; Markdown Scope; build/lint in verify; visual `skipped=false`; review FP `tests/hidden`; hidden gated only by dir; init blind to AGENTS.md.

## New problems

### P2 — AGENTS.md “hints” extraction pulls Go code blocks

- **Repro**: `tb init` at the crush root with a rich `AGENTS.md`.
- **Expected**: command tips (`go test`, `task lint:log`, …).
- **Actual**: `agent_docs.yaml` includes whole functions (`CharmtonePantera`, mock test templates) as `hints`, mixed with useful commands.

### P2 — `config.yaml` `bench.tasks_dir` / `fixtures_dir` do not feed `tb bench run`

- **Repro**: set absolute paths in YAML; run `tb bench run -r 1` without `-t`/`-f`.
- **Expected**: use config dirs (like `default_agent`).
- **Actual**: `open bench/tasks: no such file or directory`; relative CLI defaults win.

### P3 — Review “completeness” warns on acceptance already covered by tests in the same commit

- **Repro**: `rd.md` with `- [x] go test ./internal/stringext/ passes` + diff with `string_test.go`; `tb review`.
- **Expected**: silence or positive evidence.
- **Actual**: warning `critério de aceite pouco evidenciado no diff`.

### P3 — Bench with `echo` as `default_agent` reported 100% on `bugfix-go`

- **Repro**: `default_agent: echo AGENT-RAN {prompt}` + `tb bench run --task bugfix-go`.
- **Expected**: verification failure if the agent does not edit the fixture.
- **Actual**: `passed: true` (output only echoed the prompt). Possible harness/fixture gap — not deeply investigated in this repo.

## What tb caught and what it let slip

**Caught**

- Callers/impact of `Capitalize` via the Go graph (UI + agent).
- Drift when Markdown scope was too narrow (`string_test.go` out).
- Custom guard on `string_test.go` and hardcoded on `tests/hidden` / `thresholds.yaml`.
- Verify with build + lint + test + hidden without a `tests/hidden/` folder.
- Visual honestly `skipped` in the root report.
- Presence of `AGENTS.md` in init (path + hint attempt).

**Let slip / did not cover**

- Did not automatically suggest `task lint:log` / `go build` as `lint_cmd`/`build_cmd` from AGENTS/Taskfile (only listed noisy hints).
- Did not prevent (or detect) title-case use in Capitalize *before* the task — the bug came from human inspection + new tests.
- Weak completeness warnings (noise, not a useful block).
- YAML `tasks_dir` ignored in bench until passing `-t`.

Compared with native: `go test ./internal/stringext/` + `task lint:log` + `go build` were the source of truth; tb verify reflected that **after** I configured the cmds.

## Interaction with existing instrumentation / onboarding

| Aspect | Observation |
|--------|-------------|
| `tb init` | Now **detects** `AGENTS.md` and writes `agent_docs.yaml`; correct Go toolchain. |
| `--write-agents` | Correct: does not overwrite crush; seeds only when absent. |
| Parse quality | Hints mix guide code samples with real commands — barely actionable for filling verify. |
| Skills `.agents/` | Still **outside** tb onboarding (not referenced). |
| mem vs AGENTS duplication | Generic `mem init` invariants still parallel AGENTS capitalization/CGO rules. |
| Graph | Indexes nested AGENTS (sections) and Go code with useful `calls`. |

## Comparison with round 1

| Dimension | Round 1 (`c8aae04`) | Round 2 (`a46c295`) |
|-----------|---------------------|---------------------|
| Task | lint `slog.*Context` | `Capitalize` sentence-case |
| SkipDirs `/tmp/target` | breaks (0 nodes if root=target) | **fixed** (5183 nodes) |
| Markdown Scope | `scope-missing` | **parses** `## Escopo` + bullets/`**` |
| guard.yaml | ignored | **honored** |
| `--strict --json` | exit 0 with violation | **exit 1** |
| build/lint in verify | dead | **run** |
| hidden without dir | ignored | **runs** cmd |
| default_agent / compare --agent | broken | **ok** |
| AGENTS.md in init | ignored | **read** (noisy hints) |
| visual skipped | lying root | **aggregated** |
| Review FP tests/hidden | yes | **no** (only real path) |
| Overall usefulness | fragile bootstrap | **better** harness; hint onboarding still weak |

Verdict: **better** in concrete ways on the 12 testable fixes in this Go repo; new friction is smaller (hints, YAML `tasks_dir`, completeness warnings).

## Improvement suggestions

1. **[P1][init]** Use AGENTS/Taskfile hints to **fill** `verify.lint_cmd` / `build_cmd` (e.g. `task lint:log`, `go build .`), not only list text.  
2. **[P1][init]** Filter hint extraction: short lines/commands; ignore long fences/code blocks.  
3. **[P1][bench]** Honor `config.yaml` `bench.tasks_dir` / `fixtures_dir` / `output_dir` as already done for `default_agent`.  
4. **[P2][review]** Completeness: if the diff touches `*_test.go` aligned with acceptance, do not emit a generic warning.  
5. **[P2][graph]** Default `impact` to `git diff` files when `--file` is omitted.  
6. **[P3][config]** Document or unify 0600 permissions vs manual files under `.toolbelt/`.  
7. **[P3][init]** Reference `.agents/skills/` in `agent_docs.yaml` when present (instrumented group).

## Metrics

| Metric | Round 2 | Round 1 (reference) |
|--------|---------|---------------------|
| Wall time (instrumented tb session) | ~**2 min 30 s** (`12:39:55Z` → `12:42:24Z`), excl. crush build (~1 m16 s) and toolchain download | ~**3 min** |
| tb invocations (`CMD:` in evidence) | **46** | ~30+ |
| tb failures (`EXIT=1`) | **14** (expected probes: impact without file, strict guard, narrow drift, hidden=false, bench without `-t`, etc.) | **10** |
| tb crashes | **0** | **0** |
| MCP | tools/list + `tb_version` OK | OK |
| Changes in toolbelt | **only** this file `docs/campo/rodada2/crush.md` | only `docs/campo/crush.md` |
| Changes in crush | local under `/tmp/target/crush` (no push) | local (no push) |
