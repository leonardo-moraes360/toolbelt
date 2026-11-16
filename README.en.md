[🇧🇷 Português](README.md) | 🇺🇸 English

<p align="center">
  <img src="assets/logo/toolbelt-logo-256.png" alt="Toolbelt Logo" width="200">
</p>

<h1 align="center">toolbelt (<code>tb</code>)</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white" alt="Go Version"></a>
  <a href="docs/en/arquitetura.md"><img src="https://img.shields.io/badge/Status-Phase%202-green" alt="Status"></a>
  <a href="https://github.com/leonardo-moraes360/toolbelt/actions"><img src="https://github.com/leonardo-moraes360/toolbelt/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/Protocol-MCP-purple" alt="MCP"></a>
  <a href="docs/en/identidade-visual.md"><img src="https://img.shields.io/badge/Theme-Synthwave-ff278c" alt="Theme"></a>
</p>

<p align="center">
  A toolbelt for AI coding agents, unified under a single command and protocol.
</p>

## 🎯 Goals

| Goal | How toolbelt helps |
|------|-------------------|
| **Save tokens** | Queryable knowledge graph avoids grepping files; context assembled under a token budget; compact memory with summaries and extracted facts |
| **Improve accuracy** | Isolated sandbox with bubblewrap+Landlock; capability-based permissions; anti-cheat protections; separation between implementer and evaluator |
| **Increase speed** | Parallel execution with goroutines; agent hierarchy (planner→supervisor→worker); fast graph search with FTS5 |

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Human Interface                          │
│  ┌─────────┐    ┌─────────────┐    ┌─────────────────────────┐  │
│  │ tb CLI  │    │ tb md (TUI) │    │ Obsidian (external vault)│  │
│  └────┬────┘    └──────┬──────┘    └────────────┬────────────┘  │
└───────┼────────────────┼───────────────────────┼────────────────┘
        │                │                       │
        ▼                ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Protocols                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ MCP Server  │  │ A2A Protocol│  │ JSON-RPC 2.0 (stdio/UDS)│  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                           Core                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌───────────┐  │
│  │Orchestrator│  │  Memory    │  │  Context   │  │   Graph   │  │
│  └────────────┘  └────────────┘  └────────────┘  └───────────┘  │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Storage                                    │
│  ┌─────────────────────┐    ┌─────────────────────────────────┐ │
│  │ Markdown Vault      │───▶│ SQLite + FTS5 (derived index)   │ │
│  │ (source of truth)   │    │ + edge tables                   │ │
│  └─────────────────────┘    └─────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Execution                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Worktrees   │  │ bubblewrap  │  │ cgroups (CPU/RAM/time)  │  │
│  │ Git         │  │ +Landlock   │  │                         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

See the full documentation in [docs/en/arquitetura.md](docs/en/arquitetura.md).

## 🌐 Site

Public landing: [https://leonardo-moraes360.github.io/toolbelt/](https://leonardo-moraes360.github.io/toolbelt/) (source in [`site/`](site/); guide in [`docs/en/site.md`](docs/en/site.md)).

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [docs/en/arquitetura.md](docs/en/arquitetura.md) | Detailed architecture with Mermaid diagrams |
| [docs/en/cli.md](docs/en/cli.md) | `tb` command tree (implemented + planned) |
| [docs/en/benchmark.md](docs/en/benchmark.md) | KPIs and evaluation methodology |
| [docs/en/site.md](docs/en/site.md) | Landing page and GitHub Pages deploy |
| [docs/en/adr/](docs/en/adr/) | Architecture Decision Records (ADRs) |
| [docs/en/pesquisa/](docs/en/pesquisa/) | Primary sources, top 10, and research findings (2026-11) |
| [docs/en/campo/SINTESE.md](docs/en/campo/SINTESE.md) | Field study across 20 repos + improvement ranking |
| [docs/en/identidade-visual.md](docs/en/identidade-visual.md) | Logo, Synthwave palette, and theme tokens |

## 🚀 Installation

### Binaries (GitHub Releases)

Download the archive from the [Releases page](https://github.com/leonardo-moraes360/toolbelt/releases) for your OS/arch (`linux`, `darwin`, or `windows` × `amd64`/`arm64`), extract it, and put `tb` on your `PATH`:

```bash
# Example (Linux amd64, adjust tag/archive)
curl -sL "https://github.com/leonardo-moraes360/toolbelt/releases/download/v0.0.1/toolbelt_0.0.1_linux_amd64.tar.gz" \
  | tar -xz tb
sudo mv tb /usr/local/bin/
tb version
```

Verify integrity with `checksums.txt` from the same release.

### Via Go

```bash
go install github.com/leonardo-moraes360/toolbelt/cmd/tb@latest
```

> `go install` uses the default embedded version (`dev`) unless the tag/module injects ldflags. Prefer the Release binary if you need the tag version.

### Via Homebrew (coming soon)

```bash
brew install leonardo-moraes360/tap/toolbelt
```

### Local build

```bash
git clone https://github.com/leonardo-moraes360/toolbelt.git
cd toolbelt
make build
./bin/tb version
```

## 📖 Quick Start

`tb` is **100% local**: it **does not talk to GitHub**. `review` / `drift` /
`verify --ticket` / `stack` read an `rd.md` (or `--ticket`) from disk. To
start from an issue/PR, export the text with `gh issue view` / `gh pr view` and
build the file (scope, forbidden, acceptance) — details in
[docs/en/cli.md](docs/en/cli.md#local-first-sem-github).

```bash
# Initializes local config (detects toolchain → test/build/lint_cmd)
tb init

# If AGENTS.md/CLAUDE.md exists, init reads and registers short command hints
# (filters long fences; never overwrites). Seed AGENTS.md citing test_cmd
# (and build/lint if present):
tb init --write-agents

# Shows version
tb version

# Lists available benchmark tasks
tb bench list

# Runs benchmark with built-in fake agent (works in any repo)
tb bench run --fake

# Runs benchmark with external agent ({prompt} with safe quoting)
tb bench run --agent "cursor-agent {prompt} --workdir {workdir}" -r 3

# Without --agent: uses bench.default_agent from .toolbelt/config.yaml
# Without -t/-f: uses bench.tasks_dir / fixtures_dir (CLI flag > YAML > default)
tb bench run -r 1

# Generates report from existing results
tb bench report --input bench/results/2026-01-01-120000.jsonl

# Compares two benchmarks (success, time, tokens)
tb bench compare --before results-v1.jsonl --after results-v2.jsonl

# A/B comparison between configs/feature flags
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml --fake

# Drift/review against local ticket (no issue/PR fetch)
tb drift --ticket rd.md
tb review --ticket rd.md
tb guard check HEAD~1..HEAD
```

## 🗺️ Roadmap / Status

### Phase 0 (completed) ✅
- [x] CLI skeleton with Cobra (`tb version`, `tb init`, `tb help`)
- [x] Hierarchical configuration (global + local)
- [x] Structured logging with slog and Synthwave theme
- [x] `tb bench` harness to measure baseline
- [x] Benchmark fixtures
- [x] Deterministic fake agent for CI
- [x] GitHub Actions CI
- [x] Makefile, golangci-lint, goreleaser
- [x] GitHub Releases via GoReleaser (`v*` tags)

### Phase 1 (completed) ✅
- [x] `tb mcp` — stateless MCP server
- [x] Worktrees + Landlock sandbox
- [x] `tb guard` / `tb verify` (tests ruler)
- [x] `tb mem` — invariants outside compaction (`show`/`edit`/`retire`/`search` + `--ref`)

### Phase 2 (in progress)
- [x] `tb bench compare` A/B (configs/feature flags, token/time/success metrics)
- [x] On-demand `tb graph` (CLI + MCP) with A/B graph-on vs graph-off
- [x] `tb review` — fresh-context review against ticket/rd.md + invariants (+ `--approved-sha` / patch-id)
- [x] `tb drift` — drift detection (scope / base commit / pinned patch)
- [x] Markdown Scope parser (`## Scope` / lists) and globs with mid-path `**`
- [x] Visual ruler: screenshot + headless browser (graceful skip; `skipped` aggregated in report)
- [x] CLI docs aligned with real commands/flags (`docs/cli.md` / README)
- [x] MCP with field-loop parity (`mem` / `verify` / `review` / `drift` / `guard`)
- [x] `tb stack` — stacks verify/review/drift/guard with shared diff/index/memory (+ MCP `tb_stack`)
- [x] Landing page in [`site/`](site/) + GitHub Pages ([docs/en/site.md](docs/en/site.md) · [live site](https://leonardo-moraes360.github.io/toolbelt/))
- [ ] Agent hierarchy / A2A / `tb md` (later)

### Input for the next phase (field study)
- Field study of `tb` across 20 public repositories (reports in [`docs/en/campo/`](docs/en/campo/) + improvement synthesis in [`docs/en/campo/SINTESE.md`](docs/en/campo/SINTESE.md)), as prioritized input for upcoming product PRs.

### Phase 3
- [ ] Router/Workers for scalable task delegation
- [ ] Direct delivery — follow-ups go straight to the owning worker
- [ ] `tb ticket` / `tb board` — work ticket management
- [ ] `tb router` / `tb watchdog` — delegation control and health
- [ ] `tb merge` — aggregation of worker results
- [ ] Optional vector index
- [ ] Graphify integration (or reimplementation)
- [ ] Elephant-Goldfish Model integration (or reimplementation)
- [ ] Optional Rust components for local inference
- [ ] Additional rulers for `tb verify`

## 🤝 Contributing

See [CONTRIBUTING.en.md](CONTRIBUTING.en.md) for details on:
- Branch and PR workflow
- Conventional Commits
- How to propose ADRs
- Development setup

## 📖 Inspirations

This project was inspired by:
- **[Graphify](https://github.com/Graphify-Labs/graphify)**: mapping projects into queryable knowledge graphs
- **[Obsidian](https://obsidian.md/)**: Markdown vault with wikilinks, backlinks, and a local-first graph
- **[Elephant-Goldfish Model](https://github.com/google/elephant-goldfish-model)**: review with fresh contexts, author/reviewer separation, drift detection

See the ADRs for details:
- [ADR-006: Graphify and Obsidian](docs/en/adr/006-inspiracoes-graphify-obsidian.md)
- [ADR-007: Elephant-Goldfish Model](docs/en/adr/007-inspiracao-elephant-goldfish.md)
- [ADR-008: Adversarial Verification](docs/en/adr/008-verificacao-adversarial.md)
- [ADR-009: Router and Workers](docs/en/adr/009-router-e-workers.md)

## 📄 License

[Apache License 2.0](LICENSE) — Copyright 2026 Leonardo Moraes

## 📊 Project Status

This project is in **Phase 2**: functional CLI with verify/guard/mem/graph/review/drift/MCP and a benchmark harness. The full architecture is documented in [docs/en/arquitetura.md](docs/en/arquitetura.md).

### Implemented Commands

| Command | Status | Description |
|---------|--------|-------------|
| `tb version` | ✅ | Shows version, commit, and build time |
| `tb init` | ✅ | Initializes local config; detects toolchain (`verify.test_cmd` with `PATH` check/fallback; Node/TS via lockfiles + `test`/`build`|`typecheck`/`lint` scripts; no marker → empty `test_cmd` with warning, never `go test` without `go.mod`; `build_cmd` / `lint_cmd`), reads `AGENTS.md`/`CLAUDE.md` (filtered hints) and optionally seeds with `--write-agents` (seed cites detected test/build/lint) |
| `tb bench run` | ✅ | Runs benchmark |
| `tb bench list` | ✅ | Lists available tasks |
| `tb bench report` | ✅ | Generates Markdown report |
| `tb bench compare` | ✅ | Compares two benchmarks |
| `tb sandbox` | ✅ | Worktrees + Landlock (Linux) |
| `tb guard` | ✅ | Anti-cheat check on diff (strict by default / exit ≠ 0 on `SeverityError`; `--warn-only` opt-out; `--strict` deprecated) |
| `tb verify` | ✅ | `tests` + `visual` rulers (`tb verify run`; hidden suite via `--hidden-test-cmd` / `verify.hidden_test_cmd` without requiring `tests/hidden/`; manifesto; optional `verify.build_cmd` / `verify.lint_cmd`; fail-fast if command missing/not executable; custom `protected_paths` from `guard.yaml` do not abort the loop — Scope exception via `--ticket`) |
| `tb mem` | ✅ | Invariants (`add`/`list`/`show`/`edit`/`retire`/`search`/`check`/`init`; `--ref` ADR; soft-retire + `--all`) |
| `tb graph` | ✅ | On-demand multi-language graph + frameworks + docs (ADR-011); `javascript` covers TypeScript (`ts`/`tsx`) |
| `tb review` | ✅ | Fresh-context review against local ticket/`rd.md` (no GitHub); `--approved-sha` |
| `tb drift` | ✅ | Scope / base commit / pinned patch drift against `rd.md` (not vault vs code) |
| `tb stack` | ✅ | Stacks verify/review/drift/guard with shared context (aggregated JSON; MCP `tb_stack`) |
| `tb workspace` | ✅ | Isolated workspaces via git worktree |
| `tb router` | 🚧 | Stub (not implemented) |
| `tb mcp` | ✅ | Stateless MCP server (stdio); mem/verify/review/drift/guard/stack + graph tools (incl. `tb_graph_impact`; typed schemas) |
