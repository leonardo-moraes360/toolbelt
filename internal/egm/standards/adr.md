# Standard: adr

An architecture decision record captures one decision, the forces behind it,
and what it costs, so a later reader can tell whether it still applies.

Required headings (either language satisfies each line; the whole heading line
must match):

| Section | English | Portuguese |
|---------|---------|------------|
| Status | `## Status` | `## Status` |
| Context | `## Context` | `## Contexto` |
| Decision | `## Decision` | `## Decisão` |
| Consequences | `## Consequences` | `## Consequências` |
| Alternatives | `## Alternatives Considered` | `## Alternativas Consideradas` |

Criteria:
- Status: one of proposed, accepted, superseded or deprecated, with a link to
  the superseding record when applicable.
- Context: the forces and constraints that make the decision necessary, stated
  without arguing for the outcome.
- Decision: one decision, stated in the active voice, precise enough that a
  reviewer can tell whether code follows it.
- Consequences: both the benefits and the costs, including what becomes harder,
  what must be maintained, and what is now forbidden.
- Alternatives considered: each real option, with the reason it lost; a
  "do nothing" option when it was viable.
- The record does not mix several unrelated decisions.
