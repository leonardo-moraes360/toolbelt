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
