🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/egm-trust.md)

# Spec: EGM trust, Phase A.2 (CI as the trust boundary)

Author: toolbelt · Status: Draft, round 2 (rewritten after Phase A, one fresh-context review applied); implementation slice 1 (receipt) done · Date: 2026-11-19 · Decision: [ADR-015](../adr/015-egm-nativo.md) · Depends on: [core](egm-nativo.md) (Phase A, implemented)

## Overview

The [core](egm-nativo.md) assumes an honest but sloppy elephant. An elephant that **actively commits fraud** as the same user can reach `PASS` in Phase A: it can point `egm.agent_cmd` at a script, delete the state, or choose the gate's range.

The first draft of this spec answered with nine local defenses: pins, TTY confirmation, a hash-chained anchor, a trust ref, and others. A process of the same user can bypass every one of them. In the Phase A measurement ([dogfood #32 and #33](../campo/dogfood.md)), the draft grew from 73 to 193 lines and never converged.

This version moves the trust boundary **off the developer's machine and into CI**. CI runs with its own credentials, and its workflow and config cannot be changed without a human review. The local gate stays advisory. The only new local piece is a **receipt**: a file committed next to the review that lists the author's claims about the findings, which CI checks.

## Problem

Phase A leaves three gaps:

- **No signal in CI.** The state is private (`$XDG_STATE_HOME`), so the gate in a CI checkout returns `NO_STATE` and `ok: false` (R33a). CI therefore either skips the `egm` op or re-runs the whole loop.
- **The local result is self-reported.** Everything `status` reads (state, config, agent binary, refs) can be edited by the process being judged.
- **The range is chosen by the caller.** `tb stack --base` decides which documents get checked.

## Goals

- **T-G1:** a merged gated document has passed at least one review round that the elephant did not run. The elephant did not choose that round's config or its credential either.
- **T-G2:** the range, the document set and the content under check all come from the CI event and the commit, never from the elephant.
- **T-G3:** every claim the elephant makes is visible in the diff (the receipt) and is either re-checked by CI or fails closed. Whatever remains possible is listed in the Declared Limit.
- **T-G4:** the CI cost is bounded and known in advance: one round per selected document, plus at most one arbiter run per finding that the receipt claims the arbiter withdrew.

## Non-Goals

- Local defenses against a process of the same user (see Dropped Mechanisms).
- Running reviewers under another user or in a VM.
- Unifying git hardening across router, merge and egm (`internal/gitsafe`).
- Drift detection for documents that were not touched (Phase B).
- Hosting providers other than GitHub Actions. The requirements are provider-neutral, but only one example workflow ships.

## Critical User Journeys (CUJs)

- **CUJ-T1.** *Given* a spec that converged locally, with its receipt committed, *when* the PR runs CI, *then* CI checks the receipt against the commit and runs one fresh round. If it finds nothing at `major` or above, the check passes.
- **CUJ-T2.** *Given* an agent that deleted its state and wrote a receipt by hand, *when* CI runs, *then* the fresh round still reviews the document. A `major` that the receipt does not account for fails the check.
- **CUJ-T3.** *Given* a receipt that claims the arbiter withdrew a `major`, *when* CI's fresh round finds the same issue again, *then* CI runs its own arbiter on it. If that arbiter answers `uphold`, the check fails.
- **CUJ-T4.** *Given* a PR that edits `.github/workflows/egm.yml` to skip the step, *when* the PR is opened, *then* CODEOWNERS requires a human review of that file. This protection comes from repository policy, not from tb.

## Technical Plan

### Requirements

- **T-R1 (receipt).** `tb egm receipt <doc>` writes `.toolbelt/egm/receipts/<slug>.json` (R14's slug), plus the MCP tool `tb_egm_receipt`.
  - **When it runs:** only if `status` on the worktree is `PASS` or `PASS_PARTIAL`.
  - **Format:** sorted keys and a trailing newline. It contains no prompts and no credentials, and messages go through R8 redaction.
  - **Fields:** `version`, `doc`, `doc_sha256`, `sources` (path → sha256, the same as the last round's), and `claims`.
  - **What goes in `claims`:** every lineage of `major` severity or above that falls in one of two cases. In the first, the arbiter withdrew it (R29 moves a withdrawn `major` or `blocker` to `fixed`) and it has not been opened or reopened since. The owner's later confirmation keeps the claim. In the second, its status is `declined` or `settled` with a `forced` or `forced_blocker` downgrade.
  - **Each claim holds:** `id`, `severity`, `kind` (`arbiter_withdrawn`, `declined` or `forced`), `heading`, `lines` and `message`. `lines` refer to the document at `doc_sha256`. A claim without `lines` is invalid.
  - **What stays out:** other lineages, because CI never relies on them.

  `.toolbelt/**` is already excluded from sources (core R7), so a receipt never changes the reviewed content.

  A receipt is a claim, not a proof. CI trusts no field without checking it (T-R2).
- **T-R2 (CI mode).** `tb stack` gains the `--egm-mode local|ci` flag (default `local`). The `tb_stack` schema (`internal/registry/parity.go`) gains `egm_mode`. The `ci` mode implies `--strict`, requires `-c`, and runs the fresh round with `--engine llm` (R12). Without `-c`, it fails with exit 2.
  - **Selection:** R33's, over `merge-base(--base, --head)..--head`.
  - **Order of checks:** each selected document is checked in the order below, and the first failure decides its result:
    1. **Receipt present and valid.** The receipt blob exists at `--head` and parses. Every claim has `lines` inside the document. Otherwise the result is `no_receipt` or `receipt_invalid`, with `ok: false`.
    2. **Receipt fresh.** `doc_sha256` and `sources` equal what R7a recomputes at `--head`. Otherwise the result is `receipt_stale`, with `ok: false`.
    3. **Fresh round.** CI runs one round with CI's settings. The state goes to a temporary directory and is discarded. If a required reviewer ends `not_run`, or the budget (T-R6) runs out, the result is `ok: false`. Otherwise, CI's findings of `major` or above go to T-R3.
  - **Deleted or moved documents:** a gated path that is deleted in the range, or renamed to a path that is not gated (for example an index `README.md`), produces `gated_doc_removed` and `ok: false`.
- **T-R3 (fresh-round outcome).** A CI finding matches a claim when R18's `region` holds (same heading, line gap at most `match.gap`) and `sim(message) ≥ match.sim`. Owner and persona are ignored: CI has no snapshots for R19's owner rule or for R20's scope mapping, and the claim's `lines` already refer to the content under check. If the CI finding's severity is higher than the claim's, it counts as unmatched (R20).

  | CI finding (`major` or above) | Result |
  |-------------------------------|--------|
  | Matches an `arbiter_withdrawn` claim | CI runs its own arbiter on the CI finding. `uphold` → `ci_upheld`, `ok: false`. `withdraw` → passes |
  | Matches a `declined` or a `forced` claim | `ci_downgrade`, `ok: false`, with the claim listed for the human |
  | Anything else | `ci_finding`, `ok: false` |

  CI lists findings below `major` under `warn`, and they never fail the check. A human who accepts a `ci_downgrade` merges through the repository's own override (T-R7), which is outside tb.
- **T-R4 (CI without a credential).** When the vendor profile cannot authenticate, for example in a PR from a fork without secrets or with `auth_expired`, the result is `ci_offline` and `ok: false`. The `--egm-allow-offline` flag turns this into a deterministic-only round capped at `PASS_PARTIAL` with `warn`. The flag is meant for workflows that accept that ceiling.
- **T-R5 (CI settings).** The user config comes from `-c <file>`. The workflow writes that file from its own definition, and `stack.Options` passes it to the `egm` op (today the op does not receive it).
  - Repository-only keys still only count from that file (R34). The repository's `.toolbelt/config.yaml` can still only tighten.
  - The credential reaches the reviewer through `egm.env.pass`, for example `ANTHROPIC_API_KEY` from a CI secret. This is the spike's first auth preference.
- **T-R6 (cost).** `egm.ci.max_docs` (default 5) joins R34's limit keys, which the repository can only lower.
  - **Over the cap:** the result is `ci_budget` and `ok: false`, never a silent skip.
  - **Cost per document:** one round (one agent run per required reviewer), plus at most one arbiter run per `arbiter_withdrawn` match.
  - **Budget:** since the CI state is fresh, the core per-cycle `max_agent_runs` (R27) bounds the whole step.
- **T-R7 (repository protection, documented).** `docs/examples/egm-ci.yml` ships a workflow that:
  - runs on `pull_request`;
  - installs a pinned tb release and checks it with `sha256sum -c`;
  - writes the CI config to `$RUNNER_TEMP`;
  - runs `tb stack egm --egm-mode ci -c "$RUNNER_TEMP/egm.yaml" --base ${{ github.event.pull_request.base.sha }} --head ${{ github.event.pull_request.head.sha }}`.

  `--head` is the PR head, not the merge commit. Otherwise a source change on the base side would make every receipt stale.

  The docs require the following repository settings:
  - a required status check;
  - branch protection on the default branch with "include administrators";
  - CODEOWNERS on `/.github/CODEOWNERS`, `/.github/workflows/**` and `/.toolbelt/config.yaml`, with at least one owner other than the author.

  Receipts are **not** under CODEOWNERS, because the elephant updates them on every change.
- **T-R8 (local mode).** The local gate behaves as in R33. It adds `"trust": "local"` to the step result, and `receipt_missing` or `receipt_stale` warnings (never failures), so that the author can see before pushing what CI will reject.

### Dropped Mechanisms

| Draft mechanism | Why it is dropped |
|-----------------|-------------------|
| User root (ignore `XDG_*`/`HOME`) | It protects a local config that CI never reads, and it breaks users who set `XDG_*` legitimately |
| Pinned agent and interpreter hashes | CI installs the agent itself, and locally a TOCTOU (a swap between the check and the `exec`) defeats the pin anyway |
| tb build check (`trust_dev_builds`) | CI pins the tb release, and locally the check only gets in the way of dogfooding |
| Hash chain, anchor, audit trail, `repair` | The same user can rewrite the whole chain, and CI cannot verify it. Forks and clones share the `repo-id`, which produces false rewinds |
| TTY confirmation | A TTY can be forged, and requiring one blocks agentic flows |
| Gate by trust ref | CI gets the range from the event (T-R7), which covers T-G2 without a local ref |
| Per-branch series counters | Series identity under `--restart` is ambiguous, and CI's fresh round makes the local counters irrelevant |
| `internal/gitsafe` | Unrelated to fraud. It moves to its own change |

## Threat Model (trust)

| Elephant action | Effect after A.2 |
|-----------------|------------------|
| Points `agent_cmd` at a script, or edits the user config | Only the local result is affected. CI uses its own config (T-R5) |
| Deletes or edits the state | The local result is lost or faked. CI's fresh round runs anyway (T-R2) |
| Writes a receipt by hand, or makes up claims | The hashes are re-checked. A `declined` or `forced` claim fails as `ci_downgrade`, and an `arbiter_withdrawn` claim is re-arbitrated (T-R3) |
| Writes claims without `lines`, or with `lines` that cover the whole document | Without `lines`, the receipt is `receipt_invalid`. With broad lines, a match is still only possible for a `declined` or `forced` claim (which fails) or an `arbiter_withdrawn` claim (which CI re-arbitrates) |
| Picks `--base`, or deletes or moves the document | The base comes from the event (T-R7). A gated path that leaves the selection gives `gated_doc_removed` |
| Edits the workflow, CODEOWNERS or `.toolbelt/config.yaml` | A human review is required through CODEOWNERS (T-R7) |
| Uses the user's `gh` token to merge as an admin, or to change branch protection | Not stopped by tb. With "include administrators" and a second owner the merge still needs that owner. In a solo repository it is possible (Declared Limit) |
| Waters down the document until it passes | Out of scope: egm judges the document as it is, not what it should have said |

## Declared Limit

- **The fresh round is a single sample.** A `major` that the local loop missed and a single CI round also misses gets merged. The guarantee is "at least one independent round", not convergence in CI.
- **Process versus outcome.** An elephant that skips the local loop, writes a receipt without claims and gets a clean fresh round passes. T-G1 holds. The local loop is just a way to make that round clean.
- **Repository policy is outside tb.** Without the T-R7 settings, or in a repository where the only code owner is also the admin whose `gh` token the elephant can use, the elephant can change the workflow or merge directly. tb cannot detect missing CODEOWNERS or branch protection.
- **Source-only drift is not checked.** A change to a document's sources that does not touch the document itself selects nothing. Detecting it is Phase B.
- **The vendor is trusted.** CI's credential and the vendor's model are outside the threat model.

## Open Questions

- **Flakiness.** How often does a fresh round on a locally converged document produce a new `major`? Measure it on this repository's ADRs and specs before the example workflow drops `continue-on-error`.
- **Repeat runs.** Should CI skip the fresh round when an earlier CI run already checked the same `doc_sha256` and `sources`, for example through a cache keyed by those hashes? Skipping trades T-G1 for cost on re-runs.

## Detailed Implementation Plan

| Slice | Files | Requirements |
|-------|-------|--------------|
| 1 | `internal/egm/receipt.go` (create), `cmd/tb/egm.go`, `internal/registry/egm.go` | T-R1 |
| 2 | `internal/egm/gatestack.go`, `internal/egm/ci.go` (create), `internal/egm/config.go` (`egm.ci.max_docs`), `internal/stack/egm.go` and `stack.go` (`ConfigFile`, `EgmMode`), `cmd/tb/stack.go`, `internal/registry/parity.go` | T-R2 to T-R6, T-R8 |
| 3 | `docs/examples/egm-ci.yml`, `docs/cli.md`, the flakiness measurement in `docs/campo/dogfood.md` (each with its pt-BR mirror) | T-R7, Open Questions |

### Verification Plan

- **Unit:** a receipt round-trip with deterministic bytes; claim selection (arbiter withdraw, decline, forced); each T-R2 check with fake receipts (missing, invalid, stale); `gated_doc_removed`; every row of T-R3 with the egm fake agent in the `findings`, `uphold` and `withdraw` modes; higher severity counted as unmatched; `ci_offline` with and without `--egm-allow-offline`; `ci_budget`; `not_run`; the exit-2 refusal without `-c`.
- **e2e:** a temporary repository with a converged document and its receipt, committed, then `tb stack egm --egm-mode ci` with the fake agent. A hand-written receipt with a `declined` claim that the fake agent re-finds must give `ok: false`.
- **Dogfood:** turn on the example workflow in this repository with `continue-on-error` and record the flakiness rate.

*Based on the EGM pattern*
