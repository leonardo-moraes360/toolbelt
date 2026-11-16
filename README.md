<p align="center">
  <img src="assets/logo/toolbelt-logo-256.png" alt="Logo do Toolbelt" width="200">
</p>

<h1 align="center">toolbelt (<code>tb</code>)</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white" alt="Go Version"></a>
  <a href="docs/arquitetura.md"><img src="https://img.shields.io/badge/Status-Fase%202-green" alt="Status"></a>
  <a href="https://github.com/leonardo-moraes360/toolbelt/actions"><img src="https://github.com/leonardo-moraes360/toolbelt/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/Protocol-MCP-purple" alt="MCP"></a>
  <a href="docs/identidade-visual.md"><img src="https://img.shields.io/badge/Theme-Synthwave-ff278c" alt="Theme"></a>
</p>

<p align="center">
  Cinto de ferramentas para agentes de código com IA, unificadas sob um único comando e protocolo.
</p>

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
| [docs/cli.md](docs/cli.md) | Árvore de comandos do `tb` (implementado + planejado) |
| [docs/benchmark.md](docs/benchmark.md) | KPIs e metodologia de avaliação |
| [docs/adr/](docs/adr/) | Registros de Decisão Arquitetural (ADRs) |
| [docs/pesquisa/](docs/pesquisa/) | Fontes primárias, top 10 e achados da pesquisa (2026-11) |
| [docs/campo/SINTESE.md](docs/campo/SINTESE.md) | Estudo de campo em 20 repos + ranking de melhorias |
| [docs/identidade-visual.md](docs/identidade-visual.md) | Logo, paleta Synthwave e tokens de tema |

## 🚀 Instalação

### Via Go

```bash
go install github.com/leonardo-moraes360/toolbelt/cmd/tb@latest
```

### Via Homebrew (em breve)

```bash
brew install leonardo-moraes360/tap/toolbelt
```

### Build local

```bash
git clone https://github.com/leonardo-moraes360/toolbelt.git
cd toolbelt
make build
./bin/tb version
```

## 📖 Uso Rápido

```bash
# Inicializa configuração local (detecta toolchain → verify.test_cmd)
tb init

# Mostra versão
tb version

# Lista tarefas de benchmark disponíveis
tb bench list

# Executa benchmark com agente fake embutido (funciona em qualquer repo)
tb bench run --fake

# Executa benchmark com agente externo ({prompt} com quoting seguro)
tb bench run --agent "cursor-agent {prompt} --workdir {workdir}" -r 3

# Sem --agent: usa bench.default_agent de .toolbelt/config.yaml
tb bench run -r 1

# Gera relatório de resultados existentes
tb bench report --input bench/results/2026-01-01-120000.jsonl

# Compara dois benchmarks (sucesso, tempo, tokens)
tb bench compare --before results-v1.jsonl --after results-v2.jsonl

# Comparação A/B entre configs/feature flags
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml --fake
```

## 🗺️ Roadmap / Status

### Fase 0 (concluída) ✅
- [x] CLI skeleton com Cobra (`tb version`, `tb init`, `tb help`)
- [x] Configuração hierárquica (global + local)
- [x] Logging estruturado com slog e tema Synthwave
- [x] `tb bench` harness para medir baseline
- [x] Fixtures de benchmark
- [x] Agente fake determinístico para CI
- [x] GitHub Actions CI
- [x] Makefile, golangci-lint, goreleaser

### Fase 1 (concluída) ✅
- [x] `tb mcp` — servidor MCP stateless
- [x] Worktrees + sandbox Landlock
- [x] `tb guard` / `tb verify` (ruler de testes)
- [x] `tb mem` — invariantes fora da compactação

### Fase 2 (em andamento)
- [x] `tb bench compare` A/B (configs/feature flags, métricas token/tempo/sucesso)
- [x] `tb graph` sob demanda (CLI + MCP) com A/B graph-on vs graph-off
- [x] `tb review` — revisão fresh-context contra ticket/rd.md + invariantes
- [x] `tb drift` — detecção de drift (escopo / commit base)
- [x] Parser Markdown de Scope (`## Scope` / listas) e globs com `**` no meio
- [x] Ruler visual: screenshot + browser headless (skip gracioso; `skipped` agregado no relatório)
- [x] Docs CLI alinhados aos comandos/flags reais (`docs/cli.md` / README)
- [ ] Hierarquia de agentes / A2A / `tb md` (posterior)

### Entrada para a próxima fase (estudo de campo)
- Estudo de campo do `tb` em 20 repositórios públicos (relatórios em [`docs/campo/`](docs/campo/) + síntese de melhorias em [`docs/campo/SINTESE.md`](docs/campo/SINTESE.md)), como input priorizado para os próximos PRs de produto.

### Fase 3
- [ ] Router/Workers para delegação escalável de tarefas
- [ ] Entrega direta (direct delivery) — follow-ups vão direto ao worker dono
- [ ] `tb ticket` / `tb board` — gerenciamento de tickets de trabalho
- [ ] `tb router` / `tb watchdog` — controle de delegação e saúde
- [ ] `tb merge` — agregação de resultados de workers
- [ ] Índice vetorial opcional
- [ ] Integração com Graphify (ou reimplementação)
- [ ] Integração com Elephant-Goldfish Model (ou reimplementação)
- [ ] Componentes opcionais em Rust para inferência local
- [ ] Rulers adicionais para `tb verify`

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
- [ADR-008: Verificação Adversarial](docs/adr/008-verificacao-adversarial.md)
- [ADR-009: Router e Workers](docs/adr/009-router-e-workers.md)

## 📄 Licença

[Apache License 2.0](LICENSE) — Copyright 2026 Leonardo Moraes

## 📊 Status do Projeto

Este projeto está na **Fase 2**: CLI funcional com verify/guard/mem/graph/review/drift/MCP e harness de benchmark. A arquitetura completa está documentada em [docs/arquitetura.md](docs/arquitetura.md).

### Comandos Implementados

| Comando | Status | Descrição |
|---------|--------|-----------|
| `tb version` | ✅ | Mostra versão, commit e build time |
| `tb init` | ✅ | Inicializa configuração local (detecta toolchain e preenche `verify.test_cmd`) |
| `tb bench run` | ✅ | Executa benchmark |
| `tb bench list` | ✅ | Lista tarefas disponíveis |
| `tb bench report` | ✅ | Gera relatório Markdown |
| `tb bench compare` | ✅ | Compara dois benchmarks |
| `tb sandbox` | ✅ | Worktrees + Landlock (Linux) |
| `tb guard` | ✅ | Check anti-trapaça em diff |
| `tb verify` | ✅ | Rulers `tests` + `visual` (`tb verify run`; suíte oculta via `--hidden-test-cmd` / `verify.hidden_test_cmd` sem exigir `tests/hidden/`; manifesto; `verify.build_cmd` / `verify.lint_cmd` opcionais) |
| `tb mem` | ✅ | Invariantes (`tb mem add <regra> [--category]`) |
| `tb graph` | ✅ | Grafo sob demanda multi-linguagem + frameworks + docs (ADR-011) |
| `tb review` | ✅ | Revisão fresh-context contra ticket/rd.md |
| `tb drift` | ✅ | Drift de escopo / commit base |
| `tb workspace` | ✅ | Workspaces isolados via git worktree |
| `tb router` | 🚧 | Stub (não implementado) |
| `tb mcp` | ✅ | Servidor MCP stateless (stdio) |
