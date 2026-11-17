[🇧🇷 Português](../../adr/009-router-e-workers.md) | 🇺🇸 English

# ADR-009: Router and Workers Pattern for Task Delegation

## Status

proposed

## Context

In coding agent systems, a centralized architecture where a single agent processes all tasks has limitations:

1. **Context accumulation**: context grows indefinitely, increasing cost and latency
2. **Serial bottleneck**: independent tasks run sequentially
3. **Wide failure scope**: an error in one subtask can corrupt the entire context
4. **Specialization difficulty**: a single prompt must cover all domains

Research on agent scalability shows that delegation to isolated workers can significantly improve throughput and cost-efficiency. Empirical observations suggest:

- ~40% reduction in input tokens via context isolation
- ~60% increase in throughput via parallelization
- Improvement in p95 latency via load distribution

**Note**: These metrics are initial hypotheses based on agent scalability research. toolbelt-specific benchmarks will validate the real values.

## Decision

We adopt the **Router/Workers** pattern inspired by scalable delegation architectures:

```
┌─────────────────────────────────────────────────────────────────┐
│                          ROUTER                                 │
│  • Receives task from user/planner                              │
│  • Decomposes into atomic subtasks (Tickets)                    │
│  • Delegates to Workers via Board                               │
│  • Does not execute implementation                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │      BOARD        │
                    │  (ticket queue)   │
                    └─────────┬─────────┘
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│    WORKER A     │ │    WORKER B     │ │    WORKER C     │
│  (iso sandbox)  │ │  (iso sandbox)  │ │  (iso sandbox)  │
│  Min. context   │ │  Min. context   │ │  Min. context   │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

### Main Concepts

#### Ticket

Atomic unit of delegable work:

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

Defines the worker's scope and permissions:

- **scope**: files it can read/write
- **permissions**: allowed operations
- **forbidden**: explicitly blocked paths
- **resource_limits**: CPU, memory, time

#### Board

Coordination queue between Router and Workers:

- Workers pull tickets when available
- Supports prioritization and dependencies
- Detects deadlocks in circular dependencies
- Provides progress visibility

#### Watchdog

Independent monitor that observes workers:

- Detects stuck or slow workers
- Cancels tickets that exceed timeout
- Scales workers if the queue grows
- Reports metrics to the Router

#### Merge

Result aggregator:

- Collects worker outputs
- Resolves conflicts on files edited by multiple workers
- Applies changes atomically
- Reports consolidated result to the Router

### Delegation Principles

1. **Minimal context**: worker receives only what is needed for its task
2. **Failure isolation**: an error in one worker does not affect others
3. **Parallelization**: independent tickets run in parallel
4. **Optional specialization**: workers can have specialized prompts
5. **Transparency**: the entire flow is auditable
6. **Direct delivery**: follow-ups can go straight to the owning worker

### Direct Delivery (Subscription)

When a worker completes a ticket that has dependents, it can **subscribe** to receive dependent tickets directly, without those going through the Board. This is called **direct delivery** or **subscription**.

```
┌─────────────────────────────────────────────────────────────────┐
│                    FLOW VIA BOARD (default)                     │
│  Router → Board → Worker A (TKT-001 done)                       │
│  Router → Board → Worker B (TKT-002 depends on TKT-001)         │
│                                                                 │
│  Latency: ticket goes through Board, worker B may be another    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    DIRECT DELIVERY (subscription)               │
│  Router → Board → Worker A (TKT-001 done)                       │
│           Worker A subscribes to TKT-002                        │
│  Router ────────→ Worker A (TKT-002 direct, no Board)           │
│                                                                 │
│  Latency: ticket goes direct, TKT-001 context reused            │
└─────────────────────────────────────────────────────────────────┘
```

**Benefits of direct delivery**:

1. **Lower latency**: ticket does not wait in the Board queue
2. **Context affinity**: worker that did TKT-001 already has relevant context for TKT-002
3. **Maximized cache-read**: previous ticket context tokens are in cache
4. **Fewer conflicts**: same worker edits related files sequentially

**Subscription mechanism**:

```yaml
ticket:
  id: "TKT-001"
  title: "Criar modelo User"
  # ... other fields ...
  
  subscription:
    worker: "worker-001"           # worker that completed this ticket
    subscribed_to: ["TKT-002"]     # tickets this worker will receive directly
    ttl: "30m"                     # subscription expires if ticket does not arrive
    direct_delivery: true          # enables direct delivery
```

**Subscription rules**:

- Worker can only subscribe to tickets that **depend** on one it completed
- Subscription has a TTL (time-to-live) to avoid eternal locks
- If the worker fails or the subscription expires, the ticket returns to the Board
- Router can veto subscription if the worker is overloaded

**Router configuration**:

```yaml
router:
  direct_delivery:
    enabled: true
    subscription_ttl: 30m
    max_subscriptions_per_worker: 5
    veto_on_worker_load: 80%  # vetoes if worker > 80% utilization
```

### Execution Flow

```
1. Router receives a complex task
2. Router decomposes into atomic tickets
3. Router analyzes dependencies and orders tickets
4. Tickets without dependencies go to the Board
5. Available workers pull tickets
6. Each worker:
   a. Spawns in an isolated sandbox
   b. Receives only files from the mandate
   c. Executes with minimal context
   d. Returns result or error
   e. (optional) Subscribes to dependent tickets
7. Watchdog monitors timeouts and health
8. Dependent tickets are:
   a. Delivered directly to the subscribed worker, OR
   b. Released to the Board if there is no subscription
9. Merge aggregates all results
10. Router reports consolidated success/failure
```

## Consequences

### Positive

- **Context reduction**: each worker has ~80% less context than a monolithic approach
- **Parallelization**: independent tasks run simultaneously
- **Isolation**: failure in one worker does not pollute others' context
- **Scalability**: adding workers increases throughput linearly
- **Audit**: each ticket has an isolated, traceable history
- **Cache-read**: workers leverage token cache via repeated contexts
- **Specialization**: workers can be optimized by domain

### Negative

- **Coordination overhead**: Router + Board + Merge add latency
- **Merge complexity**: conflicts between workers require resolution
- **Imperfect decomposition**: poorly defined tickets may fail or duplicate work
- **Spawn cost**: each worker has an initialization cost
- **Complex dependencies**: dependency graphs can create bottlenecks

### Success Metrics

| Metric | Target | Justification |
|--------|--------|---------------|
| cache_read_tokens_pct | ≥ 70% | Workers reuse context |
| router_latency_p50 | < 500ms | Delegation must be fast |
| tickets_parallel_max | ≥ 3 | Effective parallelization |
| merge_conflict_rate | < 5% | Well-defined mandates |
| direct_delivery_rate | ≥ 50% | Direct delivery reduces latency |

## Alternatives Considered

### Monolith with Compressed Context

- **Description**: keep a single agent, compress context via summaries
- **Why not**: does not solve the serial bottleneck or failure scope

### Workers Without Router

- **Description**: autonomous workers that compete for files
- **Why not**: frequent conflicts, no dependency coordination

### Stateless Microservices

- **Description**: workers as pure functions without state
- **Why not**: overhead of serializing/deserializing code state

## References

- Research on coding agent scalability
- Distributed systems patterns: work queues, fan-out/fan-in
- Studies of context isolation in LLMs
