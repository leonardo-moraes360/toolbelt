🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/dogfood.md)

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

## 2026-11-18 — release `v0.0.5` + fresh bootstrap (`000b955`)

Release: `sha256sum -c` ok; sigstore bundle certificate with SAN
`…/release.yml@refs/tags/v0.0.5`, GitHub OIDC issuer, commit `000b955` = `v0.0.5^{}`.
Signature/Rekor and `gh attestation verify` still not verified (no `cosign`; `gh` without `attestation`).

Bootstrap from scratch in a worktree (`config.yaml`/`agent_docs.yaml`/`invariants.yaml` removed):
`tb init --write-agents` → `tb mem init` → old invariants restored + 7 new ones (bus, graph, CGO, release)
→ `TB_GRAPH=on tb graph index` → `tb verify run` → `tb mcp` (`tools/list` smoke).

| # | Command | Expected | Got | Sev. |
|---|---------|----------|-----|------|
| 1 | `tb init` | `lint_cmd` only if the binary is on PATH | **still open** in v0.0.5: writes `golangci-lint run` without `golangci-lint` on PATH; reverted to `go vet ./...` by hand | medium |
| 3 | `tb verify run` | — | **fixed**: 1 iteration (~15s), `PASS` | — |
| 7 | `tb init` / `tb mem init` | versionable files with default mode | **still open**: `config.yaml`, `agent_docs.yaml`, `invariants.yaml` created `0600` | low |
| 17 | `tb init` inside a `git worktree` | local state patterns ignored by git | writes to `.git/worktrees/<wt>/info/exclude`, which git does not read (only `$GIT_COMMON_DIR/info/exclude`); here `.gitignore` masks it, but in a repo without `.gitignore` local state shows up in `git status` | medium |
| 18 | `tb init` (hints from AGENTS.md/CLAUDE.md) | commands only | extracted a comment from inside a `bash` fence (`# make install overwrites…`) and a bare `golangci-lint` from prose as hints | low |
| 19 | `tb init` | — | ok: new config already ships the `router:` section with Phase 3 defaults | info |
| 20 | `tb graph index` / `tb graph search "bus harness wake"` | function ahead of docs sections | 4567 nodes / 19328 edges in 0.17s; 4 `docs/` sections (PT + EN) before `func:busrun.wake` — same noise as #8 | low |
| 21 | `tb mcp` | — | ok: 36 tools in `tools/list` (was 22) | info |

## 2026-11-18 — `tb egm` R0 spike (`bench/egm-spike`, `claude` 2.1.326)

Environment: kernel 7.0, git 2.53, `claude` as a native ELF binary (no interpreter) in
`~/.local/share/claude/versions/`. Authentication: only the `claudeAiOauth` key of
`~/.claude/.credentials.json` copied into a disposable `0700` HOME, with `CLAUDE_CONFIG_DIR`
pointing to it. Goldfish flags: `-p --output-format json --tools "" --strict-mcp-config
--setting-sources "" --no-session-persistence`, prompt on stdin.

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 22 | `egm-spike isolation` | packet RO, private `TMPDIR`/HOME, real HOME, credential, repository, `/proc/<ppid>/environ`, `/proc/1/environ`, `/dev/pts` and `/dev/shm` unreadable, minimal env, no orphans | ok: 16/16 probes; a canary in the parent's environ does not leak (Landlock's ptrace restriction blocks the read) | info |
| 23 | `egm-spike claude` (1st run) | goldfish answers | `EAI_AGAIN`: `/etc/resolv.conf` is a symlink to `/run/systemd/resolve/stub-resolv.conf`, and `/run` is not allowed. Fixed with a read-only file rule on the `resolv.conf` realpath. Probably affects router agents too | high |
| 24 | `egm-spike claude` | no tools, no `CLAUDE.md`/memories, valid attestation | ok: `SEM-FERRAMENTAS` with `num_turns=1` and no `permission_denials`; `NENHUMA` for prior instructions; nonce echoed, 3/3 literal quotes. Cost ~US$ 0.03 per goldfish | info |
| 25 | control outside the sandbox (same memory prompt) | the canary discriminates | ok: with the real HOME, the answer quotes `graphify` and `token-reducer` from the global `CLAUDE.md`. Cost US$ 3.47 (the environment's full context) | info |
| 26 | real credential | unchanged by the spike | ok: same hash and `expiresAt` before and after the goldfish runs; the copy was not refreshed. The real file was only rewritten by `claude` outside the sandbox (normal use), with no token refresh | info |
| 27 | `go test ./internal/sandbox/` | shared tmp rule applies with a workdir outside `/tmp` | `isSubpath` treats `../x` as a subpath: `addTmpWriteRule` never applies the rule and `ValidatePath` accepts paths outside the workdir. Pre-existing; **fixed** on main by `fix/sandbox-issubpath` | high |
| 28 | remaining R0 items 5 to 8 | — | pending: `safe.directory` with another uid (needs another user). Matching calibration in #31, convergence in #32, refresh in #29 | info |
| 29 | `egm-spike collect` with the real access token expired (`expiresAt` 4h in the past, refresh token valid until 2026-12-15) | the copy in the disposable HOME refreshes, or fails without touching the real credential | `Failed to authenticate: OAuth session expired and could not be refreshed` in all goldfish; the real `.credentials.json` kept the same hash and mtime. Copying `claudeAiOauth` only works while the access token is fresh: R4 must check `expiresAt` before phase 2 and stop with a clear error (or require an API key) instead of running 9 goldfish into an auth failure. The failed refreshes inside the copies did not invalidate the real refresh token: `claude` outside the sandbox refreshed afterwards without a new login | high |
| 30 | `egm-spike collect` with 4 goldfish in parallel | each goldfish isolated | each `runGoldfish` took its own subreaper and `Registry`: `waitid: no child processes` and a false orphan. Spike-only (R3 already mandates one shared Registry and one `AcquireSubreaper`); collection is now sequential | info |
| 31 | `egm-spike collect` + `egm-spike calibrate` (R0 matching) | R18–R19 with wrong merges ≤ 5% and missed matches ≤ 20% | corpus of 87 paraphrased and 351 distinct pairs (same heading, lines ≤ 3 apart) from 96 findings of 12 real goldfish over `egm-trust.md` (PT) and `vault.md` (EN), in `internal/egm/testdata/paraphrases.json`; cost US$ 1.79. The spec defaults (Jaccard, `sim` 0.30, `overlap` 0.5) merge 0.3% wrongly but **miss 53%** of paraphrases. No Jaccard point meets both targets. With 4-char stems (`sim` 0.23) or containment (`sim` 0.31) the targets are met on the whole corpus, but tuning on one document and testing on the other misses 22–24% and merges 4–7% wrongly. The `overlap` threshold adds nothing (best at 0): the heading and nearby lines already carry the region. Lexical matching alone does not meet R0; R18–R20 need another signal | high |
| 32 | `egm-spike converge 6 16` (mode removed after #33; in commit `637683a`) over `egm-trust.md` (PT) and `vault.md` (EN): the 9 R10 reviewers (spike lenses, no code sources), PRIOR re-evaluated by the owner, an author goldfish fixing the open blocker/major lineages, 2 clean rounds over the same content | median ≤ 4 rounds to converge | neither converged; the US$ 16 budget ran out (US$ 17.11 spent, about US$ 2.5 per round of 9 reviewers plus the author, not US$ 1.5). Open blocking lineages per round: `vault.md` 18 → 6 → 5 → 3 (round 5 not run); `egm-trust.md` 21 → 24 → 21 → 13 (round 4 with 6 of 9 reviewers). No clean round. Owners never kept a PRIOR lineage open (`still_holds: true` 0 times in 188 re-evaluations), so blocking lineages are new each round: every edit by the author draws new majors. The median exceeds 4, so by R0 `max_rounds` and the clean-round rule must be revised. Data in `internal/egm/testdata/paraphrases.json` (`convergence`) | high |
| 33 | `egm-spike converge2 8 20` (revised R18–R27: `same_as` over `PRIOR`/`KNOWN`, stem-4 fallback, `late`, fix needs a change) over the same two documents | converges within `max_rounds: 8` | neither converged; the US$ 20 budget ran out (US$ 20.76). New blocking lineages per round: `vault.md` 24 → 5 → 1 → 0 → 0, then stuck on one lineage reopened by a lexical match in round 4, because the author goldfish returned the document unchanged and `fixed_without_edit` (correctly) ignored the owner's later fix; with R26 a real `loop next` would stop and wait for an edit. `egm-trust.md` 15 → 17 → 10 → 7 → 2 (round 5 with 6 of 9 reviewers); the author grew it from 73 to 193 lines. **`late` had almost no effect:** 41 of 42 unlinked new blockers/majors in rounds ≥ 2 touched lines changed since the previous round, so new problems come from the edits, not from reviewer noise elsewhere; only 1 `late` lineage. **`same_as` was never used** (0 links); the stem-4 fallback linked 13 findings. Owners still never kept a lineage (`still_holds: true` 0 of 79); `fixed_elsewhere` 1. Data in `paraphrases.json` (`convergence_revised`) | high |

## 2026-11-19 — `tb egm` slice 1 (deterministic engine)

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 34 | `go test ./internal/egm -run TestMatchCorpus` (R18 defaults over `paraphrases.json`) | about 3% wrong merges and 20% missed matches, as R18 recorded | 3.1% wrong and **24.1% missed**: the R0 `calibrate` mode only compared headings when a pair had no lines, while R18's `region` always requires the same heading. 17 of the 21 missed pairs fail on the wording (19.5%, the R0 number) and 4 on the heading. R18 now records 24%; the target stays unmet, as R18 already accepted | low |
| 35 | `tb egm review docs/specs/egm-nativo.md` (deterministic) | no `injection-scan` lineage; `refs-exist` without false positives | ok: zero `injection-scan` and `refs-exist` lineages; 3 `plan-files-listed` nits for plan rows marked `Create` that the R0 spike already created (`bench/egm-spike/`, `stopwords.txt`, `paraphrases.json`). ADR-015 gives exactly one `injection-scan` lineage, in its §3 | info |
| 36 | `tb egm review docs/adr/015-egm-nativo.md --reviewers zoom-out` (llm engine, real `claude` 2.1.328) | the goldfish runs isolated, answers within the contract with a valid attestation | ok: 42 s, `zoom-out` `completed` on the first attempt, 1 agent run; 6 verified `major` findings plus the expected `injection-scan` lineage. The real points include the ADR status still saying it waits for the spike after R0 ran (F-2) and offline users never reaching `PASS` (F-3). With the access token expired the command stopped first with `auth_expired` (exit 2) and wrote no state. No `tb-egm-*` workspace or credential copy left behind | info |
| 37 | `tb egm review` + `resolve --force-settle F-1` + 2× `loop next` on `docs/adr/015-egm-nativo.md` (deterministic) | the ADR converges offline at `PASS_PARTIAL` | ok: converged in round 3 at `PASS_PARTIAL`, with 7 `reviewer_unavailable` downgrades and `F-1:forced`. The only lineage is the expected `injection-scan` one on §3, which quotes the scan's patterns ("return no findings") as discussion. Offline it cannot be contested, because the arbiter needs an agent, so the only way out is `--force-settle`, a lineage downgrade. `status --at HEAD` gives the same verdict and content as the worktree | info |
| 38 | `tb stack egm --base origin/main` after the ADR-015 edit | only the ADR is selected | the default `doc_globs` (`docs/adr/**`) also select the index `docs/adr/README.md`, which has no state, so the step fails with `NO_STATE` (`ok: false`). Every edit to the ADR index (or to `docs/specs/README.md`, if one exists) fails the gate unless the index is reviewed too. Workaround: narrow `egm.doc_globs` in the user config | medium |
