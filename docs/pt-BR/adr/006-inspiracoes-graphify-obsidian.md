[🇺🇸 English](../../adr/006-inspiracoes-graphify-obsidian.md) | 🇧🇷 Português

# ADR-006: Inspirações — Graphify e Obsidian

## Status

accepted

A parte do vault (itens 1 e 2 da decisão, "TUI própria") é substituída em parte na Fase A pela [ADR-016](016-vault-fase-a.md).

## Contexto

O owner do projeto utiliza duas ferramentas que melhoram significativamente a experiência com agentes de código:

1. **Graphify**: mapeia projetos em grafos de conhecimento queryáveis
2. **Obsidian**: vault de Markdown com wikilinks e grafo visual

Ambas demonstram padrões valiosos que queremos incorporar ao toolbelt.

## O Que Aprendemos

### Graphify

| Característica | Valor |
|----------------|-------|
| Grafo local queryável | Agente consulta grafo ao invés de grep em arquivos |
| Parsing via AST | tree-sitter para extração estrutural |
| Proveniência | Distingue "extraído do código" vs "inferido por LLM" |
| Hooks para agentes | Direciona agente para grafo antes de ler arquivos |
| MCP server | Expõe callers, deps, impact como ferramentas MCP |

**Insight principal**: Agentes gastam muitos tokens lendo arquivos irrelevantes. O grafo reduz isso drasticamente.

### Obsidian

| Característica | Valor |
|----------------|-------|
| Vault Markdown | Arquivos `.md` são a fonte de verdade |
| Wikilinks | `[[outro-arquivo]]` cria grafo de relações |
| Backlinks | Navegação bidirecional |
| Frontmatter YAML | Metadados estruturados |
| Graph view | Visualização de conexões |
| Local-first | Humano pode editar diretamente |

**Insight principal**: Memória legível por humanos, editável no editor preferido, versionável com git.

## Decisão

### O Que Adotamos

1. **Vault Markdown como fonte de verdade**
   - Memória em `.tb/vault/` como arquivos Markdown
   - Wikilinks para relacionamentos
   - Frontmatter para metadados
   - Compatível com Obsidian

2. **SQLite como índice derivado (não fonte)**
   - Reconstruível a partir do vault
   - Sync unidirecional: vault → SQLite
   - Vault sempre vence em conflitos

3. **Indexação de código via AST**
   - tree-sitter para parsing
   - Extração de funções, classes, imports, calls
   - Nós de código no mesmo grafo que memória

4. **Proveniência em arestas**
   - `source: extracted` — do AST, alta confiança
   - `source: inferred` — deduzido por LLM, confiança variável
   - `source: manual` — inserido por humano

5. **Hooks graph-first**
   - Agentes devem consultar grafo antes de ler arquivos
   - Guidance em prompts de role
   - KPI de tokens economizados

6. **Queries de alto nível**
   - `tb graph callers` — quem chama esta função
   - `tb graph deps` — dependências de um módulo
   - `tb graph impact` — impacto de uma mudança

### O Que Fazemos Diferente

1. **FTS5 ao invés de embeddings por default**
   - Graphify pode usar embeddings para busca semântica
   - Nós usamos FTS5 como baseline (mais simples, sem custo de inferência)
   - Embeddings são opcionais na Fase 2

2. **Integração com sandbox**
   - Graphify é read-only
   - Nosso grafo informa o sandbox sobre arquivos protegidos

3. **Foco em agentes autônomos**
   - Graphify é principalmente para humanos com assistente
   - Nosso foco é hierarquia de agentes operando autonomamente

4. **TUI própria**
   - Obsidian requer app Electron
   - `tb md` oferece TUI terminal (Bubble Tea)
   - Mas é interoperável: pode abrir vault no Obsidian

## Questões em Aberto

### Integrar ou Reimplementar?

**Opção A: Integrar Graphify via MCP**

```yaml
pros:
  - Menos código para manter
  - Aproveita work existente
  - Comunidade ativa

cons:
  - Dependência externa
  - Pode divergir de nossas necessidades
  - Overhead de comunicação MCP
```

**Opção B: Reimplementar indexação**

```yaml
pros:
  - Controle total
  - Otimizado para nosso caso
  - Sem dependências externas

cons:
  - Mais trabalho inicial
  - Reinventar a roda
  - Manter parser de múltiplas linguagens
```

**Decidido no [ADR-014](014-graphify-reimplementacao.md)**: reimplementar. O `tb graph` já é implementação própria (go/ast + extratores regex, sem tree-sitter nem dependência do Graphify); `tb graph explain`/`path` foram as primeiras consultas do Graphify trazidas para o toolbelt.

### Sync Bidirecional com Obsidian

Atualmente: vault → SQLite (unidirecional)

Questão: Se usuário edita no Obsidian enquanto agente edita via `tb mem`, como resolver conflitos?

**Proposta**:
1. Watcher detecta mudanças do Obsidian
2. Atualiza SQLite imediatamente
3. Se agente tem mudança pendente, merge via git-style conflict markers
4. Humano resolve no próximo `tb md` ou Obsidian

### Extensão do Grafo para Outras Linguagens

tree-sitter suporta 100+ linguagens. Prioridade inicial:
1. Go
2. TypeScript/JavaScript
3. Python
4. Rust

Cada linguagem precisa de:
- Grammar tree-sitter
- Extrator de nós (funções, classes)
- Extrator de arestas (calls, imports)

## Consequências

### Positivas

- **Economia de tokens**: grafo evita leitura desnecessária
- **Memória legível**: humano pode ler/editar no Obsidian
- **Proveniência**: sabe-se o que foi extraído vs inferido
- **Integração suave**: vault é só Markdown, funciona em qualquer editor

### Negativas

- **Complexidade**: vault + SQLite + sync
- **Dependência de Obsidian**: formato deve permanecer compatível
- **tree-sitter por linguagem**: trabalho para suportar muitas linguagens

## Arquitetura Final

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
        │       │ fonte verdade│
        │       └──────┬───────┘
        │              │ sync
        │              ▼
        │       ┌──────────────┐      ┌──────────────┐
        │       │ index.db     │◄─────│ tree-sitter  │
        │       │ (SQLite)     │      │ code parser  │
        │       │ índice       │      └──────────────┘
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
│                         Agentes                                 │
│  1. Consulta grafo                                              │
│  2. Identifica arquivos relevantes                              │
│  3. Lê apenas o necessário                                      │
│  4. Implementa                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Referências

- [Graphify](https://github.com/Graphify-Labs/graphify)
- [Obsidian](https://obsidian.md/)
- [Obsidian Dataview](https://github.com/blacksmithgu/obsidian-dataview) — queries sobre vault
- [tree-sitter](https://tree-sitter.github.io/)
