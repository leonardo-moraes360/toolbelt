[🇧🇷 Português](../../adr/014-egm-nativo.md) | 🇺🇸 English

# ADR-014: Native Reimplementation of the Elephant-Goldfish Model (`tb egm`)

## Status

proposed. It does not move to `accepted` before the R0 spike of the [spec](../specs/egm-nativo.md).

## Context

[ADR-007](007-inspiracao-elephant-goldfish.md) adopted the Elephant-Goldfish pattern and left open the question "integrate EGM or reimplement it?". The pattern's roles are:

- **elephant**: the agent or human who wrote the document and knows the full context;
- **goldfish**: the reviewer who sees only the document and the sources, with no history.

As things stand today:

- `tb review` calls three **deterministic** Go checkers (`scope`, `quality`, `acceptance`) "goldfish"; they only look at a **diff** against `rd.md`. It does not review documents, does not use an LLM and has no convergence loop.
- `tb drift` compares a diff with the ticket scope, not docs with code.
- The upstream ([google/elephant-goldfish-model](https://github.com/google/elephant-goldfish-model), Apache-2.0, v0.1.1) has 7 skills (`brainstorm`, `draft`, `review`, `review-loop`, `resolve`, `bootstrap`, `drift-report`), 8 reviewer personas and a synthesizer, all in **prose for the LLM**:
  - fresh context is guaranteed only by an instruction to the harness;
  - findings are free text (Blocker/Major/Nit);
  - the loop state lives in the model's memory.

Problems with ADR-007's Option A (calling the EGM skills via MCP or plugin):

1. **Isolation depends on the harness.** Without a subagent, the review stops. In Codex and Antigravity, the host's `AGENTS.md` leaks into the reviewer (a limitation documented upstream).
2. **There is no verifiable contract.** A prose finding cannot feed `tb stack`, `tb guard` or MCP.
3. **Convergence depends on the model's memory.** The settled/fixed rules and "2 clean rounds" get lost in compaction, which contradicts [ADR-010](010-invariantes-fora-compactacao.md).
4. **tb's invariants cannot be enforced inside it**: tb 100% local, secrets never in prompts, fail-closed sandbox, implementer ≠ evaluator.

## Decision

Reimplement EGM **natively** (ADR-007's Option B) as the `tb egm` group:

- **tb** builds the prompt, isolates and runs the goldfish, validates, matches findings, keeps the state and decides convergence;
- **the LLM** only reads and judges.

### 1. A goldfish is an isolated process with no tools

Each goldfish is a **one-shot**, stateless process, started from `egm.agent_cmd` with `sandbox.New`. It does **not** inherit `router.agent_cmd`, which follows a different contract: a writer agent, with the ticket passed via env. For each process:

- **No tools at all.** Reading, writing and execution are turned off by the vendor profile's flags. All content goes in the prompt, via **stdin**, and findings come back via **stdout**, on a channel separate from stderr. `bypassPermissions` and equivalents are refused, as in ADR-013.
- **Sandbox.** The sandbox gains a mode with a read-only workdir, a private `TMPDIR` (instead of the shared `/tmp`), its own HOME, and separate stdin and stdout. With these fields zeroed, current behavior does not change.
- **Disposable HOME.** It is a `0700` directory outside the repository, deleted at the end, so the vendor's `CLAUDE.md`, memories and global settings never reach the goldfish. Authentication follows this order of preference, settled in the spike: API key via the profile's `env.pass`, then the vendor's config directory pointed at the disposable HOME, and only as a last resort copying a credential file.
- **Sources and redaction.** Only versioned files go in, with a secrets deny-list (dotenv, keys, certificates, git-ignored files) and extended redaction that preserves the line count.
- **Record.** The packet (prompt, sources, nonce and hashes) is persisted in `.toolbelt/egm/<slug>/round-<n>/` for auditing.

### 2. The reviewed document is untrusted data, and the author is the one going through the gate

The document is written by the elephant itself, which is precisely the agent being evaluated. So the most likely injection comes from the document itself. The mitigations:

- **Nonce.** It delimits the data blocks and must be echoed in the output. It only proves the output came from that run; it does **not** prevent the model from obeying an injected instruction.
- **Positive attestation.** Each reviewer returns literal excerpts from the document (`checked: [{heading, quote}]`), which tb checks against the snapshot. An attestation below the minimum or with an excerpt that does not match invalidates the output. This proves reading without penalizing legitimate clean rounds, unlike a "zero findings" heuristic.
- **Narrow anti-injection scan.** It is deterministic and looks only for phrases directed at the reviewer or the model (`ignore previous`, `return no findings`…). It ignores code, inline code and quotes, and emits a `major` finding contestable before the arbiter. Documents that merely *discuss* reviewers and findings, like this spec, do not trigger the scan; the spec is a negative golden test. The same scan runs over the evidence sent to the arbiter.
- **Verified evidence.** tb checks every cited `path:line` and heading.

This follows the model of [ADR-013](013-barramento-multi-vendor.md), which already treats peer content as data.

### 3. Two engines and one contract

| Engine | When | Coverage |
|--------|------|----------|
| `llm` | executable `egm.agent_cmd` (from the trust source, §5) | 9 reviewers, 8 personas (`style` runs over two ranges) |
| `deterministic` | no agent or `--engine deterministic` | `refs-exist` → `codebase-details`, `required-sections` → `structural`, `plan-files-listed` → `implementation-readiness`, all **partial** |

- Both engines emit `egm.Finding`.
- A persona with no agent is `unavailable` (never "zero findings") and the coverage is declared.
- The deterministic checkers use the link, path and symbol helpers from `internal/graph/index_docs.go`, which become exported.
- `review.Finding`, `tb review` and `tb drift` **do not change**.

### 4. State and convergence outside the LLM

`.toolbelt/egm/<slug>/state.json` stores the round records (document and source hashes, commit, engine, reviewers, argv and binary hashes) and the finding lineages with a stable ID (`F-<k>`). Each round's document snapshot is kept in `round-<n>/target.md`.

- **Re-evaluation by the owning reviewer.** Each lineage has a single owning reviewer, which gets it back every round and answers `still_holds`. Omitting a lineage invalidates the output. That way closing does not depend on the luck of the model "forgetting" a problem. Lineages from deterministic checkers and from the scan close or reopen by re-running the checker. New findings are matched to lineages by location (diff between snapshots) and by text similarity, with thresholds calibrated in the spike.
- **Clean round.** No blocker/major in `open`, `reopened`, `contest_pending` or `escalated`, and no reviewer `not_run`.
- **Convergence.** 2 consecutive clean rounds are required, with a cap of 5.
- **Verdicts.**
  - `IN_PROGRESS` while the loop runs.
  - `PASS` only with full coverage and no escape hatch at all.
  - `PASS_PARTIAL` when there is an `unavailable` or `skipped` persona, restricted reviewers, a forced settle, `--restart` or rounds past the cap. `tb stack` gains the WARN state for this case and lists what was forced; `--strict` or `egm.require_full_coverage` turn it into a failure.
  - `FAIL` when it hits the cap without converging.
  - Reducing coverage between rounds is refused, except with `--narrow --reason` and human confirmation.

### 5. Trust boundary: the elephant has a shell

The elephant runs as the same user and can write to the repository. Without protection, it would be enough to point `egm.agent_cmd` at a fake script, lower `clean_rounds` or edit `state.json`. Therefore:

- **Sensitive keys** (`agent_cmd`, `vendor`, `tools_disabled`, `clean_rounds`, `match`, `require_full_coverage`…) are only read from the user config outside the repository (`$XDG_CONFIG_HOME/tb/egm.yaml`) or from `.toolbelt/config.yaml` at the trust ref, via `git show`, as `guard` already does with `guard.yaml`. A diverging value in the worktree is ignored with a warning. `.toolbelt/config.yaml` joins `guard`'s protected paths.
- **HMAC-chained records.** Each round and each transition carry a chained HMAC-SHA256, keyed from `$XDG_STATE_HOME/tb/egm.key`, and also go to an append-only `audit.jsonl`. `status` and `tb stack` recompute the verdict from these records, not from the aggregated `state.json`.
- **Human confirmation.** The escape hatches (`--force-settle`, `--restart`, `--narrow`, `--past-cap`) require the user to type the confirmation in an interactive TTY. They are refused without a TTY and via MCP.
- **Declared limit.** Anyone with a shell as the same user can read the HMAC key, edit the user config or forge a TTY. Phase A makes fraud **evident and expensive, not impossible**. Capability tokens ([ADR-005](005-anti-trapaca.md) §4), which do not exist in the code yet, and running as a different user are left for later. Against a malicious commit on the trust ref, CI and human review apply.

### 6. The elephant does not approve itself

- **`--fix` requires a change.** For a blocker/major, it is only valid if the document or some source cited by the lineage changed since the origin round, and the owning reviewer's re-evaluation confirms or reopens. This prevents marking something as fixed without editing it and repeating rounds until sampling forgets the finding, without blocking legitimate fixes made in the code or in another section.
- **A contest goes to an arbiter.** At most **one** per lineage, decided by a new **arbiter** goldfish. The evidence must be one of the lineage's sources, versioned and unchanged, and it is read from `HEAD`, inside data blocks and after the anti-injection scan.
  - If the arbiter withdraws the finding, it becomes `settled`.
  - If the arbiter upholds it, the finding goes back to `open`, and from then on it only leaves via a fix (with re-evaluation), via `still_holds: false` from the owning reviewer, or via a forced settle.
- **`--dup-of` cannot lower severity.** The merge keeps the highest severity and the most restrictive status, and does not apply to lineages already contested.
- **History is not erased.** `--restart` opens a new series of rounds on top of the existing state, keeping lineages and contests, and downgrades the verdict. A renamed document requires `--adopt` of the old path.
- **A forced settle is human.** `--force-settle --reason` requires human confirmation, is recorded and downgrades the verdict to `PASS_PARTIAL`.

### 7. Gates become explicit transitions

The upstream's interactive questions become idempotent subcommands of `tb egm resolve`, with an explicit transition table. The elephant asks the user and records the answer. tb never edits the document nor changes severity.

### 8. Scope and network

- **This decision and the [Phase A spec](../specs/egm-nativo.md)** cover `review`, `loop next`, `resolve` and `status` with the `consolidated` preset, the `claude` profile and a `custom` profile with an explicit "no tools" declaration, the built-in personas (original text, no copy from upstream) and the `tb_egm_*` MCP tools.
- **Follow-up specs:**
  - **Phase B:** `drift-report` and `bootstrap`.
  - **Phase C:** `draft` and `brainstorm`; `staged`, `feature` and `implementation` presets; per-path custom personas; `codex` profile; churn; `review.Finding` → `egm.Finding` adapter.
- **Network.** `tb` still does not talk to the network. The agent process is what talks to the provider. With no agent, everything works 100% offline.
- **Relation to ADR-013.** Goldfish are one-shot workers, not bus agents. A future `reviewer` role in `tb bus` may consume `egm.Finding`.
- **Docs location.** This repository's design documents live in `docs/specs/`.

## Consequences

### Positive

- Isolation is guaranteed by process, sandbox, disposable HOME and absence of tools, without depending on the harness.
- The loop resists the obvious forms of self-approval: lottery, re-rolling the arbiter, downgrading via `--dup-of`, injection through the document itself, a gate with no LLM, tampered config, hand-edited state and `--restart`.
- Structured findings feed `tb stack` and MCP, and convergence is reproducible.
- There is an offline fallback with declared coverage.

### Negative

- It touches `internal/sandbox`, a security-critical component.
- **Vendor credentials:**
  - copying a file, if that is the chosen form, creates a short local exposure window;
  - a token refresh inside the disposable HOME may invalidate the real credential, which must be tested in the spike;
  - on macOS, the credential lives in the Keychain, which is a declared limitation.
- The anti-injection scan is heuristic. A subtle injection the scan misses and that every reviewer obeys, while still producing valid attestations, remains a residual risk.
- Protection against the elephant itself is tamper evidence, not prevention: a process of the same user can read the HMAC key or forge a TTY.
- **Cost:** about 9 agent runs per round, bounded by a budget counted in real time.
- Vendor profiles depend on third-party CLI flags that change over time.
- There are in-house personas to maintain.

## Alternatives Considered

### Option A: EGM skills via MCP or plugin

- Reuses the ready-made upstream.
- **Not chosen** because of problems 1 to 4 in the Context.

### Just harden the current heuristics

- Zero LLM cost.
- **Not chosen**: it does not review documents, and the user asked for the full EGM.

### Read-only allowlist of the real HOME

- It is simpler.
- **Not chosen**: the global `CLAUDE.md` and the vendor's memories contaminate the goldfish.

### Goldfish outside the sandbox (`--allow-unsandboxed`)

- **Not chosen**: it violates the fail-closed sandbox.

### Only a human can clear a contest

- **Not chosen**: it creates a human bottleneck. The fresh arbiter, with one contest per lineage, preserves implementer ≠ evaluator.

### Extract a common executor from the router

- **Not chosen**: the router's executor is almost entirely ticket-specific (workspace, escapes, commit). `sandbox.Config` is already the common spawn path.

## References

- [Spec: Native EGM, Phase A](../specs/egm-nativo.md)
- [ADR-004: Sandbox](004-sandbox-isolamento.md)
- [ADR-005: Anti-cheating](005-anti-trapaca.md)
- [ADR-007: Elephant-Goldfish inspiration](007-inspiracao-elephant-goldfish.md)
- [ADR-009: Router and Workers](009-router-e-workers.md)
- [ADR-010: Invariants outside compaction](010-invariantes-fora-compactacao.md)
- [ADR-013: Multi-vendor bus](013-barramento-multi-vendor.md)
