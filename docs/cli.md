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
