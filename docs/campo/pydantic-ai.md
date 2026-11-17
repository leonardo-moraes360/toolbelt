🇧🇷 Português | [🇺🇸 English](../en/campo/pydantic-ai.md)

# Campo: pydantic/pydantic-ai

Estudo de campo do `tb` (build local `make build`, versão injetada `c8aae04`) contra clone local de [pydantic/pydantic-ai](https://github.com/pydantic/pydantic-ai) em `/tmp/target/pydantic-ai`. Sem push, issues, PRs ou comentários no alvo.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | `pydantic/pydantic-ai` |
| Linguagem | Python ≥3.11 (workspace `uv`: `pydantic-ai-slim`, `pydantic-graph`, `pydantic-evals`, `pydantic-ai-harness`, `clai`, `pydantic-clai2`) |
| Tamanho (clone shallow) | ~473 MiB; ~6023 arquivos no tree; ~2507 `.py` (antes de `.venv` / `.toolbelt`) |
| Commit base do estudo | `f93bcff0` (main no momento do clone) |
| Instrumentação AI existente | `AGENTS.md` + symlink `CLAUDE.md`; `AGENTS.md`/`CLAUDE.md` em vários subpacotes (`docs/`, `tests/`, `pydantic_ai_slim/…`, harness, clai2); `.agents/skills/` e `.claude/skills/` (branch-context, pushing-commits, initialize-worktree, etc.); `.claude/settings.json`; `.gemini/config.yaml`; `agent_docs/`; `PLAN.md`; `THREAT_MODEL.md` |
| Toolchain | **Sim, parcial.** `uv` 0.13.8 instalado; `make install` (sync + pre-commit) ok; `make lint` ok; `uv run pytest` ok nos testes alvo; `pyright` pontual no arquivo alterado ok. **Não rodado:** suite completa `make test` / `make` / matrix multi-Python (`install-all-python`), por custo; keys de providers LLM não configuradas (irrelevantes para a tarefa escolhida). |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `MessagesBuilder.last_modified` para devolver a mensagem **mais recente** do tipo pedido desde o checkpoint (usar `reversed(_modified_since(...))`), com teste de regressão |
| Por quê | Bug pequeno, reprodutível sem LLM, um arquivo + teste; maintainer já descreveu a correção em uma linha |
| Issue | https://github.com/pydantic/pydantic-ai/issues/13535 |
| Completa? | **Sim, localmente.** Commit local `1ddd0d1` em branch `campo/fix-last-modified` (nunca pushado). Diff: `_messages_builder.py` + `tests/test_ui.py` |
| Checks nativos | `ruff format/check` nos arquivos: pass; `pyright` no arquivo: 0 erros; `pytest tests/test_ui.py -k messages_builder`: 3 passed. Suite completa / typecheck global: **não executados** |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK (0.007 s); criou `.toolbelt/` com defaults Go | atrapalhou (defaults) | `verify.test_cmd: go test ./...` num repo Python; `config.yaml` mode `600` |
| `tb mem init/add/list/check` | OK; 4 invariantes | neutro/ajudou | `mem add` só posicional (`tb mem add <regra>`); `invariants.yaml` mode `600`; check sha256 ok |
| `tb graph --enable rebuild` | OK (5.5 s); 70906 nós / 135068 arestas | ajudou (mapa) | Index Python/markdown; `graph.json` ~53 MiB mode `600` |
| `tb graph search last_modified` | Achou `func:…ui.last_modified` | ajudou | Localizou o símbolo rápido |
| `tb graph impact` sem `--file` | Erro `--file é obrigatório` | atrapalhou | UX exige flag; não infere do diff |
| `tb graph impact --file …/_messages_builder.py` | OK mas saída `(vazio)` | atrapalhou | Sem impacto útil apesar do índice |
| `tb graph callers` / `deps` | Posicional rejeitado; com flags OK | atrapalhou (UX) | `callers --function last_modified` → vazio; `deps --file` listou pkgs |
| Ticket `rd.md` (Scope) | Drift limpo com paths literais | ajudou | Frontmatter `base_commit` + `scope` + `forbidden` |
| `tb verify run` (pytest) | PASS visível; oculto `disponível: false` | parcial | Só roda hidden se existir dir `tests/hidden/`; ignora `--hidden-test-cmd` sem esse dir |
| `tb verify` + thresholds no diff | FAIL iter 0, sem rodar testes | atrapalhou | Guard `protected_path` em `.toolbelt/thresholds.yaml` aborta verify |
| `tb verify --ruler visual` | PASS com skip | neutro | JSON: `"skipped": true`, reason manifesto ausente |
| `tb review` | OK; 1 warning completeness | neutro | Critério “existing … tests still pass” “pouco evidenciado” apesar do teste novo |
| `tb drift` | OK no scope literal | ajudou | Sem out_of_scope no ticket real |
| `tb drift` com `**/*.py` | Marcou arquivos in-scope como fora | atrapalhou | `filepath.Match` não trata `**` recursivo |
| `tb guard check` | OK no diff limpo; viola `thresholds.yaml` | ajudou (anti-cheat) | `guard.yaml` custom **ignorado** (deny do arquivo do fix → `violations: null`) |
| `tb bench run --fake` | OK em `bugfix-go` (repo toolbelt) | neutro | No alvo não há `bench/tasks`; harness usa fixtures do toolbelt |
| `tb bench compare` | A/B fake OK; flag `--agent` inexistente | atrapalhou | `unknown flag: --agent` |
| `tb mcp` | tools/list + `tb_version` OK | neutro | Ferramentas graph/mem/verify **não** espelhadas além de graph_* + echo/env/run/version |
| Onboarding vs AGENTS.md | tb **não lê** AGENTS.md/skills | atrapalhou | `tb ctx`/import AGENTS.md só em docs de pesquisa (“pendente”); init não referencia skills do repo |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare … --agent` → `unknown flag: --agent`; help sem a flag |
| `guard.yaml` ignored | **confirmado** | `.toolbelt/guard.yaml` com deny do path do fix; `guard check` → `violations: null` |
| outdated CLI docs | **confirmado** | `docs/cli.md` documenta `tb ticket`/`tb board`/`tb run`/`tb proc`; CLI real: `ticket`/`board` inexistentes; `router` “não implementado” |
| version shows `unknown` | **confirmado (sem ldflags)** | `go build` sem `-ldflags`: `commit: unknown` / `built: unknown`; `make build` injeta hash (`c8aae04`) |
| config files created 0600 | **confirmado** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → mode `600` |
| visual ruler reports `skipped=false` when skipping | **refutado** | `/tmp/verify-visual.json`: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` |
| `default_agent` ignored | **confirmado** | `bench.default_agent` no YAML; `tb bench run` sem `--agent`/`--fake` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | Diff com `.toolbelt/thresholds.yaml` → violation `protected_path`; verify FAIL iterações 0 |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake --task bugfix-go` sucesso sem `--fake-config`; `FakeAgentCommand("")` embutido |
| `mem add` positional | **confirmado** | Usage `tb mem add <regra>`; `--rule` → unknown flag |
| Drift/Scope need literal paths (no globs) | **confirmado (parcial)** | Prefixo de dir e `foo/**` ok; `pydantic_ai_slim/**/*.py` **não** casa (ambos arquivos out_of_scope); `ui/*` single-segment via `filepath.Match` ok |
| `graph impact` needs `--file` | **confirmado** | Sem flag: exit 1 `--file é obrigatório` |
| review false positives on the tests/hidden invariant | **refutado (neste fluxo)** | Review do fix limpo: só warning de completeness; invariante `tests/hidden` só disparou ao editar de fato `tests/hidden/` |
| verify runs only partial checks and misses build errors | **confirmado** | Erro de sintaxe em `scripts/_tb_campo_broken.py` + `test-cmd` pytest pontual → verify **PASS**; ruff nativo falha. Defaults `go test` num repo Python → ESCALATE sem validar Python |
| tb does not push the agent toward completing the whole task | **confirmado (observacional)** | AGENTS.md exige skills, docs, typecheck amplo, PR template, etc.; tb não importa nem cobra isso — só diff/scope/test_cmd. Agente pode “passar” no tb com tarefa incompleta perante o padrão do projeto |

## Problemas novos

| ID | sev | repro | esperado | atual |
|----|-----|-------|----------|-------|
| N1 | P1 | `tb init` em repo Python; `tb verify run` sem sobrescrever cmds | Detectar ecossistema ou falhar com orientação clara | Defaults `go test ./...`; ESCALATE opaco |
| N2 | P1 | `tb verify … --hidden-test-cmd 'pytest …'` sem dir `tests/hidden/` | Honrar o comando passado | `hidden_available: false`; comando ignorado |
| N3 | P2 | `tb graph impact --file <py com símbolos>` após rebuild | Lista callers/deps impactados | `(vazio)` |
| N4 | P2 | `tb graph callers last_modified` (posicional, como search) | Aceitar posicional ou mensagem única | Exige `--function`; UX inconsistente com `search` |
| N5 | P2 | MCP `tools/list` após usar CLI mem/verify/review | Paridade CLI↔MCP | Só `tb_graph_*`, `tb_version`, `tb_echo`, `tb_env`, `tb_run` |
| N6 | P3 | Scope `pydantic_ai_slim/pydantic_ai/ui/*` vs arquivo no dir | Documentar que `*` é single-segment / preferir `dir/` ou `dir/**` | Comportamento misto confunde (“globs”) |
| N7 | P2 | Repo com AGENTS.md rico; `tb init` | Importar/resumir regras do projeto no vault/invariants | Ignora por completo; risco de conflito com invariantes genéricas Go-oriented (`tests/hidden/`) |

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de escopo quando o ticket listava paths literais corretos.
- Edição de `tests/hidden/` (forbidden + invariantes) e de `.toolbelt/thresholds.yaml` (guard).
- Falha de testes visíveis quando o próprio `test_cmd` importava arquivo com sintaxe quebrada.
- Skip visual com manifesto ausente (`skipped: true`).

**Deixou passar**
- Erro de sintaxe em arquivo **fora** do `test_cmd` (PASS do verify vs FAIL do ruff).
- Defaults Go em projeto Python (agente precisa reconfigurar na mão).
- Completeness “soft”: warning por critério de aceite textual, mas sem checar se docs/skills/THREAT_MODEL do AGENTS.md foram atualizados (neste caso não precisavam — o ponto é que tb não sabe).
- `graph impact` vazio → não sugeriu `tests/test_ui.py` como arquivo a tocar.
- `guard.yaml` do operador não aplicado.
- Não cobrou lint/typecheck do projeto; só o que estava em `test_cmd`.

## Interação com instrumentação existente / onboarding

- **tb não lê** `AGENTS.md`, skills `.agents/`/`.claude/`, nem `agent_docs/`. Busca no código Go/docs do toolbelt: AGENTS.md aparece só como ideia futura (`tb ctx`, arquitetura “pendente”).
- **Duplicação potencial:** invariantes default (`tests/hidden/`, `thresholds.yaml`) são do harness Go do tb; pydantic-ai já tem anti-patterns e workflow em AGENTS.md/skills — conjuntos distintos, sem merge.
- **Conflito:** onboarding `tb init` sugere workflow centrado em `rd.md` + `.toolbelt/`, enquanto o repo manda usar skills (`initialize-worktree`, `branch-context`, `pushing-commits-to-the-repo`) e não empurrar a main (pre-commit `no-commit-to-branch`). O agente que seguir só o tb ignora o processo real.
- **O que tb deveria importar:** (1) paths de AGENTS.md hierárquicos; (2) comandos de `Makefile`/`docs/contributing.md` → `verify.test_cmd` / lint / typecheck; (3) restrições de scope das skills (narrow bugfix); (4) invariantes de qualidade do projeto (type-safety, sem `Co-Authored-By` de bot, etc.) para `tb mem`.

## Sugestões de melhoria

1. **[P0 · verify/init]** Auto-detect Python/`uv`/`make` e preencher `verify.test_cmd` (e lint) no `tb init`; nunca defaultar `go test` fora de repos Go.  
2. **[P0 · verify]** Se `--hidden-test-cmd` for passado, executar sempre; não amarrar a existência de `tests/hidden/`.  
3. **[P0 · mem/ctx]** `tb init`/`tb ctx`: ler AGENTS.md (e skills index) → invariantes + checklist de completion do projeto.  
4. **[P1 · guard]** Honrar `.toolbelt/guard.yaml` (ou documentar que é morto e remover).  
5. **[P1 · bench]** Usar `bench.default_agent` quando `--agent` omitido; adicionar `--agent` a `bench compare` (ou documentar só `--fake`/configs).  
6. **[P1 · drift/review]** Suportar globs `**` de verdade; docs com exemplos `dir/`, `dir/**`, paths literais.  
7. **[P1 · graph]** `impact` a partir do diff `base..head` sem `--file`; popular arestas `calls` em Python (hoje impact vazio).  
8. **[P2 · mcp]** Expor mem/verify/review/drift/guard no MCP com paridade CLI.  
9. **[P2 · docs]** Marcar comandos não implementados em `docs/cli.md` / README; alinhar árvore “planejada” vs `tb --help`.  
10. **[P2 · verify]** Ruler composto: `test_cmd` + `lint_cmd` + `typecheck_cmd` + opcional build, para não “passar” com checks parciais.  
11. **[P3 · ux]** Uniformizar posicionais (`graph search` vs `callers --function`); permissões de arquivos `.toolbelt` alinhadas ao umask do repo (não forçar 0600 se o time compartilha worktree).

## Métricas

| métrica | valor |
|---------|-------|
| Wall time (soma dos comandos cronometrados no log) | ~57 s de CPU-wall de invocações; sessão de estudo ~25–30 min incluindo clone/`make install`/leitura |
| Invocações tb (+ checks nativos no harness de evidência) | ~65 seções no log; ~53 comandos com EXIT/TIME parseáveis na primeira passagem estruturada |
| Falhas tb (exit ≠ 0) no log estruturado | 26 na amostra principal (inclui negativos esperados: flags ausentes, drift proposital, shallow HEAD~1 inicial, etc.) |
| Crash/segfault | nenhum observado |
| Tarefa entregue no alvo | sim (local only); evidência: pytest MessagesBuilder 3/3 pass; diff 2 arquivos |
