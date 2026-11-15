# Identidade Visual do Toolbelt

<p align="center">
  <img src="../assets/logo/toolbelt-logo.png" alt="Logo do Toolbelt" width="256">
</p>

O toolbelt adota o tema **Synthwave** como identidade visual oficial, inspirado na estética retro-futurista dos anos 80 com tons vibrantes de roxo, magenta, rosa e ciano.

---

## Logo

### Versões Disponíveis

| Arquivo | Tamanho | Uso |
|---------|---------|-----|
| `toolbelt-logo.png` | 1024×1024 | Original, alta resolução |
| `toolbelt-logo-512.png` | 512×512 | Documentação, web |
| `toolbelt-logo-256.png` | 256×256 | README, avatares |
| `toolbelt-logo-128.png` | 128×128 | Ícones médios |
| `toolbelt-logo-64.png` | 64×64 | Ícones pequenos |
| `toolbelt-logo.svg` | Vetorial | Escalável, impressão |
| `favicon.ico` | 32×32 | Favicon web |

### Elementos do Logo

O logo representa um cinto de ferramentas estilizado com elementos que simbolizam o projeto:

1. **Círculo violeta** (#4A1999): fundo que representa o ambiente de execução
2. **Cinto roxo com pontas escuras**: a estrutura unificadora das ferramentas
3. **Bolsinha com degradê magenta→rosa**: o pocket/bolso de armazenamento (vault)
4. **Botão branco**: ponto focal, simplicidade
5. **Colchetes `< >`**: representam código e a CLI
6. **Robô ciano**: o agente de IA que utiliza as ferramentas

### Regras de Uso

#### Área de Respiro

Mantenha uma margem mínima igual a **10% do tamanho do logo** em todos os lados.

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

#### Tamanho Mínimo

| Contexto | Tamanho Mínimo |
|----------|----------------|
| Digital (web, app) | 32px |
| Impresso | 12mm |
| Favicon | 16px (use o ICO específico) |

#### Fundo Permitido

| Cor de Fundo | Versão do Logo |
|--------------|----------------|
| Escuro (#29282D ou mais escuro) | Logo padrão ✓ |
| Violeta (#4A1999) | Logo padrão ✓ |
| Claro (#F5F3FA ou branco) | Logo padrão ✓ (círculo violeta cria contraste) |

**Proibido:**
- Esticar ou distorcer proporções
- Aplicar efeitos (sombra, brilho) que alterem as cores
- Usar sobre fundos com padrões complexos
- Alterar as cores do logo

---

## Paleta de Cores

### Cores Primárias (extraídas do logo)

| Nome | Hex | RGB | Uso |
|------|-----|-----|-----|
| **Violeta** | `#4A1999` | 74, 25, 153 | Fundo principal, cor institucional |
| **Roxo** | `#982BD4` | 152, 43, 212 | Primária, botões, links |
| **Roxo Sombra** | `#761DC5` | 118, 29, 197 | Hover, bordas, sombras |
| **Magenta** | `#BE2AC6` | 190, 42, 198 | Início do degradê, destaques |
| **Rosa** | `#F9278C` | 249, 39, 140 | Fim do degradê, erros, alertas |
| **Ciano** | `#0CD7FF` | 12, 215, 255 | Accent, links, interativo |
| **Azul Escuro** | `#00579A` | 0, 87, 154 | Ciano escuro, contraste |
| **Quase Preto** | `#29282D` | 41, 40, 45 | Superfícies, texto em fundo claro |
| **Branco** | `#FFFFFF` | 255, 255, 255 | Texto em fundo escuro |

### Cores Semânticas

| Propósito | Modo Escuro | Modo Claro |
|-----------|-------------|------------|
| Fundo | `#4A1999` | `#F5F3FA` |
| Superfície | `#29282D` | `#FFFFFF` |
| Texto principal | `#FFFFFF` | `#29282D` |
| Texto secundário | `#A099B0` | `#6B6680` |
| Sucesso | `#2DD4A7` | `#1AA87A` |
| Erro | `#F9278C` | `#D91A6B` |
| Aviso | `#FFB020` | `#D99000` |

### Degradês

| Nome | Definição | Uso |
|------|-----------|-----|
| **Primary** | `linear-gradient(180deg, #BE2AC6 → #F9278C)` | Destaque principal, CTAs |
| **Belt** | `linear-gradient(90deg, #29282D → #761DC5 → #982BD4 → #761DC5 → #29282D)` | Decorativo, headers |
| **Accent** | `linear-gradient(135deg, #0CD7FF → #00579A)` | Accent secundário |

---

## Contraste WCAG

Combinações de texto verificadas para acessibilidade:

### Modo Escuro (fundo violeta #4A1999)

| Texto | Fundo | Ratio | WCAG AA | WCAG AAA |
|-------|-------|-------|---------|----------|
| Branco `#FFFFFF` | Violeta `#4A1999` | 8.5:1 | ✅ Pass | ✅ Pass |
| Ciano `#0CD7FF` | Violeta `#4A1999` | 7.2:1 | ✅ Pass | ✅ Pass |
| Rosa `#F9278C` | Violeta `#4A1999` | 4.6:1 | ✅ Pass | ❌ Fail |
| Roxo `#982BD4` | Violeta `#4A1999` | 2.1:1 | ❌ Fail | ❌ Fail |

### Modo Escuro (superfície #29282D)

| Texto | Fundo | Ratio | WCAG AA | WCAG AAA |
|-------|-------|-------|---------|----------|
| Branco `#FFFFFF` | Quase Preto `#29282D` | 14.3:1 | ✅ Pass | ✅ Pass |
| Ciano `#0CD7FF` | Quase Preto `#29282D` | 11.8:1 | ✅ Pass | ✅ Pass |
| Rosa `#F9278C` | Quase Preto `#29282D` | 7.5:1 | ✅ Pass | ✅ Pass |
| Roxo `#982BD4` | Quase Preto `#29282D` | 3.4:1 | ✅ Pass (large) | ❌ Fail |

### Modo Claro (fundo #F5F3FA)

| Texto | Fundo | Ratio | WCAG AA | WCAG AAA |
|-------|-------|-------|---------|----------|
| Quase Preto `#29282D` | Claro `#F5F3FA` | 12.8:1 | ✅ Pass | ✅ Pass |
| Violeta `#4A1999` | Claro `#F5F3FA` | 7.6:1 | ✅ Pass | ✅ Pass |
| Roxo `#982BD4` | Claro `#F5F3FA` | 4.1:1 | ✅ Pass | ❌ Fail |
| Azul Escuro `#00579A` | Claro `#F5F3FA` | 5.8:1 | ✅ Pass | ✅ Pass (large) |

### Recomendações

- **Texto de corpo**: use Branco sobre Violeta/Quase Preto ou Quase Preto sobre fundo claro
- **Links e interativos**: Ciano sobre fundos escuros, Azul Escuro sobre fundos claros
- **Alertas/Erros**: Rosa é acessível sobre superfícies (#29282D), mas não sobre Violeta para texto pequeno
- **Evitar**: Roxo como cor de texto (baixo contraste); use apenas para elementos decorativos ou grandes

---

## Tokens de Design

Os tokens estão disponíveis em dois formatos:

### JSON (Design Tokens Format)

```
assets/theme/tokens.json
```

Segue o padrão [Design Tokens](https://design-tokens.github.io/community-group/format/) para integração com ferramentas como Figma, Style Dictionary, etc.

### CSS Custom Properties

```
assets/theme/theme.css
```

Variáveis CSS prontas para uso:

```css
/* Importar no projeto */
@import 'assets/theme/theme.css';

/* Usar variáveis */
.meu-componente {
  background: var(--tb-bg);
  color: var(--tb-text);
  border-radius: var(--tb-radius-md);
}

/* Degradê do logo */
.destaque {
  background: var(--tb-gradient-primary);
}
```

### Nomenclatura das Variáveis CSS

| Prefixo | Categoria | Exemplos |
|---------|-----------|----------|
| `--tb-color-*` | Cores base | `--tb-color-violeta`, `--tb-color-ciano` |
| `--tb-gradient-*` | Degradês | `--tb-gradient-primary`, `--tb-gradient-belt` |
| `--tb-bg*` | Fundos | `--tb-bg`, `--tb-bg-secondary` |
| `--tb-surface*` | Superfícies | `--tb-surface`, `--tb-surface-elevated` |
| `--tb-primary*` | Cor primária | `--tb-primary`, `--tb-primary-hover` |
| `--tb-accent*` | Cor accent | `--tb-accent`, `--tb-accent-dark` |
| `--tb-text*` | Texto | `--tb-text`, `--tb-text-muted` |
| `--tb-spacing-*` | Espaçamento | `--tb-spacing-sm`, `--tb-spacing-lg` |
| `--tb-radius-*` | Bordas | `--tb-radius-md`, `--tb-radius-full` |
| `--tb-font-*` | Tipografia | `--tb-font-mono`, `--tb-font-size-md` |
| `--tb-shadow-*` | Sombras | `--tb-shadow-sm`, `--tb-shadow-lg` |
| `--tb-error/success/warning` | Status | `--tb-error`, `--tb-success` |

---

## CLI: Mapeamento ANSI

A saída colorida do CLI (`tb`) deve seguir a mesma paleta. Use os códigos abaixo:

### Tabela de Mapeamento

| Cor Semântica | Hex | ANSI 256 | ANSI Escape | Uso |
|---------------|-----|----------|-------------|-----|
| Background | `#4A1999` | 54 | `\033[48;5;54m` | Fundo (se suportado) |
| Primary | `#982BD4` | 134 | `\033[38;5;134m` | Headers, comandos |
| Accent | `#0CD7FF` | 51 | `\033[38;5;51m` | Valores, paths, URLs |
| Error | `#F9278C` | 198 | `\033[38;5;198m` | Erros, falhas |
| Success | `#2DD4A7` | 43 | `\033[38;5;43m` | Sucesso, ✓ |
| Warning | `#FFB020` | 214 | `\033[38;5;214m` | Avisos, ⚠ |
| Text | `#FFFFFF` | 15 | `\033[38;5;15m` | Texto padrão |
| Muted | `#A099B0` | 249 | `\033[38;5;249m` | Texto secundário |

### TrueColor (24-bit)

Para terminais com suporte a TrueColor:

```bash
# Exemplo: texto roxo
printf '\033[38;2;152;43;212mTexto Roxo\033[0m\n'

# Exemplo: texto ciano
printf '\033[38;2;12;215;255mTexto Ciano\033[0m\n'
```

### Exemplo de Output CLI

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

## Aplicação em UIs

### `tb md` (TUI Markdown)

O leitor/editor Markdown TUI deve usar os tokens para:

- Fundo da aplicação: `--tb-bg`
- Texto de corpo: `--tb-text`
- Headers: `--tb-primary` ou degradê
- Links: `--tb-accent`
- Código inline: `--tb-surface` + `--tb-accent`
- Blocos de código: `--tb-surface` com borda `--tb-border`
- Seleção/cursor: `--tb-accent` com opacidade

### Obsidian (Vault)

Para quem usar o vault `.tb/vault/` no Obsidian, um snippet CSS personalizado pode ser criado baseado nos tokens para manter consistência visual.

---

## Arquivos

```
assets/
├── logo/
│   ├── toolbelt-logo.png      # Original 1024×1024
│   ├── toolbelt-logo-512.png
│   ├── toolbelt-logo-256.png
│   ├── toolbelt-logo-128.png
│   ├── toolbelt-logo-64.png
│   ├── toolbelt-logo.svg      # Versão vetorial
│   └── favicon.ico            # 32×32
└── theme/
    ├── tokens.json            # Design tokens (JSON)
    └── theme.css              # Variáveis CSS
```

---

## Histórico

| Data | Versão | Alteração |
|------|--------|-----------|
| 2026-11-15 | 1.0.0 | Criação inicial com logo e tema Synthwave |
