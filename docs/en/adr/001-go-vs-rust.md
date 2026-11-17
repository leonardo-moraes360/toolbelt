[🇧🇷 Português](../../adr/001-go-vs-rust.md) | 🇺🇸 English

# ADR-001: Go as the Primary Language

## Status

accepted

## Context

toolbelt needs a language for its main binary that meets these requirements:

1. **Single static binary**: simple distribution without runtime dependencies
2. **Native concurrency**: orchestrate multiple agents, processes, and I/O in parallel
3. **Fast iteration**: quick compilation for short development cycles
4. **MCP ecosystem**: official support for the MCP protocol
5. **Adequate performance**: not the bottleneck (inference is on GPU/API)

## Decision

**Go** will be the primary language of toolbelt.

### Rationale

1. **Static binary**: `CGO_ENABLED=0 go build` produces a binary with no dependencies
2. **Goroutines and channels**: lightweight concurrency model, ideal for orchestrating agents
3. **Context**: native cancellation and timeout propagation
4. **Compilation**: seconds, not minutes
5. **Official MCP SDK**: [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)
6. **CLI ecosystem**: cobra, bubbletea, lipgloss for modern CLIs and TUIs
7. **Familiarity**: mainstream language, easy to contribute to

## Consequences

### Positive

- Simple deploy (one file)
- Good I/O and concurrency performance
- Active community, many libraries
- Gentle learning curve
- Mature tooling (go vet, staticcheck, golangci-lint)

### Negative

- No advanced generics (may complicate abstractions)
- Garbage collector can cause latency in edge scenarios
- Less expressive than Rust for type invariants
- Verbose error handling

## Alternatives Considered

### Rust

**Why considered**:
- Maximum performance
- Compile-time memory safety guarantees
- Emerging LLM ecosystem (mistral.rs, candle)

**Why not chosen**:
- Inference speed comes from GPU kernels (llama.cpp, vLLM), not the client
- Rust only pays off if we embed local inference, which is not MVP
- Slow compilation hurts iteration
- Steep learning curve for contributors
- Less mature MCP SDK in Rust

**Future decision**: if we add local inference via mistral.rs/candle, specific components may be written in Rust and called via FFI or as a separate process.

### Python

**Why considered**:
- Dominant ML/AI ecosystem
- Fast prototyping

**Why not chosen**:
- Complex distribution (virtualenv, dependencies)
- Inferior concurrency performance (GIL)
- Does not easily produce a static binary

### TypeScript/Node

**Why considered**:
- Official MCP SDK
- Strong web ecosystem

**Why not chosen**:
- Heavy runtime (Node.js)
- Distribution requires a bundler or Node installed
- Less predictable concurrency

## References

- [Go SDK MCP](https://github.com/modelcontextprotocol/go-sdk)
- [Internal language discussion] — not formally documented
