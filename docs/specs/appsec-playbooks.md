🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/appsec-playbooks.md)

# Spec: AppSec red/blue playbooks (`tb red` / `tb blue`)

Author: toolbelt · Status: proposed (not implemented) · Date: 2026-11-20 · Decision: [ADR-017](../adr/017-appsec-red-blue-labs.md) · Companion: [red/blue labs](red-blue-labs.md)

## Overview

`tb red` and `tb blue` give the agent native, owned AppSec methodology: a **red** corpus (offensive — how a vulnerability class manifests and is surfaced in code or a running service) and a **blue** corpus (defensive — detection, hardening, incident response). Content is authored in-repo in `tb`'s voice, scoped to software security, with no runtime dependency on any external corpus. Playbooks are indexed into the existing SQLite + FTS5 graph and served — compact in search, budgeted on load — over the CLI and MCP.

Crucially, a playbook is not inert prose: a blue playbook can emit **runnable checks** in `tb`'s existing verification mechanisms, so hardening becomes something the project keeps enforcing.

This spec defines the playbook format, the in-scope taxonomy, red↔blue pairing, the index, the command and MCP surface, the budget contract, and the verification hook. It adds no new storage engine and no executable attack tooling of its own (execution happens only inside a lab — see the companion spec).

## Goals / Non-Goals

**Goals**
- Native, owned corpus — reimplementation, not vendoring (ADR-014/015).
- Scope strictly to AppSec / secure-SDLC.
- Red↔blue pairing by vulnerability class.
- Blue steps that compile to `tb verify` / `tb guard` / `tb stack` checks.
- Reuse the graph (ADR-003/011/014) and MCP server (ADR-002); honor the ADR-011 token discipline.

**Non-Goals**
- No broad pentest breadth (wireless, AD, C2, social engineering) — future extension, out of scope now.
- No external `SKILL.md` ingestion.
- No payload execution in this surface; that is `tb lab`'s job under isolation.

## In-Scope Taxonomy (initial)

| Area | Classes (red) → mirror (blue) |
|------|------------------------------|
| Web/API | injection (SQL/NoSQL/command), XSS, SSRF, SSTI, XXE, IDOR, open-redirect, request-smuggling, deserialization, file-upload, GraphQL abuse |
| Auth | JWT flaws, OAuth/OIDC misuse, session/authz gaps |
| Crypto | weak primitives, misuse (IV/nonce/mode), TLS config |
| Supply chain | dependency confusion, typosquat, lockfile/integrity, build/CI secrets |
| Secrets | hardcoded secrets, leakage in logs/history, env handling |
| Platform | container hardening, cloud IAM/exposure, CSPM gaps |

Every red class has a blue mirror (detection + hardening + IR). The list is the Phase-1 target, not a frozen set.

## Playbook Format

Native Markdown with YAML frontmatter, authored under `playbooks/`:

```
playbooks/
  red/<class>/PLAYBOOK.md
  blue/<class>/PLAYBOOK.md
```

Frontmatter:

```yaml
name: red-insecure-deserialization
color: red                         # red | blue
area: web
class: insecure-deserialization
mirror: blue-insecure-deserialization
triggers: ["deserialization", "gadget chain", "ObjectInputStream", "pickle", "unmarshal"]
summary: "Surface insecure deserialization: sources, sinks, gadget discovery, impact."
```

Body sections (stable headings, the unit of budget slicing):

- Red: `## Where It Hides`, `## Detection` (how to confirm), `## Exploitation` (methodology, not weaponized payloads), `## Impact`, `## Remediation` (points to the blue mirror).
- Blue: `## Signals`, `## Detection Rules` (our own rule syntax or references), `## Hardening` (each step compilable to a check — see below), `## Incident Response`, `## References`.

## Verification Hook (what makes a playbook runnable)

A blue `## Hardening` step may declare a check:

```yaml
checks:
  - id: deser-no-native-java
    kind: guard            # guard | verify
    description: "No raw ObjectInputStream on untrusted input"
    detector: "grep"       # reuses an existing tb guard/verify detector
    args: ["-RnE", "new ObjectInputStream\\(", "--include=*.java"]
    severity: high
```

`tb blue apply <class>` registers the playbook's checks into the project's `tb guard`/`verify` config (ADR-008), so the hardening is enforced on every `tb stack`. This is the concrete difference from the inspiration libraries: the defense persists as a gate, not a paragraph.

## Index Schema (on top of ADR-003)

Built by `tb <color> index` into the graph DB:

| Column | Source |
|--------|--------|
| name, color, area, class | frontmatter |
| summary, triggers | frontmatter (FTS) |
| path | relative `PLAYBOOK.md` |
| sections | `##` headings + byte ranges (slicing) |
| checks | count + ids (blue only) |
| digest | sha256 (drift check) |

`play_edge(from, to, kind="mirror")` derives from the `mirror:` field, bidirectional at query time. Ranking reuses the graph's BM25 config.

## CLI Surface

`tb red` binds color `red`, `tb blue` binds `blue`:

```
tb <color> list [--area A] [--json]
tb <color> search QUERY [--area A] [--limit N] [--json]
tb <color> load NAME [--section "H" | --budget TOKENS] [--full] [--json]
tb <color> pair NAME                 # resolve the mirror in the other color
tb <color> index                     # (re)build the index
tb blue    apply CLASS [--dry-run]   # register the blue playbook's checks into tb guard/verify
tb red     explain CLASS             # red + blue mirror side by side (attack ⇄ defense)
```

- **search / list** — compact, one line per hit: `[red/web] red-sqli — <summary>`, bodies never printed.
- **load** — body under `secplay.load_budget` (default 2000 tokens); over budget returns frontmatter + section index + top sections that fit + `truncated: true`; `--section` returns one block; `--full` overrides.
- **pair / explain** — follow the mirror edge; `explain` renders attack and defense together.
- **blue apply** — writes checks into the project config; `--dry-run` previews.

## MCP Surface (ADR-002)

| Tool | Annotation | Returns |
|------|------------|---------|
| `tb_red_search` / `tb_blue_search` | readOnly | compact hits |
| `tb_red_load` / `tb_blue_load` | readOnly | budgeted body / named section |
| `tb_sec_pair` | readOnly | the mirror playbook |
| `tb_blue_apply` | **not** readOnly (`destructiveHint`, idempotent) | checks registered into guard/verify config |

Read tools are `readOnlyHint: true, openWorldHint: false`. `tb_blue_apply` mutates project config, so it is annotated accordingly and appears in `TestAllToolsAnnotated`. Over MCP, search/load never reindex: on `stale: true` the caller runs the matching `index` tool.

## Token-Budget Contract

1. `search`/`list` output is header-only, ≤ 1 line per result.
2. `load` never exceeds `budget` tokens unless `--full`/`full:true`.
3. Over-budget playbooks return frontmatter + section index + the top sections that fit (by query relevance, else document order) with `truncated: true`.

## Trust Model

- Playbook bodies are owned content, reviewed at author time; still, the engine treats a loaded body as text to display, never as commands to run (the egm untrusted-content guarantees, ADR-008, extend here).
- No `tb red`/`tb blue` path executes anything or opens a network connection. Any exploitation methodology is informational; actual execution is confined to `tb lab` under isolation.
- `tb red` prints the one-time authorized-use notice (recorded in `.toolbelt`); `tb blue` does not gate.

## Acceptance Criteria

- A1 — Every in-scope red playbook resolves a blue mirror and vice-versa (`tb red pair`, `tb blue pair`).
- A2 — `tb red search "deserialization"` returns `red-insecure-deserialization` compact; body not printed.
- A3 — `tb red load <big>` over budget returns `truncated: true`, section index, ≤ budget tokens; `--section`/`--full` behave per contract.
- A4 — `tb blue apply insecure-deserialization` registers the playbook's checks; a subsequent `tb guard`/`tb stack` runs them; `--dry-run` writes nothing.
- A5 — ≥ 80% of blue hardening steps across the corpus carry a runnable check.
- A6 — No CLI or MCP read path executes a command, opens a socket, or emits a user secret — enforced by test.
- A7 — Read MCP tools annotated `readOnlyHint:true, openWorldHint:false`; `tb_blue_apply` annotated destructive; all in `TestAllToolsAnnotated`.
- A8 — EN + `docs/pt-BR/` mirror present (`TestDocsTranslationMirrorComplete`).

## Open Questions

1. **Rule syntax for blue `## Detection Rules`.** Author our own compact rule format, or reference Sigma/YARA by example without depending on their toolchains?
2. **Check detectors.** Which `tb guard`/`verify` detector kinds to expose to playbook checks (grep-class now; AST/semgrep-class later)?
3. **Corpus location.** `playbooks/` at repo root vs under `docs/`? It is data consumed by `tb`, not prose, so likely a code-adjacent dir indexed at build.
4. **Shared token estimator.** Confirm the existing estimator is exported for `internal/secplay`, or factor it out.

## Implementation status (Phase 1)

Implemented in `internal/secplay` with CLI `tb red` / `tb blue` and the six MCP tools. Three design points were resolved against the codebase as built; they refine the sections above:

- **Index.** The repo "graph" is a JSON store (`graph.json`) with in-memory BM25, not SQLite/FTS5. The corpus is small, so Phase 1 ships a self-contained engine (embedded corpus, own compact ranker, own section slicer, own token estimator `≈ runes/4`) rather than writing into `graph.json`. Wiring playbook nodes + a `mirror` edge into the graph (so `tb graph path` links them) is **Phase 2**.
- **Runnable checks.** `guard` is hardcoded Go and `verify` is four command strings — neither hosts a list of named checks. So blue checks live in a dedicated, versioned **`.toolbelt/secchecks.yaml`** written by `tb blue apply` and run by **`tb blue check`** (a grep/regex scanner). Acceptance A4's "a subsequent `tb guard`/`tb stack` runs them" becomes `tb blue check` for Phase 1; a `tb stack` seccheck op is **Phase 2**.
- **Corpus location.** Authored and **embedded** at `internal/secplay/corpus/{red,blue}/<class>.md` via `go:embed` — owned, shipped in the binary, and outside `docs/` (no pt-BR mirror burden).

**Shipped corpus (core, 10 pairs):** sql-injection, xss, ssrf, ssti, xxe, idor, rce, insecure-deserialization, jwt, oauth. Blue checks ship for all but the two logic-only classes (idor, oauth). **Deferred:** SIEM rule formats (Sigma/KQL/YARA) and the rest of the taxonomy.

## Implementation status (Phase 2)

- **Graph mirror edge.** The embedded corpus now contributes to the code graph through a `graph.RegisterNodeSource` hook (`internal/secplay/graphsource.go`): each playbook is a `playbook:<name>` node (type `playbook`) and each red playbook emits a `mirror` edge to its blue counterpart. With `TB_GRAPH=on`, `tb graph path playbook:red-sqli playbook:blue-sqli` and `tb graph explain` link attack and defense. The corpus `.md` files are also indexed as ordinary docs; the node source adds the typed nodes and the mirror edges on top.
- **`tb stack seccheck`.** A new read-only, parallel stack op runs `secplay.Run` over the working tree (the checks registered by `tb blue apply`). It is regex-only — it never executes matched content — and fails the step on error-severity findings unless `--warn-only`.
