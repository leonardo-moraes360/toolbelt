🇺🇸 English | [🇧🇷 Português](pt-BR/cli.md)

# `tb` Command Tree

This document describes the toolbelt CLI: **implemented** commands (mirrored from `tb --help`); planned items are marked inline, planned commands (`tb run`, `tb proc`, `tb serve`) are listed in the [README roadmap](../README.md), and `tb md` and `tb agent` are dropped. On divergence, the Cobra CLI is the source of truth.

## Local-first: no GitHub

`tb` **does not talk to GitHub** (or other forges): it is **100% local**. It reads
the working-tree git, files on disk, and `.toolbelt/`. It does not open issues,
download PRs, or call the GitHub API. That is why `tb review`, `tb drift`,
`tb verify --ticket`, and `tb stack` need an **`rd.md`** (or another local ticket
via `--ticket` / a positional argument) — not an issue number.

### How to obtain or generate `rd.md`

Write the file at the repo root (or pass `--ticket path`). Accepted formats:
YAML frontmatter or Markdown with `## Scope` / `## Escopo`,
`## Forbidden` / `## Proibido`, and acceptance checkboxes (details in `tb review`).

From an **issue** or **PR** (outside `tb`, with `gh` or the browser):

```bash
# Issue → local draft
gh issue view 19 --json title,body > /tmp/issue-19.json
# Copy title/body into rd.md and declare scope/forbidden/acceptance

# PR → body + touched files as a starting point for Scope
gh pr view 42 --json title,body,files > /tmp/pr-42.json
gh pr diff 42 --name-only
```

Minimal example:

```markdown
---
title: "fb3/19-docs"
base_commit: main
scope:
  - docs/cli.md
  - README.md
  - rd.md
forbidden:
  - tests/hidden/**
acceptance:
  - docs describe the real behavior
---

## Notes
Free text from the issue/PR (context for humans/agents).
```

Or Markdown only:

```markdown
# fb3/19-docs

## Scope
- docs/cli.md
- README.md
- `rd.md`

## Forbidden
- tests/hidden/**

- [ ] docs describe the real behavior
```

`tb ticket create --from rd.md` (Phase 3) creates the ticket from that file;
you still write the `rd.md` yourself.

## Overview

```
tb  (every top-level command; tier = stability under the 1.0 freeze)
├── init         Initializes local toolbelt configuration                                      [stable]
├── version      Shows the toolbelt version (also: tb --version)                               [stable]
├── mem          Invariants (add|list|show|edit|retire|search|check|init)                      [stable]
├── verify       Adversarial verification (run; rulers tests|coverage|visual)                  [stable]
├── guard        Anti-cheat protections (check)                                                [stable]
├── review       Fresh-context review (Elephant-Goldfish)                                      [stable]
├── drift        Detects scope and base commit drift                                           [stable]
├── stack        Stacks verification operations (verify|review|drift|guard stable; vault|egm|seccheck experimental)  [stable]
├── sandbox      Execution with Landlock isolation (check|run)                                 [stable]
├── graph        On-demand code graph (index|rebuild|search|callers|deps|impact|explain|path|stats; TB_GRAPH=on or --enable)  [stable]
├── ticket       Work tickets (create|list|show|cancel|retry)                                  [stable]
├── workspace    Isolated workspaces via git worktree (create|list|remove; alias: ws)          [stable]
├── merge        Merges done tickets (fail|last_wins|semantic stable; prompt experimental)     [stable]
├── mcp          Starts an MCP server over stdio                                               [stable]
├── board        Ticket board (list|watch|pause|resume|drain)                                  [experimental]
├── router       Foreground scheduler (run|status|config)                                      [experimental]
├── watchdog     Worker alerts and control (status|alerts|kill)                                [experimental]
├── bus          Agent bus (channel|invite|join|send|recv|claim|complete|heartbeat|watchdog|log|tree|run|discover)  [experimental]
├── egm          Design-document review loop (review|loop|resolve|status|receipt|drift-report|bootstrap|brainstorm|draft)  [experimental]
├── vault        Markdown vault of notes (new|list|show|search|links|backlinks|check)          [experimental]
├── red          Offensive AppSec playbooks (list|search|load|explain|pair)                    [experimental]
├── blue         Defensive AppSec playbooks and checks (list|search|load|pair|apply|check)     [experimental]
├── lab          Isolated red+blue exercise environments (new|list|status|up|run|logs|down|verify)  [experimental]
└── bench        Benchmark harness (run|list|report|compare)                                   [experimental]

Dropped (ADR-020, ADR-016): tb agent, tb md. Planned, not implemented: tb run, tb proc, tb serve.
```

**Stability tiers.** Every command and MCP tool is `stable` or `experimental`. Under the 1.0 freeze a stable command keeps its path, flags, defaults and exit codes, and a stable tool keeps its name, parameters and result shape; both only grow within 1.x. Experimental surface may change in any minor release: its help line starts with `(experimental)` and its MCP description with `[experimental]`. The `[stable]`/`[experimental]` column above is checked against the CLI by a test. Experimental: `bus`, `router`, `board`, `watchdog`, `egm`, `vault`, `red`, `blue`, `lab`, `bench`, `merge --strategy prompt`, and the `egm`, `vault` and `seccheck` ops of `stack`; tools `tb_bus_*`, `tb_router_status`, `tb_board`, `tb_watchdog_alerts`, `tb_egm_*`, `tb_vault_*`, `tb_red_*`, `tb_blue_*`, `tb_sec_pair`. A deprecated flag keeps working, hidden from help and with a stderr warning, until 2.0 (`--no-check`, see `tb verify run`).

MCP naming rule: a parameter is the CLI flag with `-` turned into `_`, plural when the flag is repeatable (`--skip-check` is `skip_checks`, `--egm-doc` is `egm_docs`); a positional argument becomes a named parameter. One documented exception: `tb_merge` takes `apply` (default `false`, a dry-run) where the CLI takes `--dry-run`, because an agent must opt in to moving refs. `cwd` is optional on every tool that takes one and defaults to the server's working directory.

---

## Detailed Commands

### `tb graph` — On-Demand Graph (✓ Phase 2, ADR-011)

Queries and indexes the code graph **on demand**. Disabled by default;
enable with `TB_GRAPH=on` or `--enable` (without that, CLI and MCP respond
`graph disabled (enable with --enable or TB_GRAPH=on; ADR-011)`). Measure ROI via
`tb bench compare --config-a bench/ab/graph-off.yaml --config-b bench/ab/graph-on.yaml`.

Languages and docs (extensible via `LanguageIndexer`, light regex, no cgo):

| `graph.languages` | Extensions |
|-------------------|------------|
| `go` | `.go` (AST; `calls` edges) |
| `javascript` | covers TypeScript (`ts`/`tsx`): `.js`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.jsx` (generic functions `foo<T>`, `const` arrow, JSX `<Comp />` as `calls`; indexes `*.test.*`/`*.spec.*`/`__tests__`/`e2e`; resolves NodeNext relative imports (`.js`→`.ts`, no extension, `index`) and `tsconfig.json`/`jsconfig.json` aliases (`extends`/`baseUrl`/`paths`); follows `import { a as b }` and `import * as ns`). There is no separate `typescript` language — use `javascript` |
| `python` | `.py` (defines/imports + `calls` edges by qualified module; `callers`/`impact` ignore edges with confidence &lt; 0.6) |
| `java` | `.java` |
| `kotlin` | `.kt`, `.kts` |
| `csharp` | `.cs` |
| `cpp` | `.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.hpp`, `.hh`, `.hxx` (defines/imports; no `calls` edges — `callers`/`impact` return an explicit error) |
| `rust` | `.rs` (defines/imports + minimal resolvable `calls` edges in the repository) |
| `php` | `.php` |
| `ruby` | `.rb` |
| `swift` | `.swift` |
| `shell` | `.sh`, `.bash`, `.zsh`, `.ksh` |

| `graph.docs` | Extensions | What it indexes |
|--------------|------------|-----------------|
| `markdown` | `.md` | document, headings, links, mentions |
| `html` | `.html`, `.htm` | document, `<h1>`–`<h6>`, anchors |
| `txt` | `.txt` | document, mentioned paths/symbols |

| `graph.frameworks` | Extensions / convention | What it indexes |
|--------------------|-------------------------|-----------------|
| `react` | `.tsx`, `.jsx` | components, `use*` hooks, `renders` edges (in parallel with `javascript` for `calls`/callers/impact) |
| `vue` | `.vue` | SFC script/template, imports, components in the template |
| `svelte` | `.svelte` | script, imports, components in markup |
| `astro` | `.astro` | frontmatter, imports, components in the body |
| `angular` | `*.component.ts` / `.html`, services/modules | `@Component`/`@Injectable`/`@NgModule`, selector, `templateUrl` |
| `routes` | Next `app/`/`pages/`, SvelteKit `routes/`, Nuxt `pages/` | `route` nodes by file convention |
| `mdx` | `.mdx` | doc (headings/links) + JSX imports/components |

Frameworks run together with the language indexer on the same file (e.g.
`.tsx` gets `react` + `javascript`, for components/`renders` and
`calls`/callers/impact). Docs can also index the same path when the
extension matches.

```yaml
# .toolbelt/config.yaml (all enabled by default)
graph:
  languages: [go, javascript, python, java, kotlin, csharp, cpp, rust, php, ruby, swift, shell]
  docs: [markdown, html, txt]
  frameworks: [react, vue, svelte, astro, angular, routes, mdx]
  max_file_bytes: 524288   # skips larger files (capped at 16 MiB)
  index_on_change: true    # reindexes on search/callers/deps/impact/explain/path/stats if the fingerprint changed
```

Ignores subdirectories such as `node_modules`, `vendor`, `dist`, `build`,
`target`, `.git`, etc. The walk root is never skipped by name (e.g. indexing
`/tmp/target` works; a child `target/` is still ignored). If the root is
a symlink (e.g. `/tmp/target` → real dir), it is resolved with
`EvalSymlinks` before the walk — otherwise the index would be empty, because
`filepath.Walk` does not descend into symlinks. The graph remains on demand (ADR-011):
`TB_GRAPH=on` or `--enable`.

The walk reads only regular files and never follows a symlink: a link inside
the repository (`docs/notes.md` → `../.env`) is not indexed, and a FIFO or
other special file is skipped instead of blocking `tb graph index`. Files are
read through the same reader as the vector index (no symlink on any path
component, regular file, size cap).

Go node IDs: a package is `pkg:<import path>` (the module path from the
nearest `go.mod` plus the directory; the root-relative directory when there is
no `go.mod`), so every `cmd/*` `package main` is its own node. A function is
`func:<import path>.<Name>` and a method `func:<import path>.<Recv>.<Name>`
(pointer and type parameters stripped from the receiver). Calls through an
import resolve to the imported package's ID, and `recv.m()` inside a method
resolves to the same receiver's method. `callers --function Name` still
matches a method by its bare name. Stored indexes carry `"version": 2`; an
index without it (or older) reads as stale and is rebuilt on the next
`tb graph index`. **Breaking for stored IDs:** `func:main.Run` and
`pkg:main` from earlier builds no longer exist.

Index freshness: each `graph.json` stores a `fingerprint` (mtime+size of
indexable files). On `search`/`callers`/`deps`/`impact`/`explain`/`path`/`stats`, if the
fingerprint diverges and `graph.index_on_change` is `true` (default), the graph is
reindexed automatically; if `false`, output warns and JSON includes
`stale: true`. Query JSON outputs use the envelope
`{"stale": false, "nodes": [...]}`; `stats`/`index` include `stale` in the
statistics object.

Over MCP, `tb_graph_search`/`callers`/`deps`/`impact`/`explain`/`path` never
reindex, whatever `graph.index_on_change` says: they are annotated
`readOnlyHint`, so a client can auto-approve or parallelize them. A stale index
returns `stale: true` with a `hint` to call `tb_graph_index`, which is the only
MCP graph tool that writes.

```bash
tb graph [--enable] <subcommand>

Subcommands:
  search    Searches the graph ✅
  callers   Finds callers of a function ✅
  deps      Shows dependencies ✅
  impact    Analyzes the impact of changes ✅
  explain   Explains a node: incoming/outgoing edges with provenance ✅
  path      Shortest path between two nodes (edges in both directions) ✅
  index     Indexes code on demand ✅
  rebuild   Rebuilds the full graph ✅
  stats     Graph statistics ✅
```

Graph MCP tools (call requires `TB_GRAPH=on`): `tb_graph_search`,
`tb_graph_callers`, `tb_graph_deps`, `tb_graph_impact`, `tb_graph_explain`,
`tb_graph_path`, `tb_graph_index`.

There is no `tb graph query` — use `tb graph search`.

**`tb graph search`:**

```bash
tb graph search <query> [flags]

Flags:
  --limit <n>         Maximum results (default: 20)
  --mode <mode>       lexical | semantic | hybrid (default: lexical)
  --json              JSON output (global flag)
```

Results come in three tiers: (1) title equal to the query; (2) nodes that contain the query as a substring of title/path/package/ID — what the old search returned; (3) nodes that only match terms. Within each tier, BM25 ranking over title, path and package (title weighs 3×), with camelCase, `snake_case` and path separators split into terms; a multi-word query matches any term. Ties are ordered by ID. With more results than `--limit`, the cut follows this order.

With `--mode semantic` the query is embedded and ranked by cosine similarity over the file chunks, grouped by file node; `--mode hybrid` fuses the lexical and semantic rankings with RRF (k = 60), ties by node ID. Both need [the vector index](#vector-index) and never reindex it. JSON for those modes: `{"stale", "mode", "stale_vectors", "nodes": [...]}`, each node with its usual fields plus `lines`, `score_semantic`, `rank_lexical`, `rank_semantic` and `score_fused`; text output appends `:<lines>` to the path. The default `lexical` output is unchanged.

**`tb graph callers`:**

```bash
tb graph callers [flags]

Flags:
  --function <name>   Function name (accepts a qualified suffix, e.g. scrubber.redact_secrets)
  --depth <n>         Depth (default: 1)
```

There is no `--file` on `callers` (use `--function`). For impact by file, use `tb graph impact --file`.

In Python/JS/TS, `--function` resolves by title or qualified id suffix
(`func:pkg.mod.nome`; React components/hooks too; Go: see node IDs above). `calls` edges with
confidence &lt; 0.6 (ambiguous targets / no import) are left out of the default
`callers` and `impact` result. In TS/JS, relative imports
(`../components/Layout`, `./utils.js` → `utils.ts`, `./index`) and
`tsconfig`/`jsconfig` aliases (`@/…`) resolve to the imported file/symbol;
`import { helper as format }` and `import * as Utils` / `Utils.helper()` also
feed `callers`/`impact` (including functions in test files).

**`tb graph deps`:**

```bash
tb graph deps [flags]

Flags:
  --module <name>     Module/package
  --file <path>       File
  --transitive        Includes transitive dependencies
```

**`tb graph impact`:**

```bash
tb graph impact [flags]

Flags:
  --file <path>       Modified file
  --depth <n>         Depth (default: 2)
```

**`tb graph explain`:**

```bash
tb graph explain <node> [flags]

Flags:
  --limit <n>         Maximum explained nodes (default: 5)
```

Shows the node with its outgoing and incoming edges (`source`, `confidence`).
The node resolves by exact ID → exact title → function suffix → file/package.
Calls whose target has no node in the index (stdlib, unresolved receiver) are
counted in `unresolved_out`. Up to 50 neighbors per direction (`truncated_*`).

**`tb graph path`:**

```bash
tb graph path <from> <to> [flags]

Flags:
  --max-depth <n>     Max hops (default: 6)
```

Shortest path (BFS) walking edges in both directions; each step says
`forward` (→) or `backward` (←). JSON: `{"stale", "hops", "steps": [...]}`.
No path or unknown node ⇒ error and exit ≠ 0. Both ignore `calls` with
confidence &lt; 0.6 ([ADR-014](adr/014-graphify-reimplementacao.md)).

**`tb graph index`:**

```bash
tb graph index [path] [flags]

Flags:
  --vectors           Also build or update the vector index (.toolbelt/graph.vec; needs graph.vector.enabled)
```

The path is **positional** (there is no `--path`).

With `--vectors` (also on `tb graph rebuild`), after indexing tb builds or updates `.toolbelt/graph.vec` (mode 0600, written atomically), re-embedding only the chunks whose text sha256 changed. JSON gains `vectors`: `{path, files, units, embedded, reused, dropped, denied, truncated, rebuilt}`; text output prints a `vectors: …` line.

**Examples:**

```bash
# Search
tb graph search "ValidateToken"
# NODE_ID      TYPE       PATH                      TITLE
# node-123     function   src/auth/jwt.go          ValidateToken

# Callers of a function
tb graph callers --function "ValidateToken" --depth 2

# Dependencies of a module
tb graph deps --module "auth" --transitive

# Change impact
tb graph impact --file "src/config/parser.go"

# Re-index code after changes (positional path)
tb graph index src/

# Rebuild the full graph (optional positional path, like index)
tb graph rebuild
tb graph rebuild src/

# Statistics
tb graph stats
```

#### Vector index

Semantic and hybrid search ([spec](specs/vector-index.md)) are off by default. The keys are read **only from the user config** (the `-c` file or the global config); values in the repository's `.toolbelt/config.yaml` are ignored with a `config_untrusted` warning.

```yaml
graph:
  vector:
    enabled: true
    max_chunks: 50000
    chunk_lines: 40
    chunk_bytes: 1536
    overlap_lines: 8
  embed:
    model: /home/me/models/embed.gguf
    model_sha256: <hex>
    query_prefix: "query: "
    doc_prefix: "passage: "
    threads: 4
    timeout: 2m
    allow_unsandboxed: false
```

- `graph.vector.enabled` (default `false`); `graph.embed.model` is an absolute path to a GGUF file and `graph.embed.model_sha256` is required.
- `graph.embed.threads` defaults to min(4, NumCPU), `graph.embed.timeout` to `2m`; `graph.vector.max_chunks` 50000; `chunk_lines`/`chunk_bytes`/`overlap_lines` 40/1536/8.
- The embedding child `tb __graph-embed` runs sandboxed (Landlock + seccomp: no sockets, read-only empty workdir, private tmp, empty `HOME`, minimal env). Deny-listed files (secrets) are never embedded, and each file is redacted as a whole before chunking. `graph.embed.allow_unsandboxed: true` lets it run without the full sandbox (`embed_unsandboxed` warning).
- Reasons: `vectors_disabled`, `vectors_stale`, `vectors_invalid`, `vectors_too_many`, `model_path`, `model_mismatch`, `model_unsupported`, `embed_unavailable`, `embed_unsandboxed`, `embed_protocol`, `embed_crashed`, `embed_timeout`, `config_untrusted`. A build without `embednative` answers `embed_unavailable`.
- MCP: `tb_graph_search` takes `mode` (`lexical`|`semantic`|`hybrid`, default `lexical`) and returns the same JSON as the CLI (`mode`, `stale_vectors`, nodes with `lines`, `score_semantic`, `rank_lexical`, `rank_semantic`, `score_fused`) plus a `warnings` array; `tb_graph_index` takes `vectors` (boolean, default `false`) and its payload gains the `vectors` stats. Without them the output is unchanged. MCP queries never reindex vectors. Errors use the same reasons, with the warnings appended to the error text.
- `tb mcp` keeps one warm embedding child per server: reused across queries, restarted when the embed config changes or the child breaks, killed after 5 minutes idle and on exit.
- Evaluation (44 intent queries over this repository, all-MiniLM-L6-v2 Q8_0): recall@10 lexical 0.273, semantic 0.670, hybrid 0.568. `semantic` scored highest on this suite; `hybrid` keeps exact-name matches from the lexical ranking. See [dogfood](campo/dogfood.md) #45.

---

### `tb mem` — Invariants (✓ Phase 1)

Manages the `.toolbelt/invariants.yaml` file (outside compaction, ADR-010).

```bash
tb mem <subcommand>

Subcommands:
  add       Adds an invariant
  list      Lists invariants (omits retired; --all includes them)
  show      Shows an invariant by id
  edit      Edits fields of an invariant
  retire    Retires an invariant (soft-retire; omitted from list/check/search by default)
  search    Searches invariants
  check     Checks the integrity of the invariants file
  init      Creates default anti-cheat invariants
```

Notes that are not invariants live in the Markdown vault: see [`tb vault`](#tb-vault--markdown-vault-phase-a). Invariants are never mirrored there ([ADR-016](adr/016-vault-fase-a.md)).

An unknown subcommand (`tb mem foo`) exits with code ≠ 0 and message `unknown command`.

**`tb mem add`:**

```bash
tb mem add <rule> [flags]

Flags:
  --category <cat>    Category: anti_cheat, mandate, security (default: anti_cheat)
  --ref <path>        ADR/decisions.md path (e.g. docs/adr/0001.md)
  --immutable         Marks the invariant as immutable (default: mutable)
```

**Immutability default:** new invariants start **mutable**. Use `--immutable` (CLI) or `immutable: true` (MCP) to protect them. `tb mem init` writes the anti-cheat set already with `immutable: true`. Category `anti_cheat` and `immutable: true` require `--force --reason` for `edit`/`retire`.

Invalid category or `--ref` outside the ADR/decisions format exits ≠ 0 with the list
of valid values (`anti_cheat, mandate, security`; refs like
`docs/adr/<file>.md` / `docs/decisions/<file>.md`).

**`tb mem list` / `search` / `check`:**

```bash
tb mem list [--category <cat>] [--all]
tb mem search <query> [--all]
tb mem check [--expect-sha <sha>] [--all]
```

`search` is case- and accent-insensitive (Unicode NFD + mark stripping).

**`tb mem show` / `edit` / `retire`:**

```bash
tb mem show <id>
tb mem edit <id> [--category <cat>] [--rule <text>] [--ref <path>] [--force] [--reason <reason>]
tb mem retire <id> [--force] [--reason <reason>]
```

**`tb mem init`:** creates the default anti-cheat set and prints the path + the
created invariants (or `N invariants already existed in <path>` if the file already had entries). With
`--json`, emits `path`/`created`/`count`/`invariants`.

**`tb mem check` (implemented):**

```bash
tb mem check [flags]

Flags:
  --expect-sha <sha>  Expected hash after compaction
  --all               Include retired invariants
```

`mem check` **fails closed**: missing/unreadable/corrupt file, divergent SHA, missing IDs, or non-evaluable rule (empty id/category/rule) → exit ≠ 0. Does not auto-create defaults (use `tb mem init`).

**Redaction:** `show`/`list`/`search` (text and `--json`) and equivalent MCP payloads mask common patterns (`ghp_`/`gho_`/`github_pat_`, `AKIA…`, PEM private key, `Bearer`, `xox*`, `*_TOKEN=`) with `[REDACTED]`.

**History:** forced changes to protected invariants are append-only in `.toolbelt/invariants_history.jsonl` (who, when, reason, previous value).

**Examples:**

```bash
# Add an invariant (required positional argument)
tb mem add "Do not modify tests/hidden/**"
tb mem add --category security --ref docs/adr/0005.md "Do not relax thresholds without review"
tb mem add --immutable --category mandate "Fixed ticket scope"

# List / search / show / edit / retire
tb mem list
tb mem list --category anti_cheat
tb mem list --all
tb mem search thresholds
tb mem search cafe   # finds "café"
tb mem show inv-123
tb mem edit inv-123 --ref docs/decisions/auth.md --rule "Scope via ticket"
tb mem edit inv-123 --rule "new" --force --reason "incident"
tb mem retire inv-123
tb mem retire inv-123 --force --reason "superseded by ADR"
tb mem check
tb mem init
tb mem init --json
```

> Soft-retire keeps the entry in the YAML; `list`/`search`/`check` omit retired unless `--all`.
> Vault flags `--type`/`--title` **do not exist** — `tb mem add` accepts a positional rule, `--category`, `--ref`, and `--immutable`.
> `tb mem init --json` returns `{path, created, count, invariants}`; empty lists come out as `[]`.

---

### `tb red` / `tb blue` — AppSec Playbooks (✓ Phase 1, ADR-017)

Native offensive (red) and defensive (blue) AppSec methodology, paired by vulnerability class. Search is compact; `load` respects a token budget (default 2000) and slices by `##` section. Playbooks are reference text — the read paths never execute anything. Blue playbooks declare grep/regex checks that `tb blue apply` registers into `.toolbelt/secchecks.yaml` and `tb blue check` runs over the working tree. `tb red` prints a one-time authorized-use notice per repo.

```bash
tb red search "deserialization"
tb red list --area web
tb red load red-sqli --section Detection
tb red load red-rce --budget 1500
tb red load red-xxe --full
tb red pair red-sqli
tb red explain sql-injection
tb blue search "detection"
tb blue apply sql-injection --dry-run
tb blue check
```

MCP parity: `tb_red_search`, `tb_blue_search`, `tb_red_load`, `tb_blue_load`, `tb_sec_pair` (read-only), `tb_blue_apply` (writes `secchecks.yaml`). See [docs/specs/appsec-playbooks.md](specs/appsec-playbooks.md).

With `TB_GRAPH=on`, each playbook is a `playbook:<name>` graph node and every red playbook has a `mirror` edge to its blue counterpart, so `tb graph path playbook:red-sqli playbook:blue-sqli` and `tb graph explain` link attack and defense. `tb stack seccheck` runs the registered checks as a read-only stack op (fails on error-severity findings unless `--warn-only`).

---

### `tb lab` — Isolated red+blue labs (✓ Phase 1 sandbox tier, ADR-017)

Ephemeral, isolated environments that prove a red technique works **and** its paired blue check detects it, against a lab-local target — never the host, never a third party. Isolation is tiered: a single-step lab runs on the existing sandbox (Landlock, in-process pure-Go target); a multi-service topology runs on rootless Podman. Phase 1 implements the **sandbox tier**; the **Podman tier fails closed** (clear remediation) when Podman is unavailable rather than degrading isolation. Manifests live at `labs/<name>/lab.yaml` (versioned); runtime state lives outside the repo. `tb lab` shares the one-time authorized-use notice with `tb red`.

```bash
tb lab new demo --class sql-injection
tb lab list
tb lab status sqli
tb lab up sqli
tb lab up sqli --tier podman
tb lab run sqli
tb lab logs sqli
tb lab down sqli
tb lab verify
```

`tb lab run` reports a verdict the agent can act on: `attack=reproduced|failed` and `defense=detected|missed`. The seed `labs/sqli/lab.yaml` reports `attack=reproduced defense=detected`. The Podman topology tier, cgroups resource caps, artifact export, and MCP `tb_lab_*` tools are Phase 2. See [docs/specs/red-blue-labs.md](specs/red-blue-labs.md).

---

### `tb md` and `tb agent` — dropped

> **Dropped, not planned.** `tb md` (Markdown TUI) left with the vault's Phase A ([ADR-016](adr/016-vault-fase-a.md)): any editor, Obsidian included, edits the vault; see [`tb vault`](#tb-vault--markdown-vault-phase-a). `tb agent` (`spawn --parent`, `list`, `approve`, `reject`, `budget`) was dropped by [ADR-020](adr/020-agent-hierarchy-a2a.md): an agent is admitted with `tb bus invite` + `tb bus join`, run with `tb bus run` or `tb router run`, and watched with `tb watchdog` / `tb bus watchdog`; there is no pending-action queue to approve. Delegation trees on the bus are specified in [specs/agent-hierarchy.md](specs/agent-hierarchy.md).

---

### `tb stack` — Verification Operation Stacking (✓ Phase 2)

Runs `verify`, `review`, `drift`, `guard` and/or `egm` in a single invocation, sharing
expensive work (unified diff, changed-file index, memory/invariants, and
config). Independent read-only steps (`guard`, `drift`, `review`) run in
parallel (errgroup + `--jobs`); `verify` (build/test) stays serial/isolated.
JSON output preserves the order requested in the arguments, regardless of
completion order. Output is always aggregated JSON
(`ok`, `steps`, `total_duration_ms`); the process exit code is ≠ 0 if
any step fails (same semantics as each standalone command). Durations are
`float` milliseconds (with sub-ms fraction). Preparation or operation errors
appear in `steps[].error` inside the JSON (not only on stderr).

**Not a branch stack** (like Graphite or git-stack). `tb stack` only stacks
verification/review operations with shared context.

```bash
tb stack <op> [<op>...] [flags]

Flags:
  --base <ref>           Shared base ref (default: HEAD~1; validated/resolved)
  --head <ref>           Shared head ref (default: HEAD; validated/resolved)
  --fail-fast            Cancel remaining steps via context on the first failure
  --jobs <n>             Concurrency limit (default: CPU count, max 32; 1 = serial)
  --ticket <path>        Ticket/rd.md (review/drift/verify; default: rd.md)
  --approved-sha <sha>   Approved SHA; verbatim patch-id (review/drift; validated/resolved)
  --approved-base <ref>  Base of the approved range (default: merge-base(approved, base); validated/resolved)
  --require-approved-sha Fail if --approved-sha is missing (or `review.require_approved_sha`)
  --warn-only            guard: exit 0 even with SeverityError
  --test-cmd <cmd>       verify: visible tests
  --hidden-test-cmd <cmd> verify: hidden tests
  --build-cmd <cmd>      verify: build command (empty + explicit flag or --no-build to skip the config one)
  --lint-cmd <cmd>       verify: lint command (empty + explicit flag or --no-lint to skip)
  --no-build             verify: skips build_cmd
  --no-lint              verify: skips lint_cmd
  --ruler <list>         verify: comma-separated rulers (tests, coverage; visual is refused here)
  --coverage-cmd <cmd>   verify: coverage command
  --coverage-pattern <re> verify: regexp with one capture group for the coverage percentage
  --min-coverage <n>     verify: coverage floor in [0, 100] (only raises the base threshold)
  --typecheck-cmd <cmd>  verify: typecheck command (gate step after build)
  --no-typecheck         verify: skips typecheck_cmd
  --check <name=cmd>     verify: adds or overrides a named check (repeatable)
  --skip-check <name>    verify: skips one named check (repeatable; --no-check is a hidden deprecated alias until 2.0)
  --no-checks            verify: skips every named check
  --dry-run              verify: shows the plan (cmds/dir) without executing
  --max-iter <n>         verify: max iterations
  --timeout <duration>   verify: timeout (default: 10m)
  --reviewers <n>        review: max goldfish
  --standards <csv>      review: roles (scope,quality,acceptance)
  --egm-doc <path>       egm: document added to the selection (repeatable)
  --strict               egm: PASS_PARTIAL and an empty selection fail
  --egm-mode <mode>      egm: local (default) | ci
  --egm-allow-offline    egm ci: without a credential, a deterministic-only round capped at PASS_PARTIAL
  --env <KEY>            (global) extra allowlist for stacked verify
  --pass-env <KEY>       (global) explicit secret opt-in for verify
```

With no operation, or an unknown operation, exits with a short error listing the valid
ops (`verify`, `review`, `drift`, `guard`, `egm`) — no `Usage` dump.
A missing or unreadable ticket (file/ID pointed to by `--ticket`) makes the
`review`/`drift` steps fail with exit ≠ 0 and a message citing the path looked up
(same semantics as `tb review` / `tb drift` / MCP).

**Examples:**

```bash
tb stack verify review drift guard
tb stack guard drift review --jobs 4
tb stack guard drift --fail-fast
tb stack review drift --base HEAD~1 --head HEAD --ticket rd.md
tb stack verify guard --test-cmd "true" --max-iter 1
tb stack guard drift review --jobs 1
tb stack verify --test-cmd "pnpm test" --no-build --no-lint --max-iter 1
tb stack verify --typecheck-cmd "pnpm exec tsc --noEmit" --check "audit=pnpm audit" --max-iter 1
tb stack verify --dry-run --test-cmd "go test ./..."
tb stack verify --test-cmd "true" --env APP_ENV --pass-env GITHUB_TOKEN
```

Each `steps` item carries `op`, `ok`, `exit_code`, `duration_ms`, `result` (the JSON
the standalone command would produce) and, on failure, `error`. In `Prepare`,
independent git calls (`diff` / `name-status` / `name-only`) and invariant loading
also run in parallel. Stacked `verify` uses the same shared `--base`/`--head`:
if head diverges from the worktree `HEAD`, build/lint/test run in a temporary
detached worktree (see `tb verify`); the step `result` includes `head_sha` and
`workdir`.

**`egm` op (gate):** selects the gated documents (`egm.doc_globs`, default
`docs/specs/**` and `docs/adr/**`; a `README.md` index is never selected) added or
modified in `merge-base(base, head)..head`, plus `--egm-doc`, and evaluates each one with `tb egm status --at <head>`. The
document, its sources and the repository config come from the commit, so a converged
document that changed after the review is `STALE`. `PASS` is `ok`; `PASS_PARTIAL` is
`ok` with `warn: true` (`ok: false` with `--strict` or `egm.require_full_coverage`);
any other verdict, or a lineage with `forced_blocker`, is `ok: false`. An empty
selection is `ok: true, warn: true` with `no_gated_docs` (`ok: false` with
`--strict`), and `ok: false` with `uncommitted_gated_docs` when the worktree has
gated documents that differ from head. A gated document that was not touched but cites a path that
changed in the range gets a `sources_changed` warning for each path of its source drift
(against its receipt, or a converged local round; see `tb egm drift-report`). `egm`
never runs in parallel and is always moved to the end; `Result.warn` is the OR of the
steps' `warn` and never changes the exit code. In the default `--egm-mode local`
the gate is local (`trust: "local"`): in a CI checkout there is no state, so the result
is `NO_STATE`. It also warns `receipt_missing`, `receipt_invalid` or `receipt_stale`
(never a failure) so the author sees what CI will reject, `gated_doc_removed` for a
gated path deleted or renamed out of the gate, and `receipt_removed` for a deleted receipt
of a document that is still gated.

**`--egm-mode ci`** ([egm trust](specs/egm-trust.md); setup guide in [egm-ci.md](egm-ci.md)) is the gate for CI
(`trust: "ci"`). It needs `-c` (CI's own egm user config) and implies `--strict`.
A range that edits a receipt selects its document; deleting the receipt of a document that is
still gated fails (`receipt_removed`). `ci_source_drift` flags an untouched document that was
fresh against its **merge-base** receipt and drifted in this range; it is a warning unless
`egm.drift.ci: fail` (the repository config can only raise it).
For each selected document it checks, in order: the receipt committed at head
(`tb egm receipt`) exists and is valid (`no_receipt`, `receipt_invalid`); its document
and source hashes equal the commit's (`receipt_stale`); then it runs **one fresh
round** with `--engine llm` in a throwaway state. A fresh round with a downgrade is `ci_partial`. A fresh finding at `major` or above
fails as `ci_finding`, unless it matches a receipt claim (same heading, both with lines
within `egm.match.gap`, `sim ≥ egm.match.sim`, not more severe): a `declined` or `forced`
claim fails as `ci_downgrade`; an `arbiter_withdrawn` claim on a contestable finding
(not a checker, `injection-scan` or comprehension) is re-arbitrated by CI's own arbiter
(`withdraw` passes, anything else is `ci_upheld`). Findings below `major`
only warn. A required reviewer that does not complete fails (`ci_not_run`). Without a
credential (an unset `egm.env.pass` variable, or `auth_expired`) the result is `ci_offline`; a broken setup (no `agent_cmd`, no Landlock) is exit 2, unless `--egm-allow-offline` runs a
deterministic-only round capped at `PASS_PARTIAL`. More selected documents than
`egm.ci.max_docs` (default 5; the gate globs are the union of the merge-base's and head's, so the PR's config cannot narrow them), or a step past `egm.max_agent_runs`, is `ci_budget`; a
removed gated document fails. Each document's `ci_findings` lists the fresh findings
with their `result`.

```bash
tb stack verify guard egm --base origin/main
tb stack egm --base origin/main --egm-doc docs/adr/015-egm-nativo.md --strict
tb stack egm --egm-mode ci -c "$RUNNER_TEMP/egm.yaml" --base "$BASE_SHA" --head "$HEAD_SHA"
```

---

### `tb review` — Fresh-Context Review (✓ Phase 2)

Implements the Elephant-Goldfish pattern: **distinct** goldfish review the diff
against `ticket/rd.md` and `tb mem` invariants, without inheriting the author's context.
There are no clones of the same checker — `--reviewers` only limits how many roles run.

```bash
tb review [ticket] [flags]

Flags:
  --base <ref>               Base commit/ref (default: HEAD~1 or the ticket's base_commit)
  --head <ref>               Head commit/ref (default: HEAD)
  --approved-sha <sha>       Approved SHA; compares `git patch-id --verbatim` of base..approved vs base..head
  --approved-base <ref>      Base of the approved range (default: merge-base(approved, base))
  --require-approved-sha     Fails if `--approved-sha` is missing (or `review.require_approved_sha` in config)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --reviewers <n>            Maximum distinct goldfish (default: 3; does not clone)
  --standards <stds>         Roles: scope,quality,acceptance (default; aliases: invariants,completeness)
  --output <path>            JSON report (no `-o` shorthand; use `--output`)
  --json                     JSON output (global flag)
```

A missing or unreadable ticket exits with code ≠ 0 and message
`ticket not found: <path>` / `ticket unreadable (<path>): …`
(CLI, MCP `tb_review`, and `tb stack`/`tb_stack`).

Also reports scope and base-commit drift. Fresh isolation is mandatory.
`tb review -o …` is rejected (`unknown shorthand flag: 'o'`); use `--output`.

Base and head are resolved to SHA **once** at the start of the flow (anti-TOCTOU) and
that pinned SHA is used for the rest.

The JSON always includes `pinned_approval`. Without `--approved-sha`:
`{"checked": false}`. With the flag: `checked`, `approved_sha`, `patch_id`,
`head_patch_id`, `match`, and if they diverge, `range_diff`. Comparison uses
`git patch-id --verbatim` (whitespace-sensitive). A patch different from the approved one
prints a `git range-diff` summary and exits with code ≠ 0. If `--approved-sha`
coincides with the base (empty `base..approved` range), the CLI does **not** dump
`git range-diff` usage — it shows a clear message about the invalid range.
With `--require-approved-sha` (or `review.require_approved_sha: true` in
`.toolbelt/config.yaml`), missing `--approved-sha` is an error.

The ticket/rd.md accepts YAML frontmatter or Markdown: section `## Scope` / `## Escopo`
(or `Scope:` / `Escopo:`) and `## Forbidden` / `## Proibido` (or `Forbidden:` /
`Proibido:`) with lists (`-`, `*`, numbered) and paths in backticks.
Acceptance criteria come from `acceptance:` in YAML or `- [ ]` / `- [x]` checkboxes
in Markdown. If the YAML frontmatter does not declare `scope:`, the Markdown body is still
read to fill the scope.
Globs in scope/forbidden include `src/*.go`, `src/**`, `tests/hidden/**`, and
patterns with `**` in the middle (e.g. `src/**/foo.go`). Touches on Forbidden paths
fill `forbidden_hit` in drift and produce a critical finding in review.

Path invariants (e.g. `tests/hidden/`, `thresholds.yaml`) are matched against
files **changed** by the same path/glob matcher (`tests/hidden/**`,
`tests/hidden/*`) — not by substring in the diff or ticket text. Mentioning
`forbidden: tests/hidden/**` in `rd.md` without touching that directory does not produce
a critical finding. Mandate content invariants (e.g. forbid
`console.log` or comments) are applied to added (`+`) lines of the diff.

**Distinct reviewers (goldfish)**:

| Reviewer | `--standards` | What it checks |
|----------|---------------|----------------|
| `goldfish-scope` | `scope` (+ `invariants`) | Files outside scope, touches on forbidden; `tb mem` invariants applied to changed paths **and** to `+` lines of the diff (e.g. `console.log`, forbidden comments) |
| `goldfish-quality` | `quality` | `TODO`/`FIXME`/`XXX`/`HACK` markers (whole uppercase word, in comments); very large functions in the diff (≥80 lines +); changed source without tests |
| `goldfish-acceptance` | `acceptance` (alias: `completeness`) | Each ticket acceptance criterion has evidence in changed tests/code files (does not count the ticket itself); considers `Test*` (Go) and `test`/`it`/`describe` (JS/TS) added in the diff |

Heuristics are deterministic and offline (no LLM).

Each finding JSON includes `role` (goldfish role: `scope`, `quality`, `acceptance`, `invariants`).
Acceptance findings also carry `criterion` and `evidence` (tests/files seen in the diff).
`agreement` counts how many distinct roles agreed on the same finding (usually 1).
In text mode, prints `role=<role>` (and `agreeing_roles=N` only if N>1) — no longer uses the old `acordo=N/M`.

**Examples:**

```bash
# Default review: 3 distinct roles
tb review --ticket rd.md
# ═══════════════════════════════════════════════════════════
#  REVIEW REPORT: rd.md
#  Base..Head: <base-sha>..<head-sha> | Reviewers: 3 | Fresh: true
# ═══════════════════════════════════════════════════════════
# FINDINGS:
#   [warning] (goldfish-quality/quality) TODO/FIXME/XXX/HACK added in a comment in the diff [src/a.go]  role=quality
#   [warning] (goldfish-acceptance/acceptance) acceptance criterion without evidence in changed tests/files: works  role=acceptance

# Scope + quality only (no acceptance)
tb review --ticket rd.md --standards scope,quality --reviewers 2

# Asking for 5 reviewers still runs at most the distinct roles (no clones)
tb review --ticket rd.md --reviewers 5 --standards scope,quality,acceptance

# JSON output for integration (--output only; no -o)
tb review --ticket rd.md --json --output review-results.json

# Check that head is still the same patch as the approved SHA
tb review --base main --approved-sha abc1234 --ticket rd.md --json
```

**Mandatory isolation**: If the system cannot create fresh contexts for the reviewers, the command fails with an error instead of proceeding with contaminated context.

---

### `tb drift` — Scope/Base Drift (✓ Phase 2)

Compares files changed in `base...head` with the **scope** declared in the
ticket/`rd.md`, detects touches on `Forbidden`/`Proibido` (`forbidden_hit`) and
a base that is not an ancestor of head (`base_mismatch`). It does **not** compare the
Markdown vault against code — the report is only scope/base/forbidden
(+ optional `pinned_approval`).
The scope/forbidden parser is the same as `tb review`
(YAML frontmatter or Markdown `## Scope` / `## Forbidden` / lists / globs with `**`).

The diff uses **three-dot** (`git diff base...head` ≡
`git diff $(git merge-base base head) head`), the same criterion as GitHub PRs:
after merge/squash on the base, only branch files enter — not what came from main.

Explicit `--base` on the CLI/MCP **always** takes precedence over the ticket's `base_commit`.
Without `--base`, the default `HEAD~1` yields to `base_commit` when the ticket declares one.
Refs (`HEAD^`, tags, short SHA) are resolved with `git rev-parse`.
`base_mismatch` is `true` when the ticket's declared `base_commit` (or the effective
base, if the ticket does not declare one) is **not** an ancestor of head
(`git merge-base --is-ancestor`) — so `--base main` on a divergent PR does not
produce a false mismatch; a SHA from another line (not an ancestor) does. JSON exposes
`actual_base` (resolved SHA of the base used in the diff) and `merge_base` (common
ancestor with head).

```bash
tb drift [ticket] [flags]

Flags:
  --base <ref>               Base commit/ref (default: HEAD~1 or the ticket's base_commit)
  --head <ref>               Head commit/ref (default: HEAD)
  --approved-sha <sha>       Approved SHA; compares `git patch-id --verbatim` of base..approved vs base..head
  --approved-base <ref>      Base of the approved range (default: merge-base(approved, base))
  --require-approved-sha     Fails if `--approved-sha` is missing (or `review.require_approved_sha`)
  --ticket <path>            Ticket/rd.md (default: rd.md)
  --output <path>            JSON output (no `-o` shorthand)
  --json                     JSON output (global flag)
```

A missing or unreadable ticket exits with code ≠ 0 and message
`ticket not found: <path>` / `ticket unreadable (<path>): …`
(CLI, MCP `tb_drift`, and `tb stack`/`tb_stack`).

The drift JSON includes `pinned_approval` in the same format as `tb review`
(`checked: false` without the flag; verbatim patch-id with the flag). Divergence exits
with code ≠ 0 and shows `git range-diff`. Base/head are pinned to SHA at the start.
`--approved-sha` equal to the base follows the same clear message as `tb review` (no
usage dump).

**Examples:**

```bash
# Drift of the HEAD~1..HEAD diff against rd.md (default)
tb drift
# ═══════════════════════════════════════════════════════════
#  DRIFT REPORT
# ═══════════════════════════════════════════════════════════
# PINNED APPROVAL:
#   checked: false
#
# Base: declared= actual=<sha> merge_base=<sha>
# Out of scope (1):
#   - b.go
# Forbidden touched (0):

# Explicit ticket + JSON output for CI
tb drift --ticket rd.md --json --output drift-report.json

# Approval pin: fails if the head patch changed since the approved SHA
tb drift --base main --approved-sha abc1234 --ticket rd.md
```

JSON (`--json` / `--output`) uses `out_of_scope`, `forbidden_hit`,
`base_mismatch`, `declared_base`, `actual_base` (and `pinned_approval` when
`--approved-sha` is set). Exit ≠ 0 if there is drift or a divergent pinned patch.

**CI integration**:

```bash
# In CI, fail if there are files out of scope
tb drift --ticket rd.md --json | jq '.out_of_scope | length' | xargs -I {} test {} -eq 0
```

---

### `tb egm` — Design-Document Review Loop (Phase A)

Reviews specs and ADRs and repeats the round until two clean rounds over the same
content converge ([spec](specs/egm-nativo.md), [ADR-015](adr/015-egm-nativo.md)).
Both engines run the checkers `refs-exist`, `required-sections` and
`plan-files-listed` plus the anti-injection scan. The **`llm` engine** also runs the
9 reviewers of the consolidated preset as goldfish: one-shot `egm.agent_cmd`
processes in a Landlock sandbox with a read-only packet, a disposable HOME, a minimal
env and no tools (Linux). Without `egm.agent_cmd`, `auto` uses the **deterministic
engine**, where the LLM personas are `unavailable` and the best verdict is
`PASS_PARTIAL`. `status --at <rev>` is the gate mode used by the `egm` op of
[`tb stack`](#tb-stack--verification-operation-stacking--phase-2).

```yaml
# ~/.config/toolbelt/config.yaml
egm:
  agent_cmd: [/home/me/.local/bin/claude]   # absolute; vendor claude adds the no-tools flags
  # timeout_per_reviewer: 5m, max_parallel: 4, max_agent_runs: 120, clean_rounds: 2, max_rounds: 8
```

With the `claude` profile, tb copies only the `claudeAiOauth` key of
`~/.claude/.credentials.json` into each goldfish's disposable HOME and refuses to start
(`auth_expired`) when the access token has less than 10 minutes left: a copy cannot
refresh it, so run `claude` once outside tb first. A round of 9 reviewers costs about
US$ 2–2.5.

```bash
tb egm review docs/specs/x.md            # round 1 (exit 0; 1 on FAIL)
tb egm loop next docs/specs/x.md         # next round; refused when nothing changed or already converged (exit 0)
tb egm resolve docs/specs/x.md --fix F-1,F-2
tb egm resolve docs/specs/x.md --decline F-4
tb egm resolve docs/specs/x.md --force-settle F-3 --reason "accepted risk"
tb egm resolve docs/specs/x.md --contest F-5 --evidence internal/x.go:40   # arbiter runs on the next loop next
tb egm status docs/specs/x.md [--strict]  # 0 PASS, 1 FAIL, 3 IN_PROGRESS/STALE/NO_STATE
tb egm status docs/specs/x.md --at HEAD   # gate mode: document, sources and config from the commit
tb egm receipt docs/specs/x.md            # writes .toolbelt/egm/receipts/<slug>.json (needs PASS or PASS_PARTIAL)
tb egm drift-report [doc...] [--at REV]   # source drift against the receipt: 0 fresh/no_baseline, 1 drift, 2 error
tb egm drift-report --json                # v egm-source-drift/0.2: adds symbols_changed, symbols_missing, outside_symbols
tb egm drift-report --engine llm          # plus one drift goldfish per drifted doc (egm.drift.max_docs); findings reported, exit unchanged
tb egm bootstrap docs/specs/new.md --from internal/x/   # new skeleton wired to the code (never overwrites, never commits); refusals exit 2: usage, bootstrap_exists, bootstrap_path, bootstrap_sources
tb egm bootstrap docs/specs/new.md --from internal/x/ --engine llm   # a goldfish drafts the body; also bootstrap_injection, bootstrap_invalid, bootstrap_not_run, tools_unverifiable, agent_unavailable
tb egm brainstorm --brief rd.md [--from internal/x/] [--out notes/ideas.md]   # concept brief via one goldfish; not gated, written outside the tree by default (0600); exit 2: usage, brief_path, brief_secret, brief_injection, brainstorm_gated, brainstorm_exists, brainstorm_invalid, brainstorm_injection, agent_unavailable, tools_unverifiable
tb egm draft docs/specs/new.md --brief rd.md --from internal/x/ --body body.md   # new spec or ADR from an elephant-written body (no agent); never overwrites, never commits, no review state; exit 2: usage, draft_exists, draft_path, draft_sources, draft_injection, draft_invalid, brief_*
tb egm draft docs/specs/new.md --brief rd.md --from internal/x/ --engine llm      # a goldfish writes the body; also draft_not_run, tools_unverifiable, agent_unavailable
tb egm draft docs/specs/x.md --update --brief rd.md --engine llm   # proposal for an existing document under the state directory plus a diff; the document is never edited
tb egm review docs/specs/x.md --restart   # new cycle after a cap (needs an edit)
```

- **`brainstorm` and `draft` ([Phase C spec](specs/egm-phase-c.md), C1–C3):** both run only in the sandboxed goldfish runner (`egm.agent_cmd`, tools verifiably off) or, for `draft --body`, not at all; the generated text is cleaned (HTML comments, level-1 headings, early `Status:` lines), injection-scanned and written only if it passes. A brief (`--brief`, a regular file without symlinks, at most `egm.max_source_bytes`) with a secret-like value or injection-like text is refused before any agent runs. A concept brief lands in `$XDG_STATE_HOME/tb/egm/<repo-id>/briefs/` unless `--out` names a new file that is not an `egm.doc_globs` path; a draft may use one as `--brief` but never cites it. `draft` gives exactly one of `--body` and `--engine llm`, assembles the required sections (a missing one keeps `TODO`) and `## Sources` of the accepted `--from` files, and refuses a body that cites anything else or holds a secret-like value (`draft_secret`; `--update --engine llm` refuses a document with one too). The target must match `egm.doc_globs` or the repository's extra gated globs, for a new draft and for `--update` alike. It writes no state, round or receipt: the draft reaches `PASS` only through `tb egm review`. `--update` takes no `--from`, keeps the document's title, status and `## Sources`, and the proposal must resolve to the same R7 sources. Limits: `egm.brainstorm.max_bytes` (16 KiB) and `egm.draft.max_bytes` (32 KiB), lowered only by the repository config.

- **Flags:** `review` and `loop next` take `--engine auto|llm|deterministic`, `--reviewers a,b` and `--dry-run`; `review` takes `--restart`; `status` takes `--strict` and `--at <rev>`. Every command accepts `--json`.
- **Gate mode (`--at`):** the document, the sources (mode from `ls-tree`, bytes from `cat-file blob`; a source behind a versioned symlink is excluded) and the repository config come from the commit, never from the worktree. The repository is found from the current directory, and the document path is resolved lexically against it, so a missing or symlinked directory in the worktree changes nothing. A tracked source counts even when it matches `.gitignore`, in both modes. The `content` of a committed, unchanged worktree is the same in both modes. A source with a `.gitattributes` filter is not detected (`filtered_source` is a declared limitation).
- **Receipt (`receipt`):** writes `.toolbelt/egm/receipts/<slug>.json`, to commit with the document: the document and source hashes plus the claims, which are the lineages at `major` or above that the arbiter withdrew (and nothing reopened since), that were declined, or that were force-settled. Each claim carries `lines` in the current document; a scope without lines falls back to its heading's section. The bytes are deterministic (sorted keys, trailing newline). It is refused (exit 3, nothing written) unless the verdict is `PASS` or `PASS_PARTIAL`, refused with exit 2 `receipt_uncommitted` when the document or an included source differs from `HEAD` (commit them first; CI recomputes the hashes from the commit), and refused with exit 2 `receipt_path_untrusted` when a component of the path is a symlink or not a directory. The CI mode that checks receipts comes in a later slice ([egm trust](specs/egm-trust.md)).
- **Symbol citations (`path.go#Name`, `path.go#Recv.Name`):** a Go source may be cited with a symbol fragment, as a link or a backtick span; `L10`-style anchors stay file-level. Review checks each cited symbol on the snapshot's bytes with a pure parser (no graph, no LLM, also under `--engine deterministic`): a missing name is a `major` `refs` finding, an ambiguous one (`init`, `_`, or declared twice) a `major` `symbol_ambiguous`, and a file that does not parse one nit. The symbol's range starts at its declaration, doc comment excluded, and is hashed as raw bytes. Rounds record the hashes as `symbols`.
- **Receipt v3 (`egm.receipt.write_v3: true`, user-only):** v2 plus `symbols` (`"path#Name"` to sha256, possibly empty); readers accept 1, 2 and 3, and CI recomputes each symbol hash at head (a mismatch is `receipt_stale`; a key the document never cites is `receipt_invalid`).
- **Symbol drift (`drift-report`, `egm-source-drift/0.2`):** a source cited **only** through symbols is compared by those symbols: `symbol_changed` or `symbol_missing` (reason `deleted`, `ambiguous`, `unparsed`, `too_large`) exit 1, while a change elsewhere in the file is listed in `outside_symbols` and the document stays `fresh`. A path also cited plainly, and any v1/v2 receipt or round without `symbols`, stays file-scoped. Severity: `unreadable` > `source_missing` > `symbol_missing` > `drifted` > `symbol_changed` > `sources_differ` > `fresh` > `no_baseline`. `ci_source_drift` and the local `sources_changed` warning follow the same rule.
- **MCP:** `tb_egm_status` (`cwd`, `doc`, `at`, `strict`), `tb_egm_receipt` (`cwd`, `doc`) and `tb_egm_resolve` (`cwd`, `doc` and the resolve flags as `fix`, `decline`, `force_settle`, `reason`, `contest`, `evidence`). `cwd` must be absolute. `review` and `loop next` take minutes and stay CLI-only.
- **Auth (claude):** with `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN` listed in `egm.env.pass` and set, the key reaches the goldfish through the env and no credential file is checked or copied (the CI setup); note that a developer who lists `ANTHROPIC_API_KEY` in `egm.env.pass` and exports it locally is billed on that key instead of their claude login; otherwise the `claudeAiOauth` entry of `~/.claude/.credentials.json` is copied after the `expiresAt` precheck.
- **Config:** read from the user config (`~/.config/toolbelt/config.yaml`, `egm:` section) or from `-c`, never from the merged config. The repository's `.toolbelt/config.yaml` can only harden it: execution keys (`agent_cmd`, `env`, …) are ignored with `config_untrusted`, limits only go down, policy only goes up.
- **Git:** a closed list of hardened git calls (no hooks, filters, fsmonitor, pager or external diff); a `.git` with fatal keys (`include.*`, partial clone, alternates, …) stops with exit 2 `hostile_git_dir`.
- **State:** `$XDG_STATE_HOME/tb/egm/<repo-id>/<slug>/` (`state.json`, `round-<n>/target.md`, `report.{json,md}`), outside the repository and shared by its worktrees.
- **Exit 2** for usage errors and refusals: `state_exists`, `no_state`, `no_change`, `locked`, `invalid_transition`, `contest_refused`, `restart_needs_change`, `budget`, `owner_unavailable`, `auth_expired`, `forbidden_flag`, `env_forbidden`, `no_repository`, `hostile_git_dir`.

---

### `tb vault` — Markdown Vault (Phase A)

Notes of project knowledge that are not invariants (facts, decisions, context, logs), as Markdown files with optional YAML frontmatter and `[[wikilinks]]` in `vault.path` (default `.toolbelt/vault/`), editable in Obsidian or any editor ([spec](specs/vault.md), [ADR-016](adr/016-vault-fase-a.md)). The files are the only store: every command scans the vault from disk.

```bash
tb vault new "<title>" [--type T] [--tag t]... [--source S --confidence C] [--dir sub] [--body -|text]
tb vault search <query> [--limit N]
tb vault list [--type T] [--tag t] [--source manual|extracted|inferred]
tb vault show <note>          # normalized frontmatter + body; --json adds links and backlinks
tb vault links <note>         # outgoing wikilinks with state
tb vault backlinks <note>     # notes linking to <note>
tb vault check                # exit 1 on errors; warnings keep exit 0
```

- **Root and config:** the root is the parent of the `.toolbelt/` found from the current directory, and the config is read from `<root>/.toolbelt/config.yaml` (`vault.path`, `vault.max_note_bytes` default 256 KiB, `vault.types` default `fact, decision, context, log`). `vault.path` must resolve inside the repository (`vault_path_outside_repo`, exit 2); a missing vault is `vault_not_found` (exit 1).
- **Scan:** symlinks are never followed, directories starting with `.` (`.obsidian/`, `.trash/`) are skipped, FIFOs and other non-regular files are `not_regular` warnings and are never opened; reads use `O_NOFOLLOW|O_NONBLOCK` plus an `fstat` check.
- **Frontmatter:** `title`, `type`, `tags`, `aliases`, `source` (default `manual`), `confidence` (required with `source: inferred`), `created`, `updated`; unknown fields are kept and ignored.
- **Wikilinks:** `[[note]]`, `[[folder/note]]` (also a partial path, as in Obsidian), `[[note#heading]]`, `[[note|label]]`, `![[embed]]`. Matching is case-insensitive. Two notes with the same name are `ambiguous_link` (an error, where Obsidian picks the shortest path); aliases do not resolve links. A target with no matching note is an attachment when an attachment matches it or its extension is 1–8 letters/digits (`pic.png`, not `Dr. Smith`): found is `attachment`, missing a `missing_attachment` warning, several an `ambiguous_attachment` warning. Notes too large or unreadable still resolve as link targets.
- **`<note>`** in `show`/`links`/`backlinks` is a note path (with or without `.md`) or a link (`a`, `[[a]]`, `a#heading`, `[[a|label]]`).
- **Check:** errors `bad_frontmatter`, `missing_confidence`, `note_too_large`, `broken_link`, `ambiguous_link`; warnings `not_regular`, `unreadable`, `bad_tag`, `unused_confidence`, `broken_fragment`, `missing_attachment`, `ambiguous_attachment`. Text output quotes file names and titles with control characters.
- **Output:** with `--json`, note text (title, tags, aliases, body, link targets) sits inside `data` objects, and every output is redacted (GitHub, AWS, Slack, `Bearer`, PEM, `*_TOKEN=`, `sk-`, `password:`/`secret=`). Same notes, same bytes.
- **Stack:** `tb stack vault` runs `check` as a parallel, read-only op.
- **`new`:** the file name is the slug of the redacted title (up to 80 bytes) in `--dir`; it never overwrites (`note_exists`, exit 1), refuses a title with control characters or no letters (`bad_title`), and a `--dir` that leaves the vault or crosses a symlink (`dir_outside_vault`). The frontmatter is generated with a YAML encoder; title and body are redacted before writing (`redacted: true`).
- **`search`:** case- and accent-insensitive substring over title, aliases, tags and body, after redacting each note, so a query never probes a secret; one hit per note with the line and a snippet of up to 200 runes.
- **MCP:** `tb_vault_list`, `tb_vault_show`, `tb_vault_search`, `tb_vault_links`, `tb_vault_backlinks`, `tb_vault_check` (read-only) and `tb_vault_new`, which always writes `source: inferred` and requires `confidence`.
- **Graph:** with `TB_GRAPH=on`, `tb graph index` adds each note as `note:<path>` (title redacted, provenance from `source`/`confidence`), its headings as sections, resolved wikilinks as `links` edges and tags as `tag:<tag>` nodes; `tb graph explain note:<path>` and `tb graph path` work on them. The vault is never read by the generic Markdown indexer (which follows symlinks); `vault.graph: false` keeps notes out.

### `tb verify` — Adversarial Verification (✓ Phase 1/2)

Runs the generator→discriminator loop with pluggable rulers.

```bash
tb verify run [flags]

Flags:
  --ruler <list>           Comma-separated rulers (default: tests): tests, coverage, visual; visual cannot be combined; unknown name → exit 2
  --coverage-cmd <cmd>     Coverage command (verify.coverage_cmd; default with go.mod: go test -coverprofile)
  --coverage-pattern <re>  Regexp with one capture group; its last match in the coverage command's output is the percentage (verify.coverage_pattern; required with a non-Go coverage_cmd)
  --min-coverage <n>       Coverage floor in [0, 100]; only raises the base ref's .toolbelt/thresholds.yaml
  --test-cmd <cmd>         Visible tests command (default: verify.test_cmd; `tb init` detects go/npm/pnpm/yarn/pytest/cargo/make and also build_cmd/lint_cmd)
  --hidden-test-cmd <cmd>  Hidden tests command (verify.hidden_test_cmd; runs even without tests/hidden/; if the dir exists and the cmd is empty: detects the toolchain or marks N/A)
  --build-cmd <cmd>        Build command (default: verify.build_cmd; empty + explicit flag or --no-build to skip)
  --lint-cmd <cmd>         Lint command (default: verify.lint_cmd; empty + explicit flag or --no-lint to skip)
  --no-build               Skips build_cmd (config and default)
  --no-lint                Skips lint_cmd (config and default)
  --typecheck-cmd <cmd>    Typecheck command (default: verify.typecheck_cmd; gate step after build, before lint)
  --no-typecheck           Skips typecheck_cmd (config and flag)
  --check <name=cmd>       Adds or overrides a named check (repeatable; name [a-z0-9-]{1,32})
  --skip-check <name>      Skips one named check (repeatable; `--no-check` is a hidden deprecated alias until 2.0)
  --no-checks              Skips every named check (verify.checks)
  --dry-run                Shows the plan (cmds and directory) without executing
  --max-iter <n>           Maximum iterations (default: 3)
  --timeout <dur>          Timeout (default: 10m)
  --threshold <float>      Visual similarity threshold (default: 0.95)
  --manifest <path>        Screens manifest (default: .toolbelt/screens.yaml)
  --skip-unavailable       Graceful skip (SKIPPED verdict) if browser/refs/screens are unavailable (default: true)
  --base <ref>             Base ref
  --head <ref>             Head ref (build/lint/test run at this commit)
  --ticket <path>          Ticket with Scope (exception to custom `protected_paths` in guard.yaml)
  --output <path> / -o     JSON output file
  --json                   JSON output (global flag)
  --env <KEY>              (global, repeatable) allows an environment variable in the child (does not bypass secret blocking)
  --pass-env <KEY>         (global, repeatable) explicit opt-in to pass a secret to the child
```

When `--head` (or the `--base`/`--head` pair) resolves to a SHA **different** from the
current worktree `HEAD`, verify creates a temporary `git worktree add --detach`
at that SHA, runs `build_cmd` / `lint_cmd` / `test_cmd` / `hidden_test_cmd` there, and
removes the worktree when finished (also on error or signal). JSON includes
`head_sha` and `workdir` (the directory where the commands ran). If
`head == HEAD` and the worktree is dirty, it emits a warning — commands run in the
current worktree and may see uncommitted changes.

Verify child processes (`sh -c` of `test_cmd`/`build_cmd`/`lint_cmd`/`hidden_test_cmd`)
receive only an environment **allowlist** (PATH, HOME, LANG, TMPDIR, GOPATH/GOCACHE,
essential NODE/npm/pnpm, proxies, etc.). Names containing `TOKEN`, `SECRET`, `KEY`, or
`PASSWORD` **never** pass, unless `--pass-env KEY` (or `env.pass` in
`.toolbelt/config.yaml`). `--env KEY` extends the allowlist, but does **not** bypass
the secret block.

`tb init` fills `verify.test_cmd` from the repository toolchain
(`go.mod` → `go test ./...`, `package.json` + scripts.test → `npm`/`pnpm`/`yarn`/`bun test`
via lockfile `pnpm-lock.yaml`/`yarn.lock`/`package-lock.json`/`bun.lockb`,
`pyproject.toml`/`uv.lock` → `pytest`/`uv run pytest`, `Cargo.toml` → `cargo test`,
`Makefile` with `test`/`check` target → `make test`/`make check`). Without recognized
markers, `verify.test_cmd` stays **empty** and init emits a warning — it never
keeps `go test ./...` without `go.mod`. The chosen command binary is validated on
`PATH`: if `uv` is missing with `uv.lock`, it falls back to `pytest` or
`python -m pytest` / `python3 -m pytest` (and warns); missing Node managers
fall back to `npm test` when possible.

When `AGENTS.md`/`CLAUDE.md` exist, init writes `.toolbelt/agent_docs.yaml` with
`sources` and short-command `hints` (inline, shell fences, and prose like
“use/run/prefer …”). Long code fences (e.g. ` ```go `) are ignored so they
do not pollute the hints.

When there are clear markers, init also fills `verify.build_cmd`,
`verify.typecheck_cmd` and `verify.lint_cmd` conservatively (a tool is written only
when it resolves on `PATH` or through the package manager):

| Toolchain | `build_cmd` | `typecheck_cmd` | `lint_cmd` |
|-----------|-------------|-----------------|------------|
| `go.mod` | `go build ./...` | (none: the build type-checks) | `.golangci.y*ml`/`.toml` → `golangci-lint run` if the binary is on PATH, else `go vet ./...`; no config → `make lint` target |
| `package.json` | `build` script → `npm run …` / `pnpm run …` / `yarn …` | `typecheck`/`type-check` script → `<pm> run …`; else `tsconfig.json` + local `node_modules/.bin/tsc` → `npm exec -- tsc --noEmit` / `pnpm exec tsc --noEmit` / `yarn tsc --noEmit` / `bunx tsc --noEmit` | `lint` script → analogous |
| `Cargo.toml` | `cargo build` | (none: the build type-checks) | `cargo clippy` |
| Python (`pyproject`/`uv.lock`/…) | (empty) | mypy (`[tool.mypy]`, `mypy.ini`, `.mypy.ini`, `setup.cfg [mypy]`) or pyright (`[tool.pyright]`, `pyrightconfig.json`) configured and on PATH or in `.venv/bin` → `mypy .` / `pyright` (`uv run …` with `uv.lock`; `.venv/bin/mypy .` / `.venv/bin/pyright` when only in `.venv/bin`) | `[tool.ruff]`/`ruff.toml` → `ruff check .` (or `uv run …`); mypy is no longer a lint fallback |
| only `Makefile` | `build` target → `make build` | (none) | `lint` target → `make lint` |

Re-running `tb init` on a new config therefore writes a Node `typecheck` script as
`typecheck_cmd` (it used to be `build_cmd`) and no longer writes mypy as the Python
lint. An existing `build_cmd` that is a typecheck script keeps working as a build
until you move it.

Named checks are suggested, never added silently: `govulncheck ./...` (Go, when
`govulncheck` is on PATH) and `cargo clippy --all-targets -- -D warnings` (Rust, when
`cargo clippy -V` works). `tb init --write-checks` writes them into
`verify.checks` of a new `config.yaml`; otherwise init logs each suggestion and,
with `--json`, lists them in `suggested_checks`.

Missing script/config → field omitted. Without a detected `test_cmd`, verify
does not invent `go test` (configure by hand or add a script/marker).

The hidden suite remains hidden from the agent (paths under `tests/hidden/` protected
by `tb guard`). The command configured in `verify.hidden_test_cmd` or
`--hidden-test-cmd` **always runs**, even without the `tests/hidden/` directory
(typical in JS/Python/Rust monorepos). If the command is empty and the
directory exists, `go test ./tests/hidden/...` is used. With no command and no
directory, the hidden suite is ignored (`hidden_available: false`,
`hidden_passed: null`). Optional `build_cmd` and `lint_cmd` fields run before
tests (in that order) and make verify fail if the command exits with an error;
absent = previous behavior (only `test_cmd` + hidden).

If the `test_cmd` / `hidden_test_cmd` / `build_cmd` / `lint_cmd` binary does not
exist (exit 127), is not executable (exit 126), or `exec` fails with
`ErrNotFound`, verify **fails immediately** without retries, with a message like
`command not found: X — adjust verify.test_cmd`. The `tb` process exits with
code **127**. With `--json` (or `--output`), it emits the normal `verify.Result`
envelope including `error` (and `verdict: FAIL`), instead of exiting without JSON. Genuine
command failures (exit ≠ 0/126/127) continue in the `--max-iter` loop.

`ESCALATE` and `FAIL` verdicts exit with **exit ≠ 0** also with `--json`
(same as text mode). Only `PASS` and `SKIPPED` (visual ruler) use exit 0.
A missing command uses exit **127** (see the exit-code table).

Failure output is meant for agents: child commands get `NO_COLOR=1 FORCE_COLOR=0`
(unless either is already set), ANSI escapes are stripped from `reason`, and a long
output keeps the failure lines (`FAIL`, `Error:`, `✗`, `panic:` …) first, then
the tail where runners print their summary. When two iterations in a row fail
with the same named failure lines (durations, addresses and ports ignored),
verify stops with `ESCALATE` and `reason: deterministic failure …` instead of
spending the remaining `--max-iter`; a silent failure (no recognizable line) is
still retried. On a non-PASS verdict the JSON carries `filtered_env`: the
**names** (never values, and never secret-looking names) of the variables the allowlist removed, so a failure
that only happens inside `tb verify` points at the missing `env.allow` entry.

In a repository that holds a single commit, the implicit `--base HEAD~1` falls back
to `HEAD` (empty diff, a warning on stderr) instead of failing; build, lint and
tests still run. Shallow clones, an orphan branch next to other history, and an
explicit `--base` that does not resolve still fail (an empty diff there would hide
edits to protected paths).

**Guard inside verify (anti-cheat vs mandate):**

Verify always runs `tb guard` on the diff and includes the result in `guard`. The decision
to **abort before build/test** depends on the origin of the `protected_path`:

| Origin | Without `--ticket` | With `--ticket` and path in Scope | With `--ticket` and path outside Scope |
|--------|--------------------|-----------------------------------|----------------------------------------|
| Built-in defaults (`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`, `.toolbelt/guard.yaml`) | **aborts** (FAIL, 0 iterations) | **aborts** | **aborts** |
| `protected_paths` from `.toolbelt/guard.yaml` on the **base** (custom) | reports as **warning** and **continues** build/test | reports warning (Scope exception) and **continues** | **aborts** |

`tb guard check` treats custom violations as
`SeverityError` and fails the exit code by default; use `--warn-only` to opt out.
Only the `tb verify run` loop applies the exception above so it does not kill the mandate
when the manifest lists the task's own file.

```yaml
# .toolbelt/config.yaml
verify:
  build_cmd: "pnpm --filter @pkg build"
  typecheck_cmd: "pnpm exec tsc --noEmit"
  lint_cmd: "pnpm --filter @pkg lint"
  test_cmd: "pnpm --filter @pkg test"
  coverage_cmd: "pnpm exec vitest run --coverage"
  coverage_pattern: 'All files\s*\|\s*[\d.]+\s*\|\s*[\d.]+\s*\|\s*[\d.]+\s*\|\s*([\d.]+)'
  hidden_test_cmd: "pnpm --filter @pkg test:hidden"
  checks:
    - name: audit              # [a-z0-9-]{1,32}, unique
      cmd: "pnpm audit --prod"
```

**Steps of the `tests` ruler.** Each iteration runs, in order: build → typecheck →
lint → named checks → visible tests → hidden tests. Build, typecheck, lint and each
check are **gate steps**: they run once per verify run (cached across iterations),
and a failure ends verify with `ESCALATE` (the failure is deterministic for the
agent's loop), with the excerpt in `reason`. Checks run in the order of
`verify.checks`; exit 0 passes, anything else fails the check; a missing executable
fails fast with exit 127. `--check name=cmd` adds or overrides one check,
`--skip-check name` skips one and `--no-checks` skips all (`--no-check`, the old spelling of `--skip-check`, still works but is hidden and prints a deprecation warning; it goes away in 2.0). A scan that needs the
network (e.g. `govulncheck`) belongs in CI, or is skipped locally with `--skip-check`.

New JSON keys, all `omitempty` (a run with today's config and flags prints the same
keys as before): `typecheck_ran`, `typecheck_passed`, `checks`
(`[{name, passed, reason}]`), `config_source` (`none`, only without a base file), `warnings`,
`overrides` and `skip_kind`. `overrides` lists, sorted, every verify key a flag or MCP
argument set to a value other than the trusted config's (base/global/`-c`), e.g.
`test_cmd`, and every skip toggle that disabled a configured step (`no_typecheck`,
`skip_checks:govulncheck`, `no_checks`; an added or replaced check is `checks:<name>`).
Text mode (also `tb stack`) prints the warnings and `Warning: overrides applied: …` on
stderr.

**Verify's commands come from the base ref.** The `verify:` section is read from
`git show <base>:.toolbelt/config.yaml`, merged over the global config and the
defaults (with `-c`, only that file; see below), never from the working tree, so the change under test cannot
rewrite its own gate (e.g. `test_cmd: true`). Flags and MCP arguments still
override, since they come from the caller. Guard compares the `verify:` section at
base and head: a difference is an error finding `verify_config_changed` that aborts
verify like a protected-path hit, listing the changed keys; other sections of the
file stay unrestricted. A symlinked config at base or head is compared by its target
text: any change is a `verify_config_changed` finding (`verify`; it cannot be parsed).
A malformed file at base is the warning finding `verify_config_unreadable` (guard and
`-c` runs keep working; verify without `-c` aborts on it). `--base` is chosen by the caller, so **CI must pin the base**
(the PR's merge base); verify warns (`base resolves to head`) when the base SHA equals
the resolved head SHA.

Config layers for verify (R0): with `-c`, only the `-c` file and the defaults (the
repository config is not read, as before). Without `-c`: defaults < global config <
`.toolbelt/config.yaml` as committed at the base ref. The working-tree file is never
read, for `verify:` and for `env:` (`allow`/`pass`), so a change cannot pass secrets
to its own tests. When the working tree's `verify:`/`env:` keys differ from the base
file's, verify warns, e.g. `ignored .toolbelt/config.yaml keys that differ from the base ref:
verify.test_cmd (verify reads the base ref)` (the file at the repository top level, also
when run from a subdirectory). `config_source` is omitted
in the normal case and is `"none"` when the base ref has no file (global config and
defaults only) or commits it as a symlink, which is not followed (warning
`symlinked .toolbelt/config.yaml at the base ref is not followed; using the global config`).
A malformed file at the base ref aborts verify without `-c`. A malformed global or `-c` config aborts verify.

```bash
tb verify run --test-cmd "pnpm test" --hidden-test-cmd "pnpm test:hidden"
# no tests/hidden/: the hidden_test_cmd above still runs
tb verify run --test-cmd "pnpm test" --no-build --no-lint
tb verify run --build-cmd "pnpm build" --lint-cmd "pnpm lint" --test-cmd "pnpm test"
tb verify run --dry-run --test-cmd "go test ./..."
tb verify run --typecheck-cmd "pnpm exec tsc --noEmit" --test-cmd "pnpm test"
tb verify run --check "vuln=govulncheck ./..." --skip-check clippy
tb verify run --no-typecheck --no-checks
```

Ruler `visual` ([docs/verificacao-visual.md](verificacao-visual.md)): headless capture + pixel diff;
if Chromium/Chrome or references are missing, it gracefully skips.

**Examples:**

```bash
# Verify with the tests ruler
tb verify run
tb verify run --test-cmd "go test ./..." --hidden-test-cmd "go test ./tests/hidden/..."
tb verify run --test-cmd "true" --no-build --no-lint --max-iter 1
tb verify run --dry-run --build-cmd "go build ./..." --test-cmd "go test ./..."

# Coverage ruler (threshold from the base ref's .toolbelt/thresholds.yaml)
tb verify run --ruler coverage
tb verify run --ruler tests,coverage --min-coverage 80
tb verify run --ruler coverage --coverage-cmd "pytest --cov" --coverage-pattern 'TOTAL.*\s(\d+(?:\.\d+)?)%'

# Visual verification (threshold is a 0–1 fraction, not an integer percentage)
tb verify run --ruler visual --threshold 0.95
tb verify run --ruler visual --manifest .toolbelt/screens.yaml --json -o verify-result.json

# Graceful skip (default): browser/refs missing, screens: [] or all screens skipped
# → top-level report skipped=true and verdict SKIPPED (not PASS)
tb verify run --ruler visual --skip-unavailable
```

**Available rulers:**

| Ruler | Metric | Default Threshold | Specific Cheating |
|-------|--------|-------------------|-------------------|
| `visual` | Pixel similarity | 0.95 | Reference edits |
| `tests` | Gate steps (build → typecheck → lint → named checks) pass, then visible and hidden tests pass | 100% | Test modification; rewriting `verify.*` commands (blocked: read from base, `verify_config_changed`) |
| `coverage` | Coverage percentage (statements for Go, whatever the tool reports otherwise) | `coverage.min_percent` in base `.toolbelt/thresholds.yaml` | Lowering the threshold (read from base) |

`--ruler` takes a comma-separated list. An unknown name fails before anything runs
with exit **2** (`invalid ruler "x": …`); it used to run `tests` silently. `visual`
cannot be combined with another ruler yet. `--ruler tests,coverage` runs both with
one guard, one ref resolution and one head worktree; the verdict is the worst of the two.

**The `coverage` ruler.** It runs the coverage command in the head worktree and reads
one total percentage:

- **Go** (default when `go.mod` exists and `coverage_cmd` is empty): `go test -coverprofile=<tmp> ./...`;
  the total is statement-weighted, with blocks deduplicated (a block repeated by
  `-coverpkg` counts as covered if any copy ran), like `go tool cover -func`.
- **Generic:** `verify.coverage_cmd` plus `verify.coverage_pattern`, a regexp with exactly
  one capture group; the **last** match in the output is the percentage and must be a
  number in [0, 100]. A `coverage_cmd` without `coverage_pattern` fails with its own
  reason (`coverage: coverage_cmd set but no coverage_pattern to read its output`); an
  output with no match fails with `coverage: pattern did not match`.
- **Threshold:** `coverage.min_percent` in `.toolbelt/thresholds.yaml` read **from the
  base ref only** (never the working tree), so a change cannot lower its own floor.
  Schema: `version: 1`, `coverage: {min_percent: <0–100>}`; unknown keys, a bad value, a
  malformed file or a symlink abort verify instead of meaning "no threshold".
  `--min-coverage` (MCP `min_coverage`) can only **raise** it: the effective floor is
  `max(base, flag)`, reported as `threshold_source: "base"` or `"flag"`.
- **Trust:** the coverage command runs as the same user as the code under test, so a
  test can tamper with its own coverage output; the threshold is a floor, not proof. Prefer
  an anchored, generic pattern (e.g. `(?m)^TOTAL\s.*\s(\d+)%$`) over a loose one. For Go,
  the profile is accepted only for files of the module's packages (`go list ./...`) that
  exist in the head worktree (`coverage: profile names unknown file <key>`); negative
  counts and inconsistent statement counts fail. The Go default is decided by `go.mod`
  at the **base** commit.
- **Verdicts:** no threshold anywhere → `SKIPPED` / `not_applicable` (it does not demote
  a combined `PASS`); a threshold but no coverage command for the repository → `FAIL`
  (`coverage: threshold set but no coverage_cmd for this repository`); below the
  threshold → `FAIL` (not `ESCALATE`: the agent can add tests); otherwise `PASS`.
- **JSON:** `coverage: {percent, threshold, threshold_source, cmd, metric}` and
  `metrics.coverage_percent`. With several rulers the output is
  `{verdict, rulers: {tests: …, coverage: …}}`. Flag or MCP overrides of
  `coverage_cmd`, `coverage_pattern` or `min_coverage` appear in `overrides`.

> Ruler `review`, flag `--discriminator-only`, and invocations like `tb verify <task-id>` **do not exist** — use `tb verify run` and `tb review` separately.

**Discriminator briefing template:**

```yaml
# .toolbelt/discriminator-brief.yaml
task: ui-implementation
ruler: visual

context:
  description: "Login screen implementation per the design"
  design_reference: "references/login-v2.png"
  spec_link: "vault/specs/login-redesign.md"

screens:
  - name: login
    modes: [light, dark]
    reference_light: "references/login-light.png"
    reference_dark: "references/login-dark.png"
    masks:
      - region: [100, 200, 50, 20]
        reason: "Dynamic timestamp"

acceptance:
  min_similarity: 95
  wcag_level: AA
  
cheating_rules:
  - no_reference_edits
  - no_mask_without_reason
  - no_threshold_changes
```

**Report format:**

```yaml
# verify-report.yaml
task: ui-implementation
ruler: visual
verdict: PASS | FAIL
iterations: 2

per_screen:
  - screen: login
    light_similarity: 97.2
    dark_similarity: 95.8
    size_match: true
    wcag_pass: true
    
  - screen: dashboard
    light_similarity: 96.5
    dark_similarity: 95.1
    size_match: true
    wcag_pass: false
    wcag_violations:
      - element: ".btn-secondary"
        contrast: 3.8
        required: 4.5

gaps:
  - id: G1
    file: "src/components/Button.tsx"
    line: 45
    issue: "Insufficient contrast in dark mode"
    proposed_fix: "Use --text-on-dark instead of --text-muted"

accepted_deviations:
  - reason: "Roboto font not available, using system font"
    decision_id: "vault/decisions/font-fallback.md"

cheating_detected: []

metrics:
  total_time: "2m 15s"
  build_time: "45s"
  capture_time: "30s"
  comparison_time: "5s"
```

---

### `tb guard check` — Protected Path Check (✓ implemented, stable)

Checks changes to protected paths between two commits.

```bash
tb guard check [base..head] [flags]

Flags:
  --base <ref>          Base ref (default: HEAD~1; ignored if base..head is passed)
  --head <ref>          Head ref (default: HEAD; ignored if base..head is passed)
  --warn-only           Do not fail the exit code on SeverityError (opt-out of the CI-safe default)
  --json                JSON output
```

Accepts the positional range `base..head` (classic form) or the `--base`/`--head` flags, aligned with `tb review` / `tb drift` / `tb verify`.

By default, exit ≠ 0 when there is `SeverityError` (CI-safe); `--warn-only` opts out.
`--strict` was removed in 1.0 (it had been a no-op since it was deprecated); so was
the `strict` argument of `tb_guard_check`. A ref starting with `-` or an unknown SHA
fails with a clear error (the ref is never passed to `git` as an option). In text mode
with no violations it prints `guard: ok (N rules, 0 violations)` (`N` = valid
protected paths loaded: defaults + `.toolbelt/guard.yaml` from the **base**; invalid
patterns are ignored and do not count toward `rules_checked`).

**Examples:**

```bash
# Check changes between main and the current branch (exit ≠ 0 on SeverityError)
tb guard check main..HEAD

# Same range via flags (like review/drift)
tb guard check --base main --head HEAD

# Defaults to HEAD~1..HEAD when no range is given
tb guard check --json

# Opt-out: lists violations but exits 0
tb guard check origin/main..HEAD --warn-only

# JSON + exit ≠ 0 on SeverityError (default)
tb guard check main..HEAD --json
```

**Protected paths manifesto (`.toolbelt/guard.yaml`):**

The guard reads `protected_paths` from `.toolbelt/guard.yaml` **on the base ref**
(`git show base:.toolbelt/guard.yaml`) and **merges** them with the built-in defaults
(`tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`,
`.toolbelt/guard.yaml`). Any change to `.toolbelt/guard.yaml` in the diff
is a violation. Malformed YAML on the base fails **closed** (exit ≠ 0, clear error).
Path matching is **case-insensitive** and honors globs (`*`, `?`, `**`).
Renames (`R*`) check source and destination. The anti-cheat scan uses
diff content / `git show head:path` (not the working tree) and covers Go and
JS/TS (`it.skip` / `describe.skip` / `test.skip` / `xit` / `.only` /
trivial assertions such as `expect(true).toBe(true)`). Each YAML entry may be
a string (`lib/help.js`) or an object with `pattern`. A trailing slash (`secrets/`)
protects the directory and children. Without the file on the base, only the defaults apply.
Violations get `source: default` or `source: custom`. In `tb verify run`,
only `default` (and custom outside the `--ticket` Scope) abort the loop; custom in the
mandate is reported and verify continues — see the verify section above.

```yaml
# .toolbelt/guard.yaml
version: 1

protected_paths:
  # Whole directory (trailing slash = prefix; protects children)
  - secrets/
  # One-level glob
  - pattern: "secrets/*"
    reason: "Secrets cannot be edited"
  # Literal path
  - lib/help.js

  # Visual references
  - pattern: "references/**"
    reason: "Visual references cannot be edited without a recorded decision"
    allow_additions: true    # new files OK
    allow_deletions: false   # deletions require a decision
    allow_modifications: false
    
  # Masks
  - pattern: "masks/**"
    reason: "Masks require a dynamic-region justification"
    require_reason_in_commit: true
    
  # Comparator
  - pattern: "**/comparator.{js,ts,go,py}"
    reason: "Comparison logic is immutable"
    allow_modifications: false
    
  - pattern: ".toolbelt/thresholds.yaml"
    reason: "Thresholds cannot be relaxed without approval"
    detect_relaxation: true  # specifically detects decreasing values
    
  # Seeds
  - pattern: "**/*.seed"
    reason: "Deterministic seeds are fixed per scenario"
    
  # Benchmark tests
  - pattern: "benchmarks/**"
    reason: "Benchmark tests are read-only"
    allow_modifications: false
    
  # Hidden tests
  - pattern: "tests/hidden/**"
    reason: "Hidden tests cannot be accessed by the implementer"
    visibility: discriminator_only

decision_sources:
  - "vault/decisions/**"
  - ".toolbelt/approved-changes.yaml"

on_violation:
  ci: fail
  local: warn
```

**Approved decision format:**

```yaml
# .toolbelt/approved-changes.yaml
approved_changes:
  - path: "references/login-light.png"
    action: delete
    reason: "Design updated to v3, old ref obsolete"
    approved_by: "owner"
    approved_at: "2026-01-15T10:30:00Z"
    ticket: "DESIGN-123"
    
  - path: ".toolbelt/thresholds.yaml"
    action: modify
    field: "min_similarity"
    old_value: 95
    new_value: 93
    reason: "System font varies more than expected"
    approved_by: "owner"
    approved_at: "2026-01-15T11:00:00Z"
    temporary: true
    expires_at: "2026-02-15"
```

---

### `tb ticket` — Ticket Management (✓ Phase 3)

Creates and manages the work tickets consumed by the router. Each ticket is a
versionable YAML file at `.toolbelt/tickets/TKT-NNN.yaml` (monotonic IDs, 3+
digits). Writes are atomic (temp + rename) and serialized by
`.toolbelt/tickets/.lock`. Every subcommand honors `--json`.

```bash
tb ticket create --title "Create User model" --scope "src/models/**" --accept "go test ./src/models passes" --timeout 10m
tb ticket create --title "Implementar JWT signing" --dep TKT-001 --priority 1
tb ticket create --from rd.md
tb ticket create --title "Refactor payments" --dry-run
tb ticket list
tb ticket list --status pending,running
tb ticket list --deps TKT-001
tb ticket show TKT-002
tb ticket cancel TKT-002
tb ticket retry TKT-002
```

**Subcommands and flags:**

| Subcommand | Flags | Description |
|------------|-------|-------------|
| `create` | `--title`, `--description`, `--from <rd.md>`, `--scope`, `--forbid`, `--accept`, `--dep`, `--priority <n>`, `--timeout <dur>`, `--dry-run` | Creates a ticket (`--title` or `--from`). `--scope`/`--forbid`/`--dep` take a list or repetition; `--accept` is repeatable |
| `list` | `--status a,b`, `--deps <id>` | Lists tickets; `--deps` shows only those depending on the ID |
| `show <id>` | — | Details: status, priority, deps, mandate, acceptance, result, merge |
| `cancel <id>` | — | Cancels a `pending`/`running` ticket |
| `retry <id>` | — | Moves a `failed`/`canceled` ticket back to `pending` |

`--from` reads an `rd.md` / ticket file (the same format used by `tb review`
and `tb drift`: `## Scope`, `## Forbidden` and an acceptance checklist). `tb`
does not decompose free-form tasks into tickets and does not talk to GitHub —
use `--title` or an `rd.md` you wrote. Unknown dependency IDs and cycles are
rejected at creation time.

**Output examples:**

```bash
tb ticket create --title "Create User model"
# ticket TKT-001 created: Create User model
tb ticket list
# TKT-001	pending  	p0	Create User model
# TKT-002	pending  	p1	JWT signing	(deps: TKT-001)
```

**Statuses and transitions:**

| From | To |
|------|----|
| `pending` | `running`, `canceled` |
| `running` | `done`, `failed`, `pending` (retry), `canceled` |
| `failed` / `canceled` | `pending` (`tb ticket retry`) |
| `done` | terminal |

**Ticket format:**

```yaml
# .toolbelt/tickets/TKT-001.yaml
id: TKT-001
title: Create User model
status: done            # pending | running | done | failed | canceled
priority: 0             # higher first
mandate:
    scope: [src/models/**]
    forbidden: [tests/hidden/**]
    permissions: [read, write, run_tests]
acceptance:
    criteria: ["go test ./src/models passes"]
    timeout: 10m
dependencies: []
attempts: 1
worker: w1
result:                 # filled in by the router
    branch: tb-workspace/TKT-001
    base_commit: 356c934e...
    head_commit: a3e62ba7...
    workspace: .toolbelt/workspaces/TKT-001
    exit_code: 0
    log: .toolbelt/router/logs/TKT-001.log
```

A ticket may also carry a `subscription` (worker, `direct_delivery`,
`expires_at`) — see direct delivery under `tb router`. `merged_at` is written
by `tb merge`.

---

### `tb board` — Board View (✓ Phase 3)

Shows tickets in sections and controls whether new tickets get assigned. The
control state (`paused`/`draining`) lives in `.toolbelt/board.yaml`, created on
the first `pause`/`drain`.

```bash
tb board list
tb board list --status pending,failed
tb board list --worker w1 --strategy priority
tb board watch --interval 2s
tb board pause
tb board resume
tb board drain
```

**Subcommands and flags:**

| Subcommand | Flags | Description |
|------------|-------|-------------|
| `list` | `--status a,b`, `--worker <id>`, `--strategy fifo\|priority\|dependency` | Sections RUNNING / READY / BLOCKED / DONE / FAILED / CANCELED |
| `watch` | `--interval <dur>` (default 2s), `--status`, `--worker`, `--strategy` | Reprints the board until Ctrl+C |
| `pause` | — | No new assignments; running work continues |
| `resume` | — | Clears `pause` and `drain` |
| `drain` | — | Finishes what is running and assigns nothing new |

Section ordering (`--strategy`, default `dependency`): `fifo` (creation order),
`priority` (priority desc., then fifo), `dependency` (topological depth asc.,
then priority). READY = `pending` with all dependencies `done` and the board
not paused; BLOCKED lists what is being waited on.

**Example:**

```bash
tb board list
# board: active (strategy: dependency)
#
# RUNNING (0)
#
# READY (1)
#   TKT-001	pending  	p0	Create User model
#
# BLOCKED (1)
#   TKT-002	JWT signing	(waiting: TKT-001: pending)
#
# DONE (0)
# FAILED (0)
# CANCELED (0)
```

---

### `tb router` — Router and Workers (✓ Phase 3)

The router is a **foreground scheduler** (not a daemon: there is no
`start`/`stop`). `tb router run` distributes ready board tickets across N
workers (`w1..wN`); each ticket runs in its own worktree
(`.toolbelt/workspaces/<ticket>`, branch `tb-workspace/<ticket>`), executing
the agent command **inside the sandbox**. The watchdog runs inside the same
loop.

```bash
tb router run --agent-cmd "my-agent --task-file $TB_TICKET_FILE"
tb router run --workers 2 --strategy priority --once
tb router run --dry-run
tb router run --allow-unsandboxed
tb router status
tb router config
```

**Subcommands and flags:**

| Subcommand | Flags | Description |
|------------|-------|-------------|
| `run` | `--workers <n>`, `--strategy fifo\|priority\|dependency`, `--agent-cmd "<cmd>"`, `--once`, `--dry-run`, `--check`, `--allow-unsandboxed` | Runs until the queue is empty; Ctrl-C returns `running` tickets to `pending`. With `router.bus` (global config or `-c` only) it adds bus slots `b1..bN` that hand tickets to `router_tasks` workers on a bus channel; `max_workers: 0` = bus only (no `agent_cmd`, no Landlock). `--check` validates the bus session and channel. Spec: [bus-router.md](bus-router.md) |
| `status` | — | Reads `state.json`; detects stale state (dead PID) |
| `config` | — | Prints the effective configuration (`router:`) |

- `--dry-run` prints the per-round plan (worker, ticket, `board`/`direct` origin, base) without executing.
- `--once` does one scheduling round and exits when the started tickets finish.
- Without Landlock the sandbox is **fail-closed**: the router refuses to run. `--allow-unsandboxed` is an explicit opt-in (see `tb sandbox`).
- The child environment is filtered (allowlist; secrets only with `--pass-env`) and receives `TB_TICKET_ID`, `TB_TICKET_FILE` (absolute path), `TB_WORKER_ID` and `TB_WORKSPACE`.
- Per-ticket timeout = the ticket's `acceptance.timeout` or `router.timeouts.per_ticket`.
- When the agent exits with changes, the router runs `git add -A` and commits `tb: <id> <title>` in the worktree, and records `base_commit`/`head_commit` on the ticket. Exit 0 → `done`; otherwise `failed`, or back to `pending` while `attempts < router.retry.max_attempts`.
- **Single instance:** only one `tb router run` per repository, enforced by the exclusive lock `.toolbelt/router/lock` (`flock`; on non-unix systems an `O_EXCL` lockfile — if one is left behind after a crash, remove it manually).
- **Worktree integrity:** before committing, the router verifies that `<ws>/.git` is the expected worktree pointer file and runs git with hardened configuration (fsmonitor and hooks disabled, `--no-verify`, the user's global/system config turned off — `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` — so user filters such as git-lfs do **not** run on the agent's content; `user.name`/`user.email` are read once from the root with the normal config and passed via `-c`, falling back to `tb <tb@localhost>`). The router's own `git worktree add/remove` also runs without hooks (`core.hooksPath=/dev/null`), fsmonitor and detached maintenance: worktrees created by the router never execute the user's `post-checkout` hooks. A violation ⇒ ticket `failed` without retry (`worktree integrity violated`).
- **Nested `.git`, gitlinks and `.gitmodules`:** the router's git runs outside the sandbox, so a repository/submodule created by the agent (e.g. `sub/.git` with `filter.*.clean` inside a gitlink of the base commit) is never touched: before any git call the router walks the worktree (without following symlinks) and refuses any `.git` entry other than the root pointer; after `git add -A` it refuses commits that add/modify gitlinks (mode 160000) or `.gitmodules`. Either way the ticket becomes `failed` without retry (`worktree integrity violated`). The router's git also runs with `--ignore-submodules=all` and submodules disabled (`diff.ignoreSubmodules`, `submodule.recurse`, `fetch.recurseSubmodules`).
- **Detached processes (Linux):** the router becomes a *subreaper* (`PR_SET_CHILD_SUBREAPER`) and tracks the pids it starts itself (agents and git). After each agent and on every watchdog tick, every untracked direct child — an agent descendant that called `setsid`/`setpgid` and escaped the process group — is killed (`SIGKILL`) and reaped. Policy: agents **must not detach**; any escaped process found is killed. If that happens during a ticket, the `agent_escaped` alert (error) is emitted and the ticket fails **without retry and without running git on the worktree**. Attribution uses the escaped process's own `TB_TICKET_ID` (only that variable is read from `/proc/<pid>/environ`): only the owning ticket fails; the others carry on. If `TB_TICKET_ID` is missing (`env -u`), unreadable or unknown, the escape is **unattributable** and, conservatively, **every** ticket whose agent was running at detection time fails (an `agent_escaped` alert with no ticket, listing them). An orphan found inside an agent's process group whose `TB_TICKET_ID` differs from the group's owner (`setpgid` into another agent's pgid to hide) is also an escape. Daemons born during worktree creation (the `git worktree add` window) without `TB_TICKET_ID` are killed with an `orphan_killed` alert (warn) and no ticket fails. Limitation: an agent that forges another active ticket's `TB_TICKET_ID` gets the escape attributed to that ticket. Outside Linux (macOS/Windows) there is no subreaper: only the process group applies (a descendant that detaches is neither detected nor killed — a **limitation**). `tb sandbox run` is unchanged.
- **The mandate is not enforced:** the ticket mandate's `Scope`, `Forbidden` and `Permissions` are **recorded** but not enforced by the router — the agent's edits are committed as-is. Run `tb drift`, `tb review` and `tb guard` (or `tb stack --ticket`) on the result **before** `tb merge`.
- **Stale control:** `control/kill-*.json` requests are deleted when `tb router run` starts, and requests whose `requested_at` predates the run start are ignored.
- **Unreadable board:** a transiently invalid ticket YAML does not take the router down: it emits `board_unreadable` (warn), keeps the workers and retries on the next tick; after 30 consecutive failing ticks the router stops with `stalled` (non-zero exit).
- **Cancel then retry:** a ticket is only assigned again after the slot still holding it finishes killing the previous agent.
- **Agent output:** exit 0 that leaves background child processes counts as success; the process group is killed after the exit. Kept/logged output is capped to the last 1 MiB; logs use mode 0600. Exit 0 wins over a late stuck/kill verdict.
- **`stalled` summary:** if `pending` tickets remain blocked by `failed`/`canceled` dependencies (the `unschedulable` list), the summary reason is `stalled` and `tb router run` exits non-zero.

**Direct delivery:** when a ticket finishes `done`, each `pending` ticket that
depends on it receives a *subscription* to that worker (TTL
`subscription_ttl`, at most `max_subscriptions_per_worker` per worker). A
worker prefers tickets subscribed to it; if the subscribed ticket has exactly
one dependency with a `head_commit`, the worktree starts from that commit
(context affinity). A ticket with a single dependency always starts from its
`head_commit`, with or without direct delivery; with ≥ 2 dependencies it starts
from `HEAD` (`multi_dep_base` alert if any is unmerged). Expired subscriptions are ignored, and if the worker fails
its subscriptions are cleared and the ticket returns to the board.

**Example:**

```bash
tb router run --agent-cmd "sh -c 'echo x > f-$TB_TICKET_ID.txt'"
# router started: 3 workers, strategy dependency
# [w1] TKT-001 started (board, base 356c934e, attempt 1)
# [w1] TKT-001 done (a3e62ba7)
# [w1] TKT-002 started (direct, base a3e62ba7, attempt 1)
# [w1] TKT-002 done (a6ebf42e)
# router stopped (completed)
```

**Configuration** (`.toolbelt/config.yaml`; there is no `router.yaml`):

```yaml
router:
  max_workers: 3
  strategy: dependency        # fifo | priority | dependency
  agent_cmd: []               # agent argv (required, or --agent-cmd)
  direct_delivery:
    enabled: true
    subscription_ttl: 30m
    max_subscriptions_per_worker: 5
  timeouts:
    per_ticket: 10m
    global: 1h
  retry:
    max_attempts: 2
  slow_threshold: 0.8         # timeout fraction for worker_slow
  # bus:                      # bus slots b1..bN; ONLY from the global config or -c (ignored in the repository's)
  #   channel: feature-x
  #   as: router              # the router's CLI session (tb bus join --role router)
  #   slots: 1
  #   to: role:worker         # or agent:<name>
  #   claim_timeout: 5m
```

**Runtime files** (`.toolbelt/router/`, gitignored):

| Path | Content |
|------|---------|
| `state.json` | PID, strategy, workers (idle/working, ticket, subscriptions; bus slots also thread, agent, phase and `claimed_at`) and queue (ready/blocked/running); updated on every change |
| `alerts.jsonl` | Watchdog alerts, one JSON per line (created on the first alert) |
| `control/` | Requests to the running router (e.g. `kill-w1.json`) |
| `logs/TKT-NNN.log` | Agent stdout/stderr per ticket |

---

### `tb watchdog` — Worker Monitoring (✓ Phase 3)

The watchdog **runs inside the router loop** (1s tick). The `tb watchdog`
command only **reads** the persisted state (`.toolbelt/router/`) and sends
control requests; there is no `scale` (the router does not scale itself).

```bash
tb watchdog status
tb watchdog status --worker w1
tb watchdog alerts --since 10m --kind worker_stuck --limit 20
tb watchdog kill w1 --reason "stuck in a loop"
```

| Subcommand | Flags | Description |
|------------|-------|-------------|
| `status` | `--worker <id>` | Worker state and alert counts per kind |
| `alerts` | `--since <10m\|2h\|RFC3339>`, `--kind <kind>`, `--limit <n>` | Lists `alerts.jsonl` |
| `kill <worker>` | `--reason` | Writes `control/kill-<worker>.json`; the router cancels the running ticket (counts as an attempt). Errors if no router is running. For a bus slot (`bN`) the router revokes the thread; the remote harness stops its own child |

**Alerts:**

| Alert | Severity | Condition | Action |
|-------|----------|-----------|--------|
| `worker_slow` | warn | elapsed ≥ `slow_threshold` × timeout (once) | `none` |
| `worker_stuck` | critical | elapsed ≥ ticket timeout; the worker is canceled | `retry` (attempts left) or `escalate` |
| `worker_crash` | error (critical on escalate) | agent exits with code ≠ 0 | `retry` or `escalate` |
| `worker_killed` | warn | `tb watchdog kill` | `retry` or `escalate` |
| `queue_growing` | warn | ready > 2 × workers (once per transition) | `none` (suggestion only) |
| `global_timeout` | critical | `router.timeouts.global` reached; router exits | `stop` |
| `ticket_canceled` | warn | ticket canceled (`tb ticket cancel`) while running; the watchdog kills the agent, the result is discarded and the ticket stays canceled | `none` |
| `multi_dep_base` | warn | ticket with ≥ 2 dependencies not all merged is based on `HEAD`; run `tb merge` on the dependencies first | `none` |
| `agent_escaped` | error | an agent descendant escaped the process group (`setsid`/`setpgid`); it was killed and the owning ticket (by `TB_TICKET_ID`; unattributable ⇒ all running ones) fails without retry or git (Linux) | `escalate` |
| `orphan_killed` | warn | a non-agent orphan was killed without failing any ticket (daemon born during worktree creation without `TB_TICKET_ID`, or an orphan with no agent running) | `none` |
| `board_unreadable` | warn | a ticket YAML is invalid; workers kept, retried every tick | `none` |
| `bus_unclaimed` / `bus_claim_rejected` / `bus_lease_lost` / `bus_verdict_fail` | warn | bus slots: no valid claim in time; a claim without the marker, by a non-worker or a non-addressee, or before the assign; the claim changed hands, was released or its owner is not alive; the agent reported FAIL ([bus-router.md](bus-router.md)) | `none` |
| `bus_channel_expired` / `bus_write_failed` | warn | the bus channel expired (bus slots idle); a bus write failed (expired channel: the attempt is requeued) | `none` |

**Example:**

```bash
tb watchdog alerts --kind worker_stuck
# 11:57:56  critical worker_stuck   w1 TKT-001     TKT-001 on w1 exceeded the timeout of 2s; canceling [action: retry]
# 11:57:59  critical worker_stuck   w1 TKT-001     TKT-001 on w1 exceeded the timeout of 2s; canceling [action: escalate]
```

---

### `tb merge` — Result Aggregation (✓ Phase 3)

Integrates the commits of `done` tickets (`base_commit..head_commit`) into a
temporary detached worktree and only then advances the target branch.

```bash
tb merge TKT-001 TKT-002
tb merge --dry-run TKT-001 TKT-002
tb merge --strategy last_wins TKT-001 TKT-002
tb merge --strategy semantic TKT-001 TKT-002
tb merge --strategy prompt TKT-001 TKT-002
tb merge --strategy prompt --resolutions 3f2a9c1d5b7e4a60 TKT-001 TKT-002
tb merge --into integration TKT-001 TKT-002
tb merge --output changes.patch TKT-001 TKT-002
tb merge --force TKT-001
tb merge --allow-sensitive TKT-001
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--strategy fail\|last_wins\|semantic\|prompt` | Conflict handling (default `fail`) |
| `--resolutions <id>` | `prompt` only: replays the receipt of an earlier proposal instead of calling the agent |
| `--dry-run` | Integrates in the temporary worktree, reports and discards |
| `--into <branch>` | Target branch (default: current branch) |
| `--output <file.patch>` | Writes the `target..integrated` diff without moving refs |
| `--force` | Allows re-merging already merged tickets |
| `--allow-sensitive` | Allows applying tickets that change sensitive paths (see below) |
| `--json` | JSON output |

Apply order: dependencies first, then argument order. Each ticket must be
`done` with a `head_commit`.

**Conflict strategies:**

| Strategy | Behavior |
|----------|----------|
| `fail` (default) | Aborts on the first conflict, lists files/tickets and **changes no ref** |
| `last_wins` | On a conflicting file, keeps the **whole** file from the later ticket (no hunk merging) and records the resolution |
| `semantic` | Resolves the hunks git left in conflict with deterministic rules and reports each one (see below); if any hunk stays unresolved it behaves like `fail` |
| `prompt` | Runs `semantic`, then a sandboxed one-shot agent on the hunks left; **two steps**: the proposal writes a receipt and never moves a ref, `--resolutions` applies it |

**`semantic`** (spec: [merge-strategies](specs/merge-strategies.md)): for each unmerged file tb reads the index stages and runs `git merge-file --diff3` (no attributes, no merge driver), then tries two rules per conflict region: `disjoint` (both sides edit disjoint base lines: both edits apply) and `go_imports` (different imports added to one parenthesized Go import block: the union, minus specs a side removed, sorted by path). A resolved file must have no conflict-marker line that its inputs lacked, and a Go file that parsed before must still parse (`parse_failed`). Never resolved, whatever the flags: binary (`binary`), symlink, gitlink, delete/modify (`delete_modify`), add/add (`add_add`), mode (`mode`) and sensitive-path (`sensitive`, even with `--allow-sensitive`) conflicts, and files above `merge.semantic.max_file_bytes` (`too_large`). Overlapping edits are `overlap`; a region above `merge.semantic.max_region_lines` is `region_too_large`. Once every conflict resolves it applies in one call, like `fail`/`last_wins` (`--dry-run`, `--output` and the sensitive gate unchanged). JSON: `conflicts[].resolution` is `semantic`, `reason` and `hunks` (`region`, `ours_lines`, `theirs_lines`, `rule` or `reason`) say how or why not; `files[].status` is `semantic`; `resolutions[]` has one entry per resolved region with `rule` and the sha256 of its base, ours, theirs and result lines; `result_tree` is the integrated tree.

**`prompt`**: after `semantic`, the regions left go to the egm agent (`egm.agent_cmd` and the other agent keys, read from the user config only; there is no second agent config), one sandboxed run per file: the same Landlock, subreaper, disposable `HOME` and vendor profile as `tb egm --engine llm`, read-only empty workdir, prompt on stdin. Without Landlock it refuses (`no_landlock`) and never falls back; without `egm.agent_cmd` the proposal fails with `agent_unavailable`. The agent cannot touch disk: it returns one JSON object `{nonce, regions:[{id,text}]}` (or `{nonce, refuse}`) and tb splices the texts into the conflict regions. Answers are validated (`agent_protocol` nonce/IDs/encoding, `agent_oversized`, `agent_markers`, `parse_failed`, `agent_secret`, `agent_refused`, `agent_novel`). Never sent to the agent: sensitive paths, deny-listed paths (`secret_path`) and files whose regions or context match a secret pattern (`secret_in_context`); tb refuses instead of redacting. Resolved lines that appear in none of base, ours and theirs are reported as `novel_lines` (a ceiling is `merge.prompt.max_novel_lines`, unset by default). Transient failures (`timeout`, `stdout_overflow`, `missing_output`, `exit_<n>`) are retried (`merge.prompt.attempts`); `tools_used`, `escaped_process`, `unattributed_orphan`, `sandbox_error` and an interrupted or over-budget run abort the whole merge with no receipt. The proposal prints and returns `receipt` (`id`, `path`) and `result_tree`, whatever `--dry-run` says. The receipt is a 0600 JSON file under `$XDG_STATE_HOME/toolbelt/merge/<repo hash>/` (never in the repository; its ID is the first 16 hex characters of its sha256) with the tickets, agent identity, run records (no stderr, no environment), the resolutions with their text and the result tree; read it with `cat`/`jq`, receipts older than 30 days are swept. `--resolutions <id>` re-integrates, replays each agent region without starting any agent and fails before moving a ref with `receipt_tampered` (bytes changed), `receipt_stale` (target, start SHA or tickets changed: rerun the proposal), `receipt_mismatch` (a region's input hashes or the resulting tree differ) or `receipt_incomplete` (the proposal left conflicts). Budgets (user config only, `merge:` section): `merge.semantic.max_region_lines` (2000), `max_file_bytes` (1 MiB); `merge.prompt.max_files` (10), `context_lines` (20), `max_prompt_bytes` (256 KiB, over it `prompt_budget`), `timeout_per_file` (5m), `max_total` (20m), `attempts` (2), `max_stdout_bytes` (1 MiB), `max_novel_lines`. A repository `.toolbelt/config.yaml` that sets `merge.*` or the egm agent keys is ignored with a `config_untrusted` warning. MCP `tb_merge` accepts `semantic` and refuses `prompt` with `prompt_cli_only`.

**Atomicity:** the target branch is never touched during integration. The
advance uses compare-and-swap (`git update-ref refs/heads/X new old`) and only
happens if the target is still at the starting SHA; if the target branch is
checked out and dirty, the merge is refused. The temporary worktree is always
removed. On success the tickets get `merged_at`.

**Trust gate:** every `git` call in merge runs hardened (`core.hooksPath=/dev/null`,
`core.fsmonitor=false`, `diff.external=`, `core.pager=cat`, `commit.gpgsign=false`,
`maintenance.auto=false`, `gc.auto=0`, `submodule.recurse=false`), so hooks
written by agents (e.g. `post-merge` in `.githooks/`) do not run on the
fast-forward or in temporary worktrees. Additionally, when **applying**, the merge
is refused (no refs changed; lists ticket + path + reason) if any ticket changes
sensitive paths: the hooks directory (`core.hooksPath`, `.husky/`, `.githooks/`),
`.toolbelt/**`, `.github/workflows/**`, `.gitmodules`, gitlinks (mode 160000),
`.gitattributes` at any level, and the `guard` protected paths. Review and use
`--allow-sensitive` (`--force` remains only for re-merge). `--dry-run`/`--output`
only report; JSON carries `sensitive: [{ticket, path, reason}]`.

**`last_wins` details:** integration commits run with `gpgsign` and hooks
disabled; if `--continue` fails, the merge aborts without changing any ref;
overlap detection uses `--no-renames` (renames count for both paths). In the
MCP tool `tb_merge`, the conflict report is returned with `ok:false`.

**Output examples:**

```bash
tb merge --dry-run TKT-001 TKT-002
# merge TKT-001 TKT-002 -> main (fail) [dry-run]
# FILE           TICKETS  STATUS
# f-TKT-001.txt  TKT-001  merge
# f-TKT-002.txt  TKT-002  merge
# result: a6ebf42ee47be5c4ca290b2055efe75b00f8eaf6 (start 356c934e)
tb merge TKT-002 TKT-003
# merge TKT-002 TKT-003 -> main (fail) [not applied]
# FILE     TICKETS          STATUS
# a.txt    TKT-002,TKT-003  conflict
# Error: merge conflict: 1 file(s) in conflict; no ref was changed (use --strategy last_wins to resolve)
tb merge --strategy semantic --dry-run TKT-004 TKT-005
# merge TKT-004 TKT-005 -> main (semantic) [dry-run]
# FILE      TICKETS          STATUS
# shared.go TKT-004,TKT-005  semantic
# conflict: shared.go (TKT-005, commit 1a2b3c4d; also: TKT-004) - resolved by semantic (go_imports)
# result: 9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b (start 356c934e)
# result tree: 0f1e2d3c4b5a69788796a5b4c3d2e1f001122334
```

---

### `tb bus` — Agent Bus (✓ B1, ADR-013)

Local bus for agents from different vendors (Claude Code, Codex, Grok, ...) to exchange `bus/0.1` messages through the same store (`.toolbelt/bus/bus.db`, SQLite WAL, no daemon). Envelope, Agent Card and invite spec: [bus-protocolo.md](bus-protocolo.md). Lease, heartbeat and watchdog (B2): [bus-lease.md](bus-lease.md); the `tb bus run` harness (B3) is in [bus-harness.md](bus-harness.md).

```bash
tb bus channel create feature-x --owner claude-main --ttl 2h
tb bus invite feature-x --role worker
tb bus invite feature-x --role supervisor --name claude-main
tb bus join feature-x --invite K7Q2-9XMD --name grok-1 --vendor xai --role worker
tb bus send --channel feature-x --as claude-main --to role:worker --type task.assign --thread TKT-001 --payload '{"ticket":"TKT-001"}'
tb bus send --channel feature-x --as claude-main --to role:worker --type task.assign --thread TKT-001/api --parent TKT-001
tb bus recv --channel feature-x --as grok-1 --wait 20s
tb bus claim TKT-001 --channel feature-x --as grok-1
tb bus complete TKT-001 --channel feature-x --as grok-1 --epoch 1 --result '{"verdict":"PASS"}'
tb bus log feature-x --thread TKT-001
tb bus tree feature-x
tb bus channel list
tb bus channel show feature-x
tb bus heartbeat --channel feature-x --as grok-1
tb bus watchdog feature-x --dry-run
tb bus channel extend feature-x --ttl 2h
tb bus run --channel feature-x --as marin --check
tb bus run --channel feature-x --as marin
tb bus run --channel feature-x --as claude-code --once
tb bus discover
```

| Command | Who | Purpose |
|---|---|---|
| `channel create <channel> --owner N` | human | creates the channel (`--ttl`, no expiry by default; `--lease`, default `2m`, `30s`–`1h`) and the owner invite (role `supervisor`; `--invite-ttl`, default `10m`) |
| `channel list` / `channel show <channel>` | human | lists channels / shows the channel's lease, the agents (role, alive, `last_seen`) and the open claims (owner, epoch, held/expired/released); no Agent Card `session`, token or payload |
| `channel extend <channel> --ttl D` | human | resets the expiry to now + `D` (`0` = no expiry); reopens an expired channel keeping sessions and history |
| `invite <channel> --role R` | human | single-use invite with a pinned role (`--ttl`, default `10m`); `--name` binds it to an agent and, if that agent is already in the channel, recovers its lost session (new token) |
| `join <channel> --invite C` | agent | joins with an Agent Card (`--name`, `--vendor`, `--model`, `--tier`, `--wake`, `--session`, `--capability` or `--card card.json`); `--role` must match the invite's role; stores the session in the bus private directory, outside the repository (`$XDG_STATE_HOME/toolbelt/bus/`, `0600`) |
| `send` | agent | `--to agent:\|role:\|topic:`, `--type`, `--thread`, `--causation`, `--idempotency-key`, `--ttl`, `--payload` (JSON or `@file`); `--parent T` (only on `task.assign`) links the sub-task to thread `T`, which the sender must hold with a live claim (the router may link any known parent): the broker refuses with `parent_unknown`, `parent_not_owned`, `parent_immutable`, `depth_exceeded` (max depth 3) or `fanout_exceeded` (max 8 open children), see [bus-protocolo.md](bus-protocolo.md#delegation-trees) |
| `recv` | agent | reads and advances the cursor; `--wait` (max 25s), `--limit`, `--peek`, `--since` |
| `claim <thread>` | agent | single owner of the thread; prints the `epoch`; takes over a released claim or one whose owner lost its lease |
| `complete <thread>` | owner | `--epoch` (required, from `claim`), `--result`; publishes `task.result` to `topic:board`; `lease_lost` when the claim changed hands or was released; completing a parent with open children succeeds and prints `warning: … completed with N open children (…)` on stderr (`open_children` in `--json`) |
| `heartbeat` | agent | renews the lease; lists held threads (with epochs) and lost ones (with the new owner) |
| `watchdog <channel>` | human | releases claims of agents that are not alive and publishes `task.status` `lease_expired` from `system:watchdog` to `topic:board`; `--dry-run` lists only. Not exposed over MCP |
| `log <channel>` | human | audit trail (redacted payload); `--thread`, `--limit` |
| `tree <channel>` | human | delegation trees built by `send --parent`: one node per line with state (`assigned`, `claimed`, `released`, `completed`, `revoked`, `expired`), owner, epoch and open children; `--thread T` (only that subtree), `--all` (also threads without children), `--limit N` (trees, newest first, default 50); no payload, result, token or Agent Card `session`; works on an expired channel. CLI only: not exposed over MCP |
| `run --channel C --as N` | human | B3 harness: waits without an LLM and wakes the agent through the wake file (`exec`, `harness` or `notify`, which pings the human for GUI-only agents); `--once`, `--check`, `--dry-run`, `--invite`, `--role`, `--config`, `--allow-unsandboxed`. Spec: [bus-harness.md](bus-harness.md), [bus-notify-discover.md](bus-notify-discover.md) |
| `discover` | human | read-only catalog of known agent CLIs on `PATH` and MCP client configs that run `tb mcp` (other servers only counted), with next steps; executes nothing; `--cwd`, `--json` |

Agent commands require `--channel` and `--as`. The session token is never printed. `--json` returns JSON-RPC envelopes.

---

### `tb bench` — Benchmark ✅ IMPLEMENTED (A/B in Phase 2)

Runs benchmarks and measures KPIs. Supports A/B comparison between configs/feature flags.

```bash
tb bench <subcommand>

Subcommands:
  run       Runs the benchmark ✅
  compare   Compares two benchmarks or A/B configs ✅
  report    Generates a Markdown report of results ✅
  list      Lists available tasks ✅
```

**`tb bench run`:**

```bash
tb bench run [flags]

Flags:
  --fake               Uses the fake agent built into the `tb` binary (portable, any repo)
  --fake-config <path> Fake agent configuration file
  -a, --agent <cmd>    External agent command (fallback: bench.default_agent)
  -t, --tasks-dir      Tasks directory (fallback: bench.tasks_dir)
  -f, --fixtures-dir   Fixtures directory (fallback: bench.fixtures_dir)
  -r, --repetitions <n> Repetitions per task (default: 1)
  -o, --output <path>  JSONL output file (without the flag: under bench.output_dir)
  --label <name>       A/B variant label
  --flag key=value     Feature flag (repeatable)
  --ab-config <yaml>   A/B variant from a YAML file
  --task <id>          Filters tasks
```

Path and agent precedence: **explicit CLI flag** > **`config.yaml`** > **default**.
The global `-c/--config` flag feeds `config.Load()` (including `bench.*`).

If `--agent` and `--fake` are omitted, `tb bench run` uses `bench.default_agent`
from the loaded YAML (when set). Without `-t`/`-f`/`-o`, it uses
`bench.tasks_dir` / `bench.fixtures_dir` / `bench.output_dir`.

```yaml
# .toolbelt/config.yaml
bench:
  default_agent: "cursor-agent {prompt} --workdir {workdir}"
  tasks_dir: /abs/path/to/bench/tasks
  fixtures_dir: /abs/path/to/bench/fixtures
  output_dir: /abs/path/to/bench/results
```

Placeholders `{prompt}`, `{workdir}`, and `{timeout}` are interpolated with
POSIX quoting (single quotes), avoiding injection via quotes, `$()`, or `;`.
The harness also exports `TB_BENCH_PROMPT`, `TB_BENCH_WORKDIR`, and
`TB_BENCH_TIMEOUT` for agents that prefer reading env instead of argv.

A task only marks `passed: true` when the **agent** exits 0 **and** verification
reaches `expected_exit_code`. Agent exit ≠ 0 (e.g. `false`)
forces `passed: false` even if the fixture passes. If the agent command does not
exist (exit 127), `tb bench run` writes the JSONL and exits with code ≠ 0.

**`tb bench compare` (A/B):**

```bash
# Compare two existing JSONL files
tb bench compare --before graph-off.jsonl --after graph-on.jsonl

# Runs two configs on the same set of tasks and compares them
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml --fake

# Same flow with an external agent (--agent or bench.default_agent)
tb bench compare --config-a bench/ab/graph-off.yaml \
                 --config-b bench/ab/graph-on.yaml \
                 --agent "cursor-agent {prompt} --workdir {workdir}"
```

The Markdown report and JSONL include **success (pass@1)**, **time**, and **tokens** metrics, with A→B deltas.

**Examples:**

```bash
tb bench run --fake --label graph-off --flag graph=off
tb bench run --fake --ab-config bench/ab/graph-on.yaml
tb bench run -r 1   # uses bench.default_agent / tasks_dir from config.yaml
tb -c /tmp/alt-config.yaml bench run -r 1   # -c feeds config.Load()
tb bench compare --before bench/results/graph-off.jsonl \
                 --after bench/results/graph-on.jsonl
tb bench list
tb bench report --input bench/results/graph-on.jsonl
```

---

### `tb mcp` — MCP Server (✓ implemented)

Starts toolbelt as an MCP server over **stdio** (JSON-RPC 2.0). There is no `tb serve` in the current CLI.

```bash
tb mcp [flags]

Flags:
  --bus-as <channel>/<name>  Binds the connection to the bus session <channel>/<name> (used by tb bus run)
  --bus-root <path>          Bus repository root (default: searches for .toolbelt from the cwd)
  --bus-state <dir>          Base of the bus private directory (default $XDG_STATE_HOME/toolbelt/bus)
```

Unix/tcp transport and `--socket`/`--port`/`--tls-*` flags are planned; today only stdio.

**Tools exposed in `tools/list` / `tools/call`** (52; graph tools are listed, but calling them requires `TB_GRAPH=on`):

| Tool | CLI parity | Notes |
|------|------------|-------|
| `tb_version` | `tb version` | |
| `tb_run` | — | `sh -c` with timeout inside the Landlock sandbox (see [MCP `tb_run` sandbox](#mcp-tb_run-sandbox)); filtered env; `env`/`pass_env` args (arrays); `cwd` must be inside the repository |
| `tb_mem_add` / `tb_mem_list` / `tb_mem_show` / `tb_mem_edit` / `tb_mem_retire` / `tb_mem_search` / `tb_mem_check` / `tb_mem_init` | `tb mem …` | optional `cwd` arg; `ref`/`all` (boolean; legacy strings ok) where applicable |
| `tb_verify_run` | `tb verify run` | `ruler` (comma list: `tests`, `coverage`, `visual`); args mirror flags + `env`/`pass_env`, including `coverage_cmd`, `coverage_pattern`, `min_coverage` (number), `typecheck_cmd`, `no_typecheck` (boolean), `checks` (array of `{name, cmd}`), `skip_checks` (array of names), `no_checks` (boolean) |
| `tb_review` | `tb review` | ticket/base/head/approved_sha/require_approved_sha/reviewers/standards |
| `tb_drift` | `tb drift` | returns `{drift, has_drift, pinned_approval}` (`checked` explicit) |
| `tb_guard_check` | `tb guard check` | `base`/`head` or `range=A..B`; `strict_fail` by default on SeverityError; `warn_only=true` opt-out |
| `tb_stack` | `tb stack` | `ops` (array, includes `egm`, always moved to the end), `base`/`head`, `fail_fast` (boolean), `jobs` (integer), `approved_sha`/`approved_base`/`require_approved_sha`, `env`/`pass_env`, `egm_docs` (array), `strict` (boolean), `egm_mode` (`local`/`ci`), `egm_config` (absolute path, required by `ci`), `egm_allow_offline` (boolean); verify fields `ruler` (`visual` refused), `coverage_cmd`, `coverage_pattern`, `min_coverage`, `typecheck_cmd`, `no_typecheck`, `checks`, `skip_checks`, `no_checks`; returns aggregated JSON with `warn` |
| `tb_egm_status` / `tb_egm_resolve` / `tb_egm_receipt` | `tb egm status` / `tb egm resolve` / `tb egm receipt` | optional `cwd` (absolute) and `doc`; status takes `at`/`strict`, resolve takes `fix`/`decline` (arrays), `force_settle`/`reason`, `contest`/`evidence`; returns `{report, exit_code}` (receipt adds `receipt`, the written path) |
| `tb_vault_list` / `tb_vault_show` / `tb_vault_search` / `tb_vault_links` / `tb_vault_backlinks` / `tb_vault_check` / `tb_vault_new` | `tb vault …` | optional `cwd` (absolute); `note` for show/links/backlinks, `query`/`limit` for search, `type`/`tag`/`source` for list; new takes `title`, `confidence` (required), `type`, `tags`, `dir`, `body` and always writes `source: inferred`; note text comes back inside `data` |
| `tb_ticket_list` / `tb_ticket_show` | `tb ticket list` / `tb ticket show` | `status`, `deps` / `id`; optional arg `cwd` |
| `tb_ticket_create` | `tb ticket create` | `title`, `description`, `scope`/`forbid`/`accept`/`dep` (arrays), `priority` (integer), `timeout`, `dry_run` (boolean) |
| `tb_board` | `tb board` | `action=list\|pause\|resume\|drain`, `strategy`, `status`, `worker` |
| `tb_router_status` | `tb router status` | read-only view of `state.json` |
| `tb_watchdog_alerts` | `tb watchdog alerts` | `since`, `kind`, `limit` (integer) |
| `tb_merge` | `tb merge` | `ids` (array), `strategy` (`fail`, `last_wins`, `semantic`; `prompt` is refused with `prompt_cli_only`), `into`, `force`, `allow_sensitive` (default `false`); **dry-run by default**; only changes refs with `apply=true` (the CLI takes `--dry-run` instead: the one documented naming exception, because an agent must opt in to moving refs) |
| `tb_bus_join` / `tb_bus_send` / `tb_bus_recv` / `tb_bus_claim` / `tb_bus_complete` / `tb_bus_heartbeat` / `tb_bus_status` | `tb bus …` | identity bound to the connection after `tb_bus_join` (`channel`, `invite`, `name` + Agent Card); `payload`/`result` as objects; `tb_bus_complete` requires the integer `epoch` and reports `open_children`; `tb_bus_send` takes an optional `parent` (`task.assign` only; the broker checks it); no tree tool; `tb mcp` heartbeats bound sessions in the background; no watchdog tool; `wait` string (max 25s); creating channels/invites does **not** exist through MCP; `tb mcp --bus-as <channel>/<name>` binds the connection to a CLI session (used by `tb bus run`) |
| `tb_graph_search` / `tb_graph_callers` / `tb_graph_deps` / `tb_graph_impact` / `tb_graph_explain` / `tb_graph_path` / `tb_graph_index` | `tb graph …` | call requires `TB_GRAPH=on`; `depth`/`limit`/`max_depth` integer; `transitive` boolean; `explain` takes `node`, `path` takes `from`/`to`; `tb_graph_search` `mode` and `tb_graph_index` `vectors` (see [vector index](#vector-index)) |

Schemas type `depth`/`limit`/`reviewers`/`max_iter` as integer, `strict`/`warn_only`/`transitive`/`skip_unavailable`/`all` (mem) as boolean, `threshold` as number, and `standards` as a string array; legacy string values are still accepted. verify/review/drift/guard reports come back as JSON in `content` (without a process exit code). Empty lists use `[]`; `hidden_passed` is `null` if the hidden suite does not apply.

---

### `tb init` ✅ IMPLEMENTED

```bash
tb init [flags]

Flags:
  --global         Initializes global configuration (~/.config/toolbelt)
  --write-agents   Seeds a minimal AGENTS.md if missing (never overwrites)
  --write-checks   Writes the suggested verify.checks (govulncheck, clippy) into a new config.yaml
  --json           JSON output (global flag)
```

With `--json`, emits a payload with `path`, detected `test_cmd` / `build_cmd` / `lint_cmd`
(when present), optional `warning`, `agents` (`seeded`, `sources`, …) and, when checks
were suggested but not written, `suggested_checks`.

Toolchain detection details and `verify.*` population are in the `tb verify` section.

**Local state vs config (`.git/info/exclude`):** in a Git repository (without `--global`),
init writes to `.git/info/exclude` (does not change the project's `.gitignore`) the
**cache/derived** patterns:

- `.toolbelt/graph.json`
- `.toolbelt/logs/`
- `.toolbelt/workspaces/`
- `.toolbelt/captures/`
- `.toolbelt/bench/`
- `.toolbelt/bus/`
- `<vault.path>/.obsidian/` (the vault itself is versioned; `tb init` creates a custom `vault.path` and warns `vault_gitignored` when a `.gitignore` rule ignores it)

On a repository initialized before the vault was versioned, run `tb init` again: the old `.toolbelt/vault/` line leaves the section, and a section with a lost end marker keeps your lines.

In a `git worktree`, the file is the common directory's `info/exclude`
(`$GIT_COMMON_DIR`), the only one git reads. Versionable files
(`config.yaml`, `agent_docs.yaml`, `invariants.yaml`, `AGENTS.md`) are created
with mode `0644` (subject to umask).

So `git stash -u` / `git clean` do not wipe the index/graph by mistake, without
blocking config versioning.

**Recommendation — version invariants:** `invariants.yaml` (and, when it makes
sense for the team, `config.yaml` / `guard.yaml` / `thresholds.yaml`) should
remain **versionable**. They are shared policy (ADR-010), not cache.
Init does **not** put all of `.toolbelt/` in exclude precisely to separate
cache from config.

---

### `tb sandbox` ✅ IMPLEMENTED

Landlock isolation on Linux (`check` / `run`). Does not create worktrees — use `tb workspace` for that.

```bash
tb sandbox check
tb sandbox run <command> [args...] [flags]
```

**Flags for `tb sandbox run`:**

| Flag | Default | Description |
|------|---------|-------------|
| `--workdir <path>` | cwd | Working directory (write allowed) |
| `--timeout <dur>` | `30s` | Process timeout |
| `--allow-unsandboxed` | `false` | Explicit opt-in to run **without** Landlock |

**Fail-closed** behavior: if the kernel has no Landlock (or ruleset application fails), `tb sandbox run` **refuses** to execute and returns a clear error asking for `--allow-unsandboxed`. There is no silent fallback to unsandboxed execution.

On Linux with Landlock, writes are restricted to `--workdir` (and explicitly allowed paths); the system temp directory (`os.TempDir()`, typically `/tmp`) gets no access (not even read) outside the workdir — for temporary files, point `TMPDIR` inside the workdir. System paths needed for execution (`/bin`, `/usr`, `/lib`, `/etc`, `/dev`, `/proc`, …) are read-only. Minimal per-file exception under `/dev`: `/dev/null`, `/dev/zero`, `/dev/full` are read+write (plus truncate on ABI ≥ 3), `/dev/random` and `/dev/urandom` are read-only; missing nodes are skipped. Terminals (`/dev/tty`, `/dev/ptmx`, `/dev/pts/*`) are deliberately blocked (escape/TIOCSTI injection into other terminals). The rest of `/dev` (including `/dev/shm`) stays read-only. The ruleset is applied in the child process (re-exec + `PR_SET_NO_NEW_PRIVS` + `landlock_restrict_self`) **before** `exec` of the command.

```bash
tb sandbox check
# Landlock: true
# ABI: 6

tb sandbox run --workdir /tmp/job sh -c 'echo ok > out.txt'
# write outside workdir → Permission denied

# Without Landlock (or for CI without LSM): explicit opt-in
tb sandbox run --allow-unsandboxed echo hello
```

With `--json`, `sandbox check` emits `os`/`arch`/`landlock`/`version`/`error`; `sandbox run` emits the `Result` in snake_case (`exit_code`, `timed_out`, `duration_ns`, `sandboxed`, …). If the child exits ≠ 0 without timeout, `tb` propagates that exit code.

---

### `tb workspace` ✅ IMPLEMENTED

Isolated workspaces via git worktree. Alias: `tb ws`.

```bash
tb workspace create <id> [--base <ref>] [--allow <path>] [--forbid <path>]
tb workspace list
tb workspace remove <id>
```

`--base` defaults to `HEAD`. `--allow`/`--forbid` are repeatable (`stringSlice`). create/list JSON uses snake_case.

---

### `tb version` / `tb --version` ✅ IMPLEMENTED

```bash
tb version [flags]
tb --version
```

Flags (subcommand `version`):
  --json    JSON output ✅

The global `--version` flag prints the same textual output as `tb version`
(version, commit, build, go, os/arch).

**Examples:**

```bash
tb version
tb --version
# toolbelt dev
#   commit: unknown
#   built:  unknown
#   go:     go1.22.2
#   os:     linux/amd64
#   embed:  llama.cpp b12600 (29b52d398b29)

tb version --json
# {"version":"dev","commit":"unknown","build_time":"unknown","go_version":"go1.22.2","embed":"llama.cpp b12600 (29b52d398b29)"}
```

A release build; a build without `-tags embednative` (`go build ./cmd/tb`) prints `embed:  unavailable (built without embednative)`.

All CLI `--json` outputs (and domain payloads embedded in MCP tools)
use **`snake_case`** keys consistently (`exit_code`, `base_ref`,
`timed_out`, …). MCP protocol envelopes (`isError`, `inputSchema`, …)
remain in the camelCase required by the spec.

---

## Colors and Visual Theme

Colored CLI output follows the **Synthwave** theme defined in [docs/identidade-visual.md](identidade-visual.md). Colors are applied automatically when the terminal supports them.

| Element | Color | ANSI 256 Code |
|---------|-------|---------------|
| Headers, commands | Purple `#982BD4` | 134 |
| Values, paths, URLs | Cyan `#0CD7FF` | 51 |
| Errors, failures | Pink `#F9278C` | 198 |
| Success | Green `#2DD4A7` | 43 |
| Warnings | Yellow `#FFB020` | 214 |
| Secondary text | Gray `#A099B0` | 249 |

**Control variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `NO_COLOR` | Disables colors (freedesktop standard) | - |
| `TERM=dumb` | Disables colors | - |

`TB_COLOR` / `TB_COLOR_MODE` are planned and are not yet read by the CLI.
See `assets/theme/tokens.json` for the full ANSI/256/TrueColor mapping table.

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TB_GRAPH` | Enables on-demand graph (`on`/`1`/`true`) | off |
| `NO_COLOR` | Disables colors (freedesktop standard) | - |
| `TB_BROWSER` | Browser for the visual verify ruler | (auto) |
| `TB_HOME` | Echoed in MCP; does not set the CLI config dir | - |
| `TB_BENCH_PROMPT` / `TB_BENCH_WORKDIR` / `TB_BENCH_TIMEOUT` | Exported by the `tb bench run` harness | - |
| `TB_BUS_STATE_DIR` | Bus private directory (sessions, wake files, harness state) | `$XDG_STATE_HOME/toolbelt/bus/<hash>/` |

Planned variables (not yet read by the CLI): `TB_VAULT`, `TB_LOG_LEVEL`,
`TB_SANDBOX_DEFAULT`, `TB_JSON_OUTPUT`, `TB_TIMEOUT_DEFAULT`, `TB_COLOR`,
`TB_COLOR_MODE`.

### Environment allowlist for child processes

`tb verify` / `tb stack` (verify step), MCP `tb_run` / `tb_verify_run` / `tb_stack`
do not pass the full environment to `sh -c`. They only pass keys from the built-in allowlist
(PATH, HOME, LANG, TMPDIR, Go/Node/npm/pnpm toolchain, browser paths
(`PLAYWRIGHT_BROWSERS_PATH`, `PUPPETEER_CACHE_DIR`, `PUPPETEER_EXECUTABLE_PATH`, …), proxies, XDG, etc.), plus:

| Source | Effect |
|--------|--------|
| `.toolbelt/config.yaml` → `env.allow` | Extends the allowlist |
| `.toolbelt/config.yaml` → `env.pass` | Explicit secret opt-in |
| Global flag `--env KEY` / MCP arg `env` | Extends the allowlist (does not bypass secrets) |
| Global flag `--pass-env KEY` | Explicit secret opt-in |
| MCP arg `pass_env` | Only keys already in `env.pass` of the **global** config (`~/.config/toolbelt/config.yaml` or `-c`); any other key is refused (the model cannot opt secrets in, and the repository's own config does not count because the agent can write it) |

Secrets: any name containing `TOKEN`, `SECRET`, `KEY`, or `PASSWORD`
(case-insensitive) is blocked without opt-in. For MCP `tb_run`, `env.pass` is read
only from the global config.

### MCP `tb_run` sandbox

MCP `tb_run` runs `sh -c` under Landlock, like `tb router` and `tb bus run`.
The child can:

- **write** to the repository root (the directory holding `.toolbelt`, or the
  server's working directory), to its own `TMPDIR` (created per call and removed
  afterwards) and to the toolchain caches that exist: `GOCACHE`, `GOMODCACHE`,
  `$GOPATH/pkg/sumdb`, the npm, pip and uv caches, `$CARGO_HOME/registry`;
- **read** the system dirs (`/usr`, `/etc`, `/lib`...), every `PATH` entry
  except `/`, `$HOME` and its ancestors, `GOROOT`, and the dir holding the `GOENV`
  file;
- reach nothing else: `$HOME` (`~/.ssh`, `~/.gitconfig`, shell history), the
  shared `/tmp` and other repositories stay closed. Git works, but warns that it
  cannot read `~/.gitconfig`.

`cwd` must be the repository root or below it. Extra paths and the opt-out
come only from the global config, because the repository (and its
`.toolbelt/config.yaml`) is writable from inside the sandbox:

```yaml
# ~/.config/toolbelt/config.yaml
sandbox:
  read: [~/.nvm]              # read-only, absolute or ~/ (must exist)
  write: [~/.cache/my-tool]   # read-write
  allow_unsandboxed: false    # true: run without isolation when Landlock is unavailable
```

Without Landlock (macOS, Windows, kernels without the LSM) `tb_run` fails
closed unless `sandbox.allow_unsandboxed: true`. The result carries
`sandboxed: true|false`. The network is not restricted.

---

## JSON Envelopes

Every JSON document `tb` prints on stdout with `--json`, and every MCP tool result body, has a top-level string field `v` = `<kind>/<major>.<minor>`. A minor bump adds fields; any other change bumps the major. Consumers must ignore unknown fields. The CLI and the matching MCP tool emit the same kind. The kinds are defined in one place, `internal/envelope`.

| Kind | Producer |
|------|----------|
| `error/1.0` | the error body (see below); MCP tool errors |
| `version/1.0` | `tb version`, `tb_version` |
| `init/1.0` | `tb init` |
| `mem/1.0` | `tb mem *`, `tb_mem_*` (a list is `{"v", "invariants": [...]}`) |
| `verify/1.0` | `tb verify run`, `tb_verify_run` |
| `guard/1.0` | `tb guard check`, `tb_guard_check` |
| `review/1.0`, `drift/1.0`, `stack/1.0` | `tb review`, `tb drift`, `tb stack` and their tools |
| `sandbox-run/1.0`, `sandbox-check/1.0` | `tb sandbox run`, `tb_run`; `tb sandbox check` |
| `graph-<op>/1.0` | `tb graph search\|callers\|deps\|impact\|explain\|path\|stats\|index\|rebuild`, `tb_graph_*` |
| `ticket/1.0`, `ticket-list/1.0` | `tb ticket *`, `tb_ticket_*` (a list is `{"v", "tickets": [...]}`) |
| `workspace/1.0` | `tb workspace *` (a list is `{"v", "workspaces": [...]}`) |
| `merge/1.0` | `tb merge`, `tb_merge` |

Experimental commands stamp `board/1.0`, `router/1.0`, `router-status/1.0`, `watchdog/1.0` and `watchdog-alerts/1.0` by the same rule (not frozen; `tb watchdog alerts` and `tb_watchdog_alerts` are `{"v", "alerts": [...]}`; `tb_router_status` is `router-status/1.0`); the bus, egm, vault, red, blue, lab and bench outputs are not stamped yet. Envelopes that already carried a `v` keep it at their own version (`bus/0.1`, `bus-wake/0.1`, `bus-discover/0.1`, `router-task/0.1`, `egm-source-drift/0.2`, `egm-draft/0.1`, `egm-brainstorm/0.1`).

On-disk files use an integer `version` instead: `.toolbelt/graph.json` (derived cache: a file without `version` or with another one is discarded and rebuilt by the next `tb graph index`, or reported `stale: true` over MCP) and `.toolbelt/tickets/TKT-*.yaml` (optional `version: 1`, absent means 1; a newer version is refused, never rewritten: because tickets are loaded together, one ticket with a newer version makes every ticket command that scans the directory fail, with an error naming the file and its version, until `tb` is upgraded).

## Errors and Usage

- Expected domain/runtime errors (`RunE`, e.g. missing required business flag, git failure, missing file) exit with **exit 1** and a stderr message **without** a Cobra `Usage` dump.
- Genuine **flag** or **args** parse errors (unknown flag, `ExactArgs`/`MinimumNArgs`, a missing required flag, etc.) exit with **exit 2** and keep `Usage` on stderr to guide invocation.
- An unknown subcommand at any level (e.g. `tb mem nao-existe`, `tb bench nope`) exits with **exit 2** and error `unknown command "…"`, instead of printing help with exit 0.
- With `--json`, an error is also printed on stdout as one `error/1.0` document, `{"v": "error/1.0", "error": "<message>", "reason": "<code>", "exit_code": <n>}`; the text message stays on stderr. `reason` is `usage` for exit 2, `command_not_found` for exit 127, the command's own reason where it has one (egm, vault), otherwise `error`. A command that already printed its result document and then fails (`tb mem check`, `tb merge`) does not add a second one. Over MCP a tool error is `isError: true` whose text is the same body (`reason` is the `code:` prefix of the message when it has one, `tool_not_found` with exit code 2 for an unknown tool, otherwise `error` with exit code 1).
- Git failures in `guard` / `review` / `drift` / `verify` (and in the `stack` that stacks them) do **not** show only `exit status 128/129`: the message includes git stderr, the missing ref (`ref missing: …`), and a `git fetch origin <ref>` hint. For a relative ref (`HEAD~2`, `main^`) the hint is about short history instead (single-commit repo or shallow clone: `--base <sha>` or `git fetch --deepen=1`).
- `--approved-sha` equal to the base (empty `base..approved` range) does not dump `git range-diff` usage; exits with a clear message about the invalid range.

## Exit Codes

The table is stable (1.x may add codes, never change a meaning).

| Code | Meaning |
|------|---------|
| 0 | Success: `PASS`/`SKIPPED` in verify; guard without `SeverityError` or with `--warn-only`; a check that passed |
| 1 | The check ran and failed (`FAIL`/`ESCALATE` in verify, `SeverityError` in guard, vault check, egm `FAIL`), or a runtime error after work started |
| 2 | Usage error or refusal before any work: bad flag or argument, unknown subcommand, missing required flag, invalid `--ruler`/`--check` (`tb verify run`, `tb stack`), egm/vault refusal reasons |
| 3 | `tb egm`: the verdict does not allow the operation (`receipt` is refused unless `PASS`/`PASS_PARTIAL`) |
| 124 | `tb sandbox run`: the child hit `--timeout` (with `--json` too; the document has `timed_out: true`) |
| 127 | A configured command was not found or is not executable: `tb verify run` (`test_cmd` / `build_cmd` / `typecheck_cmd` / `lint_cmd` / a named check / `hidden_test_cmd`; with `--json` it emits the `verify.Result` envelope with an `error` field) and the `tb sandbox run` command (not found, or not executable; the `error/1.0` body has `reason: "command_not_found"`, the same for a bare name and a path) |
| 130 | Interrupted by SIGINT/SIGTERM (`tb mcp`; `tb sandbox run` when canceled) |
| child | `tb sandbox run` exits with the child's non-zero code, with or without `--json` (a child killed by signal `n` exits `128+n`, and the document has `signal`, e.g. `"SIGKILL"`) |
| max | `tb stack` exits with the highest exit code among its steps |

When the configured verify command does not exist/is not executable, verify fails immediately (no retries) and the `tb` process exits **127**.

---

## Configuration

Local configuration file at `.toolbelt/config.yaml` (global:
`~/.config/toolbelt/config.yaml`). The `-c/--config` flag points to an alternate YAML.

A key `tb` does not know (a typo, or a key removed before 1.0) is ignored: each file prints
one stderr line per such key, once per run, with the full key path
(`warning: .toolbelt/config.yaml: unknown config key graph.auto_index (ignored)`). It is never an
error, so old files keep loading. Removed keys: `defaults.*`, `sandbox.allow_net`, `sandbox.cpu_limit`,
`sandbox.mem_limit`, `sandbox.time_limit`, `graph.auto_index`, `graph.infer_relations`,
`vault.sync_on_change` and `logging.*`; they were parsed but no command read them, so they enforced nothing.

```yaml
# .toolbelt/config.yaml (exemplo)
verify:
  test_cmd: "go test ./..."
env:
  allow: [APP_ENV, CUSTOM_OK]
  pass: [GITHUB_TOKEN]
bench:
  default_agent: "cursor-agent {prompt} --workdir {workdir}"
  tasks_dir: bench/tasks
  fixtures_dir: bench/fixtures
  output_dir: bench/results
graph:
  languages: [go, javascript, python]
  docs: [markdown]
  frameworks: [react]
  max_file_bytes: 524288
```
