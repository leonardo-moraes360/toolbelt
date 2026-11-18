[🇺🇸 English](../../campo/playwright-mcp.md) | 🇧🇷 Português

# Estudo de campo: microsoft/playwright-mcp

Evidência coletada em 2026-11-15 no VM local. Clone em `/tmp/target/playwright-mcp` (shallow). Nenhuma interação com o remoto do alvo (sem push/issue/PR/comentário). Binário `tb` buildado deste repo (`make build` → `toolbelt c8aae04`). Log bruto: `/tmp/tb-campo-evidence.log`.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [microsoft/playwright-mcp](https://github.com/microsoft/playwright-mcp) (`@playwright/mcp` 0.0.85) |
| Linguagem | JavaScript/TypeScript (wrapper npm; servidor MCP real vive em `microsoft/playwright` / `playwright-core`) |
| Tamanho | ~49 arquivos fora de `node_modules`/`.git`; ~552 KiB tree; ~3,5 kLOC contados em `.js`/`.ts`/`.md` |
| Instrumentação AI existente | `AGENTS.md`; `.claude/skills/release.md`; workflows `.github/workflows/agent-{triage,review,task,roll,release}.yml` |
| Toolchain | **OK:** Node 22, `npm ci`, Chrome do sistema + Playwright Chromium/Chrome, Firefox baixado. **Não exercitado de ponta a ponta:** WebKit, `npm test` multi-browser, `dtest`/Docker, roll a partir de checkout `../playwright`. `npm run build` é no-op (`echo OK`). |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir geração de docs de env vars em `update-readme.js` + regenerar `README.md` + regressão em `tests/cli.spec.ts` |
| Por quê | Bug real e local a este repo (não exige monorepo upstream); maintainer-plausível; tem issue aberta e caminho de teste claro |
| Issue | https://github.com/microsoft/playwright-mcp/issues/1861 |
| Concluída localmente? | **Sim** no clone (commit local `80d3280`); **não** enviada ao upstream (proibido pelas regras do estudo) |
| Resultado nativo | `npx playwright test tests/cli.spec.ts --project=chrome`: **2 passed**. `npm run lint`: **OK** após o fix. Com erro de sintaxe introduzido só em `update-readme.js`, `node --check` e `npm run lint` falham, mas o teste de README ainda passa |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK (~7 ms); cria `.toolbelt/config.yaml` | atrapalhou (defaults) | Defaults `verify.test_cmd: go test ./...` e `hidden_test_cmd: go test ./tests/hidden/...` num repo JS; não lê `AGENTS.md` |
| `tb mem init/list/add/check` | OK | neutro→ajudou | `mem add --type fact` falha (`unknown flag`); `mem add "<regra>" --category mandate` funciona; `check` SHA256 OK |
| Ticket / Scope (`rd.md` manual) | OK via arquivo; `tb ticket` **inexistente** | atrapalhou | `tb ticket` → `unknown command`; Scope em frontmatter YAML funcionou com paths literais |
| `tb graph --enable rebuild/stats/search/deps/impact/index` | rebuild 132 nós / 413 arestas; search achou `optionEnvName` | ajudou | `impact` sem `--file` exit 1 (`--file é obrigatório`); com `--file update-readme.js` OK; deps listou `fs`/`path`/`playwright-core` |
| `tb verify run` (tests) | PASS com `--project=chrome`; ESCALATE com `--project=chromium` (projeto inexistente) | atrapalhou se mal configurado | Só roda `test_cmd`; não inclui `npm run lint`/`node --check`; hidden só se existir `tests/hidden/` |
| `tb verify run` (visual) | PASS | atrapalhou (UX) | Manifesto ausente: `skipped=true`. `screens: []`: `skipped=false` com `screens: null` e PASS |
| `tb review` | PASS no fix limpo; FAIL (false positive) se diff menciona `tests/hidden` | atrapalhou | Comentário `// note: do not touch tests/hidden` gerou finding critical idêntico a tocar o path de verdade |
| `tb drift` | OK com scope literal; FAIL com `**/*.js` | ajudou (literal) / atrapalhou (glob `**`) | Scope literal: sem drift. `**/*.js`/`**/*.md`: README.md e update-readme.js “fora do scope” |
| `tb guard check` | OK no fix; detecta `.toolbelt/thresholds.yaml`; **ignora** `.toolbelt/guard.yaml` | atrapalhou | `package.json` em `guard.yaml` protected_paths → `violations: null`; thresholds hardcoded → violation + verify FAIL imediato |
| `tb bench run/list/compare` | list/run --fake OK; compare A/B OK; `--agent` em compare rejeitado | neutro | `default_agent` no config **ignorado** (`--agent ou --fake é obrigatório`). `--fake` **sem** `--fake-config` funcionou |
| `tb mcp` | tools/list OK | ajudou | Resposta JSON-RPC com `tb_version`, `tb_run`, `tb_graph_*`, `tb_echo`, `tb_env` |
| `tb version` | `c8aae04` via `make build`; `commit: unknown` via `go build` sem ldflags | neutro | Confirma ambiguidade de versão conforme modo de build |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare ... --agent 'echo hi'` → `unknown flag: --agent`; help só lista `--fake` |
| `guard.yaml` ignored | **confirmado** | Arquivo `.toolbelt/guard.yaml` com `package.json`; diff só em `package.json` → guard `violations: null`. Docs (`docs/cli.md`, `docs/arquitetura.md`) ainda prometem `--manifest`/guard.yaml; CLI real não tem `--manifest` |
| outdated CLI docs | **confirmado** | `docs/cli.md` documenta `tb mem add --type/--title`; CLI exige `tb mem add <regra>`. Guard docs divergem do código |
| version shows `unknown` | **confirmado** (sem ldflags) | `go build -o /tmp/tb-nolflags ./cmd/tb` → `commit: unknown` / `built: unknown`. Com `make build`, versão = git describe |
| config files created 0600 | **confirmado** | `stat`: `.toolbelt/config.yaml` e `invariants.yaml` → modo `600` |
| visual ruler `skipped=false` when skipping | **confirmado** | `screens.yaml` com `screens: []` → `"skipped": false`, `"screens": null`, `"verdict": "PASS"` |
| `default_agent` ignored | **confirmado** | Config com `default_agent: "echo AGENT_OK {prompt}"`; `tb bench run` sem flags → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | Diff em `.toolbelt/thresholds.yaml` → guard error + `tb verify` verdict `FAIL` iterations 0 (não roda testes) |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake` sem `--fake-config` completou 9 tarefas; `FakeAgentCommand("")` omite `--config` de propósito |
| `mem add` positional | **confirmado** | Help: `tb mem add <regra>`; flags docs `--type/--title` inexistentes |
| Drift/Scope need literal paths (no globs) | **parcialmente confirmado** | `filepath.Match` simples (`update-readme.*`) funciona; `**/*.js` **não** casa (README/update-readme fora do scope) |
| `graph impact` needs `--file` | **confirmado** | Sem flag: `Error: --file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmado** | Diff que só **menciona** a string `tests/hidden` em comentário → finding critical “possível violação” |
| verify runs only partial checks / misses build errors | **confirmado** | Com `update-readme.js` sintaticamente quebrado: `node --check` e `npm run lint` falham; `tb verify` com só `tests/cli.spec.ts` → **PASS** |
| tb does not push agent toward completing whole task | **confirmado** (observacional) | Sem `tb ticket`/board/progress; init não importa `AGENTS.md` (que avisa que a maior parte dos bugs é upstream); verify verde sem lint/build |

## Problemas novos

| ID | sev | repro | esperado | atual |
|----|-----|-------|----------|-------|
| N1 | P1 | `tb init` em repo Node; rodar `tb verify run` sem editar config | Defaults alinhados à toolchain detectada, ou onboarding que falha cedo com mensagem clara | Defaults Go; ESCALATE 3× se o agente copiar `chromium` do AGENTS.md enquanto o `playwright.config.ts` só define project `chrome` |
| N2 | P2 | `tb drift` com scope `**/*.js` | Ou suporte a `**`, ou erro explícito “glob não suportado” | Trata como não-match silencioso → falso drift |
| N3 | P2 | `tb review` + invariante default `tests/hidden` + diff com a substring no conteúdo | Heurística por path tocado, não substring no diff | False positive bloqueante |
| N4 | P2 | Criar `.toolbelt/guard.yaml` e alterar path listado | Guard carrega o manifesto (como a docs promete) | Manifesto ignorado; só lista hardcoded em `internal/guard/guard.go` |
| N5 | P3 | `tb verify --ruler visual` com manifesto vazio `screens: []` | `skipped=true` + razão “nenhuma tela” | `skipped=false`, PASS |
| N6 | P3 | `tb mem add --type fact ...` seguindo `docs/cli.md` | Comando da docs funciona ou docs atualizadas | `unknown flag: --type` |
| N7 | P1 | Agente usa só `tb verify` configurado com teste parcial após editar gerador README | Verify exige (ou sugere) também `npm run lint` / syntax check do gerador | PASS com gerador quebrado |

## O que o tb pegou e o que deixou passar

**Pegou**
- Scope/drift: alteração de `package.json` fora do mandate (forbidden).
- Guard: alteração de `.toolbelt/thresholds.yaml` (path hardcoded).
- Graph: localizou `optionEnvName` e dependências de `update-readme.js`.
- Review no diff limpo do fix: sem findings.
- MCP: ferramentas listáveis via stdio.

**Deixou passar**
- Env vars fabricadas no README **antes** do fix (tb não tem ruler de docs↔código).
- `guard.yaml` customizado (package.json “protegido” na config do usuário).
- Erro de sintaxe em `update-readme.js` quando `test_cmd` só lia o README.
- Ausência de `npm run lint` no loop de verify (lint é o check nativo que regenera/valida README).
- Invariantes default Go-centricas (`tests/hidden/`, thresholds) sem relação com este repo.
- Orientação crítica de `AGENTS.md` (“most bugs/features are upstream”) — tb não a importou.

**Nativo vs tb:** nativo (`lint` + `cli.spec.ts` no chrome) valida o fix completo; tb só espelha o que estiver em `test_cmd`. Com config errada (`chromium`), tb falhou onde o ajuste de project name resolve.

## Interação com instrumentação existente / onboarding

- `tb init` **ignora** `AGENTS.md`, `.claude/skills/` e workflows `agent-*`. Vault permanece vazio; invariantes são o template anti-trapaça genérico.
- Conflito potencial: `AGENTS.md` instrui `npx playwright test ... --project=chromium`, mas este checkout só declara project `chrome` → agente que seguir AGENTS.md + `tb verify` copia o comando e recebe ESCALATE.
- Duplicação: mem/invariants e AGENTS.md ambos falam de processo, mas em vocabulários diferentes; tb não linka os dois.
- O que tb deveria importar: (1) trechos de build/test de `AGENTS.md` → `verify.test_cmd` / checklist; (2) aviso “fix here only what this repo owns” → mandate; (3) convenção de commit/branch; (4) skills `.claude/skills/*` como playbooks opcionais no vault.

## Sugestões de melhoria

1. **[P0][verify/init]** Detectar toolchain (package.json scripts) no `tb init` e preencher `test_cmd`/`lint_cmd`/`build_cmd`; verify rodar lint+test+build configuráveis.  
2. **[P0][guard]** Carregar `.toolbelt/guard.yaml` de verdade (ou remover das docs); unificar com a lista hardcoded.  
3. **[P1][docs/cli]** Sincronizar `docs/cli.md` com a CLI real (`mem add`, guard flags, bench compare).  
4. **[P1][mem/onboarding]** Importar/resumir `AGENTS.md` (e skills) no vault no init; flag `--import-agents`.  
5. **[P1][review]** Matching de invariantes por path tocado / AST, não substring no diff.  
6. **[P2][drift]** Suporte explícito a `**` ou rejeição clara de globs não suportados.  
7. **[P2][bench]** Honrar `default_agent`; opcionalmente `--agent` em `bench compare` A/B.  
8. **[P2][verify/visual]** Marcar `skipped=true` quando não há screens / refs.  
9. **[P3][init]** Permissões de config 0644 (ou documentar 0600).  
10. **[P3][ticket]** Entregar `tb ticket` mínimo (já usado via `rd.md` ad hoc) para fechar o loop Scope→verify→review.

## Métricas

| métrica | valor |
|---------|-------|
| Wall time (log evidência) | ~20:43:07–20:46:30 UTC (~3,4 min de comandos cronometrados; build `tb` ~6 s antes) |
| Soma das durações cronometradas | ~41,2 s |
| Invocações `tb-*` no log | 43 |
| Falhas `tb-*` (exit ≠ 0) | 14 (inclui falhas esperadas de probes: impact sem file, compare --agent, default_agent, globs, false positive review, verify mal configurado, etc.) |
| Invocações totais logadas (tb+nativo) | 56 |
| Tarefa local | concluída (fix #1861) com testes chrome + lint verdes |
