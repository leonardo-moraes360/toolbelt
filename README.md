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

## 🌐 Site

Landing pública: [https://leonardo-moraes360.github.io/toolbelt/](https://leonardo-moraes360.github.io/toolbelt/) (fonte em [`site/`](site/); guia em [`docs/site.md`](docs/site.md)).

## 📚 Documentação

| Documento | Descrição |
|-----------|-----------|
| [docs/arquitetura.md](docs/arquitetura.md) | Arquitetura detalhada com diagramas Mermaid |
| [docs/cli.md](docs/cli.md) | Árvore de comandos do `tb` (implementado + planejado) |
| [docs/benchmark.md](docs/benchmark.md) | KPIs e metodologia de avaliação |
| [docs/site.md](docs/site.md) | Landing page e deploy no GitHub Pages |
| [docs/adr/](docs/adr/) | Registros de Decisão Arquitetural (ADRs) |
| [docs/pesquisa/](docs/pesquisa/) | Fontes primárias, top 10 e achados da pesquisa (2026-11) |
| [docs/campo/SINTESE.md](docs/campo/SINTESE.md) | Estudo de campo em 20 repos + ranking de melhorias |
| [docs/identidade-visual.md](docs/identidade-visual.md) | Logo, paleta Synthwave e tokens de tema |

## 🚀 Instalação

### Binários (GitHub Releases)

Baixe o arquivo da [página de Releases](https://github.com/leonardo-moraes360/toolbelt/releases) para o seu SO/arch (`linux`, `darwin` ou `windows` × `amd64`/`arm64`), verifique checksum e assinatura, extraia e coloque `tb` no `PATH`. Requer [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/).

```bash
# Exemplo (Linux amd64) — ajuste VERSION/ASSET ao release desejado
VERSION=v0.0.2
ASSET=toolbelt_0.0.2_linux_amd64.tar.gz
BASE="https://github.com/leonardo-moraes360/toolbelt/releases/download/${VERSION}"

curl -sLO "${BASE}/${ASSET}"
curl -sLO "${BASE}/checksums.txt"
curl -sLO "${BASE}/checksums.txt.sigstore.json"

cosign verify-blob \
  --certificate-identity "https://github.com/leonardo-moraes360/toolbelt/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --bundle checksums.txt.sigstore.json \
  checksums.txt

sha256sum -c --ignore-missing checksums.txt
tar -xzf "${ASSET}" tb
sudo mv tb /usr/local/bin/
tb version
```

Cada archive também publica um SBOM (`.sbom.json`) e o workflow anexa atestação de proveniência SLSA (verifique com `gh attestation verify`).

### Via Go

```bash
go install github.com/leonardo-moraes360/toolbelt/cmd/tb@latest
```

> `go install` usa a versão embutida padrão (`dev`) a menos que a tag/módulo injete ldflags. Prefira o binário do Release se precisar da versão da tag.

### Build local

```bash
git clone https://github.com/leonardo-moraes360/toolbelt.git
cd toolbelt
make build
./bin/tb version
```

## 📖 Uso Rápido

O `tb` é **100% local**: **não fala com o GitHub**. `review` / `drift` /
`verify --ticket` / `stack` leem um `rd.md` (ou `--ticket`) no disco. Para
partir de uma issue/PR, exporte o texto com `gh issue view` / `gh pr view` e
monte o arquivo (scope, forbidden, acceptance) — detalhes em
[docs/cli.md](docs/cli.md#local-first-sem-github).

```bash
# Inicializa configuração local (detecta toolchain → test/build/lint_cmd)
tb init

# Se existir AGENTS.md/CLAUDE.md, o init lê e registra hints curtos de comandos
# (filtra fences longos; nunca sobrescreve). Semear AGENTS.md citando test_cmd
# (e build/lint se houver):
tb init --write-agents

# Mostra versão
tb version

# Lista tarefas de benchmark disponíveis
tb bench list

# Executa benchmark com agente fake embutido (funciona em qualquer repo)
tb bench run --fake

# Executa benchmark com agente externo ({prompt} com quoting seguro)
tb bench run --agent "cursor-agent {prompt} --workdir {workdir}" -r 3

# Sem --agent: usa bench.default_agent de .toolbelt/config.yaml
# Sem -t/-f: usa bench.tasks_dir / fixtures_dir (flag CLI > YAML > default)
tb bench run -r 1

# Gera relatório de resultados existentes
tb bench report --input bench/results/2026-01-01-120000.jsonl

# Compara dois benchmarks (sucesso, tempo, tokens)
tb bench compare --before results-v1.jsonl --after results-v2.jsonl

# Comparação A/B entre configs/feature flags
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml --fake

# Drift/review contra ticket local (não há fetch de issue/PR)
tb drift --ticket rd.md
tb review --ticket rd.md
tb guard check HEAD~1..HEAD
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
- [x] GitHub Releases via GoReleaser (tags `v*`)

### Fase 1 (concluída) ✅
- [x] `tb mcp` — servidor MCP stateless
- [x] Worktrees + sandbox Landlock
- [x] `tb guard` / `tb verify` (ruler de testes)
- [x] `tb mem` — invariantes fora da compactação (`show`/`edit`/`retire`/`search` + `--ref`)

### Fase 2 (em andamento)
- [x] `tb bench compare` A/B (configs/feature flags, métricas token/tempo/sucesso)
- [x] `tb graph` sob demanda (CLI + MCP) com A/B graph-on vs graph-off
- [x] `tb review` — revisão fresh-context contra ticket/rd.md + invariantes (+ `--approved-sha` / patch-id)
- [x] `tb drift` — detecção de drift (escopo / commit base / patch pinado)
- [x] Parser Markdown de Scope (`## Scope` / listas) e globs com `**` no meio
- [x] Ruler visual: screenshot + browser headless (skip gracioso; `skipped` agregado no relatório)
- [x] Docs CLI alinhados aos comandos/flags reais (`docs/cli.md` / README)
- [x] MCP com paridade do loop de campo (`mem` / `verify` / `review` / `drift` / `guard`)
- [x] `tb stack` — empilha verify/review/drift/guard com diff/índice/memória compartilhados (+ MCP `tb_stack`)
- [x] Landing page em [`site/`](site/) + GitHub Pages ([docs/site.md](docs/site.md) · [site ao vivo](https://leonardo-moraes360.github.io/toolbelt/))
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
| `tb init` | ✅ | Inicializa configuração local; detecta toolchain (`verify.test_cmd` com checagem de `PATH`/fallback; Node/TS via lockfiles + scripts `test`/`build`|`typecheck`/`lint`; sem marcador → `test_cmd` vazio com aviso, nunca `go test` sem `go.mod`; `build_cmd` / `lint_cmd`), lê `AGENTS.md`/`CLAUDE.md` (hints filtrados) e opcionalmente semeia com `--write-agents` (seed cita test/build/lint detectados) |
| `tb bench run` | ✅ | Executa benchmark |
| `tb bench list` | ✅ | Lista tarefas disponíveis |
| `tb bench report` | ✅ | Gera relatório Markdown |
| `tb bench compare` | ✅ | Compara dois benchmarks |
| `tb sandbox` | ✅ | Worktrees + Landlock (Linux) |
| `tb guard` | ✅ | Check anti-trapaça em diff (default estrito / exit ≠ 0 em `SeverityError`; `--warn-only` opt-out; `--strict` deprecated) |
| `tb verify` | ✅ | Rulers `tests` + `visual` (`tb verify run`; `--build-cmd`/`--lint-cmd`/`--no-build`/`--no-lint`/`--dry-run`; suíte oculta via `--hidden-test-cmd` ou detecção de toolchain em `tests/hidden/` — N/A se inaplicável; cache de build/lint na execução; `reason`/`error` no JSON; `--base`/`--head` rodam no worktree do head; fail-fast se o comando não existe/não é executável; `protected_paths` custom do `guard.yaml` não abortam o loop — exceção de Scope via `--ticket`) |
| `tb mem` | ✅ | Invariantes (`add`/`list`/`show`/`edit`/`retire`/`search`/`check`/`init`; `--ref` ADR; soft-retire + `--all`) |
| `tb graph` | ✅ | Grafo sob demanda multi-linguagem + frameworks + docs (ADR-011); `javascript` cobre TypeScript (`ts`/`tsx`) |
| `tb review` | ✅ | Revisão fresh-context contra ticket/`rd.md` local (sem GitHub); `--approved-sha` |
| `tb drift` | ✅ | Drift de escopo / commit base / patch pinado contra `rd.md` (não é vault vs código) |
| `tb stack` | ✅ | Empilha verify/review/drift/guard com contexto compartilhado (JSON agregado; MCP `tb_stack`) |
| `tb workspace` | ✅ | Workspaces isolados via git worktree |
| `tb router` | 🚧 | Stub (não implementado) |
| `tb mcp` | ✅ | Servidor MCP stateless (stdio); tools de mem/verify/review/drift/guard/stack + graph (incl. `tb_graph_impact`; schemas tipados) |
