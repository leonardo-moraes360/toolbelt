🇧🇷 Português | [🇺🇸 English](../../en/campo/rodada2/click.md)

# Estudo de campo (rodada 2): pallets/click

Grupo: **without existing AI-agent instrumentation**.  
Binário `tb`: `go build -o /tmp/tb ./cmd/tb` a partir de `main` (`a46c295` local após pull).  
Clone alvo: `/tmp/click-campo2` (shallow de https://github.com/pallets/click).  
Nenhuma interação com o remoto do Click (sem push/issue/PR/comentário).  
Evidências: `/tmp/tb-campo2-click-evidence.log`.

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | pallets/click |
| Linguagem | Python (≥3.11), pacote `click` |
| Tamanho | shallow ~12M sem `.venv`; ~208M com deps; 76 `.py` em `src/`+`tests/`; ~12.8k LOC em `src/` |
| Instrumentação AI existente | Nenhuma (`AGENTS.md`/`CLAUDE.md`/`.cursorrules`/copilot/aider ausentes no clone) |
| Toolchain | **Sim, parcial**: `uv` + venv, `uv sync` (groups `tests`/`typing`/`dev`), `pytest` 9.0.2, `ruff` 0.15.9, `mypy` 1.20.0; instalei `less` (faltava). **Não configurei**: Sphinx/docs, tox matrix completa, pre-commit hooks em CI local, pyright suite completa. Baseline: `2626 passed, 24 skipped, 1 xfailed`; após o patch: `2627 passed`. |

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Evitar open/close eager de `_LazyFile` em FIFOs (`stat.S_ISFIFO`) para não consumir o writer antes do open lazy |
| Por quê | Bug pequeno, reprodutível em Linux, um arquivo de produção + um teste; diferente da rodada 1 |
| Issue | https://github.com/pallets/click/issues/2645 |
| Completou? | **Sim, localmente** (commit só em `/tmp/click-campo2`; não enviado ao upstream) |
| Checks nativos | `pytest tests/test_utils/test_LazyFile.py`: 2 passed; suite: **2627 passed**; `ruff check` nos tocados: OK; `mypy src/click/utils.py`: Success |
| Rodada 1 (comparação) | `get_error_hint` + `auto_envvar_prefix` / issue [#3914](https://github.com/pallets/click/issues/3914) |

Diff essencial: em `src/click/utils.py`, `_LazyFile.__init__` só faz `open(...).close()` early se o path **não** for FIFO; teste `test_lazyfile_skips_eager_open_for_fifo` em `tests/test_utils/test_LazyFile.py`. Ticket `rd.md` com `## Scope` / listas Markdown e globs.

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Dir real `/tmp/target` com `src/`+`tests/`: `1456 nós`. Em rodada 1, `/tmp/target` → 0 nós. **Nuance**: `ln -s /tmp/click-campo2 /tmp/target` → 0 nós (`filepath.Walk` não desce symlink de raiz) — não é regressão do SkipDirs |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir; `hidden_test_cmd` no YAML → `hidden_available: true`, `hidden_passed: true`. Com `--hidden-test-cmd false` → `hidden_passed: false`, verdict `ESCALATE` |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Com `protected_paths: [src/click/utils.py]` e diff no arquivo: JSON com violation e `EXIT_STRICT:1`; sem `--strict` exit 0 |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | Mesmo manifesto: `type=protected_path`, `file=src/click/utils.py`. Sem YAML custom, só defaults; `core.py` no YAML bloqueou probe |
| 5 | init detecção de toolchain | **funcionou** | Em Click (com `uv.lock`): `verify.test_cmd: uv run pytest`. Fixture só `pyproject.toml`: `pytest`. Não mais `go test ./...` |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket cita `tests/hidden/**` em Forbidden; review limpa: `findings: []` (rodada 1: FP critical por substring) |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `default_agent: echo fake-agent` + `tb bench run -r 1` sem `--agent` → OK. Sem default → exit 1 exigindo flag. `compare --config-a/--config-b --agent 'echo hi'` → OK (~1.1s) |
| 8 | review/drift parse `## Scope`/`## Escopo`, listas, `**` | **funcionou** | `## Scope` + backticks: drift sem `out_of_scope`. `## Escopo` + `src/click/**/*.py` / `tests/**/*.py`: só `rd.md` fora (esperado) |
| 9 | verify `build_cmd` / `lint_cmd` | **funcionou** (via YAML) | PASS com `build_ran/lint_ran/passed: true`. Flags CLI `--build-cmd`/`--lint-cmd` **não existem** (só config) |
| 10 | init lê AGENTS/CLAUDE e `--write-agents` | **funcionou** (parcial hints) | `--write-agents` criou `AGENTS.md` + `agent_docs.yaml`. Fixture com `CLAUDE.md`: `sources: [CLAUDE.md]`; `hints: []` apesar de texto “use pytest” |
| 11 | graph Python `calls` | **funcionou** | Rebuild: `1956 nós / 5584 arestas`, `edges_calls: 2815`. `callers --function open_stream` e `format_filename` não vazios; `impact --file src/click/utils.py` listou tipos/testes |
| 12 | visual skipped agregado + docs CLI | **funcionou** / **parcial docs** | `--ruler visual` sem manifesto: `skipped: true`, `skip_reason: manifesto de telas ausente`. `docs/cli.md` alinhado em mcp/mem/guard/bench; `tb ticket` permanece como “planejado Fase 3+” |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK `0.1.0-dev`, commit/built `unknown` | neutro | Sem ldflags; ~0.006s |
| `tb init --write-agents` | `.toolbelt/` + `AGENTS.md` + `uv run pytest` | **ajudou** | Toolchain certa; mode `600` nos configs |
| `tb mem init/list/add/check` | OK; 4 invariantes; retenção 1.00 | ajudou | `mem add --type` ainda rejeitado (docs já não vendem como implementado) |
| Ticket / Scope | Manual `rd.md`; `tb ticket` inexistente | neutro | Docs marcam Fase 3+; parser Markdown funcionou |
| `tb graph rebuild .` | 1956/5584; calls presentes | **ajudou** | Em `/tmp/click-campo2` (~0.21s). Search achou `_LazyFile` |
| `tb graph` em dir `target` | real: nós>0; symlink: 0 | ajudou / nuance | SkipDirs raiz corrigido; symlink raiz ainda cego |
| `tb graph impact/callers` | impact útil; callers por símbolo misto | ajudou | `_LazyFile` callers vazio; `open_stream`/`format_filename` OK |
| `tb verify run` | PASS com build+lint+visible+hidden | **ajudou** | Após não proteger `utils.py` no guard.yaml da tarefa |
| `tb verify --ruler visual` | PASS `skipped=true` | neutro | Sem manifesto de telas |
| `tb guard check A..B --strict --json` | exit 1 com violation | **ajudou** | Sintaxe é range, não `--base` |
| `tb drift` / `tb review` | Scope OK; review sem FP | **ajudou** | `--base HEAD^` literal → falso `base_mismatch`; SHA completo OK |
| `tb bench run` / `compare --agent` | default_agent e `--agent` OK | ajudou parcial | Precisou copiar `bench/tasks` do toolbelt; init não semeia tasks |
| `tb mcp` | tools/list + `tb_version` + search OK | ajudou parcial | Sem `TB_GRAPH=on`, `tb_graph_search` `isError` |
| Checks nativos vs tb | suite/ruff/mypy OK | — | tb cobriu build/lint/test configurados; não substituiu mypy |

## Problemas que continuam

| problema (rodada 1 / síntese) | status nesta rodada | evidência |
|------------------------------|---------------------|-----------|
| `version` → `unknown` sem ldflags | continua | `commit: unknown`, `built: unknown` |
| Config mode `0600` | continua | `stat`: `600` em `config.yaml` e `AGENTS.md` |
| MCP graph exige `TB_GRAPH=on` | continua (por desenho ADR-011) | sem env → isError |
| Sem `tb ticket` | continua (agora documentado) | `docs/cli.md` “Não implementado” |
| Init não cria `bench/tasks` | continua | `bench list` falha até copiar fixtures |
| Review/guard ainda exigem operador disciplinado | parcial | Guard.yaml protegendo o arquivo da tarefa aborta verify cedo (anti-cheat correto, mas atrapalha o fluxo se o manifesto for ingenioso demais) |

## Problemas novos

1. **P2 — `filepath.Walk` em raiz symlink → grafo 0 nós**  
   - Repro: `ln -s /tmp/click-campo2 /tmp/target && cd /tmp/target && TB_GRAPH=on tb graph --enable rebuild .` → `0 nós`. Dir real `/tmp/target` com fontes → >0.  
   - Esperado: documentar ou resolver symlink da raiz antes do walk.  
   - Atual: silêncio com índice vazio.

2. **P2 — `tb drift --base HEAD^` compara string, não o SHA resolvido**  
   - Repro: ticket com `base_commit: <sha>` + `--base HEAD^` → `base_mismatch: true` (`actual_base: "HEAD^"`).  
   - Esperado: resolver ref git antes de comparar.  
   - Atual: mismatch falso; com SHA expandido, OK.

3. **P3 — `init` lê `CLAUDE.md` mas não extrai hints de comandos**  
   - Repro: `CLAUDE.md` com “use pytest -q” → `hints: []`.  
   - Esperado: dicas de test/lint quando o texto é óbvio, ou documentar limites.  

4. **P3 — `build_cmd`/`lint_cmd` só no YAML, sem flags CLI**  
   - Repro: `tb verify run --build-cmd …` → `unknown flag`.  
   - Esperado: flags espelhando `test_cmd`/`hidden-test-cmd`, ou docs explícitas de que só YAML vale.

5. **P3 — callers pelo nome da classe `_LazyFile` vazio**  
   - Repro: `graph callers --function _LazyFile` → `(vazio)` apesar de `calls` no índice e instancias no código/testes.  
   - Esperado: resolver construtor/`Class(` ou mensagem de cobertura.

## O que o tb pegou e o que deixou passar

**Pegou**
- Path protegido via `guard.yaml` (`utils.py` / `core.py`) e exit code `--strict --json`.
- Drift/review com `## Scope` + globs `**`; FP de `tests/hidden` no ticket **não** voltou.
- Verify rodando build + lint + visible + hidden sem pasta `tests/hidden/`.
- Graph Python com milhares de `calls`; impact em `utils.py` apontou consumidores.
- Init com `uv run pytest` e seed de `AGENTS.md`.
- Bench com `default_agent` e `compare --agent`.

**Deixou passar / não ajudou**
- Não guiou a escolha da tarefa nem a escrita do ticket (`tb ticket` ausente).
- Grafo mudo se o cwd for symlink chamado `target`.
- Falso `base_mismatch` com refs relativas (`HEAD^`).
- Sem mypy/typecheck no pipeline padrão (só o que configurei em lint/build).
- `hints` de CLAUDE.md vazios.

## Interação com instrumentação existente / onboarding

Repo **sem** instrumentação prévia. Com `--write-agents`:

| Artefato | Qualidade |
|----------|-----------|
| `.toolbelt/config.yaml` | `test_cmd: uv run pytest` correto; `build_cmd`/`lint_cmd`/`hidden`/`default_agent` precisei preencher à mão |
| `AGENTS.md` | Esqueleto útil apontando verify/guard/mem |
| `agent_docs.yaml` | Registrou `AGENTS.md`; hints vazios |
| `bench/` | dirs vazios; tasks **não** criadas |
| `guard.yaml` | Não gerado pelo init; criei manualmente e foi honrado |

Onboarding bem melhor que a rodada 1 (não mais `go test` default), mas ainda falta seed de ticket/bench e extração de hints.

## Comparação com a rodada 1

| Aspecto | Rodada 1 | Rodada 2 | Veredito |
|---------|----------|----------|----------|
| Init toolchain | `go test ./...` | `uv run pytest` | **melhor** |
| Graph em `/tmp/target` | 0 nós | dir real OK; symlink 0 | **melhor** (com nuance) |
| Python `calls` / impact | vazios | 2815 calls; impact útil | **melhor** |
| guard.yaml | ignorado | lido + strict JSON | **melhor** |
| Review FP `tests/hidden` | critical FP | findings vazios | **melhor** |
| Scope Markdown / `**` | frágil | `## Scope`/`## Escopo` OK | **melhor** |
| hidden sem dir | não rodava | roda / falha de verdade | **melhor** |
| build/lint no verify | inerte | honrados no YAML | **melhor** |
| bench default_agent / compare `--agent` | quebrado | funciona | **melhor** |
| AGENTS.md | nem lia nem gerava | `--write-agents` + leitura | **melhor** |
| `tb ticket` / version unknown / 0600 / TB_GRAPH | dor | igual | **igual** |
| Completar a tarefa | apesar do tb | tb ajudou mais (verify/graph/guard) | **melhor** |

## Sugestões de melhoria

1. **[P2][graph]** Resolver `EvalSymlinks` na raiz antes do walk (ou avisar se a raiz é symlink).  
2. **[P2][drift]** Resolver refs git (`HEAD^`, tags) antes de comparar com `base_commit`.  
3. **[P2][init]** Semear `bench/tasks` mínimo ou falhar com mensagem “copie fixtures / rode no repo toolbelt”.  
4. **[P3][init]** Extrair hints de comandos de `AGENTS.md`/`CLAUDE.md` ou documentar que só paths são registrados.  
5. **[P3][verify]** Flags `--build-cmd` / `--lint-cmd` espelhando o YAML.  
6. **[P3][graph]** Melhorar resolução de callers para classes / `__init__` Python.  
7. **[P3][init]** Template `rd.md` com `## Scope` para fechar o loop ticket→drift→review.

## Métricas

| Métrica | Rodada 2 | Rodada 1 (click) |
|---------|----------|------------------|
| Wall time sessão (clone→relatório) | ~ordem de dezenas de minutos | similar |
| Seções de exercício instrumentadas | 41 | — |
| Invocações `tb` (aprox., sessão) | ~55–65 (init/mem/graph/verify/guard/drift/review/bench/mcp + retests) | 48 |
| Falhas exit ≠ 0 (várias intencionais) | ~13 marcadores + erros de flag/docs | 19 |
| Soma durações cronometradas no log | ~1.8s parciais + verify/pytest ~segundos + bench compare ~1.4s | ~7.3s |
| Mais lentos | `bench compare --agent` ~1.1s; suite nativa pytest ~3.7–5s; `graph rebuild` ~0.21s | bench compare fake ~5.1s |
| Falhas de produto relevantes restantes | symlink-raiz→0 nós; `HEAD^` base_mismatch; hints vazios; sem flags build/lint | SkipDirs; guard.yaml morto; init Go; review FP; sem calls |

Artefatos locais (não no toolbelt): `/tmp/tb-campo2-click-evidence.log`, clone `/tmp/click-campo2` com commit experimental `a3160c8` (não publicado upstream).
