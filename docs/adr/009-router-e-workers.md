# ADR-009: Padrão Router e Workers para Delegação de Tarefas

## Status

proposed

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
7. Watchdog monitora timeouts e saúde
8. Tickets dependentes são liberados quando deps completam
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

## Referências

- Pesquisas em escalabilidade de agentes de codificação
- Padrões de sistemas distribuídos: work queues, fan-out/fan-in
- Estudos de isolamento de contexto em LLMs
