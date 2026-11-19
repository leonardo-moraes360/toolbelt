# Standard: implementation-readiness

A plan is ready when an engineer who did not write it can execute it in
order, verify each step, and know when it is done.

Criteria:
- Files: a table of every file the work creates, modifies or deletes, with the
  action and the requirement it serves. Paths are exact and relative to the
  repository root.
- Coverage: every requirement maps to at least one planned change and one
  verification; no planned change lacks a requirement.
- Order: steps build on each other; each step leaves the tree compiling and the
  tests passing, or the plan says why not.
- Verification: tests are named with what they assert, including failure
  cases; manual checks say what to run and what to expect.
- Existing code: names, signatures and behavior the plan relies on match the
  cited sources.
- Operations: configuration, migration, documentation (including translated
  mirrors) and release steps are listed when the change needs them.
- Risks: steps that can break users or data say how to detect and roll back.
