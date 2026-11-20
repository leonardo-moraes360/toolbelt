🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/vault.md)

# Spec: Obsidian-compatible Markdown vault, Phase A (`tb vault`)

Author: toolbelt · Status: Draft (round 3: refreshed against main, fresh-context review applied, Q1 resolved by [ADR-016](../adr/016-vault-fase-a.md); Q4 open) · Date: 2026-11-20 · Decision: [ADR-016](../adr/016-vault-fase-a.md) · Originating decisions: [ADR-006](../adr/006-inspiracoes-graphify-obsidian.md), [ADR-003](../adr/003-sqlite-fts5-grafo.md) · Related: [ADR-010](../adr/010-invariantes-fora-compactacao.md), [ADR-011](../adr/011-grafo-sob-demanda.md)

## Overview

ADR-006 decided that toolbelt's long-term memory would be an Obsidian-compatible Markdown vault: notes with frontmatter, wikilinks, backlinks and direct editing by humans. None of it was implemented. This spec defines **Phase A**: the note format, wikilink resolution, the `tb vault` commands and their MCP tools, and the integration with `tb graph`.

Phase A is deliberately small and **diverges** from accepted parts of ADR-006 and ADR-003 (see Q1 and [ADR-016](../adr/016-vault-fase-a.md)):
- the vault is the source of truth and **there is no persistent derived index** (no SQLite/FTS5): every command reads the vault from disk;
- the vault lives in `.toolbelt/vault/`, not `.tb/vault/`;
- invariants stay in `invariants.yaml` and do not move into the vault;
- there is no TUI (`tb md`), watcher or bidirectional sync.

## Current state (2026-11-20)

| Item | State |
|------|-------|
| `VaultConfig{Path, SyncOnChange}` in `internal/config/config.go` | Exists, defaults `path: .toolbelt/vault`, `sync_on_change: true`; **never read** |
| `.toolbelt/vault/` | Created by `tb init`, empty |
| `config.LocalCacheExcludePatterns` | Includes `.toolbelt/vault/`: the vault is treated as a local cache and is **not versioned**, contradicting ADR-006 |
| This repository's `.gitignore` | Ignores `.toolbelt/vault/`, so even without the exclude the vault would not be versioned here |
| `tb mem` | Invariants in `.toolbelt/invariants.yaml`; no Markdown |
| `internal/graph/index_docs.go` | Does not recognize `[[wikilink]]` or interpret frontmatter: frontmatter is treated as plain text, so paths in it become `mentions` edges and `# ` lines (YAML comments, or inside code fences) become sections |
| Graph persistence | `.toolbelt/graph.json`; no SQLite/FTS5 in `go.mod` |
| Graph walk | Does not skip `.toolbelt/`; `readSource` follows symlinks and does not check file type |
| `docs/arquitetura.md` §3 and §7, README | Describe vault, SQLite, `tb md` and sync as existing, under `.tb/vault/` |

## Glossary

| Term | Definition |
|------|------------|
| root | Parent directory of the `.toolbelt/` found by `mem.LocateRepoToolbeltDir(cwd)`, with symlinks resolved (R0). Same root `tb mem` uses. |
| vault | The `vault.path` directory from config, relative to the root (default `.toolbelt/vault`). |
| note | Regular `.md` file inside the vault, outside ignored directories (R3). |
| attachment | Any other regular file inside the vault, outside ignored directories (R3, R8). |
| note name | File name without `.md`. |
| note path | Path relative to the vault, with `/`, without `.md`. E.g. `decisoes/sqlite-adiado`. |
| wikilink | `[[target]]`, `[[target\|label]]`, `[[target#heading]]`, `[[target#^block]]`, `[[#heading]]` (same note), or any of them prefixed by `!` (embed). |
| target | Part before the first `#` or `\|`, trimmed, without a `.md` suffix. May be empty only when a fragment is present. |
| backlink | Note B is a backlink of A if B contains a wikilink that resolves to A. |
| broken / ambiguous link | Wikilink whose target resolves to no note / more than one note (R8). |
| provenance | Frontmatter `source` field: `manual`, `extracted` or `inferred` (ADR-006). |
| note data | Any string derived from a note's content: title, aliases, tags, body, snippets, link targets and messages quoting them. |

## Problem

1. Agents and humans have nowhere to record **knowledge** that is not an invariant: small decisions, project facts, context from an investigation. `invariants.yaml` is for short rules checked by `tb mem check` and does not fit this.
2. The documentation promises a vault that does not exist, and the only implemented piece (the directory) is git-excluded, so nothing written there would be shared.
3. The graph does not understand wikilinks, so a vault opened in Obsidian and `tb graph` would see different graphs.

## Goals

- G1: notes readable and editable in Obsidian without conversion. In the subset where tb and Obsidian agree (unambiguous links, R8), tb resolves each wikilink to the same note Obsidian does; where Obsidian would break a tie by shortest path, tb reports an error. A golden fixture checked in real Obsidian (verification plan, Q4) is the oracle.
- G2: CLI and MCP to create, list, read, search and check notes, and to list links and backlinks.
- G3: `tb vault check` fails closed (missing vault, invalid frontmatter, broken and ambiguous links) and runs inside `tb stack`.
- G4: the graph, when enabled, includes notes and wikilinks with provenance, including in repositories whose `config.yaml` predates this spec.
- G5: vault versioned by default.
- G6: no behavior change in `tb mem`, `invariants.yaml` or `tb mem check`.

## Non-Goals

- Replacing or mirroring invariants in the vault. Invariants are a structured file outside compaction (ADR-010, still `proposed`), with a fail-closed check in `tb mem check`; a human-editable vault offers neither.
- SQLite/FTS5 index (ADR-003). Deferred until the performance NFR fails. Without an index, ADR-006's "bidirectional sync" question does not exist in Phase A.
- `tb md` TUI, `tb md --obsidian`, watcher. `sync_on_change` stays reserved and has no effect.
- Inline tags (`#tag` in the body), Dataview, Canvas, Obsidian plugins, `.obsidian/` as configuration read by tb.
- Editing existing notes through tb (renaming with link updates, moving).
- Wikilinks to files outside the vault. Relative Markdown links cover that; a `refs` frontmatter field is deferred to Phase B.
- Hardening the graph walk outside the vault (symlinks and FIFOs in `docs/`, code, etc.). Same problem as R21, recorded in Q5.
- Embeddings or semantic search.

## Critical user journeys (CUJs)

- **CUJ-1.** *Given* a repository after `tb init`, *when* I run `tb vault new "SQLite adiado" --type decision --tag storage`, *then* `.toolbelt/vault/sqlite-adiado.md` is created with valid frontmatter and `source: manual`, and the path is printed. Running it again with the same title fails without overwriting.
- **CUJ-2.** *Given* I open `.toolbelt/vault/` in Obsidian, write `[[sqlite-adiado]]` in a new note and save, *when* I run `tb vault backlinks sqlite-adiado`, *then* the new note is listed.
- **CUJ-3.** *Given* a note containing `[[nota-que-nao-existe]]`, *when* I run `tb vault check`, *then* the exit code is 1 and the output names the file, line and target.
- **CUJ-4.** *Given* `TB_GRAPH=on` and a `config.yaml` generated before this spec, *when* I run `tb graph index` and `tb graph explain note:sqlite-adiado`, *then* the graph shows the note, its outgoing links and backlinks.
- **CUJ-5.** *Given* an agent over MCP, *when* it calls `tb_vault_search` with `"sqlite"`, *then* it gets notes with path and metadata, and all note data comes inside a `data` object, redacted by the R15 patterns.
- **CUJ-6.** *Given* a repository initialized before this spec, *when* I run `tb init` again, *then* `.toolbelt/vault/` leaves `.git/info/exclude` and `.toolbelt/vault/.obsidian/` enters it, without losing the user's lines in that file.
- **CUJ-7.** *Given* an agent over MCP, *when* it calls `tb_vault_new`, *then* the note is created with `source: inferred` and the `confidence` the agent supplied; the agent cannot write `source: manual`.

## Technical Plan

```
human (Obsidian/editor) ──┐                 ┌── agent (MCP tb_vault_*)
                          ▼                 ▼
                .toolbelt/vault/*.md  ◄── tb vault (CLI)
                (source of truth, versioned)
                          │ on-demand reads (no persistent index)
                          ▼
         internal/vault: safe scan → parse → resolve
                          │                    ▲
                          │          internal/mdparse (leaf):
                          │          slugify, fence-aware headings,
                          │          frontmatter, wikilinks
                          ▼                    ▼
  list/show/search/links/         internal/graph: VaultIndexer
  backlinks/check                 (only with TB_GRAPH=on, ADR-011)
```

`internal/mdparse` imports nothing from the project. `internal/vault` and `internal/graph` import `mdparse`; `VaultIndexer` imports `internal/vault`, and `internal/vault` does **not** import `internal/graph`. This avoids the cycle that would appear if the vault used the graph's `slugify` and the graph used the vault's parser.

### Requirements

**Location, config and versioning**

- R0: the root is the parent of the `.toolbelt/` directory returned by `mem.LocateRepoToolbeltDir(cwd)`, with symlinks resolved; over MCP, `cwd` is the tool's `cwd` argument, never the server's. The config is read from `<root>/.toolbelt/config.yaml` explicitly (like `internal/router`), never from the process cwd. A root without `.toolbelt/` is exit 2 with `no_toolbelt_dir`.
- R1: the vault is `vault.path`, relative to the root. tb applies `filepath.Clean`, joins it with the root, resolves symlinks with `EvalSymlinks` and requires the result to be inside the root; otherwise exit 2 with `vault_path_outside_repo`. The resolved vault is the root of the R3 walk. Missing vault: exit 1 with `vault_not_found` in every command, including `check`.
- R2: `EnsureLocalGitExclude` takes extra patterns derived from config: `<vault.path>/.obsidian/`. `.toolbelt/vault/` leaves `LocalCacheExcludePatterns`. `tb init` loads the config (if present) to compute these patterns, creates `vault.path` when it is not the default (`InitDetailed` hardcodes `vault` and returns early when a config exists), and its log lists the extra patterns too. Since `upsertExcludeSection` rewrites the whole section, `tb init` on an old repository applies the swap (CUJ-6). Fixed alongside: today, with a begin marker and no end marker, `upsertExcludeSection` keeps only the text before the begin marker and drops the user's lines after it. It now removes the orphan begin line and, after it, only lines equal to a pattern tb writes or wrote (current and previous lists), keeps every other line, and appends a fresh block. A `.gitignore` that ignores the vault is not edited; `tb init` warns `vault_gitignored` (this repository's `.gitignore` gets the line removed in the same change).
- R3: the scan walks the vault with `filepath.WalkDir`, without following symlinks. Directories starting with `.` (including `.obsidian/` and `.trash/`) are skipped. A note candidate is an entry whose `Lstat` is a regular file with a `.md` extension; other regular files are attachments (R8). Any entry that is neither a regular file nor a directory (symlink, FIFO, socket, device) is reported as `not_regular` by `check` (warning). Reads open with `O_RDONLY|O_NOFOLLOW|O_NONBLOCK`, then `fstat` the descriptor and require a regular file (a file swapped for a FIFO after the `Lstat` is refused), and read at most `max_note_bytes + 1` bytes. A note larger than `vault.max_note_bytes` (default 256 KiB) is reported as `note_too_large` (error) and not parsed.

**Note format**

- R4: a note may start with YAML frontmatter: the first line is `---` (accepts `\r\n`, a leading UTF-8 BOM and trailing spaces or tabs) and the frontmatter ends at the next line that is only `---` or `...` (trailing spaces or tabs allowed). A file that starts with a `---` thematic break is read as frontmatter, as in Obsidian. Recognized fields:

  | Field | Type | Default |
  |-------|------|---------|
  | `title` | string | First H1 outside fences; without H1, the note name |
  | `type` | string, one of `vault.types` | `fact` |
  | `tags` | list of strings or a single string; each tag matches `^[a-z0-9][a-z0-9/_-]*$` | `[]` |
  | `aliases` | list of strings or a single string (metadata and search only, R8) | `[]` |
  | `source` | `manual`, `extracted` or `inferred` | `manual` |
  | `confidence` | number in [0, 1] | absent |
  | `created`, `updated` | RFC 3339 or `YYYY-MM-DD` | absent |

  Problem classification:

  | Case | Code | Level |
  |------|------|-------|
  | Invalid YAML, unterminated frontmatter, recognized field with wrong type | `bad_frontmatter` | error |
  | `source` not in the list, `type` not in `vault.types`, `confidence` outside [0, 1], invalid date | `bad_frontmatter` | error |
  | `source: inferred` without `confidence` | `missing_confidence` | error |
  | `confidence` with `source` other than `inferred` | `unused_confidence` | warning |
  | Tag not matching the pattern | `bad_tag` | warning (tag dropped) |

  Unknown fields are preserved and ignored. In commands other than `check`, a note with an error uses defaults for the affected field and gets `warnings: [{code, line, message}]` in `--json` output.
- R5: notes without frontmatter are valid. tb never rewrites an existing note.
- R6: headings are ATX lines (`#` to `######` followed by a space) **outside** the frontmatter and fenced code blocks (` ``` ` or `~~~`, closed by the same marker with length ≥ the opening one). The heading slug is `mdparse.Slugify`, the same function `MarkdownIndexer` switches to (today the unexported `slugify` in `index_docs.go`).

**Wikilinks**

- R7: extraction. Outside the frontmatter, fenced blocks and inline code (backtick spans, with CommonMark's same-length rule), the parser looks for `[[`, reads up to the next `]]` on the same line, and drops the candidate if the inner text contains `[`, `]` or a newline. Embed is a `!` immediately before `[[`. In the inner text:
  1. split the label at the first unescaped `|`; `\|` counts as a separator (the form used inside tables) and the `\` is removed;
  2. split the fragment at the first `#` of what remains;
  3. target = the rest, `TrimSpace`d, without a `.md` suffix (case-insensitive).

  An empty target with a fragment links to the same note. An empty target without a fragment is dropped. Each link stores line (1-based), column (1-based, in runes, of the `!` for an embed and of the first `[` otherwise), target, fragment, label and `embed`. A backslash-escaped backtick does not open a code span. Phase A limits: code spans and wikilinks are recognized within one line, and fences inside list items or blockquotes are not recognized, so links inside them are extracted.
- R8: resolution of a non-empty target `t`, case-insensitive (`strings.EqualFold`):
  - **Attachment:** if `t` has an extension other than `.md` (for example `img.png`, `doc.pdf`), it is an attachment link. It resolves against the vault's regular non-note files by the same two steps below; zero candidates is `missing_attachment` (warning), never an error, because Obsidian vaults routinely embed files tb does not manage.
  - **Note**, in order, stopping at the first step with candidates:
    1. if `t` contains `/`: notes whose path is `t` or ends with `/t` (Obsidian's partial-path match);
    2. otherwise, notes whose name is `t`.
  - Zero candidates: `broken_link`. One: resolved. More than one: `ambiguous_link`. Obsidian breaks ties by shortest path; tb does not, because that choice silently changes when someone creates a same-named note closer to the root, and the agent would follow the wrong note without warning.
  - `aliases` do not resolve links: in Obsidian they only feed autocomplete, which writes `[[note|alias]]`.
- R9: a `#heading` fragment is compared by slug (R6) against the target note's headings. Missing: `broken_fragment` (warning, since Obsidian accepts it). `#^block` is not checked in Phase A.

**Commands**

- R10: `tb vault new <title> [--type T] [--tag t]... [--source S --confidence C] [--dir sub] [--body -|text]`:
  - the title must not contain control characters (newlines included): exit 2 with `bad_title`;
  - title and body go through the vault redaction (R15) first; the output includes `redacted: true` when anything changed;
  - file name: `mdparse.Slugify(redacted title)`, cut to 80 bytes at a rune boundary, + `.md`, in `<vault>/<sub>/`; an empty slug is exit 2 with `bad_title` (never the graph's `section` fallback);
  - `--dir` must be relative; tb creates missing directories one by one, `Lstat`ing each component and refusing symlinks, and checks that the final directory, resolved, is inside the vault; otherwise exit 2 with `dir_outside_vault`;
  - writes with `O_CREATE|O_EXCL|O_NOFOLLOW`, mode `0644`; if it exists, exit 1 with `note_exists` and the path;
  - frontmatter is produced by `yaml.Marshal` of a struct (never by string interpolation): `title`, `type`, `tags` (if any), `source`, `confidence` (if any), `created` (RFC 3339 UTC); body: `# <title>` followed by `--body` (`-` reads stdin, up to `max_note_bytes`);
  - on the CLI, `--source` defaults to `manual`; `inferred` requires `--confidence`.
- R11: `tb vault list [--type T] [--tag t] [--source S]` lists path, title, type, tags, `source` and `updated` (frontmatter only; mtime is never output), sorted by path.
- R12: `tb vault show <target>` resolves `<target>` by R7–R8 (also accepts the note path) and prints normalized frontmatter and body. With `--json`, it includes `links` (with state) and `backlinks`.
- R13: `tb vault search <query> [--limit N]` first redacts each note's whole text (R15), then does case- and accent-insensitive substring search over the redacted title, aliases, tags and body, so a query can never probe a secret through hit counts or order. The normalization is `normalizeForSearch` (currently unexported, in `internal/mem/validate.go`; it moves to `mdparse.NormalizeForSearch` and `mem` reuses it). The search walks the redacted text rune by rune, comparing each rune's normalized form, so match offsets are valid in that text. The snippet has up to 200 runes around the first match. Order: path, then line of the first match. `--limit` defaults to 20.
- R14: `tb vault links <target>` lists outgoing wikilinks with state (`ok`, `broken`, `ambiguous`, `broken_fragment`, `attachment`, `missing_attachment`); `tb vault backlinks <target>` lists notes pointing to the note, with line. Order: path, line, column.
- R15: outputs containing note data go through the vault redaction, applied to the whole text before any cutting or matching, on the CLI and over MCP. The vault redaction is its own list in `internal/vault`: `mem.SecretPatterns()` plus `ghu_`, `ghs_`, `sk-` (≥ 20 characters) and `password|passwd|secret` followed by `=` or `:`. `mem`'s own patterns do not change, so `tb mem` and `tb egm` output stay the same (G6). Anything outside these patterns is residual risk (Threat model).
- R16: `tb vault check` validates the whole vault. Errors (exit 1): `vault_not_found`, `bad_frontmatter`, `missing_confidence`, `note_too_large`, `broken_link`, `ambiguous_link`. Warnings (exit 0): `not_regular`, `bad_tag`, `unused_confidence`, `broken_fragment`, `missing_attachment`. Existing, empty vault: exit 0 with `notes: 0`.
- R17: every command accepts `--json`, with snake_case keys, and the same inputs produce the same output byte for byte (NFR).

**MCP and stack**

- R18: tools `tb_vault_new`, `tb_vault_list`, `tb_vault_show`, `tb_vault_search`, `tb_vault_links`, `tb_vault_backlinks` and `tb_vault_check`, with typed schemas and a required absolute `cwd` (R0), registered in `internal/registry/parity.go` to keep CLI/MCP parity.
  - **Data separation:** all note data (glossary) goes in a `data` object per item; outside it there are only tb-generated fields (path, codes, lines, states, counts). `check` messages quote the target only inside `data`.
  - **Provenance:** `tb_vault_new` does not accept `source`; it always writes `source: inferred` and requires `confidence` (CUJ-7).
- R19: `tb stack` gains the `vault` op (runs R16), chosen like the others, by positional argument (`tb stack verify guard vault`). It is read-only and runs in parallel like `guard` (`isParallelOp`); it ignores `--strict`, which stays an `egm` flag. The op lists and error messages in `internal/stack/stack.go` and `cmd/tb/stack.go`, and the `tb_stack` tool schema in `internal/registry/parity.go`, include it.

**Graph (only with `TB_GRAPH=on`, ADR-011)**

- R20: `BuildOptions` gains `VaultDir` (the R1 vault, empty when it cannot be resolved), filled by `graphOptionsFromConfig` and `graphBuildOptions`. The per-file indexers skip files under `VaultDir`, and a build-scoped pass after the walk (like `resolveSoftMentions`) scans the whole vault once through `internal/vault`, because R8 resolution needs every note. The pass runs when `vault` is enabled: `allowsIndexer` treats `vault` as enabled whenever `markdown` is in `Docs`, so repositories with an old `config.yaml` (listing `markdown, html, txt`) do not lose notes (G4), unless `vault.graph: false`; `vault` is also added to the `Docs` default.
- R21: the vault pass does **not** use `readSource`: it reads through `internal/vault` (R3). Symlinks, FIFOs and oversized files in the vault are never read by the graph, so a cloned repository cannot, through a versioned symlink in the vault, make `graph.json` (and the `tb_graph_*` tools) carry content from outside the repository.
- R22: each note becomes a `NodeNote` node (new) with ID `note:<note path>`, the file's `Path` and the R4 `Title`, redacted (R15); `Source` and `Confidence` come from frontmatter (`manual` → 1.0, `extracted` → 0.9, `inferred` → its `confidence`). Headings (R6) become `NodeSection` with ID `section:<file>#<slug>`.
- R23: a resolved wikilink becomes an `EdgeLinks` edge from `note:A` to `note:B`, or to `section:<B's file>#<slug>` with a fragment, `Source: "extracted"`, confidence 0.9. Broken, ambiguous and attachment links produce no edge. The body (without frontmatter) still produces Markdown links, autolinks and mentions as in `MarkdownIndexer`, with the note as the source: `indexMarkupDoc` takes the source node ID as a parameter instead of assuming `file:`.
- R24: tags become `NodeTag` nodes (new, `tag:<tag>`) with an `EdgeTagged` edge (new) from the note to the tag.
- R25: `tb graph explain`, `tb graph path` and `tb graph search` reach `note:` and `tag:` nodes through the existing exact-ID resolution (`resolveNodeIDs`); the change in `cmd/tb/graph.go` and `internal/registry/graph.go` is only to list the new node types in help and schemas. No new flags.

### Configuration

```yaml
vault:
  path: .toolbelt/vault        # existing
  sync_on_change: true         # existing (current default true); reserved, no effect in Phase A
  max_note_bytes: 262144       # new
  types: [fact, decision, context, log]  # new
  graph: true                  # new; false keeps notes out of the graph even with markdown enabled
graph:
  docs: [markdown, vault, html, txt]     # vault new in default; with only markdown, vault is implied (R20)
```

`VaultConfig.Validate` rejects a `path` that is absolute, has `..` escaping the root, or contains a newline, a control character or a glob character (`*`, `?`, `[`, `]`, `!`, a leading `#`), since it is written into `.git/info/exclude`; `max_note_bytes` ≤ 0 or above 4 MiB; and `types` that is empty, has duplicates, or has values outside `^[a-z][a-z0-9_-]*$`.

## Threat model

| Threat | Mitigation |
|--------|------------|
| A note written by an agent or from a cloned repository instructs another agent (injection via MCP) | R18 (all note data inside `data`), R15. Residual risk: the agent still reads the text and may follow it. |
| "Private data + untrusted content + external output" combination (ADR-010) | tb has no external output (local-first). `tb_vault_*` hands untrusted content to the agent, and the agent's harness decides whether it has external output. Recorded as a harness risk, not a tb risk. |
| A secret pasted into a note reaches a prompt | R10 (redaction on write), R15 (redaction on read, before cutting), R13 (search over redacted text, so no probing by hit count). Residual risk: secrets outside the R15 patterns. |
| A secret in a title becomes a file name | R10 (the slug comes from the redacted title) |
| A title with a newline forges frontmatter (`source: manual`) | R10 (control characters refused, frontmatter from `yaml.Marshal`) |
| `vault.path` or `--dir` pointing outside; symlink in an intermediate component; a pattern injected into `.git/info/exclude` | R1 (`EvalSymlinks` of the vault root), R10 (per-component `Lstat`, `O_NOFOLLOW`), `VaultConfig.Validate` |
| Versioned symlink in the vault to `~/.ssh/...` read by the graph and returned by `tb_graph_*` | R21 |
| Huge note or FIFO hangs the scan, `tb graph index` or `tb stack`, including a swap between `Lstat` and open | R3 (`O_NONBLOCK` + `fstat`), R21 |
| `tb vault new` overwrites a human note | R10 (`O_EXCL`) |
| An agent note poses as human knowledge (`source: manual`, confidence 1.0) | R18 (MCP always writes `inferred`). Residual risk: an agent with file or CLI access can write `manual` by hand; provenance is declarative, not signed. |
| Note titles and tags reach agents through `tb_graph_*`, outside a `data` object | R22 (titles redacted). Residual risk: the graph tools return node titles as plain fields, so an injected title still reaches the agent. |
| A link resolved differently from Obsidian sends the agent to the wrong note | R8 (ambiguity is an error) |
| A typo in `vault.path` makes `check` pass with zero notes | R1, R16 (`vault_not_found` is an error) |
| Obsidian config with personal data versioned by accident | R2 |

## Non-functional requirements (NFRs)

- **Performance:** `tb vault check` on a vault with 2,000 notes of 4 KiB runs in under 1 s on a development machine. If it fails, Phase B adds a fingerprint cache (like `graph.json`) before considering SQLite.
- **Determinism:** the same notes produce the same `--json` output, byte for byte (orders defined in R11, R13, R14; nothing derived from mtime).
- **Local-first:** no command talks to the network (project invariant).

## Alternatives

- **Move `tb mem` into vault notes.** Rejected: loses the structured format and the fail-closed `tb mem check`, and immutable invariants would become editable in any editor.
- **SQLite/FTS5 index now (ADR-003, ADR-006).** Deferred: a new dependency, a sync to maintain and state that can drift, with no measured performance problem.
- **Resolve ambiguity like Obsidian.** Rejected for the reason in R8.
- **Vault outside the repository (e.g. `$XDG_DATA_HOME`).** Rejected as the default: the point is to share knowledge through git. R1 requires a path inside the root; a personal vault can be reconsidered if there is demand.
- **Export the graph's `slugify` and import the graph from the vault.** Rejected: import cycle with the vault pass. Hence the leaf `mdparse` package.
- **Add the new secret patterns to `mem`.** Rejected for Phase A: `mem.secretPatterns` also feeds `RedactInvariant` and `tb egm`, so it would change their output (G6). The vault keeps its own list on top of `mem.SecretPatterns()`.

## Open questions

- Q1 (resolved): the divergences from ADR-006 (`.tb/vault/`, SQLite index, sync, `tb md`) and ADR-003 (SQLite+FTS5 as derived index) are recorded in [ADR-016](../adr/016-vault-fase-a.md), which partially supersedes both for Phase A.
- Q2: wikilink to an invariant (`[[inv:<id>]]`) resolving into `invariants.yaml`? Proposal: Phase B.
- Q3: should `tb review` and `tb drift` read `type: decision` notes linked to the ticket? Out of Phase A. Notes are never `tb egm` sources: egm excludes `.toolbelt/**` from sources, which also keeps note text out of reviewer packets.
- Q4: the golden fixture (G1) has to be checked once in real Obsidian before R8 is frozen; until then R8's partial-path and attachment rules are the expected behavior, not a verified one.
- Q5: the graph walk outside the vault has the same weakness as R21 (`readSource` follows symlinks and reads FIFOs). Worth a separate fix, using the same safe reader, for all indexers.
- Deferred to Phase B (cut from this round to keep Phase A small): `refs` frontmatter with `bad_ref`/`ref_missing` and the `Store.Impact` step for notes, `orphan_note`, `check --strict`.

## Detailed Implementation Plan

Four slices, each mergeable on its own with its tests: 1 `mdparse` (no user-facing change); 2 read-only vault and `tb stack vault`; 3 write path, search, MCP and versioning; 4 graph.

| File | Action | Reason |
|------|--------|--------|
| `internal/mdparse/slug.go`, `internal/mdparse/headings.go`, `internal/mdparse/frontmatter.go`, `internal/mdparse/wikilink.go`, `internal/mdparse/normalize.go` | Create | Slice 1: Leaf package (R4, R6, R7, R13) |
| `internal/mdparse/mdparse_test.go` | Create | Slice 1: Parser cases of the verification plan |
| `internal/graph/index_docs.go` | Modify | Slice 1: Use `mdparse.Slugify` and fence-aware headings (R6) |
| `internal/mem/validate.go` | Modify | Slice 1: Reuse `mdparse.NormalizeForSearch` |
| `internal/config/config.go`, `internal/config/config_test.go` | Modify | Slice 2: `max_note_bytes`, `types`, `graph`, `VaultConfig.Validate` (R0, R1, Configuration) |
| `internal/vault/root.go`, `internal/vault/scan.go`, `internal/vault/read.go` | Create | Slice 2: Root and config from the root, safe walk and reads (R0, R1, R3) |
| `internal/vault/note.go`, `internal/vault/resolve.go`, `internal/vault/redact.go` | Create | Slice 2: Note, defaults, classification, resolution, vault redaction (R4–R9, R15) |
| `internal/vault/vault.go`, `internal/vault/check.go` | Create | Slice 2: `List`, `Show`, `Links`, `Backlinks`, `Check` (R11, R12, R14, R16) |
| `internal/vault/vault_test.go` | Create | Slice 2: Resolution, frontmatter, security and golden-fixture tests |
| `cmd/tb/vault.go`, `cmd/tb/vault_test.go` | Create | Slice 2: `list`, `show`, `links`, `backlinks`, `check` (R17) |
| `internal/stack/stack.go`, `cmd/tb/stack.go` | Modify | Slice 2: `vault` op (R19) |
| `internal/vault/new.go`, `internal/vault/search.go` | Create | Slice 3: `New`, `Search` (R10, R13) |
| `internal/registry/vault.go` | Create | Slice 3: `tb_vault_*` tools (R18) |
| `internal/registry/parity.go`, `internal/registry/parity_test.go`, `internal/registry/annotations.go` | Modify | Slice 3: Registration, parity, `vault` op in the `tb_stack` schema (R18, R19) |
| `internal/config/exclude.go`, `internal/config/exclude_test.go` | Modify | Slice 3: Extra patterns, drop `vault/`, orphan begin marker (R2) |
| `cmd/tb/main.go`, `cmd/tb/init_exclude_test.go` | Modify | Slice 3: `init` loads config, creates a custom vault, logs patterns, `vault_gitignored` (R2, CUJ-6) |
| `.gitignore` | Modify | Slice 3: Stop ignoring `.toolbelt/vault/` in this repository (G5) |
| `cmd/tb/json_snake_case_test.go` | Modify | Slice 3: Samples of `tb vault` outputs |
| `docs/cli.md`, `docs/arquitetura.md`, `README.md` (+ `docs/pt-BR/` mirrors) | Modify | Slice 3: Pair each doc with its mirror (the docs translation test requires it). Replace `.tb/vault` with `.toolbelt/vault`; mark SQLite, `tb md` and sync as planned; document `tb vault` |
| `CHANGELOG.md` | Modify | Slice 3: Unreleased entry |
| `internal/graph/indexer.go` | Modify | Slice 4: `BuildOptions.VaultDir`, vault pass after the walk, implied `vault` (R20, R21) |
| `internal/graph/index_vault.go`, `internal/graph/index_vault_test.go` | Create | Slice 4: Notes, sections, wikilink and tag edges (R22–R24) |
| `internal/graph/index_docs.go` | Modify | Slice 4: `indexMarkupDoc` with a source ID parameter; skip `VaultDir` (R20, R23) |
| `internal/graph/graph.go`, `internal/graph/graph_test.go` | Modify | Slice 4: `NodeNote`, `NodeTag`, `EdgeTagged` (R22, R24) |
| `internal/registry/graph.go`, `cmd/tb/graph.go` | Modify | Slice 4: `VaultDir` in build options; new node types in help and schemas (R20, R25) |

### Verification plan

- **Parser (`mdparse`):** one test per glossary form: `[[a]]`, `[[a|b]]`, `[[a\|b]]` (in a table), `[[a#h]]`, `[[a#^x]]`, `[[#h]]`, `![[a]]`, `[[folder/a]]`, `[[a.md]]`, `[[ a ]]`, `[[A]]` against `a.md`; ignored: inside ` ``` `, inside `~~~`, inside inline code with one and two backticks, in the frontmatter; dropped: `[[]]`, `[[a]` + newline. Headings inside fences and YAML comments are not headings. CRLF: correct lines and frontmatter.
- **Resolution:** unique name; full path; partial path (`[[b/a]]` against `x/b/a.md`); two `a.md` in different folders (`ambiguous_link`); an alias never resolves; attachment found and missing (`missing_attachment` warning); existing and missing fragment.
- **Golden fixture (G1):** vault in `internal/vault/testdata/obsidian/` with each link's expected result in `expected.json`, checked once in real Obsidian (logged in `docs/campo/dogfood.md`); the test compares tb against `expected.json`.
- **Frontmatter:** every row of the R4 classification table.
- **Security:** symlink out of the vault (scan and graph); symlinked intermediate directory in `--dir`; FIFO `x.md` in the vault (scan, `tb graph index` and `tb stack vault` terminate); oversized note; `vault.path: ../x`, with a newline, and with a glob character; missing `vault.path` (`vault_not_found`, exit 1); title with a newline (`bad_title`) and with a secret (redacted file name); secret in `--body` (written redacted); secret in a hand-edited note, including one split by the snippet window (redacted output in `show`, `search` and MCP) and a `search` for its prefix (no hit); `tb_vault_new` writing `inferred` even if the agent tries `source`; MCP `cwd` different from the server's.
- **Exclude:** `init` on a repository with the old section swaps `vault/` for `vault/.obsidian/` (whole-line check, not substring, because `.toolbelt/vault/.obsidian/` contains `.toolbelt/vault/`); a file with an orphan begin marker keeps the user's lines, drops the orphan marker and old tb lines, and a second `init` changes nothing; a custom `vault.path` produces the right pattern and directory; `vault_gitignored` when `.gitignore` ignores the vault.
- **Graph:** with `TB_GRAPH=on` and an old `config.yaml` (`docs: [markdown, html, txt]`), each note yields a single `note:` node and no `file:` node; edges for wikilink, fragment and tag; `vault.graph: false` keeps notes out; redacted note titles; `explain note:` and `path` between two notes; with the graph disabled, `tb vault` works the same.
- **Regression:** tests for `tb mem`, `tb egm`, `tb graph`, `json_snake_case`, registry parity and docs mirror pass.
- **Performance:** Go benchmark with 2,000 generated notes (NFR).
