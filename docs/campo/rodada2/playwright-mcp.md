# Estudo de campo rodada 2: microsoft/playwright-mcp

Evidência coletada em 2026-11-16 no VM. Clone local em `/tmp/target/playwright-mcp` (shallow). Nenhuma interação com o remoto do alvo. Binário `tb` buildado de `main` atual (`go build -o /tmp/tb ./cmd/tb` → `toolbelt 0.1.0-dev`, `commit: unknown` sem ldflags). Log bruto: `/tmp/tb-campo2-playwright-mcp.log`. Grupo: **com instrumentação AI existente**.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [microsoft/playwright-mcp](https://github.com/microsoft/playwright-mcp) (`@playwright/mcp` 0.0.85) |
| Linguagem | JavaScript/TypeScript (wrapper npm; servidor MCP real em `microsoft/playwright` / `playwright-core`) |
| Tamanho | ~50 arquivos fora de `node_modules`/`.git`/`.toolbelt`; ~564 KiB tree (du); ~4,9 kLOC em `.js`/`.ts`/`.md`/`.json` contados no clone |
| Instrumentação AI existente | `AGENTS.md`; `.claude/skills/release.md`; workflows `.github/workflows/agent-{triage,review,task,roll,release}.yml` |
| Toolchain | **OK:** Node 22, `npm ci`, Chrome do sistema + Chromium Playwright. **Não exercitado:** WebKit, Firefox multi-project, `dtest`/Docker, roll a partir de `../playwright`. `npm run build` é no-op (`echo OK`). |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Alinhar `AGENTS.md` aos projects reais (`--project=chrome`) e adicionar regressões em `tests/cli.spec.ts` para `cli.js --version` / `--help` |
| Por quê | Bug local deste pacote (não exige monorepo upstream); o exemplo `--project=chromium` em `AGENTS.md` conflita com `playwright.config.ts` (só `chrome` + `chromium-docker` opcional) e já causou ESCALATE na rodada 1 |
| Issue | Sem issue dedicada; contraste com [#1861](https://github.com/microsoft/playwright-mcp/issues/1861) (env vars README) e [#1864](https://github.com/microsoft/playwright-mcp/issues/1864) (upstream `browser_close`) |
| Concluída localmente? | **Sim** no clone (commit local `c921ce1`); **não** enviada ao upstream |
| Resultado nativo | `npx playwright test tests/cli.spec.ts --project=chrome`: **3 passed**. `npm run lint`: **OK**. `npm run build`: **OK** |
| Rodada 1 (comparação) | Fix de `update-readme.js` + regeneração README + regressão env vars (issue #1861). Esta rodada usou tarefa **diferente**. |

## Verificação das 12 correções

| # | correção | status | evidência |
|---|----------|--------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Clone/cópia com basename `target` (`/tmp/skipdirs-probe/target`): `tb graph --enable rebuild` → **130 nós / 408 arestas** (não 0). Também `cwd=/tmp/target`: **126 nós** |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Config `hidden_test_cmd: echo HIDDEN_RAN && exit 0` sem pasta `tests/hidden/`; `tb verify run --ruler tests --json` → `"hidden_available": true`, `"hidden_passed": true` |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Com violação `package.json`: `tb guard check 'HEAD~1..HEAD' --strict --json` → **EXIT=1** + JSON com violations. Sem violação: **EXIT=0** |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `package.json`; diff tocando só `package.json` → `violations: [{type:protected_path,file:package.json}]`. Diff limpo → `violations: null` |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em repo com `package.json` → `verify.test_cmd: npm test` (não mais `go test ./...`) |
| 6 | review `tests/hidden` por path | **funcionou** | Commit que só **menciona** `// note: do not touch tests/hidden` em `tests/cli.spec.ts` → `tb review` **findings: []**, EXIT=0 (na R1 era FP critical) |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** (com ressalva) | `bench.default_agent: echo AGENT_OK {prompt}` + `-t/-f` → run sem `--agent/--fake` EXIT=0; sem default → erro claro. `tb bench compare --config-a/--config-b --agent 'echo CMP {prompt}'` EXIT=0. **Ressalva:** `bench.tasks_dir` absoluto no YAML **não** é aplicado pela CLI (continua `bench/tasks`) |
| 8 | review/drift `## Scope`/`## Escopo`, listas MD e `**` globs | **funcionou** | Ticket com `## Escopo` + bullets + `` `**/*.spec.ts` ``: drift/review no escopo EXIT=0; alteração `index.js` fora → `out_of_scope: [index.js]`; glob `**/*.spec.ts` aceita `tests/cli.spec.ts`; `**/*.md` rejeita o `.spec.ts` |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Config com `build_cmd: npm run build`, `lint_cmd: npm run lint`; verify JSON → `build_ran/passed` e `lint_ran/passed` true, verdict PASS |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** | Init registrou `sources: AGENTS.md` + hints (`npm ci`, `npm run lint`, …) em `.toolbelt/agent_docs.yaml`; `--write-agents` **não** sobrescreveu AGENTS.md existente. Em dir vazio: semeou AGENTS.md mínimo apontando ao tb |
| 11 | graph Python `calls` | **não se aplica** | Repo sem `.py`; JS indexou `edges_calls: 208` (indexador JS já existia) |
| 12 | verify visual skipped + docs CLI | **parcial** | Manifesto ausente → `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` (**ok**). `screens: []` → ainda `"skipped": false`, `"screens": null`, PASS (**não ok**; `aggregateVisualSkipped` retorna cedo se `len(screens)==0`). `docs/cli.md` mais alinhado à CLI real que na R1 |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK (~5 ms); detecta npm; lê AGENTS.md | ajudou | `test_cmd: npm test`; hints extraídos; ainda sugere `npm test` “multi-project” copiado do AGENTS.md antigo (antes do fix) |
| `tb init --write-agents` | OK; não sobrescreve | ajudou | Em repo com AGENTS.md: preserva; em probe vazio: cria stub Toolbelt |
| `tb mem init/list/add/check` | OK | neutro→ajudou | Mandate local sobre ownership do pacote; `mem check` OK |
| Ticket / Scope (`rd.md`) | OK via Markdown `## Escopo` | ajudou | Listas + `**/*.spec.ts` funcionaram; `rd.md` fora do scope se não listado (esperado) |
| `tb graph --enable rebuild/stats/deps/impact` | rebuild 130 nós / 408 arestas | ajudou | SkipDirs ok em raiz `target`; `deps --file cli.js` listou `playwright-core`; `search install-browser` → `null`; `impact --file cli.js` → `null` |
| `tb verify run` (tests) | PASS com build+lint+hidden | ajudou | Roda `build_cmd`/`lint_cmd`/`test_cmd`/`hidden_test_cmd`; 3 testes chrome verdes |
| `tb verify run` (visual) | PASS skipped / PASS não-skipped | neutro→atrapalhou | Ausente: skip correto; `screens: []`: `skipped=false` |
| `tb review` | PASS no fix com scope completo; FP substring sumiu | ajudou | Mencionar `tests/hidden` não gera finding; scope Markdown ok |
| `tb drift` | OK no escopo; flagra OOS e globs | ajudou | `index.js` fora; `**/*.spec.ts` vs `**/*.md` discriminam |
| `tb guard check` | OK; lê `guard.yaml`; `--strict --json` exit 1 | ajudou | Sintaxe `base..head` (não `--base`); UX ainda confunde quem veio da R1 |
| `tb bench run/list/compare` | OK com `-t/-f` + default_agent / `--agent` | neutro→ajudou | Sem tasks locais no alvo, `-t` obrigatório; YAML `tasks_dir` ignorado pela CLI |
| `tb mcp` | tools/list OK | ajudou | JSON-RPC: `tb_version`, `tb_graph_*`, `tb_echo`, … |
| `tb version` | `0.1.0-dev` / commit unknown | neutro | `go build` sem ldflags |

## Problemas que continuam

| problema (R1 / síntese) | status nesta rodada | evidência |
|-------------------------|---------------------|-----------|
| `version` → `unknown` sem ldflags | continua | `tb version` → `commit: unknown` |
| Config files mode `0600` | não re-testado a fundo | init criou `.toolbelt/`; não medido `stat` nesta rodada |
| Visual `skipped=false` com manifesto vazio | **continua** | `screens: []` → skipped false |
| Guard lista hardcoded ainda aborta verify em thresholds | não re-testado | foco em `guard.yaml` custom |
| `graph impact`/`search` às vezes vazios | continua (parcial) | `impact`/`search install-browser` → null apesar de deps úteis |
| tb não empurra completar a tarefa sozinho | continua (observacional) | Sem `tb ticket`/board; agente ainda monta `rd.md` à mão |
| Docs/CLI drift residual | parcial | Guard usa `base..head`; quem tenta `--base` falha (`unknown flag`) |
| Init hints ecoam AGENTS.md desatualizado | continua até o humano corrigir AGENTS | Antes do fix, hints incluíam `--project=chromium` |

## Problemas novos

| ID | sev | repro | esperado | atual |
|----|-----|-------|----------|-------|
| N1 | P2 | Em `.toolbelt/config.yaml` setar `bench.tasks_dir` / `fixtures_dir` absolutos apontando ao harness do toolbelt; rodar `tb bench run --task doc-function` sem `-t` | CLI usa paths do YAML | Log mostra `tasks_dir: bench/tasks` e falha `open bench/tasks/...`; só `default_agent` parece lido do config |
| N2 | P2 | `tb verify run --ruler visual` com `.toolbelt/screens.yaml` = `screens: []` | `skipped=true` + razão “nenhuma tela” (agregação) | `skipped=false`, `screens: null`, PASS (`aggregateVisualSkipped` retorna se len==0) |
| N3 | P3 | `tb guard check --base HEAD~1 --head HEAD` (flags estilo review/drift) | Ou aceitar as flags, ou docs/help cruzados | `unknown flag: --base`; exige `HEAD~1..HEAD` |
| N4 | P3 | Ticket com `## Escopo` sem incluir `rd.md`, commit que adiciona o ticket | Review/drift não tratam o ticket como ruído, ou documentam exclusão | `rd.md` sempre `out_of_scope` se não listado |

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift/review de escopo com `## Escopo` + globs `**`.
- Guard via `guard.yaml` em `package.json`.
- Verify com build+lint+test+hidden configurados (PASS só quando a cadeia inteira passa).
- Init toolchain npm + import de hints de `AGENTS.md`.
- Graph deps de `cli.js`; SkipDirs não zera índice em raiz `target`.
- Review sem FP de substring `tests/hidden`.

**Deixou passar**
- Inconsistência AGENTS.md ↔ `playwright.config.ts` **antes** do fix (tb importou hints errados em vez de validá-los contra a config real).
- Visual vazio ainda “PASS” sem skipped.
- `bench.tasks_dir` no YAML (agente pode achar que configurou o harness e falha).
- Ausência de cobertura CLI `--version`/`--help` antes dos testes novos (sem ruler de docs↔ binário além do que o humano configurou).

**Nativo vs tb:** nativo (`cli.spec.ts` chrome + lint) validou o fix; tb verify espelhou bem quando `build_cmd`/`lint_cmd`/`test_cmd` estavam preenchidos — melhora clara vs R1 (só `test_cmd`).

## Interação com instrumentação existente / onboarding

- `tb init` **agora lê** `AGENTS.md` e grava `agent_docs.yaml` + hints de comandos (melhoria vs R1, que ignorava).
- Ainda **não valida** hints contra a toolchain real: ecoou `--project=chromium` / lista multi-browser do AGENTS.md enquanto só existe project `chrome`.
- `--write-agents` respeita arquivo existente (correto neste grupo “com instrumentação”).
- Skills `.claude/skills/release.md` e workflows `agent-*` continuam fora do vault/mem.
- Conflito R1 “AGENTS.md manda chromium” foi mitigado na tarefa (editamos AGENTS.md); o tb sozinho não teria corrigido.

## Comparação com a rodada 1

| dimensão | R1 | R2 | veredito |
|----------|----|----|----------|
| Init defaults Go | atrapalhou | detecta `npm test` | **melhor** |
| AGENTS.md | ignorado | lido + hints | **melhor** (ainda ecoa erros do arquivo) |
| Verify build/lint | não existia | `build_ran`/`lint_ran` | **melhor** |
| hidden sem dir | não rodava | roda se `hidden_test_cmd` set | **melhor** |
| guard.yaml | ignorado | violations reais | **melhor** |
| guard `--strict --json` | exit 0 com violação | exit 1 | **melhor** |
| review FP `tests/hidden` | confirmado | refutado | **melhor** |
| Scope `## Escopo` + `**` | frágil / glob falhava | funcionou | **melhor** |
| SkipDirs raiz `target` | 0 nós (outros reports) | 130 nós | **melhor** |
| default_agent / compare `--agent` | quebrado | funciona | **melhor** |
| Visual empty screens | skipped=false | skipped=false | **igual** |
| Completeness / ticket CLI | ausente | ausente | **igual** |
| Tarefa local | concluída (#1861) | concluída (AGENTS+cli tests) | **igual** (sucesso) |

No conjunto: **claramente melhor** para o loop init→scope→verify→guard→review; regressões graves da R1 nas 12 correções não reapareceram, salvo o buraco visual `screens: []` e o `bench.tasks_dir` do YAML.

## Sugestões de melhoria

1. **[P1][bench]** Honrar `bench.tasks_dir` / `fixtures_dir` / `output_dir` do `config.yaml` (hoje só `default_agent` é lido de forma confiável).
2. **[P1][verify/visual]** Tratar manifesto com `screens: []` como `skipped=true` (ajustar `aggregateVisualSkipped` para len==0).
3. **[P2][init]** Validar hints de AGENTS.md contra `playwright.config.*` / scripts reais (avisar se `--project=X` não existe).
4. **[P2][guard/docs]** Aceitar `--base`/`--head` como aliases de `base..head`, ou documentar só a forma posicional em todos os exemplos.
5. **[P2][review/drift]** Auto-incluir o path do `--ticket` no scope, ou warning explícito.
6. **[P3][graph]** Melhorar search textual em JS (string `install-browser` em `cli.js` não indexou).
7. **[P3][mem/onboarding]** Importar `.claude/skills/*` como playbooks opcionais.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time (timestamps do log) | ~12:36:12–12:38:43 UTC (~2,5 min de comandos cronometrados; exclui `npm ci`/install browsers feitos antes) | ~3,4 min cronometrados |
| Soma durações `tb-*` no log | ~3,5 s (dominado por verify tests ~3,1 s) | ~41 s |
| Invocações `tb` no log | 49 | 43 |
| Falhas `tb` (exit ≠ 0) | 18 (inclui probes esperados: UX guard flags, drift OOS, bench sem `-t`, review sem rd no scope, etc.) | 14 |
| Tarefa local | concluída (3 testes chrome + lint) | concluída (2 testes + lint) |
