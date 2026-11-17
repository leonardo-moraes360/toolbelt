[🇧🇷 Português](../../campo/gin.md) | 🇺🇸 English

# Field study: gin-gonic/gin

Study of the group **without AI agent instrumentation**, using `tb` (toolbelt @ `c8aae04`) end-to-end on a local clone of [gin-gonic/gin](https://github.com/gin-gonic/gin). No push, issue, PR, or comment was made on the target repository.

## Repository

| Field | Value |
|-------|-------|
| Name | `gin-gonic/gin` |
| Language | Go (`go 1.26.0` in `go.mod`) |
| Size | shallow clone ~1.7 MiB; **99** `.go` files; ~**24 574** Go LOC |
| Clone base commit | `0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646` (shallow master) |
| Existing AI instrumentation | **None** — no `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.copilot*`, `.aider*`, `GEMINI.md` |
| Toolchain | **Partially ok** |

Detailed toolchain:

- `go test ./...` — **ok** (~31 s on 1st run with dep download; ~2 s after).
- `go build ./...` — **ok**.
- `golangci-lint`: v1.64.8 **failed** (`Go language version (go1.24) ... lower than ... 1.26.0`). v2.15.0 (built with go1.27) **ran**; reported 6 pre-existing issues in the tree (gofumpt/gosec/govet), not introduced by the task.
- Headless browser (`google-chrome`) present, but visual capture failed in the environment (dbus/socket; process killed) — visual ruler only exercised in skip mode.

Not configured / not used: `tests/hidden/` suite (does not exist in gin); special App Engine tags beyond what standard tests cover.

## Chosen task

| Field | Value |
|-------|-------|
| What | Include the **field name** (`form`/`uri` tag) in error messages from `BindUnmarshaler` / `encoding.TextUnmarshaler` |
| Why | Realistic enhancement, small scope, with a documented failing test; maintainer would plausibly accept |
| Issue | https://github.com/gin-gonic/gin/issues/4296 (there is third-party PR #4435 — **we did not interact**; independent implementation only on the local clone) |
| Completed? | **Yes**, on the local clone |
| Main diff | `binding/form_mapping.go` (`wrapFieldError` + `trySetCustom`/`trySetUsingParser` signatures); test `TestMappingCustomUnmarshalParamErrorIncludesFieldName` in `binding/form_mapping_test.go` |
| Native checks | `go test ./binding/...` PASS; `go test ./...` PASS; `go build ./...` PASS; `golangci-lint run ./binding/...` FAIL due to **pre-existing** issues (not newly relevant on the changed file) |

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb init` | EXIT 0; created `.toolbelt/config.yaml` (mode **600**); defaults `verify.test_cmd=go test ./...`, `hidden_test_cmd=go test ./tests/hidden/...` | neutral | ~5 ms; did not detect gin beyond the Go default; did not generate `AGENTS.md` or lint/build |
| `tb mem init/add/list/check` | EXIT 0; 3 default invariants + 1 added; `mem add` is positional (`<regra>`), joins args with space | helped (light) | `mem check` sha256 and `compactacao_segura: true`; mode 600 on `invariants.yaml` |
| `tb graph` (without `--enable`/`TB_GRAPH`) | EXIT 1: “grafo desabilitado” | neutral | ADR-011 ok, but UX requires the flag on every call |
| `tb graph --enable index/rebuild/stats` | EXIT 0; ~726→731 nodes, ~2572→2581 edges | helped | index ~44–48 ms on gin |
| `tb graph search BindUnmarshaler` | EXIT 0, **(vazio)** even after rebuild | hindered | interface/type name not indexed/findable |
| `tb graph search wrapFieldError` | finds function after rebuild | helped | needs rebuild after changes |
| `tb graph callers` positional | EXIT 1: `--function é obrigatório` | hindered | inconsistent with `search` (positional) |
| `tb graph deps` positional | EXIT 1: `--module ou --file é obrigatório` | hindered | same |
| `tb graph impact` without `--file` | EXIT 1: `--file é obrigatório` | hindered | confirms known problem |
| `tb graph impact --file binding/form_mapping.go` | EXIT 0; lists useful callers/impacted | helped | setByForm, MapFormWithTag, Bind, etc. |
| ticket/`rd.md` + Scope | created manually (no `tb ticket`) | neutral | YAML frontmatter + markdown; clone base SHA |
| `tb verify run` (tests) | PASS with `--test-cmd go test ./binding/...` | helped (partial) | does not run lint/build; YAML `lint_cmd`/`build_cmd` **ignored** (schema only has `test_cmd`/`hidden_test_cmd`) |
| `tb verify` + hidden | PASS; `Oculto: false (disponível: false)` without `tests/hidden/` | neutral | default points to nonexistent path in gin |
| `tb verify --ruler visual` | PASS with screens skipped; top-level `skipped: false` when refs/capture fail | hindered | see known/new problems |
| `tb review` | PASS on fix diff; **FP critical** if the diff only *mentions* `tests/hidden` in a comment | hindered (FP) | agreement 3/3 goldfish on the FP |
| `tb drift` | EXIT 0 on literal/`binding/`/`binding/*.go` scope; EXIT 1 on wrong scope and on `binding/**/*.go` | helped | mid-path `**` does not work as expected |
| `tb guard check` | clean on fix diff; violates `protected_path` on `.toolbelt/thresholds.yaml` | helped | hardcoded paths; repo `guard.yaml` **ignored** |
| `tb bench run` without `--agent`/`--fake` | EXIT 1 despite `bench.default_agent` in config | hindered | `default_agent` ignored |
| `tb bench run --fake` | runs without `--fake-config`, but **does not apply solutions** → 0% pass | hindered | needs fake-config with `files_to_modify` to be useful |
| `tb bench compare` | A/B with `--fake` ok; **does not accept `--agent`** | hindered | unknown flag |
| `tb mcp` | starts on stdio; `initialize`+`tools/list` ok; with `TB_GRAPH=on` exposes graph tools; **without** mem/verify/review/guard/drift | neutral/partial | `tb_mem_list` → “Tool not found” |
| `tb version` (make build) | shows commit `c8aae04` | neutral | |
| `tb version` (`go build` without ldflags) | `0.1.0-dev`, commit/built **unknown** | hindered | |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare ... --agent "echo hi"` → unknown flag; help only has `--fake` |
| `guard.yaml` ignored | **confirmed** | `.toolbelt/guard.yaml` with `binding/form_mapping.go` protected; `guard check` on fix diff → `violations: null`; docs/cli.md still document the manifesto |
| outdated CLI docs | **confirmed** | `docs/cli.md` describes `run/proc/ticket/board/md/agent/watchdog/merge`; `tb --help` does **not** list them. README “Status do Projeto” still says **Fase 0** while roadmap marks Fase 2 |
| version shows `unknown` | **confirmed** (build without ldflags) | `/tmp/tb-nover/tb version` → commit/built `unknown`; `make build` fills |
| config files created 0600 | **confirmed** | `stat` → `600 .toolbelt/config.yaml`, `600 .toolbelt/invariants.yaml` |
| visual ruler reports `skipped=false` when skipping | **confirmed** | with manifesto+browser, screen `skipped:true` (“referência ausente” / “falha na captura”), top-level `"skipped": false`, `verdict: PASS` |
| `default_agent` ignored | **confirmed** | config with `default_agent: echo-agent-should-be-used`; `tb bench run` → “`--agent` ou `--fake` é obrigatório” |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | touch `.toolbelt/thresholds.yaml` → guard `protected_path` error; verify `verdict: FAIL`, `iterations: 0` |
| default fake agent needs `--fake-config` | **confirmed** (to be useful) | `--fake` alone completes “agent” but verification fails (0% pass) without `files_to_*` |
| `mem add` positional | **confirmed** | `Use: tb mem add <regra>`; `MinimumNArgs(1)` + `strings.Join(args," ")` |
| Drift/Scope need literal paths (no globs) | **partially refuted** | `binding/*.go` and `binding/` work; `binding/**/*.go` **fails** (Match does not treat mid-path `**`) |
| `graph impact` needs `--file` | **confirmed** | positional rejected; `--file` required |
| review false positives on tests/hidden invariant | **confirmed** | comment containing substring `tests/hidden` → critical finding “possível violação de invariante” (agreement 3) |
| verify runs only partial checks / misses build errors | **confirmed** | `gin.go` with syntax error: `go build ./...` FAIL, `go test ./binding/...` PASS → verify PASS; schema without lint/build |
| tb does not push agent to complete whole task | **not tested** (no controlled A/B on this repo) | qualitative observation: init does not create progress log / feature checklist; no `tb ticket`/`board`; flow depends on the agent self-organizing |

## New problems

1. **P1 — `graph search` does not find interface/type names**  
   - Repro: `tb graph --enable rebuild && tb graph --enable search BindUnmarshaler` → `(vazio)` while `wrapFieldError`/`mappingByPtr` appear.  
   - Expected: find the interface in `binding/form_mapping.go`.  
   - Actual: empty.

2. **P1 — MCP does not expose mem/verify/review/guard/drift**  
   - Repro: `tools/list` (with `TB_GRAPH=on`) → only `tb_version/echo/env/run` + graph_*; `tools/call tb_mem_list` → “Tool not found”.  
   - Expected: parity with Phase 1/2 CLI.  
   - Actual: minimal subset.

3. **P2 — Inconsistent positional flags on graph**  
   - Repro: `tb graph --enable callers trySetCustom` / `deps binding/form_mapping.go` fail; `search` accepts positional.  
   - Expected: same ergonomics.  
   - Actual: callers/deps/impact only via flags.

4. **P2 — YAML `lint_cmd`/`build_cmd` silently ignored**  
   - Repro: write in `.toolbelt/config.yaml`; `VerifyConfig` only has `test_cmd`/`hidden_test_cmd`.  
   - Expected: unknown-config error or execution.  
   - Actual: silent ignore.

5. **P2 — `tb review` without shorthand `-o` (verify has `-o`)**  
   - Repro: `tb review ... -o /tmp/x.json` → unknown shorthand; only `--output`.  
   - Expected: aligned output flags.  
   - Actual: UX inconsistency.

6. **P3 — Documentable/unexpected `**` glob**  
   - Repro: scope `binding/**/*.go` marks out_of_scope; `binding/*.go` ok.  
   - Expected: either `**` support, or clear docs “only `*` and prefix `dir/` / `dir/**`”.  

7. **P3 — golangci / Go 1.26 on onboarding**  
   - Repro: old linter incompatible with gin’s `go 1.26.0`.  
   - Expected: `tb init` warn about toolchain or suggest detected lint command.  
   - Actual: no hint.

## What tb caught and what it let slip

**Caught**

- Scope drift when files leave Scope / hit `forbidden` (`tests/hidden/`).
- Guard on hardcoded path `.toolbelt/thresholds.yaml` and verify abort.
- Impact of `binding/form_mapping.go` via graph (after `--enable` + `--file`).
- Clean review on the legitimate fix diff (scope+base ok).
- Invariants persisted outside compaction (`mem check`).

**Let slip / failed**

- **Build** error in the root package while `test_cmd` is only `./binding/...` (verify PASS).
- Pre-existing lint / any lint (verify does not execute).
- Textual mention of `tests/hidden` → **false positive** in review.
- Custom project `guard.yaml`.
- Interface name `BindUnmarshaler` in graph search.
- Real visual capture (graceful skip with misleading signal in top-level `skipped` field).

Native comparison: `go test ./...` and `go build ./...` validated the fix; tb verify configured in a “realistic per-package” way would mask a compilation regression outside that package.

## Interaction with existing instrumentation / onboarding

Gin **has no** agent instrumentation. Evaluation of `tb` bootstrap:

| Artifact | Generated by `tb init` / `mem init`? | Quality |
|----------|--------------------------------------|---------|
| `.toolbelt/config.yaml` | yes | useful as skeleton; generic Go defaults; graph languages on; **no** detection of gin’s `golangci-lint`/Makefile/CI |
| `.toolbelt/invariants.yaml` | via `mem init` | generic anti-cheat defaults (`tests/hidden/`, `thresholds.yaml`) poorly aligned with an HTTP lib without a hidden suite |
| `AGENTS.md` / skills / progress log / features JSON | **no** | gap vs findings (harness-style `tb init`) |
| Automatic test/lint/build detection | **no** | default `test_cmd` `go test ./...` useful by happenstance; lint/build not |
| Graph | on demand | fast on gin; requires `--enable`; incomplete search for types |
| MCP | available | does not replace full CLI; graph tools only with `TB_GRAPH=on` |

Conclusion: onboarding **does not** leave an agent “ready” on a repo without instrumentation — missing project briefing, real gin CI commands (workflow `.github/workflows/gin.yml`), and incomplete MCP surface.

## Improvement suggestions

Ordered by impact; tb area tag:

1. **[verify] P0** — Run (or require) `build_cmd`/`lint_cmd` beyond tests; fail if the module does not compile. Reject or warn on unknown YAML keys.
2. **[review] P0** — `tests/hidden` invariant: match by changed path, not substring in the diff/comment.
3. **[guard] P1** — Honor `.toolbelt/guard.yaml` (as docs promise) instead of only a hardcoded list.
4. **[bench] P1** — `bench compare` accept `--agent` (parity with `run`); honor `default_agent`; document that `--fake` without `--fake-config` does not resolve fixtures.
5. **[graph] P1** — Index/search types & interfaces; accept positionals on callers/deps/impact **or** document flags-only; auto-hint `--enable`.
6. **[init/onboarding] P1** — Detect test/lint/build commands (Makefile, CI, `go.mod`); optionally generate `AGENTS.md` + progress checklist.
7. **[verify/visual] P1** — If all screens are `skipped`, top-level `skipped=true` (and/or a verdict distinct from “full” PASS).
8. **[mcp] P2** — Expose mem/verify/review/guard/drift; CLI parity.
9. **[cli/docs] P2** — Sync `docs/cli.md` and README “Fase 0” with real commands; align `-o`/`--output`.
10. **[version] P3** — Embed version via `runtime/debug.BuildInfo` when ldflags are absent.
11. **[config] P3** — Revisit mode 0600 vs 0644 for non-secret configs in shared monorepos.

## Metrics

| metric | value |
|--------|-------|
| Wall time (active study timer until end of CLI evidence) | **~5.2 min** (`2026-11-15T20:45:21Z` → `20:50:17Z`); total agent session (docs+clone+implementation+MCP+writeup) longer |
| Recorded `tb` invocations (`CMD:` lines + extra MCP/visual/review) | **~50** (35 with explicit `CMD:` in the log + later visual/review/MCP/graph runs) |
| `tb` failures (EXIT ≠ 0) in structured log | **11** (mostly intentional probes: graph off, missing flags, drift, default_agent, compare --agent, verify abort by guard) |
| Binary crashes | **0** observed |
| `tb` build used | `make build` → `toolbelt c8aae04` |

Raw evidence files on this VM (not committed): `/tmp/campo-gin-evidence.log`, `/tmp/verify-*.json`, `/tmp/review-*.json`, clone at `/tmp/target/gin` (local commits only).
