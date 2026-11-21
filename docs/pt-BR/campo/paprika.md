[🇺🇸 English](../../campo/paprika.md) | 🇧🇷 Português

# Estudo de campo: Paprika (monorepo TypeScript/pnpm do zero)

Estudo feito em 2026-11-21 com o `tb` compilado deste repositório (`make build` → `toolbelt 4ed7d1f`, go1.27.4, linux/amd64). O alvo foi um repositório **novo**, criado do zero durante o estudo: o Paprika, uma ferramenta de design local com servidor MCP embutido. O `tb` foi usado como harness do início ao fim (bootstrap, invariantes, grafo, verify, guard, review, drift, stack). Nada foi enviado ao upstream do toolbelt.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | Paprika (local, sem remoto) |
| Linguagem | TypeScript 5.9, Node 22, React 19, Vite 8, Vitest 4, Playwright 1.63 |
| Estrutura | Monorepo pnpm: `packages/core`, `packages/server`, `apps/web` (49 arquivos versionados) |
| Commits avaliados | `d5e5af7..` 5 commits (base = primeiro commit) |
| Instrumentação de IA prévia | Nenhuma: `AGENTS.md` semeado pelo `tb init --write-agents` e depois reescrito |
| Toolchain | Sim, completa: build, typecheck, 25 testes unitários/integração e 6 E2E no Chromium |

## Tarefa

| Campo | Valor |
|-------|-------|
| O quê | Implementar o Paprika v0.1: modelo de nós com zod, transações atômicas com undo/redo, MCP (stdio e HTTP, 16 ferramentas), canvas React ao vivo via WebSocket, lint de contraste WCAG, screenshot e export React |
| Concluída? | Sim. `tb stack verify guard review drift --base d5e5af7 --ticket rd.md` → `ok: true`, verify PASS na 1ª iteração (15,4 s) |

## Uso do tb por comando

| Comando | Resultado | Ajudou / atrapalhou / neutro | Evidência |
|---------|-----------|------------------------------|-----------|
| `tb init --write-agents` | OK em 0,02 s; detectou `pnpm test` e `pnpm run build` pelo lockfile | ajudou | Sem defaults de Go (o problema do estudo do python-sdk não se repetiu). `config.yaml` agora é 0644 (dogfood #7 corrigido). |
| `tb init` (config gerada) | Bloco `egm` de ~40 linhas e diretórios vazios `.toolbelt/bench` e `.toolbelt/vault` num repo sem specs nem bench | neutro | Ruído de leitura para quem abre a config pela primeira vez. `graph.languages` não lista `typescript` (fica dentro de `javascript`), o que confunde. |
| `tb init` (AGENTS.md semeado) | Texto genérico ("may use toolbelt") que fica desatualizado quando `verify.*` muda | neutro | Mudei `test_cmd` e `lint_cmd` e o AGENTS.md seguiu citando os valores antigos. |
| `tb mem init` | 3 invariantes padrão | atrapalhou (leve) | Semeia "NEVER change thresholds in `.toolbelt/thresholds.yaml`", mas o arquivo não existe e o `init` não o cria. |
| `tb mem add` (×5) + `mem check` | OK; sha256 e retention 1.00 | ajudou | Saída clara. Faltou uma categoria para regras de arquitetura (só existem `anti_cheat`, `mandate` e `security`); usei `mandate`. |
| `tb verify run --json` (repo com 1 commit) | Erro: `base: ref missing or invalid "HEAD~1"` | atrapalhou | A dica sugere `git fetch origin HEAD~1^{commit}`, mas o repo não tem remoto. Num repo novo, o primeiro `verify` sempre falha. |
| `tb verify run --json` (com Playwright) | ESCALATE após 3 iterações (13 s); o mesmo teste passa no shell | atrapalhou (alto) | O allowlist de env removeu `PLAYWRIGHT_BROWSERS_PATH` e o Chromium não foi encontrado. Nada na saída diz que variáveis foram filtradas. Resolvi com `env.allow: [PLAYWRIGHT_BROWSERS_PATH, PAPRIKA_CHROMIUM]`. |
| `tb verify run --json` (saída de falha) | `reason` com códigos ANSI (`\u001b[31m…`) e truncado a partir do início | atrapalhou | As linhas `FAIL` só aparecem no log DEBUG (dogfood #10 continua aberto). A falha determinística rodou 3 vezes (dogfood #3 continua aberto). |
| `tb verify run` (após o ajuste de env) | PASS, iter=1; build + lint (`pnpm typecheck`) + testes + E2E em 15,6 s | ajudou | `test_cmd: pnpm test && pnpm test:e2e` funciona (o comando roda via shell). |
| `TB_GRAPH=on tb graph index` | 236 nós e 506 arestas em 0,07 s | ajudou | Rápido. Indexou `.tsx` e componentes React. |
| `tb graph search applyOps` | 1º resultado correto | ajudou | Os resultados seguintes são outras funções do mesmo arquivo (ruído aceitável). |
| `tb graph callers --function nodeStyle` | Só `render.ts`; **não encontra** `apps/web/src/Canvas.tsx` | atrapalhou | O import de workspace `@paprika/core` vira o nó externo `pkg:@paprika/core`. Em monorepo pnpm/TS, `callers` e `impact` não atravessam pacotes. Era justamente o invariante "nodeStyle é a única fonte de renderização" que eu queria checar. |
| `tb graph deps --file apps/web/src/Canvas.tsx` | Lista `@paprika/core` e `react` | neutro | Import relativo `./Canvas` (de `App.tsx`) aparece como `pkg:./Canvas`. |
| `tb guard` (via stack) | 4 regras, 0 violações, 57 ms | neutro | Os 5 invariantes `mandate`/`security` que adicionei são só texto: guard e review não os verificam. |
| `tb review --ticket rd.md` (via stack) | 5 warnings, 22 ms | neutro | 4 de `quality-large-func` em componentes React (App 142, Inspector 138, Canvas 262 e o hook 80 linhas). O do Canvas é legítimo; os outros são o formato normal de componente. 1 de `accept-3`: o critério "build, typecheck, testes e E2E passam" sai "sem evidência", embora o `verify` do **mesmo stack** tenha dado PASS. Os findings não trazem linha. |
| `tb drift --ticket rd.md` (via stack) | OK; sem itens fora de escopo; base confere | ajudou | Frontmatter YAML com globs `packages/**` funcionou. |
| `tb stack verify guard review drift` | `ok: true` em 15,4 s; um JSON agregado | ajudou | Bom formato de "gate final" para o agente. |
| `tb mcp` | Não exercitado | — | Usei a CLI. As ferramentas `tb_*` via MCP não foram testadas neste estudo. |

## Problemas novos

### P1: o filtro de env quebra o Playwright em silêncio
`verify` remove `PLAYWRIGHT_BROWSERS_PATH`, então a suíte E2E falha só dentro do `tb`. Proposta: (a) o `tb init` detecta `playwright`/`@playwright/test` no `package.json` e já preenche `env.allow` com `PLAYWRIGHT_*`; (b) em FAIL, o JSON inclui `filtered_env: [...]` com os nomes removidos (sem valores).

### P1: o `verify` não funciona num repo com um único commit
O default `--base HEAD~1` falha e a dica manda dar `git fetch origin`. Proposta: sem `HEAD~1`, usar a árvore vazia (`4b825dc…`) como base, ou analisar a working tree; trocar a dica quando não há remoto.

### P2: grafo cego entre pacotes de workspace
Imports de pacotes declarados em `pnpm-workspace.yaml`, `workspaces` do npm/yarn ou `paths` do tsconfig deveriam resolver para os arquivos do pacote local (via `exports`/`main` do `package.json`), e não virar `pkg:` externo. Imports relativos (`./X`) não deveriam virar nós `pkg:`.

### P2: a saída de falha do `verify` não serve bem para agentes
Remover ANSI (ou rodar o filho com `NO_COLOR=1 FORCE_COLOR=0`), priorizar as linhas com `FAIL`/`Error`/`✗` no trecho do `reason` e não repetir uma suíte determinística sem diff entre as iterações.

### P3: o review não aproveita o resultado do verify do mesmo stack
Critérios de aceite do tipo "testes/build passam" deveriam contar com o `verdict: PASS` do `verify` na mesma execução do `tb stack`.

### P3: `quality-large-func` em componentes React
Para `.tsx`/`.jsx`, medir o tamanho do JSX separado da lógica, ou usar um limite próprio para componentes. Incluir a linha no finding.

### P3: invariante semeado aponta para um arquivo inexistente
`mem init` cita `.toolbelt/thresholds.yaml`, que nada cria. Sugestão: semear só quando o arquivo existir, ou criar o arquivo junto.

## O que o tb pegou e o que deixou passar

- **Pegou:** o problema de ambiente do Playwright (indiretamente, pela divergência entre o shell e o `tb verify`) e o escopo do trabalho (o drift ficou limpo, com evidência).
- **Deixou passar:** dois bugs reais de UI que só o E2E pegou (o `blur` regravava o campo depois do Enter e duplicava a entrada de histórico; a seleção de um nó recém-criado era limpa antes de o servidor confirmar). E a injeção de CSS via `fontFamily`, achada em revisão manual. Nenhum desses é papel do `tb`, mas o `review` não sinalizou nada de segurança em `style` inline.

## Métricas

| Métrica | Valor |
|---------|-------|
| `tb init` | 0,02 s |
| `tb graph index` | 0,07 s (236 nós / 506 arestas) |
| `tb verify run` (PASS) | 15,6 s, 1 iteração |
| `tb verify run` (falha de env) | 13,5 s desperdiçados em 3 iterações idênticas |
| `tb stack verify guard review drift` | 15,4 s no total; guard 57 ms, review 22 ms, drift 12 ms |
| Invariantes | 8 (3 semeados + 5 do projeto), retention 1.00 |

## Status

Os sete problemas acima foram corrigidos em [#152](https://github.com/leonardo-moraes360/toolbelt/pull/152) (merge em 2026-11-21). Duas diferenças em relação às propostas: o `tb init` não preenche `env.allow`; em vez disso, `PLAYWRIGHT_BROWSERS_PATH`, `PUPPETEER_CACHE_DIR` e afins entram na allowlist padrão pelo nome exato. E o fallback do `--base` vale só para um repositório com um único commit que não seja clone raso; ali a base passa a ser o próprio `HEAD` (diff vazio), não a árvore vazia.
