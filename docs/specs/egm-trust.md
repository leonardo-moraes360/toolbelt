🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/egm-trust.md)

# Spec: EGM trust, Phase A.2 (active fraud by the same user)

Author: toolbelt · Status: Draft, not reviewed · Date: 2026-11-17 · Decision: [ADR-015](../adr/015-egm-nativo.md) · Depends on: [core](egm-nativo.md), R0 spike

## Overview

The [core](egm-nativo.md) assumes an honest but sloppy elephant. This spec gathers the protections against an elephant that **actively commits fraud** with a shell as the same user: an agent that tries to reach `PASS` without a real review by editing config, state, binaries or refs.

The content came from review rounds 2 to 8 of the monolithic version of the spec. There it became clear that this part does not converge while the core is not implemented and the R0 spike has not brought real data. The goal of this phase is to make fraud **detectable and expensive**, not impossible.

## Problem

Against a process of the same user, every local defense can be bypassed, because the attacker reads keys, deletes state, rewrites refs and forges a TTY. With no defense at all, however, an agent can reach `PASS` in seconds, for example:

- pointing `egm.agent_cmd` at a script that returns valid attestations;
- lowering `clean_rounds`;
- deleting the state;
- choosing the gate's range.

## Goals

- T-G1: every known fraud shortcut leaves a verifiable trace, or requires a recorded human step.
- T-G2: the `tb stack` gate cannot be chosen by the elephant: not the range, not the document, not the content.
- T-G3: the declared limit is complete, that is, everything that remains possible is listed.

## Non-Goals

- Preventing fraud by a process of the same user (it is impossible locally).
- Running under another user or in a VM (left for later).
- Implementing the capability tokens of [ADR-005](../adr/005-anti-trapaca.md) §4.

## Technical Plan

The mechanisms below are candidates. Each one became a requirement in the monolithic version and was reviewed at least once. The table summarizes the design and the problems that remained open.

| Mechanism | Summary | Already known problems |
|-----------|---------|------------------------|
| User root | User config, `<state>` and anchors resolved via `os/user.Current().HomeDir`, ignoring `XDG_*`/`HOME`, with the `xdg_ignored` warning; `-c` ignored for sensitive keys | Diverges from `config.GlobalConfigPath()`; affects users with legitimate `XDG_*` |
| Key classes | Execution and secret only in the user config (already in the core); trust-ref policy via `cat-file blob`; `config_untrusted` on divergence | The local trust ref is rewritable; the fallback to the local `main` selects an empty range when the commit goes straight to `main` |
| Pinned agent | sha256 of the binary, the interpreter (including `/usr/bin/env X`) and the runtime trees; tool canary on `pin`; `agent_cmd` different from tb's binary | TOCTOU between the check and the `exec`; tree hashing costs I/O on every run; vendor updates require `pin` |
| tb build | `debug.ReadBuildInfo` per round; a local build downgrades unless `trust_dev_builds` | Gets in the way of dogfooding in this repository |
| Chain and anchor | `hash = sha256(prev ‖ record)`, append-only `audit.jsonl`, separate anchor per `repo-id`, human `repair` | A crash between the audit and the state forks the chain, so a batched WAL is needed; forks and clones share the `repo-id` and produce a false `trust_ref_rewound` |
| Human confirmation | TTY (`os.ModeCharDevice`) for `--force-settle`, `--restart`, `--narrow`, `--past-cap`, `--decline` of a major, `pin`, `repair` and rewind | TTY is forgeable; blocks 100% agentic flows |
| Gate by trust ref | Documents selected by `merge-base(trust_sha, head)..head`, independent of `--base` | Empty selection, or undefined without a merge-base (shallow clone) |
| Per-branch series | Cap and budget counters per branch | Ambiguous series identity with `--restart` |
| `internal/gitsafe` | Unify the git hardening of router, merge and egm | The union of the lists brings `diff.ignoreSubmodules=all` to merge and hides gitlinks in the sensitive-path check; the env without `GIT_CONFIG_*` breaks the router's `withGitNoDetach` and merge's `hooksDirs` |

## Declared Limit (draft)

Even with everything above, a process of the same user can do any of these things:

- edit the user config;
- rewrite local refs and make commits;
- delete or edit `<state>`;
- make a shallow clone;
- rename the document to start over;
- forge a TTY;
- swap pinned files between the check and the `exec`;
- run a modified `tb`;
- set up a hostile `.git/config` in a repository it created itself.

## Open Questions

- Is it worth keeping each mechanism, given that all of them can be bypassed locally? The R0 spike and real use of the core should tell which shortcuts show up in practice.
- Should the strict gate require running under another user, in a VM or in CI with its own credentials, instead of local defenses?

## Detailed Implementation Plan

Not defined yet. This spec only moves on to review after the core converges and R0 is completed.

*Based on the EGM pattern*
