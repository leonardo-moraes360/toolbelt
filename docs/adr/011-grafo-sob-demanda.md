# ADR-011: Grafo de Código Sob Demanda com A/B

## Status

accepted (implementação Fase 2: `tb graph` + MCP + A/B `bench/ab/graph-*.yaml`)

## Contexto

A intuição comum é que grafos de código (AST, call graph, dependency graph) sempre economizam tokens ao evitar leituras desnecessárias de arquivos. No entanto, pesquisas recentes desafiam essa suposição:

- [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) (Sepidband et al., 2026, preprint): mapas de código cortam 17–39% de tokens **quando localizar é o gargalo**, mas **sobem a latência** e nem sempre compensam
- [Analyzing and Mitigating Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) (Hu et al., 2026, preprint): busca estrutural pode **encarecer até 28,14%** em certos cenários; skills escritas por humanos cortam até 41,73%

A implicação é que o toolbelt não deve forçar o uso do grafo em toda tarefa, mas oferecê-lo como ferramenta opcional com medição de quando ajuda vs. quando encarece.

## Decisão

Adotamos uma abordagem de **grafo sob demanda com medição A/B**:

### 1. Grafo como Ferramenta, Não como Pré-requisito

```yaml
# O grafo é uma ferramenta que o agente pode escolher usar
tools:
  - name: tb_graph_search
    description: "Busca semântica no grafo de código"
    optional: true
    
  - name: tb_graph_callers
    description: "Encontra chamadores de uma função"
    optional: true
    
  - name: tb_graph_deps
    description: "Lista dependências de um módulo"
    optional: true
```

**Mudança**: o hook "consulte o grafo antes de ler arquivos" vira **recomendação**, não obrigação.

### 2. Saída Enxuta por Padrão

```bash
# Antes (verboso)
tb graph search "autenticação"
# Retorna: título, descrição, código, contexto, arquivos relacionados...

# Depois (enxuto)
tb graph search "autenticação" --format compact
# Retorna: [file:line] título (1 linha por resultado)

# Detalhes sob demanda
tb graph search "autenticação" --file src/auth.go --expand
```

### 3. A/B por Tipo de Tarefa

O benchmark inclui experimentos A/B:

| Tipo de Tarefa | Condição A | Condição B | Métricas |
|----------------|------------|------------|----------|
| Localização em repo grande | Sem grafo | Com grafo | tokens, tempo, acurácia |
| Bug fix localizado | Sem grafo | Com grafo | tokens, tempo, acurácia |
| Refatoração cross-file | Sem grafo | Com grafo | tokens, tempo, acurácia |
| Implementação de feature | Sem grafo | Com grafo | tokens, tempo, acurácia |

**Hipótese a testar**: grafo compensa mais em localização e refatoração cross-file, menos em bug fixes localizados.

### 4. Watchdog de Desperdício

O router detecta padrões de ineficiência identificados no estudo:

```yaml
# Padrões a detectar (apareceram em 79–98% das tarefas no estudo)
inefficiency_patterns:
  - redundant_search: "Mesma query executada 2+ vezes"
  - similar_scripts: "Scripts com >80% de similaridade executados"
  - test_reexecution: "Mesmos testes executados sem mudança de código"
  - excessive_graph_calls: ">10 chamadas de grafo sem leitura de arquivo"
```

O watchdog reporta métricas de desperdício como KPI.

### 5. Skills Humanas Priorizadas

Com base no achado de que skills escritas por humanos cortam até 41,73% do custo:

```yaml
# Skills têm prioridade sobre busca automática
skills:
  - name: "localizar-handler-go"
    description: "Localiza handlers HTTP em projetos Go"
    trigger: "busca por handler, endpoint, rota em projeto Go"
    steps:
      - "grep -r 'func.*Handler' --include='*.go'"
      - "filtrar por padrão http.HandleFunc ou chi.Route"
    # Skill humana evita N chamadas de grafo
```

## Consequências

### Positivas

- **Custo real medido**: A/B identifica quando o grafo vale o investimento
- **Flexibilidade**: agente escolhe ferramenta certa para a tarefa
- **Menos desperdício**: watchdog detecta padrões ineficientes
- **Skills reutilizáveis**: conhecimento humano capturado e priorizado

### Negativas

- **Complexidade de benchmark**: requer infraestrutura de A/B
- **Variância**: resultados dependem do tipo de tarefa e tamanho do repo
- **Risco de subuso**: agente pode não usar grafo quando deveria
- **Manutenção de skills**: requer curadoria humana contínua

### Métricas de Sucesso

| Métrica | Meta | Justificativa |
|---------|------|---------------|
| graph_roi_positive_pct | ≥ 60% | Grafo deve compensar na maioria das tarefas onde é usado |
| wasteful_pattern_rate | ≤ 20% | Padrões de desperdício devem ser raros |
| skill_hit_rate | ≥ 30% | Skills humanas devem ser acionadas quando aplicáveis |

## Alternativas Consideradas

### Grafo Obrigatório

- **Descrição**: forçar consulta ao grafo antes de qualquer leitura de arquivo
- **Por que não**: pesquisa mostra que pode encarecer até 28,14% em certos cenários

### Grafo Desabilitado por Padrão

- **Descrição**: grafo só ligado explicitamente pelo usuário
- **Por que não**: perde benefício de 17–39% de economia quando localização é o gargalo

### Heurística Estática

- **Descrição**: regra fixa (ex: "use grafo se repo > 1000 arquivos")
- **Por que não**: não captura variação por tipo de tarefa; A/B é mais preciso

## Referências

- [How Should Agents See Code?](https://arxiv.org/abs/2611.07286) — Sepidband et al., 2026 (preprint)
- [Analyzing and Mitigating Cost-Inefficient Behaviors in Coding Agents](https://arxiv.org/abs/2609.30725) — Hu et al., 2026 (preprint)
- [Improving agent with semantic search](https://cursor.com/blog/semsearch) — Cursor, 2025
- [Building a better repository map with tree sitter](https://aider.chat/2023/10/22/repomap.html) — Aider, 2023
