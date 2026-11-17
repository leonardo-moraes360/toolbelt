[🇧🇷 Português](../../adr/002-mcp-a2a-protocolos.md) | 🇺🇸 English

# ADR-002: MCP + A2A as Communication Protocols

## Status

accepted

## Context

toolbelt needs communication protocols for two scenarios:

1. **Agent ↔ Tools**: how the language model invokes toolbelt tools
2. **Agent ↔ Agent**: how agents in the hierarchy (planner, supervisor, worker) communicate

Requirements:
- Open standard, not proprietary
- Support for multiple transports (stdio, socket, HTTP)
- Simple serialization (JSON)
- Compatibility with the existing IDE and model ecosystem

## Decision

1. **MCP (Model Context Protocol)** for agent ↔ tools communication
2. **A2A (Agent-to-Agent)** for agent ↔ agent communication
3. **JSON-RPC 2.0** as the common base for both

### MCP for Tools

`tb serve` exposes an MCP server that:
- Lists available tools (`tb_run`, `tb_graph_search`, etc.)
- Receives tool calls from the model
- Returns structured results
- Works via stdio (default) or Unix socket

### A2A for Agents

Communication between hierarchy levels:
- Planner → Supervisor: task delegation
- Supervisor → Worker: work assignment
- Worker → Supervisor: results and status
- Any → Any: coordination queries

### JSON-RPC 2.0 as Base

Both protocols use JSON-RPC 2.0:

```json
{
  "jsonrpc": "2.0",
  "method": "tool/call",
  "params": {
    "name": "tb_run",
    "arguments": {"command": "go test"}
  },
  "id": 1
}
```

## Consequences

### Positive

- **Interoperability**: any IDE/model with MCP support works
- **Open standard**: we do not depend on a specific vendor
- **Extensibility**: easy to add new tools
- **Debugging**: readable JSON, easy to inspect
- **Flexible transports**: stdio for IDE, socket for internal processes

### Negative

- **Serialization overhead**: JSON is not the most efficient
- **A2A still emerging**: Google protocol, may evolve
- **Management complexity**: two protocols to maintain

## Alternatives Considered

### gRPC

**Why considered**:
- High performance (binary protobuf)
- Bidirectional streaming
- Code generation

**Why not chosen**:
- Additional complexity (protoc compiler)
- Not standard in the MCP ecosystem
- Overhead for small messages
- IDEs do not support it natively

### Proprietary Protocol

**Why considered**:
- Full control over design
- Optimized for our use case

**Why not chosen**:
- Reinventing the wheel
- No support from existing tools
- Barrier for contributors

### MCP Only (no A2A)

**Why considered**:
- Simplicity (one protocol only)

**Why not chosen**:
- MCP is optimized for tool calling, not agent coordination
- A2A has specific primitives: task assignment, status, delegation

## Transport Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Transports                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  stdio (MCP default)         Unix Socket           HTTP/2      │
│  ───────────────────         ───────────           ──────      │
│  IDE ↔ tb serve              tb ↔ processes        remote      │
│  Cursor, VSCode              internal, agents      (future)    │
│                              on the same machine                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       JSON-RPC 2.0                              │
│  ─────────────────────────────────────────────────────────────  │
│  {"jsonrpc":"2.0","method":"...","params":{...},"id":1}         │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│          MCP            │     │          A2A            │
│  ─────────────────────  │     │  ─────────────────────  │
│  tools/list             │     │  task/assign            │
│  tools/call             │     │  task/status            │
│  resources/read         │     │  task/complete          │
│  prompts/get            │     │  agent/query            │
└─────────────────────────┘     └─────────────────────────┘
```

## References

- [MCP Specification](https://spec.modelcontextprotocol.io/)
- [Google A2A Protocol](https://github.com/google/A2A)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
