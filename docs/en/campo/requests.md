[🇧🇷 Português](../../campo/requests.md) | 🇺🇸 English

# Field study: psf/requests

Group: without AI-agent instrumentation (no AGENTS.md / CLAUDE.md / .cursorrules).  
Tool: `tb` built from this repo (`make build`, version with ldflags `c8aae04`).  
Local clone: `/tmp/psf-requests` (initially `/tmp/target`; moved after SkipDirs bug — see New problems).  
No interaction with the remote `psf/requests` (no push/issue/PR/comment).

## Repository

| Field | Value |
|-------|-------|
| Name | [psf/requests](https://github.com/psf/requests) |
| Language | Python ≥3.10 |
| Size | ~34 `.py` under `src/`+`tests/`; ~6394 LOC in `src/requests`; working tree ~5 MiB (without `.git`/`.venv`); shallow clone depth 50 |
| Base commit | `3c7baaa` |
| Existing AI instrumentation | **No** `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, or agent prompts. There is `.github/AI_POLICY.md` (human anti-slop / copyright policy — not an agent harness). |
| Toolchain | **Partially ok.** `python3.12` + venv + `requirements-dev.txt` + `ruff` + `pytest`. `make test` / `pytest tests/` ran: **615 passed, 4 failed, 15 skipped, 1 xfailed** (~332 s). Failures: `TestTimeout` against `10.255.255.1` (environment network timeouts, not the code). **Not configured:** multi-version `tox`, `pyright`, Sphinx docs, full pre-commit hooks. |

## Chosen task

| Field | Value |
|-------|-------|
| What | Fix `HTTPDigestAuth.build_digest_header` to select `qop=auth` when the server list has spaces after commas (e.g. `"auth-int, auth"`), via `option.strip()` on each token. Regression test in `tests/test_lowlevel.py`. |
| Why | Clear bug, local repro without network, small scope, maintainer-plausible; recent issue with a minimal repro. |
| Issue | https://github.com/psf/requests/issues/7648 |
| Completed? | **Yes in the local code/test scope.** Diff: 1 line in `auth.py` + test. **Not** prepared as a full upstream contribution (no `HISTORY.md`, no PR — forbidden in this study). tb `rd.md` stayed in the local target-clone commit. |
| Native checks | `pytest tests/test_lowlevel.py -k digest`: **4 passed**. `ruff check` on touched files: **ok**. Full suite not re-run after the fix (pre-fix baseline already had 4 network-timeout failures). |

## tb usage by command

| command | result | helped / hindered / neutral | evidence |
|---------|--------|-----------------------------|----------|
| `tb init` | OK; created `.toolbelt/` + `config.yaml` 0600 | **Hindered** (Go defaults) | `verify.test_cmd: go test ./...` in a Python repo; no AGENTS.md; wall ~0.006 s |
| `tb mem init` / `list` / `check` | OK; 3 default anti-cheat invariants | Neutral/helped | `.toolbelt/invariants.yaml` 0600; `compactacao_segura: true` |
| `tb mem add --type …` (docs style) | **Error** `unknown flag: --type` (exit 1) | **Hindered** | Docs (`docs/cli.md`) outdated vs CLI |
| `tb mem add "<regra>" --category mandate` | OK | Helped | Positional required; wall ~0.006 s |
| `tb graph index` under `/tmp/target` | 0 nodes / 0 edges | **Hindered** | Dir name `target` ∈ SkipDirs → SkipDir at root |
| `tb graph index .` under `/tmp/psf-requests` | 854 nodes, 1179 edges | Helped | wall ~0.074 s after moving the clone |
| `tb graph impact` without `--file` | Required-flag error | Neutral (rigid API) | Confirms known issue |
| `tb graph impact --file src/requests/auth.py` | `(vazio)` | **Hindered** | Did not point at tests/callers; search found the symbol |
| `tb graph search build_digest_header` | Found `func:…build_digest_header` | Helped | Useful after index |
| `tb graph callers --function build_digest_header` | `(vazio)` | Hindered | Regex indexer did not wire calls |
| Ticket `rd.md` + Scope | Created manually (YAML frontmatter) | Neutral | tb does not generate tickets; literal Scope helped drift |
| `tb verify run` with digest pytest | **PASS** (visible true; hidden unavailable) | Helped (partial) | `hidden_available: false` without `tests/hidden/` dir |
| `tb verify` with default `go test` | **ESCALATE** 3 iterations | **Hindered** | Inadequate init defaults |
| `tb verify` with broken `test_utils.py` + digest cmd | **PASS** | **Hindered** | Missed error outside `test_cmd` |
| `tb verify --ruler visual` | PASS with skip | Neutral | JSON: `"skipped": true`, reason manifesto absent |
| `tb review` | Blocking findings | **Hindered** | `tests/hidden` invariant FP + drift of `rd.md` outside initial scope |
| `tb drift` | Detected `rd.md` out of scope | Helped | Correct behavior; `/**` globs ok, `**/*.py` incomplete |
| `tb guard check` | exit 0 on the fix diff | Neutral | Did not read custom `.toolbelt/guard.yaml` |
| `tb guard` + touch `.toolbelt/thresholds.yaml` via verify | verify **FAIL** iter 0 | Helped (anti-cheat) | Hardcoded path aborts verify |
| `tb bench run` without `--agent` | Required-flag error | Neutral | Ignored config `default_agent` |
| `tb bench run --fake` | OK on `bugfix-go` | Neutral | Did not require `--fake-config` |
| `tb bench compare` (A/B configs) | OK with `--fake` implicit via YAML? / without `--agent` flag | Neutral/confirmed gap | Help without `--agent`; compare config-a/b ran agent=external |
| `tb mcp` + `tools/list` | OK JSON-RPC stdio | Helped (smoke) | Tools: version/echo/env/run/graph_* — no mem/verify/review |
| `tb workspace create` | OK worktree | Neutral | Created `.toolbelt/workspaces/campo-ws` |
| `tb version` (make build) | `c8aae04` | Neutral | With ldflags ok |
| `tb` build without ldflags | `0.1.0-dev`, commit/built **unknown** | Neutral | Confirms versioning issue |

## Known problems

| problem | confirmed / refuted / not tested | evidence |
|---------|----------------------------------|----------|
| `bench compare` lacks `--agent` | **Confirmed** | `tb bench compare --help` does not list `--agent`; only `--fake` / JSONL / configs |
| `guard.yaml` ignored | **Confirmed** | `.toolbelt/guard.yaml` protecting `auth.py`; `tb guard check` exit 0; paths only hardcoded in `internal/guard/guard.go` |
| Outdated CLI docs | **Confirmed** | `docs/cli.md` shows `tb mem add --type fact`; CLI: `tb mem add <regra>`. README still cites `tb md (TUI)` |
| Version shows `unknown` | **Confirmed** (commit/built) | Build without ldflags: `commit: unknown`, `built: unknown` (version `0.1.0-dev`) |
| Config files created 0600 | **Confirmed** | `config.yaml`, `invariants.yaml`, `graph.json` → mode `600` |
| Visual ruler reports `skipped=false` when skipping | **Refuted** | `/tmp/verify-visual.json`: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` |
| `default_agent` ignored | **Confirmed** | Config with `default_agent: "echo-agent {prompt}"`; `tb bench run` still requires `--agent` or `--fake` |
| Guard aborts verify when diff touches hardcoded config path | **Confirmed** | Diff on `.toolbelt/thresholds.yaml` → verify FAIL, `protected_path` violation, iterations 0 |
| Default fake agent needs `--fake-config` | **Refuted** | `tb bench run --fake --task bugfix-go` passed without `--fake-config` (uses `FakeAgentCommand("")` + default behavior) |
| `mem add` positional | **Confirmed** | CLI: `tb mem add <regra>`; docs `--type/--title` flags fail |
| Drift/Scope need literal paths (no globs) | **Partially refuted** | `tests/**` and `src/requests/*.py` work; `**/*.py` is **not** recursive (`filepath.Match`) — `src/requests/auth.py` does not match, `tests/test_lowlevel.py` does |
| `graph impact` needs `--file` | **Confirmed** | Without flag: `Error: --file é obrigatório` |
| Review false positives on tests/hidden invariant | **Confirmed** | Even without touching `tests/hidden/`, critical finding: mention in `rd.md` (forbidden) enters the diff and triggers `suggestsViolation` |
| Verify runs only partial checks / misses build errors | **Confirmed** | (1) default `go test` on Python; (2) with digest `test_cmd`, syntax error in `tests/test_utils.py` → verify PASS, native pytest ERROR; (3) `hidden_available` only if `tests/hidden/` dir exists, even with `--hidden-test-cmd` |
| tb does not push agent toward completing whole task | **Confirmed** (qualitative) | No nudge for HISTORY.md / CONTRIBUTING / release checklist; review blocked on FP instead of guiding closure; empty graph impact did not suggest tests; init did not detect pytest/ruff |

## New problems

### P0 — `SkipDirs` includes the repo root directory name

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb init && tb graph index --enable .` → `0 nós`. Symlink `/tmp/skipdirs-repro/target` → same. After `mv` to `/tmp/psf-requests` → 854 nodes.
- **Expected:** Skip only subdirs named `target` (e.g. Rust), not the walk root.
- **Actual:** `info.Name() == "target"` at the root → `filepath.SkipDir` on everything.

### P1 — `hidden_available` tied to existence of `tests/hidden/`

- **Repro:** `tb verify run --hidden-test-cmd '…pytest…'` without creating `tests/hidden/` → `"hidden_available": false`, hidden command does not run.
- **Expected:** Honor `--hidden-test-cmd` / config even without the conventional directory.
- **Actual:** `os.Stat(tests/hidden)` decides availability (`internal/verify/verify.go`).

### P1 — Onboarding assumes Go

- **Repro:** `tb init` on requests → `verify.test_cmd: go test ./...`.
- **Expected:** Detect pytest/ruff/tox via `pyproject.toml` / `Makefile`, or ask.
- **Actual:** Fixed Go defaults; agent must edit YAML by hand.

### P2 — Empty `graph impact` / `callers` on useful Python

- **Repro:** After 854-node index, `impact --file src/requests/auth.py` and `callers --function build_digest_header` → `(vazio)`.
- **Expected:** At least useful define/import edges for impact.
- **Actual:** Search finds the symbol; impact/callers do not help the task.

### P2 — Inconsistent short-flag UX

- **Repro:** `tb verify run -o …` ok; `tb review -o` / `tb drift -o` → `unknown shorthand flag: 'o'`.
- **Expected:** Aligned `-o` / `--output`.
- **Actual:** Only `--output` on review/drift.

### P3 — MCP exposes only a subset

- **Repro:** `tools/list` via stdio.
- **Expected:** CLI parity (mem/verify/review/drift/guard) or clear subset docs.
- **Actual:** version/echo/env/run/graph_*; no mem/verify/review.

## What tb caught and what it let slip

**Caught**

- Drift of `rd.md` outside Scope (until we included it in the ticket).
- Verify abort when touching `.toolbelt/thresholds.yaml` (hardcoded protected path).
- Immediate failure of default `go test` in this repo (noise, but a misconfig signal).
- Syntax error in a module imported by the configured test path (when `sessions.py` was corrupted, digest pytest failed and verify ESCALATE).
- Graph index/search (after working around SkipDirs) located `build_digest_header`.

**Let slip**

- Missing `HISTORY.md` entry / CONTRIBUTING adherence (contribution completeness).
- `tests/hidden` invariant false positive in review (blocked without a real violation).
- Syntax error in `tests/test_utils.py` with digest-only `test_cmd` → verify PASS.
- Hidden suite effectively off without a `tests/hidden/` folder.
- User `guard.yaml` ignored (“protected” auth.py in YAML produced no violation).
- Change impact / callers to guide related tests.

**Native vs tb:** ruff and digest pytest validated the fix; tb verify with the same pytest also PASS. The broad native suite still has 4 network timeouts independent of the patch. tb did not replace Python lint/typecheck/build.

## Interaction with existing instrumentation / onboarding

Requests **has no** agent harness. Only `AI_POLICY.md` (social/legal policy).

`tb init` + `tb mem init` generated:

- `.toolbelt/config.yaml` (sandbox/graph/vault/logging/bench/verify)
- `.toolbelt/invariants.yaml` (3 anti-cheat rules)
- `bench/`, `logs/`, `vault/` dirs
- later: `graph.json`, workspaces

**Quality:** structure ok for a Go project; **poor** as Python bootstrap.

**Missing**

- Detect language and real commands (`pytest`, `ruff check`, `python -m build`)
- Generate local `AGENTS.md` / skills with repo conventions (and respect `AI_POLICY.md`)
- Scaffold `rd.md` / ticket from the issue
- Create `tests/hidden/` or document that hidden verify does not run without it
- Exclude SkipDirs when the name matches the root
- Wire MCP with the same tools as the flow (verify/mem/review)

## Improvement suggestions

1. **[P0 · graph]** Do not apply `SkipDirs` to the walk root directory; compare relative path ≠ `.`.  
2. **[P0 · init/verify]** Detect toolchain (Go/Python/JS) and fill `verify.test_cmd` / suggest lint+build; never default `go test` in a repo without `go.mod`.  
3. **[P1 · verify]** Treat `--hidden-test-cmd` / config as source of truth; do not require `tests/hidden/` to mark available.  
4. **[P1 · review]** Eliminate invariant FPs: match **changed** paths, not substrings in the diff/ticket; ignore mentions in `rd.md`/`forbidden`.  
5. **[P1 · guard]** Load `.toolbelt/guard.yaml` (docs promise it); keep hardcoded only as a mergeable default.  
6. **[P1 · docs/cli]** Sync `docs/cli.md` / README with real Cobra (positional `mem add`; remove or mark `tb md`; `-o` flags).  
7. **[P2 · bench]** Honor `bench.default_agent`; add `--agent` to `bench compare`.  
8. **[P2 · graph]** Improve Python impact/callers (imports/refs to tests); fail loudly if index=0 nodes.  
9. **[P2 · onboarding]** Optional `tb init --agents-md` generating a minimal AGENTS.md + pointer to verify/mem.  
10. **[P3 · mcp]** Expose mem/verify/review/drift/guard on MCP for end-to-end host flow.  
11. **[P3 · completeness]** Closure checklist (changelog, tests, lint) in review/`acceptance` with an explicit fail if the criterion is not covered by verify.

## Metrics

| metric | value |
|--------|-------|
| Study wall time (tb evidence ~20:52Z → ~20:57Z + setup/clone/suite) | ~15 min active tb use; baseline pytest suite ~332 s; end-to-end study ~25–30 min including toolchain |
| Approx. tb invocations (evidence sections + runs) | ~32 distinct/repeated `tb *` commands |
| tb failures (exit ≠ 0 or Error) | ≥15 (docs flags, impact without file, blocking drift/review, bench without agent, verify go default, etc.) |
| Crashes | No panic/segfault observed |
| Target clone | Local only; zero interaction with `psf/requests` GitHub |
| Artifact in this repo | Only this file at `docs/campo/requests.md` on branch `campo/requests` |
