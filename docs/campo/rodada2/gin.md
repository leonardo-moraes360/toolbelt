🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/gin.md)

# Field study round 2: gin-gonic/gin

Study for the group **without existing AI-agent instrumentation**, using `tb` (toolbelt @ `a46c295`) end to end on a local clone of [gin-gonic/gin](https://github.com/gin-gonic/gin). No push, issue, PR, or comment was made on the target repository.

## Repository

| Field | Value |
|-------|-------|
| Name | `gin-gonic/gin` |
| Language | Go (`go 1.26.0` in `go.mod`) |
| Size | shallow clone ~2.6 MiB; **100** `.go` files after the task; ~**24 579** LOC Go |
| Clone base commit | `0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646` (shallow master) |
| Existing AI instrumentation | **None** — no `AGENTS.md`/`CLAUDE.md`/`.cursorrules`/`.copilot*`/`.aider*`/`GEMINI.md` on the upstream tree |
| Toolchain | **Partially ok** |
| Group | without existing AI-agent instrumentation |

Detailed toolchain:

- `go test ./...` — **ok** (~30 s first run with download; ~2 s afterward).
- `go build ./...` — **ok**.
- `go vet ./binding/...` — **ok**.
- `golangci-lint`: **not installed** on this VM (`command not found`); not configured.
- Headless browser (`google-chrome` at `/usr/local/bin/google-chrome`) present; visual ruler exercised with missing refs (aggregated skip).

Not configured: `tests/hidden/` suite (does not exist in gin); golangci-lint.

## Chosen task

| Field | Value |
|-------|-------|
| What | Preserve the **input index** in JSON slice validation errors (`[1]:` instead of `[0]:` when the 2nd element fails) |
| Why | Realistic, small bug with a documented failing test; distinct from round 1 |
| Issue | https://github.com/gin-gonic/gin/issues/4928 (there is unrelated PR #4929 — **we did not interact**; independent implementation on the local clone only) |
| Completed? | **Yes**, on the local clone |
| Main diff | `binding/default_validator.go` (pre-allocates `SliceValidationError` with the slice size and stores errors at the original position); test `TestJSONSliceValidationReportsInputIndex` in `binding/slice_index_validation_test.go` |
| Native checks | `go test ./binding/...` PASS; `go test ./...` PASS; `go build ./...` PASS; `go vet ./binding/...` PASS |
| Round 1 (comparison) | Round 1 used issue #4296 — include field name in `BindUnmarshaler` / `encoding.TextUnmarshaler` errors |

## Verification of the 12 fixes

| # | fix | worked / did not work / not tested / not applicable | evidence |
|---|-----|-----------------------------------------------------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `TB_GRAPH=on tb graph index` from `/tmp/target` (basename `target`) → **729** nodes / **2573** edges and `search ValidateStruct` finds `binding/default_validator.go`. Child `target/` under `/tmp/target-skipprobe` remains ignored (`Hello` empty; `Root` ok). |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` directory; config with `hidden_test_cmd: go test ./binding/ -run TestDoesNotExistHiddenProbe -count=1` → JSON `hidden_available: true`, `hidden_passed: true`, `verdict: PASS`. |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff touching `.toolbelt/thresholds.yaml`: `--json` alone → EXIT **0** with violations; `--json --strict` → EXIT **1** with the same violation JSON. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** (with a glob caveat) | `.toolbelt/guard.yaml` with `secrets/*` → violation on `secrets/token`. Pattern `secrets/` (trailing slash only) **does not** match via `filepath.Match` — needs `secrets/*` or a literal path. |
| 5 | init toolchain detection | **worked** | On gin and a probe with only `go.mod`: `verify.test_cmd: go test ./...`, `hidden_test_cmd: ""`. Does not fill `build_cmd`/`lint_cmd` by itself (still manual). |
| 6 | review `tests/hidden` matches by path | **worked** | Diff with only comment `// note about tests/hidden suite` in `default_validator.go` → findings `[]` (no critical invariant FP). |
| 7 | bench `default_agent` and `compare --agent` | **worked** | `bench.default_agent: "echo agent-default"` → `tb bench run -r 1 ...` starts `agent=external` without requiring `--agent`/`--fake` (task `bugfix-go` 100%). `tb bench compare ... --agent 'echo hi'` accepted (EXIT 0 on toolbelt with A/B configs). |
| 8 | review `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** (partial) | **Markdown-only** ticket with `## Escopo` + lists → scope ok (no `scope-missing`); `binding/**/*.go` in drift EXIT 0. With **YAML frontmatter without `scope:`**, the body `## Escopo` is **ignored** (`LoadTicket` returns early) → false `scope-missing`. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | `build_ran/passed: true`, `lint_ran/passed: true` in `/tmp/verify-gin-r2-ok.json` with `go build ./...` and `go vet ./binding/...`. |
| 10 | init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents` | **worked** | Without files: log “use `--write-agents`”. With `--write-agents`: creates `AGENTS.md` + `.toolbelt/agent_docs.yaml` (`sources: [AGENTS.md]`). Later re-init reads the existing file (does not overwrite). |
| 11 | graph Python `calls` edges | **not applicable** (auxiliary smoke **worked**) | Gin is Go. Probe `/tmp/py-calls-probe/mod.py`: `edges_calls: 1`; `callers --function helper` → `mod.main`. |
| 12 | aggregated visual skipped + CLI docs | **worked** | `.toolbelt/screens.yaml` with 2 screens without refs → top-level `skipped: true`, `skip_reason: "todas as telas foram skipped"`, each screen `skipped: true`. `docs/cli.md` documents the behavior (L762–763). |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init --write-agents` | EXIT 0; Go config + seeded `AGENTS.md` (mode 600) | **helped** | ~4 ms; `test_cmd=go test ./...`; agent_docs recorded |
| `tb mem init/add/list/check` | EXIT 0; 3 defaults + 1 rule; `compactacao_segura: true` | helped (light) | stable sha256 in `invariants.yaml` |
| `tb graph` without enable | (not re-probed in detail; ADR-011 kept) | neutral | requires `--enable`/`TB_GRAPH` |
| `tb graph --enable index/rebuild/stats` | 729→733 nodes after fix; ~35–45 ms | **helped** | fast index on gin |
| `tb graph search ValidateStruct` | finds `ValidateStruct`/`validateStruct` | helped | better than round 1 for this symbol |
| `tb graph impact/deps --file binding/default_validator.go` | limited impact (`Error`); pkg deps ok | neutral/partial | impact not very rich for the file |
| `tb graph callers --function ValidateStruct` | `(vazio)` | hindered | method on interface/`validate()` not wired as useful calls |
| manual ticket `rd.md` | created by hand (no `tb ticket`) | neutral | `## Escopo` + globs |
| `tb drift` | EXIT 0 on scope `binding/**/*.go` / `binding/` | **helped** | no out_of_scope on the fix diff |
| `tb verify run` (tests+build+lint+hidden) | full PASS | **helped** | build/lint/hidden now execute |
| `tb verify` + guard on fix path | immediate FAIL when `guard.yaml` listed `binding/default_validator.go` | hindered (if misconfigured) / helped (protection) | `iterations: 0`, build does not even run |
| `tb verify --ruler visual` | PASS with aggregated `skipped: true` | neutral | docs aligned; gin has no real UI |
| `tb review` (fix diff, MD ticket) | EXIT 0; weak acceptance warning possible | helped (partial) | no `tests/hidden` FP |
| `tb review` + YAML frontmatter | false `scope-missing` | **hindered** | Markdown body ignored |
| `tb guard check A..B --json/--strict` | clean on fix diff; flags thresholds/secrets | **helped** | `base..head` syntax (not `--base`) |
| `tb bench run` with `default_agent` | honors default; needs tasks | helped | with toolbelt `-t`: bugfix-go 100% |
| `tb bench compare --agent` | flag accepted; A/B runs | **helped** | round-1 regression fixed |
| `tb mcp` + `TB_GRAPH=on` | init+tools/list ok; graph tools; no mem/verify/review | neutral/partial | `tb_mem_list` → tool not found |
| `tb version` (`make build`) | `a46c295` | neutral | |

## Problems that persist

From round 1 (`docs/campo/gin.md`) that **still happen**:

| problem | status this round | evidence |
|---------|-------------------|----------|
| MCP without mem/verify/review/guard/drift | **persists** | `tools/list` only version/echo/env/run + graph_*; `tb_mem_list` error |
| Inconsistent positional flags on graph (`callers`/`deps`/`impact`) | **persists** (not exhaustively revalidated; API still flag-only in help) | callers help requires `--function` |
| `graph callers` not very useful / incomplete search for some symbols | **persists** (partial) | `callers ValidateStruct` empty |
| Config files mode `0600` | **persists** | `stat` → `600` on `config.yaml` and `AGENTS.md` |
| `version` unknown without ldflags | **not retested** with a raw build; `make build` ok | — |
| CLI docs still list unimplemented commands (`run`/`proc`/`ticket`/…) | **persists** (partial) | tree at top of `docs/cli.md` still shows planned items; Phase 2 sections improved |
| No `tb ticket` / progress checklist | **persists** | manual ticket |
| `**` glob / protected-path matching poorly documented in practice | **persists** (new nuance) | `secrets/` ≠ `secrets/*` in guard |

Round 1 problems **fixed** on this base: root SkipDirs; hidden without dir; guard strict+json; guard.yaml; default_agent / compare --agent; build/lint in verify; review path match; Markdown Scope (when no frontmatter); visual skipped; init toolchain + `--write-agents`.

## New problems

1. **P1 — `LoadTicket` with YAML frontmatter ignores `## Scope`/`## Escopo` in the body**  
   - Repro: `rd.md` with `---` frontmatter (title/base/forbidden) + `## Escopo` section with lists → `tb review` emits `scope-missing`. Markdown-only ticket with the same section works.  
   - Expected: merge body scope when YAML does not declare `scope`.  
   - Actual: `parseTicketFrontmatter` returns early without calling `parseMarkdownScope`.

2. **P2 — `protected_paths: secrets/` (directory) does not protect children**  
   - Repro: guard.yaml with only `secrets/`; commit `secrets/token` → `violations: null`. With `secrets/*` or `secrets/token` → violation.  
   - Expected: documented directory convention or prefix match.  
   - Actual: only literal `filepath.Match`.

3. **P2 — absolute `bench.tasks_dir` in YAML can be shadowed by the `-t` flag default**  
   - Repro: config with `tasks_dir: /agent/repos/toolbelt/bench/tasks` but `tb bench run` still logs `tasks_dir=bench/tasks` until `-t` is passed explicitly.  
   - Expected: local YAML wins when the flag is at its default.  
   - Actual: operator must repeat `-t`/`-f`.

4. **P3 — `tb guard check` UX `base..head` vs review/drift `--base` flags**  
   - Repro: `tb guard check --base A --head B` → `unknown flag: --base`.  
   - Expected: flag parity across diff subcommands.

5. **P3 — Go file impact not very informative**  
   - Repro: `impact --file binding/default_validator.go` lists only `binding.Error`.  
   - Expected: callers of `ValidateStruct` / uses via `validate()`.

## What tb caught and what it let slip

**Caught**

- Scope drift ok on package `binding/` (including `**` glob in the MD ticket).
- Guard on `.toolbelt/thresholds.yaml` (default) and `secrets/*` (yaml).
- Verify abort when the task file itself was in `protected_paths`.
- Build + lint + tests + hidden_cmd in verify (PASS on the fix).
- Review without FP from a textual mention of `tests/hidden`.
- Visual aggregating `skipped: true` when every screen skips.
- Onboarding with `AGENTS.md` via `--write-agents` in a repo without instrumentation.

**Let slip / failed**

- Markdown scope hidden behind YAML frontmatter (false warning / risk of accepting drift).
- `protected_paths` with a naïve directory pattern (`secrets/`).
- Incomplete call graph for the task’s central symbol (`ValidateStruct`).
- MCP still does not replace the CLI for mem/verify/review/guard.
- Real gin lint (`golangci-lint`) — verify used only manually configured `go vet`; golangci absent on the VM.

Native comparison: `go test ./...` and `go build ./...` validated the fix; tb verify aligned with build/lint stopped masking compile failures (clear improvement vs round 1).

## Interaction with existing instrumentation / onboarding

Gin **has no** upstream agent instrumentation. Assessment of `tb` bootstrap this round:

| Artifact | Generated / read? | Quality |
|----------|-------------------|---------|
| `.toolbelt/config.yaml` | yes (`tb init`) | Go detection ok (`test_cmd`); build/lint still manual |
| `AGENTS.md` | yes with `--write-agents` | useful minimum pointing at verify/guard/mem; mode 600 |
| `.toolbelt/agent_docs.yaml` | yes | records sources; `hints: []` (nothing to extract in gin) |
| `CLAUDE.md` | n/a (absent) | init advises seeding if desired |
| `mem` invariants | yes | generic anti-cheat defaults still poorly aligned to an HTTP lib without a hidden suite |
| Graph | on demand | fast; root SkipDirs fixed |
| MCP | available | subset; graph with `TB_GRAPH=on` |

Conclusion: onboarding **improved** vs round 1 (toolchain + AGENTS), but still does not import gin’s real CI (`.github/workflows/`) nor suggest `golangci-lint` automatically.

## Comparison with round 1

| dimension | round 1 | round 2 | verdict |
|-----------|---------|---------|---------|
| Binary | `c8aae04` | `a46c295` | base with the 12 fixes |
| Task | #4296 BindUnmarshaler field name | #4928 slice validation index | different (ok) |
| verify build/lint | ignored | executed and PASS | **better** |
| hidden_test_cmd | blocked without `tests/hidden/` | runs without the dir | **better** |
| guard.yaml / strict+json | broken | work | **better** |
| default_agent / compare --agent | broken | work | **better** |
| review FP `tests/hidden` | confirmed | did not reproduce | **better** |
| visual skipped top-level | misleading `skipped: false` | `skipped: true` | **better** |
| init AGENTS / toolchain | absent / generic | detects Go + `--write-agents` | **better** |
| SkipDirs `/tmp/target` | 0 nodes | 729 nodes | **better** |
| MCP parity | incomplete | incomplete | **same** |
| Scope + frontmatter | N/A (older different bug) | new YAML+MD merge gap | **worse** in this corner |
| Evidence CLI wall time | ~5.2 min | ~3.8 min (`12:37:23Z`→`12:41:12Z`) | similar / slightly **better** |
| Local task conclusion | yes | yes | **same** |

Overall: **better** than round 1 for the verify/guard/bench/init/visual/root-graph flow; MCP, ticket ergonomics with frontmatter, and graph richness remain.

## Improvement suggestions

1. **[review] P0** — When loading a ticket with frontmatter, if YAML `scope` is empty, merge with `parseMarkdownScope` from the body (`## Scope`/`## Escopo`).
2. **[guard] P1** — Treat patterns with a trailing `/` (or without `*`) as a directory prefix; document `secrets/*` vs `secrets/` examples.
3. **[bench] P1** — Make local config `tasks_dir`/`fixtures_dir` override cobra flag defaults.
4. **[mcp] P1** — Expose mem/verify/review/guard/drift (CLI parity).
5. **[graph] P2** — Improve `calls` edges for exported functions used via helpers (`validate` → `ValidateStruct`); enrich `impact --file`.
6. **[init] P2** — Suggest `build_cmd`/`lint_cmd` from CI/Makefile/`golangci` when detectable.
7. **[cli] P2** — Unify commit-range flags (`--base`/`--head` vs `A..B`) across guard/review/drift.
8. **[config] P3** — Revisit mode 0600 on `AGENTS.md`/non-secret configs.

## Metrics

| metric | round 2 | round 1 (ref.) |
|--------|---------|----------------|
| Wall time (structured CLI evidence) | **~3.8 min** (`2026-11-16T12:37:23Z` → `12:41:12Z`); total agent session larger (docs+clone+implementation+probes) | ~5.2 min evidence |
| `tb` invocations with `CMD:` in the log | **48** (+ auxiliary runs outside the prefix) | ~50 |
| Recorded `tb` failures (EXIT ≠ 0) | **7** (intentional probes: guard strict, review drift/FP setup, bad guard flags, etc.) | 11 |
| Binary crashes | **0** | 0 |
| `tb` build used | `make build` → `toolbelt a46c295` | `c8aae04` |

Raw evidence files on this VM (not committed): `/tmp/campo-gin-r2-evidence.log`, `/tmp/verify-gin-r2*.json`, `/tmp/review-*.json`, `/tmp/verify-visual*.json`, clone at `/tmp/target/gin` (local commits only).
