🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/008-verificacao-adversarial.md)

# ADR-008: Adversarial Verification (Generator vs Discriminator)

## Status

accepted

## Context

Autonomous agents frequently "drift" from the original objective, especially in visual tasks where the result should match a design reference. Even with reference images or verifiable HTML, implementations tend to diverge subtly.

Observed problems:
- Self-verification is biased: the agent that implemented tends to "not see" its own mistakes
- Success metrics can be manipulated by the implementer
- Tests can be modified to pass artificially
- References can be edited to match the wrong implementation

The solution is an **adversarial loop** inspired by GANs (Generative Adversarial Networks), where an independent discriminator tries to **fail** the generator's work.

## Decision

Adopt **adversarial verification** as a first-class primitive in toolbelt, generalizing the concept to multiple domains beyond UI.

### GAN-like Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         ADVERSARIAL LOOP                        │
└─────────────────────────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         GENERATOR                               │
│  (implementer)                                                  │
│                                                                 │
│  - Receives task and references                                 │
│  - Implements solution                                          │
│  - Runs local verification (does NOT count as proof)            │
│  - Submits to discriminator                                     │
└─────────────────────────────────────────────────────────────────┘
              │
              │ submission
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       DISCRIMINATOR                             │
│  (independent verifier, never saw the code)                     │
│                                                                 │
│  1. Guard check: verifies changes in protected paths            │
│  2. Clean build: rebuilds everything from scratch               │
│  3. Regenerates references (if applicable)                      │
│  4. Captures fresh current state                                │
│  5. Runs measurement with configured ruler                      │
│  6. Checks for cheating (ref edits, invalid masks, etc)         │
│  7. Emits PASS or FAIL with report                              │
└─────────────────────────────────────────────────────────────────┘
              │
              │ result
              ▼
     ┌────────┴────────┐
     │                 │
   PASS              FAIL
     │                 │
     ▼                 ▼
  Done         ┌──────────────┐
               │ Iteration < 3? │
               └──────┬───────┘
                      │
            ┌─────────┴─────────┐
            │                   │
           Yes                 No
            │                   │
            ▼                   ▼
    Back to generator    Escalate to human
```

### Fundamental Principles

1. **Only the discriminator's measurement counts as proof**
   - Generator measurements are informative, not evidentiary
   - This eliminates self-bias and manipulation

2. **The discriminator tries to FAIL**
   - Adversarial mindset: looks for gaps, not confirmation
   - Checks for cheating beyond the main metric

3. **Maximum 3 iterations, then escalation**
   - Avoids infinite loops
   - Human resolves genuine stalemates

4. **Complete isolation**
   - Discriminator has its own sandbox, port, and DB
   - Cannot push, switch branch, or stash
   - Does not fix anything, only reports

### Relation to Existing Principles

This pattern reinforces and generalizes concepts already present in toolbelt:

| Principle | How Adversarial Verification Reinforces It |
|-----------|---------------------------------------------|
| Implementer ≠ evaluator (ADR-005) | Discriminator is structurally separate from the generator |
| Fresh context for reviewers | Discriminator starts from scratch, without generator context |
| Test immutability | Guard check detects reference/comparator edits |
| Reward hacking detection | Cheating is checked explicitly with defined rules |

### Pluggable Rulers

The verification system is generic, with domain-specific **rulers**:

| Ruler | Domain | What It Measures |
|-------|--------|------------------|
| `visual` | UI/screens | Pixel-to-pixel similarity + WCAG contrast |
| `tests` | Code | Pass rate on hidden tests |
| `review` | Documents | Coverage and quality of fresh-context review |

Each ruler defines:
- Main metric and pass threshold
- Specific cheating rules
- Report format

## Consequences

### Positive

- **Elimination of self-bias**: independent measurement is more reliable
- **Cheat detection**: explicit rules prevent manipulation
- **Generalization**: same pattern for UI, code, and documents
- **Scalability**: automatic loop with controlled escalation
- **Auditability**: detailed reports per iteration

### Negative

- **Computational overhead**: each iteration requires a full build/capture
- **Setup complexity**: discriminator needs an isolated environment
- **Latency**: multiple iterations increase total time
- **False negatives**: an overly strict discriminator may reject valid work

## Alternatives Considered

### Self-Verification (Self-Verify)

**Why considered**:
- Simpler, no separate agent
- Lower latency

**Why not chosen**:
- Self-bias: same context = same blind spots
- Easy manipulation: generator controls the metrics
- History shows significant drift without external verification

### Human Verification

**Why considered**:
- Higher-quality judgment
- Can evaluate nuances

**Why not chosen as the only method**:
- Does not scale
- Subjective and inconsistent
- Human is reserved for escalation (3rd failure)

### Verification by Another Persistent Agent

**Why considered**:
- Dedicated verifier agent accumulates experience

**Why not chosen**:
- Accumulated context can create biases
- Discriminator must be fresh by design

## Open Questions

### Optimal Threshold per Domain

- Visual: is 95% adequate? Does a 5% margin cover anti-aliasing?
- Tests: 100% or allow known flaky tests?
- How to calibrate thresholds for different projects?

### Iteration Cost

- Are 3 max iterations enough?
- How to optimize feedback to reduce iterations?
- Trade-off between discriminator rigor and false-FAIL rate

### Reference Stability

- How to detect unstable references (e.g. animations, dynamic data)?
- Policy for boards that change between runs
- Versioning references alongside code

### CI Integration

- Should the discriminator run in CI?
- How to parallelize verification of multiple screens/tests?
- Caching builds to speed up iterations

## References

- [ADR-005: Anti-Cheat Strategy](005-anti-trapaca.md) — implementer ≠ evaluator principle
- [Generative Adversarial Networks](https://arxiv.org/abs/1406.2661) — conceptual inspiration
- [WCAG 2.1 Contrast Requirements](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html)
