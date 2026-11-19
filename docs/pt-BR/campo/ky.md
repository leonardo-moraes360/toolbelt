[🇺🇸 English](../../campo/ky.md) | 🇧🇷 Português

# Estudo de campo: `tb` em sindresorhus/ky

Evidência coletada em 2026-11-15 num clone local em `/tmp/target` (sem push, issue, PR ou comentário no upstream). `tb` compilado deste repo (`make build` → `bin/tb`, versão `c8aae04`). Mudança de produto só neste arquivo; o trabalho no ky ficou restrito à VM.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [sindresorhus/ky](https://github.com/sindresorhus/ky) |
| Linguagem | TypeScript (ESM), HTTP client sobre Fetch |
| Tamanho | ~116 arquivos fora de `node_modules`/`.git`/`distribution`; ~1,9 MiB; ~4,9 kLOC em `source/**/*.ts`, ~30,9 kLOC em `test/**/*.ts` |
| Instrumentação AI | `AGENTS.md` (convenção `null` vs `undefined`); sem `CLAUDE.md` / `.cursorrules`; CI em `.github/workflows/main.yml` |
| Toolchain | **Sim, com ressalvas.** Node v22.23.4 + npm 10.9.9; `npm install` + `npm run build` OK; Playwright Chromium/Firefox/WebKit instalados; `xo` OK (warnings pré-existentes); suíte não-browser `ava` 1528 passed após a mudança. Amostra Chromium de `test/browser.ts`: 23 passed. Não rodei a matriz completa firefox+webkit+todos os browsers no `npm test` final (custoso); no primeiro `npm test` antes de instalar todos os browsers houve 41 falhas por binários Playwright ausentes. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Rejeitar retornos inválidos de `retry.jitter` (função) com `TypeError` (`must return a non-negative finite number`), encadeando o erro original como `cause`; atualizar exemplos Absolute jitter para `Math.max(0, ...)`; testes em `test/retry.ts` e `test/retry-decisions.ts`. |
| Por quê | `retry.delay` já rejeita NaN/negativo; `retry.jitter` fazia fallback silencioso para o delay sem jitter (esconde misconfiguração). Alinha com `AGENTS.md` (erro claro em vez de fallback). Mantenedor plausível. |
| Issue | Nenhuma issue aberta útil no momento da coleta (`gh issue list --state open` vazio). Tarefa derivada do código + convenção local, não de issue. |
| Completa? | **Sim, localmente** (commit `056a1f0` só no clone; não enviado ao upstream). |
| Checks nativos | `npm run build` OK; `xo` nos arquivos tocados: 2 warnings pré-existentes; `tsc --project tsconfig.test.json --noEmit` OK; `ava 'test/!(browser).ts'` → **1528 passed**; amostra Chromium OK. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; cria `.toolbelt/` + `config.yaml` | neutro | 0,007 s; config nasce `0600` |
| `tb mem init/list/add/check` | OK com posicional; flags `--type/--title/--body` da docs falham | ajudou (invariantes) / atrapalhou (docs) | `mem add "…"` OK; `unknown flag: --type` |
| `tb graph index` sem `--enable` | erro ADR-011 | neutro | `grafo desabilitado` |
| `tb graph index --enable` em `/tmp/target` (cwd) | **0 nós** | atrapalhou | SkipDirs inclui `target`; basename do clone |
| `tb graph index --enable source` | 180 nós / 631 arestas | ajudou | indexação TS útil |
| `tb graph index --enable .` em `/tmp/kyrepo` | 590 nós / 3804 arestas | ajudou | confirma bug do nome `target` |
| `tb graph impact --file` | exige `--file`; path relativo ao root indexado | atrapalhou UX | `source/core/Ky.ts` → vazio se root=`source`; `core/Ky.ts` → hit |
| `tb graph search/callers/deps` | search por título parcial OK; `#calculateDelay` privado não indexado; callers vazio | neutro/atrapalhou | métodos `#` privados sumidos do grafo |
| tickets / Scope | **sem** `tb ticket`; `rd.md` manual com frontmatter YAML | atrapalhou | docs listam `tb ticket`; CLI: `unknown command` |
| `tb verify` (tests) | PASS nos testes configurados; hidden indisponível | ajudou parcial | `visible_passed=true`, `hidden_available=false` |
| `tb verify` (visual) | PASS com screens `skipped=true`, report `skipped=false` | atrapalhou | JSON em `/tmp/tb-evidence/verify-visual.json` |
| `tb review` | exit 1 com falsos positivos de invariantes | atrapalhou | flags `tests/hidden` e `thresholds.yaml` no diff do ticket |
| `tb drift` | OK com paths literais; globs rasos funcionam | ajudou | drift final `out_of_scope: null` |
| `tb guard check` | hardcoded OK; `guard.yaml` ignorado | atrapalhou config | `source/index.ts` no yaml custom → sem violation; `.toolbelt/thresholds.yaml` → violation |
| `tb bench run/compare` | `--fake` OK; `default_agent` ignorado; compare sem `--agent` | neutro/atrapalhou | `--agent ou --fake é obrigatório` mesmo com `default_agent` no yaml |
| `tb mcp` | sobe stdio; `tools/list` e `tb_version` OK | ajudou | 8 tools (`tb_echo`, `tb_env`, `tb_run`, graph_*, `tb_version`) |
| `tb version` | com ldflags: `c8aae04`; sem: `0.1.0-dev` + `commit/built: unknown` | neutro | `/tmp/tb-nolink version` |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | help só tem `--fake` / JSONL / configs A/B; sem `--agent` |
| `guard.yaml` ignored | **confirmado** | `.toolbelt/guard.yaml` com `source/index.ts`; diff nesse path → `violations: null` |
| outdated CLI docs | **confirmado** | docs: `tb ticket/run/proc/md/agent`, `mem add --type`; CLI real sem esses comandos/flags |
| version shows `unknown` | **confirmado** (sem ldflags) | `go build` puro → `commit: unknown`, `built: unknown` (versão `0.1.0-dev`) |
| config files created 0600 | **confirmado** | `tb init` → `.toolbelt/config.yaml` mode `600` |
| visual ruler `skipped=false` when skipping | **confirmado** | report `"skipped": false` com screens `"skipped": true` / `referência ausente` |
| `default_agent` ignored | **confirmado** | yaml com `default_agent: "echo AGENT {prompt}"`; `bench run` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | diff em `.toolbelt/thresholds.yaml` → verify `FAIL`, `iterations: 0`, violation `protected_path` |
| default fake agent needs `--fake-config` | **parcialmente refutado** | `--fake` sem config roda (sucesso 0 %); `--fake-config` opcional |
| `mem add` positional | **confirmado** | `tb mem add <regra>`; flags docs `--type/--title/--body` inexistentes |
| Drift/Scope need literal paths (no globs) | **parcialmente refutado** | `test/*.ts` e `source/**/*.ts` (1 nível) casam via `filepath.Match`; `**` recursivo real não; literais mais previsíveis |
| `graph impact` needs `--file` | **confirmado** | `--file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmado** | findings critical mesmo sem tocar `tests/hidden/` (texto no diff/`rd.md`) |
| verify runs only partial checks / misses build errors | **confirmado** | erro TS em `source/index.ts`: `npm run build` falha; `tb verify` com só `ava` → PASS; hidden default Go path `hidden_available=false` |
| tb does not push agent toward completing whole task | **confirmado** (qualitativo) | sem `tb ticket`/board/progress; agente monta `rd.md`/verify na mão; review/guard geram atrito sem checklist de conclusão |

## Problemas novos

### P0 — `SkipDirs` trata o basename do root como diretório a pular

- **Repro:** clonar em `/tmp/target` (ou qualquer pasta chamada `target`/`build`/`dist`/`bin`/…) e rodar `tb graph index --enable .` → `0 nós`. Mesmo tree em `/tmp/kyrepo` → centenas de nós.
- **Expected:** pular só *subdiretórios* com esses nomes, nunca a raiz do walk.
- **Actual:** `info.Name()=="target"` no root → `filepath.SkipDir` em tudo.

### P1 — Paths do grafo são relativos ao root indexado, sem normalizar o cwd do repo

- **Repro:** `tb graph index --enable source` depois `tb graph impact --enable --file source/core/Ky.ts` → `(vazio)`; `--file core/Ky.ts` → hit.
- **Expected:** aceitar path do repo ou documentar/echo o root usado.
- **Actual:** silêncio/`(vazio)`.

### P2 — Indexador JS não captura métodos privados `#foo`

- **Repro:** buscar/`callers` de `#calculateDelay` / `calculateDelay` após indexar `source` → vazio (função existe em `Ky.ts`).
- **Expected:** pelo menos indexar o nome sem `#` ou documentar limitação.
- **Actual:** símbolo ausente; impacto/`callers` inúteis para API interna TS moderna.

### P2 — `tb review`/`drift` só aceitam `--output`, não `-o` (verify aceita `-o`)

- **Repro:** `tb review … -o out.json` → `unknown shorthand flag: 'o'`.
- **Expected:** flag curta consistente ou docs claras.
- **Actual:** inconsistência entre subcomandos.

### P3 — MCP `tools/list` não espelha verify/review/drift/guard/mem

- **Repro:** `tools/list` → só echo/env/run/graph_*/version.
- **Expected** (fase atual): ao menos tools alinhadas ao workflow de campo, ou docs dizendo o recorte.
- **Actual:** agente MCP não opera o loop verify/review via protocolo.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift real quando `rd.md` ficou fora do Scope.
- Guard em path hardcoded `.toolbelt/thresholds.yaml` e abort do verify nesse caso.
- Review sinalizou (corretamente) mudança fora de scope quando o ticket omitia `rd.md`.
- Graph deps de `core/Ky.ts` listou imports após indexação correta.
- Verify (tests) passou a suíte focada de jitter quando o código estava certo.

**Deixou passar / atrapalhou**

- Não leu `AGENTS.md` nem impôs a convenção `null`.
- Não indexou o repo no path padrão `/tmp/target` (bug SkipDirs).
- Não viu erro de build TypeScript se `test_cmd` ≠ `npm run build` / `npm test` completo.
- Hidden tests default (`go test ./tests/hidden/...`) irrelevantes para ky → `hidden_available=false` sem falhar o veredicto.
- Review marcou violação de `tests/hidden` e `thresholds.yaml` só porque o texto apareceu no diff do ticket (falso positivo).
- `guard.yaml` do projeto foi ignorado.
- Visual “passou” sem comparar pixels úteis (`skipped` por tela, `skipped=false` no report).
- Não empurrou fechamento da tarefa (segundo teste Infinity em `retry-decisions.ts` só apareceu no `ava` nativo completo).

## Interação com instrumentação existente / onboarding

- **`AGENTS.md`:** zero referências em código Go do tb; só mencionado em docs de pesquisa (`tb ctx` / pendente). tb **ignora** o arquivo.
- **Duplicação:** `tb mem init` cria invariantes genéricas (Go/`tests/hidden`/thresholds) que **não** refletem as regras do ky e ainda geram falsos positivos no review.
- **Conflito:** invariantes default assumem layout Go/toolbelt; ky é TS/ava/xo.
- **Onboarding:** `tb init` não detecta stack npm, não sugere `test_cmd` a partir de `package.json`, não importa `AGENTS.md`, não cria ticket/progress. Config verify default permanece `go test ./...`.
- **O que tb deveria importar:** conteúdo de `AGENTS.md` como invariantes/`mandate`; scripts `package.json` (`test`/`build`) como `verify.test_cmd` / passo de build; eventualmente `xo` como lint ruler.

## Sugestões de melhoria

1. **[P0][graph]** Não aplicar `SkipDirs` ao diretório raiz do walk; só a filhos.  
2. **[P0][verify]** Rodar (ou exigir) comando de build da stack detectada; não dar PASS só com testes parciais.  
3. **[P1][mem/onboarding]** Em `tb init`, detectar Node/Go/etc., preencher `verify.*` e importar `AGENTS.md` → invariantes.  
4. **[P1][guard]** Ler `.toolbelt/guard.yaml`; deixar de hardcodar só três paths.  
5. **[P1][review]** Matching de invariantes por path tocado, não por substring no diff/ticket.  
6. **[P1][docs/cli]** Sincronizar `docs/cli.md` com a árvore real; remover ou marcar “não implementado”.  
7. **[P1][bench]** Honrar `bench.default_agent`; em `compare`, aceitar `--agent` (não só `--fake`).  
8. **[P2][verify/visual]** Se qualquer screen skippar, `report.skipped=true` (ou veredicto SKIP distinto).  
9. **[P2][graph]** Normalizar `--file` ao root do repo; indexar `#private` methods.  
10. **[P2][workflow]** Comando mínimo de ticket/progress (mesmo que crie só `rd.md` + checklist) para empurrar completude.  
11. **[P3][config]** `0644` para yaml não-secretos (ou documentar 0600).  
12. **[P3][mcp]** Expor verify/review/drift/guard/mem no catálogo MCP.

## Métricas

| métrica | valor |
|---------|-------|
| Wall time (soma instrumentada de invocações no log) | ~66,8 s |
| Wall time só invocações `tb*` (aprox.) | ~40,7 s |
| Invocações registradas (tb + nativo + probes) | 70 |
| Invocações tb (aprox., excl. native/docs) | 61 |
| Falhas tb no log | 14 (quase todas probes intencionais: flags docs, graph off, drift/review bloqueantes, default_agent, thresholds) |
| Crashes do binário | 0 |
| Tempo estudo ponta-a-ponta (clone, npm, playwright, implementação, tb) | ~15–20 min de sessão (não só CLI) |

Arquivos brutos de evidência (VM): `/tmp/tb-evidence/`.
