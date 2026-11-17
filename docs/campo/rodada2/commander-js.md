🇧🇷 Português | [🇺🇸 English](../../en/campo/rodada2/commander-js.md)

# Estudo de campo ROUND 2: commander.js

Grupo: **without existing AI-agent instrumentation** (sem instrumentação de agente de IA além de política humana).  
Alvo local: `/tmp/commander-js` (clone shallow de https://github.com/tj/commander.js); probe SkipDirs em `/tmp/target`.  
Binário: `tb` buildado com `make build` em `a46c295` (`./bin/tb`).  
Data: 2026-11-16.  
Regra: nenhum push/issue/PR/comentário no upstream; só clone local.

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | `tj/commander.js` (npm: `commander` 15.1.0) |
| Linguagem | JavaScript (ESM) + TypeScript typings (`typings/`); testes com Node test runner |
| Tamanho | ~2.9 MiB no clone (com `node_modules`); **220** arquivos versionados; ~18 k LoC `.js` versionados |
| Instrumentação de agente | **Ausente** como tooling de agente: sem `AGENTS.md`/`CLAUDE.md`/`.cursor`/Copilot instructions no upstream. Existe `AI_POLICY.md` (política humana). Após `tb init --write-agents`, o harness gerou `AGENTS.md` local (não enviado ao upstream) |
| Toolchain | **OK**: Node `v22.23.4`, npm `10.9.9`, `npm install` (214 pkgs), baseline `npm test` (**1430** pass) e `npm run check` verdes. Chrome em `/usr/local/bin/google-chrome` (ruler visual) |

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Issue [#2603](https://github.com/tj/commander.js/issues/2603): com `process.versions.electron` **e** `ELECTRON_RUN_AS_NODE` definido, `parse()` deve usar convenção Node (`argv.slice(2)`), não Electron packaged (`argv.slice(1)`). Patch em `lib/command.js` + teste em `tests/command.parse.test.js` |
| Por quê | Bug real com repro e patch mínimos; maintainer-plausível; escopo claro com testes |
| Issue | https://github.com/tj/commander.js/issues/2603 (somente leitura; label *pending release* no upstream — fix ainda **não** está no `master` shallow clonado) |
| Completa? | **Sim, localmente** (commit `485c4f5` na branch local `campo2-local-fix` do clone; **não** enviado ao upstream) |
| Checks nativos | Após a correção: `npm test` → pass; `npm run check` → pass. Probes laterais (lint/`tests/hidden`/`lib/help.js`) foram revertidos com `git reset --hard` |
| Rodada 1 (comparação) | Rodada 1 usou [#2683](https://github.com/tj/commander.js/issues/2683) (`Help.groupItems` Map→record + tipagem de `alias()`). Esta rodada usa tarefa **diferente** (#2603) |

## Verificação das 12 correções

| # | correção | funcionou/não funcionou/não testado/não se aplica | evidência |
|---|----------|-----------------------------------------------------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Clone em `/tmp/target`: `tb graph --enable rebuild` → **426 nós / 2047 arestas** (EXIT=0). Na rodada 1 o mesmo path dava 0 nós |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Config `hidden_test_cmd: npm run check:lint` sem dir: JSON `"hidden_available": true`, `"hidden_passed": true` e texto `Oculto: true (disponível: true)` |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff tocando `lib/help.js` (via `guard.yaml`): JSON com `protected_path` + **EXIT=1** |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `lib/help.js`; commit de probe → `{"type":"protected_path","file":"lib/help.js",...}` EXIT=1. Path hardcoded `tests/hidden/sentinel.js` também violado |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em repo npm → `verify.test_cmd: npm test` (não `go test`). `hidden_test_cmd` ficou `""` (melhor que o Go default da rodada 1) |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket `rd-mention-hidden.md` cita `tests/hidden`/`forbidden` sem tocar o dir: finding foi só **fora do scope** (`tests/command.parse.test.js`), sem critical de invariante por substring. Ao **tocar** `tests/hidden/sentinel.js`: `[critical] (invariants) … tests/hidden/` |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Config `default_agent: echo agent-default`; `tb bench run -r 1 -t …/bench/tasks` **sem** `--agent` → EXIT=0, `agent=external`. `tb bench compare --agent "echo hi" --fake …` → EXIT=0 (~120 ms). Help de `compare` lista `--agent` |
| 8 | review parse `## Scope`/`## Escopo`, listas MD e `**` | **funcionou** | `rd.md` com `## Escopo` + bullets `` `lib/command.js` `` / `` `tests/**/*.js` ``: review **sem** “sem scope declarado”; `tests/command.parse.test.js` dentro do scope. `rd-narrow.md` só `typings/**/*.ts` → drift EXIT=1 com `lib/command.js` e `tests/command.parse.test.js` fora |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Com ambos setados: `Build: true` / `Lint: true`. Probe lint em `lib/option.js` + `lint_cmd`: verify falha no step `build_or_lint` (eslint unused). Sem `lint_cmd`: mesmo lint nativo FAIL + verify **PASS** (falso negativo opt-out) |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** | Sem arquivos: `tb init --write-agents` criou `AGENTS.md` (EXIT=0). Re-`init` com AGENTS presente: `documentação de agentes encontrada sources=AGENTS.md` + `.toolbelt/agent_docs.yaml`. Flag inexistente na rodada 1 |
| 11 | graph arestas `calls` em Python | **não se aplica** | Repo JS; indexer JS já emite `calls` (stats: `edges_calls: 1132`). Não exercitei fixture Python |
| 12 | verify visual skipped agregado + docs CLI alinhados | **funcionou** (parcial nos docs) | `tb verify run --ruler visual --json` → `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. `docs/cli.md` documenta `tb mcp` (não `serve`) e `## Scope`; ainda há exemplos residuais com `mem add --category` e `tb mem add --type` continua rejeitado pela CLI |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `make build` / `tb version` | OK; `a46c295` | neutro | JSON com commit/build_time; ~5 ms |
| `tb init` | OK; `test_cmd: npm test` | **ajudou** | Detecção npm; `hidden_test_cmd: ""` |
| `tb init --write-agents` | OK; gerou `AGENTS.md` | **ajudou** | Arquivo mode 600; texto mínimo toolbelt |
| `tb mem init/add/list/check` | OK | ajudou | `mem check` retenção; `mem add --type` ainda unknown flag |
| Ticket `rd.md` (`## Escopo`) | OK | **ajudou** | Drift/review consumiram heading+bullets+`**` |
| `tb graph` sem enable | Erro ADR-011 | neutro | Esperado |
| `tb graph --enable rebuild` (e em `/tmp/target`) | 419–426 nós | **ajudou** | SkipDirs raiz corrigido; indexação útil |
| `tb graph search _prepareUserArgs` | hits de seção/ticket | neutro/parcial | Pouco ROI para achar o método no `lib/` |
| `tb graph callers --function _prepareUserArgs` / `parse` / `emit` | `(vazio)` | atrapalhou | Apesar de `edges_calls: 1132`, callers nomeados da tarefa ficaram vazios |
| `tb graph query` | Inexistente (cai no help) | atrapalhou (docs/expectativa) | EXIT=0 com help; subcomando ausente |
| `tb verify run` (build+lint+test+hidden) | PASS ~22 s | **ajudou** | JSON: build/lint/visible/hidden true |
| `tb verify` sem `lint_cmd` + lint quebrado | PASS indevido | atrapalhou (se mal configurado) | Confirma que lint é opt-in |
| `tb verify --ruler visual --json` | PASS skipped | neutro | Manifesto ausente; Chrome detectado |
| `tb review --ticket rd.md` | 0 findings na tarefa | ajudou | Fresh=true; scope OK |
| `tb review` + toque `tests/hidden` | critical invariant | ajudou | Verdadeiro positivo por path |
| `tb drift` (scope completo / narrow) | OK / EXIT=1 | **ajudou** | Narrow listou 2 arquivos fora |
| `tb guard check HEAD~1..HEAD` | OK na tarefa; FAIL em probes | **ajudou** | `guard.yaml` + hardcoded |
| `tb guard check --strict --json` | EXIT=1 com violação | **ajudou** | JSON válido + exit ≠ 0 |
| `tb bench run` sem tasks locais | FAIL open bench/tasks | neutro | Esperado no alvo |
| `tb bench run` com `default_agent` + `-t` toolbelt | OK 11.1% | ajudou (DX) | Não exigiu `--agent` |
| `tb bench run --fake` / `compare --agent` | OK | neutro | Fake ~63 ms; compare ~120 ms |
| `tb mcp` (stdio) | OK | ajudou parcial | `tools/list` + `tb_version`; graph tools com `TB_GRAPH=on` |

## Problemas que continuam

Da rodada 1 (`docs/campo/commander-js.md`), ainda observados neste run:

| problema rodada 1 | status agora | evidência |
|-------------------|--------------|-----------|
| `tb graph query` inexistente | **continua** | Cai no help do `tb graph` |
| `graph callers`/search pouco úteis para a tarefa JS | **continua** (parcial) | Index grande, mas `callers --function _prepareUserArgs` → `(vazio)` |
| Config mode `0600` | **continua** | `stat`: `config.yaml` e `AGENTS.md` → **600** |
| `mem add` flags de docs antigas | **continua** (parcial) | `unknown flag: --type`; docs ainda citam `--category` em exemplos |
| Verify PASS sem lint se `lint_cmd` vazio | **continua** (por desenho opt-in) | Probe lint FAIL nativo + verify PASS |
| `tb bench list` sem tasks no alvo | **continua** | `open bench/tasks: no such file` |
| Init não preenche lint/build/hidden npm automaticamente | **continua** (parcial) | Detecta `npm test`, mas não `npm run check` / type / lint |
| Grafo off sem `--enable`/`TB_GRAPH` | **continua** (desejado ADR-011) | Erro explícito |

Itens da rodada 1 **corrigidos** neste binário: SkipDirs+`/tmp/target`, `hidden_test_cmd` gated, `guard.yaml` ignorado, `--strict --json` exit 0, init Go-default, `default_agent` morto, parser sem `## Scope`, review FP substring `tests/hidden`.

## Problemas novos

1. **P2 — `callers`/`impact` JS ainda fracos para guiar a mudança apesar do índice populado**  
   - Repro: após `rebuild`, `tb graph --enable callers --function _prepareUserArgs` → `(vazio)`; `impact --file lib/command.js` só ecoa o próprio arquivo.  
   - Esperado: callers de `parse`/`parseAsync` → `_prepareUserArgs` ou impacto útil.  
   - Atual: `edges_calls` alto, mas resolução por nome da tarefa falha — ROI baixo para localização (já notado na R1; permanece pós-fixes).

2. **P3 — `tb graph query` ainda sugerido por hábitos/docs de campo e ausente**  
   - Repro: `tb graph --enable query '…'` imprime help e EXIT=0.  
   - Esperado: subcomando ou erro “unknown command” claro (EXIT≠0).  
   - Atual: silêncio útil baixo.

3. **P3 — Init não sugere `npm run check` / `build_cmd` / `lint_cmd` a partir de `package.json` scripts**  
   - Repro: `tb init` → só `test_cmd: npm test`; `hidden_test_cmd: ""`.  
   - Esperado (pós P1-1): pelo menos hint ou preenchimento de lint/check.  
   - Atual: agente ainda precisa editar YAML à mão para evitar falso PASS (confirmado no probe sem `lint_cmd`).

4. **P3 — Exemplos em `docs/cli.md` ainda mostram `mem add --category`**  
   - Repro: grep em `docs/cli.md` (~L348, ~L895); CLI real rejeita flags tipo `--type`.  
   - Esperado: exemplos só com forma implementada.  
   - Atual: alinhamento #12 parcial.

5. **P3 — Relatório de drift imprime `Base: <declared> → <actualBase>` com o mesmo SHA mesmo quando o head diverge**  
   - Repro: `tb drift --ticket rd.md` após commit da tarefa → `Base: a5a7dc… → a5a7dc…` enquanto review mostra `a5a7dc…..HEAD` e o narrow lista arquivos do commit.  
   - Esperado: mostrar base→head do range analisado.  
   - Atual: confunde leitura humana (comparação de arquivos parece funcionar).

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de escopo com `## Escopo` + globs `**` (narrow).
- `guard.yaml` custom (`lib/help.js`) e path hardcoded `tests/hidden/`.
- Verify com `build_cmd`/`lint_cmd`/`hidden_test_cmd` configurados (PASS completo da tarefa; FAIL em lint probe).
- Review invariant por path real em `tests/hidden/`.
- SkipDirs na raiz `/tmp/target` (grafo não-vazio).
- `default_agent` no bench; MCP `tb_version` / tools de grafo.

**Deixou passar / não ajudou**
- Lint introduzido de propósito quando `lint_cmd` não está no YAML (verify PASS).
- Localização da função via `callers`/`search` para `_prepareUserArgs` (vazio).
- Completude narrativa do aceite do ticket: review 0 findings na mudança correta (não valida comportamento Electron).
- `AI_POLICY.md` do upstream não foi lido/indexado (só `AGENTS.md` gerado pelo próprio tb).

**Checks nativos vs tb**
- Nativos (`npm test` + `npm run check`) validaram a correção.
- tb verify ≈ type + lint + test + lint-as-hidden **quando** o YAML é preenchido manualmente; init sozinho ≈ só `npm test`.

## Interação com instrumentação existente / onboarding

- Upstream: **sem** AGENTS/Cursor; só `AI_POLICY.md`. O tb **não** referenciou a política.
- `tb init`: `.toolbelt/config.yaml` com `npm test`; dirs `bench/`/`logs/`/`vault/`.
- `tb init --write-agents`: criou `AGENTS.md` mínimo apontando verify/guard/mem.
- Re-init: gravou `agent_docs.yaml` com `sources: [AGENTS.md]`.
- `tb mem init`: invariantes anti-cheat padrão.
- Bootstrap: **melhor que a rodada 1** (toolchain + seed AGENTS). Ainda exige edição manual de `lint_cmd`/`build_cmd`/`hidden_test_cmd` e ticket `rd.md` para um fluxo adversarial completo.

## Comparação com a rodada 1

| Aspecto | Rodada 1 | Rodada 2 | Veredito |
|---------|----------|----------|----------|
| SkipDirs `/tmp/target` | 0 nós | 426 nós | **melhor** |
| `hidden_test_cmd` | ignorado sem dir | executa | **melhor** |
| `guard.yaml` | ignorado | honrado + exit strict/json | **melhor** |
| Init toolchain | `go test` | `npm test` | **melhor** |
| Scope Markdown | frágil / manual `scope:` | `## Escopo` + `**` OK | **melhor** |
| Review FP `tests/hidden` | mitigado na prática R1; código agora path-based | confirmado path-based | **melhor** |
| Bench `default_agent` / `compare --agent` | quebrados | funcionam | **melhor** |
| build/lint no verify | ausentes | honrados se setados | **melhor** |
| AGENTS seed/leitura | ausente | `--write-agents` + re-leitura | **melhor** |
| Callers/search JS para a tarefa | fraco | ainda fraco | **igual** |
| Falso PASS sem lint no YAML | sim | sim (opt-in) | **igual** (mitigável) |
| Wall time workflow | ~5 min 47 s | ~3 min 46 s (12:39:37→12:43:23Z) + setup npm prévio | **melhor** (menos retrabalho em bugs P0) |

No geral: **claramente melhor** nos 12 fixes; gargalos restantes são onboarding profundo (auto lint/check), grafo JS callers e completude de aceite.

## Sugestões de melhoria

1. **[P1][init]** Preencher ou sugerir `lint_cmd`/`build_cmd`/`hidden_test_cmd` a partir de scripts `package.json` (`check`, `lint`, `typecheck`).  
2. **[P1][graph]** Melhorar resolução de `callers`/`search` para métodos de classe JS (ex. `_prepareUserArgs`).  
3. **[P2][graph]** Remover ou implementar `tb graph query`; se ausente, EXIT≠0.  
4. **[P2][drift]** Imprimir `base..head` reais do range (não só declared→actualBase iguais).  
5. **[P2][docs]** Remover exemplos `mem add --category` / flags mortas restantes em `docs/cli.md`.  
6. **[P3][verify]** Aviso alto quando só `test_cmd` está setado mas `package.json` tem `lint`/`check`.  
7. **[P3][init]** Opcionalmente indexar/avisar `AI_POLICY.md` / CONTRIBUTING quando não há AGENTS.  
8. **[P3][config]** Revisar mode `0600` se vault/AGENTS forem compartilhados no time.

## Métricas

| métrica | valor | rodada 1 |
|---------|--------|----------|
| Wall time (workflow evidenciado) | **≈3 min 46 s** (12:39:37Z→12:43:23Z) após rebuild tb + re-init; npm install/baseline antes ~47 s | ≈5 min 47 s |
| Invocações tb (blocos `=== CMD ===`) | **54** (+ MCP stdio e alguns graph avulsos) | 70 |
| Falhas EXIT≠0 nesses blocos | **19** (muitas propositalmente: graph off, drift narrow, guard probes, lint, docs flags, bench sem tasks) | 23 |
| Crashes | **0** | 0 |
| `tb verify` útil (PASS tarefa completa) | ~21–23 s com type+lint+test+hidden | ~5.3 s (`npm test` só) |
| `tb bench run --fake` (9 tasks) | ~63 ms (1 rep); compare ~120 ms | ~3.3 s / ~1.8 s |
| Arquivo de evidência bruta | `/tmp/tb-campo2-commander-evidence.log` (VM; não commitado) | `/tmp/tb-campo-evidence.log` |
