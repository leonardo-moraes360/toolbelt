🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/010-invariantes-fora-compactacao.md)

# ADR-010: Invariants Pinned Outside Compaction

## Status

proposed

## Context

Recent studies on LLM context compaction reveal a critical problem for coding agents:

- [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) (Salle et al., 2026, preprint) shows that the 1st summary already loses about 1/3 of anti-cheat rules
- "Don't do" rules (prohibitions) drop ~6× more than other information during compaction
- [Compact-Bench](https://arxiv.org/abs/2610.31710) (Sun et al., 2026, preprint) shows that LLM summaries retain at most 16% of recurring details, while mechanical extraction retains 79.1%

For toolbelt, this means critical mandate rules (forbidden paths, security limits, scope restrictions) can be lost during context compaction, opening gaps for reward hacking and security violations.

## Decision

We adopt a **pinned invariants** strategy combined with **hybrid compaction**:

### 1. Invariants File

A separate file (`invariants.yaml` or `.toolbelt/invariants.md`) contains rules that **never** go through the compaction process:

```yaml
# .toolbelt/invariants.yaml
version: 1

anti_cheat:
  - "NUNCA modifique arquivos em tests/hidden/"
  - "NUNCA altere thresholds em .toolbelt/thresholds.yaml"
  - "NUNCA edite código de pontuação em benchmarks/"
  - "NUNCA sobrescreva __eq__ para falsificar comparações"
  - "NUNCA guarde estado entre chamadas de teste"

mandate_constraints:
  - "Scope limitado aos arquivos listados no mandate"
  - "Permissões: apenas {permissions}"
  - "Timeout: {timeout}"
  - "Recursos: CPU {cpu_limit}, memória {mem_limit}"

security:
  - "Dados privados + conteúdo não confiável + saída externa = proibido"
  - "Secrets nunca aparecem em logs ou prompts"
  - "Rede negada exceto allowlist explícita"
```

### 2. Hybrid Compaction Process

```
┌─────────────────────────────────────────────────────────────────┐
│                    BEFORE COMPACTION                            │
│  1. Mechanical extraction (deterministic):                      │
│     - Files touched                                             │
│     - Commands executed                                         │
│     - Test failures (exit codes, error messages)                │
│     - Decisions made (with timestamp)                           │
│                                                                 │
│  2. Extracted state saved in structured format                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    LLM COMPACTION                               │
│  - Summary of reasoning and progress                            │
│  - Narrative context                                            │
│  - Does NOT include invariants or mechanical state              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    REASSEMBLED CONTEXT                          │
│  1. Invariants (intact, from original file)                     │
│  2. Mechanical state (intact, from extraction)                  │
│  3. LLM summary (compacted)                                     │
└─────────────────────────────────────────────────────────────────┘
```

### 3. Retention KPI

Internal benchmark that measures invariant retention:

```
retenção_invariantes = (invariantes preservados após N compactações) / (invariantes originais)
```

**Target**: 100% retention (by design, since invariants do not go through compaction).

## Consequences

### Positive

- **Guaranteed security**: anti-cheat rules and security constraints are never lost
- **Auditability**: invariants are verifiable before and after each turn
- **Compatibility**: mechanical extraction works with any compaction model
- **Token savings**: structured information is more compact than prose

### Negative

- **Implementation overhead**: requires a parser and structured format for invariants
- **Rigidity**: changes to invariants require explicit file updates
- **Potential duplication**: some information may appear in both the summary and the invariants

## Alternatives Considered

### Prompt Engineering to Preserve Rules

- **Description**: instruct the compaction model to preserve certain information
- **Why not**: research shows this is unreliable; "don't do" rules drop ~6× more even with explicit instructions

### Mechanical Compaction Only

- **Description**: eliminate LLM compaction and use only deterministic extraction
- **Why not**: loses narrative context and reasoning that can be useful

### Invariant Hash

- **Description**: hash of the invariants file verified every turn
- **Why not**: does not prevent loss during compaction, only detects it afterward
- **Note**: can be used **additionally** as a check

## References

- [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) — Salle et al., 2026 (preprint)
- [Compact-Bench](https://arxiv.org/abs/2610.31710) — Sun et al., 2026 (preprint)
- [Effective context engineering for AI agents](https://www.anthropic.com/research/effective-context-engineering-for-ai-agents) — Anthropic, 2025
