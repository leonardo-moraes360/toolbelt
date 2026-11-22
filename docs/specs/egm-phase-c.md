🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/egm-phase-c.md)

# `tb egm brainstorm`, `tb egm draft` and symbol-level drift — Phase C spec

Author: toolbelt · Status: **draft, round 1** (not reviewed; the items marked **USER DECISION** need an answer before the slice that depends on them starts) · Date: 2026-11-22 · Decision: [ADR-015](../adr/015-egm-nativo.md) (§8) · Depends on: [core](egm-nativo.md) (Phase A), [trust](egm-trust.md) (Phase A.2), [drift and bootstrap](egm-drift-bootstrap.md) (Phase B)

## Overview

Phase C closes the three items the README roadmap lists for `tb egm`: the two authoring commands of the upstream workflow ([ADR-007](../adr/007-inspiracao-elephant-goldfish.md): the elephant "brainstorms, writes concept briefs, and drafts specifications") and drift at the level of the code symbols a document cites, not only the files.

- **`tb egm brainstorm`** turns a brief into a **concept brief**: ideas, trade-offs, a recommended direction and open questions. It is an artifact for the elephant and the human, **not gated**, never reviewed by tb, and written outside the repository by default.
- **`tb egm draft`** writes the **first version of a spec or ADR** from a brief. The draft is only input to the existing review loop: it gets no state, no receipt and no verdict, and it reaches `PASS` only through `tb egm review` / `loop next` like any hand-written document.
- **Symbol-level drift** lets a document cite `internal/x/x.go#Name`. Review verifies the symbol exists, the receipt certifies the symbol's own bytes, and `drift-report` reports `symbol_changed` / `symbol_missing`, all from blobs, deterministically, with no LLM and no graph database.

Phase C ships in slices (see [Slices](#slices)); C1 is one pull request.

## Current state

- **Commands.** `tb egm` has `review`, `loop next`, `resolve`, `status`, `receipt` ([cmd/tb/egm.go:137](../../cmd/tb/egm.go)), `bootstrap` ([cmd/tb/egm_bootstrap.go:21](../../cmd/tb/egm_bootstrap.go)) and `drift-report` ([cmd/tb/egm_drift.go:23](../../cmd/tb/egm_drift.go)). No spec defines `draft` or `brainstorm`; [egm-nativo.md](egm-nativo.md) (Non-Goals) and [egm-drift-bootstrap.md](egm-drift-bootstrap.md) (Out of Phase B) defer them to Phase C.
- **Engines.** `auto`, `llm`, `deterministic` ([internal/egm/review.go:13-17](../../internal/egm/review.go)); `auto` falls back to `deterministic` without `egm.agent_cmd` ([review.go:214](../../internal/egm/review.go)).
- **Isolated generation already exists.** The B3 bootstrap goldfish ([internal/egm/bootstrapllm.go:28](../../internal/egm/bootstrapllm.go)) runs one tool-less agent in the sandboxed `Runner` ([internal/egm/run.go:114](../../internal/egm/run.go), [run.go:227](../../internal/egm/run.go)), cleans the body ([bootstrapllm.go:142](../../internal/egm/bootstrapllm.go)), injection-scans it and checks the R7 source set. The deterministic skeleton is [internal/egm/bootstrap.go:251](../../internal/egm/bootstrap.go). `draft` reuses both.
- **Personas.** Embedded in `internal/egm/personas/`; `drift` and `bootstrap` are kept off the reviewer list. `inventive-catalyst` is a reviewer persona (it looks for better alternatives inside a review), not a brainstorm generator.
- **Citations (R7).** Sources are Markdown links (the fragment is stripped) and backtick spans ending in a known extension, with no `:` ([internal/egm/sources.go:45-66](../../internal/egm/sources.go), [sources.go:127](../../internal/egm/sources.go)). The `refs` checker verifies that cited paths exist ([internal/egm/checker_refs.go](../../internal/egm/checker_refs.go)); nothing verifies a symbol.
- **Receipts.** `Receipt` has `claims`, `commit`, `doc`, `doc_sha256`, `sources`, `version` ([internal/egm/receipt.go:32-41](../../internal/egm/receipt.go)); versions 1 and 2, v2 only under `egm.receipt.write_v2` ([receipt.go:108](../../internal/egm/receipt.go)). Readers use `DisallowUnknownFields` ([internal/egm/ci.go:52](../../internal/egm/ci.go)), so a new field needs a new version. CI checks freshness by file hash only ([ci.go:110-119](../../internal/egm/ci.go)). State rounds keep `sources` as path → sha256 ([internal/egm/state.go:68](../../internal/egm/state.go)).
- **Drift is file-level.** `drift-report` compares baseline file hashes ([internal/egm/drift.go:308-355](../../internal/egm/drift.go)); statuses `fresh`, `no_baseline`, `sources_differ`, `drifted`, `source_missing`, `unreadable` ([drift.go:45](../../internal/egm/drift.go)). Any edit to a cited file, even far from what the document describes, is `drifted`.
- **The graph has no line ranges.** `Node` has no start/end line ([internal/graph/graph.go:49-57](../../internal/graph/graph.go)). Go function IDs are `func:<pkg>.<name>` ([internal/graph/index_go.go:52](../../internal/graph/index_go.go)): methods drop the receiver, so `(A) Name` and `(B) Name` in one package share an ID; `funcIDs` is keyed by bare name ([index_go.go:54](../../internal/graph/index_go.go)) and `findFunc` returns the first declaration with that name ([index_go.go:76](../../internal/graph/index_go.go), [index_go.go:106](../../internal/graph/index_go.go)), so the call edges of the second method are attributed to the first. Package IDs are `pkg:<name>` ([index_go.go:34](../../internal/graph/index_go.go)), so two packages named `main` share a node. `SearchHit.Lines` exists only for vector chunks ([internal/graph/search.go:38](../../internal/graph/search.go)). The graph is on demand and off by default ([ADR-011](../adr/011-grafo-sob-demanda.md)).

## Goals

1. An elephant (or a human with `egm.agent_cmd`) can go brief → concept brief → draft spec → review loop with tb commands, and every generated text stays untrusted data until a normal review converges.
2. A draft never approves itself: nothing tb writes for `draft` or `brainstorm` can count toward a verdict, a receipt or the CI gate.
3. A document can tie a claim to one Go symbol, and drift over that document reports the symbol, not the whole file.
4. Everything symbol-related is deterministic pure Go over git blobs: same commit, same tb build, same answer; no LLM, no network, no dependency on the graph being enabled.

## Non-goals

- tb editing an existing document (Phase B principle 2 stays): `draft --update` writes a proposal outside the tree.
- Auto-chaining `draft` into `review`, or opening review lineages from brainstorm output.
- Symbols for languages other than Go (the Python, Rust and JS indexers can follow the same contract later).
- MCP tools for the new commands (they take minutes; CLI-only, as `review` and `bootstrap`).
- The ADR-015 Phase C items deferred in [Scope](#scope-of-adr-015-phase-c-items).

## Principles

1. **tb decides, the LLM writes or judges.** tb collects, redacts, scans, assembles, hashes and records. A model call is an isolated goldfish-like process with no tools (Phase A isolation).
2. **Generated text is data.** It is cleaned, injection-scanned and source-checked before it touches disk; it carries a generated status line; tb never stages or commits it.
3. **No self-approval.** Generation writes no review state, no round, no receipt; the generator persona is refused as a reviewer and as the arbiter.
4. **Offline by default.** Without `egm.agent_cmd`, the LLM paths refuse with `agent_unavailable`; the deterministic paths (`draft --body`, symbol drift) never call an agent.
5. **Symbols are file-scoped.** A symbol is identified by the cited path plus a name inside that file, never by a global graph ID, so graph ID collisions cannot reach egm.

## Part 1: `tb egm brainstorm`

### Command

```bash
tb egm brainstorm --brief rd.md
tb egm brainstorm --brief rd.md --from internal/router/ --out notes/router-ideas.md
tb egm brainstorm --brief rd.md --json
```

**R1. Brief.** `--brief` is one file inside the repository: regular, no symlink in any component, not under `.git`, at most `max_source_bytes`. It may be untracked (an `rd.md` usually is). tb reads it, applies `Redact`, refuses a `secret_like` brief with `brief_secret`, and runs `ScanInjectionSource`; a match refuses with `brief_injection` before any agent runs. The brief is a DATA block, never part of the instructions.

**R2. Context.** `--from` (optional, repeatable) follows the bootstrap rules exactly (tracked files only, Phase A path rules, R7 limits, `secret_like` skipped and listed). Brainstorm has no `## Sources` wiring: the concept brief is not gated, so its citations are not certified.

**R3. Engine.** LLM only. One goldfish with the embedded `brainstorm` persona (new; original text) and the Phase A isolation and tool-use verification; one retry, as bootstrap. Without `egm.agent_cmd`: exit 2, `agent_unavailable`. With `tools_check` off: `tools_unverifiable`. The run counts against `egm.max_agent_runs`, `max_stdout_bytes` and `timeout_per_reviewer`.

**R4. Output contract.** A Markdown body of at most `egm.brainstorm.max_bytes` (default 16 KiB; a limit key, the repository config can only lower it) with these level-2 sections, in order: `## Problem`, `## Ideas` (each idea a level-3 heading with its trade-offs), `## Recommended direction`, `## Open questions`. tb applies the bootstrap cleaning (HTML comments until none is left, level-1 headings, `Status:` lines before the first `##`) and the injection scan on the body (`brainstorm_injection`); a missing section or an empty, oversized or non-UTF-8 body twice gives `brainstorm_invalid`.

**R5. Where it lands.** tb adds a title (from the brief's first heading, else its file name) and the status line `Status: **concept brief, generated by tb egm brainstorm** (not gated, not reviewed).`

- **Default:** `<state>/egm/<repo-id>/briefs/<slug>-<n>.md` (the Phase A state root, outside the repository, mode `0600`, `n` monotonic). Nothing in the tree changes, so no gate, mirror test or commit is involved.
- **`--out <path>`:** a new file inside the repository with the bootstrap target rules (`O_CREAT|O_EXCL`, parent exists, no symlink in any component, not under `.git`, not R7-denied). A path that matches `egm.doc_globs` is refused with `brainstorm_gated`: a concept brief is not a spec, and putting one where the gate looks would make tb's own output gated by accident.

`--json` prints `{"brief_sha256", "engine", "out", "v": "egm-brainstorm/0.1", "warnings"}`. tb prints the next step: `tb egm draft <doc> --brief <out>`.

## Part 2: `tb egm draft`

### Command

```bash
# Elephant-authored body (deterministic: tb only assembles and checks)
tb egm draft docs/specs/router.md --brief rd.md --from internal/router/ --body /tmp/body.md

# Isolated drafter goldfish
tb egm draft docs/specs/router.md --brief rd.md --from internal/router/ --engine llm

# Proposal for an existing gated document (slice C3)
tb egm draft docs/specs/router.md --update --brief rd.md --engine llm
```

**R6. Two ways to author, one assembly.** The elephant in ADR-007 holds the full context; tb cannot. So `draft` supports both:

- **`--body <file>` (deterministic, the default when given).** The elephant writes the body with its own context; tb assembles and checks it. No agent runs. This is the faithful mapping of the upstream `draft` skill.
- **`--engine llm`.** One isolated drafter goldfish (embedded `draft` persona, new) receives the derived standard, the required headings, the redacted brief and the accepted `--from` files as DATA blocks, and returns the body. Same isolation, retry, budgets and refusals as R3 (`agent_unavailable`, `tools_unverifiable`).

Giving both, or neither, exits 2 with `usage`. There is no deterministic body generator: `tb egm bootstrap` already writes the skeleton.

> **USER DECISION D1.** Keep both modes (recommended), or ship only one: `--body` only (no new LLM surface, smallest), or `--engine llm` only (works without an agent harness).

**R7. Target and type.** The bootstrap target rules: new file (`O_CREAT|O_EXCL`), matches `egm.doc_globs`, type derived from `egm.doc_types` (no `--type`). Refusals reuse the bootstrap codes renamed for draft: `draft_exists`, `draft_path`, `draft_sources`.

**R8. Brief.** As R1 (`brief_secret`, `brief_injection`). `--brief` may also name a concept brief in this repository's `<state>/egm/<repo-id>/briefs/`. The brief is **not cited** in the draft: an untracked brief would be an R7 skip and start the first review downgraded (`ci_partial` in CI). The draft must stand alone; the reviewers see only it and its sources.

**R9. Assembly.** tb builds the file as the bootstrap skeleton does (title, status line, required sections, `## Sources` of relative links to the accepted `--from` files), with the body in place of the `TODO` sections, and:

- the status line is `Status: **draft, generated by tb egm draft** (not reviewed).`;
- the body goes through the bootstrap cleaning, then the Phase A `injection-scan` (`draft_injection`);
- a body that omits a required section keeps that section with its `TODO` line (the review's `required-sections` checker then reports it);
- the R7 collection on the assembled file must give exactly the accepted `--from` files; any other citation, or any accepted file R7 would exclude, refuses with `draft_sources`;
- an empty, oversized (`egm.draft.max_bytes`, default 32 KiB, limit key) or non-UTF-8 body refuses with `draft_invalid`.

Nothing is written on a refusal.

**R10. No self-approval.**

- `draft` writes no state directory, round, lineage, convergence or receipt, and reads none.
- `draft` and `brainstorm` personas are refused by `--reviewers` and never loaded by the arbiter (as `drift` and `bootstrap`).
- `tb egm receipt` on a drafted document fails exactly as on any unreviewed one (no convergence); CI requires the full A.2 check for a new gated document anyway.
- The generated status line is a fact about the file, not a gate input. **USER DECISION D2:** should the `required-sections` (or a new `generated-status`) checker report a `minor` finding while a `generated by tb egm` status line is still present, so a converged document does not keep claiming "not reviewed"? Recommended: yes, `minor`, deterministic.

**R11. `--update` (slice C3).** For an existing gated document, tb never edits it. The drafter goldfish (or `--body`) produces a full new version, assembled and checked as R9 (the status line is not changed; the existing `## Sources` is kept and the new set must still be exactly what R7 resolves), and tb writes it to `<state>/egm/<repo-id>/<slug>/drafts/draft-<n>.md`, then prints a unified diff (tb's hardened `DiffNoIndex`). The elephant applies it; the next `tb egm review` sees a new `doc_sha256`, so earlier convergence is `STALE`, as for any edit.

**R12. Output.** Text prints the target, the accepted and skipped files, and the next step (`tb egm review <doc>`; in this repository also the pt-BR mirror). `--json`: `{"doc", "engine", "skipped", "sources", "v": "egm-draft/0.1", "warnings"}` (`proposal` and `diff` instead of `doc` for `--update`). tb never stages or commits.

## Part 3: symbol-level drift

### What a symbol is

**R13. Go symbols (pure extractor).** A new pure function in `internal/graph`, `GoSymbols(src []byte) ([]Symbol, error)`, parses one file's bytes with `go/parser` (no file system, `SkipObjectResolution`, comments kept for positions) and returns the top-level declarations:

| Kind | Name in the citation | Byte range hashed |
|---|---|---|
| `func` | `Name` | `func` keyword to the end of the body |
| `method` | `Recv.Name` (pointer and type parameters stripped: `(*T[K]) M` → `T.M`) | same |
| `type` | `Name` | the `TypeSpec` (its own line range; for a grouped `type (...)`, the spec only) |
| `const`, `var` | `Name` | the `ValueSpec` that declares it |

Each `Symbol` has `Name`, `Kind`, `StartLine`, `EndLine` (1-based, inclusive) and `SHA256` of the **raw bytes** of the range. The doc comment is outside the range (editing a comment is not drift). `_`, `init`, and any name declared twice in the file (a non-compiling file) are **ambiguous**. A parse error makes every symbol of that file unavailable (`unparsed`). Files over `max_source_bytes` are not parsed.

> **RESOLVED D3 (user, 2026-11-22): hash the raw bytes.** Hash raw bytes (recommended: stable across tb builds; a pure `gofmt` reflow counts as a change) or `go/printer` output (ignores layout, but its output can change between Go versions, so two tb builds could disagree).

### Citing a symbol

**R14. Syntax.** A citation of a `.go` file may carry a symbol fragment:

- link: `[Drift](../../internal/egm/drift.go#Drift)`, `[ExitCode](../../internal/egm/drift.go#DriftReport.ExitCode)`;
- span: `` `internal/egm/drift.go#Drift` ``.

The fragment must match `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$` and must not be a line anchor (`^L[0-9]+(-L[0-9]+)?$`, kept as today: stripped, file-level). Any other fragment is stripped as today. The file itself stays an R7 source (the path rules, limits and packet are unchanged). The span form is new for R7: today a span ending in `#Name` is not a candidate ([sources.go:59](../../internal/egm/sources.go)); after C4 its path becomes one, so a document that already had such a span gains a source (`sources_differ` once). No such span exists in this repository's docs; the CHANGELOG states it.

**R15. Symbol-scoped source.** A source path cited **only** through symbol citations is *symbol-scoped*; a path also cited plainly (a link or span with no symbol fragment) is *file-scoped*, as today.

### Review

**R16. Verification (`refs` checker).** For each symbol citation whose file resolves, tb runs `GoSymbols` on the snapshot content: a missing name gives a `refs` finding at `major` ("symbol `X` not found in `path`"), an ambiguous one `symbol_ambiguous` at `major`, an unparsed file one `nit` finding per file. Deterministic; it runs with `--engine deterministic` too. This is ADR-015's "symbol verification in the graph", done on the snapshot instead of the graph index, so it works with the graph off and cannot read a stale index.

**R17. Round state.** Each round records `symbols` (`"path#Name"` → sha256) beside `sources` ([state.go:68](../../internal/egm/state.go)) for every resolved, unambiguous cited symbol. Rounds without the field are read as "no symbol baseline".

**R18. Receipt version 3.** v3 is v2 plus `symbols` (a map, possibly empty, same key format; keys sorted by the existing deterministic encoding). Readers accept 1, 2 and 3; tb writes v3 only with the user-only key `egm.receipt.write_v3: true` (which implies the v2 `commit`), so a CI pinned to an older tb is upgraded first, as the B3 rollout. A v3 receipt whose `symbols` keys are not exactly the document's resolved symbol citations, or whose values are not 64-hex, is `receipt_invalid`.

**R19. CI freshness.** `fresh` ([ci.go:110](../../internal/egm/ci.go)) also recomputes every v3 symbol hash at head; a mismatch is the same failure as a source hash mismatch. Receipt edits that only rewrite `symbols` select the document (B1 check 1 already covers any receipt edit).

### Drift

**R20. Comparison.** For a baseline (receipt v3, or a state round with `symbols`):

- a **file-scoped** source is compared by file hash, exactly as Phase B;
- a **symbol-scoped** source is compared by its symbols: each baseline symbol is extracted at `--at` and its hash compared. A file whose hash changed while every cited symbol is unchanged is **not** drift: the document stays `fresh` (if nothing else changed) and the path is listed in `outside_symbols` (informative, exit 0);
- with a v1/v2 receipt or a round without `symbols`, every source is file-scoped (no change in behavior).

**R21. New statuses and fields.**

| Status | Meaning | Exit |
|---|---|---|
| `symbol_changed` | a cited symbol's hash differs (`symbols_changed`: `{"path", "symbol"}`) | 1 |
| `symbol_missing` | a cited symbol no longer resolves (`symbols_missing`: `{"path", "symbol", "reason"}`, reason `deleted`, `ambiguous`, `unparsed`, `too_large`) | 1 |

A file-level `source_missing` of a symbol-scoped path stays `source_missing`. Severity order: `unreadable` > `source_missing` > `symbol_missing` > `drifted` > `symbol_changed` > `sources_differ` > `fresh` > `no_baseline`. JSON adds `symbols_changed`, `symbols_missing`, `outside_symbols` (always present, possibly empty) and bumps `v` to `egm-source-drift/0.2`.

> **RESOLVED D4 (user, 2026-11-22): a change outside the cited symbols is informational (`outside_symbols`), not drift.** R20 makes drift *less* conservative for symbol-scoped sources: a change in a helper the cited symbol calls is not reported. Recommended: accept (citing the file plainly restores file-level drift, and `outside_symbols` keeps the change visible). Alternative: report symbol statuses only as extra detail and keep `drifted` for any file change.

**R22. CI and local gate.** `ci_source_drift` (B1) and the local `sources_changed` warning use R20, so they raise on `symbol_changed` / `symbol_missing` and not on `outside_symbols`. The `drift` goldfish (`--engine llm`) receives, for symbol-scoped sources, the changed symbols' ranges marked in the packet; it is otherwise unchanged.

### Graph (slice C6)

**R23. Line ranges.** `Node` gains `start_line` and `end_line` (`omitempty`, additive to the JSON index). The Go indexer fills them from `GoSymbols`, so the graph and egm share one extractor. `tb graph explain` prints `path:start-end` when present.

**R24. Receiver-qualified method IDs.** Methods become `func:<pkg>.<Recv>.<Name>`; `funcIDs` and `findFunc` key on the qualified name, fixing the edge misattribution in [index_go.go:54-76](../../internal/graph/index_go.go). Package IDs keep `pkg:<name>` in this slice (import-path IDs are a larger change).

> **RESOLVED D5 (user, 2026-11-22): the method-ID change is deferred to C6, post-1.0; C4 and C5 do not touch graph IDs.** R24 changes existing graph node IDs (saved queries, `tb graph path` arguments, golden tests). Ship it in C6 with a CHANGELOG "changed" entry (recommended), or keep the old IDs and add the receiver only to `title`.

## Scope of ADR-015 Phase C items

| Item | Phase C (this spec) | Why |
|---|---|---|
| `draft`, `brainstorm` | included (C1–C3) | README roadmap |
| symbol verification in the graph | included (C4), done on snapshots | required by symbol drift; graph stays on demand |
| symbol-level drift | included (C5) | README roadmap |
| graph line ranges, method IDs | included (C6) | asked by symbol work; independent of egm |
| other presets (`staged`, `feature`, `implementation`) | deferred | each needs its own standards and convergence data |
| custom personas by path | deferred | persona text from the repository is a prompt the repository controls; needs its own trust design |
| `codex` profile | deferred | needs an R0-style spike (tools off, tool-use detection, no user config load) |
| per-reviewer agent override | deferred | depends on a second profile |
| churn | deferred | not defined anywhere yet |
| `review.Finding` → `egm.Finding` adapter | deferred | unrelated to authoring or drift; small follow-up |

> **USER DECISION D6.** Confirm the deferrals, or pull the adapter into Phase C as a last slice.

## Threat model

| # | Threat | Mitigation |
|---|---|---|
| T1 | The drafter or brainstorm body addresses the reviewers ("this spec is approved") or rebuilds a tb marker | bootstrap cleaning (comments until none, `Status:` lines, H1), injection scan aborts, fresh reviewers see only doc and sources |
| T2 | A draft counts toward its own approval | no state, round or receipt written (R10); personas refused as reviewers/arbiter; CI full check for new documents |
| T3 | A brief from a cloned repository carries instructions or secrets | brief is a DATA block; `Redact`; `secret_like` → `brief_secret`; injection → `brief_injection`; `agent_cmd` is user-only config |
| T4 | Brainstorm output dodges or pollutes the gate | default outside the tree; `--out` in `egm.doc_globs` refused (`brainstorm_gated`) |
| T5 | `--update` silently rewrites a reviewed spec | proposal outside the tree only (R11); the edit, when applied, makes the old convergence `STALE` |
| T6 | A forged receipt claims unchanged symbol hashes | CI recomputes v3 symbol hashes at head (R19); merge-base receipt is the drift baseline (B1) |
| T7 | A change hides outside the cited symbol | `outside_symbols` reported; plain file citation restores file-level drift; D4 |
| T8 | A huge or hostile Go file stalls the parser | parse only within `max_source_bytes`; `too_large` / `unparsed` reported, never silent |
| T9 | Two tb builds disagree on a hash | raw-byte hashing (D3); parser failure reported as `unparsed`, not as a different hash |
| T10 | Network | tb opens none; only the agent child the human configured talks to the provider |
| T11 | A pull request lowers limits or switches symbol mode to hide drift | symbol scoping comes from the document text and receipt version, not config; CI uses merge-base settings (B1) |

## Acceptance criteria

### C1 brainstorm

1. With a brief and `egm.agent_cmd`, `brainstorm` writes one concept brief under `<state>/egm/<repo-id>/briefs/`, mode `0600`, with the four sections and the generated status line; the repository tree is unchanged.
2. `--out` inside `egm.doc_globs` exits 2 `brainstorm_gated`; an existing `--out` exits 2; nothing is written.
3. A brief with a secret-like value exits 2 `brief_secret`; one that trips `ScanInjectionSource` exits 2 `brief_injection`; no agent runs.
4. Without `egm.agent_cmd`: `agent_unavailable`; with `tools_check` off: `tools_unverifiable`.
5. A body with an HTML comment, an H1 or a `Status:` line is cleaned; one that trips the injection scan writes nothing; a body missing a section twice gives `brainstorm_invalid`.
6. `--reviewers brainstorm` is refused.

### C2 draft (new document)

7. `draft --body` writes a document whose R7 sources are exactly the accepted `--from` files, with the draft status line and every required section; no agent runs.
8. `draft --engine llm` does the same from the goldfish body; `--body` with `--engine llm`, or neither, exits 2 `usage`.
9. A body citing a file outside `--from` exits 2 `draft_sources`; one that trips the injection scan exits 2 `draft_injection`; nothing is written.
10. After `draft`, `tb egm status` reports no rounds, `tb egm receipt` fails, and no file exists in the document's state directory.
11. An untracked brief is not cited in the draft.
12. tb never stages or commits the draft.

### C3 draft `--update`

13. `--update` leaves the document byte-identical, writes a proposal under the state directory and prints a diff; a proposal whose sources differ from what R7 resolves exits 2 `draft_sources`.

### C4 symbol citations and verification

14. `GoSymbols` returns funcs, methods (`T.M` for `(*T[K]) M`), types, consts and vars with 1-based inclusive ranges; `init`, `_` and duplicates are ambiguous; a parse error gives `unparsed`.
15. A document citing `x.go#Missing` gets a `major` `refs` finding under `--engine deterministic`; `x.go#L10` stays a file-level citation with no symbol finding.
16. Round state records `symbols` for resolved citations.

### C5 receipt v3 and symbol drift

17. With `egm.receipt.write_v3: true`, the receipt is v3 with `symbols`; readers accept v1–v3; v3 with a wrong key set is `receipt_invalid`.
18. In CI, a range that edits the body of a cited symbol of a receipted, untouched document raises `ci_source_drift`; a range that edits only another function in the same symbol-scoped file raises nothing and `drift-report` lists the path in `outside_symbols`, status `fresh`.
19. Deleting or renaming a cited symbol gives `symbol_missing` (`deleted`); a doc-comment-only edit gives `fresh`.
20. A file cited both plainly and with a symbol stays file-scoped: any edit is `drifted`.
21. A v1/v2 receipt gives exactly the Phase B result for the same tree.
22. `drift-report --json` has `v: egm-source-drift/0.2` and the three new arrays on every document.
23. Two runs at the same commit give byte-identical JSON.

### C6 graph

24. Go function, method and type nodes carry `start_line`/`end_line`; two methods `Name` on different receivers in one package are two nodes with their own call edges.

## Slices

| Slice | Content | PR |
|---|---|---|
| **C1** | `tb egm brainstorm` (R1–R5), `brainstorm` persona, `egm.brainstorm.max_bytes`, docs | one PR |
| C2 | `tb egm draft` new document (R6–R10, R12), `draft` persona, `egm.draft.max_bytes`; D2 checker if accepted | one PR |
| C3 | `draft --update` (R11) | one PR |
| C4 | `graph.GoSymbols`, symbol citations (R13–R17), `refs` verification | one PR |
| C5 | receipt v3, CI freshness, drift statuses (R18–R22), `docs/egm-ci.md` | one PR |
| C6 | graph line ranges and method IDs (R23–R24) | one PR, independent of C1–C5 |

C1 and C4 have no dependency on each other and can run in parallel; C5 needs C4; C3 needs C2.

## Docs to update with the implementation

ADR-015 status (§8 Phase C), [egm-nativo.md](egm-nativo.md) Non-Goals, [egm-drift-bootstrap.md](egm-drift-bootstrap.md) "Out of Phase B" (link here), `docs/cli.md` egm section (and graph section for C6), [docs/egm-ci.md](../egm-ci.md) (C5: v3 receipts, symbol drift), README roadmap, CHANGELOG (C4: the new span form; C6: changed graph IDs), and the pt-BR mirror of each.

## Open questions

1. **ADR-015 status vs R0 item 8.** [ADR-015](../adr/015-egm-nativo.md) line 7 is `accepted` and also says R0 item 8 is still open; [egm-nativo.md](egm-nativo.md) line 124 says item 8 "still blocks ADR-015 from `accepted`". **RESOLVED D7 (2026-11-22, the recommended option):** amend egm-nativo R0 to say item 8 is a known open gap that no longer blocks acceptance (recommended), or move ADR-015 back to `proposed`. Phase C does not depend on item 8.
2. **egm-trust status.** [egm-trust.md](egm-trust.md) says "Draft, round 2 … slices 1–3 done; the flakiness measurement is pending", ADR-015 says the trust spec "stays a draft", and the README says Phase A.2 "is implemented". Proposal: status "implemented (slices 1–3); flakiness measurement pending", and the same in ADR-015 and README.
3. **`--ticket`.** Should `brainstorm`/`draft` read a `.toolbelt/tickets/TKT-NNN.yaml` directly, or is `--brief rd.md` enough? Not in C1.
4. **Several briefs or ideation rounds.** One goldfish per brainstorm keeps the budget bounded; a multi-persona brainstorm (diverge, then consolidate) can come later if one run proves too narrow.
5. **pt-BR for drafts.** In this repository every `docs/**/*.md` needs a pt-BR twin (`TestDocsTranslationMirrorComplete`). `draft` writes English only; should it print the mirror path as a reminder (proposed) or also draft the translation?
