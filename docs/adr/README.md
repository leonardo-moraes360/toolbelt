# Registros de Decisão Arquitetural (ADRs)

Este diretório contém os ADRs (Architecture Decision Records) do projeto toolbelt.

## Formato

Cada ADR segue o formato:

1. **Título**: decisão em uma frase
2. **Status**: proposed | accepted | deprecated | superseded
3. **Contexto**: situação que motivou a decisão
4. **Decisão**: o que foi decidido
5. **Consequências**: impactos positivos e negativos
6. **Alternativas consideradas**: outras opções avaliadas

## Índice

| ADR | Título | Status |
|-----|--------|--------|
| [001](001-go-vs-rust.md) | Go como linguagem principal | accepted |
| [002](002-mcp-a2a-protocolos.md) | MCP + A2A como protocolos de comunicação | accepted |
| [003](003-sqlite-fts5-grafo.md) | SQLite + FTS5 como grafo e índice de memória | accepted |
| [004](004-sandbox-isolamento.md) | Sandbox com bubblewrap + Landlock + seccomp | accepted |
| [005](005-anti-trapaca.md) | Estratégia anti-trapaça | accepted |
| [006](006-inspiracoes-graphify-obsidian.md) | Inspirações: Graphify e Obsidian | accepted |
| [007](007-inspiracao-elephant-goldfish.md) | Inspiração: Elephant-Goldfish Model | accepted |
| [008](008-verificacao-adversarial.md) | Verificação adversarial (gerador vs discriminador) | accepted |

## Como Propor um ADR

1. Crie um arquivo `NNN-titulo-em-kebab-case.md`
2. Use o template abaixo
3. Abra um PR com status `proposed`
4. Após discussão e aprovação, mude para `accepted`

## Template

```markdown
# ADR-NNN: Título da Decisão

## Status

proposed | accepted | deprecated | superseded by [ADR-XXX](XXX-outro.md)

## Contexto

Descrição da situação que motivou a decisão.

## Decisão

O que foi decidido.

## Consequências

### Positivas
- ...

### Negativas
- ...

## Alternativas Consideradas

### Alternativa A
- Descrição
- Por que não foi escolhida

### Alternativa B
- Descrição
- Por que não foi escolhida
```
