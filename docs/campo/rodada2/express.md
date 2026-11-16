# Campo rodada 2 — expressjs/express

Grupo: without existing AI-agent instrumentation.
Clone local apenas (`/tmp/express-r2` e `/tmp/target` para regressão SkipDirs); sem push/issue/PR/comentário no upstream.
`tb` buildado deste repo em `main` @ `a46c295` (`make build` → `bin/tb`).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [expressjs/express](https://github.com/expressjs/express) |
| Linguagem | JavaScript (Node.js), CommonJS |
| Versão no clone | 5.3.0 (shallow `HEAD` `43fca52`) |
| Tamanho | ~2–3 MiB sem contar `node_modules` de forma limpa; ~68 MiB com deps; 142 `.js` fora de `node_modules`; ~18,7 kLOC em `lib/`+`test/*.js` |
| Instrumentação AI existente | Nenhuma (`AGENTS.md`/`CLAUDE.md`/`.cursorrules`/copilot instructions ausentes antes do `tb init --write-agents`) |
| Toolchain | `npm install` OK (Node v22.23.4, npm 10.9.9). Scripts: `npm test` (mocha), `npm run lint` (eslint 8). Sem passo de build de produção. |
| Não configurado | Redis real (`connect-redis`); não necessário aos testes unitários da tarefa. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `res.send(DataView)` que reconhecia a view via `ArrayBuffer.isView()` mas convertia com `Buffer.from(dataView)` → Buffer vazio e `Content-Length: 0`. |
| Por quê | Bug real, escopo pequeno (`lib/response.js` + `test/res.send.js`), maintainer-plausível, com repro claro e issue aberta. |
| Issue | https://github.com/expressjs/express/issues/7510 |
| Completou? | Sim, localmente (commit só no clone `/tmp/express-r2`, sem push). |
| Diff | `isDataView` → `Buffer.from(view.buffer, byteOffset, byteLength)`; 2 testes novos (com/sem ETag). |
| Checks nativos | `npm test -- --grep DataView` → 2 passing; `npm test` completo → **1292 passing**; `npm run lint` → PASS. Repro manual: antes `len 0`; depois `len 3 hex 010203`. |
| Rodada 1 (comparação) | Rodada 1 usou `req.subdomains` com host IPv6 mapeado (`[::ffff:127.0.0.1]`, issue #7532). Esta rodada é outra superfície (`res.send` / DataView). |

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph --enable index .` em `/tmp/target` → **452 nós, 3051 arestas** (rodada 1: 0 nós). Em `/tmp/express-r2`: 455 nós / 3054 arestas. |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; config/`--hidden-test-cmd` → `hidden_available: true`, `hidden_passed: true`. Com cmd `false` → `hidden_passed: false`, verdict `ESCALATE` (3 iters). |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Com `protected_paths: [lib/response.js]`: `--strict --json` imprimiu JSON da violação e `EXIT=1`. Sem `--strict` + JSON: `EXIT=0` (esperado). |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` protegendo `lib/response.js` → `protected_path` no diff do fix. Protegendo só `lib/request.js` (intocado) → `violations: null`. |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em Express → `verify.test_cmd: npm test` (não mais `go test ./...`). |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket `rd.md` menciona `tests/hidden/**` em Forbidden; `--standards invariants` → `findings: []` (sem FP de substring). |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `tb bench run -r 1` sem `--agent` usou `default_agent: echo DEFAULT_AGENT_RAN {prompt}` (EXIT=0). `tb bench compare … --agent 'echo COMPARE_AGENT {prompt}'` EXIT=0. |
| 8 | review/drift `## Scope` / listas / `**` | **funcionou** | Ticket com `## Scope` + bullets + `test/**/*.js`; drift do fix sem `out_of_scope`. Glob só `test/**/*.js`+`lib/**/*.js` também manteve in-scope. `lib/**` sufixo OK. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | JSON do verify: `build_ran/passed: true`, `lint_ran/passed: true` com `lint_cmd: npm run lint` e `build_cmd: node -e "require('.')"` . |
| 10 | init lê AGENTS/CLAUDE e `--write-agents` | **funcionou** | Sem arquivos: log “use --write-agents”. Com flag: semeou `AGENTS.md`, gravou `.toolbelt/agent_docs.yaml` (`sources: [AGENTS.md]`). Não havia CLAUDE.md prévio para ler. |
| 11 | graph Python `calls` edges | **não se aplica** | Repo JS; não exercitou indexador Python. Em JS o grafo emitiu `edges_calls: 2191`. |
| 12 | verify visual skipped agregado + docs CLI | **funcionou** | Manifesto com ref ausente → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"` (rodada 1: `skipped: false`). `docs/cli.md` documenta hidden sem dir, `## Scope`, `guard.yaml`, `default_agent`, visual skipped. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` (ldflags) | OK — `toolbelt a46c295` | neutro | `EXIT=0 TIME=0.006s` |
| `tb init` | `.toolbelt/`; `test_cmd: npm test`; avisa ausência de AGENTS | **ajudou** | toolchain JS detectada; mode `600` em config |
| `tb init --write-agents` | semeou `AGENTS.md` + `agent_docs.yaml` | **ajudou** | seed mínimo apontando verify/guard/mem |
| `tb mem init/add/list/check` | 4 invariantes; `compactacao_segura: true` | ajudou | `mem add` ainda só posicional |
| ticket manual `rd.md` (`## Scope`) | parse OK em drift/review | **ajudou** | sem `tb ticket` (docs: planejado Fase 3+) |
| `tb graph index` sem `--enable` | erro ADR-011 | neutro | esperado |
| `tb graph --enable index .` | 455 nós / 3054 arestas | **ajudou** | também OK em `/tmp/target` (fix SkipDirs) |
| `tb graph search/impact/deps/callers` | search achou `res.send*`; impact/deps de `lib/response.js` populados; callers `--function send` → `lib/response.js` | **ajudou** (melhor que R1) | R1: impact/deps vazios em `lib/request.js` |
| `tb verify run` (npm+lint+build+hidden) | `PASS`; hidden disponível sem dir | **ajudou** | iters=1; build/lint/visible/hidden true |
| `tb verify --ruler visual` | skip agregado honesto | ajudou | `skipped: true` com screens skipped |
| `tb review` | EXIT=0; 3 warnings soft de completeness | neutro | sem FP de `tests/hidden`; aceites genéricos pouco amarrados ao diff |
| `tb drift` | EXIT=0 no fix; EXIT=1 com `Readme.extra.md` fora do scope | **ajudou** | mandate Markdown funcionou |
| `tb guard check` | limpo no fix; yaml custom e thresholds hardcoded OK | **ajudou** | `--strict --json` exit 1 com violação |
| `tb bench run` / `compare` | default_agent e `--agent` OK; `--fake` OK | ajudou | 9 tasks; sucesso ~33% no echo/fake (esperado) |
| `tb mcp` | tools/list + `tb_version` OK | ajudou | 8 tools; graph tools pedem enable/env |

## Problemas que continuam

| problema (R1 / checklist) | status nesta rodada | evidência |
|---------------------------|---------------------|-----------|
| Config files mode `0600` | continua | `config.yaml` e até `AGENTS.md` semeado ficaram `600` |
| `tb ticket` inexistente | continua | docs/cli marca como Fase 3+; fluxo ainda exige `rd.md` manual |
| Review completeness ruidosa | continua | 3 warnings (“etag…”, “Uint8Array…”, “lint…”) sem olhar evidência real nos testes |
| Guard não analisa JS (skip/assert) | continua | só paths protegidos + heurísticas Go; diff mocha não gera `test_skip` |
| Grafo off por default | continua (por desenho ADR-011) | `index` sem `--enable` falha |
| `mem add` só posicional | continua | help/Args exigem `<regra>` posicional |
| Docs ainda listam stubs (`ticket`, `run`, `proc`…) | parcial | melhor alinhados (marcados planejados), mas árvore grande ainda mistura Fase 3 |

## Problemas novos

### P2 — `AGENTS.md` semeado com mode `0600`

- **Repro:** `tb init --write-agents` em repo sem AGENTS → `stat` mostra `600` em `AGENTS.md`.
- **Expected:** arquivo de documentação legível pelo time (`0644`) ou documentar a escolha.
- **Actual:** mesma permissão de configs sensíveis; atrito em clones compartilhados.

### P3 — Review completeness não amarra aceite a nomes de teste no diff

- **Repro:** aceites sobre ETag/Uint8Array/lint com testes e lint realmente no diff → ainda `warning` “pouco evidenciado”.
- **Expected:** detectar `it('…DataView…')` / menção a lint no ticket+verify como evidência.
- **Actual:** matching fraco de tokens; ruído em PRs bons.

### P3 — Bench default_agent `echo` marca ~33% sucesso

- **Repro:** `default_agent: echo …` + `tb bench run -r 1` → sucesso 33.3%.
- **Expected:** OK para smoke de wiring; mas fácil confundir com regressão de harness se o operador não notar que o agente é noop.
- **Actual:** harness “verde” com taxa baixa sem falhar o comando.

## O que o tb pegou e o que deixou passar

**Pegou**
- Toolchain npm no init; seed de `AGENTS.md` quando pedido.
- Hidden suite configurada sem pasta `tests/hidden/`.
- Build/lint no pipeline de verify (não só test).
- Drift/scope com heading Markdown + globs `**`.
- Guard custom via `guard.yaml` + exit code estrito com JSON.
- SkipDirs na raiz `/tmp/target` (regressão R1 corrigida).
- Visual skipped agregado honesto.
- Graph impact/deps/callers úteis em `lib/response.js` (JS `calls` presentes).

**Deixou passar / não ajudou**
- Não descobriu o bug sozinho (search por “DataView” vazio antes do fix; a pista veio da issue).
- Review não validou de fato os aceites IPv— DataView (só warnings soft).
- Sem `tb ticket`/board para empurrar fechamento do aceite.
- Guard não inspeciona conteúdo JS do patch (só path).
- Typed arrays multi-byte (`Uint16Array`, issue #7538) ficaram de fora do escopo — o tb não sugeriu ampliar.

Comparativo nativo: `npm test` (1292) + `eslint` validaram o fix ponta a ponta; o `tb verify` alinhado a esses comandos acompanhou o sinal (incluindo hidden grep e lint/build).

## Interação com instrumentação existente / onboarding

Express **não** tinha instrumentação de agente. Com as correções da rodada:

| Artefato | Gerado/lido? | Qualidade |
|----------|--------------|-----------|
| `.toolbelt/config.yaml` | sim | `test_cmd: npm test` correto; lint/build/hidden/default_agent editados à mão para o estudo |
| `AGENTS.md` via `--write-agents` | sim | esqueleto útil; mode `600`; hints vazios (`hints: []`) |
| `.toolbelt/agent_docs.yaml` | sim | registra `sources: [AGENTS.md]` |
| `CLAUDE.md` | n/a | inexistente; init não força criação |
| Ticket/Scope | manual `rd.md` | parser novo consumiu `## Scope` + listas + `**` |
| Guard manifesto | sim, honrado | `.toolbelt/guard.yaml` efetivo |
| MCP | sim | stdio OK |

Conclusão: o onboarding **melhorou bastante** vs R1 (não assume mais Go; pode semear AGENTS). Ainda falta empurrar lint/build automaticamente a partir de `package.json` scripts e um mandate/ticket de primeira classe.

## Comparação com a rodada 1

| Dimensão | R1 | R2 | Veredito |
|----------|----|----|----------|
| Init JS | `go test ./...` | `npm test` | **melhor** |
| AGENTS | não gerava | `--write-agents` funciona | **melhor** |
| Grafo em `/tmp/target` | 0 nós | 452 nós | **melhor** |
| impact/deps JS | vazios (request) | populados (response) | **melhor** |
| Hidden sem dir | ignorado | executa | **melhor** |
| build/lint no verify | inerte | honrados | **melhor** |
| guard.yaml / strict JSON | quebrados | OK | **melhor** |
| Scope Markdown / `**` | frágil | OK neste run | **melhor** |
| Visual skipped | `false` mentiroso | `true` agregado | **melhor** |
| default_agent / compare `--agent` | quebrados | OK | **melhor** |
| Completeness review / ticket CLI / mode 0600 | ruidoso / ausente / 0600 | igual | **igual** |
| Empurrar fechar a tarefa | fraco | ainda fraco | **igual** |

No agregado: **claramente melhor** nas 12 correções exercitáveis; lacunas de produto (ticket, completeness inteligente, análise JS no guard) permanecem.

## Sugestões de melhoria

1. **[P1][init]** Prefill `lint_cmd`/`build_cmd` a partir de `package.json` scripts (`lint`, `check`) quando existirem — área: init/config.
2. **[P1][review]** Aceites: considerar nomes `it`/`describe` no diff e status do último verify como evidência — área: review.
3. **[P2][init]** `AGENTS.md` seed com `0644`; configs secretas continuam `0600` — área: init.
4. **[P2][ticket]** `tb ticket` mínimo (create/show a partir de issue URL) ou template `rd.md` no init — área: ticket/onboarding.
5. **[P2][guard]** Heurísticas multi-linguagem leves (skip/assert em JS) ou aviso “scanner Go-only” — área: guard.
6. **[P3][bench]** Quando `default_agent` é noop/`echo`, avisar que a taxa de sucesso não mede o repo-alvo — área: bench UX.
7. **[P3][graph]** Search por APIs da issue (`DataView`) antes do patch ainda vazio — indexar comentários/identifiers de forma mais ampla — área: graph.

## Métricas

| métrica | rodada 2 | rodada 1 (express.md) |
|---------|----------|------------------------|
| Wall time comandos cronometrados (soma) | ~19,0 s (46 invocações rotuladas no log) | ~20,1 s (76 invocações) |
| Wall time sessão instrumentada (aprox.) | ~4–5 min incl. npm install, tarefa, bateria tb | ~2,5 min cmds + ~15 s setup |
| `EXIT=0` / `EXIT≠0` no log R2 | 40 / 6 (negativos esperados: drift OOS, guard strict, graph off, hidden fail) | 54 / 22 |
| Falhas inesperadas do tb | nenhuma das 12 correções falhou no que era aplicável | SkipDirs, hidden gate, Go default, guard.yaml, default_agent, visual skipped |
| Commits locais no alvo (não publicados) | 1 fix DataView (+ commits experimentais revertidos para drift/guard) | fix IPv6 + experimentais |
| Binário tb | `a46c295` | `c8aae04` |

Log bruto da sessão: `/tmp/tb-campo2-express.log` (VM do estudo; não versionado).
