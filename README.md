🇺🇸 English | [🇧🇷 Português](docs/pt-BR/README.md)

<p align="center">
  <img src="assets/logo/toolbelt-logo-256.png" alt="Toolbelt Logo" width="200">
</p>

<h1 align="center">toolbelt (<code>tb</code>)</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white" alt="Go Version"></a>
  <a href="docs/arquitetura.md"><img src="https://img.shields.io/badge/Status-Phase%203%20in%20progress-yellow" alt="Status"></a>
  <a href="https://github.com/leonardo-moraes360/toolbelt/actions"><img src="https://github.com/leonardo-moraes360/toolbelt/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/Protocol-MCP-purple" alt="MCP"></a>
  <a href="https://m8ven.ai/mcp/leonardo-moraes360-toolbelt-fa03ue?s=readme"><img src="https://m8ven.ai/badge/mcp/leonardo-moraes360-toolbelt-fa03ue?v=ac65034c346c6c7522ef08b6df2496ef" alt="M8ven Score"></a>
  <a href="docs/identidade-visual.md"><img src="https://img.shields.io/badge/Theme-Synthwave-ff278c" alt="Theme"></a>
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

See the full documentation in [docs/arquitetura.md](docs/arquitetura.md).

## 🌐 Site

Public landing: [https://leonardo-moraes360.github.io/toolbelt/](https://leonardo-moraes360.github.io/toolbelt/) (source in [`site/`](site/); guide in [`docs/site.md`](docs/site.md)).

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [docs/arquitetura.md](docs/arquitetura.md) | Detailed architecture with Mermaid diagrams |
| [docs/cli.md](docs/cli.md) | `tb` command tree (implemented + planned) |
| [docs/bus-protocolo.md](docs/bus-protocolo.md) · [docs/bus-harness.md](docs/bus-harness.md) | Local `tb bus` (`bus/0.1` protocol and `tb bus run` harness) |
| [docs/benchmark.md](docs/benchmark.md) | KPIs and evaluation methodology |
| [docs/site.md](docs/site.md) | Landing page and GitHub Pages deploy |
| [docs/adr/](docs/adr/) | Architecture Decision Records (ADRs) |
| [docs/pesquisa/](docs/pesquisa/) | Primary sources, top 10, and research findings (2026-11) |
| [docs/campo/SINTESE.md](docs/campo/SINTESE.md) | Field study across 20 repos + improvement ranking |
| [docs/identidade-visual.md](docs/identidade-visual.md) | Logo, Synthwave palette, and theme tokens |

## 🚀 Installation

### Binaries (GitHub Releases)

Download the archive from the [Releases page](https://github.com/leonardo-moraes360/toolbelt/releases) for your OS/arch (`linux`, `darwin`, or `windows` × `amd64`/`arm64`), verify checksum and signature, extract it, and put `tb` on your `PATH`. Requires [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/).

```bash
# Example (Linux amd64) — adjust VERSION/ASSET to the desired release
VERSION=v0.0.5
ASSET=toolbelt_0.0.5_linux_amd64.tar.gz
BASE="https://github.com/leonardo-moraes360/toolbelt/releases/download/${VERSION}"

curl -sLO "${BASE}/${ASSET}"
curl -sLO "${BASE}/checksums.txt"
curl -sLO "${BASE}/checksums.txt.sigstore.json"

cosign verify-blob \
  --certificate-identity "https://github.com/leonardo-moraes360/toolbelt/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --bundle checksums.txt.sigstore.json \
  checksums.txt

sha256sum -c --ignore-missing checksums.txt
tar -xzf "${ASSET}" tb
sudo mv tb /usr/local/bin/
tb version
```

Each archive also publishes an SBOM (`.sbom.json`) and the workflow attaches an SLSA provenance attestation (verify with `gh attestation verify`).

### Via Go

```bash
go install github.com/leonardo-moraes360/toolbelt/cmd/tb@latest
```

> `go install` uses the default embedded version (`dev`) unless the tag/module injects ldflags. Prefer the Release binary if you need the tag version.

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
[docs/cli.md](docs/cli.md#local-first-no-github).

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

# Phase 3: tickets -> router/workers -> merge (all local)
tb ticket create --title "Criar modelo User" --scope "src/models/**"
tb ticket create --title "JWT signing" --dep TKT-001
tb router run --agent-cmd "meu-agente --task-file $TB_TICKET_FILE"
tb board list
tb watchdog alerts
tb merge --dry-run TKT-001 TKT-002
tb merge TKT-001 TKT-002

# Local bus between agents (ADR-013); only the human creates channels/invites
tb bus channel create feature-x --owner claude-main --ttl 2h
tb bus invite feature-x --role worker
tb bus join feature-x --invite K7Q2-9XMD --name grok-1 --vendor xai
tb bus recv --channel feature-x --as grok-1 --wait 20s
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
- [x] Landing page in [`site/`](site/) + GitHub Pages ([docs/site.md](docs/site.md) · [live site](https://leonardo-moraes360.github.io/toolbelt/))
- [ ] Agent hierarchy / A2A / `tb md` (later)

### Input for the next phase (field study)
- Field study of `tb` across 20 public repositories (reports in [`docs/campo/`](docs/campo/) + improvement synthesis in [`docs/campo/SINTESE.md`](docs/campo/SINTESE.md)), as prioritized input for upcoming product PRs.

### Phase 3 (in progress)
- [x] Router/Workers for scalable task delegation (`tb router run`, foreground, workers in isolated worktrees, fail-closed sandbox)
- [x] Direct delivery — follow-ups go straight to the owning worker
- [x] `tb ticket` / `tb board` — work ticket management
- [x] `tb router` / `tb watchdog` — delegation control and health
- [x] `tb merge` — aggregation of worker results (`fail` | `last_wins`)
- [ ] Optional vector index
- [x] Graphify: in-house reimplementation ([ADR-014](docs/adr/014-graphify-reimplementacao.md)) — `tb graph explain`/`path`
- [x] `tb graph search` with BM25 ranking (title/path/package, camelCase/`snake_case` splitting, multi-term)
- [x] Obsidian-compatible Markdown vault, Phase A (`tb vault`, [ADR-016](docs/adr/016-vault-fase-a.md), [spec](docs/specs/vault.md)): notes with frontmatter and wikilinks, `new`/`list`/`show`/`search`/`links`/`backlinks`/`check`, MCP tools, `tb stack vault`, graph nodes; Q4 (golden fixture checked in real Obsidian) open
- [x] Elephant-Goldfish Model reimplementation, Phase A (`tb egm`, [ADR-015](docs/adr/015-egm-nativo.md) accepted): R0 spike; deterministic and `llm` engines, arbiter, gate in `tb stack` (`status --at`) and MCP tools
- [ ] `tb egm` next phases: CI as the trust boundary against active fraud (Phase A.2, [draft spec](docs/specs/egm-trust.md)), `drift-report` and `bootstrap` (Phase B); R0 item 8 open
- [ ] Optional Rust components for local inference
- [ ] Additional rulers for `tb verify`
- [ ] `tb merge` `prompt` / `semantic` strategies

### Multi-vendor bus (`tb bus`, [ADR-013](docs/adr/013-barramento-multi-vendor.md))
- [x] B0 — `bus/0.1` spec: envelope, Agent Card and invite ([docs/bus-protocolo.md](docs/bus-protocolo.md))
- [x] B1 — SQLite store, CLI + MCP, single-use invite, `join`/`send`/`recv`/`claim`/`complete`/`log`
- [ ] B2 — lease, heartbeat and watchdog on the bus ([draft spec](docs/bus-lease.md))
- [ ] B3 (partial) — `tb bus run` with `exec` and `harness` adapters done ([docs/bus-harness.md](docs/bus-harness.md)); `notify` adapter and `tb bus discover` pending
- [ ] B4 — Phase 3 router/board on top of the bus

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on:
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
- [ADR-006: Graphify and Obsidian](docs/adr/006-inspiracoes-graphify-obsidian.md)
- [ADR-007: Elephant-Goldfish Model](docs/adr/007-inspiracao-elephant-goldfish.md)
- [ADR-008: Adversarial Verification](docs/adr/008-verificacao-adversarial.md)
- [ADR-009: Router and Workers](docs/adr/009-router-e-workers.md)

## 📄 License

[Apache License 2.0](LICENSE) — Copyright 2026 Leonardo Moraes

## 📊 Project Status

This project is in **Phase 3 (in progress)**: besides verify/guard/mem/graph/review/drift/MCP and a benchmark harness, it now has tickets, board, router/workers, watchdog, merge and the local `tb bus` (B0/B1 and part of B3 of [ADR-013](docs/adr/013-barramento-multi-vendor.md)) (scaling gains not yet measured — see [ADR-009](docs/adr/009-router-e-workers.md)). The full architecture is documented in [docs/arquitetura.md](docs/arquitetura.md).

### Implemented Commands

| Command | Status | Description |
|---------|--------|-------------|
| `tb version` | ✅ | Shows version, commit, and build time |
| `tb init` | ✅ | Initializes local config; detects toolchain (`verify.test_cmd` with `PATH` check/fallback; Node/TS via lockfiles + `test`/`build`|`typecheck`/`lint` scripts; no marker → empty `test_cmd` with warning, never `go test` without `go.mod`; `build_cmd` / `lint_cmd`), reads `AGENTS.md`/`CLAUDE.md` (filtered hints) and optionally seeds with `--write-agents` (seed cites detected test/build/lint) |
| `tb bench run` | ✅ | Runs benchmark |
| `tb bench list` | ✅ | Lists available tasks |
| `tb bench report` | ✅ | Generates Markdown report |
| `tb bench compare` | ✅ | Compares two benchmarks |
| `tb sandbox` | ✅ | Worktrees + Landlock (Linux) (`check`/`run`) |
| `tb guard` | ✅ | Anti-cheat check on diff (strict by default / exit ≠ 0 on `SeverityError`; `--warn-only` opt-out; `--strict` deprecated) |
| `tb verify` | ✅ | `tests` + `visual` rulers (`tb verify run`; `--build-cmd`/`--lint-cmd`/`--no-build`/`--no-lint`/`--dry-run`; hidden suite via `--hidden-test-cmd` or toolchain detection in `tests/hidden/` — N/A if not applicable; build/lint cache within the run; `reason`/`error` in JSON; `--base`/`--head` run in the head worktree; fail-fast if command missing/not executable; custom `protected_paths` from `guard.yaml` do not abort the loop — Scope exception via `--ticket`) |
| `tb mem` | ✅ | Invariants (`add`/`list`/`show`/`edit`/`retire`/`search`/`check`/`init`; `--ref` ADR; soft-retire + `--all`) |
| `tb graph` | ✅ | On-demand multi-language graph + frameworks + docs (ADR-011); `javascript` covers TypeScript (`ts`/`tsx`); BM25-ranked `search`, `callers`/`deps`/`impact`, `explain`/`path` (ADR-014) |
| `tb review` | ✅ | Fresh-context review against local ticket/`rd.md` (no GitHub); `--approved-sha` |
| `tb drift` | ✅ | Scope / base commit / pinned patch drift against `rd.md` (not vault vs code) |
| `tb stack` | ✅ | Stacks verify/review/drift/guard with shared context (aggregated JSON; MCP `tb_stack`) |
| `tb workspace` | ✅ | Isolated workspaces via git worktree |
| `tb ticket` | ✅ | Work tickets in `.toolbelt/tickets/` (`create`/`list`/`show`/`cancel`/`retry`; MCP `tb_ticket_*`) |
| `tb board` | ✅ | RUNNING/READY/BLOCKED/DONE/FAILED board; `pause`/`resume`/`drain`/`watch` (MCP `tb_board`) |
| `tb router` | ✅ | Foreground scheduler (`run`/`status`/`config`) with direct delivery; fail-closed sandbox (`--allow-unsandboxed`) |
| `tb watchdog` | ✅ | Worker alerts and control (`status`/`alerts`/`kill`); runs inside the router loop |
| `tb merge` | ✅ | Atomically merges `done` tickets; `fail` (default) or `last_wins` (MCP `tb_merge`); `prompt`/`semantic` not implemented |
| `tb bus` | 🟡 | Local bus between agents (ADR-013): `channel`/`invite`/`join`/`send`/`recv`/`claim`/`complete`/`log` over `.toolbelt/bus/bus.db`; `tb bus run` wakes the agent (`exec`/`harness` adapters); lease/heartbeat (B2), `notify` and `discover` pending |
| `tb mcp` | ✅ | Stateless MCP server (stdio); 36 tools in v0.0.5: mem/verify/review/drift/guard/stack, graph (`search`/`callers`/`deps`/`impact`/`explain`/`path`/`index`; requires `TB_GRAPH=on`), ticket/board/router/watchdog/merge and bus (`tb_bus_join`/`send`/`recv`/`claim`/`complete`); `--bus-as` binds the connection to a bus session |
