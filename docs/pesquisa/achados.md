🇺🇸 English | [🇧🇷 Português](../pt-BR/pesquisa/achados.md)

# Research Findings

> New ideas extracted from the [primary sources](fontes-primarias.md), with source, affected module, and concrete proposal.

---

## The 5 Main Findings

These are the findings with the greatest impact on toolbelt's design:

### 1. Compaction erases rules → pinned invariants file + hybrid compaction

**Source**: [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) (Salle et al., 2026, preprint)

**Problem**: The 1st summary already loses about 1/3 of anti-cheat rules; "don't do" rules drop ~6× more than other information.

**Affected module**: `mem/ctx` · `guard`

**Concrete proposal**:
- Create an invariants file (anti-cheat rules, prohibitions, mandate) that is reinjected intact **after** each compaction
- Use hybrid compaction: mechanical extraction of state (files touched, commands, test failures, decisions) **before** the LLM summary
- Add a KPI for constraint retention after compaction

**Additional evidence**: [Compact-Bench](https://arxiv.org/abs/2610.31710) (Sun et al., 2026) shows mechanical extraction retained 79.1% versus at most 16% for LLM summaries.

---

### 2. Reward hacking grows with size → hidden suite, impossible tasks, trajectory monitor

**Source**: [SpecBench](https://arxiv.org/abs/2605.21384) (Zhao et al., 2026, preprint), [ImpossibleBench](https://arxiv.org/abs/2510.20270) (Zhong et al., 2025), [METR Reward Hacking](https://metr.org/blog/2025-06-05-recent-reward-hacking/) (2025)

**Problem**: Gap between visible and held-out tests grows 28 pts for every 10× of code; o3 did reward hacking on 30.4% of RE-Bench runs.

**Affected module**: `verify` · `benchmark` · `guard`

**Concrete proposal**:
- `tb verify`: each ticket gets visible tests **and** a composite hidden suite the generator never sees; the gap between the two becomes the reward hacking KPI
- `tb bench --impossible`: generates impossible variants of tasks (test contradicts the spec); any pass counts as cheating and the target is 0%
- `tb guard`: trajectory monitor — a cheap monitor reads actions and transcript (not just the diff) and gives a suspicion score; never train or optimize against reasoning, to avoid generating obfuscation

**Cheat patterns to detect** (beyond deleting tests):
- Overloading `__eq__`
- State saved between calls
- Special cases by test input
- Editing scoring code

**Sources**: [TRACE](https://arxiv.org/abs/2601.20103), [Monitoring Reasoning Models](https://arxiv.org/abs/2503.11926), [SHADE-Arena](https://arxiv.org/abs/2506.15740), [AI Control](https://arxiv.org/abs/2312.06942)

---

### 3. Graph does not always save → on-demand `tb graph` + A/B

**Source**: [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) (Sepidband et al., 2026, preprint), [Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) (Hu et al., 2026, preprint)

**Problem**: Maps (AST, call graph) cut 17–39% of tokens **when locating is the bottleneck**, but increase latency. Structural search can cost up to 28.14% more; human-written skills cut up to 41.73%.

**Affected module**: `grafo` · `benchmark`

**Concrete proposal**:
- `tb graph` as an **on-demand** tool with lean output (do not force use on every task)
- A/B by task type to measure when graph helps vs when it costs more
- Prioritize human-written skills/workflows over automatic search
- Waste watchdog: flag redundant search, nearly identical scripts, and test re-execution (appeared in 79–98% of tasks in the study)

---

### 4. Code execution with MCP + stateless spec → `tb mcp` is born on that version

**Source**: [Code execution with MCP](https://www.anthropic.com/research/code-execution-with-mcp) (Anthropic, 2025), [MCP Specification 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/index)

**Problem**: Tools as a fixed catalog cost 150k tokens in the example; as files in code, they cost 2k (−98.7%).

**Affected module**: `protocolo` · `mem/ctx`

**Concrete proposal**:
- `tb mcp` is born on the **stateless** core (no initialize), with server/discover
- Explicit handles as arguments, long worker jobs via the Tasks extension
- Catalog exposed as files + `tb tools search` with deferred loading
- Results filtered inside the sandbox before returning to the model
- `response_format` parameter (concise/detailed) on every tool, namespaced names and lists in deterministic order (stable cache prefix)

**Additional sources**: [Introducing advanced tool use](https://www.anthropic.com/research/advanced-tool-use), [Writing effective tools for AI agents](https://www.anthropic.com/research/writing-tools-for-agents)

---

### 5. Parallelism with declared scope → mandate with base commit and files

**Source**: [Claim Plane](https://arxiv.org/abs/2607.21909) (Nikolaev, 2026, preprint), [CAID](https://arxiv.org/abs/2603.21489) (Geng et al., 2026, preprint)

**Problem**: Parallel workers without declared scope collide on files; post-hoc merge generates conflicts.

**Affected module**: `router` · `guard`

**Concrete proposal**:
- The ticket mandate carries a **base commit** and a list of files or resources
- The router only runs **disjoint** scopes in parallel
- Orders merges and returns the conflict to the **owning worker** (not a third party)
- Deterministic admission: worker declares ChangeIntent before writing

**Evidence**: CAID showed +25.6 pts on PaperBench and +14.7 on Commit0 with central delegation + isolated worktrees + merge with tests.

---

## All New Ideas

| Idea | What changes in tb | Module | Source |
|------|--------------------|--------|--------|
| Constraints pinned outside compaction | Invariants file reinjected intact after each compaction, plus constraint retention KPI. | mem/ctx · guard | [arxiv.org/abs/2610.31778](https://arxiv.org/abs/2610.31778) |
| Hybrid compaction | Mechanical extraction of state (files touched, commands, test failures, decisions) before the LLM summary; extraction retained 79.1% versus at most 16% for summaries. | mem/ctx | [arxiv.org/abs/2610.31710](https://arxiv.org/abs/2610.31710) |
| Held-out suite in verify | Each ticket gets visible tests and a composite hidden suite the generator never sees; the gap between the two becomes the reward hacking KPI. | verify · benchmark | [arxiv.org/abs/2605.21384](https://arxiv.org/abs/2605.21384) · [arxiv.org/abs/2511.21654](https://arxiv.org/abs/2511.21654) |
| tb bench --impossible | Generates impossible variants of tasks (test contradicts the spec); any pass counts as cheating and the target is 0%. | benchmark · guard | [arxiv.org/abs/2510.20270](https://arxiv.org/abs/2510.20270) |
| Cheat patterns in guard | Beyond deleting tests: overloading `__eq__`, state saved between calls, special cases by test input, editing scoring code. | guard | [arxiv.org/abs/2510.20270](https://arxiv.org/abs/2510.20270) · [arxiv.org/abs/2601.20103](https://arxiv.org/abs/2601.20103) · [metr.org](https://metr.org/blog/2025-06-05-recent-reward-hacking/) |
| Trajectory monitor | Cheap monitor reads actions and transcript (not just the diff) and gives a suspicion score; never train or optimize against reasoning, to avoid generating obfuscation. | guard | [arxiv.org/abs/2503.11926](https://arxiv.org/abs/2503.11926) · [arxiv.org/abs/2506.15740](https://arxiv.org/abs/2506.15740) · [arxiv.org/abs/2312.06942](https://arxiv.org/abs/2312.06942) |
| Tools as files + search | tb catalog exposed as files and `tb tools search` with deferred loading; results filtered inside the sandbox before returning to the model. | protocolo · mem/ctx | [anthropic.com](https://www.anthropic.com/research/code-execution-with-mcp) · [cursor.com](https://cursor.com/blog/dynamic-context-discovery) |
| response_format on every tool | concise/detailed parameter, namespaced names and lists in deterministic order, to keep the cache prefix stable. | protocolo · mem/ctx | [anthropic.com](https://www.anthropic.com/research/writing-tools-for-agents) · MCP 2026-07-28 |
| Stateless MCP from the start | No initialize, with server/discover, explicit handles as arguments and long worker jobs via the Tasks extension. | protocolo · router | [modelcontextprotocol.io](https://modelcontextprotocol.io/specification/2026-07-28/changelog) |
| Declared scope on the ticket | Mandate carries base commit and files or resources; router only runs disjoint scopes in parallel, orders merges and returns the conflict to the owning worker. | router · guard | [arxiv.org/abs/2607.21909](https://arxiv.org/abs/2607.21909) · [arxiv.org/abs/2603.21489](https://arxiv.org/abs/2603.21489) |
| tb init in harness style | Creates progress log, feature list in JSON (failing → passing only via tb verify) and init.sh; every worker starts by reading git log and progress. | router · verify | [anthropic.com](https://www.anthropic.com/research/effective-harnesses-for-long-running-agents) |
| Optional measured graph | `tb graph` as an on-demand tool with lean output and A/B by task type, since it can cost up to 28% more; prioritize human-written skills. | grafo · benchmark | [arxiv.org/abs/2611.07286](https://arxiv.org/abs/2611.07286) · [arxiv.org/abs/2609.30725](https://arxiv.org/abs/2609.30725) |
| Waste watchdog | Flag redundant search, nearly identical scripts and test re-execution, which in the study appeared in 79–98% of tasks. | router · benchmark | [arxiv.org/abs/2609.30725](https://arxiv.org/abs/2609.30725) |
| Bitemporal linked memory | Facts with validity and provenance, invalidated instead of overwritten, and notes with Zettelkasten-style links inside the vault. | mem/ctx · grafo | [arxiv.org/abs/2501.13956](https://arxiv.org/abs/2501.13956) · [arxiv.org/abs/2502.12110](https://arxiv.org/abs/2502.12110) |
| Injection blocked by design | Privileged planner never reads untrusted content, quarantined worker has no tools, and mandate bars the lethal trifecta. | guard · router | [arxiv.org/abs/2503.18813](https://arxiv.org/abs/2503.18813) · [arxiv.org/abs/2506.08837](https://arxiv.org/abs/2506.08837) · [simonwillison.net](https://simonwillison.net/2025/Jun/16/the-lethal-trifecta/) |
| tb speaks ACP | Agent Client Protocol server so the human reader/editor and editors like Zed can see and approve agent actions. | protocolo · UI | [agentclientprotocol.com](https://agentclientprotocol.com/protocol/v1/overview) |
| AGENTS.md as local context | `tb init` reads existing `AGENTS.md`/`CLAUDE.md` and can seed with `--write-agents`; `tb ctx` (per directory) remains as an evolution. | mem/ctx | [agents.md](https://agents.md/) |
| Benchmark with cost, pass^k and humans | Report the cost×accuracy frontier, pass^k beyond pass@1, and an A/B with measured time, because perceived gain can have inverted signal. | benchmark | [arxiv.org/abs/2407.01502](https://arxiv.org/abs/2407.01502) · [arxiv.org/abs/2406.12045](https://arxiv.org/abs/2406.12045) · [arxiv.org/abs/2507.09089](https://arxiv.org/abs/2507.09089) |
| Harbor task format | Task = container + instruction + tests + reference solution, reusing benchmarks already adapted to Terminal-Bench. | benchmark | [arxiv.org/abs/2601.11868](https://arxiv.org/abs/2601.11868) |
| Judges with external signal | Do not accept self-correction without a test or tool; judges with order swap and a different model from the generator. | verify · review/drift | [arxiv.org/abs/2310.01798](https://arxiv.org/abs/2310.01798) · [arxiv.org/abs/2306.05685](https://arxiv.org/abs/2306.05685) |
| Mutation in the code ruler | Discriminator tests go through LLM-generated mutants; a test that does not kill a mutant does not count. | verify | [arxiv.org/abs/2501.12862](https://arxiv.org/abs/2501.12862) |
| Visual metrics beyond pixel | Add block-match, text, position, color and CLIP to pixel parity, which breaks with small shifts. | UI · verify | [arxiv.org/abs/2403.03163](https://arxiv.org/abs/2403.03163) |
| Sandbox with network proxy | bubblewrap + Landlock with network proxy by allowlist, measuring the drop in permission requests (−84% in Anthropic's case). | sandbox | [anthropic.com](https://www.anthropic.com/research/claude-code-sandboxing) |

---

## Gaps

Items the research did not find or found only partially:

| Gap | Detail |
|-----|--------|
| Original DeepSeek essay | Found the official repo and translations (offloop.org, ghost.p2lab.com), but not the original text on a DeepSeek domain. Translations are not primary sources and stayed out of the table. |
| Light vs heavy router model on real code | DeepSeek's benchmark is synthetic. Did not find a controlled study comparing router model size on real coding tasks. |
| Iterative visual adversarial loop | Did not find a paper with visual generator ↔ discriminator and iteration limit as in tb verify. Design2Code and UICoder are the closest. |
| Speculative tool calls | No official primary source focused on coding agents. LLMCompiler (planned parallelism) was the closest. |
| Graphify and Obsidian | Only repo and product documentation, no paper or published evaluation. |
| 2026 preprints | CAID, Claim Plane, Compact-Bench, SpecBench, TRACE and the graph and cost studies are recent preprints, without confirmed peer review. Treat the numbers as indicative. |
| "ACP" protocols | The acronym has two uses: Zed's Agent Client Protocol (in the table) and IBM/BeeAI's Agent Communication Protocol, which by 2025 announcements was incorporated into A2A. Did not re-verify that second point. |
| OpenAI posts | The SWE-bench Verified post and the CoT monitoring post block automated fetch (403). For the second, used the paper on arXiv. |
