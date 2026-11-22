🇺🇸 English | [🇧🇷 Português](pt-BR/bus-protocolo.md)

# Bus protocol (`bus/0.1`)

Specification for stage **B0** of [ADR-013](adr/013-barramento-multi-vendor.md): message envelope, Agent Card, invite and broker rules. Stage **B1** implements this spec in `tb bus` (CLI) and in the `tb_bus_*` MCP tools.

> The ADR names the tools `bus_*`. In `tb mcp` they follow the project prefix: `tb_bus_join`, `tb_bus_send`, `tb_bus_recv`, `tb_bus_claim`, `tb_bus_complete`, `tb_bus_heartbeat`, `tb_bus_status`. `tb_bus_send` takes an optional `parent`; the tree has no MCP tool.

## Store

- File: `.toolbelt/bus/bus.db` (SQLite in WAL mode), at the root of the repository that contains `.toolbelt/`. The `.toolbelt/bus/` directory is created with mode `0700` (which also protects `-wal`/`-shm`) and the database with `0600`.
- No daemon: every command opens the store, runs the operation in a transaction and closes it.
- `PRAGMA user_version` versions the schema; an unknown version is refused (fail-closed).
- Local file, ignored through `.git/info/exclude` (`tb init`).
- **Private directory** (`<private>`): sessions, wake files and harness state live **outside the repository**, in `$XDG_STATE_HOME/toolbelt/bus/<root hash>/` (default `~/.local/state/...`; `TB_BUS_STATE_DIR` changes the base). Directories `0700`, files `0600`. That way an agent working with the repository root as its workdir cannot reach other agents' tokens.

## Envelope

Every message is a JSON-RPC 2.0 object with `method: "bus/message"`. `params` fields:

| Field | Set by | Rule |
|---|---|---|
| `id` | broker | ULID (26 Crockford base32 characters); the sender does not choose it |
| `seq` | broker | increasing integer in the store; defines read order |
| `v` | broker | `bus/0.1` |
| `channel` | sender | existing, non-expired channel |
| `from` | broker | `agent:<name>` of the authenticated session; never taken from the sender. `system:*` (today only `system:watchdog`) is broker-only: no session can send as it |
| `to` | sender | `agent:<name>` (must exist in the channel), `role:<role>` or `topic:<name>` |
| `thread` | sender | optional; `[A-Za-z0-9._:/-]{1,128}` (e.g. `TKT-001`) |
| `parent` | sender | optional; same syntax as `thread`; only on `task.assign` with a `thread` different from `parent`; links the sub-task to a thread the sender holds (see [Delegation trees](#delegation-trees)) |
| `type` | sender | `task.assign`, `task.status`, `query`, `reply`, `ack`, `urgent`; `task.result` only through `complete` |
| `causation_id` | sender | optional; ID of the message that caused this one |
| `idempotency_key` | sender | optional; `[A-Za-z0-9._:/-]{1,128}` |
| `ttl` | sender | optional; Go duration (`30m`); after it the message is not delivered |
| `payload` | sender | a single JSON object, up to 64 KiB; numbers kept without loss; strings with known secrets are redacted before being stored. **Opaque to the broker:** it never reads, validates or orders by it. Ordering by dependency is `tb router`'s job (ticket `dependencies`); a `task.assign` payload with `deps` or `dependencies` is not enforced, and `tb bus send` warns on stderr. Bus-level dependencies are not part of 1.0 |
| `created_at` | broker | RFC 3339 UTC |

Example as returned by `tb bus recv --json`:

```json
{
  "jsonrpc": "2.0",
  "method": "bus/message",
  "params": {
    "id": "01JD7Y3K2V8Q4M6N1P0R5S7T9W",
    "seq": 12,
    "v": "bus/0.1",
    "channel": "feature-x",
    "from": "agent:claude-main",
    "to": "role:worker",
    "thread": "TKT-001",
    "type": "task.assign",
    "idempotency_key": "TKT-001/assign/1",
    "payload": {"ticket": "TKT-001", "ref": "a1b2c3d"},
    "created_at": "2026-11-17T15:00:00Z"
  }
}
```

### Delivery and ordering

- **Order**: increasing `seq`; guaranteed within a thread.
- **At least once**: the sender may resend; with the same `idempotency_key` (same channel and sender) the broker returns the already stored message with `duplicate: true` instead of duplicating it.
- **Backlog**: the cursor starts at 0; a late joiner receives earlier messages addressed to its role and to `topic:*` (including `task.assign`s that still have no owner). The log is durable on purpose.
- **Cursor**: each agent has one cursor per channel. `recv` returns messages with `seq` above the cursor and advances it; `recv --peek` reads without advancing; `recv --since N` re-reads from `N` (the log is durable).
- **Addressing**: an agent receives messages for `agent:<itself>`, for `role:<its role>` and for any `topic:*`. It never receives its own.
- **Long-poll**: `recv --wait D` waits up to `D` (at most 25s, below clients' tool-call timeout) for at least one message.

## Agent Card

Declared on `join`. It is informational: **capability is not permission**; the role comes from the invite.

| Field | Type | Rule |
|---|---|---|
| `name` | string | required; `[a-z0-9][a-z0-9._-]{0,62}`; becomes `agent:<name>`, unique in the channel |
| `vendor` | string | e.g. `anthropic`, `openai`, `xai`, `local` |
| `model` | string | |
| `tier` | string | e.g. `cheap`, `standard`, `premium` |
| `context_window` | integer | ≥ 0 |
| `capabilities` | string[] | |
| `tools` | string[] | |
| `sandbox` | string | e.g. `landlock`, `none` |
| `max_concurrency` | integer | ≥ 0 |
| `wake` | string | `exec`, `harness`, `notify` or `manual` (default `manual`; see [bus-harness.md](bus-harness.md)) |
| `session` | string | vendor session handle (for `--resume` in B3) |

## Invite

- `tb bus channel create` creates the channel and an invite for the owner (role `supervisor`, name pinned by `--owner`).
- `tb bus invite` issues more invites, one per agent, with a pinned role (`router`, `supervisor`, `worker`, `reviewer`). `--name` binds the invite to an agent.
- An **unbound** invite does not accept the channel owner's name nor names that still have an open bound invite.
- Code: 8 Crockford base32 characters in two groups (`K7Q2-9XMD`), generated with `crypto/rand`; the store keeps only its SHA-256.
- Single use; expires after `--ttl` (default `10m`; the CLI prints the expiry as RFC 3339 with date and local zone). A used, expired or other-channel invite, or a valid one used with a name or role other than the pinned ones, is refused with the same generic message (and, in the last case, is not consumed).
- Creating channels and issuing invites does **not** exist through MCP: only the human, on the CLI.

## Session and identity

- `join` returns `agent_id` and role. The session token is random (256 bits), the store keeps its SHA-256 and the token **never** appears on stdout, in logs or in MCP responses.
- CLI: the token goes to `<private>/sessions/<channel>/<name>/token` (`0600`); later commands use `--as <name>`. B1 limitation: any process of the same user can read the file, so the CLI trusts the local user.
- MCP: after `tb_bus_join` the identity is bound to the `tb mcp` stdio connection; the `tb_bus_*` tools do not accept another agent as an argument.
- **Lost session** (MCP server restarted, session file deleted): the name stays registered. The human issues `tb bus invite <channel> --role <role> --name <name>`; a `join` with that invite replaces the token (the old one stops working) and keeps the cursor.
- **B1 trust boundary**: everything runs as the same user. An agent with a shell (including through `tb_run` in MCP) can read `.toolbelt/bus/` and run `tb bus invite`/`channel create`. Invites and per-connection identity protect against prompt injection and misrouting, not against a hostile local process.

## Broker rules (B1)

- `task.assign` only from `router` or `supervisor`; with `parent`, also the checks of [Delegation trees](#delegation-trees).
- `task.result` only through `complete`, by the owner of the thread's claim.
- `claim <thread>`: single owner; returns the claim's `epoch`. A second agent gets an error naming the current owner while that owner's lease is alive. Re-claiming one's own held thread is idempotent. A released claim, or one whose owner lost its lease, is taken over with `epoch + 1` ([bus-lease.md](bus-lease.md)).
- `complete <thread> --epoch E`: owner only, with the epoch `claim` returned; stores `task.result` for `topic:board` and closes the claim. A wrong epoch, a released claim or another owner gives `lease_lost`. A completed thread accepts no new claim.
- Lease: each agent is alive while `now < last_seen + lease` (channel `--lease`, default `2m`); `join`, `heartbeat`, `claim`, `complete`, `send` and a `recv` that advances the cursor renew it, on an active channel.
- An expired channel (channel `--ttl`) refuses `join`, `send`, `claim` and `complete`; `recv` and `log` still read. `tb bus channel extend <channel> --ttl D` (human only) resets the expiry to now + `D` and reopens the channel with sessions and history.
- Every message from another agent is **untrusted data**.

## Delegation trees

Spec: [specs/agent-hierarchy.md](specs/agent-hierarchy.md), decision: [ADR-020](adr/020-agent-hierarchy-a2a.md). The protocol version stays `bus/0.1`: `parent` is additive and absent from every older message, which behave exactly as before. `recv`, `log` and `--json` output carry `parent` on messages that have it. The broker runs these checks inside the `send` transaction, after the idempotency lookup (a duplicate returns the stored message) and before the insert; the client's claim of ownership is never trusted.

| Code | Refused when |
|---|---|
| `parent_requires_assign` | `parent` on a type other than `task.assign` |
| `parent_requires_thread` | `parent` without a `thread` |
| `parent_self` | `thread` equals `parent` |
| `parent_unknown` | `parent` has no earlier `task.assign` in the channel |
| `parent_not_owned` | the sender is not the `router` and does not hold the live claim on `parent` (a claim that is completed, released, revoked or taken over no longer entitles its old owner) |
| `parent_immutable` | `thread` already has a `task.assign` with no link or with another parent; the link is set once, on the thread's first `task.assign`, so cycles cannot form. A re-send to the same parent is accepted, and a later parentless `task.assign` keeps the link |
| `child_claimed` | `thread` has no `task.assign` yet but already has a claim row (claimed or completed without an assign); linking it afterwards is refused |
| `depth_exceeded` | the new link would sit below depth 3 (a root has depth 0) |
| `fanout_exceeded` | `parent` already has 8 open children |

- The limits are constants (depth 3, 8 open children), not configuration. A child is open unless its claim is completed or revoked, or it has no claim and every `task.assign` on it expired by `ttl`; a released claim is open, since it can be taken over.
- Error codes are the error text prefix, like `lease_lost`.
- `complete` on a thread with open children still succeeds and reports them (`open_children`, sorted); the CLI prints a warning on stderr. The `task.result` message is unchanged.
- `tb bus tree <channel>` shows the trees and the state of every node (`revoked`, `completed`, `released`, `claimed`, `expired`, `assigned`) with no payload, result, token or Agent Card `session`. It is CLI only: there is no MCP tool for it.
- Store: schema v4 adds `messages.parent` and the `task_links` table (thread, parent, depth, first assign). The v3 to v4 migration is one-way and runs in the same `IMMEDIATE` transaction as the others; existing threads have no links (all roots). An older `tb` refuses a v4 store, so upgrade every `tb` that serves a channel together.

## Out of B1

Lease, heartbeat and watchdog arrived in B2 ([bus-lease.md](bus-lease.md)). B3 is complete: `tb bus run` ([bus-harness.md](bus-harness.md)), the `notify` adapter and `tb bus discover` ([bus-notify-discover.md](bus-notify-discover.md)). Router integration (B4): [bus-router.md](bus-router.md).
