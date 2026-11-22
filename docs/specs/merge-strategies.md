🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/merge-strategies.md)

# `tb merge` strategies `semantic` and `prompt` — spec

Status: **Phases A (`semantic`) and B (`prompt`) implemented**; Phase C is open. It closes the README roadmap item "`tb merge` `prompt` / `semantic` strategies".

## Overview

`tb merge` integrates `done` tickets in a temporary worktree and moves the target ref only at the end, with a compare-and-swap. Today a conflict either aborts the merge (`fail`) or keeps the later ticket's whole file (`last_wins`). `last_wins` silently drops the earlier ticket's edits to that file.

This spec adds two strategies. They form a ladder: each one does the work of the previous one first.

| Strategy | Who resolves | LLM | Advances refs in one call |
|----------|--------------|-----|---------------------------|
| `semantic` | tb, deterministic rules on git's remaining conflict hunks | no | yes, like `fail`/`last_wins` |
| `prompt` | `semantic` first, then a sandboxed one-shot agent for the hunks left | yes (child process) | **no**: it proposes a receipt; a second call applies it |

Both refuse rather than guess: a hunk that no rule (or no valid agent answer) resolves leaves the merge in conflict, and no ref changes.

## Problem

- **Parallel tickets conflict on trivial edits.** Two tickets that add different imports to one Go file, or edit adjacent lines, conflict. Git refuses adjacent edits on purpose. `fail` then blocks the whole merge.
- **`last_wins` loses work.** It replaces the whole file (`internal/merge/merge.go:503-505`), so the earlier ticket's edits to that file are gone, with no hunk-level report.
- **No design on record.** `prompt` and `semantic` are reserved names that fail with "not implemented". No doc says what they should do or how far an agent-written resolution is trusted.

## Current state

| What | Where |
|------|-------|
| `Strategy` constants: only `fail` and `last_wins` | `internal/merge/merge.go:23-28` |
| `ParseStrategy` rejects `prompt`/`semantic` as "not implemented" | `internal/merge/merge.go:31-42` |
| `Options` (`DryRun`, `Output`, `AllowSensitive`, …) | `internal/merge/merge.go:47-60` |
| `File`, `Conflict` (`Resolution` is `""` or `last_wins`), `Result` | `internal/merge/merge.go:62-96` |
| `Run`: sensitive gate, temp worktree, `pickAll`, `finalize` | `internal/merge/merge.go:165-217` |
| `checkSensitive` (refuses only when applying) | `internal/merge/merge.go:242-255` |
| `validateOptions` repeats the strategy check | `internal/merge/merge.go:257-272` |
| `pickAll` stops at the first conflict under `fail` | `internal/merge/merge.go:325-347` |
| `pick`: one `cherry-pick` per commit; on conflict reads `--diff-filter=U` | `internal/merge/merge.go:506-552` |
| `resolveTheirs`: whole file from the commit (`last_wins`) | `internal/merge/merge.go:557-570` |
| `finishPick`: `--skip` when empty, else `--continue` | `internal/merge/merge.go:575-582` |
| `advance`: ff-only in a checked-out worktree, else `update-ref` CAS | `internal/merge/merge.go:585-610` |
| Hardened git args (no hooks, fsmonitor, external diff, pager, signing) | `internal/merge/sensitive.go:20-29` |
| Sensitive paths (hooks dirs, `.toolbelt/`, workflows, `.gitmodules`, `.gitattributes`, gitlinks) plus guard protected paths | `internal/merge/sensitive.go:86-107`, `:111-146` |
| CLI flags and help text ("prompt/semantic are not implemented") | `cmd/tb/merge.go:15-40` |
| MCP `tb_merge`: dry-run unless `apply:true`; conflict returned with `ok:false` | `internal/registry/merge.go:10-61` |
| Sandboxed one-shot agent runner (Landlock, subreaper, disposable HOME, stdout cap, tool-use check) | `internal/egm/run.go:114` (`NewRunner`), `:227` (`Run`), `:124-161` (`classify`) |
| Agent profile: `egm.agent_cmd[0]` absolute, forbidden flags, `tools_disabled` for custom vendors | `internal/egm/vendor.go:48-96` |
| User-only config read (never the repository's file) | `internal/egm/config.go:32-48` |
| Secret redaction and deny-list | `internal/secrets/secrets.go:32`, `:58` |
| The router never calls merge | `internal/router/` (no reference) |

## Goals

- G1: `semantic` resolves the common, mechanical conflicts of parallel tickets with no LLM, deterministically, and reports every resolved hunk.
- G2: `prompt` resolves the remaining hunks with an agent that can change **only** the conflict hunks, cannot touch disk, runs sandboxed, and whose answer is validated before use.
- G3: an agent-written resolution never moves a ref in the call that produced it. A human reviews a receipt, and the apply step replays it without calling the agent.
- G4: a full audit trail: per hunk, the rule or agent run that resolved it, the hashes of its inputs, and the result tree.
- G5: no change for current users. `fail` stays the default, and the `fail`/`last_wins` output and JSON stay byte-identical.

## Non-Goals

- Router auto-merge. The router keeps not calling merge.
- Resolving conflicts in sensitive paths, binary files, symlinks, gitlinks, or delete/modify, rename and mode conflicts.
- AST-level merging for languages other than Go, or a general structural merge engine (mergiraf-style).
- Running build/test commands on the merged tree in Phase A or B (see Phase C and Open questions).
- Exposing `prompt` through MCP in Phase B.
- Any network access by tb. The agent child may use the network the human configured for it; tb opens none.

## Design

### Conflict model

Conflicts keep being found by the per-commit `cherry-pick` in the temporary worktree (`pick`, `merge.go:506`). For each unmerged path, tb reads the index stages with `git ls-files -u -z` (mode and blob of stages 1, 2 and 3) and classifies the file:

- **file-level refusals** (reason in parentheses): a missing stage 1 (`add_add`), a missing stage 2 or 3 (`delete_modify`), stages with different modes (`mode`), mode `120000` (`symlink`) or `160000` (`gitlink`), a NUL byte in the first 8000 bytes of any stage (`binary`), a sensitive path (`sensitive`, see M14), a stage larger than `merge.semantic.max_file_bytes` (`too_large`);
- **text files**: tb writes the three blobs to a private temp directory (0700) and runs `git merge-file -p --diff3 --marker-size=<n> -L ours -L base -L theirs` with the hardened args. `<n>` is one more than the longest run of `<`, `=`, `>` or `|` at the start of any line of the three stages, so tb's parser never mistakes file content for a marker. The output is split into clean text and **conflict regions**, each with its base, ours and theirs lines.

`git merge-file` reads no attributes and runs no merge driver, so no repository config can run code here.

"Ours" is the accumulated integration (`HEAD` of the temp worktree), "theirs" is the ticket commit being picked, and "base" is that commit's parent version.

### Requirements

**Strategy parsing and options**

- M1: `ParseStrategy` accepts `semantic` and `prompt` (lower-case, trimmed). `validateOptions` accepts the four strategies. The error text for unknown strategies lists all four.
- M2: `Options` gains `Resolutions string` (a receipt ID, `prompt` only) and `Config string` (the root `-c` user config override). `--resolutions` with any other strategy is a usage error.

**`semantic` (deterministic, Phase A)**

- M3: For every conflict region, tb tries the rules below in order; the first that applies resolves the region. A file is resolved only when all its regions are; the result is the clean text with each region replaced by its resolution, staged with `git add`. Otherwise the file stays unresolved, with a reason per region.
- M4: Rule `disjoint`: tb diffs ours against base and theirs against base inside the region (a pure-Go LCS, bounded by `merge.semantic.max_region_lines`, else reason `region_too_large`). When the two sides change disjoint base line ranges, the resolution applies both edit sets. Two insertions at the same base position, or edits that touch a common base line, are reason `overlap`.
- M5: Rule `go_imports`: for a `.go` file whose ours and theirs stages both parse (`go/parser`, `ImportsOnly`), a region whose base, ours and theirs lines all lie inside the same parenthesized `import` declaration and are each a single import spec (no blank lines and no comments) resolves to the set ours ∪ theirs minus the specs that either side removed from base, sorted by import path, one per line, with ours' indentation. Anything else in the region falls through to the next rule.
- M6: Post-check for a resolved file: (a) every resolved region has no line that is a conflict marker of size 7 or `<n>` (`<<<<<<<`, `|||||||`, `=======`, `>>>>>>>`, alone or followed by a space) unless that same line occurs in that region's base, ours or theirs; (b) a `.go` file whose ours and theirs stages parse must parse after resolution (`go/parser`, full file), else the file is unresolved with reason `parse_failed`. Phase A checks no other language.
- M7: When any file of a pick stays unresolved, `semantic` behaves like `fail`: `cherry-pick --abort`, stop, change no ref. The error wraps `ErrConflict` and suggests `--strategy prompt` or `--strategy last_wins`.
- M8: `semantic` needs no extra flag to apply: once every conflict resolves, it finalizes like `fail`/`last_wins` (dry-run, `--output` and the sensitive gate unchanged). See USER DECISION 2.

**Result and JSON (Phase A)**

- M9: `Conflict` gains `reason` (empty when resolved) and `hunks` (per region: `region`, `ours_lines`, `theirs_lines` as `[start,end]` in the stage file, `reason` or `rule`). `Conflict.Resolution` gains `semantic` and `prompt`. `File.Status` gains `semantic` and `prompt`. With `fail`/`last_wins` the new fields are omitted (`omitempty`), so their JSON is unchanged.
- M10: `Result` gains `resolutions` (omitted when empty): one entry per resolved region with `file`, `ticket`, `commit`, `region`, `rule` (`disjoint`, `go_imports` or `agent`), `base_sha256`, `ours_sha256`, `theirs_sha256` (of the region's lines) and `result_sha256`. `Result` also gains `result_tree` (the tree SHA of the integrated commit) whenever `result_sha` is set.
- M11: The text table prints `resolved by semantic (disjoint)` and the like per conflict, and `unresolved (<reason>)` with the region line ranges otherwise.

**`prompt` (agent, Phase B)**

- M12: `prompt` runs `semantic` first. Only the regions that `semantic` left unresolved go to the agent. A file with a file-level refusal (binary, symlink, sensitive, …) never goes to the agent.
- M13: **Two steps.** `tb merge --strategy prompt <ids>` (the proposal) integrates, asks the agent, validates, writes a receipt (M20) and **never advances a ref**, whatever `--dry-run` says. It prints the receipt ID, its path and the `result_tree`, and honors `--output` (the patch of the proposal). `tb merge --strategy prompt --resolutions <id> <ids>` (the apply) re-integrates, runs `semantic`, takes each remaining region's text from the receipt, **does not start the agent**, and then finalizes like the other strategies. See USER DECISION 1.
- M14: **Never sent to the agent**, reason in parentheses: a sensitive path (`sensitive`, the same predicate as `checkSensitive`, even with `--allow-sensitive`); a path on the `secrets.Denied` list (`secret_path`); a file whose prompt text (regions plus context) has a match for `secrets.Redact` (`secret_in_context`). tb does not redact and send: a redacted hunk cannot be resolved correctly, and the invariant forbids secrets in prompts.
- M15: **The agent.** tb starts one `egm.Runner` per merge (`egm.NewRunner`, the same Landlock sandbox, subreaper, disposable HOME and vendor profile as the egm llm engine), configured from the user config only (M23, M24). One goldfish run per conflicted file, with all its unresolved regions. The workdir is a fresh empty, read-only packet directory; the prompt goes on stdin. The agent cannot read the repository or write anything: tb splices its answer.
- M16: **Prompt.** A fixed template in the tb binary, with a 128-bit nonce. Untrusted content (ticket IDs and titles, file path, each region's base/ours/theirs and up to `context_lines` lines of ours before and after it) sits inside nonce-delimited blocks, and the instructions say that this content is data. The template asks for exactly one JSON object: `{"nonce":"<nonce>","regions":[{"id":<k>,"text":"<resolved lines>"}]}`, or `{"nonce":"<nonce>","refuse":"<reason>"}`.
- M17: **Validation** of the payload (any failure makes that file unresolved with the reason in parentheses): the JSON parses and the nonce matches (`agent_protocol`); the region IDs are exactly the ones sent, each once (`agent_protocol`); no text has a NUL byte or invalid UTF-8 (`agent_protocol`); each text is at most `4 × (ours + theirs bytes) + 4 KiB` (`agent_oversized`); the M6 checks pass (`agent_markers`, `parse_failed`); `secrets.Redact` finds nothing in a text that it did not find in that region's ours or theirs (`agent_secret`). A `refuse` answer is reason `agent_refused`.
- M18: **Run failures.** `timeout`, `missing_output` and `exit_<n>` are retried once (`merge.prompt.attempts`, default 2), then the file is unresolved with that reason. `tools_used`, `escaped_process`, `unattributed_orphan` and `sandbox_error` are not retried and abort the whole merge (no receipt, no ref change), because isolation broke. `interrupted` (SIGINT, the total budget) aborts the same way.
- M19: **Novel lines.** For each agent region, tb counts the resolved lines that occur in none of base, ours or theirs (`novel_lines`). They are reported in the JSON, the table and the receipt. With `merge.prompt.max_novel_lines` set, a region above it is unresolved (`agent_novel`). The default is unset (report only). See USER DECISION 4.

**Receipt and apply (Phase B)**

- M20: **Receipt.** A JSON file written once (0600, atomic temp plus rename) under `$XDG_STATE_HOME/toolbelt/merge/<repo hash>/` (directory 0700; `TB_MERGE_STATE_DIR` overrides it for tests). It is outside the repository and outside the git directory, so a worker or a ticket cannot plant one. Its ID is the first 16 hex characters of its sha256. It holds:
  - `v`, `created_at`, `target`, `start_sha`, `strategy`;
  - `tickets`: `id`, `base`, `head` per ticket, in apply order;
  - `agent`: `vendor`, `output`, `argv_sha256` (`egm.ArgvSHA`), and the tb version;
  - `runs`: per file and attempt, `reason`, `duration_ms`, `prompt_sha256`, `payload_sha256` (no stderr, no environment);
  - `resolutions`: the M10 entries, plus `text` and `novel_lines` for `agent` entries;
  - `unresolved`: the files and regions that stayed in conflict, with reasons;
  - `result_tree`: the tree of the proposed integration, empty when anything stayed unresolved.

  A proposal with unresolved regions still writes its receipt (for audit) but exits with `ErrConflict`, and that receipt cannot be applied (`receipt_incomplete`).
- M21: **Apply checks**, all before any ref moves (reason in parentheses): the file's sha256 matches its ID (`receipt_tampered`); `target`, `start_sha` and the ticket list (IDs, bases, heads, order) match this run (`receipt_stale`, which says to rerun the proposal); every region that `semantic` leaves unresolved has an `agent` entry with equal `file`, `commit`, `region` and input hashes (`receipt_mismatch`); the integrated tree equals `result_tree` (`receipt_mismatch`). Then the normal finalize runs: the sensitive gate, `--dry-run`, `--output`, the CAS advance and `merged_at`.
- M22: `tb merge receipts` is out of scope for Phase B; receipts are read with `cat`/`jq` from the printed path. Receipts older than 30 days are swept on the next `prompt` run.

**Configuration (Phase A for `semantic`, Phase B for `prompt`)**

- M23: Every new key is read from the **user config only** (the `-c` file, else the global config), through `config.LoadUser` (the generalized `LoadUserConfig`, as the vector-index spec does). A repository `.toolbelt/config.yaml` that sets any `merge.*` key gets a `config_untrusted` warning and the value is ignored.
- M24: The agent is the egm agent: `egm.agent_cmd`, `vendor`, `output`, `env.pass`, `home.*`, `path`, `tools_disabled`, `tools_check` and `forbidden_flags`, validated by `egm.ResolveProfile`. There is no second `agent_cmd`. See USER DECISION 5.

| Key | Default | Meaning |
|-----|---------|---------|
| `merge.semantic.max_region_lines` | `2000` | Above it a region is `region_too_large` |
| `merge.semantic.max_file_bytes` | `1048576` | Above it a file is `too_large` |
| `merge.prompt.max_files` | `10` | More conflicted files than this for the agent: refuse before any run (`prompt_budget`) |
| `merge.prompt.context_lines` | `20` | Lines of ours before and after each region |
| `merge.prompt.max_prompt_bytes` | `262144` | Per file; above it the file is unresolved (`prompt_budget`), never truncated |
| `merge.prompt.timeout_per_file` | `5m` | Per goldfish run |
| `merge.prompt.max_total` | `20m` | Whole proposal; past it the run is `interrupted` |
| `merge.prompt.attempts` | `2` | Per file, transient failures only (M18) |
| `merge.prompt.max_stdout_bytes` | `1048576` | Passed to the runner |
| `merge.prompt.max_novel_lines` | unset | M19 |

**Platform and sandbox (Phase B)**

- M25: `prompt` needs what the egm llm engine needs (Linux, Landlock, subreaper). Without them `NewRunner` fails and the merge stops with that reason (`no_landlock`, `no_subreaper`) before integrating. There is no fallback to `semantic` or to an unsandboxed run.
- M26: Without `egm.agent_cmd`, `prompt` fails with `agent_unavailable`. `semantic` never reads the egm config.

**CLI and MCP**

- M27: CLI: `--strategy` help lists `fail|last_wins|semantic|prompt`; new `--resolutions <id>`; the user config override is the root `-c/--config` flag (`cmd/tb/main.go:68`). The long help replaces "prompt/semantic are not implemented" with a summary of M3–M8 and M13.
- M28: MCP `tb_merge`: `strategy` accepts `semantic` (dry-run unless `apply:true`, as today). `prompt` is refused with `prompt_cli_only` in Phase B: an MCP caller is usually an agent that wrote the tickets, and must not start a second agent nor apply a receipt. No `resolutions` parameter. See USER DECISION 3.

### Why these choices

- **`semantic` on top of `git merge-file`, not a new diff3.** Git already merged everything it could; tb only adds rules for the regions git left. A pure-Go diff3 could disagree with git on what conflicts, and would be one more merge engine to trust.
- **Splicing, not whole-file answers.** The agent returns region texts and tb writes the file, so "only conflicted hunks changed" holds by construction, not by a check after the fact.
- **Two steps for `prompt`.** The agent read content that other agents wrote (the tickets) and can be steered by it. A human reading the receipt is the fresh-context evaluator that the "implementer and evaluator are separate" invariant asks for. The apply replays bytes that were reviewed, bound by input hashes and the result tree.
- **Refuse rather than redact.** A redacted region cannot be resolved correctly, and the invariant says that secrets never reach a prompt.

#### Alternatives considered

| Alternative | Why not (now) |
|-------------|---------------|
| `semantic` = plain `git merge-file` per file | It reproduces the cherry-pick's own conflicts; it adds nothing |
| A union rule (`merge=union`: keep both sides' lines) in `semantic` | Silently duplicates or reorders code; kept as an open question for append-only regions |
| Language-aware structural merge for many languages | Large, per-language parsers; out of scope; Go is the repository's own language |
| `prompt` writes files in a writable worktree copy | Needs a post-check that only conflicted files changed and widens the sandbox; splicing is simpler and stronger |
| `prompt` applies in one call behind a flag (`--accept-agent`) | Skips the review that the trust model rests on; see USER DECISION 1 |
| Redact secrets and send | Wrong resolutions, and it weakens the "secrets never in prompts" invariant |
| A second `merge.agent_cmd` key | Two agent configs to harden and keep in sync; the egm runner already enforces the profile |
| Receipts in `.toolbelt/` or in the git dir | Inside paths that workers or tickets can write; `.toolbelt/` is a sensitive path |

## Security

| Threat | Mitigation |
|--------|------------|
| A ticket's content steers the agent (prompt injection) toward a malicious resolution | No tools, read-only empty workdir, splicing (M15); nonce-delimited data (M16); validation (M17); `novel_lines` reported (M19); no ref moves without a reviewed receipt (M13) |
| An agent edits files beyond the conflict | It writes nothing; tb splices only the region IDs it sent (M15, M17) |
| Sensitive paths (hooks, CI, `.toolbelt/`, guard paths) resolved by a rule or an agent | Never resolved by `semantic` or `prompt`, even with `--allow-sensitive` (M14); the existing sensitive gate still runs on apply |
| Secrets in prompts or receipts | Deny-listed paths and files with secret matches are never sent (M14); an answer that adds a secret is refused (M17); receipts hold no stderr or environment (M20) |
| A forged or edited receipt | Stored outside the repository and the git dir (0700/0600); its ID is its sha256; apply checks ticket heads, input hashes and the result tree (M21) |
| A stale receipt applied after the target moved | `start_sha` must match (M21); the CAS advance still applies |
| Sandbox missing or broken | `NewRunner` fails closed (M25); isolation failures abort the merge, no retry (M18) |
| Repository config chooses the agent or raises budgets | `merge.*` and the agent keys come from the user config only (M23, M24) |
| Code execution through git during resolution | Hardened git args everywhere; `git merge-file` reads no attributes and runs no drivers |
| Resource exhaustion | Region, file, prompt and stdout caps; per-file and total timeouts; file count cap (M23 table) |
| Network | tb opens none; only the human-configured agent child may (invariant) |

## Acceptance criteria

### Phase A (`semantic`)

1. `fail` and `last_wins` produce byte-identical text and JSON against the release before Phase A (golden tests on the existing fixtures).
2. Two tickets that edit adjacent, non-overlapping lines of one file: `fail` conflicts; `semantic` applies both edits, `files[].status` is `semantic` and one `resolutions` entry has `rule: disjoint`.
3. Two tickets that add different imports to the same Go import block: `semantic` resolves with `go_imports`, the file parses, and imports are sorted and deduplicated.
4. Two tickets that change the same line differently: `semantic` exits with `ErrConflict`, reason `overlap` with line ranges, and no ref changes.
5. Binary, symlink, gitlink, delete/modify, add/add and mode conflicts are never resolved and carry their reason.
6. A conflict on a sensitive path is never resolved by `semantic`, with or without `--allow-sensitive`.
7. A region whose resolution would leave a marker line not present in the inputs, or a Go file that no longer parses, stays unresolved (`parse_failed`).
8. A file whose content holds lines like `=======` or `<<<<<<<` (a Markdown heading, a test fixture) is parsed correctly thanks to `--marker-size`.
9. A `merge.semantic.*` key in the repository config is ignored with `config_untrusted`.
10. MCP `tb_merge` with `strategy: semantic` is dry-run unless `apply:true` and returns the same JSON as the CLI.

### Phase B (`prompt`)

11. A proposal never advances a ref, with or without `--dry-run`, and prints the receipt ID, path and `result_tree`.
12. With a fake agent (`vendor: custom`, `tools_disabled: true`) that returns valid regions, the proposal writes a receipt; the apply with `--resolutions` does not start any agent process (checked with a fake that fails when run) and produces a tree equal to `result_tree`.
13. Each of these fake answers leaves the file unresolved with the stated reason: a wrong nonce, a missing or extra region ID (`agent_protocol`); an oversized text (`agent_oversized`); a new marker line (`agent_markers`); a Go file that no longer parses (`parse_failed`); an added secret (`agent_secret`); `refuse` (`agent_refused`).
14. A file on the deny list, or one whose regions or context hold a secret, is never in any prompt (a recording fake agent shows it), and is unresolved with `secret_path` or `secret_in_context`.
15. An apply fails without moving refs when: the receipt bytes changed (`receipt_tampered`); the target moved or a ticket head changed (`receipt_stale`); a region's inputs differ, or the tree differs (`receipt_mismatch`); the receipt has unresolved regions (`receipt_incomplete`).
16. On a host without Landlock, `prompt` fails with `no_landlock` before integrating; nothing falls back to `semantic`.
17. An agent that uses tools or leaves an escaped process aborts the whole merge without a receipt and without retry; a timeout is retried once.
18. `egm.agent_cmd` or `merge.prompt.*` in the repository config is ignored with `config_untrusted`; without a user `egm.agent_cmd`, `prompt` fails with `agent_unavailable`.
19. MCP `tb_merge` with `strategy: prompt` fails with `prompt_cli_only`.
20. The receipt holds no stderr, no environment value, and no `env.pass` value (checked by searching for a canary).

## Phases

### Phase A: `semantic` (one PR)

- `internal/merge/strategy.go`: strategy constants and parsing (M1).
- `internal/merge/semantic.go`: stage classification, `git merge-file` call, marker-size choice, region parser, `disjoint` and `go_imports` rules, post-checks (M3–M7).
- `internal/merge/merge.go`: `pick` routes conflicts to the semantic resolver; `Conflict`, `File` and `Result` fields (M9, M10).
- `internal/config`: `LoadUser` and the `merge.semantic.*` keys (M23).
- `cmd/tb/merge.go`, `internal/registry/merge.go`: accept `semantic`; the table (M11, M27, M28).
- Tests: the Phase A criteria, with fixtures built by `git` in temp repos, like `merge_test.go`.

### Phase B: `prompt`

- `internal/merge/prompt.go`: prompt template, runner use, payload validation, novel lines (M12–M19).
- `internal/merge/receipt.go`: receipt write, sweep, apply checks (M20–M22).
- `internal/config`: `merge.prompt.*`.
- CLI `--resolutions`; MCP `prompt_cli_only`.
- If importing `internal/egm` from `internal/merge` creates a cycle or drags too much, extract the runner into `internal/agentrun` first, with egm's tests unchanged.

### Phase C (optional, after use)

- `--verify`: run `tb verify`'s build/lint (no hidden tests) in the temporary worktree before advancing. It runs repository commands outside the sandbox, so it is opt-in. See USER DECISION 6.
- More deterministic rules (append-only regions, other import syntaxes), each with its own criteria.
- MCP proposal-only `prompt`, if USER DECISION 3 allows it.

## Docs to update with the implementation

- **README:** the roadmap item and the `tb merge` row of the feature table.
- **`docs/cli.md`:** the `tb merge` section (strategy table, flags, receipts, new reasons).
- **`docs/arquitetura.md`:** the "Merge results (strategies: …)" example comment.
- **`cmd/tb/merge.go`** help text and the `tb_merge` MCP description.
- **CHANGELOG**, and the pt-BR mirror of each.

## USER DECISIONS

These change the trust model; the spec picks a default for each.

1. **Two-step `prompt`** (default) versus one call with `--accept-agent`. Two steps cost one more command but keep a human between the agent and the refs.
2. **`semantic` applies in one call** (default). `disjoint` resolves conflicts that git refuses on purpose. The alternative is to make `semantic` propose-only, like `prompt`.
3. **MCP `prompt`**: refused in Phase B (default). Alternative: proposal only, never apply.
4. **Novel lines**: report only (default) versus a default `max_novel_lines: 0`, which limits the agent to choosing and ordering existing lines.
5. **Reuse the egm agent config** (default) versus a separate `merge.agent_cmd`.
6. **`--verify` before advancing** (Phase C, opt-in, unsandboxed build) versus never running repository commands from merge.

## Resolved decisions

The user accepted the proposed default of each USER DECISION above:

1. **`prompt` is two-step.** The proposal writes a receipt outside the repository and never moves a ref; `--resolutions <id>` replays it after checking the start SHA, ticket heads, hunk input hashes and result tree.
2. **`semantic` applies in one call** when every hunk resolves by `disjoint` or `go_imports`.
3. **MCP** allows `semantic` (dry-run unless `apply:true`) and refuses `prompt` with `prompt_cli_only`.
4. **Novel lines** are reported, not refused (`merge.prompt.max_novel_lines` is unset by default).
5. **The agent** is the egm agent: `egm.agent_cmd` from the user config only; there is no `merge.agent_cmd`.
6. **`--verify`** is Phase C, not implemented now; merge never runs repository commands.

Implementation notes (where it differs from the text above):

- `result_tree` is reported only by `semantic` and `prompt`, not whenever `result_sha` is set, so the `fail` and `last_wins` JSON stays byte-identical (G5, criterion 1).
- `go_imports` sorts the specs of the conflict region only, not the whole import block.
- `ours_lines` and `theirs_lines` are found by a forward search of the region text in the stage file; they are exact when the region text is unique in the file.
- A `prompt` apply does not need `egm.agent_cmd`: it never starts an agent. Only the proposal fails with `agent_unavailable`.
- The agent answer may be wrapped in a Markdown code fence; tb strips one fence before parsing.
- `stdout_overflow` is a transient run failure (retried like `timeout`).

## Open questions

- Should `last_wins` gain a hunk-level mode (keep the later side only in conflicting regions) now that the region parser exists?
- Is an `append_both` rule (both sides insert at the end of the same block; keep ours, then theirs) safe enough for `semantic` when the Go file still parses?
- Should `prompt` send the ticket mandate or acceptance text as context? It helps resolution but is more untrusted text in the prompt.
- Should the receipt carry an attestation (for example, signed by the user's git signing key) for teams that share receipts?
