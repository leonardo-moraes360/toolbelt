🇺🇸 English | [🇧🇷 Português](../../pt-BR/campo/rodada2/ripgrep.md)

# Field round 2: BurntSushi/ripgrep

Field study ROUND 2 of `tb` (group: **without existing AI-agent instrumentation**). Local clone only — no push/issue/PR/comment upstream.

- **tb build**: `make build` on `main` → `toolbelt a46c295` (commit `a46c295`, built `2026-11-16T12:42:05Z`)
- **Target**: shallow clone at `/tmp/target/ripgrep` (initial HEAD `178cc02`)
- **Window**: ~2026-11-16T12:35Z → 12:43Z UTC
- **Evidence**: `/tmp/tb-campo2-ripgrep/`

---

## Repository

| Field | Value |
|-------|--------|
| Name | [BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) |
| Language | Rust (edition 2024 / `rust-version` 1.96; `grep-*` workspace) |
| Size | ~104 `.rs` / ~55k LOC; shallow ~4.8 MiB; with `target/` ~676 MiB after build |
| Existing AI instrumentation | Only `AI_POLICY.md` (+ mention in `CONTRIBUTING.md`). No `AGENTS.md`/`CLAUDE.md`/`.cursorrules`/`.cursor/` in the original tree. Policy forbids autonomous agents upstream; study 100% local. |
| Toolchain | Rustup `stable` **1.100.0**. `cargo test --tests` (114 + 332) and `cargo build` ok. `rustfmt`/`clippy` available. Did not run multi-target CI/`cross`/PCRE2 matrix; workspace-wide `cargo clippy -D warnings` was not the main gate this run. |

---

## Chosen task

| Field | Value |
|-------|--------|
| What | Avoid `Path::canonicalize` in `StandardSink::start_hyperlink` when the hyperlink format is empty (#3594) |
| Why | Open bug on master; small scope in `grep-printer` with a clear regression; different from round 1 |
| Issue | https://github.com/BurntSushi/ripgrep/issues/3594 (PR #3596 closed **without** merge — fix still absent at `178cc02`) |
| Completed? | **Yes, local only** (commit `69fedaf` under `/tmp/target/ripgrep`; **not** sent to GitHub) |
| Native result | `cargo test -q -p grep-printer --lib` → 110 ok (includes `empty_hyperlink_skips_path_canonicalization`); `cargo test -q --tests` → 114 + 332 ok; `cargo fmt --check` ok |
| Round 1 (comparison) | #2779 — wrong line numbers with `-U`+`-r` on adjacent multiline matches |

Files touched: `crates/printer/src/standard.rs`, `crates/printer/src/util.rs` (+ `#[cfg(test)]` accessor on the hyperlink cache).

---

## Verification of the 12 fixes

| # | fix | worked / did not work / not tested / not applicable | evidence |
|---|-----|-----------------------------------------------------|----------|
| 1 | graph SkipDirs at walk root | **worked** | `tb graph index --enable /tmp/target` → 2544 nodes / 3221 edges with paths `ripgrep/...` (root `/tmp/target`). In round 1 the same path yielded 0 nodes. |
| 2 | verify `hidden_test_cmd` without `tests/hidden/` | **worked** | No `tests/hidden/` dir; `--hidden-test-cmd false` → `hidden_available: true`, `hidden_passed: false`, verdict `ESCALATE` exit 1. With a real test cmd → `hidden_passed: true` on the final PASS. |
| 3 | guard `--strict --json` exit ≠ 0 | **worked** | Diff on `.toolbelt/thresholds.yaml` + `--strict --json` → JSON with `protected_path` and **EXIT=1**. |
| 4 | guard reads `protected_paths` from `guard.yaml` | **worked** | `.toolbelt/guard.yaml` with `crates/core/main.rs`; diff touching that path → `violations: [{type: protected_path, file: crates/core/main.rs}]`, `--strict --json` EXIT=1. |
| 5 | init toolchain detection | **worked** | `tb init` → `verify.test_cmd: cargo test` (no longer `go test ./...`). |
| 6 | review `tests/hidden` matches by path | **worked** | Ticket with text “Não tocar em tests/hidden/” + diff only in printer → `findings: []` (no critical from substring). |
| 7 | bench `default_agent` and `compare --agent` | **worked** | With `bench.default_agent: echo AGENT_OK {prompt}` in `.toolbelt/config.yaml`, `tb bench run -r 1 --task bugfix-go` (without `--agent`/`--fake`) → 100% success; `tb bench compare ... --agent 'echo COMPARE_AGENT {prompt}'` accepted the flag and produced an A/B report. |
| 8 | review/drift `## Scope`/`## Escopo`, MD lists and `**` globs | **worked** | `## Escopo` + bullets → review without `scope-missing`; wrong `## Escopo` (`crates/core/main.rs`) → drift lists printer files `out_of_scope` EXIT=1; `crates/printer/src/**/*.rs` kept files in-scope. |
| 9 | verify `build_cmd` and `lint_cmd` | **worked** | Config with `cargo build`/`cargo fmt --check` → `build_ran/lint_ran: true` on PASS. `build_cmd: false` → `ESCALATE`, `build_passed: false`. |
| 10 | init reads `AGENTS.md`/`CLAUDE.md` and `--write-agents` | **worked** | Without instrumentation: `tb init --write-agents` seeded `AGENTS.md` + `.toolbelt/agent_docs.yaml` (`sources=AGENTS.md`). `CLAUDE.md` absent (ok). Did not import `AI_POLICY.md`. |
| 11 | graph Python `calls` edges | **not applicable** | Rust repo; after index, edge types only `defines`/`imports`/`mentions`/`links` — **0** `calls` (expected: the fix is for Python). |
| 12 | aggregated visual skipped + CLI docs aligned | **worked (partial)** | Visual without manifest: `skipped: true`, `skip_reason: manifesto de telas ausente` (but `verdict: PASS` still). Docs: `default_agent`, `guard.yaml`, and `tb ticket` (marked Phase 3+) aligned; residual noise vs CLI remains in old `mem`/manifest-flag examples. |

---

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` / `--write-agents` | `.toolbelt/` + `test_cmd: cargo test`; seed `AGENTS.md` | **helped** | Rust onboarding ok; `AI_POLICY.md` still ignored |
| `tb mem init/add/list/check` | 3 anti-cheat + 1 custom; `retencao: 1.00` | neutral | Generic invariants; `mem add --type` still rejected (legacy docs) |
| `tb graph index --enable /tmp/target` | 2544 nodes / 3221 edges | **helped** | Root SkipDirs fixed |
| `tb graph search --enable hyperlink` | Found `HyperlinkFormat`, `as_hyperlink`, etc. | **helped** | Speeds finding `standard.rs` |
| `tb graph impact --enable --file …` | `(vazio)` / JSON `null` | **hindered** | Still no blast radius in Rust |
| `tb graph callers/deps` | callers requires `--function` (no `--file`); deps lists pkgs | neutral→hindered | UX still unintuitive |
| Ticket `rd.md` (`## Escopo` + globs) | Parse ok in review/drift | **helped** | Strong contrast with round 1 |
| `tb verify run` (build/lint/test/hidden) | `PASS`; hidden ran | **helped** | Full pipeline; `hidden_available: true` without dir |
| `tb verify --ruler visual` | `PASS` + `skipped: true` | neutral | Honest `skipped` field; PASS verdict still confuses |
| `tb review` | `findings: []` with MD scope; no `tests/hidden` FP | **helped** | |
| `tb drift` | Detects outside `## Escopo`; `**` globs ok | **helped** | EXIT≠0 + `out_of_scope` list |
| `tb guard check` | Clean on the fix; honors `guard.yaml`; `--strict --json` EXIT≠0 | **helped** | |
| `tb bench run/compare` | `default_agent` and `--agent` ok; toolbelt fixtures | neutral | Does not measure ripgrep itself |
| `tb mcp` | `tools/list`: env/run/graph_*/version/echo | neutral | No review/verify/drift/guard/mem |
| `tb ticket` | `unknown command` | neutral | Docs now mark as planned |

---

## Problems that persist

From round 1 (`docs/campo/ripgrep.md`) that **still happen** with `tb@a46c295`:

1. **Empty Rust `graph impact`** — `tb graph impact --enable --file crates/printer/src/standard.rs` → empty/`null` (same as round 1).
2. **No Rust `calls` edges** — index only `defines`/`imports`/`mentions`/`links`; callers/impact stay weak outside Go/JS/Python.
3. **Visual: `verdict: PASS` with `skipped: true`** — missing manifest; easy to read as “visual ok”.
4. **MCP << CLI** — no review/verify/drift/guard/mem/bench tools.
5. **Configs mode `0600`** — `config.yaml` and seeded `AGENTS.md` stayed `600`.
6. **`tb` does not push completeness alone** — better bootstrap, but task (#3594) still needs human initiative for issue/code/test.
7. **Does not read `AI_POLICY.md`** — the repo’s human policy stays invisible to the harness (only relevant if someone were to contribute upstream; we did not).

Round 1 problems **fixed in this run**: Go-default init; Markdown Scope; ignored guard.yaml; `default_agent`; review FP `tests/hidden`; verify without build/lint; hidden gated by dir; mute graph under `/tmp/target`; missing `--write-agents`.

---

## New problems

1. **P2 — `verdict: PASS` on the visual ruler when `skipped: true`**  
   - Repro: `tb verify run --ruler visual --skip-unavailable` without `.toolbelt/screens.yaml`.  
   - Expected: `SKIP`/`N/A` verdict (or non-PASS) aligned with `skipped`.  
   - Actual: JSON `skipped: true` + `verdict: PASS`.

2. **P2 — global `-c/--config` flag does not feed `config.Load()`**  
   - Repro: `tb -c /tmp/.../config.yaml bench run -r 1` with `default_agent` only in that file → still requires `--agent`/`--fake`.  
   - Expected: `-c` overrides the loaded YAML.  
   - Actual: only cwd/global `.toolbelt/config.yaml`; the flag seems decorative.

3. **P3 — `graph impact` stays silent (no “no impact” message)**  
   - Repro: after a rich Rust index, impact on the printer → `(vazio)`.  
   - Expected: at least a diagnosis (“0 edges” / “language without calls”).  
   - Actual: silence; search finds symbols in the same file.

4. **P3 — `AGENTS.md` seed mode 0600 and generic**  
   - Repro: `tb init --write-agents` on ripgrep.  
   - Expected: 0644 for non-secret docs; optionally cite `cargo test`/`AI_POLICY.md`.  
   - Actual: toolbelt-centric stub; does not reference the project policy.

No obvious regression caused by the 12 fixes was observed on the exercised paths (guard/verify/review/drift/init/graph root).

---

## What tb caught and what it let slip

**Caught**
- Cargo toolchain in init; `AGENTS.md` seed under `--write-agents`.
- Graph index under `/tmp/target` + hyperlink search → `standard.rs`.
- Markdown scope + globs in review/drift; `tests/hidden` FP eliminated.
- Verify with build/lint/test/hidden (hidden without folder) → PASS on the fix; ESCALATE if hidden/`build_cmd` fail.
- Guard with custom `guard.yaml` + strict exit code with JSON.
- Bench `default_agent` / `compare --agent`.

**Let slip / did not help**
- Did not suggest the root cause (#3594) or point at the early-return in hyperlink.
- `graph impact` did not guide blast radius (`path.rs`/`summary.rs` also call `as_hyperlink`).
- Visual PASS skipped; incomplete MCP; ignored `AI_POLICY.md`.
- Workspace-wide clippy / ripgrep CI matrix did not enter the harness.

**Native vs tb**: the new regression was seen by `cargo test -p grep-printer`; tb mirrored it via configured `test_cmd`/`hidden_test_cmd`.

---

## Interaction with existing instrumentation / onboarding

- **Existing**: only `AI_POLICY.md` (forbids autonomous agents upstream).
- **`tb init --write-agents` generated**: `.toolbelt/{config.yaml,vault,logs,bench}`, stub `AGENTS.md`, `agent_docs.yaml`. Then `mem init` → `invariants.yaml`; graph → `graph.json`; `guard.yaml` created manually for the test.
- **Quality**: much better than round 1 (automatic `cargo test` + AGENTS). Still generic; does not import CONTRIBUTING/AI_POLICY nor suggest `cargo fmt`/`clippy` in the seed.
- **Conflict**: none with agent instrumentation (there was none). Potential conflict with AI_POLICY only if there were intent for an autonomous PR — there was not.

---

## Comparison with round 1

| Aspect | Round 1 (`c8aae04`) | Round 2 (`a46c295`) |
|--------|---------------------|---------------------|
| Init | `go test` on Rust | `cargo test` + `--write-agents` |
| `/tmp/target` graph | 0 nodes | 2544 nodes |
| Markdown Scope | “no scope” | parse ok |
| guard.yaml | ignored | honored |
| verify build/lint/hidden | absent / gated | ran |
| review FP `tests/hidden` | critical | clean |
| bench default_agent / compare --agent | broken | ok |
| Rust impact | empty | still empty |
| Task | #2779 multiline | #3594 hyperlink canonicalize |
| Local conclusion | yes | yes |

**Verdict**: **better** in concrete ways on bootstrap and the 12 listed fixes; **same** on Rust graph/impact gaps, visual PASS-skipped, and MCP surface.

---

## Improvement suggestions

1. **[verify/visual] P1** — Do not emit `PASS` when `skipped: true` (use `SKIP`/`N/A`).  
2. **[graph] P1** — Rust `impact`/callers (even a conservative regex) **or** an explicit “no calls edges in this language” error.  
3. **[config/cli] P2** — Honor `-c/--config` in `config.Load()` (today the flag does not change the loaded YAML).  
4. **[init] P2** — When seeding `AGENTS.md`, detect `Cargo.toml` → document `cargo test`/`fmt`; warn if `AI_POLICY.md` exists.  
5. **[mcp] P2** — Expose verify/review/drift/guard/mem or document the subset in `tools/list`.  
6. **[ux] P3** — 0644 permissions for `AGENTS.md`/non-secret configs; consistent `-o` already partly improved.

---

## Metrics

| metric | round 2 | round 1 (ref) |
|--------|---------|---------------|
| Wall time (active study) | ~8–12 min of commands + local implement/test | ~15–20 min wall |
| Approx. tb invocations | **~55–65** (includes re-probes after `a46c295` rebuild and probes of the 12 fixes) | ~49 |
| Observed tb failures/errors | **~8–12** (intentional drift, hidden=`false`→ESCALATE, build_cmd=`false`, flag/docs probes, empty impact) | ~11–16 |
| Binary | `toolbelt a46c295` | `toolbelt c8aae04` |
| Artifacts | `/tmp/tb-campo2-ripgrep/`; ripgrep change only under `/tmp/target/ripgrep` | `/tmp/tb-campo-ripgrep/` |

**Not run / why**: ripgrep multi-OS/PCRE2/`cross` CI; workspace-wide clippy `-D warnings`; bench with a real LLM agent (only echo/`--fake`); any interaction with ripgrep’s GitHub (forbidden).
