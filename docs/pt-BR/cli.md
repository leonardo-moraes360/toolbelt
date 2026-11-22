[🇺🇸 English](../cli.md) | 🇧🇷 Português

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
tb  (implemented today)
├── init         Initializes local toolbelt configuration
├── bench        Benchmark harness (run|list|report|compare)
├── drift        Detects scope and base commit drift
├── egm          Design-document review loop (review|loop next|resolve|status)  ✓ Phase A
├── graph        On-demand code graph (TB_GRAPH=on or --enable)
├── guard        Anti-cheat protections (check)
├── mcp          Starts an MCP server over stdio
├── mem          Invariants (add|list|show|edit|retire|search|check|init)
├── review       Fresh-context review (Elephant-Goldfish)
├── sandbox      Execution with Landlock isolation (check|run)
├── stack        Stacks verification operations (verify/review/drift/guard/vault/egm/seccheck), not branches
├── vault        Markdown vault of notes (new|list|show|search|links|backlinks|check)
├── verify       Adversarial verification (run; rulers tests|visual)
├── workspace    Isolated workspaces via git worktree (create|list|remove; alias: ws)
├── version      Shows the toolbelt version (also: tb --version)
├── ticket       Work tickets (create|list|show|cancel|retry)       ✓ Phase 3
├── board        Ticket board (list|watch|pause|resume|drain)          ✓ Phase 3
├── router       Foreground scheduler (run|status|config)           ✓ Phase 3
├── watchdog     Worker alerts and control (status|alerts|kill)       ✓ Phase 3
├── merge        Merges done tickets (fail|last_wins|semantic|prompt)     ✓ Phase 3
└── bus          Agent bus (channel|invite|join|send|recv|claim|complete|heartbeat|watchdog|log|run|discover)  ✓ B1–B3

tb  (planned)
├── run / proc / md / agent
└── serve        (today use: tb mcp)
```

---

## Comandos Detalhados

### `tb run` — Execução de Comandos (planejado — Fase 3+)

> **Não implementado como `tb run`.** Hoje use `tb sandbox run` para isolamento Landlock.

Executa comandos com isolamento opcional.

```bash
tb run [flags] <command>

Flags:
  --timeout <duration>    Maximum execution time (default: 30s)
  --sandbox               Runs in an isolated sandbox (default: false)
  --allow-net <hosts>     Hosts allowed on the network (requires --sandbox)
  --allow-read <paths>    Additional read paths (requires --sandbox)
  --allow-write <paths>   Additional write paths (requires --sandbox)
  --cgroup-cpu <percent>  CPU limit in % (default: 100)
  --cgroup-mem <bytes>    Memory limit (default: 1G)
  --json                  JSON output
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

Subcommands:
  start     Starts a process
  stop      Stops a process
  restart   Restarts a process
  list      Lists active processes
  logs      Shows a process's logs
  status    Status of a specific process
```

**`tb proc start`:**

```bash
tb proc start [flags] <command>

Flags:
  --name <name>           Process name (required)
  --log <path>            Log file (default: .tb/logs/<name>.log)
  --restart <policy>      Restart policy: never|on-failure|always
  --max-restarts <n>      Maximum automatic restarts
  --sandbox               Runs in a sandbox
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
`graph disabled (enable with --enable or TB_GRAPH=on; ADR-011)`). Medir ROI via
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
  index_on_change: true    # reindexa em search/callers/deps/impact/explain/path/stats se o fingerprint mudou
```

Ignora subdiretórios como `node_modules`, `vendor`, `dist`, `build`,
`target`, `.git`, etc. A raiz do walk nunca é pulada pelo nome (ex.: indexar
`/tmp/target` funciona; um `target/` filho continua ignorado). Se a raiz for
um symlink (ex.: `/tmp/target` → dir real), ela é resolvida com
`EvalSymlinks` antes do walk — caso contrário o índice ficaria vazio, pois
`filepath.Walk` não desce symlinks. O grafo continua sob demanda (ADR-011):
`TB_GRAPH=on` ou `--enable`.

Frescor do índice: cada `graph.json` guarda um `fingerprint` (mtime+tamanho dos
arquivos indexáveis). Em `search`/`callers`/`deps`/`impact`/`explain`/`path`/`stats`, se o
fingerprint divergir e `graph.index_on_change` for `true` (default), o grafo é
reindexado automaticamente; se for `false`, a saída avisa e o JSON inclui
`stale: true`. Saídas JSON de consulta usam o envelope
`{"stale": false, "nodes": [...]}`; `stats`/`index` incluem `stale` no objeto
de estatísticas.

Via MCP, `tb_graph_search`/`callers`/`deps`/`impact`/`explain`/`path` nunca
reindexam, seja qual for o `graph.index_on_change`: são anotadas com
`readOnlyHint`, então o cliente pode aprová-las automaticamente ou rodá-las em
paralelo. Índice obsoleto devolve `stale: true` com um `hint` para chamar
`tb_graph_index`, a única ferramenta de grafo do MCP que escreve.

```bash
tb graph [--enable] <subcommand>

Subcommands:
  search    Searches the graph ✅
  callers   Finds callers of a function ✅
  deps      Shows dependencies ✅
  impact    Analyzes the impact of changes ✅
  explain   Explains a node: incoming/outgoing edges with provenance ✅
  path      Shortest path between two nodes (edges in both directions) ✅
  index     Indexes code on demand ✅
  rebuild   Rebuilds the full graph ✅
  stats     Graph statistics ✅
```

Ferramentas MCP de grafo (chamada requer `TB_GRAPH=on`): `tb_graph_search`,
`tb_graph_callers`, `tb_graph_deps`, `tb_graph_impact`, `tb_graph_explain`,
`tb_graph_path`, `tb_graph_index`.

Não há `tb graph query` — use `tb graph search`.

**`tb graph search`:**

```bash
tb graph search <query> [flags]

Flags:
  --limit <n>         Maximum results (default: 20)
  --mode <mode>       lexical | semantic | hybrid (default: lexical)
  --json              JSON output (global flag)
```

Ordem em três faixas: (1) título idêntico à consulta; (2) nós que contêm a consulta como substring de título/path/pacote/ID — o que a busca antiga trazia; (3) nós que só casam termos. Dentro de cada faixa, ranking BM25 sobre título, path e pacote (título pesa 3×), com camelCase, `snake_case` e separadores de path quebrados em termos; consulta com várias palavras casa qualquer termo. Empates ordenam por ID. Com mais resultados que `--limit`, o corte é feito nessa ordem.

Com `--mode semantic`, a consulta vira embedding e é ranqueada por similaridade de cosseno sobre os chunks dos arquivos, agrupados por nó de arquivo; `--mode hybrid` funde os rankings léxico e semântico com RRF (k = 60), empates por ID do nó. Os dois precisam do [índice vetorial](#índice-vetorial) e nunca o reindexam. JSON nesses modos: `{"stale", "mode", "stale_vectors", "nodes": [...]}`, cada nó com os campos de sempre mais `lines`, `score_semantic`, `rank_lexical`, `rank_semantic` e `score_fused`; a saída de texto acrescenta `:<lines>` ao path. A saída do `lexical` (padrão) não muda.

**`tb graph callers`:**

```bash
tb graph callers [flags]

Flags:
  --function <name>   Function name (accepts a qualified suffix, e.g. scrubber.redact_secrets)
  --depth <n>         Depth (default: 1)
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
  --module <name>     Module/package
  --file <path>       File
  --transitive        Includes transitive dependencies
```

**`tb graph impact`:**

```bash
tb graph impact [flags]

Flags:
  --file <path>       Modified file
  --depth <n>         Depth (default: 2)
```

**`tb graph explain`:**

```bash
tb graph explain <node> [flags]

Flags:
  --limit <n>         Maximum explained nodes (default: 5)
```

Mostra o nó com as arestas de saída e entrada (`source`, `confidence`). O nó
resolve por ID exato → título exato → sufixo de função → arquivo/pacote.
Chamadas cujo alvo não tem nó no índice (stdlib, receptor não resolvido) são
contadas em `unresolved_out`. Até 50 vizinhos por sentido (`truncated_*`).

**`tb graph path`:**

```bash
tb graph path <from> <to> [flags]

Flags:
  --max-depth <n>     Max hops (default: 6)
```

Menor caminho (BFS) andando arestas nos dois sentidos; cada passo indica
`forward` (→) ou `backward` (←). JSON: `{"stale", "hops", "steps": [...]}`.
Sem caminho ou nó inexistente ⇒ erro e exit ≠ 0. Ambos ignoram `calls` com
confiança &lt; 0.6 ([ADR-014](adr/014-graphify-reimplementacao.md)).

**`tb graph index`:**

```bash
tb graph index [path] [flags]

Flags:
  --vectors           Also build or update the vector index (.toolbelt/graph.vec; needs graph.vector.enabled)
```

O path é **posicional** (não existe `--path`).

Com `--vectors` (também no `tb graph rebuild`), depois de indexar o tb cria ou atualiza o `.toolbelt/graph.vec` (modo 0600, escrita atômica), refazendo o embedding só dos chunks cujo sha256 do texto mudou. O JSON ganha `vectors`: `{path, files, units, embedded, reused, dropped, denied, truncated, rebuilt}`; a saída de texto imprime uma linha `vectors: …`.

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

#### Índice vetorial

A busca semântica e a híbrida ([spec](specs/vector-index.md)) vêm desligadas. As chaves são lidas **só da config do usuário** (o arquivo do `-c` ou a config global); valores no `.toolbelt/config.yaml` do repositório são ignorados com o aviso `config_untrusted`.

```yaml
graph:
  vector:
    enabled: true
    max_chunks: 50000
    chunk_lines: 40
    chunk_bytes: 1536
    overlap_lines: 8
  embed:
    model: /home/me/models/embed.gguf
    model_sha256: <hex>
    query_prefix: "query: "
    doc_prefix: "passage: "
    threads: 4
    timeout: 2m
    allow_unsandboxed: false
```

- `graph.vector.enabled` (padrão `false`); `graph.embed.model` é um path absoluto para um arquivo GGUF e `graph.embed.model_sha256` é obrigatório.
- `graph.embed.threads` tem padrão min(4, NumCPU), `graph.embed.timeout` `2m`; `graph.vector.max_chunks` 50000; `chunk_lines`/`chunk_bytes`/`overlap_lines` 40/1536/8.
- O filho de embedding `tb __graph-embed` roda em sandbox (Landlock + seccomp: sem sockets, workdir vazio e só leitura, tmp privado, `HOME` vazio, env mínimo). Arquivos da deny list (segredos) nunca viram embedding, e cada arquivo é redigido por inteiro antes de virar chunks. `graph.embed.allow_unsandboxed: true` deixa o filho rodar sem a sandbox completa (aviso `embed_unsandboxed`).
- Motivos: `vectors_disabled`, `vectors_stale`, `vectors_invalid`, `vectors_too_many`, `model_path`, `model_mismatch`, `model_unsupported`, `embed_unavailable`, `embed_unsandboxed`, `embed_protocol`, `embed_crashed`, `embed_timeout`, `config_untrusted`. Um build sem `embednative` responde `embed_unavailable`.
- MCP: o `tb_graph_search` aceita `mode` (`lexical`|`semantic`|`hybrid`, padrão `lexical`) e devolve o mesmo JSON da CLI (`mode`, `stale_vectors`, nós com `lines`, `score_semantic`, `rank_lexical`, `rank_semantic`, `score_fused`) mais um array `warnings`; o `tb_graph_index` aceita `vectors` (booleano, padrão `false`) e o payload ganha as estatísticas `vectors`. Sem eles a saída não muda. As consultas pelo MCP nunca reindexam os vetores. Os erros usam os mesmos motivos, com os avisos anexados ao texto do erro.
- O `tb mcp` mantém um filho de embedding aquecido por servidor: reaproveitado entre consultas, reiniciado quando a config de embed muda ou o filho quebra, morto depois de 5 minutos ocioso e na saída.
- Avaliação (44 consultas por intenção sobre este repositório, all-MiniLM-L6-v2 Q8_0): recall@10 lexical 0,273, semantic 0,670, hybrid 0,568. O `semantic` teve a maior nota nesta suíte; o `hybrid` mantém os acertos por nome exato do ranking léxico. Veja o [dogfood](campo/dogfood.md) #45.

---

### `tb mem` — Invariantes (✓ Fase 1)

Gerencia o arquivo `.toolbelt/invariants.yaml` (fora da compactação, ADR-010).

```bash
tb mem <subcommand>

Subcommands:
  add       Adds an invariant
  list      Lists invariants (omits retired; --all includes them)
  show      Shows an invariant by id
  edit      Edits fields of an invariant
  retire    Retires an invariant (soft-retire; omitted from list/check/search by default)
  search    Searches invariants
  check     Checks the integrity of the invariants file
  init      Creates default anti-cheat invariants
```

Notas que não são invariantes ficam no vault Markdown: ver [`tb vault`](#tb-vault--vault-markdown-fase-a). As invariantes nunca são espelhadas lá ([ADR-016](adr/016-vault-fase-a.md)).

Subcomando desconhecido (`tb mem foo`) sai com código ≠ 0 e mensagem `unknown command`.

**`tb mem add`:**

```bash
tb mem add <rule> [flags]

Flags:
  --category <cat>    Category: anti_cheat, mandate, security (default: anti_cheat)
  --ref <path>        ADR/decisions.md path (e.g. docs/adr/0001.md)
  --immutable         Marks the invariant as immutable (default: mutable)
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
tb mem edit <id> [--category <cat>] [--rule <text>] [--ref <path>] [--force] [--reason <reason>]
tb mem retire <id> [--force] [--reason <reason>]
```

**`tb mem init`:** cria o conjunto padrão anti-trapaça e imprime o caminho + as
invariantes criadas (ou `N invariants already existed in <path>` se o arquivo já tinha entradas). Com
`--json`, emite `path`/`created`/`count`/`invariants`.

**`tb mem check` (implementado):**

```bash
tb mem check [flags]

Flags:
  --expect-sha <sha>  Expected hash after compaction
  --all               Include retired invariants
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

### `tb red` / `tb blue` — Playbooks AppSec (✓ Fase 1, ADR-017)

Metodologia AppSec nativa ofensiva (red) e defensiva (blue), pareada por classe de vulnerabilidade. A busca é compacta; `load` respeita um orçamento de tokens (padrão 2000) e fatia por seção `##`. Playbooks são texto de referência — os caminhos de leitura nunca executam nada. Playbooks blue declaram checks grep/regex que `tb blue apply` registra em `.toolbelt/secchecks.yaml` e `tb blue check` roda sobre a árvore de trabalho. `tb red` imprime um aviso de uso autorizado uma vez por repo.

```bash
tb red search "deserialization"
tb red list --area web
tb red load red-sqli --section Detection
tb red load red-rce --budget 1500
tb red load red-xxe --full
tb red pair red-sqli
tb red explain sql-injection
tb blue search "detection"
tb blue apply sql-injection --dry-run
tb blue check
```

Paridade MCP: `tb_red_search`, `tb_blue_search`, `tb_red_load`, `tb_blue_load`, `tb_sec_pair` (somente leitura), `tb_blue_apply` (grava `secchecks.yaml`). Ver [docs/specs/appsec-playbooks.md](specs/appsec-playbooks.md).

Com `TB_GRAPH=on`, cada playbook é um nó `playbook:<nome>` no grafo e todo playbook red tem uma aresta `mirror` para o par blue, então `tb graph path playbook:red-sqli playbook:blue-sqli` e `tb graph explain` ligam ataque e defesa. `tb stack seccheck` roda os checks registrados como op somente-leitura do stack (falha em achados de severidade error, a menos que `--warn-only`).

---

### `tb lab` — Labs red+blue isolados (✓ Fase 1 tier sandbox, ADR-017)

Ambientes efêmeros e isolados que provam que uma técnica red funciona **e** que o check blue par detecta, contra um alvo local do lab — nunca o host, nunca um terceiro. O isolamento é escalonado: um lab de passo único roda no sandbox existente (Landlock, alvo Go puro in-process); uma topologia multi-serviço roda em Podman rootless. A Fase 1 implementa o **tier sandbox**; o **tier Podman falha fechado** (remediação clara) quando o Podman não está disponível, em vez de degradar o isolamento. Os manifestos ficam em `labs/<nome>/lab.yaml` (versionados); o estado de execução fica fora do repo. `tb lab` compartilha o aviso único de uso autorizado com `tb red`.

```bash
tb lab new demo --class sql-injection
tb lab list
tb lab status sqli
tb lab up sqli
tb lab up sqli --tier podman
tb lab run sqli
tb lab logs sqli
tb lab down sqli
tb lab verify
```

`tb lab run` reporta um veredito acionável: `attack=reproduced|failed` e `defense=detected|missed`. O seed `labs/sqli/lab.yaml` reporta `attack=reproduced defense=detected`. O tier de topologia Podman, os limites de recursos por cgroups, o export de artefatos e as ferramentas MCP `tb_lab_*` são Fase 2. Veja [docs/specs/red-blue-labs.md](specs/red-blue-labs.md).

---

### `tb md` — TUI Markdown (planejado — Fase 3+)

> **Não implementado, e fora da Fase A do vault** ([ADR-016](adr/016-vault-fase-a.md)): qualquer editor, inclusive o Obsidian, edita o vault; ver [`tb vault`](#tb-vault--vault-markdown-fase-a).

Interface TUI para navegar e editar o vault.

```bash
tb md [flags] [path]

Flags:
  --obsidian          Opens in Obsidian instead of the TUI
  --read-only         Read-only mode
  --graph             Starts in graph view
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

Subcommands:
  spawn     Creates a new agent
  list      Lists active agents
  status    Status of a specific agent
  logs      Agent logs
  stop      Stops an agent
  budget    Shows/changes the budget
  approve   Approves a pending action
  reject    Rejects a pending action
```

**`tb agent spawn`:**

```bash
tb agent spawn [flags]

Flags:
  --role <role>       Agent role/persona (required)
  --parent <id>       Parent agent ID
  --task <task>       Initial task
  --budget <tokens>   Token budget
  --timeout <dur>     Maximum timeout
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

Executa `verify`, `review`, `drift`, `guard` e/ou `egm` numa única invocação, compartilhando
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
  --base <ref>           Shared base ref (default: HEAD~1; validated/resolved)
  --head <ref>           Shared head ref (default: HEAD; validated/resolved)
  --fail-fast            Cancel remaining steps via context on the first failure
  --jobs <n>             Concurrency limit (default: CPU count, max 32; 1 = serial)
  --ticket <path>        Ticket/rd.md (review/drift/verify; default: rd.md)
  --approved-sha <sha>   Approved SHA; verbatim patch-id (review/drift; validated/resolved)
  --approved-base <ref>  Base of the approved range (default: merge-base(approved, base); validated/resolved)
  --require-approved-sha Fail if --approved-sha is missing (or `review.require_approved_sha`)
  --warn-only            guard: exit 0 even with SeverityError
  --test-cmd <cmd>       verify: visible tests
  --hidden-test-cmd <cmd> verify: hidden tests
  --build-cmd <cmd>      verify: build command (empty + explicit flag or --no-build to skip the config one)
  --lint-cmd <cmd>       verify: lint command (empty + explicit flag or --no-lint to skip)
  --no-build             verify: skips build_cmd
  --no-lint              verify: skips lint_cmd
  --dry-run              verify: shows the plan (cmds/dir) without executing
  --max-iter <n>         verify: max iterations
  --timeout <duration>   verify: timeout (default: 10m)
  --reviewers <n>        review: max goldfish
  --standards <csv>      review: roles (scope,quality,acceptance)
  --egm-doc <path>       egm: document added to the selection (repeatable)
  --strict               egm: PASS_PARTIAL and an empty selection fail
  --egm-mode <mode>      egm: local (default) | ci
  --egm-allow-offline    egm ci: without a credential, a deterministic-only round capped at PASS_PARTIAL
  --env <KEY>            (global) extra allowlist for stacked verify
  --pass-env <KEY>       (global) explicit secret opt-in for verify
```

Sem operação, ou com operação desconhecida, sai com erro curto listando as ops
válidas (`verify`, `review`, `drift`, `guard`, `egm`) — sem dump de `Usage`.
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

**Op `egm` (gate):** seleciona os documentos sob gate (`egm.doc_globs`, por padrão
`docs/specs/**` e `docs/adr/**`; um índice `README.md` nunca é selecionado) adicionados ou modificados em
`merge-base(base, head)..head`, mais os de `--egm-doc`, e avalia cada um com
`tb egm status --at <head>`. O documento, as fontes e a config do repositório vêm do
commit, então um documento convergido que mudou depois da revisão fica `STALE`.
`PASS` é `ok`; `PASS_PARTIAL` é `ok` com `warn: true` (`ok: false` com `--strict` ou
`egm.require_full_coverage`); qualquer outro veredito, ou uma linhagem com
`forced_blocker`, é `ok: false`. Seleção vazia é `ok: true, warn: true` com
`no_gated_docs` (`ok: false` com `--strict`), e `ok: false` com
`uncommitted_gated_docs` quando a worktree tem documentos sob gate diferentes do head.
Um documento sob gate que não foi tocado, mas cita um caminho que mudou no intervalo,
recebe o aviso `sources_changed` para cada caminho do drift das fontes dele (contra o recibo,
ou uma rodada local convergida; veja `tb egm drift-report`). A `egm` nunca roda em paralelo e é sempre movida para
o fim; `Result.warn` é o OR dos `warn` das etapas e nunca muda o exit code. No
`--egm-mode local` padrão o gate é local (`trust: "local"`): num checkout de CI não há
estado, então o resultado é `NO_STATE`. Ele também avisa `receipt_missing`,
`receipt_invalid` ou `receipt_stale` (nunca falha), para o autor ver o que o CI vai
recusar, `gated_doc_removed` para um caminho sob gate apagado ou renomeado para fora do
gate, e `receipt_removed` para o recibo apagado de um documento que continua sob gate.

**`--egm-mode ci`** ([egm trust](specs/egm-trust.md); guia de configuração em [egm-ci.md](egm-ci.md)) é o gate do CI (`trust: "ci"`).
Exige `-c` (a config de usuário do egm do próprio CI) e implica `--strict`.
Um intervalo que edita um recibo seleciona o documento dele; apagar o recibo de um documento
que continua sob gate reprova (`receipt_removed`). O `ci_source_drift` aponta um documento
intocado que estava fresh contra o recibo do **merge-base** e teve drift neste intervalo; é
aviso, a menos que `egm.drift.ci: fail` (a config do repositório só pode endurecê-lo). Para cada
documento selecionado verifica, em ordem: o recibo commitado em head (`tb egm receipt`)
existe e é válido (`no_receipt`, `receipt_invalid`); os hashes do documento e das fontes
batem com os do commit (`receipt_stale`); depois roda **uma rodada nova** com
`--engine llm` num estado descartável. Uma rodada nova com downgrade é `ci_partial`. Um finding novo de `major` para cima reprova como
`ci_finding`, a menos que case com uma alegação do recibo (mesmo heading, os dois com linhas
até `egm.match.gap`, `sim ≥ egm.match.sim`, sem severidade maior): uma alegação
`declined` ou `forced` reprova como `ci_downgrade`; uma `arbiter_withdrawn` sobre um finding
contestável (não checker, `injection-scan` nem comprehension) passa de novo pelo árbitro do
próprio CI (`withdraw` passa, qualquer outra coisa é `ci_upheld`).
Findings abaixo de `major` só avisam. Um revisor obrigatório que não completa reprova
(`ci_not_run`). Sem credencial (variável do `egm.env.pass` vazia, ou `auth_expired`) o resultado é `ci_offline`; CI mal configurado (sem `agent_cmd`, sem Landlock) é exit 2, a menos que
`--egm-allow-offline` rode uma rodada só determinística com teto `PASS_PARTIAL`. Mais
documentos selecionados que `egm.ci.max_docs` (padrão 5; os globs do gate são a união dos do merge-base e dos de head, então a config do PR não os estreita), ou um passo além de
`egm.max_agent_runs`, é `ci_budget`; um documento sob gate removido reprova. O
`ci_findings` de cada documento lista os findings novos com o `result`.

```bash
tb stack verify guard egm --base origin/main
tb stack egm --base origin/main --egm-doc docs/adr/015-egm-nativo.md --strict
tb stack egm --egm-mode ci -c "$RUNNER_TEMP/egm.yaml" --base "$BASE_SHA" --head "$HEAD_SHA"
```

---

### `tb guard` — Proteções de Segurança (✓ implementado: `check`)

Analisa o diff entre commits em busca de paths protegidos e padrões anti-trapaça.

```bash
tb guard check [base..head] [flags]

Flags:
  --base <ref>   Base ref (default: HEAD~1)
  --head <ref>   Head ref (default: HEAD)
  --warn-only    Do not fail the exit code on SeverityError (opt-out of the CI-safe default)
  --strict       Deprecated: failing on SeverityError is already the default; use --warn-only to opt out
  --json         JSON output (global flag)
```

Refs que começam com `-` ou SHA inexistente saem com erro claro (sem chamar
`git` de forma que interprete a ref como opção).
Por default, exit ≠ 0 quando há `SeverityError` (comportamento estrito).
`--strict` é **deprecated** (alias sem efeito: o default já falha em
`SeverityError`); prefira omitir a flag ou usar `--warn-only` para opt-out.
No modo texto limpo imprime `guard: ok (N rules, 0 violations)` (`N` = regras
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
  --base <ref>               Base commit/ref (default: HEAD~1 or the ticket's base_commit)
  --head <ref>               Head commit/ref (default: HEAD)
  --approved-sha <sha>       Approved SHA; compares `git patch-id --verbatim` of base..approved vs base..head
  --approved-base <ref>      Base of the approved range (default: merge-base(approved, base))
  --require-approved-sha     Fails if `--approved-sha` is missing (or `review.require_approved_sha` in config)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --reviewers <n>            Maximum distinct goldfish (default: 3; does not clone)
  --standards <stds>         Roles: scope,quality,acceptance (default; aliases: invariants,completeness)
  --output <path>            JSON report (no `-o` shorthand; use `--output`)
  --json                     JSON output (global flag)
```

Ticket inexistente ou ilegível sai com código ≠ 0 e mensagem
`ticket not found: <path>` / `ticket unreadable (<path>): …`
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
No modo texto, imprime `role=<role>` (e `agreeing_roles=N` só se N>1) — não usa mais o antigo `acordo=N/M`.

**Exemplos:**

```bash
# Revisão padrão: 3 papéis distintos
tb review --ticket rd.md
# ═══════════════════════════════════════════════════════════
#  REVIEW REPORT: rd.md
#  Base..Head: <base-sha>..<head-sha> | Reviewers: 3 | Fresh: true
# ═══════════════════════════════════════════════════════════
# FINDINGS:
#   [warning] (goldfish-quality/quality) TODO/FIXME/XXX/HACK added in a comment in the diff [src/a.go]  role=quality
#   [warning] (goldfish-acceptance/acceptance) acceptance criterion without evidence in changed tests/files: works  role=acceptance

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
  --base <ref>               Base commit/ref (default: HEAD~1 or the ticket's base_commit)
  --head <ref>               Head commit/ref (default: HEAD)
  --approved-sha <sha>       Approved SHA; compares `git patch-id --verbatim` of base..approved vs base..head
  --approved-base <ref>      Base of the approved range (default: merge-base(approved, base))
  --require-approved-sha     Fails if `--approved-sha` is missing (or `review.require_approved_sha`)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --output <path>            JSON output (no `-o` shorthand)
  --json                     JSON output (global flag)
```

Ticket inexistente ou ilegível sai com código ≠ 0 e mensagem
`ticket not found: <path>` / `ticket unreadable (<path>): …`
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
#  DRIFT REPORT
# ═══════════════════════════════════════════════════════════
# PINNED APPROVAL:
#   checked: false
#
# Base: declared= actual=<sha> merge_base=<sha>
# Out of scope (1):
#   - b.go
# Forbidden touched (0):

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

### `tb egm` — Loop de Revisão de Documentos de Design (Fase A)

Revisa specs e ADRs e repete a rodada até que duas rodadas limpas sobre o mesmo
conteúdo convirjam ([spec](specs/egm-nativo.md), [ADR-015](adr/015-egm-nativo.md)).
As duas engines rodam os checkers `refs-exist`, `required-sections` e
`plan-files-listed` e o scan anti-injeção. A **engine `llm`** também roda os 9
revisores do preset consolidado como goldfish: processos únicos de `egm.agent_cmd`
num sandbox Landlock com packet só de leitura, HOME descartável, env mínimo e sem
ferramentas (Linux). Sem `egm.agent_cmd`, o `auto` usa a **engine determinística**,
em que as personas LLM ficam `unavailable` e o melhor veredito é `PASS_PARTIAL`.
`status --at <rev>` é o modo gate usado pela op `egm` do
[`tb stack`](#tb-stack--empilhamento-de-operações-de-verificação--fase-2).

```yaml
# ~/.config/toolbelt/config.yaml
egm:
  agent_cmd: [/home/eu/.local/bin/claude]   # absoluto; o vendor claude acrescenta as flags sem ferramentas
  # timeout_per_reviewer: 5m, max_parallel: 4, max_agent_runs: 120, clean_rounds: 2, max_rounds: 8
```

Com o perfil `claude`, o tb copia só a chave `claudeAiOauth` de
`~/.claude/.credentials.json` para o HOME descartável de cada goldfish e se recusa a
começar (`auth_expired`) quando o access token tem menos de 10 minutos: uma cópia não
consegue renová-lo, então rode `claude` uma vez fora do tb antes. Uma rodada de 9
revisores custa cerca de US$ 2–2,5.

```bash
tb egm review docs/specs/x.md            # rodada 1 (exit 0; 1 em FAIL)
tb egm loop next docs/specs/x.md         # próxima rodada; recusada se nada mudou ou já convergiu (exit 0)
tb egm resolve docs/specs/x.md --fix F-1,F-2
tb egm resolve docs/specs/x.md --decline F-4
tb egm resolve docs/specs/x.md --force-settle F-3 --reason "risco aceito"
tb egm resolve docs/specs/x.md --contest F-5 --evidence internal/x.go:40   # o árbitro roda no próximo loop next
tb egm status docs/specs/x.md [--strict]  # 0 PASS, 1 FAIL, 3 IN_PROGRESS/STALE/NO_STATE
tb egm status docs/specs/x.md --at HEAD   # modo gate: documento, fontes e config do commit
tb egm receipt docs/specs/x.md            # grava .toolbelt/egm/receipts/<slug>.json (exige PASS ou PASS_PARTIAL)
tb egm drift-report [doc...] [--at REV]   # drift das fontes contra o recibo: 0 fresh/no_baseline, 1 drift, 2 erro
tb egm drift-report --engine llm          # mais um goldfish de drift por documento drifted (egm.drift.max_docs); findings relatados, exit igual
tb egm bootstrap docs/specs/novo.md --from internal/x/  # esqueleto novo ligado ao código (nunca sobrescreve, nunca commita); recusas saem com 2: usage, bootstrap_exists, bootstrap_path, bootstrap_sources
tb egm bootstrap docs/specs/novo.md --from internal/x/ --engine llm  # um goldfish redige o corpo; também bootstrap_injection, bootstrap_invalid, bootstrap_not_run, tools_unverifiable, agent_unavailable
tb egm review docs/specs/x.md --restart   # novo ciclo depois do teto (exige edição)
```

- **Flags:** `review` e `loop next` aceitam `--engine auto|llm|deterministic`, `--reviewers a,b` e `--dry-run`; `review` aceita `--restart`; `status` aceita `--strict` e `--at <rev>`. Todo comando aceita `--json`.
- **Modo gate (`--at`):** o documento, as fontes (modo vindo do `ls-tree`, bytes do `cat-file blob`; uma fonte atrás de um symlink versionado fica de fora) e a config do repositório vêm do commit, nunca da worktree. O repositório é encontrado a partir do diretório atual, e o caminho do documento é resolvido lexicamente contra ele, então um diretório ausente ou com symlink na worktree não muda nada. Uma fonte versionada conta mesmo quando casa com o `.gitignore`, nos dois modos. O `content` de uma worktree commitada e sem mudanças é o mesmo nos dois modos. Uma fonte com filtro de `.gitattributes` não é detectada (`filtered_source` é uma limitação declarada).
- **Recibo (`receipt`):** grava `.toolbelt/egm/receipts/<slug>.json`, para commitar junto com o documento: os hashes do documento e das fontes mais as alegações, que são as linhagens de `major` para cima que o árbitro retirou (sem reabertura depois), que foram recusadas ou que foram encerradas à força. Cada alegação traz `lines` no documento atual; um scope sem linhas cai na seção do heading. Os bytes são determinísticos (chaves ordenadas, quebra de linha no fim). É recusado (exit 3, nada gravado) se o veredito não for `PASS` ou `PASS_PARTIAL`, recusado com exit 2 `receipt_uncommitted` quando o documento ou uma fonte incluída difere de `HEAD` (commite antes; o CI recalcula os hashes a partir do commit), e recusado com exit 2 `receipt_path_untrusted` quando um componente do caminho é symlink ou não é diretório. O modo CI que confere os recibos vem numa fatia posterior ([egm trust](specs/egm-trust.md)).
- **MCP:** `tb_egm_status` (`cwd`, `doc`, `at`, `strict`), `tb_egm_receipt` (`cwd`, `doc`) e `tb_egm_resolve` (`cwd`, `doc` e as flags do resolve como `fix`, `decline`, `force_settle`, `reason`, `contest`, `evidence`). `cwd` precisa ser absoluto. `review` e `loop next` levam minutos e ficam só na CLI.
- **Autenticação (claude):** com `ANTHROPIC_API_KEY` ou `CLAUDE_CODE_OAUTH_TOKEN` listada no `egm.env.pass` e preenchida, a chave chega ao goldfish pelo env e nenhum arquivo de credencial é verificado nem copiado (o caso do CI); quem lista `ANTHROPIC_API_KEY` no `egm.env.pass` e a exporta localmente passa a ser cobrado nessa chave, não no login do claude; senão a entrada `claudeAiOauth` de `~/.claude/.credentials.json` é copiada depois da checagem de `expiresAt`.
- **Config:** lida da config de usuário (`~/.config/toolbelt/config.yaml`, seção `egm:`) ou do `-c`, nunca da config mesclada. O `.toolbelt/config.yaml` do repositório só pode endurecê-la: chaves de execução (`agent_cmd`, `env`, …) são ignoradas com `config_untrusted`, limites só descem, política só sobe.
- **Git:** lista fechada de chamadas git endurecidas (sem hooks, filtros, fsmonitor, pager nem diff externo); um `.git` com chaves fatais (`include.*`, partial clone, alternates, …) para com exit 2 `hostile_git_dir`.
- **Estado:** `$XDG_STATE_HOME/tb/egm/<repo-id>/<slug>/` (`state.json`, `round-<n>/target.md`, `report.{json,md}`), fora do repositório e compartilhado pelas worktrees dele.
- **Exit 2** para erros de uso e recusas: `state_exists`, `no_state`, `no_change`, `locked`, `invalid_transition`, `contest_refused`, `restart_needs_change`, `budget`, `owner_unavailable`, `auth_expired`, `forbidden_flag`, `env_forbidden`, `no_repository`, `hostile_git_dir`.

---

### `tb vault` — Vault Markdown (Fase A)

Notas de conhecimento do projeto que não são invariantes (fatos, decisões, contexto, registros), como arquivos Markdown com frontmatter YAML opcional e `[[wikilinks]]` em `vault.path` (default `.toolbelt/vault/`), editáveis no Obsidian ou em qualquer editor ([spec](specs/vault.md), [ADR-016](adr/016-vault-fase-a.md)). Os arquivos são o único armazenamento: todo comando varre o vault do disco.

```bash
tb vault new "<título>" [--type T] [--tag t]... [--source S --confidence C] [--dir sub] [--body -|texto]
tb vault search <consulta> [--limit N]
tb vault list [--type T] [--tag t] [--source manual|extracted|inferred]
tb vault show <nota>          # frontmatter normalizado + corpo; --json acrescenta links e backlinks
tb vault links <nota>         # wikilinks de saída com estado
tb vault backlinks <nota>     # notas que apontam para <nota>
tb vault check                # exit 1 com erros; avisos mantêm exit 0
```

- **Raiz e config:** a raiz é o pai do `.toolbelt/` encontrado a partir do diretório atual, e a config é lida de `<raiz>/.toolbelt/config.yaml` (`vault.path`, `vault.max_note_bytes` default 256 KiB, `vault.types` default `fact, decision, context, log`). O `vault.path` precisa resolver dentro do repositório (`vault_path_outside_repo`, exit 2); vault inexistente é `vault_not_found` (exit 1).
- **Varredura:** symlinks nunca são seguidos, diretórios que começam com `.` (`.obsidian/`, `.trash/`) são pulados, FIFOs e outros arquivos não regulares viram avisos `not_regular` e nunca são abertos; a leitura usa `O_NOFOLLOW|O_NONBLOCK` mais uma checagem com `fstat`.
- **Frontmatter:** `title`, `type`, `tags`, `aliases`, `source` (default `manual`), `confidence` (obrigatório com `source: inferred`), `created`, `updated`; campos desconhecidos são mantidos e ignorados.
- **Wikilinks:** `[[nota]]`, `[[pasta/nota]]` (também caminho parcial, como no Obsidian), `[[nota#heading]]`, `[[nota|rótulo]]`, `![[embed]]`. O casamento ignora maiúsculas. Duas notas com o mesmo nome são `ambiguous_link` (erro, onde o Obsidian escolhe o caminho mais curto); aliases não resolvem links. Um alvo sem nota correspondente é anexo quando um anexo casa com ele ou a extensão tem 1–8 letras/dígitos (`pic.png`, não `Dr. Smith`): encontrado é `attachment`, ausente é aviso `missing_attachment`, vários são aviso `ambiguous_attachment`. Notas grandes demais ou ilegíveis continuam resolvendo como alvo de link.
- **`<nota>`** em `show`/`links`/`backlinks` é um caminho de nota (com ou sem `.md`) ou um link (`a`, `[[a]]`, `a#heading`, `[[a|rótulo]]`).
- **Check:** erros `bad_frontmatter`, `missing_confidence`, `note_too_large`, `broken_link`, `ambiguous_link`; avisos `not_regular`, `unreadable`, `bad_tag`, `unused_confidence`, `broken_fragment`, `missing_attachment`, `ambiguous_attachment`. A saída em texto põe entre aspas nomes de arquivo e títulos com caracteres de controle.
- **Saída:** com `--json`, o texto das notas (título, tags, aliases, corpo, alvos de link) fica dentro de objetos `data`, e toda saída é redigida (GitHub, AWS, Slack, `Bearer`, PEM, `*_TOKEN=`, `sk-`, `password:`/`secret=`). Mesmas notas, mesmos bytes.
- **Stack:** `tb stack vault` roda o `check` como op paralela, só de leitura.
- **`new`:** o nome do arquivo é o slug do título redigido (até 80 bytes) em `--dir`; nunca sobrescreve (`note_exists`, exit 1), recusa título com caractere de controle ou sem letras (`bad_title`) e um `--dir` que sai do vault ou passa por symlink (`dir_outside_vault`). O frontmatter é gerado por um encoder YAML; título e corpo são redigidos antes de gravar (`redacted: true`).
- **`search`:** substring sem diferenciar maiúsculas e acentos em título, aliases, tags e corpo, depois de redigir cada nota, então uma consulta nunca sonda um segredo; um acerto por nota, com a linha e um trecho de até 200 runas.
- **MCP:** `tb_vault_list`, `tb_vault_show`, `tb_vault_search`, `tb_vault_links`, `tb_vault_backlinks`, `tb_vault_check` (só leitura) e `tb_vault_new`, que grava sempre `source: inferred` e exige `confidence`.
- **Grafo:** com `TB_GRAPH=on`, o `tb graph index` acrescenta cada nota como `note:<caminho>` (título redigido, proveniência de `source`/`confidence`), os headings dela como seções, wikilinks resolvidos como arestas `links` e tags como nós `tag:<tag>`; `tb graph explain note:<caminho>` e `tb graph path` funcionam sobre eles. O vault nunca é lido pelo indexador Markdown genérico (que segue symlinks); `vault.graph: false` deixa as notas de fora.

### `tb verify` — Verificação Adversarial (✓ Fase 1/2)

Executa o loop gerador→discriminador com rulers plugáveis.

```bash
tb verify run [flags]

Flags:
  --ruler <type>           Ruler: tests|visual (default: tests)
  --test-cmd <cmd>         Visible tests command (default: verify.test_cmd; `tb init` detects go/npm/pnpm/yarn/pytest/cargo/make and also build_cmd/lint_cmd)
  --hidden-test-cmd <cmd>  Hidden tests command (verify.hidden_test_cmd; runs even without tests/hidden/; if the dir exists and the cmd is empty: detects the toolchain or marks N/A)
  --build-cmd <cmd>        Build command (default: verify.build_cmd; empty + explicit flag or --no-build to skip)
  --lint-cmd <cmd>         Lint command (default: verify.lint_cmd; empty + explicit flag or --no-lint to skip)
  --no-build               Skips build_cmd (config and default)
  --no-lint                Skips lint_cmd (config and default)
  --dry-run                Shows the plan (cmds and directory) without executing
  --max-iter <n>           Maximum iterations (default: 3)
  --timeout <dur>          Timeout (default: 10m)
  --threshold <float>      Visual similarity threshold (default: 0.95)
  --manifest <path>        Screens manifest (default: .toolbelt/screens.yaml)
  --skip-unavailable       Graceful skip (SKIPPED verdict) if browser/refs/screens are unavailable (default: true)
  --base <ref>             Base ref
  --head <ref>             Head ref (build/lint/test run at this commit)
  --ticket <path>          Ticket with Scope (exception to custom `protected_paths` in guard.yaml)
  --output <path> / -o     JSON output file
  --json                   JSON output (global flag)
  --env <KEY>              (global, repeatable) allows an environment variable in the child (does not bypass secret blocking)
  --pass-env <KEY>         (global, repeatable) explicit opt-in to pass a secret to the child
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
| `go.mod` | `go build ./...` | `.golangci.y*ml`/`.toml` → `golangci-lint run` se o binário estiver no PATH, senão `go vet ./...`; sem config, target `make lint` |
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
`command not found: X — adjust verify.test_cmd`. O processo `tb` sai com
código **127**. Com `--json` (ou `--output`), emite o envelope `verify.Result`
normal incluindo `error` (e `verdict: FAIL`), em vez de sair sem JSON. Falhas
genuínas do `test_cmd`/`hidden_test_cmd` (exit ≠ 0/126/127) continuam no loop
de `--max-iter`.

Veredictos `ESCALATE` e `FAIL` saem com **exit ≠ 0** também com `--json`
(igual ao modo texto). Só `PASS` e `SKIPPED` (ruler visual / `--dry-run`) usam
exit 0. Comando inexistente usa exit **127** (ver tabela de códigos). O JSON
de verify (e cada etapa do `tb stack`) inclui `reason`/`error` com o motivo da
falha ou do ESCALATE (não só em DEBUG no stderr).

A saída de falha é pensada para agentes: os comandos filhos recebem `NO_COLOR=1 FORCE_COLOR=0`
(a menos que um dos dois já esteja definido), os escapes ANSI são removidos do `reason` e uma
saída longa mantém primeiro as linhas de falha (`FAIL`, `Error:`, `✗`, `panic:` …) e depois o
final, onde os runners imprimem o resumo. Quando duas iterações seguidas falham com as mesmas
linhas de falha nomeadas (durações, endereços e portas ignorados), o verify para com `ESCALATE`
e `reason: deterministic failure …` em vez de gastar o resto do `--max-iter`; uma falha
silenciosa (sem linha reconhecível) continua sendo repetida. Em veredicto diferente de PASS, o
JSON traz `filtered_env`: os **nomes** (nunca os valores, e nunca nomes com cara de segredo) das variáveis que a allowlist
removeu, para que uma falha que só acontece dentro do `tb verify` aponte a entrada que falta
em `env.allow`.

Num repositório que tem um único commit, o `--base HEAD~1` implícito cai para `HEAD` (diff vazio,
aviso no stderr) em vez de falhar; build, lint e testes rodam normalmente. Clones rasos, uma
branch órfã ao lado de outro histórico e um `--base` explícito que não resolve continuam
falhando (ali um diff vazio esconderia edições em paths protegidos).

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
  --base <ref>          Base ref (default: HEAD~1; ignored if base..head is passed)
  --head <ref>          Head ref (default: HEAD; ignored if base..head is passed)
  --warn-only           Do not fail the exit code on SeverityError (opt-out of the CI-safe default)
  --strict              Deprecated: failing on SeverityError is already the default (no-op alias; use --warn-only)
  --json                JSON output
```

Aceita o intervalo posicional `base..head` (forma clássica) ou as flags `--base`/`--head`, alinhadas a `tb review` / `tb drift` / `tb verify`.

Por default, exit ≠ 0 quando há `SeverityError` (CI-safe). `--strict` está
**deprecated** (substituído pelo default estrito + `--warn-only`). No modo texto
sem violações imprime `guard: ok (N rules, 0 violations)` (`N` = paths protegidos
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
# ticket TKT-001 created: Criar modelo User
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
# board: active (strategy: dependency)
#
# RUNNING (0)
#
# READY (1)
#   TKT-001	pending  	p0	Criar modelo User
#
# BLOCKED (1)
#   TKT-002	JWT signing	(waiting: TKT-001: pending)
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
| `run` | `--workers <n>`, `--strategy fifo\|priority\|dependency`, `--agent-cmd "<cmd>"`, `--once`, `--dry-run`, `--check`, `--allow-unsandboxed` | Executa até esvaziar a fila; Ctrl-C devolve tickets `running` para `pending`. Com `router.bus` (só config global ou `-c`) acrescenta slots do barramento `b1..bN` que entregam tickets a workers com `router_tasks` num canal do barramento; `max_workers: 0` = só barramento (sem `agent_cmd`, sem Landlock). `--check` valida a sessão e o canal do barramento. Spec: [bus-router.md](bus-router.md) |
| `status` | — | Lê `state.json`; detecta estado obsoleto (PID morto) |
| `config` | — | Imprime a configuração efetiva (`router:`) |

- `--dry-run` mostra o plano por rodada (worker, ticket, origem `board`/`direct`, base) sem executar.
- `--once` faz uma rodada de agendamento e sai quando os tickets iniciados terminam.
- Sem Landlock o sandbox é **fail-closed**: o router recusa executar. `--allow-unsandboxed` é opt-in explícito (veja `tb sandbox`).
- O ambiente do filho é filtrado (allowlist; segredos só com `--pass-env`) e recebe `TB_TICKET_ID`, `TB_TICKET_FILE` (caminho absoluto), `TB_WORKER_ID` e `TB_WORKSPACE`.
- Timeout por ticket = `acceptance.timeout` do ticket ou `router.timeouts.per_ticket`.
- Ao sair do agente com mudanças, o router faz `git add -A` e commit `tb: <id> <título>` na worktree e grava `base_commit`/`head_commit` no ticket. Exit 0 → `done`; senão `failed`, ou volta a `pending` enquanto `attempts < router.retry.max_attempts`.
- **Instância única:** só um `tb router run` por repositório, via lock exclusivo `.toolbelt/router/lock` (`flock`; em sistemas não-unix, lockfile `O_EXCL` — se sobrar após um crash, remova o arquivo manualmente).
- **Integridade do worktree:** antes do commit o router verifica que `<ws>/.git` é o arquivo-ponteiro de worktree esperado e roda o git com configuração endurecida (fsmonitor e hooks desabilitados, `--no-verify`, config global/de sistema do usuário desligada — `GIT_CONFIG_GLOBAL=/dev/null` e `GIT_CONFIG_NOSYSTEM=1` —, logo filtros do usuário como git-lfs **não** rodam sobre o conteúdo do agente; `user.name`/`user.email` são lidos uma vez da raiz com a config normal e passados via `-c`, com fallback `tb <tb@localhost>`). O `git worktree add/remove` do router também roda sem hooks (`core.hooksPath=/dev/null`), fsmonitor e manutenção destacada: worktrees criados pelo router nunca executam hooks `post-checkout` do usuário. Violação ⇒ ticket `failed` sem retry (`worktree integrity violated`).
- **`.git` aninhado, gitlinks e `.gitmodules`:** o git do router roda fora do sandbox, então um repositório/submódulo criado pelo agente (p.ex. `sub/.git` com `filter.*.clean` dentro de um gitlink do commit base) nunca é tocado: antes de qualquer git o router percorre o worktree (sem seguir symlinks) e recusa qualquer entrada `.git` além do ponteiro raiz; depois do `git add -A` recusa commits que adicionem/alterem gitlinks (modo 160000) ou `.gitmodules`. Em ambos os casos o ticket vira `failed` sem retry (`worktree integrity violated`). O git do router também roda com `--ignore-submodules=all` e submódulos desabilitados (`diff.ignoreSubmodules`, `submodule.recurse`, `fetch.recurseSubmodules`).
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
# router started: 3 workers, strategy dependency
# [w1] TKT-001 started (board, base 356c934e, attempt 1)
# [w1] TKT-001 done (a3e62ba7)
# [w1] TKT-002 started (direct, base a3e62ba7, attempt 1)
# [w1] TKT-002 done (a6ebf42e)
# router stopped (completed)
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
  # bus:                      # slots do barramento b1..bN; SÓ na config global ou -c (ignorado na do repositório)
  #   channel: feature-x
  #   as: router              # a sessão CLI do router (tb bus join --role router)
  #   slots: 1
  #   to: role:worker         # ou agent:<nome>
  #   claim_timeout: 5m
```

**Arquivos de runtime** (`.toolbelt/router/`, no `.gitignore`):

| Caminho | Conteúdo |
|---------|----------|
| `state.json` | PID, estratégia, workers (idle/working, ticket, assinaturas; slots do barramento também thread, agente, fase e `claimed_at`) e fila (ready/blocked/running); atualizado a cada mudança |
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
| `kill <worker>` | `--reason` | Grava `control/kill-<worker>.json`; o router cancela o ticket em execução (conta como tentativa). Erro se não há router em execução. Num slot do barramento (`bN`) o router revoga a thread; o harness remoto para o próprio filho |

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
| `bus_unclaimed` / `bus_claim_rejected` / `bus_lease_lost` / `bus_verdict_fail` | warn | slots do barramento: nenhum claim válido no prazo; um claim sem o marker, de quem não é worker ou não é o destinatário, ou anterior ao assign; o claim trocou de mãos, foi liberado ou o dono não está vivo; o agente informou FAIL ([bus-router.md](bus-router.md)) | `none` |
| `bus_channel_expired` / `bus_write_failed` | warn | o canal do barramento expirou (slots do barramento parados); uma escrita no barramento falhou (canal expirado: a tentativa volta para a fila) | `none` |

**Exemplo:**

```bash
tb watchdog alerts --kind worker_stuck
# 11:57:56  critical worker_stuck   w1 TKT-001     TKT-001 on w1 exceeded the timeout of 2s; canceling [action: retry]
# 11:57:59  critical worker_stuck   w1 TKT-001     TKT-001 on w1 exceeded the timeout of 2s; canceling [action: escalate]
```

---

### `tb merge` — Agregação de Resultados (✓ Fase 3)

Integra os commits dos tickets `done` (`base_commit..head_commit`) numa
worktree temporária destacada e só então avança o branch alvo.

```bash
tb merge TKT-001 TKT-002
tb merge --dry-run TKT-001 TKT-002
tb merge --strategy last_wins TKT-001 TKT-002
tb merge --strategy semantic TKT-001 TKT-002
tb merge --strategy prompt TKT-001 TKT-002
tb merge --strategy prompt --resolutions 3f2a9c1d5b7e4a60 TKT-001 TKT-002
tb merge --into integracao TKT-001 TKT-002
tb merge --output changes.patch TKT-001 TKT-002
tb merge --force TKT-001
tb merge --allow-sensitive TKT-001
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--strategy fail\|last_wins\|semantic\|prompt` | Conflito (padrão `fail`) |
| `--resolutions <id>` | Só `prompt`: reaplica o recibo de uma proposta anterior em vez de chamar o agente |
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
| `semantic` | Resolve os hunks que o git deixou em conflito com regras determinísticas e relata cada um (ver abaixo); se algum hunk ficar sem resolução, age como `fail` |
| `prompt` | Roda `semantic` e depois um agente one-shot em sandbox nos hunks restantes; **duas etapas**: a proposta grava um recibo e nunca move uma ref, `--resolutions` o aplica |

**`semantic`** (spec: [merge-strategies](specs/merge-strategies.md)): para cada arquivo não mesclado o tb lê os estágios do índice e roda `git merge-file --diff3` (sem atributos, sem merge driver), depois tenta duas regras por região em conflito: `disjoint` (os dois lados editam linhas disjuntas da base: as duas edições se aplicam) e `go_imports` (imports diferentes num mesmo bloco `import (...)` de Go: a união, menos os specs que um lado removeu, ordenada por caminho). Um arquivo resolvido não pode ter linha de marcador de conflito que as entradas não tinham, e um arquivo Go que fazia parse antes precisa continuar fazendo (`parse_failed`). Nunca resolvidos, seja qual for a flag: conflitos binário (`binary`), symlink, gitlink, delete/modify (`delete_modify`), add/add (`add_add`), modo (`mode`) e em caminho sensível (`sensitive`, mesmo com `--allow-sensitive`), e arquivos acima de `merge.semantic.max_file_bytes` (`too_large`). Edições sobrepostas dão `overlap`; região acima de `merge.semantic.max_region_lines` dá `region_too_large`. Quando todos os conflitos resolvem, aplica numa chamada, como `fail`/`last_wins` (`--dry-run`, `--output` e o gate de sensíveis inalterados). JSON: `conflicts[].resolution` é `semantic`, `reason` e `hunks` (`region`, `ours_lines`, `theirs_lines`, `rule` ou `reason`) dizem como ou por que não; `files[].status` é `semantic`; `resolutions[]` traz uma entrada por região resolvida com `rule` e o sha256 das linhas de base, ours, theirs e resultado; `result_tree` é a árvore integrada.

**`prompt`**: depois do `semantic`, as regiões restantes vão ao agente do egm (`egm.agent_cmd` e as demais chaves de agente, lidas só da config do usuário; não há segunda config de agente), uma execução em sandbox por arquivo: o mesmo Landlock, subreaper, `HOME` descartável e perfil de vendor do `tb egm --engine llm`, workdir vazio somente leitura, prompt no stdin. Sem Landlock recusa (`no_landlock`) e nunca recua; sem `egm.agent_cmd` a proposta falha com `agent_unavailable`. O agente não toca o disco: devolve um objeto JSON `{nonce, regions:[{id,text}]}` (ou `{nonce, refuse}`) e o tb costura os textos nas regiões em conflito. As respostas são validadas (`agent_protocol` nonce/IDs/codificação, `agent_oversized`, `agent_markers`, `parse_failed`, `agent_secret`, `agent_refused`, `agent_novel`). Nunca vão ao agente: caminhos sensíveis, caminhos da deny-list (`secret_path`) e arquivos cujas regiões ou contexto casam um padrão de segredo (`secret_in_context`); o tb recusa em vez de redigir. Linhas resolvidas que não existem em base, ours nem theirs são relatadas como `novel_lines` (o teto é `merge.prompt.max_novel_lines`, sem valor por padrão). Falhas transitórias (`timeout`, `stdout_overflow`, `missing_output`, `exit_<n>`) são repetidas (`merge.prompt.attempts`); `tools_used`, `escaped_process`, `unattributed_orphan`, `sandbox_error` e execução interrompida ou acima do orçamento abortam o merge inteiro sem recibo. A proposta imprime e devolve `receipt` (`id`, `path`) e `result_tree`, qualquer que seja o `--dry-run`. O recibo é um JSON 0600 em `$XDG_STATE_HOME/toolbelt/merge/<hash do repo>/` (nunca no repositório; o ID são os 16 primeiros hex do sha256) com os tickets, a identidade do agente, os registros de execução (sem stderr, sem ambiente), as resoluções com o texto e a árvore resultante; leia com `cat`/`jq`, recibos com mais de 30 dias são varridos. `--resolutions <id>` reintegra, reaplica cada região do agente sem iniciar nenhum agente e falha antes de mover uma ref com `receipt_tampered` (bytes alterados), `receipt_stale` (alvo, SHA inicial ou tickets mudaram: refaça a proposta), `receipt_mismatch` (hashes de entrada de uma região ou a árvore resultante diferem) ou `receipt_incomplete` (a proposta deixou conflitos). Orçamentos (só config do usuário, seção `merge:`): `merge.semantic.max_region_lines` (2000), `max_file_bytes` (1 MiB); `merge.prompt.max_files` (10), `context_lines` (20), `max_prompt_bytes` (256 KiB, acima disso `prompt_budget`), `timeout_per_file` (5m), `max_total` (20m), `attempts` (2), `max_stdout_bytes` (1 MiB), `max_novel_lines`. Um `.toolbelt/config.yaml` do repositório que defina `merge.*` ou as chaves de agente do egm é ignorado com aviso `config_untrusted`. O MCP `tb_merge` aceita `semantic` e recusa `prompt` com `prompt_cli_only`.

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
# FILE           TICKETS  STATUS
# f-TKT-001.txt  TKT-001  merge
# f-TKT-002.txt  TKT-002  merge
# result: a6ebf42ee47be5c4ca290b2055efe75b00f8eaf6 (start 356c934e)
tb merge TKT-002 TKT-003
# merge TKT-002 TKT-003 -> main (fail) [not applied]
# FILE     TICKETS          STATUS
# a.txt    TKT-002,TKT-003  conflict
# Error: merge conflict: 1 file(s) in conflict; no ref was changed (use --strategy last_wins to resolve)
tb merge --strategy semantic --dry-run TKT-004 TKT-005
# merge TKT-004 TKT-005 -> main (semantic) [dry-run]
# FILE      TICKETS          STATUS
# shared.go TKT-004,TKT-005  semantic
# conflict: shared.go (TKT-005, commit 1a2b3c4d; also: TKT-004) - resolved by semantic (go_imports)
# result: 9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b (start 356c934e)
# result tree: 0f1e2d3c4b5a69788796a5b4c3d2e1f001122334
```

---

### `tb bus` — Barramento entre Agentes (✓ B1, ADR-013)

Barramento local para agentes de vendors diferentes (Claude Code, Codex, Grok, ...) trocarem mensagens `bus/0.1` pelo mesmo store (`.toolbelt/bus/bus.db`, SQLite WAL, sem daemon). Spec do envelope, Agent Card e convite: [bus-protocolo.md](bus-protocolo.md). Lease, heartbeat e watchdog (B2): [bus-lease.md](bus-lease.md); o harness `tb bus run` (B3) está em [bus-harness.md](bus-harness.md).

```bash
tb bus channel create feature-x --owner claude-main --ttl 2h
tb bus invite feature-x --role worker
tb bus invite feature-x --role supervisor --name claude-main
tb bus join feature-x --invite K7Q2-9XMD --name grok-1 --vendor xai --role worker
tb bus send --channel feature-x --as claude-main --to role:worker --type task.assign --thread TKT-001 --payload '{"ticket":"TKT-001"}'
tb bus recv --channel feature-x --as grok-1 --wait 20s
tb bus claim TKT-001 --channel feature-x --as grok-1
tb bus complete TKT-001 --channel feature-x --as grok-1 --epoch 1 --result '{"verdict":"PASS"}'
tb bus log feature-x --thread TKT-001
tb bus channel list
tb bus channel show feature-x
tb bus heartbeat --channel feature-x --as grok-1
tb bus watchdog feature-x --dry-run
tb bus channel extend feature-x --ttl 2h
tb bus run --channel feature-x --as marin --check
tb bus run --channel feature-x --as marin
tb bus run --channel feature-x --as claude-code --once
tb bus discover
```

| Comando | Quem | Função |
|---|---|---|
| `channel create <channel> --owner N` | humano | cria canal (`--ttl`, padrão sem expiração; `--lease`, padrão `2m`, `30s`–`1h`) e convite do dono (papel `supervisor`; `--invite-ttl`, padrão `10m`) |
| `channel list` / `channel show <channel>` | humano | lista canais / mostra o lease do canal, os agentes (papel, vivo, `last_seen`) e os claims abertos (dono, epoch, seguro/expirado/liberado); sem `session` do Agent Card, token nem payload |
| `channel extend <channel> --ttl D` | humano | redefine a expiração para agora + `D` (`0` = sem expiração); reabre canal expirado mantendo sessões e histórico |
| `invite <channel> --role R` | humano | convite de uso único com papel fixado (`--ttl`, padrão `10m`); `--name` vincula a um agente e, se ele já estiver no canal, recupera a sessão perdida (token novo) |
| `join <channel> --invite C` | agente | entra com Agent Card (`--name`, `--vendor`, `--model`, `--tier`, `--wake`, `--session`, `--capability` ou `--card card.json`); `--role` precisa coincidir com o papel do convite; grava a sessão no diretório privado do bus, fora do repositório (`$XDG_STATE_HOME/toolbelt/bus/`, `0600`) |
| `send` | agente | `--to agent:\|role:\|topic:`, `--type`, `--thread`, `--causation`, `--idempotency-key`, `--ttl`, `--payload` (JSON ou `@arquivo`) |
| `recv` | agente | lê e avança o cursor; `--wait` (máx. 25s), `--limit`, `--peek`, `--since` |
| `claim <thread>` | agente | dono único da thread; imprime o `epoch`; toma um claim liberado ou cujo dono perdeu o lease |
| `complete <thread>` | dono | `--epoch` (obrigatório, vindo do `claim`), `--result`; publica `task.result` em `topic:board`; `lease_lost` quando o claim trocou de mãos ou foi liberado |
| `heartbeat` | agente | renova o lease; lista threads seguras (com epochs) e perdidas (com o novo dono) |
| `watchdog <channel>` | humano | libera claims de agentes não vivos e publica `task.status` `lease_expired` de `system:watchdog` em `topic:board`; `--dry-run` só lista. Não exposto no MCP |
| `log <channel>` | humano | trilha de auditoria (payload redigido); `--thread`, `--limit` |
| `run --channel C --as N` | humano | harness B3: espera sem LLM e acorda o agente pelo arquivo de wake (`exec`, `harness` ou `notify`, que avisa o humano para agentes só de GUI); `--once`, `--check`, `--dry-run`, `--invite`, `--role`, `--config`, `--allow-unsandboxed`. Spec: [bus-harness.md](bus-harness.md), [bus-notify-discover.md](bus-notify-discover.md) |
| `discover` | humano | catálogo só de leitura das CLIs de agente conhecidas no `PATH` e das configs de cliente MCP que rodam `tb mcp` (outros servidores só contados), com próximos passos; não executa nada; `--cwd`, `--json` |

Comandos de agente exigem `--channel` e `--as`. O token de sessão nunca é impresso. `--json` devolve envelopes JSON-RPC.

---

### `tb bench` — Benchmark ✅ IMPLEMENTADO (A/B na Fase 2)

Executa benchmarks e mede KPIs. Suporta comparação A/B entre configs/feature flags.

```bash
tb bench <subcommand>

Subcommands:
  run       Runs the benchmark ✅
  compare   Compares two benchmarks or A/B configs ✅
  report    Generates a Markdown report of results ✅
  list      Lists available tasks ✅
```

**`tb bench run`:**

```bash
tb bench run [flags]

Flags:
  --fake               Uses the fake agent built into the `tb` binary (portable, any repo)
  --fake-config <path> Fake agent configuration file
  -a, --agent <cmd>    External agent command (fallback: bench.default_agent)
  -t, --tasks-dir      Tasks directory (fallback: bench.tasks_dir)
  -f, --fixtures-dir   Fixtures directory (fallback: bench.fixtures_dir)
  -r, --repetitions <n> Repetitions per task (default: 1)
  -o, --output <path>  JSONL output file (without the flag: under bench.output_dir)
  --label <name>       A/B variant label
  --flag key=value     Feature flag (repeatable)
  --ab-config <yaml>   A/B variant from a YAML file
  --task <id>          Filters tasks
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
tb mcp [flags]

Flags:
  --bus-as <channel>/<name>  Binds the connection to the bus session <channel>/<name> (used by tb bus run)
  --bus-root <path>          Bus repository root (default: searches for .toolbelt from the cwd)
  --bus-state <dir>          Base of the bus private directory (default $XDG_STATE_HOME/toolbelt/bus)
```

Transporte unix/tcp e flags `--socket`/`--port`/`--tls-*` são planejados; hoje só stdio.

**Ferramentas expostas em `tools/list` / `tools/call`** (36 na v0.0.5; as de grafo aparecem na lista, mas a chamada exige `TB_GRAPH=on`):

| Tool | Paridade CLI | Notas |
|------|--------------|-------|
| `tb_version` | `tb version` | |
| `tb_echo` | — | smoke |
| `tb_env` | — | cwd/os/arch |
| `tb_run` | — | `sh -c` com timeout dentro do sandbox Landlock (ver [sandbox do `tb_run` no MCP](#sandbox-do-tb_run-no-mcp)); env filtrado; args `env`/`pass_env` (arrays); `cwd` precisa estar dentro do repositório |
| `tb_mem_add` / `tb_mem_list` / `tb_mem_show` / `tb_mem_edit` / `tb_mem_retire` / `tb_mem_search` / `tb_mem_check` / `tb_mem_init` | `tb mem …` | arg opcional `cwd`; `ref`/`all` (boolean; strings legadas ok) onde aplicável |
| `tb_verify_run` | `tb verify run` | `ruler=tests\|visual`; args espelham flags + `env`/`pass_env` |
| `tb_review` | `tb review` | ticket/base/head/approved_sha/approved_base/require_approved_sha/reviewers/standards |
| `tb_drift` | `tb drift` | retorna `{drift, has_drift, pinned_approval}` (`checked` explícito); `approved_sha`/`approved_base` |
| `tb_guard_check` | `tb guard check` | `base`/`head` ou `range=A..B`; `strict_fail` por default em SeverityError; `warn_only=true` opt-out; `strict` deprecated (boolean) |
| `tb_stack` | `tb stack` | `ops` (array, inclui `egm`, sempre movida para o fim), `base`/`head`, `fail_fast` (boolean), `jobs` (integer), `approved_sha`/`approved_base`/`require_approved_sha`, `env`/`pass_env`, `egm_docs` (array), `strict` (boolean), `egm_mode` (`local`/`ci`), `egm_config` (caminho absoluto, exigido por `ci`), `egm_allow_offline` (boolean); retorna JSON agregado com `warn` |
| `tb_egm_status` / `tb_egm_resolve` / `tb_egm_receipt` | `tb egm status` / `tb egm resolve` / `tb egm receipt` | `cwd` (absoluto) e `doc`; status aceita `at`/`strict`, resolve aceita `fix`/`decline` (arrays), `force_settle`/`reason`, `contest`/`evidence`; retorna `{report, exit_code}` (receipt acrescenta `receipt`, o caminho gravado) |
| `tb_vault_list` / `tb_vault_show` / `tb_vault_search` / `tb_vault_links` / `tb_vault_backlinks` / `tb_vault_check` / `tb_vault_new` | `tb vault …` | `cwd` (absoluto); `note` em show/links/backlinks, `query`/`limit` em search, `type`/`tag`/`source` em list; new recebe `title`, `confidence` (obrigatório), `type`, `tags`, `dir`, `body` e grava sempre `source: inferred`; o texto das notas volta dentro de `data` |
| `tb_ticket_list` / `tb_ticket_show` | `tb ticket list` / `tb ticket show` | `status`, `deps` / `id`; arg opcional `cwd` |
| `tb_ticket_create` | `tb ticket create` | `title`, `description`, `scope`/`forbid`/`accept`/`dep` (arrays), `priority` (integer), `timeout`, `dry_run` (boolean) |
| `tb_board` | `tb board` | `action=list\|pause\|resume\|drain`, `strategy`, `status`, `worker` |
| `tb_router_status` | `tb router status` | só leitura de `state.json` |
| `tb_watchdog_alerts` | `tb watchdog alerts` | `since`, `kind`, `limit` (integer) |
| `tb_merge` | `tb merge` | `ids` (array), `strategy` (`fail`, `last_wins`, `semantic`; `prompt` é recusada com `prompt_cli_only`), `into`, `force`, `allow_sensitive` (padrão `false`); **dry-run por padrão** — só altera refs com `apply=true` |
| `tb_bus_join` / `tb_bus_send` / `tb_bus_recv` / `tb_bus_claim` / `tb_bus_complete` / `tb_bus_heartbeat` / `tb_bus_status` | `tb bus …` | identidade presa à conexão após `tb_bus_join` (`channel`, `invite`, `name` + Agent Card); `payload`/`result` como objeto; `tb_bus_complete` exige o `epoch` inteiro; o `tb mcp` manda heartbeat das sessões ligadas em segundo plano; sem tool de watchdog; `wait` string (máx. 25s); criar canal/convite **não** existe via MCP; `tb mcp --bus-as <channel>/<name>` liga a conexão a uma sessão CLI (usado por `tb bus run`) |
| `tb_graph_search` / `tb_graph_callers` / `tb_graph_deps` / `tb_graph_impact` / `tb_graph_explain` / `tb_graph_path` / `tb_graph_index` | `tb graph …` | chamada requer `TB_GRAPH=on`; `depth`/`limit`/`max_depth` integer; `transitive` boolean; `explain` usa `node`, `path` usa `from`/`to`; `mode` no `tb_graph_search` e `vectors` no `tb_graph_index` (veja o [índice vetorial](#índice-vetorial)) |

Schemas tipam `depth`/`limit`/`reviewers`/`max_iter` como integer, `strict`/`warn_only`/`transitive`/`skip_unavailable`/`all` (mem) como boolean, `threshold` como number e `standards` como array de strings; valores string legados ainda são aceitos. Relatórios de verify/review/drift/guard voltam como JSON no `content` (sem código de saída de processo). Listas vazias usam `[]`; `hidden_passed` é `null` se a suíte oculta não se aplica.

---

### `tb init` ✅ IMPLEMENTADO

```bash
tb init [flags]

Flags:
  --global         Initializes global configuration (~/.config/toolbelt)
  --write-agents   Seeds a minimal AGENTS.md if missing (never overwrites)
  --json           JSON output (global flag)
```

Com `--json`, emite payload com `path`, `test_cmd` / `build_cmd` / `lint_cmd` detectados
(quando presentes), `warning` opcional e `agents` (`seeded`, `sources`, …).

Detalhes de detecção de toolchain e preenchimento de `verify.*` estão na seção `tb verify`.

**Estado local vs config (`.git/info/exclude`):** em repositório Git (sem `--global`),
o init escreve em `.git/info/exclude` (não altera o `.gitignore` do projeto) os
padrões de **cache/derivados**:

- `.toolbelt/graph.json`
- `.toolbelt/logs/`
- `.toolbelt/workspaces/`
- `.toolbelt/captures/`
- `.toolbelt/bench/`
- `.toolbelt/bus/`
- `<vault.path>/.obsidian/` (o vault em si é versionado; o `tb init` cria um `vault.path` customizado e avisa `vault_gitignored` quando uma regra do `.gitignore` o ignora)

Num repositório inicializado antes de o vault ser versionado, rode `tb init` de novo: a linha antiga `.toolbelt/vault/` sai da seção, e uma seção com o marcador de fim perdido mantém as suas linhas.

Numa `git worktree`, o arquivo é o `info/exclude` do diretório comum
(`$GIT_COMMON_DIR`), o único que o git lê. Os arquivos versionáveis
(`config.yaml`, `agent_docs.yaml`, `invariants.yaml`, `AGENTS.md`) são criados
com modo `0644` (sujeito ao umask).

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
#   embed:  llama.cpp b12600 (29b52d398b29)

tb version --json
# {"version":"dev","commit":"unknown","build_time":"unknown","go_version":"go1.22.2","embed":"llama.cpp b12600 (29b52d398b29)"}
```

Um build de release; um build sem `-tags embednative` (`go build ./cmd/tb`) imprime `embed:  unavailable (built without embednative)`.

Todas as saídas `--json` da CLI (e os payloads de domínio embutidos nas tools MCP)
usam chaves **`snake_case`** de forma consistente (`exit_code`, `base_ref`,
`timed_out`, …). Envelopes do protocolo MCP (`isError`, `inputSchema`, …)
permanecem no camelCase exigido pela spec.

---

## Cores e Tema Visual

A saída colorida do CLI segue o tema **Synthwave** definido em [docs/pt-BR/identidade-visual.md](identidade-visual.md). As cores são aplicadas automaticamente quando o terminal suporta.

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
| `TB_BUS_STATE_DIR` | Diretório privado do bus (sessões, wake, estado do harness) | `$XDG_STATE_HOME/toolbelt/bus/<hash>/` |

Variáveis planejadas (ainda não lidas pela CLI): `TB_VAULT`, `TB_LOG_LEVEL`,
`TB_SANDBOX_DEFAULT`, `TB_JSON_OUTPUT`, `TB_TIMEOUT_DEFAULT`, `TB_COLOR`,
`TB_COLOR_MODE`.

### Allowlist de ambiente nos processos filhos

`tb verify` / `tb stack` (etapa verify), MCP `tb_run` / `tb_verify_run` / `tb_stack`
não repassam o ambiente completo ao `sh -c`. Só passam chaves da allowlist embutida
(PATH, HOME, LANG, TMPDIR, toolchain Go/Node/npm/pnpm, paths de navegador
(`PLAYWRIGHT_BROWSERS_PATH`, `PUPPETEER_CACHE_DIR`, `PUPPETEER_EXECUTABLE_PATH`, …), proxies, XDG, etc.), mais:

| Fonte | Efeito |
|-------|--------|
| `.toolbelt/config.yaml` → `env.allow` | Estende a allowlist |
| `.toolbelt/config.yaml` → `env.pass` | Opt-in explícito de segredos |
| Flag global `--env KEY` / arg MCP `env` | Estende a allowlist (não contorna segredos) |
| Flag global `--pass-env KEY` | Opt-in explícito de segredos |
| Arg MCP `pass_env` | Só chaves já presentes em `env.pass` da config **global** (`~/.config/toolbelt/config.yaml` ou `-c`); qualquer outra é recusada (o modelo não libera segredos, e a config do próprio repositório não conta porque o agente pode escrevê-la) |

Segredos: qualquer nome contendo `TOKEN`, `SECRET`, `KEY` ou `PASSWORD`
(case-insensitive) é bloqueado sem opt-in. No `tb_run` do MCP, o `env.pass` só é
lido da config global.

### Sandbox do `tb_run` no MCP

O `tb_run` do MCP roda `sh -c` sob Landlock, como `tb router` e `tb bus run`.
O filho pode:

- **escrever** na raiz do repositório (o diretório com `.toolbelt`, ou o
  diretório de trabalho do servidor), no próprio `TMPDIR` (criado a cada chamada e
  apagado depois) e nos caches de toolchain que existirem: `GOCACHE`, `GOMODCACHE`,
  `$GOPATH/pkg/sumdb`, os caches de npm, pip e uv, `$CARGO_HOME/registry`;
- **ler** os diretórios do sistema (`/usr`, `/etc`, `/lib`...), toda entrada do
  `PATH` exceto `/`, `$HOME` e seus ancestrais, o `GOROOT` e o diretório do
  arquivo `GOENV`;
- nada além disso: `$HOME` (`~/.ssh`, `~/.gitconfig`, histórico do shell), o `/tmp`
  compartilhado e outros repositórios ficam fechados. O git funciona, mas avisa
  que não consegue ler `~/.gitconfig`.

O `cwd` precisa ser a raiz do repositório ou estar abaixo dela. Caminhos extras e
o opt-out vêm só da config global, porque o repositório (e o
`.toolbelt/config.yaml` dele) é gravável de dentro do sandbox:

```yaml
# ~/.config/toolbelt/config.yaml
sandbox:
  read: [~/.nvm]              # só leitura, absoluto ou ~/ (precisa existir)
  write: [~/.cache/my-tool]   # leitura e escrita
  allow_unsandboxed: false    # true: roda sem isolamento quando não há Landlock
```

Sem Landlock (macOS, Windows, kernels sem o LSM), o `tb_run` falha fechado, a
menos que `sandbox.allow_unsandboxed: true`. O resultado traz
`sandboxed: true|false`. A rede não é restringida.

---

## Erros e Usage

- Erros esperados de domínio/runtime (`RunE`, ex.: flag de negócio obrigatória ausente, falha de git, arquivo inexistente) saem com **exit ≠ 0** e mensagem em stderr **sem** dump de `Usage` do Cobra.
- Erros genuínos de parse de **flags** ou **args** (flag desconhecida, `ExactArgs`/`MinimumNArgs`, etc.) mantêm `Usage` no stderr para orientar a invocação.
- Subcomando desconhecido em qualquer nível (ex.: `tb mem nao-existe`, `tb bench nope`) sai com **exit ≠ 0** e erro `unknown command "…"`, em vez de imprimir help com exit 0.
- Falhas de git em `guard` / `review` / `drift` / `verify` (e no `stack` que os empilha) **não** mostram só `exit status 128/129`: a mensagem inclui o stderr do git, a ref ausente (`ref missing: …`) e uma dica `git fetch origin <ref>`. Para uma ref relativa (`HEAD~2`, `main^`), a dica fala de histórico curto (repo de um commit ou clone raso: `--base <sha>` ou `git fetch --deepen=1`).
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
