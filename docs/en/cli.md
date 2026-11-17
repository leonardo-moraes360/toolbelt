[🇧🇷 Português](../cli.md) | 🇺🇸 English

# `tb` Command Tree

This document describes the toolbelt CLI: **implemented** commands (mirrored from `tb --help`) and **planned** sections (Phase 3+, marked explicitly). On divergence, the Cobra CLI is the source of truth.

## Local-first: no GitHub

`tb` **does not talk to GitHub** (or other forges): it is **100% local**. It reads
the working-tree git, files on disk, and `.toolbelt/`. It does not open issues,
download PRs, or call the GitHub API. That is why `tb review`, `tb drift`,
`tb verify --ticket`, and `tb stack` need an **`rd.md`** (or another local ticket
via `--ticket` / a positional argument) — not an issue number.

### How to obtain or generate `rd.md`

Write the file at the repo root (or pass `--ticket path`). Accepted formats:
YAML frontmatter or Markdown with `## Scope` / `## Escopo`,
`## Forbidden` / `## Proibido`, and acceptance checkboxes (details in `tb review`).

From an **issue** or **PR** (outside `tb`, with `gh` or the browser):

```bash
# Issue → rascunho local
gh issue view 19 --json title,body > /tmp/issue-19.json
# Copie title/body para rd.md e declare scope/forbidden/acceptance

# PR → corpo + arquivos tocados como ponto de partida do Scope
gh pr view 42 --json title,body,files > /tmp/pr-42.json
gh pr diff 42 --name-only
```

Minimal example:

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

Or Markdown only:

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

`tb ticket` (create tickets automatically) is **planned** — until then the file
is manual.

## Overview

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
└── router       Stub (não implementado)

tb  (planejado — Fase 3+)
├── run / proc / md / agent
├── ticket / board / watchdog / merge
└── serve        (hoje use: tb mcp)
```

---

## Detailed Commands

### `tb run` — Command Execution (planned — Phase 3+)

> **Not implemented as `tb run`.** Today use `tb sandbox run` for Landlock isolation.

Runs commands with optional isolation.

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

**Examples:**

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

### `tb proc` — Process Management (planned — Phase 3+)

> **Not implemented.** The current CLI does not expose `tb proc`.

Manages long-running processes (servers, watchers).

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

**Examples:**

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

### `tb graph` — On-Demand Graph (✓ Phase 2, ADR-011)

Queries and indexes the code graph **on demand**. Disabled by default;
enable with `TB_GRAPH=on` or `--enable` (without that, CLI and MCP respond
`grafo desabilitado (ative com --enable ou TB_GRAPH=on; ADR-011)`). Measure ROI via
`tb bench compare --config-a bench/ab/graph-off.yaml --config-b bench/ab/graph-on.yaml`.

Languages and docs (extensible via `LanguageIndexer`, light regex, no cgo):

| `graph.languages` | Extensions |
|-------------------|------------|
| `go` | `.go` (AST; `calls` edges) |
| `javascript` | covers TypeScript (`ts`/`tsx`): `.js`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.jsx` (generic functions `foo<T>`, `const` arrow, JSX `<Comp />` as `calls`; indexes `*.test.*`/`*.spec.*`/`__tests__`/`e2e`; resolves NodeNext relative imports (`.js`→`.ts`, no extension, `index`) and `tsconfig.json`/`jsconfig.json` aliases (`extends`/`baseUrl`/`paths`); follows `import { a as b }` and `import * as ns`). There is no separate `typescript` language — use `javascript` |
| `python` | `.py` (defines/imports + `calls` edges by qualified module; `callers`/`impact` ignore edges with confidence &lt; 0.6) |
| `java` | `.java` |
| `kotlin` | `.kt`, `.kts` |
| `csharp` | `.cs` |
| `cpp` | `.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.hpp`, `.hh`, `.hxx` (defines/imports; no `calls` edges — `callers`/`impact` return an explicit error) |
| `rust` | `.rs` (defines/imports + minimal resolvable `calls` edges in the repository) |
| `php` | `.php` |
| `ruby` | `.rb` |
| `swift` | `.swift` |
| `shell` | `.sh`, `.bash`, `.zsh`, `.ksh` |

| `graph.docs` | Extensions | What it indexes |
|--------------|------------|-----------------|
| `markdown` | `.md` | document, headings, links, mentions |
| `html` | `.html`, `.htm` | document, `<h1>`–`<h6>`, anchors |
| `txt` | `.txt` | document, mentioned paths/symbols |

| `graph.frameworks` | Extensions / convention | What it indexes |
|--------------------|-------------------------|-----------------|
| `react` | `.tsx`, `.jsx` | components, `use*` hooks, `renders` edges (in parallel with `javascript` for `calls`/callers/impact) |
| `vue` | `.vue` | SFC script/template, imports, components in the template |
| `svelte` | `.svelte` | script, imports, components in markup |
| `astro` | `.astro` | frontmatter, imports, components in the body |
| `angular` | `*.component.ts` / `.html`, services/modules | `@Component`/`@Injectable`/`@NgModule`, selector, `templateUrl` |
| `routes` | Next `app/`/`pages/`, SvelteKit `routes/`, Nuxt `pages/` | `route` nodes by file convention |
| `mdx` | `.mdx` | doc (headings/links) + JSX imports/components |

Frameworks run together with the language indexer on the same file (e.g.
`.tsx` gets `react` + `javascript`, for components/`renders` and
`calls`/callers/impact). Docs can also index the same path when the
extension matches.

```yaml
# .toolbelt/config.yaml (todos ligados por padrão)
graph:
  languages: [go, javascript, python, java, kotlin, csharp, cpp, rust, php, ruby, swift, shell]
  docs: [markdown, html, txt]
  frameworks: [react, vue, svelte, astro, angular, routes, mdx]
  max_file_bytes: 524288   # ignora arquivos maiores
  index_on_change: true    # reindexa em search/callers/deps/impact/stats se o fingerprint mudou
```

Ignores subdirectories such as `node_modules`, `vendor`, `dist`, `build`,
`target`, `.git`, etc. The walk root is never skipped by name (e.g. indexing
`/tmp/target` works; a child `target/` is still ignored). If the root is
a symlink (e.g. `/tmp/target` → real dir), it is resolved with
`EvalSymlinks` before the walk — otherwise the index would be empty, because
`filepath.Walk` does not descend into symlinks. The graph remains on demand (ADR-011):
`TB_GRAPH=on` or `--enable`.

Index freshness: each `graph.json` stores a `fingerprint` (mtime+size of
indexable files). On `search`/`callers`/`deps`/`impact`/`stats`, if the
fingerprint diverges and `graph.index_on_change` is `true` (default), the graph is
reindexed automatically; if `false`, output warns and JSON includes
`stale: true`. Query JSON outputs use the envelope
`{"stale": false, "nodes": [...]}`; `stats`/`index` include `stale` in the
statistics object.

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

Graph MCP tools (call requires `TB_GRAPH=on`): `tb_graph_search`,
`tb_graph_callers`, `tb_graph_deps`, `tb_graph_impact`, `tb_graph_index`.

There is no `tb graph query` — use `tb graph search`.

**`tb graph search`:**

```bash
tb graph search <query> [flags]

Flags:
  --limit <n>         Máximo de resultados (default: 20)
  --json              Saída em JSON (flag global)
```

**`tb graph callers`:**

```bash
tb graph callers [flags]

Flags:
  --function <name>   Nome da função (aceita sufixo qualificado, ex. scrubber.redact_secrets)
  --depth <n>         Profundidade do grafo (default: 1)
```

There is no `--file` on `callers` (use `--function`). For impact by file, use `tb graph impact --file`.

In Python/JS/TS, `--function` resolves by title or qualified id suffix
(`func:pkg.mod.nome`; React components/hooks too). `calls` edges with
confidence &lt; 0.6 (ambiguous targets / no import) are left out of the default
`callers` and `impact` result. In TS/JS, relative imports
(`../components/Layout`, `./utils.js` → `utils.ts`, `./index`) and
`tsconfig`/`jsconfig` aliases (`@/…`) resolve to the imported file/symbol;
`import { helper as format }` and `import * as Utils` / `Utils.helper()` also
feed `callers`/`impact` (including functions in test files).

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

**`tb graph index`:**

```bash
tb graph index [path] [flags]
```

The path is **positional** (there is no `--path`).

**Examples:**

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

### `tb mem` — Invariants (✓ Phase 1)

Manages the `.toolbelt/invariants.yaml` file (outside compaction, ADR-010).

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

An unknown subcommand (`tb mem foo`) exits with code ≠ 0 and message `unknown command`.

**`tb mem add`:**

```bash
tb mem add <regra> [flags]

Flags:
  --category <cat>    Categoria: anti_cheat|mandate|security (default: anti_cheat)
  --ref <path>        Vínculo com ADR/decisions.md (ex: docs/adr/0001.md)
  --immutable         Marca como imutável (default: mutável)
```

**Immutability default:** new invariants start **mutable**. Use `--immutable` (CLI) or `immutable: true` (MCP) to protect them. `tb mem init` writes the anti-cheat set already with `immutable: true`. Category `anti_cheat` and `immutable: true` require `--force --reason` for `edit`/`retire`.

Invalid category or `--ref` outside the ADR/decisions format exits ≠ 0 with the list
of valid values (`anti_cheat, mandate, security`; refs like
`docs/adr/<arquivo>.md` / `docs/decisions/<arquivo>.md`).

**`tb mem list` / `search` / `check`:**

```bash
tb mem list [--category <cat>] [--all]
tb mem search <query> [--all]
tb mem check [--expect-sha <sha>] [--all]
```

`search` is case- and accent-insensitive (Unicode NFD + mark stripping).

**`tb mem show` / `edit` / `retire`:**

```bash
tb mem show <id>
tb mem edit <id> [--category <cat>] [--rule <texto>] [--ref <path>] [--force] [--reason <motivo>]
tb mem retire <id> [--force] [--reason <motivo>]
```

**`tb mem init`:** creates the default anti-cheat set and prints the path + the
created invariants (or `já existiam N…` if the file already had entries). With
`--json`, emits `path`/`created`/`count`/`invariants`.

**`tb mem check` (implemented):**

```bash
tb mem check [flags]

Flags:
  --expect-sha <sha>  Hash esperado pós-compactação
  --all               Inclui invariantes aposentadas
```

`mem check` **fails closed**: missing/unreadable/corrupt file, divergent SHA, missing IDs, or non-evaluable rule (empty id/category/rule) → exit ≠ 0. Does not auto-create defaults (use `tb mem init`).

**Redaction:** `show`/`list`/`search` (text and `--json`) and equivalent MCP payloads mask common patterns (`ghp_`/`gho_`/`github_pat_`, `AKIA…`, PEM private key, `Bearer`, `xox*`, `*_TOKEN=`) with `[REDACTED]`.

**History:** forced changes to protected invariants are append-only in `.toolbelt/invariants_history.jsonl` (who, when, reason, previous value).

**Examples:**

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

> Soft-retire keeps the entry in the YAML; `list`/`search`/`check` omit retired unless `--all`.
> Vault flags `--type`/`--title` **do not exist** — `tb mem add` accepts a positional rule, `--category`, `--ref`, and `--immutable`.
> `tb mem init --json` returns `{path, created, count, invariants}`; empty lists come out as `[]`.

---

### `tb md` — TUI Markdown (planned — Phase 3+)

> **Not implemented.** The current CLI does not expose `tb md`.

TUI interface to navigate and edit the vault.

```bash
tb md [flags] [path]

Flags:
  --obsidian          Abre no Obsidian ao invés da TUI
  --read-only         Modo somente leitura
  --graph             Inicia na visualização de grafo
```

**Examples:**

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

**TUI shortcuts:**

| Key | Action |
|-----|--------|
| `j/k` | Navigate list |
| `Enter` | Open file |
| `e` | Edit file |
| `g` | Graph view |
| `s` | Search |
| `/` | Filter |
| `b` | View backlinks |
| `l` | View links |
| `q` | Quit |
| `?` | Help |

---

### `tb agent` — Agent Management (planned — Phase 3+)

> **Not implemented.** The current CLI does not expose `tb agent`.

Manages agents in the hierarchy.

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

**Examples:**

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

### `tb stack` — Verification Operation Stacking (✓ Phase 2)

Runs `verify`, `review`, `drift`, and/or `guard` in a single invocation, sharing
expensive work (unified diff, changed-file index, memory/invariants, and
config). Argument order is execution order. Output is always aggregated JSON
(`ok`, `steps`, `total_duration_ms`); the process exit code is ≠ 0 if
any step fails (same semantics as each standalone command). Durations are
`float` milliseconds (with sub-ms fraction). Preparation or operation errors
appear in `steps[].error` inside the JSON (not only on stderr).

**Not a branch stack** (like Graphite or git-stack). `tb stack` only stacks
verification/review operations with shared context.

```bash
tb stack <op> [<op>...] [flags]

Flags:
  --base <ref>           Ref base compartilhada (default: HEAD~1)
  --head <ref>           Ref head compartilhada (default: HEAD)
  --fail-fast            Para na primeira etapa que falhar
  --ticket <path>        Ticket/rd.md (review/drift/verify; default: rd.md)
  --approved-sha <sha>   SHA aprovado; patch-id verbatim (review/drift)
  --require-approved-sha Falha se --approved-sha estiver ausente (ou `review.require_approved_sha`)
  --warn-only            guard: exit 0 mesmo com SeverityError
  --test-cmd <cmd>       verify: testes visíveis
  --hidden-test-cmd <cmd> verify: testes ocultos
  --max-iter <n>         verify: máximo de iterações
  --timeout <duration>   verify: timeout (default: 10m)
  --reviewers <n>        review: máximo de goldfish
  --standards <csv>      review: papéis (scope,quality,acceptance)
  --env <KEY>            (global) allowlist extra para o verify empilhado
  --pass-env <KEY>       (global) opt-in explícito de segredo para o verify
```

With no operation, or an unknown operation, exits with a short error listing the valid
ops (`verify`, `review`, `drift`, `guard`) — no `Usage` dump.

**Examples:**

```bash
tb stack verify review drift guard
tb stack guard drift --fail-fast
tb stack review drift --base HEAD~1 --head HEAD --ticket rd.md
tb stack verify guard --test-cmd "true" --max-iter 1
tb stack verify --test-cmd "true" --env APP_ENV --pass-env GITHUB_TOKEN
```

Each `steps` item carries `op`, `ok`, `exit_code`, `duration_ms`, `result` (the JSON
the standalone command would produce) and, on failure, `error`. Stacked `verify` uses the
same shared `--base`/`--head`: if head diverges from the worktree `HEAD`,
build/lint/test run in a temporary detached worktree (see `tb verify`);
the step `result` includes `head_sha` and `workdir`.

---

### `tb guard` — Security Protections (✓ implemented: `check`)

Analyzes the diff between commits for protected paths and anti-cheating patterns.

```bash
tb guard check [base..head] [flags]

Flags:
  --base <ref>   Ref base (default: HEAD~1)
  --head <ref>   Ref head (default: HEAD)
  --warn-only    Exit 0 mesmo com SeverityError (opt-out do default CI-safe)
  --strict       Deprecated: falha em SeverityError já é o default; use --warn-only para opt-out
  --json         Saída em JSON (flag global)
```

By default, exit ≠ 0 when there is `SeverityError` (strict behavior).
`--strict` is **deprecated** (no-op alias: the default already fails on
`SeverityError`); prefer omitting the flag or using `--warn-only` to opt out.
In clean text mode it prints `guard: ok (N regras, 0 violações)` (`N` = valid
rules from the base + defaults).

**Examples:**

```bash
tb guard check HEAD~1..HEAD
tb guard check --base HEAD~1 --head HEAD
tb guard check --warn-only HEAD~1..HEAD
tb guard check --json HEAD~1..HEAD
```

> Subcommands `snapshot`, `verify`, `protect`, and `audit` **do not exist** in the current CLI — they are planned.

---

### `tb review` — Fresh-Context Review (✓ Phase 2)

Implements the Elephant-Goldfish pattern: **distinct** goldfish review the diff
against `ticket/rd.md` and `tb mem` invariants, without inheriting the author's context.
There are no clones of the same checker — `--reviewers` only limits how many roles run.

```bash
tb review [ticket] [flags]

Flags:
  --base <ref>          Commit base (default: HEAD~1 ou base_commit do ticket)
  --head <ref>          Commit head (default: HEAD)
  --approved-sha <sha>       SHA aprovado; compara `git patch-id --verbatim` de base..approved vs base..head
  --require-approved-sha     Falha se `--approved-sha` estiver ausente (ou `review.require_approved_sha` na config)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --reviewers <n>            Máximo de goldfish distintos (default: 3; não clona)
  --standards <stds>         Papéis: scope,quality,acceptance (default)
  --output <path>            Relatório JSON (sem shorthand `-o`; use `--output`)
  --json                     Saída em JSON (flag global)
```

Also reports scope and base-commit drift. Fresh isolation is mandatory.
`tb review -o …` is rejected (`unknown shorthand flag: 'o'`); use `--output`.

Base and head are resolved to SHA **once** at the start of the flow (anti-TOCTOU) and
that pinned SHA is used for the rest.

The JSON always includes `pinned_approval`. Without `--approved-sha`:
`{"checked": false}`. With the flag: `checked`, `approved_sha`, `patch_id`,
`head_patch_id`, `match`, and if they diverge, `range_diff`. Comparison uses
`git patch-id --verbatim` (whitespace-sensitive). A patch different from the approved one
prints a `git range-diff` summary and exits with code ≠ 0. If `--approved-sha`
coincides with the base (empty `base..approved` range), the CLI does **not** dump
`git range-diff` usage — it shows a clear message about the invalid range.
With `--require-approved-sha` (or `review.require_approved_sha: true` in
`.toolbelt/config.yaml`), missing `--approved-sha` is an error.

The ticket/rd.md accepts YAML frontmatter or Markdown: section `## Scope` / `## Escopo`
(or `Scope:` / `Escopo:`) and `## Forbidden` / `## Proibido` (or `Forbidden:` /
`Proibido:`) with lists (`-`, `*`, numbered) and paths in backticks.
Acceptance criteria come from `acceptance:` in YAML or `- [ ]` / `- [x]` checkboxes
in Markdown. If the YAML frontmatter does not declare `scope:`, the Markdown body is still
read to fill the scope.
Globs in scope/forbidden include `src/*.go`, `src/**`, `tests/hidden/**`, and
patterns with `**` in the middle (e.g. `src/**/foo.go`). Touches on Forbidden paths
fill `forbidden_hit` in drift and produce a critical finding in review.

Path invariants (e.g. `tests/hidden/`, `thresholds.yaml`) are matched against
files **changed** by the same path/glob matcher (`tests/hidden/**`,
`tests/hidden/*`) — not by substring in the diff or ticket text. Mentioning
`forbidden: tests/hidden/**` in `rd.md` without touching that directory does not produce
a critical finding. Mandate content invariants (e.g. forbid
`console.log` or comments) are applied to added (`+`) lines of the diff.

**Distinct reviewers (goldfish)**:

| Reviewer | `--standards` | What it checks |
|----------|---------------|----------------|
| `goldfish-scope` | `scope` (+ `invariants`) | Files outside scope, touches on forbidden; `tb mem` invariants applied to changed paths **and** to `+` lines of the diff (e.g. `console.log`, forbidden comments) |
| `goldfish-quality` | `quality` | `TODO`/`FIXME`/`XXX`/`HACK` markers (whole uppercase word, in comments); very large functions in the diff (≥80 lines +); changed source without tests |
| `goldfish-acceptance` | `acceptance` (alias: `completeness`) | Each ticket acceptance criterion has evidence in changed tests/code files (does not count the ticket itself); considers `Test*` (Go) and `test`/`it`/`describe` (JS/TS) added in the diff |

Heuristics are deterministic and offline (no LLM).

Each finding JSON includes `role` (goldfish role: `scope`, `quality`, `acceptance`, `invariants`).
Acceptance findings also carry `criterion` and `evidence` (tests/files seen in the diff).
`agreement` counts how many distinct roles agreed on the same finding (usually 1).
In text mode, prints `papel=<role>` (and `papéis_acordo=N` only if N>1) — no longer uses `acordo=N/M`.

**Examples:**

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

**Mandatory isolation**: If the system cannot create fresh contexts for the reviewers, the command fails with an error instead of proceeding with contaminated context.

---

### `tb drift` — Scope/Base Drift (✓ Phase 2)

Compares files changed in `base...head` with the **scope** declared in the
ticket/`rd.md`, detects touches on `Forbidden`/`Proibido` (`forbidden_hit`) and
a base that is not an ancestor of head (`base_mismatch`). It does **not** compare the
Markdown vault against code — the report is only scope/base/forbidden
(+ optional `pinned_approval`).
The scope/forbidden parser is the same as `tb review`
(YAML frontmatter or Markdown `## Scope` / `## Forbidden` / lists / globs with `**`).

The diff uses **three-dot** (`git diff base...head` ≡
`git diff $(git merge-base base head) head`), the same criterion as GitHub PRs:
after merge/squash on the base, only branch files enter — not what came from main.

Explicit `--base` on the CLI/MCP **always** takes precedence over the ticket's `base_commit`.
Without `--base`, the default `HEAD~1` yields to `base_commit` when the ticket declares one.
Refs (`HEAD^`, tags, short SHA) are resolved with `git rev-parse`.
`base_mismatch` is `true` when the ticket's declared `base_commit` (or the effective
base, if the ticket does not declare one) is **not** an ancestor of head
(`git merge-base --is-ancestor`) — so `--base main` on a divergent PR does not
produce a false mismatch; a SHA from another line (not an ancestor) does. JSON exposes
`actual_base` (resolved SHA of the base used in the diff) and `merge_base` (common
ancestor with head).

```bash
tb drift [ticket] [flags]

Flags:
  --base <ref>          Commit base (default: HEAD~1 ou base_commit do ticket)
  --head <ref>          Commit head (default: HEAD)
  --approved-sha <sha>       SHA aprovado; compara `git patch-id --verbatim` de base..approved vs base..head
  --require-approved-sha     Falha se `--approved-sha` estiver ausente (ou `review.require_approved_sha`)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --output <path>            Saída JSON (sem shorthand `-o`)
  --json                     Saída em JSON (flag global)
```

The drift JSON includes `pinned_approval` in the same format as `tb review`
(`checked: false` without the flag; verbatim patch-id with the flag). Divergence exits
with code ≠ 0 and shows `git range-diff`. Base/head are pinned to SHA at the start.
`--approved-sha` equal to the base follows the same clear message as `tb review` (no
usage dump).

**Examples:**

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

JSON (`--json` / `--output`) uses `out_of_scope`, `forbidden_hit`,
`base_mismatch`, `declared_base`, `actual_base` (and `pinned_approval` when
`--approved-sha` is set). Exit ≠ 0 if there is drift or a divergent pinned patch.

**CI integration**:

```bash
# No CI, falhar se houver arquivos fora do scope
tb drift --ticket rd.md --json | jq '.out_of_scope | length' | xargs -I {} test {} -eq 0
```

---

### `tb verify` — Adversarial Verification (✓ Phase 1/2)

Runs the generator→discriminator loop with pluggable rulers.

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
  --head <ref>             Ref head (build/lint/test rodam neste commit)
  --ticket <path>          Ticket com Scope (exceção de `protected_paths` custom)
  --output <path> / -o     Arquivo de saída JSON
  --json                   Saída em JSON (flag global)
  --env <KEY>              (global, repetível) inclui KEY na allowlist do filho
  --pass-env <KEY>         (global, repetível) opt-in explícito para segredo no filho
```

When `--head` (or the `--base`/`--head` pair) resolves to a SHA **different** from the
current worktree `HEAD`, verify creates a temporary `git worktree add --detach`
at that SHA, runs `build_cmd` / `lint_cmd` / `test_cmd` / `hidden_test_cmd` there, and
removes the worktree when finished (also on error or signal). JSON includes
`head_sha` and `workdir` (the directory where the commands ran). If
`head == HEAD` and the worktree is dirty, it emits a warning — commands run in the
current worktree and may see uncommitted changes.

Verify child processes (`sh -c` of `test_cmd`/`build_cmd`/`lint_cmd`/`hidden_test_cmd`)
receive only an environment **allowlist** (PATH, HOME, LANG, TMPDIR, GOPATH/GOCACHE,
essential NODE/npm/pnpm, proxies, etc.). Names containing `TOKEN`, `SECRET`, `KEY`, or
`PASSWORD` **never** pass, unless `--pass-env KEY` (or `env.pass` in
`.toolbelt/config.yaml`). `--env KEY` extends the allowlist, but does **not** bypass
the secret block.

`tb init` fills `verify.test_cmd` from the repository toolchain
(`go.mod` → `go test ./...`, `package.json` + scripts.test → `npm`/`pnpm`/`yarn`/`bun test`
via lockfile `pnpm-lock.yaml`/`yarn.lock`/`package-lock.json`/`bun.lockb`,
`pyproject.toml`/`uv.lock` → `pytest`/`uv run pytest`, `Cargo.toml` → `cargo test`,
`Makefile` with `test`/`check` target → `make test`/`make check`). Without recognized
markers, `verify.test_cmd` stays **empty** and init emits a warning — it never
keeps `go test ./...` without `go.mod`. The chosen command binary is validated on
`PATH`: if `uv` is missing with `uv.lock`, it falls back to `pytest` or
`python -m pytest` / `python3 -m pytest` (and warns); missing Node managers
fall back to `npm test` when possible.

When `AGENTS.md`/`CLAUDE.md` exist, init writes `.toolbelt/agent_docs.yaml` with
`sources` and short-command `hints` (inline, shell fences, and prose like
“use/run/prefer …”). Long code fences (e.g. ` ```go `) are ignored so they
do not pollute the hints.

When there are clear markers, init also fills `verify.build_cmd` and
`verify.lint_cmd` conservatively:

| Toolchain | `build_cmd` | `lint_cmd` |
|-----------|-------------|------------|
| `go.mod` | `go build ./...` | `.golangci.y*ml`/`.toml` → `golangci-lint run`; else `make lint` target |
| `package.json` | `build` script, else `typecheck`/`type-check` → `npm run …` / `pnpm run …` / `yarn …` | `lint` script → analogous |
| `Cargo.toml` | `cargo build` | `cargo clippy` |
| Python (`pyproject`/`uv.lock`/…) | (empty) | `[tool.ruff]`/`ruff.toml` → `ruff check .` (or `uv run …`); else mypy |
| only `Makefile` | `build` target → `make build` | `lint` target → `make lint` |

Missing script/config → field omitted. Without a detected `test_cmd`, verify
does not invent `go test` (configure by hand or add a script/marker).

The hidden suite remains hidden from the agent (paths under `tests/hidden/` protected
by `tb guard`). The command configured in `verify.hidden_test_cmd` or
`--hidden-test-cmd` **always runs**, even without the `tests/hidden/` directory
(typical in JS/Python/Rust monorepos). If the command is empty and the
directory exists, `go test ./tests/hidden/...` is used. With no command and no
directory, the hidden suite is ignored (`hidden_available: false`,
`hidden_passed: null`). Optional `build_cmd` and `lint_cmd` fields run before
tests (in that order) and make verify fail if the command exits with an error;
absent = previous behavior (only `test_cmd` + hidden).

If the `test_cmd` / `hidden_test_cmd` / `build_cmd` / `lint_cmd` binary does not
exist (exit 127), is not executable (exit 126), or `exec` fails with
`ErrNotFound`, verify **fails immediately** without retries, with a message like
`comando não encontrado: X — ajuste verify.test_cmd`. The `tb` process exits with
code **127**. With `--json` (or `--output`), it emits the normal `verify.Result`
envelope including `error` (and `verdict: FAIL`), instead of exiting without JSON. Genuine
command failures (exit ≠ 0/126/127) continue in the `--max-iter` loop.

`ESCALATE` and `FAIL` verdicts exit with **exit ≠ 0** also with `--json`
(same as text mode). Only `PASS` and `SKIPPED` (visual ruler) use exit 0.
A missing command uses exit **127** (see the exit-code table).

**Guard inside verify (anti-cheat vs mandate):**

Verify always runs `tb guard` on the diff and includes the result in `guard`. The decision
to **abort before build/test** depends on the origin of the `protected_path`:

| Origin | Without `--ticket` | With `--ticket` and path in Scope | With `--ticket` and path outside Scope |
|--------|--------------------|-----------------------------------|----------------------------------------|
| Built-in defaults (`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`, `.toolbelt/guard.yaml`) | **aborts** (FAIL, 0 iterations) | **aborts** | **aborts** |
| `protected_paths` from `.toolbelt/guard.yaml` on the **base** (custom) | reports as **warning** and **continues** build/test | reports warning (Scope exception) and **continues** | **aborts** |

`tb guard check` (and the deprecated `--strict` alias) treats custom violations as
`SeverityError` and fails the exit code by default; use `--warn-only` to opt out.
Only the `tb verify run` loop applies the exception above so it does not kill the mandate
when the manifest lists the task's own file.

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

Ruler `visual` ([docs/verificacao-visual.md](verificacao-visual.md)): headless capture + pixel diff;
if Chromium/Chrome or references are missing, it gracefully skips.

**Examples:**

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

**Available rulers:**

| Ruler | Metric | Default Threshold | Specific Cheating |
|-------|--------|-------------------|-------------------|
| `visual` | Pixel similarity | 0.95 | Reference edits |
| `tests` | Pass on hidden tests | 100% | Test modification |

> Ruler `review`, flag `--discriminator-only`, and invocations like `tb verify <task-id>` **do not exist** — use `tb verify run` and `tb review` separately.

**Discriminator briefing template:**

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

**Report format:**

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

### `tb guard check` — Protected Path Check

Checks changes to protected paths between two commits.

```bash
tb guard check [base..head] [flags]

Flags:
  --base <ref>          Commit/ref base (default: HEAD~1; ignorado se base..head for passado)
  --head <ref>          Commit/ref head (default: HEAD; ignorado se base..head for passado)
  --warn-only           Exit 0 mesmo com SeverityError (opt-out do default CI-safe)
  --strict              Deprecated: falha em SeverityError já é o default (alias sem efeito; use --warn-only)
  --json                Saída em JSON
```

Accepts the positional range `base..head` (classic form) or the `--base`/`--head` flags, aligned with `tb review` / `tb drift` / `tb verify`.

By default, exit ≠ 0 when there is `SeverityError` (CI-safe). `--strict` is
**deprecated** (replaced by the strict default + `--warn-only`). In text mode
with no violations it prints `guard: ok (N regras, 0 violações)` (`N` = valid
protected paths loaded: defaults + `.toolbelt/guard.yaml` from the **base**; invalid
patterns are ignored and do not count toward `rules_checked`).

**Examples:**

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

**Protected paths manifesto (`.toolbelt/guard.yaml`):**

The guard reads `protected_paths` from `.toolbelt/guard.yaml` **on the base ref**
(`git show base:.toolbelt/guard.yaml`) and **merges** them with the built-in defaults
(`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`,
`.toolbelt/guard.yaml`). Any change to `.toolbelt/guard.yaml` in the diff
is a violation. Malformed YAML on the base fails **closed** (exit ≠ 0, clear error).
Path matching is **case-insensitive** and honors globs (`*`, `?`, `**`).
Renames (`R*`) check source and destination. The anti-cheat scan uses
diff content / `git show head:path` (not the working tree) and covers Go and
JS/TS (`it.skip` / `describe.skip` / `test.skip` / `xit` / `.only` /
trivial assertions such as `expect(true).toBe(true)`). Each YAML entry may be
a string (`lib/help.js`) or an object with `pattern`. A trailing slash (`secrets/`)
protects the directory and children. Without the file on the base, only the defaults apply.
Violations get `source: default` or `source: custom`. In `tb verify run`,
only `default` (and custom outside the `--ticket` Scope) abort the loop; custom in the
mandate is reported and verify continues — see the verify section above.

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

**Approved decision format:**

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

### `tb ticket` — Ticket Management (planned — Phase 3+)

> **Not implemented.** The current CLI does not expose `tb ticket`. Use a manual `rd.md` consumed by `tb drift` / `tb review`.

Creates and manages work tickets for the Router/Workers pattern.

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

**Examples:**

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

**Examples:**

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

**Ticket format:**

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

### `tb board` — Board Visualization (planned — Phase 3+)

> **Not implemented.**

Displays and manages the ticket board (work queue).

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

**Examples:**

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

### `tb router` — Router Control (stub — not implemented)

Today `tb router` is only a stub in the Cobra tree (no local subcommands or flags).
It prints a “not yet implemented” warning and exits 0. The design below
is **planned** (Phase 5) and does not yet exist in the CLI.

```bash
tb router
```

**Planned (not available):**

```bash
# subcomandos futuros — NÃO implementados
# tb router start [--workers N] [--strategy fifo|priority|dependency]
# tb router stop
# tb router status
# tb router config
```

**Planned Router configuration:**

```yaml
# .toolbelt/router.yaml (planejado)
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

### `tb watchdog` — Worker Monitoring (planned — Phase 3+)

> **Not implemented.** The current CLI does not expose `tb watchdog`.

Monitors worker health and performance.

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

**Examples:**

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

**Monitored alerts:**

| Alert | Condition | Action |
|-------|-----------|--------|
| `worker_slow` | > 80% of timeout | Warning in the log |
| `worker_stuck` | 100% of timeout | Cancel ticket, kill worker |
| `worker_crash` | Exit code != 0 | Retry or escalate |
| `queue_growing` | Queue > 2× workers | Suggest scaling |
| `resource_high` | CPU/Mem > 90% | Pause new assignments |

---

### `tb merge` — Result Aggregation (planned — Phase 3+)

> **Not implemented.** The current CLI does not expose `tb merge`.

Aggregates results from multiple workers and resolves conflicts.

```bash
tb merge [flags] <ticket-ids>

Flags:
  --strategy <type>     Conflito: prompt|last_wins|fail (default: prompt)
  --dry-run             Mostra mudanças sem aplicar
  --force               Ignora conflitos e usa último
  --output <path>       Patch unificado de saída
  --json                Saída em JSON
```

**Examples:**

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

**Conflict strategies:**

| Strategy | Behavior |
|----------|----------|
| `prompt` | Ask the user for each conflict |
| `last_wins` | Use the most recent ticket's version |
| `fail` | Fail if there is any conflict |
| `semantic` | Try semantic merge via LLM (experimental) |

---

### `tb bench` — Benchmark ✅ IMPLEMENTED (A/B in Phase 2)

Runs benchmarks and measures KPIs. Supports A/B comparison between configs/feature flags.

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

Path and agent precedence: **explicit CLI flag** > **`config.yaml`** > **default**.
The global `-c/--config` flag feeds `config.Load()` (including `bench.*`).

If `--agent` and `--fake` are omitted, `tb bench run` uses `bench.default_agent`
from the loaded YAML (when set). Without `-t`/`-f`/`-o`, it uses
`bench.tasks_dir` / `bench.fixtures_dir` / `bench.output_dir`.

```yaml
# .toolbelt/config.yaml
bench:
  default_agent: "cursor-agent {prompt} --workdir {workdir}"
  tasks_dir: /abs/path/to/bench/tasks
  fixtures_dir: /abs/path/to/bench/fixtures
  output_dir: /abs/path/to/bench/results
```

Placeholders `{prompt}`, `{workdir}`, and `{timeout}` are interpolated with
POSIX quoting (single quotes), avoiding injection via quotes, `$()`, or `;`.
The harness also exports `TB_BENCH_PROMPT`, `TB_BENCH_WORKDIR`, and
`TB_BENCH_TIMEOUT` for agents that prefer reading env instead of argv.

A task only marks `passed: true` when the **agent** exits 0 **and** verification
reaches `expected_exit_code`. Agent exit ≠ 0 (e.g. `false`)
forces `passed: false` even if the fixture passes. If the agent command does not
exist (exit 127), `tb bench run` writes the JSONL and exits with code ≠ 0.

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

The Markdown report and JSONL include **success (pass@1)**, **time**, and **tokens** metrics, with A→B deltas.

**Examples:**

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

### `tb mcp` — MCP Server (✓ implemented)

Starts toolbelt as an MCP server over **stdio** (JSON-RPC 2.0). There is no `tb serve` in the current CLI.

```bash
tb mcp
```

Unix/tcp transport and `--socket`/`--port`/`--tls-*` flags are planned; today only stdio.

**Tools exposed in `tools/list` / `tools/call`:**

| Tool | CLI parity | Notes |
|------|------------|-------|
| `tb_version` | `tb version` | |
| `tb_echo` | — | smoke |
| `tb_env` | — | cwd/os/arch |
| `tb_run` | — | `sh -c` with timeout; filtered env; `env`/`pass_env` args (arrays) |
| `tb_mem_add` / `tb_mem_list` / `tb_mem_show` / `tb_mem_edit` / `tb_mem_retire` / `tb_mem_search` / `tb_mem_check` / `tb_mem_init` | `tb mem …` | optional `cwd` arg; `ref`/`all` (boolean; legacy strings ok) where applicable |
| `tb_verify_run` | `tb verify run` | `ruler=tests\|visual`; args mirror flags + `env`/`pass_env` |
| `tb_review` | `tb review` | ticket/base/head/approved_sha/require_approved_sha/reviewers/standards |
| `tb_drift` | `tb drift` | returns `{drift, has_drift, pinned_approval}` (`checked` explicit) |
| `tb_guard_check` | `tb guard check` | `base`/`head` or `range=A..B`; `strict_fail` by default on SeverityError; `warn_only=true` opt-out; `strict` deprecated (boolean) |
| `tb_stack` | `tb stack` | `ops` (array), `base`/`head`, `fail_fast` (boolean), `env`/`pass_env`; returns aggregated JSON |
| `tb_graph_search` / `tb_graph_callers` / `tb_graph_deps` / `tb_graph_impact` / `tb_graph_index` | `tb graph …` | call requires `TB_GRAPH=on`; `depth`/`limit` integer; `transitive` boolean |

Schemas type `depth`/`limit`/`reviewers`/`max_iter` as integer, `strict`/`warn_only`/`transitive`/`skip_unavailable`/`all` (mem) as boolean, `threshold` as number, and `standards` as a string array; legacy string values are still accepted. verify/review/drift/guard reports come back as JSON in `content` (without a process exit code). Empty lists use `[]`; `hidden_passed` is `null` if the hidden suite does not apply.

---

### `tb init` ✅ IMPLEMENTED

```bash
tb init [flags]

Flags:
  --global         Inicializa em ~/.config/toolbelt
  --write-agents   Semeia AGENTS.md mínimo se ausente (nunca sobrescreve)
  --json           Saída em JSON (flag global)
```

With `--json`, emits a payload with `path`, detected `test_cmd` / `build_cmd` / `lint_cmd`
(when present), optional `warning`, and `agents` (`seeded`, `sources`, …).

Toolchain detection details and `verify.*` population are in the `tb verify` section.

**Local state vs config (`.git/info/exclude`):** in a Git repository (without `--global`),
init writes to `.git/info/exclude` (does not change the project's `.gitignore`) the
**cache/derived** patterns:

- `.toolbelt/graph.json`
- `.toolbelt/logs/`
- `.toolbelt/vault/`
- `.toolbelt/workspaces/`
- `.toolbelt/captures/`
- `.toolbelt/bench/`

So `git stash -u` / `git clean` do not wipe the index/graph by mistake, without
blocking config versioning.

**Recommendation — version invariants:** `invariants.yaml` (and, when it makes
sense for the team, `config.yaml` / `guard.yaml` / `thresholds.yaml`) should
remain **versionable**. They are shared policy (ADR-010), not cache.
Init does **not** put all of `.toolbelt/` in exclude precisely to separate
cache from config.

---

### `tb sandbox` ✅ IMPLEMENTED

Landlock isolation on Linux (`check` / `run`). Does not create worktrees — use `tb workspace` for that.

```bash
tb sandbox check
tb sandbox run <command> [args...] [flags]
```

**Flags for `tb sandbox run`:**

| Flag | Default | Description |
|------|---------|-------------|
| `--workdir <path>` | cwd | Working directory (write allowed) |
| `--timeout <dur>` | `30s` | Process timeout |
| `--allow-unsandboxed` | `false` | Explicit opt-in to run **without** Landlock |

**Fail-closed** behavior: if the kernel has no Landlock (or ruleset application fails), `tb sandbox run` **refuses** to execute and returns a clear error asking for `--allow-unsandboxed`. There is no silent fallback to unsandboxed execution.

On Linux with Landlock, writes are restricted to `--workdir` and the system temp directory (`os.TempDir()`, typically `/tmp`) when that is not an ancestor of the workdir. System paths needed for execution (`/bin`, `/usr`, `/lib`, `/etc`, `/dev`, `/proc`, …) are read-only. The ruleset is applied in the child process (re-exec + `PR_SET_NO_NEW_PRIVS` + `landlock_restrict_self`) **before** `exec` of the command.

```bash
tb sandbox check
# Landlock: true
# ABI: 6

tb sandbox run --workdir /tmp/job sh -c 'echo ok > out.txt'
# write outside workdir → Permission denied

# Without Landlock (or for CI without LSM): explicit opt-in
tb sandbox run --allow-unsandboxed echo hello
```

With `--json`, `sandbox check` emits `os`/`arch`/`landlock`/`version`/`error`; `sandbox run` emits the `Result` in snake_case (`exit_code`, `timed_out`, `duration_ns`, `sandboxed`, …). If the child exits ≠ 0 without timeout, `tb` propagates that exit code.

---

### `tb workspace` ✅ IMPLEMENTED

Isolated workspaces via git worktree. Alias: `tb ws`.

```bash
tb workspace create <id> [--base <ref>] [--allow <path>] [--forbid <path>]
tb workspace list
tb workspace remove <id>
```

`--base` defaults to `HEAD`. `--allow`/`--forbid` are repeatable (`stringSlice`). create/list JSON uses snake_case.

---

### `tb version` / `tb --version` ✅ IMPLEMENTED

```bash
tb version [flags]
tb --version
```

Flags (subcommand `version`):
  --json    Saída em JSON ✅

The global `--version` flag prints the same textual output as `tb version`
(version, commit, build, go, os/arch).

**Examples:**

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

All CLI `--json` outputs (and domain payloads embedded in MCP tools)
use **`snake_case`** keys consistently (`exit_code`, `base_ref`,
`timed_out`, …). MCP protocol envelopes (`isError`, `inputSchema`, …)
remain in the camelCase required by the spec.

---

## Colors and Visual Theme

Colored CLI output follows the **Synthwave** theme defined in [docs/identidade-visual.md](identidade-visual.md). Colors are applied automatically when the terminal supports them.

| Element | Color | ANSI 256 Code |
|---------|-------|---------------|
| Headers, commands | Purple `#982BD4` | 134 |
| Values, paths, URLs | Cyan `#0CD7FF` | 51 |
| Errors, failures | Pink `#F9278C` | 198 |
| Success | Green `#2DD4A7` | 43 |
| Warnings | Yellow `#FFB020` | 214 |
| Secondary text | Gray `#A099B0` | 249 |

**Control variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `NO_COLOR` | Disables colors (freedesktop standard) | - |
| `TERM=dumb` | Disables colors | - |

`TB_COLOR` / `TB_COLOR_MODE` are planned and are not yet read by the CLI.
See `assets/theme/tokens.json` for the full ANSI/256/TrueColor mapping table.

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TB_GRAPH` | Enables on-demand graph (`on`/`1`/`true`) | off |
| `NO_COLOR` | Disables colors (freedesktop standard) | - |
| `TB_BROWSER` | Browser for the visual verify ruler | (auto) |
| `TB_HOME` | Echoed in MCP; does not set the CLI config dir | - |
| `TB_BENCH_PROMPT` / `TB_BENCH_WORKDIR` / `TB_BENCH_TIMEOUT` | Exported by the `tb bench run` harness | - |

Planned variables (not yet read by the CLI): `TB_VAULT`, `TB_LOG_LEVEL`,
`TB_SANDBOX_DEFAULT`, `TB_JSON_OUTPUT`, `TB_TIMEOUT_DEFAULT`, `TB_COLOR`,
`TB_COLOR_MODE`.

### Environment allowlist for child processes

`tb verify` / `tb stack` (verify step), MCP `tb_run` / `tb_verify_run` / `tb_stack`
do not pass the full environment to `sh -c`. They only pass keys from the built-in allowlist
(PATH, HOME, LANG, TMPDIR, Go/Node/npm/pnpm toolchain, proxies, XDG, etc.), plus:

| Source | Effect |
|--------|--------|
| `.toolbelt/config.yaml` → `env.allow` | Extends the allowlist |
| `.toolbelt/config.yaml` → `env.pass` | Explicit secret opt-in |
| Global flag `--env KEY` / MCP arg `env` | Extends the allowlist (does not bypass secrets) |
| Global flag `--pass-env KEY` / MCP arg `pass_env` | Explicit secret opt-in |

Secrets: any name containing `TOKEN`, `SECRET`, `KEY`, or `PASSWORD`
(case-insensitive) is blocked without opt-in.

---

## Errors and Usage

- Expected domain/runtime errors (`RunE`, e.g. missing required business flag, git failure, missing file) exit with **exit ≠ 0** and a stderr message **without** a Cobra `Usage` dump.
- Genuine **flag** or **args** parse errors (unknown flag, `ExactArgs`/`MinimumNArgs`, etc.) keep `Usage` on stderr to guide invocation.
- An unknown subcommand at any level (e.g. `tb mem nao-existe`, `tb bench nope`) exits with **exit ≠ 0** and error `unknown command "…"`, instead of printing help with exit 0.
- Git failures in `guard` / `review` / `drift` / `verify` (and in the `stack` that stacks them) do **not** show only `exit status 128/129`: the message includes git stderr, the missing ref (`ref ausente: …`), and a `git fetch origin <ref>` hint.
- `--approved-sha` equal to the base (empty `base..approved` range) does not dump `git range-diff` usage; exits with a clear message about the invalid range.

## Exit Codes

The current CLI does not use a stable table of codes 2–7. In practice:

| Code | Meaning |
|------|---------|
| 0 | Success (`PASS`/`SKIPPED` in verify; guard without `SeverityError` or with `--warn-only`) |
| ≠0 (typically 1) | Parse/domain error, `FAIL`/`ESCALATE` in verify, `SeverityError` in guard, unknown subcommand |
| 127 | `tb verify run`: `test_cmd` / `build_cmd` / `lint_cmd` / `hidden_test_cmd` not found (or not executable / `ErrNotFound`); with `--json` emits the `verify.Result` envelope with an `error` field |
| child exit | `tb sandbox run` propagates the process exit code when ≠ 0 and without timeout |

When the configured verify command does not exist/is not executable, verify fails immediately (no retries) and the `tb` process exits **127**.

---

## Configuration

Local configuration file at `.toolbelt/config.yaml` (global:
`~/.config/toolbelt/config.yaml`). The `-c/--config` flag points to an alternate YAML.

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
