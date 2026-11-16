# Estudo de campo rodada 2: modelcontextprotocol/python-sdk

Estudo realizado em 2026-11-16 com `tb` buildado de `main` atual (`make build` → `toolbelt a46c295`). Alvo só em clone local (`/tmp/python-sdk` e cópia `/tmp/target`); sem push, issue, PR ou comentário no upstream.

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | [modelcontextprotocol/python-sdk](https://github.com/modelcontextprotocol/python-sdk) |
| Linguagem | Python 3.11+ (SDK MCP; pacote `mcp`) |
| Tamanho | ~25 MiB shallow clone; ~303 MiB com `.venv`; ~420 `.py` em `src/`+`tests/` |
| Commit base do clone | `77001c9` (`main` shallow) |
| Grupo | Com instrumentação AI existente |
| Instrumentação AI | `AGENTS.md` (166 linhas), `CLAUDE.md` (`@AGENTS.md`), `.claude/commands/`, `.claude/skills/test-quality/`, workflow Claude |
| Toolchain | **Sim, parcial.** `uv 0.13.8` + `uv sync --frozen` (Python 3.12). Rodados: pytest alvo, `ruff check` nos arquivos tocados, `compileall` pontual. **Não rodados:** `./scripts/test` completo, `pyright` full, docs/`zensical`, matrix 3.11–3.15. |

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Em `get_display_name` (`src/mcp/shared/metadata_utils.py`), tratar `title` (e `annotations.title`) em branco como ausente, caindo para o próximo nível / `name`; testes em `tests/shared/test_metadata_utils.py` |
| Por quê | Bug/UX de cliente pequeno, testável, fora do hot path de transporte; plausível para maintainer; exercício diferente da rodada 1 |
| Issue | Nenhuma (mudança local; sem interação com upstream) |
| Completada? | **Sim, localmente** (branch `campo2-local-display-name`, commit `036fec8`). Não enviada ao upstream. |
| Native test/lint/build | `uv run --frozen pytest tests/shared/test_metadata_utils.py tests/server/mcpserver/test_title.py -q` → **10 passed**; `ruff check` nos tocados → OK; `compileall` do módulo → OK. Suite completa / pyright **não** executados. |
| Rodada 1 (comparação) | Default `StdioServerParameters.encoding_error_handler`: `"strict"` → `"replace"` (issue #2454) |

## Verificação das 12 correções

| # | Correção | Resultado | Evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph --enable rebuild` em `/tmp/target` → **16521 nós / 63000 arestas** (~2,1 s). Na rodada 1 o mesmo path dava 0 nós. |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`, com `hidden_test_cmd: uv run --frozen ruff check …` → `"hidden_available": true`, `"hidden_passed": true`, verdict PASS. |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Toque em path protegido + `--strict --json` → JSON com violation e **EXIT 1**. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `src/mcp/shared/metadata_utils.py` → `protected_path` + EXIT 1 sob `--strict --json`. |
| 5 | init detecção de toolchain | **funcionou** | `tb init` → `verify.test_cmd: uv run pytest` (não mais `go test`). `hidden_test_cmd` veio vazio. |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket só citando `tests/hidden/**` → findings=[]; commit real em `tests/hidden/probe.py` → finding critical da invariante. |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `bench.default_agent: true {prompt}` → `tb bench run` sem `--agent/--fake` OK; sem default → erro obrigatório; `tb bench compare … --agent 'true {prompt}'` OK. |
| 8 | review `## Scope`/`## Escopo`, listas MD e `**` | **funcionou** | Ticket `## Escopo` + bullets → drift/review limpos (sem “sem scope declarado”). Frontmatter `src/mcp/shared/**/*.py` também aceito. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Verify final: `build_ran/passed` e `lint_ran/passed` true antes dos testes. |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** | Init logou sources + hints (`uv run --frozen …`); gravou `.toolbelt/agent_docs.yaml`. Em dir vazio, `--write-agents` semeou `AGENTS.md` sem clobber. |
| 11 | graph Python `calls` | **funcionou** | Stats: **19400** `edges_calls` (rodada 1: 100). `callers --function get_display_name` e `impact --file …/metadata_utils.py` listaram callers reais. |
| 12 | verify visual skipped agregado + docs CLI | **funcionou (parcial)** | Manifesto ausente → `skipped: true`. Telas com ref ausente → top-level `skipped: true` + `skip_reason: todas as telas foram skipped`. **Ainda:** `screens: []` → `skipped: false`, `screens: null`. Docs: `hidden_test_cmd` alinhado; `tb ticket` marcado “Não implementado”, mas exemplos longos de `tb ticket`/`mem add --type` ainda aparecem. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; detectou uv/pytest; leu AGENTS/CLAUDE | ajudou | Wall ~0,006 s. `test_cmd=uv run pytest` (sem `--frozen` exigido pelo AGENTS). Config mode 0600. |
| `tb init --write-agents` | OK em repo sem AGENTS | ajudou | Semear `/tmp/noagents/AGENTS.md`; no python-sdk não sobrescreveu. |
| `tb version` | `a46c295` com `make build` | neutro | Sem ldflags ainda mostra `unknown` (não retestado neste ciclo). |
| `tb mem init/list/add/check` | OK; 3 anti-cheat defaults | neutro | Não importa regras do AGENTS.md para o vault; só template genérico. |
| `tb graph rebuild` `/tmp/target` | 16521 nós | ajudou | Confirma fix SkipDirs. |
| `tb graph rebuild` `/tmp/python-sdk` | 16541 nós / 63030 arestas | ajudou | ~2,0 s. |
| `tb graph search/callers/impact/deps` | search + callers + impact úteis; deps = `mcp_types` | ajudou | Calls Python viabilizam navegação da tarefa. |
| Ticket `## Escopo` + `**` | drift/review OK | ajudou | Diferente da rodada 1 (parser quebrado). |
| `tb verify run` (build+lint+test+hidden) | PASS; hidden rodou sem dir | ajudou | ~1,7 s no caminho feliz. |
| `tb verify --ruler visual` | skip agregado OK; `screens: []` ainda `skipped:false` | neutro/atrapalhou | Caso vazio continua mentiroso. |
| `tb review` / `tb drift` | PASS no fix com frontmatter ou `## Escopo` | ajudou | Fresh_context true; findings=[]. |
| `tb guard check` | OK no fix; honra `guard.yaml` | ajudou | Strict+JSON exit 1 com violation. |
| `tb bench run/compare` | default_agent + `--agent` no compare | ajudou | Fixtures do toolbelt; repo alvo sem `bench/tasks`. |
| `tb mcp` | tools/list + `tb_version` + `tb_graph_search` OK | ajudou | Com `TB_GRAPH=on`, search achou `get_display_name`. |

## Problemas que continuam

| problema (rodada 1) | status nesta rodada | evidência |
|---------------------|---------------------|-----------|
| Init não usa `--frozen` / checks do AGENTS | **continua (parcial)** | Lê hints em `agent_docs.yaml`, mas `verify.test_cmd` fica `uv run pytest` sem `--frozen`; não preenche lint/build/pyright/`./scripts/test`. |
| mem defaults genéricos vs AGENTS | **continua** | `mem init` só anti-cheat Go-ish; não materializa regras uv-only / encoding= / coverage. |
| Verify checks parciais / miss fora do cmd | **continua** | Syntax error em `examples/_campo_broken.py` + test_cmd estreito → verify PASS. |
| Config files mode 0600 | **continua** | `stat` → `config.yaml` e `agent_docs.yaml` mode 600. |
| Docs CLI com superfícies inventadas | **continua (reduzido)** | `tb ticket` agora “Não implementado”, mas blocos de exemplo e `mem add --type/--title` ainda presentes. |
| Visual PASS enganoso | **continua (caso residual)** | `screens: []` → top-level `skipped: false`. |
| Grafo off sem `TB_GRAPH`/`--enable` | **continua (by design)** | ADR-011; MCP search precisa do env. |

Problemas da rodada 1 **resolvidos** neste binário: SkipDirs/`/tmp/target`, `guard.yaml`, `--strict --json`, hidden sem dir, Scope markdown, default_agent, callers Python, leitura de AGENTS.md, build/lint cmds, FP de `tests/hidden` por substring.

## Problemas novos

### P2 — `screens: []` ainda reporta `skipped: false`

- **Repro:** `.toolbelt/screens.yaml` com `screens: []` + `tb verify run --ruler visual --skip-unavailable --json`
- **Expected:** `skipped: true` (nada a avaliar)
- **Actual:** `"skipped": false`, `"screens": null`, verdict PASS

### P2 — Init lê AGENTS.md mas não aplica ao harness

- **Repro:** `tb init` no python-sdk → `agent_docs.yaml` com `uv run --frozen pytest` / ruff / pyright; `config.yaml` com `test_cmd: uv run pytest` e lint/build vazios
- **Expected:** mapear hints para `verify.test_cmd` / `lint_cmd` / checklist ou avisar mismatch
- **Actual:** dicas só no log/YAML auxiliar; agente ainda precisa reescrever verify à mão

### P3 — `hidden_test_cmd` default vazio após init Python

- **Repro:** init em repo com `uv.lock` → `hidden_test_cmd: ""`
- **Expected:** default útil (ex. ruff) ou documentar que hidden é opt-in explícito
- **Actual:** campo vazio; hidden só roda se o agente configurar

### P3 — Impacto lista callers de símbolo, não “arquivos afetados” de forma óbvia

- **Repro:** `tb graph impact --file src/mcp/shared/metadata_utils.py` → funções que chamam `get_display_name` (útil), sem ranking de risco/teste sugerido
- **Expected:** (melhoria) apontar testes/exemplos prioritários a partir do impact
- **Actual:** lista crua; o agente ainda decide cobertura

## O que o tb pegou e o que deixou passar

**Pegou**

- Escopo via `## Escopo` / frontmatter e globs `**`
- Guard via `guard.yaml` + hardcoded + exit code strict/JSON
- Build + lint + test + hidden (sem pasta `tests/hidden/`)
- Callers/impact Python para `get_display_name`
- Onboarding: descoberta de AGENTS/CLAUDE e seed `--write-agents`
- Bench com `default_agent` e `compare --agent`
- MCP graph search com grafo habilitado

**Deixou passar / não cobriu**

- `uv run` sem `--frozen` no default de init vs mandato do AGENTS.md
- pyright / `./scripts/test` / cobertura 100% — não enfileirados
- Erro de sintaxe fora dos cmds configurados → PASS
- Manifesto visual vazio (`screens: []`) aparenta execução
- Regras densas do AGENTS.md não viram invariantes `mem`

## Interação com instrumentação existente / onboarding

| Comportamento | Detalhe |
|---------------|---------|
| tb **lê** AGENTS.md/CLAUDE.md | `agent_docs.yaml` + log de hints (`uv run --frozen …`, ruff, pyright, coverage). |
| tb **não aplica** automaticamente | `verify.*` não herda `--frozen` nem lint/typecheck; vault `mem` ignora o conteúdo. |
| `--write-agents` | Só semeia se ausente; no python-sdk preservou o AGENTS maduro. |
| Sobreposição | Anti-cheat `tests/hidden` do `mem init` ainda é genérico; skill `.claude/skills/test-quality/` não é indexada no vault. |
| O que ainda falta no onboarding | (1) promover hints → `verify.test_cmd`/`lint_cmd`/`build_cmd`; (2) invariantes a partir de AGENTS (uv-only, encoding=); (3) apontar `scripts/test` e skill de qualidade. |

## Comparação com a rodada 1

| Aspecto | Rodada 1 (`c8aae04`) | Rodada 2 (`a46c295`) |
|---------|----------------------|----------------------|
| Grafo em `/tmp/target` | 0 nós | ~16k nós |
| Calls Python | ~100; callers vazios | ~19k; callers/impact úteis |
| Init | `go test`; ignora AGENTS | `uv run pytest`; lê AGENTS + `--write-agents` |
| Scope markdown | quebrado sem frontmatter | `## Escopo` + `**` OK |
| Hidden / guard.yaml / strict JSON / default_agent | falhavam | funcionam |
| Visual skipped | `skipped:false` com refs skipped | agregado OK; residual `screens: []` |
| Conclusão | tb atrapalhava onboarding Python | **melhor de forma concreta** nas 12 correções; lacuna principal agora é **aplicar** AGENTS ao harness, não só lê-lo |

## Sugestões de melhoria

1. **[P1][init/verify]** Ao detectar hints `uv run --frozen …` / ruff / pyright, preencher `verify.test_cmd`/`lint_cmd` (ou falhar init com diff esperado vs AGENTS).
2. **[P1][mem]** Opcional: extrair invariantes de seções “FORBIDDEN”/mandatos do AGENTS.md.
3. **[P2][verify/visual]** `screens: []` ou zero screens avaliáveis → `skipped: true`.
4. **[P2][docs]** Remover ou isolar exemplos de `tb ticket` / `mem add --type` atrás de “planejado”, sem parecer CLI atual.
5. **[P2][init]** Default `hidden_test_cmd` para Python (ex. `uv run --frozen ruff check .`) ou omitir o campo com doc clara.
6. **[P3][config]** Revisar 0600 em monorepos; 0644 para YAML não secretos.
7. **[P3][graph]** `impact` sugerir testes tocados / exemplos callers como checklist de verify.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Binário | `toolbelt a46c295` | `toolbelt c8aae04` |
| Wall somado (amostras cronometradas, logs 1+2) | **~24,7 s** / 72 amostras | ~35,4 s / 70 |
| Wall do reteste canônico (log 2, main atual) | **~9,7 s** / 25 amostras | — |
| Falhas EXIT≠0 (todos os logs) | **9** (probes: write-agents antigo, bench flags, guard intencional, drift `**/*.py` no binário intermediário, etc.) | 25/70 |
| Falhas no caminho feliz do fix (a46c295) | **0** (verify/review/drift/guard finais PASS) | poucas após frontmatter |
| Maior WALL individual | **3,37 s** (pytest nativo inicial) / graph rebuild ~2,1 s | 9,96 s |
| Verify final | **~1,7 s**, PASS, hidden+build+lint | 3,52 s |
| Grafo útil | 16541 nós, 19400 calls | 16550 nós, 100 calls |
| MCP smoke | tools/list + search OK | tools/list OK; search nulo em `/tmp/target` |
