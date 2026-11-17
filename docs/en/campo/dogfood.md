[🇧🇷 Português](../../campo/dogfood.md) | 🇺🇸 English

# Dogfood — `tb` on its own repository

Log of friction and bugs found while using `tb` to develop toolbelt.
Format: date · command · expected vs. actual · severity.

## 2026-11-17 — bootstrap (`v0.0.2-20-ga76cee4`)

Sequence: `make install` → `tb init --write-agents` → `tb mem init` → `tb mem add` (×6)
→ `TB_GRAPH=on tb graph index` → `tb verify run` → `tb mcp` (`tools/list` smoke).

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 1 | `tb init` | `lint_cmd` only if the binary is on PATH (like `test_cmd`) | writes `golangci-lint run` without a PATH check; `verify run` fails with `comando não encontrado` | medium |
| 2 | `tb verify run --json` | on FAIL/ESCALATE, say which test/package failed | only `visible_passed: false` + `ESCALATE` after 3 iterations; no output excerpt — had to run `go test` by hand | high |
| 3 | `tb verify run` | — | 3 iterations (~20s) re-running the same deterministic suite with no code change in between | low |
| 4 | `go test ./...` (tb suite) | passes regardless of host git config | `internal/review` fails with `init.defaultBranch=main`: hardcoded `git checkout master` (`review_test.go:1279,1326,1407`) | medium |
| 5 | `tb init --write-agents` | — | `agent_docs.yaml` records `sources: [AGENTS.md]` — the file init itself just seeded (self-reference) | low |
| 6 | `tb mem edit --ref` | error message consistent with what is accepted | error lists only `docs/adr/…`/`docs/decisions/…`, but `docs/en/adr/…` is accepted; no way to clear `ref` (needed `retire` + `add`) | low |
| 7 | `tb init` | versionable files with default mode | `config.yaml`/`invariants.yaml` created `0600` | low |
| 8 | `tb graph index` | — | ok: 3960 nodes / 14746 edges in 0.16s; 2940 nodes are doc `section`s (PT + EN duplicated) — assess search noise | info |
| 9 | `tb mcp` | — | ok: 22 tools in `tools/list` | info |

## 2026-11-17 — release `v0.0.3` (`2eb65c1`)

Verification: `sha256sum -c` ok; sigstore bundle certificate with SAN
`…/release.yml@refs/tags/v0.0.3`, GitHub OIDC issuer, commit `2eb65c1` = `v0.0.3^{}`.
Signature/Rekor **not** verified (no `cosign`); `gh attestation verify` unavailable in `gh` 2.46.

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 2 | `tb verify run --json` | failure cause in JSON | **fixed**: `reason`/`error` carry the `test_cmd` output | — |
| 10 | `tb verify run --json` | excerpt with the `--- FAIL` lines | `reason` truncated from the start (`[no test files]`, `ok …`); failures in later packages vanish in the `…` | medium |
| 11 | `tb init` + `TestDocsEnglishMirrorComplete` | `--write-agents`/bootstrap does not break the suite | creating `docs/campo/*.md` without a pair in `docs/en/` breaks `go test` — the PT/EN docs invariant is caught by the test, not by `tb guard` | info |
| 12 | `tb verify run --dry-run` | — | ok: `verdict: SKIPPED` + plan | info |

## 2026-11-17 — Phase 3 (ticket/board/router/watchdog/merge)

| # | Command | Expected | Got | Sev. |
|---|---------|----------|-----|------|
| 4 | `go test ./...` | — | **fixed**: `initGitRepo` pins `-b master` | — |
| 13 | `tb stack … --ticket rd.md` | analyze uncommitted changes (working tree) | only compares `--base..--head` (commits); needed a throwaway commit in a detached worktree to run the stack before committing | medium |
| 14 | `tb stack drift review` | — | ok: caught `internal/review/review_test.go` outside the rd.md Scope | info |
| 15 | `tb guard` (via stack) | environment-conditional `t.Skip` (Landlock/git missing) distinguishable from cheating skips | 24 `test_skip` warnings for legitimate environment skips; noise | low |
| 16 | `tb sandbox run -- sh -c 'echo > /dev/null'` | writing to `/dev/null` allowed | `Permission denied` (also in v0.0.3): git/shells break inside the sandbox — fixed in this phase | high |
