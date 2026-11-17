[🇧🇷 Português](../bus-protocolo.md) | 🇺🇸 English

# Bus protocol (`bus/0.1`)

Specification for stage **B0** of [ADR-013](adr/013-barramento-multi-vendor.md): message envelope, Agent Card, invite and broker rules. Stage **B1** implements this spec in `tb bus` (CLI) and in the `tb_bus_*` MCP tools.

> The ADR names the tools `bus_*`. In `tb mcp` they follow the project prefix: `tb_bus_join`, `tb_bus_send`, `tb_bus_recv`, `tb_bus_claim`, `tb_bus_complete`.

## Store

- File: `.toolbelt/bus/bus.db` (SQLite in WAL mode), at the root of the repository that contains `.toolbelt/`. The `.toolbelt/bus/` directory is created with mode `0700` (which also protects `-wal`/`-shm`) and the database with `0600`.
- No daemon: every command opens the store, runs the operation in a transaction and closes it.
- `PRAGMA user_version` versions the schema; an unknown version is refused (fail-closed).
- Local file, ignored through `.git/info/exclude` (`tb init`).

## Envelope

Every message is a JSON-RPC 2.0 object with `method: "bus/message"`. `params` fields:

| Field | Set by | Rule |
|---|---|---|
| `id` | broker | ULID (26 Crockford base32 characters); the sender does not choose it |
| `seq` | broker | increasing integer in the store; defines read order |
| `v` | broker | `bus/0.1` |
| `channel` | sender | existing, non-expired channel |
| `from` | broker | `agent:<name>` of the authenticated session; never taken from the sender |
| `to` | sender | `agent:<name>` (must exist in the channel), `role:<role>` or `topic:<name>` |
| `thread` | sender | optional; `[A-Za-z0-9._:/-]{1,128}` (e.g. `TKT-001`) |
| `type` | sender | `task.assign`, `task.status`, `query`, `reply`, `ack`, `urgent`; `task.result` only through `complete` |
| `causation_id` | sender | optional; ID of the message that caused this one |
| `idempotency_key` | sender | optional; `[A-Za-z0-9._:/-]{1,128}` |
| `ttl` | sender | optional; Go duration (`30m`); after it the message is not delivered |
| `payload` | sender | JSON object, up to 64 KiB; strings with known secrets are redacted before being stored |
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
| `wake` | string | `harness`, `notify` or `manual` (default `manual`) |
| `session` | string | vendor session handle (for `--resume` in B3) |

## Invite

- `tb bus channel create` creates the channel and an invite for the owner (role `supervisor`, name pinned by `--owner`).
- `tb bus invite` issues more invites, one per agent, with a pinned role (`router`, `supervisor`, `worker`, `reviewer`).
- Code: 8 Crockford base32 characters in two groups (`K7Q2-9XMD`), generated with `crypto/rand`; the store keeps only its SHA-256.
- Single use; expires after `--ttl` (default `10m`). A used, expired or other-channel invite is refused with the same generic message.
- Creating channels and issuing invites does **not** exist through MCP: only the human, on the CLI.

## Session and identity

- `join` returns `agent_id` and role. The session token is random (256 bits), the store keeps its SHA-256 and the token **never** appears on stdout, in logs or in MCP responses.
- CLI: the token goes to `.toolbelt/bus/sessions/<channel>/<name>.token` (`0600`); later commands use `--as <name>`. B1 limitation: any process of the same user can read the file, so the CLI trusts the local user.
- MCP: after `tb_bus_join` the identity is bound to the `tb mcp` stdio connection; the tools do not accept another agent as an argument. Restarting the MCP server requires a new invite.

## Broker rules (B1)

- `task.assign` only from `router` or `supervisor`.
- `task.result` only through `complete`, by the owner of the thread's claim.
- `claim <thread>`: single owner; a second agent gets an error naming the current owner. Re-claiming one's own thread is idempotent.
- `complete <thread>`: owner only; stores `task.result` for `topic:board` and closes the claim. A completed thread accepts no new claim.
- An expired channel (channel `--ttl`) refuses `join`, `send`, `claim` and `complete`; `recv` and `log` still read.
- Every message from another agent is **untrusted data**.

## Out of B1

Expiring lease, heartbeat and Watchdog (B2); `tb bus run`, wake adapters and `tb bus discover` (B3); Router integration (B4).
