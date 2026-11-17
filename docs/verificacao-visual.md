🇧🇷 Português | [🇺🇸 English](en/verificacao-visual.md)

# Verificação Visual (Ruler `visual`)

Este documento especifica o ruler de verificação visual para o sistema `tb verify`. É uma instância do padrão de verificação adversarial aplicado a interfaces de usuário.

**Status Fase 2**: implementado em `tb verify run --ruler visual`. Captura via Chromium/Chrome headless; comparação PNG pixel-a-pixel com padding magenta para tamanhos diferentes; skip gracioso quando browser ou referências estão indisponíveis (`--skip-unavailable`, default true). Quando o manifesto está ausente, `screens: []`, ou há telas e **todas** resultam `skipped` (ex.: referências ausentes), o relatório JSON agrega `skipped: true` no top-level e o veredicto é `SKIPPED` (não `PASS`).

## Visão Geral

O ruler visual compara capturas de tela contra referências de design aprovadas, medindo similaridade pixel-a-pixel e conformidade com padrões de acessibilidade.

```
┌─────────────────────────────────────────────────────────────────┐
│                    PIPELINE DE VERIFICAÇÃO                      │
└─────────────────────────────────────────────────────────────────┘
                              │
    ┌─────────────────────────┼─────────────────────────┐
    │                         │                         │
    ▼                         ▼                         ▼
┌─────────┐            ┌─────────────┐           ┌──────────┐
│ Geração │            │  Captura    │           │ Medição  │
│ de Refs │            │  Fresh      │           │          │
└─────────┘            └─────────────┘           └──────────┘
    │                         │                         │
    ▼                         ▼                         ▼
┌─────────┐            ┌─────────────┐           ┌──────────┐
│ Design  │            │ Screenshot  │           │ Pixel    │
│ Source  │            │ do App      │           │ Diff     │
└─────────┘            └─────────────┘           └──────────┘
                              │
                              ▼
                       ┌─────────────┐
                       │ Similaridade│
                       │ + WCAG      │
                       └─────────────┘
```

---

## Regras de Medição

### Tabela de Regras

| Regra | Valor | Descrição |
|-------|-------|-----------|
| **Threshold de passagem** | ≥ 95% | Por tela, por modo (light/dark) |
| **Margem de tolerância** | ~5% | Cobre anti-aliasing de fontes |
| **Anti-aliasing** | Excluído | Pixels de borda de texto não contam como diff |
| **Tamanhos diferentes** | Pad com cor única | Diferença de altura custa score |
| **Máscaras** | Permitidas | Apenas para regiões dinâmicas documentadas |
| **Contraste texto** | ≥ 4.5:1 | Texto normal (WCAG AA) |
| **Contraste texto grande** | ≥ 3:1 | Texto ≥ 18pt ou 14pt bold |
| **Contraste controles** | ≥ 3:1 | Botões, inputs, etc |
| **Tamanho mínimo texto** | ≥ 12px | Legibilidade básica |

### Fórmula de Similaridade

```
similarity = 1 - (diff_pixels / (total_pixels - masked_pixels))
```

Onde:
- `diff_pixels`: pixels que diferem além do threshold de cor
- `total_pixels`: área total da imagem
- `masked_pixels`: área coberta por máscaras válidas

### Tratamento de Tamanhos Diferentes

Quando a captura e a referência têm tamanhos diferentes:

1. Identificar qual dimensão difere (largura, altura, ou ambas)
2. Pad a imagem menor com uma cor que **nunca** aparece no design (ex: `#FF00FF`)
3. A área de padding conta como diferença, reduzindo o score
4. Reportar como `SIZE≠` no relatório

```
Referência: 1920×1080
Captura:    1920×1050  (30px mais baixa)

Padding: 30 linhas de #FF00FF na captura
Diff: 30 × 1920 = 57,600 pixels = ~2.8% de diferença só por altura
```

---

## Modos de Captura

Cada tela deve ser verificada em **ambos os modos**:

| Modo | Descrição | Referência |
|------|-----------|------------|
| `light` | Tema claro | `{tela}-light.png` |
| `dark` | Tema escuro | `{tela}-dark.png` |

O threshold de 95% deve ser atingido **independentemente** em cada modo.

---

## Máscaras

### Quando Usar

Máscaras são permitidas **apenas** para:

- **Dados dinâmicos**: timestamps, contadores, IDs únicos
- **Conteúdo de usuário**: avatares, nomes, dados pessoais
- **Elementos com estado imprevisível**: spinners em mid-animation
- **Desvios aprovados**: diferenças aceitas por decisão registrada

### Quando NÃO Usar

Máscaras são **trapaça** se usadas para:

- Esconder diferenças de layout
- Cobrir cores ou fontes erradas
- Ocultar elementos faltantes
- Compensar implementação incorreta

### Formato de Máscara

```yaml
# no manifesto de tela
masks:
  - region: [x, y, width, height]
    reason: "Timestamp atualizado a cada segundo"
    type: dynamic_data
    
  - region: [100, 50, 40, 40]
    reason: "Avatar do usuário logado"
    type: user_content
    
  - region: [200, 300, 100, 20]
    reason: "Desvio aprovado: fonte fallback"
    type: approved_deviation
    decision: "vault/decisions/font-fallback.md"
```

---

## Auditoria de Contraste (WCAG)

### Requisitos

| Elemento | Contraste Mínimo | Referência WCAG |
|----------|------------------|-----------------|
| Texto normal (< 18pt) | 4.5:1 | AA 1.4.3 |
| Texto grande (≥ 18pt ou ≥ 14pt bold) | 3:1 | AA 1.4.3 |
| Controles de UI | 3:1 | AA 1.4.11 |
| Elementos gráficos | 3:1 | AA 1.4.11 |

### Detecção

O discriminador:
1. Identifica elementos de texto e controles na captura
2. Extrai cor de foreground e background
3. Calcula razão de contraste
4. Reporta violações com elemento, contraste atual e requerido

### Relatório de Violação

```yaml
wcag_violations:
  - element: "button.btn-secondary"
    selector: ".login-form > button:nth-child(2)"
    foreground: "#6B7280"
    background: "#F3F4F6"
    contrast: 3.8
    required: 4.5
    type: "text_normal"
    
  - element: "input border"
    selector: "#email-input"
    foreground: "#E5E7EB"
    background: "#FFFFFF"
    contrast: 1.3
    required: 3.0
    type: "ui_control"
```

---

## Determinismo

### Armadilhas Comuns

| Problema | Causa | Solução |
|----------|-------|---------|
| **Fontes diferentes** | Font fallback entre sistemas | Declarar font-family com fallbacks conhecidos, ou usar web fonts |
| **Relógio do servidor** | Timestamps mudam entre execuções | Congelar relógio do servidor para valor fixo |
| **Porta/DB compartilhados** | Dados de outros testes vazam | Porta e DB dedicados por agente |
| **Referências instáveis** | Board muda entre renderizações | Rodar referência 2x, comparar, rejeitar se diferir |
| **Animações** | Estado mid-animation capturado | Aguardar animações completarem, ou desabilitar |
| **Dados dinâmicos** | IDs, counters mudam | Usar máscaras ou seeds fixos |
| **Altura declarada incorreta** | Componente cresce além do esperado | Declarar altura no manifesto, detectar overflow |

### Configuração de Determinismo

```yaml
# .toolbelt/determinism.yaml
server:
  frozen_clock: "2026-01-15T10:00:00Z"
  timezone: "UTC"

database:
  per_agent: true
  prefix: "test_{agent_id}_"
  seed_file: "fixtures/seed.sql"

network:
  port_allocation: dynamic_per_agent
  port_range: [3000, 4000]

visual:
  seed: 42
  disable_animations: true
  wait_for_idle: 500ms

reference_stability:
  runs: 2
  max_diff: 0.1%  # referências devem ser idênticas entre runs
```

### Pipeline de Referência

```
┌─────────────────────────────────────────────────────────────────┐
│                   GERAÇÃO DE REFERÊNCIA                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │ Renderizar design │
                    │ (1ª vez)          │
                    └─────────┬─────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │ Renderizar design │
                    │ (2ª vez)          │
                    └─────────┬─────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │ Comparar runs     │
                    └─────────┬─────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
         diff < 0.1%                    diff ≥ 0.1%
              │                               │
              ▼                               ▼
    ┌───────────────────┐           ┌───────────────────┐
    │ Referência estável│           │ REJEITAR          │
    │ Salvar            │           │ Board instável    │
    └───────────────────┘           └───────────────────┘
```

---

## Manifesto de Telas

### Formato

```yaml
# .toolbelt/screens.yaml
version: 1

screens:
  - name: login
    route: "/login"
    modes: [light, dark]
    viewport: [1920, 1080]
    
    references:
      light: "references/login-light.png"
      dark: "references/login-dark.png"
      
    masks:
      - region: [850, 620, 200, 30]
        reason: "Ano no copyright muda"
        type: dynamic_data
        
    expected_height: 1080
    allow_overflow: false
    
  - name: dashboard
    route: "/dashboard"
    modes: [light, dark]
    viewport: [1920, 1080]
    
    setup:
      - action: login
        user: "test@example.com"
      - action: wait
        selector: ".dashboard-loaded"
        timeout: 5000
        
    references:
      light: "references/dashboard-light.png"
      dark: "references/dashboard-dark.png"
      
    masks:
      - region: [100, 50, 40, 40]
        reason: "Avatar do usuário"
        type: user_content
      - region: [1800, 20, 100, 20]
        reason: "Hora atual"
        type: dynamic_data
        
    wcag:
      level: AA
      exceptions:
        - selector: ".decorative-icon"
          reason: "Elemento decorativo, não informativo"

thresholds:
  min_similarity: 95
  max_size_diff_px: 0
  
determinism:
  seed: 42
  frozen_clock: "2026-01-15T10:00:00Z"
```

### Campos Obrigatórios

| Campo | Descrição |
|-------|-----------|
| `name` | Identificador único da tela |
| `route` | URL/rota para navegar |
| `modes` | Lista de modos a testar |
| `references` | Caminhos para imagens de referência |

### Campos Opcionais

| Campo | Descrição | Default |
|-------|-----------|---------|
| `viewport` | Dimensões da viewport | [1920, 1080] |
| `masks` | Regiões a ignorar | [] |
| `setup` | Ações antes da captura | [] |
| `expected_height` | Altura esperada | altura da referência |
| `allow_overflow` | Permitir conteúdo além da viewport | false |
| `wcag.level` | Nível de conformidade | AA |

---

## Regras de Trapaça (Visual)

### FAIL Imediato

| Tipo | Descrição | Detecção |
|------|-----------|----------|
| **Edição de referência** | Modificar PNG de referência existente | Hash check + guard |
| **Deleção de referência** | Remover referência sem decisão | Guard check |
| **Mask inválida** | Mask cobrindo erro de implementação | Análise de justificativa |
| **Alteração de threshold** | Reduzir min_similarity | Guard check |
| **Alteração de padding** | Mudar cor ou lógica de padding | Guard check |
| **Mudança de seed** | Alterar seed para resultado diferente | Guard check |
| **Estado incorreto** | Capturar tela em estado não especificado | Comparação com spec |
| **Design invertido** | Alterar design para corresponder ao código | Diff no arquivo de design |
| **Crop de referência** | Cortar referência para esconder diferença | Comparação de dimensões |

### Detecção de Mask Inválida

Uma mask é considerada inválida se:

1. Cobre área que deveria ser estática (texto fixo, cores, layout)
2. Não tem `reason` documentada
3. Reason não corresponde a uma das categorias permitidas
4. Área mascarada é grande demais (> 10% da tela sem justificativa)

---

## Fonte de Verdade para Texto

### Hierarquia

1. **Spec/design**: texto exato conforme especificado
2. **i18n/l10n**: arquivos de tradução do projeto
3. **Código**: texto hardcoded (menos preferido)

### Tratamento de Diferenças

Diferenças de texto são **reportadas**, não forçadas:

```yaml
text_differences:
  - location: "login.title"
    reference: "Bem-vindo de volta"
    captured: "Bem vindo de volta"  # falta hífen
    source: "i18n/pt-BR.json"
    action: report
    
  - location: "dashboard.greeting"
    reference: "Olá, {name}"
    captured: "Oi, {name}"
    source: "spec/dashboard.md"
    action: report
```

O discriminador reporta, mas não modifica. O gerador decide se é erro ou evolução intencional.

---

## Questões em Aberto

### Fontes e Renderização

- Como lidar com diferenças de renderização de fonte entre sistemas?
- Tolerância de anti-aliasing é suficiente?
- Devemos normalizar fontes antes de comparar?

### Responsividade

- Como testar múltiplos viewports eficientemente?
- Cada viewport é uma "tela" separada?
- Breakpoints críticos vs testes exaustivos?

### Componentes Interativos

- Como verificar estados de hover, focus, active?
- Sequência de capturas para cada estado?
- Estados de erro e validação?

### Performance

- Tempo de captura para muitas telas?
- Paralelização de capturas?
- Caching de referências entre runs?
