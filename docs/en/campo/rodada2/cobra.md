[🇧🇷 Português](../../../campo/rodada2/cobra.md) | 🇺🇸 English

# Field study (round 2): spf13/cobra

Group: **without AI-agent instrumentation**. Work only on a local clone (`/tmp/target/cobra`). No interaction with upstream (no push, issue, PR, or comment).

`tb` used: local binary from `/agent/repos/toolbelt` (`make build` from `origin/main` @ `a46c295` with ldflags).

---

## Repository

| Field | Value |
|-------|--------|
| Name | [spf13/cobra](https://github.com/spf13/cobra) |
| Language | Go (`go 1.15` in `go.mod`) |
| Size | shallow clone ~**1.3 MiB**; **36** `.go` files; ~**16.8 kLOC** Go (`wc` on sources) |
| Existing AI instrumentation | **None** on the upstream tree (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot/Windsurf/Aider: absent). `tb init --write-agents` seeded `AGENTS.md` locally. |
| Toolchain | **Partially ok**: `go test ./...` PASS; `go build ./...` PASS; `go vet ./...` PASS; `gofmt` ok; `golangci-lint` **v2.1.6** installed (v1 rejected `.golangci.yml` v2) → **1 pre-existing issue** `nolintlint` in `completions.go`; Docker/`addlicense` from CI **not** configured; no multi-Go matrix |

---

## Chosen task

| Field | Value |
|-------|--------|
| What | Include `CommandPath()` in error messages from `ExactArgs` / `MinimumNArgs` / `MaximumNArgs` / `RangeArgs` (e.g. `command "root child" accepts 1 arg(s), received 2`) |
| Why | Small feature, aligned with the style of `NoArgs`/`OnlyValidArgs` (already cite the path); natural tests; **different** from round 1 |
| Issue | None (local choice; no issue opened upstream) |
| Completed? | **Yes, locally only** (commit `e2643be` on the clone; **not** pushed to GitHub) |
| Native tests/lint/build | `go test ./...` → **PASS** (~0.2–1.1 s); `go build ./...` → **PASS**; `gofmt -l` clean; `golangci-lint run ./...` → **FAIL** pre-existing (`completions.go`), not introduced by the change |
| Round 1 (comparison) | Round 1: `SetErrPrefix("")` (issue [#2226](https://github.com/spf13/cobra/issues/2226)). This round: command path in positional arg-count errors |

Implementation: `args.go` + asserts in `args_test.go` + `TestExactArgsErrorIncludesCommandPath`.

---

## Verification of the 12 fixes

| # | fix | worked / did not work / not tested / not applicable | evidence |
|---|-----|-----------------------------------------------------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph --enable index /tmp/target` → **463 nodes / 2103 edges** (before: 0 nodes when the root was named `target`) |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | no dir + `hidden_test_cmd: "echo HIDDEN_RAN && false"` → `hidden_available: true`, verdict **ESCALATE**; with `echo HIDDEN_OK` → **PASS** / `hidden_passed: true` |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | `tb guard check BASE..HEAD --strict --json` with a violation → JSON stdout + **EXIT:1**; without `--strict` + `--json` → EXIT:0 (expected) |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | YAML with `args.go` → `protected_path` on `args.go`; switching to only `command.go` (untouched) → `violations: null` |
| 5 | init toolchain detection | **worked** | `tb init` on cobra → `verify.test_cmd: go test ./...`; `hidden_test_cmd: ""` (no longer a blind Go hidden default) |
| 6 | review `tests/hidden` matches by path | **worked** | ticket `rd.md` mentions `tests/hidden` / `thresholds.yaml` in prose; review on the task diff → **no** critical invariant finding; only drift/`completeness` |
| 7 | bench `default_agent` and `compare --agent` | **worked** | YAML `default_agent: "echo AGENT_RAN {prompt}"` + `tb bench run -r 1` **without** `--agent`/`--fake` → 100% success; `tb bench compare --help` exposes `-a/--agent` |
| 8 | review/drift `## Scope`/`## Escopo`, MD lists and `**` | **worked** | `## Escopo` + bullets + `**/*args*.go` in `rd.md` → drift only flagged `rd.md` out of scope; tickets `/tmp/rd-scope-test.md` and glob `**/*args*.go` → `out_of_scope: null` on the feature commit |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | config with `build_cmd: go build ./...` and `lint_cmd: gofmt -l .` → JSON `build_ran/passed: true`, `lint_ran/passed: true` |
| 10 | init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents` | **worked** | without files: `--write-agents` seeded `AGENTS.md` + `agent_docs.yaml`; probe with existing AGENTS+CLAUDE → `sources: [AGENTS.md, CLAUDE.md]` and **did not** overwrite |
| 11 | graph Python `calls` | **worked** (probe outside cobra) | fixture `/tmp/py-calls-probe/a.py` (`main`→`helper`): `edges_calls: 1`; `tb graph callers --function helper` → `func:a.main`. On cobra (Go) there were already `edges_calls: 1470` via the Go AST |
| 12 | aggregated visual skipped + CLI docs aligned | **worked** | manifesto with missing ref → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"`; `docs/cli.md` documents `mem add <regra>`, `## Scope`, `protected_paths`, `default_agent`, `hidden_test_cmd` |

---

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` / `--write-agents` | ok (~0.004 s); config + `AGENTS.md` + `agent_docs.yaml` | **helped** | Go toolchain detected; useful seed in a repo without instrumentation |
| `tb version` | `a46c295` with ldflags | neutral | correct metadata on the study binary |
| `tb mem init/list/add/check` | init 3 invariants; positional add ok; check retention 1.00 | **helped** | aligned with current `docs/cli.md` (`tb mem add <regra>`) |
| ticket `rd.md` + `## Escopo` | parser accepts heading + bullets + `**` glob | **helped** | drift/review saw `args.go`/`args_test.go` in scope |
| `tb graph --enable index/stats/search/rebuild/impact` | index 463/2103; search found `ExactArgs`; impact on `args.go` listed callers | **helped** | root SkipDirs fixed; search located the API |
| `tb graph callers` | requires `--function` (`--file` flag **removed**/absent) | **mixed** | UX still asymmetric vs positional `search`; works with `--function ExactArgs` failed without a calls index on the short name — Python probe ok |
| `tb verify run` (+ build/lint/hidden) | PASS with build/lint; hidden respects cmd without dir | **helped** | round-1 false PASS (test_cmd only) mitigated when lint/build are configured |
| `tb verify --ruler visual` | PASS `skipped=true` (missing manifesto or missing refs) | neutral | honest aggregation; little value on a Go CLI without UI |
| `tb review` | fix-only: weak `completeness` warning; with `rd.md` in the diff: drift error | **mixed** | no `tests/hidden` FP; still flags “go test passes” acceptance without strong evidence in the diff |
| `tb drift` | ok with `## Escopo`; `rd.md` out of scope → exit 1 | **helped** | `**` globs ok; ticket outside scope is noise if the agent commits it |
| `tb guard check BASE..HEAD` | reads `guard.yaml`; `--strict --json` → exit 1 | **helped** | positional `base..head` syntax (not `--base`); custom paths work |
| `tb bench run/list/compare` | `default_agent` honored; compare has `--agent`; `--fake` ok | **helped** | round-1 dead field fixed |
| `tb mcp` | stdio JSON-RPC; `tools/list` with graph/version/echo/env/run | **helped** (smoke) | with `TB_GRAPH=on` lists graph tools |
| native checks vs tb | tb verify PASS ≠ `golangci-lint` (pre-existing issue) | **mixed** | init **does not** fill `lint_cmd` with `golangci-lint`/`make lint` automatically |

---

## Problems that persist

| problem (round 1 / synthesis) | status this round | evidence |
|-------------------------------|-------------------|----------|
| Config/generated files mode `0600` | **persists** | `.toolbelt/config.yaml` and `AGENTS.md` → `-rw-------` |
| `version` unknown without ldflags | **not retested** (study binary used `make build`) | expected by build design |
| Guard/verify abort when touching `.toolbelt/thresholds.yaml` | **persists** (intentional anti-cheat) | probe: verify **FAIL** `iterations: 0` when committing `thresholds.yaml` |
| Init does not fill lint/build from Makefile/CI | **persists** | only `test_cmd: go test ./...`; `lint_cmd`/`build_cmd` need manual edits |
| `graph callers` UX inconsistent with `search` | **persists** (partial) | `callers` still flag-only (`--function`); `--file` absent from current help |
| tb does not push maintainer-level native lint | **persists** | verify PASS with `gofmt` while `golangci-lint` fails on the repo |
| Hint extraction from `AGENTS.md`/`CLAUDE.md` | **still weak** | `agent_docs.yaml` → `hints: []` even with text “Use make test” / “Prefer pytest” |

---

## New problems

### P2 — `tb init` does not suggest `golangci-lint`/`make lint` even with Makefile + `.golangci.yml`

- **Repro**: `tb init` under `/tmp/target/cobra` (has Makefile `lint` target and `.golangci.yml`)
- **Expected**: fill or suggest `lint_cmd` (and maybe `build_cmd`)
- **Actual**: only `verify.test_cmd: go test ./...`; lint/build left to the agent

### P3 — Review `completeness` warns “go test ./... passes” without looking at verify’s result

- **Repro**: `tb review --ticket rd.md --base <base> --head e2643be` after green tests
- **Expected**: test criterion satisfied if harness/verify passed, or no warning
- **Actual**: `warning` / `accept-2` finding “acceptance criterion weakly evidenced in the diff”

### P3 — Visual manifesto ignores custom `reference:` and uses `references/<screen>-light.png`

- **Repro**: `.toolbelt/screens.yaml` with `reference: .toolbelt/refs/missing.png` + `tb verify run --ruler visual --manifest ...`
- **Expected**: use the declared path (or a clear error citing it)
- **Actual**: `skip_reason: referência ausente` pointing at `references/home-light.png`

### P3 — Committing `rd.md` outside `## Escopo` becomes a drift/review error even when the feature is in-scope

- **Repro**: scope only `args*.go` + ticket committed on the same `HEAD`
- **Expected**: ticket docs ignored or a convention to include them in scope
- **Actual**: `out_of_scope: ["rd.md"]` and review exit ≠ 0

No P0/P1 regression observed on the 12 tested fixes.

---

## What tb caught and what it let slip

**Caught**

- Locating `ExactArgs` via `graph search`
- Drift when Markdown scope was well formed (including `**` globs)
- Guard on custom `protected_paths` (`args.go`) and defaults (`thresholds.yaml`)
- Configured hidden-suite failure without a `tests/hidden/` folder
- Configured build/lint in verify (when the agent fills the YAML)
- Review FP from substring `tests/hidden` **did not** return
- MCP starts and lists tools

**Let slip / did not cover**

- Pre-existing `golangci-lint` issue (verify with `gofmt` ≠ CI lint)
- Maintainer-level completeness (upstream changelog, public docs, multi-Go) — no nudge
- Actionable hints from `AGENTS.md`/`CLAUDE.md` (`hints: []`)
- Real UI quality (visual ruler always-skip in this repo)

---

## Interaction with existing instrumentation / onboarding

Repo **without** upstream AI instrumentation. Bootstrap from scratch (post-fixes):

| Artifact | Generated/read? | Quality |
|----------|-----------------|-----------|
| `.toolbelt/config.yaml` | generated | correct Go `test_cmd`; empty `hidden_test_cmd` (better than blind default); lint/build **not** detected |
| `.toolbelt/agent_docs.yaml` | generated | records sources; `hints: []` |
| `AGENTS.md` via `--write-agents` | generated | minimal stub pointing at `tb verify`/`guard`/`mem` — useful, generic |
| `.toolbelt/invariants.yaml` (`mem init`) | generated | default anti-cheat; now matches by path in review |
| `.toolbelt/guard.yaml` | manual | **honored** when created |
| Reading pre-existing AGENTS/CLAUDE | ok in probe | does not overwrite with `--write-agents` |

Conclusion: onboarding **improved** vs round 1 (AGENTS seed + toolchain + guard/scope/verify), but still requires the agent to configure lint/build and the ticket in the right format.

---

## Comparison with round 1

| area | round 1 | round 2 | verdict |
|------|---------|---------|---------|
| Graph under `/tmp/target` | risk of 0 nodes (SkipDirs) | 463 nodes at root `target` | **better** |
| Markdown Scope | bullets broke drift | `## Escopo` + lists + `**` ok | **better** |
| `guard.yaml` | ignored | `protected_paths` applied | **better** |
| `--strict --json` | exit 0 with violation | exit 1 | **better** |
| `hidden_test_cmd` | gated by dir | runs without dir | **better** |
| `build_cmd`/`lint_cmd` | inert | executed | **better** |
| `default_agent` / `compare --agent` | dead / absent | honored | **better** |
| Review FP `tests/hidden` | confirmed | did not reproduce | **better** |
| Init AGENTS | did not read/generate | `--write-agents` + read | **better** |
| Visual skipped | aggregation lied | `skipped: true` aggregated | **better** |
| CLI docs | outdated | examples match tested CLI | **better** |
| Automatic lint/build detection | absent | still absent | **same** |
| Maintainer completeness / golangci | gap | gap | **same** |
| Config mode 0600 | confirmed | confirmed | **same** |

Overall: **clearly better** on the 12 fixes; deep onboarding gaps (Makefile→lint) and completeness judgment remain.

---

## Improvement suggestions

1. **[P1][init/verify]** Detect `Makefile`/`golangci`/workflows and fill or suggest `lint_cmd`/`build_cmd` — area: `internal/config/detect.go`
2. **[P2][init/mem]** Extract real `hints` from `AGENTS.md`/`CLAUDE.md` (test/lint commands) — area: init/`agent_docs`
3. **[P2][review]** Tie test acceptance criteria to `tb verify` results / presence of `*_test.go` in scope — area: `internal/review`
4. **[P3][verify/visual]** Honor manifesto `reference` field (or document canonical `references/` path) — area: `internal/verify/visual.go`
5. **[P3][drift/review]** Treat `rd.md`/ticket path as in-scope by default or document inclusion under `## Scope` — area: drift/review
6. **[P3][graph]** Accept positional query on `callers` like `search`, or a message that cites `--function` — area: `cmd/tb/graph.go`
7. **[P3][config]** Mode `0644` for generated non-secret configs (`config.yaml`, `AGENTS.md`) on shared clones — area: init

---

## Metrics

| metric | round 2 | round 1 (reference) |
|--------|---------|---------------------|
| Wall time (tb evidence log) | ~**2.8 min** (12:37:00 → 12:39:48 UTC); full study with golangci-lint v2 install ~**5–7 min** | ~3.6 min (tb) / ~6–8 min total |
| Recorded `tb` invocations | **40** | **64** |
| `tb` invocations with exit ≠ 0 | **9** (expected probes: guard strict, out-of-scope drift/review, hidden fail, bad flags) | **19** |
| tb crashes | **0** | **0** |
| Local commits on cobra clone | 1 feature (`e2643be`) + 1 ticket `rd.md` (`3f01345`); thresholds probes discarded | 1 functional + probes |
| Interactions with target GitHub | **0** | **0** |
| tb binary | `a46c295` | `c8aae04` |
