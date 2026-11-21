🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/egm-drift-bootstrap.md)

# `tb egm drift-report` and `tb egm bootstrap` — Phase B spec

Status: **B1 implemented**; B2 and B3 not implemented (spec round 2, after one fresh-context review).

Phase B of [ADR-015](../adr/015-egm-nativo.md) (§8). It builds on the core ([egm-nativo.md](egm-nativo.md), Phase A) and the trust spec ([egm-trust.md](egm-trust.md), Phase A.2). The upstream names come from [ADR-007](../adr/007-inspiracao-elephant-goldfish.md): `drift-report` "compares docs with current code" and `bootstrap` "documents an existing codebase".

Phase B ships in three parts, each usable alone:

- **B1:** deterministic `drift-report`, plus the CI check that closes the gap egm-trust.md leaves to Phase B (source-only drift, and receipt edits).
- **B2:** deterministic `bootstrap` (a skeleton with its sources wired).
- **B3:** the LLM parts of both commands, and the receipt `commit` field that only the drift diff needs.

## Problem

- **Source-only drift goes unnoticed.** A reviewed document certifies its content and its sources at one point (receipt and local state). When a source changes and the document does not, the `egm` op of `tb stack` only warns `sources_changed`, from local state, and only for documents reviewed on this machine. CI never sees it. A spec can describe code that no longer exists, and every gate stays green.
- **Receipts can change without a review.** A pull request that edits or deletes only `.toolbelt/egm/receipts/<slug>.json` selects no gated document today, so CI checks nothing.
- **Existing code has no documents to gate.** A repository that adopts `tb egm` late has code but no specs or ADRs that cite it. Writing the first draft by hand, with its sources wired, is the step that keeps teams from starting.

## Principles

1. **tb decides, the LLM reads and judges.** tb collects, hashes, compares and records; any model call (B3 only) is an isolated goldfish-like process with no tools.
2. **tb never edits an existing document.** `drift-report` reports; `bootstrap` only creates a new file.
3. **The baseline is what the target branch committed.** In CI, drift and receipt checks compare against the receipt at the merge-base, never against a receipt the pull request supplies.
4. **Offline by default.** B1 and B2 never call an agent. B3 is opt-in per run (`--engine llm`).
5. **Generated text is data.** A bootstrap draft is untrusted, goes into a new file marked as generated, is never committed by tb, and counts only after a normal `tb egm review`.

## Part B1: `tb egm drift-report` and source drift in CI

### Command

```bash
tb egm drift-report                       # every gated document
tb egm drift-report docs/specs/x.md       # some documents (outside egm.doc_globs: error)
tb egm drift-report --at HEAD~3           # as of a commit (default HEAD), from blobs
tb egm drift-report --json
```

The repository config at `--at` governs the comparison (globs and limits), like the Phase A gate. The command is CLI-only (no MCP tool in Phase B).

### Baseline

For each gated document at `--at`, the baseline is, in order:

1. the **committed receipt** at `--at`, when its `doc_sha256` equals the document at `--at`;
2. otherwise the **local state round** for this document whose `doc_sha256` equals it **and whose content and policy have a `PASS` or `PASS_PARTIAL` verdict** (the rounds a receipt could have been written from);
3. otherwise **none**.

A baseline for another version of the document is no baseline: the document itself changed, which is the job of `tb egm review` and the gate.

### Comparison

Drift compares **hashes of the baseline's paths**, not review packets, so the packet budget never decides it:

- Each baseline source path is resolved at `--at` with the Phase A path rules (inside the root, no symlink in any component, tracked in the index with mode `100644` or `100755`, not denied by the config at `--at`), with **no size limit**. If it resolves, its sha256 is compared with the baseline's.
- Separately, every source candidate of the document (R7 links and spans) that resolves under the same path rules, and is not in the baseline, is **added**. Neither the packet budget nor content checks decide it.
- A baseline source over 256 MiB counts as changed (a baseline hash always comes from a smaller R7 source).

| Status | Meaning | Exit |
|---|---|---|
| `fresh` | every baseline path resolves with the same sha256, and nothing was added | 0 |
| `no_baseline` | no receipt or converged round matches this version of the document | 0 |
| `sources_differ` | the baseline paths are unchanged, but a newly resolving path was added | 1 |
| `drifted` | at least one baseline path resolves with a different sha256 (`changed`) | 1 |
| `source_missing` | a baseline path no longer resolves (`missing`, each with the reason: `deleted`, `symlink` (the path or a parent directory), `not_regular`, `denied`) | 1 |
| `unreadable` | the document or the receipt could not be read at `--at` | 2 |

A document gets the most severe status (`unreadable` > `source_missing` > `drifted` > `sources_differ` > `fresh` > `no_baseline`), and every list is reported. The command's exit code is the highest one among the documents; usage errors exit 2, as R32 defines (`ExitError`).

### Output

Text groups documents by status, most severe first. `--json` (a stable key order; `reason` appears only for `unreadable`):

```json
{"at": "<commit>", "documents": [
  {"added": [], "baseline": "receipt", "changed": ["internal/x/x.go"], "doc": "docs/specs/x.md",
   "missing": [], "status": "drifted"}
], "v": "egm-source-drift/0.1"}
```

`baseline` is `receipt`, `state` or `none`; `missing` items are `{"path", "reason"}`.

### CI: receipt edits and source drift

`--egm-mode ci` gains two checks. Both use the **receipt at the merge-base** (`merge-base(base, head)`), so nothing the pull request commits can serve as its own baseline.

1. **Receipt edits select their document.** A range that adds, modifies or deletes `.toolbelt/egm/receipts/<slug>.json` selects that receipt's gated document for the full Phase A.2 check (T-R2: receipt at head, one fresh round, claims). Deleting the receipt of a gated document that still exists, and was receipted at the merge-base, fails with `receipt_removed`.
2. **`ci_source_drift`.** For each gated document the range did not touch, whose merge-base receipt matches its content at the merge-base and was `fresh` there, the comparison above runs at head against that merge-base receipt. A document that becomes `drifted`, `source_missing` or `sources_differ` in this range raises `ci_source_drift`. Documents already drifted at the merge-base do not blame this range.
   - Its severity is `egm.drift.ci`: `warn` (default) or `fail`. It is a policy key: the repository config can only raise it to `fail`.
   - The way to clear it is to update the document or re-review it; check 1 then holds the new receipt to a full review in CI.
   - Both comparisons use the **merge-base** settings (the user config `-c` plus the target branch's repository config), so the pull request's own config cannot lower a limit to hide drift.

The bound is one source collection per gated document at the merge-base and at head, with no agent run; `egm.ci.max_docs` does not apply to it (it bounds agent runs).

### Local gate

The `egm` op of `tb stack` keeps the warning code `sources_changed` (consumers parse it), now computed by the comparison above for documents whose baseline paths changed in the stack range: from receipts first, from converged state rounds when there is no receipt. It stays a warning; a document whose drift cannot be computed gets `drift_unavailable` instead of disappearing.

## Part B2: `tb egm bootstrap`

### Command

```bash
tb egm bootstrap docs/specs/router.md --from internal/router/
tb egm bootstrap docs/adr/020-x.md --from internal/x/x.go --from cmd/tb/x.go
```

- **Target:** must not exist (`O_CREAT|O_EXCL`), must match `egm.doc_globs`, and its parent directory must exist inside the repository with no symlink in any component (as `tb vault new` does).
- **Type:** derived from `egm.doc_types` for the target path, exactly as `required-sections` will derive it at review time. There is no `--type` flag.
- **`--from`:** files or directories inside the repository, at least one.
  - A directory expands to its tracked files (`git ls-files` under it); untracked files are never listed.
  - Each file must pass the Phase A path rules (no symlink in any component, regular, tracked, not denied). A file that does not is skipped and printed with the reason; it is not written into the draft.
  - tb reads no file content in B2.

### Skeleton

tb writes, with no model:

- a title from the target's file name;
- the status line `Status: **draft, generated by tb egm bootstrap** (not reviewed).`;
- every section the derived standard requires (the list `required-sections` checks), each with a `TODO` line;
- a `## Sources` section listing each accepted file as a **Markdown link relative to the document** (links resolve any file name; backtick spans need a known extension).

Before the file is created, tb runs the R7 collection on the assembled text. If the included set is not exactly the accepted files (a file dropped as `secret_like`, or by `max_source_bytes`/`max_packet_bytes`), tb refuses with `bootstrap_sources` and lists why, so a review of the draft never starts with an excluded source (which would be a downgrade that CI fails as `ci_partial`).

### After bootstrap

tb prints the next steps: edit the draft, then `tb egm review <target>`. The draft has no receipt or state, so `drift-report` lists it as `no_baseline`. tb never stages or commits it.

## Part B3: LLM parts

### Receipt `commit` (advisory)

- Receipts may carry `commit`: `HEAD` at write time. It is **advisory**: used only to build the drift diff, never to judge a receipt.
- tb uses it only when the object exists and every receipt source at `commit` hashes to the receipt's value (and the document to `doc_sha256`). Otherwise the report says `no_diff` and falls back to the current content.
- **Rollout:** readers accept `version` 1 and 2 (2 requires a 40- or 64-hex `commit`); tb writes version 2 only with the user-only config key `egm.receipt.write_v2: true`, so a CI pinned to an older tb is upgraded first.

### Drift goldfish (`tb egm drift-report --engine llm`)

`--engine` accepts `deterministic` (the default here, not `auto`) and `llm`.

For each `drifted` document, up to `egm.drift.max_docs` (default 5, a limit key: the repository can only lower it), one drift goldfish runs with the Phase A isolation (no tools, sandbox, disposable HOME, minimal env, nonce-delimited DATA blocks) and the Phase A finding schema. Its packet:

- the embedded `drift` persona: *does each claim of this document about these sources still hold?*;
- the document;
- each changed source **at `--at` in full** (so evidence lines verify against it), within the R7 limits;
- each changed source's unified diff from `commit` to `--at` when `commit` is usable.

Both sides of every diff are built from `Redact` output; a source that is `secret_like` on either side is left out, with the reason; both sides go through `ScanInjectionSource`. Diffs come from tb's hardened git: `CatFileBlob` for each side and a `DiffNoIndex` variant with context lines in a private temp dir (`--no-ext-diff --no-textconv`).

Findings are **reported, not stored** as review lineages, under each document's `findings`. They do not change the exit code. Each run counts against `egm.max_agent_runs` and `max_stdout_bytes`. A vendor profile whose tool use cannot be verified (`tools_check` off) is refused with `tools_unverifiable`; `--engine llm` with no `egm.agent_cmd` fails with `agent_unavailable`.

### Bootstrap draft (`tb egm bootstrap --engine llm`)

One bootstrap goldfish runs, isolated the same way, with the embedded `bootstrap` persona (describe what the code does; leave intent and decisions as explicit open questions), the derived standard, and the accepted `--from` files read like R7 sources (from the index, `Redact` applied, `secret_like` skipped, `ScanInjectionSource` applied). The output contract is a plain Markdown body of at most `egm.bootstrap.max_bytes` (default 32 KiB).

tb assembles the file as the skeleton, with the model body in place of the `TODO` sections, and then:

- removes from the body every HTML comment (tb markers such as `<!-- tb:egm-example -->` included), every level-1 heading, every line starting with `Status:` before the first `##`, and any `## Sources` section;
- runs the Phase A `injection-scan` on the body; a finding aborts with `bootstrap_injection`;
- runs the R7 collection on the assembled file; a source set other than the accepted files (the body cited something else) aborts with `bootstrap_sources`;
- refuses an empty, oversized or non-UTF-8 body with `bootstrap_invalid`.

Nothing is written on an abort.

### Personas

`drift` and `bootstrap` are embedded but kept off the reviewer list: `egm.reviewers` and the arbiter refuse them.

## Out of Phase B

- Opening review lineages from drift findings, or re-certifying automatically.
- Bootstrapping several documents at once, or choosing the files by itself.
- MCP tools for the new commands.
- `draft` and `brainstorm`, symbol-level drift and the other Phase C items.

## Docs to update with the implementation

ADR-015 status, [egm-nativo.md](egm-nativo.md) Non-Goals, [egm-trust.md](egm-trust.md) (the two Phase B mentions), `docs/cli.md` egm section, README roadmap, CHANGELOG, and the pt-BR mirror of each.

## Acceptance criteria

### B1

1. A document receipted at a commit, whose cited source changes later, is `drifted` with that path in `changed`, baseline `receipt`; exit 1.
2. A deleted source, or one that became a symlink, is `source_missing` with the reason; a newly resolving cited path is `sources_differ`; both exit 1.
3. A source that grows past `max_source_bytes` is still compared by hash (`drifted` or `fresh`), not reported missing; another source growing never changes it.
4. A document whose own text changed after its receipt is `no_baseline`; exit 0.
5. Without a receipt, only a converged `PASS`/`PASS_PARTIAL` round serves as baseline (`state`); a `FAIL` round does not.
6. `--at <commit>` reads blobs and the config at that commit, without touching the worktree.
7. An explicit document outside `egm.doc_globs`, and an unreadable receipt, exit 2.
8. In CI, a range that only rewrites a receipt's sha256s selects its document for the full check; a range that deletes the receipt of a still-gated document fails with `receipt_removed`.
9. In CI, a range that changes a cited source of an untouched, fresh document raises `ci_source_drift`, as a warning by default and as a failure with `egm.drift.ci: fail`; the repository config cannot lower `fail` to `warn`.
10. A document already drifted at the merge-base does not raise `ci_source_drift` in an unrelated range.
11. The local gate still emits `sources_changed`, now also for receipted documents never reviewed on this machine.

### B2

12. A bootstrap writes the title, the generated status line, every section the derived standard requires and a `## Sources` section of relative links; the R7 sources of the new file are exactly the accepted `--from` files.
13. Refused with nothing written: an existing target, a target outside `egm.doc_globs`, a symlink in the target path, a `--from` outside the repository.
14. A `--from` directory with an untracked file, a `testdata` file, a denied secrets file and a symlinked component lists none of them and prints each skipped file with its reason.
15. A `--from` file that R7 would exclude (secret-like content, over the limits) refuses the bootstrap with `bootstrap_sources`.
16. tb never stages or commits the draft.

### B3

17. A v2 receipt with a usable `commit` gives the drift goldfish a diff; a v1 receipt, a missing object or a `commit` whose blobs do not match gives `no_diff`, never `receipt_invalid`.
18. Readers accept v1 and v2; tb writes v2 only with `egm.receipt.write_v2: true`.
19. A diff whose old side holds a secret-like value leaves that source out of the packet.
20. Drift findings appear in the report and nothing is written to review state; at most `egm.drift.max_docs` goldfish run.
21. A bootstrap body with an HTML comment, a level-1 heading, a `Status:` line, a `## Sources` section or a citation of another file is cleaned or refused as specified; one that trips the injection scan writes nothing.
22. `tools_check` off refuses `--engine llm` with `tools_unverifiable`; `egm.reviewers` refuses `drift` and `bootstrap`.
