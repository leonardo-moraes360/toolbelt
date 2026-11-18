🇧🇷 Português | [🇺🇸 English](en/cli.md)

# Árvore de Comandos do `tb`

Este documento descreve a CLI do toolbelt: comandos **implementados** (espelham `tb --help`) e seções **planejadas** (`tb run`/`proc`/`md`/`agent`, marcadas explicitamente). Em caso de divergência, a CLI Cobra é a fonte da verdade.

## Local-first: sem GitHub

O `tb` **não fala com o GitHub** (nem com outras forges): é **100% local**. Lê o
git do working tree, arquivos no disco e `.toolbelt/`. Não abre issues, não
baixa PRs e não chama a API do GitHub. Por isso `tb review`, `tb drift`,
`tb verify --ticket` e `tb stack` precisam de um **`rd.md`** (ou outro ticket
local via `--ticket` / argumento posicional) — não de um número de issue.

### Como obter ou gerar o `rd.md`

Escreva o arquivo na raiz do repo (ou passe `--ticket caminho`). Formatos
aceitos: frontmatter YAML ou Markdown com `## Scope` / `## Escopo`,
`## Forbidden` / `## Proibido` e checkboxes de aceite (detalhes em `tb review`).

A partir de uma **issue** ou **PR** (fora do `tb`, com `gh` ou o browser):

```bash
# Issue → rascunho local
gh issue view 19 --json title,body > /tmp/issue-19.json
# Copie title/body para rd.md e declare scope/forbidden/acceptance

# PR → corpo + arquivos tocados como ponto de partida do Scope
gh pr view 42 --json title,body,files > /tmp/pr-42.json
gh pr diff 42 --name-only
```

Exemplo mínimo:

```markdown
---
title: "fb3/19-docs"
base_commit: main
scope:
  - docs/cli.md
  - README.md
  - rd.md
forbidden:
  - tests/hidden/**
acceptance:
  - docs descrevem o comportamento real
---

## Notas
Texto livre da issue/PR (contexto para humanos/agentes).
```

Ou só Markdown:

```markdown
# fb3/19-docs

## Scope
- docs/cli.md
- README.md
- `rd.md`

## Forbidden
- tests/hidden/**

- [ ] docs descrevem o comportamento real
```

`tb ticket create --from rd.md` (Fase 3) cria o ticket a partir desse arquivo;
ainda é você quem escreve o `rd.md`.

## Visão Geral

```
tb  (implementado hoje)
├── init         Inicializa configuração local/global
├── bench        Harness de benchmark (run|list|report|compare)
├── drift        Drift de escopo / commit base
├── graph        Grafo sob demanda (TB_GRAPH=on ou --enable)
├── guard        Proteções anti-trapaça (check)
├── mcp          Servidor MCP sobre stdio
├── mem          Invariantes (add|list|show|edit|retire|search|check|init)
├── review       Revisão fresh-context
├── sandbox      Isolamento Landlock (check|run)
├── stack        Empilha ops de verificação (verify/review/drift/guard); não é stack de branches
├── verify       Verificação adversarial (run; rulers tests|visual)
├── workspace    Workspaces via git worktree (create|list|remove; alias: ws)
├── version      Mostra versão (também: tb --version)
├── ticket       Tickets de trabalho (create|list|show|cancel|retry)       ✓ Fase 3
├── board        Board de tickets (list|watch|pause|resume|drain)          ✓ Fase 3
├── router       Scheduler em primeiro plano (run|status|config)           ✓ Fase 3
├── watchdog     Alertas e controle de workers (status|alerts|kill)       ✓ Fase 3
├── merge        Integra tickets done (fail|last_wins)                     ✓ Fase 3
└── bus          Barramento entre agentes (channel|invite|join|send|recv|claim|complete|log|run)  ✓ B1

tb  (planejado)
├── run / proc / md / agent
└── serve        (hoje use: tb mcp)
```

---

## Comandos Detalhados

### `tb run` — Execução de Comandos (planejado — Fase 3+)

> **Não implementado como `tb run`.** Hoje use `tb sandbox run` para isolamento Landlock.

Executa comandos com isolamento opcional.

```bash
tb run [flags] <command>

Flags:
  --timeout <duration>    Tempo máximo de execução (default: 30s)
  --sandbox               Executa em sandbox isolado (default: false)
  --allow-net <hosts>     Hosts permitidos na rede (requer --sandbox)
  --allow-read <paths>    Caminhos adicionais de leitura (requer --sandbox)
  --allow-write <paths>   Caminhos adicionais de escrita (requer --sandbox)
  --cgroup-cpu <percent>  Limite de CPU em % (default: 100)
  --cgroup-mem <bytes>    Limite de memória (default: 1G)
  --json                  Saída em JSON
```

**Exemplos:**

```bash
# Execução simples
tb run "go test ./..."

# Com timeout
tb run --timeout 2m "npm run build"

# Em sandbox completo
tb run --sandbox --timeout 30s "python script.py"

# Sandbox com rede limitada
tb run --sandbox --allow-net "api.example.com:443" "curl https://api.example.com/data"

# Com limites de recursos
tb run --cgroup-cpu 50 --cgroup-mem 512M "heavy-process"
```

---

### `tb proc` — Gerenciamento de Processos (planejado — Fase 3+)

> **Não implementado.** A CLI atual não expõe `tb proc`.

Gerencia processos de longa duração (servidores, watchers).

```bash
tb proc <subcommand>

Subcomandos:
  start     Inicia um processo
  stop      Para um processo
  restart   Reinicia um processo
  list      Lista processos ativos
  logs      Exibe logs de um processo
  status    Status de um processo específico
```

**`tb proc start`:**

```bash
tb proc start [flags] <command>

Flags:
  --name <name>           Nome do processo (obrigatório)
  --log <path>            Arquivo de log (default: .tb/logs/<name>.log)
  --restart <policy>      Política de restart: never|on-failure|always
  --max-restarts <n>      Máximo de restarts automáticos
  --sandbox               Executa em sandbox
```

**Exemplos:**

```bash
# Iniciar servidor de desenvolvimento
tb proc start --name "dev-server" "npm run dev"

# Com restart automático
tb proc start --name "worker" --restart on-failure --max-restarts 3 "./worker"

# Listar processos
tb proc list
# NAME         PID     STATUS    UPTIME      CPU    MEM
# dev-server   12345   running   10m30s      2.1%   128M
# worker       12346   running   5m15s       0.5%   64M

# Ver logs
tb proc logs dev-server --follow --tail 100

# Parar processo
tb proc stop dev-server

# Parar todos
tb proc stop --all
```

---

### `tb graph` — Grafo Sob Demanda (✓ Fase 2, ADR-011)

Consulta e indexa o grafo de código **sob demanda**. Desabilitado por padrão;
ative com `TB_GRAPH=on` ou `--enable` (sem isso, CLI e MCP respondem
`grafo desabilitado (ative com --enable ou TB_GRAPH=on; ADR-011)`). Medir ROI via
`tb bench compare --config-a bench/ab/graph-off.yaml --config-b bench/ab/graph-on.yaml`.

Linguagens e docs (extensível via `LanguageIndexer`, regex leve, sem cgo):

| `graph.languages` | Extensões |
|-------------------|-----------|
| `go` | `.go` (AST; arestas `calls`) |
| `javascript` | cobre TypeScript (`ts`/`tsx`): `.js`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.jsx` (funções genéricas `foo<T>`, `const` arrow, JSX `<Comp />` como `calls`; indexa `*.test.*`/`*.spec.*`/`__tests__`/`e2e`; resolve imports relativos NodeNext (`.js`→`.ts`, sem extensão, `index`) e aliases de `tsconfig.json`/`jsconfig.json` (`extends`/`baseUrl`/`paths`); segue `import { a as b }` e `import * as ns`). Não há linguagem `typescript` separada — use `javascript` |
| `python` | `.py` (defines/imports + arestas `calls` por módulo qualificado; `callers`/`impact` ignoram arestas com confiança &lt; 0.6) |
| `java` | `.java` |
| `kotlin` | `.kt`, `.kts` |
| `csharp` | `.cs` |
| `cpp` | `.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.hpp`, `.hh`, `.hxx` (defines/imports; sem arestas `calls` — `callers`/`impact` retornam erro explícito) |
| `rust` | `.rs` (defines/imports + arestas `calls` mínimas resolvíveis no repositório) |
| `php` | `.php` |
| `ruby` | `.rb` |
| `swift` | `.swift` |
| `shell` | `.sh`, `.bash`, `.zsh`, `.ksh` |

| `graph.docs` | Extensões | O que indexa |
|--------------|-----------|--------------|
| `markdown` | `.md` | documento, headings, links, menções |
| `html` | `.html`, `.htm` | documento, `<h1>`–`<h6>`, âncoras |
| `txt` | `.txt` | documento, paths/símbolos mencionados |

| `graph.frameworks` | Extensões / convenção | O que indexa |
|--------------------|-----------------------|--------------|
| `react` | `.tsx`, `.jsx` | componentes, hooks `use*`, arestas `renders` (em paralelo com `javascript` para `calls`/callers/impact) |
| `vue` | `.vue` | SFC script/template, imports, componentes no template |
| `svelte` | `.svelte` | script, imports, componentes no markup |
| `astro` | `.astro` | frontmatter, imports, componentes no body |
| `angular` | `*.component.ts` / `.html`, services/modules | `@Component`/`@Injectable`/`@NgModule`, selector, `templateUrl` |
| `routes` | Next `app/`/`pages/`, SvelteKit `routes/`, Nuxt `pages/` | nós `route` por convenção de arquivo |
| `mdx` | `.mdx` | doc (headings/links) + imports/componentes JSX |

Frameworks rodam junto com o indexador de linguagem no mesmo arquivo (ex.:
`.tsx` recebe `react` + `javascript`, para componentes/`renders` e
`calls`/callers/impact). Docs também podem indexar o mesmo path quando a
extensão casa.

```yaml
# .toolbelt/config.yaml (todos ligados por padrão)
graph:
  languages: [go, javascript, python, java, kotlin, csharp, cpp, rust, php, ruby, swift, shell]
  docs: [markdown, html, txt]
  frameworks: [react, vue, svelte, astro, angular, routes, mdx]
  max_file_bytes: 524288   # ignora arquivos maiores
  index_on_change: true    # reindexa em search/callers/deps/impact/stats se o fingerprint mudou
```

Ignora subdiretórios como `node_modules`, `vendor`, `dist`, `build`,
`target`, `.git`, etc. A raiz do walk nunca é pulada pelo nome (ex.: indexar
`/tmp/target` funciona; um `target/` filho continua ignorado). Se a raiz for
um symlink (ex.: `/tmp/target` → dir real), ela é resolvida com
`EvalSymlinks` antes do walk — caso contrário o índice ficaria vazio, pois
`filepath.Walk` não desce symlinks. O grafo continua sob demanda (ADR-011):
`TB_GRAPH=on` ou `--enable`.

Frescor do índice: cada `graph.json` guarda um `fingerprint` (mtime+tamanho dos
arquivos indexáveis). Em `search`/`callers`/`deps`/`impact`/`stats`, se o
fingerprint divergir e `graph.index_on_change` for `true` (default), o grafo é
reindexado automaticamente; se for `false`, a saída avisa e o JSON inclui
`stale: true`. Saídas JSON de consulta usam o envelope
`{"stale": false, "nodes": [...]}`; `stats`/`index` incluem `stale` no objeto
de estatísticas.

```bash
tb graph [--enable] <subcommand>

Subcomandos:
  search    Busca no grafo ✅
  callers   Encontra chamadores de uma função ✅
  deps      Mostra dependências ✅
  impact    Analisa impacto de mudanças ✅
  explain   Explica um nó (arestas + proveniência) ✅
  path      Menor caminho entre dois nós ✅
  index     Indexa código sob demanda ✅
  rebuild   Reconstrói grafo completo ✅
  stats     Estatísticas do grafo ✅
```

Ferramentas MCP de grafo (chamada requer `TB_GRAPH=on`): `tb_graph_search`,
`tb_graph_callers`, `tb_graph_deps`, `tb_graph_impact`, `tb_graph_explain`,
`tb_graph_path`, `tb_graph_index`.

Não há `tb graph query` — use `tb graph search`.

**`tb graph search`:**

```bash
tb graph search <query> [flags]

Flags:
  --limit <n>         Máximo de resultados (default: 20)
  --json              Saída em JSON (flag global)
```

Ordem em três faixas: (1) título idêntico à consulta; (2) nós que contêm a consulta como substring de título/path/pacote/ID — o que a busca antiga trazia; (3) nós que só casam termos. Dentro de cada faixa, ranking BM25 sobre título, path e pacote (título pesa 3×), com camelCase, `snake_case` e separadores de path quebrados em termos; consulta com várias palavras casa qualquer termo. Empates ordenam por ID. Com mais resultados que `--limit`, o corte é feito nessa ordem.

**`tb graph callers`:**

```bash
tb graph callers [flags]

Flags:
  --function <name>   Nome da função (aceita sufixo qualificado, ex. scrubber.redact_secrets)
  --depth <n>         Profundidade do grafo (default: 1)
```

Não há `--file` em `callers` (use `--function`). Para impacto por arquivo, use `tb graph impact --file`.

Em Python/JS/TS, `--function` resolve por título ou sufixo do id qualificado
(`func:pkg.mod.nome`; componentes/hooks React também). Arestas `calls` com
confiança &lt; 0.6 (alvos ambíguos / sem import) ficam de fora do resultado
default de `callers` e `impact`. Em TS/JS, imports relativos
(`../components/Layout`, `./utils.js` → `utils.ts`, `./index`) e aliases de
`tsconfig`/`jsconfig` (`@/…`) resolvem para o arquivo/símbolo importado;
`import { helper as format }` e `import * as Utils` / `Utils.helper()` também
alimentam `callers`/`impact` (incluindo funções em arquivos de teste).

**`tb graph deps`:**

```bash
tb graph deps [flags]

Flags:
  --module <name>     Nome do módulo/pacote
  --file <path>       Arquivo para analisar
  --transitive        Inclui dependências transitivas
```

**`tb graph impact`:**

```bash
tb graph impact [flags]

Flags:
  --file <path>       Arquivo modificado
  --depth <n>         Profundidade da análise (default: 2)
```

**`tb graph explain`:**

```bash
tb graph explain <nó> [flags]

Flags:
  --limit <n>         Máximo de nós explicados (default: 5)
```

Mostra o nó com as arestas de saída e entrada (`source`, `confidence`). O nó
resolve por ID exato → título exato → sufixo de função → arquivo/pacote.
Chamadas cujo alvo não tem nó no índice (stdlib, receptor não resolvido) são
contadas em `unresolved_out`. Até 50 vizinhos por sentido (`truncated_*`).

**`tb graph path`:**

```bash
tb graph path <origem> <destino> [flags]

Flags:
  --max-depth <n>     Máximo de saltos (default: 6)
```

Menor caminho (BFS) andando arestas nos dois sentidos; cada passo indica
`forward` (→) ou `backward` (←). JSON: `{"stale", "hops", "steps": [...]}`.
Sem caminho ou nó inexistente ⇒ erro e exit ≠ 0. Ambos ignoram `calls` com
confiança &lt; 0.6 ([ADR-014](adr/014-graphify-reimplementacao.md)).

**`tb graph index`:**

```bash
tb graph index [path] [flags]
```

O path é **posicional** (não existe `--path`).

**Exemplos:**

```bash
# Busca
tb graph search "ValidateToken"
# NODE_ID      TYPE       PATH                      TITLE
# node-123     function   src/auth/jwt.go          ValidateToken

# Chamadores de uma função
tb graph callers --function "ValidateToken" --depth 2

# Dependências de um módulo
tb graph deps --module "auth" --transitive

# Impacto de mudança
tb graph impact --file "src/config/parser.go"

# Re-indexar código após mudanças (path posicional)
tb graph index src/

# Reconstruir grafo completo (path posicional opcional, como index)
tb graph rebuild
tb graph rebuild src/

# Estatísticas
tb graph stats
```

---

### `tb mem` — Invariantes (✓ Fase 1)

Gerencia o arquivo `.toolbelt/invariants.yaml` (fora da compactação, ADR-010).

```bash
tb mem <subcommand>

Subcomandos:
  add       Adiciona regra invariante
  list      Lista invariantes (omite retired; --all inclui)
  show      Exibe invariante por id
  edit      Edita category/rule/ref
  retire    Soft-retire (omitida de list/check/search por padrão)
  search    Busca em id/categoria/regra/ref
  check     Verifica integridade (hash e retenção)
  init      Cria conjunto padrão anti-trapaça

Subcomandos (planejados — vault Markdown):
  link      Cria link entre entradas
  tag       Gerencia tags
  export    Exporta memória
  import    Importa memória
  sync      Sincroniza vault → SQLite
```

Subcomando desconhecido (`tb mem foo`) sai com código ≠ 0 e mensagem `unknown command`.

**`tb mem add`:**

```bash
tb mem add <regra> [flags]

Flags:
  --category <cat>    Categoria: anti_cheat|mandate|security (default: anti_cheat)
  --ref <path>        Vínculo com ADR/decisions.md (ex: docs/adr/0001.md)
  --immutable         Marca como imutável (default: mutável)
```

**Default de imutabilidade:** novas invariantes nascem **mutáveis**. Use `--immutable` (CLI) ou `immutable: true` (MCP) para protegê-las. `tb mem init` grava o conjunto anti-trapaça já com `immutable: true`. Categoria `anti_cheat` e `immutable: true` exigem `--force --reason` para `edit`/`retire`.

Categoria inválida ou `--ref` fora do formato ADR/decisions saem ≠ 0 com a lista
dos valores válidos (`anti_cheat, mandate, security`; refs como
`docs/adr/<arquivo>.md` / `docs/decisions/<arquivo>.md`).

**`tb mem list` / `search` / `check`:**

```bash
tb mem list [--category <cat>] [--all]
tb mem search <query> [--all]
tb mem check [--expect-sha <sha>] [--all]
```

`search` é case- e accent-insensitive (Unicode NFD + remoção de marcas).

**`tb mem show` / `edit` / `retire`:**

```bash
tb mem show <id>
tb mem edit <id> [--category <cat>] [--rule <texto>] [--ref <path>] [--force] [--reason <motivo>]
tb mem retire <id> [--force] [--reason <motivo>]
```

**`tb mem init`:** cria o conjunto padrão anti-trapaça e imprime o caminho + as
invariantes criadas (ou `já existiam N…` se o arquivo já tinha entradas). Com
`--json`, emite `path`/`created`/`count`/`invariants`.

**`tb mem check` (implementado):**

```bash
tb mem check [flags]

Flags:
  --expect-sha <sha>  Hash esperado pós-compactação
  --all               Inclui invariantes aposentadas
```

`mem check` **falha fechado**: arquivo ausente/ilegível/corrompido, SHA divergente, IDs ausentes ou regra não avaliável (id/categoria/regra vazios) → exit ≠ 0. Não cria defaults automaticamente (use `tb mem init`).

**Redação:** `show`/`list`/`search` (texto e `--json`) e payloads MCP equivalentes mascaram padrões comuns (`ghp_`/`gho_`/`github_pat_`, `AKIA…`, PEM private key, `Bearer`, `xox*`, `*_TOKEN=`) com `[REDACTED]`.

**Histórico:** mudanças forçadas em protegidas são append-only em `.toolbelt/invariants_history.jsonl` (quem, quando, motivo, valor antigo).

**Exemplos:**

```bash
# Adicionar invariante (argumento posicional obrigatório)
tb mem add "Não modificar tests/hidden/**"
tb mem add --category security --ref docs/adr/0005.md "Não relaxar thresholds sem revisão"
tb mem add --immutable --category mandate "Escopo fixo do ticket"

# Listar / buscar / exibir / editar / aposentar
tb mem list
tb mem list --category anti_cheat
tb mem list --all
tb mem search thresholds
tb mem search autenticacao   # encontra "autenticação"
tb mem show inv-123
tb mem edit inv-123 --ref docs/decisions/auth.md --rule "Escopo via ticket"
tb mem edit inv-123 --rule "nova" --force --reason "incidente"
tb mem retire inv-123
tb mem retire inv-123 --force --reason "substituída por ADR"
tb mem check
tb mem init
tb mem init --json
```

> Soft-retire mantém a entrada no YAML; `list`/`search`/`check` omitem retired a menos que `--all`.
> Flags vault `--type`/`--title` **não existem** — `tb mem add` aceita regra posicional, `--category`, `--ref` e `--immutable`.
> `tb mem init --json` devolve `{path, created, count, invariants}`; listas vazias saem como `[]`.

---

### `tb md` — TUI Markdown (planejado — Fase 3+)

> **Não implementado.** A CLI atual não expõe `tb md`.

Interface TUI para navegar e editar o vault.

```bash
tb md [flags] [path]

Flags:
  --obsidian          Abre no Obsidian ao invés da TUI
  --read-only         Modo somente leitura
  --graph             Inicia na visualização de grafo
```

**Exemplos:**

```bash
# Abrir TUI na raiz do vault
tb md

# Abrir arquivo específico
tb md vault/facts/auth-design.md

# Abrir no Obsidian
tb md --obsidian

# Modo somente leitura para agentes
tb md --read-only
```

**Atalhos da TUI:**

| Tecla | Ação |
|-------|------|
| `j/k` | Navegar lista |
| `Enter` | Abrir arquivo |
| `e` | Editar arquivo |
| `g` | Visualização de grafo |
| `s` | Buscar |
| `/` | Filtrar |
| `b` | Ver backlinks |
| `l` | Ver links |
| `q` | Sair |
| `?` | Ajuda |

---

### `tb agent` — Gerenciamento de Agentes (planejado — Fase 3+)

> **Não implementado.** A CLI atual não expõe `tb agent`.

Gerencia agentes na hierarquia.

```bash
tb agent <subcommand>

Subcomandos:
  spawn     Cria novo agente
  list      Lista agentes ativos
  status    Status de agente específico
  logs      Logs de agente
  stop      Para agente
  budget    Exibe/modifica orçamento
  approve   Aprova ação pendente
  reject    Rejeita ação pendente
```

**`tb agent spawn`:**

```bash
tb agent spawn [flags]

Flags:
  --role <role>       Role/persona do agente (obrigatório)
  --parent <id>       ID do agente pai
  --task <task>       Tarefa inicial
  --budget <tokens>   Orçamento de tokens
  --timeout <dur>     Timeout máximo
```

**Exemplos:**

```bash
# Criar agente worker
tb agent spawn --role worker-backend --parent supervisor-001 \
  --task "Implementar endpoint /api/users" \
  --budget 5000 --timeout 5m

# Listar agentes
tb agent list
# ID              ROLE              STATUS     PARENT          TOKENS   UPTIME
# planner-001     planner           working    -               45000    10m
# supervisor-001  supervisor        idle       planner-001     18000    8m
# worker-001      worker-backend    working    supervisor-001  3200     3m
# worker-002      worker-backend    blocked    supervisor-001  1500     2m

# Ver status detalhado
tb agent status worker-001

# Ver ações pendentes
tb agent list --pending
# ID          ACTION              RESOURCE            REQUESTED_BY    WAITING
# action-123  file:delete         tests/old.go        worker-001      2m30s
# action-124  network:external    api.example.com     worker-002      1m15s

# Aprovar ação
tb agent approve action-123 --reason "Arquivo obsoleto"

# Rejeitar ação
tb agent reject action-124 --reason "Não autorizado para rede externa"
```

---

### `tb stack` — Empilhamento de Operações de Verificação (✓ Fase 2)

Executa `verify`, `review`, `drift` e/ou `guard` numa única invocação, compartilhando
o trabalho caro (diff unificado, índice de arquivos alterados, memória/invariantes e
config). Etapas somente leitura e independentes (`guard`, `drift`, `review`) rodam em
paralelo (errgroup + `--jobs`); `verify` (build/test) fica serial/isolado. A saída JSON
preserva a ordem pedida nos argumentos, independente da ordem de término. A saída é
sempre um JSON agregado (`ok`, `steps`, `total_duration_ms`); o exit code do processo
é ≠ 0 se qualquer etapa falhar (mesma semântica de cada comando avulso). Durações são
`float` em milissegundos (com fração sub-ms). Erros de preparação ou de uma
operação aparecem em `steps[].error` dentro do JSON (não só no stderr).

**Não é stack de branches** (como Graphite ou git-stack). `tb stack` só empilha
operações de verificação/revisão com contexto compartilhado.

```bash
tb stack <op> [<op>...] [flags]

Flags:
  --base <ref>           Ref base compartilhada (default: HEAD~1; validada/resolvida)
  --head <ref>           Ref head compartilhada (default: HEAD; validada/resolvida)
  --fail-fast            Cancela as demais etapas via context na primeira falha
  --jobs <n>             Limite de concorrência (default: nº de CPUs, máx. 32; 1 = serial)
  --ticket <path>        Ticket/rd.md (review/drift/verify; default: rd.md)
  --approved-sha <sha>   SHA aprovado; patch-id verbatim (review/drift; validado/resolvido)
  --approved-base <ref>  Base do range aprovado (default: merge-base(approved, base); validada/resolvida)
  --require-approved-sha Falha se --approved-sha estiver ausente (ou `review.require_approved_sha`)
  --warn-only            guard: exit 0 mesmo com SeverityError
  --test-cmd <cmd>       verify: testes visíveis
  --hidden-test-cmd <cmd> verify: testes ocultos
  --build-cmd <cmd>      verify: build (vazio + flag explícita ou --no-build para pular o do config)
  --lint-cmd <cmd>       verify: lint (vazio + flag explícita ou --no-lint para pular)
  --no-build             verify: não roda build_cmd
  --no-lint              verify: não roda lint_cmd
  --dry-run              verify: mostra o plano (comandos/dir) sem executar
  --max-iter <n>         verify: máximo de iterações
  --timeout <duration>   verify: timeout (default: 10m)
  --reviewers <n>        review: máximo de goldfish
  --standards <csv>      review: papéis (scope,quality,acceptance)
  --env <KEY>            (global) allowlist extra para o verify empilhado
  --pass-env <KEY>       (global) opt-in explícito de segredo para o verify
```

Sem operação, ou com operação desconhecida, sai com erro curto listando as ops
válidas (`verify`, `review`, `drift`, `guard`) — sem dump de `Usage`.
Ticket inexistente ou ilegível (arquivo/ID apontado por `--ticket`) faz as etapas
`review`/`drift` falharem com exit ≠ 0 e mensagem citando o caminho procurado
(mesma semântica de `tb review` / `tb drift` / MCP).

**Exemplos:**

```bash
tb stack verify review drift guard
tb stack guard drift review --jobs 4
tb stack guard drift --fail-fast
tb stack review drift --base HEAD~1 --head HEAD --ticket rd.md
tb stack verify guard --test-cmd "true" --max-iter 1
tb stack guard drift review --jobs 1
tb stack verify --test-cmd "pnpm test" --no-build --no-lint --max-iter 1
tb stack verify --dry-run --test-cmd "go test ./..."
tb stack verify --test-cmd "true" --env APP_ENV --pass-env GITHUB_TOKEN
```

Cada item de `steps` traz `op`, `ok`, `exit_code`, `duration_ms`, `reason`/`error`
(quando a etapa falha) e `result` (o JSON que o comando avulso produziria). No
`Prepare`, chamadas git independentes (`diff` / `name-status` / `name-only`) e carga
de invariantes também rodam em paralelo. O `verify` empilhado usa o mesmo
`--base`/`--head` do contexto compartilhado: se o head divergir do `HEAD` do
worktree, build/lint/test rodam num worktree detach temporário (ver `tb verify`); o
`result` do passo inclui `head_sha` e `workdir`.

---

### `tb guard` — Proteções de Segurança (✓ implementado: `check`)

Analisa o diff entre commits em busca de paths protegidos e padrões anti-trapaça.

```bash
tb guard check [base..head] [flags]

Flags:
  --base <ref>   Ref base (default: HEAD~1; validada/resolvida para SHA)
  --head <ref>   Ref head (default: HEAD; validada/resolvida para SHA)
  --warn-only    Exit 0 mesmo com SeverityError (opt-out do default CI-safe)
  --strict       Deprecated: falha em SeverityError já é o default; use --warn-only para opt-out
  --json         Saída em JSON (flag global)
```

Refs que começam com `-` ou SHA inexistente saem com erro claro (sem chamar
`git` de forma que interprete a ref como opção).
Por default, exit ≠ 0 quando há `SeverityError` (comportamento estrito).
`--strict` é **deprecated** (alias sem efeito: o default já falha em
`SeverityError`); prefira omitir a flag ou usar `--warn-only` para opt-out.
No modo texto limpo imprime `guard: ok (N regras, 0 violações)` (`N` = regras
válidas da base + defaults).

**Exemplos:**

```bash
tb guard check HEAD~1..HEAD
tb guard check --base HEAD~1 --head HEAD
tb guard check --warn-only HEAD~1..HEAD
tb guard check --json HEAD~1..HEAD
```

> Subcomandos `snapshot`, `verify`, `protect` e `audit` **não existem** na CLI atual — são planejados.

---

### `tb review` — Revisão Fresh-Context (✓ Fase 2)

Implementa o padrão Elephant-Goldfish: goldfish **distintos** revisam o diff
contra `ticket/rd.md` e invariantes de `tb mem`, sem herdar contexto do autor.
Não há clones do mesmo checker — `--reviewers` só limita quantos papéis rodam.

```bash
tb review [ticket] [flags]

Flags:
  --base <ref>          Commit base (default: HEAD~1 ou base_commit do ticket)
  --head <ref>          Commit head (default: HEAD)
  --approved-sha <sha>       SHA aprovado; compara `git patch-id --verbatim` de approved_base..approved vs head_base..head
  --approved-base <ref>      Base do range aprovado (default: `git merge-base <approved> <base>`)
  --require-approved-sha     Falha se `--approved-sha` estiver ausente (ou `review.require_approved_sha` na config)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --reviewers <n>            Máximo de goldfish distintos (default: 3; não clona)
  --standards <stds>         Papéis: scope,quality,acceptance (default)
  --output <path>            Relatório JSON (sem shorthand `-o`; use `--output`)
  --json                     Saída em JSON (flag global)
```

Ticket inexistente ou ilegível sai com código ≠ 0 e mensagem
`ticket não encontrado: <caminho>` / `ticket ilegível (<caminho>): …`
(CLI, MCP `tb_review` e `tb stack`/`tb_stack`).

Também reporta drift de escopo e commit base. Isolamento fresco é obrigatório.
`tb review -o …` é rejeitado (`unknown shorthand flag: 'o'`); use `--output`.

Base e head são resolvidos para SHA **uma vez** no início do fluxo (anti-TOCTOU) e
esse SHA pinado é usado em todo o restante.

O JSON sempre inclui `pinned_approval`. Sem `--approved-sha`:
`{"checked": false}`. Com a flag: `checked`, `approved_sha`, `approved_base`,
`head_base`, `patch_id`, `head_patch_id`, `match` e, se divergir, `range_diff`.
As bases default são `merge-base(approved, base)` e `merge-base(head, base)`.
A comparação usa `git patch-id --verbatim` (sensível a whitespace); patch-id igual
**ou** `git range-diff` só com `=` conta como match (rebase limpo). Patch diferente
do aprovado imprime um resumo de `git range-diff` e sai com código ≠ 0. Se
`--approved-sha` coincidir com a base (intervalo `base..approved` vazio), a CLI
**não** despeja o usage do `git range-diff` — mostra mensagem clara sobre o intervalo inválido.
Com `--require-approved-sha` (ou `review.require_approved_sha: true` em
`.toolbelt/config.yaml`), a ausência de `--approved-sha` é erro.

O ticket/rd.md aceita frontmatter YAML ou Markdown: seção `## Scope` / `## Escopo`
(ou `Scope:` / `Escopo:`) e `## Forbidden` / `## Proibido` (ou `Forbidden:` /
`Proibido:`) com listas (`-`, `*`, numeradas) e paths entre backticks.
Critérios de aceite vêm de `acceptance:` no YAML ou checkboxes `- [ ]` / `- [x]`
no Markdown. Se o frontmatter YAML não declara `scope:`, o body Markdown ainda
é lido para preencher o escopo.
Globs no scope/forbidden incluem `src/*.go`, `src/**`, `tests/hidden/**` e
padrões com `**` no meio (ex.: `src/**/foo.go`). Toques em paths do Forbidden
preenchem `forbidden_hit` no drift e geram finding critical no review.

Invariantes de path (ex.: `tests/hidden/`, `thresholds.yaml`) são casadas contra
arquivos **alterados** pelo mesmo matcher de path/glob (`tests/hidden/**`,
`tests/hidden/*`) — não por substring no texto do diff ou do ticket. Mencionar
`forbidden: tests/hidden/**` em `rd.md` sem tocar esse diretório não gera
finding critical. Invariantes de mandate sobre conteúdo (ex.: proibir
`console.log` ou comentários) são aplicadas às linhas adicionadas (`+`) do diff.

**Revisores (goldfish) distintos**:

| Revisor | `--standards` | O que verifica |
|---------|---------------|----------------|
| `goldfish-scope` | `scope` (+ `invariants`) | Arquivos fora do scope, toques em forbidden; invariantes de `tb mem` aplicadas a paths alterados **e** a linhas `+` do diff (ex.: `console.log`, comentários proibidos) |
| `goldfish-quality` | `quality` | Marcadores `TODO`/`FIXME`/`XXX`/`HACK` (palavra inteira em maiúsculas, em comentários); funções muito grandes no diff (≥80 linhas +); código fonte alterado sem testes |
| `goldfish-acceptance` | `acceptance` (alias: `completeness`) | Cada critério de aceite do ticket tem evidência em testes/arquivos de código alterados (não conta o próprio ticket); considera `Test*` (Go) e `test`/`it`/`describe` (JS/TS) adicionados no diff |

Heurísticas são determinísticas e offline (sem LLM).

Cada finding JSON inclui `role` (papel do goldfish: `scope`, `quality`, `acceptance`, `invariants`).
Achados de aceite também trazem `criterion` e `evidence` (testes/arquivos vistos no diff).
`agreement` conta quantos papéis distintos concordaram no mesmo achado (em geral 1).
No modo texto, imprime `papel=<role>` (e `papéis_acordo=N` só se N>1) — não usa mais `acordo=N/M`.

**Exemplos:**

```bash
# Revisão padrão: 3 papéis distintos
tb review --ticket rd.md
# ═══════════════════════════════════════════════════════════
#  RELATÓRIO DE REVISÃO: rd.md
#  Base..Head: HEAD~1..HEAD | Revisores: 3 | Fresh: true
# ═══════════════════════════════════════════════════════════
# FINDINGS:
#   [warning] (goldfish-quality/quality) TODO/FIXME/XXX/HACK adicionado em comentário no diff [src/a.go]  papel=quality
#   [warning] (goldfish-acceptance/acceptance) critério de aceite sem evidência...  papel=acceptance

# Só escopo + qualidade (sem aceite)
tb review --ticket rd.md --standards scope,quality --reviewers 2

# Pedir 5 revisores ainda roda no máximo os papéis distintos (sem clones)
tb review --ticket rd.md --reviewers 5 --standards scope,quality,acceptance

# Saída JSON para integração (só --output; sem -o)
tb review --ticket rd.md --json --output review-results.json

# Verificar que o head ainda é o mesmo patch do SHA aprovado
tb review --base main --approved-sha abc1234 --ticket rd.md --json
```

**Isolamento obrigatório**: Se o sistema não conseguir criar contextos frescos para os revisores, o comando falha com erro ao invés de prosseguir com contexto contaminado.

---

### `tb drift` — Drift de Escopo/Base (✓ Fase 2)

Compara os arquivos alterados em `base...head` com o **scope** declarado no
ticket/`rd.md`, detecta toques em `Forbidden`/`Proibido` (`forbidden_hit`) e
base que não é ancestral do head (`base_mismatch`). **Não** compara vault
Markdown contra código — o relatório é só escopo/base/forbidden
(+ `pinned_approval` opcional).
O parser de scope/forbidden é o mesmo do `tb review`
(YAML frontmatter ou Markdown `## Scope` / `## Forbidden` / listas / globs com `**`).

O diff usa **three-dot** (`git diff base...head` ≡
`git diff $(git merge-base base head) head`), o mesmo critério de PRs no GitHub:
após merge/squash na base, só entram arquivos do branch — não o que veio da main.

`--base` / `base_commit` do ticket (ex.: `HEAD^`, tags, SHA curto) e
`--head` / `--approved-sha` (CLI e MCP) são validados (rejeitam refs que
começam com `-`) e resolvidos com
`git rev-parse --verify --end-of-options <ref>^{commit}` antes de qualquer
`git diff`/`range-diff` — evita injeção de opções do git, SHA inexistente
opaco e falso `base_mismatch` por comparar a string da ref com o SHA
declarado. O SHA resolvido é o que segue no restante do fluxo.

`--base` explícito na CLI/MCP **sempre** prevalece sobre `base_commit` do ticket.
Sem `--base`, o default `HEAD~1` cede a `base_commit` quando o ticket declara um.
`base_mismatch` é `true` quando o `base_commit` declarado no ticket (ou a base
efetiva, se o ticket não declara) **não** é ancestral do head
(`git merge-base --is-ancestor`) — assim `--base main` em PR divergente não
gera falso mismatch; um SHA de outra linha (não ancestral) sim. O JSON expõe
`actual_base` (SHA resolvido da base usada no diff) e `merge_base` (ancestral
comum com o head).

```bash
tb drift [ticket] [flags]

Flags:
  --base <ref>          Commit base (default: HEAD~1 ou base_commit do ticket)
  --head <ref>          Commit head (default: HEAD)
  --approved-sha <sha>       SHA aprovado; compara `git patch-id --verbatim` de approved_base..approved vs head_base..head
  --approved-base <ref>      Base do range aprovado (default: `git merge-base <approved> <base>`)
  --require-approved-sha     Falha se `--approved-sha` estiver ausente (ou `review.require_approved_sha`)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --output <path>            Saída JSON (sem shorthand `-o`)
  --json                     Saída em JSON (flag global)
```

Ticket inexistente ou ilegível sai com código ≠ 0 e mensagem
`ticket não encontrado: <caminho>` / `ticket ilegível (<caminho>): …`
(CLI, MCP `tb_drift` e `tb stack`/`tb_stack`).

O JSON de drift inclui `pinned_approval` no mesmo formato do `tb review`
(`checked: false` sem a flag; com a flag: bases `approved_base`/`head_base`,
patch-id verbatim; match por patch-id igual ou range-diff só com `=`).
Divergência sai com código ≠ 0 e mostra `git range-diff`. Base/head são pinados
a SHA no início.
`--approved-sha` igual à base segue a mesma mensagem clara do `tb review` (sem
dump de usage).

**Exemplos:**

```bash
# Drift do diff HEAD~1..HEAD contra rd.md (default)
tb drift
# ═══════════════════════════════════════════════════════════
#  RELATÓRIO DE DRIFT
# ═══════════════════════════════════════════════════════════
# Base: HEAD~1 → abc1234
# Fora do scope (1):
#   - b.go
# Proibidos tocados (0):

# Ticket explícito + saída JSON para CI
tb drift --ticket rd.md --json --output drift-report.json

# Pin de aprovação: falha se o patch do head mudou desde o SHA aprovado
tb drift --base main --approved-sha abc1234 --ticket rd.md
```

JSON (`--json` / `--output`) usa `out_of_scope`, `forbidden_hit`,
`base_mismatch`, `declared_base`, `actual_base` (e `pinned_approval` quando
há `--approved-sha`). Exit ≠ 0 se houver drift ou patch pinado divergente.

**Integração com CI**:

```bash
# No CI, falhar se houver arquivos fora do scope
tb drift --ticket rd.md --json | jq '.out_of_scope | length' | xargs -I {} test {} -eq 0
```

---

### `tb verify` — Verificação Adversarial (✓ Fase 1/2)

Executa o loop gerador→discriminador com rulers plugáveis.

```bash
tb verify run [flags]

Flags:
  --ruler <type>           Tipo de ruler: tests|visual (default: tests)
  --test-cmd <cmd>         Testes visíveis (default: verify.test_cmd; `tb init` detecta go/npm/pnpm/yarn/pytest/cargo/make e também build_cmd/lint_cmd)
  --hidden-test-cmd <cmd>  Suite oculta (verify.hidden_test_cmd; se vazio e existir tests/hidden/: detecta toolchain ou marca N/A)
  --build-cmd <cmd>        Build (default: verify.build_cmd; vazio + flag explícita ou --no-build para pular)
  --lint-cmd <cmd>         Lint (default: verify.lint_cmd; vazio + flag explícita ou --no-lint para pular)
  --no-build               Não executa build_cmd
  --no-lint                Não executa lint_cmd
  --dry-run                Mostra o plano (comandos e diretório) sem executar
  --max-iter <n>           Máximo de iterações (default: 3)
  --timeout <dur>          Timeout (default: 10m)
  --threshold <float>      Threshold de similaridade visual (default: 0.95)
  --manifest <path>        Manifesto de telas (default: .toolbelt/screens.yaml)
  --skip-unavailable       Skip gracioso sem browser/refs/telas (default: true; veredicto SKIPPED)
  --base <ref>             Ref base
  --head <ref>             Ref head (build/lint/test rodam neste commit)
  --ticket <path>          Ticket com Scope (exceção de `protected_paths` custom)
  --output <path> / -o     Arquivo de saída JSON
  --json                   Saída em JSON (flag global)
  --env <KEY>              (global, repetível) inclui KEY na allowlist do filho
  --pass-env <KEY>         (global, repetível) opt-in explícito para segredo no filho
```

Quando `--head` (ou o par `--base`/`--head`) resolve para um SHA **diferente** do
`HEAD` do worktree atual, o verify cria um `git worktree add --detach` temporário
nesse SHA, roda `build_cmd` / `lint_cmd` / `test_cmd` / `hidden_test_cmd` nele e
remove o worktree ao terminar (também em erro ou sinal). O JSON inclui
`head_sha` e `workdir` (o diretório em que os comandos rodaram). Se
`head == HEAD` e o worktree estiver sujo, emite aviso — os comandos rodam no
worktree atual e podem ver mudanças não commitadas.

Processos filhos do verify (`sh -c` de `test_cmd`/`build_cmd`/`lint_cmd`/`hidden_test_cmd`)
recebem só uma **allowlist** de ambiente (PATH, HOME, LANG, TMPDIR, GOPATH/GOCACHE,
NODE/npm/pnpm essenciais, proxies, etc.). Nomes com `TOKEN`, `SECRET`, `KEY` ou
`PASSWORD` **nunca** passam, a menos de `--pass-env KEY` (ou `env.pass` no
`.toolbelt/config.yaml`). `--env KEY` estende a allowlist, mas **não** contorna o
bloqueio de segredos.

`tb init` preenche `verify.test_cmd` a partir da toolchain do repositório
(`go.mod` → `go test ./...`, `package.json` + scripts.test → `npm`/`pnpm`/`yarn`/`bun test`
via lockfile `pnpm-lock.yaml`/`yarn.lock`/`package-lock.json`/`bun.lockb`,
`pyproject.toml`/`uv.lock` → `pytest`/`uv run pytest`, `Cargo.toml` → `cargo test`,
`Makefile` com target `test`/`check` → `make test`/`make check`). Sem marcadores
reconhecidos, `verify.test_cmd` fica **vazio** e o init emite um aviso — nunca
mantém `go test ./...` sem `go.mod`. O binário do comando escolhido é validado no
`PATH`: se `uv` estiver ausente com `uv.lock`, faz fallback para `pytest` ou
`python -m pytest` / `python3 -m pytest` (e avisa); managers Node ausentes
caem para `npm test` quando possível.

Quando há `AGENTS.md`/`CLAUDE.md`, o init grava `.toolbelt/agent_docs.yaml` com
`sources` e `hints` de comandos curtos (inline, fences shell e prosa do tipo
“use/run/prefer …”). Fences de código longo (ex. ` ```go `) são ignorados para
não poluir as dicas.

Quando há marcadores claros, o init também preenche `verify.build_cmd` e
`verify.lint_cmd` de forma conservadora:

| Toolchain | `build_cmd` | `lint_cmd` |
|-----------|-------------|------------|
| `go.mod` | `go build ./...` | `.golangci.y*ml`/`.toml` → `golangci-lint run`; senão target `make lint` |
| `package.json` | script `build`, senão `typecheck`/`type-check` → `npm run …` / `pnpm run …` / `yarn …` | script `lint` → análogo |
| `Cargo.toml` | `cargo build` | `cargo clippy` |
| Python (`pyproject`/`uv.lock`/…) | (vazio) | `[tool.ruff]`/`ruff.toml` → `ruff check .` (ou `uv run …`); senão mypy |
| só `Makefile` | target `build` → `make build` | target `lint` → `make lint` |

Ausência de script/config → campo omitido. Sem `test_cmd` detectado, o verify
não inventa `go test` (configure à mão ou adicione script/marcador).

A suite oculta permanece escondida do agente (paths em `tests/hidden/` protegidos
pelo `tb guard`). O comando configurado em `verify.hidden_test_cmd` ou
`--hidden-test-cmd` **sempre executa**, mesmo sem o diretório `tests/hidden/`
(caso típico em monorepos JS/Python/Rust). Se o comando estiver vazio e o
diretório existir, o verify **detecta a toolchain** da suíte (`.go`+`go.mod` →
`go test ./tests/hidden/...`; `.py` → `pytest tests/hidden`; `.js`/`.mjs`/`.ts` →
`node --test tests/hidden`). Sem suíte aplicável, marca `hidden_available: false`
(N/A, `hidden_passed: null`) e não roda `go test` fora de projeto Go. Sem comando e sem diretório, a
suite oculta também é ignorada. Campos opcionais `build_cmd` e `lint_cmd` (YAML
ou `--build-cmd`/`--lint-cmd`) rodam antes dos testes (nessa ordem); `--no-build` /
`--no-lint` ou valor vazio com a flag explícita pulam o comando do config.
Build/lint bem-sucedidos são **cacheados** dentro da mesma execução (não
repetem a cada iteração); falha determinística de build/lint **não** entra no
retry de `--max-iter` (ESCALATE imediato). `--dry-run` devolve o plano
(`dir` + `commands`) com veredicto `SKIPPED` sem executar nada.

Se o binário do `test_cmd` / `hidden_test_cmd` / `build_cmd` / `lint_cmd` não
existe (exit 127), não é executável (exit 126) ou o `exec` falha com
`ErrNotFound`, o verify **falha imediatamente** sem retries, com mensagem do tipo
`comando não encontrado: X — ajuste verify.test_cmd`. O processo `tb` sai com
código **127**. Com `--json` (ou `--output`), emite o envelope `verify.Result`
normal incluindo `error` (e `verdict: FAIL`), em vez de sair sem JSON. Falhas
genuínas do `test_cmd`/`hidden_test_cmd` (exit ≠ 0/126/127) continuam no loop
de `--max-iter`.

Veredictos `ESCALATE` e `FAIL` saem com **exit ≠ 0** também com `--json`
(igual ao modo texto). Só `PASS` e `SKIPPED` (ruler visual / `--dry-run`) usam
exit 0. Comando inexistente usa exit **127** (ver tabela de códigos). O JSON
de verify (e cada etapa do `tb stack`) inclui `reason`/`error` com o motivo da
falha ou do ESCALATE (não só em DEBUG no stderr).

**Guard dentro do verify (anti-cheat vs mandate):**

O verify sempre roda `tb guard` no diff e inclui o resultado em `guard`. A decisão
de **abortar antes de build/test** depende da origem do `protected_path`:

| Origem | Sem `--ticket` | Com `--ticket` e path no Scope | Com `--ticket` e path fora do Scope |
|--------|----------------|--------------------------------|-------------------------------------|
| Defaults embutidos (`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`, `.toolbelt/guard.yaml`) | **aborta** (FAIL, 0 iterações) | **aborta** | **aborta** |
| `protected_paths` de `.toolbelt/guard.yaml` na **base** (custom) | reporta como **warning** e **continua** build/test | reporta warning (exceção de Scope) e **continua** | **aborta** |

`tb guard check` (e o alias deprecated `--strict`) trata violações custom como
`SeverityError` e falha o exit code por default; use `--warn-only` para opt-out.
Só o loop de `tb verify run` aplica a exceção acima para não matar o mandate
quando o manifesto lista o próprio arquivo da tarefa.

```yaml
# .toolbelt/config.yaml
verify:
  build_cmd: "pnpm --filter @pkg build"
  lint_cmd: "pnpm --filter @pkg lint"
  test_cmd: "pnpm --filter @pkg test"
  hidden_test_cmd: "pnpm --filter @pkg test:hidden"
```

```bash
tb verify run --test-cmd "pnpm test" --hidden-test-cmd "pnpm test:hidden"
# sem tests/hidden/: o hidden_test_cmd acima ainda roda
tb verify run --test-cmd "pnpm test" --no-build --no-lint
tb verify run --build-cmd "pnpm build" --lint-cmd "pnpm lint" --test-cmd "pnpm test"
tb verify run --dry-run --test-cmd "go test ./..."
```

Ruler `visual` (docs/verificacao-visual.md): captura headless + diff de pixels;
se Chromium/Chrome ou referências estiverem ausentes, faz skip gracioso.

**Exemplos:**

```bash
# Verificação com ruler de testes
tb verify run
tb verify run --test-cmd "go test ./..." --hidden-test-cmd "go test ./tests/hidden/..."
tb verify run --test-cmd "true" --no-build --no-lint --max-iter 1
tb verify run --dry-run --build-cmd "go build ./..." --test-cmd "go test ./..."

# Verificação visual (threshold é fração 0–1, não porcentagem inteira)
tb verify run --ruler visual --threshold 0.95
tb verify run --ruler visual --manifest .toolbelt/screens.yaml --json -o verify-result.json

# Skip gracioso (default): browser/refs ausentes, screens: [] ou todas as telas skipped
# → relatório top-level skipped=true e veredicto SKIPPED (não PASS)
tb verify run --ruler visual --skip-unavailable
```

**Rulers disponíveis:**

| Ruler | Métrica | Threshold Default | Trapaça Específica |
|-------|---------|-------------------|-------------------|
| `visual` | Similaridade pixel | 0.95 | Edição de refs |
| `tests` | Passagem em testes ocultos | 100% | Modificação de testes |

> Ruler `review`, flag `--discriminator-only` e invocações no estilo `tb verify <task-id>` **não existem** — use `tb verify run` e `tb review` separadamente.

**Template de briefing do discriminador:**

```yaml
# .toolbelt/discriminator-brief.yaml
task: ui-implementation
ruler: visual

context:
  description: "Implementação da tela de login conforme design"
  design_reference: "references/login-v2.png"
  spec_link: "vault/specs/login-redesign.md"

screens:
  - name: login
    modes: [light, dark]
    reference_light: "references/login-light.png"
    reference_dark: "references/login-dark.png"
    masks:
      - region: [100, 200, 50, 20]
        reason: "Timestamp dinâmico"

acceptance:
  min_similarity: 95
  wcag_level: AA
  
cheating_rules:
  - no_reference_edits
  - no_mask_without_reason
  - no_threshold_changes
```

**Formato do relatório:**

```yaml
# verify-report.yaml
task: ui-implementation
ruler: visual
verdict: PASS | FAIL
iterations: 2

per_screen:
  - screen: login
    light_similarity: 97.2
    dark_similarity: 95.8
    size_match: true
    wcag_pass: true
    
  - screen: dashboard
    light_similarity: 96.5
    dark_similarity: 95.1
    size_match: true
    wcag_pass: false
    wcag_violations:
      - element: ".btn-secondary"
        contrast: 3.8
        required: 4.5

gaps:
  - id: G1
    file: "src/components/Button.tsx"
    line: 45
    issue: "Contraste insuficiente no modo escuro"
    proposed_fix: "Usar --text-on-dark ao invés de --text-muted"

accepted_deviations:
  - reason: "Fonte Roboto não disponível, usando system font"
    decision_id: "vault/decisions/font-fallback.md"

cheating_detected: []

metrics:
  total_time: "2m 15s"
  build_time: "45s"
  capture_time: "30s"
  comparison_time: "5s"
```

---

### `tb guard check` — Verificação de Paths Protegidos

Verifica mudanças em paths protegidos entre dois commits.

```bash
tb guard check [base..head] [flags]

Flags:
  --base <ref>          Commit/ref base (default: HEAD~1; ignorado se base..head for passado)
  --head <ref>          Commit/ref head (default: HEAD; ignorado se base..head for passado)
  --warn-only           Exit 0 mesmo com SeverityError (opt-out do default CI-safe)
  --strict              Deprecated: falha em SeverityError já é o default (alias sem efeito; use --warn-only)
  --json                Saída em JSON
```

Aceita o intervalo posicional `base..head` (forma clássica) ou as flags `--base`/`--head`, alinhadas a `tb review` / `tb drift` / `tb verify`.

Por default, exit ≠ 0 quando há `SeverityError` (CI-safe). `--strict` está
**deprecated** (substituído pelo default estrito + `--warn-only`). No modo texto
sem violações imprime `guard: ok (N regras, 0 violações)` (`N` = paths protegidos
válidos carregados: defaults + `.toolbelt/guard.yaml` da **base**; padrões
inválidos são ignorados e não entram em `rules_checked`).

**Exemplos:**

```bash
# Verificar mudanças entre main e branch atual (exit ≠ 0 se SeverityError)
tb guard check main..HEAD

# Mesmo intervalo via flags (como review/drift)
tb guard check --base main --head HEAD

# Defaults HEAD~1..HEAD quando nenhum intervalo é passado
tb guard check --json

# Opt-out: lista violações mas exit 0
tb guard check origin/main..HEAD --warn-only

# JSON + exit ≠ 0 se houver SeverityError (default)
tb guard check main..HEAD --json
```

**Manifesto de paths protegidos (`.toolbelt/guard.yaml`):**

O guard lê `protected_paths` de `.toolbelt/guard.yaml` **na ref base**
(`git show base:.toolbelt/guard.yaml`) e **une** aos defaults embutidos
(`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`,
`.toolbelt/guard.yaml`). Qualquer alteração em `.toolbelt/guard.yaml` no diff
é violação. YAML malformado na base falha **fechado** (exit ≠ 0, erro claro).
Matching de paths é **case-insensitive** e honra globs (`*`, `?`, `**`).
Renames (`R*`) checam origem e destino. A varredura anti-trapaça usa o
conteúdo do diff/`git show head:path` (não o working tree) e cobre Go e
JS/TS (`it.skip` / `describe.skip` / `test.skip` / `xit` / `.only` /
asserções triviais como `expect(true).toBe(true)`). Cada entrada do YAML pode
ser string (`lib/help.js`) ou objeto com `pattern`. Barra final (`secrets/`)
protege o diretório e filhos. Sem o arquivo na base, só os defaults valem.
Violações ganham `source: default` ou `source: custom`. Em `tb verify run`,
só `default` (e custom fora do Scope do `--ticket`) abortam o loop; custom no
mandate é reportado e o verify segue — ver seção verify acima.

```yaml
# .toolbelt/guard.yaml
version: 1

protected_paths:
  # Diretório inteiro (barra final = prefixo; protege filhos)
  - secrets/
  # Glob de um nível
  - pattern: "secrets/*"
    reason: "Segredos não podem ser editados"
  # Path literal
  - lib/help.js

  # Referências visuais
  - pattern: "references/**"
    reason: "Referências visuais não podem ser editadas sem decisão registrada"
    allow_additions: true    # novos arquivos OK
    allow_deletions: false   # deleções requerem decisão
    allow_modifications: false
    
  # Masks
  - pattern: "masks/**"
    reason: "Masks requerem justificativa de região dinâmica"
    require_reason_in_commit: true
    
  # Comparador
  - pattern: "**/comparator.{js,ts,go,py}"
    reason: "Lógica de comparação é imutável"
    allow_modifications: false
    
  - pattern: ".toolbelt/thresholds.yaml"
    reason: "Thresholds não podem ser relaxados sem aprovação"
    detect_relaxation: true  # detecta especificamente valores diminuindo
    
  # Seeds
  - pattern: "**/*.seed"
    reason: "Seeds determinísticos são fixos por cenário"
    
  # Testes de benchmark
  - pattern: "benchmarks/**"
    reason: "Testes de benchmark são read-only"
    allow_modifications: false
    
  # Testes ocultos
  - pattern: "tests/hidden/**"
    reason: "Testes ocultos não podem ser acessados pelo implementador"
    visibility: discriminator_only

decision_sources:
  - "vault/decisions/**"
  - ".toolbelt/approved-changes.yaml"

on_violation:
  ci: fail
  local: warn
```

**Formato de decisão aprovada:**

```yaml
# .toolbelt/approved-changes.yaml
approved_changes:
  - path: "references/login-light.png"
    action: delete
    reason: "Design atualizado para v3, ref antiga obsoleta"
    approved_by: "owner"
    approved_at: "2026-01-15T10:30:00Z"
    ticket: "DESIGN-123"
    
  - path: ".toolbelt/thresholds.yaml"
    action: modify
    field: "min_similarity"
    old_value: 95
    new_value: 93
    reason: "Fonte do sistema tem variação maior que esperado"
    approved_by: "owner"
    approved_at: "2026-01-15T11:00:00Z"
    temporary: true
    expires_at: "2026-02-15"
```

---

### `tb ticket` — Gerenciamento de Tickets (✓ Fase 3)

Cria e gerencia os tickets de trabalho consumidos pelo router. Cada ticket é um
arquivo YAML versionável em `.toolbelt/tickets/TKT-NNN.yaml` (IDs monotônicos,
3+ dígitos). Escritas são atômicas (temp + rename) e serializadas por
`.toolbelt/tickets/.lock`. Todos os subcomandos honram `--json`.

```bash
tb ticket create --title "Criar modelo User" --scope "src/models/**" --accept "go test ./src/models passa" --timeout 10m
tb ticket create --title "Implementar JWT signing" --dep TKT-001 --priority 1
tb ticket create --from rd.md
tb ticket create --title "Refatorar pagamentos" --dry-run
tb ticket list
tb ticket list --status pending,running
tb ticket list --deps TKT-001
tb ticket show TKT-002
tb ticket cancel TKT-002
tb ticket retry TKT-002
```

**Subcomandos e flags:**

| Subcomando | Flags | Descrição |
|------------|-------|-----------|
| `create` | `--title`, `--description`, `--from <rd.md>`, `--scope`, `--forbid`, `--accept`, `--dep`, `--priority <n>`, `--timeout <dur>`, `--dry-run` | Cria ticket (`--title` ou `--from`). `--scope`/`--forbid`/`--dep` aceitam lista ou repetição; `--accept` é repetível |
| `list` | `--status a,b`, `--deps <id>` | Lista tickets; `--deps` mostra só os que dependem do ID |
| `show <id>` | — | Detalhes: status, prioridade, deps, mandato, aceite, resultado, merge |
| `cancel <id>` | — | Cancela ticket `pending`/`running` |
| `retry <id>` | — | Recoloca ticket `failed`/`canceled` em `pending` |

`--from` lê um `rd.md` / arquivo de ticket (o mesmo formato de `tb review` e
`tb drift`: `## Scope`, `## Forbidden` e checklist de aceite). O `tb` não gera
tickets por decomposição de tarefa livre nem fala com o GitHub — use `--title`
ou um `rd.md` escrito por você. IDs de dependência desconhecidos e ciclos são
rejeitados na criação.

**Exemplos de saída:**

```bash
tb ticket create --title "Criar modelo User"
# ticket TKT-001 criado: Criar modelo User
tb ticket list
# TKT-001	pending  	p0	Criar modelo User
# TKT-002	pending  	p1	JWT signing	(deps: TKT-001)
```

**Status e transições:**

| De | Para |
|----|------|
| `pending` | `running`, `canceled` |
| `running` | `done`, `failed`, `pending` (re-tentativa), `canceled` |
| `failed` / `canceled` | `pending` (`tb ticket retry`) |
| `done` | terminal |

**Formato do ticket:**

```yaml
# .toolbelt/tickets/TKT-001.yaml
id: TKT-001
title: Criar modelo User
status: done            # pending | running | done | failed | canceled
priority: 0             # maior primeiro
mandate:
    scope: [src/models/**]
    forbidden: [tests/hidden/**]
    permissions: [read, write, run_tests]
acceptance:
    criteria: ["go test ./src/models passa"]
    timeout: 10m
dependencies: []
attempts: 1
worker: w1
result:                 # preenchido pelo router
    branch: tb-workspace/TKT-001
    base_commit: 356c934e...
    head_commit: a3e62ba7...
    workspace: .toolbelt/workspaces/TKT-001
    exit_code: 0
    log: .toolbelt/router/logs/TKT-001.log
```

Um ticket também pode carregar `subscription` (worker, `direct_delivery`,
`expires_at`) — ver entrega direta em `tb router`. `merged_at` é gravado por
`tb merge`.

---

### `tb board` — Visualização do Board (✓ Fase 3)

Exibe os tickets em seções e controla a atribuição de novos tickets. O estado
de controle (`paused`/`draining`) fica em `.toolbelt/board.yaml`, criado no
primeiro `pause`/`drain`.

```bash
tb board list
tb board list --status pending,failed
tb board list --worker w1 --strategy priority
tb board watch --interval 2s
tb board pause
tb board resume
tb board drain
```

**Subcomandos e flags:**

| Subcomando | Flags | Descrição |
|------------|-------|-----------|
| `list` | `--status a,b`, `--worker <id>`, `--strategy fifo\|priority\|dependency` | Seções RUNNING / READY / BLOCKED / DONE / FAILED / CANCELED |
| `watch` | `--interval <dur>` (padrão 2s), `--status`, `--worker`, `--strategy` | Reimprime o board até Ctrl+C |
| `pause` | — | Nenhuma nova atribuição; o que roda continua |
| `resume` | — | Limpa `pause` e `drain` |
| `drain` | — | Termina o que roda e não atribui novos |

Ordenação das seções (`--strategy`, padrão `dependency`): `fifo` (criação),
`priority` (prioridade desc., depois fifo), `dependency` (profundidade
topológica asc., depois prioridade). READY = `pending` com todas as
dependências `done` e board não pausado; BLOCKED lista o que aguarda.

**Exemplo:**

```bash
tb board list
# board: ativo (estratégia: dependency)
#
# RUNNING (0)
#
# READY (1)
#   TKT-001	pending  	p0	Criar modelo User
#
# BLOCKED (1)
#   TKT-002	JWT signing	(aguardando: TKT-001: pending)
#
# DONE (0)
# FAILED (0)
# CANCELED (0)
```

---

### `tb router` — Router e Workers (✓ Fase 3)

O router é um **scheduler em primeiro plano** (não é daemon: não há
`start`/`stop`). `tb router run` distribui tickets prontos do board entre N
workers (`w1..wN`); cada ticket roda numa worktree própria
(`.toolbelt/workspaces/<ticket>`, branch `tb-workspace/<ticket>`) executando o
comando do agente **dentro do sandbox**. O watchdog roda dentro do mesmo loop.

```bash
tb router run --agent-cmd "meu-agente --task-file $TB_TICKET_FILE"
tb router run --workers 2 --strategy priority --once
tb router run --dry-run
tb router run --allow-unsandboxed
tb router status
tb router config
```

**Subcomandos e flags:**

| Subcomando | Flags | Descrição |
|------------|-------|-----------|
| `run` | `--workers <n>`, `--strategy fifo\|priority\|dependency`, `--agent-cmd "<cmd>"`, `--once`, `--dry-run`, `--allow-unsandboxed` | Executa até esvaziar a fila; Ctrl-C devolve tickets `running` para `pending` |
| `status` | — | Lê `state.json`; detecta estado obsoleto (PID morto) |
| `config` | — | Imprime a configuração efetiva (`router:`) |

- `--dry-run` mostra o plano por rodada (worker, ticket, origem `board`/`direct`, base) sem executar.
- `--once` faz uma rodada de agendamento e sai quando os tickets iniciados terminam.
- Sem Landlock o sandbox é **fail-closed**: o router recusa executar. `--allow-unsandboxed` é opt-in explícito (veja `tb sandbox`).
- O ambiente do filho é filtrado (allowlist; segredos só com `--pass-env`) e recebe `TB_TICKET_ID`, `TB_TICKET_FILE` (caminho absoluto), `TB_WORKER_ID` e `TB_WORKSPACE`.
- Timeout por ticket = `acceptance.timeout` do ticket ou `router.timeouts.per_ticket`.
- Ao sair do agente com mudanças, o router faz `git add -A` e commit `tb: <id> <título>` na worktree e grava `base_commit`/`head_commit` no ticket. Exit 0 → `done`; senão `failed`, ou volta a `pending` enquanto `attempts < router.retry.max_attempts`.
- **Instância única:** só um `tb router run` por repositório, via lock exclusivo `.toolbelt/router/lock` (`flock`; em sistemas não-unix, lockfile `O_EXCL` — se sobrar após um crash, remova o arquivo manualmente).
- **Integridade do worktree:** antes do commit o router verifica que `<ws>/.git` é o arquivo-ponteiro de worktree esperado e roda o git com configuração endurecida (fsmonitor e hooks desabilitados, `--no-verify`, config global/de sistema do usuário desligada — `GIT_CONFIG_GLOBAL=/dev/null` e `GIT_CONFIG_NOSYSTEM=1` —, logo filtros do usuário como git-lfs **não** rodam sobre o conteúdo do agente; `user.name`/`user.email` são lidos uma vez da raiz com a config normal e passados via `-c`, com fallback `tb <tb@localhost>`). O `git worktree add/remove` do router também roda sem hooks (`core.hooksPath=/dev/null`), fsmonitor e manutenção destacada: worktrees criados pelo router nunca executam hooks `post-checkout` do usuário. Violação ⇒ ticket `failed` sem retry (`integridade do worktree violada`).
- **`.git` aninhado, gitlinks e `.gitmodules`:** o git do router roda fora do sandbox, então um repositório/submódulo criado pelo agente (p.ex. `sub/.git` com `filter.*.clean` dentro de um gitlink do commit base) nunca é tocado: antes de qualquer git o router percorre o worktree (sem seguir symlinks) e recusa qualquer entrada `.git` além do ponteiro raiz; depois do `git add -A` recusa commits que adicionem/alterem gitlinks (modo 160000) ou `.gitmodules`. Em ambos os casos o ticket vira `failed` sem retry (`integridade do worktree violada`). O git do router também roda com `--ignore-submodules=all` e submódulos desabilitados (`diff.ignoreSubmodules`, `submodule.recurse`, `fetch.recurseSubmodules`).
- **Processos que se desacoplam (Linux):** o router vira *subreaper* (`PR_SET_CHILD_SUBREAPER`) e rastreia os pids que ele próprio inicia (agentes e git). Após cada agente e a cada tick do watchdog, todo filho direto não rastreado — descendente do agente que fez `setsid`/`setpgid` e escapou do grupo de processos — é morto (`SIGKILL`) e colhido. Política: agentes **não devem se desacoplar**; qualquer processo escapado encontrado é morto. Se isso ocorre durante um ticket, o alerta `agent_escaped` (error) é emitido e o ticket falha **sem retry e sem rodar git no worktree**. A atribuição é pelo `TB_TICKET_ID` do ambiente do processo escapado (lido só essa variável de `/proc/<pid>/environ`): só o ticket dono falha; os demais seguem. Se o `TB_TICKET_ID` está ausente (`env -u`), ilegível ou é desconhecido, o escape é **não atribuível** e, de forma conservadora, **todos** os tickets com agente rodando no momento da detecção falham (alerta `agent_escaped` sem ticket, listando-os). Um órfão achado dentro do grupo de processos de um agente cujo `TB_TICKET_ID` difere do dono do grupo (`setpgid` para o pgid de outro agente para se esconder) também é escape. Daemons que nascem durante a criação do worktree (janela do `git worktree add`) sem `TB_TICKET_ID` são mortos com o alerta `orphan_killed` (warn) e nenhum ticket falha. Limitação: um agente que forja o `TB_TICKET_ID` de outro ticket ativo faz o escape ser atribuído a esse ticket. Fora do Linux (macOS/Windows) não há subreaper: vale só o grupo de processos (um descendente que se desacopla não é detectado nem morto — **limitação**). `tb sandbox run` não muda.
- **Mandato não é imposto:** `Scope`, `Forbidden` e `Permissions` do mandato do ticket são **registrados**, mas o router não os impõe — as edições do agente são commitadas como estão. Rode `tb drift`, `tb review` e `tb guard` (ou `tb stack --ticket`) sobre o resultado **antes** de `tb merge`.
- **Controle obsoleto:** pedidos `control/kill-*.json` são apagados ao iniciar `tb router run`, e pedidos com `requested_at` anterior ao início da execução são ignorados.
- **Board ilegível:** um YAML de ticket transitoriamente inválido não derruba o router: emite `board_unreadable` (warn), mantém os workers e tenta de novo no próximo tick; após 30 ticks consecutivos falhando o router encerra com `stalled` (exit ≠ 0).
- **Cancelar e reenviar:** um ticket só é atribuído de novo depois que o slot que ainda o segura terminar de encerrar o agente anterior.
- **Saída do agente:** exit 0 que deixa processos filhos em background conta como sucesso; o grupo de processos é morto após a saída. A saída mantida/logada é limitada aos últimos 1 MiB; logs com permissão 0600. Exit 0 prevalece sobre um veredito tardio de stuck/kill.
- **Resumo `stalled`:** se restam tickets `pending` bloqueados por dependências `failed`/`canceled` (lista `unschedulable`), o motivo do resumo é `stalled` e `tb router run` sai com código ≠ 0.

**Entrega direta (direct delivery):** quando um ticket termina `done`, cada
ticket `pending` que depende dele recebe uma *subscription* para aquele worker
(TTL `subscription_ttl`, no máximo `max_subscriptions_per_worker` por worker).
O worker prefere tickets assinados por ele; se o ticket assinado tem
exatamente uma dependência com `head_commit`, a worktree parte desse commit
(afinidade de contexto). Ticket com uma única dependência parte sempre do
`head_commit` dela, com ou sem entrega direta; com ≥ 2 dependências parte de
`HEAD` (alerta `multi_dep_base` se alguma não foi mergeada). Assinaturas expiradas são ignoradas, e se o worker
falha as assinaturas dele são limpas e o ticket volta ao board.

**Exemplo:**

```bash
tb router run --agent-cmd "sh -c 'echo x > f-$TB_TICKET_ID.txt'"
# router iniciado: 3 workers, estratégia dependency
# [w1] TKT-001 iniciado (board, base 356c934e, tentativa 1)
# [w1] TKT-001 concluído (a3e62ba7)
# [w1] TKT-002 iniciado (direct, base a3e62ba7, tentativa 1)
# [w1] TKT-002 concluído (a6ebf42e)
# router encerrado (completed)
```

**Configuração** (`.toolbelt/config.yaml`; não existe `router.yaml`):

```yaml
router:
  max_workers: 3
  strategy: dependency        # fifo | priority | dependency
  agent_cmd: []               # argv do agente (obrigatório, ou --agent-cmd)
  direct_delivery:
    enabled: true
    subscription_ttl: 30m
    max_subscriptions_per_worker: 5
  timeouts:
    per_ticket: 10m
    global: 1h
  retry:
    max_attempts: 2
  slow_threshold: 0.8         # fração do timeout para worker_slow
```

**Arquivos de runtime** (`.toolbelt/router/`, no `.gitignore`):

| Caminho | Conteúdo |
|---------|----------|
| `state.json` | PID, estratégia, workers (idle/working, ticket, assinaturas) e fila (ready/blocked/running); atualizado a cada mudança |
| `alerts.jsonl` | Alertas do watchdog, um JSON por linha (criado no primeiro alerta) |
| `control/` | Pedidos para o router em execução (ex.: `kill-w1.json`) |
| `logs/TKT-NNN.log` | stdout/stderr do agente por ticket |

---

### `tb watchdog` — Monitoramento de Workers (✓ Fase 3)

O watchdog **roda dentro do loop do router** (tick de 1s). O comando
`tb watchdog` só **lê** o estado persistido (`.toolbelt/router/`) e envia
pedidos de controle; não há `scale` (o router não escala sozinho).

```bash
tb watchdog status
tb watchdog status --worker w1
tb watchdog alerts --since 10m --kind worker_stuck --limit 20
tb watchdog kill w1 --reason "travado em loop"
```

| Subcomando | Flags | Descrição |
|------------|-------|-----------|
| `status` | `--worker <id>` | Estado dos workers e contagem de alertas por tipo |
| `alerts` | `--since <10m\|2h\|RFC3339>`, `--kind <tipo>`, `--limit <n>` | Lista `alerts.jsonl` |
| `kill <worker>` | `--reason` | Grava `control/kill-<worker>.json`; o router cancela o ticket em execução (conta como tentativa). Erro se não há router em execução |

**Alertas:**

| Alerta | Severidade | Condição | Ação |
|--------|-----------|----------|------|
| `worker_slow` | warn | elapsed ≥ `slow_threshold` × timeout (uma vez) | `none` |
| `worker_stuck` | critical | elapsed ≥ timeout do ticket; cancela o worker | `retry` (há tentativas) ou `escalate` |
| `worker_crash` | error (critical ao escalar) | agente sai com código ≠ 0 | `retry` ou `escalate` |
| `worker_killed` | warn | `tb watchdog kill` | `retry` ou `escalate` |
| `queue_growing` | warn | prontos > 2 × workers (uma vez por transição) | `none` (apenas sugere) |
| `global_timeout` | critical | `router.timeouts.global` atingido; router encerra | `stop` |
| `ticket_canceled` | warn | ticket cancelado (`tb ticket cancel`) enquanto roda; o watchdog mata o agente, o resultado é descartado e o ticket permanece cancelado | `none` |
| `multi_dep_base` | warn | ticket com ≥ 2 dependências nem todas mergeadas é baseado em `HEAD`; rode `tb merge` nas dependências antes | `none` |
| `agent_escaped` | error | descendente do agente escapou do grupo de processos (`setsid`/`setpgid`); foi morto e o ticket dono (por `TB_TICKET_ID`; não atribuível ⇒ todos os rodando) falha sem retry nem git (Linux) | `escalate` |
| `orphan_killed` | warn | órfão não-agente encerrado sem falhar ticket (daemon nascido na criação do worktree sem `TB_TICKET_ID`, ou órfão sem nenhum agente rodando) | `none` |
| `board_unreadable` | warn | um ticket YAML está inválido; workers mantidos, nova tentativa a cada tick | `none` |

**Exemplo:**

```bash
tb watchdog alerts --kind worker_stuck
# 11:57:56  critical worker_stuck   w1 TKT-001     TKT-001 em w1 excedeu o timeout de 2s; cancelando [ação: retry]
# 11:57:59  critical worker_stuck   w1 TKT-001     TKT-001 em w1 excedeu o timeout de 2s; cancelando [ação: escalate]
```

---

### `tb merge` — Agregação de Resultados (✓ Fase 3)

Integra os commits dos tickets `done` (`base_commit..head_commit`) numa
worktree temporária destacada e só então avança o branch alvo.

```bash
tb merge TKT-001 TKT-002
tb merge --dry-run TKT-001 TKT-002
tb merge --strategy last_wins TKT-001 TKT-002
tb merge --into integracao TKT-001 TKT-002
tb merge --output changes.patch TKT-001 TKT-002
tb merge --force TKT-001
tb merge --allow-sensitive TKT-001
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--strategy fail\|last_wins` | Conflito (padrão `fail`) |
| `--dry-run` | Integra no worktree temporário, reporta e descarta |
| `--into <branch>` | Branch alvo (padrão: branch atual) |
| `--output <arquivo.patch>` | Escreve o diff `alvo..integrado` sem mover refs |
| `--force` | Permite refazer merge de tickets já mergeados |
| `--allow-sensitive` | Permite aplicar tickets que alteram caminhos sensíveis (ver abaixo) |
| `--json` | Saída JSON |

Ordem de aplicação: dependências primeiro, depois a ordem dos argumentos. Cada
ticket precisa estar `done` com `head_commit`.

**Estratégias de conflito:**

| Estratégia | Comportamento |
|------------|---------------|
| `fail` (padrão) | Aborta no primeiro conflito, lista arquivos/tickets e **não altera nenhuma ref** |
| `last_wins` | No arquivo em conflito, mantém o arquivo **inteiro** do ticket mais recente (não mescla hunks) e registra a resolução |
| `prompt` / `semantic` | **Não implementadas** (erro explícito) |

**Atomicidade:** o merge nunca toca o branch alvo durante a integração. O
avanço usa compare-and-swap (`git update-ref refs/heads/X novo antigo`) e só
ocorre se o alvo ainda está no SHA inicial; se o branch alvo está
checked-out e sujo, o merge é recusado. A worktree temporária é sempre removida.
Em sucesso, os tickets recebem `merged_at`.

**Gate de confiança:** todo `git` do merge roda endurecido (`core.hooksPath=/dev/null`,
`core.fsmonitor=false`, `diff.external=`, `core.pager=cat`, `commit.gpgsign=false`,
`maintenance.auto=false`, `gc.auto=0`, `submodule.recurse=false`), então hooks
escritos por agentes (ex.: `post-merge` em `.githooks/`) não executam no
fast-forward nem nos worktrees temporários. Além disso, ao **aplicar**, o merge é
recusado (sem alterar refs; lista ticket + caminho + motivo) se algum ticket
alterar caminhos sensíveis: o diretório de hooks (`core.hooksPath`, `.husky/`,
`.githooks/`), `.toolbelt/**`, `.github/workflows/**`, `.gitmodules`, gitlinks
(modo 160000), `.gitattributes` em qualquer nível e os paths protegidos do
`guard`. Revise e use `--allow-sensitive` (`--force` continua só para
re-merge). `--dry-run`/`--output` apenas reportam; o JSON traz
`sensitive: [{ticket, path, reason}]`.

**Detalhes de `last_wins`:** os commits de integração rodam com `gpgsign` e
hooks desabilitados; se o `--continue` falha, o merge é abortado sem alterar
refs; a detecção de sobreposição usa `--no-renames` (renomeações contam para
os dois caminhos). Na ferramenta MCP `tb_merge`, o relatório de conflito é
devolvido com `ok:false`.

**Exemplos de saída:**

```bash
tb merge --dry-run TKT-001 TKT-002
# merge TKT-001 TKT-002 -> main (fail) [dry-run]
# ARQUIVO        TICKETS  STATUS
# f-TKT-001.txt  TKT-001  merge
# f-TKT-002.txt  TKT-002  merge
# resultado: a6ebf42ee47be5c4ca290b2055efe75b00f8eaf6 (início 356c934e)
tb merge TKT-002 TKT-003
# merge TKT-002 TKT-003 -> main (fail) [não aplicado]
# ARQUIVO  TICKETS          STATUS
# a.txt    TKT-002,TKT-003  conflict
# Error: conflito de merge: 1 arquivo(s) em conflito; nenhuma ref foi alterada (use --strategy last_wins para resolver)
```

---

### `tb bus` — Barramento entre Agentes (✓ B1, ADR-013)

Barramento local para agentes de vendors diferentes (Claude Code, Codex, Grok, ...) trocarem mensagens `bus/0.1` pelo mesmo store (`.toolbelt/bus/bus.db`, SQLite WAL, sem daemon). Spec do envelope, Agent Card e convite: [bus-protocolo.md](bus-protocolo.md). Lease/heartbeat (B2) ainda não existe; o harness `tb bus run` (B3) está em [bus-harness.md](bus-harness.md).

```bash
tb bus channel create feature-x --owner claude-main --ttl 2h
tb bus invite feature-x --role worker
tb bus invite feature-x --role supervisor --name claude-main
tb bus join feature-x --invite K7Q2-9XMD --name grok-1 --vendor xai --role worker
tb bus send --channel feature-x --as claude-main --to role:worker --type task.assign --thread TKT-001 --payload '{"ticket":"TKT-001"}'
tb bus recv --channel feature-x --as grok-1 --wait 20s
tb bus claim TKT-001 --channel feature-x --as grok-1
tb bus complete TKT-001 --channel feature-x --as grok-1 --result '{"verdict":"PASS"}'
tb bus log feature-x --thread TKT-001
tb bus channel list
tb bus channel show feature-x
tb bus channel extend feature-x --ttl 2h
tb bus run --channel feature-x --as marin --check
tb bus run --channel feature-x --as marin
tb bus run --channel feature-x --as claude-code --once
```

| Comando | Quem | Função |
|---|---|---|
| `channel create <canal> --owner N` | humano | cria canal (`--ttl`, padrão sem expiração) e convite do dono (papel `supervisor`) |
| `channel extend <canal> --ttl D` | humano | redefine a expiração para agora + `D` (`0` = sem expiração); reabre canal expirado mantendo sessões e histórico |
| `invite <canal> --role R` | humano | convite de uso único com papel fixado (`--ttl`, padrão `10m`); `--name` vincula a um agente e, se ele já estiver no canal, recupera a sessão perdida (token novo) |
| `join <canal> --invite C` | agente | entra com Agent Card (`--name`, `--vendor`, `--model`, `--tier`, `--wake`, `--session`, `--capability` ou `--card card.json`); grava a sessão no diretório privado do bus, fora do repositório (`$XDG_STATE_HOME/toolbelt/bus/`, `0600`) |
| `send` | agente | `--to agent:\|role:\|topic:`, `--type`, `--thread`, `--causation`, `--idempotency-key`, `--ttl`, `--payload` (JSON ou `@arquivo`) |
| `recv` | agente | lê e avança o cursor; `--wait` (máx. 25s), `--limit`, `--peek`, `--since` |
| `claim <thread>` | agente | dono único da thread |
| `complete <thread>` | dono | `--result`; publica `task.result` em `topic:board` |
| `log <canal>` | humano | trilha de auditoria (payload redigido); `--thread`, `--limit` |
| `run --channel C --as N` | humano | harness B3: espera sem LLM e acorda o agente pelo arquivo de wake (`exec` ou `harness`); `--once`, `--check`, `--dry-run`, `--invite`, `--config`, `--allow-unsandboxed`. Spec: [bus-harness.md](bus-harness.md) |

Comandos de agente exigem `--channel` e `--as`. O token de sessão nunca é impresso. `--json` devolve envelopes JSON-RPC.

---

### `tb bench` — Benchmark ✅ IMPLEMENTADO (A/B na Fase 2)

Executa benchmarks e mede KPIs. Suporta comparação A/B entre configs/feature flags.

```bash
tb bench <subcommand>

Subcomandos:
  run       Executa suite de benchmark ✅
  compare   Compara resultados ou configs A/B ✅
  report    Gera relatório ✅
  list      Lista tarefas disponíveis ✅
```

**`tb bench run`:**

```bash
tb bench run [flags]

Flags:
  --fake               Usa agente fake embutido no binário `tb` (portátil, qualquer repo)
  --fake-config <path> Arquivo de configuração do agente fake
  -a, --agent <cmd>    Comando do agente externo (fallback: bench.default_agent)
  -t, --tasks-dir      Diretório das tarefas (fallback: bench.tasks_dir)
  -f, --fixtures-dir   Diretório das fixtures (fallback: bench.fixtures_dir)
  -r, --repetitions <n> Número de repetições (default: 1)
  -o, --output <path>  Arquivo de saída JSONL (sem flag: sob bench.output_dir)
  --label <name>       Rótulo da variante A/B
  --flag key=value     Feature flag (repetível)
  --ab-config <yaml>   Variante A/B a partir de YAML
  --task <id>          Filtra tarefas
```

Precedência dos paths e do agente: **flag CLI explícita** > **`config.yaml`** > **default**.
A flag global `-c/--config` alimenta `config.Load()` (inclui `bench.*`).

Se `--agent` e `--fake` forem omitidos, `tb bench run` usa `bench.default_agent`
do YAML carregado (quando definido). Sem `-t`/`-f`/`-o`, usa
`bench.tasks_dir` / `bench.fixtures_dir` / `bench.output_dir`.

```yaml
# .toolbelt/config.yaml
bench:
  default_agent: "cursor-agent {prompt} --workdir {workdir}"
  tasks_dir: /abs/path/to/bench/tasks
  fixtures_dir: /abs/path/to/bench/fixtures
  output_dir: /abs/path/to/bench/results
```

Placeholders `{prompt}`, `{workdir}` e `{timeout}` são interpolados com
quoting POSIX (aspas simples), evitando injeção via aspas, `$()` ou `;`.
O harness também exporta `TB_BENCH_PROMPT`, `TB_BENCH_WORKDIR` e
`TB_BENCH_TIMEOUT` para agentes que preferem ler env em vez do argv.

Uma tarefa só marca `passed: true` quando o **agente** sai com exit 0 **e** a
verificação atinge `expected_exit_code`. Exit do agente ≠ 0 (ex.: `false`)
força `passed: false` mesmo se a fixture passar. Se o comando do agente não
existir (exit 127), `tb bench run` grava o JSONL e sai com código ≠ 0.

**`tb bench compare` (A/B):**

```bash
# Compara dois JSONL já gerados
tb bench compare --before graph-off.jsonl --after graph-on.jsonl

# Roda duas configs no mesmo conjunto de tarefas e compara
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml --fake

# Mesmo fluxo com agente externo (--agent ou bench.default_agent)
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml \
                 --agent "cursor-agent {prompt} --workdir {workdir}"
```

O relatório Markdown e o JSONL incluem métricas de **sucesso (pass@1)**, **tempo** e **tokens**, com deltas A→B.

**Exemplos:**

```bash
tb bench run --fake --label graph-off --flag graph=off
tb bench run --fake --ab-config bench/ab/graph-on.yaml
tb bench run -r 1   # usa bench.default_agent / tasks_dir do config
tb -c /tmp/alt-config.yaml bench run -r 1   # -c alimenta config.Load()
tb bench compare --before bench/results/graph-off.jsonl \
                 --after bench/results/graph-on.jsonl
tb bench list
tb bench report --input bench/results/graph-on.jsonl
```

---

### `tb mcp` — Servidor MCP (✓ implementado)

Inicia o toolbelt como servidor MCP sobre **stdio** (JSON-RPC 2.0). Não há `tb serve` na CLI atual.

```bash
tb mcp
```

Transporte unix/tcp e flags `--socket`/`--port`/`--tls-*` são planejados; hoje só stdio.

**Ferramentas expostas em `tools/list` / `tools/call`:**

| Tool | Paridade CLI | Notas |
|------|--------------|-------|
| `tb_version` | `tb version` | |
| `tb_echo` | — | smoke |
| `tb_env` | — | cwd/os/arch |
| `tb_run` | — | `sh -c` com timeout; env filtrado; args `env`/`pass_env` (arrays) |
| `tb_mem_add` / `tb_mem_list` / `tb_mem_show` / `tb_mem_edit` / `tb_mem_retire` / `tb_mem_search` / `tb_mem_check` / `tb_mem_init` | `tb mem …` | arg opcional `cwd`; `ref`/`all` (boolean; strings legadas ok) onde aplicável |
| `tb_verify_run` | `tb verify run` | `ruler=tests\|visual`; args espelham flags + `env`/`pass_env` |
| `tb_review` | `tb review` | ticket/base/head/approved_sha/approved_base/require_approved_sha/reviewers/standards |
| `tb_drift` | `tb drift` | retorna `{drift, has_drift, pinned_approval}` (`checked` explícito); `approved_sha`/`approved_base` |
| `tb_guard_check` | `tb guard check` | `base`/`head` ou `range=A..B`; `strict_fail` por default em SeverityError; `warn_only=true` opt-out; `strict` deprecated (boolean) |
| `tb_stack` | `tb stack` | `ops` (array), `base`/`head`, `fail_fast` (boolean), `jobs` (integer), `approved_sha`/`approved_base`/`require_approved_sha`, `env`/`pass_env`; retorna JSON agregado |
| `tb_ticket_list` / `tb_ticket_show` | `tb ticket list` / `tb ticket show` | `status`, `deps` / `id`; arg opcional `cwd` |
| `tb_ticket_create` | `tb ticket create` | `title`, `description`, `scope`/`forbid`/`accept`/`dep` (arrays), `priority` (integer), `timeout`, `dry_run` (boolean) |
| `tb_board` | `tb board` | `action=list\|pause\|resume\|drain`, `strategy`, `status`, `worker` |
| `tb_router_status` | `tb router status` | só leitura de `state.json` |
| `tb_watchdog_alerts` | `tb watchdog alerts` | `since`, `kind`, `limit` (integer) |
| `tb_merge` | `tb merge` | `ids` (array), `strategy`, `into`, `force`, `allow_sensitive` (padrão `false`); **dry-run por padrão** — só altera refs com `apply=true` |
| `tb_bus_join` / `tb_bus_send` / `tb_bus_recv` / `tb_bus_claim` / `tb_bus_complete` | `tb bus …` | identidade presa à conexão após `tb_bus_join` (`channel`, `invite`, `name` + Agent Card); `payload`/`result` como objeto; `wait` string (máx. 25s); criar canal/convite **não** existe via MCP; `tb mcp --bus-as <canal>/<nome>` liga a conexão a uma sessão CLI (usado por `tb bus run`) |
| `tb_graph_search` / `tb_graph_callers` / `tb_graph_deps` / `tb_graph_impact` / `tb_graph_index` | `tb graph …` | chamada requer `TB_GRAPH=on`; `depth`/`limit` integer; `transitive` boolean |

Schemas tipam `depth`/`limit`/`reviewers`/`max_iter` como integer, `strict`/`warn_only`/`transitive`/`skip_unavailable`/`all` (mem) como boolean, `threshold` como number e `standards` como array de strings; valores string legados ainda são aceitos. Relatórios de verify/review/drift/guard voltam como JSON no `content` (sem código de saída de processo). Listas vazias usam `[]`; `hidden_passed` é `null` se a suíte oculta não se aplica.

---

### `tb init` ✅ IMPLEMENTADO

```bash
tb init [flags]

Flags:
  --global         Inicializa em ~/.config/toolbelt
  --write-agents   Semeia AGENTS.md mínimo se ausente (nunca sobrescreve)
  --json           Saída em JSON (flag global)
```

Com `--json`, emite payload com `path`, `test_cmd` / `build_cmd` / `lint_cmd` detectados
(quando presentes), `warning` opcional e `agents` (`seeded`, `sources`, …).

Detalhes de detecção de toolchain e preenchimento de `verify.*` estão na seção `tb verify`.

**Estado local vs config (`.git/info/exclude`):** em repositório Git (sem `--global`),
o init escreve em `.git/info/exclude` (não altera o `.gitignore` do projeto) os
padrões de **cache/derivados**:

- `.toolbelt/graph.json`
- `.toolbelt/logs/`
- `.toolbelt/vault/`
- `.toolbelt/workspaces/`
- `.toolbelt/captures/`
- `.toolbelt/bench/`

Assim `git stash -u` / `git clean` não apagam o índice/grafo por engano, sem
bloquear o versionamento de config.

**Recomendação — versionar invariantes:** `invariants.yaml` (e, quando fizer
sentido para o time, `config.yaml` / `guard.yaml` / `thresholds.yaml`) deve
permanecer **versionável**. São política compartilhada (ADR-010), não cache.
O init **não** coloca `.toolbelt/` inteiro no exclude justamente para separar
cache de config.

---

### `tb sandbox` ✅ IMPLEMENTADO

Isolamento Landlock no Linux (`check` / `run`). Não cria worktrees — use `tb workspace` para isso.

```bash
tb sandbox check
tb sandbox run <command> [args...] [flags]
```

**Flags de `tb sandbox run`:**

| Flag | Default | Descrição |
|------|---------|-----------|
| `--workdir <path>` | cwd | Diretório de trabalho (escrita permitida) |
| `--timeout <dur>` | `30s` | Timeout do processo |
| `--allow-unsandboxed` | `false` | Opt-in explícito para rodar **sem** Landlock |

Comportamento **fail-closed**: se o kernel não tiver Landlock (ou a aplicação do ruleset falhar), `tb sandbox run` **recusa** executar e devolve erro claro pedindo `--allow-unsandboxed`. Não há fallback silencioso para execução sem isolamento.

No Linux com Landlock, a escrita fica restrita ao `--workdir` (e a paths liberados explicitamente); o diretório temporário do sistema (`os.TempDir()`, tipicamente `/tmp`) fica sem acesso (nem leitura) fora do workdir — para arquivos temporários, aponte `TMPDIR` para dentro do workdir. Paths de sistema necessários à execução (`/bin`, `/usr`, `/lib`, `/etc`, `/dev`, `/proc`, …) entram somente leitura. Exceção mínima em `/dev`, por arquivo: `/dev/null`, `/dev/zero`, `/dev/full` com leitura+escrita (e truncate na ABI ≥ 3), `/dev/random` e `/dev/urandom` só leitura; nós ausentes são ignorados. Terminais (`/dev/tty`, `/dev/ptmx`, `/dev/pts/*`) ficam bloqueados de propósito (injeção de escape/TIOCSTI em outros terminais). O resto de `/dev` (incl. `/dev/shm`) continua somente leitura. O ruleset é aplicado no processo filho (re-exec + `PR_SET_NO_NEW_PRIVS` + `landlock_restrict_self`) **antes** do `exec` do comando.

```bash
tb sandbox check
# Landlock: true
# ABI: 6

tb sandbox run --workdir /tmp/job sh -c 'echo ok > out.txt'
# escrita fora do workdir → Permission denied

# Sem Landlock (ou para CI sem LSM): opt-in explícito
tb sandbox run --allow-unsandboxed echo hello
```

Com `--json`, `sandbox check` emite `os`/`arch`/`landlock`/`version`/`error`; `sandbox run` emite o `Result` em snake_case (`exit_code`, `timed_out`, `duration_ns`, `sandboxed`, …). Se o filho sai ≠ 0 sem timeout, `tb` propaga esse exit code.

---

### `tb workspace` ✅ IMPLEMENTADO

Workspaces isolados via git worktree. Alias: `tb ws`.

```bash
tb workspace create <id> [--base <ref>] [--allow <path>] [--forbid <path>]
tb workspace list
tb workspace remove <id>
```

`--base` default `HEAD`. `--allow`/`--forbid` são repetíveis (`stringSlice`). JSON de create/list usa snake_case.

---

### `tb version` / `tb --version` ✅ IMPLEMENTADO

```bash
tb version [flags]
tb --version
```

Flags (subcommand `version`):
  --json    Saída em JSON ✅

A flag global `--version` imprime a mesma saída textual de `tb version`
(versão, commit, build, go, os/arch).

**Exemplos:**

```bash
tb version
tb --version
# toolbelt dev
#   commit: unknown
#   built:  unknown
#   go:     go1.22.2
#   os:     linux/amd64

tb version --json
# {"version":"dev","commit":"unknown","build_time":"unknown","go_version":"go1.22.2"}
```

Todas as saídas `--json` da CLI (e os payloads de domínio embutidos nas tools MCP)
usam chaves **`snake_case`** de forma consistente (`exit_code`, `base_ref`,
`timed_out`, …). Envelopes do protocolo MCP (`isError`, `inputSchema`, …)
permanecem no camelCase exigido pela spec.

---

## Cores e Tema Visual

A saída colorida do CLI segue o tema **Synthwave** definido em [docs/identidade-visual.md](identidade-visual.md). As cores são aplicadas automaticamente quando o terminal suporta.

| Elemento | Cor | Código ANSI 256 |
|----------|-----|-----------------|
| Headers, comandos | Roxo `#982BD4` | 134 |
| Valores, paths, URLs | Ciano `#0CD7FF` | 51 |
| Erros, falhas | Rosa `#F9278C` | 198 |
| Sucesso | Verde `#2DD4A7` | 43 |
| Avisos | Amarelo `#FFB020` | 214 |
| Texto secundário | Cinza `#A099B0` | 249 |

**Variáveis de controle:**

| Variável | Descrição | Default |
|----------|-----------|---------|
| `NO_COLOR` | Desabilita cores (padrão freedesktop) | - |
| `TERM=dumb` | Desabilita cores | - |

`TB_COLOR` / `TB_COLOR_MODE` são planejadas e ainda não são lidas pela CLI.
Consulte `assets/theme/tokens.json` para a tabela completa de mapeamento ANSI/256/TrueColor.

---

## Variáveis de Ambiente

| Variável | Descrição | Default |
|----------|-----------|---------|
| `TB_GRAPH` | Habilita grafo sob demanda (`on`/`1`/`true`) | desligado |
| `NO_COLOR` | Desabilita cores (padrão freedesktop) | - |
| `TB_BROWSER` | Browser para ruler visual do verify | (auto) |
| `TB_HOME` | Ecoado em MCP; não define o config dir da CLI | - |
| `TB_BENCH_PROMPT` / `TB_BENCH_WORKDIR` / `TB_BENCH_TIMEOUT` | Exportados pelo harness `tb bench run` | - |

Variáveis planejadas (ainda não lidas pela CLI): `TB_VAULT`, `TB_LOG_LEVEL`,
`TB_SANDBOX_DEFAULT`, `TB_JSON_OUTPUT`, `TB_TIMEOUT_DEFAULT`, `TB_COLOR`,
`TB_COLOR_MODE`.

### Allowlist de ambiente nos processos filhos

`tb verify` / `tb stack` (etapa verify), MCP `tb_run` / `tb_verify_run` / `tb_stack`
não repassam o ambiente completo ao `sh -c`. Só passam chaves da allowlist embutida
(PATH, HOME, LANG, TMPDIR, toolchain Go/Node/npm/pnpm, proxies, XDG, etc.), mais:

| Fonte | Efeito |
|-------|--------|
| `.toolbelt/config.yaml` → `env.allow` | Estende a allowlist |
| `.toolbelt/config.yaml` → `env.pass` | Opt-in explícito de segredos |
| Flag global `--env KEY` / arg MCP `env` | Estende a allowlist (não contorna segredos) |
| Flag global `--pass-env KEY` / arg MCP `pass_env` | Opt-in explícito de segredos |

Segredos: qualquer nome contendo `TOKEN`, `SECRET`, `KEY` ou `PASSWORD`
(case-insensitive) é bloqueado sem opt-in.

---

## Erros e Usage

- Erros esperados de domínio/runtime (`RunE`, ex.: flag de negócio obrigatória ausente, falha de git, arquivo inexistente) saem com **exit ≠ 0** e mensagem em stderr **sem** dump de `Usage` do Cobra.
- Erros genuínos de parse de **flags** ou **args** (flag desconhecida, `ExactArgs`/`MinimumNArgs`, etc.) mantêm `Usage` no stderr para orientar a invocação.
- Subcomando desconhecido em qualquer nível (ex.: `tb mem nao-existe`, `tb bench nope`) sai com **exit ≠ 0** e erro `unknown command "…"`, em vez de imprimir help com exit 0.
- Falhas de git em `guard` / `review` / `drift` / `verify` (e no `stack` que os empilha) **não** mostram só `exit status 128/129`: a mensagem inclui o stderr do git, a ref ausente (`ref ausente: …`) e uma dica `git fetch origin <ref>`.
- `--approved-sha` igual à base (intervalo `base..approved` vazio) não despeja o usage do `git range-diff`; sai com mensagem clara sobre o intervalo inválido.

## Códigos de Saída

A CLI atual não usa uma tabela estável de códigos 2–7. Na prática:

| Código | Significado |
|--------|-------------|
| 0 | Sucesso (`PASS`/`SKIPPED` no verify; guard sem `SeverityError` ou com `--warn-only`) |
| ≠0 (tipicamente 1) | Erro de parse/domínio, `FAIL`/`ESCALATE` no verify, `SeverityError` no guard, subcomando desconhecido |
| 127 | `tb verify run`: `test_cmd` / `build_cmd` / `lint_cmd` / `hidden_test_cmd` não encontrado (ou não executável / `ErrNotFound`); com `--json` emite o envelope `verify.Result` com campo `error` |
| exit do filho | `tb sandbox run` propaga o exit code do processo quando ≠ 0 e sem timeout |

Quando o comando configurado no verify não existe/não é executável, o verify falha imediatamente (sem retries) e o processo `tb` sai **127**.

---

## Configuração

Arquivo de configuração local em `.toolbelt/config.yaml` (global:
`~/.config/toolbelt/config.yaml`). A flag `-c/--config` aponta para um YAML
alternativo.

```yaml
# .toolbelt/config.yaml (exemplo)
verify:
  test_cmd: "go test ./..."
env:
  allow: [APP_ENV, CUSTOM_OK]
  pass: [GITHUB_TOKEN]
bench:
  default_agent: "cursor-agent {prompt} --workdir {workdir}"
  tasks_dir: bench/tasks
  fixtures_dir: bench/fixtures
  output_dir: bench/results
graph:
  languages: [go, javascript, python]
  docs: [markdown]
  frameworks: [react]
  max_file_bytes: 524288
```
