🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/014-graphify-reimplementacao.md)

# ADR-014: Graphify — Reimplement, Don't Integrate

## Status

accepted

[ADR-018](018-cgo-componentes-nativos.md) (proposed) enables cgo for native components; tree-sitter stays deferred, but cgo no longer rules it out.

## Context

[ADR-006](006-inspiracoes-graphify-obsidian.md) left open the question
"integrate Graphify via MCP or reimplement indexing?", with a provisional
decision to try integration first.

In practice the project went the other way: `tb graph` (Phase 2,
[ADR-011](011-grafo-sob-demanda.md)) is an in-house reimplementation in
`internal/graph`:

- Go via `go/ast`; TS/JS, Python, Rust and frameworks (React, Vue, Svelte,
  Angular, Astro, MDX) via conservative regex extractors; Markdown docs with
  wikilinks.
- Provenance on nodes and edges (`source`, `confidence`), with the
  `MinCallsConfidence = 0.6` filter in `callers`/`impact`.
- Local JSON index (`.toolbelt/graph.json`) with a freshness fingerprint.
- CLI ↔ MCP parity (`tb graph *` ↔ `tb_graph_*`).

Not a single line depends on Graphify. The field study ([SINTESE](../campo/SINTESE.md))
rated "deep Graphify integration" as **unsupported**: the observed bottleneck
was extractor quality (empty callers/impact outside Go/JS), not the lack of an
external backend.

## Decision

**Reimplement.** toolbelt does not integrate Graphify (neither as a Go
dependency nor as an external MCP server). Graphify remains a product
reference: we copy ideas, not code.

Deciding criteria:

1. **Local-first invariant** — `tb` is 100% local with no network. Graphify
   includes LLM steps (`INFERRED` edges) and a Python pipeline; integrating it
   would add a second runtime and optional network dependencies that are hard
   to fence in the sandbox.
2. **Single Go binary** — no new `go.mod` dependency, no tree-sitter via cgo
   (keeps reproducible release + SBOM + cosign simple).
3. **Provenance already compatible** — `extracted`/`inferred`/`manual` +
   confidence covers Graphify's `EXTRACTED/INFERRED/AMBIGUOUS` model.
4. **On demand** — the graph stays off by default (ADR-011); a heavy external
   backend contradicts the cost measured in A/B.

### First delivery under this decision

Two Graphify queries, reimplemented deterministically (no LLM):

| Graphify | toolbelt | What it does |
|----------|----------|--------------|
| `explain "<node>"` | `tb graph explain <node>` / `tb_graph_explain` | Node + incoming/outgoing edges with `source`/`confidence`; counts calls with no node in the index (`unresolved_out`) |
| `path "A" "B"` | `tb graph path <a> <b>` / `tb_graph_path` | Shortest path (BFS) between two nodes, walking edges both ways; each step says `forward`/`backward` |

Shared rules: `calls` edges with confidence &lt; 0.6 are ignored (as in
`callers`/`impact`); edges whose target is not a node are never walked; node
resolution goes exact ID → exact title → function suffix → file/package; ties
broken by ID (stable output).

Also in v0.0.5: `tb graph search` / `tb_graph_search` now ranks in pure Go
(exact title → substring → terms only; BM25 over title/path/package within
each tier, with camelCase/`snake_case` splitting).

### Roadmap (out of this delivery)

Ordered by value measured in the field, one PR each:

1. **Receiver-based call resolution in Go** — as of 2026-11-17, 9,580 of 17,302
   edges (~55%) in this repo's index point to targets with no node (`func:s.Method`, stdlib,
   builtins). Biggest win for `callers`/`impact`/`path`.
2. **God nodes / hubs** in `tb graph stats` — top-N by degree, like Graphify's
   report.
3. **Communities** (deterministic label propagation) — group modules to
   orient the agent in large repositories.
4. **Markdown report in the vault** (`GRAPH_REPORT.md`-like), Obsidian
   compatible (ADR-006).

LLM-`inferred` edges stay out until there is a local model (Phase 3, optional
Rust components).

## Consequences

### Positive

- ADR-006's open question is closed; docs now describe the real code (go/ast +
  regex, not tree-sitter).
- Zero new external dependencies; release stays a single binary.
- Agents get `explain`/`path`, useful to locate before reading files.

### Negative

- Maintaining per-language extractors is on us (regex has a precision ceiling).
- Without LLM-inferred edges, implicit relations (DI, reflection) are missing.

## Alternatives Considered

- **Integrate Graphify via MCP** — rejected: second runtime (Python), LLM/network
  steps against the local-first invariant, and the field study showed no gain
  to justify it.
- **tree-sitter via cgo** — deferred: better precision, but cgo complicates
  cross-compilation and reproducibility; revisit if regex becomes a real ceiling.
