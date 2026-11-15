# Árvore de Comandos do `tb`

Este documento descreve a interface de linha de comando planejada para o toolbelt.

## Visão Geral

```
tb
├── run          Executa comandos no sandbox
├── proc         Gerencia processos de longa duração
├── graph        Grafo de código sob demanda (✓ Fase 2)
├── mem          Gerencia a memória (vault)
├── md           TUI para visualização e edição do vault
├── agent        Gerencia agentes
├── guard        Proteções e verificações de segurança
├── review       Revisão fresh-context (✓ Fase 2)
├── drift        Drift de escopo/base (✓ Fase 2)
├── verify       Verificação adversarial (gerador vs discriminador)
├── ticket       Gerencia tickets de trabalho
├── board        Visualiza e gerencia o board de tarefas
├── router       Controla o router de delegação
├── watchdog     Monitora saúde dos workers
├── merge        Agrega resultados de workers
├── bench        Executa benchmarks
├── mcp          Servidor MCP sobre stdio (✓ Fase 1)
└── version      Exibe versão
```

---

## Comandos Detalhados

### `tb run` — Execução de Comandos

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

**`tb mem add`:**

```bash
tb mem add [flags]

Flags:
  --type <type>       Tipo: fact|decision|summary|note
  --title <title>     Título da entrada
  --tags <tags>       Tags separadas por vírgula
  --links <links>     Wikilinks para outras entradas
  --source <source>   Proveniência: manual|extracted|inferred
  --file <path>       Arquivo Markdown existente
```

**Exemplos:**

```bash
# Adicionar fato
tb mem add --type fact --title "Convenção de Nomes Go" \
  --tags "convention,go" \
  "Usamos camelCase para variáveis locais e PascalCase para exportadas."

# Adicionar decisão
tb mem add --type decision --title "SQLite para storage" \
  --tags "architecture,database" \
  --links "adr-003,database-module" \
  "Decidimos usar SQLite por simplicidade e performance."

# Buscar na memória
tb mem search "autenticação"

# Listar decisões recentes
tb mem list --type decision --limit 10 --sort updated

# Criar link
tb mem link "auth-module" "jwt-validation"

# Sincronizar manualmente
tb mem sync
```

---

### `tb md` — TUI Markdown

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

### `tb guard` — Proteções de Segurança

Gerencia snapshots, verificações e proteções contra trapaça.

```bash
tb guard <subcommand>

Subcomandos:
  snapshot  Cria snapshot de arquivos
  verify    Verifica integridade de snapshot
  protect   Marca arquivos como protegidos
  audit     Consulta log de auditoria
  check     Verifica padrões suspeitos
```

**`tb guard snapshot`:**

```bash
tb guard snapshot [flags]

Flags:
  --paths <patterns>  Globs de arquivos para snapshot
  --id <id>           ID do snapshot (auto-gerado se omitido)
  --verify-before     Verifica snapshot anterior antes de criar novo
```

**`tb guard verify`:**

```bash
tb guard verify [flags]

Flags:
  --snapshot-id <id>  ID do snapshot a verificar
  --paths <patterns>  Verificar apenas estes caminhos
  --strict            Falha em qualquer divergência
```

**Exemplos:**

```bash
# Snapshot antes de tarefa
tb guard snapshot --paths "tests/**" "benchmarks/**"
# Snapshot criado: snap-20260115-103000
# Arquivos: 45
# Hash: sha256:abc123...

# Verificar após tarefa
tb guard verify --snapshot-id snap-20260115-103000
# Todos os 45 arquivos intactos

# Se houver violação:
# VIOLAÇÃO DETECTADA
# - tests/auth_test.go: MODIFICADO (hash diverge)
# - tests/old_test.go: DELETADO

# Marcar arquivos como permanentemente protegidos
tb guard protect --paths "benchmarks/**" --mode readonly

# Verificar padrões suspeitos
tb guard check --type reward-hacking --path "src/"
# Verificando padrões de reward hacking...
# t.Skip() adicionado em src/auth/auth_test.go:45
# String literal suspeita em src/parser/parse_test.go:78

# Consultar auditoria
tb guard audit --since "1h" --action "file:*"
# TIMESTAMP            AGENT        ACTION       RESOURCE          STATUS
# 2026-01-15T10:30:00  worker-001   file:write   src/parser.go     approved
# 2026-01-15T10:31:00  worker-001   file:delete  src/old.go        blocked
```

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

Detecta mudanças fora do scope declarado no ticket/rd.md e divergência
do commit base (mandate).

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
  --ruler <type>           Tipo de ruler: tests|visual (default: tests) ✅
  --max-iter <n>           Máximo de iterações (default: 3)
  --threshold <pct>        Threshold de similaridade visual (default: 0.95) ✅
  --manifest <path>        Manifesto de telas (default: .toolbelt/screens.yaml) ✅
  --skip-unavailable       Skip gracioso sem browser/refs (default: true) ✅
  --output <path>          Arquivo de saída do relatório
  --json                   Saída em JSON
```

Ruler `visual` (docs/verificacao-visual.md): captura headless + diff de pixels;
se Chromium/Chrome ou referências estiverem ausentes, faz skip gracioso.

**Exemplos:**

```bash
# Verificação visual de UI
tb verify ui-implementation --ruler visual --threshold 95
# ═══════════════════════════════════════════════════════════════
# VERIFICAÇÃO ADVERSARIAL: ui-implementation
# Ruler: visual | Threshold: 95% | Max iterações: 3
# ═══════════════════════════════════════════════════════════════
# 
# [Iteração 1/3]
# ├── Guard check: OK (0 paths protegidos modificados)
# ├── Clean build: 45s
# ├── Captura: 8 telas × 2 modos = 16 screenshots
# ├── Comparação:
# │   ┌─────────────┬────────┬────────┬──────────┐
# │   │ Tela        │ Light  │ Dark   │ Tamanho  │
# │   ├─────────────┼────────┼────────┼──────────┤
# │   │ login       │ 97.2%  │ 95.8%  │ OK       │
# │   │ dashboard   │ 92.1%  │ 91.5%  │ OK       │  ← FAIL
# │   │ settings    │ 98.4%  │ 97.9%  │ OK       │
# │   │ profile     │ 96.0%  │ 95.2%  │ SIZE≠    │  ← FAIL
# │   └─────────────┴────────┴────────┴──────────┘
# ├── Contraste WCAG: 2 violações
# └── Veredicto: FAIL
# 
# Gaps encontrados:
# G1: src/components/Dashboard.tsx:45
#     Cor de fundo difere 8% → usar var(--bg-secondary)
# G2: src/pages/Profile.tsx:120
#     Altura do container 450px, esperado 480px
# 
# Retornando ao gerador...

# Verificação de código com testes
tb verify feature-auth --ruler tests
# Executa testes ocultos como discriminador

# Verificação de documentação
tb verify design-spec --ruler review --threshold 80
# Spawna revisores fresh-context como discriminador

# Apenas discriminador (debug)
tb verify ui-fix --ruler visual --discriminator-only
# Roda uma passagem do discriminador sem loop

# Saída JSON para CI
tb verify ui --ruler visual --json --output verify-result.json
```

**Rulers disponíveis:**

| Ruler | Métrica | Threshold Default | Trapaça Específica |
|-------|---------|-------------------|-------------------|
| `visual` | Similaridade pixel + WCAG | 95% | Edição de refs, masks inválidas |
| `tests` | Passagem em testes ocultos | 100% | Modificação de testes |
| `review` | Cobertura de revisão | 80% | Contexto contaminado |

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
tb guard check <base>..<head> [flags]

Flags:
  --manifest <path>     Manifesto de paths protegidos (default: .toolbelt/guard.yaml)
  --strict              Falha se qualquer path protegido foi modificado sem decisão
  --json                Saída em JSON
  --output <path>       Arquivo de saída
```

**Exemplos:**

```bash
# Verificar mudanças entre main e branch atual
tb guard check main..HEAD
# ═══════════════════════════════════════════════════════════════
# GUARD CHECK: main..HEAD
# ═══════════════════════════════════════════════════════════════
# 
# Paths protegidos modificados: 2
# 
# ┌───────────────────────────────────────────────────────────────┐
# │ references/login-light.png                                    │
# │ ├── Regra: "Referências visuais requerem decisão"             │
# │ ├── Modificação: DELETED                                      │
# │ └── Decisão: ⚠ NÃO ENCONTRADA                                 │
# ├───────────────────────────────────────────────────────────────┤
# │ .toolbelt/thresholds.yaml                                     │
# │ ├── Regra: "Thresholds não podem ser relaxados"               │
# │ ├── Modificação: min_similarity 95 → 90                       │
# │ └── Decisão: ⚠ NÃO ENCONTRADA                                 │
# └───────────────────────────────────────────────────────────────┘
# 
# ⚠ 2 modificações requerem justificativa
# 
# Para aprovar, crie decisões em:
#   tb mem add --type decision --title "Remoção de login-light.png" ...
#   tb mem add --type decision --title "Relaxamento de threshold" ...

# Modo strict para CI
tb guard check origin/main..HEAD --strict
# Exit code 1 se houver modificação sem decisão

# Saída JSON
tb guard check main..HEAD --json
```

**Manifesto de paths protegidos (`.toolbelt/guard.yaml`):**

```yaml
# .toolbelt/guard.yaml
version: 1

protected_paths:
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

### `tb ticket` — Gerenciamento de Tickets

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

### `tb board` — Visualização do Board

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
  --fake               Usa agente fake (CI)
  --agent <cmd>        Comando do agente externo
  --repetitions <n>    Número de repetições (default: 1)
  --output <path>      Arquivo de saída JSONL
  --label <name>       Rótulo da variante A/B
  --flag key=value     Feature flag (repetível)
  --ab-config <yaml>   Variante A/B a partir de YAML
  --task <id>          Filtra tarefas
```

**`tb bench compare` (A/B):**

```bash
# Compara dois JSONL já gerados
tb bench compare --before graph-off.jsonl --after graph-on.jsonl

# Roda duas configs no mesmo conjunto de tarefas e compara
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml --fake
```

O relatório Markdown e o JSONL incluem métricas de **sucesso (pass@1)**, **tempo** e **tokens**, com deltas A→B.

**Exemplos:**

```bash
tb bench run --fake --label graph-off --flag graph=off
tb bench run --fake --ab-config bench/ab/graph-on.yaml
tb bench compare --before bench/results/graph-off.jsonl \
                 --after bench/results/graph-on.jsonl
tb bench list
tb bench report --input bench/results/graph-on.jsonl
```

---

### `tb serve` — Servidor MCP

Inicia o toolbelt como servidor MCP.

```bash
tb serve [flags]

Flags:
  --transport <type>  Transporte: stdio|unix|tcp (default: stdio)
  --socket <path>     Caminho do Unix socket (para --transport unix)
  --port <port>       Porta TCP (para --transport tcp)
  --tls-cert <path>   Certificado TLS (para --transport tcp)
  --tls-key <path>    Chave TLS (para --transport tcp)
```

**Exemplos:**

```bash
# Modo padrão (stdio) para integração com IDE
tb serve

# Unix socket para comunicação local entre processos
tb serve --transport unix --socket /tmp/tb.sock

# TCP com TLS para acesso remoto
tb serve --transport tcp --port 8443 --tls-cert cert.pem --tls-key key.pem
```

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
