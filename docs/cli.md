# Árvore de Comandos do `tb`

Este documento descreve a interface de linha de comando planejada para o toolbelt.

## Visão Geral

```
tb
├── run          Executa comandos no sandbox
├── proc         Gerencia processos de longa duração
├── graph        Consulta e gerencia o grafo de conhecimento
├── mem          Gerencia a memória (vault)
├── md           TUI para visualização e edição do vault
├── agent        Gerencia agentes
├── guard        Proteções e verificações de segurança
├── review       Revisão com contextos frescos (Elephant-Goldfish)
├── drift        Detecta divergência entre docs e código
├── verify       Verificação adversarial (gerador vs discriminador)
├── bench        Executa benchmarks
├── serve        Inicia servidor MCP
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

### `tb graph` — Grafo de Conhecimento

Consulta e gerencia o grafo derivado do código e do vault.

```bash
tb graph <subcommand>

Subcomandos:
  search    Busca no grafo
  callers   Encontra chamadores de uma função
  deps      Mostra dependências
  impact    Analisa impacto de mudanças
  index     Re-indexa código
  rebuild   Reconstrói grafo completo
  stats     Estatísticas do grafo
  export    Exporta grafo
```

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

### `tb mem` — Gerenciamento de Memória

Gerencia o vault Markdown e a memória persistente.

```bash
tb mem <subcommand>

Subcomandos:
  add       Adiciona fato ou decisão
  search    Busca na memória
  list      Lista entradas
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

### `tb review` — Revisão com Contextos Frescos

Implementa o padrão Elephant-Goldfish: spawna N revisores em contextos frescos para revisar documentos ou PRs.

```bash
tb review <target> [flags]

Targets:
  <path>              Arquivo ou diretório de docs
  pr:<number>         Pull request por número
  pr:<url>            Pull request por URL

Flags:
  --reviewers <n>       Número de revisores goldfish (default: 3)
  --roles <roles>       Roles específicos para revisores (separados por vírgula)
  --standards <stds>    Padrões de revisão: security,clarity,completeness,consistency
  --severity <level>    Mínimo de severity para reportar: info,warning,error,critical
  --output <path>       Arquivo de saída do relatório
  --json                Saída em JSON
  --resolve             Inicia etapa de resolução interativa após revisão
  --timeout <dur>       Timeout por revisor (default: 5m)
```

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

### `tb drift` — Detecção de Divergência

Compara documentação (vault ou docs) contra o grafo de código e reporta divergências.

```bash
tb drift [path] [flags]

Argumentos:
  [path]               Caminho para analisar (default: todo o vault)

Flags:
  --against <target>   Comparar contra: code,vault,both (default: code)
  --depth <level>      Profundidade: shallow,medium,deep (default: medium)
  --format <fmt>       Formato: table,json,markdown (default: table)
  --output <path>      Arquivo de saída
  --ignore <patterns>  Padrões a ignorar (globs)
  --since <ref>        Comparar desde commit/tag/data
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

### `tb verify` — Verificação Adversarial

Executa o loop gerador→discriminador com rulers plugáveis.

```bash
tb verify <task> [flags]

Flags:
  --ruler <type>        Tipo de ruler: visual|tests|review (obrigatório)
  --max-iter <n>        Máximo de iterações (default: 3)
  --threshold <pct>     Threshold de passagem (default: depende do ruler)
  --manifest <path>     Manifesto de telas/testes (default: .toolbelt/verify.yaml)
  --output <path>       Arquivo de saída do relatório
  --json                Saída em JSON
  --discriminator-only  Executa apenas discriminador (para debug)
  --skip-guard          Pula guard check (apenas para testes)
```

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

### `tb bench` — Benchmark

Executa benchmarks e mede KPIs.

```bash
tb bench <subcommand>

Subcomandos:
  run       Executa suite de benchmark
  compare   Compara resultados
  report    Gera relatório
  list      Lista suites disponíveis
```

**`tb bench run`:**

```bash
tb bench run [flags] <suite>

Flags:
  --baseline           Executa como baseline (agente sozinho)
  --toolbelt           Executa com toolbelt
  --repetitions <n>    Número de repetições (default: 5)
  --output <path>      Arquivo de saída
  --graph-first        Força abordagem graph-first
  --grep-first         Força abordagem grep-first
```

**Exemplos:**

```bash
# Executar benchmark baseline
tb bench run --baseline --repetitions 10 coding-tasks

# Executar com toolbelt
tb bench run --toolbelt --repetitions 10 coding-tasks

# Comparar abordagens de busca
tb bench run --graph-first coding-tasks --output graph-results.json
tb bench run --grep-first coding-tasks --output grep-results.json
tb bench compare graph-results.json grep-results.json

# Gerar relatório
tb bench report --input results.json --format markdown > report.md

# Listar suites
tb bench list
# SUITE           TASKS   DESCRIPTION
# coding-tasks    50      Tarefas de implementação
# refactoring     30      Tarefas de refatoração
# debugging       25      Tarefas de debugging
# documentation   20      Tarefas de documentação
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

### `tb version`

```bash
tb version [flags]

Flags:
  --json    Saída em JSON
  --short   Apenas versão, sem detalhes
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

## Variáveis de Ambiente

| Variável | Descrição | Default |
|----------|-----------|---------|
| `TB_HOME` | Diretório de dados do toolbelt | `.tb/` |
| `TB_VAULT` | Caminho do vault Markdown | `$TB_HOME/vault/` |
| `TB_LOG_LEVEL` | Nível de log: debug, info, warn, error | `info` |
| `TB_SANDBOX_DEFAULT` | Sandbox por padrão em `tb run` | `false` |
| `TB_JSON_OUTPUT` | Saída JSON por padrão | `false` |
| `TB_TIMEOUT_DEFAULT` | Timeout padrão para comandos | `30s` |

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
