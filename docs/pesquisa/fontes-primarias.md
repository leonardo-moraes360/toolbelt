🇺🇸 English | [🇧🇷 Português](../pt-BR/pesquisa/fontes-primarias.md)

# Primary Sources

> **Research date**: 2026-11-15  
> **Total sources**: ~100 (papers, specs, official engineering blogs, repos and benchmarks)

This is a compilation of primary sources that may influence toolbelt's design. We prioritized 2024–2026 publications and only original sources: the paper, the spec, the post from who built it, or the official repository. All links were verified (HTTP 200), except the OpenAI SWE-bench Verified post, which blocks automated access (403) but exists.

**Note on 2026 preprints**: CAID, Claim Plane, Compact-Bench, SpecBench, TRACE and the graph and cost studies are recent preprints, without confirmed peer review. Cited numbers should be treated as indicative.

---

## Index by Area

- [Multi-agent architecture](#multi-agent-architecture)
- [Context and memory](#context-and-memory)
- [Graph / repo search](#graph--repo-search)
- [Protocols](#protocols)
- [Sandbox and security](#sandbox-and-security)
- [Reward hacking](#reward-hacking)
- [Verification](#verification)
- [UI / visual](#ui--visual)
- [Benchmarks](#benchmarks)
- [Token efficiency](#token-efficiency)

---

## Multi-agent architecture

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [Building effective AI agents](https://www.anthropic.com/research/building-effective-agents) | Anthropic | 2024 | blog | Catalog of patterns (routing, orchestrator-workers, evaluator-optimizer) that maps directly to router and verify. | router · verify | high |
| [How we built our multi-agent research system](https://www.anthropic.com/research/multi-agent-research-system) | Anthropic | 2025 | blog | Lead + subagents: +90.2% on internal eval, but ~15× the tokens of a chat; tokens explain 80% of variance. | router · benchmark | high |
| [Effective harnesses for long-running agents](https://www.anthropic.com/research/effective-harnesses-for-long-running-agents) | Anthropic | 2025 | blog | Initializer agent, init.sh, progress log and feature list in JSON; one feature per session, marked only after e2e test. | router · mem/ctx · verify | high |
| [Don't Build Multi-Agents](https://cognition.com/blog/dont-build-multi-agents) | Cognition | 2025 | blog | Counterpoint: subagents without shared context make conflicting implicit decisions. | router · review/drift | high |
| [Why Do Multi-Agent LLM Systems Fail? (MAST)](https://arxiv.org/abs/2503.13657) | Cemri et al. (UC Berkeley) | 2025 | paper | Failure taxonomy (specification, misalignment, verification) that becomes the watchdog checklist. | router · benchmark | high |
| [Effective Strategies for Asynchronous Software Engineering Agents (CAID)](https://arxiv.org/abs/2603.21489) | Geng et al. | 2026 | paper | Central delegation + isolated worktrees + merge with tests: +25.6 pts on PaperBench and +14.7 on Commit0. **(preprint)** | router · sandbox | high |
| [Claim Plane: Enforceable Change Intents for Parallel Coding Agents](https://arxiv.org/abs/2607.21909) | Nikolaev | 2026 | paper | Worker declares ChangeIntent (base commit, resources, deps) before writing; deterministic admission avoids collision. **(preprint)** | router · guard | medium |
| [dsh-superagent-prototype](https://github.com/deepseek-ai/dsh-superagent-prototype) | DeepSeek | 2026 | repo | Router/workers prototype with watchdog and direct delivery; basis of ADR 009. | router | high |
| [elephant-goldfish-model](https://github.com/google/elephant-goldfish-model) | Google | 2026 | repo | Writer with context + clean-context reviewers; basis of tb review / tb drift. | review/drift | medium |
| [Magentic-One](https://arxiv.org/abs/2411.04468) | Fourney et al. (Microsoft) | 2024 | paper | Orchestrator with Task Ledger and Progress Ledger that replans when stuck. | router | medium |
| [MetaGPT](https://arxiv.org/abs/2308.00352) | Hong et al. | 2023 | paper | Roles with SOPs and structured artifacts between them; reference for personas. | router | medium |
| [AutoGen](https://arxiv.org/abs/2308.08155) | Wu et al. (Microsoft) | 2023 | paper | Conversation between agents as a primitive; useful as a baseline of what to avoid in tokens. | router | low |
| [ChatDev](https://arxiv.org/abs/2307.07924) | Qian et al. | 2023 | paper | Chain of software company roles; historical reference for personas. | router | low |
| [OpenAI Agents SDK](https://github.com/openai/openai-agents-python) | OpenAI | 2025 | repo | Handoffs, guardrails and tracing; successor to Swarm (github.com/openai/swarm). | router · guard | medium |
| [LangGraph](https://github.com/langchain-ai/langgraph) | LangChain | 2023– | repo | State machine with durable checkpoints and resume; model for the board. | router | medium |
| [OpenHands](https://arxiv.org/abs/2407.16741) | Wang et al. | 2024 | paper | Open platform with Docker sandbox and event stream; runtime reference. | sandbox · benchmark | medium |
| [Executable Code Actions Elicit Better LLM Agents (CodeAct)](https://arxiv.org/abs/2402.01030) | Wang et al. | 2024 | paper | Actions as code instead of JSON: fewer turns and more composition. | protocol · mem/ctx | medium |
| [An LLM Compiler for Parallel Function Calling](https://arxiv.org/abs/2312.04511) | Kim et al. | 2023 | paper | Plans tool calls as a DAG and executes in parallel; less latency. | router | medium |
| [paperclip](https://github.com/paperclipai/paperclip) | Paperclip AI | 2026 | repo | Control plane for agent teams: task watchdog that reviews a stopped work tree once per stop fingerprint, upgrade-safe run state, layered prompt-injection guard, low-trust preset. tb review in [paperclip.md](paperclip.md). | bus · router · egm | high |
| [alethe-orchestrate-skill](https://github.com/Kc1t/alethe-orchestrate-skill) · [alethe-agents](https://github.com/Kc1t/alethe-agents) | Kc1t | 2026 | repo | Tier + profile routing, YAML playbooks (cascade, review-gate, best-of-n), per-worker `outcome.json`, local quota guard; map of `claude -p` stream-json events. tb review in [alethe-cma.md](alethe-cma.md). | router · bus · bench | high |
| [CMA: plan big, execute small](https://github.com/anthropics/claude-cookbooks/blob/main/managed_agents/CMA_plan_big_execute_small.ipynb) | Anthropic | 2026 | notebook | Strong orchestrator without tools, cheap workers: 84–98% of input billed at worker rate; matched-rigor method for comparing solo vs. delegated. tb review in [alethe-cma.md](alethe-cma.md). | router · bench | medium |

---

## Context and memory

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [Effective context engineering for AI agents](https://www.anthropic.com/research/effective-context-engineering-for-ai-agents) | Anthropic | 2025 | blog | Compaction, structured notes, just-in-time context and subagents: the tb ctx manual. | mem/ctx | high |
| [Context Rot](https://www.trychroma.com/research/context-rot) | Chroma | 2025 | paper | Performance drops with input size across 18 models, even on simple tasks. | mem/ctx · benchmark | high |
| [Lost in the Middle](https://arxiv.org/abs/2307.03172) | Liu et al. | 2023 | paper | Information in the middle of context is forgotten most; guides assembly order. | mem/ctx | medium |
| [Compact-Bench](https://arxiv.org/abs/2610.31710) | Sun et al. | 2026 | paper | LLM summaries retain at most 16% of recurring details; mechanical extraction retains 79.1%. **(preprint)** | mem/ctx · benchmark | high |
| [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) | Salle et al. | 2026 | paper | The 1st summary already loses about 1/3 of anti-cheat rules; "don't do" rules drop ~6× more. **(preprint)** | mem/ctx · guard | high |
| [MemGPT](https://arxiv.org/abs/2310.08560) | Packer et al. | 2023 | paper | OS-style hierarchical memory (in-context vs archived) with paging. | mem/ctx | medium |
| [Generative Agents](https://arxiv.org/abs/2304.03442) | Park et al. | 2023 | paper | Retrieval by recency, importance and relevance, plus periodic reflection. | mem/ctx | medium |
| [A-MEM: Agentic Memory for LLM Agents](https://arxiv.org/abs/2502.12110) | Xu et al. | 2025 | paper | Zettelkasten-style notes with evolving links; fits the Obsidian vault. | mem/ctx · graph | high |
| [Zep: A Temporal Knowledge Graph Architecture for Agent Memory](https://arxiv.org/abs/2501.13956) | Rasmussen et al. | 2025 | paper | Edges with temporal validity: invalidates facts instead of overwriting (code: getzep/graphiti). | mem/ctx · graph | high |
| [Mem0](https://arxiv.org/abs/2504.19413) | Chhikara et al. | 2025 | paper | Memory extraction and update pipeline (add, update, delete). | mem/ctx | medium |
| [Agent Workflow Memory](https://arxiv.org/abs/2409.07429) | Wang et al. | 2024 | paper | Induces reusable workflows from successful trajectories. | mem/ctx | medium |
| [Equipping agents with Agent Skills](https://www.anthropic.com/research/equipping-agents-for-the-real-world-with-agent-skills) | Anthropic | 2025 | blog | Progressive disclosure: only name and description in context, body loaded on demand. | mem/ctx · protocol | high |
| [Dynamic context discovery](https://cursor.com/blog/dynamic-context-discovery) | Cursor | 2026 | blog | Files as context interface; −46.9% tokens on runs that use MCP. | mem/ctx | high |
| [Prompt caching (Claude)](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) | Anthropic | 2024– | spec | Prefix rules and cache breakpoints; defines stable context order. | mem/ctx · benchmark | high |
| [Prompt caching (OpenAI)](https://developers.openai.com/api/docs/guides/prompt-caching) | OpenAI | 2024– | spec | Automatic cache by prefix; same stable-prefix principle. | mem/ctx | medium |
| [AGENTS.md](https://agents.md/) | AAIF / Linux Foundation | 2025 | spec | Open format for per-project instructions; standard local context. | mem/ctx · protocol | high |

---

## Graph / repo search

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [RepoGraph](https://arxiv.org/abs/2410.14684) | Ouyang et al. | 2024 | paper | Line-level code graph pluggable into SWE-bench agents. | graph | high |
| [LocAgent](https://arxiv.org/abs/2503.09089) | Chen et al. | 2025 | paper | Heterogeneous graph (contains, imports, calls, inherits) with search tools for location. | graph | high |
| [CodexGraph](https://arxiv.org/abs/2408.03910) | Liu et al. | 2024 | paper | Agent queries a code graph database with structured queries. | graph | medium |
| [Building a better repository map with tree sitter](https://aider.chat/2023/10/22/repomap.html) | Aider | 2023 | blog | Repo map with tree-sitter + graph ranking under a token budget. | graph · mem/ctx | high |
| [SWE-agent: Agent-Computer Interfaces](https://arxiv.org/abs/2405.15793) | Yang et al. | 2024 | paper | Tool interface designed for the agent (viewer, search, lint on edit). | protocol · graph | high |
| [Agentless](https://arxiv.org/abs/2407.01489) | Xia et al. | 2024 | paper | Locate → repair → validate without an agentic loop; cheap baseline for the benchmark. | benchmark · router | medium |
| [From Local to Global: GraphRAG](https://arxiv.org/abs/2404.16130) | Edge et al. (Microsoft) | 2024 | paper | Communities and hierarchical summaries for global questions about the corpus. | graph | medium |
| [LightRAG](https://arxiv.org/abs/2410.05779) | Guo et al. | 2024 | paper | Graph + vector at two levels with incremental update. | graph | medium |
| [Improving agent with semantic search](https://cursor.com/blog/semsearch) | Cursor | 2025 | blog | Semantic search gives 12.5% more accuracy on average than grep alone. | graph | high |
| [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) | Sepidband et al. | 2026 | paper | Maps (AST, call graph) cut 17–39% of tokens when locating is the bottleneck, but increase latency. **(preprint)** | graph · benchmark | high |
| [Analyzing and Mitigating Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) | Hu et al. | 2026 | paper | Structural search can cost up to 28.14% more; human-written skills cut up to 41.73%. **(preprint)** | graph · benchmark | high |
| [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) | Tree-sitter | 2018– | spec | Incremental multi-language parser with Go bindings for graph extraction. | graph | medium |
| [SQLite FTS5](https://www.sqlite.org/fts5.html) | SQLite | 2015– | spec | Full-text index with BM25 and tokenizers; tb's search engine. | graph | medium |
| [Graphify](https://github.com/Graphify-Labs/graphify) | Graphify-Labs | 2026 | repo | Queryable graph of code, docs and schemas; inspiration already adopted. | graph | medium |

---

## Protocols

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [MCP Specification 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/index) | MCP / AAIF | 2026 | spec | Stateless core, server/discover, cacheable lists with deterministic order, Tasks as extension. | protocol | high |
| [MCP Security Best Practices](https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices) | MCP | 2025– | spec | Confused deputy, token passthrough, SSRF and minimal scopes. | protocol · guard | high |
| [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) | Linux Foundation | 2025– | spec | Agent Card, tasks, streaming and JSON-RPC/gRPC/REST bindings for agent↔agent. | protocol · router | high |
| [Agent Client Protocol (ACP)](https://agentclientprotocol.com/protocol/v1/overview) | Zed | 2025 | spec | JSON-RPC over stdio between editor and agent, with permission requests; bridge for the human reader. | protocol · UI | high |
| [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification) | JSON-RPC WG | 2010 | spec | Base of MCP, A2A and ACP; tb's transport. | protocol | medium |
| [Writing effective tools for AI agents](https://www.anthropic.com/research/writing-tools-for-agents) | Anthropic | 2025 | blog | Namespacing and response_format concise/detailed (~1/3 of tokens in the example). | protocol · mem/ctx | high |
| [is-it-legit-mcp](https://github.com/m8ven/is-it-legit-mcp) · [M8ven Verified](https://m8ven.ai/verified/how-we-verify) | M8ven | 2026 | repo | Reference for MCP server checks: annotations (readOnly/destructive/idempotent/openWorld) justified by the handler's real effect, code-vs-declaration verification, sandbox, supply chain and publisher. tb review in [mcp-sinais-m8ven.md](mcp-sinais-m8ven.md). | protocol · guard | high |

---

## Sandbox and security

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [Landlock: unprivileged access control](https://docs.kernel.org/userspace-api/landlock.html) | Linux kernel | 2021– | spec | FS and network restriction without root; main sandbox layer. | sandbox | high |
| [Seccomp BPF](https://docs.kernel.org/userspace-api/seccomp_filter.html) | Linux kernel | 2012– | spec | Syscall filter for worker processes. | sandbox | medium |
| [bubblewrap](https://github.com/containers/bubblewrap) | containers | 2016– | repo | Privilege-free sandbox via namespaces; the same used by Claude Code on Linux. | sandbox | high |
| [Making Claude Code safer with sandboxing](https://www.anthropic.com/research/claude-code-sandboxing) | Anthropic | 2025 | blog | FS isolation + network proxy; −84% permission requests in internal use. | sandbox · guard | high |
| [gVisor](https://gvisor.dev/docs/) | Google | 2018– | spec | User-space kernel for stronger isolation than namespaces. | sandbox | medium |
| [Firecracker](https://github.com/firecracker-microvm/firecracker) | AWS | 2017– | repo | MicroVMs for remote or untrusted workers. | sandbox | low |
| [git-worktree](https://git-scm.com/docs/git-worktree) | Git | 2015– | spec | One working tree per agent; basis of edit isolation. | sandbox · router | medium |
| [Defeating Prompt Injections by Design (CaMeL)](https://arxiv.org/abs/2503.18813) | Debenedetti et al. (Google DeepMind) | 2025 | paper | Separates control and data flow, with capabilities per value. | guard · sandbox | high |
| [Design Patterns for Securing LLM Agents against Prompt Injections](https://arxiv.org/abs/2506.08837) | Beurer-Kellner et al. | 2025 | paper | Six patterns (plan-then-execute, dual LLM, map-reduce…) applicable to the router. | guard · router | high |
| [The Dual LLM pattern](https://simonwillison.net/2023/Apr/25/dual-llm-pattern/) | Simon Willison | 2023 | blog | Privileged LLM never sees untrusted content; quarantined LLM without tools. | guard | medium |
| [The lethal trifecta for AI agents](https://simonwillison.net/2025/Jun/16/the-lethal-trifecta/) | Simon Willison | 2025 | blog | Private data + untrusted content + external output = exfiltration; simple rule for the mandate. | guard | high |
| [OWASP Top 10 for LLM Applications](https://genai.owasp.org/llm-top-10/) | OWASP | 2025 | spec | Risk list (injection, excessive agency…) for the threat model. | guard | medium |
| [Agentic AI – Threats and Mitigations](https://genai.owasp.org/resource/agentic-ai-threats-and-mitigations/) | OWASP | 2025 | spec | Agent-specific threats (poisoned memory, tool abuse). | guard | medium |
| [AgentDojo](https://arxiv.org/abs/2406.13352) | Debenedetti et al. | 2024 | benchmark | Environment to measure prompt injection attacks and defenses in agents. | benchmark · guard | medium |

---

## Reward hacking

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [Recent Frontier Models Are Reward Hacking](https://metr.org/blog/2025-06-05-recent-reward-hacking/) | METR | 2025 | blog | o3 did reward hacking on 39 of 128 RE-Bench runs (30.4%), tampering with scoring code. | guard · benchmark | high |
| [ImpossibleBench](https://arxiv.org/abs/2510.20270) | Zhong, Raghunathan, Carlini | 2025 | benchmark | Tasks with tests that contradict the spec: any pass is cheating (code: safety-research/impossiblebench). | guard · benchmark | high |
| [SpecBench](https://arxiv.org/abs/2605.21384) | Zhao et al. | 2026 | benchmark | Gap between visible and held-out tests grows 28 pts for every 10× of code. **(preprint)** | verify · guard | high |
| [EvilGenie](https://arxiv.org/abs/2511.21654) | Gabor et al. | 2025 | benchmark | LLM judge detects obvious hacks well; held-out tests help little. | guard · verify | high |
| [TRACE: Reward Hack Detection via Contrastive Analysis](https://arxiv.org/abs/2601.20103) | Deshpande et al. | 2026 | benchmark | Taxonomy of 54 exploit categories; becomes the guard's pattern list. **(preprint)** | guard | medium |
| [Monitoring Reasoning Models for Misbehavior](https://arxiv.org/abs/2503.11926) | Baker et al. (OpenAI) | 2025 | paper | CoT monitor catches reward hacking; penalizing CoT generates obfuscation. | guard | high |
| [Natural Emergent Misalignment from Reward Hacking](https://arxiv.org/abs/2511.18397) | MacDiarmid et al. (Anthropic) | 2025 | paper | Reward hacking in production code generalizes to sabotage; prevention matters. | guard | medium |
| [SHADE-Arena](https://arxiv.org/abs/2506.15740) | Kutasov et al. (Anthropic) | 2025 | benchmark | Monitor gives a 0–100 suspicion score to the trajectory; design for the guard monitor. | guard · benchmark | medium |
| [AI Control: Improving Safety Despite Intentional Subversion](https://arxiv.org/abs/2312.06942) | Greenblatt et al. | 2023 | paper | Trusted weaker model monitors the stronger one; basis of the cheap discriminator. | guard · verify | high |

---

## Verification

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [Large Language Models Cannot Self-Correct Reasoning Yet](https://arxiv.org/abs/2310.01798) | Huang et al. | 2023 | paper | Without external signal, self-correction does not improve; justifies the adversarial verifier. | verify | high |
| [Judging LLM-as-a-Judge (MT-Bench)](https://arxiv.org/abs/2306.05685) | Zheng et al. | 2023 | paper | Position, verbosity and self-preference biases in LLM judges. | verify · review/drift | high |
| [Large Language Models are not Fair Evaluators](https://arxiv.org/abs/2305.17926) | Wang et al. | 2023 | paper | Swapping answer order and calibrating reduces position bias. | verify | medium |
| [Agent-as-a-Judge](https://arxiv.org/abs/2410.10934) | Zhuge et al. | 2024 | paper | Agent evaluates the entire trajectory against requirements (DevAI). | verify · benchmark | medium |
| [Improving Factuality and Reasoning through Multiagent Debate](https://arxiv.org/abs/2305.14325) | Du et al. | 2023 | paper | Debate between instances improves answers; expensive in tokens. | verify | low |
| [Training Verifiers to Solve Math Word Problems](https://arxiv.org/abs/2110.14168) | Cobbe et al. (OpenAI) | 2021 | paper | Generator + verifier trained with best-of-n; origin of the pattern. | verify | medium |
| [CodeT: Code Generation with Generated Tests](https://arxiv.org/abs/2207.10397) | Chen et al. (Microsoft) | 2022 | paper | Generated tests + dual consensus choose the solution; ruler without seeing the implementation. | verify | medium |
| [Mutation-Guided LLM-based Test Generation at Meta (ACH)](https://arxiv.org/abs/2501.12862) | Foster et al. (Meta) | 2025 | paper | LLM generates mutants and tests that kill them; measures whether tests catch bugs. | verify | high |
| [Agentic Property-Based Testing](https://arxiv.org/abs/2510.09907) | Maaz et al. | 2025 | paper | Agent infers properties and writes Hypothesis tests; found real bugs in 100 packages. | verify | medium |

---

## UI / visual

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [Design2Code](https://arxiv.org/abs/2403.03163) | Si et al. (Stanford) | 2024 | benchmark | Visual metrics by block, text, position, color and CLIP, beyond pixel. | UI · verify | high |
| [WebSight dataset](https://arxiv.org/abs/2403.09029) | Laurençon et al. (Hugging Face) | 2024 | benchmark | 2M screenshot→HTML pairs; source of synthetic cases for the visual ruler. | UI | low |
| [UICoder](https://arxiv.org/abs/2406.07739) | Wu et al. (Apple) | 2024 | paper | Automatic filters (compilation + CLIP) as feedback to generate UI. | UI · verify | medium |
| [Playwright visual comparisons](https://playwright.dev/docs/test-snapshots) | Microsoft | 2020– | spec | toHaveScreenshot with thresholds and masks; visual ruler executor. | UI · verify | high |
| [pixelmatch](https://github.com/mapbox/pixelmatch) | Mapbox | 2015– | repo | Pixel diff with anti-aliasing; basis of the parity metric. | UI | medium |
| [WCAG 2.2](https://www.w3.org/TR/WCAG22/) | W3C | 2023 | spec | Contrast and accessibility criteria for the visual ruler. | UI · verify | medium |

---

## Benchmarks

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [SWE-bench](https://arxiv.org/abs/2310.06770) | Jimenez et al. | 2023 | benchmark | Real GitHub issues with tests; de facto standard. | benchmark | high |
| [Introducing SWE-bench Verified](https://openai.com/index/introducing-swe-bench-verified/) | OpenAI | 2024 | benchmark | Human-validated subset; lesson about underspecified tasks. **(blocks fetch: 403)** | benchmark | medium |
| [SWE-Bench Pro](https://arxiv.org/abs/2509.16941) | Deng et al. (Scale AI) | 2025 | benchmark | 1,865 long contamination-resistant tasks; Go among the languages. | benchmark | high |
| [SWE-bench Goes Live!](https://arxiv.org/abs/2505.23419) | Zhang et al. | 2025 | benchmark | Continuously renewed tasks against contamination. | benchmark | medium |
| [Terminal-Bench 2.0](https://arxiv.org/abs/2601.11868) | Merrill et al. | 2026 | benchmark | 89 terminal tasks with the Harbor harness; reusable task format. **(preprint)** | benchmark | high |
| [τ-bench](https://arxiv.org/abs/2406.12045) | Yao et al. | 2024 | benchmark | Introduces pass^k: reliability over k attempts, not just success on one. | benchmark | high |
| [Measuring AI Ability to Complete Long Software Tasks](https://arxiv.org/abs/2503.14499) | Kwa et al. (METR) | 2025 | paper | Time horizon at 50%; metric for long tasks (panel: metr.org/time-horizons). | benchmark | medium |
| [Impact of Early-2025 AI on Experienced OS Developer Productivity](https://arxiv.org/abs/2507.09089) | Becker et al. (METR) | 2025 | paper | RCT: developers 19% slower with AI thinking they were 20% faster. | benchmark | high |
| [AI Agents That Matter](https://arxiv.org/abs/2407.01502) | Kapoor et al. (Princeton) | 2024 | paper | Evaluate cost alongside accuracy (Pareto frontier) and avoid overfitting to benchmark. | benchmark | high |
| [Aider polyglot benchmark](https://aider.chat/2024/12/21/polyglot.html) | Aider | 2024 | benchmark | 225 exercises in 6 languages with published cost per run. | benchmark | medium |
| [Evaluating LLMs Trained on Code (pass@k)](https://arxiv.org/abs/2107.03374) | Chen et al. (OpenAI) | 2021 | paper | Unbiased pass@k estimator; use it, not the naive average. | benchmark | medium |

---

## Token efficiency

| Title | Author / Org. | Year | Type | Why it matters for toolbelt | Module | Priority |
|-------|---------------|------|------|-----------------------------|--------|----------|
| [Code execution with MCP](https://www.anthropic.com/research/code-execution-with-mcp) | Anthropic | 2025 | blog | Tools as code in files: 150k → 2k tokens (−98.7%) in the example. | protocol · mem/ctx | high |
| [Introducing advanced tool use](https://www.anthropic.com/research/advanced-tool-use) | Anthropic | 2025 | blog | Tool Search (defer_loading) and Programmatic Tool Calling. | protocol · mem/ctx | high |
| [LLMLingua](https://arxiv.org/abs/2310.05736) | Jiang et al. (Microsoft) | 2023 | paper | Prompt compression by perplexity; useful for logs and long outputs. | mem/ctx | medium |
| [LLMLingua-2](https://arxiv.org/abs/2403.12968) | Pan et al. (Microsoft) | 2024 | paper | Task-agnostic compression, faster and more faithful than v1. | mem/ctx | medium |
