# Benchmark e KPIs

Este documento define a metodologia de benchmark e os KPIs (Key Performance Indicators) para avaliar a eficácia do toolbelt.

## Objetivo

Medir quantitativamente se o toolbelt atinge seus três objetivos:

1. **Economizar tokens** — menos tokens consumidos por tarefa
2. **Melhorar a acurácia** — maior taxa de sucesso na primeira tentativa
3. **Aumentar a velocidade** — menor tempo de execução

---

## Metodologia

### Desenho Experimental

```
┌─────────────────────────────────────────────────────────────────┐
│                      Suite de Tarefas                           │
│  (código fechado, testes ocultos, reprodutível)                 │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│      Grupo Baseline     │     │    Grupo Experimental    │
│  (agente sozinho)       │     │  (agente + toolbelt)     │
│                         │     │                          │
│  - Acesso direto ao     │     │  - Acesso via tb graph   │
│    código (grep/read)   │     │  - Sandbox isolado       │
│  - Sem isolamento       │     │  - Memória persistente   │
│  - Sem memória          │     │  - Guardrails ativos     │
│    estruturada          │     │                          │
└─────────────────────────┘     └─────────────────────────┘
              │                               │
              └───────────────┬───────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Avaliador Isolado                          │
│  (executa testes ocultos, mede métricas, gera relatório)        │
└─────────────────────────────────────────────────────────────────┘
```

### Condições Controladas

| Aspecto | Controle |
|---------|----------|
| Modelo de linguagem | Mesmo modelo e temperatura para ambos os grupos |
| Tarefas | Suite fixa, embaralhada, sem acesso prévio |
| Testes | Ocultos do agente implementador |
| Ambiente | Mesmo hardware/VM, mesmo estado inicial do repositório |
| Avaliação | Automatizada, determinística, sem intervenção humana |

### Repetições e Variância

- **Mínimo de repetições**: 5 por tarefa por condição
- **Recomendado**: 10 repetições para significância estatística
- **Medidas**: média, mediana, desvio padrão, intervalo de confiança 95%

---

## KPIs Primários

### 1. pass@1 (Taxa de Sucesso na Primeira Tentativa)

**Definição**: Proporção de tarefas completadas corretamente na primeira tentativa, sem retrabalho.

```
pass@1 = (tarefas corretas na 1ª tentativa) / (total de tarefas)
```

**Medição**:
- O agente submete solução
- Avaliador executa suite de testes ocultos
- Passa se todos os testes passam

**Meta**: Melhorar pass@1 em relação ao baseline

### 2. tokens_per_task (Tokens por Tarefa)

**Definição**: Total de tokens consumidos (input + output) para completar uma tarefa.

```
tokens_per_task = tokens_input + tokens_output
```

**Medição**:
- Contagem via API do modelo
- Inclui todas as chamadas até conclusão ou timeout

**Meta**: Reduzir tokens_per_task em relação ao baseline

### 3. wall_time (Tempo de Execução)

**Definição**: Tempo total desde o início da tarefa até a submissão final.

```
wall_time = timestamp_fim - timestamp_inicio
```

**Medição**:
- Timestamp do sistema
- Inclui tempo de espera por API, execução de ferramentas

**Meta**: Reduzir wall_time em relação ao baseline

---

## KPIs Secundários

### 4. tool_calls (Chamadas de Ferramentas)

**Definição**: Número de chamadas a ferramentas externas (read file, run command, etc.).

**Relevância**: Menos chamadas geralmente indica uso mais eficiente do contexto.

### 5. file_reads (Leituras de Arquivos)

**Definição**: Número de arquivos lidos durante a tarefa.

**Relevância**: O grafo deve reduzir leituras desnecessárias.

### 6. rework_rate (Taxa de Retrabalho)

**Definição**: Proporção de tarefas que precisaram de correção após primeira submissão.

```
rework_rate = (tarefas com retrabalho) / (total de tarefas)
```

### 7. cost_per_task (Custo por Tarefa)

**Definição**: Custo monetário (USD) por tarefa completada.

```
cost_per_task = (tokens_input * price_input) + (tokens_output * price_output)
```

### 8. guard_violations_blocked (Violações Bloqueadas)

**Definição**: Número de tentativas de violação de segurança detectadas e bloqueadas.

**Relevância**: Demonstra eficácia das proteções.

### 9. guard_violations_unblocked (Violações Não Bloqueadas)

**Definição**: Número de violações que escaparam às proteções.

**Meta**: Zero

---

## Comparação: Graph-First vs Grep-First

Uma variante específica do benchmark compara abordagens de busca:

### Graph-First (Experimental)

1. Agente recebe tarefa
2. Consulta `tb graph search` / `callers` / `deps` primeiro
3. Identifica arquivos relevantes via grafo
4. Lê apenas arquivos necessários
5. Implementa solução

### Grep-First (Baseline)

1. Agente recebe tarefa
2. Usa grep/rg para buscar padrões
3. Lê arquivos que parecem relevantes
4. Pode ler arquivos desnecessários
5. Implementa solução

### Métricas Específicas

| Métrica | Descrição |
|---------|-----------|
| `files_read_relevant` | Arquivos lidos que eram de fato relevantes |
| `files_read_irrelevant` | Arquivos lidos que não eram relevantes |
| `precision` | files_read_relevant / total_files_read |
| `recall` | files_read_relevant / total_files_actually_relevant |
| `tokens_on_irrelevant` | Tokens gastos em arquivos irrelevantes |

---

## Suite de Tarefas

### Categorias

| Categoria | Descrição | Qtd Tarefas |
|-----------|-----------|-------------|
| `implement` | Implementar nova funcionalidade | 20 |
| `bugfix` | Corrigir bug descrito | 15 |
| `refactor` | Refatorar código existente | 10 |
| `test` | Escrever testes para código existente | 10 |
| `document` | Documentar código/API | 5 |

**Total**: 60 tarefas

### Níveis de Dificuldade

| Nível | Características |
|-------|-----------------|
| `easy` | Mudança localizada, 1-2 arquivos |
| `medium` | Múltiplos arquivos, entender contexto |
| `hard` | Arquitetura complexa, dependências cruzadas |

### Estrutura de uma Tarefa

```yaml
id: task-042
category: implement
difficulty: medium
title: "Implementar endpoint de autenticação"
description: |
  Criar endpoint POST /api/auth/login que:
  - Recebe email e senha
  - Valida credenciais contra o banco
  - Retorna JWT token
  - Implementa rate limiting

files_relevant:
  - src/api/handlers/auth.go
  - src/services/auth.go
  - src/database/users.go

hidden_tests:
  - tests/api/auth_test.go
  - tests/integration/auth_integration_test.go

expected_changes:
  - src/api/handlers/auth.go
  - src/api/routes.go

timeout: 10m
max_tokens: 20000
```

### Proteção dos Testes

Os arquivos em `hidden_tests` são:
- Não visíveis para o agente implementador
- Montados read-only no sandbox do avaliador
- Hash verificado antes e depois de cada avaliação

---

## Execução do Benchmark

### Comandos

```bash
# Executar baseline
tb bench run --baseline --repetitions 10 full-suite \
  --output baseline-results.json

# Executar com toolbelt
tb bench run --toolbelt --repetitions 10 full-suite \
  --output toolbelt-results.json

# Comparar
tb bench compare baseline-results.json toolbelt-results.json

# Gerar relatório
tb bench report --input toolbelt-results.json --format markdown
```

### Estrutura dos Resultados

```json
{
  "benchmark_id": "bench-20260115-120000",
  "config": {
    "model": "claude-sonnet-4-20250514",
    "temperature": 0.3,
    "repetitions": 10,
    "condition": "toolbelt"
  },
  "summary": {
    "pass_at_1": 0.72,
    "tokens_per_task_mean": 8542,
    "tokens_per_task_std": 1234,
    "wall_time_mean_seconds": 145.3,
    "wall_time_std_seconds": 23.5,
    "tool_calls_mean": 12.4,
    "file_reads_mean": 5.2,
    "rework_rate": 0.18,
    "cost_per_task_usd": 0.043,
    "guard_violations_blocked": 7,
    "guard_violations_unblocked": 0
  },
  "tasks": [
    {
      "task_id": "task-042",
      "repetitions": [
        {
          "rep": 1,
          "passed": true,
          "tokens_input": 6234,
          "tokens_output": 2108,
          "wall_time_seconds": 132.5,
          "tool_calls": 11,
          "file_reads": 4,
          "rework_attempts": 0
        }
      ]
    }
  ]
}
```

---

## Relatório de Resultados

### Formato

O relatório deve incluir:

1. **Resumo executivo**: melhorias percentuais em cada KPI
2. **Tabela comparativa**: baseline vs toolbelt por categoria
3. **Análise estatística**: significância das diferenças
4. **Gráficos**: distribuições, boxplots, tendências
5. **Casos notáveis**: tarefas onde toolbelt teve maior/menor impacto
6. **Violações**: detalhes de qualquer violação detectada

### Exemplo de Resumo

```markdown
## Resumo Executivo

| KPI | Baseline | Toolbelt | Δ |
|-----|----------|----------|---|
| pass@1 | 0.58 | 0.72 | +24% |
| tokens/task | 12,340 | 8,542 | -31% |
| wall_time | 203s | 145s | -29% |
| tool_calls | 18.2 | 12.4 | -32% |
| file_reads | 8.7 | 5.2 | -40% |
| rework_rate | 0.32 | 0.18 | -44% |
| cost/task | $0.062 | $0.043 | -31% |

Diferenças estatisticamente significativas (p < 0.05) em todos os KPIs.
```

---

## Questões em Aberto

### Tamanho da Suite

- 60 tarefas são suficientes para significância estatística?
- Devemos expandir para 100+ tarefas?
- Trade-off: mais tarefas = mais tempo de benchmark = mais custo

### Modelos Diferentes

- O benchmark deve ser executado em múltiplos modelos?
- Como normalizar resultados entre modelos com preços diferentes?

### Tarefas Multi-Agente

- Como medir eficácia da hierarquia de agentes?
- Métricas específicas para coordenação planner→supervisor→worker

### Reprodutibilidade

- Como garantir que a suite de tarefas não vaze para dados de treinamento?
- Devemos rotacionar tarefas periodicamente?

### Baseline Alternativo

- Considerar outros baselines além de "agente sozinho":
  - Agente com grep + sem grafo
  - Agente com outro toolkit (comparação competitiva)

---

## KPIs de Verificação Adversarial

O sistema `tb verify` introduz métricas específicas para avaliar a eficácia do loop gerador→discriminador.

### 15. mean_similarity (Similaridade Média)

**Definição**: Média de similaridade por tela no ruler visual.

```
mean_similarity = sum(similarity_per_screen) / count(screens)
```

**Medição**:
- Calculada pelo discriminador
- Separada por modo (light/dark)
- Exclui áreas mascaradas

### 16. iterations_to_pass (Iterações até PASS)

**Definição**: Número de ciclos gerador→discriminador até atingir PASS.

```
iterations_to_pass = count(iterations até verdict == PASS)
```

**Relevância**: Menos iterações = maior eficiência do gerador.

**Meta**: ≤ 2 iterações em média

### 17. cheating_attempts_detected (Tentativas de Trapaça Detectadas)

**Definição**: Número de tentativas de trapaça identificadas pelo discriminador.

```
cheating_attempts = count(violations onde tipo ∈ cheating_rules)
```

**Categorias**:
- Edição de referências
- Masks inválidas
- Alteração de threshold
- Mudança de seed
- Outros

**Meta**: 0 tentativas não detectadas

### 18. human_escalations (Escalações para Humano)

**Definição**: Proporção de tarefas que requerem intervenção humana após 3 falhas.

```
human_escalations = (tasks com 3 FAILs) / (total de tasks)
```

**Relevância**: Alta taxa indica problema no processo ou spec inadequada.

**Meta**: < 10%

### 19. discriminator_false_fail_rate (Taxa de Falso-FAIL do Discriminador)

**Definição**: Proporção de FAILs que eram falsos positivos (trabalho estava correto).

```
false_fail_rate = (FAILs revertidos por humano) / (total de FAILs)
```

**Medição**:
- Humano revisa FAILs escalados
- Marca como "falso FAIL" se implementação estava correta
- Indica discriminador muito rigoroso ou spec ambígua

**Meta**: < 5%

### 20. verification_overhead (Overhead de Verificação)

**Definição**: Tempo adicional gasto em verificação vs implementação.

```
overhead = (tempo_total_verificacao) / (tempo_implementacao)
```

**Componentes**:
- Tempo de guard check
- Tempo de build
- Tempo de captura
- Tempo de comparação

---

## Comparação: Auto-Verificação vs Verificação Adversarial

Uma variante do benchmark compara verificação pelo próprio implementador com verificação adversarial.

### Auto-Verificação (Self-Verify)

1. Gerador implementa solução
2. **Mesmo gerador** verifica seu próprio trabalho
3. Gerador tem todo o contexto e código
4. Métricas coletadas pelo próprio gerador

### Verificação Adversarial

1. Gerador implementa solução
2. **Discriminador independente** verifica
3. Discriminador não viu o código sendo escrito
4. Clean build, fresh capture, medição independente

### Métricas Específicas

| Métrica | Descrição |
|---------|-----------|
| `defects_caught_self` | Defeitos encontrados por auto-verificação |
| `defects_caught_adversarial` | Defeitos encontrados por verificação adversarial |
| `defects_missed_self` | Defeitos que passaram por auto-verificação |
| `defects_missed_adversarial` | Defeitos que passaram por verificação adversarial |
| `cheating_detected` | Tentativas de trapaça detectadas (só adversarial detecta) |
| `time_to_verify_self` | Tempo de auto-verificação |
| `time_to_verify_adversarial` | Tempo de verificação adversarial |

### Metodologia

Para medir objetivamente:

1. **Gold standard**: humanos especialistas identificam todos os defeitos em implementações
2. **Auto-verificação**: gerador verifica, mede recall contra gold standard
3. **Adversarial**: discriminador verifica, mede recall contra gold standard
4. **Comparação**: qual abordagem encontra mais defeitos?

### Hipótese

Esperamos que verificação adversarial supere auto-verificação em:
- `defects_caught`: mais defeitos encontrados
- `cheating_detected`: detecta manipulação (auto-verificação não detecta)
- `defects_missed`: menos defeitos escapam

E que auto-verificação seja melhor em:
- `time_to_verify`: mais rápida (sem spawn de discriminador)
- `false_fail_rate`: menos falsos positivos (conhece o contexto)

### Benchmark de UI

Específico para o ruler visual:

```yaml
benchmark:
  name: adversarial-vs-self-visual
  
  tasks:
    - implement UI according to design
    - fix visual differences
    - add dark mode support
    
  conditions:
    self_verify:
      verifier: generator
      clean_build: false
      fresh_capture: false
      
    adversarial:
      verifier: discriminator
      clean_build: true
      fresh_capture: true
      
  metrics:
    - similarity_achieved
    - iterations_to_pass
    - defects_found
    - defects_missed
    - cheating_attempts
    - time_total
```
