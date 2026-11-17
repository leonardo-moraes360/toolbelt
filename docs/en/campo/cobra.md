[🇧🇷 Português](../../campo/cobra.md) | 🇺🇸 English

# Field study: spf13/cobra

Group: **without AI agent instrumentation**. Work only on local clone (`/tmp/target/cobra`). No interaction with upstream (no push, issue, PR, or comment).

`tb` used: local binary from `/agent/repos/toolbelt` (`make build`, version `c8aae04` with ldflags).

---

## Repository

| Field | Value |
|-------|--------|
| Name | [spf13/cobra](https://github.com/spf13/cobra) |
| Language | Go (`go 1.15` in `go.mod`; CI tests 1.17–1.24) |
| Size | shallow clone ~1.3 MiB on disk; **36** `.go` files; ~15–17 kLOC Go (wc on sources) |
| Existing AI instrumentation | **None** (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot, Windsurf, Aider: absent in the tree) |
| Toolchain | **Partially ok**: `go test ./...` and `go vet` ok; `gofmt` ok; `golangci-lint` installed via `go install` (official script failed checksum); `make lint`/`richtest` depend on the linter; CI Docker/`addlicense` **not** configured; did not run multi-version Go matrix |

---

## Chosen task

| Field | Value |
|-------|--------|
| What | Allow `SetErrPrefix("")` (empty prefix, no fallback to `"Error:"` and no leading space on print) |
| Why | Small feature, clear API, natural tests; maintainer discussed in the thread |
| Issue | https://github.com/spf13/cobra/issues/2226 |
| Completed? | **Yes, locally only** (commit `a6ef947` on the clone; **not** sent to GitHub) |
| Native tests | `go test ./...` → **PASS** (~0.2–1.1 s) |
| Native lint | `golangci-lint run ./...` → **FAIL** due to **pre-existing** issue in `completions.go` (`nolintlint`/gosec); ErrPrefix change introduced no findings |
| Native build | `go build ./...` → **PASS** on the task commit |

Implementation: `errPrefix *string`, `printErrWithPrefix`, tests `TestEmptyErrPrefix` and `TestEmptyErrPrefixOnSubcommand`.

---

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb init` | ok (0.006 s); creates `.toolbelt/config.yaml` mode **0600**, dirs `vault/`, `bench/`, `logs/` | **helped** (minimal bootstrap) | `verify.test_cmd: go test ./...` useful; empty vault; no `AGENTS.md`/lint/build |
| `tb version` | with `make build`: `c8aae04`; `go build` without ldflags: `0.1.0-dev`, **commit/built unknown** | neutral | confirms metadata only with ldflags |
| `tb mem init/list/add/check` | init generates 3 invariants; positional add ok; `--type/--title` **rejected** | **mixed** | `cli.md` docs outdated vs real CLI (`tb mem add <regra>`) |
| ticket/`rd.md` + Scope | **no** `tb ticket`; Scope only on the **same line** (`Scope: a, b`); Markdown list captures only the 1st bullet (`- command.go`) | **hindered** | `## Scope` + bullets → `scope-missing` or partial scope; `command_test.go`/`rd.md` marked out of scope |
| `tb graph --enable index/stats/search/rebuild` | index 467 nodes / 2108 edges (~0.03–0.05 s); search found `ErrPrefix`/`SetErrPrefix` | **helped** | located API quickly; without `--enable`/`TB_GRAPH=on` fails (ADR-011) |
| `tb graph callers/deps/impact` | require `--function` / `--file` (positional args fail); impact useful but noisy on `command.go` | **mixed** | `impact` without `--file` → exit 1; callers returned only the node itself |
| `tb verify run` (tests) | PASS on task diff (~0.2–0.5 s); only runs `test_cmd` (+ hidden); **ignores** YAML `lint_cmd`/`build_cmd` | **mixed** | with `go test ./standalone/...` → PASS while `go build ./...` broke (`broken_syntax.go`) |
| `tb verify --ruler visual` | without manifesto: PASS `skipped=true`; with manifesto and missing ref: PASS **`skipped=false`** in report, screen `skipped=true` | **hindered** | false “not skipped” in the aggregator |
| `tb review` | clean fix-only (findings `[]`); textual mention of `tests/hidden`/`thresholds.yaml` in the diff → **FP critical** | **mixed** | goldfish heuristic by substring, not by touched path |
| `tb drift` | ok with `Scope: command.go, command_test.go, rd.md`; simple globs `command*.go` **work** | **mixed** | fragile Scope UX; globs are not the worst problem |
| `tb guard check` | hardcoded `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`; **does not read** `.toolbelt/guard.yaml` | **mixed** | custom protection of `command.go` in YAML → **zero** violations |
| `tb verify` + hardcoded path | diff touching `.toolbelt/thresholds.yaml` → verify **FAIL** immediately (iterations 0), without running tests | **hindered** (in probe flow) | aborts even with custom `guard.yaml` |
| `tb bench run/list/compare` | list ok; `--fake` ok **without** `--fake-config`; YAML `default_agent` **ignored**; `compare` only has `--fake`, **no** `--agent` | **mixed** | `Error: --agent ou --fake é obrigatório` with `default_agent` set |
| `tb mcp` | stdio JSON-RPC ok; `tools/list` exposes graph/version/echo/env/run | **helped** (smoke) | stateless; graph tools require `TB_GRAPH=on` in the server process (not exercised beyond list) |
| native checks vs tb | tb verify PASS ≠ CI lint; narrow `test_cmd` hides build break | **hindered** if the agent trusts only tb | see section “What tb caught…” |

---

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare --help` lists `--fake`, not `--agent` |
| `guard.yaml` ignored | **confirmed** | YAML protected `command.go`; `guard check` on ErrPrefix diff → `violations: null` |
| outdated CLI docs | **confirmed** | `docs/cli.md` describes `tb mem add --type/--title` and `tb ticket`; CLI: positional add; `ticket` nonexistent |
| version shows `unknown` | **confirmed** (without ldflags) | `go build -o /tmp/tb-nolflags` → `commit: unknown`, `built: unknown` |
| config files created 0600 | **confirmed** | `.toolbelt/config.yaml` and `invariants.yaml` → `-rw-------` |
| visual ruler reports `skipped=false` when skipping | **confirmed** | manifesto present, ref missing: top-level `"skipped": false`, screen `"skipped": true` / `"referência ausente"` |
| `default_agent` ignored | **confirmed** | YAML `default_agent: echo AGENT_RAN {prompt}` → still requires `--agent` or `--fake` |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | touch `.toolbelt/thresholds.yaml` → verify FAIL, `iterations: 0` |
| default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake ... --task bugfix-go` PASS without `--fake-config` |
| `mem add` positional | **confirmed** | help: `tb mem add <regra>`; `--type` → `unknown flag` |
| Drift/Scope need literal paths (no globs) | **partially refuted** | `Scope: command*.go` accepted the `.go` files; real failure: multiline Markdown lists / one-line parser |
| `graph impact` needs `--file` | **confirmed** | without flag → `Error: --file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmed** | comment `// note about tests/hidden thresholds.yaml` → 2 critical findings without altering those paths |
| verify runs only partial checks / misses build errors | **confirmed** | `test_cmd=go test ./standalone/...` PASS with `broken_syntax.go` in the module; `go build ./...` FAIL |
| tb does not push completing the whole task | **confirmed (qualitative)** | verify PASS with tests was enough; tb did not demand lint/`golangci`, dedicated build, API docs, or acceptance checklist; agent had to remember native checks separately |

---

## New problems

### P1 — Scope parser ignores Markdown list and swallows the first bullet via `\s`/newline

- **Repro**: `rd.md` with
  ```
  Scope:
  - command.go
  - command_test.go
  ```
  then `tb drift --ticket rd.md --base <base> --head HEAD`
- **Expected**: scope = both files (or clear format error)
- **Actual**: regex `(?mi)^(?:scope|escopo)\s*:\s*(.+)$` lets `\s*` cross newline; captures `- command.go`; `splitList` becomes `["-", "command.go"]`; `command_test.go` comes out as out_of_scope

### P2 — `tb init` does not detect Makefile lint/build nor generate an agent guide

- **Repro**: `tb init` on cobra (has `Makefile` with `test`/`lint`/`fmt`)
- **Expected**: at least suggest `golangci-lint run` / `make test`, and/or minimal `AGENTS.md`
- **Actual**: only `verify.test_cmd: go test ./...`; empty vault; no lint/build; manual YAML `lint_cmd`/`build_cmd` are **ignored** by `verify`

### P3 — `graph callers` / positional args inconsistent with search UX

- **Repro**: `tb graph --enable callers ErrPrefix` vs `tb graph --enable search ErrPrefix`
- **Expected**: same positional convention or a message that cites the flag
- **Actual**: search accepts positional query; callers/deps/impact require flags and fail in a way easy to confuse with “empty graph”

### P3 — Extra fields in `config.yaml` (`lint_cmd`, `build_cmd`) silently inert

- **Repro**: write `verify.lint_cmd` / `verify.build_cmd` and run `tb verify run`
- **Expected**: use, reject schema, or warn
- **Actual**: verify only runs test/hidden; PASS without lint

---

## What tb caught and what it let slip

**Caught**

- Located ErrPrefix API via `graph search`
- Drift when Scope was well formatted on the same line
- Guard on hardcoded paths (`thresholds.yaml`, `tests/hidden/*`)
- Failure of `go test ./...` / broken build when `test_cmd` covered the package
- MCP starts and lists tools

**Let slip / did not cover**

- Pre-existing `golangci-lint` issue (verify does not run lint)
- Broken build if `test_cmd` is narrow (`./standalone/...`)
- Protections declared in user `guard.yaml`
- “Maintainer-level” completeness (public issue docs, changelog, multi-Go) — no nudge
- Review false positive from substring `tests/hidden` / `thresholds.yaml` in the diff text

---

## Interaction with existing instrumentation / onboarding

Repo **without** AI instrumentation. From-scratch bootstrap evaluation:

| Artifact | Generated? | Quality |
|----------|------------|---------|
| `.toolbelt/config.yaml` | yes | reasonable Go defaults (`go test ./...`); graph languages/frameworks too generic; `default_agent: ""`; hidden tests point to nonexistent path until the agent creates it |
| `.toolbelt/invariants.yaml` (via `mem init`) | yes | generic anti-cheat (`tests/hidden`, `thresholds.yaml`) — useful, but produce FPs if the text appears in the diff |
| `.toolbelt/vault/` | empty dir | no Cobra domain notes |
| `AGENTS.md` / skills / Makefile-aware | **no** | missing the main thing for a repo without instrumentation |
| Automatic test/lint/build detection | **partial** | only guesses `go test ./...`; did not read `Makefile`/`CI` |

Conclusion: onboarding **insufficient** for a “bare” repo; the agent still needs to invent ticket, Scope in the right syntax, and the full quality suite.

---

## Improvement suggestions

1. **[P0][review/drift]** Accept Scope as Markdown list / multi-item YAML frontmatter; reject bullets becoming token `"-"` — area: `internal/review`  
2. **[P0][verify]** Honor `lint_cmd`/`build_cmd` (or strict schema + warning); default: detect `Makefile`/CI workflow — area: `verify` + `init`  
3. **[P1][guard]** Load `.toolbelt/guard.yaml`; do not abort verify only for hardcoded path without mandate — area: `guard`/`verify`  
4. **[P1][init/mem]** Generate `AGENTS.md` + checklist (test/lint/build/Scope) and align `docs/cli.md` with real CLI — area: `init`, docs  
5. **[P1][verify/visual]** Aggregator report: `skipped=true` if all screens skipped — area: `internal/verify/visual.go`  
6. **[P2][review]** Match invariants by touched path, not substring in the diff — area: `review.suggestsViolation`  
7. **[P2][bench]** Respect `bench.default_agent`; add `--agent` to `bench compare` — area: `bench`  
8. **[P2][graph]** Unify positional args (`callers`/`deps`/`impact`) with `search` — area: `graph` CLI  
9. **[P3][version/config]** Document ldflags in README; 0644 permissions for non-secret config in shared clone — area: build/`init`

---

## Metrics

| metric | value |
|--------|--------|
| Wall time (tb evidence log birth→last) | ~**3.6 min** (20:44:44 → 20:48:19 UTC); full study incl. clone/build/lint install ~**6–8 min** |
| Sum of measured tb invocation times | **~3.32 s** |
| Recorded tb invocations | **64** |
| Invocations with exit ≠ 0 | **19** (mostly expected failure probes / known issues; not crashes) |
| tb crashes | **0** observed |
| Local commits on cobra clone (task) | 1 functional (`fix: allow empty ErrPrefix`) + later discardable probe commits |
| Interactions with target GitHub | **0** |
