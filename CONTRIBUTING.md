🇧🇷 Português | [🇺🇸 English](CONTRIBUTING.en.md)

# Contribuindo para o toolbelt

Obrigado pelo interesse em contribuir! Este documento explica como participar do desenvolvimento.

## Status do Projeto

O toolbelt está em **fase de design**. Atualmente estamos definindo a arquitetura e documentação. Código de implementação ainda não foi iniciado.

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
2. Use o template em [docs/adr/README.md](docs/adr/README.md)
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

> ⚠️ **TBD**: O setup completo será documentado quando a implementação iniciar.

### Requisitos Planejados

| Ferramenta | Versão | Uso |
|------------|--------|-----|
| Go | 1.22+ | Compilação do binário |
| tree-sitter | latest | Parsing de código |
| SQLite | 3.x | Armazenamento de grafo |
| bubblewrap | latest | Sandbox (Linux) |

### Build (quando disponível)

```bash
# Clonar
git clone https://github.com/leonardo-moraes360/toolbelt.git
cd toolbelt

# Build
go build -o tb ./cmd/tb

# Testes
go test ./...

# Lint
golangci-lint run
```

### Estrutura de Diretórios Planejada

```
toolbelt/
├── cmd/
│   └── tb/              # entrada do CLI
├── internal/
│   ├── graph/           # motor de grafo
│   ├── mem/             # gerenciamento de memória
│   ├── sandbox/         # isolamento
│   ├── proc/            # gerenciamento de processos
│   ├── agent/           # orquestração de agentes
│   └── mcp/             # servidor MCP
├── pkg/
│   └── ...              # bibliotecas públicas
├── docs/
│   ├── arquitetura.md
│   ├── cli.md
│   ├── benchmark.md
│   └── adr/
└── tests/
    ├── integration/
    └── benchmark/
```

## Estilo de Código

### Go

- Siga [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` / `goimports`
- Lint com `golangci-lint`
- Testes junto com código (`foo_test.go`)

### Documentação

- Markdown para docs
- Português brasileiro é a **fonte de verdade**; espelho em inglês em `docs/en/` (e `*.en.md` na raiz)
- Ao mudar docs em PT, atualize o par em inglês no mesmo PR (ver [docs/en/TRANSLATION.md](docs/en/TRANSLATION.md))
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

Ao contribuir, você concorda que suas contribuições serão licenciadas sob a mesma licença do projeto ([Apache 2.0](LICENSE)).
