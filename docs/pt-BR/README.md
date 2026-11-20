[🇺🇸 English](../../README.md) | 🇧🇷 Português

<p align="center">
  <img src="../../assets/logo/toolbelt-logo-256.png" alt="Logo do Toolbelt" width="200">
</p>

<h1 align="center">toolbelt (<code>tb</code>)</h1>

<p align="center">
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white" alt="Go Version"></a>
  <a href="arquitetura.md"><img src="https://img.shields.io/badge/Status-Fase%203%20em%20andamento-yellow" alt="Status"></a>
  <a href="https://github.com/leonardo-moraes360/toolbelt/actions"><img src="https://github.com/leonardo-moraes360/toolbelt/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/Protocol-MCP-purple" alt="MCP"></a>
  <a href="https://m8ven.ai/mcp/leonardo-moraes360-toolbelt-fa03ue?s=readme"><img src="https://m8ven.ai/badge/mcp/leonardo-moraes360-toolbelt-fa03ue?v=ac65034c346c6c7522ef08b6df2496ef" alt="M8ven Score"></a>
  <a href="identidade-visual.md"><img src="https://img.shields.io/badge/Theme-Synthwave-ff278c" alt="Theme"></a>
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

Veja a documentação completa em [docs/pt-BR/arquitetura.md](arquitetura.md).

## 🌐 Site

Landing pública: [https://leonardo-moraes360.github.io/toolbelt/](https://leonardo-moraes360.github.io/toolbelt/) (fonte em [`site/`](../../site/); guia em [`docs/pt-BR/site.md`](site.md)).

## 📚 Documentação

| Documento | Descrição |
|-----------|-----------|
| [docs/pt-BR/arquitetura.md](arquitetura.md) | Arquitetura detalhada com diagramas Mermaid |
| [docs/pt-BR/cli.md](cli.md) | Árvore de comandos do `tb` (implementado + planejado) |
| [docs/pt-BR/bus-protocolo.md](bus-protocolo.md) · [docs/pt-BR/bus-harness.md](bus-harness.md) | Barramento local `tb bus` (protocolo `bus/0.1` e harness `tb bus run`) |
| [docs/pt-BR/benchmark.md](benchmark.md) | KPIs e metodologia de avaliação |
| [docs/pt-BR/site.md](site.md) | Landing page e deploy no GitHub Pages |
| [docs/pt-BR/adr/](adr/) | Registros de Decisão Arquitetural (ADRs) |
| [docs/pt-BR/pesquisa/](pesquisa/) | Fontes primárias, top 10 e achados da pesquisa (2026-11) |
| [docs/pt-BR/campo/SINTESE.md](campo/SINTESE.md) | Estudo de campo em 20 repos + ranking de melhorias |
| [docs/pt-BR/identidade-visual.md](identidade-visual.md) | Logo, paleta Synthwave e tokens de tema |

## 🚀 Instalação

### Binários (GitHub Releases)

Baixe o arquivo da [página de Releases](https://github.com/leonardo-moraes360/toolbelt/releases) para o seu SO/arch (`linux`, `darwin` ou `windows` × `amd64`/`arm64`), verifique checksum e assinatura, extraia e coloque `tb` no `PATH`. Requer [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/).

```bash
# Exemplo (Linux amd64) — ajuste VERSION/ASSET ao release desejado
VERSION=v0.0.5
ASSET=toolbelt_0.0.5_linux_amd64.tar.gz
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
[docs/pt-BR/cli.md](cli.md#local-first-sem-github).

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

# Fase 3: tickets -> router/workers -> merge (tudo local)
tb ticket create --title "Criar modelo User" --scope "src/models/**"
tb ticket create --title "JWT signing" --dep TKT-001
tb router run --agent-cmd "meu-agente --task-file $TB_TICKET_FILE"
tb board list
tb watchdog alerts
tb merge --dry-run TKT-001 TKT-002
tb merge TKT-001 TKT-002

# Barramento local entre agentes (ADR-013); só o humano cria canal/convite
tb bus channel create feature-x --owner claude-main --ttl 2h
tb bus invite feature-x --role worker
tb bus join feature-x --invite K7Q2-9XMD --name grok-1 --vendor xai
tb bus recv --channel feature-x --as grok-1 --wait 20s
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
- [x] Landing page em [`site/`](../../site/) + GitHub Pages ([docs/pt-BR/site.md](site.md) · [site ao vivo](https://leonardo-moraes360.github.io/toolbelt/))
- [ ] Hierarquia de agentes / A2A / `tb md` (posterior)

### Entrada para a próxima fase (estudo de campo)
- Estudo de campo do `tb` em 20 repositórios públicos (relatórios em [`docs/pt-BR/campo/`](campo/) + síntese de melhorias em [`docs/pt-BR/campo/SINTESE.md`](campo/SINTESE.md)), como input priorizado para os próximos PRs de produto.

### Fase 3 (em andamento)
- [x] Router/Workers para delegação escalável de tarefas (`tb router run`, em primeiro plano, workers em worktrees isoladas e sandbox fail-closed)
- [x] Entrega direta (direct delivery) — follow-ups vão direto ao worker dono
- [x] `tb ticket` / `tb board` — gerenciamento de tickets de trabalho
- [x] `tb router` / `tb watchdog` — controle de delegação e saúde
- [x] `tb merge` — agregação de resultados de workers (`fail` | `last_wins`)
- [ ] Índice vetorial opcional
- [x] Graphify: reimplementação própria ([ADR-014](adr/014-graphify-reimplementacao.md)) — `tb graph explain`/`path`
- [x] `tb graph search` com ranking BM25 (título/path/pacote, quebra de camelCase/`snake_case`, multi-termo)
- [x] Vault Markdown compatível com Obsidian, Fase A (`tb vault`, [ADR-016](adr/016-vault-fase-a.md), [spec](specs/vault.md)): notas com frontmatter e wikilinks, `new`/`list`/`show`/`search`/`links`/`backlinks`/`check`, tools MCP, `tb stack vault`, nós no grafo; Q4 (fixture dourada conferida no Obsidian real) aberta
- [x] Reimplementação do Elephant-Goldfish Model, Fase A (`tb egm`, [ADR-015](adr/015-egm-nativo.md) aceito): spike R0; engines determinística e `llm`, árbitro, gate no `tb stack` (`status --at`) e tools MCP
- [ ] Próximas fases do `tb egm`: CI como fronteira de confiança contra fraude ativa (Fase A.2, [spec em rascunho](specs/egm-trust.md)), `drift-report` e `bootstrap` (Fase B); item 8 do R0 aberto
- [ ] Componentes opcionais em Rust para inferência local
- [ ] Rulers adicionais para `tb verify`
- [ ] `tb merge` estratégias `prompt` / `semantic`

### Barramento multi-vendor (`tb bus`, [ADR-013](adr/013-barramento-multi-vendor.md))
- [x] B0 — spec `bus/0.1`: envelope, Agent Card e convite ([docs/pt-BR/bus-protocolo.md](bus-protocolo.md))
- [x] B1 — store SQLite, CLI + MCP, convite de uso único, `join`/`send`/`recv`/`claim`/`complete`/`log`
- [ ] B2 — lease, heartbeat e watchdog no barramento
- [ ] B3 (parcial) — `tb bus run` com adapters `exec` e `harness` feito ([docs/pt-BR/bus-harness.md](bus-harness.md)); falta adapter `notify` e `tb bus discover`
- [ ] B4 — router/board da Fase 3 sobre o barramento

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
- [ADR-006: Graphify e Obsidian](adr/006-inspiracoes-graphify-obsidian.md)
- [ADR-007: Elephant-Goldfish Model](adr/007-inspiracao-elephant-goldfish.md)
- [ADR-008: Verificação Adversarial](adr/008-verificacao-adversarial.md)
- [ADR-009: Router e Workers](adr/009-router-e-workers.md)

## 📄 Licença

[Apache License 2.0](../../LICENSE) — Copyright 2026 Leonardo Moraes

## 📊 Status do Projeto

Este projeto está na **Fase 3 (em andamento)**: além de verify/guard/mem/graph/review/drift/MCP e harness de benchmark, já há tickets, board, router/workers, watchdog, merge e o barramento local `tb bus` (B0/B1 e parte do B3 do [ADR-013](adr/013-barramento-multi-vendor.md)) (ganho de escala ainda não medido — ver [ADR-009](adr/009-router-e-workers.md)). A arquitetura completa está documentada em [docs/pt-BR/arquitetura.md](arquitetura.md).

### Comandos Implementados

| Comando | Status | Descrição |
|---------|--------|-----------|
| `tb version` | ✅ | Mostra versão, commit e build time |
| `tb init` | ✅ | Inicializa configuração local; detecta toolchain (`verify.test_cmd` com checagem de `PATH`/fallback; Node/TS via lockfiles + scripts `test`/`build`|`typecheck`/`lint`; sem marcador → `test_cmd` vazio com aviso, nunca `go test` sem `go.mod`; `build_cmd` / `lint_cmd`), lê `AGENTS.md`/`CLAUDE.md` (hints filtrados) e opcionalmente semeia com `--write-agents` (seed cita test/build/lint detectados) |
| `tb bench run` | ✅ | Executa benchmark |
| `tb bench list` | ✅ | Lista tarefas disponíveis |
| `tb bench report` | ✅ | Gera relatório Markdown |
| `tb bench compare` | ✅ | Compara dois benchmarks |
| `tb sandbox` | ✅ | Worktrees + Landlock (Linux) (`check`/`run`) |
| `tb guard` | ✅ | Check anti-trapaça em diff (default estrito / exit ≠ 0 em `SeverityError`; `--warn-only` opt-out; `--strict` deprecated) |
| `tb verify` | ✅ | Rulers `tests` + `visual` (`tb verify run`; `--build-cmd`/`--lint-cmd`/`--no-build`/`--no-lint`/`--dry-run`; suíte oculta via `--hidden-test-cmd` ou detecção de toolchain em `tests/hidden/` — N/A se inaplicável; cache de build/lint na execução; `reason`/`error` no JSON; `--base`/`--head` rodam no worktree do head; fail-fast se o comando não existe/não é executável; `protected_paths` custom do `guard.yaml` não abortam o loop — exceção de Scope via `--ticket`) |
| `tb mem` | ✅ | Invariantes (`add`/`list`/`show`/`edit`/`retire`/`search`/`check`/`init`; `--ref` ADR; soft-retire + `--all`) |
| `tb graph` | ✅ | Grafo sob demanda multi-linguagem + frameworks + docs (ADR-011); `javascript` cobre TypeScript (`ts`/`tsx`); `search` com ranking BM25, `callers`/`deps`/`impact`, `explain`/`path` (ADR-014) |
| `tb review` | ✅ | Revisão fresh-context contra ticket/`rd.md` local (sem GitHub); `--approved-sha` |
| `tb drift` | ✅ | Drift de escopo / commit base / patch pinado contra `rd.md` (não é vault vs código) |
| `tb stack` | ✅ | Empilha verify/review/drift/guard com contexto compartilhado (JSON agregado; MCP `tb_stack`) |
| `tb workspace` | ✅ | Workspaces isolados via git worktree |
| `tb ticket` | ✅ | Tickets de trabalho em `.toolbelt/tickets/` (`create`/`list`/`show`/`cancel`/`retry`; MCP `tb_ticket_*`) |
| `tb board` | ✅ | Board RUNNING/READY/BLOCKED/DONE/FAILED; `pause`/`resume`/`drain`/`watch` (MCP `tb_board`) |
| `tb router` | ✅ | Scheduler em primeiro plano (`run`/`status`/`config`) com entrega direta; sandbox fail-closed (`--allow-unsandboxed`) |
| `tb watchdog` | ✅ | Alertas e controle dos workers (`status`/`alerts`/`kill`); roda dentro do loop do router |
| `tb merge` | ✅ | Integra tickets `done` de forma atômica; `fail` (padrão) ou `last_wins` (MCP `tb_merge`); `prompt`/`semantic` não implementadas |
| `tb bus` | 🟡 | Barramento local entre agentes (ADR-013): `channel`/`invite`/`join`/`send`/`recv`/`claim`/`complete`/`log` sobre `.toolbelt/bus/bus.db`; `tb bus run` acorda o agente (adapters `exec`/`harness`); lease/heartbeat (B2), `notify` e `discover` pendentes |
| `tb mcp` | ✅ | Servidor MCP stateless (stdio); 36 tools na v0.0.5: mem/verify/review/drift/guard/stack, graph (`search`/`callers`/`deps`/`impact`/`explain`/`path`/`index`; exige `TB_GRAPH=on`), ticket/board/router/watchdog/merge e bus (`tb_bus_join`/`send`/`recv`/`claim`/`complete`); `--bus-as` prende a conexão a uma sessão do bus |
