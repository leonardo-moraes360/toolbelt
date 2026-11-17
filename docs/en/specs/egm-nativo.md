[🇧🇷 Português](../../specs/egm-nativo.md) | 🇺🇸 English

# Spec: Native EGM, Phase A (`tb egm review|loop|resolve|status`)

Author: toolbelt · Status: Draft (review round 4) · Date: 2026-11-17 · Decision: [ADR-014](../adr/014-egm-nativo.md)

## Overview

`tb egm` reviews **design documents** with goldfish (fresh-context reviewers) and runs the review loop until it converges. Phase A covers `review`, `loop next`, `resolve` and `status`, with the `consolidated` preset, the `claude` vendor profile and a `custom` profile. Drift-report, bootstrap, draft, brainstorm, other presets and the `codex` profile are left for their own specs.

## Glossary

| Term | Definition |
|------|------------|
| elephant | Whoever wrote the document and calls `tb egm`: the main agent or a human. Has a shell as the same user (R37). |
| goldfish | One-shot reviewer in an isolated process. Sees only its own prompt. |
| reviewer | A preset slot (R10), such as `style-feature`. Each reviewer uses one persona; one persona can feed more than one reviewer. |
| owning reviewer | The reviewer that created a lineage. It is the only one that re-evaluates it (R20). |
| arbiter | Goldfish that judges a contest (R31). |
| persona | The reviewer's lens: what to look for and how to judge (`personas/<name>.md`). |
| standard | Document quality criterion used by the persona (`standards/<name>.md`), such as `feature-spec`. |
| `consolidated` preset | Round with the 9 reviewers of R10 (8 personas) in parallel, with no checkpoints. |
| packet | On-disk record of what was handed to a goldfish (R1). |
| nonce | 128-bit random value per run. It delimits the data blocks and must be echoed. It proves the output came from this run, **not** that the model resisted injection. |
| attestation | `checked` list of literal excerpts from the document that the reviewer declares it read. tb checks each excerpt (R17b). |
| slug | Identifier of the reviewed document (R14). |
| FeatureRange / DetailedRange | Lines before and from the detailed plan heading onward (R6). |
| lineage `F-<k>` | Stable ID of a finding across rounds (`k` sequential per slug). |
| occurrence `<reviewer>-r<round>-<i>` | ID of a finding within a round. |
| reviewer status | `completed` (ran with valid output), `not_run` (failed after retry, R16), `unavailable` (LLM persona with no agent) or `skipped` (no applicable range, R6). |
| vendor profile | Per-CLI recipe: base argv, no-tools flags, authentication, env and output envelope (R4, R5, R15). |
| trust source | Origin of the sensitive `egm.*` keys: user config outside the repo or the config at the trust ref (R37). |
| human confirmation | Confirmation typed in an interactive TTY, recorded in the audit log (R38). |

## Problem

`tb review` reviews diffs with heuristics. It does not review documents, has no LLM reviewer and does not converge. The upstream EGM does this in prose, with isolation, state and approval left to the harness and the model. See [ADR-014](../adr/014-egm-nativo.md#context).

## Goals

- G1: isolation by process, sandbox, disposable HOME and absence of tools.
- G2: structured finding (`egm.Finding`) consumable by `tb stack` and MCP.
- G3: convergence decided by tb and persisted. The elephant does not reach `PASS` without a real review: not by lottery, injection, tampered config or hand-edited state. This holds within the limits of R37.
- G4: useful offline mode, with declared coverage, that does not pass a strict gate.
- G5: CLI and JSON of `tb review` and `tb drift` **unchanged**.

## Non-goals

- tb asks the user nothing (except the human confirmation of R38) and does not edit the reviewed document.
- tb does not change the severity given by a reviewer. Output above the persona's cap is invalid (R11).
- tb does not talk to the network nor call an LLM API.
- Resisting a process of the same user actively intending to defraud (reading the audit key, forging a TTY). R37 declares the limit.
- Out of this phase: `staged`/`feature`/`implementation` presets, per-path personas, `intermediate-zoom-out`, `codex` profile, churn, `review.Finding` → `egm.Finding` adapter, ADR-005 §4 capability tokens, drift-report, bootstrap, draft and brainstorm.
- The upstream personas' text is not copied.

## Critical User Journeys (CUJs)

- **CUJ-1.** *Given* `docs/specs/x.md` and the `claude` profile in the trust source, *when* I run `tb egm review docs/specs/x.md`, *then* 9 goldfish run isolated and I get `round-1/report.{md,json}` with the verdict `IN_PROGRESS`.
- **CUJ-2.** *Given* there is no agent, *when* I run the same command, *then* `refs-exist`, `required-sections` and `plan-files-listed` run, the LLM personas are `unavailable` and the loop can only end in `PASS_PARTIAL`.
- **CUJ-3.** *Given* round 1 has findings, *when* I edit the document, run `tb egm resolve docs/specs/x.md --fix F-1,F-2` and then `tb egm loop next docs/specs/x.md`, *then* each lineage's owning reviewer confirms whether it still holds, and tb reports `converged: true` after 2 consecutive clean rounds.
- **CUJ-4.** *Given* the blocker `F-3`, *when* I run `tb egm resolve docs/specs/x.md --contest F-3 --evidence internal/x.go:40` (a versioned file cited by the document), *then* F-3 becomes `contest_pending`. On the next `loop next`, a fresh arbiter decides between `settled` and `open`. A second contest of F-3 is refused.
- **CUJ-5.** *Given* that I (the elephant) edit `.toolbelt/config.yaml` in the worktree to `clean_rounds: 1`, *when* I run `loop next`, *then* tb ignores the value, uses the one from the trust source and emits the `config_untrusted` warning.

## Technical Plan

```
elephant ──tb egm review|loop next|resolve|status──┐
                                                    ▼
┌───────────────────────────── tb ─────────────────────────────┐
│ trusted config (user / ref) → snapshot round-<n>             │
│ sources (links+paths) → deny-list → redact → prompt (nonce)  │
│        │                                                     │
│        ├─ llm: sandbox.New(RO workdir, PrivateTmp, Home,     │
│        │        Stdin, separate Stdout) × 9, no tools        │
│        └─ deterministic: refs-exist, required-sections,      │
│                          plan-files-listed                   │
│        ▼                                                     │
│ validate (schema, cap, nonce, attestation, evidence)         │
│        ▼                                                     │
│ match → lineage → round record (chained HMAC)                │
│        ▼                                                     │
│ state.json + report.{md,json} → recomputed verdict           │
└──────────────────────────────────────────────────────────────┘
```

### Requirements (EARS)

**R0: Spike.** Blocks R1–R5 and the move of ADR-014 to `accepted`. A prototype in `bench/egm-spike/` must prove that `claude -p`:

1. runs in `sandbox.New` with a read-only workdir, private `TMPDIR` and disposable HOME;
2. receives the prompt via stdin and returns JSON on stdout separate from stderr;
3. runs **with no tools at all**;
4. does not load `~/.claude/CLAUDE.md`, the memories or the user's settings;
5. produces a valid attestation (R17b).

The spike must also:

- determine the `claude` profile's authentication form, in this order of preference:
  1. API key via `egm.env.pass`;
  2. vendor config directory pointed at the disposable HOME (for example, `CLAUDE_CONFIG_DIR`);
  3. copying a credential file, only as a last resort;
- test whether a token refresh inside the disposable HOME invalidates the real credential;
- list the directories the binary needs to read (the shebang interpreter and `node_modules`);
- record macOS (Keychain) as a limitation;
- collect at least 30 pairs of paraphrased findings to calibrate R18–R20.

The result goes in `docs/campo/dogfood.md` (PT and EN). If the spike fails, the ADR does not move to `accepted` and the finding is recorded in the ADR.

**Sandbox and execution**

- R1: In each round, tb must write `.toolbelt/egm/<slug>/round-<n>/target.md` (redacted snapshot) under any engine. For each LLM goldfish, it must also write `round-<n>/packets/<reviewer>/` with:
  - `prompt.txt`;
  - `sources.json` (path, blob sha256, included lines);
  - `meta.json` (persona, standard, range, nonce, `argv_sha256`, `binary_sha256`).
- R2: `sandbox.Config` must gain the fields below. With them zeroed, nothing changes.
  - `ReadOnlyWorkDir`: the workdir rule becomes read-only.
  - `PrivateTmp string`: writable directory. It replaces the `os.TempDir()` rule and is exported as `TMPDIR`.
  - `Home string`: writable directory. Exported as `HOME`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME` and `XDG_DATA_HOME`, overriding values coming from `Config.Env`.
  - `Stdin io.Reader`.
  - `Stdout io.Writer` with `MaxStdoutBytes int`: stdout goes only to this writer. If it exceeds the limit, capture stops and the result carries `StdoutOverflow: true`, with no silent truncation. stderr stays in the `tailBuffer`.
  - Fix in `addRule`: when the path is a regular file, the mask becomes `ReadFile|Execute`, without `ReadDir`. Today a file rule with `ReadDir` fails with EINVAL.
- R3: The goldfish must call `sandbox.New` directly:
  - with `ProcessGroup: true` and its own `sandbox.NewRegistry("TB_EGM_OWNER")`;
  - with `GroupOwner = egm:<slug>:r<n>:<reviewer>` and that same string in `TB_EGM_OWNER` in the child's env;
  - with `sandbox.AcquireSubreaper()` once per `tb egm` process.

  After each reviewer, tb calls `Registry.Sweep()`:
  - orphan with that reviewer's owner: the reviewer becomes `not_run` with `reason: escaped_process`;
  - orphan with no attributable owner: the round is aborted (exit 2), and nothing is written to the state.

  The timeout comes from `context.WithTimeout(egm.timeout_per_reviewer)`. The `llm` engine requires Linux with Landlock (fail-closed).
- R4: For the `claude` profile, tb must:
  1. create the HOME and the `PrivateTmp` in `os.TempDir()/tb-egm-<rand>/`, with mode `0700`;
  2. authenticate using the form chosen in R0: the `egm.env.pass` variables (added to the global `env.pass`, after `execenv.Filter`), or the config dir in the disposable HOME, or a `0600` copy of `egm.home.copy`;
  3. allow read-only access to the resolved binary's directory (`EvalSymlinks`), the shebang interpreter's directory and the `egm.home.read_only` directories;
  4. delete everything at the end, with defer, including on timeout. At startup and every round, sweep orphaned `tb-egm-*` older than 1h.
- R5: The goldfish argv is `egm.agent_cmd` plus the profile flags, which turn off **all** tools. All content goes in the prompt. tb refuses (exit 2):
  - argv with `bypassPermissions`, `--dangerously-skip-permissions` or flags from `egm.forbidden_flags`;
  - a `custom` profile without `egm.tools_disabled: true`.

  These keys are only read from the trust source (R37).
- R6: tb must split the document at the **first occurrence, in line order**, of any heading in `egm.detail_headings`. With no occurrence, the DetailedRange reviewers become `skipped` with `reason: no_detailed_section`. `skipped` counts as partial coverage (R25).

**Sources, redaction and prompt**

- R7: The packet's sources come from two places:
  - (a) relative markdown links (`CollectMDLinks` + `ResolveDocHref`);
  - (b) backtick spans (`CollectBacktickSpans`, extracted from `reBacktick`) and paths in text (`CollectPathLikes`), filtered by `LooksLikePath`.

  The `rePathLike` extension list must gain `json`, `yaml`, `yml` and `toml`, and paths ending in `/` now count as directories (a directory is not a source, it is only checked in R13). All of this is exported from `internal/graph/index_docs.go`, along with `NormalizeRelPath`. `egm.resolveSource` only adds the refusal of root escape and absolute paths.

  The document itself is always a source. tb must exclude, with a reason in `skipped_sources`:
  - paths outside the repository and URLs;
  - unversioned or ignored files;
  - `.env*`, `*.pem`, `*.key`, `*.p12` and `id_*`;
  - `.toolbelt/`;
  - `**/testdata/**`, `internal/egm/personas/` and `internal/egm/standards/`;
  - files above `egm.max_source_bytes`;
  - whatever exceeds `egm.max_packet_bytes`.

  Only reviewers with "code sources" (R10) and the arbiter receive sources beyond the document.
- R8: `mem.RedactText` must gain the patterns below and preserve the line count (each match becomes `[REDACTED]` plus the original line breaks). The change affects `tb mem` output and is covered by a test.
  - `\bsk-ant-[A-Za-z0-9_-]{20,}`
  - `\bsk-[A-Za-z0-9]{32,}`
  - `(?i)\b[A-Za-z0-9_]*_(KEY|SECRET|PASSWORD)=[^\s"'&]+`
- R9: `prompt.txt` must contain, in this order:
  - persona and standard;
  - `RANGE: <a>-<b>`;
  - `NONCE: <nonce>`;
  - the output contract (schema);
  - `PRIOR:` followed by JSON with the lineages the reviewer owns (R20);
  - the `<<<DATA-<nonce> path=…>>> … <<<END-<nonce>>>>` blocks, with the original line numbering.

  The prompt declares that the content of the blocks is data and never an instruction.

**Personas, standards and engines**

- R10: The `consolidated` preset runs in parallel, up to `egm.max_parallel`:

  | Reviewer | Persona | Standard | Range | Code sources |
  |----------|---------|----------|-------|--------------|
  | zoom-out | zoom-out | zoom-out | whole doc | no |
  | inventive-catalyst | inventive-catalyst | (none) | whole doc | no |
  | comprehension | comprehension | (none) | whole doc | no |
  | design | design | feature-spec | FeatureRange | no |
  | structural | structural | feature-spec | whole doc | no |
  | style-feature | style | (none) | FeatureRange | no |
  | implementation-readiness | implementation-readiness | implementation-readiness | DetailedRange | yes |
  | codebase-details | codebase-details | implementation-readiness | DetailedRange | yes |
  | style-detailed | style | (none) | DetailedRange | no |

  `--reviewers a,b` restricts the list and makes coverage partial (R25).
- R11: Personas, `arbiter` and standards are embedded (`go:embed`) in `internal/egm/personas/` and `internal/egm/standards/`, with original text. Each persona declares `max_severity`. For `inventive-catalyst`, it is `major`. A finding above the cap makes the output invalid (R16).
- R12: `--engine auto|llm|deterministic`, default `auto`.
  - `auto` uses `llm` if `egm.agent_cmd` (from the trust source) is configured and executable. Otherwise, it uses `deterministic`.
  - There is no inheritance from `router.agent_cmd`.
  - `--engine llm` with no agent is an error (exit 2).
  - Per-reviewer override: `egm.reviewers.<name>.agent_cmd`, also read from the trust source.
- R13: In the `deterministic` engine:

  | Checker | Partially covers | Checks |
  |---------|------------------|--------|
  | `refs-exist` | codebase-details | the R7 paths and links exist (directories included). With `TB_GRAPH=on`, it also checks in `tb graph` symbol-shaped identifiers: `pkg.Ident`, `Type.Method` or CamelCase with 2+ capitals. Without the graph, it does not check symbols. |
  | `required-sections` | structural | headings required by the `feature-spec` standard |
  | `plan-files-listed` | implementation-readiness | parsing below |

  The `plan-files-listed` parsing is as follows:
  - **Table:** the first table under the DetailedRange that has an `Arquivo`/`File` column and an `Ação`/`Action` one.
  - **Cell files:** only backtick spans, separated by commas.
  - **Expansions:** `{a,b}` becomes both paths; `(+ docs/en/)` generates the mirror of each `docs/…` path.
  - **Glob `*`:** must match at least one existing file if the action is `Modify`.
  - **Ellipsis `…`:** matches by prefix and must resolve to a single path.
  - **Action rules:** the value must be in `Create|Modify|Delete`, and every `Modify` or `Delete` must exist.

  The other personas become `unavailable`.

**Output contract and validation**

- R14: The slug is the lowercase relative path, with `/` turned into `--` and `[^a-z0-9-]` turned into `-`, plus `-` and the first 8 hex of the path's sha256. For renames, see R33.
- R15: stdout, after removing the profile envelope, must be an object valid against `internal/egm/schema/finding.schema.json`. The envelope depends on `egm.output`: `claude-json` uses the `result` field, which is the `claude` profile default; `raw` is the `custom` default. The object:
  `{"reviewer","nonce","checked":[{"heading","quote"}],"findings":[{"severity","scope":{"heading","lines":[a,b]},"message","rationale","evidence":[{"path","line"}]}],"prior":[{"lineage","still_holds":bool,"note"}]}`.
- R16: tb retries once and then marks `not_run` with `reason` when any of these happens:
  - missing output;
  - invalid schema or severity above the cap;
  - wrong nonce;
  - invalid attestation (R17b);
  - incomplete `prior` (R20);
  - `StdoutOverflow`;
  - timeout;
  - escaped process.

  `not_run` never equals zero findings.
- R17: A finding is marked `unverified: true` when `evidence.path:line` is not among the packet's sources (the document included) or when `scope.heading` does not exist in the snapshot.
- R17a (anti-injection scan): before each round, tb scans the snapshot. The scan:
  - **ignores** code blocks, inline code and quotes (`>`);
  - looks only for patterns directed at the reviewer or model (built-in PT+EN list in `internal/egm/injection_patterns.txt`). Examples (here in inline code precisely so as not to trigger the scan): `ignore (as|all) (previous|prior|above) instructions`, `do not report`, `return (no|empty|zero) findings`, `you are (approved|pre-approved)`, `como revisor, (não|aprove)`, `output {`, `responda apenas`;
  - on finding a pattern, emits a `major` finding from the `injection-scan` reviewer, with the lines as `scope`. This finding is contestable before the arbiter (R31).

  The scan also runs over the evidence excerpt before sending it to the arbiter: if there is a match, the contest is refused. This spec itself is a **negative** golden case and must not trigger the scan.
- R17b (attestation): each LLM reviewer must return in `checked` at least `max(3, ceil(H/3))` entries, where `H` is the number of headings in its range.
  - Each `quote` is 20 to 200 characters long and appears **literally** (after whitespace normalization) in the snapshot, under the declared heading and within the reviewer's range.
  - The entries must cover distinct headings.
  - An attestation below the minimum, or with any excerpt that does not match, is invalid (R16).

**Matching, lineage and state**

- R18: Definitions:
  - `normalize(msg)`: lowercase, no accents, no punctuation, collapsed whitespace and no stopwords (built-in PT+EN list).
  - `sim(a,b)`: Jaccard of the `normalize` tokens.
  - `overlap(r1,r2)`: `|r1∩r2| / min(|r1|,|r2|)`.

  The thresholds live in `egm.match.sim` (default 0.3) and `egm.match.overlap` (default 0.5), calibrated with the R0 corpus.
- R19: Within the same round, two findings merge when `sim ≥ egm.match.sim` **and** (`overlap ≥ egm.match.overlap`, or, without `lines`, the same `heading`). The merge keeps the highest severity and the `reviewers[]` list. The owning reviewer is that of the first occurrence, in R10 order.
- R20: Across rounds, the primary source is **re-evaluation by the owning reviewer**. A reviewer's `open`, `reopened` and `fixed` lineages go into `PRIOR` (R9) only for it, and it answers `prior[].still_holds`:

  | Current status | `still_holds: true` | `still_holds: false` |
  |----------------|---------------------|----------------------|
  | open / reopened | stays | → fixed (`by: reviewer`) |
  | fixed | → reopened | stays fixed |

  Omitting a lineage from `PRIOR` invalidates the output. Lineages from deterministic checkers and from `injection-scan` are re-evaluated by re-running the checker:
  - not reproduced: `open` or `reopened` → `fixed` (`by: rerun`);
  - reproduced: `fixed` → `reopened`.

  New findings are matched to lineages by line mapping (`git diff --no-index -U0` between `round-<n-1>/target.md` and `round-<n>/target.md`) and by the R19 rule. A new finding that matches a `fixed` lineage reopens it. One that matches none becomes a new `F-<k>`.
- R21: `state.json` (`0600`) contains:
  - **round records:** `n`, `doc_sha256` (original), `head_commit`, `dirty`, `sources` (path → blob sha256), engine, reviewers and status, `argv_sha256`, `binary_sha256`, `config_source`, `prev_hmac` and `hmac` (R38);
  - **map:** occurrence → lineage;
  - **lineages:** `severity`, `owner`, `persona`, `status ∈ open|fixed|reopened|contest_pending|settled|escalated|declined`, `contests` and `origin_round`;
  - **transition history:** command, round, reason and `by ∈ elephant|reviewer|rerun|arbiter|human`.

  `.toolbelt/egm/` joins `config.LocalCacheExcludePatterns`.
- R22: A `settled` or `declined` lineage that reappears records the occurrence and does not change status.

**Convergence and verdict**

- R23: A round is clean when, **after** matching and re-evaluation, there is no blocker/major lineage in `open`, `reopened`, `contest_pending` or `escalated`, and no reviewer is `not_run`.
- R24: `converged: true` requires `egm.clean_rounds` (default 2) consecutive clean rounds. `capped: true` holds when round `egm.max_rounds` (default 5) ends without converging. Rounds past the cap require `--past-cap` with human confirmation (R38) and reach, at most, `PASS_PARTIAL`.
- R25: Verdicts:
  - `IN_PROGRESS`: the loop has not converged nor hit the cap yet.
  - `PASS`: converged with full coverage (9 reviewers `completed` in every counted round), without `--force-settle`, without `--past-cap` and without `--restart`.
  - `PASS_PARTIAL`: converged with some reviewer `unavailable` or `skipped`, with restricted `--reviewers`, `--force-settle`, `--past-cap` or `--restart`.
  - `FAIL`: hit the cap (`capped`) without converging and without `--past-cap`.

  A round with less coverage than round 1 is refused, except with `--narrow --reason` and human confirmation. Starting narrow from round 1 is allowed, but the result is `PASS_PARTIAL`.
- R26: `loop next` is refused when neither the `doc_sha256` nor any `sources` sha changed since the previous round **and** there are blocker/major lineages in `open` or `reopened`. The message is `documento e fontes não mudaram desde a rodada N; há findings bloqueantes abertos` (document and sources have not changed since round N; there are open blocking findings).
- R27: (removed in round 2; churn was moved to Phase C.)
- R28: tb counts the agent runs performed in the loop (reviewers, retries and arbiters) and stops at `egm.max_agent_runs` (default 120). Pending reviewers become `not_run` with `reason: budget`. Before each round, the worst case is `reviewers × 2 + pending contests × 2`. `--dry-run` prints this worst case and the remaining budget. A round whose worst case exceeds the remainder is refused, except with `--allow-partial-budget`.

**Resolve and arbiter**

- R29: `tb egm resolve <doc>`. Repeating a transition to the current status is a successful no-op. Any other invalid transition is an error and changes nothing.

  | Flag | From | To | Constraint |
  |------|------|----|------------|
  | `--fix F-…` | open, reopened, escalated | fixed | the `doc_sha256` or the sha of some source cited by the lineage changed since the `origin_round` (otherwise, refused); the R20 re-evaluation confirms or reopens |
  | `--contest F-x --evidence path:line` | open, reopened | contest_pending | `contests == 0`; the evidence is in the lineage's `sources` or in the finding's own `evidence`, in a versioned file with the same blob at the `origin_round`'s `head_commit` and at `HEAD`, with no worktree changes; persona ≠ comprehension |
  | `--escalate F-x --note "…"` | open, reopened | escalated | `contests == 0` |
  | `--decline F-…` | open, reopened | declined | `nit` only; `contests == 0` |
  | `--force-settle F-x --reason "…"` | open, reopened, contest_pending, escalated | settled | human confirmation (R38); `forced: true`; maximum verdict `PASS_PARTIAL` |
  | `--dup-of F-a F-b` | F-a ∈ open, reopened, with `contests == 0` | F-a merged into F-b | F-b ∈ open, reopened or fixed; `overlap(scope) ≥ egm.match.overlap`; F-b inherits the highest severity and the most restrictive status (R30) |

- R30: Status restriction order, from most restrictive to least: `escalated` > `contest_pending` > `reopened` > `open` > `fixed`.
- R31: On `loop next`, before the round, each `contest_pending` lineage goes to a fresh arbiter. The arbiter receives the finding, the `scope` excerpt from the snapshot and the evidence (±20 lines of the blob at `HEAD`, redacted and already checked by R17a), all in the R9 blocks. The output is validated against `internal/egm/schema/arbiter.schema.json` (`{"nonce","verdict":"withdraw|uphold","rationale"}`), with retry and `not_run` as in R16. Result:
  - `withdraw` → `settled` (`by: arbiter`);
  - `uphold` → `open`, with `contests = 1`;
  - `not_run`, or no agent → stays `contest_pending`.

  After `uphold`, the lineage only leaves via `--fix` (with re-evaluation), via `still_holds: false` from the owning reviewer, or via `--force-settle`.
- R32: An `escalated` blocker/major lineage only leaves via `--fix` or `--force-settle`.

**Integration**

- R33: Commands:
  - `tb egm review <doc>` creates round 1. If the state already exists, it errors and suggests `loop next` or `--restart`.
  - `--restart` opens a new series of rounds **on top of** the existing state. It keeps lineages, `contests`, history and the total round count. It requires human confirmation. The maximum verdict becomes `PASS_PARTIAL`, and the report shows `restarted: N`.
  - If `<doc>` has no state, but `git diff --find-renames <trust ref>..HEAD` shows it was renamed from a path that has state, `review` refuses and requires `--adopt <old-path>`, which transfers the state.
  - There are also `loop next <doc>`, `resolve <doc>` and `status <doc>`.

  All accept `--json`. The MCP tools `tb_egm_review`, `tb_egm_loop_next`, `tb_egm_resolve` and `tb_egm_status` live in `internal/registry/parity.go`. Flags that require human confirmation are refused via MCP.
- R34: Exit codes:
  - `review`/`loop next`: 0 when the round completes without `FAIL`; 1 on `FAIL`; 2 on usage or execution error.
  - `resolve`: 0 on success or no-op; 2 on an invalid transition.
  - `status`:
    - `PASS`: 0;
    - `PASS_PARTIAL`: 0, or 1 with `--strict` or `egm.require_full_coverage: true`;
    - `IN_PROGRESS`: 3;
    - `FAIL`: 1;
    - records with invalid HMAC: 4.
- R35: `tb stack` gains:
  - the `egm` op (`tb stack egm --egm-doc <doc>`), which runs `status`;
  - the `--strict` flag;
  - `EgmDoc` and `Strict` fields in `stack.Options`;
  - `egm_doc` and `strict` arguments in the `tb_stack` tool schema in `parity.go`, with the `ops` array description updated to include `egm`;
  - an additive `warn` field in `StepResult` and in `Result` (OR of the steps).

  Mapping:
  - `PASS` → `ok: true`;
  - `PASS_PARTIAL` → `ok: true, warn: true`, or `ok: false` with `--strict` or `require_full_coverage`;
  - `IN_PROGRESS`, `FAIL` and invalid HMAC → `ok: false`.

  With `warn`, the stack lists the `forced` lineages and the `restarted` ones. The exit code with `warn` is 0.
- R36: `review.Finding`, `tb review` and `tb drift` do not change.

**Trust boundary and audit**

- R37: The keys `egm.agent_cmd`, `vendor`, `tools_disabled`, `forbidden_flags`, `output`, `env`, `home`, `reviewers`, `clean_rounds`, `max_rounds`, `match`, `require_full_coverage` and `max_agent_runs` are **sensitive**. They are read, in this order:
  1. from `$XDG_CONFIG_HOME/tb/egm.yaml`, the user config outside the repository;
  2. from `.toolbelt/config.yaml` at the **trust ref**, via `git show`, as `guard` does with `guard.yaml`. The trust ref is `egm.trust_ref`, which defaults to `origin/HEAD` and, if that does not exist, `main`.

  If the value in the worktree diverges, it is ignored with the `config_untrusted` warning. `.toolbelt/config.yaml` joins `guard`'s default `protectedPaths`. Declared limit: a process of the same user that edits the user config or commits to the trust ref is out of reach for this phase. Against that, CI and human review of the commit apply.
- R38: Each round record and each transition carry a chained HMAC-SHA256 (`prev_hmac`), keyed from `$XDG_STATE_HOME/tb/egm.key` (`0600`, created on the first run).
  - `status` and `tb stack egm` recompute the verdict from the verified records, not from the aggregated `state.json`.
  - The records also go, append-only, to `.toolbelt/egm/audit.jsonl`.
  - **Human confirmation:** stdin and stdout must be TTYs, and the user types the lineage ID (or `restart`, `narrow` or `past-cap`). Without a TTY, the flag is refused. The confirmation is recorded in the audit with `by: human`.
  - **Declared limit:** anyone with a shell as the same user can read the key or forge a TTY. The goal is to make tampering **evident** and expensive, not impossible. Capability tokens (ADR-005 §4) and a separate user are left for later.

### Configuration

```yaml
# Sensitive keys (R37): read from $XDG_CONFIG_HOME/tb/egm.yaml or from the trust ref.
egm:
  agent_cmd: []              # empty → deterministic
  vendor: claude             # claude | custom
  output: claude-json        # claude-json | raw (custom default: raw)
  tools_disabled: false      # must be true with vendor: custom
  forbidden_flags: []
  env:
    pass: []                 # e.g. [ANTHROPIC_API_KEY], added to the global env.pass
  home:
    copy: []                 # only if R0 chooses copying
    read_only: []            # e.g. ~/.nvm/versions/node/v22.11.0
  reviewers: {}              # <name>: {agent_cmd: [...]}
  match: {sim: 0.3, overlap: 0.5}   # 0 < x ≤ 1
  require_full_coverage: false
  max_rounds: 5              # ≥ 1
  clean_rounds: 2            # 1 ≤ clean_rounds ≤ max_rounds
  max_agent_runs: 120        # ≥ 1
# Common keys (may come from the worktree):
  trust_ref: origin/HEAD
  detail_headings: ["## Detailed Implementation Plan", "## Plano de Implementação Detalhado", "## Implementation Plan"]
  max_parallel: 4            # ≥ 1
  timeout_per_reviewer: 5m   # time.ParseDuration, > 0
  max_source_bytes: 65536
  max_packet_bytes: 409600
  max_stdout_bytes: 1048576
```

## Threat Model

| Threat | Mitigation |
|--------|------------|
| The document (written by the elephant) instructs the reviewers | R17a: scan for reviewer-directed patterns; R17b: mandatory literal attestation; R9: data blocks. Residual risk: a subtle injection the scan misses and that the model obeys in every reviewer, while still producing valid attestations. |
| Tampered config (fake agent, `clean_rounds: 1`) | R37 (sensitive keys come from the trust source; guard protects `.toolbelt/config.yaml`) and R1/R21 (`argv_sha256`, `binary_sha256` per round) |
| `state.json` or report edited by hand | R38 (chained HMAC; verdict recomputed from the records; exit 4) |
| Lottery (`--fix` with no change, repeat until it forgets) | R29 (`--fix` requires a change in the document or the sources), R20 (re-evaluation by the owner; omission invalidates), R26 |
| Re-rolling the arbiter, `--restart` or `git mv` | R29/R31 (one contest), R33 (`--restart` keeps history and downgrades; rename requires `--adopt`) |
| Injection into the arbiter via versioned evidence | R7 (excludes `testdata/` and personas), R29 (evidence must be a lineage source), R17a (excerpt scan), R31 (data blocks) |
| `--dup-of` from blocker to nit and then `--decline` | R29 (inherits the highest severity and the most restrictive status; no prior contest) |
| Escape hatches used by the agent itself | R38 (human confirmation via TTY, refused over MCP), R25 (downgrades to `PASS_PARTIAL`), R35 (stack lists forced and restarted) |
| CI gate with no LLM review | R25 (partial coverage), R34/R35 (`--strict` / `require_full_coverage`, protected by R37) |
| Goldfish reads `/proc/<pid>/environ`, other packets or the credential | R5 (no tools at all), R2 (RO workdir and private `TMPDIR`) |
| Contamination by the global `CLAUDE.md` | R4 (disposable HOME), R0 (verifies) |
| Secret sent to the provider | R7 (deny-list and versioned only), R8, `execenv.Filter` |
| Leaked or rotated credential | R0 (prefers env or config dir; tests refresh), R4 |
| Fake output on stderr or truncated JSON | R2 (separate stdout; overflow invalidates) |

## Non-Functional Requirements (NFRs)

- **Determinism:** in `deterministic` mode, the same inputs produce the same `report.json` byte for byte (stable ordering, no timestamps in the compared body).
- **Cost:** about 9 runs per round, bounded by R28.
- **Security:** `tb mem list` invariants.

## Alternatives

See [ADR-014](../adr/014-egm-nativo.md#alternatives-considered).

## Open Questions

- The authentication form, the no-tools flags and the read directories of the `claude` profile, as well as the matching thresholds, come out of R0.

## Detailed Implementation Plan

| File | Action | Reason |
|------|--------|--------|
| `internal/sandbox/sandbox.go`, `internal/sandbox/landlock_linux.go`, `internal/sandbox/landlock_other.go` | Modify | `ReadOnlyWorkDir`, `PrivateTmp`, `Home`, `Stdin`, `Stdout`/`MaxStdoutBytes` and file rule without `ReadDir` (R2) |
| `bench/egm-spike/` | Create | Spike (R0) |
| `internal/egm/vendor.go` | Create | `claude` and `custom` profiles, authentication, flags, envelope and bypass refusal (R4, R5, R15) |
| `internal/egm/run.go` | Create | Goldfish via `sandbox.New`, Registry and subreaper, timeout, retry (R3, R16) |
| `internal/egm/packet.go`, `internal/egm/sources.go` | Create | Snapshot, `resolveSource`, deny-list, limits, prompt and persistence (R1, R7, R9) |
| `internal/graph/index_docs.go` | Modify | Export `CollectMDLinks`, `ResolveDocHref`, `NormalizeRelPath`, `CollectPathLikes`, `CollectBacktickSpans`, `LooksLikePath` and `LooksLikeSymbol`; new extensions (R7, R13) |
| `internal/mem/redact.go` | Modify | New patterns and line preservation (R8) |
| `internal/egm/personas/*.md`, `internal/egm/standards/*.md` | Create | 8 personas and `arbiter` (with `max_severity`); `feature-spec`, `implementation-readiness` and `zoom-out` standards (R10, R11, R31) |
| `internal/egm/deterministic/{refs,sections,planfiles}.go` | Create | Offline checkers and plan parsing (R13) |
| `internal/egm/injection.go`, `internal/egm/injection_patterns.txt` | Create | Anti-injection scan (R17a) |
| `internal/egm/attest.go` | Create | Attestation validation (R17b) |
| `internal/egm/finding.go`, `internal/egm/schema/finding.schema.json`, `internal/egm/schema/arbiter.schema.json` | Create | Types, schemas, cap, nonce and evidence (R11, R15–R17) |
| `internal/egm/match.go` | Create | normalize, sim, overlap, merge, owner, diff mapping and lineage (R18–R20) |
| `internal/egm/state.go` | Create | Records, transitions, convergence, verdict, coverage and budget (R21–R26, R28) |
| `internal/egm/trust.go` | Create | Trust source, `config_untrusted`, chained HMAC, audit, TTY confirmation (R37, R38) |
| `internal/egm/resolve.go` | Create | Transition table and arbiter (R29–R32) |
| `internal/egm/review.go` | Create | Round orchestration, `--restart` and `--adopt` (R6, R10, R12, R33) |
| `internal/config/config.go`, `internal/config/exclude.go` | Modify | `EgmConfig` with Validate; `.toolbelt/egm/` in the exclude (R21) |
| `internal/guard/guard.go` | Modify | `.toolbelt/config.yaml` in the default protected paths (R37) |
| `cmd/tb/egm.go` | Create | Cobra group (R33, R34) |
| `internal/stack/stack.go`, `cmd/tb/stack.go` | Modify | `egm` op, `EgmDoc`, `Strict`, `warn` (R35) |
| `internal/registry/parity.go` | Modify | `tb_egm_*` tools; `tb_stack` schema (`egm_doc`, `strict`, `ops` description) (R33, R35) |
| `internal/bench/fake_agent.go`, `cmd/tb/fake_agent.go` | Modify | Read the prompt from stdin when `--prompt` is empty and extract `NONCE:`, `PRIOR:` and the data blocks; mode via `egm_mode` in `--config` (`findings`, `invalid`, `no-nonce`, `over-severity`, `omit-prior`, `bad-attest`, `stderr-json`, `overflow`, `sleep`, `probe-fs`, `escape`) |
| `docs/cli.md`, `docs/arquitetura.md` §8.5 (+ `docs/en/`) | Modify | Document `tb egm`; replace the fictitious `tb review pr:42` example |
| `CHANGELOG.md` | Modify | Unreleased entry |

### Verification Plan

- **R0:** script in `bench/egm-spike/`. Result in `docs/campo/dogfood.md` (PT and EN). Paraphrase corpus in `internal/egm/testdata/paraphrases.json`.
- **Sandbox:** tests for:
  - RO workdir (writing fails);
  - `PrivateTmp` (the shared `/tmp` becomes unreadable);
  - `Home` (the real HOME becomes unreadable and `Home` takes precedence over `Config.Env`);
  - separate stdout and overflow;
  - file rule without EINVAL;
  - fake-agent `probe-fs` and `escape`.

  A skip is only legitimate without Landlock.
- **Execution:** each fake-agent mode, bypass refusal, `custom` without `tools_disabled`.
- **Anti-cheating (tables):**
  - `--fix` with no change in the document or sources is refused;
  - the second `--contest` and evidence outside the sources or altered are refused;
  - evidence with injection is refused;
  - `--dup-of` blocker→nit followed by `--decline` is refused;
  - a document with injection yields a `major` from `injection-scan`;
  - **this spec does not trigger the scan**;
  - an invalid attestation yields `not_run`;
  - `clean_rounds` in the worktree yields `config_untrusted`;
  - an edited `state.json` yields exit 4;
  - an escape hatch without a TTY is refused;
  - `--restart` keeps `contests` and downgrades;
  - `git mv` requires `--adopt`;
  - coverage narrowing is refused.
- **Match:** R0 corpus, shifted lines, renamed heading, re-evaluation by the owner (both `style` instances) and closing by rerun.
- **State:**
  - R29 transitions and idempotence;
  - the four verdicts and `capped`/`--past-cap`;
  - R26 with an altered source;
  - arbiter `withdraw`, `uphold` and `not_run`;
  - budget.
- **Golden:** deterministic `report.json` of this spec, including the parsing of its own plan (R13).
- **Regression:**
  - `tb review` and `tb drift` JSON equal to the current golden (R36);
  - `tb mem` output (R8);
  - `guard` with the new protected path;
  - `go test ./...` and `TestDocsEnglishMirrorComplete`.

*Built on the EGM pattern*
