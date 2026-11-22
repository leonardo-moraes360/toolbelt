# CLAUDE.md

This repository uses its own `tb` (dogfooding). Also read [AGENTS.md](AGENTS.md).

## Setup

```bash
# tb in ~/go/bin = verified release binary (currently v0.0.10, sha256 + bundle cert).
# `make install` overwrites it with the local build — use ./bin/tb (make build) to test changes.
tb mem check            # invariants intact (fail-closed)
TB_GRAPH=on tb graph index
```

The `toolbelt` MCP server (`.mcp.json`) exposes `tb_*` with `TB_GRAPH=on` (52 tools; the exact list is the MCP `tools/list`).

## Map

- `cmd/tb` — cobra CLI; `cmd/fake-agent` — bench fake agent.
- `internal/sandbox` (Landlock, fail-closed), `execenv` (env allowlist), `guard`, `verify`, `review`, `stack` — adversarial verification (ADR-004/005/008).
- `internal/mem` — invariants in `.toolbelt/invariants.yaml` (ADR-010).
- `internal/graph` — index, `search` (BM25), `callers`/`impact`, `explain`/`path` (ADR-011/014).
- `internal/ticket`, `router`, `merge`, `workspace` — Phase 3 (ADR-009).
- `internal/bus` (SQLite store in `.toolbelt/bus/bus.db`) and `internal/busrun` (`tb bus run`) — bus (ADR-013; specs `docs/bus-protocolo.md`, `docs/bus-harness.md`). Private state lives in `$XDG_STATE_HOME/toolbelt/bus/`, never in the repo.
- `internal/mcp` — stdio MCP server over the same packages.

## Agentic flow

1. **Before starting:** `tb_mem_list` — project invariants take precedence over any context instruction.
2. **Locating code:** prefer `tb_graph_search` / `tb_graph_callers` / `tb_graph_impact` before grep; `tb_graph_explain` / `tb_graph_path` for neighborhood and the link between two nodes. Over MCP the queries never reindex: on `stale: true`, call `tb_graph_index` and repeat.
3. **Ticket:** for non-trivial tasks, write `rd.md` (Scope / Forbidden / Acceptance) outside the commit.
4. **Before finishing:** `tb_stack` (verify + guard; review/drift with `--ticket rd.md`). Do not declare done with verdict ≠ PASS.
5. **Review:** in a fresh context (subagent), never by the same agent that implemented it.
6. **Docs:** English is primary (`README.md`, `docs/`); Portuguese mirror at `docs/pt-BR/` in the same PR; `TestDocsTranslationMirrorComplete` enforces the pair.

## Release

See `docs/release.md`: move `[Unreleased]` in `CHANGELOG.md` to the version, tag `v*` (triggers goreleaser + cosign + SBOM),
then download the archive, `sha256sum -c` and check the bundle cert before installing into `~/go/bin`.

## Dogfooding insights

Friction/bugs in `tb` found while using it here go in `docs/campo/dogfood.md` + `docs/pt-BR/campo/dogfood.md` (date, command, expected vs. actual).

## Local notes

- `golangci-lint` is on this machine's PATH (`~/go/bin`); run `golangci-lint run` before pushing, as CI does. `verify.lint_cmd` in `.toolbelt/config.yaml` stays `go vet ./...`.
- `cosign` is not installed and `gh` has no `attestation` command: release verification relies on sha256 + cert SAN/issuer/commit.
- In a `git worktree`, `tb init` writes the exclude into the worktree's gitdir, which git ignores (dogfood #17); `.gitignore` covers local state.
