🇺🇸 English | [🇧🇷 Português](pt-BR/benchmark.md)

# Benchmark and KPIs

This document defines the benchmark methodology and the KPIs (Key Performance Indicators) for evaluating toolbelt effectiveness.

## Objective

Quantitatively measure whether toolbelt achieves its three goals:

1. **Save tokens** — fewer tokens consumed per task
2. **Improve accuracy** — higher first-attempt success rate
3. **Increase speed** — lower execution time

---

## Methodology

### Experimental Design

```
┌─────────────────────────────────────────────────────────────────┐
│                      Task Suite                                 │
│  (closed code, hidden tests, reproducible)                      │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│      Baseline Group     │     │    Experimental Group   │
│  (agent alone)          │     │  (agent + toolbelt)     │
│                         │     │                          │
│  - Direct access to     │     │  - Access via tb graph   │
│    code (grep/read)     │     │  - Isolated sandbox      │
│  - No isolation         │     │  - Persistent memory     │
│  - No structured        │     │  - Active guardrails     │
│    memory               │     │                          │
└─────────────────────────┘     └─────────────────────────┘
              │                               │
              └───────────────┬───────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Isolated Evaluator                         │
│  (runs hidden tests, measures metrics, generates report)        │
└─────────────────────────────────────────────────────────────────┘
```

### Controlled Conditions

| Aspect | Control |
|--------|---------|
| Language model | Same model and temperature for both groups |
| Tasks | Fixed suite, shuffled, no prior access |
| Tests | Hidden from the implementing agent |
| Environment | Same hardware/VM, same initial repository state |
| Evaluation | Automated, deterministic, no human intervention |

### Repetitions and Variance

- **Minimum repetitions**: 5 per task per condition
- **Recommended**: 10 repetitions for statistical significance
- **Measures**: mean, median, standard deviation, 95% confidence interval

---

## Primary KPIs

### 1. pass@1 (First-Attempt Success Rate)

**Definition**: Proportion of tasks completed correctly on the first attempt, without rework.

```
pass@1 = (tasks correct on 1st attempt) / (total tasks)
```

**Measurement**:
- The agent submits a solution
- The evaluator runs the hidden test suite
- Passes if all tests pass

**Goal**: Improve pass@1 relative to baseline

### 2. tokens_per_task (Tokens per Task)

**Definition**: Total tokens consumed (input + output) to complete a task.

```
tokens_per_task = tokens_input + tokens_output
```

**Measurement**:
- Count via the model API
- Includes all calls until completion or timeout

**Goal**: Reduce tokens_per_task relative to baseline

### 3. wall_time (Execution Time)

**Definition**: Total time from task start to final submission.

```
wall_time = timestamp_fim - timestamp_inicio
```

**Measurement**:
- System timestamp
- Includes API wait time and tool execution

**Goal**: Reduce wall_time relative to baseline

---

## Secondary KPIs

### 4. tool_calls (Tool Calls)

**Definition**: Number of calls to external tools (read file, run command, etc.).

**Relevance**: Fewer calls generally indicates more efficient use of context.

### 5. file_reads (File Reads)

**Definition**: Number of files read during the task.

**Relevance**: The graph should reduce unnecessary reads.

### 6. rework_rate (Rework Rate)

**Definition**: Proportion of tasks that needed correction after the first submission.

```
rework_rate = (tasks with rework) / (total tasks)
```

### 7. cost_per_task (Cost per Task)

**Definition**: Monetary cost (USD) per completed task.

```
cost_per_task = (tokens_input * price_input) + (tokens_output * price_output)
```

### 8. guard_violations_blocked (Blocked Violations)

**Definition**: Number of security violation attempts detected and blocked.

**Relevance**: Demonstrates the effectiveness of protections.

### 9. guard_violations_unblocked (Unblocked Violations)

**Definition**: Number of violations that escaped protections.

**Goal**: Zero

---

## Review KPIs (Elephant-Goldfish)

The Elephant-Goldfish pattern introduces specific metrics to evaluate the quality of reviews with fresh context.

### 10. defects_found_pre_impl (Defects Found Pre-Implementation)

**Definition**: Number of defects found in the review phase, before implementation.

```
defects_found_pre_impl = count(findings with severity >= warning)
```

**Relevance**: Defects found before implementation are much cheaper to fix.

### 11. false_positive_rate (False Positive Rate)

**Definition**: Proportion of findings that were rejected in the resolution step.

```
false_positive_rate = (rejected findings) / (total findings)
```

**Measurement**:
- A human marks each finding as accepted, rejected, or deferred
- Rejected findings are false positives

**Goal**: < 20%

### 12. tokens_per_review (Tokens per Review)

**Definition**: Total tokens consumed to review a document.

```
tokens_per_review = sum(tokens of each goldfish reviewer)
```

**Relevance**: Goldfish reviewers should be efficient because they receive minimal context.

### 13. review_coverage (Review Coverage)

**Definition**: Proportion of the document effectively analyzed by reviewers.

```
review_coverage = (sections with findings or explicit OK) / (total sections)
```

### 14. reviewer_agreement (Agreement Between Reviewers)

**Definition**: Proportion of findings where multiple reviewers agree.

```
reviewer_agreement = (findings from 2+ reviewers) / (total unique findings)
```

**Relevance**: High agreement indicates more reliable findings.

---

## Comparison: Graph-First vs Grep-First

A specific benchmark variant compares search approaches:

### Graph-First (Experimental)

1. Agent receives the task
2. Queries `tb graph search` / `callers` / `deps` first
3. Identifies relevant files via the graph
4. Reads only necessary files
5. Implements the solution

### Grep-First (Baseline)

1. Agent receives the task
2. Uses grep/rg to search for patterns
3. Reads files that seem relevant
4. May read unnecessary files
5. Implements the solution

### Specific Metrics

| Metric | Description |
|--------|-------------|
| `files_read_relevant` | Files read that were actually relevant |
| `files_read_irrelevant` | Files read that were not relevant |
| `precision` | files_read_relevant / total_files_read |
| `recall` | files_read_relevant / total_files_actually_relevant |
| `tokens_on_irrelevant` | Tokens spent on irrelevant files |

---

## Comparison: Fresh-Context vs Same-Context Review

A specific benchmark variant compares review approaches:

### Fresh-Context Review (Goldfish)

1. Document is prepared by the elephant (planner)
2. N reviewers are spawned with **fresh** context
3. Each reviewer receives only: document + referenced material
4. Reviewers do not have access to the conversation history
5. Findings are aggregated and deduplicated

### Same-Context Review (Self-Review)

1. Document is prepared by the agent
2. The **same agent** reviews its own work
3. Agent has all accumulated context
4. May be "blind" to problems it assumed

### Specific Metrics

| Metric | Description |
|--------|-------------|
| `unique_defects_found` | Distinct defects found |
| `defects_missed` | Defects that existed but were not found |
| `blind_spot_rate` | Obvious problems missed due to context bias |
| `tokens_total` | Total tokens consumed in the review |
| `tokens_per_defect` | Efficiency: tokens / defect found |
| `time_to_review` | Total review time |

### Methodology

To measure objectively:

1. **Gold standard**: expert humans review the same document and create a list of known defects
2. **Fresh-context**: N goldfish review, measure recall against the gold standard
3. **Same-context**: the same agent reviews, measure recall against the gold standard
4. **Comparison**: which approach finds more defects from the gold standard?

### Hypothesis

We expect fresh-context to outperform same-context on:
- `unique_defects_found`: more defects found
- `blind_spot_rate`: fewer blind spots
- `tokens_per_defect`: more efficient (smaller context)

And same-context to be better on:
- `time_to_review`: faster (no subagent spawn)
- `tokens_total`: fewer total tokens (one agent only)

---

## Task Suite

### Categories

| Category | Description | Task Count |
|----------|-------------|------------|
| `implement` | Implement a new feature | 20 |
| `bugfix` | Fix a described bug | 15 |
| `refactor` | Refactor existing code | 10 |
| `test` | Write tests for existing code | 10 |
| `document` | Document code/API | 5 |

**Total**: 60 tasks

### Difficulty Levels

| Level | Characteristics |
|-------|-----------------|
| `easy` | Localized change, 1–2 files |
| `medium` | Multiple files, understand context |
| `hard` | Complex architecture, cross-cutting dependencies |

### Structure of a Task

```yaml
id: task-042
category: implement
difficulty: medium
title: "Implement authentication endpoint"
description: |
  Create endpoint POST /api/auth/login that:
  - Receives email and password
  - Validates credentials against the database
  - Returns a JWT token
  - Implements rate limiting

files_relevant:
  - src/api/handlers/auth.go
  - src/services/auth.go
  - src/database/users.go

hidden_tests:
  - tests/api/auth_test.go
  - tests/integration/auth_integration_test.go

expected_changes:
  - src/api/handlers/auth.go
  - src/api/routes.go

timeout: 10m
max_tokens: 20000
```

### Test Protection

Files in `hidden_tests` are:
- Not visible to the implementing agent
- Mounted read-only in the evaluator sandbox
- Hash-verified before and after each evaluation

---

## Running the Benchmark (Phase 0 + A/B Phase 2)

The benchmark harness is implemented and can be used with:
- **Fake agent**: deterministic, to test the harness in CI
- **External agent**: any shell command that accepts a prompt
- **A/B comparison**: two configs/feature flags on the same task set

### Basic Commands (Implemented)

```bash
# Run with the fake agent (for CI/tests)
tb bench run --fake -r 3

# Run with an external agent
tb bench run --agent "cursor-agent {prompt} --workdir {workdir}"

# Without --agent/--fake: uses bench.default_agent from .toolbelt/config.yaml
tb bench run -r 1

# Without -t/-f: uses bench.tasks_dir / fixtures_dir / output_dir from the YAML
# (precedence: CLI flag > config.yaml > default; -c/--config feeds Load)
tb -c /tmp/alt-config.yaml bench run -r 1

# A/B variant with label and flags (recorded in the JSONL)
tb bench run --fake --label graph-off --flag graph=off

# List available tasks
tb bench list

# Generate a report from existing results
tb bench report --input bench/results/2026-01-01-120000.jsonl

# Compare two JSONL files (success, time, tokens)
tb bench compare --before v1.jsonl --after v2.jsonl

# Run A/B from YAML configs
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml --fake

# A/B with an external agent (--agent or bench.default_agent)
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml \
                 --agent "cursor-agent {prompt} --workdir {workdir}"
```

### Fixture Tasks

| Task ID | Category | Difficulty | Description |
|---------|----------|------------|-------------|
| `bugfix-go` | bugfix | easy | Fix division by zero |
| `feature-small` | feature | easy | Implement Multiply and Divide |
| `refactor-simple` | refactor | medium | Extract UserRepository |
| `test-coverage` | test | easy | Increase test coverage |
| `doc-function` | doc | easy | Document functions |
| `localize-callers` | localization | medium | Map cross-package callers |
| `cross-file-rename` | refactor | medium | Rename a type across multiple files |
| `api-handler` | feature | medium | /echo endpoint + route registration |
| `mcp-smoke` | smoke | easy | MCP server smoke |

### A/B Configs

Files in `bench/ab/`:

| File | Label | Flags |
|------|-------|-------|
| `graph-off.yaml` | graph-off | `graph=off` |
| `graph-on.yaml` | graph-on | `graph=on` |
| `baseline.yaml` | baseline | `graph=off`, `review=off` |

### Results Structure

```json
{
  "benchmark_id": "bench-20260115-120000",
  "config": {
    "model": "claude-sonnet-4-20250514",
    "temperature": 0.3,
    "repetitions": 10,
    "condition": "toolbelt"
  },
  "summary": {
    "pass_at_1": 0.72,
    "tokens_per_task_mean": 8542,
    "tokens_per_task_std": 1234,
    "wall_time_mean_seconds": 145.3,
    "wall_time_std_seconds": 23.5,
    "tool_calls_mean": 12.4,
    "file_reads_mean": 5.2,
    "rework_rate": 0.18,
    "cost_per_task_usd": 0.043,
    "guard_violations_blocked": 7,
    "guard_violations_unblocked": 0
  },
  "tasks": [
    {
      "task_id": "task-042",
      "repetitions": [
        {
          "rep": 1,
          "passed": true,
          "tokens_input": 6234,
          "tokens_output": 2108,
          "wall_time_seconds": 132.5,
          "tool_calls": 11,
          "file_reads": 4,
          "rework_attempts": 0
        }
      ]
    }
  ]
}
```

---

## Results Report

### Format

The report should include:

1. **Executive summary**: percentage improvements on each KPI
2. **Comparison table**: baseline vs toolbelt by category
3. **Statistical analysis**: significance of differences
4. **Charts**: distributions, boxplots, trends
5. **Notable cases**: tasks where toolbelt had the greatest/least impact
6. **Violations**: details of any detected violation

### Summary Example

```markdown
## Executive Summary

| KPI | Baseline | Toolbelt | Δ |
|-----|----------|----------|---|
| pass@1 | 0.58 | 0.72 | +24% |
| tokens/task | 12,340 | 8,542 | -31% |
| wall_time | 203s | 145s | -29% |
| tool_calls | 18.2 | 12.4 | -32% |
| file_reads | 8.7 | 5.2 | -40% |
| rework_rate | 0.32 | 0.18 | -44% |
| cost/task | $0.062 | $0.043 | -31% |

Statistically significant differences (p < 0.05) across all KPIs.
```

---

## Open Questions

### Suite Size

- Are 60 tasks enough for statistical significance?
- Should we expand to 100+ tasks?
- Trade-off: more tasks = more benchmark time = more cost

### Different Models

- Should the benchmark be run on multiple models?
- How to normalize results across models with different prices?

### Multi-Agent Tasks

- How to measure the effectiveness of the agent hierarchy?
- Specific metrics for planner→supervisor→worker coordination

### Reproducibility

- How to ensure the task suite does not leak into training data?
- Should we rotate tasks periodically?

### Alternative Baseline

- Consider other baselines beyond "agent alone":
  - Agent with grep + no graph
  - Agent with another toolkit (competitive comparison)

### Review: Ideal Number of Goldfish

- How many goldfish reviewers produce the best cost-benefit?
- Diminishing returns: 3 vs 5 vs 10 reviewers?
- Specialization: generic reviewers vs specialized by area?

### Drift Detection

- How to measure `tb drift` effectiveness?
- Ground truth: manual diffs between docs and code
- Metrics: precision/recall of detected divergences

---

## Adversarial Verification KPIs

The `tb verify` system introduces specific metrics to evaluate the effectiveness of the generator→discriminator loop.

### 15. mean_similarity (Mean Similarity)

**Definition**: Mean similarity per screen in the visual ruler.

```
mean_similarity = sum(similarity_per_screen) / count(screens)
```

**Measurement**:
- Calculated by the discriminator
- Separated by mode (light/dark)
- Excludes masked areas

### 16. iterations_to_pass (Iterations to PASS)

**Definition**: Number of generator→discriminator cycles until PASS is reached.

```
iterations_to_pass = count(iterations until verdict == PASS)
```

**Relevance**: Fewer iterations = higher generator efficiency.

**Goal**: ≤ 2 iterations on average

### 17. cheating_attempts_detected (Cheating Attempts Detected)

**Definition**: Number of cheating attempts identified by the discriminator.

```
cheating_attempts = count(violations where type ∈ cheating_rules)
```

**Categories**:
- Reference edits
- Invalid masks
- Threshold changes
- Seed changes
- Others

**Goal**: 0 undetected attempts

### 18. human_escalations (Human Escalations)

**Definition**: Proportion of tasks that require human intervention after 3 failures.

```
human_escalations = (tasks with 3 FAILs) / (total tasks)
```

**Relevance**: A high rate indicates a process problem or inadequate spec.

**Goal**: < 10%

### 19. discriminator_false_fail_rate (Discriminator False-FAIL Rate)

**Definition**: Proportion of FAILs that were false positives (work was correct).

```
false_fail_rate = (FAILs overturned by a human) / (total FAILs)
```

**Measurement**:
- A human reviews escalated FAILs
- Marks as "false FAIL" if the implementation was correct
- Indicates an overly strict discriminator or ambiguous spec

**Goal**: < 5%

### 20. verification_overhead (Verification Overhead)

**Definition**: Extra time spent on verification vs implementation.

```
overhead = (tempo_total_verificacao) / (tempo_implementacao)
```

**Components**:
- Guard check time
- Build time
- Capture time
- Comparison time

---

## Comparison: Self-Verification vs Adversarial Verification

A benchmark variant compares verification by the implementer itself with adversarial verification.

### Self-Verification (Self-Verify)

1. Generator implements the solution
2. The **same generator** verifies its own work
3. Generator has all context and code
4. Metrics collected by the generator itself

### Adversarial Verification

1. Generator implements the solution
2. An **independent discriminator** verifies
3. Discriminator did not see the code being written
4. Clean build, fresh capture, independent measurement

### Specific Metrics

| Metric | Description |
|--------|-------------|
| `defects_caught_self` | Defects found by self-verification |
| `defects_caught_adversarial` | Defects found by adversarial verification |
| `defects_missed_self` | Defects that passed self-verification |
| `defects_missed_adversarial` | Defects that passed adversarial verification |
| `cheating_detected` | Cheating attempts detected (only adversarial detects) |
| `time_to_verify_self` | Self-verification time |
| `time_to_verify_adversarial` | Adversarial verification time |

### Methodology

To measure objectively:

1. **Gold standard**: expert humans identify all defects in implementations
2. **Self-verification**: generator verifies, measure recall against the gold standard
3. **Adversarial**: discriminator verifies, measure recall against the gold standard
4. **Comparison**: which approach finds more defects?

### Hypothesis

We expect adversarial verification to outperform self-verification on:
- `defects_caught`: more defects found
- `cheating_detected`: detects manipulation (self-verification does not)
- `defects_missed`: fewer defects escape

And self-verification to be better on:
- `time_to_verify`: faster (no discriminator spawn)
- `false_fail_rate`: fewer false positives (knows the context)

### UI Benchmark

Specific to the visual ruler:

```yaml
benchmark:
  name: adversarial-vs-self-visual
  
  tasks:
    - implement UI according to design
    - fix visual differences
    - add dark mode support
    
  conditions:
    self_verify:
      verifier: generator
      clean_build: false
      fresh_capture: false
      
    adversarial:
      verifier: discriminator
      clean_build: true
      fresh_capture: true
      
  metrics:
    - similarity_achieved
    - iterations_to_pass
    - defects_found
    - defects_missed
    - cheating_attempts
    - time_total
```

---

## Router/Workers KPIs

The Router/Workers pattern introduces specific metrics to evaluate delegation and parallelization effectiveness.

### 21. cache_read_tokens_pct (Cache-Read Token Percentage)

**Definition**: Proportion of input tokens that came from cache (reused context).

```
cache_read_tokens_pct = (cache_read_tokens) / (total_input_tokens) × 100
```

**Measurement**:
- Reported by the model API (when available)
- Estimated via context repetition across workers

**Relevance**: Workers with small, repetitive mandates benefit more from cache.

**Goal**: ≥ 70% cache-read in typical workloads

### 22. router_latency_p50 / router_latency_p95 (Router Latency)

**Definition**: Time the router takes to decompose a task and create tickets.

```
router_latency = timestamp_tickets_criados - timestamp_tarefa_recebida
```

**Measurement**:
- p50 (median): typical latency
- p95 (95th percentile): tail latency

**Goal**:
- p50 < 500ms
- p95 < 2s

### 23. tickets_parallel_max (Maximum Ticket Parallelism)

**Definition**: Highest number of tickets executing simultaneously.

```
tickets_parallel_max = max(active_workers at any instant)
```

**Relevance**: Indicates parallelization effectiveness. Low parallelism suggests excessive dependencies.

**Goal**: ≥ 3 tickets in parallel for complex tasks

### 24. merge_conflict_rate (Merge Conflict Rate)

**Definition**: Proportion of files with conflicts between workers.

```
merge_conflict_rate = (files with conflicts) / (files modified) × 100
```

**Relevance**: A high rate indicates poorly defined mandates or scope overlap.

**Goal**: < 5%

### 25. delegation_rate (Delegation Rate)

**Definition**: Proportion of tasks delegated vs executed by the router itself.

```
delegation_rate = (tasks delegated to workers) / (total tasks)
```

**Relevance**: The router should delegate, not execute. A low rate indicates a bottleneck.

**Goal**: ≥ 90%

### 26. worker_utilization (Worker Utilization)

**Definition**: Time workers spend working vs idle.

```
worker_utilization = (tempo_trabalhando) / (tempo_total) × 100
```

**Measurement**: Per worker and aggregate.

**Goal**: ≥ 80% during queue processing

### 27. ticket_retry_rate (Ticket Retry Rate)

**Definition**: Proportion of tickets that needed a retry after failure.

```
ticket_retry_rate = (tickets with retry) / (total tickets)
```

**Relevance**: A high rate indicates poorly specified tickets or unstable workers.

**Goal**: < 10%

### 28. direct_delivery_rate (Direct Delivery Rate)

**Definition**: Proportion of tickets delivered directly to subscribed workers vs via the board.

```
direct_delivery_rate = (tickets delivered directly) / (total tickets) × 100
```

**Relevance**: Direct delivery reduces assignment latency and improves context affinity.

**Goal**: ≥ 50% for workloads with dependencies

---

## Comparison: Router/Workers vs Monolithic

A benchmark variant compares the Router/Workers architecture with monolithic execution.

### Monolithic (Baseline)

1. A single agent receives a complex task
2. Executes all subtasks sequentially
3. Context grows throughout execution
4. No parallelization

### Router/Workers (Experimental)

1. Router decomposes the task into tickets
2. Workers execute in parallel with minimal context
3. Follow-ups can go directly to the owning worker (direct delivery)
4. Merge aggregates results
5. Effective parallelization

### Specific Metrics

| Metric | Description |
|--------|-------------|
| `total_input_tokens` | Total input tokens (monolithic accumulates) |
| `cache_read_savings` | Savings from cache reuse |
| `wall_time_total` | Total execution time |
| `wall_time_critical_path` | Critical-path time (parallelization) |
| `context_peak_tokens` | Peak context (monolithic grows) |
| `failures_isolated` | Failures contained in one worker |
| `failures_cascaded` | Failures that affected others (monolithic) |
| `direct_delivery_latency` | Direct delivery latency vs via board |

### Hypothesis

We expect Router/Workers to outperform monolithic on:
- `total_input_tokens`: ~40% less via minimal context + cache
- `wall_time_total`: ~60% less via parallelization
- `failures_isolated`: failures do not propagate

And monolithic to be better on:
- `router_latency`: no decomposition overhead
- `merge_conflict_rate`: no conflicts (single executor)
- Simple tasks: coordination overhead does not pay off

### Delegation Benchmark

```yaml
benchmark:
  name: router-workers-vs-monolithic
  
  tasks:
    - implement feature across 5+ files
    - refactor module with dependencies
    - add tests for existing code
    
  conditions:
    monolithic:
      executor: single_agent
      context: cumulative
      parallelism: 1
      
    router_workers:
      executor: router_with_workers
      context: minimal_per_worker
      parallelism: 3-5
      direct_delivery: true
      
  metrics:
    - total_input_tokens
    - cache_read_tokens_pct
    - wall_time_total
    - wall_time_critical_path
    - context_peak_tokens
    - merge_conflict_rate
    - direct_delivery_rate
    - pass_at_1
```

---

## Proposed Metrics and Experiments (2026-11)

Based on the primary-source review ([docs/pesquisa/achados.md](pesquisa/achados.md)), the following metrics and experiments are proposed for future validation.

### Proposed Metrics

#### Invariant Retention after Compaction

**Definition**: Proportion of anti-cheating rules and security constraints preserved after N compaction cycles.

```
invariant_retention = (invariants preserved after N compactions) / (original invariants)
```

**Source**: [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) (Salle et al., 2026)

**Proposed goal**: 100% (invariants live outside compaction, by design)

**Status**: pending proposal, see [ADR-010](adr/010-invariantes-fora-compactacao.md)

---

#### Visible vs Hidden Gap (Reward Hacking)

**Definition**: Difference between pass rate on visible tests and pass rate on held-out tests.

```
visible_hidden_gap = pass_rate_visible - pass_rate_hidden
```

**Source**: [SpecBench](https://arxiv.org/abs/2605.21384) (Zhao et al., 2026) — gap grows 28 pts for every 10× of code

**Proposed goal**: gap < 10% (a small difference indicates low reward hacking)

**Status**: pending proposal

---

#### Impossible Rate (Cheating Rate)

**Definition**: Proportion of impossible tasks where the agent reports success.

```
impossible_pass_rate = (impossible tasks that pass) / (total impossible tasks)
```

**Source**: [ImpossibleBench](https://arxiv.org/abs/2510.20270) (Zhong et al., 2025) — any pass is cheating

**Proposed goal**: 0%

**Status**: pending proposal (implement via `tb bench --impossible`)

---

#### Graph ROI

**Definition**: Proportion of tasks where using the graph resulted in token savings.

```
graph_roi_positive_pct = (tasks where the graph saved) / (tasks where the graph was used)
```

**Source**: [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) (Sepidband et al., 2026) and [Cost-Inefficient Behaviors](https://arxiv.org/abs/2609.30725) (Hu et al., 2026)

**Proposed goal**: ≥ 60%

**Status**: pending proposal, see [ADR-011](adr/011-grafo-sob-demanda.md)

---

### Proposed Experiments

#### A/B: Graph vs No Graph by Task Type

```yaml
experiment:
  name: graph-ab-by-task-type
  
  task_types:
    - localization_large_repo
    - bug_fix_localized
    - refactoring_cross_file
    - feature_implementation
    
  conditions:
    control: {graph: disabled}
    treatment: {graph: enabled}
    
  metrics:
    - tokens_total
    - wall_time
    - accuracy
    - cost_usd
    
  hypothesis: "The graph pays off more for localization and cross-file refactoring, less for localized bug fixes"
  
  source: "arxiv.org/abs/2611.07286, arxiv.org/abs/2609.30725"
```

**Status**: pending proposal

---

#### A/B: Hybrid Compaction vs LLM Only

```yaml
experiment:
  name: compaction-hybrid-vs-llm
  
  conditions:
    llm_only:
      method: "LLM summary of the full context"
      
    hybrid:
      method: "mechanical extraction + LLM summary"
      extraction: ["files_touched", "commands", "test_failures", "decisions"]
      
  metrics:
    - invariant_retention
    - information_recall  # measured by a details quiz
    - tokens_after_compaction
    
  hypothesis: "Hybrid retains more structured information with fewer tokens"
  
  source: "arxiv.org/abs/2610.31710, arxiv.org/abs/2610.31778"
```

**Status**: pending proposal

---

#### Benchmark: Impossible Tasks

```yaml
experiment:
  name: impossible-tasks
  
  generation:
    method: "generate variants where the test contradicts the spec"
    examples:
      - "spec asks for ascending order, test expects descending"
      - "spec caps at 100 items, test passes with 200"
      
  conditions:
    baseline: {hidden_tests: false}
    with_monitor: {hidden_tests: true, trajectory_monitor: true}
    
  metrics:
    - impossible_pass_rate  # target: 0%
    - cheating_detection_rate  # target: 100%
    
  source: "arxiv.org/abs/2510.20270"
```

**Status**: pending proposal (implement via `tb bench --impossible`)

---

#### Measurement: pass^k Beyond pass@1

```yaml
experiment:
  name: pass-k-reliability
  
  k_values: [1, 3, 5, 10]
  
  metrics:
    - pass_at_1
    - pass_at_3
    - pass_at_5
    - pass_at_10
    - reliability_score  # pass^k: reliability across k attempts
    
  hypothesis: "high pass@1 with low pass^k indicates a brittle solution"
  
  source: "arxiv.org/abs/2406.12045 (τ-bench)"
```

**Status**: pending proposal

---

#### Cost × Accuracy Frontier

```yaml
experiment:
  name: pareto-frontier
  
  conditions:
    - {model: small, tools: minimal}
    - {model: small, tools: full}
    - {model: large, tools: minimal}
    - {model: large, tools: full}
    - {model: large, graph: enabled}
    
  metrics:
    - accuracy
    - cost_usd
    - tokens_total
    
  analysis:
    - plot: "Pareto frontier of cost vs accuracy"
    - identify: "dominated configurations (never use)"
    
  source: "arxiv.org/abs/2407.01502 (AI Agents That Matter)"
```

**Status**: pending proposal

---

## Microbenchmark: `tb stack` (real binary)

Measure wall-clock of the **binary** (`go build -o tb ./cmd/tb`), not only in-process `testing.B`.
The stack already reuses diff/index/memory; parallelization (`--jobs`) covers
read-only steps (`guard`/`drift`/`review`). `verify` stays isolated.

### Methodology

- ≥ 7 runs; report the mean
- Compare: `tb stack …` vs a sequence of standalone commands
- Variants: `guard+drift+review` and `verify+guard+drift+review` (`--test-cmd true --max-iter 1`)
- Fixtures: the toolbelt repository and a fixture with hundreds of changed files
- Controls: `--jobs 1` (internal serial) vs default (CPU count)

### Observed profile (Prepare)

| Phase | Small fixture | Large fixture (~500 files) |
|------|-----------------|------------------------------|
| Startup (`tb version`) | ~3 ms | ~3 ms |
| Unified `git diff` | ~1.4 ms | ~32 ms |
| `git diff --name-status` + `--name-only` | ~2.5 ms | ~5.5 ms |
| 3× git in parallel | ~1.6 ms | ~31 ms (dominated by unified) |

The in-process gain (parallel stack vs `--jobs 1` ~2.6×; vs 3× standalone `Run` ~7×)
overestimates the real-binary gain, because startup (~3 ms) and the unified
`git diff` dominate.

### Results (real binary, mean of 7 runs)

| Scenario | Before (serial stack) | After (parallel) | Standalone sequence | Speedup stack÷seq |
|---------|----------------------|-------------------|------------------|-------------------|
| toolbelt `guard+drift+review` | 10 ms | 5 ms | 15 ms | 3.00× |
| toolbelt `verify+…` | 10 ms | 5 ms | 19 ms | 3.80× |
| large fixture `guard+drift+review` | 60 ms | 36 ms | 80 ms | 2.22× |
| large fixture `verify+…` | 74 ms | 44 ms | 93 ms | 2.11× |
| large + `--jobs 1` | — | 47 ms | 84 ms | 1.79× (Prepare parallel only) |

On the large fixture, parallelizing steps + git in Prepare cut ~40% of stack
wall-clock (60→36 ms). `--jobs 1` keeps Prepare parallel and serves as the serial
control for steps.

### How to run

```bash
go build -o /tmp/tb ./cmd/tb
/tmp/tb stack guard drift review --ticket rd.md
# vs sequence:
/tmp/tb guard check --json
/tmp/tb drift --ticket rd.md --json
/tmp/tb review --ticket rd.md --json
```

**Status**: measured during `--jobs` development (shipped in v0.0.3, see CHANGELOG)
