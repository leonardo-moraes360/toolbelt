🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/013-barramento-multi-vendor.md)

# ADR-013: Local Multi-Vendor Agent Bus (`tb bus`)

## Status

proposed

Implementation (v0.0.5): B0 and B1 delivered; B3 delivered after v0.0.8 (`tb bus run` with `exec`, `harness` and `notify` adapters, and `tb bus discover`); B2 delivered after v0.0.7 (lease, heartbeat, watchdog: [bus-lease.md](../bus-lease.md)); B4 delivered after v0.0.8 (router bus slots and harness router tasks: [bus-router.md](../bus-router.md)). Status stays `proposed` until the success metrics are measured.

## Context

[ADR-002](002-mcp-a2a-protocolos.md) chose MCP for agent ↔ tool and A2A over JSON-RPC 2.0 for agent ↔ agent, but did not define **how** agents from different vendors (Claude Code, Codex, Grok, ...) find each other, authenticate and exchange messages on the same machine. [ADR-009](009-router-e-workers.md) defines Ticket/Mandate/Board/Watchdog, and Phase 3 (delivered in v0.0.4) implements `tb router run` as a **foreground** scheduler that runs a single-use `router.agent_cmd` per ticket, in its own worktree and inside the sandbox. That model covers ephemeral workers within one repository, but does not cover:

1. **Persistent agents from other vendors**, with their own session and context, that the user wants to collaborate on a large task.
2. **a2a conversation** beyond "receive ticket, return result": questions, status, cross-review.
3. **Presence**: knowing who is available, in which role, with which capability and cost.

The core problem is that **a vendor agent is not a daemon**. Claude Code, Codex and desktop apps only act inside a turn; none of them listens on a socket. If the LLM itself runs the wait loop (`bus_recv` → empty → `bus_recv`):

- every empty poll is a remote model step over the whole context (cache read + output);
- clients cut the turn after N tool calls or on tool-call timeout;
- once the turn ends the agent stays idle and messages pile up with nobody to wake it.

Field investigation with a GUI-only desktop app (Grok Bot 1.1.0, Electron): no local API, no debug port, deep links only for OAuth callbacks; however, the app is an **MCP client**. In other words: some apps can call `tb` tools, but `tb` cannot wake them.

Inherited constraints (project invariants):

- `tb` is local: no bus command talks to the network.
- Sandbox is fail-closed: without Landlock it refuses to run.
- Expensive features are on demand (like `TB_GRAPH`), off by default.
- Secrets never appear in logs or prompts.

## Decision

Create the **bus** (`tb bus`): a transport, presence and wake-up layer between local agents of any vendor, on top of which the Phase 3 Router/Board can later operate. Components:

1. **Store**: SQLite in WAL mode (`.toolbelt/bus.db`), a durable message log, no daemon in v1.
2. **Two entry points into the same store**: CLI `tb bus ...` (any agent with a shell) and MCP tools `bus_*` in `tb mcp` (MCP clients).
3. **Two join modes**: direct activation (v1) and introspection (later).
4. **Per-agent harness** (`tb bus run`): a deterministic process that owns the wait loop. The LLM is invoked only when there is real work.

### 1. Protocol (envelope)

JSON-RPC 2.0, with A2A-inspired primitives:

```json
{
  "jsonrpc": "2.0",
  "method": "bus/message",
  "params": {
    "id": "01JD7Y3K2V...",
    "v": "bus/0.1",
    "channel": "feature-x",
    "from": "agent:claude-main",
    "to": "agent:grok-1 | role:worker | topic:board",
    "thread": "TKT-001",
    "type": "task.assign | task.status | task.result | query | reply | ack | urgent",
    "causation_id": "01JD7Y2...",
    "idempotency_key": "TKT-001/assign/1",
    "ttl": "30m",
    "payload": {}
  }
}
```

- `id` is a ULID; ordering is guaranteed **per thread**, not globally.
- **At-least-once** delivery; consumers deduplicate by `idempotency_key`.
- The payload carries **references** (path, commit SHA, ticket ID, `tb graph` symbol), not blobs. Each message stands on its own: there is no hidden context shared across vendors.
- Lowest common denominator across vendors: text + JSON with a versioned schema. No vendor-native format.

### 2. Join modes

#### Direct activation (v1)

The user creates the channel and explicitly authorizes each agent:

```bash
tb bus channel create feature-x --owner claude-main --ttl 2h
# channel: feature-x  owner: claude-main
# owner invite: 4MPA-2HQC (role supervisor, single-use, expires 10:10:00Z)
tb bus invite feature-x --role worker
# invite: K7Q2-9XMD (channel feature-x, role worker, single-use, expires 10:10:00Z)
```

The user asks each agent: "join channel `feature-x`, invite `K7Q2-9XMD`, role `worker`". The agent calls `tb bus join` (or `bus_join` via MCP). From then on the agents proceed on their own.

The invite guarantees:

- **explicit consent**: only agents the human authorized can join;
- **injection resistance**: an agent that read "join channel X" in a page or message cannot join without the code;
- **role fixed at join**: owner and workers are decided by the user, not negotiated between agents.

#### Introspection (later)

`tb bus discover` builds a **catalog** of the machine's agents: CLI binaries on PATH and whether they support headless/resume, clients with MCP config, running app processes. The catalog tells what is possible, but **it is neither consent nor a channel**: joining a channel still requires an invite. Introspection only closes the loop for headless agents, which the harness can invoke.

### 3. Handshake

1. `join(channel, invite, agent_card)`. The **Agent Card** declares vendor, model, tier, context window, capabilities, tools, sandbox, `max_concurrency`, wake mode (`harness` | `notify` | `manual`) and session handle.
2. The broker negotiates the protocol version (`v`) and returns `agent_id`, lease token, heartbeat interval and role.
3. A lease without heartbeat expires. The Watchdog returns the tickets of an agent that lost its lease to the Board.

The broker enforces role and mandate. Capability declared in the Agent Card **is not permission**: no agent escalates its own privileges.

### 4. Per-agent harness (`tb bus run`)

```bash
tb bus run claude --channel feature-x --invite K7Q2-9XMD --role worker
```

The harness is a Go process, one per agent, that owns the loop. It:

1. holds the lease and heartbeat on the agent's behalf;
2. waits for messages without an LLM (notify on the local store/socket) — **zero tokens while waiting**;
3. answers trivial protocol from state: `ack`, ticket status, "done yet?", dedupe;
4. coalesces messages and applies the **wake budget** (max wake-ups per window), preventing agents from waking each other in a loop;
5. when real work arrives, builds a **wake packet** and invokes the vendor in headless mode, for example:
   ```bash
   claude -p --resume <session> --output-format stream-json --mcp-config tb-bus.json
   ```
6. reads the structured result, publishes it on the bus and goes back to waiting.

`--resume` keeps context and cache affinity, like ADR-009's direct delivery. The agent **never polls**; during its turn it can use `bus_send`/`bus_reply` via MCP.

Harness layers:

| Layer | Role | Cost | Default |
|---|---|---|---|
| L0 deterministic | lease, heartbeat, ack, status, coalescer, wake budget | zero | always on |
| L1 RAG | wake packet = ticket + thread top-K via FTS5 ([ADR-003](003-sqlite-fts5-grafo.md)) + original message IDs | zero LLM | on |
| L2 local ML | triage of ambiguous messages, long-thread summaries | local CPU | off (flag, like `TB_GRAPH`) |

L2 rules:

- The local model **defers, never drops**. `task.assign` and `urgent` bypass triage.
- Summaries always carry message IDs so the agent can fetch the raw text.
- Local model output is untrusted data, like any message.
- L2 fits the items already on the roadmap: optional vector index and local inference components.

Wake adapters per agent type:

| Mode | Agent | How it wakes |
|---|---|---|
| `harness` | CLIs with headless/resume (Claude Code, Codex, ...) | harness invokes the vendor with a wake packet |
| `notify` | GUI-only apps that are MCP clients | harness only notifies the human ("Marin has 3 messages in feature-x"); the agent reads via `bus_recv` when the human opens a turn |
| `manual` | any agent with a shell, no harness | the human asks the agent to run `tb bus recv` |

For Claude Code without a harness, the `Stop` hook can check the queue before ending the turn, as a lightweight alternative.

### 5. Patterns

#### Task breakdown

- Reuses ADR-009 Ticket/Mandate with no format changes.
- Parallel tickets have **disjoint file scopes**; one worktree per writing agent.
- Acceptance via `tb stack --ticket`; a verdict other than PASS does not close the ticket.

#### Cross-vendor a2a communication

- Envelope + JSON with schema only; references instead of content.
- **Every message coming from another agent is untrusted data**: cross-agent prompt injection is a real threat. The secret redaction already applied to logs also applies here.

#### Hierarchy and ownership

- Roles: `router`, `supervisor`, `worker`, `reviewer`.
- **Single owner per ticket** via lease; only the owner calls `bus_complete`.
- Routing by capability + tier + cost. Escalation ladder: after N failures on a cheap tier, the ticket moves up a tier.
- **Cross-vendor review**: one vendor's implementation is reviewed by another, applying author ≠ verifier from [ADR-008](008-verificacao-adversarial.md).

### 6. Tools and commands (v1)

| CLI | MCP | Role |
|---|---|---|
| `tb bus channel create` | — | creates channel and invite (human only) |
| `tb bus join` | `bus_join` | handshake with Agent Card |
| `tb bus send` | `bus_send` | publishes a message |
| `tb bus recv` | `bus_recv` | reads the queue (short long-poll, below the client's tool-call timeout) |
| `tb bus claim` | `bus_claim` | takes a ticket (lease) |
| `tb bus complete` | `bus_complete` | closes a ticket with a result |
| `tb bus heartbeat` | `bus_heartbeat` | renews the lease (the harness does it automatically) |
| `tb bus run` | — | per-agent harness |
| `tb bus log` | — | channel audit trail |

`channel create` is not exposed over MCP: only the human creates channels and issues invites.

### 7. Security

- Fully local: store inside the repository, Unix socket with `0600` permissions if any, no network traffic from `tb`.
- Single-use, expiring invite; the lease token never appears in logs.
- The harness invokes agents with real permissions: fail-closed Landlock sandbox, ticket mandate enforced and **never** `bypassPermissions` by default.
- Audit log of every message, with secret redaction.

### 8. Relation to Phase 3

The current `tb router run` executes a single-use `router.agent_cmd` per ticket. The bus adds a second worker type: **an agent registered on the channel**. Planned integration: the router publishes `task.assign` to `topic:board`, registered harnesses `claim`, and the result comes back as `task.result` for `tb merge`. The bus does not replace the router; it is the transport between it and persistent agents from other vendors.

### Phasing

| Stage | Deliverable |
|---|---|
| B0 | This ADR + envelope and Agent Card spec |
| B1 | SQLite store, CLI + MCP, direct activation with invite, `join`/`send`/`recv`/`claim`/`complete`; Claude ↔ Claude and Claude ↔ `fake-agent` tests |
| B2 | Lease, heartbeat and Watchdog |
| B3 | `tb bus run` (L0 + L1) and `harness`/`notify` adapters; `tb bus discover` |
| B4 | Phase 3 Router/Board on top of the bus; optional L2 |

## Consequences

### Positive

- **Real cross-vendor interoperability**: any agent with a shell or MCP client participates, with no per-vendor SDK.
- **Zero tokens while waiting**: the harness, not the LLM, owns the loop.
- **Lean wake packet**: the agent wakes with the relevant top-K instead of the whole thread.
- **Explicit consent** and injection resistance through invites.
- **Cross-vendor review** strengthens adversarial verification.
- **Reuse** of existing SQLite/FTS5, sandbox, worktrees and Ticket/Mandate.

### Negative

- **GUI-only agents** cannot be woken by the harness; they depend on the human (`notify`/`manual`).
- **Per-vendor adapters** must track changes to headless CLI flags.
- **No daemon in v1**: cross-process notification relies on the store; higher latency than a resident broker.
- **At-least-once delivery** requires idempotency in every consumer.
- **L2 brings mis-triage risk**; mitigated by deferring instead of dropping and by types that bypass triage.

### Success metrics (hypotheses, not measured)

| Metric | Target |
|---|---|
| tokens spent idle-waiting | ≈ 0 with harness |
| input tokens per wake-up | lower than resending the whole thread |
| lost messages | 0 |
| wake-ups per useful message | ≤ 1 with coalescer |

They will be measured with `tb bus` + `tb bench compare` before moving the status to `accepted`.

## Alternatives Considered

### LLM runs its own polling loop

- **Description**: the agent calls `bus_recv` repeatedly inside its turn.
- **Why not**: cost per empty poll, turn cut after N tool calls and no wake-up after the turn ends.

### Local ML as the main solution

- **Description**: a small local model handles polling, triage and summaries.
- **Why not**: polling and trivial protocol need no model; deterministic rules are cheaper and predictable. ML stays as an optional layer (L2).

### Resident broker (daemon) from v1

- **Description**: `tb bus serve` with socket and push.
- **Why not now**: more state and lifecycle to manage; SQLite in WAL is enough to validate the protocol. Revisit at B3/B4.

### A2A over HTTP as-is

- **Description**: each agent exposes an A2A HTTP endpoint.
- **Why not**: vendor agents do not expose servers; it would require a per-agent proxy just like the harness, with HTTP instead of a local store. The envelope stays A2A-inspired to ease a future bridge.

### GUI input injection (tmux `send-keys`, screen automation)

- **Description**: type messages directly into the agent's window/terminal.
- **Why not**: fragile, no delivery confirmation, no audit trail and a large risk surface. Acceptable only as a manual experiment.

## References

- [Bus protocol](../bus-protocolo.md) — B0 spec (envelope, Agent Card, invite)
- [ADR-002](002-mcp-a2a-protocolos.md) — MCP + A2A
- [ADR-003](003-sqlite-fts5-grafo.md) — SQLite + FTS5
- [ADR-004](004-sandbox-isolamento.md) — sandbox
- [ADR-008](008-verificacao-adversarial.md) — adversarial verification
- [ADR-009](009-router-e-workers.md) — Router and Workers
- [ADR-011](011-grafo-sob-demanda.md) — on-demand features
- [Google A2A Protocol](https://github.com/google/A2A)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
