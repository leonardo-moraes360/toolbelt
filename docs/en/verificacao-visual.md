[🇧🇷 Português](../verificacao-visual.md) | 🇺🇸 English

# Visual Verification (Ruler `visual`)

This document specifies the visual verification ruler for the `tb verify` system. It is an instance of the adversarial verification pattern applied to user interfaces.

**Phase 2 Status**: implemented in `tb verify run --ruler visual`. Capture via Chromium/Chrome headless; pixel-by-pixel PNG comparison with magenta padding for different sizes; graceful skip when browser or references are unavailable (`--skip-unavailable`, default true). When the manifest is missing, `screens: []`, or there are screens and **all** result in `skipped` (e.g. missing references), the JSON report aggregates `skipped: true` at the top level and the verdict is `SKIPPED` (not `PASS`).

## Overview

The visual ruler compares screenshots against approved design references, measuring pixel-by-pixel similarity and compliance with accessibility standards.

```
┌─────────────────────────────────────────────────────────────────┐
│                    VERIFICATION PIPELINE                        │
└─────────────────────────────────────────────────────────────────┘
                              │
    ┌─────────────────────────┼─────────────────────────┐
    │                         │                         │
    ▼                         ▼                         ▼
┌─────────┐            ┌─────────────┐           ┌──────────┐
│ Ref     │            │  Fresh      │           │ Measure  │
│ Genera- │            │  Capture    │           │          │
│ tion    │            │             │           │          │
└─────────┘            └─────────────┘           └──────────┘
    │                         │                         │
    ▼                         ▼                         ▼
┌─────────┐            ┌─────────────┐           ┌──────────┐
│ Design  │            │ App         │           │ Pixel    │
│ Source  │            │ Screenshot  │           │ Diff     │
└─────────┘            └─────────────┘           └──────────┘
                              │
                              ▼
                       ┌─────────────┐
                       │ Similarity  │
                       │ + WCAG      │
                       └─────────────┘
```

---

## Measurement Rules

### Rules Table

| Rule | Value | Description |
|------|-------|-------------|
| **Pass threshold** | ≥ 95% | Per screen, per mode (light/dark) |
| **Tolerance margin** | ~5% | Covers font anti-aliasing |
| **Anti-aliasing** | Excluded | Text edge pixels do not count as diff |
| **Different sizes** | Pad with a solid color | Height difference costs score |
| **Masks** | Allowed | Only for documented dynamic regions |
| **Text contrast** | ≥ 4.5:1 | Normal text (WCAG AA) |
| **Large text contrast** | ≥ 3:1 | Text ≥ 18pt or 14pt bold |
| **Control contrast** | ≥ 3:1 | Buttons, inputs, etc. |
| **Minimum text size** | ≥ 12px | Basic readability |

### Similarity Formula

```
similarity = 1 - (diff_pixels / (total_pixels - masked_pixels))
```

Where:
- `diff_pixels`: pixels that differ beyond the color threshold
- `total_pixels`: total image area
- `masked_pixels`: area covered by valid masks

### Handling Different Sizes

When the capture and the reference have different sizes:

1. Identify which dimension differs (width, height, or both)
2. Pad the smaller image with a color that **never** appears in the design (e.g. `#FF00FF`)
3. The padding area counts as difference, reducing the score
4. Report as `SIZE≠` in the report

```
Referência: 1920×1080
Captura:    1920×1050  (30px mais baixa)

Padding: 30 linhas de #FF00FF na captura
Diff: 30 × 1920 = 57,600 pixels = ~2.8% de diferença só por altura
```

---

## Capture Modes

Each screen must be verified in **both modes**:

| Mode | Description | Reference |
|------|-------------|-----------|
| `light` | Light theme | `{tela}-light.png` |
| `dark` | Dark theme | `{tela}-dark.png` |

The 95% threshold must be met **independently** in each mode.

---

## Masks

### When to Use

Masks are allowed **only** for:

- **Dynamic data**: timestamps, counters, unique IDs
- **User content**: avatars, names, personal data
- **Elements with unpredictable state**: spinners mid-animation
- **Approved deviations**: differences accepted by a recorded decision

### When NOT to Use

Masks are **cheating** if used to:

- Hide layout differences
- Cover wrong colors or fonts
- Conceal missing elements
- Compensate for incorrect implementation

### Mask Format

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

## Contrast Audit (WCAG)

### Requirements

| Element | Minimum Contrast | WCAG Reference |
|---------|------------------|----------------|
| Normal text (< 18pt) | 4.5:1 | AA 1.4.3 |
| Large text (≥ 18pt or ≥ 14pt bold) | 3:1 | AA 1.4.3 |
| UI controls | 3:1 | AA 1.4.11 |
| Graphic elements | 3:1 | AA 1.4.11 |

### Detection

The discriminator:
1. Identifies text elements and controls in the capture
2. Extracts foreground and background color
3. Calculates the contrast ratio
4. Reports violations with element, current contrast, and required contrast

### Violation Report

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

## Determinism

### Common Pitfalls

| Problem | Cause | Solution |
|---------|-------|----------|
| **Different fonts** | Font fallback across systems | Declare font-family with known fallbacks, or use web fonts |
| **Server clock** | Timestamps change between runs | Freeze the server clock to a fixed value |
| **Shared port/DB** | Data from other tests leaks | Dedicated port and DB per agent |
| **Unstable references** | Board changes between renders | Run the reference twice, compare, reject if they differ |
| **Animations** | Mid-animation state captured | Wait for animations to complete, or disable them |
| **Dynamic data** | IDs, counters change | Use masks or fixed seeds |
| **Incorrect declared height** | Component grows beyond expectation | Declare height in the manifest, detect overflow |

### Determinism Configuration

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

### Reference Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                   REFERENCE GENERATION                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │ Render design     │
                    │ (1st time)        │
                    └─────────┬─────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │ Render design     │
                    │ (2nd time)        │
                    └─────────┬─────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │ Compare runs      │
                    └─────────┬─────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
         diff < 0.1%                    diff ≥ 0.1%
              │                               │
              ▼                               ▼
    ┌───────────────────┐           ┌───────────────────┐
    │ Stable reference  │           │ REJECT            │
    │ Save              │           │ Unstable board    │
    └───────────────────┘           └───────────────────┘
```

---

## Screen Manifest

### Format

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

### Required Fields

| Field | Description |
|-------|-------------|
| `name` | Unique screen identifier |
| `route` | URL/route to navigate to |
| `modes` | List of modes to test |
| `references` | Paths to reference images |

### Optional Fields

| Field | Description | Default |
|-------|-------------|---------|
| `viewport` | Viewport dimensions | [1920, 1080] |
| `masks` | Regions to ignore | [] |
| `setup` | Actions before capture | [] |
| `expected_height` | Expected height | reference height |
| `allow_overflow` | Allow content beyond the viewport | false |
| `wcag.level` | Compliance level | AA |

---

## Cheating Rules (Visual)

### Immediate FAIL

| Type | Description | Detection |
|------|-------------|-----------|
| **Reference edit** | Modify an existing reference PNG | Hash check + guard |
| **Reference deletion** | Remove a reference without a decision | Guard check |
| **Invalid mask** | Mask covering an implementation error | Justification analysis |
| **Threshold change** | Lower min_similarity | Guard check |
| **Padding change** | Change padding color or logic | Guard check |
| **Seed change** | Alter seed for a different result | Guard check |
| **Wrong state** | Capture a screen in an unspecified state | Comparison with spec |
| **Inverted design** | Change the design to match the code | Diff on the design file |
| **Reference crop** | Crop the reference to hide a difference | Dimension comparison |

### Invalid Mask Detection

A mask is considered invalid if:

1. It covers an area that should be static (fixed text, colors, layout)
2. It has no documented `reason`
3. The reason does not match one of the allowed categories
4. The masked area is too large (> 10% of the screen without justification)

---

## Source of Truth for Text

### Hierarchy

1. **Spec/design**: exact text as specified
2. **i18n/l10n**: project translation files
3. **Code**: hardcoded text (least preferred)

### Handling Differences

Text differences are **reported**, not forced:

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

The discriminator reports, but does not modify. The generator decides whether it is an error or intentional evolution.

---

## Open Questions

### Fonts and Rendering

- How to handle font rendering differences across systems?
- Is anti-aliasing tolerance enough?
- Should we normalize fonts before comparing?

### Responsiveness

- How to test multiple viewports efficiently?
- Is each viewport a separate "screen"?
- Critical breakpoints vs exhaustive tests?

### Interactive Components

- How to verify hover, focus, and active states?
- Capture sequence for each state?
- Error and validation states?

### Performance

- Capture time for many screens?
- Capture parallelization?
- Reference caching between runs?
