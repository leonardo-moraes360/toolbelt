# Estudo de campo: excalidraw/excalidraw

Estudo realizado em 2026-11-15 com `tb` compilado de `toolbelt@c8aae04` (`make build` → `./bin/tb`). Clone local em `/tmp/target/excalidraw` (shallow). **Nenhuma** interação com o remoto do Excalidraw (sem push, issue, PR ou comentário). Evidências brutas em `/tmp/campo-evidence/` na VM do agente.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [excalidraw/excalidraw](https://github.com/excalidraw/excalidraw) |
| Linguagem | TypeScript / React (monorepo Yarn workspaces) |
| Tamanho | ~925 arquivos fonte relevantes; ~293k LOC aproximadas; checkout shallow ~93 MiB (com `node_modules` ~1.1 GiB) |
| Commit base do estudo | `82190fb` (`master` shallow) |
| Instrumentação AI existente | `AGENTS.md` (4 linhas: DOM/`ownerDocument`, preferir `Merge<>`); `CLAUDE.md` (estrutura do monorepo, comandos `yarn test:typecheck` / `yarn test:update` / `yarn fix`) |
| Toolchain | **Configurada**: Node 22, Yarn 1.22.22, `yarn install --frozen-lockfile` OK, Vitest 3.0.6, `tsc`, ESLint. Chrome headless presente (`/usr/local/bin/google-chrome`). |
| Não configurado / não rodado | App completa no browser (`yarn start` / Playwright E2E de UI); build de produção da app (`yarn build`) — fora do escopo da tarefa de math; suíte `yarn test:all` completa (typecheck+eslint+prettier+app) — typecheck e eslint nos arquivos tocados foram rodados; Playwright browsers não instalados além do Chrome do sistema. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `normalizeRadians` para retornar `0` (não `2π`) em múltiplos negativos de `2π` e ângulos negativos minúsculos; testes em `packages/math/tests/angle.test.ts`; impacto em `cartesian2Polar` / `radiansBetweenAngles`. |
| Por quê | Bug pequeno, isolado em `@excalidraw/math`, com repro claro e testes unitários — tamanho adequado a um campo study E2E. |
| Issue | https://github.com/excalidraw/excalidraw/issues/12599 |
| Completada? | **Sim, localmente** (commit local `16115e1` no clone; **não** enviado ao upstream). Correção + 6 testes passando. |
| Checks nativos | `yarn vitest run packages/math/tests` → 10 files / 64 tests PASS (~2.8 s). `yarn test:typecheck` no commit da tarefa → PASS (~13.6 s). ESLint nos arquivos tocados: 1 warning Prettier no primeiro draft do teste (`(-1e-17)`); após ajuste local → PASS. |

Trecho da correção aplicada em `packages/math/src/angle.ts`:

```ts
export const normalizeRadians = (angle: Radians): Radians => {
  const twoPi = 2 * Math.PI;
  const normalized = ((angle % twoPi) + twoPi) % twoPi;
  if (normalized === 0 || Math.abs(normalized - twoPi) < PRECISION) {
    return 0 as Radians;
  }
  return normalized as Radians;
};
```

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` (binário `make build`) | `toolbelt c8aae04`, commit/build preenchidos | neutro | build com ldflags |
| `go run ./cmd/tb version` | `0.1.0-dev`, `commit: unknown`, `built: unknown` | atrapalhou (versão inútil sem ldflags) | `23-version-norflags.txt` |
| `tb init` | criou `.toolbelt/` + `config.yaml` mode `600`; defaults Go (`verify.test_cmd: go test ./...`) | atrapalhou no onboarding JS (defaults errados para o repo) | wall 0.007 s; `stat` → `600 .toolbelt/config.yaml` |
| `tb mem init` / `add` / `list` / `check` | OK; `mem add <regra> --category`; `check` sha256 + retenção 1.00 | ajudou (invariantes rápidas) | `02`–`05`; `mem add --type/--title` (docs) → `unknown flag: --type` |
| Ticket / Scope (`rd.md` manual) | `tb ticket` **não existe**; Scope via frontmatter YAML em `rd.md` | atrapalhou (docs prometem `tb ticket`; fluxo real é arquivo manual) | `33-ticket-missing.txt`; `06-ticket.txt` |
| `TB_GRAPH=on tb graph rebuild --enable` | 4452 nós, 18465 arestas, ~1.05 s | ajudou | `07-graph-rebuild.txt` |
| `tb graph search "normalizeRadians"` | achou `func:...normalizeRadians` + seção do `rd.md` | ajudou | `08-graph-search.txt` |
| `tb graph impact --file packages/math/src/angle.ts` | listou `angle.ts` e `point.ts` | ajudou parcialmente (não listou consumidores em `packages/element`) | `09-graph-impact.txt` |
| `tb graph impact` sem `--file` | erro `--file é obrigatório` | atrapalhou | `09b-graph-impact-nofile.txt` |
| `tb graph callers --function normalizeRadians` | só devolveu o próprio arquivo | atrapalhou (falso negativo de callers cross-package) | `11-graph-callers.txt` |
| `tb graph deps --file ...` | imports `./types`, `./utils` | neutro | `10-graph-deps.txt` |
| `tb graph stats` | contagens OK | neutro | `12-graph-stats.txt` |
| `tb verify run` (vitest math) | `PASS` com `hidden_available: false` / `hidden_passed: false` | atrapalhou (PASS sem suíte oculta real; defaults Go) | `17-verify.txt` wall 2.9 s |
| `tb verify` + type error puro | `PASS` (vitest ignora erro de tipo); `tsc` falha | atrapalhou | `29g` PASS vs `29h` TS2322 |
| `tb verify --ruler visual` | sem manifesto: `skipped:true` PASS; com manifesto sem refs: **top-level `skipped:false`** e screen `skipped:true` | atrapalhou (flag top-level mentiroso) | `22`, `22c` |
| `tb review` | findings de scope + **falsos positivos** de invariantes `tests/hidden` e `thresholds.yaml` | atrapalhou | `19-review.txt` |
| `tb drift` | detectou `.toolbelt/*` e `rd.md` fora do scope; exit 1 | ajudou | `18-drift.txt` |
| `tb guard check` | sem violações no diff da tarefa; com paths hardcoded → violações; **exit 0** mesmo com violações (sem `--strict`) | neutro/atrapalhou | `20`, `21b` |
| `tb guard` + `.toolbelt/guard.yaml` | manifesto ignorado (path `angle.ts` “protegido” no yaml não gerou violação) | atrapalhou | `20b-guard-yaml.txt` |
| `tb bench run` sem `--agent`/`--fake` | erro obrigatório; **ignora** `bench.default_agent` do config | atrapalhou | `24-bench-default-agent.txt` |
| `tb bench run --fake` | no Excalidraw falha (sem `bench/tasks`); no toolbelt roda sem `--fake-config` | neutro | `25`, `25b` |
| `tb bench compare --agent ...` | `unknown flag: --agent` | atrapalhou | `26b` |
| `tb mcp` + `tools/list` | servidor stdio OK; tools graph/version/echo/env/run | ajudou (smoke) | `28-mcp-list.txt` |
| Checks nativos paralelos | vitest/tsc/eslint conforme acima | ajudou (baseline real) | `14`–`15`, `30`–`32` |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare --agent "echo hi"` → `unknown flag: --agent` (`26b`) |
| `guard.yaml` ignored | **confirmado** | `.toolbelt/guard.yaml` com `protected_paths: [packages/math/src/angle.ts]`; guard sem violação nesse path (`20b`) |
| outdated CLI docs | **confirmado** | `docs/cli.md` documenta `tb mem add --type/--title` e `tb ticket`/`tb board`; CLI real: `mem add <regra>` e sem `ticket`/`board` |
| version shows `unknown` | **confirmado** (sem ldflags) | `go run ./cmd/tb version` → commit/built `unknown`; `make build` preenche |
| config files created 0600 | **confirmado** | `config.yaml` e `invariants.yaml` mode `600` |
| visual ruler reports `skipped=false` when skipping | **confirmado** | com `screens.yaml` e ref ausente: top-level `"skipped": false`, screen `"skipped": true` (`22c`) |
| `default_agent` ignored | **confirmado** | config com `default_agent: echo default-agent-used`; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | diff com `.toolbelt/thresholds.yaml` / `tests/hidden/*` → verify `FAIL` iterations 0 (`21c`) |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake` no repo toolbelt completou 9 tasks sem `--fake-config` (`25b`) |
| `mem add` positional | **confirmado** | usage `tb mem add <regra>`; flags docs `--type` inexistentes |
| Drift/Scope need literal paths (no globs) | **parcialmente refutado** | `packages/math/**` e `packages/math/tests/*` funcionam em `inScope` (`18b`, `18c`); implementação aceita `/**` e `filepath.Match` |
| `graph impact` needs `--file` | **confirmado** | sem flag → erro obrigatório |
| review false positives on the tests/hidden invariant | **confirmado** | review critical em invariante `tests/hidden` / `thresholds.yaml` só porque o texto aparece no diff de `invariants.yaml` (`19`) |
| verify runs only partial checks and misses build errors | **confirmado** | type error `number = "not-a-number"`: vitest PASS + verify PASS; `tsc` falha (`29g`/`29h`). Defaults ainda são `go test`. |
| tb does not push the agent toward completing the whole task | **confirmado** (observacional) | init não sugeriu typecheck/eslint/prettier do CLAUDE.md; verify PASS sem hidden e sem `tsc`; commit local incluiu `.toolbelt/*` e `rd.md` fora do Scope sem o tb bloquear o “done”; agente precisou descobrir sozinho os checks nativos |

## Problemas novos

### P1 — `verify` dá `PASS` com `hidden_available: false`

- **Repro**: `tb verify run --test-cmd "yarn vitest run packages/math/tests" --hidden-test-cmd "yarn vitest run packages/math/src/angle.ts"` (ou default hidden Go em repo JS).
- **Esperado**: falhar ou marcar incompleto se a suíte oculta não rodar.
- **Atual**: `"verdict":"PASS","hidden_passed":false,"hidden_available":false` (`17-verify.txt`).

### P1 — `guard check` exit 0 com violações

- **Repro**: alterar `tests/hidden/*` ou `.toolbelt/thresholds.yaml`; `tb guard check base..head --json`.
- **Esperado**: exit ≠ 0 em `SeverityError` (ou exigir `--strict` documentado como default seguro).
- **Atual**: JSON com violações + exit 0 sem `--strict` (`21b`).

### P2 — `graph callers` / `impact` fracos em monorepo TS

- **Repro**: indexar Excalidraw; `tb graph callers --function normalizeRadians` / `impact --file packages/math/src/angle.ts`.
- **Esperado**: callers em `packages/element/src/resizeElements.ts` e `sizeHelpers.ts` (imports reais).
- **Atual**: callers só o arquivo da definição; impact só `point.ts` (`11`, `09`).

### P2 — `tb init` não adapta verify/bench ao ecossistema do repo

- **Repro**: `tb init` em monorepo Yarn/TS.
- **Esperado**: detectar package manager / scripts (`test`, `typecheck`) ou perguntar; ler `AGENTS.md`/`CLAUDE.md`.
- **Atual**: `verify.test_cmd: go test ./...`; zero menção a instrumentação existente.

### P3 — UX de erro do Cobra imprime Usage após JSON

- **Repro**: `tb drift --json` / `tb review --json` com drift.
- **Esperado**: só JSON + exit code em `--json`.
- **Atual**: JSON válido seguido de `Error:` + Usage (`18`, `19`).

### P3 — `tb help` lista `router` como comando apesar de “não implementado”

- Neutro para o estudo; gera expectativa falsa no onboarding.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift de escopo (`.toolbelt/config.yaml`, `.toolbelt/invariants.yaml`, `rd.md` fora do Scope).
- Paths hardcoded protegidos (`tests/hidden/*`, `.toolbelt/thresholds.yaml`) e abort do verify nesse caso.
- Localização da função via `graph search` / indexação rápida do monorepo.
- Smoke MCP (`tools/list`).
- Review reportou arquivos fora do scope (além dos FPs de invariante).

**Deixou passar**

- Erro de TypeScript puro quando `test_cmd` é só Vitest (verify PASS).
- Ausência de suíte oculta (`hidden_available: false` + PASS).
- Warning Prettier/ESLint no teste novo (verify não rodava eslint).
- Consumers reais de `normalizeRadians` fora de `packages/math`.
- Conteúdo de `AGENTS.md` / `CLAUDE.md` (não lidos no init/onboarding).
- Manifesto `.toolbelt/guard.yaml` customizado.
- Completude da tarefa além do `test_cmd` configurado (typecheck/lint/build da app).

## Interação com instrumentação existente / onboarding

- **`tb init`**: cria só `.toolbelt/`; **não lê** `AGENTS.md` nem `CLAUDE.md`. Docs internas (`achados.md` / arquitetura) listam “AGENTS.md como contexto local” como **pendente** (`tb ctx` ainda não existe na CLI).
- **Grafo**: indexa `AGENTS.md` como `file`/`section` (busca `"AGENTS"` encontra o arquivo), mas isso **não** injeta as regras no mandate/contexto do agente.
- **Duplicação / conflito**: invariantes default do `tb mem init` (`tests/hidden`, `thresholds.yaml`) são genéricas Go/tb e **não** espelham as regras do Excalidraw (`ownerDocument`, `Merge<>`, `yarn test:update`). Risco de ruído (já visto nos FPs do review) sem ganho local.
- **O que o tb deveria importar**: (1) comandos de verificação de `CLAUDE.md` / `package.json` scripts → `verify.test_cmd` + checklist (typecheck, eslint, prettier); (2) bullets de `AGENTS.md` → invariantes `mandate` imutáveis; (3) estrutura monorepo (workspaces) para defaults de graph/impact; (4) não sobrescrever nem ignorar silenciosamente instrumentação pré-existente.

## Sugestões de melhoria

1. **[P0 · verify]** Não emitir `PASS` se `hidden_available=false` (ou exigir `--allow-no-hidden`). Incluir profile `js/ts` com `test`+`typecheck`+`lint` a partir de `package.json`/`CLAUDE.md`. *(área: verify)*
2. **[P0 · init / mem]** Onboarding que importa `AGENTS.md`/`CLAUDE.md`/`CONTRIBUTING.md` para invariantes e `verify.*`. *(área: init, mem/ctx)*
3. **[P1 · guard]** Respeitar `.toolbelt/guard.yaml`; falhar por default em violações error; documentar `--strict`. *(área: guard)*
4. **[P1 · review]** Matching de invariantes por path tocado / AST, não substring no diff (elimina FP `tests/hidden`). *(área: review)*
5. **[P1 · docs/cli]** Sincronizar `docs/cli.md` e README com a CLI real (`mem add` posicional; ticket/board como não implementados; flags de bench compare). *(área: docs)*
6. **[P1 · visual]** Top-level `skipped=true` se todas as screens skipparem; não reportar PASS “limpo” com `skipped:false`. *(área: verify/visual)*
7. **[P2 · bench]** Honrar `bench.default_agent`; aceitar `--agent` em `bench compare` (ou documentar só `--fake`/JSONL). *(área: bench)*
8. **[P2 · graph]** Melhorar resolução de imports TS path-aliases / workspaces para callers/impact cross-package. *(área: graph)*
9. **[P2 · workflow]** Checklist de “done” (Scope limpo + verify + typecheck/lint nativos + drift clean) para empurrar completude da tarefa. *(área: router/verify — alinhado ao achado A/B de completude)*
10. **[P3 · ux]** Em `--json`, não imprimir Usage do Cobra; permissões de config menos agressivas que `0600` quando o vault não tem secrets. *(área: cli/config)*

## Métricas

| métrica | valor |
|---------|-------|
| Wall time somado dos comandos cronometrados (tb + checks nativos no log) | ~49 s |
| Invocações tb (aprox., incluindo probes de issues conhecidas) | ~45–55 |
| Falhas / exits ≠ 0 do tb observados | drift/review com drift; guard+verify em paths protegidos; bench sem agent; compare `--agent`; impact/deps/callers mal invocados; visual sem skip; `mem add` flags docs; `ticket` inexistente |
| Falhas “de produto” vs uso | maioria esperada pelos bugs conhecidos; graph callers incompleto e verify PASS sem hidden/typecheck são os mais graves para agente |
| Graph rebuild | ~1.05 s / 4452 nós / 18465 arestas |
| Verify (vitest math, tarefa OK) | ~2.9 s, PASS |
| Vitest math nativo | ~2.8 s, 64 tests PASS |
| `tsc` (tarefa OK) | ~13.6 s, PASS |
| MCP `tools/list` | ~0.007 s, OK |
| Bench fake (no repo toolbelt, 9 tasks × 1) | ~4.0 s, sucesso 33.3% (baseline do fake, não do Excalidraw) |

**Nota metodológica:** a “completude” da tarefa foi atingida pelo agente consultando a instrumentação nativa do Excalidraw; o tb sozinho (init → verify com `test_cmd` Vitest) teria declarado PASS mesmo com typecheck quebrado e sem lint — alinhado ao problema conhecido de não empurrar o agente a fechar a tarefa inteira.
