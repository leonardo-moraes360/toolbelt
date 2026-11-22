🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/release-1.0.md)

# 1.0.0 readiness plan — spec

Status: **draft** (2026-11-22). Owner decisions in [§6](#6-decisions-for-the-owner) are open.

## 1. Scope

The phase between v0.0.10 and v1.0.0 is **stability only**: fix bugs, close contract gaps, remove dead surface, make docs true, and freeze the public contract under semver. No new features. An item that adds behavior is in scope only when it fixes a safety problem or replaces an ambiguous contract that 1.0 would otherwise freeze; every such item is listed in §6 for the owner.

Inputs: four read-only audits (2026-11-22), the CI history of `main` (60 runs: 4 failures, the last 26 green), the field notes in [dogfood](../campo/dogfood.md) (entries #48–#58 come from the owner's `tb bus` session of 2026-11-22), and the code at `9071d97`. Audit claims were spot-checked; corrections are in [§8](#8-audit-corrections).

## 2. Exit criteria for `v1.0.0`

All of these hold on the commit that gets the tag:

| # | Criterion | How it is measured |
|---|-----------|--------------------|
| E1 | Every `blocker` item in §4 is merged | this spec's tables, each row with its PR |
| E2 | Every `should` item is merged or deferred to 1.1 by the owner, in writing, in this spec | same |
| E3 | 20 consecutive `CI` workflow runs on `main` succeed, every job, first attempt (a run that needed a re-run counts as a failure) | `gh run list --branch main --workflow CI --json conclusion,attempt` |
| E4 | Zero known flakes: the nightly stress job (item C6) is green 5 nights in a row, and no test is skipped or deleted to hide a flake | stress workflow history; review of the diff since v0.0.10 for new `t.Skip` |
| E5 | `golangci-lint run` clean on `main` with `.golangci.yml` as is; every new `//nolint` names its linter and a reason | the `Lint` job; `grep -rn nolint` |
| E6 | The contract snapshot (item R3) matches the freeze list in §3; any later diff to it fails CI | `TestPublicContractSnapshot` |
| E7 | Docs claim only what exists: no planned command, flag, config key or subsystem described as present; `TestDocs*` green; pt-BR mirror complete | item D1–D8 acceptance; `go test ./cmd/tb/ -run TestDocs` |
| E8 | `v1.0.0-rc.N` passed the release workflow (draft, `smoke` on 6 runners, `floors`, `publish`) and was used on this repository for ≥ 7 days with no new `high` dogfood entry | release run; [dogfood](../campo/dogfood.md) |
| E9 | `CHANGELOG.md` has a `[1.0.0]` section whose `Removed` / `Changed` lists every breaking change from §4; `SECURITY.md` names 1.0.x as supported | review |

## 3. Freeze list

Under semver, 1.x may add; it may not remove or change the meaning of anything marked **stable** below. Breaking changes need 2.0. A deprecated item keeps working, hidden from help, with a stderr warning, until 2.0.

### 3.1 Stability tiers

Every Cobra command and MCP tool carries a tier (item S7): `stable` or `experimental`. Experimental surface may change in any minor release; its help line starts with `(experimental)` and its MCP description with `[experimental]`.

| Tier | CLI | MCP |
|------|-----|-----|
| stable | `init`, `version`, `mem *`, `verify run`, `guard check`, `review`, `drift`, `stack` (ops `verify`, `review`, `drift`, `guard`), `sandbox check\|run`, `graph *` (lexical and `--vectors`/`--mode`), `ticket *`, `workspace *`, `merge` (strategies `fail`, `last_wins`, `semantic`), `mcp` | `tb_version`, `tb_run`, `tb_mem_*`, `tb_verify_run`, `tb_review`, `tb_drift`, `tb_guard_check`, `tb_stack`, `tb_graph_*`, `tb_ticket_*`, `tb_merge` |
| experimental | `bus *`, `router *`, `board *`, `watchdog *`, `egm *`, `vault *`, `red *`, `blue *`, `lab *`, `bench *`, `merge --strategy prompt`, `stack` ops `egm`, `vault`, `seccheck` | `tb_bus_*`, `tb_router_status`, `tb_board`, `tb_watchdog_alerts`, `tb_egm_*`, `tb_vault_*`, `tb_red_*`, `tb_blue_*`, `tb_sec_pair` |
| removed before 1.0 | `guard check --strict` (decision O1) | `tb_echo`, `tb_env` (O2); `tb_guard_check` `strict` |
| internal (never public) | `__graph-embed`, `fake-agent`, hidden commands | — |

Rationale for the experimental set: their ADRs are still `proposed` (009, 013, 016, 017) or their own field data says the design is unsettled (egm convergence, [dogfood](../campo/dogfood.md) #32–#34; bench KPIs not implemented). Tiering is decision O3.

### 3.2 CLI

Frozen for stable commands: command paths, flag names and types, positional arguments, defaults, the global flags `--json`, `-c/--config`, `-v/--verbose`, `--version`. The frozen inventory is the snapshot of the Cobra tree (item R3), not a hand list.

Renames that land before 1.0 (S1–S3): see §4 track S.

### 3.3 MCP

Frozen for stable tools: tool name, parameter names and types, `required`, defaults, annotations, and the result shape (§3.4). Server: stdio JSON-RPC, protocol versions `2025-11-25`, `2025-06-18`, `2024-11-05` (`internal/mcp/server.go`). Tool count after the removals: 52 (54 today).

Naming rule frozen with 1.0: an MCP parameter is the CLI flag with `-` → `_`, plural when the flag is repeatable (`--egm-doc` → `egm_docs`, `--check` → `checks`). A positional CLI argument becomes a named parameter (`tb blue apply <class>` → `class`). One documented exception: `tb_merge` takes `apply` (default `false`, dry-run) where the CLI takes `--dry-run`, because an agent must opt in to moving refs (decision O5). `cwd` is optional on every tool and defaults to the server's working directory (item S4).

### 3.4 JSON envelopes

**Convention (decision O4):** every JSON document `tb` prints on stdout with `--json`, and every MCP tool result body, has a top-level string field `v` = `<kind>/<major>.<minor>`. Minor bump: fields added. Major bump: anything else. Consumers must ignore unknown fields. This is the form seven envelopes already use; the integer `version` / `v` forms stay only in on-disk files (§3.6).

Already compliant (kept at their current version; frozen at it): `bus/0.1` (message), `bus-wake/0.1`, `bus-discover/0.1`, `router-task/0.1`, `egm-source-drift/0.2`, `egm-draft/0.1`, `egm-brainstorm/0.1`.

Must adopt it (item J1), starting at `<kind>/1.0`:

| Kind | Producer |
|------|----------|
| `error/1.0` | the CLI error body `{"error","reason","exit_code"}` (today egm and vault only; item J3 makes every command use it under `--json`) and MCP tool errors (item J4) |
| `version/1.0` | `tb version --json`, `tb_version` |
| `init/1.0` | `tb init --json` |
| `mem/1.0` | `tb mem *`, `tb_mem_*` |
| `verify/1.0` | `tb verify run`, `tb_verify_run` (`verify.Result`) |
| `guard/1.0` | `tb guard check`, `tb_guard_check` |
| `review/1.0` | `tb review`, `tb_review` |
| `drift/1.0` | `tb drift`, `tb_drift` |
| `stack/1.0` | `tb stack`, `tb_stack` |
| `sandbox-run/1.0`, `sandbox-check/1.0` | `tb sandbox run\|check`, `tb_run` |
| `graph-<op>/1.0` | `tb graph search\|callers\|deps\|impact\|explain\|path\|stats\|index\|rebuild`, `tb_graph_*` |
| `ticket/1.0`, `ticket-list/1.0` | `tb ticket *`, `tb_ticket_*` |
| `workspace/1.0` | `tb workspace *` |
| `merge/1.0` | `tb merge`, `tb_merge` |
| experimental kinds (same rule, not frozen) | `board`, `router-status`, `watchdog`, `bus-*` outputs that are not a message envelope, `egm` (`egm.Outcome`), `vault-*`, `red-*`, `blue-*`, `lab-*`, `bench-*` |

### 3.5 Exit codes

Proposed stable table (item J2). Today `main.go` maps only `verify.UsageError` to 2; egm and vault have their own 0–3; `tb sandbox run` exits 0 on timeout and under `--json` (bugs J5, J6).

| Code | Meaning | Change from today |
|------|---------|-------------------|
| 0 | success: `PASS`/`SKIPPED`, no `SeverityError` (or `--warn-only`), check passed | — |
| 1 | the check ran and failed (`FAIL`, `ESCALATE`, guard `SeverityError`, vault check, egm `FAIL`), or a runtime error after work started | — |
| 2 | usage error or refusal before any work: bad flag or argument, unknown subcommand, invalid `--ruler`/`--check`, egm/vault refusal reasons | flag/arg parse errors and unknown subcommands move from 1 to 2 |
| 3 | egm: the verdict does not allow the operation (`receipt` refused unless `PASS`/`PASS_PARTIAL`) | documented; egm only |
| 124 | `tb sandbox run`: the child hit `--timeout` | new; today 0 |
| 127 | a configured command was not found or is not executable (`verify` gate commands, the sandboxed child's exec) | — |
| 130 | interrupted by SIGINT/SIGTERM (`tb mcp` today) | documented |
| child | `tb sandbox run` passes the child's non-zero code through, with or without `--json` | `--json` today exits 0 |
| max | `tb stack` exits with the highest exit code among its steps | documented |

### 3.6 Config keys and on-disk formats

**Config** (`.toolbelt/config.yaml`, `~/.config/toolbelt/config.yaml`): every key documented in [cli.md](../cli.md#configuration) at 1.0 is frozen. Keys that are parsed but read by nothing (checked: no reader outside `internal/config`) are removed before 1.0 (item K1, decision O6): `defaults.sandbox`, `defaults.timeout`, `defaults.json_output`, `sandbox.allow_net`, `sandbox.cpu_limit`, `sandbox.mem_limit`, `sandbox.time_limit`, `graph.auto_index`, `graph.infer_relations`, `vault.sync_on_change`, `logging.level`, `logging.file`, `logging.format`. Decoding stays lenient: a removed key in an old file is ignored, with a one-line stderr warning (item K2). `router.bus.claim_timeout` is used (`internal/router/bus.go:111`) and stays. `sandbox.allow_unsandboxed` stays: it is an explicit opt-in, not a silent fallback.

**Files the user writes or commits — frozen; 1.x reads every 1.0 file and never rewrites a newer version it does not know:**

| File | Version marker |
|------|----------------|
| `.toolbelt/config.yaml` | none (keys above) |
| `.toolbelt/invariants.yaml` | `version: 1` |
| `.toolbelt/tickets/TKT-*.yaml` | none today; item J7 adds optional `version: 1` (absent = 1) |
| `.toolbelt/guard.yaml`, `.toolbelt/thresholds.yaml`, `.toolbelt/screens.yaml`, `agent_docs.yaml` | none (schema in cli.md) |
| `.toolbelt/egm/receipts/<slug>.json` (experimental feature, but committed by users) | `version` 1–3 |
| `.toolbelt/secchecks.yaml` (experimental) | `version` |

**Local state — not frozen; tb rebuilds it, migrates it forward, or refuses with a named reason, never misreads it:**

| File | Rule |
|------|------|
| `.toolbelt/graph.json` | derived cache; item J7 adds `version`; a mismatch means reindex (needed by G1, which changes node IDs) |
| `.toolbelt/graph.vec` | magic `TBVEC1` + JSON header `v`; mismatch → `vectors_stale`, rebuild |
| `.toolbelt/bus/bus.db` | `PRAGMA user_version` (4 today); 1.x migrates forward from any 1.0 schema, never down |
| router, egm `state.json`, merge prompt receipts, `$XDG_STATE_HOME/toolbelt/bus/*` | internal; refuse on unknown version |

## 4. Work items

Tiers: **blocker** (E1), **should** (E2), **nice** (may slip to 1.0.x). Each track owns a disjoint file scope; a PR that must touch another track's file says so in its description and lands after that track's open PR. Docs for a behavior change land in the same PR as the change (English + pt-BR); track D owns the cross-cutting rewrites.

### Track C — CI and flakes

Scope: `*_test.go` in the packages named, `.github/workflows/ci.yml`, a new `.github/workflows/stress.yml`. No production code.

| Id | Tier | Area | Acceptance |
|----|------|------|-----------|
| C1 | blocker | `internal/mdparse/mdparse_test.go:186-197` `TestWikilinksManyOnOneLine` | the linearity check compares operation counts or a ratio over repeated runs, not wall time; 50 runs with `-race -count=50` pass on a loaded runner |
| C2 | blocker | `internal/router/router_escape_test.go:237` `TestDaemonFromWorktreeCreationDoesNotFailTicket` and the other "ainda vivo" waits | waits poll with a deadline scaled for `-race`; failure messages in English; 50× green |
| C3 | blocker | `internal/router` `TestPauseThenResume` ("no router state found", `TempDir` RemoveAll "directory not empty") | the test waits for the router goroutine to exit before cleanup; 50× green |
| C4 | blocker | `internal/embed` `TestClientStartFailures` (`embed_crashed: exit status 2` with `GOCOVERDIR` set) | the expected failure is asserted by reason, independent of the coverage env; green under `-coverprofile` |
| C5 | should | run 44842322646: 15 router tests `signal: killed` at the 60 s timeout | router tests that spawn git get a per-test deadline and run with `t.Parallel` limits that fit one runner; the `Test` job has `timeout-minutes` with headroom |
| C6 | blocker | new `stress.yml` (nightly + `workflow_dispatch`) | `go test -race -count=10` over `internal/router`, `internal/mdparse`, `internal/embed`, `internal/bus`, `internal/busrun`; it is what E4 measures |
| C7 | should | fail-closed coverage: `TestNewRequiresAllowUnsandboxedWithoutLandlock`, `TestRunSandboxNonLinuxFailsClosed`, `TestLabUpPodmanFailsClosedCLI` always skip on the runner (it has Landlock) | a test-only seam (unexported variable set in `_test.go`, never an env var or flag) makes the "no Landlock" path run on Linux CI; the refusal is asserted. Respects the fail-closed invariant: no runtime bypass |
| C8 | nice | `internal/verify/verify_test.go:62`, `internal/stack/stack_test.go:379`: the fixture `TestMarkerOnlyOnHead` fails on purpose and its `--- FAIL` shows in CI logs | the fixture message says `expected fixture failure`, so log scans do not report it |

C1–C4 and C7 touch different packages and run in parallel; C5 and C6 both touch workflows: one PR, or C6 after C5.

### Track J — exit codes and JSON envelopes

Scope: `cmd/tb/main.go`, `cmd/tb/sandbox.go`, a new `cmd/tb/output.go` (shared `--json` printer and error body), the JSON print sites in `cmd/tb/*.go` (not the flag declarations), the result structs in `internal/{verify,guard,review,stack,graph,ticket,workspace,merge,mem,version}`, `internal/registry/registry.go` (result formatting). Docs: [cli.md](../cli.md#exit-codes) "Exit Codes" and "Errors and Usage".

| Id | Tier | Area | Acceptance |
|----|------|------|-----------|
| J1 | blocker | every envelope in §3.4 | each carries `v`; a table test lists every `--json` command and checks `v` matches `^[a-z-]+/\d+\.\d+$`; MCP results match the CLI's |
| J2 | blocker | `cmd/tb/main.go` `exitCodeFor` | the §3.5 table: Cobra flag/arg errors and unknown subcommands exit 2; a test per row; cli.md table replaced ("not stable" sentence removed) |
| J3 | should | `cmd/tb/output.go` | under `--json`, every command's error is the `error/1.0` body on stdout with `reason` and `exit_code`, as egm and vault do now |
| J4 | should | `internal/registry/registry.go` `Call` | a tool error is `isError: true` with the `error/1.0` body as text (today plain text) |
| J5 | blocker | `cmd/tb/sandbox.go:45-51` | `tb sandbox run --json` exits with the child's code (today 0) |
| J6 | blocker | `cmd/tb/sandbox.go:49` | timeout exits 124 (today 0); JSON has `timed_out: true` |
| J7 | should | `internal/graph/graph.go` (`version` in graph.json; reindex on mismatch), `internal/ticket` (optional `version`) | old files load; graph.json without `version` triggers a rebuild, not an error |
| J8 | nice | `internal/merge/receipt.go:91` integer `v` | renamed `version` with read compat for `v` (local state, low value) |

J1 and J2 conflict in `cmd/tb/*.go` print sites: do J2 + J5 + J6 first (small), then J1 + J3. J4 and J7 are independent.

### Track S — CLI/MCP surface and naming

Scope: flag declarations in `cmd/tb/{guard,verify,stack}.go`, `internal/registry/{builtin,parity,egm,vault,annotations}.go`, stability tier plumbing (Cobra `Annotations`, registry `Tool` field), `cmd/tb/docs_cli_test.go` (the `--strict` assertion), the matching cli.md sections.

| Id | Tier | Area | Acceptance | Alias |
|----|------|------|-----------|-------|
| S1 | blocker | `tb guard check --strict`, `tb stack`'s guard path, MCP `tb_guard_check` `strict` (`parity.go:290`) | removed (O1); `assertDocsGuardStrictDeprecated` becomes "cli.md says removed in 1.0"; CHANGELOG `Removed` | none: it has been a no-op since it was deprecated; `--strict` on `egm status` and `stack` (egm meaning) is unaffected |
| S2 | blocker | `internal/registry/builtin.go:29-44` `tb_echo`, `tb_env` | removed (O2); `tb_version` gains `cwd` only if O2 says so; annotation table and tests updated | none (debug tools) |
| S3 | blocker | `tb verify run --no-check`, `tb stack --no-check` (repeatable names) next to `--no-checks` (bool); MCP `skip_checks` | flag renamed `--skip-check` so the §3.3 rule maps it to `skip_checks` (O7) | `--no-check` kept hidden + deprecated until 2.0 |
| S4 | should | `internal/registry/egm.go:25,38,57`, `vault.go:23-86` | `cwd` no longer `required`; default as in `resolveToolCWD` | n/a (relaxation) |
| S5 | should | Portuguese in tool descriptions (`parity.go:320` "testes ocultos") | English descriptions; a test rejects non-ASCII letters in tool descriptions | — |
| S6 | should | `tb_merge` `apply` vs `--dry-run` | kept (O5); the description and cli.md say why | — |
| S7 | blocker | tier marker on every command and tool (§3.1) | a test fails if a command or tool lacks a tier; help and descriptions show `(experimental)` / `[experimental]` | — |

S1–S3 touch different files and run in parallel; S7 touches every command file's declaration, so it lands last in the track, after S1–S3 rebase.

### Track G — correctness bugs found in the field

Scope: `internal/graph/{index_go,index_common}.go` and the other indexers' ID helpers, `internal/verify/failure_output.go`, `internal/egm/{roundctx,review}.go` message strings, `cmd/tb/init*` and `internal/config/init_detect.go` (dogfood #1/#7 re-check only).

| Id | Tier | Area | Acceptance |
|----|------|------|-----------|
| G1 | blocker | Go indexer IDs ([dogfood](../campo/dogfood.md) 2026-11-22 note; `index_go.go:34,52,54`): `pkg:<name>` collapses every `main`; methods drop the receiver | package ID is the import path (or module-relative dir); method ID includes the receiver (`func:<pkg>.<Recv>.<Name>`); `funcIDs` keyed by receiver + name; graph.json `version` bumped (J7). Breaking for stored IDs, so it must land before 1.0 |
| G2 | blocker | `internal/graph/index_common.go:50` `readSource` follows symlinks and blocks on FIFOs ([vault spec](vault.md) Q5) | the graph walk reads through the same safe reader as `vector.go:399-410` (no symlink component, regular file, size cap); a FIFO in `docs/` does not hang `tb graph index` (test) |
| G3 | should | dogfood #10: `verify` `reason` truncated from the start | re-check on main; if `failure_output.go` already keeps the `--- FAIL` lines, close #10 in dogfood with the commit; else fix and test |
| G4 | should | bilingual or Portuguese user messages (`internal/egm/roundctx.go:89`, `review.go:222`) | English only; tests updated |
| G5 | should | re-check dogfood #1, #7, #17 (marked open; audit says fixed) and field round-2 N1, N3, N4 (playwright-mcp, pydantic-ai notes) | each row says `fixed in <sha>` or stays open with a G-item; docs updated in both languages |

G1 and G2 touch the same package but different files; run G2 first (small). G3–G5 are independent.

### Track B — bus (owner field session, 2026-11-22)

Scope: `internal/bus`, `internal/busrun`, `cmd/tb/bus*.go`, `cmd/tb/mcp.go` (`bindBusAs`), `internal/registry/bus.go`, `internal/version` (B5 only), [bus-protocolo.md](../bus-protocolo.md), [bus-harness.md](../bus-harness.md). The bus is experimental (§3.1), but B1 is a safety fix and blocks 1.0 regardless. Dogfood entries #48–#58.

Verified in code:

- `Store.Join` (`internal/bus/store.go:467`) needs an invite; a bound invite re-joins and replaces the token. But `tb mcp --bus-as <channel>/<name>` (`cmd/tb/mcp.go:58`, `registry.BindBusSession`) loads the stored session with no lock, and only `tb bus run` takes a lock (`internal/busrun/harness.go:126`). Two clients started with the same `--bus-as` share one identity and one worktree.
- No dependency field exists anywhere in the bus: `task.assign` payloads are opaque (`docs/bus-protocolo.md`), and `ClaimThread` (`internal/bus/lease.go:70`) checks only the lease. Ticket `dependencies` are honored by `tb router`, not by the bus.
- `tb bus invite` and channel create print the expiry as `15:04:05Z07:00` with no date (`cmd/tb/bus.go:119,240`).
- `wake_mismatch` (`internal/busrun/notify.go:245`) states the mismatch, not the fix.
- The SessionStart hook that reported the version mismatch is the owner's, outside this repo. On the tb side, `tb version` prints `dev`/`unknown` for a build without ldflags (`internal/version/version.go`), so a hook has nothing stable to compare but a binary or module hash.
- Claim states (`held`/`expired`/`released`) are in `tb_bus_status`; how a released claim ended (completed, expired, revoked, taken over) is not.

| Id | Tier | Kind | Area | Acceptance |
|----|------|------|------|-----------|
| B1 | blocker | bug (safety) | `cmd/tb/mcp.go` `bindBusAs`, `internal/busrun/harness.go`, `internal/bus` session files | one exclusive lock per `<channel>/<name>` under the private state dir, held by `tb mcp --bus-as` and `tb bus run` for their lifetime; a second holder exits 2 with `session_in_use` naming the holder's pid; a re-join with a bound invite while the lock is held is refused the same way; lock released on exit and on crash (flock) |
| B2 | should | contract gap | [bus-protocolo.md](../bus-protocolo.md), `tb bus send` help | docs say `payload` is opaque and that ordering by dependency is `tb router`'s job (ticket `dependencies`); `tb bus send` with a `task.assign` payload holding `deps`/`dependencies` prints a stderr warning that the bus does not enforce them. Bus-level dependencies: O8 |
| B3 | should | bug | `cmd/tb/bus.go:119,240` | expiry printed as RFC 3339 with date, local zone, as `channel show` does |
| B4 | should | bug | `internal/busrun/notify.go:245`, `check.go` | the `wake_mismatch` line says which side to change (Agent Card `wake` or the wake file) and the command that does it |
| B5 | should | contract gap | `internal/version/version.go` | without ldflags, `Version`/`Commit` come from `debug.ReadBuildInfo` (`Main.Version`, `vcs.revision`, `vcs.modified`); `tb version --json` gains `modified`; docs state the comparison rule for hooks: version + commit, never a module or binary hash |
| B6 | should | contract gap | `tb_bus_status`, `tb bus channel show` | each claim carries `outcome` (`completed`, `expired`, `revoked`, `taken_over`, or null while held). Additive, but it fixes an ambiguous field before the envelope is versioned (O9) |
| B7 | nice | regression tests | `cmd/tb/bus_test.go`, `cmd/tb/mem_test.go` | CLI-level tests for what worked: 17 `tb bus send` with idempotency keys sent twice store 17 messages; sending to an agent not in the channel names it (`store_test.go:206` covers the store only); `tb mem list` on a fresh repo lists the seeded invariants |
| — | deferred (O10–O12) | feature | — | channel task list with state; reserve paths without a task / release a claim by hand; path-overlap check across tickets |

B1 first (it touches `mcp.go` and the harness); B3, B4, B5, B6 touch different files and run in parallel with it. B2 is docs plus one warning in `cmd/tb/bus.go`: after B3.

### Track K — config cleanup

Scope: `internal/config/config.go`, `internal/config/init_detect.go`, `cmd/tb/init*.go`, cli.md "Configuration".

| Id | Tier | Area | Acceptance |
|----|------|------|-----------|
| K1 | blocker | the 13 dead keys in §3.6 | removed from the structs and `DefaultConfig`; `tb init` no longer writes them; cli.md lists only live keys |
| K2 | should | `config.Load` | a key the struct does not know prints one stderr warning per file per run (`unknown config key graph.auto_index (ignored)`); never an error, so old files keep working |
| K3 | should | dogfood #1 (`tb init` writes `golangci-lint run` without a PATH check) | re-check; fix or close (shares `init_detect.go`, so it is here, not in G5) |

K1 then K2 (same file). K3 in parallel.

### Track D — docs that overclaim

Scope: `README.md`, [arquitetura.md](../arquitetura.md), [benchmark.md](../benchmark.md), [identidade-visual.md](../identidade-visual.md), `docs/adr/*`, `site/index.html`, the cli.md overview tree and the planned `tb run`/`tb proc` sections, [appsec-playbooks.md](appsec-playbooks.md) header, `CLAUDE.md` local notes; every file with its pt-BR twin. Other tracks edit only their own command's cli.md section.

| Id | Tier | Area | Acceptance |
|----|------|------|-----------|
| D1 | blocker | arquitetura.md §3.2–3.3, §4.2–4.3, §5.1–5.2, §6, §8.2–8.3, §9.3, §13.3–13.4 | describes what exists: graph.json + BM25 (no SQLite/FTS5 index, no nodes/edges tables), vault read on demand (no watcher), SQLite only in `bus.db`, no bubblewrap in `tb sandbox`, the real `.toolbelt/` tree; roles/CapabilityToken/`tb agent` removed per ADR-020; answered open questions closed |
| D2 | blocker | README.md L28-29, L59-70, badge | no bubblewrap, cgroups, "capability-based permissions" or FTS5 claims; Go badge from `go.mod`; MCP count from the snapshot (R3) |
| D3 | blocker | cli.md planned sections (`tb run`, `tb proc`, `run / proc / md / agent`, `serve`) and overview tree | planned commands move to one README roadmap line; the tree lists every top-level command with its tier (adds `red`, `blue`, `lab`, egm `drift-report\|bootstrap\|brainstorm\|draft`) |
| D4 | should | benchmark.md | no p < 0.05 claim; unimplemented KPIs (`file_reads`, `rework_rate`, `guard_violations_*`, `defects_found`, `false_positive_rate`, `tokens_per_review`, `reviewer_agreement`, `wall_time_total`, `critical_path`) moved to "Not implemented" (O13) |
| D5 | should | ADR statuses | 003 notes graph.json superseded the SQLite index; 004 notes seccomp shipped; 010 and 018 `accepted` (shipped, stable); 009, 013, 016, 017 stay `proposed` and say "experimental in 1.0" (O14) |
| D6 | should | `site/index.html` | no "v0.0.5" / "36 tools" / Go 1.22; version-free wording or values from the release |
| D7 | nice | identidade-visual.md:272 `.tb/vault/` → `.toolbelt/vault/`; appsec-playbooks.md header status | fixed |
| D8 | should | `CLAUDE.md` "Local notes" | `golangci-lint` is on PATH now; MCP tool count from the snapshot; stale notes removed |

D1–D8 touch disjoint files and run in parallel. D3 touches cli.md: land it before tracks J, S, K edit their cli.md sections, or rebase them.

### Track R — release plumbing and the freeze

Scope: `.goreleaser.yaml`, `.github/workflows/release.yml`, [release.md](../release.md), `SECURITY.md`, `CHANGELOG.md`, `.toolbelt/invariants.yaml` (through `tb mem`), new `cmd/tb/contract_test.go` + `internal/registry/contract_test.go` + `testdata/contract/`.

| Id | Tier | Area | Acceptance |
|----|------|------|-----------|
| R1 | blocker | `.goreleaser.yaml`, `release.yml` publish step | `-rc.N` tags publish as GitHub prerelease and not `latest` (`release.prerelease: auto`; publish keeps it); a dry run with `goreleaser release --snapshot` shows it |
| R2 | blocker | invariant `inv-1795019844137842036` ("No-CGO binary … never add a dependency that needs CGO") contradicts ADR-018 and `.goreleaser.yaml` (`CGO_ENABLED=1`) | edited with `tb mem edit` to the shipped rule (cgo only for the vendored, allowlisted native code of ADR-018; pure-Go SQLite stays) (O15) |
| R3 | blocker | contract snapshot | test writes the Cobra tree (paths, flags, types, defaults, tier) and the MCP `tools/list` (names, schemas, annotations, tier) to `testdata/contract/*.json` with `-update`; CI fails on any diff; the 1.0 snapshot is the freeze list of §3.2–3.3 |
| R4 | blocker | `SECURITY.md` | supported versions: 1.0.x (latest minor of 1.x after that); seccomp listed as implemented; bubblewrap not claimed |
| R5 | blocker | `CHANGELOG.md` | `[Unreleased]` → `[1.0.0-rc.1]`, then `[1.0.0]`; `Removed` and `Changed` list every breaking item (S1, S2, S3, J2, J5, J6, K1, G1) |
| R6 | should | [release.md](../release.md) | rc flow (§7) documented; examples use `v1.0.0-rc.1`; deprecation policy (§3) linked |

R3 lands after S1–S3, S7 and J1 (it snapshots their result). R1, R2, R4, R6 run any time. R5 is the last PR before the tag.

### Tracks at a glance

| Track | Scope | Blocker | Should | Nice | Deferred |
|-------|-------|---------|--------|------|----------|
| C — CI and flakes | tests + workflows | 5 | 2 | 1 | — |
| J — exit codes and envelopes | output and result structs | 4 | 3 | 1 | — |
| S — CLI/MCP surface | flag decls, registry schemas, tiers | 4 | 3 | — | — |
| G — field bugs | graph IDs/reader, verify reason, egm strings | 2 | 3 | — | — |
| B — bus | bus, busrun, `mcp.go` bind, version | 1 | 5 | 1 | 3 |
| K — config | `internal/config`, init | 1 | 2 | — | — |
| D — docs | README, arquitetura, ADRs, cli.md tree, site | 3 | 4 | 1 | — |
| R — release and freeze | goreleaser, release.yml, contract snapshot, CHANGELOG | 5 | 1 | — | — |

Order across tracks: C, G, B, K, D start at once. J and S start at once but serialize on shared `cmd/tb` files as noted. R3 and R5 close the phase.

## 5. Implementer rules

- One item (or the small group named in a track) per PR, branch `fix/`, `docs/` or `chore/` off `main`, Conventional Commits.
- Every behavior change updates cli.md (English + pt-BR) and `CHANGELOG.md` `[Unreleased]` in the same PR.
- Review in a fresh context, never by the implementing agent; `tb stack` verdict `PASS` before asking for merge.
- Never edit `tests/hidden/` or `.toolbelt/thresholds.yaml`. No silent no-sandbox fallback (C7 uses a test-only seam). `tb` stays local: no network calls are added.
- A flake fix proves itself with `go test -race -count=50` on the package, run alone (`-p 1` on small machines).

## 6. Decisions for the owner

| # | Decision | Recommendation | Rationale |
|---|----------|----------------|-----------|
| O1 | Remove `tb guard check --strict` (and MCP `strict`) before 1.0, or freeze it as a deprecated no-op? | remove | a no-op frozen in 1.0 must be carried through all of 1.x; pre-1.0 is the only cheap moment |
| O2 | `tb_echo` / `tb_env`: remove, hide, or keep? | remove both; do not move `tb_env`'s fields into `tb_version` | debug tools in a frozen list; `tb_version` already gives os/arch/go |
| O3 | Tiering in §3.1 (bus, router, board, watchdog, egm, vault, red/blue/lab, bench, merge `prompt` experimental) | accept | their ADRs are `proposed` or their field data is unsettled; freezing them now would block fixes |
| O4 | JSON version convention | `"v": "<kind>/<major>.<minor>"` on every stdout/MCP envelope; integer `version` only inside on-disk files | already used by 7 envelopes, self-describing, and keeps file formats separate from wire formats |
| O5 | `tb_merge` `apply` (MCP) vs `--dry-run` (CLI): which wins? | keep both as is, documented as the one exception | an agent should have to opt in to moving refs; a human CLI user expects `--dry-run` |
| O6 | Remove the 13 dead config keys, or document them as reserved? | remove, with K2's warning | they promise limits (cpu/mem/time, allow_net) that `tb` does not enforce |
| O7 | `--no-check` (repeatable) vs `--no-checks` (bool) vs MCP `skip_checks` | rename the CLI flag to `--skip-check`, keep `--no-check` as a hidden alias until 2.0 | two flags one letter apart with different types; the MCP name already says `skip` |
| O8 | Bus-level task dependencies (`deps` in `task.assign`): in 1.0? | defer to 1.1; ship B2 (docs + warning) in 1.0 | it is a new broker rule with lease interactions; `tb router` already orders by ticket `dependencies` |
| O9 | Claim `outcome` in `tb_bus_status` (B6): in 1.0? | yes | small, read-only, and settles what `released` means before the envelope gets `v` |
| O10 | Channel task list with state (`tb bus tasks`): in 1.0? | defer to 1.1 | new command, additive later without breaking anything; `tb bus log` covers it meanwhile |
| O11 | Reserve paths without a task / release a claim by hand: in 1.0? | defer to 1.1 | new broker operations that need their own spec (leases, revocation); no contract is made worse by waiting |
| O12 | Path-overlap check across tickets: in 1.0? | defer to 1.1 | a new check with false-positive policy to design; `tb drift` already flags out-of-scope edits after the fact |
| O13 | Unimplemented benchmark KPIs: keep or drop from benchmark.md? | move to a "Not implemented" section; drop the significance claim | docs must claim only what exists (E7); the list stays as a roadmap |
| O14 | ADR statuses: accept 010 and 018; keep 009, 013, 016, 017 `proposed` | accept | status then matches the tier: accepted = stable, proposed = experimental |
| O15 | The "No-CGO binary" invariant vs ADR-018's cgo release | edit the invariant to the shipped rule (R2) | the invariant is false today and `tb mem check` treats invariants as top-priority instructions |
| O16 | Exit code changes in §3.5 (parse errors 1 → 2, sandbox timeout 0 → 124, sandbox `--json` passthrough) | accept | 2 for usage is already the verify/egm/vault rule; 0 on a timed-out child is a bug |
| O17 | Existing `v` kinds at `0.x` (`bus/0.1`, …): bump to `1.0` at the freeze? | keep their numbers | they version schemas, not releases; stored messages in `bus.db` keep matching |
| O18 | `tb stack` on uncommitted changes (dogfood #13): in 1.0? | defer to 1.1; document that `stack` compares commits | it is a new input mode, not a fix |
| O19 | rc soak length (E8) | 7 days, at least one rc | the owner is the main user; a week covers a full work cycle on this repository |

## 7. Release steps

Follow [release.md](../release.md); the additions for 1.0:

1. Close every blocker; the owner settles §6 and marks deferred `should` items in this spec.
2. R1 merged (prereleases are not `latest`). R5 moves `[Unreleased]` to `[1.0.0-rc.1]` in both changelogs.
3. Tag `v1.0.0-rc.1` on a green `main`; the release workflow builds the draft, runs `smoke` and `floors`, and publishes it as a prerelease.
4. Install it as release.md describes (sha256 + bundle certificate) into `~/go/bin`; use it on this repository. New findings go to [dogfood](../campo/dogfood.md). A `high` finding: fix on `main`, tag `v1.0.0-rc.2`, restart the soak.
5. When E1–E9 hold: rename `[1.0.0-rc.N]` to `[1.0.0]` (keep the rc entries' text), tag `v1.0.0` on the same commit as the last rc or a docs-only descendant of it.
6. After 1.0: `main` takes only fixes and additive changes; a breaking change opens a 2.0 spec. Never reuse a tag.

## 8. Audit corrections

The audits were read-only and partly wrong; implementers rely on this section, not on the audit text.

- `TestMarkerOnlyOnHead` is not a flake: it is a fixture that `internal/verify` and `internal/stack` tests write on purpose to make verify fail on the head; its `--- FAIL` is logged at debug level inside passing tests (C8).
- `router.bus.claim_timeout` is read (`internal/router/bus.go:111`); not dead.
- MCP "threshold vs `--min-coverage`" is not drift: both `threshold` (visual) and `min_coverage` exist on each side. `class` maps a positional argument, per the §3.3 rule.
- The 54 MCP tools in README/CLAUDE.md are correct: 50 literal registrations plus `tb_red_search`, `tb_red_load`, `tb_blue_search`, `tb_blue_load` built in `internal/registry/secplay.go`.
- `readSource` (symlinks and FIFOs) is in `internal/graph/index_common.go`, not in the vault; the vault spec's Q5 records it for the graph walk.
- `expiresAt` (`internal/egm/vendor.go:178`) reads the vendor's credential file; it is not a tb field and stays camelCase.
- Lint failures in the audit's sample (gocritic `filepathJoin` in `busrun/routertask_test.go`, `secplay`) came from PR branches; `main` is lint-clean today.
- The macOS `sha256sum` and Windows leading-`\` problems are already handled in `scripts/ci-smoke-model.sh` and `release.yml`.
