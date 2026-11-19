🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/011-grafo-sob-demanda.md)

# ADR-011: On-Demand Code Graph with A/B

## Status

accepted (Phase 2 implementation: `tb graph` + MCP + A/B `bench/ab/graph-*.yaml`)

## Context

The common intuition is that code graphs (AST, call graph, dependency graph) always save tokens by avoiding unnecessary file reads. However, recent research challenges that assumption:

- [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) (Sepidband et al., 2026, preprint): code maps cut 17–39% of tokens **when locating is the bottleneck**, but **increase latency** and do not always pay off
- [Analyzing and Mitigating Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) (Hu et al., 2026, preprint): structural search can **cost up to 28.14% more** in certain scenarios; human-written skills cut up to 41.73%

The implication is that toolbelt should not force graph use on every task, but offer it as an optional tool with measurement of when it helps vs. when it costs more.

## Decision

We adopt an **on-demand graph with A/B measurement** approach:

### 1. Graph as a Tool, Not a Prerequisite

```yaml
# The graph is a tool the agent can choose to use
tools:
  - name: tb_graph_search
    description: "Busca semântica no grafo de código"
    optional: true
    
  - name: tb_graph_callers
    description: "Encontra chamadores de uma função"
    optional: true
    
  - name: tb_graph_impact
  - name: tb_graph_deps
    description: "Lista dependências de um módulo"
    optional: true
```

**Change**: the "consult the graph before reading files" hook becomes a **recommendation**, not an obligation.

### 2. Lean Output by Default

```bash
# Before (verbose)
tb graph search "autenticação"
# Retorna: título, descrição, código, contexto, arquivos relacionados...

# After (lean)
tb graph search "autenticação" --format compact
# Retorna: [file:line] título (1 linha por resultado)

# Details on demand
tb graph search "autenticação" --file src/auth.go --expand
```

### 3. A/B by Task Type

The benchmark includes A/B experiments:

| Task Type | Condition A | Condition B | Metrics |
|-----------|-------------|-------------|---------|
| Location in large repo | Without graph | With graph | tokens, time, accuracy |
| Localized bug fix | Without graph | With graph | tokens, time, accuracy |
| Cross-file refactoring | Without graph | With graph | tokens, time, accuracy |
| Feature implementation | Without graph | With graph | tokens, time, accuracy |

**Hypothesis to test**: graph pays off more for location and cross-file refactoring, less for localized bug fixes.

### 4. Waste Watchdog

The router detects inefficiency patterns identified in the study:

```yaml
# Patterns to detect (appeared in 79–98% of tasks in the study)
inefficiency_patterns:
  - redundant_search: "Mesma query executada 2+ vezes"
  - similar_scripts: "Scripts com >80% de similaridade executados"
  - test_reexecution: "Mesmos testes executados sem mudança de código"
  - excessive_graph_calls: ">10 chamadas de grafo sem leitura de arquivo"
```

The watchdog reports waste metrics as KPIs.

### 5. Human Skills Prioritized

Based on the finding that human-written skills cut up to 41.73% of cost:

```yaml
# Skills have priority over automatic search
skills:
  - name: "localizar-handler-go"
    description: "Localiza handlers HTTP em projetos Go"
    trigger: "busca por handler, endpoint, rota em projeto Go"
    steps:
      - "grep -r 'func.*Handler' --include='*.go'"
      - "filtrar por padrão http.HandleFunc ou chi.Route"
    # Skill humana evita N chamadas de grafo
```

## Consequences

### Positive

- **Real cost measured**: A/B identifies when the graph is worth the investment
- **Flexibility**: agent chooses the right tool for the task
- **Less waste**: watchdog detects inefficient patterns
- **Reusable skills**: human knowledge captured and prioritized

### Negative

- **Benchmark complexity**: requires A/B infrastructure
- **Variance**: results depend on task type and repo size
- **Underuse risk**: agent may not use the graph when it should
- **Skill maintenance**: requires continuous human curation

### Success Metrics

| Metric | Target | Justification |
|--------|--------|---------------|
| graph_roi_positive_pct | ≥ 60% | Graph should pay off on most tasks where it is used |
| wasteful_pattern_rate | ≤ 20% | Waste patterns should be rare |
| skill_hit_rate | ≥ 30% | Human skills should fire when applicable |

## Alternatives Considered

### Mandatory Graph

- **Description**: force graph consultation before any file read
- **Why not**: research shows it can cost up to 28.14% more in certain scenarios

### Graph Disabled by Default

- **Description**: graph only enabled explicitly by the user
- **Why not**: loses the 17–39% savings benefit when location is the bottleneck

### Static Heuristic

- **Description**: fixed rule (e.g. "use graph if repo > 1000 files")
- **Why not**: does not capture variation by task type; A/B is more precise

## References

- [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) — Sepidband et al., 2026 (preprint)
- [Analyzing and Mitigating Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) — Hu et al., 2026 (preprint)
- [Improving agent with semantic search](https://cursor.com/blog/semsearch) — Cursor, 2025
- [Building a better repository map with tree sitter](https://aider.chat/2023/10/22/repomap.html) — Aider, 2023
