# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [Não Lançado]

### Adicionado
- Comparação A/B em `tb bench compare` (configs YAML / feature flags no mesmo task set)
- Métricas de sucesso, tempo e tokens no JSONL e no relatório Markdown A/B
- Fixtures realistas: `localize-callers`, `cross-file-rename`, `api-handler`
- Configs A/B de exemplo em `bench/ab/` (`graph-on`, `graph-off`, `baseline`)
- CLI skeleton com Cobra (`tb version`, `tb init`, `tb help`)
- Configuração hierárquica (global `~/.config/toolbelt` + local `.toolbelt/`)
- Logging estruturado com `log/slog` e tema Synthwave
- Suporte a `NO_COLOR` e `TERM=dumb` para ambientes sem cor
- Flags `--json` para saída estruturada
- `tb bench run` - harness para executar benchmarks contra agentes
- `tb bench list` - lista tarefas de benchmark disponíveis
- `tb bench report` - gera relatório Markdown de resultados
- `tb bench compare` - compara dois benchmarks
- Agente fake (`fake-agent`) para testes no CI sem LLM
- 5 tarefas fixture: `bugfix-go`, `feature-small`, `refactor-simple`, `test-coverage`, `doc-function`
- Resultados em formato JSONL para análise programática
- Métricas: pass@1, pass@k, tokens, tempo, tool calls
- Makefile com targets: `build`, `test`, `lint`, `bench-fake`
- GitHub Actions CI: test, lint, build, bench-fake
- Configuração golangci-lint
- Configuração goreleaser para releases
- ADR-012: Framework CLI - Cobra
- Stubs para comandos futuros: `sandbox`, `verify`, `mem`, `graph`, `router`, `mcp`

### Mudanças
- README atualizado com instalação, uso rápido e comandos implementados
- docs/cli.md marcando comandos implementados
- docs/benchmark.md com seção sobre execução na Fase 0

## [0.0.0] - 2026-11-15

### Adicionado
- Documentação de arquitetura completa
- ADRs 001-011 definindo decisões técnicas
- Roadmap em 3 fases (MVP, Hierarquia de Agentes, Ferramentas Avançadas)
- Pesquisa de fontes primárias (100+ papers e repositórios)
- Identidade visual Synthwave
- Templates de issues e PRs
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md
