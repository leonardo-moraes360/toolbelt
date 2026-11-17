[🇧🇷 Português](../../adr/005-anti-trapaca.md) | 🇺🇸 English

# ADR-005: Anti-Cheat Strategy

## Status

accepted

## Context

Language models optimize for apparent rewards, not necessarily for the real objective. In coding tasks, this can show up as:

1. **Deleting failing tests**: the test no longer fails, but the bug remains
2. **Hardcoding outputs**: returns expected values without implementing logic
3. **Skipping tests**: `t.Skip("TODO")` to pass artificially
4. **Modifying benchmarks**: changing success criteria
5. **Excessive retry**: trying until it passes by chance

We need protections that:
- Detect and block dishonest behavior
- Maintain benchmark integrity
- Create a complete audit trail
- Separate implementation from evaluation

## Decision

A five-pillar strategy:

### 1. Implementer/Evaluator Separation

```
┌─────────────────────────────────────────────────────────────────┐
│                         Task                                    │
└─────────────────────────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┐
    │                   │
    ▼                   ▼
┌─────────────┐   ┌─────────────┐
│ Implementor │   │  Evaluator  │
│   Agent     │   │   Agent     │
├─────────────┤   ├─────────────┤
│ Can:        │   │ Can:        │
│ - read code │   │ - read code │
│ - write     │   │ - run       │
│   code      │   │   tests     │
│ - run       │   │             │
│   visible   │   │ Cannot:     │
│   tests     │   │ - write     │
│             │   │ - see impl  │
│ Cannot:     │   │   history   │
│ - see/edit  │   │             │
│   hidden    │   │             │
│   tests     │   │             │
└─────────────┘   └─────────────┘
```

The evaluator:
- Runs in a separate sandbox
- Has no access to the implementer's history
- Only runs tests and reports results

### 2. Immutable Test Files

```bash
# Before the task
tb guard snapshot --paths "tests/**" "benchmarks/**"

# During execution
# tests/** mounted as read-only in the sandbox

# After the task
tb guard verify --snapshot-id $SNAP_ID
# If hash differs → VIOLATION
```

Implementation:
- Read-only bind-mount via bubblewrap
- SHA-256 hash of each file
- Verification before and after
- Log of any write attempt

### 3. Append-Only Audit Log

```
.tb/audit.log (append-only, even root cannot edit)

2026-01-15T10:30:00.123Z | worker-001 | file:read    | src/parser.go | OK
2026-01-15T10:30:01.456Z | worker-001 | file:write   | src/parser.go | OK | hash:abc→def
2026-01-15T10:30:02.789Z | worker-001 | file:write   | tests/test.go | BLOCKED | read-only
2026-01-15T10:30:03.012Z | worker-001 | run:command  | go test       | OK | exit:0
```

Characteristics:
- Append-only enforced via syscall filtering
- Chained hash (each entry includes the previous hash)
- Rotation by size/time
- Exportable for external analysis

### 4. Capability Tokens for Destructive Actions

Dangerous actions require explicit approval:

```go
type CapabilityToken struct {
    ID        string
    Action    string    // "delete:file", "modify:test"
    Resource  string    // path or pattern
    GrantedBy string    // human or authorized supervisor
    GrantedAt time.Time
    ExpiresAt time.Time
    OneTime   bool      // invalidates after use
    Signature string    // cryptographic proof
}
```

Flow:
1. Agent attempts a destructive action
2. System blocks and creates a pending request
3. Supervisor or human reviews
4. If approved, a token is issued
5. Token has short validity (minutes)
6. Action executes by presenting the token

### 5. Suspicious Pattern Detection

```go
type RewardHackingDetector struct {
    patterns []Pattern
}

var defaultPatterns = []Pattern{
    // Test deletion
    {
        Name:    "test_deletion",
        Trigger: "file:delete",
        Match:   "*_test.go",
        Action:  "block",
    },
    // Test skip
    {
        Name:    "test_skip",
        Trigger: "file:write",
        Match:   "*_test.go",
        Search:  `t\.Skip\(`,
        Action:  "flag_for_review",
    },
    // Hardcoded output
    {
        Name:    "hardcoded_output",
        Trigger: "file:write",
        Match:   "*.go",
        Search:  `return "expected_value"`,
        Action:  "flag_for_review",
    },
    // Artificial coverage
    {
        Name:    "artificial_coverage",
        Trigger: "test:run",
        Detect:  "test without assertions",
        Action:  "flag_for_review",
    },
}
```

## Consequences

### Positive

- **Integrity**: benchmarks cannot be manipulated
- **Traceability**: every action is logged
- **Review**: suspicious patterns are flagged
- **Trust**: results are verifiable

### Negative

- **Overhead**: checks add latency
- **False positives**: detection can be overly strict
- **Complexity**: multiple systems to maintain
- **UX**: approvals can frustrate the flow

## Alternatives Considered

### Trust the Model

**Why considered**:
- Simplicity
- Modern models are "well-behaved"

**Why not chosen**:
- Models optimize for apparent reward
- Edge cases of reward hacking are documented
- Security requires defense in depth

### Human Review of Everything

**Why considered**:
- Maximum security
- Human as final judge

**Why not chosen**:
- Does not scale
- Frustrates automation
- Humans also err

### Post-Hoc Mutation Testing

**Why considered**:
- Detects weak tests
- No overhead during execution

**Why not chosen as the only measure**:
- Detects after the fact, does not prevent
- Add as an extra layer (Phase 2)

## Detection Test Cases

```yaml
# Scenarios the system must detect/block:

- scenario: delete_failing_test
  action: rm tests/auth_test.go
  expected: blocked
  
- scenario: skip_test
  action: add "t.Skip()" to test
  expected: flagged_for_review
  
- scenario: hardcode_output
  action: return literal "expected_value"
  expected: flagged_for_review
  
- scenario: modify_benchmark_threshold
  action: change assert from "< 100ms" to "< 10s"
  expected: blocked (benchmark is read-only)
  
- scenario: empty_test
  action: write test with no assertions
  expected: flagged_for_review
```

## References

- [Reward Hacking in RL](https://arxiv.org/abs/1706.03741)
- [Specification Gaming Examples](https://docs.google.com/spreadsheets/d/e/2PACX-1vRPiprOaC3HsCf5Tuum8bRfzYUiKLRqJmbOoC-32JorNdfyTiRRsR7Ea5eWtvsWzuxo8bjOxCG84dAg/pubhtml)
- [AI Safety via Debate](https://arxiv.org/abs/1805.00899)
