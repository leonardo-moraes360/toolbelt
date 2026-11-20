🇺🇸 English | [🇧🇷 Português](pt-BR/bus-router.md)

# Router on the bus — B4 spec

Status: **implemented** (B4a and B4b; spec round 3, after two fresh-context reviews). Where the code is simpler than this text, the note *In this cut* says so.

Specification for stage **B4** of [ADR-013](adr/013-barramento-multi-vendor.md) (§8): the Phase 3 router ([ADR-009](adr/009-router-e-workers.md)) gains a second worker type, an agent registered on a bus channel. It builds on the protocol ([bus-protocolo.md](bus-protocolo.md)), the lease ([bus-lease.md](bus-lease.md)) and the harness ([bus-harness.md](bus-harness.md), [bus-notify-discover.md](bus-notify-discover.md)).

B4 ships in two parts, each testable alone:

- **B4a:** bus operations and the harness `router_tasks` mode. It can be tested with a fake router.
- **B4b:** bus slots in `tb router run`.

## Problem

`tb router run` today runs one single-use `router.agent_cmd` per ticket, in the Landlock sandbox, confined to the ticket's worktree. A persistent agent of another vendor (a Codex or Claude Code session woken by `tb bus run`) cannot take a ticket: there is no way to hand it one, to know it is working, or to get its result back to `tb merge`.

## Principles

1. **Same confinement as a local worker.** The model that works on a router task runs confined to that ticket's worktree. An agent that can write the repository root can rewrite `.git/config` filters, `.git/worktrees/`, tickets, router state and `bus.db`, so no router check would mean anything. B4 has no unconfined mode.
2. **The model never touches the bus for a router task.** The harness process, outside the sandbox, does the claim, keeps the lease and completes. The confined child only edits files and writes a verdict file. A child that could open `bus.db` could forge claims, results and revokes.
3. **The router stays the only writer of tickets and commits.** It commits the worktree only after the harness has completed, which happens only after the child's process group has exited, so nothing writes while it commits.
4. **Same result contract.** `tb merge` keeps reading `Result.BaseCommit`/`HeadCommit` from the local repository and does not change.
5. **Every attempt is a unique, closable thread.** Threads are never reused, and the router can close one for good with a single transaction that also tells it whether a result got there first.
6. **The claims table is the source of truth.** The router reads claims and results by thread, never by advancing a cursor.
7. **Agent output is untrusted.** The verdict only decides whether the router tries to commit; the router's integrity checks decide whether the commit counts. Agent text goes to the redacted router log only.

## Part B4a: bus and harness

### Bus operations

The store schema goes from v2 to v3: `claims` gains `revoked_at`, `revoked_by`, `revoke_reason` and `confinement` (text, written only by the harness claim below). Migration follows the v1 → v2 rules: one IMMEDIATE transaction, one-way upgrade.

| Operation | Who | Does |
|---|---|---|
| `Claim(channel, thread)` | any member | Reads one claim row: owner, epoch, `claimed_at`, `released_at`, `completed_at`, `result_id`, `revoked_at`, `revoke_reason`, `confinement`. Works on an expired channel (a read). |
| `Message(channel, id)` | any member | Reads one message by ID. Works on an expired channel. |
| `Revoke(member, thread, reason)` | `router` or `supervisor` | One IMMEDIATE transaction that returns the final claim state. |
| `ClaimConfined(member, thread, marker)` | the harness (internal API, no CLI flag or MCP tool) | As `ClaimThread`, and stores `marker` in `confinement`. |

**`Revoke` cases:**

- The thread was **completed** with a result: nothing changes, and it returns `completed` with the `result_id`. The caller uses that as its decision: a result that got there first wins.
- The thread is **open** (claimed, not completed): sets `revoked_at`, `revoked_by`, `revoke_reason` and `released_at`, and returns `revoked`.
- The thread is **unclaimed**: inserts a row owned by the caller with `revoked_at` set, and returns `revoked`.
- The thread was **already revoked**: no change; it returns `revoked`.

**After a revoke:**

- `ClaimThread` fails with `thread revoked` (checked before "already completed").
- `Complete` fails with `lease_lost` (the revoke is checked before completion).
- `Heartbeat` lists the thread under `lost` with the reason.

**`marker`** is a small JSON object, `{"router_tasks":true,"landlock":true}`. The router (B4b) accepts only claims that carry it.

- A member that calls the ordinary `tb bus claim` or `tb_bus_claim` gets no marker, and a thread it already holds that way never gains one: `ClaimConfined` on it fails.
- The marker states what the `tb` harness process enforced; it is not a credential. Within the B1 trust boundary (a hostile process of the same user is out of scope), it answers whether a misconfigured harness took the task.

### Harness `router_tasks`

The wake file gains:

```yaml
router_tasks:
  from: router        # the only agent whose router-task assigns are honored
  timeout: 30m        # cap per task; the payload timeout applies if shorter
```

- `router_tasks` is allowed with `adapter: harness` or `exec`, and refused with `notify`.
- `--check` refuses it without Landlock: `--allow-unsandboxed` is incompatible with router tasks.

**Which messages are router tasks.** A `task.assign` is handled as a router task only when all of these hold:

- the sender is `agent:<from>`, and its role in the store is `router`;
- the payload is `router-task/0.1`;
- `ticket` matches `^TKT-\d{3,}$`;
- `workspace` resolves (all symlinks followed) to exactly `<canonical root>/.toolbelt/workspaces/<ticket>`, a real directory;
- `ticket_file` resolves to exactly `<canonical root>/.toolbelt/tickets/<ticket>.yaml`, a regular file;
- these paths are checked again right before the child starts, and the child gets the canonical ones;
- `thread` equals the message thread, and `Claim(thread)` shows it neither completed nor revoked.

Any other `task.assign` is an ordinary message.

**A dedicated task worker.** A harness with `router_tasks` runs no ordinary wakes: ordinary messages stay queued, untouched. Otherwise the agent's ordinary wakes would run with the repository root writable and with bus tools, and a `recv` in one of them could consume a task. The harness reads `task.assign` messages from its own task cursor (`task_seq` in its state, advanced before each claim), in order and at most 50 per tick. A restart never handles a task twice, and a flood of invalid assigns cannot starve a valid one. `wake_on` does not apply to router tasks.

**Handling one task:**

1. **Claim.** `ClaimConfined(thread, marker)`. If the claim fails (another worker won, or the thread was revoked), the harness logs `router_task_lost` and stops: no child, no workspace access.
2. **Prepare.**
   - It copies `ticket_file` to `<agent tmp>/task/ticket.yaml`. The live file is not granted, because ticket writes rename it.
   - It creates a fresh per-task tmp dir; the child writes `<task dir>/verdict.json` itself.
   - It refuses to run at all without Landlock, however the harness was opened (`router_task_refused`).
3. **Run the child**, confined. Its grants:
   - **write:** the workspace (its workdir), the task dir (with its own tmp) and the `sandbox.write` entries;
   - **read:** `<root>/.git` (so `git status` and `git diff` work; the index may stay unwritable), the task copy and the `sandbox.read` entries;
   - **refused, at `--check` and at wake time:** any `sandbox.write` entry that equals, contains or lies inside `<root>`, `<root>/.git` or `<root>/.toolbelt`, and any read grant (`sandbox.read` or the adapter binary's directories) that contains or lies inside `<root>/.toolbelt`. Paths are compared after resolving symlinks on both sides;
   - **no bus access:** the `claude` preset gets an empty MCP config, `router_tasks.allowed_tools` (default `Read`, `Edit`, `Write`, `Glob`, `Grep`; `mcp__*` refused) and `--disallowedTools Bash,WebFetch,WebSearch,Task`, and `exec` gets no session token;
   - **environment:** `TB_TASK_TICKET`, `TB_TASK_TICKET_FILE` (the copy), `TB_TASK_VERDICT` and `TMPDIR` (the task's tmp);
   - **a fresh session:** the `claude` preset starts a new session per task, with no `--resume`, so one ticket's context never leaks into the next;
   - **timeout:** the smallest of `router_tasks.timeout`, the payload's `timeout` and the adapter's `timeout` (which defaults to 1h when `router_tasks` is set).
4. **Fixed prompt** (`claude`), or a packet with `reason: "router_task"` and a `task` object `{ticket, thread, workspace, ticket_copy, verdict_file}` (`exec`). It tells the agent to:
   - work only in the current directory;
   - read the ticket from the copy;
   - not commit (git can read but the repository is not writable);
   - write `{"verdict":"PASS"}` or `{"verdict":"FAIL","summary":"..."}` to `verdict_file`, then exit.
5. **Watch.** While the child runs, a watcher goroutine (next to the heartbeat one) polls `Claim(thread)` every second. If it sees a revoke, a release, or an owner or epoch other than the harness's, it kills the child's process group and logs `router_task_revoked`.
6. **Complete.** After the child's process group has exited (the harness kills what is left of the group), the harness reads `verdict_file`. It must be a regular file of at most 4 KiB holding one JSON object.
   - It calls `Complete(thread, epoch, verdict)`.
   - Only `PASS` and `FAIL` are accepted; the summary is cut to 500 bytes on a rune boundary and redacted.
   - A missing, oversized, symlinked or malformed verdict, a crash or a timeout completes with `{"verdict":"FAIL","summary":"<reason>"}`. Here `<reason>` is fixed text from the harness, not the child's output.
   - A `lease_lost` on complete (revoked meanwhile) is logged, and nothing else happens.
   - On harness shutdown the child is stopped and nothing is completed (`router_task_canceled`): the lease runs out or the router revokes.
7. **Audit.** `router_task_start`, `router_task_ok`, `router_task_failed`, `router_task_lost`, `router_task_revoked`, `router_task_canceled` and `router_task_refused`, with ticket, thread and epoch. Never a payload or the verdict summary.

- The harness heartbeat keeps the claim's lease while the child runs, as in B2.
- `task.status` messages play no part: revokes are read from the claim row.
- **Landlock and retries.** A child's rule binds to the inode of the workspace directory at wake time. The router revokes before it resets a worktree, and the watcher kills the child on revoke, so no child of an old attempt is running when the directory is recreated. A child that is somehow still alive keeps its rule on the old inode and cannot write the new one.

## Part B4b: router bus slots

### Configuration

The bus settings come only from **trusted config**: the global config or `-c`, through `config.LoadTrusted`, never the repository's `.toolbelt/config.yaml`. An agent that can write the repository cannot repoint the router.

```yaml
router:
  max_workers: 2          # local slots w1..wM; 0 = bus only
  bus:
    channel: feature-x    # empty = no bus slots (today's behaviour)
    as: router            # the router's CLI session in that channel
    slots: 2              # bus slots b1..bN (default 1 when channel is set)
    to: role:worker       # task.assign recipient, or agent:<name> to pin one agent
    claim_timeout: 5m     # an attempt with no valid claim within this fails (unclaimed)
```

- **`RouterConfig` holds the bus block.** `Validate` accepts `max_workers: 0` only with `bus.channel`. `router.New` stops replacing the whole config when `max_workers` is 0.
- **`--workers`** cannot express bus-only (it only overrides when greater than 0); bus-only is set in config.
- **`preflight`** with `max_workers: 0` no longer requires `agent_cmd` or Landlock (the router runs no agent); with local slots, both stay required.
- **`tb router run --check` (new)** refuses each of the following, and runs nothing:
  - a missing session, or one whose role is not `router`;
  - an expired channel;
  - more than one `router`-role member on the channel (two routers would fight over the same tickets);
  - an invalid `bus.to`.

### Slots and scheduling

- The pool is `w1..wM` (local), then `b1..bN` (bus). Free slots fill in that order.
- `ValidWorkerID` becomes `^[wb]\d+$`; `tb watchdog kill` accepts both, and its error text lists both.
- Each bus attempt runs in its own goroutine. It polls the store every second and returns `workerDone` to the Run goroutine, which keeps all ticket state. Cancel goes through the slot's `cancel()`. The `done` channel is sized `M + N`.
- **Timeouts.** The ticket timeout starts at the claim: the slot gains `claimedAt`, used by the router watchdog and `tb watchdog status`. Before the claim, `claim_timeout` bounds the wait.
- Direct delivery does not apply to bus slots: dependents go to `bus.to`.

### Lifecycle of a bus attempt

**1. Prepare.** As for a local worker: ticket `running`, `Worker = bN`, `Attempts++`, worktree reset and created from the base.

The thread is `<ID>/<rand>.<attempt>`, for example `TKT-007/k3m9q2.2`, where `<rand>` is 6 random base32 characters per thread. It is stored in a new ticket field, `bus_thread`, outside `Result`:

- written at start;
- kept across a requeue;
- cleared only once the router has confirmed the thread completed or revoked.

**2. Assign.** One `task.assign` from the router:

- recipient: `bus.to`;
- thread: the attempt thread;
- idempotency key: `<thread>/assign`;
- `ttl`: `claim_timeout`;
- payload: `router-task/0.1`, below.

**3. Claim.** The goroutine polls `Claim(thread)`. A claim is valid only when all of these hold:

- the owner's role is `worker`;
- the owner is the addressee, when `bus.to` is `agent:X`;
- `claimed_at` is not before the assign's `created_at`;
- `confinement` carries the router-tasks marker.

An invalid claim is revoked at once (reason `claim_rejected`) and logged as `bus_claim_rejected`. The router then assigns again on a new thread within the same attempt; the attempt's claim deadline does not reset.

With no valid claim by the deadline, the router revokes (reason `unclaimed`) and fails the attempt with `error: unclaimed`. The attempt is spent, so `retry.max_attempts` bounds an absent worker.

**4. Work.** The goroutine records the valid claim's owner and epoch. Every tick, it revokes the thread and fails the attempt with `bus_lease_lost` on any of:

- an owner or epoch different from the recorded ones;
- `released_at` set;
- the owner not alive by the B2 definition (`now >= last_seen + lease`).

Timeout and cancel also revoke:

| Event | Revoke reason | Attempt |
|---|---|---|
| `slow_threshold` reached | (no revoke) | logs `worker_slow` |
| ticket timeout | `timeout` | fails with `worker_stuck` |
| ticket changed outside the router, or `tb watchdog kill bN` | `canceled` | slot freed |

The kill output says the harness's child is stopped by the harness's watcher, not by the router. *In this cut* every cancel (timeout, kill, ticket change, shutdown) revokes with reason `canceled`; lease loss uses `lease_lost`, a withdraw `unclaimed`, a rejected claim `claim_rejected`.

In every case the router acts on what `Revoke` returns. `completed` means a result got there first, and the router goes to step 5 with it.

**5. Result.** The goroutine sees `completed_at` with the recorded owner and epoch, or gets `completed` from a `Revoke`. It fetches `Message(result_id)` and checks that `from` is the owner and the thread is the attempt's.

- **`verdict: "PASS"`.** The router checks:
  - `symbolic-ref HEAD` is `refs/heads/tb-workspace/<ID>`;
  - `rev-parse HEAD` is the base;
  - the branch ref is the base.

  It then runs the same `commitWorkspace` as for a local worker. When there was something to commit, the new HEAD must have exactly one parent, the base. A PASS with no changes is `done` with `HeadCommit == BaseCommit`, a no-op for `tb merge`.
  - Any mismatch is an integrity error, escalated with no retry, as today.
  - A cancel that arrives during the commit is ignored until it ends.
- **`verdict: "FAIL"`, or any other value or shape:** the attempt fails (`bus_verdict_fail`) and is retried while attempts remain.
- **The commit subject** is `tb: <ID> <title>`, with control characters stripped from the title. This is new for both worker types.

**6. Retry, shutdown, restart, expiry.**

- **Retry:** the previous thread is revoked (or confirmed completed) before the worktree is reset. A failed reset (`RemoveAll` blocked) raises `workspace_reset_failed` and fails the ticket instead of looping.
- **Shutdown and the global timeout:** each bus attempt is revoked (reason `shutdown`). If `Revoke` returns `completed`, the result is processed; otherwise the attempt goes back to `pending` with `Attempts--`. The revoke is bounded by the store's 5 s busy timeout.
- **Restart:** `recover` revokes the `bus_thread` of every `running` ticket. *In this cut* a result a previous run left completed is not honored: the ticket goes back to `pending` and the next attempt redoes the work. If the revoke fails (expired channel, busy store), the ticket goes back to `pending`, `bus_thread` is kept, `bus_write_failed` is logged, and the revoke is retried on the next start before that ticket is assigned again.
- **Expired channel:** `Claim` and `Message` still work.
  - A running attempt whose claim shows a result is honored.
  - Others go back to `pending` with `bus_thread` kept.
  - Bus slots stay idle with one `bus_channel_expired` alert, and local slots keep running.
  - Bus write errors are logged (`bus_write_failed`) and never stop the loop.

### `router-task/0.1` payload

```json
{
  "v": "router-task/0.1",
  "ticket": "TKT-007",
  "thread": "TKT-007/k3m9q2.2",
  "attempt": 2,
  "title": "Add retry to the webhook client",
  "workspace": "/home/leonardo/src/app/.toolbelt/workspaces/TKT-007",
  "ticket_file": "/home/leonardo/src/app/.toolbelt/tickets/TKT-007.yaml",
  "branch": "tb-workspace/TKT-007",
  "base": "a1b2c3d4e5f6...",
  "timeout": "10m"
}
```

- Description, mandate and acceptance criteria are read from the ticket file (through the harness's copy). They are not in the message.
- `title` is the stored ticket title. It may come from a planner model, so it is data.

### Result payload

The harness writes it from the verdict file:

```json
{"verdict": "PASS", "summary": "optional, at most 500 bytes"}
```

- The router cuts `summary` to 500 bytes and strips control characters and ANSI escapes.
- It then applies the harness's redaction rules (shared from `busrun`) and writes the summary only to `.toolbelt/router/logs/<ID>.log`.
- Other fields are ignored. An agent `commit`, `head` or `branch` is never used.

### State

- `Ticket.bus_thread`, as above.
- `Result` gains `bus_agent` and `bus_epoch`, set at done from the claim row and carried in `workerDone`.
- `state.json` gains `thread`, `agent`, `claimed_at` and `phase` (`assigned`, `claimed`, `committing`) per bus slot. `tb watchdog status` shows them.
- New alert kinds: `bus_unclaimed`, `bus_claim_rejected`, `bus_lease_lost`, `bus_verdict_fail`, `bus_channel_expired`, `bus_write_failed`, `workspace_reset_failed`.
- `tb router run --dry-run` shows which slot (`wN` or `bN`) each ready ticket would take.

## Trust and limits

- **Confinement holds only while no member with the repository root writable is on the channel.** An agent of the default `harness` mode (workdir = repository root) can still edit `.git`, tickets or `bus.db` directly, whatever the router does. Run such agents (a reviewer, a chat) in another channel, or confine them too.
- **The marker** says the `tb` harness enforced confinement, which is what a misconfiguration would break. A hostile process of the same user is out of scope, as in B1.
- **`~/.claude` in `sandbox.write`** (which the `claude` CLI needs) lets the confined model write the user's Claude settings, including hooks that run later outside any sandbox. Prefer granting only `~/.claude/projects` and the credentials file, and treat a broader grant as trusting the model with the user's Claude configuration.
- **Mandate.** The router does not enforce `mandate.scope`/`forbidden` on paths. `tb stack` (`guard`, `review --ticket`) does, before merge.
- **One host.** Workspace paths are local.

## Out of B4

- A `trust: unconfined` mode (GUI or unsandboxed agents taking router tasks).
- Direct delivery for bus slots, and agent-keyed reservations.
- A withdrawal budget separate from `retry.max_attempts`.
- Bus columns in `tb board`, and card-based checks such as `max_concurrency`.
- L2, the escalation ladder by tier and cross-vendor review (ADR-013 §5).
- Agents on another host, and picking an agent by capability or tier.
- Enforcing the mandate on file paths.

## Acceptance criteria

### B4a

1. **Revoke on an open thread.** `Revoke` returns `revoked`. Afterwards a `ClaimThread` fails with `thread revoked`, `Complete` fails with `lease_lost`, and the owner's `Heartbeat` lists the thread under `lost` with the reason.
2. **Revoke on a completed thread.** It changes nothing and returns `completed` with the `result_id`. Revoke on an unclaimed thread blocks later claims.
3. **Concurrent `Revoke` and `Complete`:** exactly one wins, and `Revoke`'s return value says which.
4. **Migration.** A v2 store migrates to v3 and keeps its data; a v4 store is refused.
5. **Marker.** `ClaimConfined` stores the marker. `tb bus claim` and `tb_bus_claim` cannot set it.
6. **Happy path, with a fake router and a fake confined child.** The harness claims, the child edits the workspace and writes a `PASS` verdict, and the harness completes with that verdict and the right epoch.
7. **A worker that loses the claim** never spawns a child and logs `router_task_lost`.
8. **Grants, on a Landlock host.** The child can:
   - write the workspace;
   - read `<root>/.git` (`git status` works) and the ticket copy.

   It cannot:
   - write `<root>`, `<root>/.git/config`, `<root>/.toolbelt/bus/bus.db` or the live ticket file;
   - open `bus.db` at all.
9. **Refused `sandbox` entries.** A `sandbox.write` entry of `<root>`, `~` (when the root is inside it) or `<root>/.toolbelt` is refused at `--check` and at wake time.
10. **Revoke during the child.** A revoke kills the child's process group within about 2 s and logs `router_task_revoked`; no complete follows.
11. **Bad verdicts.** A missing, oversized or invalid verdict file, a child crash, and a timeout each complete as `FAIL` with fixed harness text.
12. **Not router tasks.** Each of these is handled as an ordinary message, with no claim:
    - a `router-task` from a non-pinned router-role agent;
    - one from a supervisor;
    - one with `ticket: ../x`;
    - one with a symlinked workspace;
    - one whose thread differs from the message's.
13. **No rewake, no double handling.** A handled task does not rewake, and a harness restart does not handle it twice.
14. **Fresh sessions.** The `claude` preset gets a new session per task (no `--resume`) and no bus tools.

### B4b

15. **Happy path.** Bus only, with a B4a worker:
    - the ticket ends `done` with one commit `tb: <ID> <title>` whose only parent is the base;
    - `bus_agent` and `bus_epoch` are set;
    - `tb merge <ID>` applies it unchanged.
16. **Assign.** The payload matches `router-task/0.1`, with thread `<ID>/<rand>.<attempt>` and `ttl` = `claim_timeout`. It carries no description, mandate or acceptance text.
17. **Retry.** `FAIL` or a malformed result retries on a new thread with a reset worktree; after `max_attempts` the ticket is `failed`.
18. **Agent commits.** An agent commit, a branch switch or a detached HEAD fails the pre-commit check and escalates with no retry. A result naming another SHA has no effect. A no-change PASS is `done` with `HeadCommit == BaseCommit`.
19. **Invalid claims.** Each is revoked at once and the task is reassigned on a new thread:
    - a claim with no marker;
    - a claim by a reviewer;
    - with `bus.to: agent:a`, a claim by `b`;
    - a claim made before the assign.

    With no valid claim by `claim_timeout`, the attempt fails as `unclaimed`.
20. **Lease lost.** For each, the router revokes the thread, the attempt fails with `bus_lease_lost`, and the owner's later complete gets `lease_lost`:
    - an owner silent past its lease, with no human running the watchdog;
    - a takeover by another worker.
21. **Timeout.** `worker_slow`, then revoke and `worker_stuck`. A `PASS` completed before the revoke counts, through `Revoke`'s `completed` return.
22. **Stale results.** None of these ever marks a ticket `done`:
    - a result from an earlier router run;
    - one from a recreated board with the same ticket IDs;
    - one on a revoked thread.
23. **Restart.**
    - A restart revokes open threads, honors a completed one and requeues the rest.
    - With the channel expired, `bus_thread` is kept and revoked on a later start before the ticket is reassigned.
24. **Shutdown and the global timeout** end with no hang while bus slots are busy. An attempt in `committing` finishes its commit.
25. **No cursor.** `tb bus recv --as router` during a run changes nothing the router sees.
26. **Mixed slots.** Local and bus slots run side by side; local slots fill first.
27. **Expired channel mid-attempt.** An attempt with a stored result is honored, the others go back to `pending`, one `bus_channel_expired` alert is raised, and local slots keep working.
28. **Trusted config.** `router.bus` set in the repository's `.toolbelt/config.yaml` is ignored; only the global config or `-c` counts.
29. **`--check` and `preflight`.**
    - `--check` refuses a missing session, a non-router role, an expired channel and a second router-role member.
    - `preflight` refuses `max_workers: 0` without `bus.channel`, and accepts it without `agent_cmd` and without Landlock when `bus.channel` is set.
30. **Commit subject.** A title with control characters yields a commit subject without them, for local and bus workers alike.
31. **Summary.** `summary` appears only in the router log, cut, stripped and redacted, never in the ticket YAML.
32. **Watchdog kill.** `tb watchdog kill b1` revokes the attempt and frees the slot; the harness watcher stops the child.
33. **Dry run.** `--dry-run` shows the `wN`/`bN` slot of each ready ticket, and `state.json` shows a bus slot's thread, agent and phase.
