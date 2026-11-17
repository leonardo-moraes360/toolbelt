🇧🇷 Português | [🇺🇸 English](../../en/campo/rodada2/pydantic-ai.md)

# Campo rodada 2: pydantic/pydantic-ai

Estudo de campo do `tb` (build local `make build`, versão `a46c295`) contra clone local de [pydantic/pydantic-ai](https://github.com/pydantic/pydantic-ai) em `/tmp/campo2-pydantic-ai/repo`. Sem push, issues, PRs ou comentários no alvo. Grupo: **with existing AI-agent instrumentation**.

> Nota de método: o `main` local do toolbelt estava em `8b5a499` (faltavam PRs #33–#40). Após `git fetch origin main` + rebuild, o binário usado em todas as evidências abaixo é `toolbelt a46c295`.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | `pydantic/pydantic-ai` |
| Linguagem | Python ≥3.11 (workspace `uv`: `pydantic-ai-slim`, `pydantic-graph`, `pydantic-evals`, `pydantic-ai-harness`, `clai`, …) |
| Tamanho (clone shallow) | ~508 MiB sem contar `.venv`; ~6996 entradas no tree (excl. `.venv`/`.git`/`.toolbelt`); ~2528 `.py` |
| Commit base do estudo | `c7586d7` (main no clone) |
| Instrumentação AI existente | `AGENTS.md` + symlink `CLAUDE.md`; `AGENTS.md`/`CLAUDE.md` em subpacotes; `.agents/skills/` (13 skills) e `.claude/`; `.gemini/config.yaml`; `agent_docs/`; etc. |
| Toolchain | **Sim, parcial.** `uv` 0.13.8; `make install` ok; `make lint` ok; `uv run pytest` no arquivo alvo ok; `pyright` no arquivo alterado: 0 erros. **Não rodado:** suite completa `make test` / matrix multi-Python; keys de providers LLM (irrelevantes para a tarefa). |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Estender o pattern `google_api_key` em `pydantic_ai_harness.guardrails.detectors` para redigir chaves Gemini com prefixo `AQ.` (além do legado `AIza`), com testes de regressão |
| Por quê | Bug pequeno, sem LLM, um módulo + testes; MRE na issue; diferente da rodada 1 |
| Issue | https://github.com/pydantic/pydantic-ai/issues/13550 (label `needs-maintainer-discussion`; bot recomenda adicionar `AQ.` aos defaults) |
| Completa? | **Sim, localmente.** Branch local `campo/fix-aq-prefix` @ `307b3c8` (nunca pushada). Diff: `detectors.py` + `tests/harness/guardrails/test_detectors.py` + `rd.md` |
| Checks nativos | `pytest` arquivo guardrails: pass (incl. 2 novos); `make lint` pass; `pyright` no arquivo: 0 erros. Suite completa: **não** |
| Rodada 1 (comparação) | `MessagesBuilder.last_modified` (issue #13535) — tarefa distinta |

## Verificação das 12 correções

| # | correção | status | evidência |
|---|----------|--------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Em `/tmp/target` (dir real com `pkg/mod.py`): `tb graph --enable rebuild` → **4 nós / 4 arestas**, `search hello` achou `func:pkg.hello`. Antes da correção, basename `target` zerava o grafo. |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; `hidden_test_cmd` no YAML + `--hidden-test-cmd …`: JSON com `hidden_available: true`, `hidden_passed: true`, verdict `PASS` (~14 s). |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Com `protected_paths` apontando para o arquivo do fix: `--json` sozinho exit **0** (violations presentes); `--strict --json` exit **1** + JSON válido com `protected_path`. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com path do fix → violation; trocando para `secrets/never/**` → `violations: null`, `--strict --json` exit 0. Hardcoded `tests/hidden/*` também pegou `tests/hidden/cheat.py`. |
| 5 | init detecção de toolchain | **funcionou** | `tb init` → `verify.test_cmd: uv run pytest` (não mais `go test ./...`). `hidden_test_cmd` veio `""` (melhor que default Go). |
| 6 | review `tests/hidden` por path | **funcionou** (parcial) | Ticket que só *menciona* `tests/hidden/**` em Forbidden: sem finding de invariante por substring. Toque real em `tests/hidden/cheat.py`: guard pega; review/drift marcam `out_of_scope` — mas `forbidden_hit` ficou `null` e não houve finding explícito da invariante mem `tests/hidden/`. |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Em `/agent/repos/toolbelt` com `bench.default_agent: "./bin/fake-agent"`: `tb bench run --task bugfix-go -r 1` **sem** `--agent/--fake` → 100% sucesso. `tb bench compare … --agent ./bin/fake-agent` exit 0. Help documenta `--agent` em compare. |
| 8 | review/drift parseia `## Scope`/`## Escopo`, listas e `**` | **funcionou** (parcial) | `## Scope` + bullets: drift limpo quando scope cobre os 3 arquivos. `## Escopo` parseado. Glob `**/*detectors*` manteve os `.py` in-scope (só `rd.md` out). Porém `Forbidden: tests/hidden/**` **não** populou `forbidden_hit` ao tocar o dir. |
| 9 | verify `build_cmd` / `lint_cmd` | **funcionou** | PASS com `build_ran/lint_ran/passed: true`. Com `lint_cmd: false` → `lint_passed: false`, verdict `ESCALATE`, exit 1 (3 iterações). |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** | Init logou `documentação de agentes encontrada` + `agent_docs.yaml` (sources + hints: make/pytest/ruff/uv…). `--write-agents` **não** sobrescreveu `AGENTS.md` existente (exit 0). |
| 11 | graph Python `calls` edges | **funcionou** (parcial) | `edges_calls: 201764` (rodada 1: impact/callers vazios). `impact --file …/detectors.py` listou `_capability.py`, `test_detectors.py`, funcs. `callers --function secret_data` listou testes. `callers --function redact_secrets` ainda `(vazio)` (nome/resolução incompleta). |
| 12 | verify visual skipped agregado + docs CLI | **funcionou** | Manifesto ausente: `skipped: true`. Manifesto com ref ausente: `skipped: true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` marca `tb ticket`/`board` como planejados e documenta `## Scope` / `hidden_test_cmd`. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; Python/`uv` detectado; leu AGENTS.md | **ajudou** | `test_cmd: uv run pytest`; `agent_docs.yaml` com hints de Makefile |
| `tb init --write-agents` | OK; não sobrescreve | neutro | Exit 0; AGENTS.md intacto (já existia) |
| `tb mem init/add/list/check` | OK | neutro | 3 anti-cheat default + 1 regra da tarefa; `mem check` ok |
| `tb graph --enable rebuild` | OK (~11–12 s); 71000 nós / 336596 arestas | **ajudou** | Inclui `calls` em massa |
| `tb graph search` | Achou `redact_secrets` | ajudou | Localização rápida |
| `tb graph impact --file …/detectors.py` | Lista arquivos/funcs relacionados | **ajudou** | Incluiu `tests/harness/guardrails/test_detectors.py` (na rodada 1 impact era vazio) |
| `tb graph callers --function redact_secrets` | `(vazio)` | atrapalhou | Apesar de `calls` existirem; `secret_data` funciona |
| `tb graph` em `/tmp/target` | 4 nós | ajudou | SkipDirs na raiz corrigido |
| Ticket `rd.md` (`## Scope` + `**`) | Drift/review limpos após incluir `rd.md` no scope | ajudou | Primeira versão flagrou `rd.md` out_of_scope (parser ok) |
| `tb verify run` | PASS com build+lint+visible+hidden | **ajudou** | JSON completo; hidden sem dir `tests/hidden/` |
| `tb verify --ruler visual` | PASS skipped | neutro | Agregação `skipped: true` correta |
| `tb review` | Limpo no fix; pega drift de scope | ajudou | Sem FP por mencionar `tests/hidden` no ticket |
| `tb drift` | OK no scope; falha se arquivo fora | ajudou | `forbidden_hit` ainda frágil com `**` |
| `tb guard check A..B` | Lê `guard.yaml`; `--strict --json` exit 1 | **ajudou** | UX: range posicional `base..head` (não `--base`) |
| `tb bench run` / `compare --agent` | OK no repo toolbelt com default_agent | neutro | No alvo não há `bench/tasks`; harness usa fixtures do toolbelt |
| `tb mcp` | tools/list + `tb_version` OK | neutro | Tools: graph_* + echo/env/run/version — sem mem/verify/review/drift/guard |

## Problemas que continuam

Da rodada 1 (`docs/campo/pydantic-ai.md`), o que **ainda** aparece:

| problema rodada 1 | status rodada 2 |
|-------------------|-----------------|
| Init defaults Go | **corrigido** (`uv run pytest`) |
| `hidden_test_cmd` gated por dir | **corrigido** |
| `guard.yaml` ignorado | **corrigido** |
| `default_agent` / compare sem `--agent` | **corrigido** |
| Não lê AGENTS.md | **corrigido** (leitura + hints; não importa skills) |
| Python sem `calls` / impact vazio | **melhorou** (calls + impact úteis); `callers redact_secrets` ainda vazio |
| Verify sem build/lint | **corrigido** quando configurados |
| Docs CLI desatualizados (`ticket`/`board`) | **melhorou** (marcados planejados); árvore ainda lista comandos Fase 3 |
| Config mode `0600` | **continua** (`config.yaml` mode 600) |
| MCP sem paridade CLI | **continua** |
| tb não importa skills / não empurra checklist AGENTS.md | **continua** (só hints superficiais de comandos) |
| `graph impact` exige `--file` | **continua** (UX) |

## Problemas novos

| ID | sev | repro | esperado | atual |
|----|-----|-------|----------|-------|
| N1 | P2 | Ticket com `## Forbidden` / `- tests/hidden/**`; commit que adiciona `tests/hidden/cheat.py`; `tb drift`/`tb review` | `forbidden_hit` preenchido (e/ou finding de invariante mem) | Só `out_of_scope`; `forbidden_hit: null`; sem finding da invariante `tests/hidden/` |
| N2 | P2 | `tb graph callers --function redact_secrets` após rebuild com 200k+ `calls` | Callers ou mensagem de desambiguação | `(vazio)`; `secret_data` no mesmo arquivo funciona |
| N3 | P3 | `tb guard check --base HEAD~1 --head HEAD` (flags estilo verify) | Aceitar ou erro claro apontando `base..head` | `unknown flag: --base` |
| N4 | P3 | `tb init` em repo com AGENTS.md rico | Hints acionáveis amarrados a `verify.lint_cmd`/`typecheck` | Hints em `agent_docs.yaml`, mas `lint_cmd`/`build_cmd` não preenchidos; agente ainda configura na mão |
| N5 | P3 | Clone shallow sem commit local; `tb verify run` sem `--base/--head` | Mensagem clara | `Error: exit status 128` (git) opaco |

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift quando `rd.md` ou `tests/hidden/cheat.py` ficaram fora do `## Scope`.
- `guard.yaml` protected_paths e path hardcoded `tests/hidden/*`.
- Falha de `lint_cmd` → verify não PASS.
- Hidden suite configurada sem pasta `tests/hidden/`.
- Impact sugerindo o arquivo de teste a tocar.
- Skip visual honesto (`skipped: true`).

**Deixou passar**
- `forbidden_hit` para glob `tests/hidden/**` no ticket.
- Finding de invariante mem ao editar `tests/hidden/` (só drift de scope).
- Completeness “soft”: review limpo sem checar se a discussão de maintainer / docs de guardrails foram atualizados (neste caso o fix local segue o MRE; o ponto é que tb não sabe do label `needs-maintainer-discussion`).
- Não cobrou `make typecheck` global — só o que estava em `lint_cmd`/`test_cmd`.

## Interação com instrumentação existente / onboarding

- **tb lê** `AGENTS.md`/`CLAUDE.md` no init → `.toolbelt/agent_docs.yaml` (sources + hints de `make`/`pytest`/`ruff`/`uv`).
- **tb não importa** skills `.agents/skills/*`, nem regras de escopo estreito / type-safety / PR template do AGENTS.md para `mem` ou checklist de review.
- **`--write-agents`**: correto ao não sobrescrever; útil só na ausência de instrumentação.
- **Conflito residual:** invariantes default do tb (`tests/hidden/`, thresholds) vs workflow real (skills `initialize-worktree`, `branch-context`, pre-commit `no-commit-to-branch`). Coexistem sem merge.
- Onboarding melhor que na rodada 1 (toolchain + agents docs), mas o agente ainda precisa ler AGENTS.md por fora para o padrão do projeto.

## Comparação com a rodada 1

| dimensão | rodada 1 | rodada 2 | veredito |
|----------|----------|----------|----------|
| Init / verify defaults | `go test ./...` | `uv run pytest` | **melhor** |
| Hidden tests | ignorados sem dir | executam se cmd setado | **melhor** |
| Guard custom | YAML morto | `protected_paths` + strict JSON | **melhor** |
| AGENTS.md | ignorado | lido + hints | **melhor** |
| Graph Python | impact/callers vazios | calls + impact úteis | **melhor** |
| Bench flags | `--agent` ausente em compare; default morto | ambos ok | **melhor** |
| Scope Markdown | frágil | `## Scope`/`## Escopo` + `**` em paths de código | **melhor** |
| Forbidden `**` / invariante hidden | — | `forbidden_hit` null | **buraco novo/residual** |
| MCP paridade | incompleta | igual | **igual** |
| Tarefa local | completa | completa (outra issue) | **igual** |

No geral: **claramente melhor** que a rodada 1 neste repo; as 12 correções se confirmam na prática, com ressalvas em callers por nome, forbidden globs e profundidade do import de AGENTS.md.

## Sugestões de melhoria

1. **[P1 · drift/review]** Fazer `## Forbidden` com `**` popular `forbidden_hit`; alinhar invariante mem `tests/hidden/` ao mesmo matcher de path do guard.  
2. **[P1 · graph]** Resolver `callers --function` por sufixo/título quando houver um único símbolo Python (`redact_secrets`), ou listar candidatos.  
3. **[P1 · init]** Mapear hints de AGENTS.md/`Makefile` → `verify.lint_cmd` / typecheck (não só `test_cmd`).  
4. **[P2 · guard UX]** Aceitar `--base`/`--head` além de `A..B`, espelhando verify/review.  
5. **[P2 · verify]** Traduzir `exit status 128` do git em “crie um commit ou passe --base/--head”.  
6. **[P2 · mcp]** Expor mem/verify/review/drift/guard.  
7. **[P3 · mem]** Importar 3–5 invariantes textuais do AGENTS.md (anti-patterns do projeto) no `tb init`.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time (soma EXIT/TIME no log estruturado) | ~74 s de invocações cronometradas; sessão ~40–50 min incl. fetch/rebuild/clone/`make install` | ~57 s invocações; ~25–30 min |
| Invocações tb (+ nativos no log) | ~54 seções; ~48 com EXIT/TIME | ~53 parseáveis |
| Falhas tb (exit ≠ 0) no log | 12 (inclui negativos esperados: drift proposital, guard strict, flags erradas, lint fail) | 26 |
| Crash/segfault | nenhum | nenhum |
| Binário | `a46c295` | `c8aae04` |
| Tarefa entregue no alvo | sim (local only); pytest AQ./AIza 2/2 + arquivo guardrails pass | sim (local only) |
