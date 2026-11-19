[🇺🇸 English](../../campo/click.md) | 🇧🇷 Português

# Estudo de campo: pallets/click

Grupo: sem instrumentação de agente de IA.  
Binário `tb`: build local `make build` em `c8aae04` (`./bin/tb`).  
Clone alvo: `/tmp/target` (shallow de https://github.com/pallets/click).  
Nenhuma interação com o remoto do Click (sem push/issue/PR/comentário).

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | pallets/click |
| Linguagem | Python (≥3.11), pacote `click` |
| Tamanho | clone shallow ~2.9M (sem `.venv`); com deps ~9.3M; ~77 `.py` em `src/`+`tests/`; ~12.8k LOC em `src/` |
| Instrumentação AI existente | Nenhuma encontrada (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, copilot/aider ausentes) |
| Toolchain | **Sim, parcial**: `uv` + venv, `uv sync` (groups `tests`/`typing`/`dev`), `pytest` 9.0.2, `ruff` 0.15.9, `mypy` 1.20.0. Instalei `less` (faltava; 24 testes de pager falhavam). **Não configurei**: docs Sphinx, tox matrix completa, pre-commit hooks em CI local, pyright em suite completa. Baseline após `less`: `2626 passed, 24 skipped, 1 xfailed`. |

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Corrigir `Option.get_error_hint` para resolver `auto_envvar_prefix` quando `show_envvar=True` (espelhando `get_help_extra`) |
| Por quê | Bug pequeno, reprodução clara, um arquivo de produção + um teste de regressão; maintainer provavelmente aceitaria |
| Issue | https://github.com/pallets/click/issues/3914 |
| Completou? | **Sim, localmente** (commits só em `/tmp/target`; não enviado ao upstream) |
| Checks nativos | `pytest -k "show_envvar or missing_envvar"`: 17 passed; suite completa: **2627 passed**; `ruff check` nos arquivos tocados: OK após wrap E501; `mypy src/click/core.py`: exit 0 |

Diff essencial: em `src/click/core.py`, `get_error_hint` passa a derivar `MYAPP_PORT` via `ctx.auto_envvar_prefix`; teste `test_show_envvar_auto_prefix_in_error_hint` em `tests/test_options.py`. Ticket manual `rd.md` com Scope literal.

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK; versão=`c8aae04` (ldflags do Make) | neutro | `0.005s`. Sem ldflags: `0.1.0-dev`, `commit/built: unknown` |
| `tb init` | Criou `.toolbelt/` + `config.yaml` mode **600** | atrapalhou (defaults Go) | `verify.test_cmd: go test ./...` num repo Python; sem detecção de pytest/ruff; sem `AGENTS.md` |
| `tb mem init/list/add/check` | OK | ajudou (baseline anti-cheat) | `mem add <regra>` posicional funciona; flags de `docs/cli.md` (`--type`, `--title`) **falham** (`unknown flag: --type`) |
| Ticket / Scope | Manual (`rd.md`); `tb ticket` **não existe** | atrapalhou | CLI: `unknown command "ticket"`; Precisei escrever Scope/base/forbidden à mão |
| `tb graph rebuild .` em `/tmp/target` | **0 nós** | atrapalhou | SkipDirs inclui nome `target` → raiz `/tmp/target` é pulada inteira |
| `tb graph index src` | 486 nós / 662 arestas | ajudou | `search get_error_hint` achou `func:click.get_error_hint`; `0.040s` |
| `tb graph impact --file …` | vazio (com e sem `src/`) | atrapalhou | Sem `--file` → exit 1 (`--file é obrigatório`). Com arquivo indexado ainda `(vazio)` |
| `tb graph callers` | vazio para `get_error_hint` | neutro/atrapalhou | Indexação Python não montou arestas `calls` úteis aqui |
| `tb verify run` (cmds pytest) | PASS (visível OK; hidden `available=false`) | ajudou parcial | Após config manual. Default `go test` → ESCALATE inútil |
| `tb verify --ruler visual` | PASS `skipped=true` | neutro | Motivo: `manifesto de telas ausente`; `skipped` correto neste caso |
| `tb guard check` | Hardcoded paths só | atrapalhou | `.toolbelt/guard.yaml` protegendo `src/click/core.py` **ignorado**; diff do core sem violação |
| `tb guard` + verify em `thresholds.yaml` | verify FAIL / aborta | ajudou (anti-cheat) | Violação `protected_path` em `.toolbelt/thresholds.yaml` |
| `tb drift` | Detecta out-of-scope / base_mismatch | ajudou parcial | Globs `src/click/*.py`, `tests/**`, `src/**/*.py` funcionaram; ticket com globs ainda exige listar paths extras (ex. `rd.md`) |
| `tb review` | Findings críticos em review limpa | atrapalhou | FP: mencionar `tests/hidden` / `thresholds.yaml` no próprio `rd.md` do diff dispara invariantes |
| `tb bench` no Click | Falha sem `bench/tasks` | atrapalhou | No repo toolbelt, `bench compare --fake` OK (~5s); `compare --agent` → `unknown flag` |
| `tb mcp` | tools/list + `tb_version` OK | ajudou parcial | Sem `TB_GRAPH=on` no processo, `tb_graph_search` erra; com env, search/index OK |
| Checks nativos vs tb | ruff E501 só nativo | — | verify não roda lint/build além do `test_cmd` |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare … --agent "echo hi"` → `unknown flag: --agent`; help só lista `--fake` |
| `guard.yaml` ignored | **confirmado** | Manifesto protegendo `src/click/core.py`; `guard check` no commit do fix → `violations: null`. Código: paths hardcoded em `NewChecker` |
| Outdated CLI docs | **confirmado** | `docs/cli.md`: `tb serve`, `mem add --type/--title`, `graph index --path`; real: `tb mcp`, `mem add <regra>`, `graph index [path]` |
| version shows `unknown` | **confirmado (parcial)** | Sem `-ldflags`: `commit: unknown`, `built: unknown`. Com Make: versão=short SHA, não semver |
| config files created 0600 | **confirmado** | `stat`: `600 .toolbelt/config.yaml` (também graph/invariants) |
| visual ruler `skipped=false` when skipping | **refutado** (neste cenário) | `--ruler visual` sem manifesto → `"skipped": true` |
| `default_agent` ignored | **confirmado** | `bench.default_agent: echo-agent-placeholder` no config; `tb bench run` sem `--agent` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | Commit em `.toolbelt/thresholds.yaml` → verify FAIL, `iterations: 0`, `protected_path` |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake --task bugfix-go` exit 0 sem `--fake-config` (no repo toolbelt) |
| `mem add` positional | **confirmado** | Help: `tb mem add <regra>`; docs antigos com flags vault |
| Drift/Scope need literal paths (no globs) | **refutado (parcial)** | `src/click/*.py`, `tests/**`, `src/**/*.py` casaram via `filepath.Match` / prefixo `/**` |
| `graph impact` needs `--file` | **confirmado** | Sem flag → exit 1 `--file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmado** | Review do fix limpo (`core.py`+`test_options.py`+`rd.md`) → critical por texto `tests/hidden` / `thresholds.yaml` **dentro do rd.md** |
| verify runs only partial checks / misses build errors | **confirmado (parcial)** | Defaults Go no Python; sem ruff/mypy/compileall. Erro de sintaxe em módulo importado pelo pytest **foi** pegado; lint E501 **não** |
| tb does not push agent toward completing whole task | **confirmado (qualitativo)** | Sem checklist/progress/AGENTS; defaults errados; review FP e graph vazio desviam; completei a tarefa **apesar** do tb, não por orientação dele |

## Problemas novos

1. **P0 — `graph` SkipDirs engole o clone se o diretório raiz se chama `target` (ou outro skip)**  
   - Repro: `cd /tmp/target && TB_GRAPH=on tb graph --enable rebuild .` → `0 nós`. Mesmo repo em `/tmp/click-repo` → `1956 nós`.  
   - Esperado: não aplicar SkipDir ao root da walk.  
   - Atual: `info.Name()=="target"` → `filepath.SkipDir` na raiz.

2. **P1 — `tb init` não detecta toolchain não-Go**  
   - Repro: `tb init` em Click.  
   - Esperado: sugerir/preencher `pytest`/`ruff` (ou perguntar).  
   - Atual: `go test ./...` e `go test ./tests/hidden/...`.

3. **P2 — `graph impact`/`callers` inúteis após index Python**  
   - Repro: indexar `src`, `impact --file src/click/core.py` ou `click/core.py` → `(vazio)`; `callers --function get_error_hint` → vazio.  
   - Esperado: arquivos/funções impactados ou mensagem clara de cobertura.  
   - Atual: silêncio `(vazio)`.

4. **P2 — Review trata menção a path proibido no ticket como violação**  
   - Repro: `rd.md` com `forbidden: tests/hidden/` commitado; review do mesmo range.  
   - Esperado: só flagrar alterações nesses paths.  
   - Atual: `suggestsViolation` faz substring no diff inteiro.

5. **P3 — MCP graph herda env do processo; CLI `--enable` não “gruda”**  
   - Repro: `tb mcp` sem `TB_GRAPH=on` → `tb_graph_search` isError; com env OK.  
   - Esperado: documentar ou aceitar enable por tool arg/config.

6. **P3 — `guard.yaml` criado pelo usuário nunca é lido (docs prometem)**  
   - Já listado como conhecido; aqui: arquivo `199B` em `.toolbelt/guard.yaml` mode 644, zero efeito.

## O que o tb pegou e o que deixou passar

**Pegou**
- Path protegido hardcoded `.toolbelt/thresholds.yaml` (bloqueou verify).
- Drift de escopo quando `rd.md` fora do Scope.
- `base_mismatch` quando `--base` ≠ `base_commit` do ticket.
- Após config manual, verify PASS na regressão `show_envvar_auto_prefix_in_error_hint`.
- Graph search achou `get_error_hint` (com index em `src`).
- Visual ruler skip gracioso sem manifesto.

**Deixou passar / não ajudou**
- Defaults de verify Go (não guiou para pytest).
- `guard.yaml` custom ignorado (não protegeu `core.py` via manifesto).
- Ruff E501 na primeira versão do patch (só `ruff` nativo).
- Review limpa com FPs de invariante → ruído, não sinal.
- Graph rebuild na pasta `target` = grafo vazio (quase “cego”).
- Impact/callers sem valor para localizar testes consumidores.
- Ausência de `tb ticket` / progresso → não empurrou aceite completo (teste+lint+docs).

## Interação com instrumentação existente / onboarding

Repo **sem** AGENTS.md / rules de agente. Onboarding do tb:

| Gerado por `tb init` / `mem init` | Qualidade |
|----------------------------------|-----------|
| `.toolbelt/config.yaml` | Genérico Go; `default_agent: ""`; graph languages OK na lista, mas verify errado |
| dirs `bench/`, `logs/`, `vault/` | Vazios / placeholders; `bench/tasks` **não** criado → `bench` quebra no alvo |
| `invariants.yaml` (via `mem init`) | 3 regras anti-cheat úteis como seed |
| **Não gera** | `AGENTS.md`, `guard.yaml` efetivo, detecção de `pytest`/`ruff`/`mypy`, ticket template, scripts `init.sh`, progress log |

Conclusão: o bootstrap **não** prepara um agente para trabalhar bem num projeto Python limpo; o operador precisa editar verify à mão e inventar `rd.md`. Faltou auto-detect de comandos de test/lint/build e um `AGENTS.md` mínimo apontando para `tb mem`/`verify`/`graph`.

## Sugestões de melhoria

1. **[P0][graph]** Não skippar o diretório raiz da indexação; tratar SkipDirs só para descendentes (`indexer.go` walk).  
2. **[P0][init/verify]** Detectar ecossistema (pyproject/pytest/ruff/go.mod/…) e preencher `verify.test_cmd` / sugerir hidden suite.  
3. **[P1][guard]** Carregar `.toolbelt/guard.yaml` ou remover a promessa da docs; hoje só hardcoded.  
4. **[P1][review]** Matching de invariantes por path alterado, não substring no diff/ticket.  
5. **[P1][docs]** Alinhar `docs/cli.md` ao CLI real (`mcp`, `mem add`, flags graph).  
6. **[P1][init]** Gerar `AGENTS.md` + template `rd.md` + nota “configure verify”.  
7. **[P2][bench]** `bench compare --agent`; honrar `default_agent`; seed mínimo de tasks ou skip claro no `init`.  
8. **[P2][graph]** Impact aceitar path com/sem prefixo `src/`; avisar quando não há arestas.  
9. **[P2][verify]** Ruler opcional de lint/build (`ruff`/`compileall`/`go build`) além do test_cmd.  
10. **[P3][config]** Reconsiderar umask 0600 vs 0644 em monorepos compartilhados; documentar.  
11. **[P3][mcp]** Propagar enable de graph via config/tool arg, não só env do host.

## Métricas

| Métrica | Valor |
|---------|--------|
| Wall time do estudo (primeiro→último log tb) | ~174 s de comandos instrumentados; sessão completa (clone+deps+fix+relatório) ~ordem de dezenas de minutos |
| Invocações `tb` registradas | 48 |
| Falhas (exit ≠ 0) entre elas | 19 (várias intencionais: drift/review/guard/docs mismatch) |
| Soma das durações dos comandos tb | ~7.3 s |
| Comandos mais lentos | `bench compare --fake` ~5.1 s; `verify` com pytest ~0.2–0.5 s; `graph index src` ~0.04 s |
| Falhas “de produto” relevantes | graph 0 nós em `/tmp/target`; review FP; guard.yaml inerte; init Go-default; compare sem `--agent`; `ticket` inexistente |

Artefatos locais (não no toolbelt): `/tmp/tb-campo-click-evidence.log`, clone `/tmp/target` com commits de experimento (incl. reverts de syntax/thresholds/hidden).
