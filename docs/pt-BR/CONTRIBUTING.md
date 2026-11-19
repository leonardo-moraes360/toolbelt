[🇺🇸 English](../../CONTRIBUTING.md) | 🇧🇷 Português

# Contribuindo para o toolbelt

Obrigado pelo interesse em contribuir! Este documento explica como participar do desenvolvimento.

## Status do Projeto

O toolbelt está em desenvolvimento ativo, pré-1.0 (última versão: v0.0.5). O que já foi lançado está no [CHANGELOG.md](../../CHANGELOG.md); decisões em [docs/adr/](adr/README.md).

## Fluxo de Branches e PRs

### Branches

```
main                 # branch principal, sempre estável
├── feature/*        # novas funcionalidades
├── fix/*            # correções de bugs
├── docs/*           # mudanças em documentação
└── adr/*            # propostas de ADR
```

### Fluxo de Trabalho

1. Fork o repositório
2. Crie um branch a partir de `main`:
   ```bash
   git checkout -b feature/minha-feature
   ```
3. Faça suas mudanças
4. Commit seguindo Conventional Commits
5. Push para seu fork
6. Abra um Pull Request para `main`

### Revisão de PRs

- Todo PR precisa de pelo menos uma aprovação
- CI deve passar (quando implementado)
- PRs devem ser pequenos e focados
- Descreva claramente o que o PR faz

## Conventional Commits

Usamos [Conventional Commits](https://www.conventionalcommits.org/) para mensagens de commit:

```
<tipo>[escopo opcional]: <descrição>

[corpo opcional]

[rodapé opcional]
```

### Tipos

| Tipo | Descrição |
|------|-----------|
| `feat` | Nova funcionalidade |
| `fix` | Correção de bug |
| `docs` | Mudanças em documentação |
| `style` | Formatação, sem mudança de código |
| `refactor` | Refatoração sem mudança de comportamento |
| `test` | Adição ou correção de testes |
| `chore` | Manutenção, build, CI |
| `adr` | Proposta ou atualização de ADR |

### Exemplos

```bash
feat(graph): add callers query

docs(adr): add ADR-007 for vector index

fix(sandbox): handle missing cgroup controller

chore(ci): add golangci-lint workflow
```

## Como Propor ADRs

ADRs (Architecture Decision Records) documentam decisões técnicas importantes.

### Quando Criar um ADR

- Decisão que afeta múltiplos componentes
- Trade-off significativo entre alternativas
- Escolha de tecnologia ou biblioteca
- Mudança de design arquitetural

### Processo

1. Crie o arquivo em `docs/adr/NNN-titulo.md`
2. Use o template em [docs/pt-BR/adr/README.md](adr/README.md)
3. Status inicial: `proposed`
4. Abra PR no branch `adr/NNN-titulo`
5. Discussão no PR
6. Após aprovação, mude status para `accepted`

### Template Resumido

```markdown
# ADR-NNN: Título

## Status
proposed

## Contexto
Por que essa decisão é necessária?

## Decisão
O que foi decidido?

## Consequências
Positivas e negativas.

## Alternativas Consideradas
O que mais foi avaliado?
```

## Setup de Desenvolvimento

### Requisitos

| Ferramenta | Versão | Uso |
|------------|--------|-----|
| Go | conforme `go.mod` (hoje 1.27) | Compilação do binário (`CGO_ENABLED=0`; SQLite via `modernc.org/sqlite`) |
| golangci-lint | recente | Lint (`make lint`; o CI roda) |
| Linux com Landlock | kernel ≥ 5.13 | Sandbox (fail-closed) |

### Build

```bash
# Clonar
git clone https://github.com/leonardo-moraes360/toolbelt.git
cd toolbelt

# Build (gera bin/tb e bin/fake-agent)
make build

# Testes
make test

# Lint
make lint

# fmt + vet + lint + test
make check
```

### Estrutura de Diretórios

```
toolbelt/
├── cmd/
│   ├── tb/              # entrada do CLI (cobra)
│   └── fake-agent/      # agente fake do bench
├── internal/            # bench bus busrun config execenv git gitref graph guard
│                        # log mcp mem merge registry review router sandbox stack
│                        # ticket verify version workspace
├── bench/               # tarefas, fixtures e A/B do benchmark
├── site/                # site estático (GitHub Pages)
└── docs/                # inglês; traduções em docs/<language_code>/ (pt-BR)
    ├── arquitetura.md
    ├── cli.md
    ├── benchmark.md
    └── adr/
```

## Estilo de Código

### Go

- Siga [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` / `goimports`
- Lint com `golangci-lint`
- Testes junto com código (`foo_test.go`)

### Documentação

- Markdown para docs
- Inglês é a **fonte de verdade**; na raiz ficam só as docs em inglês. A tradução em português fica em `docs/pt-BR/` (mesmo caminho relativo; docs da raiz vão direto nela)
- Ao mudar docs em inglês, atualize a tradução no mesmo PR (ver [docs/TRANSLATION.md](../TRANSLATION.md))
- Inglês para código (nomes de variáveis, comentários técnicos)
- Diagramas em Mermaid quando possível

### Commits

- Mensagens em inglês
- Uma mudança lógica por commit
- Commits pequenos e frequentes

## Comunicação

- **Issues**: bugs, features, discussões
- **PRs**: code review e discussão técnica
- **Discussions** (se habilitado): perguntas gerais

## Código de Conduta

Este projeto segue o [Contributor Covenant 2.1](CODE_OF_CONDUCT.md). Ao participar, você concorda em seguir suas diretrizes.

## Licença

Ao contribuir, você concorda que suas contribuições serão licenciadas sob a mesma licença do projeto ([Apache 2.0](../../LICENSE)).
