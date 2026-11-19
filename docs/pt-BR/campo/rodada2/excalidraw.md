[🇺🇸 English](../../../campo/rodada2/excalidraw.md) | 🇧🇷 Português

# Campo rodada 2: excalidraw/excalidraw

Estudo em 2026-11-16 com `tb` de `toolbelt@a46c295` (`make build` → `./bin/tb`). Clone local em `/tmp/target/excalidraw` (shallow). Grupo: **with existing AI-agent instrumentation**. **Nenhuma** interação com o remoto do Excalidraw. Evidências em `/tmp/campo2-evidence/`.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [excalidraw/excalidraw](https://github.com/excalidraw/excalidraw) |
| Linguagem | TypeScript / React (monorepo Yarn workspaces) |
| Tamanho | ~1375 arquivos TS/JS (sem `node_modules`); checkout shallow ~93 MiB; com `node_modules` ~1.1 GiB |
| Commit base | `82190fb` (`master` shallow) |
| Instrumentação AI | `AGENTS.md` (DOM/`ownerDocument`, `Merge<>`); `CLAUDE.md` (estrutura monorepo, `yarn test:typecheck` / `yarn test:update` / `yarn fix`) |
| Toolchain | **Configurada**: Node 22, Yarn 1.22.22, `yarn install --frozen-lockfile` OK, Vitest 3.0.6, `tsc`, ESLint, Chrome headless (`/usr/local/bin/google-chrome`) |
| Não configurado | App no browser / Playwright E2E de UI; `yarn build` de produção; suíte `yarn test:all` completa (typecheck+eslint nos arquivos da tarefa e vitest math foram rodados) |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Slice 1 de [#9297](https://github.com/excalidraw/excalidraw/issues/9297): adicionar tipos `ViewportPoint` e `GenericPoint` em `@excalidraw/math`, helper `viewportPointFrom`, testes em `packages/math/tests/point.test.ts`. Migração completa dos `extends GlobalPoint \| LocalPoint` → `GenericPoint` **adiada** (typecheck do monorepo quebra quando `ViewportPoint` entra nas constraints existentes). |
| Por quê | Feature pequena/média, isolada no pacote math, com issue real e testes unitários; diferente da rodada 1. |
| Issue | https://github.com/excalidraw/excalidraw/issues/9297 |
| Completada? | **Sim, localmente** (commit `619250f` no clone; **não** enviado ao upstream). Tentativa ampla de trocar constraints gerou ~29 erros `tsc` (utils/element/excalidraw); slice tipado+helper passou. |
| Checks nativos | `yarn vitest run packages/math/tests` → 9 files / 60 tests PASS (~2.7–3.0 s). `yarn test:typecheck` → PASS (~14–15 s). ESLint nos arquivos tocados: 1 warning Prettier no 1º draft; após `--fix` → PASS. |
| Rodada 1 (comparação) | Rodada 1: bugfix `normalizeRadians` → `0` em múltiplos de 2π (issue #12599). Rodada 2: tipos `ViewportPoint`/`GenericPoint` (#9297). |

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Clone em `/tmp/target/excalidraw`; `TB_GRAPH=on tb graph rebuild --enable` → 4453–4454 nós / ~18471 arestas (~1.47 s), não grafo vazio (`09-graph-rebuild.txt`) |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; `--hidden-test-cmd "yarn vitest run …point.test.ts"` → `hidden_available: true`, `hidden_passed: true`, verdict PASS (`31b-hidden-no-dir.txt`). Com `false` → `hidden_available: true`, `hidden_passed: false`, ESCALATE (`31c`) |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Com `.toolbelt/guard.yaml` protegendo `types.ts` (alterado no diff): sem `--strict` exit 0 + violations; com `--strict --json` exit **1** + JSON válido (`29b`/`30b`) |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | YAML com `packages/math/src/types.ts` → violação `protected_path` nesse arquivo; sem YAML → `violations: null` no mesmo diff (`28`/`29`) |
| 5 | init detecção de toolchain | **funcionou** (parcial útil) | `tb init` → `verify.test_cmd: yarn test` (não mais `go test`); não preencheu sozinho `build_cmd`/`lint_cmd` a partir dos scripts/`CLAUDE.md` (`02-init.txt`) |
| 6 | review `tests/hidden` por path | **funcionou** | `rd.md` menciona `tests/hidden/**` em Forbidden; diff da tarefa não toca esse path → `findings: []` (`27-review.txt`, `40-review-no-fp-hidden.txt`) |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `bench.default_agent: echo default-agent-used` + `tb bench run -r 1 --task bugfix-go` (sem `--agent`) iniciou `agent=external` e concluiu (`33b`). `tb bench compare … --agent "echo compare-agent"` aceitou a flag e rodou A/B (`35b`). Help documenta `-a/--agent` |
| 8 | review/drift `## Scope`/`## Escopo`, listas MD e `**` globs | **funcionou** | Ticket com `## Escopo` + bullets + `packages/math/tests/**` e `packages/math/src/**/*.ts`; drift/review limpos no commit da tarefa; commit de `CONTRIBUTING.md` → `out_of_scope: ["CONTRIBUTING.md"]` (`26`, `39b`) |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Config com `build_cmd: yarn test:typecheck` e `lint_cmd: eslint…`; 1º verify → lint falhou (Prettier), `lint_ran/passed`, `build_ran/passed`, verdict ESCALATE (`23`). Após fix → PASS com build+lint+visible+hidden (`25`) |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** | Init logou sources + hints; gravou `.toolbelt/agent_docs.yaml`. `--write-agents` no Excalidraw **não** sobrescreveu `AGENTS.md`. Em repo vazio semeou `AGENTS.md` mínimo (`02`, `02c`, `38`) |
| 11 | graph Python `calls` | **não se aplica** | Repo TS/React; arestas `calls` JS presentes no stats (`edges_calls: 8871`). Não exercitado o indexador Python |
| 12 | visual skipped agregado + docs CLI alinhados | **funcionou** | Manifesto sem refs → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"` (`32-visual-skipped.txt`). `docs/cli.md` documenta hidden sem dir, build/lint, `default_agent`, ticket como planejado, visual skipped (`41`) |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` (`make build`) | `toolbelt a46c295` com commit/built | neutro | `01-version.txt` |
| `go run ./cmd/tb version` | `0.1.0-dev`, commit/built `unknown` | atrapalhou (sem ldflags) | `48-version-norflags.txt` |
| `tb init` | `.toolbelt/` + `yarn test`; leu AGENTS/CLAUDE; mode `600` | ajudou (onboarding bem melhor que rodada 1) | `02-init.txt` WALL 0.006 s |
| `tb init --write-agents` | não sobrescreve existente; semeia se ausente | ajudou | `02c`, `38` |
| `tb mem init/add/list/check` | OK; invariantes + sha256 | ajudou | `03`–`06` |
| Ticket (`rd.md` manual) | `tb ticket` ainda inexistente (docs: Fase 3+) | neutro (fluxo manual documentado) | `41`, `08-rd.md` |
| `TB_GRAPH=on tb graph rebuild` | ~4454 nós / ~18474 arestas | ajudou | `09`, `42` |
| `tb graph search viewportPointFrom` | achou a função nova | ajudou | `43` |
| `tb graph callers --function pointFrom` | callers intra-`packages/math` (curve/point/segment); pouco cross-package | atrapalhou parcialmente | `45` |
| `tb graph impact --file point.ts` | impacto limitado ao pacote math | atrapalhou parcialmente | `46` |
| `tb verify run` (build+lint+test+hidden) | FAIL→PASS; pegou Prettier; hidden sem dir | ajudou | `23`, `25`, `31b` |
| `tb verify --ruler visual` | skipped top-level honesto | ajudou | `32` |
| `tb review rd.md` | sem findings; sem FP `tests/hidden` | ajudou | `27`, `40` |
| `tb drift rd.md` | limpo na tarefa; detectou OOS; ainda imprime Usage após JSON | ajudou / UX atrapalha | `26`, `39b` |
| `tb guard check` + `guard.yaml` + `--strict --json` | YAML honrado; strict exit 1 | ajudou | `28`–`30b` |
| `tb bench run` via `default_agent` | funciona sem `--agent` | ajudou | `33b` |
| `tb bench compare --agent` | flag aceita; A/B OK | ajudou | `35b` |
| `tb mcp` + `tools/list` | initialize + tools (env/run/graph/…) | ajudou | `37b-mcp.txt` |
| Checks nativos | vitest/tsc/eslint conforme acima | ajudou (baseline) | `17`–`22`, `49`–`50` |

## Problemas que continuam

- **Callers/impact fracos cross-package no monorepo TS** (P2 rodada 1): `pointFrom` / `types.ts` ainda não listam consumidores em `packages/element` / `excalidraw` de forma útil (`45`, `46`, e busca `GlobalPoint` só achou matches de substring em nomes de função — `11`).
- **`version` → `unknown` sem ldflags** (`48`).
- **Configs mode `0600`** (`02-init.txt` stat).
- **Usage do Cobra após JSON** em `tb drift` com drift (`39b`: JSON + `Error:` + Usage).
- **Init não promove hints de `CLAUDE.md` a `build_cmd`/`lint_cmd`**: só loga hints e seta `test_cmd: yarn test`; typecheck/eslint ainda exigem edição manual do YAML (gap residual do onboarding).
- **`tb ticket` não implementado** (docs agora marcam como planejado — melhoria de expectativa, comando continua ausente).

## Problemas novos

### P2 — Hints de AGENTS/CLAUDE não viram `verify.build_cmd`/`lint_cmd`

- **Repro**: `tb init` no Excalidraw (há `yarn test:typecheck` / `yarn fix` no `CLAUDE.md`).
- **Esperado**: pelo menos sugerir/preencher build/lint além de `yarn test`.
- **Atual**: `agent_docs.yaml` com hints; `config.yaml` só com `test_cmd: yarn test` (`02-init.txt`).

### P3 — `bench.tasks_dir` absoluto no YAML pode ser ignorado conforme cwd

- **Repro**: em `/tmp/bench-probe`, config com `tasks_dir: /agent/repos/toolbelt/bench/tasks`; `tb bench run -r 1`.
- **Esperado**: abrir o path absoluto.
- **Atual**: log `tasks_dir=bench/tasks` relativo e falha `open bench/tasks` (`33-bench-default.txt`). No repo toolbelt (cwd certo) o default_agent funciona.

### P3 — Regressão de expectativa: verify “PASS” ainda não significa “tarefa maintainer-completa”

- Não é regressão de código das 12 fixes (build/lint agora funcionam se configurados), mas o default pós-init ainda passa sem typecheck/eslint do Excalidraw. Agente precisa lembrar de espelhar `CLAUDE.md` no YAML.

Nenhuma regressão observada nas 12 correções listadas quando exercitadas de propósito.

## O que o tb pegou e o que deixou passar

**Pegou**

- Warning Prettier no helper novo via `lint_cmd` (verify ESCALATE até corrigir).
- Drift de `CONTRIBUTING.md` fora do `## Escopo`.
- Path protegido via `guard.yaml` + exit 1 com `--strict --json`.
- Suíte hidden configurada sem diretório `tests/hidden/`.
- Visual com todas as telas skipped → `skipped: true` no topo.
- Indexação do monorepo sob `/tmp/target` e localização de `viewportPointFrom`.
- Leitura de `AGENTS.md`/`CLAUDE.md` no init; ausência de FP de review por mencionar `tests/hidden`.

**Deixou passar**

- Consumidores cross-package das APIs de ponto (grafo).
- Preenchimento automático de typecheck/eslint no config (só hints).
- Completude “maintainer” se o agente aceitar só o default `yarn test` pós-init.
- A tentativa falha de migrar todos os generics para `GenericPoint` (limitação da tarefa/typecheck do monorepo — o tb não analisa esse risco de API).

## Interação com instrumentação existente / onboarding

- **Melhora clara vs rodada 1**: `tb init` detecta Yarn, grava `agent_docs.yaml`, extrai comandos do `CLAUDE.md`, e `--write-agents` respeita arquivo existente.
- **Ainda incompleto**: hints não viram `build_cmd`/`lint_cmd`; invariantes default do `mem init` continuam genéricas (`tests/hidden`, `thresholds.yaml`) e não importam bullets do `AGENTS.md` (`ownerDocument`, `Merge<>`).
- **Sem conflito destrutivo**: não sobrescreveu `AGENTS.md`/`CLAUDE.md`.
- **O que ainda faltaria importar**: (1) mapear hints → `verify.*`; (2) bullets do `AGENTS.md` → `tb mem` mandate; (3) workspaces Yarn para defaults de graph/impact.

## Comparação com a rodada 1

| Aspecto | Rodada 1 (`c8aae04`) | Rodada 2 (`a46c295`) |
|---------|----------------------|----------------------|
| init / toolchain | `go test` em repo Yarn | `yarn test` + leitura AGENTS/CLAUDE |
| verify | só test_cmd; hidden morto sem dir; PASS sem tsc | build/lint/hidden honrados; pegou Prettier |
| guard.yaml / strict JSON | ignorado; strict+json exit 0 | honrado; exit 1 |
| review FP `tests/hidden` | confirmado | ausente neste ticket |
| Scope Markdown | frágil / frontmatter | `## Escopo` + globs OK |
| bench default_agent / compare `--agent` | quebrados | funcionam |
| visual skipped top-level | mentiroso | `skipped: true` |
| graph em `/tmp/target` | risco SkipDirs (outros repos) | indexou normalmente |
| callers cross-package TS | fraco | ainda fraco |
| docs CLI | desatualizados | alinhados nos pontos checados |

**Veredito:** claramente **melhor** nas 12 correções priorizadas; dores estruturais de grafo TS monorepo e de “empurrar completude” a partir da instrumentação nativa **permanecem**.

## Sugestões de melhoria

1. **[P1 · init/verify]** Aplicar hints de `CLAUDE.md`/`package.json` a `build_cmd`/`lint_cmd` (ou checklist interativa), não só log + `test_cmd`.
2. **[P1 · mem/init]** Importar bullets de `AGENTS.md` como invariantes `mandate`.
3. **[P2 · graph]** Resolver callers/impact cross-package com path aliases / workspaces Yarn (TS).
4. **[P2 · bench]** Honrar `tasks_dir`/`fixtures_dir` absolutos no config independente do cwd.
5. **[P3 · cli]** Em `--json`, não imprimir Usage do Cobra após erro (`drift`/`review`).
6. **[P3 · verify]** Profile `js-monorepo` que combine test+typecheck+lint a partir de scripts conhecidos.

## Métricas

| métrica | rodada 2 | rodada 1 (ref) |
|---------|----------|----------------|
| Wall time somado (amostra cronometrada tb + nativos no log) | ~103 s (inclui typecheck falho da tentativa ampla + verify×2) | ~49 s |
| Invocações tb (aprox., com probes das 12 fixes) | ~55–70 | ~45–55 |
| Evidência files em `/tmp/campo2-evidence` | 68 | — |
| Falhas tb observadas | verify ESCALATE (lint Prettier) até fix; guard strict exit 1 (esperado); drift OOS; bench cwd/tasks; compare sem before/after | drift/review/guard/bench bugs de produto |
| Graph rebuild | ~1.47 s / 4453 nós / 18471 arestas | ~1.05 s / 4452 / 18465 |
| Verify PASS final (build+lint+test+hidden) | ~20.2 s | ~2.9 s (só vitest) |
| Vitest math | ~3.0 s, 60 tests PASS | ~2.8 s, 64 tests |
| `tsc` (slice OK) | ~14.8 s PASS | ~13.6 s PASS |
| MCP `tools/list` | OK | OK |
| Bench fake (toolbelt, 9 tasks) | sucesso 33.3% (baseline fake) | idem |

**Nota:** a tarefa foi fechada com typecheck+eslint+vitest; o tb só “fechou” o loop depois de configurarmos `build_cmd`/`lint_cmd`/`hidden_test_cmd` manualmente — avanço real vs rodada 1, ainda dependente do agente espelhar a instrumentação existente.
