🇺🇸 English | [🇧🇷 Português](../pt-BR/pesquisa/alethe-cma.md)

# What tb can learn from alethe and "plan big, execute small"

> **Review date**: 2026-11-24 (recorded 2026-11-26, when the tb claims below were checked again on `main`)
> **References**:
> - [Kc1t/alethe-orchestrate-skill](https://github.com/Kc1t/alethe-orchestrate-skill) (HEAD `ab811045` on 2026-11-26)
> - [Kc1t/alethe-agents](https://github.com/Kc1t/alethe-agents) (HEAD `0acefe82` on 2026-11-26), mainly `docs/ORCHESTRATOR_ROADMAP.md`
> - [anthropics/claude-cookbooks, `managed_agents/CMA_plan_big_execute_small.ipynb`](https://github.com/anthropics/claude-cookbooks/blob/main/managed_agents/CMA_plan_big_execute_small.ipynb) (HEAD `8ad5de64` on 2026-11-26)
>
> **Target**: `tb router` ([ADR-009](../adr/009-router-e-workers.md)), the bus ([ADR-013](../adr/013-barramento-multi-vendor.md) §5, [bus-router.md](../bus-router.md)), `tb bus run` ([internal/busrun/](../../internal/busrun/)) and `tb bench` ([internal/bench/](../../internal/bench/))

All three deal with the problem tb's router and bus deal with: a strong model plans and cheaper workers execute. The most useful parts are things tb has already planned and not built yet. [bus-router.md](../bus-router.md) lists "the escalation ladder by tier" and "picking an agent by capability or tier" as future work.

## alethe-orchestrate-skill (most value)

1. **Tier plus profile.** Phases name only a tier (`cheap`, `standard` or `strong`). A profile (`base`, `savings`, `aggressive`) maps each tier to a CLI, a model and an effort level. That gives the router and the bus the tier routing they lack without tying them to a vendor.
2. **Declarative YAML playbooks.** Each phase has `role`, `tier`, `readOnly`, `workers`, `gate` and `after`/`if`/`then` conditions (`conflict`, `checks-failed`). tb has tickets and a board but no "recipe" layer above them. The playbooks that fit tb:
   - `cascade`: the L2 escalation ladder of ADR-013;
   - `review-gate`: author ≠ verifier, as in [ADR-008](../adr/008-verificacao-adversarial.md);
   - `scout-then-build`, `best-of-n` and `parallel-worktrees`;
   - `direct`: skips orchestration for small tasks.
3. **One standard folder per worker**: `prompt.txt`, `result.md`, `diff.patch` and `outcome.json` (status, failure kind, metrics, tripwire). The lead reads only `result.md` and the transcript stays on disk. Its report measures this (for example "5.0 KB on disk, 595 B read"), which maps directly onto tb's "save tokens" goal as a KPI.
4. **Real spend, marked `used`, `partial` or `discarded`.** Tokens and cost come from what each CLI reported, split between work that was used and work that was thrown away. `internal/bench` already has `Tokens` and `CostUSD`; router tickets do not carry them yet.
5. **Quota guard.** Before each wave it reads local Claude and Codex usage (session files, no network) and refuses to go past a cutoff. This respects tb's local-only rule.
6. **Budget per wave and per orchestration.** `tb bus run` limits only the number of wakes (`budget.max_wakes`, default 6). It has no limit in tokens or dollars.
7. **Plan-change journal**: `skip`, `add` and `switch` entries, each with a reason, shown in the final report.
8. **Worker hardening.**
   - The worker environment keeps only the provider's auth and drops everything else. tb already does this: `internal/execenv` filters the environment for `verify`, the router and `tb bus run`.
   - Tripwire: git detects a read-only worker that wrote files. This is an extra defense when a vendor CLI runs outside Landlock.
   - Internal git runs with hooks and fsmonitor off. tb already does this in `egm`, `workspace` and `merge`.

## alethe-agents (a Tauri app; little fits)

The app itself (PTY, grid, themes, plugins) is out of scope. The useful part is `docs/ORCHESTRATOR_ROADMAP.md`, which maps `claude -p --input-format stream-json --output-format stream-json` empirically:

- **A live, multi-turn process.** A persistent adapter in `tb bus run` would avoid a cold start on every wake.
- **A `rate_limit_event` on every turn** (`rateLimitType`, `resetsAt`, `status`): a free quota signal.
- **A `result` event** with `total_cost_usd`, `usage` including cache, `permission_denials` and `terminal_reason`: router telemetry with no extra work.
- **No approval channel.** Anything that needs permission is denied on the spot (`permission_denied`). This matches tb's choice of a closed allowlist for the `claude` preset.
- **Interrupt** via a `control_request` with `subtype: interrupt`. There is no way to steer in the middle of a turn.

Caveat: this was taken from the binary and is not documented, so it can change between versions.

Medium value:

- **Reading and writing each vendor's MCP config.** A `tb init` step that registers `tb mcp` in Claude, Codex, Cursor and OpenCode would fix the onboarding that failed in the field study.
- **Claude→Codex handoff through a redacted packet.** This fits the bus's payload-by-reference and its existing redaction.

## Cookbook: CMA_plan_big_execute_small

- **It backs the pattern with numbers**: 84–98% of the team's input was billed at the worker's rate.
- **Safety boundary**: the orchestrator has no tools and workers get only the tools they need. This could become a "reader" mandate preset, consistent with ADR-013's rule that a message from another agent is untrusted data.
- **Budget enforced at list price across every thread.** This reinforces item 6 above.
- **Method for `tb bench compare`:**
  - the solo arm must meet the same verification bar as the arm with workers. Otherwise solo looks cheap only because it does less rigorous work;
  - delegation has a fixed cost per worker, and thinner briefs made runs more expensive, so granularity has an optimum;
  - check the premise of the decomposition, not only the facts. In their run, both arms built the same list and got one item wrong;
  - without delegation, the strong tier was paid for nothing.
- **The orchestrator does not see the workers' prompts.** In tb the Agent Card sent at `bus join` can solve that, which is an advantage tb already has.

## Suggested order (after v1.0.0)

1. Tier and profile in the router and the bus, together with `cascade` (the planned L2).
2. The `outcome.json` format, cost accounting per ticket and a budget in tokens or dollars.
3. A persistent stream-json `claude` adapter in `tb bus run`, using `rate_limit_event` as the quota guard.
4. A catalog of YAML playbooks on top of the router.
5. A "matched rigor" arm and a "workers' share of input" KPI in the bench.

## Not worth bringing

Passive mode through a Claude plugin hook (tb is not a plugin), the desktop UI and the plugin catalog.
