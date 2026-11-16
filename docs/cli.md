# Árvore de Comandos do `tb`

Este documento descreve a CLI do toolbelt: comandos **implementados** (espelham `tb --help`) e seções **planejadas** (Fase 3+, marcadas explicitamente). Em caso de divergência, a CLI Cobra é a fonte da verdade.

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

`tb ticket` (criar tickets automaticamente) é **planejado** — até lá o arquivo
é manual.

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
| `javascript` | cobre TypeScript (`ts`/`tsx`): `.js`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.jsx` (funções/`const` arrow, JSX `<Comp />` como `calls`; resolve imports relativos ao símbolo do módulo importado). Não há linguagem `typescript` separada — use `javascript` |
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
```

Ignora subdiretórios como `node_modules`, `vendor`, `dist`, `build`,
`target`, `.git`, etc. A raiz do walk nunca é pulada pelo nome (ex.: indexar
`/tmp/target` funciona; um `target/` filho continua ignorado). Se a raiz for
um symlink (ex.: `/tmp/target` → dir real), ela é resolvida com
`EvalSymlinks` antes do walk — caso contrário o índice ficaria vazio, pois
`filepath.Walk` não desce symlinks. O grafo continua sob demanda (ADR-011):
`TB_GRAPH=on` ou `--enable`.

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

Ferramentas MCP de grafo (chamada requer `TB_GRAPH=on`): `tb_graph_search`,
`tb_graph_callers`, `tb_graph_deps`, `tb_graph_impact`, `tb_graph_index`.

Não há `tb graph query` — use `tb graph search`.

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

Não há `--file` em `callers` (use `--function`). Para impacto por arquivo, use `tb graph impact --file`.

Em Python/JS/TS, `--function` resolve por título ou sufixo do id qualificado
(`func:pkg.mod.nome`; componentes/hooks React também). Arestas `calls` com
confiança &lt; 0.6 (alvos ambíguos / sem import) ficam de fora do resultado
default de `callers` e `impact`. Em TS/JS, imports relativos
(`../components/Layout`) resolvem para o símbolo do módulo importado
(`components.Layout`), inclusive uso JSX `<Layout />`.

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
```

**`tb mem list` / `search` / `check`:**

```bash
tb mem list [--category <cat>] [--all]
tb mem search <query> [--all]
tb mem check [--expect-sha <sha>] [--all]
```

**`tb mem show` / `edit` / `retire`:**

```bash
tb mem show <id>
tb mem edit <id> [--category <cat>] [--rule <texto>] [--ref <path>]
tb mem retire <id>
```

**`tb mem check` (implementado):**

```bash
tb mem check [flags]

Flags:
  --expect-sha <sha>  Hash esperado pós-compactação
  --all               Inclui invariantes aposentadas
```

**Exemplos:**

```bash
# Adicionar invariante (argumento posicional obrigatório)
tb mem add "Não modificar tests/hidden/**"
tb mem add --category security --ref docs/adr/0005.md "Não relaxar thresholds sem revisão"

# Listar / buscar / exibir / editar / aposentar
tb mem list
tb mem list --category anti_cheat
tb mem list --all
tb mem search thresholds
tb mem show inv-123
tb mem edit inv-123 --ref docs/decisions/auth.md --rule "Escopo via ticket"
tb mem retire inv-123
tb mem check
tb mem init
```

> Soft-retire mantém a entrada no YAML; `list`/`search`/`check` omitem retired a menos que `--all`.
> Flags vault `--type`/`--title` **não existem** — `tb mem add` aceita regra posicional, `--category` e `--ref`.

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
config). A ordem dos argumentos é a ordem de execução. A saída é sempre um JSON
agregado (`ok`, `steps`, `total_duration_ms`); o exit code do processo é ≠ 0 se
qualquer etapa falhar (mesma semântica de cada comando avulso).

**Não é stack de branches** (como Graphite ou git-stack). `tb stack` só empilha
operações de verificação/revisão com contexto compartilhado.

```bash
tb stack <op> [<op>...] [flags]

Flags:
  --base <ref>           Ref base compartilhada (default: HEAD~1)
  --head <ref>           Ref head compartilhada (default: HEAD)
  --fail-fast            Para na primeira etapa que falhar
  --ticket <path>        Ticket/rd.md (review/drift/verify; default: rd.md)
  --approved-sha <sha>   SHA aprovado (review/drift)
  --warn-only            guard: exit 0 mesmo com SeverityError
  --test-cmd <cmd>       verify: testes visíveis
  --hidden-test-cmd <cmd> verify: testes ocultos
  --max-iter <n>         verify: máximo de iterações
  --timeout <duration>   verify: timeout (default: 10m)
  --reviewers <n>        review: máximo de goldfish
  --standards <csv>      review: papéis (scope,quality,acceptance)
```

Sem operação, ou com operação desconhecida, sai com erro curto listando as ops
válidas (`verify`, `review`, `drift`, `guard`) — sem dump de `Usage`.

**Exemplos:**

```bash
tb stack verify review drift guard
tb stack guard drift --fail-fast
tb stack review drift --base HEAD~1 --head HEAD --ticket rd.md
tb stack verify guard --test-cmd "true" --max-iter 1
```

Cada item de `steps` traz `op`, `ok`, `exit_code`, `duration_ms` e `result` (o JSON
que o comando avulso produziria).

---

### `tb guard` — Proteções de Segurança (✓ implementado: `check`)

Analisa o diff entre commits em busca de paths protegidos e padrões anti-trapaça.

```bash
tb guard check [base..head] [flags]

Flags:
  --base <ref>   Ref base (default: HEAD~1)
  --head <ref>   Ref head (default: HEAD)
  --warn-only    Exit 0 mesmo com SeverityError (opt-out do default CI-safe)
  --strict       Deprecated: falha em SeverityError já é o default; use --warn-only para opt-out
  --json         Saída em JSON (flag global)
```

Por default, exit ≠ 0 quando há `SeverityError` (comportamento estrito).
`--strict` é **deprecated** (alias sem efeito: o default já falha em
`SeverityError`); prefira omitir a flag ou usar `--warn-only` para opt-out.
No modo texto limpo imprime `guard: ok (N regras, 0 violações)`.

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
  --approved-sha <sha>  SHA aprovado; compara `git patch-id --stable` de base..approved vs base..head
  --ticket <path>       Ticket/rd.md (default: rd.md)
  --reviewers <n>       Máximo de goldfish distintos (default: 3; não clona)
  --standards <stds>    Papéis: scope,quality,acceptance (default)
  --output <path>       Relatório JSON (sem shorthand `-o`; use `--output`)
  --json                Saída em JSON (flag global)
```

Também reporta drift de escopo e commit base. Isolamento fresco é obrigatório.
`tb review -o …` é rejeitado (`unknown shorthand flag: 'o'`); use `--output`.

Com `--approved-sha`, o JSON inclui `pinned_approval` (`approved_sha`, `patch_id`,
`head_patch_id`, `match` e, se divergir, `range_diff`). Patch diferente do aprovado
imprime um resumo de `git range-diff` e sai com código ≠ 0.

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
finding critical.

**Revisores (goldfish) distintos**:

| Revisor | `--standards` | O que verifica |
|---------|---------------|----------------|
| `goldfish-scope` | `scope` (+ `invariants`) | Arquivos fora do scope, toques em forbidden; invariantes de `tb mem` |
| `goldfish-quality` | `quality` | TODO/FIXME adicionados; funções muito grandes no diff (≥80 linhas +); código fonte alterado sem testes |
| `goldfish-acceptance` | `acceptance` (alias: `completeness`) | Cada critério de aceite do ticket tem evidência em testes/arquivos de código alterados (não conta o próprio ticket) |

Heurísticas são determinísticas e offline (sem LLM).

**Exemplos:**

```bash
# Revisão padrão: 3 papéis distintos
tb review --ticket rd.md
# ═══════════════════════════════════════════════════════════
#  RELATÓRIO DE REVISÃO: rd.md
#  Base..Head: HEAD~1..HEAD | Revisores: 3 | Fresh: true
# ═══════════════════════════════════════════════════════════
# FINDINGS:
#   [warning] (goldfish-quality/quality) TODO/FIXME adicionado no diff [src/a.go]
#   [warning] (goldfish-acceptance/acceptance) critério de aceite sem evidência...

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

Compara os arquivos alterados em `base..head` com o **scope** declarado no
ticket/`rd.md`, detecta toques em `Forbidden`/`Proibido` (`forbidden_hit`) e
divergência do commit base (mandate). **Não** compara vault Markdown contra
código — o relatório é só escopo/base/forbidden (+ `pinned_approval` opcional).
O parser de scope/forbidden é o mesmo do `tb review`
(YAML frontmatter ou Markdown `## Scope` / `## Forbidden` / listas / globs com `**`).

`--base` / `base_commit` do ticket (ex.: `HEAD^`, tags, SHA curto) são
resolvidos com `git rev-parse` antes da comparação — evita falso
`base_mismatch` por comparar a string da ref com o SHA declarado.

```bash
tb drift [ticket] [flags]

Flags:
  --base <ref>          Commit base (default: HEAD~1 ou base_commit do ticket)
  --head <ref>          Commit head (default: HEAD)
  --approved-sha <sha>  SHA aprovado; compara `git patch-id --stable` de base..approved vs base..head
  --ticket <path>       Ticket/rd.md (default: rd.md)
  --output <path>       Saída JSON (sem shorthand `-o`)
  --json                Saída em JSON (flag global)
```

Com `--approved-sha`, o JSON de drift inclui `pinned_approval` (mesmo formato do
`tb review`). Divergência de patch-id sai com código ≠ 0 e mostra `git range-diff`.

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
  --hidden-test-cmd <cmd>  Suite oculta (verify.hidden_test_cmd; se vazio e existir tests/hidden/: go test ./tests/hidden/...)
  --max-iter <n>           Máximo de iterações (default: 3)
  --timeout <dur>          Timeout (default: 10m)
  --threshold <float>      Threshold de similaridade visual (default: 0.95)
  --manifest <path>        Manifesto de telas (default: .toolbelt/screens.yaml)
  --skip-unavailable       Skip gracioso sem browser/refs/telas (default: true; veredicto SKIPPED)
  --base <ref>             Ref base
  --head <ref>             Ref head
  --ticket <path>          Ticket com Scope (exceção de `protected_paths` custom)
  --output <path> / -o     Arquivo de saída JSON
  --json                   Saída em JSON (flag global)
```

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
diretório existir, usa-se `go test ./tests/hidden/...`. Sem comando e sem
diretório, a suite oculta é ignorada (`hidden_available: false`). Campos
opcionais `build_cmd` e `lint_cmd` rodam antes dos testes (nessa ordem) e
fazem o verify falhar se o comando sair com erro; ausentes = comportamento
anterior (só `test_cmd` + hidden).

Se o binário do `test_cmd` / `hidden_test_cmd` / `build_cmd` / `lint_cmd` não
existe (exit 127), não é executável (exit 126) ou o `exec` falha com
`ErrNotFound`, o verify **falha imediatamente** sem retries, com mensagem do tipo
`comando não encontrado: X — ajuste verify.test_cmd`. Falhas genuínas do comando
(exit ≠ 0/126/127) continuam no loop de `--max-iter`.

Veredictos `ESCALATE` e `FAIL` saem com **exit ≠ 0** também com `--json`
(igual ao modo texto). Só `PASS` e `SKIPPED` (ruler visual) usam exit 0.

**Guard dentro do verify (anti-cheat vs mandate):**

O verify sempre roda `tb guard` no diff e inclui o resultado em `guard`. A decisão
de **abortar antes de build/test** depende da origem do `protected_path`:

| Origem | Sem `--ticket` | Com `--ticket` e path no Scope | Com `--ticket` e path fora do Scope |
|--------|----------------|--------------------------------|-------------------------------------|
| Defaults embutidos (`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`) | **aborta** (FAIL, 0 iterações) | **aborta** | **aborta** |
| `protected_paths` de `.toolbelt/guard.yaml` (custom) | reporta como **warning** e **continua** build/test | reporta warning (exceção de Scope) e **continua** | **aborta** |

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
  --warn-only           Exit 0 mesmo com SeverityError (opt-out do default CI-safe)
  --strict              Deprecated: falha em SeverityError já é o default (alias sem efeito; use --warn-only)
  --json                Saída em JSON
```

Aceita o intervalo posicional `base..head` (forma clássica) ou as flags `--base`/`--head`, alinhadas a `tb review` / `tb drift` / `tb verify`.

Por default, exit ≠ 0 quando há `SeverityError` (CI-safe). `--strict` está
**deprecated** (substituído pelo default estrito + `--warn-only`). No modo texto
sem violações imprime `guard: ok (N regras, 0 violações)` (`N` = paths protegidos
carregados: defaults + `.toolbelt/guard.yaml`).

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

O guard carrega `protected_paths` de `.toolbelt/guard.yaml` (quando presente) e **une** aos defaults embutidos (`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`). Cada entrada pode ser uma string (`lib/help.js`) ou um objeto com `pattern`. Entrada com barra final (`secrets/`) protege o diretório e todos os filhos (prefixo). `secrets/*` continua válido para um nível via glob. Sem o arquivo, só os defaults valem. Violações ganham `source: default` (anti-cheat embutido) ou `source: custom` (YAML). Em `tb verify run`, só `default` (e custom fora do Scope do `--ticket`) abortam o loop; custom no mandate é reportado e o verify segue — ver seção verify acima.

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

### `tb router` — Controle do Router (stub — não implementado)

Hoje `tb router` é apenas um stub na árvore Cobra (sem subcomandos nem flags
locais). Imprime aviso de “ainda não implementado” e sai 0. O desenho abaixo
é **planejado** (Fase 5) e ainda não existe na CLI.

```bash
tb router
```

**Planejado (não disponível):**

```bash
# subcomandos futuros — NÃO implementados
# tb router start [--workers N] [--strategy fifo|priority|dependency]
# tb router stop
# tb router status
# tb router config
```

**Configuração planejada do Router:**

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

### `tb watchdog` — Monitoramento de Workers (planejado — Fase 3+)

> **Não implementado.** A CLI atual não expõe `tb watchdog`.

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

### `tb merge` — Agregação de Resultados (planejado — Fase 3+)

> **Não implementado.** A CLI atual não expõe `tb merge`.

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
| `tb_run` | — | `sh -c` com timeout |
| `tb_mem_add` / `tb_mem_list` / `tb_mem_show` / `tb_mem_edit` / `tb_mem_retire` / `tb_mem_search` / `tb_mem_check` / `tb_mem_init` | `tb mem …` | arg opcional `cwd`; `ref`/`all` onde aplicável |
| `tb_verify_run` | `tb verify run` | `ruler=tests\|visual`; args espelham flags |
| `tb_review` | `tb review` | ticket/base/head/approved_sha/reviewers/standards |
| `tb_drift` | `tb drift` | retorna `{drift, has_drift, pinned_approval}` |
| `tb_guard_check` | `tb guard check` | `base`/`head` ou `range=A..B`; `strict_fail` por default em SeverityError; `warn_only=true` opt-out; `strict` deprecated (boolean) |
| `tb_stack` | `tb stack` | `ops` (array), `base`/`head`, `fail_fast` (boolean); retorna JSON agregado |
| `tb_graph_search` / `tb_graph_callers` / `tb_graph_deps` / `tb_graph_impact` / `tb_graph_index` | `tb graph …` | chamada requer `TB_GRAPH=on`; `depth`/`limit` integer; `transitive` boolean |

Schemas tipam `depth`/`limit`/`reviewers`/`max_iter` como integer, `strict`/`warn_only`/`transitive`/`skip_unavailable` como boolean, `threshold` como number e `standards` como array de strings; valores string legados ainda são aceitos. Relatórios de verify/review/drift/guard voltam como JSON no `content` (sem código de saída de processo).

---

### `tb init` ✅ IMPLEMENTADO

```bash
tb init [flags]

Flags:
  --global         Inicializa em ~/.config/toolbelt
  --write-agents   Semeia AGENTS.md mínimo se ausente (nunca sobrescreve)
  --json           Saída em JSON (flag global)
```

Detalhes de detecção de toolchain e preenchimento de `verify.*` estão na seção `tb verify`.

---

### `tb sandbox` ✅ IMPLEMENTADO

Isolamento Landlock no Linux (`check` / `run`). Não cria worktrees — use `tb workspace` para isso.

```bash
tb sandbox check
tb sandbox run <command> [args...] [--workdir <path>] [--timeout <dur>]
```

`--timeout` default `30s`. Sem `--workdir`, usa o cwd. Com `--json`, `sandbox check` emite `os`/`arch`/`landlock`/`version`/`error`; `sandbox run` emite o `Result` em snake_case (`exit_code`, `timed_out`, `duration_ns`, …). Se o filho sai ≠ 0 sem timeout, `tb` propaga esse exit code.

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

---

## Erros e Usage

- Erros esperados de domínio/runtime (`RunE`, ex.: flag de negócio obrigatória ausente, falha de git, arquivo inexistente) saem com **exit ≠ 0** e mensagem em stderr **sem** dump de `Usage` do Cobra.
- Erros genuínos de parse de **flags** ou **args** (flag desconhecida, `ExactArgs`/`MinimumNArgs`, etc.) mantêm `Usage` no stderr para orientar a invocação.
- Subcomando desconhecido em qualquer nível (ex.: `tb mem nao-existe`, `tb bench nope`) sai com **exit ≠ 0** e erro `unknown command "…"`, em vez de imprimir help com exit 0.

## Códigos de Saída

A CLI atual não usa uma tabela estável de códigos 2–7. Na prática:

| Código | Significado |
|--------|-------------|
| 0 | Sucesso (`PASS`/`SKIPPED` no verify; guard sem `SeverityError` ou com `--warn-only`) |
| ≠0 (tipicamente 1) | Erro de parse/domínio, `FAIL`/`ESCALATE` no verify, `SeverityError` no guard, subcomando desconhecido |
| exit do filho | `tb sandbox run` propaga o exit code do processo quando ≠ 0 e sem timeout |

Quando o comando configurado no verify não existe/não é executável, o **processo filho** reporta 126/127 e o verify falha imediatamente (sem retries); o `tb` sai ≠ 0.

---

## Configuração

Arquivo de configuração local em `.toolbelt/config.yaml` (global:
`~/.config/toolbelt/config.yaml`). A flag `-c/--config` aponta para um YAML
alternativo.

```yaml
# .toolbelt/config.yaml (exemplo)
verify:
  test_cmd: "go test ./..."
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
