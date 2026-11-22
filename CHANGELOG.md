🇺🇸 English | [🇧🇷 Português](docs/pt-BR/CHANGELOG.md)

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- amd64 embedding kernels at the AVX2 tier (ADR-019, accepted; 3.2× faster indexing on a CI runner, identical recall, +1.2% binary): ggml-cpu is built with AVX, AVX2, FMA, F16C and BMI2 (`TB_X86_KERNELS=v2` keeps the old floor); the rest of `tb` keeps the x86-64-v2 floor. `tb __graph-embed` checks the CPU first and refuses with the new reason `embed_cpu_unsupported`, so native code never runs below the tier. New gate steps: `scripts/avx-audit.py` (VEX/EVEX/BMI2 code only in symbols that libggml-cpu.a alone defines), a test that only the child reaches `native.Open`, and a QEMU Nehalem run of other commands plus the refusal
- Embedding shim: a request's texts are packed into shared native evaluations (`tbe_embed_batch`, `n_seq_max` 64, unified KV, 512-token budget) instead of one call per text; vectors are unchanged (cosine ≥ 0.999). On the x86-64-v2 build: about 4% faster indexing, about 21% faster on short texts

### Added
- `tb egm brainstorm` and `tb egm draft` (Phase C, slices C1–C3, [spec](docs/specs/egm-phase-c.md)). `brainstorm --brief rd.md [--from ...] [--out f]` runs one isolated goldfish (embedded `brainstorm` persona) and writes a concept brief (Problem, Ideas, Recommended direction, Open questions) that is never gated or reviewed: by default under `$XDG_STATE_HOME/tb/egm/<repo-id>/briefs/` (mode 0600), or at `--out`, refused with `brainstorm_gated` when it matches `egm.doc_globs`. `draft <doc> (--body <file> | --engine llm) [--brief] [--from ...]` writes a new gated spec or ADR with a generated status line, every required section (TODO when missing) and `## Sources` of the `--from` files; `--body` is deterministic (no agent), `--engine llm` runs the `draft` persona. The body is cleaned, injection-scanned and may cite only the accepted sources (`draft_sources`); the brief is not cited. `draft --update` never edits a document: it writes a proposal under `<state>/egm/<repo-id>/<slug>/drafts/` and prints a diff. Neither command writes review state, a round or a receipt, so a draft reaches `PASS` only through `tb egm review`. Briefs with a secret-like value or injection text are refused before any agent runs (`brief_secret`, `brief_injection`). New limit keys `egm.brainstorm.max_bytes` (16 KiB) and `egm.draft.max_bytes` (32 KiB); the repository config can only lower them
- Vector index V2 (`docs/specs/vector-index.md`; leaves experimental status): MCP `tb_graph_search` gains `mode` (`lexical`|`semantic`|`hybrid`, default `lexical`; same JSON as the CLI plus a `warnings` array) and `tb_graph_index` gains `vectors` (boolean, default `false`; payload gains `vectors` stats); without them the output is unchanged, MCP queries never reindex vectors, and errors use the same reasons with the warnings appended. `tb mcp` keeps one warm embedding child per server (reused across queries, restarted when the config changes or the child breaks, killed after 5 minutes idle and on exit). CLI and MCP share `internal/vecsearch`. Recall evaluation: `internal/graph/eval/semantic.yaml` (44 intent queries without identifier or file-name tokens) and `TestSemanticRecall` (gated on `TB_EMBED_MODEL` and `TB_EMBED_MODEL_SHA256`; needs `-tags embednative` or `TB_EMBED_EXE`). With all-MiniLM-L6-v2 Q8_0 and llama.cpp b12600: recall@10 lexical 0.273, semantic 0.670, hybrid 0.568
- Vector index V1 (`docs/specs/vector-index.md`): `tb graph index --vectors` (also on `graph rebuild`) builds or updates `.toolbelt/graph.vec` (0600, atomic), re-embedding only chunks whose text sha256 changed; JSON gains `vectors` (`path`, `files`, `units`, `embedded`, `reused`, `dropped`, `denied`, `truncated`, `rebuilt`) and text a `vectors: …` line. `tb graph search --mode lexical|semantic|hybrid` (default `lexical`, output unchanged): `semantic` ranks by cosine over chunks grouped by file node, `hybrid` fuses lexical and semantic with RRF (k = 60, ties by node ID); their JSON is `{"stale", "mode", "stale_vectors", "nodes"}` with `lines`, `score_semantic`, `rank_lexical`, `rank_semantic`, `score_fused`, and text appends `:<lines>` to the path; `--mode` never reindexes. Keys `graph.vector.*` and `graph.embed.*` (`enabled`, `model`, `model_sha256`, prefixes, `threads`, `timeout`, `allow_unsandboxed`, `max_chunks`, chunk shape) come only from the user config (repo values ignored, `config_untrusted`). The `tb __graph-embed` child runs sandboxed (no sockets, read-only empty workdir, private tmp, empty `HOME`, minimal env); deny-listed files are never embedded and files are redacted whole before chunking. New reasons: `vectors_disabled`, `vectors_stale`, `vectors_invalid`, `vectors_too_many`, `model_path`, `model_mismatch`, `model_unsupported`, `embed_unsandboxed`, `embed_protocol`, `embed_crashed`, `embed_timeout`, `config_untrusted`. MCP `mode`/`vectors` and the recall evaluation are V2
- Vector index V0 (ADR-018, `docs/specs/vector-index.md`): every release binary is built with cgo and a vendored, allowlisted llama.cpp subset (`third_party/llama.cpp`, tag b12600, `MANIFEST` with tarball and subset hashes, `scripts/vendor-llama.sh --check`), CPU-only at the x86-64-v2/armv8.0-a floor, cross-built with a pinned zig (glibc 2.28, macOS 11). `tb version` (and `--json` `embed`) names the engine; the hidden `tb __graph-embed --smoke <model> --sha256 <hex>` loads a pinned GGUF model (absolute path, no symlinks, owned by the user, matching sha256) and checks a paraphrase ranks above unrelated text. A build without `-tags embednative` has no native code and answers `embed_unavailable`. Each archive carries the llama.cpp license and manifest under `THIRD_PARTY_LICENSES/`, and each SBOM lists llama.cpp and ggml at the pinned commit (`scripts/sbom.sh`). Release builds use `netgo,osusergo`, like the earlier pure-Go releases; darwin links through generated `.tbd` stubs for the system libraries Go itself imports (`scripts/darwin-stubs.sh`). The release is published as a draft and made public only after a six-runner native smoke matrix, `rockylinux:8` and `qemu -cpu Nehalem` pass. The new `native` CI gate runs the shim under gcc/clang `-Werror`, clang-format, clang-tidy and cppcheck, the embed tests with the smoke model, ASan/UBSan, libFuzzer (5 minutes per PR, 1 hour nightly), a static-initializer strace check, Linux hardening and glibc-floor checks on the artifacts, CodeQL `c-cpp` and a daily llama.cpp advisory check; only `internal/embed/native` may import `"C"` (a test: depguard cannot see the cgo pseudo-package). No graph command uses embeddings yet
- Vector index draft spec (`docs/specs/vector-index.md`) and ADR-018 (proposed): the release moves to cgo for vendored llama.cpp (CPU-only at an x86-64-v2/armv8.0 floor, zig cross builds with named fallbacks, glibc ≥ 2.28, a six-target native smoke matrix) under a `native` C/C++ quality gate (shim lint and static analysis, per-platform hardening checked on the artifacts, sanitizer builds, libFuzzer, an allowlisted vendor subset with manifest and advisory checks); embeddings run in a sandboxed, network-less `tb __graph-embed` child (new seccomp socket block) with a user-pinned GGUF model; `tb graph search --mode lexical|semantic|hybrid` (RRF) stays opt-in, with a recall@10 evaluation gating its experimental status

### Fixed
- Flaky `TestLLMFlowEscape` (and the shell fake behind `TestRunnerEscapeAttributed` / `TestRunnerUnattributedKillsRunning`): the fake agents' `escape` modes ran `setsid` in a background child and exited without waiting for it, so under load the agent could exit while the child was still in its process group; the group kill then took it silently and the reviewer came back `completed` instead of `not_run:escaped_process`. `__fake-agent-egm` now starts the escapee with `setsid(2)` before exec (`exec.Cmd.Start` returns after the exec), and the shell fake waits for a marker the escapee writes after `setsid`. Test-only; escape detection is unchanged

### Security
- `internal/sandbox`: new `DenyNetwork` seccomp filter blocking `socket`, `socketpair` and `io_uring` (used by the `tb __graph-embed` child)
- `internal/secrets`: shared deny list and redaction, used by the vector index before chunking

## [0.0.10] - 2026-11-21

### Added
- `tb egm` Phase B3 (`docs/specs/egm-drift-bootstrap.md`): `drift-report --engine llm` runs one isolated drift goldfish (new `drift` persona) per `drifted` document, up to `egm.drift.max_docs` (default 5, limit key), over the redacted document, each changed source and, when the receipt's advisory `commit` is usable, its unified diff (`diff --no-index -U3` in a private temp dir; a secret-like side leaves the source out); findings are reported under each document, never stored, and do not change the exit code. `bootstrap --engine llm` runs one bootstrap goldfish (new `bootstrap` persona) over the accepted files and puts its Markdown body in place of the TODO sections, after stripping HTML comments, level-1 headings, early `Status:` lines and `## Sources`; an injection in the body or a source (`bootstrap_injection`), a citation of another file (`bootstrap_sources`) or an empty, oversized (`egm.bootstrap.max_bytes`, default 32 KiB) or non-UTF-8 body (`bootstrap_invalid`) writes nothing. Both refuse a profile with unverifiable tool use (`tools_unverifiable`) or no `egm.agent_cmd` (`agent_unavailable`). Receipts may be version 2 with `commit` (HEAD at write time, advisory); readers accept 1 and 2, and tb writes 2 only with the user-only `egm.receipt.write_v2: true`
- `tb egm bootstrap <new-doc> --from <path>...` (Phase B2, `docs/specs/egm-drift-bootstrap.md`): writes a new gated document (never overwriting, in a real directory reached through no symlink) with a title, a generated status line, every section its standard requires (type from `egm.doc_types`) with `TODO`, and a `## Sources` section of links relative to the document. `--from` directories expand to their tracked files; untracked, denied, symlinked and non-regular files are skipped with the reason; the skeleton is refused (`bootstrap_sources`) unless the R7 collection includes exactly the accepted files. No file content goes into the draft (it is read only for that R7 check) and nothing is staged or committed
- `tb egm drift-report` (Phase B1, `docs/specs/egm-drift-bootstrap.md`): compares each gated document with the sources its baseline certified (the committed receipt for these document bytes, else a converged local round) by the hashes of the baseline's paths, with statuses `fresh`, `no_baseline`, `sources_differ`, `drifted`, `source_missing` and `unreadable`, `--at` from blobs and `--json` (`egm-source-drift/0.1`). In `--egm-mode ci`, a receipt edit selects its document, deleting the receipt of a still-gated document fails (`receipt_removed`), and `ci_source_drift` flags drift the range introduced against the merge-base receipt (`egm.drift.ci: warn|fail`, policy key). The local gate's `sources_changed` warning now uses receipts too

### Fixed
- `tb verify`: the env allowlist dropped `PLAYWRIGHT_BROWSERS_PATH`, so Playwright E2E suites failed only inside `tb` (field study: paprika). The browser-path variables (`PLAYWRIGHT_BROWSERS_PATH`, `PUPPETEER_CACHE_DIR`, `PUPPETEER_EXECUTABLE_PATH`, …) are now allowed by exact name, and a non-PASS result lists `filtered_env` (names only, secret-looking names left out)
- `tb verify` / `guard` / `review` / `stack`: the implicit `--base HEAD~1` failed in a single-commit repository, with a `git fetch origin HEAD~1^{commit}` hint even without a remote. When the repository holds only that commit (not a shallow clone or an orphan branch) it now falls back to `HEAD` (empty diff, warning); relative refs get a short-history hint instead of `git fetch origin`
- `tb verify`: `reason` kept ANSI escapes and the first 512 bytes of output, cutting off the failing test. Children now get `NO_COLOR=1 FORCE_COLOR=0`, ANSI is stripped, and the excerpt keeps failure lines plus the summary tail; an identical named failure in two iterations stops with `ESCALATE` instead of rerunning
- `tb graph`: imports of monorepo workspace packages (`pnpm-workspace.yaml`, npm/yarn `workspaces`) became external `pkg:` nodes, so `callers`/`impact` did not cross packages; they now resolve to the package sources through `exports` / `main` / `src/index`, including barrel re-exports. Relative imports in `.tsx`/`.jsx`/Vue/Svelte/Astro/MDX/Angular files no longer leave a `pkg:./X` node
- `tb review`: inside `tb stack`, toolchain criteria ("build, tests and E2E pass") were reported without evidence even after the same stack's verify gave PASS; that verdict now counts. `quality-large-func` findings carry `line`, and React components in `.tsx`/`.jsx` are measured without JSX markup lines (limit 120 logic lines)
- `tb mem init`: no longer seeds the `.toolbelt/thresholds.yaml` invariant when the file does not exist (guard still protects the path)
- Escape sweep (`internal/sandbox` Registry): a process killed together with an escaped group is reparented to the subreaper while still exiting, with its environment already unreadable; it was reported as an unattributable escape, so `tb egm` could flag a round as unattributed (flaky `TestRunnerEscapeAttributed`). An exiting process (`PF_EXITING`) whose environment can no longer be read is now killed and reaped without being reported, like a zombie
- Escape sweep (`internal/sandbox` Registry): an escaped multithreaded process whose leader thread called `exit(2)` while its other threads kept running shows state `Z`, so it was taken for a zombie: never classified, never signaled, and `wait4` cannot reap a leader with live threads, so every sweep spun for its 2 s deadline with the Registry mutex held and the live escape was never killed or reported. A `Z` leader with a live thread is now classified through that thread (`/proc/<tid>`), killed and reported; `SIGKILL` is also sent to zombie children (a no-op for a real zombie)

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- `tb egm` Phase B draft spec (`docs/specs/egm-drift-bootstrap.md`): B1 deterministic `drift-report` (source drift against the committed receipt, by path hashes) and CI checks against the merge-base receipt (`ci_source_drift`, receipt edits selecting their document, `receipt_removed`); B2 deterministic `bootstrap` (a skeleton with linked sources, never overwriting); B3 the LLM drift and bootstrap goldfish and an advisory receipt `commit`

## [0.0.9] - 2026-11-20

### Added
- Bus B4b (`docs/bus-router.md`): `tb router run` bus slots `b1..bN` from `router.bus` (`channel`, `as`, `slots`, `to`, `claim_timeout`), read only from trusted config; `max_workers: 0` for bus only. Each attempt sends a `router-task/0.1` assign on a unique `<ID>/<rand>.<attempt>` thread, accepts only a worker claim with the confinement marker made after the assign (to the addressee when pinned), revokes on unclaimed, lease loss, timeout, kill, ticket change and shutdown (a result stored first wins), and on PASS checks HEAD/branch/base, commits and requires the base as the only parent. New `tb router run --check`, `Ticket.bus_thread`, `Result.bus_agent`/`bus_epoch`, bus fields in `state.json`, bus slots in `--dry-run`, `tb watchdog kill bN`, and alerts `bus_unclaimed`, `bus_claim_rejected`, `bus_lease_lost`, `bus_verdict_fail`, `bus_channel_expired`, `bus_write_failed`
- Bus B4a (`docs/bus-router.md`): store operations `Claim` and `Message` (reads, also on expired channels), `Revoke` (one transaction returning the final state; a completed thread wins; revoked threads refuse claim and complete and show as lost in heartbeat) and `ClaimConfined` (confinement marker); a harness `router_tasks` mode (a dedicated task worker: no ordinary wakes, its own task cursor) that honors `router-task/0.1` assigns only from a pinned router-role agent with validated paths, claims and completes itself outside the sandbox, and runs the child confined to the ticket worktree (read-only `.git` and ticket copy, no bus access, fresh `claude` session, file tools only) with a revoke watcher and a verdict file. The wake packet gains an optional `task` object
- **Breaking:** the bus store migrates to schema v3 on open (one-way, like v2: older `tb` binaries then refuse it)

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- Commit subjects of router tickets drop control characters from the title, for local and bus workers
- Bus B4 draft spec (`docs/bus-router.md`), in two parts. B4a: bus operations `Claim`, `Message`, `Revoke` (returns the final state) and `ClaimConfined` (schema v3), and a harness `router_tasks` mode where the harness claims and completes outside the sandbox while the confined child only edits the ticket worktree and writes a verdict file. B4b: router bus slots `b1..bN`, configured only from trusted config, with unique per-attempt threads, claim validation against the assign and the confinement marker, and the router committing the worktree after HEAD/branch/parent checks

### Added
- `tb lab` red+blue isolated labs, Phase 1 sandbox tier (ADR-017, spec `docs/specs/red-blue-labs.md`): `tb lab new|list|status|up|run|logs|down|verify` over a versioned `labs/<name>/lab.yaml` manifest, with tier resolution (`auto|sandbox|podman`). The sandbox tier drives an in-process, pure-Go scenario (no containers, no sockets, no host env — enforced by an import-guard test) that reproduces a vulnerability class and runs the paired blue check, so `tb lab run sqli` reports `attack=reproduced defense=detected`. The Podman topology tier fails closed with remediation when rootless Podman is absent (never downgrades isolation); runtime state lives outside the repo; the authorized-use notice is shared with `tb red`. New exported `secplay.Corpus.ChecksForClass`. Podman topology driver, cgroups caps, egress-ns test, Containerfile seeds, `tb lab export` and MCP `tb_lab_*` are Phase 2
- Bus B3 remainder (`docs/bus-notify-discover.md`): the `notify` wake adapter for GUI-only agents (`desktop` via a root-owned `notify-send` or `osascript`, `terminal` in-process; fixed English text with counts, types and quoted sender names only; minimal child env; `urgent_urgency`; `wake_mismatch` warning; audit `backend` field) and `tb bus discover` (known agent CLIs on `PATH` and MCP client configs running `tb mcp`, read safely; executes nothing). The wake packet gains `types` and `senders` counts

## [0.0.8] - 2026-11-20

### Added
- Security CI workflows: CodeQL (`go` with a manual build, `actions`, `security-extended` queries; on PR and weekly), OpenSSF Scorecard (weekly and on branch protection changes, published results, SARIF to code scanning) and zizmor (GitHub Actions audit on PRs that change workflows); none of them runs after merge. Their actions are pinned by commit SHA
- Bus B2 (lease, heartbeat, watchdog; spec `docs/bus-lease.md`): per-agent lease (`tb bus channel create --lease`, default `2m`), epoch fencing on claims, takeover of released or expired claims, `tb bus heartbeat`, `tb bus watchdog` (CLI only, publishes `lease_expired` from the broker-only `system:watchdog`), `channel show` with agents and claims, MCP tools `tb_bus_heartbeat` and `tb_bus_status`, background heartbeats from `tb mcp` for bound sessions and from the `tb bus run` harness (`lease_lost` audit)
- Vault in `tb graph` (vault Phase A slice 4): with `TB_GRAPH=on`, notes become `note:<path>` nodes (redacted titles, provenance from `source`/`confidence`), headings sections, resolved wikilinks `links` edges and tags `tag:` nodes with `tagged` edges; the body's Markdown links and mentions keep the note as source. The vault is read through `internal/vault` only, never by the generic indexer, and its entries (minus dot ones) are in the index fingerprint; `vault.graph: false` opts out; `vault` joins the default `graph.docs`. Breaking for graph queries: vault notes were `file:.toolbelt/vault/...` nodes and are now `note:<path>` (links to them are remapped); the generic indexer no longer walks the repository's `.toolbelt/`
- `tb vault new` and `tb vault search`, and the MCP tools `tb_vault_list|show|search|links|backlinks|check|new` (vault Phase A slice 3): `new` writes a note named after the slug of the redacted title, with YAML-encoded frontmatter, `O_EXCL`, no symlinks in `--dir`, and `tb_vault_new` always writes `source: inferred` with a required `confidence`; `search` matches case- and accent-insensitively over each note's redacted text
- `tb vault list|show|links|backlinks|check` and the `vault` op of `tb stack` (vault Phase A slice 2, read-only): root and config read from `<root>/.toolbelt/config.yaml`; safe scan (no symlinks followed, dot directories skipped, FIFOs never opened, `O_NOFOLLOW|O_NONBLOCK` + `fstat`, `vault.max_note_bytes`); typed frontmatter with line numbers; Obsidian wikilink resolution with partial paths, attachments and `ambiguous_link` as an error; note text inside `data` objects and redacted; deterministic output. New config keys `vault.max_note_bytes`, `vault.types`, `vault.graph` with `VaultConfig.Validate`
- `internal/mdparse` (vault Phase A slice 1): leaf Markdown parser with `Slugify`, fence- and frontmatter-aware ATX `Headings`, `SplitFrontmatter`, Obsidian `Wikilinks` (labels, `\|` in tables, fragments, embeds, outside inline code) and `NormalizeForSearch`; no user-facing command yet

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- **Breaking:** `tb bus complete` requires `--epoch` and `tb_bus_complete` requires `epoch` (the value `claim` returned); the harness prompt passes it. The bus store migrates from schema v1 to v2 on open (one-way: older `tb` binaries then refuse it; upgrade every `tb` serving a channel together)
- Bus B2 spec (`docs/bus-lease.md`): per-agent lease with heartbeats, epoch fencing on claims (`complete` takes the epoch), on-demand watchdog sending `lease_expired` from the broker-only `system:watchdog`, heartbeats from the harness and from `tb mcp`, and a v1 → v2 store migration
- The vault is versioned: `.toolbelt/vault/` left `LocalCacheExcludePatterns`; `tb init` excludes only `<vault.path>/.obsidian/`, creates a custom `vault.path`, warns `vault_gitignored` when a `.gitignore` rule ignores the vault, and repairs an exclude section whose end marker was lost without dropping the user's lines. This repository's `.gitignore` now ignores only `.toolbelt/vault/.obsidian/`
- `tb graph` Markdown sections: `#` lines inside the frontmatter or a fenced code block are no longer sections, and a closing `#` sequence is dropped from the heading text (ATX rules). A Markdown file that starts with a `---` thematic break is now read as frontmatter up to the next `---` or `...` line, as in Obsidian

## [0.0.7] - 2026-11-20

### Added
- EGM in CI (Phase A.2 slice 3): the example workflow `docs/examples/egm-ci.yml` (pinned tb release checked with `sha256sum -c`, pinned claude, CI's egm config written in `$RUNNER_TEMP`, the PR's base/head SHAs, `continue-on-error` until flakiness is measured) and the guide `docs/egm-ci.md` (author workflow, repository settings with required check, "include administrators" and CODEOWNERS, failure table). The `claude` profile authenticates with `ANTHROPIC_API_KEY`/`CLAUDE_CODE_OAUTH_TOKEN` from `egm.env.pass` without a credential file. In `--egm-mode ci` an empty selection only warns unless `--strict` is explicit
- `tb stack egm --egm-mode ci` (Phase A.2 slice 2, T-R2 to T-R8): checks the receipt committed at head (`no_receipt`, `receipt_invalid`, `receipt_stale`), runs one fresh `llm` round in a throwaway state with CI's own `-c` config, and matches its `major`+ findings against the receipt claims (`ci_finding`, `ci_downgrade`, re-arbitrated `arbiter_withdrawn` claims or `ci_upheld`); `ci_offline` unless `--egm-allow-offline`, `ci_not_run`, `ci_budget` and `egm.ci.max_docs` (default 5, the repository can only lower it), `gated_doc_removed`; the step reports `trust` and per-document `ci_findings`. The local gate now gets the egm `-c` config and warns `receipt_missing`/`receipt_stale`. MCP `tb_stack` gains `egm_mode`, `egm_config` and `egm_allow_offline`
- `tb egm receipt` and the MCP tool `tb_egm_receipt` (Phase A.2 slice 1, T-R1): writes `.toolbelt/egm/receipts/<slug>.json` with the document and source hashes and the author's claims (lineages at `major` or above that the arbiter withdrew, declined or force-settled, each with `lines` in the current document), deterministic bytes; refused unless the verdict is `PASS` or `PASS_PARTIAL`, when the document or a source differs from `HEAD` (`receipt_uncommitted`), and when the path goes through a symlink (`receipt_path_untrusted`, written through `os.Root`)
- `tb egm` gate (Phase A slice 3): `tb egm status --at <rev>` evaluates the document, its sources (`ls-tree` mode, `cat-file blob` bytes) and the repository config from the commit (R7a); the `egm` op of `tb stack` with `--egm-doc` and `--strict` selects the gated documents added or modified in `merge-base(base, head)..head` and evaluates them at head, with `no_gated_docs`, `uncommitted_gated_docs` and `sources_changed`, and the additive `warn` field in the stack result (R33); the MCP tools `tb_egm_status` and `tb_egm_resolve`, and `egm_docs`/`strict` in `tb_stack` (R31)
- `tb egm` `llm` engine (Phase A slice 2): the 9 reviewers of the consolidated preset as goldfish in a Landlock sandbox (read-only packet, disposable HOME and `TMPDIR`, minimal env, no tools, one shared Registry and subreaper, orphan attribution), the `claude` profile (no-tools flags, `claudeAiOauth`-only credential copy with an `expiresAt` precheck, tool-use detection) and `custom`; original personas and standards, the R9 prompt with nonce-delimited data blocks, a hand-written validator with JSON schemas, the R17 attestation, retries, the agent-run budget and `--dry-run`; fuzzy lineage matching (R19), the owner's re-evaluation where a fix needs a change (`fixed_elsewhere`, `fixed_without_edit`), and `--contest` with the arbiter (R28, R29). An `egmfake` build tag adds the `__fake-agent-egm` test agent
- ADR-015 (proposed) and the Phase A specs for the native EGM (`tb egm`): core (`docs/specs/egm-nativo.md`: LLM goldfish isolated in a sandbox with a disposable HOME and minimal env, no tools, defenses against third-party content, content-based convergence, fresh arbiter) and a draft trust spec against active fraud by the same user (`docs/specs/egm-trust.md`)
- `internal/sandbox`: `ReadOnlyWorkDir`, `PrivateTmp`, `Home`, `Path`, `Stdin`, `Stdout`/`MaxStdoutBytes` (separate stdout, excess drained) and `MinimalDev` fields; Landlock file rules without directory-only rights; an absolute command skips `LookPath`
- `tb egm review|loop next|resolve|status` (Phase A slice 1, deterministic engine): the `refs-exist`, `required-sections` and `plan-files-listed` checkers and the anti-injection scan, lineages with re-run re-evaluation, `--fix`/`--decline`/`--force-settle`, convergence over the same content, cap and `--restart`, R25 verdicts and `report.{json,md}`. Hardened git (closed list of calls, R4c options and env) and `.git` inspection with fatal keys (`hostile_git_dir`, R4d); user config only, the repository's `.toolbelt/config.yaml` can only harden it (`config_untrusted`, R34). State in `$XDG_STATE_HOME/tb/egm/`. The `llm` engine, arbiter, `status --at`, the `tb stack` op and MCP tools come in later slices
- `mem.SecretPatterns()` and `graph.CollectMDLinks()` getters (no behavior change)
- `bench/egm-spike`: R0 spike for `tb egm` (goldfish isolation with a real `claude -p`), with `collect`/`calibrate` modes and the matching corpus `internal/egm/testdata/paraphrases.json`

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- EGM trust spec (Phase A.2, `docs/specs/egm-trust.md`) rewritten around CI as the trust boundary: a committed receipt of the author's claims, checked in a `tb stack --egm-mode ci` step that runs one independent fresh round with CI's own config and credential; the nine local anti-fraud mechanisms of the first draft are dropped
- ADR-015 accepted: status, authentication, the offline `PASS_PARTIAL` ceiling, the `max_rounds` cap and the convergence data aligned with the Phase A results
- CLI and MCP server output is now in English: cobra help (`Short`/`Long`/`Example`), flag usage, error messages, reports (e.g. `guard: ok (3 rules, 0 violations)`, drift `Out of scope` / `Forbidden touched`) and MCP tool/schema descriptions; input parsing still accepts the Portuguese `rd.md` headings (`Escopo`, `Proibido`, `Aceite`)

### Fixed
- Windows build: `tb egm receipt` used `syscall.O_NOFOLLOW`, which does not exist on Windows; the flag is now unix-only (`os.Root` still confines the write)
- `tb stack` `egm` op: a `README.md` index under `egm.doc_globs` is no longer a gated document, so editing `docs/adr/README.md` does not fail the gate with `NO_STATE` (dogfood #38)
- `tb egm loop next` after a convergence over the same `content` and `policy_sha` no longer opens a new cycle and round (about 9 agent runs with the `llm` engine): it shows `already converged over this content in cycle N` with the existing report and exits 0 (R26, dogfood #39)

## [0.0.6] - 2026-11-18

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- English is now the repository's default language: root docs (`README.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `CHANGELOG.md`), `docs/`, `AGENTS.md`, `CLAUDE.md`, `.github/` templates, invariants and code comments; the Brazilian Portuguese translation moves to `docs/pt-BR/` (replaces `docs/en/` and root `*.en.md`); `TestDocsTranslationMirrorComplete` replaces `TestDocsEnglishMirrorComplete`
- Landing page opens in English by default (PT toggle kept)
- Code comments that only restated what the code does were removed; the rest explain how/why
- MCP: `tb_graph_search`/`callers`/`deps`/`impact`/`explain`/`path` never reindex (now `readOnlyHint`); a stale index returns `stale: true` and a `hint` to call `tb_graph_index`. The CLI still honors `graph.index_on_change`
- MCP: `initialize` negotiates the protocol version (`2025-11-25`, `2025-06-18`, `2024-11-05`): a supported version is echoed back, anything else gets `2025-11-25`; previously the server always answered `2024-11-05`
- MCP: `tools/list` returns tools sorted by name (deterministic, cacheable)

### Documentation
- README, site, `docs/cli.md`, architecture, ADR index/status, bus protocol, release, CONTRIBUTING and SECURITY refreshed for v0.0.5 (36 MCP tools, `tb bus` stages B0–B4, `tb mcp --bus-as`, `graph explain`/`path`)

### Fixed
- `tb init`: with a golangci config but no `golangci-lint` on PATH, `verify.lint_cmd` becomes `go vet ./...` instead of a command that would fail (dogfood #1)
- `tb init` / `tb mem`: `config.yaml`, `agent_docs.yaml`, `invariants.yaml` and the seeded `AGENTS.md` are created with `0644`, not `0600` (dogfood #7)
- `tb init` in a `git worktree`: the local-state exclude goes to `$GIT_COMMON_DIR/info/exclude`, the only one git reads (dogfood #17)
- `tb init`: `AGENTS.md`/`CLAUDE.md` hints ignore comments in shell fences, commands quoted mid-line and tool names without a subcommand (`golangci-lint`, `go`) (dogfood #18)

### Added
- MCP: every tool declares `annotations` (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) justified by the handler's real effect, following the M8ven review (`docs/pesquisa/mcp-sinais-m8ven.md`)

### Security
- MCP: `tb_run` runs under Landlock: writes only to the repository root, its own `TMPDIR` and the toolchain caches; reads system dirs, `PATH`, `GOROOT` and `sandbox.read`/`sandbox.write` from the global config; `$HOME` and `/tmp` stay closed and a `cwd` outside the repository is refused. Fails closed without Landlock unless `sandbox.allow_unsandboxed: true`. The result gains `sandboxed`
- MCP: `env.pass` (for `tb_run` and the `pass_env` gate) and the `sandbox.*` keys are read only from the global or `-c` config, never from the repository's `.toolbelt/config.yaml`, which the agent can write
- MCP: the `pass_env` argument of `tb_run`, `tb_verify_run` and `tb_stack` only accepts keys already in `env.pass`; any other key is refused, so a prompt-injected model can no longer opt secrets in (`--pass-env` on the CLI is unchanged)

## [0.0.5] - 2026-11-18

### Added
- Bus B0/B1 (ADR-013): `bus/0.1` spec in `docs/bus-protocolo.md` (JSON-RPC envelope, Agent Card, invite) and `tb bus channel create|list|show`, `invite`, `join`, `send`, `recv`, `claim`, `complete`, `log` over `.toolbelt/bus/bus.db` (SQLite WAL via `modernc.org/sqlite`, no CGO; directory `0700`, database and sessions `0600`)
- Bus: single-use invite with a fixed role (only SHA-256 in the store), `from` set by the broker, dedupe by `idempotency_key`, message and channel TTL, per-agent cursor with `--peek`/`--since`, long-poll up to 25s, single owner per thread, payload ≤ 64 KiB with secret redaction
- MCP: `tb_bus_join`, `tb_bus_send`, `tb_bus_recv`, `tb_bus_claim`, `tb_bus_complete` (identity bound to the connection; creating channels/invites is not exposed)
- Bus: `tb bus invite --name` binds an invite to an agent and recovers a lost session (token rotated); the owner's name and names with an open bound invite are reserved
- Bus B3: `tb bus run` (per-agent harness; LLM-free waiting via `PRAGMA data_version`, `wake_on` filter, grouping with debounce/max_delay, wake budget, rewake/`wake_unread`, retry with backoff and `task.status` `wake_failed`) with adapters `exec` (local command, wake packet on stdin, filtered env + `pass_env`) and `harness` (preset `claude -p --resume --output-format stream-json`, no `bypassPermissions`, persisted `session_id`); spec in `docs/bus-harness.md`
- `tb mcp --bus-as <canal>/<nome> [--bus-root] [--bus-state]`: binds the MCP connection to a bus CLI session
- Bus: sessions, wake files and harness state move out of the repository to `$XDG_STATE_HOME/toolbelt/bus/<hash>/` (`TB_BUS_STATE_DIR` overrides); the woken agent reads only its own token
- Sandbox: `Config.Stdin` (child's standard input, including via the Landlock trampoline)
- `tb bus run`: `wake_topics: false` ignores `topic:*`; the woken claude uses `--strict-mcp-config` and its own temp dir
- `tb init`: `.toolbelt/bus/` is added to `.git/info/exclude`
- `tb bus channel extend <canal> --ttl D`: resets the channel expiration (`0` = no expiration) and reopens an expired channel keeping sessions and history
- `tb graph explain <nó>` and `tb graph path <origem> <destino>` (MCP `tb_graph_explain`, `tb_graph_path`): a node's neighborhood with provenance and the shortest path between two nodes, reimplementing Graphify's queries without an LLM; `calls` edges &lt; 0.6 and targets without a node in the index are left out
- ADR-014: Graphify — reimplement, don't integrate (closes ADR-006's open question; architecture docs no longer cite tree-sitter as implemented)
- `tb graph search` (MCP `tb_graph_search`): pure-Go ranking — exact title, then substring matches (what the old search returned), then terms only; BM25 over title/path/package within each tier, with camelCase/`snake_case` splitting and multi-term queries

### Fixed
- Sandbox: `isSubpath` treated any relative path starting with `../` as internal, so `ValidatePath` accepted paths outside the workdir (e.g. `/etc/passwd`) and the `ForbiddenPaths` check was unreliable; now only strict descendants count
- Sandbox Landlock: removed the write rule for `os.TempDir()` (`/tmp`), which because of the bug above was never applied; shared `/tmp` remains without access (not even read) when it does not contain the workdir, and the docs no longer promise writes there
- Sandbox Landlock: allows reading the target directory of the `/etc/resolv.conf` symlink (e.g. `/run/systemd/resolve`); previously, children with network could not resolve DNS
- `tb bus run`: the audit `detail` (child stdout/stderr) is redacted by known values (`pass_env`, harness secret variables, new `secret_files`) and generic patterns (credential headers, `segredo=valor`, credentials in URLs, long tokens); previously only `pass_env` and the `mem` patterns
- `tb bus run`: with the channel expired, suspends wakes and audits `channel_expired` once (previously it carried on silently); resumes on `extend` (`channel_reopened`)

## [0.0.4] - 2026-11-17

### Added
- Phase 3: `tb ticket` (`create`/`list`/`show`/`cancel`/`retry`; YAML tickets in `.toolbelt/tickets/`, `--from rd.md`, dependencies with cycle detection) and `tb board` (`list`/`watch`/`pause`/`resume`/`drain`; strategies `fifo`/`priority`/`dependency`)
- Phase 3: `tb router run|status|config` — foreground scheduler with workers in their own worktrees (`tb-workspace/<ticket>`), fail-closed sandbox (`--allow-unsandboxed` opt-in), direct delivery (subscriptions with TTL) and a `router:` section in `.toolbelt/config.yaml`; replaces the stub
- Phase 3: `tb watchdog status|alerts|kill` — alerts `worker_slow`, `worker_stuck`, `worker_crash`, `worker_killed`, `queue_growing` and `global_timeout` in `.toolbelt/router/alerts.jsonl`
- Phase 3: `tb merge` — integrates `done` tickets atomically; strategies `fail` (default) and `last_wins`; `prompt`/`semantic` not implemented
- MCP: `tb_ticket_list`, `tb_ticket_show`, `tb_ticket_create`, `tb_board`, `tb_router_status`, `tb_watchdog_alerts`, `tb_merge` (dry-run by default)
- Router: alerts `ticket_canceled` (ticket canceled while running; agent killed, result discarded) and `multi_dep_base` (≥ 2 unmerged dependencies ⇒ base at `HEAD`); a single-dependency ticket starts from that dependency's `head_commit`
- Router: `stalled` summary + `unschedulable` list (`pending` tickets blocked by `failed`/`canceled` dependencies) and `tb router run` exits with code ≠ 0
- Router: a single `tb router run` per repository (exclusive lock `.toolbelt/router/lock`); worktree integrity check and hardened git (fsmonitor/hooks off, `--no-verify`); agent output capped at 1 MiB, logs 0600; exit 0 prevails over a late stuck/kill verdict
- `tb merge` `last_wins`: integration commits without gpgsign/hooks, a failing `--continue` aborts without changing refs, overlap with `--no-renames`; MCP `tb_merge` returns the conflict report with `ok:false`
- `tb merge`: all git hardened (hooks/fsmonitor/external diff off) and a sensitive-paths gate (hooks, `.toolbelt/**`, workflows, `.gitmodules`, gitlinks, `.gitattributes`, guard paths) refused without `--allow-sensitive` (MCP `allow_sensitive`); JSON with `sensitive`
- Sandbox: `Config.Env` and `ProcessGroup` (kills the process group on cancel/timeout) for use by the router

### Fixed
- Sandbox Landlock: `/dev/null`, `/dev/zero`, `/dev/full` become writable (and `/dev/random`/`/dev/urandom` readable) via per-file rules; previously `/dev` was read-only and `echo hi > /dev/null`, `git` and shells failed (`cannot create /dev/null: Permission denied`; also in v0.0.3). `/dev` as a whole remains read-only; ttys (`/dev/tty`, `/dev/ptmx`, `/dev/pts`) are left out on purpose. `IOCTL_DEV` is only handled on ABI ≥ 5 (previously applied from ABI 4)

- State files (tickets/board, locks, `alerts.jsonl`) created with mode 0600 (previously 0644); `tb merge` resolves the absolute path of `git` once instead of searching `PATH` on every call
- `internal/review` tests pin the initial branch (`git init -b master`); previously they failed with `init.defaultBranch=main`

### Documentation
- `docs/cli.md`, README, architecture and ADR-009 updated for Phase 3 (no `router start/stop`, `router.yaml`, `watchdog scale`; merge via `tb merge TKT-001 TKT-002`)
- ADR-013 (proposal): local multi-vendor bus (`tb bus`)
- Dogfooding: the repository uses its own `tb` (`.toolbelt/`, invariants, `.mcp.json`, `CLAUDE.md`) and records friction in `docs/campo/dogfood.md` (PT + EN)

## [0.0.3] - 2026-11-17

### Added
- `tb verify run` / `tb stack`: flags `--build-cmd`, `--lint-cmd`, `--no-build`, `--no-lint` and `--dry-run` (plan with `dir`/`commands` without executing); MCP `tb_verify_run` / `tb_stack` with the same fields
- verify JSON and each stack step's JSON expose `reason`/`error` on ESCALATE/FAIL (not only DEBUG on stderr)
- `tb stack --jobs N`: concurrency for read-only steps (`guard`/`drift`/`review`);
  default = number of CPUs (max 32); `--jobs 1` forces serial; MCP `tb_stack` accepts `jobs`
- Release: SBOM (syft via `sboms:` in GoReleaser), cosign keyless signing of `checksums.txt` (GitHub OIDC) and SLSA provenance attestation (`actions/attest-build-provenance`)
- CI: `govulncheck` job on every push/PR
- CI: weekly Dependabot (Monday) for Go modules and GitHub Actions, with grouped updates
- Full English mirror of the documentation (`README.en.md`, `*.en.md` at the root,
  `docs/en/**`) with language selectors; PT remains the source of truth
  (`docs/en/TRANSLATION.md`, `docs/TRANSLATION.md`)
- Test `TestDocsEnglishMirrorComplete` requires a pair in `docs/en/` for each `.md` in
  `docs/`; `TestDocsCLIExamplesMatchCobra` also validates `docs/en/cli.md`
- `tb sandbox run --allow-unsandboxed`: explicit opt-in to run without Landlock isolation
- Integration test that tries to write outside the workdir (skips if the kernel lacks Landlock)
- TS/JS graph: fingerprint in `graph.json` and stale-index detection;
  with `graph.index_on_change: true` (default) `search`/`callers`/`deps`/`impact`/`stats`
  reindex automatically; with `false`, they warn in the output and expose `stale: true` in JSON
- Resolution of `tsconfig.json`/`jsconfig.json` (`extends`, `baseUrl`, `paths`) in the JS/TS indexer
- Static landing page in `site/` (HTML/CSS/JS, Synthwave theme, PT/EN) published via
  GitHub Pages (`https://leonardo-moraes360.github.io/toolbelt/`)
- Workflow `.github/workflows/pages.yml` (configure/upload/deploy-pages; no `enablement`)
- Guide `docs/site.md` (local preview and how to enable Pages in Settings)
- Global flag `tb --version` (same text output as `tb version`)
- `tb init` writes cache/derived patterns to `.git/info/exclude` (graph, logs,
  vault, workspaces, captures, bench) without touching the project's `.gitignore`;
  invariants/config remain versionable (recommendation documented in
  `docs/cli.md`)
- `tb mem init` reports the path and the invariants created (or already existing); JSON with
  `created`/`count`/`invariants`

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- Dependencies: `cobra` v1.10.2, `pflag` v1.0.9, `golang.org/x/text` v0.44.0, `golang.org/x/sync` v0.25.0; CI/Release/Pages Actions updated
- `tb stack`: `guard`/`drift`/`review` steps run in parallel (errgroup); `verify` stays
  isolated; JSON preserves the requested order; `--fail-fast` cancels the others via context;
  `Prepare` parallelizes independent git/IO
- Binary installation in the README requires `cosign verify-blob` + `sha256sum -c` (removes `curl | tar` and the mention of Homebrew without a tap)
- Go updated to `1.27.4` (`go.mod`, CI/Release workflows); golangci-lint in CI moves to v2.15.0 (compatible with Go 1.27)
- `docs/release.md` documents SBOM, signing, attestation and local validation with `--skip=sign`
- `tb review` / `tb drift` / `tb stack`: `--approved-sha` now uses
  `git patch-id --verbatim` (whitespace-sensitive); JSON always exposes
  `pinned_approval.checked` (`false` when the check did not run);
  `--require-approved-sha` and config `review.require_approved_sha` fail if the
  flag is missing; base/head are resolved to a SHA once at the start
  (anti-TOCTOU) and reused throughout the flow
- **Breaking (graph JSON):** MCP/CLI queries (`search`/`callers`/`deps`/`impact`) now
  emit `{"stale": bool, "nodes": [...]}` instead of a bare array; `stats`/`index` include `stale`
- JS/TS indexer indexes `*.test.*`, `*.spec.*`, `__tests__` and `e2e` (previously omitted);
  `impact`/`callers` now list tests that import the symbol
- JSON outputs use empty lists `[]` (never `null`) for `violations`, `out_of_scope`,
  `forbidden_hit`, `missing` and empty results of `tb mem list|search`
- `tb init --json` and `tb mem init --json` emit a payload of what was created/detected
  (path, detected cmds / invariants)
- `hidden_passed` is `null` when there is no hidden suite (`hidden_available: false`)
- `duration_ms` / `total_duration_ms` in `tb stack` become floats with sub-ms fraction
- `tb stack` preparation/operation errors go in `steps[].error` inside the JSON
  (including unknown op); not only on stderr
- MCP: `all` in `tb_mem_list` / `tb_mem_search` / `tb_mem_check` typed as boolean
  (default `false`); handlers still accept legacy strings `"true"`/`"false"`
- `tb stack` without an operation: short error listing valid ops (`verify`, `review`,
  `drift`, `guard`), without a `Usage` dump (aligned with the domain-error pattern)
- `tb stack` help/`docs/cli.md` make explicit that it stacks verification
  operations and is **not** a branch stack (Graphite/git-stack)
- `tb mem add|edit|list`: reject invalid `--category` and `--ref` outside
  ADR/decisions, listing the valid values in the error
- `tb mem search`: case- and accent-insensitive search (Unicode NFD)
- Docs: `tb drift` is no longer described as “vault vs code”; the actual report is
  scope / forbidden / base commit (+ optional `pinned_approval`)
- Docs: make explicit that `tb` **does not talk to GitHub** (100% local) and needs
  an `rd.md`/ticket on disk; examples of how to generate one from an issue/PR via `gh`
- Docs: language `graph.languages: javascript` covers TypeScript (`ts`/`tsx`); there is no
  separate `typescript` language
- Docs: note the deprecation of `tb guard check --strict` (replaced by the strict
  default and by `--warn-only`); remove the invocable example with `--strict`

### Fixed
- Stack/verify honor command overrides: `--test-cmd` + `--no-build`/`--no-lint` (or empty `--build-cmd`/`--lint-cmd`) skip the config's `build_cmd`/`lint_cmd`
- `tests/hidden/` without an applicable toolchain no longer forces `go test` (marks `hidden_available: false`); detects Go/Python/JS when there is a suite
- Successful build/lint are cached within the same run; a deterministic build/lint failure does not enter the `--max-iter` retry
- `tb review` / `tb drift` / `tb stack` (and MCP `tb_review` / `tb_drift` / `tb_stack`):
  a missing or unreadable ticket (`--ticket` / `ticket`) now fails with exit ≠ 0
  and a clear message citing the path looked up (`ticket não encontrado: …` /
  `ticket ilegível (…): …`), instead of continuing with exit 0
- `--base`/`--head`/`--approved-sha` refs (CLI and MCP in guard/drift/review/verify/stack)
  reject values starting with `-`, resolve via
  `git rev-parse --verify --end-of-options <ref>^{commit}` and use that SHA from
  then on; `git` calls pass `--end-of-options` where applicable. A nonexistent 40-hex
  SHA now fails with a clear error (`ref inexistente ou inválida`)
- `tb review` / `tb drift` / `tb stack`: `--approved-sha` compares
  `approved_base..approved` vs `head_base..head` (not the same `--base` on both
  sides); bases default via `git merge-base`; new flag `--approved-base`;
  range-diff only with `=` (and equal patch-id) counts as a match after a clean rebase;
  `pinned_approval` JSON exposes `approved_base` and `head_base`
- **Sandbox Landlock (fail-closed):** ABI probe uses `LANDLOCK_CREATE_RULESET_VERSION` (previously `flags=0` → `EFAULT` and a false “no Landlock”); `PR_SET_NO_NEW_PRIVS` before `landlock_restrict_self`; full FS rights per ABI (incl. truncate/make_*); ruleset applied in the child via re-exec before the command's `exec`; ruleset errors are no longer swallowed; without Landlock, execution refuses with a clear error (use `--allow-unsandboxed`)
- `tb review` (goldfish): detects only `TODO`/`FIXME`/`XXX`/`HACK` as a whole
  uppercase word in comments (ignores “todo” in Portuguese)
- `tb review` acceptance: each finding links `criterion` + `evidence` from the diff;
  considers added tests (`Test*` in Go; `test`/`it`/`describe` in JS/TS)
- `tb review` invariants: applies mandate rules (e.g. forbid `console.log`,
  no comments) to the diff's added lines
- `tb review`: text output uses `papel=<role>` (and `papéis_acordo` if >1) instead
  of `acordo=N/M`; JSON includes `role` (and `criterion`/`evidence` in acceptance)
- Generic TS functions (`function foo<T>(…)`) show up in the index again
- NodeNext-style relative imports (`.js`→`.ts`/`.tsx`, no extension, `index`) no longer
  stay as `pkg:./…` and resolve to `file:…`
- Import aliases (`import { a as b }`) and namespaces (`import * as ns`, `ns.fn()`) feed
  resolvable `calls` edges
- git errors in `tb guard` / `tb review` / `tb drift` / `tb verify` no longer
  show up only as `exit status 128/129`: they include git's stderr, the missing ref and
  a `git fetch origin <ref>` hint (central helper `internal/git`)
- `--approved-sha` equal to the base no longer dumps `git range-diff` usage;
  clear message about an empty `base..approved` range
- `tb verify run` with a nonexistent command (`test_cmd` / `build_cmd` / `lint_cmd` /
  `hidden_test_cmd`) exits with code **127**; with `--json` emits the
  `verify.Result` envelope (includes `error`) instead of exiting without JSON
- `tb drift` / `tb review` / `tb stack`: file diff now uses three-dot
  (`base...head` / merge-base), avoiding false `out_of_scope` after merging main
  or squashing another PR into the base
- `tb drift`: an explicit `--base` in the CLI/MCP is no longer overridden by the
  ticket's `base_commit`; `base_mismatch` uses `git merge-base --is-ancestor`
  on the declared `base_commit` (or on the effective base if absent), instead of
  string equality with `--base`; JSON includes `actual_base` and `merge_base`
- `tb verify run --base/--head`: when head resolves to a SHA different from the
  worktree's `HEAD`, build/lint/test/hidden run in a temporary `git worktree add --detach`
  (removed at the end, including on error/signal); the JSON exposes `head_sha`
  and `workdir`. If `head == HEAD` with a dirty worktree, emits a warning. The same applies to
  `tb stack verify` via the shared context.

### Security
- Child processes of `tb verify` / `tb stack` (verify) and MCP `tb_run` /
  `tb_verify_run` / `tb_stack` now receive an allowlist-filtered environment
  (PATH, HOME, LANG, TMPDIR, GOPATH/GOCACHE, NODE/npm/pnpm, etc.). Secrets
  (`*TOKEN*`/`*SECRET*`/`*KEY*`/`*PASSWORD*`) never pass without explicit opt-in
  (`--pass-env` / `env.pass` / MCP arg `pass_env`). Extension via `--env` /
  `env.allow` / MCP arg `env`.
- `tb mem`: `immutable` invariants or category `anti_cheat` require `--force` + `--reason` in `edit`/`retire`; append-only history in `.toolbelt/invariants_history.jsonl`
- `tb mem add`: new invariants are born **mutable**; use `--immutable` (MCP: `immutable`) to protect them; `mem init` keeps creating immutable defaults
- `tb mem check`: fails **closed** (unreadable/corrupt file, mismatched SHA, missing IDs, unevaluable rule → exit ≠ 0); does not auto-initialize defaults
- Secret redaction (`ghp_`/`gho_`/`github_pat_`, `AKIA`, PEM, `Bearer`, `xox*`, `*_TOKEN=`) in `mem show`/`list`/`search` (CLI `--json` and MCP)

## [0.0.2] - 2026-11-16

### Added
- `tb stack <op> [<op>…]` stacks `verify`/`review`/`drift`/`guard` in one run with
  shared context (diff, file index, memory and config); aggregated JSON
  (`ok`/`steps`/`total_duration_ms`); `--fail-fast`; MCP `tb_stack` with a typed schema
- MCP `tb_graph_impact` (parity with `tb graph impact`)
- `tb review` / `tb drift`: flag `--approved-sha` compares `git patch-id --stable`
  of `base..approved` vs `base..head`; on divergence shows `git range-diff` and
  exits ≠ 0; JSON records `pinned_approval` (`approved_sha`, `patch_id`,
  `head_patch_id`, `match`, `range_diff`)
- JS/TS indexer emits resolvable `calls` edges (calls + JSX `<Comp />`) via relative imports; `tb graph callers`/`impact` work on Next/React fixtures (`pages/` → `components/`/`lib/`)
- `tb mem show|edit|retire|search` and `--ref` link to ADR/decisions.md; soft-retire omitted from list/check/search (use `--all`); unknown subcommand exits ≠ 0
- MCP: `tb_mem_show`, `tb_mem_edit`, `tb_mem_retire`, `tb_mem_search` (+ `ref`/`all` in add/list/check)

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- MCP schemas type `depth`/`limit`/`reviewers`/`max_iter` as integer, `strict`/`warn_only`/`transitive`/`skip_unavailable` as boolean, `threshold` as number and `standards` as array; handlers still accept legacy strings
- **Breaking:** CLI JSON outputs (`--json`) and MCP domain payloads use
  `snake_case` keys exclusively. Untagged structs (`sandbox.Result`,
  `mem.Invariant`, `workspace.Workspace`/`Scope`) no longer emit PascalCase
  (`ExitCode` → `exit_code`, `TimedOut` → `timed_out`, `duration_ns`, etc.).
  MCP protocol envelopes (`isError`, `inputSchema`, …) remain camelCase.
- Frameworks (e.g. `react` in `.tsx`) run in parallel with the language indexer, not instead of it
- `verify.build_cmd` in Node also accepts `typecheck` / `type-check` scripts when
  `build` is absent
- `tb guard check` exits with exit ≠ 0 by default when there is a `SeverityError` (CI-safe); opt-out via `--warn-only`; `--strict` remains as a deprecated alias
- Clean text mode prints `guard: ok (N regras, 0 violações)`; JSON/MCP include `rules_checked`
- MCP `tb_guard_check`: `strict_fail` by default on SeverityError; `warn_only=true` to opt out (`strict` deprecated)
- `tb review`: distinct goldfish per role (`scope`, `quality`, `acceptance`) instead of the same checker cloned N times; `--reviewers` only limits roles; quality covers TODO/FIXME, large functions and untested code; acceptance requires evidence in tests/code files

### Fixed
- `tb guard check`: renames (`R*`) check source and destination;
  `protected_paths` matching is case-insensitive; globs with `**` work; invalid
  rules do not count toward `rules_checked`; config read from the **base**
  (`git show base:.toolbelt/guard.yaml`) and changing that file is a
  violation; malformed YAML fails closed (exit ≠ 0); the scan uses the
  diff/`git show head:path` (not the working tree); detection of JS/TS test
  weakening (`skip`/`only`/`xit`/trivial assertions)
- CLI long description: completes the adjective in pt-BR (`persistent` → `persistente`)
- `docs/cli.md` aligned with the real CLI: removes `graph callers --file`; `router` documented as a stub; planned sections (`proc`/`agent`/`watchdog`/`merge`) marked; `init`/`sandbox`/`workspace`, mem show/edit/retire/search, guard `--warn-only`, review standards and exit codes checked against `--help`/MCP
- Test `TestDocsCLIExamplesMatchCobra` extracts `tb …` from `docs/cli.md` and validates subcommands/flags against the Cobra tree
- `tb verify run` fails immediately (no retries) when the configured command does not exist (exit 127), is not executable (exit 126) or `exec` returns `ErrNotFound`, with the message `comando não encontrado: … — ajuste verify.test_cmd` (and equivalents for build/lint/hidden)
- `tb init` in Node/TS repos without `go.mod` no longer keeps `verify.test_cmd: go test ./...`:
  detects the package manager (`pnpm-lock.yaml`/`yarn.lock`/`package-lock.json`/`bun.lockb`),
  fills `test`/`build`|`typecheck`/`lint` from `package.json`, and if nothing is
  detectable leaves `test_cmd` empty with a warning; the `AGENTS.md` seed reflects this
- CLI: expected `RunE` errors (domain/runtime) no longer print Cobra's `Usage` dump to stderr; `Usage` remains only for genuine flag/arg parse errors. Unknown subcommands at any level exit with exit ≠ 0 and a clear message.
- Disabled-graph error (CLI and MCP) says how to enable it: `--enable` or `TB_GRAPH=on`

## [0.0.1] - 2026-11-16

### Added
- Release CI: GoReleaser (`.goreleaser.yaml`) + workflow on `v*` tags publishing `tb` binaries (linux/darwin/windows × amd64/arm64), `checksums.txt` and changelog
- Installation docs via GitHub Releases and `docs/release.md` guide
- Default version `dev` injectable via ldflags (`tb version` / MCP `tb_version`)
- `tb init --write-agents` seeds `AGENTS.md` citing the detected toolchain (`verify.test_cmd` and, if present, `build_cmd`/`lint_cmd`)
- MCP: CLI-parity tools `tb_mem_*`, `tb_verify_run`, `tb_review`, `tb_drift`, `tb_guard_check`
- Rust indexer emits minimal `calls` edges resolvable within the repository (`tb graph callers` / `impact`)
- `tb graph callers`/`impact` return an explicit error on C/C++ (no `calls` edges) instead of a silent `(vazio)`
- `tb init` fills `verify.build_cmd` and `verify.lint_cmd` from the toolchain (`build`/`lint` scripts, golangci, ruff/mypy, `cargo build`/`clippy`, make targets)
- `tb guard check` accepts `--base`/`--head` (like review/drift/verify) besides the positional range `base..head`
- Python indexer emits `calls` edges resolvable within the repository (`tb graph callers` / `impact`)
- `tb init` reads `AGENTS.md`/`CLAUDE.md` at the root (records them in `.toolbelt/agent_docs.yaml` and extracts command hints; never overwrites) and accepts `--write-agents` to seed a minimal `AGENTS.md` pointing to tb
- `tb guard` loads `protected_paths` from `.toolbelt/guard.yaml` (merged with the defaults; a missing file keeps only the defaults)
- `tb init` detects the toolchain (`go.mod`, `package.json`/npm/pnpm/yarn, pyproject/uv/pytest, `Cargo.toml`, `Makefile`) and fills `verify.test_cmd`
- Visual ruler in `tb verify run --ruler visual` (headless screenshot, pixel similarity, graceful skip)
- Fresh-context `tb review` (Elephant-Goldfish) against ticket/rd.md and `tb mem` invariants
- `tb drift` for changes outside the declared scope and base-commit divergence
- On-demand `tb graph` (ADR-011): index/search/callers/deps/impact/stats behind `TB_GRAPH`/`--enable`
- Graph MCP tools: `tb_graph_search`, `tb_graph_callers`, `tb_graph_deps`, `tb_graph_index`
- A/B comparison in `tb bench compare` (YAML configs / feature flags on the same task set)
- Success, time and token metrics in the JSONL and in the A/B Markdown report
- Realistic fixtures: `localize-callers`, `cross-file-rename`, `api-handler`
- Example A/B configs in `bench/ab/` (`graph-on`, `graph-off`, `baseline`)
- CLI skeleton with Cobra (`tb version`, `tb init`, `tb help`)
- Hierarchical configuration (global `~/.config/toolbelt` + local `.toolbelt/`)
- Structured logging with `log/slog` and Synthwave theme
- Support for `NO_COLOR` and `TERM=dumb` for colorless environments
- `--json` flags for structured output
- `tb bench run` - harness to run benchmarks against agents
- `tb bench list` - lists available benchmark tasks
- `tb bench report` - generates a Markdown results report
- `tb bench compare` - compares two benchmarks
- Fake agent (`fake-agent`) for CI tests without an LLM
- 5 fixture tasks: `bugfix-go`, `feature-small`, `refactor-simple`, `test-coverage`, `doc-function`
- Results in JSONL format for programmatic analysis
- Metrics: pass@1, pass@k, tokens, time, tool calls
- Makefile with targets: `build`, `test`, `lint`, `bench-fake`
- GitHub Actions CI: test, lint, build, bench-fake
- golangci-lint configuration
- goreleaser configuration for releases
- ADR-012: CLI framework - Cobra
- Stubs for future commands: `sandbox`, `verify`, `mem`, `graph`, `router`, `mcp`

### Fixed
- `tb graph callers`/`impact` in Python: resolves by qualified-name suffix and ignores `calls` edges with confidence &lt; 0.6 (reduces spurious targets like `setup.py`→`tests.setup`)
- docs/cli.md aligned with the real CLI: `graph index [path]` (no `--path`), no `graph query`, `review`/`drift` only `--output` (no `-o`), graph/mem flags checked against `--help`
- `tb drift` resolves git refs (`HEAD^`, tags, short SHA) via `rev-parse` before
  comparing with `base_commit` (avoids false `base_mismatch`)
- `tb verify run --json` exits with exit ≠ 0 on `ESCALATE`/`FAIL` (aligned with text mode)
- `tb init`: `AGENTS.md`/`CLAUDE.md` hints ignore long code fences and
  accept short prose (“use/run/prefer …”); `verify.test_cmd` validates the binary on
  `PATH` with fallback (e.g. missing `uv` → `pytest` / `python -m pytest`)
- `tb bench`: honors `bench.tasks_dir` / `fixtures_dir` / `output_dir` from config.yaml
  (precedence: CLI flag > YAML > default); the global flag `-c/--config` feeds `config.Load()`
- `protected_paths` with a trailing slash (`secrets/`) now protects the directory's children
- `tb review`: invariant `tests/hidden` (and `thresholds.yaml`) matches by path prefix/segment
  in the changed files, not by substring in the diff (avoids FP when the ticket
  only mentions the rule)

### Changed
- `tb egm draft`: a new draft and `--update` now gate the target by the same globs (`egm.doc_globs` plus the repository's extra gated globs; `bootstrap` shares the helper), and `--body` with a secret-like value is refused (`draft_secret`) before anything is written. Spec R10 now exempts the `drafts/` proposal directory of `--update`
- README updated with installation, quick usage and implemented commands
- docs/cli.md marking implemented commands
- docs/benchmark.md with a section on running in Phase 0

## [0.0.0] - 2026-11-15

### Added
- Complete architecture documentation
- ADRs 001-011 defining technical decisions
- Roadmap in 3 phases (MVP, Agent Hierarchy, Advanced Tools)
- Primary-source research (100+ papers and repositories)
- Synthwave visual identity
- Issue and PR templates
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md
