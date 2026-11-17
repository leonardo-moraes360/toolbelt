🇧🇷 Português | [🇺🇸 English](../en/adr/014-graphify-reimplementacao.md)

# ADR-014: Graphify — Reimplementar, Não Integrar

## Status

accepted

## Contexto

O [ADR-006](006-inspiracoes-graphify-obsidian.md) deixou em aberto a questão
"integrar o Graphify via MCP ou reimplementar a indexação?", com a decisão
provisória de experimentar a integração primeiro.

Na prática o caminho já foi outro: o `tb graph` (Fase 2, [ADR-011](011-grafo-sob-demanda.md))
é uma reimplementação própria em `internal/graph`:

- Go via `go/ast`; TS/JS, Python, Rust e frameworks (React, Vue, Svelte,
  Angular, Astro, MDX) via extratores regex conservadores; docs Markdown com
  wikilinks.
- Proveniência em nós e arestas (`source`, `confidence`), com filtro
  `MinCallsConfidence = 0.6` em `callers`/`impact`.
- Índice JSON local (`.toolbelt/graph.json`) com fingerprint de frescor.
- Paridade CLI ↔ MCP (`tb graph *` ↔ `tb_graph_*`).

Nenhuma linha depende do Graphify. O estudo de campo ([SINTESE](../campo/SINTESE.md))
classificou "Graphify integração profunda" como **sem suporte**: o gargalo
observado foi a qualidade dos extratores (callers/impact vazios fora de Go/JS),
não a falta de um backend externo.

## Decisão

**Reimplementar.** O toolbelt não integra o Graphify (nem como dependência
Go, nem como servidor MCP externo). O Graphify segue como referência de
produto: copiamos ideias, não código.

Critérios que decidiram:

1. **Invariante local-first** — `tb` é 100% local e sem rede. O Graphify
   inclui etapas com LLM (arestas `INFERRED`) e pipeline Python; integrá-lo
   traria um segundo runtime e dependências de rede opcionais difíceis de
   cercar no sandbox.
2. **Binário único em Go** — sem dependência nova em `go.mod`, sem
   tree-sitter via cgo (manteria release reprodutível + SBOM + cosign simples).
3. **Proveniência já compatível** — `extracted`/`inferred`/`manual` + confidence
   cobre o modelo `EXTRACTED/INFERRED/AMBIGUOUS` do Graphify.
4. **Sob demanda** — o grafo continua desligado por padrão (ADR-011); um
   backend externo pesado contraria o custo medido em A/B.

### Primeira entrega desta decisão

Duas consultas do Graphify, reimplementadas de forma determinística (sem LLM):

| Graphify | toolbelt | O que faz |
|----------|----------|-----------|
| `explain "<nó>"` | `tb graph explain <nó>` / `tb_graph_explain` | Nó + arestas de entrada/saída com `source`/`confidence`; conta chamadas sem nó no índice (`unresolved_out`) |
| `path "A" "B"` | `tb graph path <a> <b>` / `tb_graph_path` | Menor caminho (BFS) entre dois nós, andando arestas nos dois sentidos; cada passo diz `forward`/`backward` |

Regras comuns: arestas `calls` com confiança &lt; 0.6 são ignoradas (como em
`callers`/`impact`); arestas cujo alvo não existe como nó não entram no
caminho; resolução de nó por ID exato → título exato → sufixo de função →
arquivo/pacote; desempate por ID (saída estável).

### Roadmap (fora desta entrega)

Ordem por valor medido no campo, cada item em PR próprio:

1. **Resolução de chamadas por receptor em Go** — em 2026-11-17, 9.580 de 17.302
   arestas (~55%) do índice deste repo apontam para alvos sem nó (`func:s.Metodo`, stdlib,
   builtins). É o maior ganho para `callers`/`impact`/`path`.
2. **God nodes / hubs** em `tb graph stats` — top-N por grau, como
   o relatório do Graphify.
3. **Comunidades** (label propagation determinístico) — agrupar módulos
   para orientar o agente em repositórios grandes.
4. **Relatório Markdown no vault** (`GRAPH_REPORT.md`-like), compatível com
   Obsidian (ADR-006).

Arestas `inferred` via LLM continuam fora até haver modelo local (Fase 3,
componentes Rust opcionais).

## Consequências

### Positivas

- Questão aberta do ADR-006 fechada; docs passam a descrever o código real
  (go/ast + regex, não tree-sitter).
- Zero dependência externa nova; release continua binário único.
- Agentes ganham `explain`/`path`, úteis para localizar antes de ler arquivos.

### Negativas

- Manter extratores por linguagem é nosso trabalho (regex tem teto de precisão).
- Sem arestas inferidas por LLM, relações implícitas (DI, reflexão) ficam de fora.

## Alternativas Consideradas

- **Integrar Graphify via MCP** — rejeitada: segundo runtime (Python),
  etapas com LLM/rede contra a invariante local-first, e o campo não mostrou
  ganho que justifique.
- **tree-sitter via cgo** — adiada: melhor precisão, mas cgo complica
  cross-compile e reprodutibilidade; reavaliar se regex virar teto real.
