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

## Adendo 2026-11-21: servidor em Go com SQLite (`server-go/`)

Segunda tarefa no mesmo repo, agora com Go ao lado do TypeScript. `tb` v0.0.9 instalado com `go install …/cmd/tb@latest` (o binário reporta commit `unknown`). `test_cmd` passou a rodar também `go test`, a suíte de paridade e o E2E contra o binário Go.

| Comando | Resultado |
|---------|-----------|
| `tb verify run` | PASS em 1 iteração depois de uma correção (abaixo); build + lint + testes TS, Go, paridade e E2E |
| `tb stack verify guard review drift --base origin/main --ticket rd.md` | ok; guard 4 regras sem violação, drift limpo (0 fora do escopo, 0 proibidos), review com 9 `quality-large-func` |
| `tb mem add` | Funcionou, mas só depois de criar uma ADR (ver P2) |

### P1: `go build` falha dentro do `verify` por causa do carimbo de VCS
No ambiente filtrado do `verify`, o `git` não consegue informar o status e o `go build` falha com `error obtaining VCS status: exit status 128 … Use -buildvcs=false`. No shell normal o mesmo comando passa. Contornei com `go build -buildvcs=false`. Proposta: o `tb init` sugerir `-buildvcs=false` quando detectar `go.mod`, ou o filtro manter as variáveis de que o git precisa (`HOME`, `GIT_*` sem segredo); e o `reason` mostrar o fim da saída (a linha do erro estava cortada, só apareceu com `-v`).

### P2: `--ref` do `tb mem add` só aceita `docs/adr` ou `docs/decisions`
Tentei referenciar `server-go/README.md` e foi recusado. Bom para forçar decisão registrada, mas a mensagem poderia sugerir criar a ADR.

### P2: `tb` não está no PATH em sessões na nuvem
Precisei de `go install` e `PATH=$PATH:~/go/bin`. O `go install` gera versão sem commit (`unknown`), o que dificulta reportar bug com a versão exata. Proposta: `-ldflags` via `debug.ReadBuildInfo` (o `go install` preenche a versão do módulo).

### P3: `review`/`drift` exigem commits
Com tudo na working tree, não dá para revisar antes de commitar. Proposta: `--worktree` comparando a árvore atual com a base.

### P3: `quality-large-func` em tabelas declarativas
Os 9 avisos são definições de schema e registro de rotas/ferramentas (listas longas, sem ramificação). Proposta: medir complexidade (ramificações) além de linhas, ou ignorar corpos que são um único literal composto.

### Métricas
| Métrica | Valor |
|---------|-------|
| `tb verify run` dentro do `tb stack` (PASS) | 32,8 s (build TS + Go, typecheck, vet, testes TS, E2E TS, go test, paridade e E2E contra o Go) |
| `tb stack …` | 33 s no total; guard 136 ms, review 173 ms, drift 32 ms |

## Adendo 2026-11-21: thread "canvas em Rust/WASM" (container na nuvem)

| Situação | Resultado | Ajudou / atrapalhou | Proposta |
|----------|-----------|---------------------|----------|
| `tb` no container da nuvem | Não instalado; o clone do toolbelt fica fora do escopo de repositórios da sessão, então `tb mem list` (regra 1 do AGENTS.md), `verify` e `stack` não rodaram | atrapalhou | Publicar binários em releases (`go install …@latest` ou tarball) e documentar no AGENTS.md um fallback: ler `.toolbelt/invariants.yaml` direto quando `tb` não existir. Foi o que fiz; o arquivo YAML bastou para checar os invariantes |
| Invariantes como texto | Li `invariants.yaml` à mão e achei um conflito real: "nodeStyle é a única fonte de renderização" contra um segundo renderizador (WASM). Registrado no ADR 0001 | ajudou | Que `tb review` sinalize quando um diff cria um caminho paralelo ao citado num invariante `mandate` (ex.: novo arquivo que desenha nós sem importar `nodeStyle`) |
| Projeto poliglota (Rust em `experiments/`) | `verify.test_cmd` só cobre pnpm; `cargo test` ficou fora do gate | neutro | `tb init` detectar `Cargo.toml` em subpastas e sugerir `test_cmd` composto, ou `verify` aceitar uma lista de suites por diretório |

## Adendo 2026-11-21: thread "CI do Paprika" (container na nuvem, PR #5)

`tb` compilado do clone público (`go build -buildvcs=false ./cmd/tb`, em ~1 min); `tb mem list` e `tb verify run` rodaram (PASS em 1 iteração, 32,5 s).

| Situação | Resultado | Ajudou / atrapalhou | Proposta |
|----------|-----------|---------------------|----------|
| Gate local × gate de CI | `verify` passou, mas o CI pegou 2 falhas que o gate local não tinha como ver: clippy do Rust stable mais novo (1.100, lint `chunks_exact_to_as_chunks`) e o `govulncheck` | atrapalhou | `verify` aceitar suites por diretório (Rust e Go lint) e avisar quando a toolchain local estiver atrás da do CI; um `tb ci plan` que leia o mesmo mapa de áreas do CI evitaria duas fontes de verdade |
| `lint_cmd` sem linter Go real | `lint_cmd` só tem `go vet`; o golangci-lint achou 40 itens (2 de código morto, 1 atribuição inútil) | neutro | `tb init` sugerir `golangci-lint run` quando houver `go.mod` e o binário existir |
| Seleção por mudança | Escrevi `ci-plan.mjs` e `go-affected.sh` à mão; o grafo do `tb` (`graph`) já conhece imports Go | neutro | `tb graph affected --base <sha>` devolvendo pacotes/arquivos de teste afetados serviria ao CI e ao `verify` (rodar só o que mudou) |
| Saída do `verify` | Só mostra o veredito; sem tempo por etapa | neutro | Mostrar duração de build/lint/test para decidir o que paralelizar |

## Adendo 2026-11-21: migração para o toolbelt v0.0.10 (máquina do usuário, PR #11)

`tb` em `~/go/bin/tb`, `toolbelt 0.0.10`, commit f376165 (go1.27.4); agora o binário informa o commit, o que resolve o P2 de versão `unknown`. Bootstrap feito num worktree a partir de `origin/main` (dae178d) com `tb init --json`. Resultado: `agent_docs.yaml` com hints novos (`pnpm check`, `pnpm test:render`); `config.yaml` intocado pelo `init`, chaves novas `egm.drift` e `egm.bootstrap` adicionadas à mão; `verify.*`, `env.allow` e as 9 invariantes iguais (`tb mem check` retention 1.00). `tb stack verify guard`: ok, verify PASS em 1 iteração (19,5 s), guard 48 ms.

| Comando | Esperado | Obtido | Severidade |
|---------|----------|--------|------------|
| `tb bootstrap` / `tb help bootstrap` | Um comando de bootstrap ou migração para a v10 | Não existe ("Unknown help topic"); o bootstrap é `tb init`. "bootstrap" na doc é só a fase B2 do egm | média |
| `tb init` com `config.yaml` existente | Migrar ou acrescentar as chaves novas, ou pelo menos avisar quais faltam | No-op silencioso; achei `egm.drift`/`egm.bootstrap` comparando com um `config.yaml` gerado num repo vazio | média |
| `tb init --dry-run` | Ver o plano antes de escrever | Não há dry-run | baixa |
| `tb init` em `agent_docs.yaml` | "never overwrites" (help) | Reescreveu os hints (conteúdo correto, mas contradiz o help) | baixa |
| `tb init` em worktree | Ignore local só do worktree, ou aviso | Escreve em `.git/info/exclude` do common dir, afetando também o checkout principal, sem avisar | baixa |
| `tb verify` com falha determinística | ESCALATE na 2ª repetição da mesma falha nomeada (CHANGELOG v0.0.10) | 3 iterações idênticas (~25 s); o atrito das métricas da 1ª tarefa continua | média |
| `reason` do `verify` | Excerpt com a linha decisiva (CHANGELOG v0.0.10) | Cortou `Executable doesn't exist at …chromium_headless_shell-1243`, que só aparece no log DEBUG | média |
| `tb doctor` | Checar o ambiente (Chromium do Playwright, binários do `test_cmd`) antes do `verify` | Não existe; só se descobre pela falha do `verify` | baixa |

Resolvido na v10, em relação aos adendos anteriores: `PLAYWRIGHT_BROWSERS_PATH`/`PUPPETEER_*` no allowlist padrão (P1 do filtro de env), `filtered_env` no resultado não-PASS, fallback de `--base` em repo de 1 commit, grafo resolve pacotes de workspace pnpm, `tb mem init` não semeia mais a invariante de `thresholds.yaml` (a do Paprika continua porque foi semeada antes; não removi, é `immutable`).

Proposta principal: `tb init --migrate` (ou `tb config migrate`) que faça merge das chaves padrão ausentes, com `--dry-run` mostrando o diff, e um aviso no `tb init` quando o `config.yaml` existente estiver atrás da versão do binário (ex.: campo `version:` no config).

## Adendo 2026-11-21: bus local multi-vendor (máquina do usuário, PR #12)

`tb bus` (toolbelt 0.0.10, commit f376165c) no checkout do Paprika. Smoke test num canal descartável passou de ponta a ponta (join, assign, recv, claim, segundo claim recusado nomeando o dono, complete com epoch, log, show); `tb mcp` lista 54 ferramentas, as `tb_bus_*` incluídas. Canal real `paprika` criado com `--lease 5m`, supervisor `agent:leonardo`, invite de worker de 24h para o agente externo. Nenhum arquivo versionado precisou mudar: `.toolbelt/bus/` já estava no `.gitignore` e no `.git/info/exclude`.

| Comando | Esperado | Obtido | Severidade |
|---------|----------|--------|------------|
| `tb bus invite --ttl 24h` | Validade com data completa | `expires 16:28:07Z` sem data: parece que vence agora; a data real só lendo o `bus.db` | média |
| Saídas de horário | Um fuso só | `channel extend` e o log do `tb mcp` em hora local (`-03:00`); create, invite, list e show em `Z` | baixa |
| `channel create --ttl 15m` | Owner invite com a mesma validade, ou aviso | Owner invite com 10m (padrão de `--invite-ttl`), sem dizer que é outro valor | baixa |
| Lease do canal | Escolher/alterar depois | Padrão 2m só aparece após o create; não há comando para mudar o lease de um canal existente | baixa |
| `channel list` com canal expirado | Marcar `expired` | Lista igual aos ativos; só comparando a data | baixa |
| Encerrar canal | `channel close`/`delete` | Não existe; usei `channel extend --ttl 1s`, contraintuitivo | média |
| Invites pendentes | `invite list` / `invite revoke` | Não existem; um invite de 24h vazado não tem revogação além de esperar | média |
| `tb bus discover` com Codex instalado | Ler `~/.codex/config.toml` | `exists (not parsed)`; não diz se o `tb mcp` está configurado lá | baixa |
| Threads de agente na nuvem | Entrar no mesmo bus | Impossível: o `bus.db` é local e as threads Claude na nuvem não alcançam a máquina; a coordenação entre nuvem e bus fica só por convenção (raias de caminhos) | média (limite de desenho, não bug) |
| Divisão de trabalho | Claim protegendo arquivos | Claim protege a *thread*, não caminhos; tive de inventar `paths` no payload e checagem manual pelo supervisor | média |

Propostas: imprimir `expires` em RFC 3339 completo e sempre em UTC; `tb bus invite list|revoke`; `tb bus channel close`; `channel list` com coluna `state`; `discover` parsear TOML do Codex; um campo opcional `paths` no `claim` (ou `tb bus lock <glob>`) que recuse sobreposição com claims abertos, o que tornaria a divisão por raias verificável em vez de convenção. Para agentes na nuvem, um modo de transporte opcional (ex.: espelhar o board num arquivo versionado ou numa branch `bus/board`) permitiria que threads remotas ao menos lessem os claims.

## Adendo 2026-11-21: thread "primeira leva do backlog", item 1 (container na nuvem)

`tb guard check 1a5e5c0..HEAD` e `tb verify run --base 1a5e5c0 --head HEAD` (toolbelt v0.0.10) na branch do item 1 (grupos, páginas, seleção múltipla). Ambos passaram: guard 4 regras/0 violações, verify PASS em 1 iteração (build, lint e testes visíveis).

| Comando | Esperado | Obtido | Severidade |
|---------|----------|--------|------------|
| `tb guard check` numa branch com 2+ commits | Base = merge-base com a branch padrão | Padrão `HEAD~1`: checaria só o último commit; precisei passar o intervalo à mão | média |
| `tb verify run` com testes pulados | Contar e mostrar `skipped` (ou falhar com `--no-skip`) | PASS sem dizer que 4 E2E do WASM tinham sido pulados (faltava `wasm32-unknown-unknown`); só descobri lendo o log do vitest | média |
| Resumo do `verify` | Quantos testes rodaram por comando | Só `Visible: true`; nada de contagem | baixa |
| Raias de caminhos (bus, PR #12) | Ver a raia de outra thread antes de editar | A raia está só em `docs/agent-bus.md`, sem checagem; o aviso de conflito chegou por mensagem entre threads | baixa (ver adendo do bus) |

Propostas: `guard check` com base padrão = `merge-base origin/HEAD`; `verify` extrair `skipped` do output de vitest/go test (como já extrai falhas) e mostrar no veredito.

### Handshake com o primeiro agente externo (`agent:iq3`, 2026-11-21 16:42Z)

| Comando | Esperado | Obtido | Severidade |
|---------|----------|--------|------------|
| `tb bus log paprika --limit 30` antes da query | Mostrar as mensagens seq 1–2 (ou dizer por que estão ocultas) | "no messages in the channel", mas a query seguinte recebeu seq=3; depois o log mostrou só a #3 | média |
| Agente MCP ocioso | Saber se ele vai acordar | iq3 ficou `alive=false` 5 min após o último heartbeat e não leu a query; nada no `channel show` diz o `wake` do Agent Card (manual/notify/harness), então o supervisor não sabe se precisa cutucar o humano | média |
| Card do agente | Ver vendor/modelo/capacidades de quem entrou | `channel show` mostra só nome, role e alive; não há `tb bus agent show <nome>` | baixa |
| Agent Card × wake file | Corrigir o `wake` do card sem trocar o token | `--check` avisa `wake_mismatch (card says manual, wake file says notify)`; corrigir exige novo invite + re-join (troca token, reinicia harness). Proposta: `tb bus card update` para campos informativos | baixa |
| Agente lendo a doc do tb | Exemplos de `send` coerentes | O handoff do iq3 usou `send --json '{…}'` como payload; na v10 `--json` é formato de saída e o payload é `--payload`. Sinal de que a flag confunde agentes | baixa |

### Item 3 do backlog (colar SVG/HTML, export SVG), 2026-11-21

`tb guard check 63a90b0..HEAD` (4 regras, 0 violações) e `tb verify run --base 63a90b0 --head HEAD` (PASS em 1 iteração) na branch empilhada do item 3.

| Comando | Esperado | Obtido | Severidade |
|---------|----------|--------|------------|
| `tb guard --base <branch>` / `tb verify --base` | Aceitar a forma curta, ou sugerir o subcomando | "unknown flag: --base" e o help genérico; a flag só existe em `guard check`/`verify run`. Proposta: "did you mean `tb verify run --base`?" | baixa |
| `verify` com branches empilhadas | Base = a branch de baixo da pilha | Precisei passar o SHA do commit do item 2 à mão de novo (mesmo atrito do item 1) | média |
| `verify` × paridade/E2E do servidor Go | Rodar também os suítes que dependem de binário externo | `test_cmd` cobre só os testes TS visíveis; paridade e E2E contra o Go rodaram fora do tb (`pnpm test:parity:go`, `test:e2e:go`) e o veredito não diz isso | baixa |
| `tb bus send` fora da raiz do repo | Achar o bus pelo canal ou avisar que não há `.toolbelt/` no cwd | De /tmp: `no session for "leonardo" on channel "paprika": run tb bus join first` (o bus é resolvido pelo cwd; a mensagem sugere um join que trocaria a sessão). Proposta: dizer qual raiz foi usada e sugerir `--root`/cd | média |

## Adendo 2026-11-21: `tb` garantido nas sessões da nuvem (hook SessionStart, container efêmero)

Objetivo: threads na nuvem pulavam `tb verify` porque o container nasce sem `tb`. Solução no Paprika: hook `SessionStart` versionado (`.claude/hooks/session-start.sh`) que roda `go install github.com/leonardo-moraes360/toolbelt/cmd/tb@v0.0.10` com sumdb forçado e confere o hash do módulo no binário (`go version -m`, `h1:q1ojb4SU…`). Container com go1.24.7; primeira instalação ~28–34 s (inclui baixar a toolchain go1.27.4), execuções seguintes ~20 ms. Depois: `tb mem list` (9 invariantes, igual ao repo), `tb stack verify guard` ok: verify PASS em 1 iteração (22,6 s) e guard 4 regras/0 violações.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Instalação em ambiente efêmero | Binário de release com checksum (`SHA256SUMS`) ou script de instalação fixável | Único caminho é `go install`: exige Go no container, baixa a toolchain e compila (~30 s por container novo). A API de releases do GitHub nem é alcançável pelo proxy da sessão; `proxy.golang.org`/`sum.golang.org` são | média |
| `tb version` após `go install …@v0.0.10` | `toolbelt 0.0.10` + commit | `toolbelt dev`, `commit: unknown`, `built: unknown`. O P2 só foi resolvido para `make build`; quem instala pelo caminho documentado volta a não saber a versão. Proposta: fallback para `debug.ReadBuildInfo().Main.Version` (e `vcs.revision` quando houver) | média |
| Checar programaticamente "é o tb fixado?" | `tb version --json` com versão e hash do módulo | Tive de usar `go version -m` e `awk` na linha `mod`; sem Go no PATH não haveria como checar | baixa |
| `go 1.27.4` no `go.mod` do toolbelt | Mínimo por minor (`go 1.27`) | Patch exato força download de toolchain em qualquer Go < 1.27.4 (o container tinha 1.24.7); funciona com `GOTOOLCHAIN=auto`, mas quebra onde `GOTOOLCHAIN=local` | baixa |
| Bootstrap num clone novo | Não precisar de `tb init` | Ajudou: `config.yaml`, `invariants.yaml` e `agent_docs.yaml` versionados bastaram; `tb mem list` e `stack` rodaram sem init nem estado em `$XDG_STATE_HOME` | ajudou |
| `env.allow` no container | Repassar o Chromium da imagem | Ajudou: `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium` (escrito pelo hook no `CLAUDE_ENV_FILE`) chegou ao E2E sem mexer no config | ajudou |
| Saída do `tb stack` | Resumo curto para agentes | JSON único grande (~centenas de linhas); para ler veredito e iterações precisei de script. Proposta: `--summary` com uma linha por etapa (`verify PASS 1 it 22.6s · guard 0/4`) | baixa |
| Recomendação de integração com harness | Doc do toolbelt sobre agentes na nuvem | Não há; cada repo reinventa o hook. Proposta: `tb init --claude-hook` (ou exemplo em docs) gerando o `SessionStart` com versão e hash fixados | média |

Propostas principais: publicar binários com `SHA256SUMS` assinado nas releases, corrigir a versão de builds `go install`, `tb version --json`, e um gerador oficial do hook de sessão.

## Adendo 2026-11-21: itens 4 e 5 do backlog (edição de path, modos de tema), container na nuvem

`tb guard check <item4>..HEAD` (4 regras, 0 violações) e `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium tb verify run --base <item4> --head HEAD` (PASS em 1 iteração: build, lint, testes visíveis; hidden n/a) na branch do item 5 (PR #26). Antes do PR, a `main` (landing page + CI) foi trazida por merge para as 5 branches da pilha; checks rodados em cada uma.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `verify` com Playwright apontando para um Chromium que não existe | Detectar o navegador ausente antes de rodar e dizer como apontar outro | O Playwright da imagem pede `chromium_headless_shell-1255`, que não está instalado; sem `PAPRIKA_CHROMIUM` os testes de screenshot falham com "Executable doesn't exist" no meio da suíte. Proposta: um pré-check configurável (`requires: [chromium]`) com a mensagem de correção | média |
| `guard` × skip condicional | Não contar `it.skipIf(!chromium)` como teste pulado | O `test_skip` avisa no teste Go que faz `t.Skip` quando não há Chromium (skip legítimo de ambiente). Proposta: permitir anotar o skip (`// tb:env-skip`) ou distinguir skip incondicional de condicional | baixa |
| Veredito FAIL do `verify` | Ver qual etapa falhou e o trecho do erro | A saída final mostra `Build/Lint/Visible: true/false`, mas não o comando nem as últimas linhas do erro; precisei rodar a etapa de novo fora do tb. Proposta: imprimir o comando e as ~20 últimas linhas da etapa que falhou | média |
| `verify` numa pilha após merge da `main` | Base = branch de baixo, só o diff do item | Funcionou passando o nome da branch (`--base claude/...-4-edicao-path`) em vez de SHA; ajudou. O atrito do item 1/3 (SHA à mão) some quando a branch de baixo já está no remoto | ajudou |

## Adendo 2026-11-21: item 6 do backlog (camada de agentes), container na nuvem

`tb guard check origin/<item4>..HEAD` (4 regras, 0 violações) e `tb verify run --base origin/<item4> --head HEAD` (PASS em 1 iteração) na branch do item 6 (PR #28). A mudança tocou 56 arquivos (tradução das mensagens no core TS e no port Go).

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Mudança grande e mecânica (traduzir ~150 mensagens em TS e Go) | O `guard` apontar strings divergentes entre as implementações | Quem pegou as divergências foram os goldens (`go test ./internal/core`), não o tb; o `guard` só olha as regras de diff. Proposta: uma regra opcional "paridade de strings" alimentada por um mapa declarado no repo | baixa |
| `verify` × testes que dependem de idioma | Saber que o veredito cobriu paridade e E2E contra o Go | PASS, mas a saída não lista as suítes rodadas; conferi rodando `test:parity:go` e `test:e2e:go` à parte (mesmo atrito do item 3) | média |
| PRs empilhadas mergeadas na base errada | Algum aviso de que a pilha não chegou na `main` | O tb não sabe da pilha; descobri pelo GitHub que #17/#20/#22/#26 foram para as branches de base depois que a #16 já estava na `main`. Proposta: `tb stack status` comparar a pilha declarada com o que está na branch padrão | média |
| `tb stack verify guard` após pull (máquina do usuário) | PASS na 1ª | 1ª falhou em 13s (2 testes de screenshot: Chromium `chromium-1228` não existe mais; só 1243/1255); 2ª com `PAPRIKA_CHROMIUM=…1255` PASS em 26s, guard 4 regras sem violação. Mesma lacuna de `tb doctor` já anotada | baixa |

## Adendo 2026-11-21: correções CodeQL (#29) e renomear no lugar (#31)

`tb guard check` e `tb verify run` (PASS em 1 iteração) na branch do item 4 (correções CodeQL), no merge para o item 6 e na branch nova `-7-renomear` (PR #31, só web + E2E).

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `tb guard check origin/main..HEAD` com a `main` à frente da branch | Avaliar só os commits da branch (semântica de `git log A..B`) | Acusou `test_deletion` em arquivos que só existem na `main` (fixtures de `.github/scripts/test/`): o range parece virar `git diff A B` (árvore contra árvore). Contornei com `$(git merge-base origin/main HEAD)..HEAD`. Proposta: usar a merge-base por padrão (ou `A...B`) e dizer na saída qual base foi usada | média |
| Mudança só de UI coberta por E2E | O veredito dizer se o E2E rodou | Saída mostra `Visible: true`; precisei rodar `test:e2e` e `test:e2e:go` à parte para ter certeza (mesmo atrito dos itens 3 e 6) | média |
| `tb guard check origin/main...HEAD` (três pontos, tentando a semântica de merge-base) | Aceitar `A...B` como no git | Falhou com `hint: git fetch origin .HEAD`: o parser parte no primeiro `..` e sobra `.HEAD`. Proposta: aceitar `...` (merge-base) ou rejeitar com mensagem clara | baixa |
| CI vermelha na pilha por `ci.yml` antigo na branch (sem `fetch-depth: 2`, já corrigido na `main`) | `tb` avisar que a branch está atrás da `main` em arquivos de CI | Só descobri lendo o log do job (`no merge base`); resolvido com merge da `main`. Proposta: `tb stack status` listar arquivos de `.github/` divergentes da branch padrão | baixa |

## Adendo 2026-11-21: página de workspaces (branch `claude/project-thread-qmgfw0`)

`tb stack verify guard review drift --ticket rd.md` na sessão de nuvem (tb v0.0.10 instalado pelo hook `SessionStart`). 1ª rodada FAIL, 2ª PASS em ~3 min (verify, guard, review com 3 revisores, drift sem fora de escopo).

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `verify` na nuvem sem `PAPRIKA_CHROMIUM` | Pré-check do navegador antes da suíte | De novo 3 testes de screenshot falharam no meio da suíte (Playwright pede um headless shell ausente); PASS ao exportar `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium`. Mesmo atrito dos itens 3–6: o `tb doctor`/pré-check continua sendo o que mais economizaria | média |
| Saída do `tb stack ...` redirecionada para arquivo | JSON único para `jq`/`json.load` | O stdout trouxe JSON e as linhas de log `DBG` intercaladas no stderr; com `2>&1` o JSON fica impossível de ler. Separar resolve, mas a saída padrão podia ir só para stdout com `--json` | baixa |
| Mudança que adiciona ferramentas MCP (contrato regenerado) | O drift saber que `server-go/contract/*` é derivado | `drift` aceitou (ticket citava MCP); funcionou bem com o `rd.md` descrevendo o escopo | ajudou |

## Adendo 2026-11-21: diagramas (branch `claude/project-thread-2x0kue`, PR #44)

`tb stack verify guard` na sessão de nuvem (tb instalado em `~/go/bin`, fora do PATH padrão). 1ª rodada ESCALATE, 2ª PASS em ~3 min.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `verify` sem `PAPRIKA_CHROMIUM` exportado | Pré-check do navegador antes da suíte | De novo: 3 testes de screenshot falharam e o veredito foi `ESCALATE` após 3 iterações (~3 min perdidos). PASS ao exportar `PAPRIKA_CHROMIUM`. Continua sendo o atrito nº 1 na nuvem; um `env.require` no config que falhe em 1 s resolveria | média |
| Ler o resultado do `tb stack` | Campo de topo com o motivo da falha | O JSON tem `steps[].result.reason` com o tail do log; útil, mas o `reason` repete o mesmo trecho em `error`. Bastaria um | baixa |
| `tb` instalado por `go install` | Estar no PATH | `~/go/bin` não está no PATH da sessão; precisei `export PATH=$PATH:~/go/bin` em cada comando. O hook podia exportar ou linkar em `/usr/local/bin` | baixa |

## Adendo 2026-11-21: remover o servidor TypeScript (branch `claude/project-thread-tfkzb6`)

`tb stack verify guard review drift --ticket rd.md` na sessão de nuvem (tb v0.0.10 instalado com `go install …@latest` em `~/go/bin`, reporta `toolbelt dev`). A mudança apaga `packages/server`, move as suítes black-box para `packages/e2e` e tira o servidor TS da CI (68 arquivos). Resultado final: `verify` PASS em 1 iteração (com `--base HEAD`, ver abaixo), `review` 0 achados, `drift` sem fora de escopo; `guard` FAIL com 5 `test_deletion` explicados no PR.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Testes **movidos** com `git mv` (`packages/server/test/** → packages/e2e/test/**`, git mostra `R`) | O `guard` seguir renomes (`git diff -M`) e não acusar remoção | 4 `test_deletion` em arquivos que só mudaram de pasta; o `verify` aborta ("guard aborted verify (protected path)") antes de build e testes. Proposta: detectar renomes e reportar como `test_moved` (info) | alta |
| Remoção **intencional** de testes de código apagado (`server.test.ts` testava o servidor removido) | Um jeito auditável de aceitar a remoção (ex.: `allow_test_deletion:` no ticket com motivo, ou `--accept test_deletion:<arquivo>`) | Nenhum mecanismo: o `--ticket` só abre exceção para `protected_paths`. Rodei o `verify` com `--base HEAD --head HEAD` para obter build/lint/testes e deixei a violação explícita no PR. Funciona, mas é um contorno | alta |
| `review`/`drift` com o worktree sujo e HEAD = commit de merge | Base = merge-base com `origin/main` (= HEAD) | Usou `759a056` (o `main` **local**, desatualizado) e acusou como fora de escopo arquivos de `experiments/` que eu não toquei. Resolvido com `git branch -f main origin/main`. Proposta: preferir `origin/<default>` e avisar quando o `main` local está atrás | média |
| `rd.md` em Markdown com uma seção "Escopo" em prosa | O escopo ser lido da seção | 68/68 arquivos "fora do escopo": só o frontmatter YAML com `scope:` (globs) funciona de forma confiável. Proposta: documentar o formato no `tb review --help` ou avisar "escopo vazio/não reconhecido" em vez de marcar tudo como erro | média |
| `verify` sem `PAPRIKA_CHROMIUM` | Pré-check do navegador | Mesmo atrito dos itens anteriores: E2E falhou por `chromium_headless_shell-1255` ausente; PASS ao exportar a variável | média |
| `tb mem edit <id> --rule …` | Editar a invariante que citava o servidor TS | OK, `mem check` com retention 1.00; saída curta e clara | ajudou |

## Adendo 2026-11-21: renomear e excluir workspaces (branch `claude/project-thread-puuktn`, PR #46)

`tb stack verify guard review drift --ticket rd.md` na sessão de nuvem (tb v0.0.10 por `go install`, em `~/go/bin`, reporta `toolbelt dev`). Rodada final PASS: verify 42 s, guard ok, review 0 achados, drift nenhum; `tb guard check <merge-base>..HEAD` ok.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `review`/`drift` rodados **antes do commit** (mudanças só no worktree) | Revisar o diff do worktree, ou avisar "nada commitado desde a base" | Revisou o range padrão `HEAD~1..HEAD`, que era o merge do PR #45, e acusou `styles.css` fora de escopo por uma mudança que não era minha. Só entendi lendo `base_ref`/`head_ref` no JSON. Proposta: com o worktree sujo, avisar e revisar `HEAD` + worktree, ou falhar dizendo "commit antes" | média |
| `rd.md` com a seção `## Escopo (arquivos)` e caminhos entre crases | Reconhecer a lista de arquivos | 22/22 arquivos "fora do escopo" (pior que sem a seção, quando só 1 acusava). Funcionou com `## Scope` e caminhos sem crase, formato achado só lendo `git_errors_test.go` no módulo. Reforça o item da thread anterior: documentar o formato no `--help` e avisar "escopo não reconhecido" | média |
| `verify` com `PAPRIKA_CHROMIUM` exportado | PASS | PASS de primeira, incluindo E2E TS e Go. Exportar a variável antes de rodar continua sendo a receita | ajudou |
| `golangci-lint` do container | Rodar o lint Go local | O binário foi compilado com go1.25 e o módulo pede 1.26, então não roda. Não é do `tb`, mas o `lint_cmd` (`go vet`) não cobre o golangci; o CI cobre | baixa |

## Adendo 2026-11-21: flick ao arrastar elementos (branch `claude/project-thread-l72mwk`, PR #48)

`tb stack verify guard review drift --ticket rd.md` na sessão de nuvem (tb v0.0.10 por `go install`, em `~/go/bin`, fora do PATH; `PAPRIKA_CHROMIUM` exportado). verify PASS e guard ok de primeira (~58 s no total); review/drift FAIL na 1ª rodada só por causa do formato do `rd.md`; na 2ª, review com 3 avisos e drift sem fora de escopo.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `rd.md` com escopo em linha: `Escopo: a.tsx e b.test.ts.` | Reconhecer os dois caminhos, ou avisar que a linha não foi entendida | Só o 1º caminho entrou no escopo; o arquivo de teste saiu como "fora do escopo" e o `drift` falhou. Funcionou com `## Escopo` e uma lista `- caminho`. Terceira thread seguida com o mesmo atrito: um `tb ticket lint rd.md` (ou o escopo lido impresso no início do `review`) resolveria | média |
| `review` com critérios `- [ ]` no ticket | Ligar o critério ao teste que o cobre | 2 avisos "acceptance criterion without evidence" para critérios cobertos por `drag.e2e.test.ts` (nomes de teste em pt-BR, critério com outras palavras). A heurística parece casar só palavras; aceitar `(teste: <nome>)` no critério ajudaria | baixa |
| `review` em arquivo de teste E2E | Ignorar o tamanho de um `describe` | Aviso "added function is too large (195 lines +)" para o callback do `describe` do vitest. Proposta: não contar callbacks de `describe`/`it` como funções | baixa |
| `tb stack` com `2>&1` num arquivo | JSON legível | O JSON ficou depois de linhas de log; tive que procurar o 1º `{` para fazer o parse (mesmo item das threads anteriores) | baixa |
| `verify` com `PAPRIKA_CHROMIUM` exportado | PASS | PASS de primeira, com E2E TS e Go | ajudou |

## Adendo 2026-11-21: migrar o CSS do app para Tailwind (branch `claude/project-thread-ol0wza`)

`tb stack verify guard` na sessão de nuvem (tb v0.0.10 instalado pelo hook `session-start.sh` com `CLAUDE_CODE_REMOTE=true`, em `~/go/bin`, fora do PATH; `PAPRIKA_CHROMIUM` exportado). verify PASS de primeira (~58 s, build + typecheck + `go vet` + unit + E2E TS e Go); `tb guard check <merge-base>..HEAD` ok. A validação principal (18 screenshots antes/depois comparados pixel a pixel) foi um script à parte: o `tb` não tem passo para isso.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `tb stack verify guard` com a mudança ainda **não commitada** | O `guard` olhar o worktree, ou avisar "nada commitado desde a base" | O `verify` rodou sobre o worktree (certo), mas o `guard` checou `7c86a70..d817b6c` (o merge do PR #45, que não era meu) e respondeu ok com 0 violações. Um "ok" sobre o range errado passa a impressão de que a mudança foi checada. Mesmo atrito do item de `review`/`drift` da thread de workspaces: com o worktree sujo, avisar ou incluir o worktree | média |
| Refatoração visual que precisa ficar **idêntica** | Um passo do `verify` (ou um hook) para regressão visual: rodar um script de screenshots na base e no HEAD e comparar | Nada no `tb`; fiz `shots.mjs` + `diff.py` no scratchpad (build da base copiado à mão). Proposta: `visual_cmd` no `config.yaml`, rodado na base e no HEAD num worktree temporário, com o diff de pixels no JSON do `stack` | média |
| Hook de sessão sem `CLAUDE_CODE_REMOTE` no ambiente desta thread | `tb` disponível ao abrir a sessão | O `tb` não estava instalado; rodei `CLAUDE_CODE_REMOTE=true bash .claude/hooks/session-start.sh` e ele instalou com a verificação de hash. O hook funciona, mas não disparou sozinho nesta sessão (não é do `tb`; anotado porque é a porta de entrada dele) | baixa |
| Classes CSS usadas como seletores pelo E2E | Algo que avisasse que remover `is-active`/`layer-name` quebra testes | Descobri pelo E2E (5 falhas em cascata, uma delas por timeout de 5 s). Não é papel do `guard` hoje; um `protected_patterns` (ex.: strings citadas nos testes) seria um aviso barato. Registrado no AGENTS.md: as classes simples são ganchos | baixa |
| `verify` com `PAPRIKA_CHROMIUM` exportado | PASS | PASS de primeira, com E2E TS e Go | ajudou |
| (capa, branch local empilhada sobre o #46) `tb stack verify guard review drift` depois do commit, com `## Scope` em lista simples | PASS de primeira | PASS (verify 26 s, review 0, drift nenhum). Com o formato certo do `rd.md` e o commit feito antes, o fluxo fica rápido; os dois atritos da rodada anterior não se repetiram | ajudou |

## Adendo 2026-11-21: Dockerfile para rodar o Paprika (branch `claude/project-thread-mv4yl5`)

`tb stack verify guard review drift --ticket rd.md` na sessão de nuvem (tb v0.0.10 por `go install`, em `~/go/bin`, fora do PATH; `PAPRIKA_CHROMIUM` exportado), depois do commit e com `git branch -f main origin/main`. Tudo PASS de primeira: verify 88 s, guard 4 regras sem violação, review 0 achados, drift nenhum (escopo no frontmatter `scope:`). A validação principal (build da imagem, smoke HTTP de canvas, `/mcp`, screenshot e persistência) foi um script à parte, `.github/scripts/docker-smoke.sh`.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Mudança cujo produto é uma **imagem Docker** | O `verify` ter um passo para artefatos além de build/lint/test (ex.: `docker build` + smoke) | O `verify` rodou `pnpm build` + testes do app, que não mudou, e deu PASS sem nunca construir a imagem. O PASS é verdadeiro mas não diz nada do que foi entregue. Proposta: `extra_cmds` (ou `artifact_cmd`) no `config.yaml`, selecionável por glob de arquivos tocados (`Dockerfile` → `docker build && smoke`), com o resultado no JSON do `stack` | média |
| `verify` numa mudança só de infra/docs | Pular o que não foi afetado, como o CI faz com `ci-plan.mjs` | Rodou a suíte completa (88 s) para uma mudança que não toca código do app. Proposta: `verify --changed` lendo um seletor do repositório (o `ci-plan.mjs` já existe) | baixa |
| `guard` com um `Dockerfile` novo e um workflow mudado | Algum aviso de superfície sensível (CI e imagem de runtime) | 0 violações; as 4 regras não olham CI/Docker. Um aviso "arquivo de CI/infra alterado: confira pins e permissões" seria barato. O `check-actions-pinned.mjs` do repo cobriu os pins | baixa |
| Escopo no frontmatter `scope:` com caminhos exatos, commit antes do `review` | PASS sem falso "fora de escopo" | PASS de primeira; com a receita das threads anteriores (commit antes, `main` local atualizado, escopo em YAML) o fluxo não teve atrito | ajudou |
| `tb mem list` no início | Invariantes relevantes | Trouxe a de bind em `127.0.0.1` + validação de Host/Origin, que orientou a decisão: a imagem escuta em `0.0.0.0` só dentro do container, a doc publica no loopback do host, e o smoke testa que um `Host` estranho continua recebendo 403 | ajudou |

## Adendo 2026-11-21: ícones Lucide + barra de rolagem (PRs #51 e #53)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Hook de sessão | `tb` no PATH ao abrir a sessão | Não estava; `go install …/cmd/tb@v0.0.10` funcionou (baixou o toolchain go1.27.4 sozinho) e o binário ficou em `~/go/bin`, fora do PATH | baixa |
| `tb stack verify guard review drift` com o worktree **sujo** num HEAD que é merge (`7d489e6`, merge do #48) | Diff do worktree contra `main`, ou recusa clara | `guard`/`review`/`drift` usaram `HEAD^1..HEAD` (`8241be8..7d489e6`) e acusaram `drag.e2e.test.ts` (do PR #48) como fora de escopo; o único aviso foi `WRN dirty worktree` no stderr. Terceira thread com o mesmo atrito: incluir o worktree ou falhar com "nada commitado desde a base" | média |
| Mesmo comando após o commit | PASS | PASS (verify 44 s com E2E 29/29 e API 19/19 contra o Go) | ajudou |
| Branch empilhada (scrollbar sobre Lucide) | Base = `main` ou a base declarada | Base = `HEAD^` (o commit do Lucide), o que por sorte é o certo para PR empilhado; mas o JSON não diz de onde veio a base. Proposta: campo `base_source` (`merge-base main`/`HEAD^`/flag) | baixa |
| Mudança só de CSS (scrollbar) | Validação visual | O `verify` não tem como ver; screenshot manual no Chromium sem `--hide-scrollbars` (o headless esconde a barra por padrão, o que tornaria qualquer `visual_cmd` cego para esse tipo de mudança) | baixa |

## Adendo 2026-11-21: página do agente na lista de presença (branch `claude/project-thread-4plm47`)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Hook de sessão | `tb` no PATH | De novo fora do PATH (`/root/go/bin`); o hook instalou, mas a sessão não vê. Quarta thread com isso | baixa |
| `tb stack verify guard` sem `PAPRIKA_CHROMIUM` exportado | Um aviso claro de pré-requisito antes de rodar ~30 s de suíte | O `verify` falhou 2 iterações com "Executable doesn't exist … chromium_headless_shell-1255" e só o `DBG` dizia por quê. Proposta: `verify.require_env` no `config.yaml` (ex.: `PAPRIKA_CHROMIUM`) checado antes do build | média |
| `tb guard check origin/main..HEAD` após o commit | ok | ok (4 regras, 0 violações); commitar antes do `stack` evitou o atrito do range errado | ajudou |
| Mesmo `stack` com o env exportado | PASS | PASS (28 s, E2E 29/29) | ajudou |

## Adendo 2026-11-21: card de workspace: um clique abre, sem flick, cantos inteiros (branch `claude/project-thread-ioa23i`)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava instalado; `go install …/cmd/tb@v0.0.10` e `PATH+=/root/go/bin`. Quinta thread com isso | baixa |
| `tb stack verify guard` depois do commit, com `PAPRIKA_CHROMIUM` exportado | PASS | PASS de primeira (41 s; build, lint, testes; guard 4 regras sem violação). A receita "commit antes + env exportado" segue sem atrito | ajudou |
| Bug visual subpixel (borda comida pela capa nos cantos, a 1x) | Algum passo que pegue regressão visual | Nada no `tb`; reproduzi com Playwright em 1x/2x e recorte ampliado com ffmpeg, antes e depois. Reforça a proposta de `visual_cmd` (base vs HEAD), com DPR configurável: o defeito quase não aparece em 2x | média |
| Mudança de interação (1 clique abre) que contradiz o comportamento de um PR anterior (#46) | Aviso de que um teste existente codifica o comportamento antigo | O E2E pegou (o teste afirmava `aria-pressed`); o `guard` não tem como saber. OK ser papel do teste | baixa |

## Adendo 2026-11-21: TanStack Query no overview (PR #59)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Sexta thread com isso | baixa |
| `tb stack verify guard --ticket rd.md` com o worktree sujo (tudo em stage, antes do commit) | Diff do worktree contra `main` | PASS (47 s), mas `head_ref` no JSON é o commit da `main` atualizada e `base_ref` o `main` antigo do clone: o relatório não deixa claro que avaliou o worktree. Depois do commit, `tb guard check merge-base..HEAD` ok | baixa |
| Regressão de comportamento (contagem de GETs por rajada de escritas) | Algum jeito de provar "antes falha, depois passa" | Feito à mão: `git stash` do `apps/web`, rebuild, rodar o E2E novo (20 GETs na `main`), `stash pop`. Proposta: `tb verify --against <ref>` que roda um teste selecionado na base e no HEAD e exige falha → sucesso | média |
| `bundle` cresceu 36 kB (+10,5 kB gzip) com a dependência nova | Aviso de orçamento de bundle | Nada no `tb`; medido à mão com dois builds. Proposta: `size_cmd` opcional com limite em `thresholds.yaml` | baixa |
| E2E intermitente na CI depois do merge da main no branch (diálogo reaberto vazio, ~1 em 3) | O `verify` ajudar a separar flake de bug | Um `verify` passa ou falha conforme a sorte; achei a causa rodando a suíte 10x em loop com marcadores. Proposta: `verify --repeat N` (ou `test_repeat` no `config.yaml`) que roda a suíte N vezes e reporta a taxa de falha por teste no JSON | média |

## Adendo 2026-11-21: flick da camada WASM (CPU) ao redimensionar painéis (branch `claude/project-thread-bzqa63`)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Sétima thread com isso | baixa |
| `tb --version` | `v0.0.10` + commit | `toolbelt dev / commit: unknown` quando instalado por `go install`: dá para ler `debug.ReadBuildInfo()` (Main.Version) como fallback | baixa |
| `tb stack verify guard review drift --ticket rd.md` antes do commit | review/drift avaliarem a mudança | `verify` PASS (42 s), mas `review`/`drift` olharam `base..HEAD` com HEAD = `main` (diff vazio) e só acusaram "rd.md has no declared scope". Ninguém avisou que o worktree sujo ficou de fora. Proposta: aviso `worktree_dirty: true` no JSON quando o range não inclui o diff local | média |
| Formato do `rd.md` | Doc do formato à mão (`tb review --help`) | Só lendo `internal/review/review.go` achei o frontmatter YAML (`title`, `scope`, `forbidden`, `acceptance`). Proposta: `tb init` gerar um `rd.md` de exemplo, ou o achado `scope-missing` citar o formato | média |
| Depois do commit, com frontmatter | guard/review/drift limpos | ok, 0 achados (scope = 2 arquivos, forbidden = server-go/core) | ajudou |
| Provar "antes falha, depois passa" no E2E novo | `tb verify --against <ref>` | De novo à mão: `git stash` do arquivo, `vite build`, teste falha (8/8 frames vazios por painel), `stash pop`, passa 3x seguidas. Reforça a proposta da thread do TanStack | média |
| Bug de timing (frame pintado vazio) | Algum apoio a regressão visual por frame | Nada no `tb`; o teste usa um ResizeObserver do próprio teste para ler o canvas antes da pintura. Reforça `visual_cmd` | baixa |

## Adendo 2026-11-21: README humanizado + prompt de setup (branch `claude/project-thread-dxksae`)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@v0.0.10` baixou também o go1.27.4 (toolchain exigida pelo módulo), ~1 min a mais. Oitava thread com isso | baixa |
| `tb guard check "$(git merge-base origin/main HEAD)"..HEAD` antes de `git fetch` | Range só com o meu commit | O `origin/main` do clone estava velho: o guard acusou `test_deletion` em `packages/server/test/*` (remoção do servidor TS, PR #47, de outra pessoa). Depois do `git fetch origin main`, `guard: ok (4 rules, 0 violations)`. Proposta: o guard avisar quando o range tem commits já presentes no remoto (ou sugerir `fetch`), e o AGENTS mandar dar `fetch` antes | média |
| Mudança só de docs (README, docs/, lista de páginas do site) | Um modo "docs" com verificação proporcional | Não há; o `verify` completo (build + E2E) seria desperdício. Rodei à mão: checagem de links relativos (script), `site:build` (32 páginas), screenshots das duas línguas e `ci-plan` (nenhum job). Proposta: `verify --profile docs` com `docs_cmd` (links + build do site) | média |
| Coerência entre línguas (README.md × README.pt-BR.md, docs/ × docs/pt-BR/) | Algo que acuse seção faltando num dos espelhos | Nada no `tb`; conferi os cabeçalhos à mão. Proposta: invariante/regra do guard `mirror` que compare a lista de `##` entre pares de arquivos configurados | baixa |

## Adendo 2026-11-21: thread só de design pelo MCP (máquina do usuário)

Tarefa: desenhar a marca e as interfaces do Paprika (9 pranchas, ~1070 nós) num workspace novo do servidor local, só pelo MCP do Paprika. Nenhuma mudança de código no repositório além do registro de feedback de agentes. O `tb` só entrou no começo.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Hook `SessionStart` na máquina do usuário | Aceitar o `tb` local ou explicar o que difere | `tb on PATH (/home/leonardo/go/bin/tb, module version v0.0.10) is not the pinned v0.0.10.` A versão bate; o que falha é o hash do módulo: `go version -m` mostra `mod … v0.0.10` sem soma `h1:` (binário compilado de um checkout local). A mensagem cita a versão, não o hash, então parece uma contradição | média |
| `tb mem list` no início (regra 1 do AGENTS.md) | Os invariantes, rápido | Ajudou: 9 invariantes em milissegundos; as regras "ferramenta original, nunca copiar o Paper" e "toda mutação por applyOps" enquadraram o trabalho (desenhar só pelo MCP, sem editar o banco) | ajudou |
| Tarefa sem código (design pelo MCP) | Algum jeito de registrar "verify não se aplica" | `tb stack verify guard` não tem o que checar quando só docs mudam e a entrega vive no SQLite; o `tb` não tem noção de entrega externa (screenshots, um documento no MCP) para anexar ao ticket | baixa |

Propostas: o hook do Paprika (ou um `tb version --json` com `sum`) deve dizer "hash ausente/diferente" em vez de repetir a versão; o `tb ticket` poderia aceitar artefatos (caminhos de PNG, id de workspace) como evidência de trabalho sem código.

## Adendo 2026-11-22: PR #55 do Dependabot, Node 26 no Dockerfile sem corepack

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Instalado pelo `.claude/hooks/session-start.sh` com `CLAUDE_CODE_REMOTE=true`, mas em `/root/go/bin`, fora do PATH. Nona thread com isso | baixa |
| `tb stack verify guard` sem `PAPRIKA_CHROMIUM` | Aviso de pré-requisito antes da suíte | Falhou 2 iterações com "chromium_headless_shell-1255 doesn't exist" (~87 s perdidos). Reforça `verify.require_env` | média |
| Mesmo comando com o env exportado | PASS | PASS (build, lint, testes; E2E 30 ok + 7 skip, API 19/19 contra o Go). Guard: 1 aviso `t.Skip()` em `server-go/internal/team/helpers_test.go`, que veio da main (merge), não do PR | ajudou |
| HEAD = merge da main numa branch de terceiro (Dependabot) | Base = merge-base com `main` | `base_ref` = head antigo do Dependabot, então o guard avaliou todo o merge da main e acusou o `t.Skip()` alheio. Proposta: com HEAD de merge, usar `merge-base origin/main HEAD` como base, ou dizer no JSON que a base inclui commits de outra branch | média |
| Mudança em Dockerfile (estágio de build) | Algum passo que exercite o Dockerfile | O `verify` não toca no Docker; o `docker build` local falha no proxy do container (rustup/go mod sem a CA). Validei o estágio alterado num `docker run node:26-bookworm-slim` com a CA montada. Proposta: `docker_cmd` opcional com `--build-arg`/secret de CA configurável | baixa |

## Adendo 2026-11-22: rodada de acessibilidade WCAG 2.1 AA (branch `claude/project-thread-gui1x7`, PR #70)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@v0.0.10` (baixou go1.27.4 de novo) + `PATH+=~/go/bin`. Décima thread com isso | baixa |
| `tb mem list` | Invariantes do projeto | ok, 10 invariantes; o `mandate` design-first estava lá e guiou a decisão de não mexer em cor neste PR | ajudou |
| `tb stack verify guard review drift --ticket rd.md` com `rd.md` em prosa ("Escopo: … apps/web … apps/site build") | O escopo virar globs | `review`/`drift` olharam só `HEAD~1` (o commit do site) e acusaram os 3 arquivos do site como fora de escopo. Com uma lista `## Escopo` de globs e `--base $(git merge-base origin/main HEAD)`, os dois passaram sem achados. Proposta: o `stack` usar o merge-base como base padrão quando HEAD está numa branch, e o achado `scope` mostrar o escopo que ele entendeu | média |
| `verify` com `PAPRIKA_CHROMIUM` exportado | PASS | PASS (build, typecheck + go vet, testes core/Go, API e E2E incluindo o teste novo de a11y) | ajudou |
| Mudança de acessibilidade | Um passo de a11y no `verify` | Não há; rodei axe-core à parte em 16 cenários (script em /mnt/project-files/paprika/a11y/scripts/audit.mjs). Proposta: `a11y_cmd` opcional no config, com o resultado (violações por regra) no JSON do `verify` | média |
| Provar "antes falha, depois passa" | `tb verify --against <ref>` | À mão de novo: `git stash` de apps/web/src, build, 5/5 falham, `stash pop`, 5/5 passam. Reforça a proposta já registrada | média |
| Aviso de qualidade "source code changed without test changes" | Considerar evidência fora de testes | Disparou no commit do site (sem suíte); a evidência era o axe. Proposta: aceitar `evidence:` no rd.md para silenciar com justificativa | baixa |

## Adendo 2026-11-22: leva 2 do backlog, 6 itens em branches empilhadas (branches `claude/project-thread-lqkofu[-N-*]`)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Décima primeira thread com isso | baixa |
| `tb guard check "$(git merge-base origin/main HEAD)"..HEAD` no topo de uma pilha de 6 branches | Achados por branch/PR | Um relatório só para o range inteiro (2 avisos `test_skip` em screenshot_test.go, esperados). Proposta: `tb guard check --stack <branch…>` que avalie cada degrau contra o anterior e diga em qual PR cada achado entra | média |
| Merge da main (PR #70 de outra thread) descendo pela pilha | Ajuda para propagar o merge | À mão: merge em cada degrau, um conflito (AgentCursors.tsx). Proposta: `tb stack sync` que faça o merge em cascata e pare no primeiro conflito | média |
| Mudança no sidecar Rust + supervisor Go + UI (item 4) | `verify` cobrindo `cargo test` | O `verify` não roda `cargo` (crates/sync); rodei `cargo fmt`, `clippy -D warnings` e `cargo test` à parte, e a validação de UI com 3 instâncias reais (script Playwright). Proposta: `rust_cmd` opcional no config | média |
| Desenhar no Paprika local do usuário a partir da nuvem | Caminho previsto | Precisou de Remote Control + script MCP gravado na pasta do usuário e apagado depois. Não é do `tb`, mas o `mandate` design-first poderia dizer o que fazer quando 127.0.0.1:4777 não é alcançável | baixa |

## Adendo 2026-11-22: importar formatos e drag and drop no canvas (branch `claude/project-thread-nsgbq0`, PR #79)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@v0.0.10` + `PATH+=~/go/bin`. Décima segunda thread com isso | baixa |
| `tb --version` depois do `go install …@v0.0.10` | `v0.0.10` | `toolbelt dev`, `commit: unknown`: a versão não é gravada no binário instalado por `go install`. Proposta: ler `debug.ReadBuildInfo()` como fallback | baixa |
| `tb init` num repo já inicializado | Não tocar em arquivos versionados | Reescreveu `.toolbelt/agent_docs.yaml` com 7 hints novos tirados do AGENTS.md (diff no arquivo versionado); desfiz com `git checkout`. Proposta: só reescrever com `--refresh-docs`, ou avisar que mudou um arquivo versionado | média |
| `tb mem list` | Invariantes | ok; o `mandate` de originalidade guiou registrar `.fig` como "precisa de decisão" (engenharia reversa) em vez de prometer o import | ajudou |
| `tb guard check <merge-base>..HEAD` numa mudança só de docs | ok rápido | `ok (4 rules, 0 violations)` | ajudou |
| Segundo PR na mesma branch depois do merge do #79 (i18n, PR #81) | `tb guard check` só no commit novo | ok (0 violations) contra o merge-base com a `main` atualizada; sem atrito | ajudou |

## Adendo 2026-11-22: avaliação do protocolo A2UI (thread "Avaliar o protocolo A2UI no Paprika", sem branch nem PR)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@latest` + `PATH+=~/go/bin`. Décima terceira thread com isso | baixa |
| `tb mem search original` / `tb mem search unknown` | Invariantes que pesam na decisão | ok, achou o `mandate` de originalidade e o de "campos desconhecidos são recusados"; os dois entraram nos riscos (A2UI só por import, com relatório do que foi ignorado) | ajudou |
| Tarefa só de avaliação (spec externa + spike descartável fora do repo) | Um modo de "estudo" do `tb` | Nada se aplica: `verify`/`guard`/`drift` pedem diff. `tb egm` revisaria o relatório, mas exige o documento dentro do repo e o relatório foi para um Claude Doc. Proposta: `tb egm review --file <caminho fora do repo>` em modo determinístico só com `refs-exist` e anti-injeção | baixa |
| Spike chamando o MCP do Paprika por HTTP | `radius: "$token"` aceito como `fontSize` | Recusado ("expected number, received string"). É atrito do MCP do Paprika, não do `tb`; registrado no relatório como item próprio | média |

## Adendo 2026-11-22: planejar uma leva e publicá-la no bus (máquina do usuário)

Tarefa: ler o backlog do Paprika, planejar a leva 3 (17 tickets, raias A/B) e publicá-la no bus local como supervisor (`--as leonardo`). Sem mudança de código.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Mesmo aviso do `SessionStart` do adendo anterior | — | Ainda `module version v0.0.10 is not the pinned v0.0.10` | média |
| `tb bus send --payload @arquivo` em loop, com `--idempotency-key` | Publicação em lote, segura para repetir | Ajudou: 17 envios, uma linha cada (`sent <ulid> seq=N`), a chave torna a repetição inofensiva | ajudou |
| Saber o próximo id livre (`PAP-B-<nnn>`) | `tb bus log --thread-prefix` ou uma lista de threads | Precisei de `tb bus log paprika --limit 200 | grep -o 'PAP-[A-Z]-[0-9]*' | sort -u`; o log redige os payloads mas não os ids de thread, e foi isso que funcionou. Proposta: `tb bus threads <canal>` com estado (atribuída, reclamada, concluída) | média |
| Tickets da raia A (threads cloud, fora do bus) | Um tipo de mensagem para reservar paths sem atribuir | Não há `task.reserve`/`announce`; usei `task.status` para `topic:board`, que um worker pode ler como progresso. Proposta: um tipo `announce` ou um campo `reserved_paths` que o `claim` verifique | média |
| Dependências entre tickets (`deps` no payload) | `claim` recusar ou avisar enquanto a dependência não concluiu | `deps` é JSON livre que o bus ignora; um worker pode reclamar a PAP-B-023 antes da PAP-B-022 terminar | média |
| Sobreposição de paths entre tickets abertos | Checagem do lado do supervisor antes do envio | Manual (comparei `paths` à mão); `tb bus channel show` lista claims, não os paths de tickets atribuídos e não reclamados | baixa |

Propostas: `tb bus threads`, uma mensagem de reserva/anúncio para a raia A, `deps` respeitado pelo `claim` e checagem de sobreposição dos `paths` atribuídos no envio.

## Adendo 2026-11-22: desenhar ícones pelo MCP depois de uma mudança de diretórios (máquina do usuário)

Tarefa: redesenhar seis ícones planos de vegetais no Paprika local pelo MCP e registrar o atrito. Nenhuma mudança de código no repo além do log de feedback dos agentes. O checkout estava em `chore/house-cleaning`, cujo último commit moveu `server-go/` para `server/` e `docs/` para `docs/en-US/`.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `tb mem list` no início | As invariantes, rápido | Ajudou: 10 invariantes na hora; "nunca copiar o Paper" e "toda mutação pelo applyOps" enquadraram o trabalho (desenhar só pelo MCP) | ajudou |
| `tb stack guard` com a base padrão `HEAD~1` | Passar, ou apontar mudanças reais em testes | `ok: false` com 14 erros (11 `test_deletion`, 3 `test_skip`) em 0,3 s. `git show -M --stat` mostra os arquivos de teste como renomeações puras (`{server-go => server}/…_test.go`, 0 linhas mudadas). O guard lê renomeação como remoção; o agente teria de descartar 14 erros falsos à mão antes de dar o trabalho por pronto | alta |
| Texto das invariantes depois da mudança | Invariantes que citam caminhos continuam válidas | A invariante `server-go is the only server` (e o AGENTS.md) ainda cita `server-go/`, que não existe mais; `tb mem check` não olha caminhos | média |
| Tarefa sem código (design pelo MCP) | Um jeito de dizer "verify não se aplica" | Igual ao adendo de design de 2026-11-21: nada no `tb` aceita screenshots ou id de workspace como evidência | baixa |

Propostas: o guard deveria rodar com detecção de renomeação (`git diff -M`) e tratar renomeação com ≥90 % de similaridade como movimento, não `test_deletion`; `tb mem check` poderia avisar quando uma invariante cita um caminho que não existe na árvore.

## Adendo 2026-11-22: migração para o toolbelt v1.0.0-rc.3 (máquina do usuário, worktree isolado)

Tarefa: levar o Paprika do `tb` v0.0.10 para o v1.0.0-rc.3 (config, hook `SessionStart`, AGENTS.md e docs) a partir de um worktree sobre `origin/main`, sem mexer no barramento `paprika` ativo. A mudança `server-go/` → `server/` ainda está só no branch local `chore/house-cleaning`, sem push; por isso a `main` continua com `server-go/` e a invariante continua correta lá.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Descobrir o que mudar depois de atualizar | Uma nota de migração por release, ou `tb init --migrate` / `tb doctor` | Só o CHANGELOG (60 KB; só a rc.1 tem ~40 itens). Resolvi rodando `tb init --json` numa cópia do repo e comparando o `config.yaml` gerado com o commitado. `tb init` nunca migra um arquivo existente | alta |
| Qualquer comando `tb` com a config antiga | As chaves removidas avisadas uma vez | Nove linhas `warning: .toolbelt/config.yaml: unknown config key …` no stderr em **todo** comando (`tb mem list`, `tb guard`, `tb bus …`), antes da saída. Claro sobre o problema, ruidoso até alguém migrar. Um `tb config check` (ou o aviso dizer em que release a chave saiu) tornaria isso acionável | média |
| PR que edita `verify:` (aqui: adicionar `typecheck_cmd`, mudar `lint_cmd`) | O guard aponta a mudança; o verify roda com a config confiável da base | `verify_config_changed` é erro **e aborta o verify**, inclusive com `-c <arquivo>`. Um PR que migra a config de verify nunca consegue `PASS` em `tb verify`/`tb stack verify`; rodei os gates à mão (`pnpm go:vet && pnpm check`). O motivo de segurança é bom, mas não há caminho para uma mudança de config revisada: proposta: rodar o verify com a config da base e só reportar o achado, ou um `--accept-config-change <sha>` que só um humano (não o MCP) pode passar | alta |
| `tb verify run --base origin/main` num worktree | Base = a main atualizada | `git fetch origin main` só moveu o `FETCH_HEAD`, então `origin/main` estava velho e o guard do verify rodou contra uma base antiga (dezenas de `test_deletion` sem relação). Não é bug do tb, mas o verify poderia avisar quando a base está atrás do upstream, ou mostrar a data da base | baixa |
| Falso positivo de renomeação (relatado no adendo anterior) | Corrigido na rc.3 | Continua: um repo de teste com `git mv server-go server` dá 20 erros `test_deletion` mais avisos `test_skip` de skips que já estavam nos arquivos. Causa: `parseDiff` em `internal/guard/guard.go` marca a origem de toda entrada `R*`/`C*` como `deleted: true`. Proposta: num `R`, só contar remoção quando o destino não é arquivo de teste (e não reportar de novo skips inalterados em arquivos `R100`) | alta |
| `go install …@v1.0.0-rc.3` no hook da nuvem | O mesmo binário da release | Funciona e o hash `h1:` está no sum.golang.org. `tb version` agora mostra a versão (corrigido desde a rc.1), mas `commit: unknown` e `embed: unavailable (built without embednative)`: busca semântica/híbrida no grafo não existe em sessões na nuvem que instalam via `go install`. Vale uma linha na doc de instalação | baixa |
| Barramento durante a atualização | Saber se o canal ativo está seguro | O store já estava no schema v4 (tabela `task_links`) quando olhei (só leitura, `mode=ro`), então outro `tb` já tinha migrado. Três processos `tb mcp` ainda rodavam um binário apagado (um `tb` mais antigo) contra esse store v4. Nada no `tb bus` avisa o supervisor que um cliente roda versão mais antiga que o store; proposta: agentes informam a versão do `tb` no `heartbeat` e `channel show` aponta divergências | média |
| Velocidade de `tb mem list` / `tb guard` | Rápido | Ajudou: ambos instantâneos; o guard mostrou o bug de renomeação e o bloqueio de config em menos de um segundo | ajudou |

Propostas principais: uma seção de migração por release (ou `tb init --migrate`), um caminho revisado para mudanças em `verify:`, a correção de renomeação no `parseDiff` e versão reportada no barramento.

## Adendo 2026-11-23: fechar um canal e abrir outro para três sessões do Cursor (máquina do usuário)

Tarefa: aposentar o canal `paprika` (nenhum agente vivo, nenhum claim aberto), abrir `paprika-2` e emitir uma identidade de worker distinta por sessão do Cursor, depois que duas sessões com a mesma identidade colidiram num worktree (PAP-B-013).

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Conferir que dá para fechar | Agentes vivos e claims abertos numa só visão | Ajudou: `tb bus channel show paprika` listou quatro agentes (todos `alive=false`) e quatro claims (todos `released outcome=expired`) numa chamada | ajudou |
| Fechar o canal antigo | `tb bus channel close` (ou `delete`/`archive`) | Não existe. Usei `tb bus channel extend paprika --ttl 1s` para ele expirar; funciona e é reversível (`extend --ttl 0`), mas parece gambiarra e o `channel list` continua mostrando o canal ao lado de canais de smoke antigos. Proposta: `channel close <ch>` que recusa enquanto houver claim ou agente vivo, e `list --open` | média |
| `tb bus invite --name cursor-N` | A saída diz a que agente o convite está preso | A saída é `invite: XEP9-D4ZF (channel paprika-2, role worker, single-use, expires …)`, sem o nome. Com três convites emitidos num laço, o mapa convite → nome vem só da ordem do laço; não há `invite list` para conferir sem gastar o convite | média |
| Uma janela do Cursor, vários chats | Uma identidade por chat | `tb mcp` prende a identidade à conexão MCP, e um cliente que roda um só processo MCP para vários chats daria a mesma identidade a todos: a mesma colisão do PAP-B-013. Usei o join pela CLI (`--as <nome>` em todo comando) e uma janela do Cursor por worktree. Proposta: a doc dizer isso com todas as letras, e o `join` recusar uma segunda identidade na mesma conexão com erro claro | alta |
| Convite do supervisor com `--invite-ttl 24h` | Tempo para entregar | Ajudou: `channel create` aceita `--invite-ttl`, e `invite --ttl 24h` cobre um usuário que cola os convites horas depois | ajudou |
| Nove avisos `unknown config key` | Uma vez | Continuam em todo comando (ver o adendo da migração rc.3) | baixa |

Propostas: `channel close`, o nome preso na saída do `invite` (e um `invite list` para o humano), e um aviso explícito de "uma identidade por conexão MCP".

## Adendo 2026-11-24: abrir o `paprika-3` depois de fechar o `paprika-2` (máquina do usuário, tb v1.0.0-rc.4)

Tarefa: abrir um canal novo para os mesmos três workers do Cursor (`cursor-1/2/3`) e o supervisor `leonardo`, ainda sem tickets (o foco é estabilizar a v0.1.0). Mesma receita do adendo de 2026-11-23.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Hook `SessionStart` | Aceitar o `tb` local ou dizer o que difere | `tb on PATH … module version v1.0.0-rc.4 is not the pinned v0.0.10`: o hook do repo ainda fixa a v0.0.10, embora o projeto já use rc.x. Deriva do lado do Paprika, mas o `tb` não tem como declarar "este repo precisa de tb ≥ X" para o hook ler | baixa |
| Criar o canal | Uma chamada com lease e convite de dono longo | Ajudou: `channel create paprika-3 --owner leonardo --ttl 0 --lease 5m --invite-ttl 24h` | ajudou |
| Emitir três convites vinculados | A saída nomeia o agente vinculado | Continua `invite: 1SV8-XMNH (channel paprika-3, role worker, …)` sem o nome; o mapeamento vem da ordem do loop (igual a 2026-11-23) | média |
| Arquivos de wake do canal novo | `tb bus run` criar/copiar do canal anterior | Não há comando; copiei `wake/paprika-2/cursor-N.yaml` para `wake/paprika-3/` com `sed` no comentário e `chmod 600`. Proposta: `tb bus run --init --from <canal>` ou um template de `wake` por agente | baixa |
| Subir um worker com um comando | Entrar e esperar juntos | Ajudou: `tb bus run --invite <código>` faz o join quando ainda não há sessão, então cada janela do Cursor precisa de uma linha para o harness | ajudou |
| Canais antigos em `channel list` | Fechados escondidos | `paprika` e `paprika-2` (expirados via `extend --ttl`) ainda aparecem junto dos canais de smoke; continua sem `channel close` ou `list --open` | baixa |
| Avisos `unknown config key` | Sumirem depois da migração | Sumiram: nenhum aviso em nenhum comando nesta sessão | ajudou |
| Recuperar payloads de tickets cancelados de um canal fechado | `tb bus log --thread X --payload` para o supervisor, ou reenviar a partir de uma thread | Supus que o `log` ainda escondia payloads (como diz o `docs/agent-bus.md` do Paprika) e li `messages.payload` do `bus.db` com Python em `mode=ro`. Depois o `tb bus log paprika-3` mostrou os payloads inteiros na rc.4, então o contorno não era necessário; o que está desatualizado é a doc do consumidor. Proposta: dizer no CHANGELOG/help do `log` se payloads aparecem, e oferecer `send --from-thread <canal>/<thread>` para republicar | baixa |
| Republicar 10 tickets com `deps` num canal novo | O bus dizer o que faz com `deps` | Ajudou (novo na rc.4): todo send avisa que o broker não aplica `deps` e indica `tb router` ou mandar a próxima tarefa ao concluir. Claro, mas impresso 10 vezes no loop; uma vez por execução bastaria | ajudou |
| Worker entrou sem `--wake` | O harness do `run` usar o arquivo de wake, ou o `join` avisar que já existe um para esse nome | O `join` usa `--wake manual` por padrão; o `run` mostra `manual` no cartão e um aviso `wake_mismatch`, e corrigir exige convite novo e um segundo join (o primeiro convite já foi gasto). As instruções do supervisor (as minhas) não tinham a flag. Proposta: o `join` ler o adapter do arquivo de wake quando existir, ou o `run` poder atualizar o modo declarado sem convite novo | média |

## Adendo 2026-11-24: avaliação de libs de canvas (thread "Avaliar libs de canvas para o editor", sem branch nem PR)

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Início da sessão | `tb` no PATH | Não estava; `go install …/cmd/tb@latest` (~1 min). Décima quarta thread com isso; `tb --version` segue `toolbelt dev`, `commit: unknown` | baixa |
| `tb mem list` | Invariantes que pesam na decisão | ok, e decisivo: "nodeStyle é a fonte única de renderização" e "toda mutação passa por applyOps" descartam Konva/Fabric/Pixi/tldraw/Excalidraw sem precisar de spike | ajudou |
| Avaliação de dependência externa (licença, encaixe) | Algum apoio do `tb` para "avaliar dependência" | Nada se aplica: `verify`/`guard`/`drift` pedem diff. Proposta: `tb mem check-dep <pacote> <licença>` cruzando a licença com uma allowlist em invariants.yaml (ex. só MIT/Apache/BSD), para barrar licenças como a do tldraw SDK cedo | baixa |

## Adendo 2026-11-24: fix do PR #166 (sessão na nuvem, tb v1.0.0-rc.3)

Mudança de 1 commit restrita a `server-go/cmd/paprika`. Validação com `go test ./...` e `golangci-lint` direto, sem o `verify` completo.

| Situação | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| Instalação numa sessão nova | `tb` pronto ao abrir a sessão | `go install …/cmd/tb@v1.0.0-rc.3` exige go ≥ 1.27.4; o auto-switch da toolchain baixou o 1.27.4 sem erro, mas custou ~1 min. Proposta: binário pré-compilado no hook, ou uma mensagem clara quando a toolchain local estiver abaixo da exigida | baixa |
| `tb guard check origin/main..HEAD` num PR de 1 commit | ok rápido | `ok (4 rules, 0 violations)`, rápido e sem ruído | ajudou |
| `tb mem list` | Invariantes legíveis | ok, nada a ajustar | ajudou |
| `tb verify run` | Gate completo | Não rodado: mudança restrita a um pacote Go; `go test ./...` e `golangci-lint` rodaram fora do `tb` | neutro |

## Adendo 2026-11-25: conceitos de marca (thread "Refinar conceito da marca Paprika")

| Comando | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `tb` na sessão da nuvem | já no PATH pelo hook SessionStart (TB_VERSION v1.0.0-rc.3) | ausente; precisou `go install …/cmd/tb@v1.0.0-rc.3` → ~/go/bin (fora do PATH) | média |
| `tb guard` | rodar a checagem padrão do diff | imprime a ajuda; a checagem é `tb guard check` | baixa |
| `tb guard check` / `tb mem check` | ok em mudança só de docs | `guard: ok (4 rules, 0 violations)`; `invariants: 10, retention 1.00` | ok |

## Adendo 2026-11-25: tempo das GitHub Actions com act (thread "Reduzir o tempo das GitHub Actions", branch `claude/project-thread-0puwlr`, sem PR)

| Comando | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `tb` na sessão da nuvem | no PATH | ausente; `go install …/cmd/tb@latest` → ~/go/bin; `tb --version` = `toolbelt dev`, `commit: unknown` | baixa |
| `tb guard check` (commit do fix no sidecar Rust) | ok | `guard: ok (4 rules, 0 violations)` | ok |
| Investigação de CI (tempos por job/step via `gh api`, `act` local) | algum apoio para medir pipeline | nada se aplica; agreguei os tempos com script próprio. Proposta: `tb bench ci <run-ids>` que agregue mediana/p90 por job e step e aponte o caminho crítico | baixa |
| Teste com espera fixa (sidecar morto após 5 s em todo teste) | o `tb` acusar teste lento demais | nada; achei medindo à mão. Proposta: regra do `guard`/`verify` que sinalize teste cuja duração ≈ múltiplo exato de um timeout do código | média |

## Adendo 2026-11-25: house cleaning (thread "Fazer house cleaning no repositório", branch `claude/project-thread-rkbjtz`)

| Comando | Esperado | Obtido | Severidade |
|----------|----------|--------|------------|
| `tb` na sessão da nuvem | no PATH pelo hook SessionStart | ausente; rodei o hook à mão (`CLAUDE_CODE_REMOTE=true bash .claude/hooks/session-start.sh`), que instalou o v1.0.0-rc.3 pinado em ~/go/bin (fora do PATH) | baixa |
| `tb guard check <merge-base>..HEAD` com `git mv server-go server` | renomes reconhecidos | 60 `test_deletion` (error) nos caminhos antigos dos `_test.go` movidos; o falso positivo conhecido do rc.3 escala mal num renome de diretório inteiro. Proposta: seguir renomes com `git diff -M` e reportar `test_moved` (info) | alta |
| `tb guard check` com remoção intencional de testes (bench/) | um jeito de declarar a remoção no ticket | só `test_deletion` (error), sem como aceitar; precisei provar à mão contando `func Test`/`it(` antes e depois. Proposta: `allow_test_deletion: [glob]` no frontmatter do rd.md | média |
| `tb guard check` depois de tirar comentários | nada sobre linhas que não mudaram de sentido | `test_skip` (warning) em todo `t.Skip()` de arquivo tocado, mesmo sem mudar a linha. Proposta: só acusar `t.Skip` adicionado no diff | média |
| `tb stack verify guard --ticket rd.md` | checar o range da branch | o guard do stack usou `HEAD~1` como base (só o último commit); o range inteiro exige `tb guard check` à parte. Proposta: base padrão = merge-base com `main` | média |
| `bench:` em `.toolbelt/config.yaml` apontando para diretório removido | aviso ou erro | removi a seção e `tb mem list` seguiu ok; nenhum comando reclamou antes, mesmo com o diretório ausente | baixa |
| `tb stack verify` (build, typecheck, go:vet, testes) | ok | ok, com `PAPRIKA_CHROMIUM=/opt/pw-browsers/chromium` (o caminho `chromium-1243/...` dispara a checagem de revisão 1243 ≠ 1255) | ok |
