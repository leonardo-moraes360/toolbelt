🇧🇷 Português | [🇺🇸 English](../en/campo/typer.md)

# Estudo de campo: fastapi/typer

Grupo: sem instrumentação de agente de IA para contribuição no próprio repositório.  
Binário: `/agent/repos/toolbelt/bin/tb` (build `make build`, versão `c8aae04`).  
Clone de trabalho: `/tmp/typer-repo` (workaround; ver P0).  
Data: 2026-11-15.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [fastapi/typer](https://github.com/fastapi/typer) |
| Linguagem | Python (≥3.10), CLI sobre Click vendido |
| Tamanho | shallow clone ~5.5–8.1 MiB; **643** arquivos `.py`; **211** `tests/test_*.py` |
| Instrumentação de IA | **Não há** `AGENTS.md`, `.cursorrules`, `CLAUDE.md` nem regras Cursor para quem contribui no repo. Existe `typer/.agents/skills/typer/SKILL.md` (skill oficial para *consumidores* escreverem apps Typer — não é onboarding de agente no codebase do Typer). Também há `scripts/agent_banner.py` / `includes/agent-banner.md` (banner de docs). |
| Toolchain | **Parcialmente configurada.** `pip install -e .` + deps de teste (`pytest`, `ruff`, `mypy`, `pytest-xdist`, …). `uv` **não** estava no PATH; sync via `uv.lock` não foi usado. Docs MkDocs / `prek` / suíte completa CI **não** foram instalados. Smoke: `pytest tests/test_types_file.py` e `tests/test_core.py` OK; `ruff check` OK nos arquivos tocados. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir flakiness de `tests/test_types_file.py::test_file_error` quando `TMPDIR` é longo (Rich quebra o path e a assert `"cannot-open.txt" in output` falha). |
| Por quê | Bug de teste real reportado em discussão; patch mínimo, só testes, aceitável por maintainer. |
| Link | https://github.com/fastapi/typer/discussions/1984 |
| Completa? | **Sim**, localmente: `monkeypatch.chdir(tmp_path)` + `Path("cannot-open.txt")` (mesmo approach do commit apontado na discussão, `2c40f62`, que não estava no shallow `main`). |
| Checks nativos | `pytest tests/test_types_file.py` → 24 passed, 2 skipped; com `TMPDIR` artificial longo → `test_file_error` PASS; `ruff check tests/test_types_file.py` PASS; `import typer` OK. **Não** abri PR/issue no upstream. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; cria `.toolbelt/` + `config.yaml` (mode **0600**); defaults Go (`verify.test_cmd: go test ./...`); **não** gera `AGENTS.md` | atrapalhou (onboarding Python) | WALL 0.007 s; config sem detecção de pytest/ruff |
| `tb version` | Com `make build`: `c8aae04`. Sem ldflags: `0.1.0-dev`, `commit: unknown`, `built: unknown` | neutro | `/tmp/tb-nolink version` |
| `tb mem init/list/add/check` | OK; 3 invariantes default + 1 adicionada; `mem add <regra>` posicional | ajudou (leve) | `mem add --rule` falha (flag inexistente); docs CLI descrevem outra API |
| `tb graph rebuild/stats/search/impact/callers` | Em `/tmp/target` (nome do dir = `target`): **0 nós** (P0). Em `/tmp/typer-repo`: 3056 nós / 6093 arestas; `search test_file_error` achou a função; `impact`/`callers format_filename` → `(vazio)` | misto | SkipDirs inclui `target`; impacto pouco útil neste patch |
| ticket `rd.md` (Scope) | Manual; `tb ticket` **não existe** | atrapalhou | `unknown command "ticket"`; ticket YAML usado por drift/review |
| `tb verify run` | PASS nos testes configurados; **não** há `lint_cmd`/`build_cmd` no código; suíte oculta só se existir dir `tests/hidden/`; toque em `.toolbelt/thresholds.yaml` → FAIL iter 0 (guard) | misto | JSON: `hidden_available` false sem o dir; visual skip PASS |
| `tb review` | Diff no escopo → findings `[]`; menção a `tests/hidden` em comentário → finding **critical** falso positivo | atrapalhou (FP) | agreement 3/3 no goldfish |
| `tb drift` | Escopo literal e glob `tests/*.py` OK; escopo errado detecta out_of_scope | ajudou | `filepath.Match` implementado |
| `tb guard check` | Sintaxe `base..head` (não `--base`); paths hardcoded; `guard.yaml` ignorado | atrapalhou | custom path do yaml → `violations: null`; thresholds → protected_path |
| `tb bench` | Sem `bench/tasks` no Typer falha; com tasks copiadas do toolbelt, `--fake` roda; `default_agent` do config **ignorado**; `compare` sem `--agent` | neutro/atrapalhou | `--agent ou --fake é obrigatório` apesar de `default_agent` setado |
| `tb mcp` | stdio OK; `tools/list` expõe version/echo/env/run/graph_* | ajudou (smoke) | initialize + tools/list em 0.007 s |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` carece de `--agent` | **confirmado** | `--help` só tem `--fake` para rodar configs A/B; sem flag de agent externo |
| `guard.yaml` ignorado | **confirmado** | `NewChecker` hardcoda `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`; path custom do yaml não gerou violation |
| docs CLI desatualizados | **confirmado** | docs: `tb mem add --type/--title`, `tb ticket`, `tb run`, `tb guard check --type…`; CLI real diverge |
| `version` mostra `unknown` | **confirmado** (sem ldflags) | `go build` puro → `commit: unknown` / `built: unknown` |
| configs criados 0600 | **confirmado** | `stat` → `600 .toolbelt/config.yaml` e `invariants.yaml` / `graph.json` |
| ruler visual reporta `skipped=false` ao pular | **refutado** (neste build) | JSON `"skipped": true`; stdout “Skip gracioso…” |
| `default_agent` ignorado | **confirmado** | config com `default_agent: echo-agent-SHOULD-BE-USED`; `tb bench run` → `--agent ou --fake é obrigatório` |
| guard aborta verify se diff toca path hardcoded | **confirmado** | thresholds no diff → verify FAIL iterações 0 |
| fake default precisa `--fake-config` | **parcial** | `--fake` roda sem a flag; sem config com `FilesToCreate`, pass@1=0% na task `api-handler` |
| `mem add` posicional | **confirmado** (docs vs CLI) | CLI: `tb mem add <regra>`; docs ainda mostram flags `--type/--title` |
| Drift/Scope precisam paths literais (sem globs) | **refutado** | `tests/*.py` manteve arquivo no escopo via `filepath.Match` |
| `graph impact` precisa `--file` | **confirmado** | sem flag → `Error: --file é obrigatório` |
| review FP na invariante tests/hidden | **confirmado** | comentário `# probe tests/hidden…` → critical |
| verify só checks parciais / miss build | **confirmado** (parcial) | só `test_cmd`/`hidden_test_cmd`; sem lint/build; hidden só com dir `tests/hidden/` |
| tb não empurra completar a tarefa inteira | **observado** | init não gera checklist/progress/AGENTS; agente precisa montar ticket e cmds na mão |

## Problemas novos

### P0 — `SkipDirs` inclui o basename `target` e zera o grafo

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb init && TB_GRAPH=on tb graph rebuild` → `0 nós, 0 arestas`. Mesmo repo em `/tmp/typer-repo` → 3056 nós.
- **Esperado:** pular só subdirs `target/` de build (ex. Rust), não a raiz do projeto se o cwd se chama `target`.
- **Atual:** `skipDirs[info.Name()]` no walk aplica-se à raiz.

### P1 — Onboarding assume Go e não detecta toolchain Python

- **Repro:** `tb init` num repo Python.
- **Esperado:** detectar `pyproject.toml` / pytest / ruff e preencher `verify.*`.
- **Atual:** `test_cmd: go test ./...`, `hidden_test_cmd: go test ./tests/hidden/...`; sem `AGENTS.md`.

### P1 — Suíte oculta do verify amarrada à existência de `tests/hidden/`

- **Repro:** `--hidden-test-cmd 'pytest …'` sem criar o diretório → `hidden_available: false` e o comando configurado **não roda**.
- **Esperado:** honrar `hidden_test_cmd` sempre que definido.
- **Atual:** `Stat(tests/hidden)` decide disponibilidade.

### P2 — `graph impact` / `callers` vazios para símbolo Python relevante ao patch

- **Repro:** após rebuild, `tb graph impact --file tests/test_types_file.py` e `callers --function format_filename` → `(vazio)`.
- **Esperado:** arestas úteis para orientar mudança de teste.
- **Atual:** search acha a função, impact/callers não ajudam neste caso.

### P2 — UX de `guard check` vs docs/flags de review

- **Repro:** `tb guard check --base HEAD~1 --head HEAD` → `unknown flag: --base`.
- **Esperado:** mesma ergonomia que drift/review **ou** docs alinhados a `base..head`.
- **Atual:** só `tb guard check <base>..<head>`.

### P3 — MCP sem ferramentas de mem/verify/review/drift/guard

- **Repro:** `tools/list` após initialize.
- **Esperado:** paridade com o fluxo de campo.
- **Atual:** só version/echo/env/run/graph_*.

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de escopo quando o ticket apontava arquivo errado.
- Guard em `.toolbelt/thresholds.yaml` (e abort de verify).
- Review “fresh” sem findings no patch legítimo.
- Grafo (após workaround de path): localizou `test_file_error`.
- Verify com `test_cmd` pytest: PASS no fix; FAIL quando `__init__.py` tinha SyntaxError (via import no conftest).

**Deixou passar / não cobriu**
- Não sugeriu nem configurou pytest/ruff sozinho.
- Não validou lint/build como stages separados.
- Ignorou `guard.yaml` customizado.
- Falso positivo de invariante `tests/hidden` em comentário de teste.
- Impact/callers não guiaram o patch.
- Não empurrrou checklist de “tarefa completa” (ticket, tests, lint, docs).

**Comparando com nativo:** pytest+ruff confirmaram o fix; o tb verify, depois de config manual, espelhou o pytest. Sem config manual, o default Go seria inútil/falso negativo neste repo.

## Interação com instrumentação existente / onboarding

- Instrumentação existente limitada a skill de *usuário* Typer (`.agents/skills/...`), irrelevante para o fluxo `tb` de contribuição.
- `tb init` **não** leu a skill, **não** gerou `AGENTS.md`, **não** detectou comandos de teste/lint/build, **não** criou ticket/progress log estilo harness.
- Memória default é genérica (anti-cheat Go-oriented: `tests/hidden`, `thresholds.yaml`).
- Conclusão: bootstrap “from nothing” é fraco para Python; o agente humano/IA precisa reescrever verify e inventar `rd.md`.

## Sugestões de melhoria

1. **[P0][graph]** Não aplicar `SkipDirs` ao diretório raiz do walk; comparar path relativo, não só `filepath.Base` da raiz.  
2. **[P0][init/verify]** Detectar ecossistema (`pyproject.toml`, `package.json`, `go.mod`) e preencher `test_cmd`/`lint`/`build`; opcionalmente emitir `AGENTS.md`.  
3. **[P1][verify]** Tratar `hidden_test_cmd` como fonte de verdade; `tests/hidden/` só como convenção default.  
4. **[P1][guard]** Carregar `.toolbelt/guard.yaml`; unificar flags com drift/review.  
5. **[P1][docs]** Sincronizar `docs/cli.md` com Cobra real (`mem`, `guard`, ausência de `ticket`/`run`).  
6. **[P2][review]** Matching de invariantes por path tocado / hunk, não substring solta no diff.  
7. **[P2][bench]** Honrar `bench.default_agent`; adicionar `--agent` em `bench compare`.  
8. **[P2][mcp]** Expor mem/verify/review/drift/guard.  
9. **[P3][init]** Template de `rd.md` + progress log para empurrar fechamento da tarefa.

## Métricas

| métrica | valor |
|---------|-------|
| Wall time dos comandos `tb`/checks logados (soma) | ~8.9 s (79 invocações com timer) |
| Wall time aproximado da sessão de campo | ~15–20 min (build, clone, toolchain, implementação, probes) |
| Invocações logadas | 79 |
| Saídas ≠0 | 20 (muitas intencionais: flags erradas, drift, probes negativos) |
| Falhas inesperadas relevantes | grafo vazio em `/tmp/target`; bench sem tasks no Typer; hidden suite silenciosamente off |
| MCP | smoke OK (1 sessão) |
| Commits no alvo | 1 local apenas (`test: keep file error path short for long TMPDIR`); **sem** push/PR/issue no fastapi/typer |
