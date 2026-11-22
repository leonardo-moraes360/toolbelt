🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/020-agent-hierarchy-a2a.md)

# ADR-020: Agent hierarchy as delegation trees on the local bus; A2A stays an envelope shape, with no network in `tb`

## Status

proposed (2026-11-22). Partially supersedes [ADR-002](002-mcp-a2a-protocolos.md): the "A2A for Agents" section, the `tb serve` Unix socket and the HTTP/2 transport. Confirms the rejection of "A2A over HTTP as-is" in [ADR-013](013-barramento-multi-vendor.md) and the drop of `tb md` in [ADR-016](016-vault-fase-a.md) §5. Spec: [`docs/specs/agent-hierarchy.md`](../specs/agent-hierarchy.md).

## Context

The README roadmap has carried one line since Phase 2: "Agent hierarchy / A2A / `tb md` (later)". It bundles three things that are in different states.

**A2A.** ADR-002 (accepted) chose A2A for agent ↔ agent traffic, with Planner → Supervisor → Worker delegation over a `tb serve` Unix socket and HTTP/2 "for remote" later. None of that was built. What exists is the `tb bus` ([ADR-013](013-barramento-multi-vendor.md), [bus-protocolo.md](../bus-protocolo.md)): a JSON-RPC 2.0 envelope (`bus/0.1`) with A2A-inspired primitives over a SQLite store. There is no socket, no HTTP server and no A2A client anywhere in the code. ADR-013 rejected "A2A over HTTP as-is" because vendor agents do not expose servers, and kept the envelope A2A-shaped "to ease a future bridge".

**Hierarchy.** What delegation exists today:

- `tb router run` ([ADR-009](009-router-e-workers.md)) is a deterministic scheduler: one ticket per worker, local workers in the sandbox or bus workers through B4 slots ([bus-router.md](../bus-router.md)).
- Bus roles (`router`, `supervisor`, `worker`, `reviewer`) are fixed by the invite the human issues. Only `router` and `supervisor` may send `task.assign`; only the claim owner may `complete`; leases, heartbeats and the watchdog recover lost owners.
- A supervisor agent can already claim a thread and send `task.assign` for sub-work. But the bus does not know that the sub-thread belongs to the first one. `thread` and `causation_id` are free-form, so no command can show the tree, bound its fan-out, or tell a supervisor that a child is still open when it completes its own thread.

`docs/arquitetura.md` §8 and `docs/cli.md` (`tb agent spawn --parent`, `approve`, `reject`) still describe a planner/supervisor/worker tree with per-level token budgets as if it were the plan.

**`tb md`.** ADR-016 dropped the TUI from Phase A and said the README and `docs/arquitetura.md` must stop describing it. Both still did, as did `docs/cli.md` and `docs/identidade-visual.md`.

Constraints (project invariants, `.toolbelt/invariants.yaml`):

- `tb` is 100% local and opens no network; network happens only in a child process the human configured.
- Bus waiting uses no LLM.
- The bus MCP never creates channels or invites; identity is bound to the connection and the broker sets `from`.
- The sandbox is fail-closed; the harness never uses `bypassPermissions`.
- Implementer and evaluator are separate.

## Decision

### 1. A2A: the local `bus/0.1` envelope is the answer; `tb` ships no A2A transport

- `tb` contains no A2A server, client, HTTP listener or socket, and no `a2a` adapter for `tb bus run`. The `bus/0.1` envelope stays A2A-shaped. The table below is a **non-normative** guide for anyone who maps the two; it adds no protocol.

  | A2A concept | `bus/0.1` |
  |---|---|
  | Agent Card | Agent Card on `join` (informational; the role comes from the invite) |
  | Task | a thread with `task.assign` and a claim |
  | Task states (working, completed, canceled, ...) | claimed, completed (`task.result`), revoked |
  | Message | `query` / `reply` / `task.status` |
  | Artifact | references in the payload (paths, SHAs, ticket IDs), never blobs |

- **A bridge is an ordinary bus agent.** If a human wants a remote A2A agent to take part, the bridge is a separate program that the human writes or installs, runs outside `tb` and admits with its own invite and role, like any other agent. It holds the network; `tb` does not. It is untrusted like any agent, and its role caps what it can do: a `worker` bridge cannot assign work, so a remote party cannot push tasks into the channel unless the human made the bridge a `supervisor`.
- `tb bus run`'s `exec` adapter already lets a human-written script call out on each wake (a webhook, for example). That is the extent of network support around the bus.

Why not a built-in bridge or adapter: harness adapters are short-lived wake calls, while an A2A server listens all the time. Putting one under `tb` would make `tb` the parent of a network listener. Inbound A2A would also let any remote client that can reach it put work on the local machine through one invite, and the invite model gives consent per agent.

### 2. Hierarchy: router + bus roles is the hierarchy; Phase A adds delegation trees on the bus

Delegation is already one level deep and works: the router or a supervisor assigns, workers claim and complete. There is no planner role and no new process. The human (or a supervisor agent the human admitted) plans, and the router schedules deterministically.

Phase A (one PR, [spec](../specs/agent-hierarchy.md)) makes the second level real on the bus, inside the existing broker rules:

1. **`parent` on `task.assign`.** An optional envelope field naming the parent thread. The broker accepts it only when the sender is the `router` or holds the live claim on the parent thread, and only on the first `task.assign` of a new thread. A thread's parent never changes, so cycles cannot form.
2. **Structural limits instead of a token budget.** At most 3 levels below a root and at most 8 open children per parent, both checked by the broker. `tb` cannot meter a vendor agent's tokens, so a token budget it passes down would be a number nobody enforces. Depth and fan-out are what the broker can enforce.
3. **`tb bus tree <channel>`.** A read-only, human-only CLI view (like `tb bus log`) of each tree: thread, parent, state (assigned, claimed, completed, revoked, released), owner and epoch. It shows no payload.
4. **Open children are reported, not blocking.** `complete` on a parent with open children succeeds. The result says so, and `tb bus tree` marks it.
5. **CLI and MCP.** `tb bus send --parent T` and a `parent` argument on `tb_bus_send`. No new MCP tool creates channels or invites, and `tree` is CLI only.

Store schema goes from v3 to v4 (one new table, one-way upgrade, same rules as v1 → v2 and v2 → v3).

### 3. `tb md` and `tb agent` are dropped

- `tb md` stays dropped (ADR-016 §5): any editor, Obsidian included, edits the vault.
- `tb agent` (`spawn --parent`, `list`, `approve`, `reject`, `budget`) is dropped too. Its parts exist under other names: admitting an agent is `tb bus invite` + `tb bus join`; running one is `tb bus run` or `tb router run`; health is `tb watchdog` and `tb bus watchdog`. Approve/reject assumed a pending-action queue that does not exist. The gate is the fail-closed sandbox and the mandate, not a prompt.
- The README, `docs/arquitetura.md`, `docs/cli.md` and `docs/identidade-visual.md` stop presenting either command as planned. Their history stays in ADR-006, ADR-016 and this ADR.

### 4. Roadmap

The README line "Agent hierarchy / A2A / `tb md` (later)" is replaced by one open item, the Phase A of this ADR. A2A and `tb md` are recorded as decided.

## Consequences

### Positive

- The roadmap says what remains, and the docs stop describing a socket, an HTTP transport and two commands that will not be built.
- The no-network invariant stays absolute for `tb`. Every network hop is in a program the human chose.
- Sub-delegation becomes visible and bounded without a new process, role or daemon: it reuses claims, leases, epochs and revokes.

### Negative

- No out-of-the-box A2A interop. Someone who wants it writes a bridge, and the bus protocol (not A2A) is the contract it follows.
- A schema bump: every `tb` that serves a channel has to be upgraded together, as in B2 and B4.
- Depth and fan-out limits are coarse. They bound runaway delegation; they do not bound cost.
- A parent can be completed while a child still runs. Phase A only reports it.

## Alternatives considered

- **Built-in A2A bridge (`tb bus run` adapter `a2a`, or `tb serve --a2a`).** Rejected: `tb` would own a listener, and inbound tasks from the network would bypass per-agent consent. The external-agent route gives the same interop with the network outside `tb`.
- **A2A as the bus's wire format.** Rejected: A2A assumes HTTP endpoints and an evolving spec; vendor agents do not expose servers (ADR-013). The envelope stays A2A-shaped and versioned by `bus/0.1`.
- **Declare the hierarchy done (router + bus roles) and add nothing.** Simplest. Rejected for now because sub-delegation already happens through supervisors, and without a parent link nobody can see or bound it. This stays the fallback if Phase A is not wanted.
- **Token or time budget inheritance.** Deferred: `tb` sees neither vendor token use nor real work time, only wake counts, `ttl` and claim timestamps. A budget it cannot enforce would be documentation, not a control.
- **A planner role and `tb agent spawn`.** Rejected: planning is the human's or an admitted supervisor's job, and spawning is `tb bus invite` + `tb bus run`. A spawning command would let one agent create others, which the invite model forbids.
- **Use `causation_id` as the parent.** Rejected: `causation_id` points to a message, is unchecked and already has a meaning. A parent link that grants nothing unless it is checked has to be a separate, broker-validated field.

## References

- [Spec: delegation trees on the bus](../specs/agent-hierarchy.md)
- [ADR-002](002-mcp-a2a-protocolos.md), [ADR-009](009-router-e-workers.md), [ADR-013](013-barramento-multi-vendor.md), [ADR-016](016-vault-fase-a.md)
- [Bus protocol](../bus-protocolo.md), [bus lease](../bus-lease.md), [bus harness](../bus-harness.md), [router on the bus](../bus-router.md)
- [Google A2A Protocol](https://github.com/google/A2A)
