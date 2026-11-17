[🇧🇷 Português](../../campo/ripgrep.md) | 🇺🇸 English

# Field: BurntSushi/ripgrep

Field study of `tb` (group: without AI-agent instrumentation), local clone only — no push/issue/PR/comment upstream.

- **tb build**: `make build` → `toolbelt c8aae04` (commit `c8aae04`, built `2026-11-15T20:44:23Z`)
- **Target**: shallow clone at `/tmp/target/ripgrep` (initial HEAD `178cc02`)
- **Window**: ~2026-11-15T20:44Z → 20:52Z UTC

---

## Repository

| Field | Value |
|-------|--------|
| Name | [BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) |
| Language | Rust (edition 2024, `rust-version` 1.96; `grep-*` crate workspace) |
| Size | ~104 `.rs` / ~55–56k LOC; shallow clone ~4.8 MiB tree; working tree with `target/` ~699 MiB after build |
| Existing AI instrumentation | **Only** `AI_POLICY.md` (+ mention in `CONTRIBUTING.md`). No `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.cursor/`. The policy **forbids autonomous agents** contributing to the project; this study stayed 100% local. |
| Toolchain | Rustup `stable` updated to **1.100.0** (initial PATH had 1.83 and would not compile). `cargo test --tests` and `cargo build` ok. `rustfmt`/`clippy` available. **Did not** run the full multi-target CI/`cross`/PCRE2 matrix; workspace `cargo clippy -D warnings` failed on pre-existing lints in crates (e.g. `grep-matcher`), not because of the change. |

---

## Chosen task

| Field | Value |
|-------|--------|
| What | Fix line numbers (and offsets) in the standard printer when `-U` + `-r` hit **adjacent** multiline matches (#2779) |
| Why | Confirmed open bug on master (`15.2.0`/HEAD); maintainer pointed at `grep-printer`; small–medium scope with a clear regression test |
| Issue | https://github.com/BurntSushi/ripgrep/issues/2779 |
| Completed? | **Yes, locally only** (commit `f2df235` under `/tmp/target/ripgrep`; **not** sent to GitHub) |
| Native result | Before: `1:fnord / 2:boccob / …`. After: `1 / 4 / 7 / 10` and `-b` `0 / 30 / 61 / 98`. `cargo test -q -p grep-printer --lib` → 110 ok; `cargo test -q --tests` → 114 + **333** ok (includes `replace_adjacent_line_numbers` and `r2095`). `cargo fmt --check` ok after `cargo fmt`. |

Files touched: `crates/printer/src/util.rs`, `crates/printer/src/standard.rs`, `tests/multiline.rs`.

---

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | Created `.toolbelt/` + `config.yaml` (mode **600**); verify defaults = `go test ./...` | hindered | Rust repo; agent must rewrite verify by hand. No `AGENTS.md`, no Cargo detection. |
| `tb mem init` / `add` / `list` / `check` | `invariants.yaml` mode **600**; `add` accepts positional rule; `mem check` OK (retention 1.00) | neutral | `add --type/--title` (docs) → `unknown flag: --type`. Generic invariants (`tests/hidden/`) poorly aligned with ripgrep. |
| `tb graph index --enable` | 2546 nodes, 3225 edges → `.toolbelt/graph.json` (~1.2 MiB) | helped | Rust indexing worked; ~0.2 s. |
| `tb graph stats --enable` | Counts by type (functions 1661, etc.) | neutral | Useful for sanity; little for the task. |
| `tb graph search --enable replace` | Found `Replacer`, `replace_all`, tests, GUIDE | helped | Speeds finding the printer vs blind grep. |
| `tb graph impact --enable --file …` | Without `--file` → error; **with** `--file` → `(vazio)` / JSON `null` | hindered | Did not point at callers/dependents of `standard.rs` despite the index. |
| `tb graph deps` / `callers` | Require `--file`/`--module` and `--function`; positional invocation fails | hindered | UX differs from intuition/implicit docs. |
| Ticket + Scope (`rd.md`) | Markdown `## Scope` **does not** parse; needs `scope: a, b` and `base_commit: …` | hindered | With wrong format: review → `sem scope declarado`; drift → `out_of_scope: null` even with wrong paths. |
| `tb verify run` (cargo) | `PASS` (iter 1); `hidden_available: false` | helped (partial) | With `--test-cmd 'cargo test -q --tests'` mirrored the real change suite. |
| `tb verify` default/Go | `ESCALATE` (3 iters) with `go test ./...` | hindered | Post-`init` default unusable; hides the problem until configured. |
| `tb verify --ruler visual` | `verdict: PASS`, **`skipped: true`**, `skip_reason: manifesto de telas ausente` | neutral | Graceful skip; PASS verdict can confuse. |
| `tb review` | With correct scope: `findings: []`. Invalid `-o` (only `--output`) | neutral→hindered | Critical FP if the diff only **mentions** the string `tests/hidden` (see Known problems). |
| `tb drift` | Detects out of scope with correct `scope:`; globs `*` and `/**` **work** | helped | Exit ≠0 + `out_of_scope` list when `crates/core/main.rs` vs real diff. |
| `tb guard check` | No violations on the fix; violates `protected_path` on `.toolbelt/thresholds.yaml` | helped (partial) | Paths **hardcoded**; created `.toolbelt/guard.yaml` → **ignored**. |
| `tb bench run/compare` | `--fake` ok on `bugfix-go`; `compare --agent` nonexistent; config `default_agent` **ignored** | neutral | Bench uses toolbelt’s own fixtures, not ripgrep’s. |
| `tb mcp` | `tools/list` via stdio: `tb_env`, `tb_run`, `tb_graph_*`, `tb_version`, `tb_echo` | neutral | Started; does not cover review/verify/drift/guard/mem. |
| `tb version` | With `make`: `c8aae04`. `go build` without ldflags: `0.1.0-dev`, commit/built **`unknown`** | neutral | Confirms build gap without Makefile. |
| `tb ticket` | `unknown command` | hindered | Documented in `docs/cli.md`, absent from the binary. |

---

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **confirmed** | `tb bench compare --agent …` → `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmed** | File with `protected_paths: crates/printer/**`; `guard check` on the printer diff → `violations: null`. Code in `internal/guard` uses a hardcoded slice. |
| Outdated CLI docs | **confirmed** | Docs: `tb mem add --type`, `tb ticket`, `guard.yaml` as manifesto; real CLI diverges. |
| version shows `unknown` | **confirmed (partial)** | Without ldflags: version `0.1.0-dev`, `commit: unknown`, `built: unknown`. With `make build`, ok. |
| config files created 0600 | **confirmed** | `.toolbelt/config.yaml`, `invariants.yaml`, verify JSON outputs → mode `600`. |
| visual ruler `skipped=false` when skipping | **refuted (in this run)** | JSON: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. |
| `default_agent` ignored | **confirmed** | `bench.default_agent: "echo AGENT_SHOULD_RUN {prompt}"` in config; `tb bench run` without flags → `--agent ou --fake é obrigatório`. |
| guard aborts verify when diff touches hardcoded config path | **confirmed** | Diff on `.toolbelt/thresholds.yaml` → guard `protected_path`; verify `verdict: FAIL`, `iterations: 0`, exit 1. |
| default fake agent needs `--fake-config` | **refuted (basic use)** | `tb bench run --fake --task bugfix-go` → 100% success without `--fake-config`. Flag only customizes the command. |
| `mem add` positional | **confirmed** | Usage: `tb mem add <regra>`; `--type` rejected. |
| Drift/Scope need literal paths (no globs) | **refuted (partial)** | `scope: crates/printer/src/*.rs, tests/**` matched the files. What **breaks** is markdown `## Scope` / `- path` lists (not parsed). |
| `graph impact` needs `--file` | **confirmed** | Without flag: `--file é obrigatório`. |
| review false positives on tests/hidden invariant | **confirmed** | Commit with only comment `// … tests/hidden …` → `critical` finding “possível violação… NUNCA modifique arquivos em tests/hidden/”. |
| verify runs only partial checks / misses build errors | **confirmed** | With `--test-cmd 'cargo test -q -p grep-printer --lib'`, verify `PASS` while invalid `crates/core/main.rs` → `cargo build` exit 101. |
| tb does not push agent to complete whole task | **confirmed (observational)** | Init did not generate checklist/AGENTS; did not detect Cargo; wrong Scope/docs; Go verify default; empty impact. The task only closed by initiative outside tb’s “happy path”. |

---

## New problems

1. **P1 — `tb init` assumes Go in a Rust repo**  
   - Repro: `tb init` on ripgrep → `verify.test_cmd: go test ./...`.  
   - Expected: detect `Cargo.toml` / ask / make “not detected” explicit.  
   - Actual: Go defaults; Go verify → `ESCALATE`.

2. **P1 — ticket/Scope format not documented operationally**  
   - Repro: `## Scope` + `- path` list → review “sem scope”; drift marks nothing.  
   - Expected: accept markdown heading **or** docs/CLI show `scope: a, b` + YAML frontmatter.  
   - Actual: only same-line `scope:` / `escopo:` (regex in `parseTicketMarkdown`).

3. **P2 — `graph impact` returns empty on an indexed Rust crate**  
   - Repro: after `graph index`, `tb graph impact --enable --file crates/printer/src/standard.rs` → empty/`null`.  
   - Expected: defines/imports edges or a “no indexed impact” message.  
   - Actual: silence; search finds symbols in the same file.

4. **P2 — visual `verify`: `PASS` + `skipped: true`**  
   - Repro: `--ruler visual --skip-unavailable` without `screens.yaml`.  
   - Expected: verdict `SKIP`/`N/A`, not `PASS`.  
   - Actual: easy to read as visual verification ok.

5. **P2 — inconsistent short flags**  
   - Repro: `tb review -o file` / `tb drift -o file` → `unknown shorthand flag: 'o'`; verify accepts `-o`.  
   - Expected: `-o` alias of `--output` everywhere.

6. **P3 — MCP tool surface << CLI**  
   - Repro: `tools/list` without mem/verify/review/drift/guard/bench.  
   - Expected: parity or documented discovery of gaps.

7. **P3 — verify JSON created 0600**  
   - Same pattern as configs; hinders reading by other uids on shared CI.

---

## What tb caught and what it let slip

**Caught**
- Index/`graph search` located printer/replacer.
- `verify` with `cargo test -q --tests` passed on the final fix (and would fail with `go test`).
- `drift`/`review` with correct `scope:` validated the perimeter.
- `guard` blocked changing `.toolbelt/thresholds.yaml` (hardcoded path).
- Intact `mem check`; MCP responded.

**Let slip / did not help**
- Did not suggest the root cause or ticket format.
- `graph impact` did not guide blast radius.
- Would not run a full build if `test_cmd` is only `-p grep-printer` (demo: broken main + verify PASS).
- `hidden_available: false` always here — no hidden suite generated/discovered.
- Review FP from a textual mention of `tests/hidden`.
- Did not read `AI_POLICY.md` / did not warn about conflict with autonomous contribution (only relevant if someone were to open a PR — we did not).

**Native vs tb**: the `r2095` regression was seen by `cargo test --tests`, not by a tb-specific ruler; tb only mirrors the configured `test_cmd`.

---

## Interaction with existing instrumentation / onboarding

- **Existing**: only `AI_POLICY.md` (human policy; forbids autonomous agents upstream). Nothing like AGENTS/Cursor skills.
- **`tb init` generated**: `.toolbelt/{config.yaml,bench,logs,vault}`; then `mem init` → `invariants.yaml`; graph → `graph.json`. **Did not** generate `AGENTS.md`, Rust verify scripts, or import CONTRIBUTING/AI_POLICY.
- **Quality**: structure ok; content disconnected (Go, `tests/hidden`, thresholds).
- **Missing**: detect `Cargo.toml` → `cargo test`/`cargo fmt`/`clippy`; ticket scaffold with `scope:`/`base_commit:` examples; optional minimal `AGENTS.md`; read repo policies; useful Rust `graph impact`; MCP aligned with the workflow.

---

## Improvement suggestions

Ordered; tagged by area.

1. **[init/onboarding] P0** — Detect ecosystem (Cargo/Go/npm) and fill `verify.test_cmd` / suggest lint/build; generate stub `AGENTS.md` + `rd.md` example with `scope:`/`base_commit:`.
2. **[docs/cli] P0** — Sync `docs/cli.md` with the real tree (`mem add`, no `ticket`, `compare` flags, hardcoded `guard` vs `guard.yaml`).
3. **[drift/review] P1** — Accept markdown `## Scope` lists; document globs; reduce invariant FPs (match paths in the diff, not substring in text).
4. **[guard] P1** — Honor `.toolbelt/guard.yaml` or remove from docs; do not abort verify on toolbelt’s own config paths without a mandate.
5. **[graph] P1** — Make `impact` return something useful in Rust (defines/imports) or an explicit error.
6. **[verify] P1** — Separate `SKIP` from `PASS`; optional `build_cmd` beyond `test_cmd`; warn when `hidden` is unavailable.
7. **[bench] P2** — Honor `default_agent`; align `compare` with `--agent` or document only configs/`--fake`.
8. **[mcp] P2** — Expose verify/review/drift/mem or document the subset.
9. **[ux] P3** — Consistent `-o`; 0644 permissions for non-secret configs.

---

## Metrics

| metric | value |
|--------|--------|
| Wall time (active study) | ~8–10 min of timed command execution (~12.6 s summed only from short tb `time` log entries) + ~15–20 min total session wall including reading/implementation |
| Approx. tb invocations | **~49** (init/mem/graph/verify/review/drift/guard/bench/mcp/version + retries) |
| Observed tb failures/errors | **~11–16** UX exits/errors (flags, defaults, intentional drift, guard/verify on thresholds, compare `--agent`, `ticket`, etc.) — several were probes of known problems |
| Artifacts | evidence under `/tmp/tb-campo-ripgrep/`; ripgrep change only under `/tmp/target/ripgrep` (not on the remote) |

**Not run / why**: ripgrep multi-OS CI; PCRE2/`cross` matrix; workspace-wide clippy `-D warnings` (pre-existing lints); A/B bench with a real agent (only `--fake`); any interaction with ripgrep’s GitHub (forbidden by field rules).
