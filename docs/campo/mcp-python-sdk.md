# Estudo de campo: modelcontextprotocol/python-sdk

Estudo realizado em 2026-11-15 com `tb` buildado localmente deste repositório (`make build` → `toolbelt c8aae04`). O alvo foi usado só em clone local (`/tmp/target` e cópia `/tmp/python-sdk`); não houve push, issue, PR nem comentário no upstream.

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | [modelcontextprotocol/python-sdk](https://github.com/modelcontextprotocol/python-sdk) |
| Linguagem | Python 3.11+ (SDK MCP; pacote `mcp`) |
| Tamanho | ~22 MiB working tree sem `.venv` / ~292 MiB com `.venv`; shallow clone ~25 MiB antes do sync; ~419 `.py` em `src/`+`tests/`, ~3673 `.py` no tree completo pós-deps |
| Commit base do clone | `77001c9` (`main` shallow) |
| Instrumentação AI existente | `AGENTS.md` (166 linhas, guidelines obrigatórias), `CLAUDE.md` (`@AGENTS.md`), `.claude/commands/review-pr.md`, `.claude/skills/test-quality/`, workflow `.github/workflows/claude.yml` |
| Toolchain | **Sim, parcial.** Instalado `uv 0.13.8`; `uv sync --frozen` OK com Python 3.12 do sistema. Rodados: `pytest` (stdio client + interaction), `ruff check` nos arquivos tocados. **Não rodados:** `./scripts/test` completo (~cobertura 100% + matrix), `pyright` full, docs (`zensical`), pre-commit completo, Windows/3.11–3.15. Motivo: tempo/escopo do estudo; suite completa é cara e o AGENTS.md já indica ~23 s só no script de cobertura default. |

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Default de `StdioServerParameters.encoding_error_handler`: `"strict"` → `"replace"`, alinhado ao servidor (PR #2302), + testes de regressão em `tests/client/test_stdio.py` |
| Por quê | Bug P2 `ready for work` com repro claro; mudança mínima, testável, plausível para maintainer; ainda aberto em `main` no momento do estudo |
| Issue | https://github.com/modelcontextprotocol/python-sdk/issues/2454 |
| Completada? | **Sim, localmente** (branch local `campo-local-2454`, commit `960549b`). Não enviada ao upstream (proibido pelas regras do estudo). |
| Native test/lint/build | `uv run --frozen pytest tests/client/test_stdio.py -q` → **37 passed**; `tests/interaction/transports/test_stdio.py` → **2 passed**; `ruff check` nos arquivos alterados → OK. Build/typecheck global e `./scripts/test` **não** executados. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; criou `.toolbelt/` + `config.yaml` (mode **0600**); defaults de `verify` = `go test ./...` | atrapalhou | Wall 0.008 s. Defaults Go num repo Python; agente precisa reescrever `verify.test_cmd` à mão. Config 0600 confirma issue conhecido. |
| `tb version` | Com `make build`: `c8aae04`. Com `go build` sem ldflags: `0.1.0-dev` / commit **unknown** | neutro | Ambos medidos; “unknown” só no binário sem `-ldflags`. |
| `tb mem init/list/add/check` | OK; 5 invariantes; `mem check` sha256 + retenção 1.00 | ajudou (parcial) | Positional `tb mem add "<regra>"` funciona. Docs em `docs/cli.md` descrevem `--type/--title` inexistentes → atrito. `invariants.yaml` também 0600. |
| `tb graph rebuild/index` em `/tmp/target` | **0 nós / 0 arestas** em ~0.006 s | atrapalhou | Diretório raiz chamado `target` está em `SkipDirs`; `filepath.Walk` dá `SkipDir` na própria raiz. |
| `tb graph rebuild` em `/tmp/python-sdk` | **16550 nós, 43746 arestas**, 1.345 s | ajudou | Mesmo código; só o basename do cwd muda. |
| `tb graph search encoding_error_handler` | vazio mesmo com grafo cheio | atrapalhou | Busca só em title/path/package/id; não indexa campos/atributos. |
| `tb graph impact --file src/mcp/client/stdio.py` | vazio | neutro/atrapalhou | Impacto pouco útil aqui (poucas arestas `calls`: 100 no repo inteiro). |
| `tb graph callers --function stdio_client` | vazio | atrapalhou | Indexador Python extrai poucas calls. |
| `tb graph deps --file src/mcp/client/stdio.py` | lista imports (`anyio`, `mcp_types`, …) | ajudou | Único subcomando de grafo claramente útil nesta tarefa. |
| `tb graph impact` sem `--file` | EXIT 1, “--file é obrigatório” | neutro | Confirma issue conhecido de UX. |
| Ticket `rd.md` + Scope | YAML com `---` funciona; lista markdown sem frontmatter **não** parseia scope multilinha | atrapalhou | Sem `---`, drift marcou `tests/client/test_stdio.py` fora do scope mesmo listado. Com frontmatter: drift limpo. |
| `tb verify run` (pytest stdio) | PASS iter=1; **Oculto: disponível=false** | ajudou (parcial) | Detecta presença de `tests/hidden/` por path, não pelo `hidden_test_cmd` configurado. Só rodou suíte visível. |
| `tb verify` + erro de sintaxe em `examples/...` | PASS | atrapalhou | Native `py_compile` falhou; verify configurado só com pytest pontual passou. Confirma “checks parciais”. |
| `tb verify --ruler visual` manifesto ausente + `--skip-unavailable` | PASS, `"skipped": true` | neutro | Correto. |
| `tb verify --ruler visual` com `screens.yaml` vazio + skip | PASS, **`"skipped": false`**, `screens: null` | atrapalhou | Confirma issue do ruler visual. |
| `tb review` (ticket frontmatter correto) | PASS, findings=[] | ajudou | Wall ~0.008 s; fresh_context true. |
| `tb review` + touch em `tests/hidden/` | FAIL critical invariant | atrapalhou | Falso positivo para quem cria a suíte oculta (invariante default do `mem init`). |
| `tb drift` paths literais + frontmatter | OK | ajudou | |
| `tb drift` glob `src/**/*.py` | FAIL out_of_scope | atrapalhou | `filepath.Match` não trata `**`. Glob simples `src/mcp/client/*.py` **funcionou**. |
| `tb guard check` no fix | OK, violations null | ajudou | |
| `tb guard` + `.toolbelt/guard.yaml` deny | **ignorou** o YAML; ainda PASS | atrapalhou | Confirmado: guard.yaml não é lido. |
| `tb guard` + `.toolbelt/thresholds.yaml` no diff | violation `protected_path`; `verify` FAIL iterações 0 | atrapalhou | Path hardcoded aborta o fluxo de verify. |
| `tb bench run --fake` | OK com `-t/-f` apontando para fixtures do toolbelt | neutro | Repo alvo não tem `bench/tasks`; default_agent no config **não** dispensa `--agent/--fake`. |
| `tb bench compare` | OK com `--fake`; **sem** flag `--agent` | neutro | Help confirma ausência de `--agent` no compare. |
| `tb mcp` | tools/list + tools/call `tb_version` OK; `tb_graph_search` → `null` em `/tmp/target` | ajudou (parcial) | MCP sobe; grafo MCP herda o bug do `SkipDirs`/`target`. |
| Onboarding / AGENTS.md | `tb init` **não** lê nem importa `AGENTS.md` / `.claude/` | atrapalhou | Arquitetura marca AGENTS.md como “pendente”. |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare --help` sem `--agent`; só `--fake` / JSONL. |
| `guard.yaml` ignored | **confirmado** | Deny em `.toolbelt/guard.yaml` para `stdio.py`; `guard check` → `violations: null`. |
| Outdated CLI docs | **confirmado** | `docs/cli.md` documenta `tb ticket`, `tb mem add --type/--title`; CLI real: sem `ticket`; `mem add <regra> --category`. |
| Version shows `unknown` | **confirmado (parcial)** | Sem ldflags: `commit: unknown` / `0.1.0-dev`. Com `make build`: commit do git. |
| Config files created 0600 | **confirmado** | `stat` → `config.yaml` e `invariants.yaml` mode 600; `graph.json` também 600. |
| Visual ruler reports `skipped=false` when skipping | **confirmado** | Manifesto vazio + `--skip-unavailable` → `"skipped": false`, `screens: null`. Ausência de manifesto → `skipped: true` (OK). |
| `default_agent` ignored | **confirmado** | Config com `default_agent: cursor-agent ...`; `tb bench run` sem flags → `--agent ou --fake é obrigatório`. |
| Guard aborts verify when diff touches hardcoded config path | **confirmado** | Diff só com `.toolbelt/thresholds.yaml` → verify FAIL, iterações 0. |
| Default fake agent needs `--fake-config` | **refutado (no caminho happy)** | `tb bench run --fake` sem `--fake-config` passou (100%). Flag existe para customizar; não é obrigatória. |
| `mem add` positional | **confirmado como gap docs/UX** | Positional funciona; docs sugerem flags que não existem (`--type`, `--title`, `--rule` → unknown flag). |
| Drift/Scope need literal paths (no globs) | **parcialmente confirmado** | `*` num segmento funciona; `**` não. Scope multilinha sem frontmatter YAML falha. |
| `graph impact` needs `--file` | **confirmado** | Sem `--file` → EXIT 1. |
| Review false positives on tests/hidden invariant | **confirmado** | Adicionar `tests/hidden/test_x.py` no scope → finding critical da invariante default. |
| Verify runs only partial checks / misses build errors | **confirmado** | Syntax error em `examples/mcpserver/unicode_example.py`: native `py_compile` falha; `tb verify` com pytest pontual → PASS. Hidden suite “indisponível” mesmo com `hidden_test_cmd` setado se não existe dir `tests/hidden`. |
| tb does not push agent toward completing whole task | **confirmado (qualitativo neste estudo)** | Init defaults Go; não importa AGENTS.md; verify não exige lint/typecheck/docs do projeto; ticket CLI ausente; grafo vazio se cwd=`target`. Agente pode “fechar” o loop tb com PASS e ainda omitir checks nativos do AGENTS.md (`pyright`, `./scripts/test`, encoding rules, etc.). |

## Problemas novos

### P0 — `SkipDirs` engole a raiz se o basename for `target`/`build`/`dist`/`bin`/…

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb graph rebuild --enable` → `0 nós`. Copiar para `/tmp/python-sdk` → 16550 nós.
- **Expected:** nunca pular o root do walk; comparar paths relativos ou só filhos.
- **Actual:** `skipDirs[info.Name()]` no próprio root → `filepath.SkipDir` imediato.

### P1 — `tb init` defaults Go + não importa instrumentação do repo

- **Repro:** `tb init` em python-sdk → `verify.test_cmd: go test ./...`; vault vazio; AGENTS.md ignorado.
- **Expected:** detectar ecossistema (pyproject/uv) e/ou importar AGENTS.md/.claude skills como contexto inicial.
- **Actual:** template genérico; onboarding não aponta o agente aos checks reais.

### P1 — Scope markdown multilinha sem frontmatter é silenciosamente incompleto

- **Repro:** `rd.md` com `scope:\n  - a\n  - b` sem `---`; drift acusa falso positivo.
- **Expected:** parse YAML da lista ou erro explícito “scope ilegível”.
- **Actual:** regex só lê `scope: ...` na mesma linha; lista ignorada.

### P2 — `graph search` não encontra símbolos de campo / `callers` quase vazio em Python

- **Repro:** após indexar 16k nós, `search encoding_error_handler` e `callers stdio_client` → vazio; só 100 arestas `calls` no SDK inteiro.
- **Expected:** pelo menos hits em definições/atributos indexados; callers úteis em Python.
- **Actual:** busca superficial; call-graph Python insuficiente para navegação da tarefa.

### P2 — Visual PASS com `skipped=false` e zero screens

- **Repro:** `.toolbelt/screens.yaml` com `screens: []` + `--skip-unavailable` → JSON `skipped: false`.
- **Expected:** `skipped: true` (ou FAIL) quando nada foi comparado.
- **Actual:** PASS aparentando execução real.

### P3 — MCP `tb_graph_*` herda grafo vazio / precisa `TB_GRAPH=on` sem feedback rico

- **Repro:** em `/tmp/target`, `tools/call tb_graph_search` → `"null"`.
- **Expected:** erro explícito (“grafo vazio / root skipped / enable graph”).
- **Actual:** resultado nulo silencioso.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift/review de escopo quando o ticket está em frontmatter YAML correto.
- Guard em paths protegidos hardcoded (`thresholds.yaml`, `tests/hidden/`).
- Verify rodando o `test_cmd` configurado (pytest stdio) e falhando quando o import do pacote quebra.
- Invariantes persistentes (`mem`) e revisão fresh-context sem findings no fix limpo.
- Bench fake + compare A/B (com fixtures do próprio toolbelt).
- MCP tools/list e `tb_version`.

**Deixou passar / não cobriu**

- Erro de sintaxe em arquivo fora do `test_cmd` (example) → verify PASS.
- Suíte “oculta” configurada por comando mas marcada indisponível sem diretório `tests/hidden/`.
- Lint/typecheck/docs/`./scripts/test` exigidos pelo AGENTS.md — tb não os sugere nem os enfileira no init.
- `guard.yaml` do projeto.
- Conteúdo de AGENTS.md / skills Claude (não lidos).
- Grafo inútil enquanto o clone vive em `/tmp/target`.
- Completeness da tarefa além do que o ticket+test_cmd estreito medem (ex.: atualizar docs do SDK, conformance).

## Interação com instrumentação existente / onboarding

O python-sdk tem instrumentação AI madura (`AGENTS.md` denso, `CLAUDE.md` alias, skill `test-quality`, comando `review-pr`).

| Comportamento observado | Detalhe |
|-------------------------|---------|
| tb **ignora** AGENTS.md | Nenhum comando lido/importado; vault permanece vazio após init. |
| tb **duplica** parcialmente a ideia de invariantes | `mem init` cria anti-cheat genérico (Go-ish: `tests/hidden`, `thresholds.yaml`) que **conflita** com o modelo do próprio tb (criar hidden suite) e não reflete regras do AGENTS.md (uv-only, encoding=, coverage 100%, sem `Test*` classes, etc.). |
| tb **não substitui** o harness nativo | AGENTS.md manda `uv run --frozen`, ruff, pyright, `./scripts/test`; verify default/`test_cmd` único não cobre isso. |
| O que o tb deveria importar | (1) Ler `AGENTS.md`/`CLAUDE.md` no `init` e materializar invariantes/mandate; (2) mapear scripts de qualidade (`scripts/test`, ruff, pyright) para `verify.test_cmd` / checklist; (3) indexar `.claude/skills/*` no vault; (4) avisar conflitos (ex. invariante “nunca toque tests/hidden” vs. necessidade de criar a suíte). |

## Sugestões de melhoria

1. **[P0][graph]** Não aplicar `SkipDirs` ao root do walk; tratar `target`/`build` só como filhos.  
2. **[P0][init/verify]** Detectar stack (Go/Python/Node) e preencher `verify.*` com comandos reais; falhar init se defaults claramente errados.  
3. **[P1][mem/ctx]** Importar `AGENTS.md` no onboarding; gerar invariantes a partir dele em vez do template genérico.  
4. **[P1][drift/review]** Aceitar scope YAML multilinha sem exigir `---` **ou** erro explícito; documentar frontmatter.  
5. **[P1][verify]** Tratar `hidden_test_cmd` configurado como disponível; não depender só de `tests/hidden/` existir. Adicionar ruler opcional `lint/build`.  
6. **[P1][guard]** Carregar `.toolbelt/guard.yaml`; separar “touch thresholds” de abortar verify sem mensagem acionável.  
7. **[P1][docs]** Sincronizar `docs/cli.md` com Cobra real (remover/marcar `ticket`, corrigir `mem add`).  
8. **[P2][bench]** Honrar `bench.default_agent`; adicionar `--agent` em `compare`.  
9. **[P2][verify/visual]** Se zero screens avaliadas → `skipped=true` (ou FAIL), nunca PASS silencioso.  
10. **[P2][graph]** Melhorar index Python (atributos, callers) e mensagem quando rebuild produz 0 nós.  
11. **[P2][graph]** Suporte a globs `**` no scope (ou documentar só `path/*` e `path/**`).  
12. **[P3][config]** Revisar 0600 em monorepos compartilhados; 0644 + secrets separados.  
13. **[P3][mcp]** Erros estruturados quando grafo desabilitado/vazio.

## Métricas

| métrica | valor |
|---------|--------|
| Wall time somado dos comandos cronometrados no log | **~35,4 s** (70 amostras `WALL=`; exclui clone/`uv sync`/leitura humana) |
| Wall time aproximado da sessão de estudo (incl. sync, cópias, exploração) | **~15–25 min** de calendário |
| Invocações tb cronometradas | **~55–60** comandos `tb*` (resto = native/git) |
| Falhas (EXIT≠0) entre todas as amostras timed | **25 / 70** (inclui falhas **intencionais** de repro de bugs e checks nativos negativos) |
| Falhas tb “inesperadas” no caminho feliz do fix | poucas após ticket frontmatter correto: verify/drift/review/guard finais PASS |
| Maior WALL individual | **9,96 s** (`pytest` nativo antes do venv apontar o patch certo) |
| Rebuild de grafo útil | **1,345 s** / 16550 nós (`/tmp/python-sdk`) |
| Verify final do fix | **3,518 s**, PASS |
| MCP smoke | **<0,01 s** por request |
