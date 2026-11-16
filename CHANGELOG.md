# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [Não Lançado]

### Adicionado
- `tb init` preenche `verify.build_cmd` e `verify.lint_cmd` a partir da toolchain (scripts `build`/`lint`, golangci, ruff/mypy, `cargo build`/`clippy`, targets make)
- `tb guard check` aceita `--base`/`--head` (como review/drift/verify) além do intervalo posicional `base..head`
- Indexador Python emite arestas `calls` resolvíveis no repositório (`tb graph callers` / `impact`)

### Corrigido
- `tb graph callers`/`impact` em Python: resolve por sufixo de nome qualificado e ignora arestas `calls` com confiança &lt; 0.6 (reduz alvos espúrios como `setup.py`→`tests.setup`)
- `tb init` lê `AGENTS.md`/`CLAUDE.md` na raiz (registra em `.toolbelt/agent_docs.yaml` e extrai dicas de comandos; nunca sobrescreve) e aceita `--write-agents` para semear um `AGENTS.md` mínimo apontando ao tb
- `tb guard` carrega `protected_paths` de `.toolbelt/guard.yaml` (unidos aos defaults; arquivo ausente mantém só os defaults)
- `tb init` detecta toolchain (`go.mod`, `package.json`/npm/pnpm/yarn, pyproject/uv/pytest, `Cargo.toml`, `Makefile`) e preenche `verify.test_cmd`
- Ruler visual em `tb verify run --ruler visual` (screenshot headless, similaridade pixel, skip gracioso)
- `tb review` fresh-context (Elephant-Goldfish) contra ticket/rd.md e invariantes de `tb mem`
- `tb drift` para mudanças fora do scope declarado e divergência de commit base
- `tb graph` sob demanda (ADR-011): index/search/callers/deps/impact/stats atrás de `TB_GRAPH`/`--enable`
- Ferramentas MCP de grafo: `tb_graph_search`, `tb_graph_callers`, `tb_graph_deps`, `tb_graph_index`
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

### Corrigido
- `tb bench`: honra `bench.tasks_dir` / `fixtures_dir` / `output_dir` do config.yaml
  (precedência: flag CLI > YAML > default); a flag global `-c/--config` alimenta `config.Load()`
- `protected_paths` com barra final (`secrets/`) passa a proteger filhos do diretório
- `tb review`: invariante `tests/hidden` (e `thresholds.yaml`) casa por prefixo/segmento
  de path nos arquivos alterados, não por substring no diff (evita FP quando o ticket
  só menciona a regra)

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
