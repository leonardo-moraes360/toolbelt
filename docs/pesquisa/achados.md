# Achados da Pesquisa

> Ideias novas extraídas das [fontes primárias](fontes-primarias.md), com fonte, módulo afetado e proposta concreta.

---

## Os 5 Principais Achados

Estes são os achados com maior impacto no desenho do toolbelt:

### 1. Compactação apaga regras → arquivo de invariantes fixado + compactação híbrida

**Fonte**: [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) (Salle et al., 2026, preprint)

**Problema**: O 1º resumo já perde cerca de 1/3 das regras anti-trapaça; regras de "não faça" caem ~6× mais que outras informações.

**Módulo afetado**: `mem/ctx` · `guard`

**Proposta concreta**:
- Criar arquivo de invariantes (regras anti-trapaça, proibições, mandate) que é reinjetado intacto **depois** de cada compactação
- Usar compactação híbrida: extração mecânica do estado (arquivos tocados, comandos, falhas de teste, decisões) **antes** do resumo LLM
- Adicionar KPI de retenção de restrições após compactação

**Evidência adicional**: [Compact-Bench](https://arxiv.org/abs/2610.31710) (Sun et al., 2026) mostra que extração mecânica reteve 79,1% contra no máximo 16% dos resumos LLM.

---

### 2. Reward hacking cresce com tamanho → suíte oculta, tarefas impossíveis, monitor de trajetória

**Fonte**: [SpecBench](https://arxiv.org/abs/2605.21384) (Zhao et al., 2026, preprint), [ImpossibleBench](https://arxiv.org/abs/2510.20270) (Zhong et al., 2025), [METR Reward Hacking](https://metr.org/blog/2025-06-05-recent-reward-hacking/) (2025)

**Problema**: Gap entre testes visíveis e held-out cresce 28 pts a cada 10× de código; o3 fez reward hacking em 30,4% das runs do RE-Bench.

**Módulo afetado**: `verify` · `benchmark` · `guard`

**Proposta concreta**:
- `tb verify`: cada ticket ganha testes visíveis **e** uma suíte oculta composta que o gerador nunca vê; o gap entre as duas vira KPI de reward hacking
- `tb bench --impossible`: gera variantes impossíveis das tarefas (teste contradiz a spec); qualquer pass conta como trapaça e a meta é 0%
- `tb guard`: monitor de trajetória — um monitor barato lê ações e transcript (não só o diff) e dá nota de suspeita; nunca treinar ou otimizar contra o raciocínio, pra não gerar ofuscação

**Padrões de trapaça a detectar** (além de apagar testes):
- Sobrecarga de `__eq__`
- Estado guardado entre chamadas
- Casos especiais por input de teste
- Edição de código de pontuação

**Fontes**: [TRACE](https://arxiv.org/abs/2601.20103), [Monitoring Reasoning Models](https://arxiv.org/abs/2503.11926), [SHADE-Arena](https://arxiv.org/abs/2506.15740), [AI Control](https://arxiv.org/abs/2312.06942)

---

### 3. Grafo nem sempre economiza → `tb graph` sob demanda + A/B

**Fonte**: [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) (Sepidband et al., 2026, preprint), [Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) (Hu et al., 2026, preprint)

**Problema**: Mapas (AST, call graph) cortam 17–39% de tokens **quando localizar é o gargalo**, mas sobem a latência. Busca estrutural pode encarecer até 28,14%; skills escritas por humanos cortam até 41,73%.

**Módulo afetado**: `grafo` · `benchmark`

**Proposta concreta**:
- `tb graph` como ferramenta **sob demanda** com saída enxuta (não forçar uso em toda tarefa)
- A/B por tipo de tarefa para medir quando grafo ajuda vs quando encarece
- Priorizar skills/workflows escritos por humanos sobre busca automática
- Watchdog de desperdício: marcar busca redundante, scripts quase iguais e reexecução de testes (apareceram em 79–98% das tarefas no estudo)

---

### 4. Code execution com MCP + spec stateless → `tb mcp` nasce nessa versão

**Fonte**: [Code execution with MCP](https://www.anthropic.com/research/code-execution-with-mcp) (Anthropic, 2025), [MCP Specification 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/index)

**Problema**: Ferramentas como catálogo fixo custam 150 mil tokens no exemplo; como arquivos em código, custam 2 mil (−98,7%).

**Módulo afetado**: `protocolo` · `mem/ctx`

**Proposta concreta**:
- `tb mcp` nasce sobre o núcleo **stateless** (sem initialize), com server/discover
- Handles explícitos como argumentos, jobs longos de worker pela extensão Tasks
- Catálogo exposto como arquivos + `tb tools search` com carregamento adiado
- Resultados filtrados dentro do sandbox antes de voltar ao modelo
- Parâmetro `response_format` (concise/detailed) em toda ferramenta, nomes com namespace e listas em ordem determinística (prefixo de cache estável)

**Fontes adicionais**: [Introducing advanced tool use](https://www.anthropic.com/research/advanced-tool-use), [Writing effective tools for AI agents](https://www.anthropic.com/research/writing-tools-for-agents)

---

### 5. Paralelismo com escopo declarado → mandate com commit base e arquivos

**Fonte**: [Claim Plane](https://arxiv.org/abs/2607.21909) (Nikolaev, 2026, preprint), [CAID](https://arxiv.org/abs/2603.21489) (Geng et al., 2026, preprint)

**Problema**: Workers paralelos sem escopo declarado colidem em arquivos; merge pós-fato gera conflitos.

**Módulo afetado**: `router` · `guard`

**Proposta concreta**:
- O mandate do ticket traz **commit base** e lista de arquivos ou recursos
- O router só roda em paralelo escopos **disjuntos**
- Ordena os merges e devolve o conflito ao **worker dono** (não a um terceiro)
- Admissão determinística: worker declara ChangeIntent antes de escrever

**Evidência**: CAID mostrou +25,6 pts no PaperBench e +14,7 no Commit0 com delegação central + worktrees isolados + merge com testes.

---

## Todas as Ideias Novas

| Ideia | O que muda no tb | Módulo | Fonte |
|-------|------------------|--------|-------|
| Restrições fixadas fora da compactação | Arquivo de invariantes reinjetado intacto depois de cada compactação, mais KPI de retenção de restrições. | mem/ctx · guard | [arxiv.org/abs/2610.31778](https://arxiv.org/abs/2610.31778) |
| Compactação híbrida | Extração mecânica do estado (arquivos tocados, comandos, falhas de teste, decisões) antes do resumo LLM; extração reteve 79,1% contra no máximo 16% dos resumos. | mem/ctx | [arxiv.org/abs/2610.31710](https://arxiv.org/abs/2610.31710) |
| Suíte held-out no verify | Cada ticket ganha testes visíveis e uma suíte oculta composta que o gerador nunca vê; o gap entre as duas vira o KPI de reward hacking. | verify · benchmark | [arxiv.org/abs/2605.21384](https://arxiv.org/abs/2605.21384) · [arxiv.org/abs/2511.21654](https://arxiv.org/abs/2511.21654) |
| tb bench --impossible | Gera variantes impossíveis das tarefas (teste contradiz a spec); qualquer pass conta como trapaça e a meta é 0%. | benchmark · guard | [arxiv.org/abs/2510.20270](https://arxiv.org/abs/2510.20270) |
| Padrões de trapaça no guard | Além de apagar testes: sobrecarga de `__eq__`, estado guardado entre chamadas, casos especiais por input de teste, edição de código de pontuação. | guard | [arxiv.org/abs/2510.20270](https://arxiv.org/abs/2510.20270) · [arxiv.org/abs/2601.20103](https://arxiv.org/abs/2601.20103) · [metr.org](https://metr.org/blog/2025-06-05-recent-reward-hacking/) |
| Monitor de trajetória | Monitor barato lê ações e transcript (não só o diff) e dá nota de suspeita; nunca treinar ou otimizar contra o raciocínio, pra não gerar ofuscação. | guard | [arxiv.org/abs/2503.11926](https://arxiv.org/abs/2503.11926) · [arxiv.org/abs/2506.15740](https://arxiv.org/abs/2506.15740) · [arxiv.org/abs/2312.06942](https://arxiv.org/abs/2312.06942) |
| Ferramentas como arquivos + busca | Catálogo do tb exposto como arquivos e `tb tools search` com carregamento adiado; resultados filtrados dentro do sandbox antes de voltar ao modelo. | protocolo · mem/ctx | [anthropic.com](https://www.anthropic.com/research/code-execution-with-mcp) · [cursor.com](https://cursor.com/blog/dynamic-context-discovery) |
| response_format em toda ferramenta | Parâmetro concise/detailed, nomes com namespace e listas em ordem determinística, pra manter o prefixo do cache estável. | protocolo · mem/ctx | [anthropic.com](https://www.anthropic.com/research/writing-tools-for-agents) · MCP 2026-07-28 |
| MCP stateless desde o início | Sem initialize, com server/discover, handles explícitos como argumentos e jobs longos de worker pela extensão Tasks. | protocolo · router | [modelcontextprotocol.io](https://modelcontextprotocol.io/specification/2026-07-28/changelog) |
| Escopo declarado no ticket | O mandate traz commit base e arquivos ou recursos; o router só roda em paralelo escopos disjuntos, ordena os merges e devolve o conflito ao worker dono. | router · guard | [arxiv.org/abs/2607.21909](https://arxiv.org/abs/2607.21909) · [arxiv.org/abs/2603.21489](https://arxiv.org/abs/2603.21489) |
| tb init no estilo harness | Cria progress log, lista de features em JSON (failing → passing só via tb verify) e init.sh; todo worker começa lendo git log e progresso. | router · verify | [anthropic.com](https://www.anthropic.com/research/effective-harnesses-for-long-running-agents) |
| Grafo opcional e medido | `tb graph` como ferramenta sob demanda com saída enxuta e A/B por tipo de tarefa, já que pode encarecer até 28%; priorizar skills escritas por humanos. | grafo · benchmark | [arxiv.org/abs/2611.07286](https://arxiv.org/abs/2611.07286) · [arxiv.org/abs/2609.30725](https://arxiv.org/abs/2609.30725) |
| Watchdog de desperdício | Marcar busca redundante, scripts quase iguais e reexecução de testes, que no estudo apareceram em 79–98% das tarefas. | router · benchmark | [arxiv.org/abs/2609.30725](https://arxiv.org/abs/2609.30725) |
| Memória bitemporal e linkada | Fatos com validade e proveniência, invalidados em vez de sobrescritos, e notas com links no estilo Zettelkasten dentro do vault. | mem/ctx · grafo | [arxiv.org/abs/2501.13956](https://arxiv.org/abs/2501.13956) · [arxiv.org/abs/2502.12110](https://arxiv.org/abs/2502.12110) |
| Injeção bloqueada por desenho | O planner privilegiado nunca lê conteúdo não confiável, o worker em quarentena não tem ferramentas, e o mandate barra a trifecta letal. | guard · router | [arxiv.org/abs/2503.18813](https://arxiv.org/abs/2503.18813) · [arxiv.org/abs/2506.08837](https://arxiv.org/abs/2506.08837) · [simonwillison.net](https://simonwillison.net/2025/Jun/16/the-lethal-trifecta/) |
| tb fala ACP | Servidor Agent Client Protocol pra o leitor/editor humano e editores como o Zed verem e aprovarem as ações dos agentes. | protocolo · UI | [agentclientprotocol.com](https://agentclientprotocol.com/protocol/v1/overview) |
| AGENTS.md como contexto local | `tb init` lê `AGENTS.md`/`CLAUDE.md` existentes e pode semear com `--write-agents`; `tb ctx` (por diretório) permanece como evolução. | mem/ctx | [agents.md](https://agents.md/) |
| Benchmark com custo, pass^k e humanos | Reportar a fronteira custo×acurácia, pass^k além de pass@1, e um A/B com tempo medido, porque a percepção de ganho pode ter o sinal invertido. | benchmark | [arxiv.org/abs/2407.01502](https://arxiv.org/abs/2407.01502) · [arxiv.org/abs/2406.12045](https://arxiv.org/abs/2406.12045) · [arxiv.org/abs/2507.09089](https://arxiv.org/abs/2507.09089) |
| Formato de tarefa Harbor | Tarefa = container + instrução + testes + solução de referência, reaproveitando os benchmarks já adaptados ao Terminal-Bench. | benchmark | [arxiv.org/abs/2601.11868](https://arxiv.org/abs/2601.11868) |
| Juízes com sinal externo | Não aceitar autocorreção sem teste ou ferramenta; juízes com troca de ordem e modelo diferente do gerador. | verify · review/drift | [arxiv.org/abs/2310.01798](https://arxiv.org/abs/2310.01798) · [arxiv.org/abs/2306.05685](https://arxiv.org/abs/2306.05685) |
| Mutação na régua de código | Os testes do discriminador passam por mutantes gerados por LLM; teste que não mata mutante não conta. | verify | [arxiv.org/abs/2501.12862](https://arxiv.org/abs/2501.12862) |
| Métricas visuais além de pixel | Somar block-match, texto, posição, cor e CLIP à paridade de pixel, que quebra com pequenos deslocamentos. | UI · verify | [arxiv.org/abs/2403.03163](https://arxiv.org/abs/2403.03163) |
| Sandbox com proxy de rede | bubblewrap + Landlock com proxy de rede por allowlist, medindo a queda de pedidos de permissão (−84% no caso da Anthropic). | sandbox | [anthropic.com](https://www.anthropic.com/research/claude-code-sandboxing) |

---

## Lacunas

Itens que a pesquisa não encontrou ou encontrou apenas parcialmente:

| Lacuna | Detalhe |
|--------|---------|
| Ensaio original da DeepSeek | Achei o repo oficial e traduções (offloop.org, ghost.p2lab.com), mas não o texto original num domínio da DeepSeek. As traduções não são fontes primárias e ficaram fora da tabela. |
| Router em modelo leve vs pesado em código real | O benchmark da DeepSeek é sintético. Não achei estudo controlado comparando o tamanho do modelo do router em tarefas reais de código. |
| Loop adversarial visual iterativo | Não achei paper com gerador ↔ discriminador visual e limite de iterações como no tb verify. Design2Code e UICoder são os mais próximos. |
| Chamadas de ferramenta especulativas | Sem fonte primária oficial focada em agentes de código. LLMCompiler (paralelismo planejado) foi o mais próximo. |
| Graphify e Obsidian | Só repo e documentação de produto, sem paper ou avaliação publicada. |
| Preprints de 2026 | CAID, Claim Plane, Compact-Bench, SpecBench, TRACE e os estudos de grafo e custo são preprints recentes, sem revisão por pares confirmada. Tratar os números como indicativos. |
| Protocolos "ACP" | A sigla tem dois usos: o Agent Client Protocol da Zed (na tabela) e o Agent Communication Protocol da IBM/BeeAI, que pelos anúncios de 2025 foi incorporado ao A2A. Não reverifiquei esse segundo ponto. |
| Posts da OpenAI | O post do SWE-bench Verified e o de monitoramento de CoT bloqueiam fetch automatizado (403). Para o segundo usei o paper no arXiv. |
