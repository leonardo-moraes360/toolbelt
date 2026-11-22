🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/016-vault-fase-a.md)

# ADR-016: Markdown vault, Phase A — the vault is the only store

## Status

proposed (2026-11-20); experimental in 1.0. Partially supersedes [ADR-006](006-inspiracoes-graphify-obsidian.md) (decision items 1 and 2, "own TUI") and [ADR-003](003-sqlite-fts5-grafo.md) ("Vault → SQLite Sync") for Phase A. Spec: [`docs/specs/vault.md`](../specs/vault.md).

## Context

ADR-006 decided that toolbelt's long-term memory would be an Obsidian-compatible Markdown vault in `.tb/vault/`, with SQLite + FTS5 as a derived index fed by a watcher (ADR-003), and a `tb md` TUI. None of it was built. Since then:

- the graph went to `.toolbelt/graph.json` on demand (ADR-011, ADR-014), with no SQLite in `go.mod`;
- invariants live in `.toolbelt/invariants.yaml` with a fail-closed `tb mem check` (ADR-010);
- `tb init` creates `.toolbelt/vault/` but git-excludes it as a local cache, so nothing written there is shared.

The documentation still describes the vault, SQLite, `tb md` and sync as existing.

## Decision

For Phase A of the vault:

1. **The Markdown files are the only store.** Every `tb vault` command reads the vault from disk. There is no persistent derived index, so there is no sync, no watcher and no "vault wins" conflict rule to maintain. A fingerprint cache (like `graph.json`) comes before SQLite if the performance NFR of the spec fails.
2. **Location `.toolbelt/vault/`, versioned.** The vault leaves the local-cache git exclude; only `.obsidian/` inside it stays excluded.
3. **Invariants stay in `invariants.yaml`.** The vault holds knowledge that is not an invariant (facts, decisions, context, logs) and never replaces `tb mem`.
4. **Ambiguous wikilinks are errors.** Where Obsidian breaks a tie by shortest path, tb reports `ambiguous_link`.
5. **No TUI.** `tb md` and `tb md --obsidian` are dropped from Phase A; any editor, Obsidian included, edits the vault.
6. **Graph integration only with `TB_GRAPH=on`** (ADR-011): notes, wikilinks and tags become nodes and edges with provenance.

ADR-006's other decisions (AST indexing, provenance on edges, graph-first hooks, high-level queries) are unchanged. ADR-003's SQLite + FTS5 stays a future option, not Phase A.

## Consequences

- A new dependency and a second copy of the data are avoided; every read reflects the files as they are.
- Each command pays a full scan; the spec's NFR (2,000 notes in under 1 s) is the trigger to add a cache.
- Repositories initialized before this change need `tb init` again to version the vault, and a `.gitignore` that ignores `.toolbelt/vault/` (as this repository's did) has to drop that line by hand; `tb init` only warns.
- `docs/arquitetura.md` and the README must stop describing `.tb/vault/`, SQLite, sync and `tb md` as existing.

## Alternatives Considered

- **Keep ADR-006/003 as is and build the SQLite index first.** Rejected: a dependency, a sync and state that can drift, with no measured performance problem.
- **Move invariants into the vault.** Rejected: loses the structured format and the fail-closed check (ADR-010).
- **Resolve ambiguity like Obsidian.** Rejected: the choice silently changes when a same-named note appears closer to the root.

## References

- [Spec: Obsidian-compatible Markdown vault, Phase A](../specs/vault.md)
- [ADR-003](003-sqlite-fts5-grafo.md), [ADR-006](006-inspiracoes-graphify-obsidian.md), [ADR-010](010-invariantes-fora-compactacao.md), [ADR-011](011-grafo-sob-demanda.md)
