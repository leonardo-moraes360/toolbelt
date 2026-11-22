🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/agent-hierarchy.md)

# Spec: delegation trees on the bus, Phase A (`task.assign --parent`, `tb bus tree`)

Author: toolbelt · Status: Draft (round 1, not yet reviewed in a fresh context) · Date: 2026-11-22 · Decision: [ADR-020](../adr/020-agent-hierarchy-a2a.md) · Builds on: [bus-protocolo.md](../bus-protocolo.md) (B1), [bus-lease.md](../bus-lease.md) (B2), [bus-router.md](../bus-router.md) (B4) · Related: [ADR-009](../adr/009-router-e-workers.md), [ADR-013](../adr/013-barramento-multi-vendor.md)

## Overview

A supervisor on a bus channel can already claim a thread and send `task.assign` for sub-work, but the bus does not record that the sub-thread belongs to the first one. This spec adds a broker-checked parent link, depth and fan-out limits, and a read-only tree view. It adds no role, process, daemon or network. Phase A is one PR.

## Current state (2026-11-22)

| Item | State |
|------|-------|
| `task.assign` | Only from `router` or `supervisor` (`validateSend`, `internal/bus/store.go`) |
| `thread`, `causation_id` | Free-form, validated only as `[A-Za-z0-9._:/-]{1,128}`; no link between threads |
| Claims | One row per `(channel, thread)`: owner, epoch, completed, released, revoked (schema v3) |
| `Revoke` | `router` or `supervisor`, any thread; internal API used by the router, no CLI |
| Views | `tb bus log` (messages), `tb bus channel show` (agents, open claims); no tree |
| Store schema | `PRAGMA user_version = 3` |

## Goals

- G1. A `task.assign` can name a parent thread, and the broker accepts it only from someone entitled to delegate that parent.
- G2. Delegation is bounded: a maximum depth and a maximum number of open children per parent.
- G3. The human can see each delegation tree and the state of every node, without payloads.
- G4. Completing a parent with open children is reported to the completer.

## Non-Goals

- Token, cost or time budget inheritance (ADR-020: `tb` cannot measure it).
- A planner role, `tb agent spawn`, or any way for an agent to create agents, channels or invites.
- Refusing `complete` on a parent with open children (reported only; see Q2).
- Cascading revoke of a subtree, or a revoke CLI.
- Router (B4) changes: the router keeps sending parentless assigns, and parented assigns are never router tasks.
- Wake packet or harness prompt changes.
- A2A transport of any kind.

## Critical user journeys

1. **Supervisor splits a ticket.** `claude-main` (supervisor) claims `TKT-001`, then sends two `task.assign`s with `--parent TKT-001` on threads `TKT-001/api` and `TKT-001/ui` to `role:worker`. Workers claim and complete them. `claude-main` completes `TKT-001`.
2. **Human inspects.** `tb bus tree feature-x` shows `TKT-001` with its two children, their owners and states.
3. **Runaway delegation.** A misbehaving supervisor tries a ninth open child or a fourth level; the broker refuses with `fanout_exceeded` or `depth_exceeded`.
4. **Unentitled parent.** A supervisor that does not hold `TKT-001` sends `--parent TKT-001`; the broker refuses with `parent_not_owned`.

## Requirements

### Envelope

- **R1.** `bus/0.1` gains an optional `parent` field, set by the sender, same syntax as `thread`. It is accepted only on `task.assign` (`parent_requires_assign` otherwise) with a non-empty `thread` (`parent_requires_thread`) different from `parent` (`parent_self`). The protocol version stays `bus/0.1`: the field is additive and absent from every existing message.
- **R2.** `recv`, `log` and `--json` output carry `parent` on messages that have it.

### Broker checks (inside the `Send` transaction)

Run after the idempotency lookup (a duplicate returns the stored message, as today) and before insertion.

- **R3. Known parent.** `parent` must have at least one earlier `task.assign` in the channel (`parent_unknown`).
- **R4. Entitled sender.** The sender is the `router`, or holds the live claim on `parent`: the claim row's `owner` is the sender and `completed_at`, `released_at` and `revoked_at` are null (`parent_not_owned`). A supervisor that lost its lease has `released_at` set after the watchdog or a takeover, and is refused.
- **R5. New thread, immutable parent.** If `thread` already has a `task.assign`:
  - with a link to the same `parent`: accepted (a re-send of the same subtask);
  - with no link or a different one: refused (`parent_immutable`).
  A later parentless `task.assign` on a linked thread is accepted and keeps the link. Because a link is only created on a thread's first `task.assign`, after the parent's, cycles cannot form.
- **R6. Depth.** A root (a thread with a `task.assign` and no link) has depth 0. `depth(child) = depth(parent) + 1`, at most **3** (`depth_exceeded`).
- **R7. Fan-out.** A parent may have at most **8 open children** (`fanout_exceeded`). A child is open unless its claim row is completed or revoked, or it has no claim row and every `task.assign` on it has expired by `ttl`. A released claim is open, since it can be taken over.
- **R8.** The limits are constants in Phase A, not configuration (Q3).
- **R9.** Errors use the codes above as the error text prefix, like `lease_lost`, so the CLI, MCP and harness can match them.

### Complete

- **R10.** `Complete` on a thread with open children succeeds as today and adds `open_children` (sorted thread list) to its result; empty is omitted. The CLI prints `warning: TKT-001 completed with 2 open children (TKT-001/api, TKT-001/ui)` to stderr. The `task.result` message is unchanged.

### Tree view

- **R11.** `tb bus tree <channel>` (human, CLI only, like `tb bus log`; works on an expired channel). Flags:
  - `--thread T`: only the subtree rooted at `T`;
  - `--all`: also roots with no children (by default only trees with at least one link are shown, so router channels are not flooded);
  - `--limit N`: at most `N` roots, newest first (default 50);
  - `--json`.
- **R12.** Per node: `thread`, `parent`, `depth`, `state`, `owner`, `epoch`, `assigned_by`, `assign_id`, `open_children`. `state` is, in order: `revoked`, `completed`, `released`, `claimed`, `expired` (no claim, every assign past its `ttl`), `assigned`. No payload, result content, token or Agent Card `session` is shown.
- **R13.** Text output, one node per line, indented two spaces per depth:

  ```text
  TKT-001            claimed    agent:claude-main  epoch 1  open 1
    TKT-001/api      completed  agent:codex-1      epoch 1
    TKT-001/ui       claimed    agent:grok-1       epoch 2
  ```

### CLI and MCP

- **R14.** `tb bus send --parent T`. `tb_bus_send` gains an optional `parent` string. No other MCP change; no MCP tool for the tree.
- **R15.** `docs/bus-protocolo.md` documents `parent`, R3–R10 under the broker rules; `docs/cli.md` gets the `send --parent` flag and the `tree` row. Both with pt-BR mirrors.

### Store

- **R16.** Schema v3 → v4 in the existing `IMMEDIATE` migration, one-way, as in [bus-lease.md](../bus-lease.md):
  - `messages.parent TEXT`;
  - a new table:
    ```sql
    CREATE TABLE task_links (
      channel    TEXT NOT NULL REFERENCES channels(name),
      thread     TEXT NOT NULL,
      parent     TEXT NOT NULL,
      depth      INTEGER NOT NULL,
      assign_id  TEXT NOT NULL,
      created_at INTEGER NOT NULL,
      PRIMARY KEY (channel, thread)
    );
    CREATE INDEX task_links_parent ON task_links(channel, parent);
    ```
  - A fresh store is created directly at v4. Existing threads have no links (all roots). An older `tb` refuses a v4 store, so every `tb` serving a channel is upgraded together.

## Threat model

| Threat | Mitigation |
|--------|------------|
| An agent claims to be a sub-task of someone else's work to gain legitimacy | R4: only the parent's live owner or the router can link |
| Prompt-injected supervisor fans out without bound | R6, R7: broker-enforced depth and fan-out |
| Re-parenting to hide or form cycles | R5: link set once, on the thread's first assign |
| Tree view leaks content | R12: no payload, result or secrets; human-only CLI |
| A remote party drives delegation through an A2A bridge | ADR-020: a bridge is an ordinary agent; as `worker` it cannot assign, and as `supervisor` R4 and R7 still bind it |

Trust boundary unchanged from B1: same local user; invites and per-connection identity protect against injection and misrouting, not a hostile local process.

## Non-functional requirements

- Send with `parent` adds at most four indexed lookups to the existing transaction.
- `tb bus tree` on a channel with 10,000 messages and 500 links: under 200 ms.

## Alternatives

See [ADR-020](../adr/020-agent-hierarchy-a2a.md). In this spec: storing the link only on the message (not a table) was rejected because depth and fan-out checks would scan the log on every send.

## Open questions

- **Q1 (USER DECISION).** Build Phase A at all, or declare the hierarchy done by the router and bus roles (ADR-020 fallback)?
- **Q2 (USER DECISION).** Keep `complete` with open children as a warning (this spec) or refuse it with `open_children` until they are completed or revoked? Refusing needs a supervisor revoke path, which has no CLI today.
- **Q3 (USER DECISION).** Limits: depth 3 and 8 open children as constants, or per-channel options on `tb bus channel create`?

## Detailed implementation plan (one PR)

1. `internal/bus/store.go`: schema v4 (`messages.parent`, `task_links`), migration step 3 → 4; `SendInput.Parent`, `Message.Parent`; checks R1, R3–R7 in `Send`; `insertMessage` and selects carry `parent`.
2. `internal/bus/lease.go`: `CompleteResult.OpenChildren` (R10).
3. `internal/bus/tree.go` (new): `Tree(channel, root string, all bool, limit int)` building nodes and states (R11, R12).
4. `cmd/tb`: `bus send --parent`, `bus tree` (text and `--json`), completion warning.
5. `internal/mcp`: `parent` on `tb_bus_send`.
6. Docs: `docs/bus-protocolo.md`, `docs/cli.md` and pt-BR mirrors; README roadmap item checked; this spec's status.

### Verification plan

- Unit tests in `internal/bus`: each error code in R1–R7; idempotent duplicate with `parent`; re-send to the same parent (R5); parentless re-send keeps the link; depth 3 accepted and 4 refused; 8 open accepted and the ninth refused, then accepted after one child completes, is revoked, or expires unclaimed; released child stays open; lost-lease supervisor refused (R4); router links any known parent.
- Migration test: a v3 store with messages and claims opens at v4 with no links; a v4 store is refused by a v3 reader (version check).
- `Complete` with open children returns them and still completes.
- `tree` tests: states, `--thread`, `--all`, `--limit`, JSON shape, and a golden check that no payload text appears.
- CLI docs test (`TestDocsCLIExamplesMatchCobra`) passes with the new examples; the translation mirror test passes.
- MCP: `tb_bus_send` with `parent` round-trips; tool list unchanged otherwise.
- `tb stack verify guard` PASS; review in a fresh context.
