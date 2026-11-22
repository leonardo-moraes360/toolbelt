[🇺🇸 English](../../adr/003-sqlite-fts5-grafo.md) | 🇧🇷 Português

# ADR-003: SQLite + FTS5 como Grafo e Índice de Memória

## Status

accepted quanto à decisão de manter um índice derivado; **superada na prática**: o índice SQLite + FTS5 com tabelas `nodes`/`edges` nunca foi construído. O grafo é o `.toolbelt/graph.json` com ranking BM25 ([ADR-011](011-grafo-sob-demanda.md), [ADR-014](014-graphify-reimplementacao.md)), e o SQLite no `tb` é só o store do barramento local (`bus.db`, [ADR-013](013-barramento-multi-vendor.md)).

A seção "Sincronização Vault → SQLite" é substituída em parte na Fase A pela [ADR-016](016-vault-fase-a.md): o vault não tem índice derivado. A seção "Índice vetorial opcional" é substituída pela [spec do índice vetorial](../specs/vector-index.md) e pela [ADR-018](018-cgo-componentes-nativos.md): os vetores ficam ao lado do `graph.json`, não no sqlite-vec.

## Contexto

O toolbelt precisa de um sistema de armazenamento para:

1. **Grafo de conhecimento**: nós (funções, classes, arquivos, fatos) e arestas (calls, imports, references)
2. **Memória persistente**: fatos, decisões, sumários
3. **Busca rápida**: full-text search para queries semânticas
4. **Proveniência**: distinguir dados extraídos vs inferidos

Requisitos:
- Arquivo único (distribuição simples)
- Queries complexas (grafos, full-text, filtros)
- Performance adequada para projetos grandes (100k+ nós)
- Sem servidor (embedded)

## Decisão

**SQLite + FTS5** como índice derivado, com vault Markdown como fonte de verdade.

### Arquitetura

```
┌─────────────────────────────────────────────────────────────────┐
│                    Vault Markdown                               │
│                  (fonte de verdade)                             │
│  ─────────────────────────────────────────────────────────────  │
│  .tb/vault/                                                     │
│  ├── facts/                                                     │
│  ├── decisions/                                                 │
│  └── agents/                                                    │
└─────────────────────────────────────────────────────────────────┘
              │
              │  sync unidirecional
              │  (vault → SQLite)
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   SQLite + FTS5                                 │
│                  (índice derivado)                              │
│  ─────────────────────────────────────────────────────────────  │
│  .tb/index.db                                                   │
│  ├── nodes (nós do grafo)                                       │
│  ├── nodes_fts (full-text index)                                │
│  └── edges (arestas do grafo)                                   │
└─────────────────────────────────────────────────────────────────┘
```

### Schema Resumido

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

## Consequências

### Positivas

- **Arquivo único**: `.tb/index.db` fácil de backup/restaurar
- **Zero config**: sem servidor, sem containers
- **FTS5 embutido**: busca full-text sem Elasticsearch
- **Queries SQL**: flexibilidade para grafos (CTEs recursivas)
- **Performance**: otimizado para leitura, adequado para 100k+ nós
- **Transações ACID**: consistência garantida
- **Reconstruível**: pode ser deletado e reconstruído do vault

### Negativas

- **Single-writer**: lock de escrita limita concorrência de indexação
- **Sem índice vetorial nativo**: busca semântica requer extensão
- **Modelo relacional**: grafos são cidadãos de segunda classe
- **Escala limitada**: milhões de nós podem ficar lentos

## Alternativas Consideradas

### Neo4j / Graph Database

**Por que considerado**:
- Modelo nativo de grafos
- Cypher é expressivo para traversals
- Índices otimizados para grafos

**Por que não escolhido**:
- Requer servidor rodando
- Complexidade de deploy
- Overkill para nosso tamanho de dados
- Licenciamento (Community vs Enterprise)

### PostgreSQL

**Por que considerado**:
- Maduro, feature-rich
- Full-text search nativo
- JSON support

**Por que não escolhido**:
- Requer servidor
- Overhead para deploy local
- SQLite é suficiente para nosso caso

### Índice Vetorial (FAISS, Qdrant)

**Por que considerado**:
- Busca semântica via embeddings
- Similar search mais poderosa

**Por que não escolhido como default**:
- Complexidade adicional (gerar embeddings)
- Custo de inferência para indexação
- FTS5 é suficiente para MVP
- **Será adicionado opcionalmente na Fase 2**

### LevelDB / RocksDB

**Por que considerado**:
- Key-value rápido
- Embedded

**Por que não escolhido**:
- Sem queries SQL
- Teria que implementar índices manualmente
- Sem full-text search nativo

## Índice Vetorial Opcional (Fase 2)

Quando FTS5 não for suficiente, adicionaremos:

```sql
CREATE VIRTUAL TABLE nodes_vec USING vec0(
    embedding float[1536]
);
```

Opções de implementação:
- [sqlite-vec](https://github.com/asg017/sqlite-vec): extensão SQLite
- [Qdrant](https://qdrant.tech/): servidor separado, mais poderoso

## Sincronização Vault → SQLite

A direção é sempre vault → SQLite:

1. Watcher detecta mudança no vault
2. Parser extrai frontmatter, wikilinks, conteúdo
3. UPSERT no SQLite
4. FTS5 atualizado automaticamente

Se SQLite corromper ou divergir:

```bash
tb graph rebuild  # reconstrói tudo do vault + código
```

## Referências

- [SQLite FTS5](https://www.sqlite.org/fts5.html)
- [sqlite-vec](https://github.com/asg017/sqlite-vec)
- [Recursive CTEs para grafos](https://www.sqlite.org/lang_with.html)
