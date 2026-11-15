# Estudo de campo: commander.js

Grupo: **sem instrumentação de agente de IA** (além de política humana).  
Alvo local: `/tmp/target` (clone shallow de https://github.com/tj/commander.js).  
Binário: `tb` buildado com `make build` em `c8aae04` (`./bin/tb`).  
Data: 2026-11-15.  
Regra: nenhum push/issue/PR/comentário no upstream; só clone local.

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | `tj/commander.js` (npm: `commander` 15.1.0) |
| Linguagem | JavaScript (ESM) + TypeScript typings (`typings/`), testes em Node test runner |
| Tamanho | ~2.2 MiB no clone shallow; ~217 arquivos versionados; ~18 k LoC `.js` (incl. examples/tests); suite nativa **1430** testes (baseline) |
| Instrumentação de agente | **Ausente** como tooling de agente: sem `AGENTS.md`, `.cursor/`, `.cursorrules`, Copilot instructions, etc. Existe `AI_POLICY.md` (política humana: AI ok com humano no loop; proíbe PRs autônomos/gerados em massa) + menção no `CONTRIBUTING.md` |
| Toolchain | **OK**: Node `v22.23.4` (≥22.12), npm `10.9.9`, `npm install` (214 pkgs), `npm test` e `npm run check` (type+lint+format) verdes no baseline. Não configurei browser/Chromium extra para ruler visual além do Chrome já presente em `/usr/local/bin/google-chrome`. |

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Issue [#2683](https://github.com/tj/commander.js/issues/2683): (1) `Help.groupItems` de `Map` → record (`Object.hasOwn` + `Object.entries`); (2) getter `Command.alias()` tipado/documentado como podendo retornar `undefined`; teste cobrindo getter sem alias |
| Por quê | Bug/compat pequeno, escopo claro, maintainer-plausível, com tipagens + teste; encaixa small-to-medium |
| Issue | https://github.com/tj/commander.js/issues/2683 (somente leitura) |
| Completa? | **Sim, localmente** (commit `3383634` na branch local `campo-local-fix` do clone; **não** enviado ao upstream) |
| Checks nativos | Após a correção: `npm test` → pass (1431 testes); `npm run check` → pass (type/lint/format). Probes laterais (lint-only / `tests/hidden`) foram revertidos com `git reset --hard` para o commit da tarefa |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` (make build) | OK; versão=`c8aae04` | neutro | `EXIT=0` ~6 ms; com ldflags do Makefile |
| `go build` sem ldflags → `version` | OK; `commit/built: unknown`, `0.1.0-dev` | atrapalhou (DX) | `/tmp/tb-nolink version --json` → `"commit":"unknown","build_time":"unknown"` |
| `tb init` | OK; criou `.toolbelt/` | neutro→atrapalhou | `EXIT=0` 5 ms; `config.yaml` mode **600**; `verify.test_cmd` default **`go test ./...`** em repo JS; `default_agent: ""` depois editado manualmente; sem AGENTS.md / detecção de scripts npm |
| `tb mem init/list/check/add` | OK para fluxo real (`add <regra>`); flags da doc falham | ajudou (invariantes) / atrapalhou (docs) | `mem add "…"` OK; `mem add --type fact …` → `unknown flag: --type`; `check` hash+retenção 1.00; arquivo mode **600** |
| Ticket / Scope (`rd.md` manual) | OK | ajudou | Sem `tb ticket` no binário; `rd.md` com `base_commit` + `scope:` literal; drift/review consumiram |
| `tb graph` sem `--enable`/`TB_GRAPH` | Erro esperado | neutro | `grafo desabilitado (… ADR-011)` |
| `tb graph --enable rebuild` / `index .` | **0 nós** | atrapalhou | Clone em `/tmp/target`: `SkipDirs` inclui nome `"target"` → raiz inteira em `SkipDir` (repro unitário: dirs=1 skippedDir=1) |
| `tb graph --enable index lib` | 46 nós / 631 arestas | ajudou parcial | Indexação por subdir funciona; `search alias` / `callers --function alias` / `impact --file …` → `(vazio)` após index útil |
| `tb graph impact` sem `--file` | Erro | neutro | `--file é obrigatório` (EXIT=1) |
| `tb graph query` | Inexistente | atrapalhou (docs/expectativa) | Cai no help do `tb graph`; subcomando não existe |
| `tb verify run` (config npm) | PASS quando `tests/hidden` ausente | ajudou parcial | Com `verify.test_cmd: npm test` → PASS ~5.3 s; **`hidden_test_cmd` só roda se existir dir `tests/hidden/`** — senão `disponível: false` e ignora o comando configurado |
| `tb verify` + probe lint-only | PASS indevido | atrapalhou | `npm run check:lint` FAIL (`__campoLintProbe` unused); `tb verify --test-cmd npm test` → **PASS** (não roda eslint) |
| `tb verify` + diff em `tests/hidden` | FAIL iter=0 | ajudou (anti-cheat) | Guard embutido aborta antes dos testes: `Veredicto: FAIL (iterações: 0)` |
| `tb verify --ruler visual --json` | PASS skipped | neutro | `"verdict":"PASS","skipped":true,"skip_reason":"manifesto de telas ausente"`; Chrome detectado |
| `tb review` (diff da tarefa) | OK, 0 findings | ajudou | Fresh=true, 3 goldfish, sem drift no scope declarado |
| `tb review` (probe `tests/hidden`) | Findings critical/error | ajudou | Detectou fora de scope + invariante `tests/hidden/` (verdadeiro positivo) |
| `tb drift` | OK no scope; detecta fora | ajudou | Scope completo → OK; `rd-narrow` (só typings) → 4 fora do scope, EXIT=1 |
| `tb guard check` | Hardcoded `tests/hidden` sim; `guard.yaml` não | atrapalhou | Custom `.toolbelt/guard.yaml` protegendo `lib/help.js` **ignorado**; `--manifest` **unknown flag**; toque em `tests/hidden/sentinel.js` → `error protected_path …` |
| `tb bench run/list` no alvo | Falha sem tasks locais | neutro | `open bench/tasks: no such file`; `--agent` obrigatório mesmo com `default_agent` no config |
| `tb bench run --fake` + tasks do toolbelt | OK 33.3% | neutro | `-t /agent/repos/toolbelt/bench/tasks` ~3.3 s; **não** precisou `--fake-config` |
| `tb bench compare` | A/B `--fake` OK; sem `--agent` | neutro/atrapalhou | Compare configs graph-off/on OK; `compare --agent …` → `unknown flag: --agent` |
| `tb mcp` | OK stdio JSON-RPC | ajudou parcial | `tools/list` + `tools/call` `tb_version` responderam; graph tools listadas com `TB_GRAPH=on` |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | Help de `compare` sem `--agent`; `tb bench compare --agent "echo hi" …` → `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmado** | Arquivo com `protected_paths: lib/help.js` não gerou violação no diff que altera `lib/help.js`; só paths hardcoded (ex. `tests/hidden`) |
| Outdated CLI docs | **confirmado** | `docs/cli.md`: `mem add --type/--title`, `tb serve`, `guard snapshot/protect`, globs de manifesto; CLI real: `mem add <regra>`, `tb mcp`, só `guard check` |
| version shows `unknown` | **confirmado** (build sem ldflags) | `/tmp/tb-nolink version` → commit/built `unknown`. Com `make build`, mostra hash (não “unknown”) |
| config files created 0600 | **confirmado** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → mode **600** |
| visual ruler reports `skipped=false` when skipping | **refutado** | JSON: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` |
| `default_agent` ignored | **confirmado** | Config com `default_agent: echo agent-default`; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** (para `tests/hidden`) | Diff com `tests/hidden/sentinel.js` → verify FAIL iter=0; `guard check --strict` EXIT=1 |
| default fake agent needs `--fake-config` | **refutado** (caminho feliz) | `tb bench run --fake` com tasks do toolbelt concluiu sem `--fake-config`. Flag existe e `fake-agent --config` é opcional |
| `mem add` positional | **confirmado** | Usage: `tb mem add <regra>`; docs com flags vault estão erradas |
| Drift/Scope need literal paths (no globs) | **refutado** (globs simples) | Código usa `filepath.Match` e `/**`; `scope: lib/*.js` aceitou `lib/help.js`/`lib/command.js` (só tipagens/teste ficaram fora) |
| `graph impact` needs `--file` | **confirmado** | Sem flag → `--file é obrigatório` |
| review false positives on tests/hidden invariant | **refutado** neste run | Review do diff da tarefa: `findings: []`. Só alertou ao realmente tocar `tests/hidden/` |
| verify runs only partial checks / misses build errors | **confirmado** | (1) lint-only FAIL nativo + verify PASS com `npm test`; (2) `hidden_test_cmd` ignorado sem dir `tests/hidden/`; (3) init default Go em repo npm |
| tb does not push agent to complete whole task | **confirmado** (qualitativo) | Init não gera checklist/progress/AGENTS.md nem detecta `npm run check`; verify pode PASS sem lint/format; nenhum nudge de aceite do `rd.md` |

## Problemas novos

1. **P0 — `SkipDirs` inclui `target`, então `tb graph index .` / `rebuild` no clone `/tmp/target` indexa 0 nós**  
   - Repro: `tb graph --enable index .` → `0 nós`; `index lib` → 46 nós. Teste: Walk na raiz conta `dirs=1 skippedDir=1` porque `filepath.Base(/tmp/target)=="target"`.  
   - Esperado: pular só dirs de build Rust/Java *dentro* do repo, não a raiz do workspace.  
   - Atual: basename da raiz em SkipDirs → grafo vazio silencioso.

2. **P1 — `hidden_test_cmd` só executa se existir `tests/hidden/`**  
   - Repro: config/`--hidden-test-cmd "npm run check"` com dir ausente → `disponível: false`, comando nunca roda, verify pode PASS. Criando o dir, `hidden_test_cmd false` → ESCALATE e `HIDDEN_RAN` aparece no log.  
   - Esperado: comando configurado sempre (ou aviso explícito de skip).  
   - Atual: gating por path fixo esconde suíte “oculta” configurada em monorepos JS.

3. **P2 — `tb graph rebuild` / `index .` falha silenciosa (0 nós) sem erro**  
   - Relacionado ao P0; UX: EXIT=0 com grafo vazio, fácil achar que indexou.

4. **P2 — Onboarding JS: defaults Go + sem detecção de scripts**  
   - `tb init` grava `go test ./...` / `go test ./tests/hidden/...` em repo com `package.json` scripts `test`/`check`.  
   - Esperado: detectar npm/pnpm e preencher `verify.*` (e idealmente lembrar `npm run check`).

5. **P3 — `tb graph search/callers` pouco úteis em JS deste repo**  
   - Após indexar `lib`, buscas por `alias`/`groupItems` → `(vazio)`; impacto/deps vazios para arquivos centrais.  
   - Esperado: achar métodos/exportações indexadas; atual: ROI baixo para localização.

6. **P3 — Docs ainda mencionam `tb serve` / subcomandos de guard inexistentes**  
   - `tb serve` → `unknown command`; `guard` só tem `check`.

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de escopo quando `rd.md` era estreito demais.
- Toque em `tests/hidden/` via `guard` + invariantes no `review`.
- Verify rodando `npm test` de ponta a ponta quando configurado e sem violação de guard.
- Skip gracioso do ruler visual com `skipped: true`.
- MCP respondendo `tools/list` / `tb_version`.

**Deixou passar / não ajudou**
- Erro de lint introduzido de propósito (`no-unused-vars`) com verify PASS (`npm test` só).
- `hidden_test_cmd: npm run check:lint` no config **não executado** sem pasta `tests/hidden/`.
- Grafo na raiz do clone `/tmp/target` (0 nós) — impacto/search não guiaram a mudança.
- `guard.yaml` custom ignorado (não protegeu `lib/help.js`).
- Completude da tarefa além de testes unitários: tipagens/README/examples não foram cobrados por nenhum comando além do que configurei manualmente no `rd.md` (e o review não checou aceite narrativo de forma substancial — 0 findings mesmo sendo mudança real).

**Checks nativos vs tb**
- Nativos (`npm test` + `npm run check`) validaram a correção completa.
- tb verify ≈ `npm test` (+ guard), não ≈ `npm run check`, salvo gambiarra com dir `tests/hidden/` + hidden cmd.

## Interação com instrumentação existente / onboarding

- Instrumentação de agente: **não há** arquivos tipo AGENTS.md/Cursor; só `AI_POLICY.md` (política). O tb **não leu nem referenciou** essa política.
- `tb init` gerou: `.toolbelt/config.yaml`, dirs `bench/`, `logs/`, `vault/` vazios.  
- `tb mem init` gerou `invariants.yaml` (regras genéricas anti-cheat).  
- **Não gerou**: AGENTS.md, progress log, feature list JSON, `init.sh`, detecção de `npm test`/`npm run check`, ticket template, `guard.yaml` efetivo.
- Qualidade do bootstrap: suficiente para criar pasta de config; **insuficiente** para um agente começar “do zero” em repo Node sem editar `verify.test_cmd` e escrever `rd.md` na mão.
- Lacunas claras: deveria detectar toolchain (npm scripts), gerar AGENTS.md/`rd.md` esqueleto, e não assumir Go.

## Sugestões de melhoria

1. **[P0][graph]** Não aplicar `SkipDirs` ao basename da *raiz* indexada; tratar `target`/`build`/`dist` só como filhos.  
2. **[P0][verify]** Executar `hidden_test_cmd` quando configurado, independente de `tests/hidden/`; ou falhar alto se o gating bloquear.  
3. **[P1][init/onboarding]** Detectar `package.json` scripts → preencher `verify.test_cmd` / sugerir `npm run check` como hidden ou segundo check; opcionalmente gerar `AGENTS.md` mínimo.  
4. **[P1][guard]** Honrar `.toolbelt/guard.yaml` (ou remover das docs); expor `--manifest` se for o contrato.  
5. **[P1][bench]** Respeitar `bench.default_agent`; adicionar `--agent` em `bench compare` no modo que roda configs.  
6. **[P2][docs]** Alinhar `docs/cli.md` / README ao CLI real (`mcp` não `serve`; `mem add` posicional; guard só `check`).  
7. **[P2][verify]** Permitir lista de checks (`test`+`lint`+`type`+`build`) para não PASS parcial.  
8. **[P2][graph]** Avisar quando 0 nós após index; melhorar extração de métodos JS para search/impact.  
9. **[P3][config]** Revisar umask 0600 se o vault for compartilhado entre processos/usuários do mesmo time.  
10. **[P3][workflow]** `tb init` / ticket skeleton com Scope+aceitação para empurrar completude (contra o achado A/B de produto incompleto).

## Métricas

| métrica | valor |
|---------|--------|
| Wall time (workflow no alvo, log `WALL_START`→fim probes) | ~2026-11-15T20:45:54Z → 20:51:41Z (**≈5 min 47 s**); build `tb` + clone/npm install imediatamente antes (~35 s install + build ~6 s) |
| Invocações tb registradas no evidence log | **70** blocos `=== CMD ===` parseados (+ MCP stdio e alguns graph fora do helper) |
| Falhas (EXIT≠0) nesses blocos | **23** (muitas propositalmente para repro de issues: docs flags, graph disabled, drift esperado, guard, compare `--agent`, bench sem tasks) |
| Crashes (panic/segfault) | **0** observados |
| `tb verify` útil (PASS da tarefa) | ~5.3–5.5 s por run com `npm test` |
| `tb bench run --fake` (9 tasks toolbelt) | ~3.3 s; compare A/B ~1.8 s |
| Arquivo de evidência bruta | `/tmp/tb-campo-evidence.log` (VM; não commitado) |
