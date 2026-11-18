🇺🇸 English | [🇧🇷 Português](docs/pt-BR/CONTRIBUTING.md)

# Contributing to toolbelt

Thanks for your interest in contributing! This document explains how to take part in development.

## Project Status

toolbelt is under active development, pre-1.0 (latest release: v0.0.5). What has shipped is in [CHANGELOG.md](CHANGELOG.md); decisions are in [docs/adr/](docs/adr/README.md).

## Branch and PR Workflow

### Branches

```
main                 # main branch, always stable
├── feature/*        # new features
├── fix/*            # bug fixes
├── docs/*           # documentation changes
└── adr/*            # ADR proposals
```

### Workflow

1. Fork the repository
2. Create a branch from `main`:
   ```bash
   git checkout -b feature/minha-feature
   ```
3. Make your changes
4. Commit following Conventional Commits
5. Push to your fork
6. Open a Pull Request against `main`

### PR Review

- Every PR needs at least one approval
- CI must pass (once implemented)
- PRs should be small and focused
- Clearly describe what the PR does

## Conventional Commits

We use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages:

```
<type>[optional scope]: <description>

[optional body]

[optional footer]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `style` | Formatting, no code change |
| `refactor` | Refactoring without behavior change |
| `test` | Adding or fixing tests |
| `chore` | Maintenance, build, CI |
| `adr` | ADR proposal or update |

### Examples

```bash
feat(graph): add callers query

docs(adr): add ADR-007 for vector index

fix(sandbox): handle missing cgroup controller

chore(ci): add golangci-lint workflow
```

## How to Propose ADRs

ADRs (Architecture Decision Records) document important technical decisions.

### When to Create an ADR

- Decision that affects multiple components
- Significant trade-off between alternatives
- Technology or library choice
- Architectural design change

### Process

1. Create the file at `docs/adr/NNN-titulo.md`
2. Use the template in [docs/adr/README.md](docs/adr/README.md)
3. Initial status: `proposed`
4. Open a PR on branch `adr/NNN-titulo`
5. Discuss in the PR
6. After approval, change status to `accepted`

### Short Template

```markdown
# ADR-NNN: Title

## Status
proposed

## Context
Why is this decision needed?

## Decision
What was decided?

## Consequences
Positive and negative.

## Alternatives Considered
What else was evaluated?
```

## Development Setup

### Requirements

| Tool | Version | Use |
|------|---------|-----|
| Go | per `go.mod` (currently 1.27) | Binary compilation (`CGO_ENABLED=0`; SQLite via `modernc.org/sqlite`) |
| golangci-lint | recent | Lint (`make lint`; CI runs it) |
| Linux with Landlock | kernel ≥ 5.13 | Sandbox (fail-closed) |

### Build

```bash
# Clone
git clone https://github.com/leonardo-moraes360/toolbelt.git
cd toolbelt

# Build (produces bin/tb and bin/fake-agent)
make build

# Tests
make test

# Lint
make lint

# fmt + vet + lint + test
make check
```

### Directory Structure

```
toolbelt/
├── cmd/
│   ├── tb/              # CLI entrypoint (cobra)
│   └── fake-agent/      # fake agent for the bench
├── internal/            # bench bus busrun config execenv git gitref graph guard
│                        # log mcp mem merge registry review router sandbox stack
│                        # ticket verify version workspace
├── bench/               # benchmark tasks, fixtures and A/B
├── site/                # static site (GitHub Pages)
└── docs/                # English; translations in docs/<language_code>/ (pt-BR)
    ├── arquitetura.md
    ├── cli.md
    ├── benchmark.md
    └── adr/
```

## Code Style

### Go

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` / `goimports`
- Lint with `golangci-lint`
- Tests alongside code (`foo_test.go`)

### Documentation

- Markdown for docs
- English is the **source of truth**; only English docs live at the root. The Brazilian Portuguese translation lives under `docs/pt-BR/` (same relative path; root docs go straight under it)
- When changing English docs, update the translation in the same PR (see [docs/TRANSLATION.md](docs/TRANSLATION.md))
- English for code (variable names, technical comments)
- Mermaid diagrams when possible

### Commits

- Messages in English
- One logical change per commit
- Small, frequent commits

## Communication

- **Issues**: bugs, features, discussions
- **PRs**: code review and technical discussion
- **Discussions** (if enabled): general questions

## Code of Conduct

This project follows the [Contributor Covenant 2.1](CODE_OF_CONDUCT.md). By participating, you agree to follow its guidelines.

## License

By contributing, you agree that your contributions will be licensed under the same project license ([Apache 2.0](LICENSE)).
