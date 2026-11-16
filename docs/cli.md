# Árvore de Comandos do `tb`

Este documento descreve a CLI do toolbelt: comandos **implementados** (espelham `tb --help`) e seções **planejadas** (Fase 3+, marcadas explicitamente). Em caso de divergência, a CLI Cobra é a fonte da verdade.

## Visão Geral

```
tb  (implementado hoje)
├── init         Inicializa configuração local/global
├── bench        Harness de benchmark (run|list|report|compare)
├── drift        Drift de escopo / commit base
├── graph        Grafo sob demanda (TB_GRAPH=on ou --enable)
├── guard        Proteções anti-trapaça (check)
├── mcp          Servidor MCP sobre stdio
├── mem          Invariantes fora da compactação (add|list|check|init)
├── review       Revisão fresh-context
├── sandbox      Worktrees + Landlock (check|run)
├── verify       Verificação adversarial (run; rulers tests|visual)
├── workspace    Workspaces via git worktree
├── version      Mostra versão
└── router       Stub (não implementado)

tb  (planejado — Fase 3+)
├── run / proc / md / agent
├── ticket / board / watchdog / merge
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

### `tb proc` — Gerenciamento de Processos

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
ative com `TB_GRAPH=on` ou `--enable`. Medir ROI via
`tb bench compare --config-a bench/ab/graph-off.yaml --config-b bench/ab/graph-on.yaml`.

Linguagens e docs (extensível via `LanguageIndexer`, regex leve, sem cgo):

| `graph.languages` | Extensões |
|-------------------|-----------|
| `go` | `.go` (AST; arestas `calls`) |
| `javascript` | `.js`, `.mjs`, `.cjs`, `.ts` (`.tsx`/`.jsx` via `react` quando ligado; arestas `calls`) |
| `python` | `.py` (defines/imports + arestas `calls` resolvíveis no repositório) |
| `java` | `.java` |
| `kotlin` | `.kt`, `.kts` |
| `csharp` | `.cs` |
| `cpp` | `.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.hpp`, `.hh`, `.hxx` |
| `rust` | `.rs` |
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
| `react` | `.tsx`, `.jsx` | componentes, hooks `use*`, arestas `renders` |
| `vue` | `.vue` | SFC script/template, imports, componentes no template |
| `svelte` | `.svelte` | script, imports, componentes no markup |
| `astro` | `.astro` | frontmatter, imports, componentes no body |
| `angular` | `*.component.ts` / `.html`, services/modules | `@Component`/`@Injectable`/`@NgModule`, selector, `templateUrl` |
| `routes` | Next `app/`/`pages/`, SvelteKit `routes/`, Nuxt `pages/` | nós `route` por convenção de arquivo |
| `mdx` | `.mdx` | doc (headings/links) + imports/componentes JSX |

Frameworks têm prioridade sobre o indexador genérico de linguagem no mesmo
arquivo (ex.: `.tsx` com `react` ligado não cai só como JS).

```yaml
# .toolbelt/config.yaml (todos ligados por padrão)
graph:
  languages: [go, javascript, python, java, kotlin, csharp, cpp, rust, php, ruby, swift, shell]
  docs: [markdown, html, txt]
  frameworks: [react, vue, svelte, astro, angular, routes, mdx]
  max_file_bytes: 524288   # ignora arquivos maiores
```

Ignora subdiretórios como `node_modules`, `vendor`, `dist`, `build`,
`target`, `.git`, etc. A raiz do walk nunca é pulada pelo nome (ex.: indexar
`/tmp/target` funciona; um `target/` filho continua ignorado). O grafo
continua sob demanda (ADR-011): `TB_GRAPH=on` ou `--enable`.

```bash
tb graph [--enable] <subcommand>

Subcomandos:
  search    Busca no grafo ✅
  callers   Encontra chamadores de uma função ✅
  deps      Mostra dependências ✅
  impact    Analisa impacto de mudanças ✅
  index     Indexa código sob demanda ✅
  rebuild   Reconstrói grafo completo ✅
  stats     Estatísticas do grafo ✅
```

Ferramentas MCP (quando `TB_GRAPH=on`): `tb_graph_search`, `tb_graph_callers`,
`tb_graph_deps`, `tb_graph_index`.

**`tb graph search`:**

```bash
tb graph search [flags] <query>

Flags:
  --type <types>      Tipos de nós: file,function,class,fact,decision
  --source <sources>  Proveniência: extracted,inferred,manual
  --limit <n>         Máximo de resultados (default: 20)
  --json              Saída em JSON
```

**`tb graph callers`:**

```bash
tb graph callers [flags]

Flags:
  --function <name>   Nome da função
  --file <path>       Arquivo da função
  --depth <n>         Profundidade do grafo (default: 1)
  --transitive        Inclui chamadores indiretos
```

**`tb graph deps`:**

```bash
tb graph deps [flags]

Flags:
  --module <name>     Nome do módulo/pacote
  --file <path>       Arquivo para analisar
  --transitive        Inclui dependências transitivas
  --direction <dir>   upstream|downstream|both
```

**`tb graph impact`:**

```bash
tb graph impact [flags]

Flags:
  --file <path>       Arquivo modificado
  --function <name>   Função modificada
  --depth <n>         Profundidade da análise
```

**Exemplos:**

```bash
# Busca semântica
tb graph search "autenticação JWT"
# NODE_ID      TYPE       PATH                      TITLE
# node-123     function   src/auth/jwt.go          ValidateToken
# node-124     fact       vault/facts/auth.md      Decisão: JWT para auth
# node-125     class      src/auth/claims.go       Claims

# Chamadores de uma função
tb graph callers --function "ValidateToken" --depth 2
# ValidateToken
# ├── AuthMiddleware (src/middleware/auth.go:45)
# │   ├── HandleRequest (src/server/handler.go:23)
# │   └── WebSocketAuth (src/ws/auth.go:12)
# └── RefreshHandler (src/auth/refresh.go:78)

# Dependências de um módulo
tb graph deps --module "auth" --transitive
# auth
# ├── crypto/jwt
# ├── database/users
# │   └── database/core
# └── config

# Impacto de mudança
tb graph impact --file "src/config/parser.go"
# Arquivos potencialmente afetados:
# - src/server/init.go (imports config)
# - src/auth/config.go (imports config)
# - src/database/config.go (imports config)
# Total: 15 funções, 8 arquivos

# Re-indexar código após mudanças
tb graph index --path "src/"

# Reconstruir grafo completo
tb graph rebuild

# Estatísticas
tb graph stats
# Nós:      1,234 (845 extracted, 312 inferred, 77 manual)
# Arestas:  5,678 (4,892 extracted, 786 inferred)
# Arquivos: 156
# Funções:  892
# Classes:  123
```

---

### `tb mem` — Invariantes (✓ Fase 1)

Gerencia o arquivo `.toolbelt/invariants.yaml` (fora da compactação, ADR-010).

```bash
tb mem <subcommand>

Subcomandos:
  add       Adiciona regra invariante
  list      Lista invariantes
  check     Verifica integridade (hash e retenção)
  init      Cria conjunto padrão anti-trapaça

Subcomandos (planejados — vault Markdown):
  search    Busca na memória
  show      Exibe entrada
  show      Exibe entrada específica
  edit      Edita entrada
  link      Cria link entre entradas
  tag       Gerencia tags
  export    Exporta memória
  import    Importa memória
  sync      Sincroniza vault → SQLite
```

**`tb mem add` (implementado):**

```bash
tb mem add <regra> [flags]

Flags:
  --category <cat>    Categoria: anti_cheat|mandate|security (default: anti_cheat)
```

**`tb mem list` (implementado):**

```bash
tb mem list [flags]

Flags:
  --category <cat>    Filtrar por categoria
```

**Exemplos:**

```bash
# Adicionar invariante (argumento posicional obrigatório)
tb mem add "Não modificar tests/hidden/**"
tb mem add --category security "Não relaxar thresholds sem revisão"

# Listar / checar / seed anti-trapaça
tb mem list
tb mem list --category anti_cheat
tb mem check
tb mem init
```

> Subcomandos de vault Markdown (`search`, `show`, `edit`, `link`, `tag`, `export`, `import`, `sync`) e flags `--type`/`--title` **não existem** na CLI atual — são planejados.

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

### `tb agent` — Gerenciamento de Agentes

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

### `tb guard` — Proteções de Segurança (✓ implementado: `check`)

Analisa o diff entre commits em busca de paths protegidos e padrões anti-trapaça.

```bash
tb guard check <base>..<head> [flags]

Flags:
  --strict    Exit ≠ 0 quando há violações de severidade erro
  --json      Saída em JSON (flag global)
```

**Exemplos:**

```bash
tb guard check HEAD~1..HEAD
tb guard check --strict HEAD~1..HEAD
tb guard check --json HEAD~1..HEAD
```

> Subcomandos `snapshot`, `verify`, `protect` e `audit` **não existem** na CLI atual — são planejados.

---

### `tb review` — Revisão Fresh-Context (✓ Fase 2)

Implementa o padrão Elephant-Goldfish: N goldfish revisam o diff contra
`ticket/rd.md` e invariantes de `tb mem`, sem herdar contexto do autor.

```bash
tb review [ticket] [flags]

Flags:
  --base <ref>          Commit base (default: HEAD~1 ou base_commit do ticket)
  --head <ref>          Commit head (default: HEAD)
  --ticket <path>       Ticket/rd.md (default: rd.md)
  --reviewers <n>       Número de goldfish (default: 3)
  --standards <stds>    scope,invariants,completeness
  --output <path>       Relatório JSON
  --json                Saída em JSON
```

Também reporta drift de escopo e commit base. Isolamento fresco é obrigatório.

O ticket/rd.md aceita frontmatter YAML ou Markdown: seção `## Scope` / `## Escopo`
(ou `Scope:` / `Escopo:`) e `## Forbidden` / `## Proibido` (ou `Forbidden:` /
`Proibido:`) com listas (`-`, `*`, numeradas) e paths entre backticks.
Se o frontmatter YAML não declara `scope:`, o body Markdown ainda é lido para preencher o escopo.
Globs no scope/forbidden incluem `src/*.go`, `src/**`, `tests/hidden/**` e
padrões com `**` no meio (ex.: `src/**/foo.go`). Toques em paths do Forbidden
preenchem `forbidden_hit` no drift e geram finding critical no review.

Invariantes de path (ex.: `tests/hidden/`, `thresholds.yaml`) são casadas contra
arquivos **alterados** pelo mesmo matcher de path/glob (`tests/hidden/**`,
`tests/hidden/*`) — não por substring no texto do diff ou do ticket. Mencionar
`forbidden: tests/hidden/**` em `rd.md` sem tocar esse diretório não gera
finding critical.

**Exemplos:**

```bash
# Revisar documento de design
tb review docs/design-auth.md
# Spawning 3 fresh-context reviewers...
# 
# ═══════════════════════════════════════════════════════════════
# RELATÓRIO DE REVISÃO: docs/design-auth.md
# ═══════════════════════════════════════════════════════════════
# 
# CRÍTICOS (2):
# ┌────┬──────────────────────────────────────────────────────────┐
# │ #1 │ [security] Não especifica como tokens são invalidados   │
# │    │ Linha 45: "JWT tokens são emitidos..."                  │
# │    │ Revisores: 2/3 concordam                                │
# ├────┼──────────────────────────────────────────────────────────┤
# │ #2 │ [completeness] Falta tratamento de rate limiting        │
# │    │ Seção 3.2 menciona mas não detalha implementação        │
# │    │ Revisores: 3/3 concordam                                │
# └────┴──────────────────────────────────────────────────────────┘
# 
# WARNINGS (5): ...
# INFO (3): ...
# 
# Total: 2 críticos, 5 warnings, 3 info

# Revisar com roles específicos
tb review docs/api-spec.md --roles "api-reviewer,security-reviewer,ux-reviewer"

# Revisar PR
tb review pr:42 --reviewers 5 --standards security,completeness
# Fetching PR #42...
# Analyzing 12 files changed, 450 additions, 120 deletions
# Spawning 5 fresh-context reviewers...
# ...

# Revisar com resolução interativa
tb review docs/design.md --resolve
# ... (relatório)
# 
# ═══════════════════════════════════════════════════════════════
# ETAPA DE RESOLUÇÃO
# ═══════════════════════════════════════════════════════════════
# 
# [1/2] Crítico: Não especifica invalidação de tokens
#   (a) Aceitar - adicionar seção sobre invalidação
#   (b) Rejeitar - já coberto em outro doc
#   (c) Adiar - precisa mais contexto
#   Escolha [a/b/c]: a
#   
# Resolução salva. Execute `tb review --apply` para aplicar.

# Saída JSON para integração
tb review docs/spec.md --json --output review-results.json
```

**Padrões de revisão disponíveis**:

| Padrão | O que verifica |
|--------|---------------|
| `security` | Vulnerabilidades, vazamento de dados, autenticação |
| `clarity` | Ambiguidades, termos indefinidos, contradições |
| `completeness` | Requisitos faltantes, edge cases, fluxos incompletos |
| `consistency` | Alinhamento com código existente, convenções, outros docs |
| `testability` | Se os requisitos são testáveis |
| `feasibility` | Riscos de implementação, dependências |

**Isolamento obrigatório**: Se o sistema não conseguir criar contextos frescos para os revisores, o comando falha com erro ao invés de prosseguir com contexto contaminado.

---

### `tb drift` — Drift de Escopo/Base (✓ Fase 2)

Detecta mudanças fora do scope declarado no ticket/rd.md, toques em
`Forbidden`/`Proibido` (`forbidden_hit`) e divergência do commit base
(mandate). O parser de scope/forbidden é o mesmo do `tb review`
(YAML frontmatter ou Markdown `## Scope` / `## Forbidden` / listas / globs com `**`).

```bash
tb drift [ticket] [flags]

Flags:
  --base <ref>     Commit base (default: HEAD~1 ou base_commit do ticket)
  --head <ref>     Commit head (default: HEAD)
  --ticket <path>  Ticket/rd.md (default: rd.md)
  --output <path>  Saída JSON
  --json           Saída em JSON
```

**Exemplos:**

```bash
# Verificar drift de todo o vault
tb drift
# Analisando vault contra código...
# 
# ═══════════════════════════════════════════════════════════════
# RELATÓRIO DE DRIFT
# ═══════════════════════════════════════════════════════════════
# 
# DOCUMENTAÇÃO DESATUALIZADA (3):
# ┌─────────────────────────────────────────────────────────────────────────────┐
# │ vault/facts/auth-module.md                                                  │
# │ ├── Menciona: validateToken(string) -> bool                                 │
# │ ├── Código atual: ValidateToken(ctx, string) -> (Claims, error)             │
# │ └── Última sync: 2026-01-10 (15 dias atrás)                                 │
# ├─────────────────────────────────────────────────────────────────────────────┤
# │ vault/facts/database-schema.md                                              │
# │ ├── Menciona: tabela 'users' com 5 colunas                                  │
# │ ├── Código atual: tabela 'users' com 8 colunas                              │
# │ └── Colunas novas: created_at, updated_at, deleted_at                       │
# └─────────────────────────────────────────────────────────────────────────────┘
# 
# CÓDIGO SEM DOCUMENTAÇÃO (5):
# - src/api/handlers/webhook.go (handler novo)
# - src/services/notification.go (serviço novo)
# - src/database/migrations/003_add_audit.sql (migration nova)
# ...
# 
# DOCUMENTAÇÃO ÓRFÃ (1):
# - vault/facts/legacy-parser.md → código deletado em abc123
# 
# Resumo: 3 desatualizados, 5 não documentados, 1 órfão

# Verificar drift de arquivo específico
tb drift docs/api-spec.md
# Comparando docs/api-spec.md contra código...
# 
# Endpoints documentados: 15
# Endpoints no código: 18
# 
# FALTANDO NA DOC:
# - POST /api/v2/webhooks (src/api/routes.go:145)
# - DELETE /api/v2/sessions/:id (src/api/routes.go:162)
# - GET /api/v2/health/detailed (src/api/routes.go:23)

# Verificar drift desde último release
tb drift --since v0.5.0
# Analisando mudanças desde v0.5.0...
# 
# Arquivos de código modificados: 45
# Documentação potencialmente afetada: 12
# Documentação atualizada: 7
# Documentação possivelmente desatualizada: 5

# Saída em markdown para relatório
tb drift --format markdown --output drift-report.md

# Ignorar certos padrões
tb drift --ignore "vendor/**" --ignore "**/*_test.go"
```

**Níveis de profundidade**:

| Nível | O que verifica |
|-------|---------------|
| `shallow` | Existência de arquivos, nomes de funções/classes |
| `medium` | Assinaturas de funções, estrutura de tipos, imports |
| `deep` | Semântica de implementação, comportamento documentado vs real |

**Integração com CI**:

```bash
# No CI, falhar se houver drift crítico
tb drift --format json | jq '.critical_count' | xargs -I {} test {} -eq 0
```

---

### `tb verify` — Verificação Adversarial (✓ Fase 1/2)

Executa o loop gerador→discriminador com rulers plugáveis.

```bash
tb verify run [flags]

Flags:
  --ruler <type>           Tipo de ruler: tests|visual (default: tests)
  --test-cmd <cmd>         Testes visíveis (default: verify.test_cmd; `tb init` detecta go/npm/pnpm/yarn/pytest/cargo/make e também build_cmd/lint_cmd)
  --hidden-test-cmd <cmd>  Suite oculta (verify.hidden_test_cmd; se vazio e existir tests/hidden/: go test ./tests/hidden/...)
  --max-iter <n>           Máximo de iterações (default: 3)
  --timeout <dur>          Timeout (default: 10m)
  --threshold <float>      Threshold de similaridade visual (default: 0.95)
  --manifest <path>        Manifesto de telas (default: .toolbelt/screens.yaml)
  --skip-unavailable       Skip gracioso sem browser/refs/telas (default: true; veredicto SKIPPED)
  --base <ref>             Ref base
  --head <ref>             Ref head
  --output <path> / -o     Arquivo de saída JSON
  --json                   Saída em JSON (flag global)
```

`tb init` preenche `verify.test_cmd` a partir da toolchain do repositório
(`go.mod` → `go test ./...`, `package.json` + scripts.test → `npm`/`pnpm`/`yarn test`,
`pyproject.toml`/`uv.lock` → `pytest`/`uv run pytest`, `Cargo.toml` → `cargo test`,
`Makefile` com target `test`/`check` → `make test`/`make check`). Sem marcadores
reconhecidos, permanece o default Go.

Quando há marcadores claros, o init também preenche `verify.build_cmd` e
`verify.lint_cmd` de forma conservadora:

| Toolchain | `build_cmd` | `lint_cmd` |
|-----------|-------------|------------|
| `go.mod` | `go build ./...` | `.golangci.y*ml`/`.toml` → `golangci-lint run`; senão target `make lint` |
| `package.json` | script `build` → `npm run build` / `pnpm run build` / `yarn build` | script `lint` → análogo |
| `Cargo.toml` | `cargo build` | `cargo clippy` |
| Python (`pyproject`/`uv.lock`/…) | (vazio) | `[tool.ruff]`/`ruff.toml` → `ruff check .` (ou `uv run …`); senão mypy |
| só `Makefile` | target `build` → `make build` | target `lint` → `make lint` |

Ausência de script/config → campo omitido (verify continua só com `test_cmd`).

A suite oculta permanece escondida do agente (paths em `tests/hidden/` protegidos
pelo `tb guard`). O comando configurado em `verify.hidden_test_cmd` ou
`--hidden-test-cmd` **sempre executa**, mesmo sem o diretório `tests/hidden/`
(caso típico em monorepos JS/Python/Rust). Se o comando estiver vazio e o
diretório existir, usa-se `go test ./tests/hidden/...`. Sem comando e sem
diretório, a suite oculta é ignorada (`hidden_available: false`). Campos
opcionais `build_cmd` e `lint_cmd` rodam antes dos testes (nessa ordem) e
fazem o verify falhar se o comando sair com erro; ausentes = comportamento
anterior (só `test_cmd` + hidden).

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
```

Ruler `visual` (docs/verificacao-visual.md): captura headless + diff de pixels;
se Chromium/Chrome ou referências estiverem ausentes, faz skip gracioso.

**Exemplos:**

```bash
# Verificação com ruler de testes
tb verify run
tb verify run --test-cmd "go test ./..." --hidden-test-cmd "go test ./tests/hidden/..."

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
  --strict              Falha se qualquer path protegido foi modificado sem decisão
  --json                Saída em JSON
```

Aceita o intervalo posicional `base..head` (forma clássica) ou as flags `--base`/`--head`, alinhadas a `tb review` / `tb drift` / `tb verify`.

**Exemplos:**

```bash
# Verificar mudanças entre main e branch atual
tb guard check main..HEAD

# Mesmo intervalo via flags (como review/drift)
tb guard check --base main --head HEAD

# Defaults HEAD~1..HEAD quando nenhum intervalo é passado
tb guard check --json

# Modo strict para CI
tb guard check origin/main..HEAD --strict
# Exit code 1 se houver SeverityError

# Saída JSON (exit 0 mesmo com violações)
tb guard check main..HEAD --json

# Strict + JSON: imprime JSON válido e sai ≠ 0 se houver SeverityError
tb guard check origin/main..HEAD --strict --json
```

**Manifesto de paths protegidos (`.toolbelt/guard.yaml`):**

O guard carrega `protected_paths` de `.toolbelt/guard.yaml` (quando presente) e **une** aos defaults embutidos (`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`). Cada entrada pode ser uma string (`lib/help.js`) ou um objeto com `pattern`. Entrada com barra final (`secrets/`) protege o diretório e todos os filhos (prefixo). `secrets/*` continua válido para um nível via glob. Sem o arquivo, só os defaults valem.

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

### `tb ticket` — Gerenciamento de Tickets (planejado — Fase 3+)

> **Não implementado.** A CLI atual não expõe `tb ticket`. Use um `rd.md` manual consumido por `tb drift` / `tb review`.

Cria e gerencia tickets de trabalho para o padrão Router/Workers.

```bash
tb ticket <subcommand>

Subcomandos:
  create    Cria tickets a partir de tarefa
  list      Lista tickets existentes
  show      Exibe detalhes de um ticket
  cancel    Cancela ticket pendente
  retry     Reenfileira ticket falhado
```

**`tb ticket create`:**

```bash
tb ticket create [flags] <task-description>

Flags:
  --from <file>         Arquivo de spec para decompor
  --max-tickets <n>     Máximo de tickets a criar (default: 10)
  --parallel <n>        Sugestão de tickets paralelos (default: 3)
  --mandate <file>      Template de mandate (default: .toolbelt/mandate.yaml)
  --output <path>       Arquivo de saída JSON/YAML
  --json                Saída em JSON
  --dry-run             Mostra tickets sem criar
```

**Exemplos:**

```bash
# Criar tickets a partir de descrição
tb ticket create "Implementar autenticação JWT completa"
# ═══════════════════════════════════════════════════════════════
# DECOMPOSIÇÃO DE TAREFA
# ═══════════════════════════════════════════════════════════════
# 
# Tarefa: Implementar autenticação JWT completa
# 
# Tickets criados: 4
# 
# ┌────────────┬────────────────────────────────────────┬──────────┐
# │ ID         │ Título                                 │ Deps     │
# ├────────────┼────────────────────────────────────────┼──────────┤
# │ TKT-001    │ Criar modelo User                      │ -        │
# │ TKT-002    │ Implementar JWT signing                │ TKT-001  │
# │ TKT-003    │ Criar handler POST /auth/login         │ TKT-002  │
# │ TKT-004    │ Criar middleware de validação JWT      │ TKT-002  │
# └────────────┴────────────────────────────────────────┴──────────┘
# 
# Paralelismo máximo: 2 (TKT-003 e TKT-004 após TKT-002)
# Tempo estimado: 15-25 min

# Criar a partir de spec
tb ticket create --from docs/specs/auth.md --max-tickets 5

# Dry run para revisar antes de criar
tb ticket create --dry-run "Refatorar módulo de pagamentos"
```

**`tb ticket list`:**

```bash
tb ticket list [flags]

Flags:
  --status <statuses>   Filtrar por status: pending,running,done,failed,cancelled
  --deps <ticket-id>    Mostrar dependentes de um ticket
  --json                Saída em JSON
```

**Exemplos:**

```bash
# Listar todos os tickets
tb ticket list
# ID         STATUS    TITLE                              WORKER
# TKT-001    done      Criar modelo User                  worker-001
# TKT-002    done      Implementar JWT signing            worker-002
# TKT-003    running   Criar handler POST /auth/login     worker-001
# TKT-004    pending   Criar middleware validação JWT     -

# Filtrar por status
tb ticket list --status pending,running

# Ver dependentes de um ticket
tb ticket list --deps TKT-002
```

**Formato do Ticket:**

```yaml
# .toolbelt/tickets/TKT-001.yaml
ticket:
  id: "TKT-001"
  title: "Criar modelo User"
  status: pending
  created_at: "2026-01-15T10:30:00Z"
  
  mandate:
    scope:
      - "src/models/user.go"
      - "src/models/user_test.go"
    permissions: [read, write, run_tests]
    forbidden:
      - "src/database/*"
      - "tests/hidden/*"
      - ".toolbelt/thresholds.yaml"
      
  context:
    files:
      - "docs/specs/auth.md"
      - "vault/decisions/user-schema.md"
    symbols:
      - "Session"
      - "validateEmail"
    graph_queries:
      - "callers:Session"
      
  acceptance:
    criteria: "Testes em src/models/user_test.go passam"
    timeout: "10m"
    
  dependencies: []
  
  subscription:
    worker: "worker-001"
    direct_delivery: true
  
  resources:
    cpu_limit: "50%"
    mem_limit: "512M"
```

---

### `tb board` — Visualização do Board (planejado — Fase 3+)

> **Não implementado.**

Exibe e gerencia o board de tickets (fila de trabalho).

```bash
tb board <subcommand>

Subcomandos:
  list      Lista tickets no board
  watch     Monitora board em tempo real
  pause     Pausa processamento de novos tickets
  resume    Retoma processamento
  drain     Espera workers finalizarem e para
```

**`tb board list`:**

```bash
tb board list [flags]

Flags:
  --status <statuses>   Filtrar: pending,running,done,failed
  --worker <id>         Filtrar por worker
  --priority            Ordenar por prioridade
  --json                Saída em JSON
```

**Exemplos:**

```bash
# Ver estado atual do board
tb board list
# ═══════════════════════════════════════════════════════════════
# BOARD: auth-implementation
# ═══════════════════════════════════════════════════════════════
# 
# RUNNING (2):
# ┌────────────┬────────────────────────────────────┬────────────┐
# │ TKT-003    │ Criar handler POST /auth/login     │ worker-001 │
# │ TKT-005    │ Implementar rate limiting          │ worker-002 │
# └────────────┴────────────────────────────────────┴────────────┘
# 
# PENDING (3):
# ┌────────────┬────────────────────────────────────┬──────────┐
# │ TKT-004    │ Criar middleware validação JWT     │ blocked  │
# │ TKT-006    │ Adicionar logout endpoint          │ ready    │
# │ TKT-007    │ Testes de integração               │ ready    │
# └────────────┴────────────────────────────────────┴──────────┘
# 
# Workers: 2 ativos, 1 idle
# Tempo estimado restante: 8-12 min

# Monitorar em tempo real
tb board watch
# (atualiza a cada 2s)

# Pausar board
tb board pause
# Board pausado. Tickets running continuam, novos não são atribuídos.

# Retomar
tb board resume
```

---

### `tb router` — Controle do Router

Controla o router de delegação de tarefas.

```bash
tb router <subcommand>

Subcomandos:
  start     Inicia router para processar tarefas
  stop      Para router graciosamente
  status    Exibe status do router
  config    Mostra/edita configuração
```

**`tb router start`:**

```bash
tb router start [flags]

Flags:
  --workers <n>         Número máximo de workers (default: 3)
  --strategy <type>     Estratégia: fifo|priority|dependency (default: dependency)
  --timeout <dur>       Timeout global por ticket (default: 10m)
  --config <path>       Arquivo de configuração
  --direct-delivery     Habilita entrega direta a workers subscritos (default: true)
```

**Exemplos:**

```bash
# Iniciar router com configuração padrão
tb router start
# ═══════════════════════════════════════════════════════════════
# ROUTER: iniciado
# ═══════════════════════════════════════════════════════════════
# Workers máx: 3
# Estratégia: dependency
# Timeout: 10m
# Entrega direta: habilitada
# 
# Aguardando tickets em .toolbelt/board/

# Iniciar com mais workers
tb router start --workers 5 --strategy priority

# Ver status
tb router status
# Router: running
# Workers: 3/5 ativos
# Tickets processados: 12
# Tickets pendentes: 4
# Entregas diretas: 8 (67%)
# Tempo médio por ticket: 3m 45s
# Falhas: 1 (TKT-008, timeout)
```

**Configuração do Router:**

```yaml
# .toolbelt/router.yaml
router:
  max_workers: 3
  strategy: dependency  # fifo | priority | dependency
  
  direct_delivery:
    enabled: true
    subscription_ttl: 30m
  
  timeouts:
    per_ticket: 10m
    idle_worker: 5m
    global: 1h
    
  retry:
    max_attempts: 2
    backoff: exponential
    
  workers:
    model: "claude-sonnet-4-20250514"
    temperature: 0.3
    capabilities:
      - read:code
      - write:code
      - run:tests
      - query:graph
      
  merge:
    conflict_strategy: prompt  # prompt | last_wins | fail
    atomic: true
```

---

### `tb watchdog` — Monitoramento de Workers

Monitora saúde e performance dos workers.

```bash
tb watchdog <subcommand>

Subcomandos:
  status    Exibe status de todos os workers
  kill      Força término de worker travado
  scale     Ajusta número de workers
  alerts    Lista alertas recentes
```

**`tb watchdog status`:**

```bash
tb watchdog status [flags]

Flags:
  --worker <id>         Status de worker específico
  --json                Saída em JSON
```

**Exemplos:**

```bash
# Status de todos os workers
tb watchdog status
# ═══════════════════════════════════════════════════════════════
# WATCHDOG STATUS
# ═══════════════════════════════════════════════════════════════
# 
# ┌────────────┬──────────┬────────────┬──────────┬────────────┬───────────┐
# │ WORKER     │ STATUS   │ TICKET     │ ELAPSED  │ HEALTH     │ SUBSCRIBED│
# ├────────────┼──────────┼────────────┼──────────┼────────────┼───────────┤
# │ worker-001 │ working  │ TKT-003    │ 2m 30s   │ ██████████ │ TKT-006   │
# │ worker-002 │ working  │ TKT-005    │ 4m 15s   │ ████████░░ │ -         │
# │ worker-003 │ idle     │ -          │ 30s      │ ██████████ │ TKT-007,08│
# └────────────┴──────────┴────────────┴──────────┴────────────┴───────────┘
# 
# Alertas: 0
# Tickets timeout: 0
# CPU total: 45%
# Memória total: 1.2GB

# Forçar término de worker
tb watchdog kill worker-002 --reason "Travado em loop"
# worker-002 terminado. Ticket TKT-005 retornado ao board.

# Escalar workers
tb watchdog scale --workers 5
# Workers escalados: 3 → 5
```

**Alertas monitorados:**

| Alerta | Condição | Ação |
|--------|----------|------|
| `worker_slow` | > 80% do timeout | Warning no log |
| `worker_stuck` | 100% do timeout | Cancela ticket, mata worker |
| `worker_crash` | Exit code != 0 | Retry ou escalate |
| `queue_growing` | Fila > 2x workers | Sugere escalar |
| `resource_high` | CPU/Mem > 90% | Pausa novas atribuições |

---

### `tb merge` — Agregação de Resultados

Agrega resultados de múltiplos workers e resolve conflitos.

```bash
tb merge [flags] <ticket-ids>

Flags:
  --strategy <type>     Conflito: prompt|last_wins|fail (default: prompt)
  --dry-run             Mostra mudanças sem aplicar
  --force               Ignora conflitos e usa último
  --output <path>       Patch unificado de saída
  --json                Saída em JSON
```

**Exemplos:**

```bash
# Merge automático de tickets concluídos
tb merge TKT-001 TKT-002 TKT-003
# ═══════════════════════════════════════════════════════════════
# MERGE: 3 tickets
# ═══════════════════════════════════════════════════════════════
# 
# Arquivos modificados: 5
# 
# ┌──────────────────────────────────┬──────────────────────────┐
# │ ARQUIVO                          │ STATUS                   │
# ├──────────────────────────────────┼──────────────────────────┤
# │ src/models/user.go               │ TKT-001 (único)          │
# │ src/services/jwt.go              │ TKT-002 (único)          │
# │ src/handlers/auth.go             │ ⚠ CONFLITO (TKT-002,003) │
# │ src/middleware/auth.go           │ TKT-003 (único)          │
# │ go.mod                           │ TKT-001 + TKT-002 (merge)│
# └──────────────────────────────────┴──────────────────────────┘
# 
# Conflito em src/handlers/auth.go:
# 
# TKT-002 (linha 45):
#   func validateToken(token string) bool { ... }
# 
# TKT-003 (linha 45):
#   func validateToken(token string) (*Claims, error) { ... }
# 
# [1] Manter TKT-002
# [2] Manter TKT-003
# [3] Editar manualmente
# Escolha: 2
# 
# Merge concluído. 5 arquivos atualizados.

# Dry run para ver conflitos
tb merge --dry-run TKT-001 TKT-002

# Forçar merge usando último
tb merge --strategy last_wins TKT-001 TKT-002

# Gerar patch unificado
tb merge --output changes.patch TKT-001 TKT-002 TKT-003
```

**Estratégias de conflito:**

| Estratégia | Comportamento |
|------------|---------------|
| `prompt` | Pergunta ao usuário para cada conflito |
| `last_wins` | Usa a versão do ticket mais recente |
| `fail` | Falha se houver qualquer conflito |
| `semantic` | Tenta merge semântico via LLM (experimental) |

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
  --agent <cmd>        Comando do agente externo (fallback: bench.default_agent)
  -t, --tasks-dir      Diretório das tarefas (fallback: bench.tasks_dir)
  -f, --fixtures-dir   Diretório das fixtures (fallback: bench.fixtures_dir)
  --repetitions <n>    Número de repetições (default: 1)
  --output <path>      Arquivo de saída JSONL (sem flag: sob bench.output_dir)
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

---

### `tb version` ✅ IMPLEMENTADO

```bash
tb version [flags]

Flags:
  --json    Saída em JSON ✅
```

**Exemplos:**

```bash
tb version
# toolbelt v0.1.0
# go version: go1.22.0
# build time: 2026-01-15T10:00:00Z
# commit: abc123

tb version --json
# {"version":"0.1.0","go":"1.22.0","commit":"abc123","build_time":"2026-01-15T10:00:00Z"}
```

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
| `TB_COLOR` | Habilita cores: auto, always, never | `auto` |
| `TB_COLOR_MODE` | Modo de cor: ansi, 256, truecolor | `auto` |
| `NO_COLOR` | Desabilita cores (padrão freedesktop) | - |

Consulte `assets/theme/tokens.json` para a tabela completa de mapeamento ANSI/256/TrueColor.

---

## Variáveis de Ambiente

| Variável | Descrição | Default |
|----------|-----------|---------|
| `TB_HOME` | Diretório de dados do toolbelt | `.tb/` |
| `TB_VAULT` | Caminho do vault Markdown | `$TB_HOME/vault/` |
| `TB_LOG_LEVEL` | Nível de log: debug, info, warn, error | `info` |
| `TB_SANDBOX_DEFAULT` | Sandbox por padrão em `tb run` | `false` |
| `TB_JSON_OUTPUT` | Saída JSON por padrão | `false` |
| `TB_TIMEOUT_DEFAULT` | Timeout padrão para comandos | `30s` |
| `TB_COLOR` | Habilita cores: auto, always, never | `auto` |
| `TB_COLOR_MODE` | Modo de cor: ansi, 256, truecolor | `auto` |

---

## Códigos de Saída

| Código | Significado |
|--------|-------------|
| 0 | Sucesso |
| 1 | Erro genérico |
| 2 | Erro de argumentos |
| 3 | Timeout |
| 4 | Violação de segurança |
| 5 | Recurso não encontrado |
| 6 | Permissão negada |
| 7 | Snapshot divergente |

---

## Configuração

Arquivo de configuração em `.tb/config.yaml`:

```yaml
defaults:
  sandbox: false
  timeout: 30s
  json_output: false

sandbox:
  allow_net: []
  cpu_limit: 100
  mem_limit: 1G
  time_limit: 5m

graph:
  auto_index: true
  index_on_change: true
  infer_relations: false

vault:
  path: .tb/vault
  sync_on_change: true

logging:
  level: info
  file: .tb/logs/tb.log
  format: json
```
