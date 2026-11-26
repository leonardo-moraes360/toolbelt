🇺🇸 English | [🇧🇷 Português](../pt-BR/pesquisa/paperclip.md)

# What tb can learn from Paperclip

> **Review date**: 2026-11-26
> **Reference**: [paperclipai/paperclip](https://github.com/paperclipai/paperclip) at commit `dadbc495` (MIT license)
> **Target**: `tb bus` ([internal/bus/](../../internal/bus/), [internal/busrun/](../../internal/busrun/)), `tb egm` ([internal/egm/](../../internal/egm/)), [ADR-008](../adr/008-verificacao-adversarial.md) and the sandbox ([ADR-004](../adr/004-sandbox-isolamento.md))

## What Paperclip is

Paperclip is a Node.js/TypeScript server with Postgres and a React UI that orchestrates "companies" of AI agents: org charts, tickets, dollar budgets, approval gates, routines and many tenants per deployment. Agents (Claude Code, Codex, Cursor, Gemini CLI, …) are woken by a heartbeat and work on tasks.

It is a different product from tb. tb is a local Go CLI with no daemon, and the two share no code that could be ported. The value is in design decisions that Paperclip has already documented for problems tb also has: work that stops for the wrong reason, state that outlives a release, hostile input and slow tests. The license is MIT, so ideas and snippets can be adapted with attribution.

## Findings, by value to tb

### 1. Task watchdog with a stop fingerprint

**Source**: [`doc/TASK-WATCHDOG.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/TASK-WATCHDOG.md)

**What it does.** A watchdog is attached to one task and watches that task and its descendants. It fires only when the whole subtree has stopped (no live run, no queued wake, no scheduled retry). It then computes a SHA-256 over the stopped leaves (ids, statuses, blockers, pending interactions) plus the watchdog's own configuration, and compares it with `lastReviewedFingerprint`. A match means the same stopped state was already reviewed, so it does nothing. A new hash wakes a reviewer agent whose fixed mandate is:

- treat every stopped leaf as a **claim** to be checked against evidence (CI, tests, comments, screenshots). "I could not" and "waiting for approval" are not accepted by default;
- leave genuinely done leaves alone, with a short note on what was checked;
- otherwise reopen, reassign or create a follow-up inside the subtree; if the blocker is real, name who unblocks it and the next action.

Rules that stop it from looping or escalating:

- its own review tasks carry an origin marker (`originKind = 'task_watchdog'`) and are left out of the walk, so it cannot trigger itself;
- it cannot wake itself, edit its own configuration or create another watchdog;
- custom instructions can only narrow its focus; authority is enforced on the server side, per route;
- it cannot approve formal decisions or stand in for a typed reviewer.

Paperclip keeps three mechanisms apart that all use the word "watchdog": the **task watchdog** (subtree stopped), the **silent active-run watchdog** (a running process with no output for N seconds) and **liveness recovery** (in-progress work with no live path).

**Gap in tb.** `tb bus watchdog` ([bus-lease.md](../bus-lease.md), R5) covers only liveness: it releases claims whose owner lost the lease. Nothing reviews a `complete` that was declared without proof, and ADR-008's discriminator runs per task when called, not when a tree of work comes to rest. A silence watchdog for the process run by `tb bus run` was not found in [internal/busrun/](../../internal/busrun/).

**Proposal.** A bus-level "rest review": when every thread of a channel (or of one parent thread) is completed or released, hash the stopped state and, if the hash is new, publish a review task to the router with the ADR-008 mandate. Reuse the existing pieces: the `system:*` sender for trust, epoch fencing for the reviewer's claim, and an origin marker that keeps review threads out of the next hash. No daemon is needed: the check can run on the `complete` that brings the channel to rest, in line with principle 1 of bus-lease.md.

**Priority**: high · **Module**: bus · router · verify

### 2. Upgrade-safe run state

**Source**: [`rules/upgrade-safe-run-state.mdc`](https://github.com/paperclipai/paperclip/blob/dadbc495/rules/upgrade-safe-run-state.mdc)

**What it says.** State saved by one release (sessions, leases, launch config, checkpoints) is routinely resumed by the next one, because deploys happen mid-run. So:

- every PR that changes persisted state says what happens to state saved by the previous release: it carries forward, it is migrated or it is refused;
- refusal uses a **permanent, named** error code, never a generic retryable one. A retry cannot change saved identity, and a replacement can destroy saved state;
- refusal keeps the saved state intact;
- prefer one admission rule that classifies release-incompatible state over one more error code per case;
- each change gets an old-saved/new-resumed regression test, plus a deploy-lands-mid-run case when a run can be in flight.

**Gap in tb.** A search of `internal/` for `schema_version`, "cutover" or "previous release" finds nothing. The bus store, the vault index and the egm receipts are all persisted state that a 1.0.x binary will read after an rc binary wrote it.

**Proposal.** Before v1.0.0, list every persisted format (bus SQLite schema, `.tb/` index, egm receipts, harness lock and heartbeat files), give each a version, and add an old-saved/new-read test per format with a named refusal code. Adopt the PR question ("what happens to state saved by the previous release?") in `CONTRIBUTING.md`.

**Priority**: high (timely for the 1.0.0-rc soak) · **Module**: bus · graph · egm · vault

### 3. Layered prompt-injection guard with fail-closed fallback

**Source**: [`doc/prompt-injection-prevention.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/prompt-injection-prevention.md)

**What it does.** Built-in regex detection is always on. An optional model classifier gets the secret-redacted tool result and a fixed, versioned policy, and a valid verdict from it **replaces** the regex verdict, which lets benign security examples through. Any failure falls back to the regex over the complete original result: no budget, provider error, invalid verdict, a 10-second timeout, or a request body over 256 KiB (never truncated to a prefix). If redaction hides content that would still be delivered to the agent, the model is skipped (`input_redaction_mismatch`) so instructions cannot hide in secret-named fields. The audit log records which detector decided and why it fell back. MCP `resources/read` contents go through the same guard as tool results.

**Gap in tb.** [internal/egm/injection.go](../../internal/egm/injection.go) is regex only (`injection_patterns.txt`).

**Proposal.** Keep the regex as the floor and add an optional classifier behind the egm LLM budget, with the same rules: the regex always runs when the model cannot, the input is never truncated, a redaction mismatch skips the model, and the deciding detector is recorded in the finding.

**Priority**: medium · **Module**: egm · guard

### 4. Low-trust containment preset

**Source**: [`doc/LOW-TRUST-PRESETS.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/LOW-TRUST-PRESETS.md)

**What it does.** `low_trust_review` is for work that reads hostile input (untrusted PRs, external tickets, dependency diffs). Policy comes from three sources (agent, project, run), which are intersected, and the narrowest wins. A risky grant without a concrete scope fails closed. Raw untrusted output is never promoted automatically into the context of a higher-trust agent, and a low-trust agent cannot change persistent instructions or skill configuration.

**Relevance to tb.** The sandbox and capability model already contain the process. What tb lacks is the **context** rule: output from a worker that read untrusted input should not flow into the planner or supervisor prompt without a mark or a review step. That fits the router ([ADR-009](../adr/009-router-e-workers.md)) and the red/blue labs ([ADR-017](../adr/017-appsec-red-blue-labs.md)).

**Priority**: medium · **Module**: router · sandbox · lab

### 5. Test budget enforced by a script

**Source**: [`rules/test-budget.mdc`](https://github.com/paperclipai/paperclip/blob/dadbc495/rules/test-budget.mdc)

**What it says.** PR tests do not sleep more than 1 second on the real clock unless the line is annotated with a reason. Soak, lease-expiry and restart suites are listed in a nightly-only registry. Suites that cover core invariants (isolation, authz, atomic checkout, redaction) can never be moved to nightly. A script (`scripts/check-test-budget.mjs`) enforces all of this on changed files.

**Relevance to tb.** Low for now: the real-clock waits in `internal/bus` and `internal/busrun` tests are all under 1 second (the longest is 300 ms). The "core invariants never move to nightly" rule is still worth writing down for the lease and epoch-fencing tests if a nightly tier is added.

**Priority**: low · **Module**: CI

## Low value

- [`doc/memory-landscape.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/memory-landscape.md): a survey of memory systems (mem0, MemOS, memU, supermemory, …), useful as input for [fontes-primarias.md](fontes-primarias.md).
- [`skills/para-memory-files`](https://github.com/paperclipai/paperclip/tree/dadbc495/skills/para-memory-files): a two-tier entity memory (`summary.md` loaded first, `items.yaml` on demand), close to what `tb mem` already does with summaries and facts.
- [`doc/task-recovery.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/task-recovery.md): retry with `idempotencyKey` and `observedRunId`, plus rules for reusing a failed run's worktree. The bus already has idempotent operations; the `priorWorkspace` hint (path, branch, bounded list of uncommitted files) could inform `tb bus run` retries.
- [`doc/UNTRUSTED-PR-REVIEW.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/UNTRUSTED-PR-REVIEW.md): Docker review of untrusted PRs. bubblewrap+Landlock already cover this.
- [`rules/*.mdc`](https://github.com/paperclipai/paperclip/tree/dadbc495/rules): scoped agent rules with globs and review criteria, a format reference for `AGENTS.md`.

## No value

The org chart, dollar budgets, multiple companies per deployment, the UI, plugins and chat connectors serve a hosted control plane, not a local CLI. `tools/agent-shim` is a 40-line Go `exec` wrapper.

## Suggested order

1. Item 2 before v1.0.0, because every persisted format ships with the release.
2. Item 1 as a bus stage after the release.
3. Items 3 and 4 alongside the next egm and router work.
