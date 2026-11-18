[🇺🇸 English](../../campo/express.md) | 🇧🇷 Português

# Estudo de campo — expressjs/express

Grupo: sem instrumentação de agente de IA no repositório-alvo.
Clone local apenas (`/tmp/target`); sem push/issue/PR/comentário no upstream.
`tb` buildado deste repo: `make build` → `bin/tb` (ldflags com commit `c8aae04`).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [expressjs/express](https://github.com/expressjs/express) |
| Linguagem | JavaScript (Node.js), CommonJS |
| Versão no clone | 5.3.0 (shallow `HEAD` `43fca52`) |
| Tamanho | ~2,0 MiB sem `node_modules`; 217 arquivos de fonte; ~142 `.js`; ~22 kLOC JS |
| Instrumentação AI existente | Nenhuma encontrada (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, copilot instructions ausentes) |
| Toolchain | `npm install` OK (Node v22.23.4, npm 10.9.9). Scripts: `npm test` (mocha), `npm run lint` (eslint 8). Sem passo de `build` de produção. Cobertura/`nyc` disponível via `test-ci`/`test-cov` — não exigidos para a tarefa. |
| Não configurado | Redis real para exemplos/sessões (devDependency `connect-redis`); não necessário aos testes unitários usados. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `req.subdomains` para hosts IPv6 com pontos (ex. `[::ffff:127.0.0.1]`), que eram partidos em “subdomínios” porque `net.isIP()` recebia o literal com colchetes e retornava `0`. |
| Por quê | Bug real, escopo pequeno (`lib/request.js` + testes), maintainer-plausível, com repro claro. |
| Issue | https://github.com/expressjs/express/issues/7532 |
| Completou? | Sim, localmente (commit só no clone `/tmp/target`, sem push). |
| Diff | Strip de `[…]` antes de `isIP`; 2 testes novos em `test/req.subdomains.js`. |
| Checks nativos | `npm test -- --grep subdomains` → 13 passing; `npm test` completo → PASS (~2,5 s); `npm run lint` → PASS. Repro manual: antes `["0","[::ffff:127"]`, depois `[]`. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` (build com ldflags) | OK — `toolbelt c8aae04` | neutro | `EXIT=0 TIME=0.005s` |
| `tb version` (build sem ldflags) | `0.1.0-dev`; `commit: unknown`; `built: unknown` | atrapalhou | `/tmp/tb-nolflags version` |
| `tb init` | criou `.toolbelt/config.yaml` (modo `600`); defaults Go (`verify.test_cmd: go test ./...`); sem `AGENTS.md` | atrapalhou (onboarding JS) | path `/tmp/target/.toolbelt`; nenhum arquivo de agente gerado |
| `tb mem init/add/list/check` | OK; 5 invariantes; `compactacao_segura: true` | ajudou | `mem add` exige arg posicional; arquivos `0600` |
| `tb ticket …` | comando inexistente | atrapalhou | `Error: unknown command "ticket"`; docs/cli.md ainda documenta |
| ticket manual `rd.md` (Scope) | parse OK por `drift`/`review` | ajudou | `scope: lib/request.js,test/req.subdomains.js` + `base_commit` |
| `tb graph index` / `index .` em `/tmp/target` | 0 nós / 0 arestas, EXIT=0 | atrapalhou | basename do root `target` ∈ `SkipDirs` → `filepath.SkipDir` na raiz |
| `tb graph --enable index lib` | 130 nós, 426 arestas | ajudou (parcial) | preciso passar subpath; `index` sem `--enable` falha (ADR-011) |
| `tb graph index .` em `/tmp/express-src` | 452 nós, 3051 arestas | confirma causa | mesmo código, dirname ≠ `target` |
| `tb graph search/callers/impact/deps` | search achou `subdomains`; impact/deps em `lib/request.js` vazios; callers fraco | neutro/atrapalhou | impact `(vazio)` mesmo após index de `lib` |
| `tb graph impact` sem `--file` | erro obrigatório | neutro | `Error: --file é obrigatório` |
| `tb verify run` (npm configurado) | PASS; oculto `disponível: false` | ajudou (parcial) | hidden só ativa se existir dir `tests/hidden/`, ignora `--hidden-test-cmd` sozinho |
| `tb verify` defaults Go no repo JS | ESCALATE (3 iters) | atrapalhou | `go test ./...` sem módulo Go |
| `tb verify --test-cmd true` | PASS apesar de JS quebrado fora do check | atrapalhou | demonstra checks parciais |
| `tb verify --ruler visual` sem manifesto | PASS `skipped: true` | neutro | manifesto ausente |
| `tb verify --ruler visual` com screens sem refs | PASS report `skipped: false` mas screens `skipped: true` | atrapalhou | falso “não skipped” no relatório agregado |
| `tb review` (diff bom) | EXIT=0; warning completeness em aceite de regressão | neutro | FP soft: “Testes existentes… continuam passando” |
| `tb review -o` | falha | atrapalhou | só `--output`, não `-o` (help lista `--output`) |
| `tb drift` (paths literais) | EXIT=0, sem out_of_scope | ajudou | bateu com o scope do ticket |
| `tb drift` com `lib/*.js` | EXIT=0 (glob simples funciona) | neutro | `filepath.Match` |
| `tb drift` com `src/**/*.ts` | EXIT=1, out_of_scope real | neutro | `**` no meio não é suportado |
| `tb guard check` no fix | sem violações | neutro | não varre JS (só heurísticas Go + paths) |
| `tb guard` + `.toolbelt/guard.yaml` custom | yaml ignorado | atrapalhou | protected custom em `lib/request.js` não gerou violation |
| `tb guard` em `.toolbelt/thresholds.yaml` | `protected_path` | ajudou | lista hardcoded |
| `tb verify` após tocar thresholds | FAIL iters=0 (abort pelo guard) | atrapalhou no fluxo verify | confirma abort quando path hardcoded muda |
| `tb bench run` sem `--agent`/`--fake` | erro, ignora `bench.default_agent` | atrapalhou | config tinha `echo DEFAULT_AGENT_RAN {prompt}` |
| `tb bench run --fake` | OK sem `--fake-config` | ajudou | 9 tasks, ~3,8 s |
| `tb bench compare` | A/B com `--fake` OK; flag `--agent` inexistente | atrapalhou (A/B real) | `unknown flag: --agent` |
| `tb mcp` | tools/list + tools/call `tb_version` OK via stdio | ajudou | 8 tools expostas; graph tools pedem `TB_GRAPH=on` |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | confirmado | `tb bench compare --agent …` → `unknown flag: --agent`; help só tem `--fake` |
| `guard.yaml` ignored | confirmado | `.toolbelt/guard.yaml` protegendo `lib/request.js` → violations null no mesmo diff |
| outdated CLI docs | confirmado | `docs/cli.md` documenta `tb ticket`, `tb run`, `tb proc`, `tb board`, etc.; CLI top-level não tem `ticket`/`run`/`proc`/`board`/`merge`/`md`/`agent` |
| version shows `unknown` | confirmado (parcial) | sem ldflags: versão `0.1.0-dev`, mas `commit` e `built` = `unknown` |
| config files created 0600 | confirmado | `stat`: `600` em `config.yaml`, `invariants.yaml`, `graph.json` |
| visual ruler reports `skipped=false` when skipping | confirmado | manifesto presente, refs ausentes: report `"skipped": false` + screen `"skipped": true`, verdict PASS |
| `default_agent` ignored | confirmado | `bench.default_agent` setado; `tb bench run` ainda exige `--agent` ou `--fake` |
| guard aborts verify when diff touches hardcoded config path | confirmado | diff em `.toolbelt/thresholds.yaml` → verify `FAIL (iterações: 0)` |
| default fake agent needs `--fake-config` | refutado | `tb bench run --fake` OK sem flag; até `--fake-config` apontando arquivo inexistente ainda rodou |
| `mem add` positional | confirmado | `tb mem add --category security` → `requires at least 1 arg(s)`; só `<regra>` posicional |
| Drift/Scope need literal paths (no globs) | refutado (parcial) | `lib/*.js` funciona; `src/**/*.ts` não; sufixo `/**` é prefixo especial no código |
| `graph impact` needs `--file` | confirmado | `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | refutado (neste caso) | no diff limpo, invariante `tests/hidden` não disparou; no cheat commit, disparou corretamente. Houve FP de *completeness*, não dessa invariante |
| verify runs only partial checks and misses build errors | confirmado | defaults Go no repo JS; `--test-cmd true` dá PASS; hidden_cmd ignorado sem dir `tests/hidden`; init não detecta `npm test`/`eslint` |
| tb does not push agent toward completing whole task | confirmado (como gap de produto) | sem `tb ticket`/board/progress; review só warning soft; init não cria harness de progresso nem checklist viva; agente precisa improvisar `rd.md` |

## Problemas novos

### P0 — `graph` não indexa repo cujo basename ∈ `SkipDirs` (ex.: diretório `target`)

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb graph --enable index .` → `0 nós`. `clone … /tmp/express-src && tb graph --enable index .` → `452 nós`. `SkipDirs` inclui `"target"` (Rust/Java) e o walk faz `SkipDir` quando `info.Name()` da raiz é `target`.
- **Expected:** indexar a árvore do projeto; skip só de *subdiretórios* ignoráveis.
- **Actual:** grafo vazio com EXIT=0 (falso sucesso).

### P1 — `verify` só considera hidden suite se existir `tests/hidden/`, ignorando `--hidden-test-cmd` / config

- **Repro:** configurar `hidden_test_cmd: npm test -- --grep IPv6-mapped` sem criar o diretório → `Oculto: false (disponível: false)`.
- **Expected:** comando hidden configurado roda (ou aviso explícito).
- **Actual:** flag/config silenciosamente inerte.

### P1 — `tb init` assume toolchain Go e não detecta npm/eslint/mocha

- **Repro:** `tb init` em express → `verify.test_cmd: go test ./...`.
- **Expected:** detectar `package.json` scripts ou perguntar.
- **Actual:** verify default inutilizável até edição manual.

### P2 — `graph impact` / `deps` retornam vazio para arquivo JS central após index

- **Repro:** indexar `lib`, `tb graph --enable impact --file lib/request.js` → `(vazio)`; `deps --file lib/request.js` → `(vazio)`.
- **Expected:** pelo menos imports/`requires` e arquivos impactados.
- **Actual:** sem sinal útil para planejamento da mudança.

### P2 — relatório visual agrega `skipped: false` com screens skipped e PASS

- Já listado como conhecido; reforçado aqui com manifesto malformado/refs ausentes no Express (sem UI).

### P3 — UX: `tb review -o` rejeitado; help não mostra shorthand

- **Repro:** `tb review … -o /tmp/x.json` → `unknown shorthand flag: "o"`.
- **Expected:** `-o` alias de `--output` (como em `verify`/`bench`).
- **Actual:** só `--output`.

### P3 — `guard` não analisa JS (skip/eq/etc.)

- **Repro:** alterações em testes mocha não acionam `test_skip`/`trivial_assert` (scanner Go-only).
- **Expected:** ao menos heurísticas multi-linguagem ou aviso.
- **Actual:** só `protected_path` hardcoded + AST Go.

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de escopo quando paths fora do ticket (`tests/hidden/cheat.js`).
- Violação de invariante ao tocar `tests/hidden/`.
- Path protegido `.toolbelt/thresholds.yaml` e abort correspondente no verify.
- Review freshness + warning de completeness (ainda que ruidoso).
- Graph search encontrou `subdomains` depois de indexar `lib`.
- Bench fake e compare A/B (`--fake`) funcionaram como harness.

**Deixou passar / não ajudou**
- Não guiou a descoberta do bug nem sugeriu checar `isIP`+brackets (graph impact/deps vazios).
- Não configurou verify/lint/test do Express sozinho.
- `guard.yaml` do operador ignorado — não dá para proteger paths do projeto.
- Não exige (nem acompanha) suite nativa completa; com `--test-cmd` estreito ou `true`, passa.
- Review não validou de fato os critérios de aceite IPv6 (só o texto genérico de regressão).
- Não há ticket/board para empurrar “feche o aceite / rode lint+test full”.
- Index default no path `/tmp/target` falhou em silêncio (0 nós).

Comparativo nativo: `npm test` + `npm run lint` validaram o fix de ponta a ponta; o tb verify, uma vez apontado para `npm test -- --grep subdomains`, acompanhou o sinal dos testes visíveis, mas sem hidden real e sem lint/build dedicados.

## Interação com instrumentação existente / onboarding

O Express **não** tem instrumentação de agente. O onboarding do tb a partir do zero:

| Artefato | Gerado? | Qualidade |
|----------|---------|-----------|
| `.toolbelt/config.yaml` | sim | genérico Go; graph languages OK (inclui javascript); verify/bench errados para Node |
| `.toolbelt/invariants.yaml` via `mem init` | sim | anti-cheat útil, mas centrado em `tests/hidden` / thresholds (convenção tb, não do Express) |
| `AGENTS.md` / skills / progress log | não | lacuna vs achados de pesquisa (AGENTS.md, harness de longo prazo) |
| Detecção automática test/lint/build | não | precisei editar YAML à mão |
| Ticket/Scope | não (comando ausente) | criei `rd.md` manualmente; isso funcionou com drift/review |
| Guard manifesto | docs citam `guard.yaml`; runtime ignora | confunde onboarding |
| MCP | sobe fácil (`tb mcp`) | útil; tools de graph dependem de enable/env |

Conclusão: o tb **inicializa um esqueleto**, mas **não bootstrapa** um repo JS “do nada” até um fluxo verificável. Falta detecção de toolchain, geração de contexto para agentes (`AGENTS.md`), e um mandate/ticket de primeira classe.

## Sugestões de melhoria

1. **[P0][graph]** Não aplicar `SkipDirs` ao diretório raiz do walk; comparar path relativo, não só `filepath.Base` da raiz. (área: graph)
2. **[P1][verify]** Ativar hidden quando `hidden_test_cmd`/config estiver setado; ou falhar alto se a dir faltar. Auto-sugerir comandos a partir de `package.json` / Makefile. (verify / init)
3. **[P1][init]** Detectar ecossistema (npm/go/cargo…) e preencher `verify.test_cmd`, lint, e opcionalmente gerar `AGENTS.md` + `rd.md` template. (init / mem)
4. **[P1][guard]** Honrar `.toolbelt/guard.yaml`; separar “abort verify” de “reportar violation” com flag. (guard / verify)
5. **[P1][bench]** Respeitar `default_agent`; adicionar `--agent` em `bench compare`. (bench)
6. **[P2][graph]** Fazer `index` default percorrer o repo mesmo sem args; impact/deps JS via `require`/`import`. (graph)
7. **[P2][docs]** Sincronizar `docs/cli.md` / README com comandos realmente implementados; marcar stubs. (docs)
8. **[P2][review]** Critérios de aceite: exigir evidência de teste (nome de describe/it no diff) antes de warning; alinhar `-o` com outros comandos. (review)
9. **[P2][verify/visual]** Se todas as screens foram skipped, report `skipped: true` (não PASS silencioso). (verify)
10. **[P3][mem/UX]** Aceitar `mem add --rule` além de posicional; documentar modos de arquivo `0600` ou usar `0640`/`0644` para config não-secreta. (mem / init)
11. **[P3][router/ticket]** Implementar `tb ticket` mínimo ou remover da docs — sem isso o tb não empurra fechamento da tarefa. (ticket / router)

## Métricas

| métrica | valor |
|---------|-------|
| Wall time da sessão instrumentada (STUDY_START→END) | ~147 s (~2,5 min) de comandos cronometrados em sequência; setup npm/clone/build à parte (~15 s) |
| Soma dos tempos reportados nos comandos logados | ~20,1 s |
| Invocações rotuladas no log (`===== … =====`) | 76 |
| Resultados `EXIT=0` / `EXIT=1` no log | 54 / 22 (muitos `EXIT=1` são testes negativos esperados) |
| Falhas inesperadas relevantes do tb | graph 0 nós em `/tmp/target`; `ticket` missing; `default_agent` ignorado; `guard.yaml` ignorado; visual `skipped=false` |
| Commits locais no alvo (não publicados) | fix IPv6 + commits experimentais de thresholds/hidden para provar guard |

Log bruto da sessão: `/tmp/tb-campo-express.log` (VM do estudo; não versionado).
