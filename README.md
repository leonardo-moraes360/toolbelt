# toolbelt (`tb`)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Status](https://img.shields.io/badge/Status-Em%20Design-yellow)](docs/arquitetura.md)
[![MCP](https://img.shields.io/badge/Protocol-MCP-purple)](https://modelcontextprotocol.io/)

Cinto de ferramentas para agentes de código com IA, unificadas sob um único comando e protocolo.

## 🎯 Objetivos

| Objetivo | Como o toolbelt ajuda |
|----------|----------------------|
| **Economizar tokens** | Grafo de conhecimento queryável evita grep em arquivos; contexto montado sob orçamento de tokens; memória compacta com sumários e fatos extraídos |
| **Melhorar a acurácia** | Sandbox isolado com bubblewrap+Landlock; permissões por capacidade; proteções contra trapaça; separador entre implementador e avaliador |
| **Aumentar a velocidade** | Execução paralela com goroutines; hierarquia de agentes (planner→supervisor→worker); busca rápida em grafo com FTS5 |

## 🏗️ Arquitetura

```
┌─────────────────────────────────────────────────────────────────┐
│                        Interface Humana                         │
│  ┌─────────┐    ┌─────────────┐    ┌─────────────────────────┐  │
│  │ tb CLI  │    │ tb md (TUI) │    │ Obsidian (vault externo)│  │
│  └────┬────┘    └──────┬──────┘    └────────────┬────────────┘  │
└───────┼────────────────┼───────────────────────┼────────────────┘
        │                │                       │
        ▼                ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Protocolos                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ MCP Server  │  │ A2A Protocol│  │ JSON-RPC 2.0 (stdio/UDS)│  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                           Núcleo                                │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌───────────┐  │
│  │ Orquestrador│  │  Memória   │  │  Contexto  │  │   Grafo   │  │
│  └────────────┘  └────────────┘  └────────────┘  └───────────┘  │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Armazenamento                              │
│  ┌─────────────────────┐    ┌─────────────────────────────────┐ │
│  │ Vault Markdown      │───▶│ SQLite + FTS5 (índice derivado) │ │
│  │ (fonte de verdade)  │    │ + tabelas de arestas            │ │
│  └─────────────────────┘    └─────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Execução                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Worktrees   │  │ bubblewrap  │  │ cgroups (CPU/RAM/tempo) │  │
│  │ Git         │  │ +Landlock   │  │                         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

Veja a documentação completa em [docs/arquitetura.md](docs/arquitetura.md).

## 📚 Documentação

| Documento | Descrição |
|-----------|-----------|
| [docs/arquitetura.md](docs/arquitetura.md) | Arquitetura detalhada com diagramas Mermaid |
| [docs/cli.md](docs/cli.md) | Árvore de comandos planejada para `tb` |
| [docs/benchmark.md](docs/benchmark.md) | KPIs e metodologia de avaliação |
| [docs/adr/](docs/adr/) | Registros de Decisão Arquitetural (ADRs) |

## 🗺️ Roadmap

### MVP (Fase 1)
- [ ] Sandbox com isolamento (bubblewrap + Landlock + seccomp)
- [ ] Vault Markdown como fonte de verdade para memória
- [ ] Grafo SQLite+FTS5 como índice derivado
- [ ] Indexação de código via tree-sitter com proveniência
- [ ] `tb run` / `tb proc` para execução de scripts
- [ ] Servidor MCP
- [ ] Harness de benchmark

### Fase 2
- [ ] Hierarquia de agentes (planner → supervisor → worker)
- [ ] Protocolo A2A para comunicação entre agentes
- [ ] `tb md` TUI para revisão de memória e planos
- [ ] Interoperabilidade com Obsidian
- [ ] `tb review` — revisão com contextos frescos (Elephant-Goldfish)
- [ ] `tb drift` — detecção de divergência docs/código
- [ ] `tb verify` — verificação adversarial com rulers plugáveis
- [ ] Ruler visual: similaridade pixel + contraste WCAG
- [ ] Guard check com manifesto de paths protegidos

### Fase 3
- [ ] Router/Workers para delegação escalável de tarefas
- [ ] `tb ticket` / `tb board` — gerenciamento de tickets de trabalho
- [ ] `tb router` / `tb watchdog` — controle de delegação e saúde
- [ ] `tb merge` — agregação de resultados de workers
- [ ] Índice vetorial opcional
- [ ] Integração com Graphify (ou reimplementação)
- [ ] Integração com Elephant-Goldfish Model (ou reimplementação)
- [ ] Componentes opcionais em Rust para inferência local

## 🤝 Contribuindo

Veja [CONTRIBUTING.md](CONTRIBUTING.md) para detalhes sobre:
- Fluxo de branches e PRs
- Conventional Commits
- Como propor ADRs
- Setup de desenvolvimento

## 📖 Inspirações

Este projeto foi inspirado por:
- **[Graphify](https://github.com/Graphify-Labs/graphify)**: mapeamento de projetos em grafos de conhecimento queryáveis
- **[Obsidian](https://obsidian.md/)**: vault de Markdown com wikilinks, backlinks e grafo local-first
- **[Elephant-Goldfish Model](https://github.com/google/elephant-goldfish-model)**: revisão com contextos frescos, separação autor/revisor, drift detection

Veja os ADRs para detalhes:
- [ADR-006: Graphify e Obsidian](docs/adr/006-inspiracoes-graphify-obsidian.md)
- [ADR-007: Elephant-Goldfish Model](docs/adr/007-inspiracao-elephant-goldfish.md)
- [ADR-009: Router e Workers](docs/adr/009-router-e-workers.md)

## 📄 Licença

[Apache License 2.0](LICENSE) — Copyright 2026 Leonardo Moraes

## 📊 Status do Projeto

Este projeto está em **fase de design**. A documentação de arquitetura está sendo desenvolvida antes de qualquer implementação. Nenhum código funcional foi escrito ainda.
