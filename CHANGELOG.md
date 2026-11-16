# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [Não Lançado]

### Adicionado
- `tb sandbox run --allow-unsandboxed`: opt-in explícito para executar sem isolamento Landlock
- Teste de integração que tenta escrever fora do workdir (pula se o kernel não tiver Landlock)

### Corrigido
- **Sandbox Landlock (fail-closed):** probe de ABI usa `LANDLOCK_CREATE_RULESET_VERSION` (antes `flags=0` → `EFAULT` e falso “sem Landlock”); `PR_SET_NO_NEW_PRIVS` antes de `landlock_restrict_self`; rights FS completos por ABI (incl. truncate/make_*); ruleset aplicado no filho via re-exec antes do `exec` do comando; erros de ruleset não são mais engolidos; sem Landlock a execução recusa com erro claro (use `--allow-unsandboxed`)

## [0.0.2] - 2026-11-16

### Adicionado
- `tb stack <op> [<op>…]` empilha `verify`/`review`/`drift`/`guard` numa execução com
  contexto compartilhado (diff, índice de arquivos, memória e config); JSON agregado
  (`ok`/`steps`/`total_duration_ms`); `--fail-fast`; MCP `tb_stack` com schema tipado
- MCP `tb_graph_impact` (paridade com `tb graph impact`)
- `tb review` / `tb drift`: flag `--approved-sha` compara `git patch-id --stable`
  de `base..approved` vs `base..head`; em divergência mostra `git range-diff` e
  sai ≠ 0; JSON registra `pinned_approval` (`approved_sha`, `patch_id`,
  `head_patch_id`, `match`, `range_diff`)
- Indexador JS/TS emite arestas `calls` resolvíveis (chamadas + JSX `<Comp />`) via imports relativos; `tb graph callers`/`impact` funcionam em fixtures Next/React (`pages/` → `components/`/`lib/`)
- `tb mem show|edit|retire|search` e vínculo `--ref` com ADR/decisions.md; soft-retire omitido de list/check/search (use `--all`); subcomando desconhecido sai ≠ 0
- MCP: `tb_mem_show`, `tb_mem_edit`, `tb_mem_retire`, `tb_mem_search` (+ `ref`/`all` em add/list/check)

### Mudanças
- Schemas MCP tipam `depth`/`limit`/`reviewers`/`max_iter` como integer, `strict`/`warn_only`/`transitive`/`skip_unavailable` como boolean, `threshold` como number e `standards` como array; handlers ainda aceitam strings legadas
- **Breaking:** saídas JSON da CLI (`--json`) e dos payloads de domínio no MCP usam
  exclusivamente chaves `snake_case`. Structs sem tag (`sandbox.Result`,
  `mem.Invariant`, `workspace.Workspace`/`Scope`) deixam de emitir PascalCase
  (`ExitCode` → `exit_code`, `TimedOut` → `timed_out`, `duration_ns`, etc.).
  Envelopes do protocolo MCP (`isError`, `inputSchema`, …) permanecem camelCase.
- Frameworks (ex. `react` em `.tsx`) rodam em paralelo com o indexador de linguagem, não no lugar dele
- `verify.build_cmd` em Node também aceita scripts `typecheck` / `type-check` quando
  `build` está ausente
- `tb guard check` sai com exit ≠ 0 por default quando há `SeverityError` (CI-safe); opt-out via `--warn-only`; `--strict` permanece como alias deprecated
- Modo texto limpo imprime `guard: ok (N regras, 0 violações)`; JSON/MCP incluem `rules_checked`
- MCP `tb_guard_check`: `strict_fail` por default em SeverityError; `warn_only=true` para opt-out (`strict` deprecated)
- `tb review`: goldfish distintos por papel (`scope`, `quality`, `acceptance`) em vez do mesmo checker clonado N vezes; `--reviewers` só limita papéis; qualidade cobre TODO/FIXME, funções grandes e código sem testes; aceite exige evidência em testes/arquivos de código

### Corrigido
- Descrição longa do CLI: completa o adjetivo em pt-BR (`persistent` → `persistente`)
- `docs/cli.md` alinhado à CLI real: remove `graph callers --file`; `router` documentado como stub; seções planejadas (`proc`/`agent`/`watchdog`/`merge`) marcadas; `init`/`sandbox`/`workspace`, mem show/edit/retire/search, guard `--warn-only`, review standards e códigos de saída conferidos contra `--help`/MCP
- Teste `TestDocsCLIExamplesMatchCobra` extrai `tb …` de `docs/cli.md` e valida subcomandos/flags na árvore Cobra
- `tb verify run` falha imediatamente (sem retries) quando o comando configurado não existe (exit 127), não é executável (exit 126) ou `exec` retorna `ErrNotFound`, com mensagem `comando não encontrado: … — ajuste verify.test_cmd` (e equivalentes para build/lint/hidden)
- `tb init` em repos Node/TS sem `go.mod` não mantém mais `verify.test_cmd: go test ./...`:
  detecta o package manager (`pnpm-lock.yaml`/`yarn.lock`/`package-lock.json`/`bun.lockb`),
  preenche `test`/`build`|`typecheck`/`lint` a partir de `package.json`, e se nada for
  detectável deixa `test_cmd` vazio com aviso; o seed de `AGENTS.md` reflete isso
- CLI: erros esperados de `RunE` (domínio/runtime) não imprimem mais o dump de `Usage` do Cobra no stderr; `Usage` permanece só em erros genuínos de parse de flags/args. Subcomandos desconhecidos em qualquer nível saem com exit ≠ 0 e mensagem clara.
- Erro de grafo desabilitado (CLI e MCP) cita como ligar: `--enable` ou `TB_GRAPH=on`

## [0.0.1] - 2026-11-16

### Adicionado
- CI de release: GoReleaser (`.goreleaser.yaml`) + workflow em tags `v*` publicando binários `tb` (linux/darwin/windows × amd64/arm64), `checksums.txt` e changelog
- Documentação de instalação via GitHub Releases e guia `docs/release.md`
- Versão padrão `dev` injetável via ldflags (`tb version` / MCP `tb_version`)
- `tb init --write-agents` semeia `AGENTS.md` citando a toolchain detectada (`verify.test_cmd` e, se presentes, `build_cmd`/`lint_cmd`)
- MCP: tools de paridade CLI `tb_mem_*`, `tb_verify_run`, `tb_review`, `tb_drift`, `tb_guard_check`
- Indexador Rust emite arestas `calls` mínimas resolvíveis no repositório (`tb graph callers` / `impact`)
- `tb graph callers`/`impact` retornam erro explícito em C/C++ (sem arestas `calls`) em vez de `(vazio)` silencioso
- `tb init` preenche `verify.build_cmd` e `verify.lint_cmd` a partir da toolchain (scripts `build`/`lint`, golangci, ruff/mypy, `cargo build`/`clippy`, targets make)
- `tb guard check` aceita `--base`/`--head` (como review/drift/verify) além do intervalo posicional `base..head`
- Indexador Python emite arestas `calls` resolvíveis no repositório (`tb graph callers` / `impact`)
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
- `tb graph callers`/`impact` em Python: resolve por sufixo de nome qualificado e ignora arestas `calls` com confiança &lt; 0.6 (reduz alvos espúrios como `setup.py`→`tests.setup`)
- docs/cli.md alinhado à CLI real: `graph index [path]` (sem `--path`), sem `graph query`, `review`/`drift` só `--output` (sem `-o`), flags de graph/mem verificadas contra `--help`
- `tb drift` resolve refs git (`HEAD^`, tags, SHA curto) via `rev-parse` antes de
  comparar com `base_commit` (evita falso `base_mismatch`)
- `tb verify run --json` sai com exit ≠ 0 em `ESCALATE`/`FAIL` (alinhado ao modo texto)
- `tb init`: hints de `AGENTS.md`/`CLAUDE.md` ignoram fences de código longos e
  aceitam prosa curta (“use/run/prefer …”); `verify.test_cmd` valida o binário no
  `PATH` com fallback (ex. `uv` ausente → `pytest` / `python -m pytest`)
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
