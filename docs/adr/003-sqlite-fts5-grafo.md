🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/003-sqlite-fts5-grafo.md)

# ADR-003: SQLite + FTS5 as Graph and Memory Index

## Status

accepted

## Context

toolbelt needs a storage system for:

1. **Knowledge graph**: nodes (functions, classes, files, facts) and edges (calls, imports, references)
2. **Persistent memory**: facts, decisions, summaries
3. **Fast search**: full-text search for semantic queries
4. **Provenance**: distinguish extracted vs inferred data

Requirements:
- Single file (simple distribution)
- Complex queries (graphs, full-text, filters)
- Adequate performance for large projects (100k+ nodes)
- No server (embedded)

## Decision

**SQLite + FTS5** as a derived index, with a Markdown vault as the source of truth.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Markdown Vault                               │
│                  (source of truth)                              │
│  ─────────────────────────────────────────────────────────────  │
│  .tb/vault/                                                     │
│  ├── facts/                                                     │
│  ├── decisions/                                                 │
│  └── agents/                                                    │
└─────────────────────────────────────────────────────────────────┘
              │
              │  unidirectional sync
              │  (vault → SQLite)
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   SQLite + FTS5                                 │
│                  (derived index)                                │
│  ─────────────────────────────────────────────────────────────  │
│  .tb/index.db                                                   │
│  ├── nodes (graph nodes)                                        │
│  ├── nodes_fts (full-text index)                                │
│  └── edges (graph edges)                                        │
└─────────────────────────────────────────────────────────────────┘
```

### Summary Schema

```sql
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    path TEXT,
    title TEXT,
    content TEXT,
    source TEXT NOT NULL,      -- 'vault', 'code', 'inferred'
    confidence REAL DEFAULT 1.0,
    metadata JSON
);

CREATE VIRTUAL TABLE nodes_fts USING fts5(title, content);

CREATE TABLE edges (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    type TEXT NOT NULL,        -- 'calls', 'imports', 'links_to'
    source TEXT NOT NULL,      -- 'extracted', 'inferred', 'manual'
    confidence REAL DEFAULT 1.0
);
```

## Consequences

### Positive

- **Single file**: `.tb/index.db` easy to backup/restore
- **Zero config**: no server, no containers
- **Built-in FTS5**: full-text search without Elasticsearch
- **SQL queries**: flexibility for graphs (recursive CTEs)
- **Performance**: optimized for reads, adequate for 100k+ nodes
- **ACID transactions**: guaranteed consistency
- **Rebuildable**: can be deleted and rebuilt from the vault

### Negative

- **Single-writer**: write lock limits indexing concurrency
- **No native vector index**: semantic search requires an extension
- **Relational model**: graphs are second-class citizens
- **Limited scale**: millions of nodes may become slow

## Alternatives Considered

### Neo4j / Graph Database

**Why considered**:
- Native graph model
- Cypher is expressive for traversals
- Indexes optimized for graphs

**Why not chosen**:
- Requires a running server
- Deploy complexity
- Overkill for our data size
- Licensing (Community vs Enterprise)

### PostgreSQL

**Why considered**:
- Mature, feature-rich
- Native full-text search
- JSON support

**Why not chosen**:
- Requires a server
- Overhead for local deploy
- SQLite is sufficient for our case

### Vector Index (FAISS, Qdrant)

**Why considered**:
- Semantic search via embeddings
- More powerful similar search

**Why not chosen as default**:
- Additional complexity (generating embeddings)
- Inference cost for indexing
- FTS5 is sufficient for MVP
- **Will be added optionally in Phase 2**

### LevelDB / RocksDB

**Why considered**:
- Fast key-value
- Embedded

**Why not chosen**:
- No SQL queries
- Would have to implement indexes manually
- No native full-text search

## Optional Vector Index (Phase 2)

When FTS5 is not enough, we will add:

```sql
CREATE VIRTUAL TABLE nodes_vec USING vec0(
    embedding float[1536]
);
```

Implementation options:
- [sqlite-vec](https://github.com/asg017/sqlite-vec): SQLite extension
- [Qdrant](https://qdrant.tech/): separate server, more powerful

## Vault → SQLite Sync

The direction is always vault → SQLite:

1. Watcher detects a vault change
2. Parser extracts frontmatter, wikilinks, content
3. UPSERT into SQLite
4. FTS5 updated automatically

If SQLite corrupts or diverges:

```bash
tb graph rebuild  # rebuilds everything from vault + code
```

## References

- [SQLite FTS5](https://www.sqlite.org/fts5.html)
- [sqlite-vec](https://github.com/asg017/sqlite-vec)
- [Recursive CTEs for graphs](https://www.sqlite.org/lang_with.html)
