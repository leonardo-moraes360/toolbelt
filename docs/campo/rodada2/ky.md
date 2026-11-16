# Campo rodada 2: `tb` em sindresorhus/ky

Evidência coletada em 2026-11-16 num clone local em `/tmp/target` (sem push, issue, PR ou comentário no upstream). `tb` compilado deste repo em `main@a46c295` (`make build` → `/tmp/tb`). Única mudança de produto neste estudo: este arquivo. Trabalho no ky ficou restrito à VM.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [sindresorhus/ky](https://github.com/sindresorhus/ky) |
| Linguagem | TypeScript (ESM), HTTP client sobre Fetch |
| Tamanho | ~109 arquivos fora de `node_modules`/`.git`/`distribution`/`.toolbelt`; ~1,7 MiB; ~4,9 kLOC em `source/**/*.ts`, ~30,8 kLOC em `test/**/*.ts` |
| Instrumentação AI | `AGENTS.md` (convenção `null` vs `undefined`); sem `CLAUDE.md` / `.cursorrules` |
| Grupo | com instrumentação AI-agent existente |
| Toolchain | **Sim, com ressalvas.** Node v22.23.4 + npm 10.9.9; `npm install` + `npm run build` OK; `xo` nos arquivos tocados OK; `ava test/validation.ts` 18 passed; `ava 'test/!(browser).ts'` → **1528 passed**. Não rodei a matriz completa de browsers Playwright/`npm test` final (custoso). |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Rejeitar `null` em opções RequestInit enum-like (`credentials`, `mode`, `cache`, `redirect`) com `TypeError` claro que nomeia a opção (`The \`<option>\` option must not be \`null\`. Use \`undefined\` to clear it.`), alinhado a `AGENTS.md`; teste em `test/validation.ts`. |
| Por quê | Antes esses `null` vazavam erros opacos do `Request constructor` (ex.: `null is not an accepted type. Expected one of omit, same-origin, include`). Mantenedor-plausível e distinto da rodada 1. |
| Issue | Nenhuma issue aberta útil (`gh issue list --state open` vazio). Tarefa derivada do código + `AGENTS.md`. |
| Completa? | **Sim, localmente** (commits só no clone; não enviados ao upstream). |
| Checks nativos | `npm run build` OK; `xo` nos tocados OK; `ava test/validation.ts` 18 passed; `ava 'test/!(browser).ts'` 1528 passed; `tsc --project tsconfig.test.json --noEmit` OK via `hidden_test_cmd`. |
| Rodada 1 (comparação) | Rejeitar retornos inválidos de `retry.jitter` com `TypeError` + `cause`. **Não repetida** (upstream ainda faz fallback silencioso para o delay sem jitter — comportamento documentado nos testes atuais). |

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph index --enable .` em `/tmp/target` → `589 nós, 3802 arestas` (WALL 0,246 s). Na rodada 1 o mesmo path dava 0 nós. |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; config `hidden_test_cmd: npx tsc --project tsconfig.test.json --noEmit` → verify JSON `hidden_available: true`, `hidden_passed: true`. |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff tocando `source/index.ts` (em `guard.yaml`): sem `--strict` exit 0 com violations; com `--strict --json` **exit 1** + JSON de violation. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `source/index.ts` → violation `protected_path` nesse arquivo. Hardcoded `.toolbelt/thresholds.yaml` continua protegido. |
| 5 | init detecção de toolchain | **funcionou** (parcial) | `tb init` preencheu `verify.test_cmd: npm test` a partir de `package.json`. **Não** preencheu `build_cmd`/`lint_cmd` automaticamente (`build`/`xo` existem nos scripts). |
| 6 | review `tests/hidden` casa por path | **funcionou** | Ticket `/tmp/rd-hidden-mention.md` menciona `tests/hidden` e `thresholds.yaml` só em prosa; diff real não toca esses paths → `findings: []`. |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `bench.default_agent: echo AGENT {prompt}`; `tb bench run -r 1` sem `--agent`/`--fake` rodou (agent external). `tb bench compare ... --agent "echo AGENT {prompt}"` OK; compare sem `--agent` também usou default. |
| 8 | review parseia `## Scope`/`## Escopo`, listas MD e `**` | **funcionou** | `rd.md` com `## Scope` + bullets `` `source/core/**/*.ts` `` etc. → `tb drift`/`tb review` `out_of_scope: null`. Scope estreito só `test/validation.ts` → `out_of_scope: [rd.md, source/core/Ky.ts]`. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Configurados manualmente; verify JSON: `build_ran/passed: true`, `lint_ran/passed: true`, `verdict: PASS` (WALL 5,809 s). |
| 10 | init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents` | **funcionou** (parcial) | Init: `documentação de agentes encontrada sources=AGENTS.md` → `.toolbelt/agent_docs.yaml`. `hints: []` (AGENTS.md não tem dicas de run). `--write-agents` **não** sobrescreveu AGENTS.md existente. Em `/tmp/empty-agents` sem AGENTS.md, `--write-agents` semeou AGENTS.md mínimo do tb. |
| 11 | graph arestas `calls` em Python | **não se aplica** | Repo TypeScript; index JS/TS emitiu `edges_calls: 2875`. Não há código Python no ky para validar o indexador Python. |
| 12 | verify visual skipped agregado + docs CLI alinhados | **funcionou** | `tb verify run --ruler visual` → top-level `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. `docs/cli.md` documenta hidden sem exigir dir, build/lint, Scope MD, guard.yaml, default_agent, visual skipped. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; detectou npm; leu AGENTS.md | ajudou | WALL 0,004 s; `test_cmd: npm test`; `agent_docs.yaml`; config mode `600` |
| `tb init --write-agents` | não sobrescreve existente; semeia se ausente | ajudou | ky: AGENTS.md intacto; `/tmp/empty-agents`: AGENTS.md gerado |
| `tb mem init/list/add/check` | OK; flags docs `--type/--title/--body` falham | neutro/atrapalhou | `unknown flag: --type`; invariantes default ainda Go-centric (`tests/hidden`) |
| `tb graph index --enable .` em `/tmp/target` | 589 nós / 3802 arestas | ajudou | SkipDirs na raiz corrigido |
| `tb graph impact/deps/search` | impact com path do repo OK se root=`.`; `#calculateDelay` ausente | misto | `source/core/Ky.ts` hit com root `.`; com root `source`, `--file source/core/Ky.ts` → vazio / `--file core/Ky.ts` → hit |
| `tb verify run` (tests) | PASS com build+lint+visible+hidden | ajudou | JSON completo; hidden sem dir |
| `tb verify run --ruler visual` | PASS skipped top-level | ajudou (vs R1) | `skipped: true` (antes `skipped: false` com screens skipped) |
| ticket / Scope | `rd.md` manual; sem `tb ticket` | neutro | docs marcam ticket como planejado |
| `tb review` | findings vazios no caso feliz; Scope MD OK | ajudou | exit 0; FP `tests/hidden` por substring sumiu |
| `tb drift` | OK; globs `**` no Scope | ajudou | OOS detectado com scope estreito |
| `tb guard check` | yaml + strict JSON OK; API `base..head` | ajudou / UX mista | diferente de `--base/--head` do verify/review |
| `tb bench run/compare` | default_agent e `--agent` OK | ajudou | fixtures copiadas do toolbelt (ky não tem bench/) |
| `tb mcp` | stdio; tools/list + `tb_version` OK | neutro | ainda sem tools verify/review/drift/guard/mem |
| `tb version` | com ldflags `a46c295`; sem → `0.1.0-dev` + unknown | neutro | esperado sem `-ldflags` |

## Problemas que continuam

Da rodada 1 (`docs/campo/ky.md`) que ainda apareceram:

1. **Métodos privados `#foo` não indexados** — `graph search/callers` de `calculateDelay` → vazio.
2. **Paths do grafo relativos ao root indexado** — indexar só `source` ainda exige `--file core/Ky.ts`, não `source/core/Ky.ts`.
3. **`review`/`drift` sem shorthand `-o`** — `unknown shorthand flag: 'o'` (verify aceita `-o`).
4. **MCP sem verify/review/drift/guard/mem** — só echo/env/run?/graph_*/version.
5. **`version` unknown sem ldflags** — confirmado.
6. **Config files mode `0600`** — `config.yaml` e `agent_docs.yaml` em `600`.
7. **`mem add` só posicional** — flags da docs antiga (`--type/--title/--body`) inexistentes.
8. **Sem `tb ticket`/board/progress** — agente monta `rd.md` na mão; docs agora avisam “não implementado”.
9. **Invariantes default Go-centric** — `mem init` ainda cria regras `tests/hidden` / thresholds num repo TS (não geram mais FP no review por substring, mas continuam ruído de onboarding).
10. **Init não importa regras de `AGENTS.md` como invariantes** — só registra path; `hints: []`.

## Problemas novos

### P2 — `tb init` não preenche `build_cmd`/`lint_cmd` a partir de `package.json`

- **Repro:** `tb init` num repo com scripts `build` e lint via `xo`/`npm test` que já inclui xo.
- **Esperado:** sugerir `build_cmd`/`lint_cmd` (ou documentar que só `test_cmd` é auto).
- **Atual:** só `verify.test_cmd: npm test`; build/lint exigem edição manual do yaml.

### P2 — `guard check` usa `base..head` posicional, verify/review/drift usam `--base`/`--head`

- **Repro:** `tb guard check --base A --head B` → `unknown flag: --base`. Forma correta: `tb guard check A..B`.
- **Esperado:** flags alinhadas entre subcomandos.
- **Atual:** inconsistência de UX; atrito no protocolo de campo.

### P3 — `agent_docs.yaml` com `hints: []` mesmo com `AGENTS.md` substancial

- **Repro:** init no ky → sources=[AGENTS.md], hints=[].
- **Esperado:** extrair pelo menos a convenção null→TypeError como dica/invariante, ou documentar que só extrai blocos de “como rodar”.
- **Atual:** leitura nominal sem valor operacional para este AGENTS.md.

### P3 — Bench em repo alvo exige copiar `bench/tasks` do toolbelt

- **Repro:** ky não tem `bench/tasks`; `tb bench run` só funcionou após copiar fixtures.
- **Esperado:** erro claro ou scaffold mínimo no init.
- **Atual:** depende de material externo ao repo sob teste.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift real quando Scope omitia `source/core/Ky.ts` e `rd.md`.
- Guard em `protected_paths` de `guard.yaml` e em `.toolbelt/thresholds.yaml`.
- Verify falharia se build/lint/test/hidden quebrassem (todos rodaram e passaram).
- Review sem FP de `tests/hidden` por menção no ticket.
- Graph indexável em `/tmp/target`; deps/impact úteis com root `.`.
- Visual report com `skipped: true` quando não há manifesto.

**Deixou passar / atrapalhou**

- Não extraiu a regra de `AGENTS.md` para invariantes (a tarefa foi escolhida lendo AGENTS.md fora do tb).
- Não indexou `#calculateDelay` / callers internos.
- Não sugeriu `build_cmd`/`lint_cmd` no init.
- MCP não cobre o loop verify/review.
- Não empurrou completude além do que o agente configurou no `rd.md`/verify.

## Interação com instrumentação existente / onboarding

- **`AGENTS.md`:** `tb init` **detecta e registra** em `.toolbelt/agent_docs.yaml` (melhoria vs rodada 1). Não mescla o conteúdo nas invariantes de `mem`. `hints: []`.
- **`--write-agents`:** respeita AGENTS.md existente; útil para repos sem instrumentação.
- **Duplicação residual:** `mem init` ainda injeta anti-cheat Go/toolbelt genérico ao lado das convenções do ky.
- **Onboarding:** detecta npm → `test_cmd`; não importa scripts `build`/`xo`; não cria ticket. Workflow ainda depende de `rd.md` manual + yaml editado.

## Comparação com a rodada 1

| Aspecto | Rodada 1 | Rodada 2 | Veredito |
|---------|----------|----------|----------|
| SkipDirs `/tmp/target` | 0 nós | 589 nós | **melhor** |
| hidden_test_cmd | só com dir `tests/hidden/` | roda sem o dir | **melhor** |
| build/lint no verify | ausentes | `build_ran`/`lint_ran` true | **melhor** |
| guard.yaml | ignorado | lido + strict JSON exit 1 | **melhor** |
| default_agent / compare --agent | quebrados | funcionam | **melhor** |
| review FP `tests/hidden` | confirmado | refutado neste run | **melhor** |
| Scope `##` + `**` | frágil / parcial | funcionou neste ticket | **melhor** |
| visual skipped top-level | `skipped: false` | `skipped: true` | **melhor** |
| AGENTS.md | ignorado | lido (hints vazios) | **melhor parcial** |
| `#private` no graph | ausente | ausente | **igual** |
| MCP cobertura | só graph/echo/version | igual | **igual** |
| `-o` em review/drift | ausente | ausente | **igual** |
| Wall tb instrumentado | ~40,7 s | ~6,2 s (menos probes lentos; verify 5,8 s) | **melhor** (carga diferente) |

No geral: **claramente melhor** nas 12 correções aplicáveis; problemas estruturais de graph/MCP/ticket permanecem.

## Sugestões de melhoria

1. **[P1][init]** Auto-preencher `build_cmd`/`lint_cmd` a partir de scripts `package.json` (além de `test_cmd`).
2. **[P1][init/mem]** Extrair bullets de convenção de `AGENTS.md` → invariantes (não só path em `agent_docs.yaml`).
3. **[P1][guard]** Aceitar `--base`/`--head` como verify/review/drift.
4. **[P2][graph]** Indexar métodos `#private` TS; normalizar `--file` ao root do repo.
5. **[P2][cli]** Shorthand `-o` em review/drift; scaffold `bench/tasks` no init ou erro acionável.
6. **[P2][mcp]** Expor verify/review/drift/guard/mem.
7. **[P3][config]** `0644` para yaml não-secretos (ou documentar 0600).
8. **[P3][mem]** Defaults de invariantes por toolchain detectada (não sempre `tests/hidden`).

## Métricas

| métrica | rodada 2 | rodada 1 (ky) |
|---------|----------|---------------|
| Wall time soma `*.time` (tb+nativo) | ~37,5 s | ~66,8 s |
| Wall time só tb (timed) | ~6,2 s | ~40,7 s |
| Invocações tb timed | 24 | ~61 |
| Falhas tb intencionais (probes) | várias (guard strict, drift OOS, flags docs, graph off, `-o`) | 14 |
| Crashes do binário | 0 | 0 |
| Native `ava 'test/!(browser).ts'` | 1528 passed / ~31,3 s | 1528 passed |
| `tb` versão | `a46c295` | `c8aae04` |

Arquivos brutos de evidência (VM): `/tmp/tb-evidence-r2/`.
