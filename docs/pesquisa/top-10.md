🇧🇷 Português | [🇺🇸 English](../en/pesquisa/top-10.md)

# Top 10 Leituras para Começar

> Lista ordenada das 10 fontes primárias mais importantes para entender o contexto e as decisões de design do toolbelt.

1. **[Effective context engineering for AI agents](https://www.anthropic.com/research/effective-context-engineering-for-ai-agents)** (Anthropic, 2025)  
   O vocabulário de compaction, notas e contexto just-in-time pro `tb ctx`.

2. **[Code execution with MCP](https://www.anthropic.com/research/code-execution-with-mcp) + [Introducing advanced tool use](https://www.anthropic.com/research/advanced-tool-use)** (Anthropic, 2025)  
   Como o tb expõe ferramentas sem pagar o catálogo inteiro em tokens.

3. **[MCP Specification 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/changelog)** (MCP / AAIF, 2026)  
   O `tb mcp` deve nascer sobre o núcleo stateless e a extensão Tasks.

4. **[ImpossibleBench](https://arxiv.org/abs/2510.20270)** (Zhong, Raghunathan, Carlini, 2025)  
   Mede a taxa de trapaça do setup e lista as quatro estratégias que o guard precisa pegar.

5. **[SpecBench](https://arxiv.org/abs/2605.21384)** (Zhao et al., 2026)  
   Testes held-out compostos como régua do verify e o gap visível/oculto como KPI. **(preprint)**

6. **[Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778)** (Salle et al., 2026)  
   Por que restrições precisam viver fora do resumo. **(preprint)**

7. **[Effective harnesses for long-running agents](https://www.anthropic.com/research/effective-harnesses-for-long-running-agents)** (Anthropic, 2025)  
   O esqueleto de progress log + lista de features pro board.

8. **[Effective Strategies for Asynchronous SWE Agents (CAID)](https://arxiv.org/abs/2603.21489)** (Geng et al., 2026)  
   Evidência a favor de worktree + delegação central + merge testado. **(preprint)**

9. **[How Should Agents See Code?](https://arxiv.org/abs/2611.07286) + [Analyzing and Mitigating Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725)** (2026)  
   Quando o grafo economiza e quando encarece. **(preprints)**

10. **[Defeating Prompt Injections by Design (CaMeL)](https://arxiv.org/abs/2503.18813)** (Debenedetti et al., Google DeepMind, 2025)  
    Defesa contra prompt injection por desenho, não por prompt.
