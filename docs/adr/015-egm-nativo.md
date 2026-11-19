🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/015-egm-nativo.md)

# ADR-015: Native Reimplementation of the Elephant-Goldfish Model (`tb egm`)

## Status

accepted (2026-11-19). The R0 spike of the [core spec](../specs/egm-nativo.md) ran ([dogfood #30–#34](../campo/dogfood.md)) and revised the matching and convergence design; Phase A is implemented (PRs [#115](https://github.com/leonardo-moraes360/toolbelt/pull/115), [#116](https://github.com/leonardo-moraes360/toolbelt/pull/116) and [#117](https://github.com/leonardo-moraes360/toolbelt/pull/117)). R0 item 8 (git over a repository owned by another uid, which needs a second user) is still open. The [trust spec](../specs/egm-trust.md) stays a draft.

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

Reimplement EGM **natively** (ADR-007's Option B) as the `tb egm` group, splitting responsibility like this:

- **tb** builds the prompt, isolates and runs the goldfish, validates, matches the findings, keeps the state and decides convergence;
- **the LLM** only reads and judges.

Phase A has two specs:
- **[Core](../specs/egm-nativo.md)** (Phase A): isolation, review, loop, resolve and arbiter, plus the defenses against **third-party content** (cloned repository, downloaded file, the reviewed document itself). The core assumes an honest but sloppy elephant.
- **[Trust](../specs/egm-trust.md)** (Phase A.2, after the R0 spike): defenses against **active fraud** by the elephant as the same user.

This split came after 8 review rounds of a single spec. The core then went through 5 more rounds and was frozen for the R0 spike, with the matching and convergence design marked as provisional until the spike's data; on 2026-11-19 it was revised with that data (spec R18–R27, [dogfood #31 and #32](../campo/dogfood.md)). The anti-fraud findings grew every round, including about the defenses themselves, and kept the rest from converging.

### 1. A goldfish is an isolated process with no tools

Each goldfish is a **one-shot**, stateless process, started from `egm.agent_cmd` with `sandbox.New`. It does **not** inherit `router.agent_cmd`, which has a different contract: a writer agent, with the ticket passed via env.

- **No tools at all.** The vendor profile's flags turn off every tool.
  - The prompt goes in via **stdin**, and the findings come out via **stdout**, separate from stderr.
  - On every run, tb checks in the output envelope that no tool was used.
  - `bypassPermissions` and equivalents are refused, as in ADR-013.
- **Sandbox.**
  - **New modes:** read-only workdir, private `TMPDIR`, its own HOME and `PATH`, separate stdin and stdout. With these fields zeroed, nothing changes.
  - **Workdir:** it is the goldfish's packet, outside the repository.
  - **System reads:** the existing ones (`/etc`, `/proc`…) stay allowed. Reading another process's environ goes through the ptrace check, which Landlock restricts outside the domain; the spike confirms this. The absence of tools is a second barrier.
- **Minimal env.** The env is built from scratch. Variables that inject code into the runtime (`NODE_OPTIONS`, `PYTHONPATH`, `LD_*`…) are refused.
- **Disposable HOME.** A `0700` directory outside the repository, deleted at the end. That way, the vendor's `CLAUDE.md`, memories and global settings never reach the goldfish. Authentication is an API key via `egm.env.pass` or, with the `claude` profile, a copy of only the `claudeAiOauth` key of the vendor's credential file into the disposable HOME (`CLAUDE_CONFIG_DIR`). tb refuses to start (`auth_expired`) when the access token has less than 10 minutes left, longer than a goldfish's timeout, so no goldfish refreshes the copy and the real credential is never rotated by it.

### 2. Third-party content executes nothing and leaks nothing

- **Execution and secret keys** (agent, `PATH`, env, HOME, vendor…) come **only** from the user config. If they appear in a repository's `.toolbelt/config.yaml`, they are ignored with a warning.
- **Limit keys** (budget, rounds, parallelism, timeout, sizes) can only be **lowered** by the repository, and **policy keys** (clean rounds, attestation, matching, coverage) can only be **hardened**. That way, a repository cannot burn credits, hang the run nor loosen the review.
- **The repository config** is read with a dedicated reader: regular file, no symlink in the path, up to 64 KiB.
- **Hardened and restricted git.**
  - **Closed list:** egm only uses subcommands that run no filters, drivers or submodules. It does not use `git status` and never sweeps all versioned files.
  - **Root:** it is located through the filesystem starting from the document, without going past `$HOME`, with explicit `GIT_DIR` and `GIT_WORK_TREE`. That way, a hostile `core.worktree` cannot displace the root. A downloaded document without `.git` inside a user repository is reviewed as part of that repository, with the sources limited by the source rules.
  - **Prior inspection of `.git`:** before calling git, tb checks `.git` directly in the filesystem. It refuses symlinks, FIFOs, a diverted `commondir`, `alternates` and a `gitdir:` without a back-pointer.
  - **Git config with a list of fatal keys:** `.git/config` is read by git itself, through `git config --no-includes --list`, which executes nothing, so there is no difference in interpretation between parsers. Each key falls into one of three groups:
    - **fatal:** the ones tb's `-c` options cannot neutralize (`include`, `partialclone`, `promisor`, unknown `extensions` and `alternates` files);
    - **neutralized:** the ones tb's `-c` options override or that no allowed subcommand reads. They are accepted with a warning;
    - **all the others:** accepted.

    Security comes from the closed list of subcommands, from the `-c` options and the env, and from sentinel tests that prove nothing is executed, not from enumerating dangerous keys. Common repositories work (husky, git-lfs, sparse-checkout, actions/checkout). On top of that come `GIT_NO_LAZY_FETCH`, `protocol.allow=never` and a per-call timeout.
  - **Scope:** the router and merge do not change. The loaders that `tb stack` already runs before any op also stay as they are, which is declared out of scope.
- **Sources by real path.** Sources are only regular versioned files; the reviewed document may be unversioned.
  - Every path component is checked, so a symlinked parent directory excludes the source.
  - A secrets deny-list applies before anything else, and the extended redaction preserves the line count.
  - Other gated documents and the `docs/pt-BR/` mirrors do not go in as sources.

### 3. The reviewed document is untrusted data

The document is written by the elephant itself, so the most likely injection comes from it. The mitigations:

- **Nonce.** It delimits the data blocks and must be echoed in the output. It only proves the output came from that run; it does **not** prevent the model from obeying an injected instruction.
- **Positive attestation.** Each reviewer returns literal excerpts from the document, which tb checks against the snapshot. The minimum adjusts to the size of the range, and at least one excerpt must come from the body, not from a heading.
- **Anti-injection scan.** It is deterministic and sweeps the document, the sources and the re-evaluation block, because the model reads all of that. The initial pattern list is closed and published in the spec (`ignore previous`, `return no findings`…).
  Each section's matches become a single contestable `major` finding.
  On contest, a new goldfish decides whether the excerpt is discussion or directive.
- **Protected arbiter.**
  - The flagged excerpt arrives escaped and labeled.
  - The arbiter sees the original finding and attests what it read.
  - Terms directed at the arbiter itself near the match refuse the contest.
  - For blocker and major with an LLM owning reviewer, the withdrawal only counts if the owner agrees in the next round. Checker and scan findings are withdrawn directly.
- **Elephant text kept out of the prompt.** The re-evaluation receives only fixed fields from the findings. Notes and reasons written by the elephant do not go in.

This follows the model of [ADR-013](013-barramento-multi-vendor.md), which already treats peer content as data.

### 4. Two engines, document types and one contract

| Engine | When | Coverage |
|--------|------|----------|
| `llm` | `egm.agent_cmd` in the user config | with a detailed plan: 9 reviewers (8 personas; `style` runs over two ranges). Without a plan: the remaining 6 |
| `deterministic` | no agent or `--engine deterministic` | `refs-exist` → `codebase-details`, `required-sections` → `structural`, `plan-files-listed` → `implementation-readiness`, all **partial** |

- The checkers and the scan run **in both engines**, with a defined severity.
- A persona with no agent is `unavailable`, never "zero findings".
- The type (`spec` or `adr`, by glob) determines the structural standard. The presence of the detailed plan determines the required reviewers.
- With the `llm` engine, an ADR can reach `PASS`; a spec without a plan reaches at most `PASS_PARTIAL`.
- **Without an agent, nothing reaches `PASS`.** The deterministic engine leaves the LLM personas `unavailable`, so coverage is always partial and the best verdict for any document is `PASS_PARTIAL`. An offline user gets the checkers and the scan, and the gate passes with a warning; `--strict` or `egm.require_full_coverage` make it fail.
- `review.Finding`, `tb review` and `tb drift` **do not change**.

### 5. State and convergence outside the LLM, tied to content

The state lives outside any worktree (`<state>/egm/<repo-id>/<slug>/`). It is written atomically, with a lock released by the kernel, and stores:

- rounds, with the document and source hashes;
- finding lineages with a stable ID (`F-<k>`);
- **convergence records per exact content**.

Creating and removing worktrees does not affect the state.

- **Re-evaluation by the owning reviewer.** Each lineage has a single owner: the reviewer of the highest-severity occurrence, preferring one that sees the code.
  - The owner gets the lineage back every round and answers `still_holds`. Omitting a lineage invalidates the output.
  - The owner is fixed in Phase A. If the owner cannot run and the lineage is blocking, tb refuses the round instead of running without it.
  - A new finding with a higher severity than an already declined or closed lineage becomes a new lineage, instead of being absorbed.
  - Checker lineages close or reopen by re-running the checker.
- **Clean round.** No blocking lineage (blocker or major open, contested, escalated or unconfirmed `fixed`) and no required reviewer `not_run`.
- **Convergence.**
  - It takes 2 consecutive clean rounds **over the same content**, with a cap of `egm.max_rounds` (default 8) per cycle.
  - Converging writes a record for that content and closes the cycle.
  - After the cap, the verdict is `FAIL`, and the way out is `--restart` with new content.
- **Two kinds of downgrades.**
  - **Lineage downgrades** (forced settle, `--decline` of a major) persist while the lineage stays in that status. An edit followed by a new convergence does not erase them. They only go away with `--fix` over a real fix, confirmed by the owning reviewer.
  - **Cycle downgrades** (`--restart`, partial coverage, `max_*` exclusions…) apply only to that cycle's convergence. `--restart` always requires new content.
- **Verdicts**, evaluated in a fixed order over the worktree content or, in the gate, over the commit's content:
  - `NO_STATE`;
  - `PASS`: a record for this content and this policy, with no downgrade at all and no blocking lineage;
  - `PASS_PARTIAL`: a record with a downgrade. `tb stack` gains the WARN state for this case, and `--strict` or `egm.require_full_coverage` turn it into a failure;
  - `FAIL`: the cycle hit `max_rounds` without converging;
  - `STALE`: there is a record only for another content or for another policy, or there is a blocking lineage over an already converged content;
  - `IN_PROGRESS`.

### 6. Sloppy shortcuts do not reach `PASS`

- **`--fix` requires a real change.** It only counts if, since the lineage was last opened, a diff that is not whitespace-only touched the finding's lines or the cited lines of a source. Fixes elsewhere close through the owner's re-evaluation.
- **A contest goes to an arbiter.** At most **one** per lineage, decided by a new **arbiter** goldfish, run on the next `loop next`.
  - **Evidence:** it must be among the lineage's cited and verified sources, and it goes through the source rules again.
  - **No agent:** there is no contest.
  - **Failing arbiter:** after two consecutive arbiter failures, the lineage is escalated.
  - **Giving up:** whoever contested can give up with `--fix`, which requires a real edit, like any `--fix`.
  - **If the arbiter withdraws** a `nit`, a checker finding or one from the anti-injection scan, it becomes `settled`.
  - **If the arbiter withdraws** a blocker or major with an LLM owner, the finding becomes `fixed`, and the owner confirms or reopens it.
  - **If the arbiter upholds**, the finding goes back to `open`.
- **Severity does not vanish through matching.** A new finding more severe than an already closed lineage becomes a new lineage.
- **`--decline` is no escape hatch.** It applies to `nit`. For `major`, it downgrades the verdict. For `blocker`, it does not exist.
- **The gate evaluates the commit.** The `egm` op of `tb stack` checks the gated documents touched in the stack range, by their content at `head`, not by the worktree. A document whose sources changed without it being touched only produces a warning; drift detection is left to Phase B. An empty selection produces a warning, and an empty selection with uncommitted gated documents fails. A blocker resolved with a forced settle fails the gate even without `--strict`. Choosing the range by trust ref is left to the trust spec.

### 7. Gates become explicit transitions

The upstream's interactive questions become idempotent subcommands of `tb egm resolve`, with an explicit transition table. The elephant asks the user and records the answer. tb never edits the document nor changes severity.

### 8. Scope and network

- **Core (Phase A).**
  - **Commands:** `review`, `loop next`, `resolve` and `status`, for specs and ADRs.
  - **Preset and profiles:** the `consolidated` preset, the `claude` profile and a `custom` profile with an explicit "no tools" declaration.
  - **Personas:** embedded, with original text, not copied from upstream.
  - **MCP:** in Phase A, only `tb_egm_status` and `tb_egm_resolve`. `review` and `loop next` take minutes and stay CLI-only.
- **Trust (Phase A.2):** see the [spec](../specs/egm-trust.md).
- **Following specs:**
  - **Phase B:** `drift-report` and `bootstrap`.
  - **Phase C:** `draft` and `brainstorm`, other presets, custom personas by path, `codex` profile, per-reviewer agent override, symbol verification in the graph, churn and the `review.Finding` → `egm.Finding` adapter.
- **Network.** `tb` still does not talk to the network. The agent process is what talks to the provider. With no agent, everything is 100% offline.
- **Relation to ADR-013.** The goldfish are one-shot workers, not bus agents.
- **Docs location.** Specs live in `docs/specs/` and ADRs in `docs/adr/`.

## Consequences

### Positive

- Isolation is guaranteed by process, sandbox, disposable HOME, minimal env and verified absence of tools, without depending on the harness.
- Running `tb egm` over a third-party repository or a downloaded file executes nothing and leaks no secret, whether through config, symlink or git config.
- Sloppy shortcuts do not become `PASS`:
  - `--fix` without fixing;
  - convergence over different texts;
  - re-rolling the arbiter;
  - new severity absorbed by a closed lineage;
  - editing after `PASS`;
  - a forced settle "laundered" by an edit.
- The verdict holds for an exact content and survives worktrees.
- The core spec became implementable and reviewable. The active-fraud part matures separately, with data from the spike.

### Negative

- Touching `internal/sandbox`, a security-critical component.
- egm gets its own git hardening. The router and merge keep theirs, and unification is left to the trust spec.
- **Vendor credentials:**
  - the copy of the `claudeAiOauth` key creates a short local exposure window (a `0600` file in the `0700` disposable HOME, deleted at the end);
  - the copy cannot be refreshed, so with less than 10 minutes left tb refuses to run and the user runs the vendor CLI once outside tb;
  - on macOS, the credential lives in the Keychain, a declared limitation.
- The anti-injection scan is heuristic, so a subtle injection obeyed by every reviewer remains a residual risk.
- **In Phase A, an agent that actively commits fraud as the same user can reach `PASS`.** The core promises nothing against this, and the gate is local.
- If the spike shows that Landlock does not block `/proc/<pid>/environ` of another process, protecting tb's environ comes to depend only on the vendor honoring "no tools", which tb checks in the envelope.
- Touching a gated document requires reviewing it, old ADRs included.
- Convergence with 9 non-deterministic reviewers is rare on drafts: in R0 neither real document converged in 4–6 rounds, because each edit drew new findings ([dogfood #32 and #33](../campo/dogfood.md)). Phase A targets documents past the draft stage, keeps `max_rounds: 8` and reports `capped` instead of promising convergence.
- **Cost:** about 9 agent runs per round (about US$ 2–2.5 with `claude`), limited by budget.
- Vendor profiles depend on flags and envelopes of third-party CLIs, which change.
- There are in-house personas to maintain.

## Alternatives Considered

### Option A: EGM skills via MCP or plugin

- Reuses the ready-made upstream.
- **Not chosen** because of problems 1 to 4 in the Context.

### Only harden the current heuristics

- Zero LLM cost.
- **Not chosen**: it does not review documents, and the user asked for the full EGM.

### Read-only allowlist of the real HOME

- **Not chosen**: the global `CLAUDE.md` and the vendor's memories contaminate the goldfish.

### Goldfish outside the sandbox (`--allow-unsandboxed`)

- **Not chosen**: it violates the fail-closed sandbox.

### A single spec with core and anti-fraud

- **Tried and abandoned** after 8 review rounds without a single clean round. Every round found new attacks by someone with a shell as the same user, including against the freshly added defenses, and one blocker came from a fix made in the previous round.

### Unify the git hardening of router, merge and egm now

- **Not chosen for Phase A**: the union of the lists changes merge's behavior, which would stop seeing gitlinks in the sensitive-path check, and breaks the router's env.

### Detect rename and copy by content fingerprint

- **Not chosen**: a renamed or copied document starts with no state and must converge from scratch. Detection did not protect any `PASS`.

## References

- [Spec: native EGM, Phase A, core](../specs/egm-nativo.md)
- [Spec: EGM trust, Phase A.2](../specs/egm-trust.md)
- [ADR-004: Sandbox](004-sandbox-isolamento.md)
- [ADR-005: Anti-cheating](005-anti-trapaca.md)
- [ADR-007: Elephant-Goldfish inspiration](007-inspiracao-elephant-goldfish.md)
- [ADR-009: Router and Workers](009-router-e-workers.md)
- [ADR-010: Invariants outside compaction](010-invariantes-fora-compactacao.md)
- [ADR-013: Multi-vendor bus](013-barramento-multi-vendor.md)
