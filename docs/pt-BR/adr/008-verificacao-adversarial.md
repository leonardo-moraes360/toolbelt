[🇺🇸 English](../../adr/008-verificacao-adversarial.md) | 🇧🇷 Português

# ADR-008: Verificação Adversarial (Gerador vs Discriminador)

## Status

accepted

## Contexto

Agentes autônomos frequentemente "derivam" do objetivo original, especialmente em tarefas visuais onde o resultado deveria corresponder a uma referência de design. Mesmo com imagens de referência ou HTML verificável, implementações tendem a divergir sutilmente.

Problemas observados:
- Auto-verificação é enviesada: o agente que implementou tende a "não ver" seus próprios erros
- Métricas de sucesso podem ser manipuladas pelo implementador
- Testes podem ser modificados para passar artificialmente
- Referências podem ser editadas para corresponder à implementação errada

A solução é um **loop adversarial** inspirado em GANs (Generative Adversarial Networks), onde um discriminador independente tenta **falhar** o trabalho do gerador.

## Decisão

Adotar **verificação adversarial** como primitiva de primeira classe no toolbelt, generalizando o conceito para múltiplos domínios além de UI.

### Arquitetura GAN-like

```
┌─────────────────────────────────────────────────────────────────┐
│                         LOOP ADVERSARIAL                        │
└─────────────────────────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         GERADOR                                 │
│  (implementador)                                                │
│                                                                 │
│  - Recebe tarefa e referências                                  │
│  - Implementa solução                                           │
│  - Executa verificação local (NÃO conta como prova)             │
│  - Submete para discriminador                                   │
└─────────────────────────────────────────────────────────────────┘
              │
              │ submissão
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       DISCRIMINADOR                             │
│  (verificador independente, nunca viu o código)                 │
│                                                                 │
│  1. Guard check: verifica mudanças em paths protegidos          │
│  2. Clean build: reconstrói tudo do zero                        │
│  3. Regenera referências (se aplicável)                         │
│  4. Captura fresh do estado atual                               │
│  5. Executa medição com ruler configurado                       │
│  6. Verifica trapaça (edição de refs, masks inválidas, etc)     │
│  7. Emite PASS ou FAIL com relatório                            │
└─────────────────────────────────────────────────────────────────┘
              │
              │ resultado
              ▼
     ┌────────┴────────┐
     │                 │
   PASS              FAIL
     │                 │
     ▼                 ▼
  Concluído    ┌──────────────┐
               │ Iteração < 3? │
               └──────┬───────┘
                      │
            ┌─────────┴─────────┐
            │                   │
           Sim                 Não
            │                   │
            ▼                   ▼
    Volta ao gerador     Escala para humano
```

### Princípios Fundamentais

1. **Só a medição do discriminador conta como prova**
   - Medições do gerador são informativas, não probatórias
   - Isso elimina auto-viés e manipulação

2. **O discriminador tenta FALHAR**
   - Mentalidade adversarial: procura brechas, não confirmação
   - Verifica trapaça além da métrica principal

3. **Máximo 3 iterações, depois escalonamento**
   - Evita loops infinitos
   - Humano resolve impasses genuínos

4. **Isolamento completo**
   - Discriminador tem sandbox, porta e DB próprios
   - Não pode fazer push, trocar branch, ou fazer stash
   - Não corrige nada, apenas reporta

### Relação com Princípios Existentes

Este padrão reforça e generaliza conceitos já presentes no toolbelt:

| Princípio | Como a Verificação Adversarial Reforça |
|-----------|----------------------------------------|
| Implementador ≠ avaliador (ADR-005) | Discriminador é estruturalmente separado do gerador |
| Contexto fresco para revisores | Discriminador começa do zero, sem contexto do gerador |
| Imutabilidade de testes | Guard check detecta edição de referências/comparadores |
| Detecção de reward hacking | Trapaça é verificada explicitamente com regras definidas |

### Rulers Plugáveis

O sistema de verificação é genérico, com **rulers** específicos por domínio:

| Ruler | Domínio | O que Mede |
|-------|---------|------------|
| `visual` | UI/telas | Similaridade pixel-a-pixel + contraste WCAG |
| `tests` | Código | Taxa de passagem em testes ocultos |
| `review` | Documentos | Cobertura e qualidade de revisão por contexto fresco |

Cada ruler define:
- Métrica principal e threshold de passagem
- Regras específicas de trapaça
- Formato de relatório

**Nota de implementação (2026-11, [spec verify-rulers](../specs/verify-rulers.md) fatia 1).** `tb verify run --ruler` implementa `tests` e `visual`; `review` é o comando separado `tb review`, não uma régua do `tb verify`. Em vez de réguas novas, o type checker e os checks nomeados declarados pelo usuário viraram **etapas de gate** da régua `tests` (build → typecheck → lint → checks → testes visíveis → ocultos; cada gate roda uma vez e sua falha é `ESCALATE`). `--ruler` aceita uma lista separada por vírgulas e um nome desconhecido é erro de uso (exit 2). Os comandos do verify são lidos do `.toolbelt/config.yaml` do ref **base** (R0), e o guard reporta `verify_config_changed` quando uma mudança edita a seção `verify:`, então a mudança sob teste não consegue reescrever o próprio gate. Uma régua `coverage` (threshold do `.toolbelt/thresholds.yaml` do base) está planejada como a próxima fatia.

## Consequências

### Positivas

- **Eliminação de auto-viés**: medição independente é mais confiável
- **Detecção de trapaça**: regras explícitas impedem manipulação
- **Generalização**: mesmo padrão para UI, código e documentos
- **Escalabilidade**: loop automático com escalação controlada
- **Auditabilidade**: relatórios detalhados por iteração

### Negativas

- **Overhead computacional**: cada iteração requer build/capture completo
- **Complexidade de setup**: discriminador precisa de ambiente isolado
- **Latência**: múltiplas iterações aumentam tempo total
- **Falsos negativos**: discriminador muito rigoroso pode rejeitar trabalho válido

## Alternativas Consideradas

### Auto-verificação (Self-Verify)

**Por que considerado**:
- Mais simples, sem agente separado
- Menor latência

**Por que não escolhido**:
- Auto-viés: mesmo contexto = mesmos pontos cegos
- Fácil manipulação: gerador controla as métricas
- Histórico mostra drift significativo sem verificação externa

### Verificação por Humano

**Por que considerado**:
- Julgamento de maior qualidade
- Pode avaliar nuances

**Por que não escolhido como único método**:
- Não escala
- Subjetivo e inconsistente
- Humano é reservado para escalação (3ª falha)

### Verificação por Outro Agente Persistente

**Por que considerado**:
- Agente verificador dedicado acumula experiência

**Por que não escolhido**:
- Contexto acumulado pode criar vieses
- Discriminador deve ser fresh por design

## Questões em Aberto

### Threshold Ótimo por Domínio

- Visual: 95% é adequado? Margem de 5% cobre anti-aliasing?
- Testes: 100% ou permitir flaky tests conhecidos?
- Como calibrar thresholds para diferentes projetos?

### Custo de Iterações

- 3 iterações máximas é suficiente?
- Como otimizar o feedback para reduzir iterações?
- Trade-off entre rigor do discriminador e taxa de falso-FAIL

### Estabilidade de Referências

- Como detectar referências instáveis (ex: animações, dados dinâmicos)?
- Política para boards que mudam entre execuções
- Versionamento de referências junto com código

### Integração com CI

- Discriminador deve rodar em CI?
- Como paralelizar verificação de múltiplas telas/testes?
- Caching de builds para acelerar iterações

## Referências

- [ADR-005: Estratégia Anti-Trapaça](005-anti-trapaca.md) — princípio implementador ≠ avaliador
- [Generative Adversarial Networks](https://arxiv.org/abs/1406.2661) — inspiração conceitual
- [WCAG 2.1 Contrast Requirements](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html)
