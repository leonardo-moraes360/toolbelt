[🇧🇷 Português](../../campo/jq.md) | 🇺🇸 English

# Field study: jqlang/jq

Group: without AI agent instrumentation.  
Tool: `tb` built from this repo (`make build` → `bin/tb`, version with ldflags `c8aae04`).  
Target clone: `/tmp/target/jq` (local only; no push/issue/PR/comment upstream).  
Date: 2026-11-15.

## Repository

| Field | Value |
|-------|-------|
| Name | [jqlang/jq](https://github.com/jqlang/jq) |
| Language | C (autotools + embedded oniguruma) |
| Size | shallow clone + submodules ≈ 19–56 MiB on disk; ~27 kLOC in `src/` (`.c`/`.h`); 50 `.c`/`.h` files outside `vendor/`, 176 with vendor |
| AI instrumentation | None found (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot, Aider, Gemini). Only CI workflows in `.github/workflows/` |
| Toolchain | **Yes, complete**: `autoconf`/`automake`/`libtool`/`flex`/`bison` installed; `autoreconf -i`; `./configure --with-oniguruma=builtin --disable-docs`; `make -j`; `make check` (9/9 PASS at baseline, ~12 s). HTML docs not generated (`--disable-docs`). No Valgrind/scan-build in this session |

## Chosen task

| Field | Value |
|-------|-------|
| What | Reject JSON literals with a trailing decimal point (`23.`, `-23.`, `0.`) in `check_literal` (`src/jv_parse.c`), with regression in `tests/jq.test` via `fromjson` |
| Why | Clear bug, small scope, aligned with RFC 8259 (`frac = decimal-point 1*DIGIT`), maintainer-plausible |
| Issue | https://github.com/jqlang/jq/issues/2414 |
| Complete? | **Yes** (locally). Previous attempt of #3128 (maintainer patch) **failed** (assertion `jq->subexp_nest > 0`, 5 suites FAIL) and was reverted — evidence that tb did not push to complete/fix the first task |
| Native checks | After #2414 fix: `make check` → 9/9 PASS (~7.9 s). `printf '23.' \| ./jq .` → exit 5, `Invalid numeric literal`; `23.0` still OK. No dedicated C linter in the repo beyond what CI/scanbuild would do (not run here) |

Ticket used (manual, `tb ticket` does not exist): `rd.md` with frontmatter `scope` / `base_commit` / `acceptance`.

## tb usage by command

| command | result | helped/hindered/neutral | evidence |
|---------|--------|-------------------------|----------|
| `tb version` | OK (`c8aae04` with ldflags); `go build` without ldflags → `0.1.0-dev` / `commit: unknown` | neutral | `/tmp/tb-nolf version` showed `unknown`; Makefile binary did not |
| `tb init` | Created `.toolbelt/` + `config.yaml` (0600), dirs `vault`/`logs`/`bench` | helped | exit 0 in 5 ms; defaults `verify.test_cmd: go test ./...` inadequate for C |
| `tb mem init/list/add/check` | init generated 3 invariants; positional add OK; `--type`/`--title` flags fail | helped / hindered (docs) | `mem add --type fact …` → `unknown flag: --type`; `mem add "…"` OK; `mem check` sha256 + `compactacao_segura: true` |
| `tb graph --enable index/rebuild/stats/search` | Indexed 1064 nodes / 1530 edges in ~0.1 s; search found `gen_destructure` | helped (partial) | C indexed via `cpp`; static `bind_alternation` did not appear in search |
| `tb graph callers/deps/impact` | callers requires `--function`; deps `--file`; impact requires `--file` and returned empty/`null` for `src/compile.c` | hindered | confusing UX (positional args rejected); impact with no practical value in this repo’s C |
| Ticket + Scope (`rd.md`) | Manual; `tb ticket` nonexistent | hindered | `tb ticket` → `unknown command`; docs/cli.md still document the command |
| `tb verify run --ruler tests` | With `--test-cmd 'make check'` → PASS (~8 s); Go default would be useless | helped (if configured) | JSON: `visible_passed: true`, `hidden_available: false`. With `--test-cmd './jq -n 1'` also PASS without exercising the suite — verify does not force build |
| `tb verify run --ruler visual` | PASS with screens `skipped: true` (missing ref), but report `"skipped": false` | hindered | Chrome present; example manifesto; top-level `skipped` lies |
| `tb review` | Exit 1; FP on `tests/hidden` invariant because `rd.md` mentions `tests/hidden/**` in the diff | hindered | Critical finding even without touching `tests/hidden/`; also flagged `rd.md` out of scope (correct) |
| `tb drift` | Detected `rd.md` out of scope; globs `src/*.c` and `tests/**` **work** | helped | Literal and glob tested; `src/*.c` correctly excluded `tests/jq.test` when only C was in scope |
| `tb guard check` | No violations on the fix; violates `protected_path` on `.toolbelt/thresholds.yaml` (hardcoded). Custom `guard.yaml` ignored. Exit 0 even with violation; `--strict --json` also exit 0 (JSON early return) | hindered | `.toolbelt/guard.yaml` with `src/jv_parse.c` protected → still `violations: null` on the parse diff |
| `tb bench run --fake` | OK if pointing at toolbelt tasks; on jq fails without `bench/tasks` | neutral | 9 tasks, 33.3% success, ~4 s; `--fake` did **not** require `--fake-config` |
| `tb bench compare` | No `--agent` flag; A/B only via `--fake` or JSONL | hindered | `tb bench compare --agent x` → `unknown flag: --agent` |
| `tb mcp` | Starts; `initialize` + `tools/list` via JSON-RPC newline | helped | tools: `tb_graph_*`, `tb_version`, `tb_echo`, `tb_env`, … |
| Native checks vs tb | `make check` caught the #3128 regression (crash); verify configured only with `./jq -n 1` would let it pass | — | See section “What tb caught…” |

## Known problems

| problem | confirmed/refuted/not tested | evidence |
|---------|------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | help without `--agent`; flag → `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmed** | file with `protected: [src/jv_parse.c]`; `guard check` on that file’s diff → `violations: null` (only hardcoded) |
| Outdated CLI docs | **confirmed** | `docs/cli.md`: `tb ticket`, `mem add --type/--title`, `guard --manifest`; real CLI diverges |
| `version` shows `unknown` | **confirmed** (without ldflags) | `go build -o /tmp/tb-nolf` → `commit: unknown` / `built: unknown` |
| Config files 0600 | **confirmed** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → mode `600` |
| Visual ruler `skipped=false` when skipping | **confirmed** | visual verify: screens `skipped: true`, top-level `"skipped": false`, `verdict: PASS` |
| `default_agent` ignored | **confirmed** | `bench.default_agent: echo-should-be-used` in config; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| Guard aborts verify when touching hardcoded config path | **confirmed** | commit on `.toolbelt/thresholds.yaml` → verify `FAIL`, `iterations: 0`, `protected_path` |
| Default fake agent needs `--fake-config` | **refuted** | `tb bench run --fake` without `--fake-config` ran (uses `FakeAgentCommand("")`) |
| `mem add` positional | **confirmed** | CLI: `tb mem add <regra>`; docs with named flags fail |
| Drift/Scope need literal paths (no globs) | **refuted** | `src/*.c` and `tests/**` match via `inScope` (`filepath.Match` / `/**` prefix) |
| `graph impact` needs `--file` | **confirmed** | without flag → `--file é obrigatório`; with flag → empty/`null` output in this C |
| Review FP on `tests/hidden` invariant | **confirmed** | `suggestsViolation` finds substring `tests/hidden` in the `rd.md` diff itself (`forbidden:`) |
| Verify runs partial checks and misses build errors | **confirmed** | `--test-cmd './jq -n 1'` → PASS without `make`; default config is `go test` |
| tb does not push completing the whole task | **confirmed** | 1st task (#3128) abandoned after broken patch; tb review/verify did not signal “incomplete acceptance” beyond what the operator configured; only native `make check` showed the crash |

## New problems

### P1 — `tb guard check --strict --json` does not fail the exit code

- **Repro**: `tb guard check 189e148..HEAD --strict --json` with `protected_path` violation.
- **Expected**: exit ≠ 0 when there is `SeverityError` and `--strict`.
- **Actual**: prints JSON and returns 0 because the `--json` branch returns before the `strict` block.

### P2 — `graph impact` / `callers` useless or empty in an indexed C project

- **Repro**: `tb graph --enable index` (1064 nodes); `tb graph --enable impact --file src/compile.c` → `(vazio)` / JSON `null`; `callers --function gen_destructure` → empty.
- **Expected**: impact/callers from indexed `function` nodes.
- **Actual**: shallow indexing; few `calls` edges (19); impact does not help the workflow.

### P2 — Onboarding does not detect the repo toolchain

- **Repro**: `tb init` on jq.
- **Expected**: suggest/fill `make check` / `make` / absence of Go linter.
- **Actual**: `verify.test_cmd: go test ./...`, `hidden_test_cmd: go test ./tests/hidden/...`; zero `AGENTS.md` or skills.

### P3 — `tb review` treats mention of a forbidden path in the ticket as a violation

- **Repro**: `forbidden: [tests/hidden/**]` in committed `rd.md`; `tb review` on the same range.
- **Expected**: only flag a real change under `tests/hidden/`.
- **Actual**: critical “possible violation” by substring in the diff.

### P3 — Cobra error output dumps Usage after useful JSON

- **Repro**: any `tb drift`/`review` with drift (exit 1).
- **Expected**: JSON + exit 1, without long Usage on stderr.
- **Actual**: Usage hinders agents that parse stderr.

## What tb caught and what it let slip

**Caught**

- Drift of `rd.md` outside the declared `scope`.
- Guard/verify blocking change to `.toolbelt/thresholds.yaml`.
- Verify with `make check` aligned to native (PASS after #2414).
- Mem integrity check of invariants.
- Graph search located relevant C symbols (`gen_destructure`).

**Let slip / did not help**

- Broken #3128 patch: verify would only fail if `test_cmd` were the real suite; with a partial command, misleading PASS.
- Review did not validate acceptance criteria reliably (token heuristic); FP on invariant.
- Impact/callers did not guide the patch.
- No pressure to run `make`/`make check` beyond what the human configured.
- Build errors do not enter verify unless they are in `test_cmd`.

## Interaction with existing instrumentation / onboarding

Repo **without** agent instrumentation. tb onboarding from scratch:

| Generated by `tb init` / `mem init` | Quality |
|-------------------------------------|---------|
| `.toolbelt/config.yaml` | Generic Go; wrong for jq |
| `.toolbelt/vault`, `logs`, `bench` | Empty / placeholders |
| `.toolbelt/invariants.yaml` (via `mem init`) | 3 anti-cheat rules useful in the abstract, but centered on `tests/hidden` and thresholds (Go/tb ecosystem) |
| `AGENTS.md` / detection of `Makefile`/`configure` | **Not generated / not detected** |
| Test/lint/build commands | Not inferred; operator had to set `make check` by hand |

Conclusion: bootstrap is a “toolbelt skeleton”, not a “understood project”. For C/autotools, the agent still has to discover the toolchain alone.

## Improvement suggestions

1. **[verify/init] P0** — Detect `Makefile`/`configure.ac`/`Cargo.toml`/etc. on `tb init` and fill `verify.test_cmd` / document build (e.g. `make check`).
2. **[init/mem] P0** — Option to generate minimal `AGENTS.md` (build, test, layout) when no instrumentation exists.
3. **[docs] P1** — Sync `docs/cli.md` with the real CLI (remove `ticket` or implement; fix `mem add`; `guard` without `--manifest` until it exists).
4. **[guard] P1** — Honor `.toolbelt/guard.yaml`; fix `--strict` with `--json` (check severity before/after encode).
5. **[verify/visual] P1** — If all screens are skipped, `skipped: true` in the report and/or a verdict distinct from “full” PASS.
6. **[review] P1** — Match invariants by real changed path, not substring in the diff/ticket.
7. **[bench] P1** — Respect `bench.default_agent`; accept `--agent` on `compare` or document only `--fake`/JSONL.
8. **[graph] P2** — Impact/callers with useful fallback when the C index has no `calls` edges (e.g. co-occurrence in the same TU, `#include`).
9. **[verify] P2** — “Repo checks” mode that runs declared build+test and fails if `test_cmd` is the Go default in a non-Go repo.
10. **[cli] P3** — Under `--json`, do not print Cobra Usage on stderr for domain errors (drift/review).

## Metrics

| Metric | Value |
|--------|-------|
| Wall time (timed tb commands, ~20:46–20:50 UTC) | ≈ 3.75 min invocation span; full session (clone+toolchain+implementation+tb) ≈ 25–30 min |
| Recorded tb invocations | 45 lines in `/tmp/campo-jq-evidence/invocations.tsv` (plus native `make`/`git`) |
| tb failures (exit ≠ 0) | 18 (includes expected cases: drift, review, invalid flags, graph off, missing ticket) |
| `make check` baseline | PASS 9/9, ~12.4 s |
| `make check` post-#2414 | PASS 9/9, ~7.9 s |
| `make check` post-#3128 patch (aborted) | FAIL 5/9 (assertion), ~1.5 s |
| Fake bench (toolbelt tasks) | 9 tasks × 1, 33.3% success, ~4.1 s |

Local evidence artifacts (not committed): `/tmp/campo-jq-evidence/`, clone `/tmp/target/jq` with local commits `189e148` (fix #2414) and `3d4c60d` (thresholds probe) — **not** sent to jq’s GitHub.
