🇧🇷 Português | [🇺🇸 English](../../en/campo/rodada2/typer.md)

# Estudo de campo rodada 2: fastapi/typer

Grupo: without existing AI-agent instrumentation (sem instrumentação de agente de IA para contribuição no próprio repositório).  
Binário: `/tmp/tb` (build `go build -o /tmp/tb ./cmd/tb` a partir de `main` @ `a46c295`).  
Clone de trabalho: `/tmp/typer-r2`.  
Data: 2026-11-16.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [fastapi/typer](https://github.com/fastapi/typer) |
| Linguagem | Python (≥3.10), CLI sobre Click vendido |
| Tamanho | shallow clone ~5.1 MiB; **643** arquivos `.py`; **211** `tests/test_*.py` |
| Instrumentação de IA | **Não há** `AGENTS.md`, `CLAUDE.md` nem `.cursorrules` na raiz. Existe `typer/.agents/skills/typer/SKILL.md` (skill para *consumidores* de Typer, não onboarding de contribuição). |
| Toolchain | **Parcialmente configurada.** `pip install -e ".[standard]"` + `pytest`, `ruff`, `mypy`, `pytest-xdist`, `coverage`. `uv` **não** estava no PATH (há `uv.lock`). Docs MkDocs / `prek` / suíte completa CI **não** instalados. Smoke: testes ZDOTDIR + subset completion zsh OK; `ruff check` nos arquivos tocados OK. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Fazer `install_zsh` respeitar `ZDOTDIR` (quando válido) ao gravar `.zshrc`, em vez de sempre usar `$HOME/.zshrc`. |
| Por quê | Bug real reportado; patch pequeno com testes unitários isolados (`tmp_path`), diferente da flakiness de `TMPDIR` da rodada 1. |
| Link | https://github.com/fastapi/typer/discussions/1757 |
| Completa? | **Sim**, localmente: lógica em `typer/_completion_shared.py` + 2 testes em `tests/test_completion/test_completion_install.py`. |
| Checks nativos | `pytest` dos 2 testes novos → PASS; `ruff check` nos tocados → PASS; subset `tests/test_completion/` (zsh/install) → 11 passed, 1 skipped. **Sem** push/PR/issue no upstream. |
| Rodada 1 (comparação) | Flaky `tests/test_types_file.py::test_file_error` com `TMPDIR` longo (discussion #1984). |

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Clone parcial em `/tmp/target` + `TB_GRAPH=on tb graph rebuild` → **626 nós / 1492 arestas** (rodada 1: 0 nós). |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | `tests/hidden` inexistente; `tb verify run --json` → `hidden_available: true`, `hidden_passed: true` com `hidden_test_cmd: pytest …test_completion_source_zsh`. |
| 3 | guard `--strict --json` exit ≠0 | **funcionou** | Com violação em `typer/colors.py`: sem `--strict` exit 0; com `--strict --json` **exit 1** + JSON de violations. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `typer/colors.py`; diff tocando o arquivo → `protected_path` violation. |
| 5 | init detecção de toolchain | **funcionou** (com ressalva) | `tb init` em typer → `verify.test_cmd: uv run pytest` (detectou `uv.lock`/`pyproject`). **Ressalva:** `uv` ausente no PATH → default quebrado até ajuste manual. |
| 6 | review `tests/hidden` por path | **funcionou** | Comentário `# probe tests/hidden…` só no diff → `findings: []`. Toque real em `tests/hidden/secret_test.py` → finding **critical** da invariante. |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Sem `--agent`/`--fake`, `bench run` usou `echo-agent-SHOULD-BE-USED` (`agent_output`: `not found`, exit 127). `bench compare --agent echo-compare-agent …` rodou A/B. |
| 8 | review/drift `## Scope`/`## Escopo`, listas, `**` | **funcionou** | Ticket com `## Escopo` + bullets + `tests/test_completion/**/*.py` manteve arquivos no escopo; ticket só com `typer/main.py` → drift; `tests/**/*.py` + `typer/**/*.py` manteve código, flagrou `rd.md`. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | JSON: `build_ran/passed: true`, `lint_ran/passed: true` antes dos testes. |
| 10 | init lê AGENTS/CLAUDE e `--write-agents` | **funcionou** | Sem arquivos: log “nenhum AGENTS.md/CLAUDE.md…”. Com AGENTS.md sintético: `agent_docs.yaml` + hints. `--write-agents` semeia AGENTS.md; não sobrescreve existente. |
| 11 | graph Python `calls` | **funcionou** | Stats: `edges_calls: 3827`. `graph callers --function install_zsh` retornou testes + arquivo; `impact --file typer/_completion_shared.py` não-vazio. |
| 12 | verify visual skipped + docs CLI | **funcionou** | `--ruler visual` → `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. `docs/cli.md` e `docs/verificacao-visual.md` descrevem agregação top-level. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; detecta `uv run pytest`; avisa ausência de AGENTS | misto | WALL 0.005 s; default depende de `uv` ausente |
| `tb init --write-agents` | Semeia AGENTS.md mínimo | ajudou | Probe em `/tmp/typer-agents-probe` |
| `tb mem init/add/list/check` | OK; 3 defaults + 1 regra | ajudou (leve) | `mem check` retenção 1.00 |
| `tb graph rebuild/stats/search/callers/impact` | 3054 nós / 9821 arestas; callers/impact úteis | **ajudou** | Contraste forte com rodada 1 (callers/impact vazios) |
| ticket `rd.md` | Manual (`tb ticket` inexistente) | atrapalhou (leve) | Mesmo gap da r1; parser Markdown OK |
| `tb verify run` | PASS com build+lint+test+hidden | ajudou | Após config manual de cmds |
| `tb verify --ruler visual` | Skip gracioso `skipped=true` | neutro | Sem manifesto UI no Typer |
| `tb review` | Fix limpo → `findings: []`; FP substring corrigido | ajudou | Probe path vs comentário |
| `tb drift` | Escopo Markdown/`**` OK; escopo errado detecta | ajudou | EXIT 1 no ticket ruim |
| `tb guard check` | YAML + `--strict --json` corretos | ajudou | Exit 1 com violação |
| `tb bench` | Honra `default_agent`; `compare --agent` existe | misto | Precisa copiar `bench/tasks`; agent inexistente → 0% |
| `tb mcp` | stdio OK; tools version/echo/env/run/graph_* | neutro | Sem mem/verify/review/drift/guard |
| `tb version` | `0.1.0-dev`, commit/built `unknown` | neutro | Build sem ldflags |

## Problemas que continuam

| problema (rodada 1) | status na rodada 2 | evidência |
|---------------------|--------------------|-----------|
| `version` → `unknown` sem ldflags | continua | `tb version --json` |
| configs mode `0600` | continua | `stat` em `.toolbelt/config.yaml` |
| MCP sem mem/verify/review/drift/guard | continua | `tools/list` |
| `tb ticket` inexistente / ticket manual | continua | `unknown command` na r1; r2 criou `rd.md` à mão |
| Init não preenche `lint_cmd`/`build_cmd` sozinho | continua (parcial) | Só `test_cmd` auto; lint/build exigiram YAML manual |
| Bench sem tasks no alvo | continua | Precisou copiar `bench/tasks` do toolbelt |
| Fake/`doc-function` pass@1=0% sem FilesToCreate | continua | `--fake` sucesso 0.0% |
| Skill `.agents` de consumidor ignorada | continua | Init não lê `typer/.agents/skills/...` |

## Problemas novos

### P2 — `tb init` escolhe `uv run pytest` sem `uv` no PATH

- **Repro:** `tb init` num clone com `uv.lock` onde `uv` não está instalado; inspecionar `verify.test_cmd`.
- **Esperado:** fallback para `pytest` / `python -m pytest` se `uv` não existir.
- **Atual:** `test_cmd: uv run pytest` → verify default falharia (`uv: command not found`).

### P3 — Hints de AGENTS.md não viram `lint_cmd`/`build_cmd`

- **Repro:** `AGENTS.md` com “Lint with ruff…” / “Build: python -m build”; `tb init`.
- **Esperado:** pelo menos sugerir ou preencher `lint_cmd`/`build_cmd`.
- **Atual:** hints em `agent_docs.yaml`; `config.yaml` só mexe em `test_cmd`.

### P3 — `default_agent` inválido falha opaco (exit 127, pass@1=0%)

- **Repro:** `bench.default_agent: echo-agent-SHOULD-BE-USED`; `tb bench run -r 1 --task doc-function`.
- **Esperado:** erro claro “agent não encontrado” / exit ≠0 do comando bench.
- **Atual:** bench exit 0, sucesso 0%, `agent_output` com `not found` no JSONL.

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift quando o escopo do ticket não cobria os arquivos alterados.
- Guard em path customizado via `guard.yaml` + exit code com `--strict --json`.
- Review: invariante `tests/hidden` só no path tocado; patch ZDOTDIR sem findings.
- Grafo Python: localizou `install_zsh`, callers dos novos testes, impact do módulo.
- Verify com build/lint/test/hidden configurados: PASS alinhado ao pytest/ruff nativos.

**Deixou passar / não cobriu**
- Não instalou/validou presença de `uv` ao sugerir `uv run pytest`.
- Não configurou lint/build automaticamente.
- Não leu a skill em `typer/.agents/skills/`.
- MCP não expõe o fluxo de campo completo.
- Não impede “tarefa incompleta” (ticket/progress) sozinho.

**vs nativo:** pytest+ruff confirmaram o fix; `tb verify` espelhou após config manual. Default pós-`init` (`uv run pytest`) seria pior que o `pytest` direto neste VM.

## Interação com instrumentação existente / onboarding

- Grupo **sem** instrumentação de contribuição: sem AGENTS/CLAUDE na raiz.
- Skill de consumidor em `typer/.agents/...` permanece irrelevante para o harness.
- `tb init` agora: detecta Python/uv, avisa ausência de AGENTS, opcional `--write-agents`.
- Ainda é preciso montar ticket `rd.md`, ajustar `lint_cmd`/`build_cmd`/`hidden_test_cmd`, e (neste ambiente) trocar `uv run` por `pytest`.
- Melhor que a rodada 1, mas bootstrap ainda não é “zero-touch” para Python sem `uv`.

## Comparação com a rodada 1

| Aspecto | Rodada 1 | Rodada 2 | Veredito |
|---------|----------|----------|----------|
| SkipDirs `/tmp/target` | 0 nós | 626 nós | **melhor** |
| Python `calls` / callers | vazios p/ símbolo do patch | 3827 calls; callers de `install_zsh` OK | **melhor** |
| hidden sem dir | `hidden_available: false` | `true` + comando rodou | **melhor** |
| guard.yaml / `--strict --json` | ignorado / exit 0 | lê YAML / exit 1 | **melhor** |
| init toolchain | `go test ./...` | `uv run pytest` | **melhor** (com P2 uv) |
| review FP `tests/hidden` | critical em comentário | só em path | **melhor** |
| bench default_agent / compare `--agent` | obrigava flags; compare sem `--agent` | honra config; flag existe | **melhor** |
| Scope Markdown / `**` | frágil / parcial | `## Escopo` + `**` OK | **melhor** |
| build/lint no verify | inexistentes | rodam quando configurados | **melhor** |
| AGENTS / `--write-agents` | não lia/gerava | lê + semeia | **melhor** |
| visual skipped agregação | ambíguo em alguns relatos | `skipped: true` + docs alinhados | **melhor** |
| MCP cobertura | só graph/version/… | igual | **igual** |
| Completar tarefa sozinho | fraco | ainda fraco | **igual** |

No conjunto: **claramente melhor** nas 12 correções; restam onboarding fino (uv), MCP e harness de completude.

## Sugestões de melhoria

1. **[P2][init]** Ao detectar `uv.lock`, checar se `uv` está no PATH; senão fallback `pytest` / `python -m pytest`.
2. **[P2][init]** Preencher ou sugerir `lint_cmd`/`build_cmd` a partir de `pyproject`/hints (ex. `ruff check`).
3. **[P2][mcp]** Expor tools de mem/verify/review/drift/guard.
4. **[P3][bench]** Falhar o comando (exit ≠0) se o binário de `default_agent` não existir.
5. **[P3][init]** Opcionalmente ler skills sob `.agents/` / `**/SKILL.md`, não só AGENTS/CLAUDE na raiz.
6. **[P3][cli]** Template `tb ticket init` gerando `rd.md` com `## Escopo`.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time somado dos cmds logados | ~8.1 s (37 invocações com timer) + ~6.3 s pytest nativo extra | ~8.9 s (79 invocações) |
| Wall time aproximado da sessão | ~15–25 min (build, clone, toolchain, fix ZDOTDIR, probes das 12 correções) | ~15–20 min |
| Invocações `tb` logadas | 37 (+ probes init auxiliares) | 79 |
| Saídas ≠0 | 5 intencionais (review drift, drift bad, guard strict, bench sem tasks) | 20 |
| Falhas inesperadas relevantes | default `uv run` sem `uv`; bench agent 127 | grafo `/tmp/target` vazio; hidden off; etc. |
| MCP | smoke OK | smoke OK |
| Commits no alvo | 1 local (`fix: respect ZDOTDIR…`); **sem** push/PR/issue no fastapi/typer | 1 local; sem push |

