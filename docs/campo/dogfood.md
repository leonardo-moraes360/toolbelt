🇺🇸 English | [🇧🇷 Português](../pt-BR/campo/dogfood.md)

# Dogfood — `tb` on its own repository

Log of friction and bugs found while using `tb` to develop toolbelt.
Format: date · command · expected vs. actual · severity.

## 2026-11-17 — bootstrap (`v0.0.2-20-ga76cee4`)

Sequence: `make install` → `tb init --write-agents` → `tb mem init` → `tb mem add` (×6)
→ `TB_GRAPH=on tb graph index` → `tb verify run` → `tb mcp` (`tools/list` smoke).

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 1 | `tb init` | `lint_cmd` only if the binary is on PATH (like `test_cmd`) | writes `golangci-lint run` without a PATH check; `verify run` fails with `comando não encontrado` | medium — **fixed in `b73010e`** (re-checked 2026-11-22: `tb init` without `golangci-lint` on PATH writes `go vet ./...`) |
| 2 | `tb verify run --json` | on FAIL/ESCALATE, say which test/package failed | only `visible_passed: false` + `ESCALATE` after 3 iterations; no output excerpt — had to run `go test` by hand | high |
| 3 | `tb verify run` | — | 3 iterations (~20s) re-running the same deterministic suite with no code change in between | low |
| 4 | `go test ./...` (tb suite) | passes regardless of host git config | `internal/review` fails with `init.defaultBranch=main`: hardcoded `git checkout master` (`review_test.go:1279,1326,1407`) | medium |
| 5 | `tb init --write-agents` | — | `agent_docs.yaml` records `sources: [AGENTS.md]` — the file init itself just seeded (self-reference) | low |
| 6 | `tb mem edit --ref` | error message consistent with what is accepted | error lists only `docs/adr/…`/`docs/decisions/…`, but `docs/en/adr/…` is accepted; no way to clear `ref` (needed `retire` + `add`) | low |
| 7 | `tb init` | versionable files with default mode | `config.yaml`/`invariants.yaml` created `0600` | low — **fixed in `b73010e`** (re-checked 2026-11-22: `config.yaml`, `agent_docs.yaml`, `invariants.yaml` are `0644`) |
| 8 | `tb graph index` | — | ok: 3960 nodes / 14746 edges in 0.16s; 2940 nodes are doc `section`s (PT + EN duplicated) — assess search noise | info |
| 9 | `tb mcp` | — | ok: 22 tools in `tools/list` | info |

## 2026-11-17 — release `v0.0.3` (`2eb65c1`)

Verification: `sha256sum -c` ok; sigstore bundle certificate with SAN
`…/release.yml@refs/tags/v0.0.3`, GitHub OIDC issuer, commit `2eb65c1` = `v0.0.3^{}`.
Signature/Rekor **not** verified (no `cosign`); `gh attestation verify` unavailable in `gh` 2.46.

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 2 | `tb verify run --json` | failure cause in JSON | **fixed**: `reason`/`error` carry the `test_cmd` output | — |
| 10 | `tb verify run --json` | excerpt with the `--- FAIL` lines | `reason` truncated from the start (`[no test files]`, `ok …`); failures in later packages vanish in the `…` | medium — **fixed in `610146d`** (`excerpt` keeps the `--- FAIL`/`FAIL` lines first; test in `e869e43`; re-checked 2026-11-22) |
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
| 1 | `tb init` | `lint_cmd` only if the binary is on PATH | **still open** in v0.0.5: writes `golangci-lint run` without `golangci-lint` on PATH; reverted to `go vet ./...` by hand. **Fixed in b73010e** (`detectGoLintCmd` checks PATH, falls back to `go vet ./...`; `TestDetectLintCmd_GolangciMissingFromPath` covers it) | medium |
| 3 | `tb verify run` | — | **fixed**: 1 iteration (~15s), `PASS` | — |
| 7 | `tb init` / `tb mem init` | versionable files with default mode | **still open**: `config.yaml`, `agent_docs.yaml`, `invariants.yaml` created `0600` | low — **fixed in `b73010e`** |
| 17 | `tb init` inside a `git worktree` | local state patterns ignored by git | writes to `.git/worktrees/<wt>/info/exclude`, which git does not read (only `$GIT_COMMON_DIR/info/exclude`); here `.gitignore` masks it, but in a repo without `.gitignore` local state shows up in `git status` | medium — **fixed in `b73010e`** (re-checked 2026-11-22: from a worktree, the patterns go to `$GIT_COMMON_DIR/info/exclude`) |
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
| 38 | `tb stack egm --base origin/main` after the ADR-015 edit | only the ADR is selected | the default `doc_globs` (`docs/adr/**`) also select the index `docs/adr/README.md`, which has no state, so the step fails with `NO_STATE` (`ok: false`). Every edit to the ADR index (or to `docs/specs/README.md`, if one exists) fails the gate unless the index is reviewed too. Fixed: a `README.md` index is no longer a gated document (glossary, R33); `--egm-doc` can still name it | medium |
| 39 | `tb egm review` + `loop next` on `docs/specs/egm-nativo.md` (deterministic), then `tb stack egm --base origin/main` | the spec converges offline; the gate passes with `warn` | ok: `PASS_PARTIAL` in round 2, with 32 `plan-files-listed` nits for plan rows marked `Create` that Phase A already created (nits do not block). Two frictions: (1) a document that cites `docs/campo/dogfood.md` goes `STALE` with every new dogfood entry, because cited sources are part of the `content`, so recording a run invalidates it; (2) a `loop next` after convergence over the same content opens a new cycle and round instead of stopping, which with the `llm` engine would spend about 9 agent runs for nothing. **Fixed (2):** `loop next` now refuses with `already converged over this content in cycle N` and exit 0, with no new round or cycle (R26) | low |

## 2026-11-20 — release `v0.0.7` + vault spec round 3

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 40 | `git push origin v0.0.7` (release workflow) | goreleaser publishes the 6 targets, as CI was green | `windows/amd64` and `windows/arm64` failed: `internal/egm/receipt.go` used `syscall.O_NOFOLLOW`, which Windows lacks. CI only built `linux/amd64`. Fixed by build-tagged files (#125) and a CI cross-build step (#127); the tag was recreated, since no release had been published | major |
| 41 | `tb egm review docs/specs/vault.md --engine deterministic` | only real problems | 2 real `required-sections` findings (`## Non-goals` and `## Technical plan` do not match the standard's casing) and 6 real `plan-files-listed` ones (test files written without their directory). But 6 `refs-exist` majors point at paths the spec plans to create or that belong to Obsidian (`.obsidian`, `.trash`, `internal/vault/testdata/obsidian`), and a pre-implementation spec cannot converge without declining them one by one. Also, `plan-files-listed` only reads the first table after the plan heading: splitting the plan into one table per slice turned 16 `Create` files into `refs-exist` findings | minor |
| 42 | `tb bus discover` (2026-11-20, local build) | this repository's `.mcp.json` points at a known `tb` | the `toolbelt` entry runs a bare `tb`, so `same_tb` is `null`: the client resolves it on its own `PATH`, which here is the `~/go/bin` release (v0.0.8), not the local build under test. Intended (CLAUDE.md says to test with `./bin/tb`), but discover makes the ambiguity visible | info |

## 2026-11-27 — `v1.0.0-rc.4` soak (paprika field addenda 2026-11-24/25 + re-check)

Sources: [paprika.md](paprika.md) addenda of 2026-11-24 and 2026-11-25 (rc.3 and rc.4 sessions) and a re-check of #60 with the installed rc.4. No new `high` on rc.4.

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 61 | `tb guard check HEAD~1..HEAD` after `git mv server-go server` (60 test files, 30 with `t.Skip`), installed rc.4 | no deletion findings | ok for #60: 0 `test_deletion`. But 30 `test_skip` warnings, one per untouched `t.Skip()` in a moved file: guard scans every line of a touched file, not only added lines (paprika 2026-11-25 saw the same after stripping comments) | medium — open, 1.0.x (warning only, never fails the gate) |
| 62 | `tb stack verify guard --ticket rd.md` on a multi-commit branch, no `--base` | the branch is checked as a whole | the default base was `HEAD~1`: only the last commit was checked; the full range needed a separate `tb guard check <merge-base>..HEAD` (paprika 2026-11-25, rc.3) | medium — **fixed in `fix/rc5-stack-base`** (default base: ticket `base_commit`, else merge-base with the default branch, else `HEAD~1`; JSON `base`, `base_source`) |
| 63 | `tb bus invite <channel> --name cursor-N` (×3) | the output names the bound agent | `invite: 1SV8-XMNH (channel paprika-3, role worker, …)` without the name; the mapping comes from loop order (paprika 2026-11-23/24, rc.4) | medium — open, 1.0.x (additive) |
| 64 | `tb bus join` without `--wake` when a wake file exists for the name | `join` takes the wake file's adapter, or warns | defaults to `--wake manual`; `run` then shows `wake_mismatch`, and fixing it needs a fresh invite and a second join (paprika 2026-11-24, rc.4) | medium — open, 1.0.x |
| 65 | `tb bus send` of 10 `task.assign` with `deps` in a loop | the `deps` warning (B2) is clear | clear, but printed once per send, 10 times (paprika 2026-11-24, rc.4) | low — open |
| 66 | `tb guard check` on an intentional removal of tests (`bench/`) | a way to declare it in the ticket | only `test_deletion` (error); proved by hand by counting tests before and after. Proposal: `allow_test_deletion: [glob]` in the rd.md frontmatter (paprika 2026-11-25) | medium — open, 1.1 (new ticket field) |
| 67 | `tb guard` with no subcommand | run the default diff check | prints the help; the check is `tb guard check` (paprika 2026-11-25) | low — open |
| 68 | `bench:` in `.toolbelt/config.yaml` pointing at a removed directory | a warning | nothing complained (paprika 2026-11-25) | low — open |
| 69 | `tb bus channel list` with closed and expired channels | closed ones hidden | still listed next to smoke channels; no `channel close` or `list --open` (paprika 2026-11-24, rc.4) | low — open, 1.1 |

## 2026-11-23 — field findings, paprika project on `v1.0.0-rc.3` (fixed in `fix/rc3-field`)

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 59 | `tb -c ci.yaml verify run` / `tb stack` on a PR that migrates `verify:` in `.toolbelt/config.yaml` | with a `-c` file outside the repository the repository config is not read, so the change is harmless and verify can PASS | aborted on `verify_config_changed`, so no such PR could ever get PASS | high — **fixed in `fix/rc3-field`** (finding still reported by guard; without `-c` or with an in-repo `-c` still aborts) |
| 60 | `tb guard check` / `tb verify run` after `git mv server-go server` | moved tests are not deletions | every moved test file reported `test removed` (`test_deletion`): rename sources were marked deleted | high — **fixed in `fix/rc3-field`** (only pure directory moves, same basename, are exempt; any rename that changes the basename or lands in `testdata/` etc. still counts) |

## 2026-11-22 — stability backlog (note, not a fix)

Found while specifying symbol-level drift ([egm Phase C](../specs/egm-phase-c.md)); no `tb graph` command was run. Both come from the Go indexer in `internal/graph/index_go.go` and were tracked for the graph slice (C6) and are **fixed in `7b3bd7e`** (breaking for stored node IDs, `graph.json` `version` 2):

- Methods drop the receiver in their ID (`func:<pkg>.<name>`, line 52) and `funcIDs` is keyed by the bare name (line 54), so `(A) Name` and `(B) Name` in one package become one node and the call edges of the second are attributed to the first.
- Package IDs are `pkg:<name>` (line 34), so several packages named `main` (every `cmd/*`) collapse into one node.

Fix: packages are `pkg:<import path>` and methods `func:<import path>.<Recv>.<Name>`. `tb egm` does not depend on either: its symbols are file-scoped.

Same commit: the graph walk no longer follows symlinks or blocks on a FIFO (readers shared with the vector index; vault spec Q5).

## 2026-11-21 — vector index V1 (`tb graph index --vectors`, local `embednative` build, all-MiniLM-L6-v2 Q8_0)

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 43 | `tb graph index --vectors` on this repository | builds `graph.vec` in a few minutes | the first full batch failed with `embed_protocol: reply line over 196608 bytes`: the parent's read loop picked up the 64 KiB handshake cap before the handshake raised it. Fixed before merge (regression test with 384-dim vectors). After the fix the run was stopped at ~17 min: 898 files give about 11k chunks, 70% from `third_party/` (vendored llama.cpp), and the x86-64-v2 build embeds one 1.5 KB chunk in about 135 ms on 4 threads (no AVX/FMA under the ADR-018 floor; one text per call, the shim does not batch sequences yet), so the full run takes about 25 min | medium |
| 44 | `tb graph index --vectors` + `search --mode semantic/hybrid` on a 24-file copy (`internal/embed`, `secrets`, `sandbox`, 2 docs) | intent queries find the code; reruns are incremental | ok: 125 chunks in 12 s, `.env`-style file denied, store 0600. A second run embedded 0 and reused 125; editing one file re-embedded 1 chunk. "where are symlinked model files refused" ranks `internal/embed/model.go` first in semantic and hybrid (lexical also finds it via the word "model"); "block network sockets in the child" ranks `internal/sandbox/network.go` first in hybrid. Changing `query_prefix` gives `vectors_stale` and leaves `graph.vec` byte-identical | info |

## 2026-11-21 — vector index V2 (MCP + recall evaluation, local `embednative` build)

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 45 | `TestSemanticRecall` (`internal/graph/eval/semantic.yaml`, 44 intent queries) | hybrid recall@10 ≥ lexical + 0.10 | ok: all-MiniLM-L6-v2 Q8_0 (sha256 `263215c3cadd6e16740741a7624ab4cbb6c8e777688bd5331ecfbf5681c2f8ed`), llama.cpp b12600 (`29b52d398b29`), 384 dims, local x86-64 build, 8 threads: 417 files and 2701 chunks embedded in 3m03s (`third_party/` and `docs/pt-BR/` skipped). recall@10 lexical 0.273, semantic 0.670, hybrid 0.568 (+0.295), so the feature leaves experimental status. Semantic scored highest on this suite; hybrid keeps exact-name matches. Caveat: the suite was written by the implementing session's tooling after reading the code (intent-phrased, name tokens excluded by a test); independently written queries would be stronger evidence | info |
| 46 | 3 `tb_graph_search` `mode: hybrid` calls over `tb mcp` with the real model | the warm child is reused | ok: about 10 ms per query on the warm child, one child process for the three | info |
| 47 | `tb __graph-embed` with the multi-sequence shim (`tbe_embed_batch`), all-MiniLM-L6-v2 Q8_0, x86-64-v2 build, 4 threads; then `TestSemanticRecall` (8 threads, empty cache) | batching the 64 texts of a request cuts indexing time clearly | small gain. 64 chunks of 1.5 KB: 8.70 s one call per text, 9.22 s with a 2048-token budget (+6%: a non-causal model's attention grows with the square of the packed tokens), 8.80 s with a 512-token budget (same as before: long chunks still go one per evaluation). 64 short queries: 0.31 s → 0.24 s (−21%). Eval indexing (2713 chunks): 3m03s → 2m55s (−4%), recall unchanged (0.273 / 0.670 / 0.568). Vectors agree with single calls to cosine ≥ 0.9999. The cost is the CPU floor (x86-64-v2, no AVX/FMA under ADR-018), not the number of calls | info |

## 2026-11-22 — `tb bus` field session (owner, release v0.0.10)

Recorded for the [1.0 readiness plan](../specs/release-1.0.md) (track B); each row was checked against the code there.

| # | Command | Expected | Actual | Sev. |
|---|---------|----------|--------|------|
| 48 | two clients with `tb mcp --bus-as <channel>/iq3` | the second live client for an identity is refused (or at least warned) | both ran as `iq3` and wrote to the same worktree: `bindBusAs` loads the stored session with no lock; only `tb bus run` takes one (B1) | high |
| 49 | `tb bus send` of `task.assign` with `deps` in the payload, then `tb bus claim` | a worker cannot claim a ticket before its dependency finishes | claimed early: the bus treats `payload` as opaque and has no dependency rule; ticket `dependencies` are honored only by `tb router` (B2, O8) | medium |
| 50 | finding the next task ID in a channel | a list of the channel's tasks with their state | none; needed `tb bus log \| grep` (O10) | medium |
| 51 | reserving paths without assigning a task; releasing a claim; reading how claims ended | an operation for each, and the outcome per claim | no reserve or manual release; `tb_bus_status` shows `released` but not whether it completed, expired or was revoked (B6, O11) | medium |
| 52 | SessionStart hook comparing the installed `tb` | no alarm when only the module hash differs | reported a version mismatch: a build without ldflags reports `dev`/`unknown`, so the hook had only a hash to compare (B5) | medium |
| 53 | `tb bus send` of tickets whose paths overlap | a warning about overlapping paths across tickets | no check (O12) | low |
| 54 | `tb bus invite <channel>` | expiry with date | prints only the time (`15:04:05Z07:00`) (B3) | low |
| 55 | `tb bus run --check` | `wake_mismatch` says how to fix it | states the mismatch only (B4) | low |
| 56 | `tb bus send` of 17 tickets with idempotency keys, sent again | no duplicates | ok: 17 messages, repeats returned `duplicate: true` (regression test in B7) | info |
| 57 | `tb bus send` to an agent not in the channel | clear error | ok: names the agent and the channel (B7) | info |
| 58 | `tb mem list` at session start | invariants right away | ok (B7) | info |
