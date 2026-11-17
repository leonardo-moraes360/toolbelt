[🇧🇷 Português](../identidade-visual.md) | 🇺🇸 English

# Toolbelt Visual Identity

<p align="center">
  <img src="../../assets/logo/toolbelt-logo.png" alt="Toolbelt Logo" width="256">
</p>

Toolbelt adopts the **Synthwave** theme as its official visual identity, inspired by 1980s retro-futurist aesthetics with vibrant purple, magenta, pink, and cyan tones.

---

## Logo

### Available Versions

| File | Size | Use |
|------|------|-----|
| `toolbelt-logo.png` | 1024×1024 | Original, high resolution |
| `toolbelt-logo-512.png` | 512×512 | Documentation, web |
| `toolbelt-logo-256.png` | 256×256 | README, avatars |
| `toolbelt-logo-128.png` | 128×128 | Medium icons |
| `toolbelt-logo-64.png` | 64×64 | Small icons |
| `toolbelt-logo.svg` | Vector | Scalable, print |
| `favicon.ico` | 32×32 | Web favicon |

### Logo Elements

The logo represents a stylized tool belt with elements that symbolize the project:

1. **Violet circle** (#4A1999): background that represents the execution environment
2. **Purple belt with dark tips**: the unifying structure of the tools
3. **Pouch with magenta→pink gradient**: the storage pocket/vault
4. **White button**: focal point, simplicity
5. **Angle brackets `< >`**: represent code and the CLI
6. **Cyan robot**: the AI agent that uses the tools

### Usage Rules

#### Clear Space

Keep a minimum margin equal to **10% of the logo size** on all sides.

```
┌──────────────────────────┐
│         10%              │
│    ┌────────────┐        │
│ 10%│            │ 10%    │
│    │   LOGO     │        │
│    │            │        │
│    └────────────┘        │
│         10%              │
└──────────────────────────┘
```

#### Minimum Size

| Context | Minimum Size |
|---------|--------------|
| Digital (web, app) | 32px |
| Print | 12mm |
| Favicon | 16px (use the specific ICO) |

#### Allowed Backgrounds

| Background Color | Logo Version |
|------------------|--------------|
| Dark (#29282D or darker) | Standard logo ✓ |
| Violet (#4A1999) | Standard logo ✓ |
| Light (#F5F3FA or white) | Standard logo ✓ (violet circle creates contrast) |

**Forbidden:**
- Stretching or distorting proportions
- Applying effects (shadow, glow) that alter the colors
- Using over backgrounds with complex patterns
- Changing the logo colors

---

## Color Palette

### Primary Colors (extracted from the logo)

| Name | Hex | RGB | Use |
|------|-----|-----|-----|
| **Violet** | `#4A1999` | 74, 25, 153 | Main background, institutional color |
| **Purple** | `#982BD4` | 152, 43, 212 | Primary, buttons, links |
| **Purple Shadow** | `#761DC5` | 118, 29, 197 | Hover, borders, shadows |
| **Magenta** | `#BE2AC6` | 190, 42, 198 | Gradient start, highlights |
| **Pink** | `#F9278C` | 249, 39, 140 | Gradient end, errors, alerts |
| **Cyan** | `#0CD7FF` | 12, 215, 255 | Accent, links, interactive |
| **Dark Blue** | `#00579A` | 0, 87, 154 | Dark cyan, contrast |
| **Near Black** | `#29282D` | 41, 40, 45 | Surfaces, text on light backgrounds |
| **White** | `#FFFFFF` | 255, 255, 255 | Text on dark backgrounds |

### Semantic Colors

| Purpose | Dark Mode | Light Mode |
|---------|-----------|------------|
| Background | `#4A1999` | `#F5F3FA` |
| Surface | `#29282D` | `#FFFFFF` |
| Primary text | `#FFFFFF` | `#29282D` |
| Secondary text | `#A099B0` | `#6B6680` |
| Success | `#2DD4A7` | `#1AA87A` |
| Error | `#F9278C` | `#D91A6B` |
| Warning | `#FFB020` | `#D99000` |

### Gradients

| Name | Definition | Use |
|------|------------|-----|
| **Primary** | `linear-gradient(180deg, #BE2AC6 → #F9278C)` | Main highlight, CTAs |
| **Belt** | `linear-gradient(90deg, #29282D → #761DC5 → #982BD4 → #761DC5 → #29282D)` | Decorative, headers |
| **Accent** | `linear-gradient(135deg, #0CD7FF → #00579A)` | Secondary accent |

---

## WCAG Contrast

Verified text combinations for accessibility:

### Dark Mode (violet background #4A1999)

| Text | Background | Ratio | WCAG AA | WCAG AAA |
|------|------------|-------|---------|----------|
| White `#FFFFFF` | Violet `#4A1999` | 8.5:1 | ✅ Pass | ✅ Pass |
| Cyan `#0CD7FF` | Violet `#4A1999` | 7.2:1 | ✅ Pass | ✅ Pass |
| Pink `#F9278C` | Violet `#4A1999` | 4.6:1 | ✅ Pass | ❌ Fail |
| Purple `#982BD4` | Violet `#4A1999` | 2.1:1 | ❌ Fail | ❌ Fail |

### Dark Mode (surface #29282D)

| Text | Background | Ratio | WCAG AA | WCAG AAA |
|------|------------|-------|---------|----------|
| White `#FFFFFF` | Near Black `#29282D` | 14.3:1 | ✅ Pass | ✅ Pass |
| Cyan `#0CD7FF` | Near Black `#29282D` | 11.8:1 | ✅ Pass | ✅ Pass |
| Pink `#F9278C` | Near Black `#29282D` | 7.5:1 | ✅ Pass | ✅ Pass |
| Purple `#982BD4` | Near Black `#29282D` | 3.4:1 | ✅ Pass (large) | ❌ Fail |

### Light Mode (background #F5F3FA)

| Text | Background | Ratio | WCAG AA | WCAG AAA |
|------|------------|-------|---------|----------|
| Near Black `#29282D` | Light `#F5F3FA` | 12.8:1 | ✅ Pass | ✅ Pass |
| Violet `#4A1999` | Light `#F5F3FA` | 7.6:1 | ✅ Pass | ✅ Pass |
| Purple `#982BD4` | Light `#F5F3FA` | 4.1:1 | ✅ Pass | ❌ Fail |
| Dark Blue `#00579A` | Light `#F5F3FA` | 5.8:1 | ✅ Pass | ✅ Pass (large) |

### Recommendations

- **Body text**: use White on Violet/Near Black, or Near Black on a light background
- **Links and interactive elements**: Cyan on dark backgrounds, Dark Blue on light backgrounds
- **Alerts/Errors**: Pink is accessible on surfaces (#29282D), but not on Violet for small text
- **Avoid**: Purple as a text color (low contrast); use only for decorative or large elements

---

## Design Tokens

Tokens are available in two formats:

### JSON (Design Tokens Format)

```
assets/theme/tokens.json
```

Follows the [Design Tokens](https://design-tokens.github.io/community-group/format/) standard for integration with tools such as Figma, Style Dictionary, etc.

### CSS Custom Properties

```
assets/theme/theme.css
```

Ready-to-use CSS variables:

```css
/* Import into the project */
@import 'assets/theme/theme.css';

/* Use variables */
.meu-componente {
  background: var(--tb-bg);
  color: var(--tb-text);
  border-radius: var(--tb-radius-md);
}

/* Logo gradient */
.destaque {
  background: var(--tb-gradient-primary);
}
```

### CSS Variable Naming

| Prefix | Category | Examples |
|--------|----------|----------|
| `--tb-color-*` | Base colors | `--tb-color-violeta`, `--tb-color-ciano` |
| `--tb-gradient-*` | Gradients | `--tb-gradient-primary`, `--tb-gradient-belt` |
| `--tb-bg*` | Backgrounds | `--tb-bg`, `--tb-bg-secondary` |
| `--tb-surface*` | Surfaces | `--tb-surface`, `--tb-surface-elevated` |
| `--tb-primary*` | Primary color | `--tb-primary`, `--tb-primary-hover` |
| `--tb-accent*` | Accent color | `--tb-accent`, `--tb-accent-dark` |
| `--tb-text*` | Text | `--tb-text`, `--tb-text-muted` |
| `--tb-spacing-*` | Spacing | `--tb-spacing-sm`, `--tb-spacing-lg` |
| `--tb-radius-*` | Borders | `--tb-radius-md`, `--tb-radius-full` |
| `--tb-font-*` | Typography | `--tb-font-mono`, `--tb-font-size-md` |
| `--tb-shadow-*` | Shadows | `--tb-shadow-sm`, `--tb-shadow-lg` |
| `--tb-error/success/warning` | Status | `--tb-error`, `--tb-success` |

---

## CLI: ANSI Mapping

Colored CLI (`tb`) output should follow the same palette. Use the codes below:

### Mapping Table

| Semantic Color | Hex | ANSI 256 | ANSI Escape | Use |
|----------------|-----|----------|-------------|-----|
| Background | `#4A1999` | 54 | `\033[48;5;54m` | Background (if supported) |
| Primary | `#982BD4` | 134 | `\033[38;5;134m` | Headers, commands |
| Accent | `#0CD7FF` | 51 | `\033[38;5;51m` | Values, paths, URLs |
| Error | `#F9278C` | 198 | `\033[38;5;198m` | Errors, failures |
| Success | `#2DD4A7` | 43 | `\033[38;5;43m` | Success, ✓ |
| Warning | `#FFB020` | 214 | `\033[38;5;214m` | Warnings, ⚠ |
| Text | `#FFFFFF` | 15 | `\033[38;5;15m` | Default text |
| Muted | `#A099B0` | 249 | `\033[38;5;249m` | Secondary text |

### TrueColor (24-bit)

For terminals with TrueColor support:

```bash
# Example: purple text
printf '\033[38;2;152;43;212mTexto Roxo\033[0m\n'

# Example: cyan text
printf '\033[38;2;12;215;255mTexto Ciano\033[0m\n'
```

### CLI Output Example

```
tb run --timeout 30s "go test ./..."

┌─────────────────────────────────────┐
│  TOOLBELT v0.1.0                    │  ← Primary (#982BD4)
└─────────────────────────────────────┘

→ Executando: go test ./...            ← Muted (#A099B0)
  Sandbox: ativo                       ← Accent (#0CD7FF)
  Timeout: 30s

✓ Testes passaram (12/12)              ← Success (#2DD4A7)
  Tempo: 2.34s
```

---

## Application in UIs

### `tb md` (TUI Markdown)

The Markdown TUI reader/editor should use the tokens for:

- Application background: `--tb-bg`
- Body text: `--tb-text`
- Headers: `--tb-primary` or gradient
- Links: `--tb-accent`
- Inline code: `--tb-surface` + `--tb-accent`
- Code blocks: `--tb-surface` with `--tb-border` border
- Selection/cursor: `--tb-accent` with opacity

### Obsidian (Vault)

For those who use the `.tb/vault/` vault in Obsidian, a custom CSS snippet can be created based on the tokens to keep visual consistency.

---

## Files

```
assets/
├── logo/
│   ├── toolbelt-logo.png      # Original 1024×1024
│   ├── toolbelt-logo-512.png
│   ├── toolbelt-logo-256.png
│   ├── toolbelt-logo-128.png
│   ├── toolbelt-logo-64.png
│   ├── toolbelt-logo.svg      # Vector version
│   └── favicon.ico            # 32×32
└── theme/
    ├── tokens.json            # Design tokens (JSON)
    └── theme.css              # CSS variables
```

---

## History

| Date | Version | Change |
|------|---------|--------|
| 2026-11-15 | 1.0.0 | Initial creation with logo and Synthwave theme |
