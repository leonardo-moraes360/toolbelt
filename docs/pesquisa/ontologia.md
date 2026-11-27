🇺🇸 English | [🇧🇷 Português](../pt-BR/pesquisa/ontologia.md)

# Ontology for agents: what applies to tb

> **Review date**: 2026-11-27
> **Reference**: the explainer "What Is Ontology in AI?" (a video transcript the maintainer pasted; author and publication not identified), which cites Palantir's Ontology, Databricks' Genie Ontology and Microsoft's [GraphRAG](https://arxiv.org/abs/2404.16130) (already in [fontes-primarias.md](fontes-primarias.md))
> **Target**: `tb graph` ([internal/graph/](../../internal/graph/), [ADR-011](../adr/011-grafo-sob-demanda.md), [ADR-014](../adr/014-graphify-reimplementacao.md)) and the vault ([internal/vault/](../../internal/vault/), [ADR-016](../adr/016-vault-fase-a.md))

## The argument

The text is a general explainer, with no data or primary source of its own. Its argument is a sequence of layers, each fixing a gap in the previous one:

1. Prompt stuffing, then RAG: the model gets the right facts, but as loose chunks.
2. Knowledge graphs: facts become things and relationships, but a graph says what is connected, not what it means.
3. MCP: the agent finds the tools ("the phone numbers"), but not how the organisation works ("the org chart").
4. Skills: the agent learns procedures, but over terms that each system names differently.
5. Ontology: a shared model of what exists, what it means, how it relates, which source is authoritative, what actions exist and who may take them.

Its example of the gap is one concept with many names: "client", "account", `customer_id`, "billing entity" and "organisation" are the same thing in five systems, and nothing tells the agent so.

## Where tb stands

- **The graph is structural.** Node types are `file`, `function`, `package`, `class`, `section`, `component`, `hook`, `route`, `playbook`, plus `note` and `tag` from the vault. Edge types are `imports`, `calls`, `defines`, `links`, `mentions`, `renders`, `template`, `mirror` and `tagged`. There is no node for a domain concept.
- **Docs already point at code, without types.** When a doc names an identifier in backticks, the indexer adds a `mentions` edge from the doc to that code node, with a confidence ([`docMentionEdges`](../../internal/graph/index_docs.go)). A note about "Order" that names `` `OrderDTO` `` is therefore already linked to it, but the edge does not say that the note *defines* the concept and the code *implements* it.
- **The vault knows synonyms, the graph does not.** Notes accept `aliases` in the frontmatter ([note.go](../../internal/vault/note.go)), and `tb vault search` matches on them. `tb graph search` (BM25 over code) does not use them, so a search for "customer" does not find `client_id`.
- **The action and permission layer exists elsewhere.** MCP tool annotations (from the [M8ven review](mcp-sinais-m8ven.md)) and the registry's capabilities already say what each tool does and what it may touch.

## Proposals, by value

### 1. Concept notes as typed graph nodes

Vault notes already have a `type` field, checked against `vault.types` (default `fact`, `decision`, `context`, `log`). Adding `concept` to that list, a note with `type: concept` becomes a `concept` node in the graph. Its backtick mentions of code become `represents` edges (concept → code) instead of plain `mentions`. `tb graph explain <concept>` then returns, in one call, the types, tables, routes and tests that make up that concept in the repository.

This reuses what exists: the vault's `type` field, the mention extractor and its confidence. The only new parts are one vault type, one graph node type and one edge type. It serves tb's "save tokens" goal directly, since the agent no longer has to grep for every spelling of a domain term.

**Priority**: medium · **Module**: graph · vault

### 2. Aliases in code search

When `TB_GRAPH=on` and the vault is indexed, `tb graph search` expands the query with the `aliases` of any concept note whose title or alias matches the query. The result says which alias matched, so the agent can tell an expanded hit from a literal one.

**Priority**: medium (same change as item 1) · **Module**: graph

### 3. Authoritative source per concept

An optional `authoritative:` field on a concept note names the file, table or system that wins when two sources disagree. This matches tb's existing rule that the vault is the source of truth and the index is derived, and can feed the egm drift checks later.

**Priority**: low · **Module**: vault · egm

## Not worth bringing

- **An ontology engine** (OWL, RDF, reasoners). tb is a local CLI; markdown notes with frontmatter plus the existing graph cover what an agent needs in a repository.
- **Actions and permissions per object type**, the Palantir side of the text. The MCP annotations and the registry's capabilities already do this for tb's own tools, and tb does not model business objects.
- **Databricks Genie and Palantir Ontology** as products: corporate data platforms, unrelated to a local CLI.

## Suggested order

Items 1 and 2 as one change after v1.0.0, then item 3 if concept notes get used.
