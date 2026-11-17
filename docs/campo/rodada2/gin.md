🇧🇷 Português | [🇺🇸 English](../../en/campo/rodada2/gin.md)

# Estudo de campo rodada 2: gin-gonic/gin

Estudo do grupo **without existing AI-agent instrumentation**, usando `tb` (toolbelt @ `a46c295`) de ponta a ponta num clone local de [gin-gonic/gin](https://github.com/gin-gonic/gin). Nenhum push, issue, PR ou comentário foi feito no repositório alvo.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | `gin-gonic/gin` |
| Linguagem | Go (`go 1.26.0` no `go.mod`) |
| Tamanho | shallow clone ~2,6 MiB; **100** arquivos `.go` após a tarefa; ~**24 579** LOC Go |
| Commit base do clone | `0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646` (master raso) |
| Instrumentação AI existente | **Nenhuma** — sem `AGENTS.md`/`CLAUDE.md`/`.cursorrules`/`.copilot*`/`.aider*`/`GEMINI.md` no tree upstream |
| Toolchain | **Parcialmente ok** |
| Grupo | without existing AI-agent instrumentation |

Toolchain detalhada:

- `go test ./...` — **ok** (~30 s 1ª corrida com download; ~2 s depois).
- `go build ./...` — **ok**.
- `go vet ./binding/...` — **ok**.
- `golangci-lint`: **não instalado** neste VM (`command not found`); não configurado.
- Browser headless (`google-chrome` em `/usr/local/bin/google-chrome`) presente; ruler visual exercitado com refs ausentes (skip agregado).

Não configurado: suite `tests/hidden/` (não existe no gin); golangci-lint.

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Preservar o **índice de entrada** em erros de validação de slices JSON (`[1]:` em vez de `[0]:` quando o 2º elemento falha) |
| Por quê | Bug realista e pequeno, com teste falhando documentado; distinto da rodada 1 |
| Issue | https://github.com/gin-gonic/gin/issues/4928 (há PR alheio #4929 — **não interagimos**; implementação independente só no clone local) |
| Completou? | **Sim**, no clone local |
| Diff principal | `binding/default_validator.go` (pré-aloca `SliceValidationError` com o tamanho do slice e grava erros na posição original); teste `TestJSONSliceValidationReportsInputIndex` em `binding/slice_index_validation_test.go` |
| Checks nativos | `go test ./binding/...` PASS; `go test ./...` PASS; `go build ./...` PASS; `go vet ./binding/...` PASS |
| Rodada 1 (comparação) | Rodada 1 usou issue #4296 — incluir nome do campo em erros de `BindUnmarshaler` / `encoding.TextUnmarshaler` |

## Verificação das 12 correções

| # | correção | funcionou/não funcionou/não testado/não se aplica | evidência |
|---|----------|-----------------------------------------------------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `TB_GRAPH=on tb graph index` a partir de `/tmp/target` (basename `target`) → **729** nós / **2573** arestas e `search ValidateStruct` acha `binding/default_validator.go`. Filho `target/` em `/tmp/target-skipprobe` continua ignorado (`Hello` vazio; `Root` ok). |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem diretório `tests/hidden/`; config com `hidden_test_cmd: go test ./binding/ -run TestDoesNotExistHiddenProbe -count=1` → JSON `hidden_available: true`, `hidden_passed: true`, `verdict: PASS`. |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff tocando `.toolbelt/thresholds.yaml`: `--json` sozinho → EXIT **0** com violations; `--json --strict` → EXIT **1** com o mesmo JSON de violação. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** (com ressalva de glob) | `.toolbelt/guard.yaml` com `secrets/*` → violação em `secrets/token`. Padrão `secrets/` (só barra final) **não** casa via `filepath.Match` — precisa `secrets/*` ou path literal. |
| 5 | init detecção de toolchain | **funcionou** | Em gin e em probe com só `go.mod`: `verify.test_cmd: go test ./...`, `hidden_test_cmd: ""`. Não preenche sozinho `build_cmd`/`lint_cmd` (ainda manuais). |
| 6 | review `tests/hidden` por path | **funcionou** | Diff só com comentário `// note about tests/hidden suite` em `default_validator.go` → findings `[]` (sem FP critical de invariante). |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `bench.default_agent: "echo agent-default"` → `tb bench run -r 1 ...` inicia `agent=external` sem exigir `--agent`/`--fake` (task `bugfix-go` 100%). `tb bench compare ... --agent 'echo hi'` aceito (EXIT 0 no toolbelt com configs A/B). |
| 8 | review `## Scope`/`## Escopo`, listas MD e `**` globs | **funcionou** (parcial) | Ticket **só Markdown** com `## Escopo` + listas → scope ok (sem `scope-missing`); `binding/**/*.go` no drift EXIT 0. Com **YAML frontmatter sem `scope:`**, o body `## Escopo` é **ignorado** (`LoadTicket` retorna cedo) → falso `scope-missing`. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | `build_ran/passed: true`, `lint_ran/passed: true` no `/tmp/verify-gin-r2-ok.json` com `go build ./...` e `go vet ./binding/...`. |
| 10 | init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents` | **funcionou** | Sem arquivos: log “use `--write-agents`”. Com `--write-agents`: cria `AGENTS.md` + `.toolbelt/agent_docs.yaml` (`sources: [AGENTS.md]`). Re-init posterior lê o existente (não sobrescreve). |
| 11 | graph Python `calls` edges | **não se aplica** (smoke auxiliar **funcionou**) | Gin é Go. Probe `/tmp/py-calls-probe/mod.py`: `edges_calls: 1`; `callers --function helper` → `mod.main`. |
| 12 | verify visual skipped agregado + docs CLI | **funcionou** | `.toolbelt/screens.yaml` com 2 telas sem refs → top-level `skipped: true`, `skip_reason: "todas as telas foram skipped"`, cada screen `skipped: true`. `docs/cli.md` documenta o comportamento (L762–763). |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init --write-agents` | EXIT 0; config Go + `AGENTS.md` semeado (mode 600) | **ajudou** | ~4 ms; `test_cmd=go test ./...`; agent_docs registrado |
| `tb mem init/add/list/check` | EXIT 0; 3 defaults + 1 regra; `compactacao_segura: true` | ajudou (leve) | sha256 estável em `invariants.yaml` |
| `tb graph` sem enable | (não re-probed em detalhe; ADR-011 mantido) | neutro | exige `--enable`/`TB_GRAPH` |
| `tb graph --enable index/rebuild/stats` | 729→733 nós após fix; ~35–45 ms | **ajudou** | index rápido no gin |
| `tb graph search ValidateStruct` | encontra `ValidateStruct`/`validateStruct` | ajudou | melhor que rodada 1 p/ este símbolo |
| `tb graph impact/deps --file binding/default_validator.go` | impact limitado (`Error`); deps de pkgs ok | neutro/parcial | impact pouco rico para o arquivo |
| `tb graph callers --function ValidateStruct` | `(vazio)` | atrapalhou | método em interface/`validate()` não ligado como calls úteis |
| ticket `rd.md` manual | criado à mão (sem `tb ticket`) | neutro | `## Escopo` + globs |
| `tb drift` | EXIT 0 no scope `binding/**/*.go` / `binding/` | **ajudou** | sem out_of_scope no diff da fix |
| `tb verify run` (tests+build+lint+hidden) | PASS completo | **ajudou** | build/lint/hidden agora executam |
| `tb verify` + guard em path do fix | FAIL imediato quando `guard.yaml` listava `binding/default_validator.go` | atrapalhou (se mal configurado) / ajudou (proteção) | `iterations: 0`, build nem roda |
| `tb verify --ruler visual` | PASS com `skipped: true` agregado | neutro | docs alinhadas; gin sem UI real |
| `tb review` (diff da fix, ticket MD) | EXIT 0; warning de aceite fraco possível | ajudou (parcial) | sem FP de `tests/hidden` |
| `tb review` + frontmatter YAML | falso `scope-missing` | **atrapalhou** | body Markdown ignorado |
| `tb guard check A..B --json/--strict` | limpo no diff da fix; viola thresholds/secrets | **ajudou** | sintaxe `base..head` (não `--base`) |
| `tb bench run` com `default_agent` | honra default; precisa tasks | ajudou | com `-t` do toolbelt: bugfix-go 100% |
| `tb bench compare --agent` | flag aceita; A/B roda | **ajudou** | regressão da rodada 1 corrigida |
| `tb mcp` + `TB_GRAPH=on` | init+tools/list ok; graph tools; sem mem/verify/review | neutro/parcial | `tb_mem_list` → ferramenta não encontrada |
| `tb version` (`make build`) | `a46c295` | neutro | |

## Problemas que continuam

Da rodada 1 (`docs/campo/gin.md`) que **ainda acontecem**:

| problema | status nesta rodada | evidência |
|----------|---------------------|-----------|
| MCP sem mem/verify/review/guard/drift | **continua** | `tools/list` só version/echo/env/run + graph_*; `tb_mem_list` erro |
| Flags posicionais inconsistentes no graph (`callers`/`deps`/`impact`) | **continua** (não revalidado à exaustão; API ainda flag-only no help) | help de callers exige `--function` |
| `graph callers` pouco útil / search incompleto para alguns símbolos | **continua** (parcial) | `callers ValidateStruct` vazio |
| Config files mode `0600` | **continua** | `stat` → `600` em `config.yaml` e `AGENTS.md` |
| `version` unknown sem ldflags | **não retestado** com build cru; `make build` ok | — |
| Docs CLI ainda listam comandos não implementados (`run`/`proc`/`ticket`/…) | **continua** (parcial) | árvore no topo de `docs/cli.md` ainda mostra planejados; seções Fase 2 melhoraram |
| Sem `tb ticket` / checklist de progresso | **continua** | ticket manual |
| Glob `**` / matching de paths protegidos pouco documentado na prática | **continua** (nova nuance) | `secrets/` ≠ `secrets/*` no guard |

Problemas da rodada 1 **corrigidos** nesta base: SkipDirs raiz; hidden sem dir; guard strict+json; guard.yaml; default_agent / compare --agent; build/lint no verify; review path match; Scope markdown (quando sem frontmatter); visual skipped; init toolchain + `--write-agents`.

## Problemas novos

1. **P1 — `LoadTicket` com frontmatter YAML ignora `## Scope`/`## Escopo` no body**  
   - Repro: `rd.md` com `---` frontmatter (title/base/forbidden) + seção `## Escopo` com listas → `tb review` emite `scope-missing`. Ticket só Markdown com a mesma seção funciona.  
   - Esperado: mesclar scope do body quando o YAML não declara `scope`.  
   - Atual: `parseTicketFrontmatter` retorna cedo sem chamar `parseMarkdownScope`.

2. **P2 — `protected_paths: secrets/` (diretório) não protege filhos**  
   - Repro: guard.yaml com só `secrets/`; commit `secrets/token` → `violations: null`. Com `secrets/*` ou `secrets/token` → violação.  
   - Esperado: convenção de diretório documentada ou prefix match.  
   - Atual: só `filepath.Match` literal.

3. **P2 — `bench.tasks_dir` absoluto no YAML pode ser sombreado pelo default da flag `-t`**  
   - Repro: config com `tasks_dir: /agent/repos/toolbelt/bench/tasks` mas `tb bench run` ainda loga `tasks_dir=bench/tasks` até passar `-t` explícito.  
   - Esperado: YAML local prevalecer quando a flag está no default.  
   - Atual: operador precisa repetir `-t`/`-f`.

4. **P3 — `tb guard check` UX `base..head` vs flags `--base` de review/drift**  
   - Repro: `tb guard check --base A --head B` → `unknown flag: --base`.  
   - Esperado: paridade de flags entre subcomandos de diff.  

5. **P3 — impact de arquivo Go pouco informativo**  
   - Repro: `impact --file binding/default_validator.go` lista só `binding.Error`.  
   - Esperado: callers de `ValidateStruct` / usos via `validate()`.  

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift de escopo ok no pacote `binding/` (incluindo glob `**` no ticket MD).
- Guard em `.toolbelt/thresholds.yaml` (default) e em `secrets/*` (yaml).
- Abort do verify quando o próprio arquivo da tarefa estava em `protected_paths`.
- Build + lint + testes + hidden_cmd no verify (PASS na fix).
- Review sem FP por menção textual a `tests/hidden`.
- Visual agregando `skipped: true` quando todas as telas skippam.
- Onboarding com `AGENTS.md` via `--write-agents` num repo sem instrumentação.

**Deixou passar / falhou**

- Scope Markdown escondido atrás de frontmatter YAML (falso warning / risco de aceitar drift).
- `protected_paths` com padrão de diretório “ingênuo” (`secrets/`).
- Call graph incompleto para o símbolo central da tarefa (`ValidateStruct`).
- MCP ainda não substitui o CLI para mem/verify/review/guard.
- Lint “de verdade” do gin (`golangci-lint`) — verify usou só `go vet` configurado manualmente; golangci ausente no VM.

Comparativo nativo: `go test ./...` e `go build ./...` validaram a fix; o tb verify alinhado com build/lint deixou de mascarar falha de compilação (melhoria clara vs rodada 1).

## Interação com instrumentação existente / onboarding

Gin **não tem** instrumentação de agente upstream. Avaliação do bootstrap `tb` nesta rodada:

| Artefato | Gerado / lido? | Qualidade |
|----------|----------------|-----------|
| `.toolbelt/config.yaml` | sim (`tb init`) | detecção Go ok (`test_cmd`); build/lint ainda manuais |
| `AGENTS.md` | sim com `--write-agents` | mínimo útil apontando verify/guard/mem; mode 600 |
| `.toolbelt/agent_docs.yaml` | sim | registra sources; `hints: []` (nada para extrair no gin) |
| `CLAUDE.md` | n/a (ausente) | init avisa para semear se quiser |
| invariantes `mem` | sim | defaults anti-cheat genéricos ainda pouco alinhados a lib HTTP sem suite oculta |
| Grafo | sob demanda | rápido; SkipDirs raiz corrigido |
| MCP | disponível | subset; graph com `TB_GRAPH=on` |

Conclusão: onboarding **melhorou** vs rodada 1 (toolchain + AGENTS), mas ainda não importa CI real do gin (workflow `.github/workflows/`) nem sugere `golangci-lint` automaticamente.

## Comparação com a rodada 1

| dimensão | rodada 1 | rodada 2 | veredito |
|----------|----------|----------|----------|
| Binário | `c8aae04` | `a46c295` | base com as 12 correções |
| Tarefa | #4296 BindUnmarshaler field name | #4928 índice de validação de slice | diferente (ok) |
| verify build/lint | ignorados | executados e PASS | **melhor** |
| hidden_test_cmd | bloqueado sem `tests/hidden/` | roda sem o dir | **melhor** |
| guard.yaml / strict+json | quebrados | funcionam | **melhor** |
| default_agent / compare --agent | quebrados | funcionam | **melhor** |
| review FP `tests/hidden` | confirmado | não reproduziu | **melhor** |
| visual skipped top-level | `skipped: false` enganoso | `skipped: true` | **melhor** |
| init AGENTS / toolchain | ausente / genérico | detecta Go + `--write-agents` | **melhor** |
| SkipDirs `/tmp/target` | 0 nós | 729 nós | **melhor** |
| MCP parity | incompleto | incompleto | **igual** |
| Scope + frontmatter | N/A (bug antigo outro) | gap novo de merge YAML+MD | **pior** neste canto |
| Wall time evidências CLI | ~5,2 min | ~3,8 min (`12:37:23Z`→`12:41:12Z`) | semelhante/ligeiramente **melhor** |
| Conclusão local da tarefa | sim | sim | **igual** |

No geral: **melhor** que a rodada 1 para o fluxo verify/guard/bench/init/visual/graph-raiz; restam MCP, ergonomia de ticket com frontmatter, e riqueza do grafo.

## Sugestões de melhoria

1. **[review] P0** — Ao carregar ticket com frontmatter, se `scope` YAML vazio, fazer merge com `parseMarkdownScope` do body (`## Scope`/`## Escopo`).
2. **[guard] P1** — Tratar padrões com `/` final (ou sem `*`) como prefixo de diretório; documentar exemplos `secrets/*` vs `secrets/`.
3. **[bench] P1** — Fazer `tasks_dir`/`fixtures_dir` do config local prevalecerem sobre defaults das flags cobra.
4. **[mcp] P1** — Expor mem/verify/review/guard/drift (parity CLI).
5. **[graph] P2** — Melhorar arestas `calls` para funções exportadas usadas via helpers (`validate` → `ValidateStruct`); enriquecer `impact --file`.
6. **[init] P2** — Sugerir `build_cmd`/`lint_cmd` a partir de CI/Makefile/`golangci` quando detectáveis.
7. **[cli] P2** — Unificar flags de intervalo de commits (`--base`/`--head` vs `A..B`) entre guard/review/drift.
8. **[config] P3** — Revisar mode 0600 em `AGENTS.md`/configs não secretas.

## Métricas

| métrica | rodada 2 | rodada 1 (ref.) |
|---------|----------|-----------------|
| Wall time (evidências CLI estruturadas) | **~3,8 min** (`2026-11-16T12:37:23Z` → `12:41:12Z`); sessão total do agente maior (docs+clone+implementação+probes) | ~5,2 min evidências |
| Invocações `tb` com `CMD:` no log | **48** (+ corridas auxiliares fora do prefixo) | ~50 |
| Falhas `tb` (EXIT ≠ 0) registradas | **7** (probes intencionais: guard strict, review drift/FP setup, flags guard erradas, etc.) | 11 |
| Crashes do binário | **0** | 0 |
| Build `tb` usado | `make build` → `toolbelt a46c295` | `c8aae04` |

Arquivos de evidência bruta neste VM (não commitados): `/tmp/campo-gin-r2-evidence.log`, `/tmp/verify-gin-r2*.json`, `/tmp/review-*.json`, `/tmp/verify-visual*.json`, clone em `/tmp/target/gin` (commits locais apenas).
