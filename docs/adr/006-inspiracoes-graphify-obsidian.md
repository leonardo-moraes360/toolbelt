🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/006-inspiracoes-graphify-obsidian.md)

# ADR-006: Inspirations — Graphify and Obsidian

## Status

accepted

The vault part (items 1 and 2 of the decision, "own TUI") is partially superseded for Phase A by [ADR-016](016-vault-fase-a.md).

## Context

The project owner uses two tools that significantly improve the experience with coding agents:

1. **Graphify**: maps projects into queryable knowledge graphs
2. **Obsidian**: Markdown vault with wikilinks and a visual graph

Both demonstrate valuable patterns we want to incorporate into toolbelt.

## What We Learned

### Graphify

| Characteristic | Value |
|----------------|-------|
| Queryable local graph | Agent queries the graph instead of grepping files |
| AST parsing | tree-sitter for structural extraction |
| Provenance | Distinguishes "extracted from code" vs "inferred by LLM" |
| Agent hooks | Directs agent to the graph before reading files |
| MCP server | Exposes callers, deps, impact as MCP tools |

**Main insight**: Agents spend many tokens reading irrelevant files. The graph reduces that drastically.

### Obsidian

| Characteristic | Value |
|----------------|-------|
| Markdown vault | `.md` files are the source of truth |
| Wikilinks | `[[outro-arquivo]]` creates a relation graph |
| Backlinks | Bidirectional navigation |
| YAML frontmatter | Structured metadata |
| Graph view | Visualization of connections |
| Local-first | Human can edit directly |

**Main insight**: Human-readable memory, editable in the preferred editor, versionable with git.

## Decision

### What We Adopt

1. **Markdown vault as source of truth**
   - Memory in `.tb/vault/` as Markdown files
   - Wikilinks for relationships
   - Frontmatter for metadata
   - Compatible with Obsidian

2. **SQLite as derived index (not source)**
   - Rebuildable from the vault
   - Unidirectional sync: vault → SQLite
   - Vault always wins in conflicts

3. **Code indexing via AST**
   - tree-sitter for parsing
   - Extraction of functions, classes, imports, calls
   - Code nodes in the same graph as memory

4. **Provenance on edges**
   - `source: extracted` — from AST, high confidence
   - `source: inferred` — deduced by LLM, variable confidence
   - `source: manual` — inserted by a human

5. **Graph-first hooks**
   - Agents should query the graph before reading files
   - Guidance in role prompts
   - KPI for tokens saved

6. **High-level queries**
   - `tb graph callers` — who calls this function
   - `tb graph deps` — dependencies of a module
   - `tb graph impact` — impact of a change

### What We Do Differently

1. **FTS5 instead of embeddings by default**
   - Graphify may use embeddings for semantic search
   - We use FTS5 as baseline (simpler, no inference cost)
   - Embeddings are optional in Phase 2

2. **Sandbox integration**
   - Graphify is read-only
   - Our graph informs the sandbox about protected files

3. **Focus on autonomous agents**
   - Graphify is mainly for humans with an assistant
   - Our focus is a hierarchy of agents operating autonomously

4. **Own TUI**
   - Obsidian requires an Electron app
   - `tb md` offers a terminal TUI (Bubble Tea)
   - But it is interoperable: the vault can be opened in Obsidian

## Open Questions

### Integrate or Reimplement?

**Option A: Integrate Graphify via MCP**

```yaml
pros:
  - Less code to maintain
  - Leverages existing work
  - Active community

cons:
  - External dependency
  - May diverge from our needs
  - MCP communication overhead
```

**Option B: Reimplement indexing**

```yaml
pros:
  - Full control
  - Optimized for our case
  - No external dependencies

cons:
  - More initial work
  - Reinventing the wheel
  - Maintaining parsers for multiple languages
```

**Decided in [ADR-014](014-graphify-reimplementacao.md)**: reimplement. `tb graph` is already an in-house implementation (go/ast + regex extractors, no tree-sitter and no Graphify dependency); `tb graph explain`/`path` are the first Graphify queries brought into toolbelt.

### Bidirectional Sync with Obsidian

Currently: vault → SQLite (unidirectional)

Question: If a user edits in Obsidian while an agent edits via `tb mem`, how do we resolve conflicts?

**Proposal**:
1. Watcher detects Obsidian changes
2. Updates SQLite immediately
3. If the agent has a pending change, merge via git-style conflict markers
4. Human resolves on the next `tb md` or Obsidian

### Extending the Graph to Other Languages

tree-sitter supports 100+ languages. Initial priority:
1. Go
2. TypeScript/JavaScript
3. Python
4. Rust

Each language needs:
- tree-sitter grammar
- Node extractor (functions, classes)
- Edge extractor (calls, imports)

## Consequences

### Positive

- **Token savings**: graph avoids unnecessary reading
- **Readable memory**: human can read/edit in Obsidian
- **Provenance**: know what was extracted vs inferred
- **Smooth integration**: vault is just Markdown, works in any editor

### Negative

- **Complexity**: vault + SQLite + sync
- **Obsidian dependency**: format must remain compatible
- **tree-sitter per language**: work to support many languages

## Final Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Interface                                │
│  ┌─────────┐  ┌─────────────┐  ┌─────────────────────────────┐  │
│  │ tb CLI  │  │ tb md (TUI) │  │ Obsidian                    │  │
│  └────┬────┘  └──────┬──────┘  └────────────┬────────────────┘  │
└───────┼──────────────┼─────────────────────┼────────────────────┘
        │              │                      │
        │              ▼                      │
        │       ┌──────────────┐              │
        │       │ .tb/vault/   │◄─────────────┘
        │       │ (Markdown)   │
        │       │ source of    │
        │       │ truth        │
        │       └──────┬───────┘
        │              │ sync
        │              ▼
        │       ┌──────────────┐      ┌──────────────┐
        │       │ index.db     │◄─────│ tree-sitter  │
        │       │ (SQLite)     │      │ code parser  │
        │       │ index        │      └──────────────┘
        │       └──────┬───────┘
        │              │
        ▼              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       tb graph                                  │
│  search | callers | deps | impact | index | rebuild            │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                       MCP Server                                │
│  tb_graph_search | tb_graph_callers | tb_graph_deps | impact   │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Agents                                  │
│  1. Query graph                                                 │
│  2. Identify relevant files                                     │
│  3. Read only what is needed                                    │
│  4. Implement                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## References

- [Graphify](https://github.com/Graphify-Labs/graphify)
- [Obsidian](https://obsidian.md/)
- [Obsidian Dataview](https://github.com/blacksmithgu/obsidian-dataview) — vault queries
- [tree-sitter](https://tree-sitter.github.io/)
