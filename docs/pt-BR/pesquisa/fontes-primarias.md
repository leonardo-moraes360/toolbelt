[🇺🇸 English](../../pesquisa/fontes-primarias.md) | 🇧🇷 Português

# Fontes Primárias

> **Data da pesquisa**: 2026-11-15  
> **Total de fontes**: ~100 (papers, specs, blogs oficiais de engenharia, repos e benchmarks)

Esta é uma compilação de fontes primárias que podem influenciar o desenho do toolbelt. Priorizamos publicações de 2024–2026 e apenas fontes originais: o paper, a spec, o post de quem construiu ou o repositório oficial. Todos os links foram verificados (HTTP 200), exceto o post do SWE-bench Verified da OpenAI, que bloqueia acesso automatizado (403) mas existe.

**Nota sobre preprints de 2026**: CAID, Claim Plane, Compact-Bench, SpecBench, TRACE e os estudos de grafo e custo são preprints recentes, sem revisão por pares confirmada. Os números citados devem ser tratados como indicativos.

---

## Índice por Área

- [Arquitetura multiagente](#arquitetura-multiagente)
- [Contexto e memória](#contexto-e-memória)
- [Grafo / busca no repo](#grafo--busca-no-repo)
- [Protocolos](#protocolos)
- [Sandbox e segurança](#sandbox-e-segurança)
- [Reward hacking](#reward-hacking)
- [Verificação](#verificação)
- [UI / visual](#ui--visual)
- [Benchmarks](#benchmarks)
- [Eficiência de tokens](#eficiência-de-tokens)

---

## Arquitetura multiagente

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [Building effective AI agents](https://www.anthropic.com/research/building-effective-agents) | Anthropic | 2024 | blog | Catálogo de padrões (routing, orchestrator-workers, evaluator-optimizer) que mapeia direto em router e verify. | router · verify | alta |
| [How we built our multi-agent research system](https://www.anthropic.com/research/multi-agent-research-system) | Anthropic | 2025 | blog | Lead + subagentes: +90,2% no eval interno, mas ~15× os tokens de um chat; tokens explicam 80% da variância. | router · benchmark | alta |
| [Effective harnesses for long-running agents](https://www.anthropic.com/research/effective-harnesses-for-long-running-agents) | Anthropic | 2025 | blog | Agente inicializador, init.sh, progress log e lista de features em JSON; uma feature por sessão, marcada só após teste e2e. | router · mem/ctx · verify | alta |
| [Don't Build Multi-Agents](https://cognition.com/blog/dont-build-multi-agents) | Cognition | 2025 | blog | Contraponto: subagentes sem contexto compartilhado tomam decisões implícitas conflitantes. | router · review/drift | alta |
| [Why Do Multi-Agent LLM Systems Fail? (MAST)](https://arxiv.org/abs/2503.13657) | Cemri et al. (UC Berkeley) | 2025 | paper | Taxonomia de falhas (especificação, desalinhamento, verificação) que vira checklist do watchdog. | router · benchmark | alta |
| [Effective Strategies for Asynchronous Software Engineering Agents (CAID)](https://arxiv.org/abs/2603.21489) | Geng et al. | 2026 | paper | Delegação central + worktrees isolados + merge com testes: +25,6 pts no PaperBench e +14,7 no Commit0. **(preprint)** | router · sandbox | alta |
| [Claim Plane: Enforceable Change Intents for Parallel Coding Agents](https://arxiv.org/abs/2607.21909) | Nikolaev | 2026 | paper | Worker declara ChangeIntent (commit base, recursos, deps) antes de escrever; admissão determinística evita colisão. **(preprint)** | router · guard | média |
| [dsh-superagent-prototype](https://github.com/deepseek-ai/dsh-superagent-prototype) | DeepSeek | 2026 | repo | Protótipo do router/workers com watchdog e entrega direta; base do ADR 009. | router | alta |
| [elephant-goldfish-model](https://github.com/google/elephant-goldfish-model) | Google | 2026 | repo | Redator com contexto + revisores de contexto limpo; base do tb review / tb drift. | review/drift | média |
| [Magentic-One](https://arxiv.org/abs/2411.04468) | Fourney et al. (Microsoft) | 2024 | paper | Orquestrador com Task Ledger e Progress Ledger que replaneja quando trava. | router | média |
| [MetaGPT](https://arxiv.org/abs/2308.00352) | Hong et al. | 2023 | paper | Papéis com SOPs e artefatos estruturados entre eles; referência pra personas. | router | média |
| [AutoGen](https://arxiv.org/abs/2308.08155) | Wu et al. (Microsoft) | 2023 | paper | Conversa entre agentes como primitiva; útil como baseline do que evitar em tokens. | router | baixa |
| [ChatDev](https://arxiv.org/abs/2307.07924) | Qian et al. | 2023 | paper | Cadeia de papéis de empresa de software; referência histórica de personas. | router | baixa |
| [OpenAI Agents SDK](https://github.com/openai/openai-agents-python) | OpenAI | 2025 | repo | Handoffs, guardrails e tracing; sucessor do Swarm (github.com/openai/swarm). | router · guard | média |
| [LangGraph](https://github.com/langchain-ai/langgraph) | LangChain | 2023– | repo | Máquina de estados com checkpoints duráveis e retomada; modelo pro board. | router | média |
| [OpenHands](https://arxiv.org/abs/2407.16741) | Wang et al. | 2024 | paper | Plataforma aberta com sandbox Docker e event stream; referência de runtime. | sandbox · benchmark | média |
| [Executable Code Actions Elicit Better LLM Agents (CodeAct)](https://arxiv.org/abs/2402.01030) | Wang et al. | 2024 | paper | Ações como código em vez de JSON: menos turnos e mais composição. | protocolo · mem/ctx | média |
| [An LLM Compiler for Parallel Function Calling](https://arxiv.org/abs/2312.04511) | Kim et al. | 2023 | paper | Planeja chamadas de ferramenta como DAG e executa em paralelo; menos latência. | router | média |
| [paperclip](https://github.com/paperclipai/paperclip) | Paperclip AI | 2026 | repo | Plano de controle para times de agentes: watchdog que revisa uma árvore de trabalho parada uma vez por fingerprint de parada, estado de execução seguro entre releases, guarda de prompt injection em camadas, preset de baixa confiança. Revisão do tb em [paperclip.md](paperclip.md). | bus · router · egm | alta |
| [alethe-orchestrate-skill](https://github.com/Kc1t/alethe-orchestrate-skill) · [alethe-agents](https://github.com/Kc1t/alethe-agents) | Kc1t | 2026 | repo | Roteamento por tier + profile, playbooks YAML (cascade, review-gate, best-of-n), `outcome.json` por worker, quota guard local; mapa dos eventos stream-json do `claude -p`. Revisão do tb em [alethe-cma.md](alethe-cma.md). | router · bus · bench | alta |
| [CMA: plan big, execute small](https://github.com/anthropics/claude-cookbooks/blob/main/managed_agents/CMA_plan_big_execute_small.ipynb) | Anthropic | 2026 | notebook | Orquestrador forte sem tools, workers baratos: 84–98% do input cobrado na tarifa do worker; método de rigor equivalente para comparar solo vs. delegado. Revisão do tb em [alethe-cma.md](alethe-cma.md). | router · bench | média |

---

## Contexto e memória

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [Effective context engineering for AI agents](https://www.anthropic.com/research/effective-context-engineering-for-ai-agents) | Anthropic | 2025 | blog | Compaction, notas estruturadas, contexto just-in-time e subagentes: o manual do tb ctx. | mem/ctx | alta |
| [Context Rot](https://www.trychroma.com/research/context-rot) | Chroma | 2025 | paper | Desempenho cai com o tamanho do input em 18 modelos, mesmo em tarefas simples. | mem/ctx · benchmark | alta |
| [Lost in the Middle](https://arxiv.org/abs/2307.03172) | Liu et al. | 2023 | paper | Informação no meio do contexto é a mais esquecida; orienta a ordem de montagem. | mem/ctx | média |
| [Compact-Bench](https://arxiv.org/abs/2610.31710) | Sun et al. | 2026 | paper | Resumos LLM retêm no máximo 16% dos detalhes recorrentes; extração mecânica retém 79,1%. **(preprint)** | mem/ctx · benchmark | alta |
| [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) | Salle et al. | 2026 | paper | O 1º resumo já perde cerca de 1/3 das regras anti-trapaça; regras de "não faça" caem ~6× mais. **(preprint)** | mem/ctx · guard | alta |
| [MemGPT](https://arxiv.org/abs/2310.08560) | Packer et al. | 2023 | paper | Memória hierárquica estilo SO (in-context vs arquivada) com paginação. | mem/ctx | média |
| [Generative Agents](https://arxiv.org/abs/2304.03442) | Park et al. | 2023 | paper | Recuperação por recência, importância e relevância, mais reflexão periódica. | mem/ctx | média |
| [A-MEM: Agentic Memory for LLM Agents](https://arxiv.org/abs/2502.12110) | Xu et al. | 2025 | paper | Notas estilo Zettelkasten com links que evoluem; casa com o vault Obsidian. | mem/ctx · grafo | alta |
| [Zep: A Temporal Knowledge Graph Architecture for Agent Memory](https://arxiv.org/abs/2501.13956) | Rasmussen et al. | 2025 | paper | Arestas com validade temporal: invalida fatos em vez de sobrescrever (código: getzep/graphiti). | mem/ctx · grafo | alta |
| [Mem0](https://arxiv.org/abs/2504.19413) | Chhikara et al. | 2025 | paper | Pipeline de extração e atualização de memória (adicionar, atualizar, apagar). | mem/ctx | média |
| [Agent Workflow Memory](https://arxiv.org/abs/2409.07429) | Wang et al. | 2024 | paper | Induz workflows reutilizáveis a partir de trajetórias bem-sucedidas. | mem/ctx | média |
| [Equipping agents with Agent Skills](https://www.anthropic.com/research/equipping-agents-for-the-real-world-with-agent-skills) | Anthropic | 2025 | blog | Divulgação progressiva: só nome e descrição no contexto, corpo carregado sob demanda. | mem/ctx · protocolo | alta |
| [Dynamic context discovery](https://cursor.com/blog/dynamic-context-discovery) | Cursor | 2026 | blog | Arquivos como interface de contexto; −46,9% de tokens nas execuções que usam MCP. | mem/ctx | alta |
| [Prompt caching (Claude)](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) | Anthropic | 2024– | spec | Regras de prefixo e breakpoints de cache; define a ordem estável do contexto. | mem/ctx · benchmark | alta |
| [Prompt caching (OpenAI)](https://developers.openai.com/api/docs/guides/prompt-caching) | OpenAI | 2024– | spec | Cache automático por prefixo; mesmo princípio de prefixo estável. | mem/ctx | média |
| [AGENTS.md](https://agents.md/) | AAIF / Linux Foundation | 2025 | spec | Formato aberto de instruções por projeto; contexto local padrão. | mem/ctx · protocolo | alta |

---

## Grafo / busca no repo

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [RepoGraph](https://arxiv.org/abs/2410.14684) | Ouyang et al. | 2024 | paper | Grafo de código em nível de linha plugável em agentes de SWE-bench. | grafo | alta |
| [LocAgent](https://arxiv.org/abs/2503.09089) | Chen et al. | 2025 | paper | Grafo heterogêneo (contém, importa, chama, herda) com ferramentas de busca pra localização. | grafo | alta |
| [CodexGraph](https://arxiv.org/abs/2408.03910) | Liu et al. | 2024 | paper | Agente consulta um banco de grafo de código por queries estruturadas. | grafo | média |
| [Building a better repository map with tree sitter](https://aider.chat/2023/10/22/repomap.html) | Aider | 2023 | blog | Repo map com tree-sitter + ranking de grafo sob orçamento de tokens. | grafo · mem/ctx | alta |
| [SWE-agent: Agent-Computer Interfaces](https://arxiv.org/abs/2405.15793) | Yang et al. | 2024 | paper | Interface de ferramentas desenhada pro agente (visualizador, busca, lint no edit). | protocolo · grafo | alta |
| [Agentless](https://arxiv.org/abs/2407.01489) | Xia et al. | 2024 | paper | Localizar → reparar → validar sem loop agentic; baseline barato pro benchmark. | benchmark · router | média |
| [From Local to Global: GraphRAG](https://arxiv.org/abs/2404.16130) | Edge et al. (Microsoft) | 2024 | paper | Comunidades e sumários hierárquicos pra perguntas globais sobre o corpus. | grafo | média |
| [LightRAG](https://arxiv.org/abs/2410.05779) | Guo et al. | 2024 | paper | Grafo + vetor em dois níveis com atualização incremental. | grafo | média |
| [Improving agent with semantic search](https://cursor.com/blog/semsearch) | Cursor | 2025 | blog | Busca semântica dá 12,5% mais acurácia em média que só grep. | grafo | alta |
| [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) | Sepidband et al. | 2026 | paper | Mapas (AST, call graph) cortam 17–39% de tokens quando localizar é o gargalo, mas sobem a latência. **(preprint)** | grafo · benchmark | alta |
| [Analyzing and Mitigating Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) | Hu et al. | 2026 | paper | Busca estrutural pode encarecer até 28,14%; skills escritas por humanos cortam até 41,73%. **(preprint)** | grafo · benchmark | alta |
| [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) | Tree-sitter | 2018– | spec | Parser incremental multi-linguagem com bindings em Go para a extração do grafo. | grafo | média |
| [SQLite FTS5](https://www.sqlite.org/fts5.html) | SQLite | 2015– | spec | Índice full-text com BM25 e tokenizers; o motor de busca do tb. | grafo | média |
| [Graphify](https://github.com/Graphify-Labs/graphify) | Graphify-Labs | 2026 | repo | Grafo consultável de código, docs e schemas; inspiração já adotada. | grafo | média |

---

## Protocolos

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [MCP Specification 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/index) | MCP / AAIF | 2026 | spec | Núcleo stateless, server/discover, listas cacheáveis com ordem determinística, Tasks como extensão. | protocolo | alta |
| [MCP Security Best Practices](https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices) | MCP | 2025– | spec | Confused deputy, passagem de token, SSRF e escopos mínimos. | protocolo · guard | alta |
| [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) | Linux Foundation | 2025– | spec | Agent Card, tarefas, streaming e bindings JSON-RPC/gRPC/REST pra agente↔agente. | protocolo · router | alta |
| [Agent Client Protocol (ACP)](https://agentclientprotocol.com/protocol/v1/overview) | Zed | 2025 | spec | JSON-RPC por stdio entre editor e agente, com pedidos de permissão; ponte pro leitor humano. | protocolo · UI | alta |
| [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification) | JSON-RPC WG | 2010 | spec | Base de MCP, A2A e ACP; o transporte do tb. | protocolo | média |
| [Writing effective tools for AI agents](https://www.anthropic.com/research/writing-tools-for-agents) | Anthropic | 2025 | blog | Namespacing e response_format concise/detailed (~1/3 dos tokens no exemplo). | protocolo · mem/ctx | alta |
| [is-it-legit-mcp](https://github.com/m8ven/is-it-legit-mcp) · [M8ven Verified](https://m8ven.ai/verified/how-we-verify) | M8ven | 2026 | repo | Referência de checagem de servidor MCP: anotações (readOnly/destructive/idempotent/openWorld) justificadas pelo efeito real do handler, verificação de código vs. declaração, sandbox, supply chain e publisher. Revisão do tb em [mcp-sinais-m8ven.md](mcp-sinais-m8ven.md). | protocolo · guard | alta |

---

## Sandbox e segurança

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [Landlock: unprivileged access control](https://docs.kernel.org/userspace-api/landlock.html) | Linux kernel | 2021– | spec | Restrição de FS e rede sem root; camada principal do sandbox. | sandbox | alta |
| [Seccomp BPF](https://docs.kernel.org/userspace-api/seccomp_filter.html) | Linux kernel | 2012– | spec | Filtro de syscalls para os processos do worker. | sandbox | média |
| [bubblewrap](https://github.com/containers/bubblewrap) | containers | 2016– | repo | Sandbox sem privilégio por namespaces; o mesmo usado pelo Claude Code no Linux. | sandbox | alta |
| [Making Claude Code safer with sandboxing](https://www.anthropic.com/research/claude-code-sandboxing) | Anthropic | 2025 | blog | Isolamento de FS + proxy de rede; −84% de pedidos de permissão no uso interno. | sandbox · guard | alta |
| [gVisor](https://gvisor.dev/docs/) | Google | 2018– | spec | Kernel em user-space pra isolamento mais forte que namespaces. | sandbox | média |
| [Firecracker](https://github.com/firecracker-microvm/firecracker) | AWS | 2017– | repo | MicroVMs pra workers remotos ou não confiáveis. | sandbox | baixa |
| [git-worktree](https://git-scm.com/docs/git-worktree) | Git | 2015– | spec | Uma árvore de trabalho por agente; base do isolamento de edição. | sandbox · router | média |
| [Defeating Prompt Injections by Design (CaMeL)](https://arxiv.org/abs/2503.18813) | Debenedetti et al. (Google DeepMind) | 2025 | paper | Separa fluxo de controle e de dados, com capabilities por valor. | guard · sandbox | alta |
| [Design Patterns for Securing LLM Agents against Prompt Injections](https://arxiv.org/abs/2506.08837) | Beurer-Kellner et al. | 2025 | paper | Seis padrões (plan-then-execute, dual LLM, map-reduce…) aplicáveis ao router. | guard · router | alta |
| [The Dual LLM pattern](https://simonwillison.net/2023/Apr/25/dual-llm-pattern/) | Simon Willison | 2023 | blog | LLM privilegiado nunca vê conteúdo não confiável; LLM em quarentena sem ferramentas. | guard | média |
| [The lethal trifecta for AI agents](https://simonwillison.net/2025/Jun/16/the-lethal-trifecta/) | Simon Willison | 2025 | blog | Dado privado + conteúdo não confiável + saída externa = exfiltração; regra simples pro mandate. | guard | alta |
| [OWASP Top 10 for LLM Applications](https://genai.owasp.org/llm-top-10/) | OWASP | 2025 | spec | Lista de riscos (injeção, agência excessiva…) pro modelo de ameaças. | guard | média |
| [Agentic AI – Threats and Mitigations](https://genai.owasp.org/resource/agentic-ai-threats-and-mitigations/) | OWASP | 2025 | spec | Ameaças específicas de agentes (memória envenenada, abuso de ferramentas). | guard | média |
| [AgentDojo](https://arxiv.org/abs/2406.13352) | Debenedetti et al. | 2024 | benchmark | Ambiente pra medir ataques e defesas de prompt injection em agentes. | benchmark · guard | média |

---

## Reward hacking

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [Recent Frontier Models Are Reward Hacking](https://metr.org/blog/2025-06-05-recent-reward-hacking/) | METR | 2025 | blog | o3 fez reward hacking em 39 de 128 runs no RE-Bench (30,4%), mexendo em código de pontuação. | guard · benchmark | alta |
| [ImpossibleBench](https://arxiv.org/abs/2510.20270) | Zhong, Raghunathan, Carlini | 2025 | benchmark | Tarefas com testes que contradizem a spec: qualquer pass é trapaça (código: safety-research/impossiblebench). | guard · benchmark | alta |
| [SpecBench](https://arxiv.org/abs/2605.21384) | Zhao et al. | 2026 | benchmark | Gap entre testes visíveis e held-out cresce 28 pts a cada 10× de código. **(preprint)** | verify · guard | alta |
| [EvilGenie](https://arxiv.org/abs/2511.21654) | Gabor et al. | 2025 | benchmark | Juiz LLM detecta bem hacks óbvios; testes held-out ajudam pouco. | guard · verify | alta |
| [TRACE: Reward Hack Detection via Contrastive Analysis](https://arxiv.org/abs/2601.20103) | Deshpande et al. | 2026 | benchmark | Taxonomia de 54 categorias de exploits; vira lista de padrões do guard. **(preprint)** | guard | média |
| [Monitoring Reasoning Models for Misbehavior](https://arxiv.org/abs/2503.11926) | Baker et al. (OpenAI) | 2025 | paper | Monitor de CoT pega reward hacking; penalizar o CoT gera ofuscação. | guard | alta |
| [Natural Emergent Misalignment from Reward Hacking](https://arxiv.org/abs/2511.18397) | MacDiarmid et al. (Anthropic) | 2025 | paper | Reward hacking em código de produção generaliza pra sabotagem; prevenir importa. | guard | média |
| [SHADE-Arena](https://arxiv.org/abs/2506.15740) | Kutasov et al. (Anthropic) | 2025 | benchmark | Monitor dá nota de suspeita 0–100 à trajetória; desenho pro monitor do guard. | guard · benchmark | média |
| [AI Control: Improving Safety Despite Intentional Subversion](https://arxiv.org/abs/2312.06942) | Greenblatt et al. | 2023 | paper | Modelo confiável e mais fraco monitora o mais forte; base do discriminador barato. | guard · verify | alta |

---

## Verificação

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [Large Language Models Cannot Self-Correct Reasoning Yet](https://arxiv.org/abs/2310.01798) | Huang et al. | 2023 | paper | Sem sinal externo, autocorreção não melhora; justifica o verificador adversarial. | verify | alta |
| [Judging LLM-as-a-Judge (MT-Bench)](https://arxiv.org/abs/2306.05685) | Zheng et al. | 2023 | paper | Vieses de posição, verbosidade e autopreferência em juízes LLM. | verify · review/drift | alta |
| [Large Language Models are not Fair Evaluators](https://arxiv.org/abs/2305.17926) | Wang et al. | 2023 | paper | Trocar a ordem das respostas e calibrar reduz viés de posição. | verify | média |
| [Agent-as-a-Judge](https://arxiv.org/abs/2410.10934) | Zhuge et al. | 2024 | paper | Agente avalia a trajetória inteira contra requisitos (DevAI). | verify · benchmark | média |
| [Improving Factuality and Reasoning through Multiagent Debate](https://arxiv.org/abs/2305.14325) | Du et al. | 2023 | paper | Debate entre instâncias melhora respostas; caro em tokens. | verify | baixa |
| [Training Verifiers to Solve Math Word Problems](https://arxiv.org/abs/2110.14168) | Cobbe et al. (OpenAI) | 2021 | paper | Gerador + verificador treinado com best-of-n; origem do padrão. | verify | média |
| [CodeT: Code Generation with Generated Tests](https://arxiv.org/abs/2207.10397) | Chen et al. (Microsoft) | 2022 | paper | Testes gerados + consenso duplo escolhem a solução; régua sem ver a implementação. | verify | média |
| [Mutation-Guided LLM-based Test Generation at Meta (ACH)](https://arxiv.org/abs/2501.12862) | Foster et al. (Meta) | 2025 | paper | LLM gera mutantes e testes que os matam; mede se os testes pegam bugs. | verify | alta |
| [Agentic Property-Based Testing](https://arxiv.org/abs/2510.09907) | Maaz et al. | 2025 | paper | Agente infere propriedades e escreve testes Hypothesis; achou bugs reais em 100 pacotes. | verify | média |

---

## UI / visual

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [Design2Code](https://arxiv.org/abs/2403.03163) | Si et al. (Stanford) | 2024 | benchmark | Métricas visuais por bloco, texto, posição, cor e CLIP, além de pixel. | UI · verify | alta |
| [WebSight dataset](https://arxiv.org/abs/2403.09029) | Laurençon et al. (Hugging Face) | 2024 | benchmark | 2M pares screenshot→HTML; fonte de casos sintéticos pra régua visual. | UI | baixa |
| [UICoder](https://arxiv.org/abs/2406.07739) | Wu et al. (Apple) | 2024 | paper | Filtros automáticos (compilação + CLIP) como feedback pra gerar UI. | UI · verify | média |
| [Playwright visual comparisons](https://playwright.dev/docs/test-snapshots) | Microsoft | 2020– | spec | toHaveScreenshot com limiares e máscaras; executor da régua visual. | UI · verify | alta |
| [pixelmatch](https://github.com/mapbox/pixelmatch) | Mapbox | 2015– | repo | Diff por pixel com anti-aliasing; base da métrica de paridade. | UI | média |
| [WCAG 2.2](https://www.w3.org/TR/WCAG22/) | W3C | 2023 | spec | Critérios de contraste e acessibilidade da régua visual. | UI · verify | média |

---

## Benchmarks

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [SWE-bench](https://arxiv.org/abs/2310.06770) | Jimenez et al. | 2023 | benchmark | Issues reais do GitHub com testes; padrão de facto. | benchmark | alta |
| [Introducing SWE-bench Verified](https://openai.com/index/introducing-swe-bench-verified/) | OpenAI | 2024 | benchmark | Subconjunto validado por humanos; lição sobre tarefas mal especificadas. **(bloqueia fetch: 403)** | benchmark | média |
| [SWE-Bench Pro](https://arxiv.org/abs/2509.16941) | Deng et al. (Scale AI) | 2025 | benchmark | 1.865 tarefas longas e resistentes a contaminação; Go entre as linguagens. | benchmark | alta |
| [SWE-bench Goes Live!](https://arxiv.org/abs/2505.23419) | Zhang et al. | 2025 | benchmark | Tarefas renovadas continuamente contra contaminação. | benchmark | média |
| [Terminal-Bench 2.0](https://arxiv.org/abs/2601.11868) | Merrill et al. | 2026 | benchmark | 89 tarefas de terminal com o harness Harbor; formato de tarefa reaproveitável. **(preprint)** | benchmark | alta |
| [τ-bench](https://arxiv.org/abs/2406.12045) | Yao et al. | 2024 | benchmark | Introduz pass^k: confiabilidade em k tentativas, não só sucesso em uma. | benchmark | alta |
| [Measuring AI Ability to Complete Long Software Tasks](https://arxiv.org/abs/2503.14499) | Kwa et al. (METR) | 2025 | paper | Horizonte de tempo a 50%; métrica pra tarefas longas (painel: metr.org/time-horizons). | benchmark | média |
| [Impact of Early-2025 AI on Experienced OS Developer Productivity](https://arxiv.org/abs/2507.09089) | Becker et al. (METR) | 2025 | paper | RCT: devs 19% mais lentos com IA achando que foram 20% mais rápidos. | benchmark | alta |
| [AI Agents That Matter](https://arxiv.org/abs/2407.01502) | Kapoor et al. (Princeton) | 2024 | paper | Avaliar custo junto com acurácia (fronteira de Pareto) e evitar overfitting a benchmark. | benchmark | alta |
| [Aider polyglot benchmark](https://aider.chat/2024/12/21/polyglot.html) | Aider | 2024 | benchmark | 225 exercícios em 6 linguagens com custo por execução publicado. | benchmark | média |
| [Evaluating LLMs Trained on Code (pass@k)](https://arxiv.org/abs/2107.03374) | Chen et al. (OpenAI) | 2021 | paper | Estimador sem viés de pass@k; use-o, não a média ingênua. | benchmark | média |

---

## Eficiência de tokens

| Título | Autor / Org. | Ano | Tipo | Por que importa pro toolbelt | Módulo | Prioridade |
|--------|--------------|-----|------|------------------------------|--------|------------|
| [Code execution with MCP](https://www.anthropic.com/research/code-execution-with-mcp) | Anthropic | 2025 | blog | Ferramentas como código em arquivos: 150 mil → 2 mil tokens (−98,7%) no exemplo. | protocolo · mem/ctx | alta |
| [Introducing advanced tool use](https://www.anthropic.com/research/advanced-tool-use) | Anthropic | 2025 | blog | Tool Search (defer_loading) e Programmatic Tool Calling. | protocolo · mem/ctx | alta |
| [LLMLingua](https://arxiv.org/abs/2310.05736) | Jiang et al. (Microsoft) | 2023 | paper | Compressão de prompt por perplexidade; útil pra logs e saídas longas. | mem/ctx | média |
| [LLMLingua-2](https://arxiv.org/abs/2403.12968) | Pan et al. (Microsoft) | 2024 | paper | Compressão agnóstica de tarefa, mais rápida e fiel que a v1. | mem/ctx | média |
