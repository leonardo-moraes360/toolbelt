🇺🇸 English | [🇧🇷 Português](pt-BR/bus-lease.md)

# Bus lease, heartbeat and watchdog — B2 spec

Status: **implemented** (B2).

Specification for stage **B2** of [ADR-013](adr/013-barramento-multi-vendor.md). It extends the [`bus/0.1`](bus-protocolo.md) protocol (B1) and the harness ([bus-harness.md](bus-harness.md), B3 first cut).

## Problem

In B1 a claim never expires. If the owner of a thread crashes, loses its session or is simply closed, the thread stays claimed forever: no other agent can take it and nothing tells the human. ADR-013 asks for three pieces:

- a **lease** on what an agent holds;
- a **heartbeat** that renews it;
- a **watchdog** that returns the work of an agent that lost its lease to the board.

## Principles

1. **No daemon.** The store has no background process. Expiry is a fact computed from timestamps when someone looks: a claim, a complete, a heartbeat or a watchdog run.
2. **Liveness is per agent, not per claim.** One heartbeat keeps every claim of that agent in the channel.
3. **Fencing by epoch.** Each claim has an `epoch` that increases every time the thread is claimed after a release or a takeover. `complete` must present the epoch it got from `claim`, so a stale owner, even one with the same name, never completes work that changed hands.
4. **Nothing is lost.** An expired claim is released, not deleted, and the thread can be claimed again; the log keeps the history.
5. **Zero tokens while idle still holds.** No LLM has to heartbeat: the harness and the MCP server do it for the sessions they hold.
6. **Same trust boundary as B1.** A heartbeat is authenticated like any call: the session token is the lease token. An agent cannot keep another agent's lease alive, and a hostile process of the same user remains out of scope.

## Model

- **Channel:** a `lease` duration set by `tb bus channel create --lease D` (default `2m`, minimum `30s`, maximum `1h`). B2 has no way to change it after creation. The heartbeat interval is `lease / 3`.
- **Agent:** `last_seen`, clamped to now when read (a wall clock stepped backwards cannot extend a lease). The agent is **alive** while `now < last_seen + lease`.
- **Claim:**
  - adds `epoch` (from 1), `released_at` and `prev_owner` (the owner before the last takeover);
  - is **held** when it is not completed, not released, and its owner is alive;
  - is **lost** for an agent when `prev_owner` is that agent, or when the agent still owns it and `released_at` is set.

### What renews `last_seen`

| Operation | Renews |
|---|---|
| `join` (and a session replacement), `heartbeat`, `claim`, `complete`, `send` | yes |
| `recv` that advances the cursor, on an active channel | yes |
| `recv --peek`, an empty `recv --wait`, `log`, status, any call on an expired channel | no |

## Rules

- **R1, join:** `join` returns `lease_ends_at` and `heartbeat_interval` alongside the session (ADR-013 §3).
- **R2, claim:** the result includes the `epoch`.
  - A thread nobody holds is claimed with `epoch` 1.
  - A thread whose owner is alive keeps B1's error, naming the owner and the time its lease ends. Re-claiming one's own held thread is idempotent.
  - A thread whose claim is released, or whose owner is not alive, is taken by the caller (the same agent included, for a released claim; an owner that calls `claim` is alive again and keeps its epoch). The claim gets `epoch + 1`, `prev_owner` = the old owner (when it was another agent), `released_at` cleared. If the claim was not yet released, the takeover publishes the release event of R5.
  - Two concurrent takeovers run in `IMMEDIATE` transactions: exactly one wins, the other gets the "already claimed" error.
- **R3, complete:** requires the epoch (`tb bus complete --epoch E`, `epoch` in `tb_bus_complete`). It is refused with `lease_lost` and stores nothing when:
  - the caller is not the current owner;
  - the epoch differs from the claim's;
  - or the claim is released, even by the watchdog with nobody else taking it. The board has already seen `lease_expired`, so the agent must claim again.

  An owner whose lease ran out but whose claim nobody released or took over can still complete; the call renews its `last_seen`.
- **R4, heartbeat:** `tb bus heartbeat --channel C --as N` and the `tb_bus_heartbeat` MCP tool. Each sets `last_seen` and returns:
  - `lease_ends_at` and `heartbeat_interval`;
  - the threads the agent holds, with epochs;
  - the threads it has **lost**, with the new owner when there is one.

  A heartbeat never revives a released claim.
- **R5, watchdog:** `tb bus watchdog C` (CLI only, human or router) releases every claim that is neither completed nor released and whose owner is not alive. For each one it publishes a `task.status` to `topic:board`:

  ```json
  {"state":"lease_expired","thread":"T","previous_owner":"agent:N","epoch":E,"last_seen":"RFC3339"}
  ```

  - The sender is `system:watchdog`. `system:*` is a broker-only sender that no session can use, and the `from` rule of bus-protocolo.md is amended to allow it. Consumers trust a `lease_expired` only from `system:watchdog`; the same text from an agent is just an untrusted message.
  - It is idempotent: an already released claim publishes nothing.
  - `--dry-run` lists without releasing; `--json` prints the list.
  - It is not exposed over MCP, so an agent cannot release other agents' work. R2 releases on demand, so the watchdog matters only when nobody claims the thread.
- **R6, harness:** `tb bus run` heartbeats every interval from its own goroutine, independent of the watch loop and of a wake, so a wake blocked for its whole timeout keeps the lease.
  - It logs `lease_lost` (thread and new owner, never a payload) when a heartbeat reports a new loss.
  - With `--once`, nothing heartbeats after the run exits: claims then live for one lease.
  - The harness never claims or completes on its own.
- **R7, MCP server:** `tb mcp` heartbeats, on a background timer, every bus session bound to its stdio connection while the connection is open. Agents with `wake: manual` or `notify` therefore keep their leases without spending tokens; closing the client ends the heartbeats.
- **R8, status:**
  - `tb bus channel show C` adds the agents (name, role, alive, `last_seen`) and the open claims (thread, owner, epoch, `state` held, expired, released or revoked, and `outcome`: null while held, `expired` once the owner's lease lapsed or the watchdog released it, `revoked` when a supervisor closed it; completed claims leave the open list).
  - The MCP tool `tb_bus_status` returns the same for the caller's channel.

  Neither returns Agent Cards (the `session` field is a vendor resume handle), tokens or payloads.
- **R9, expired channel:** a channel past its `--ttl` refuses `heartbeat`, `claim`, `complete` and `watchdog` like any write, and nothing renews `last_seen` there. `channel extend` reopens it with the old timestamps, so agents that did not heartbeat meanwhile come back as not alive.

## Schema and upgrade

- `PRAGMA user_version` goes from 1 to 2. `migrate` gains a `1 → 2` step in the existing `IMMEDIATE` transaction:
  - `channels.lease_ns INTEGER NOT NULL DEFAULT 120000000000`;
  - `agents.last_seen INTEGER`, set to the **migration time** for existing agents, so an upgrade does not expire everyone at once;
  - `claims.epoch INTEGER NOT NULL DEFAULT 1`, `claims.released_at INTEGER`, `claims.prev_owner TEXT`.
- A fresh store is created directly at v2.
- The `(channel, thread)` row stays the current claim. A takeover overwrites `owner`, `claimed_at` and `epoch`, sets `prev_owner` and clears `released_at`; the history lives in the log.
- The upgrade is **one-way**. Any `tb` that opens the store (even `tb bus log`) migrates it, and an older binary then refuses it, for example a release `tb mcp` running next to a local build. Upgrade every `tb` that serves a channel together.
- `complete` without an epoch becomes a usage error. That is a B1 API change for the CLI and `tb_bus_complete`; the harness prompt tells the agent to pass the epoch it got from `claim`.

## Out of B2

- **Board and reassignment (B4):** the watchdog announces; it does not pick the next agent nor resend `task.assign`.
- **Changing the lease of an existing channel, per-claim leases, per-agent durations, priorities.**
- **Clock sources:** wall-clock time of the local machine, clamped as above; every process runs on one host, as in B1.

## Acceptance criteria

1. With the clock advanced past the lease, a second agent claims the thread of a silent owner and gets `epoch` 2. `topic:board` gets one `lease_expired` from `system:watchdog`.
2. Ownership A → B → A gives epoch 3. A `complete` with epoch 1 fails with `lease_lost` and stores nothing. Before any takeover, a late `complete` with the right epoch succeeds.
3. After `tb bus watchdog` releases A's claim with nobody taking it, A's `complete` fails with `lease_lost`. A claims again and gets epoch 2.
4. Two concurrent takeovers of the same thread: exactly one wins.
5. One heartbeat keeps several claims held. Peek-only or empty `recv`, and any call on an expired channel, do not renew `last_seen`.
6. A heartbeat reports held threads with epochs and lost threads with their new owners.
7. `tb bus watchdog` releases only claims of agents that are not alive, publishes once per claim, and `--dry-run` changes nothing.
8. No session can send as `system:watchdog` (agent names cannot hold `:`, and `system:*` is broker-only).
9. The harness heartbeats at `lease / 3` with a fake clock, including during a wake blocked for its timeout, and logs `lease_lost` when a claim is taken.
10. `tb mcp` keeps a bound session alive with no tool calls, and stops when the connection closes.
11. Migrating a v1 store keeps channels, agents (alive, `last_seen` = migration time), cursors, messages and claims (`epoch` 1). A v3 store is refused.
12. `channel show`, `tb_bus_status`, `watchdog` output and the harness log contain no Agent Card `session`, token, invite code or payload.
13. `tb_bus_heartbeat` and `tb_bus_status` exist in MCP with parity. No MCP tool runs the watchdog.
