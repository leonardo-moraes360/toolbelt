🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/README.md)

# Architecture Decision Records (ADRs)

This directory contains the ADRs (Architecture Decision Records) for the toolbelt project.

## Format

Each ADR follows this format:

1. **Title**: the decision in one sentence
2. **Status**: proposed | accepted | deprecated | superseded
3. **Context**: situation that motivated the decision
4. **Decision**: what was decided
5. **Consequences**: positive and negative impacts
6. **Alternatives considered**: other options evaluated

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [001](001-go-vs-rust.md) | Go as the primary language | accepted |
| [002](002-mcp-a2a-protocolos.md) | MCP + A2A as communication protocols | accepted |
| [003](003-sqlite-fts5-grafo.md) | SQLite + FTS5 as graph and memory index | accepted |
| [004](004-sandbox-isolamento.md) | Sandbox with bubblewrap + Landlock + seccomp | accepted |
| [005](005-anti-trapaca.md) | Anti-cheat strategy | accepted |
| [006](006-inspiracoes-graphify-obsidian.md) | Inspirations: Graphify and Obsidian | accepted |
| [007](007-inspiracao-elephant-goldfish.md) | Inspiration: Elephant-Goldfish Model | accepted |
| [008](008-verificacao-adversarial.md) | Adversarial verification (generator vs discriminator) | accepted |
| [009](009-router-e-workers.md) | Router and Workers pattern for delegation | proposed |
| [010](010-invariantes-fora-compactacao.md) | Invariants pinned outside compaction | proposed |
| [011](011-grafo-sob-demanda.md) | On-demand code graph with A/B | accepted |
| [012](../decisions/012-cli-framework-cobra.md) | CLI framework: cobra | accepted |
| [013](013-barramento-multi-vendor.md) | Local multi-vendor agent bus (`tb bus`) | proposed (B0/B1 and part of B3 implemented) |
| [014](014-graphify-reimplementacao.md) | Graphify: reimplement, don't integrate | accepted |
| [015](015-egm-nativo.md) | Native reimplementation of the Elephant-Goldfish Model (`tb egm`) | proposed |

## How to Propose an ADR

1. Create a file `NNN-titulo-em-kebab-case.md`
2. Use the template below
3. Open a PR with status `proposed`
4. After discussion and approval, change to `accepted`

## Template

```markdown
# ADR-NNN: Decision Title

## Status

proposed | accepted | deprecated | superseded by [ADR-XXX](XXX-outro.md)

## Context

Description of the situation that motivated the decision.

## Decision

What was decided.

## Consequences

### Positive
- ...

### Negative
- ...

## Alternatives Considered

### Alternative A
- Description
- Why it was not chosen

### Alternative B
- Description
- Why it was not chosen
```
