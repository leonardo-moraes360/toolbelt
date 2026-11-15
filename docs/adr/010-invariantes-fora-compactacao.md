# ADR-010: Invariantes Fixados Fora da Compactação

## Status

proposed

## Contexto

Estudos recentes sobre compactação de contexto em LLMs revelam um problema crítico para agentes de código:

- [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) (Salle et al., 2026, preprint) mostra que o 1º resumo já perde cerca de 1/3 das regras anti-trapaça
- Regras de "não faça" (proibições) caem ~6× mais que outras informações durante a compactação
- [Compact-Bench](https://arxiv.org/abs/2610.31710) (Sun et al., 2026, preprint) demonstra que resumos LLM retêm no máximo 16% dos detalhes recorrentes, enquanto extração mecânica retém 79,1%

Para o toolbelt, isso significa que regras críticas do mandate (paths proibidos, limites de segurança, restrições de scope) podem ser perdidas durante compactação de contexto, abrindo brechas para reward hacking e violações de segurança.

## Decisão

Adotamos uma estratégia de **invariantes fixados** combinada com **compactação híbrida**:

### 1. Arquivo de Invariantes

Um arquivo separado (`invariants.yaml` ou `.toolbelt/invariants.md`) contém regras que **nunca** passam pelo processo de compactação:

```yaml
# .toolbelt/invariants.yaml
version: 1

anti_cheat:
  - "NUNCA modifique arquivos em tests/hidden/"
  - "NUNCA altere thresholds em .toolbelt/thresholds.yaml"
  - "NUNCA edite código de pontuação em benchmarks/"
  - "NUNCA sobrescreva __eq__ para falsificar comparações"
  - "NUNCA guarde estado entre chamadas de teste"

mandate_constraints:
  - "Scope limitado aos arquivos listados no mandate"
  - "Permissões: apenas {permissions}"
  - "Timeout: {timeout}"
  - "Recursos: CPU {cpu_limit}, memória {mem_limit}"

security:
  - "Dados privados + conteúdo não confiável + saída externa = proibido"
  - "Secrets nunca aparecem em logs ou prompts"
  - "Rede negada exceto allowlist explícita"
```

### 2. Processo de Compactação Híbrida

```
┌─────────────────────────────────────────────────────────────────┐
│                    ANTES DA COMPACTAÇÃO                         │
│  1. Extração mecânica (determinística):                        │
│     - Arquivos tocados                                          │
│     - Comandos executados                                       │
│     - Falhas de teste (exit codes, mensagens de erro)          │
│     - Decisões tomadas (com timestamp)                         │
│                                                                 │
│  2. Estado extraído salvo em formato estruturado               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    COMPACTAÇÃO LLM                              │
│  - Resumo do raciocínio e progresso                            │
│  - Contexto narrativo                                           │
│  - NÃO inclui invariantes nem estado mecânico                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    CONTEXTO REMONTADO                           │
│  1. Invariantes (intactos, do arquivo original)                │
│  2. Estado mecânico (intacto, da extração)                     │
│  3. Resumo LLM (compactado)                                    │
└─────────────────────────────────────────────────────────────────┘
```

### 3. KPI de Retenção

Benchmark interno que mede a retenção de invariantes:

```
retenção_invariantes = (invariantes preservados após N compactações) / (invariantes originais)
```

**Meta**: 100% de retenção (por design, já que invariantes não passam pela compactação).

## Consequências

### Positivas

- **Segurança garantida**: regras anti-trapaça e constraints de segurança nunca são perdidas
- **Auditabilidade**: invariantes são verificáveis antes e depois de cada turno
- **Compatibilidade**: extração mecânica funciona com qualquer modelo de compactação
- **Economia de tokens**: informação estruturada é mais compacta que prosa

### Negativas

- **Overhead de implementação**: requer parser e formato estruturado para invariantes
- **Rigidez**: mudanças em invariantes requerem atualização explícita do arquivo
- **Duplicação potencial**: algumas informações podem aparecer tanto no resumo quanto nos invariantes

## Alternativas Consideradas

### Prompt Engineering para Preservar Regras

- **Descrição**: instruir o modelo de compactação a preservar certas informações
- **Por que não**: pesquisa mostra que isso não é confiável; regras de "não faça" caem ~6× mais mesmo com instruções explícitas

### Compactação Apenas Mecânica

- **Descrição**: eliminar compactação LLM e usar apenas extração determinística
- **Por que não**: perde contexto narrativo e raciocínio que podem ser úteis

### Hash de Invariantes

- **Descrição**: hash do arquivo de invariantes verificado a cada turno
- **Por que não**: não previne a perda durante compactação, apenas detecta depois
- **Nota**: pode ser usado **adicionalmente** como verificação

## Referências

- [Benchmarks Are Surprisingly Compaction-Friendly](https://arxiv.org/abs/2610.31778) — Salle et al., 2026 (preprint)
- [Compact-Bench](https://arxiv.org/abs/2610.31710) — Sun et al., 2026 (preprint)
- [Effective context engineering for AI agents](https://www.anthropic.com/research/effective-context-engineering-for-ai-agents) — Anthropic, 2025
