[🇺🇸 English](../../adr/009-router-e-workers.md) | 🇧🇷 Português

# ADR-009: Padrão Router e Workers para Delegação de Tarefas

## Status

proposed; experimental na 1.0.

## Contexto

Em sistemas de agentes de codificação, uma arquitetura centralizada onde um único agente processa todas as tarefas apresenta limitações:

1. **Acúmulo de contexto**: contexto cresce indefinidamente, aumentando custo e latência
2. **Gargalo serial**: tarefas independentes executam sequencialmente
3. **Escopo de falha amplo**: erro em uma subtarefa pode corromper todo o contexto
4. **Dificuldade de especialização**: um único prompt deve cobrir todos os domínios

Pesquisas em escalabilidade de agentes demonstram que a delegação para workers isolados pode melhorar significativamente throughput e custo-eficiência. Observações empíricas sugerem:

- Redução de ~40% em tokens de input via isolamento de contexto
- Aumento de ~60% em throughput via paralelização
- Melhoria em p95 de latência via distribuição de carga

**Nota**: Estas métricas são hipóteses iniciais baseadas em pesquisas de escalabilidade de agentes. Benchmarks específicos do toolbelt validarão os valores reais.

## Decisão

Adotamos o padrão **Router/Workers** inspirado em arquiteturas de delegação escalável:

```
┌─────────────────────────────────────────────────────────────────┐
│                          ROUTER                                 │
│  • Recebe tarefa do usuário/planner                            │
│  • Decompõe em subtarefas atômicas (Tickets)                   │
│  • Delega para Workers via Board                               │
│  • Não executa implementação                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │      BOARD        │
                    │  (fila de tickets)│
                    └─────────┬─────────┘
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│    WORKER A     │ │    WORKER B     │ │    WORKER C     │
│  (sandbox iso)  │ │  (sandbox iso)  │ │  (sandbox iso)  │
│  Contexto mín.  │ │  Contexto mín.  │ │  Contexto mín.  │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

### Conceitos Principais

#### Ticket

Unidade atômica de trabalho delegável:

```yaml
ticket:
  id: "TKT-001"
  title: "Implementar handler de login"
  mandate:
    scope: ["src/handlers/auth.go", "src/services/auth.go"]
    permissions: [read, write]
    forbidden: ["src/database/*", "tests/hidden/*"]
  context:
    files: ["docs/specs/auth.md", "vault/decisions/jwt-choice.md"]
    symbols: ["User", "Session", "validateCredentials"]
  acceptance:
    criteria: "Testes em tests/auth_test.go passam"
    timeout: "10m"
  dependencies: []
```

#### Mandate

Define o escopo e permissões do worker:

- **scope**: arquivos que pode ler/escrever
- **permissions**: operações permitidas
- **forbidden**: paths explicitamente bloqueados
- **resource_limits**: CPU, memória, tempo

#### Board

Fila de coordenação entre Router e Workers:

- Workers puxam tickets quando disponíveis
- Suporta priorização e dependências
- Detecta deadlocks em dependências circulares
- Fornece visibilidade do progresso

#### Watchdog

Monitor independente que observa workers:

- Detecta workers travados ou lentos
- Cancela tickets que excedem timeout
- Escala workers se fila cresce
- Reporta métricas para o Router

#### Merge

Agregador de resultados:

- Coleta outputs dos workers
- Resolve conflitos de arquivos editados por múltiplos workers
- Aplica mudanças atomicamente
- Reporta resultado consolidado ao Router

### Princípios de Delegação

1. **Contexto mínimo**: worker recebe apenas o necessário para sua tarefa
2. **Isolamento de falha**: erro em um worker não afeta outros
3. **Paralelização**: tickets independentes executam em paralelo
4. **Especialização opcional**: workers podem ter prompts especializados
5. **Transparência**: todo o fluxo é auditável
6. **Entrega direta**: follow-ups podem ir direto ao worker dono

### Entrega Direta (Direct Delivery / Subscription)

Quando um worker completa um ticket que tem dependentes, ele pode **se inscrever** para receber os tickets dependentes diretamente, sem que esses passem pelo Board. Isso é chamado de **entrega direta** (direct delivery) ou **subscription**.

```
┌─────────────────────────────────────────────────────────────────┐
│                    FLUXO VIA BOARD (padrão)                     │
│  Router → Board → Worker A (TKT-001 done)                       │
│  Router → Board → Worker B (TKT-002 depende de TKT-001)         │
│                                                                 │
│  Latência: ticket passa pelo Board, worker B pode ser outro     │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    ENTREGA DIRETA (subscription)                │
│  Router → Board → Worker A (TKT-001 done)                       │
│           Worker A se inscreve para TKT-002                     │
│  Router ────────→ Worker A (TKT-002 direto, sem Board)          │
│                                                                 │
│  Latência: ticket vai direto, contexto do TKT-001 reutilizado   │
└─────────────────────────────────────────────────────────────────┘
```

**Benefícios da entrega direta**:

1. **Menor latência**: ticket não espera na fila do Board
2. **Afinidade de contexto**: worker que fez TKT-001 já tem contexto relevante para TKT-002
3. **Cache-read maximizado**: tokens de contexto do ticket anterior estão em cache
4. **Menos conflitos**: mesmo worker edita arquivos relacionados sequencialmente

**Mecanismo de subscription**:

```yaml
ticket:
  id: "TKT-001"
  title: "Criar modelo User"
  # ... outros campos ...
  
  subscription:
    worker: "worker-001"           # worker que completou este ticket
    subscribed_to: ["TKT-002"]     # tickets que este worker receberá diretamente
    ttl: "30m"                     # subscription expira se ticket não chegar
    direct_delivery: true          # habilita entrega direta
```

**Regras de subscription**:

- Worker só pode se inscrever para tickets que **dependem** de um que ele completou
- Subscription tem TTL (time-to-live) para evitar locks eternos
- Se worker falhar ou subscription expirar, ticket volta para o Board
- Router pode vetar subscription se worker estiver sobrecarregado

**Configuração do Router**:

```yaml
router:
  direct_delivery:
    enabled: true
    subscription_ttl: 30m
    max_subscriptions_per_worker: 5
    veto_on_worker_load: 80%  # veta se worker > 80% utilização
```

### Fluxo de Execução

```
1. Router recebe tarefa complexa
2. Router decompõe em tickets atômicos
3. Router analisa dependências e ordena tickets
4. Tickets sem dependência vão para o Board
5. Workers disponíveis puxam tickets
6. Cada worker:
   a. Spawna em sandbox isolado
   b. Recebe apenas arquivos do mandate
   c. Executa com contexto mínimo
   d. Retorna resultado ou erro
   e. (opcional) Se inscreve para tickets dependentes
7. Watchdog monitora timeouts e saúde
8. Tickets dependentes são:
   a. Entregues diretamente ao worker subscrito, OU
   b. Liberados para o Board se não houver subscription
9. Merge agrega todos os resultados
10. Router reporta sucesso/falha consolidado
```

## Consequências

### Positivas

- **Redução de contexto**: cada worker tem contexto ~80% menor que abordagem monolítica
- **Paralelização**: tarefas independentes executam simultaneamente
- **Isolamento**: falha em um worker não polui contexto dos outros
- **Escalabilidade**: adicionar workers aumenta throughput linearmente
- **Auditoria**: cada ticket tem histórico isolado e rastreável
- **Cache-read**: workers aproveitam cache de tokens via contextos repetidos
- **Especialização**: workers podem ser otimizados por domínio

### Negativas

- **Overhead de coordenação**: Router + Board + Merge adicionam latência
- **Complexidade de merge**: conflitos entre workers requerem resolução
- **Decomposição imperfeita**: tickets mal definidos podem falhar ou duplicar trabalho
- **Custo de spawn**: cada worker tem custo de inicialização
- **Dependências complexas**: grafos de dependência podem criar gargalos

### Métricas de Sucesso

| Métrica | Meta | Justificativa |
|---------|------|---------------|
| cache_read_tokens_pct | ≥ 70% | Workers reusam contexto |
| router_latency_p50 | < 500ms | Delegação deve ser rápida |
| tickets_parallel_max | ≥ 3 | Paralelização efetiva |
| merge_conflict_rate | < 5% | Mandates bem definidos |
| direct_delivery_rate | ≥ 50% | Entrega direta reduz latência |

## Alternativas Consideradas

### Monolito com Contexto Comprimido

- **Descrição**: manter agente único, comprimir contexto via resumos
- **Por que não**: não resolve gargalo serial nem escopo de falha

### Workers Sem Router

- **Descrição**: workers autônomos que competem por arquivos
- **Por que não**: conflitos frequentes, sem coordenação de dependências

### Microservices Stateless

- **Descrição**: workers como funções puras sem estado
- **Por que não**: overhead de serialização/deserialização do estado do código

## Implementação (Fase 3)

Implementado (ver [cli.md](../cli.md)): `tb ticket` (tickets YAML em
`.toolbelt/tickets/`), `tb board` (pause/resume/drain), `tb router run`
(scheduler **em primeiro plano**, workers `w1..wN` em worktrees próprias,
execução dentro do sandbox fail-closed), entrega direta (subscription com TTL e
limite por worker; base no commit da dependência), `tb watchdog` (alertas
`worker_slow`, `worker_stuck`, `worker_crash`, `worker_killed`, `queue_growing`,
`global_timeout`, `ticket_canceled`, `multi_dep_base`, `agent_escaped`, `board_unreadable`; roda no loop do router, sem auto-scaling) e `tb merge`
(estratégias `fail`, `last_wins`, `semantic` e `prompt`, atômico via compare-and-swap).

Endurecimento do router: instância única por repositório (lock exclusivo
`.toolbelt/router/lock`), verificação de integridade do worktree antes do commit
(git com fsmonitor/hooks desabilitados, `--no-verify`; violação ⇒ `failed` sem
retry), saída do agente limitada a 1 MiB, resumo `stalled` (exit ≠ 0) quando
tickets `pending` ficam bloqueados por dependências `failed`/`canceled`, e base
no `head_commit` da dependência única independentemente da entrega direta
(≥ 2 dependências: `HEAD` + alerta `multi_dep_base`).

Isolamento do git do router (que roda fora do sandbox): `.git` aninhado no
worktree, gitlinks e `.gitmodules` adicionados pelo agente são recusados
(`failed` sem retry); submódulos são ignorados no git do router. No Linux o
router é *subreaper* e mata descendentes que escapam do grupo de processos
(`setsid`/`setpgid`), falhando o ticket com `agent_escaped` sem rodar git;
agentes não devem se desacoplar. Em macOS/Windows isso não é detectado
(limitação). O mandato (`Scope`/`Forbidden`/`Permissions`) é só registrado: o
router **não o impõe** — use `tb drift`/`tb review`/`tb guard` (ou
`tb stack --ticket`) antes de `tb merge`.

Diferenças em relação ao desenho acima: sem daemon `start`/`stop`; configuração
na seção `router:` de `.toolbelt/config.yaml` (sem `router.yaml`);
`semantic` e `prompt` do merge estão implementados ([spec](../specs/merge-strategies.md): o `prompt` nunca move uma ref na chamada que consulta o agente, um recibo revisado é reaplicado por uma segunda chamada, e o router continua sem chamar o merge); `resource_high` e
`watchdog scale` **não existem**.

Ainda hipóteses, **não medidas** com o benchmark do toolbelt: redução de ~40%
em tokens de input, ganho de ~60% em throughput e melhoria de p95 — por isso o
status permanece `proposed`.

## Referências

- Pesquisas em escalabilidade de agentes de codificação
- Padrões de sistemas distribuídos: work queues, fan-out/fan-in
- Estudos de isolamento de contexto em LLMs
