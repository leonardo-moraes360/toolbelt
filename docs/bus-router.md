🇺🇸 English | [🇧🇷 Português](pt-BR/bus-router.md)

# Router on the bus — B4 spec

Status: **draft, round 2** (one fresh-context review applied; not implemented).

Specification for stage **B4** of [ADR-013](adr/013-barramento-multi-vendor.md) (§8): the Phase 3 router ([ADR-009](adr/009-router-e-workers.md)) gains a second worker type, an agent registered on a bus channel. It builds on the protocol ([bus-protocolo.md](bus-protocolo.md)), the lease ([bus-lease.md](bus-lease.md)) and the harness ([bus-harness.md](bus-harness.md), [bus-notify-discover.md](bus-notify-discover.md)).

## Problem

`tb router run` today runs one single-use `router.agent_cmd` per ticket, in the Landlock sandbox, confined to the ticket's worktree. A persistent agent of another vendor (a Codex or Claude Code session woken by `tb bus run`) cannot take a ticket: there is no way to hand it one, to know it is working, or to get its result back to `tb merge`.

## Principles

1. **Same confinement as a local worker.** A bus agent that takes a router task runs confined to that ticket's worktree, by its harness. An agent that can write the repository root can rewrite `.git/config` filters, `.git/worktrees/`, tickets, router state and `bus.db`, so no check by the router would mean anything. Unconfined agents are an explicit, documented opt-in.
2. **The router stays the only writer of tickets and commits.** A bus agent edits files in the worktree and reports a verdict. The router commits, checks and records.
3. **Same result contract.** `tb merge` keeps reading `Result.BaseCommit`/`HeadCommit` from the local repository and does not change.
4. **Every attempt is a unique, closable thread.** Threads are never reused, across attempts, router runs or a recreated board, and the router can close one for good.
5. **The claims table is the source of truth.** The router reads claims and results by thread, never by advancing a cursor another process could share.
6. **Agent output is untrusted.** The verdict only decides whether the router tries to commit; the router's integrity checks decide whether the commit counts. Agent text goes to the redacted router log, never into a ticket field or a prompt.

## Setup

1. The human creates the channel and a `router` invite, and joins the router from the CLI:

   ```bash
   tb bus channel create feature-x --owner leonardo
   tb bus invite feature-x --role router --name router
   tb bus join feature-x --invite <code> --name router --role router
   ```

2. Each worker joins with `--role worker` and runs `tb bus run` with the `harness` or `exec` adapter and `router_tasks: true` in its wake file (below).
3. The router config:

   ```yaml
   router:
     max_workers: 2          # local slots w1..wM; 0 = bus only
     bus:
       channel: feature-x    # empty = no bus slots (today's behaviour)
       as: router            # the router's CLI session in that channel
       slots: 2              # bus slots b1..bN (default 1 when channel is set)
       to: role:worker       # task.assign recipient, or agent:<name> to pin one agent
       claim_timeout: 5m     # an assign nobody claims within this is withdrawn
       max_withdrawals: 3    # per ticket per run; then the ticket fails (unclaimed)
       trust: confined       # confined | unconfined (see Trust)
   ```

- `max_workers: 0` becomes valid only with `bus.channel` set; `router.New` stops replacing the whole config with defaults when `max_workers` is 0, and `RouterConfig.Validate` accepts it in that case.
- With `max_workers: 0`, `preflight` no longer requires `agent_cmd` or Landlock (the router runs no agent). With local slots, both stay required.
- `tb router run --check` (new) validates the session, its `router` role, the channel being active, the slot counts and `bus.to`, and warns when the channel has more than one `router`-role agent or when `bus.to` pins an agent whose card says `max_concurrency` below `slots`. It runs nothing.

## Bus changes

B4 needs three store operations, all on the router's member (role `router`; `supervisor` also allowed for `Revoke`):

| Operation | Does |
|---|---|
| `Claim(channel, thread)` | reads one claim row: owner, epoch, `claimed_at`, `released_at`, `completed_at`, `result_id`. Readable by any member. |
| `Message(channel, id)` | reads one message by ID (to fetch the `task.result` a claim points at). |
| `Revoke(member, thread, reason)` | closes a thread for good: if unclaimed, inserts a claim row owned by the caller, completed with no result; if claimed and open, sets `released_at` and `completed_at` and sends `task.status {"state":"revoked","thread":T,"reason":R}` to `agent:<owner>`. A later `claim` fails with "already completed", a later `complete` with `lease_lost`. Router and supervisor only. |

- `Revoke` is the fence the router lacked: it stops a stale owner's result from ever counting and makes a late claim on a withdrawn thread fail.
- No other bus rule changes. `task.status` stays sendable by anyone, so consumers must check the sender (below).

## Slots and scheduling

- The pool is `w1..wM` (local), then `b1..bN` (bus). Free slots fill in that order, so local slots keep priority.
- `ValidWorkerID` becomes `^[wb]\d+$`; `tb watchdog kill` accepts both, and its error text lists both forms.
- Timeouts, `retry.max_attempts`, the global timeout and `slow_threshold` apply unchanged. The ticket timeout starts at the claim, not at the assign; the wait for a claim is bounded by `claim_timeout` instead.
- **Direct delivery does not apply to bus slots in B4**: a dependent of a ticket finished on a bus slot goes to `bus.to` like any other ticket (subscriptions are keyed by slot today, and a bus agent is not a slot).
- Each bus attempt runs in its own goroutine, like a local worker: it polls the store every tick (default 1s) and returns `workerDone` to the Run goroutine, which keeps all ticket state. Cancel goes through the slot's `cancel()`. The `done` channel is sized `M + N`.

## Lifecycle of a bus attempt

### 1. Prepare

As for a local worker: ticket `running`, `Worker = bN`, `Attempts++`, worktree `.toolbelt/workspaces/<ID>` on branch `tb-workspace/<ID>` from the base, reset first.

The thread is `<ID>/<run>.<attempt>`, for example `TKT-007/k3m9q2.2`:

- `<run>` is 6 random base32 characters chosen when the router starts;
- it is stored in the ticket as `Result.bus_thread` **at start**, so `recover` and the result check know it.

A board recreated with IDs from `TKT-001`, a second run, or a requeued attempt never reuses a thread.

### 2. Assign

One `task.assign` from the router:

- recipient: `bus.to`;
- thread: the attempt thread;
- idempotency key: `<thread>/assign`;
- payload: `router-task/0.1`, below.

### 3. Claim

The goroutine polls `Claim(thread)`. A claim counts only when all of these hold:

- the owner's role is `worker`;
- when `bus.to` is `agent:X`, the owner is `X`;
- `claimed_at` is not before the assign's `created_at`;
- with `trust: confined`, the owner's Agent Card says `sandbox: landlock` and `wake` is `harness` or `exec`.

A claim that fails any of these is revoked (`Revoke`, reason `claim_rejected`), logged as `bus_claim_rejected`, and the thread is withdrawn as below.

**Withdraw.** With no valid claim after `claim_timeout`, the router:

1. revokes the thread (reason `unclaimed`);
2. puts the ticket back to `pending` without spending the attempt (`Attempts--`, safe because the next try gets a new thread);
3. logs `bus_unclaimed`.

After `max_withdrawals` withdrawals in one run, the ticket goes `failed` with `error: unclaimed` instead.

### 4. Work

The goroutine records the owner and epoch of the valid claim. On every tick it treats as **lease lost**:

- an owner or epoch different from the recorded ones;
- `released_at` set;
- the owner not alive (B2 lease) for a full lease. The router then revokes the thread itself (reason `lease_expired`) instead of waiting for a human to run `tb bus watchdog`.

On lease loss the attempt fails with `bus_lease_lost`.

Timeout and cancel:

- at `slow_threshold`, the router logs `worker_slow`;
- at the timeout, it revokes (reason `timeout`) and fails the attempt (`worker_stuck`);
- a ticket changed outside the router, or `tb watchdog kill bN`, revokes (reason `canceled`) and frees the slot. The kill output says that the remote process is not stopped, only fenced.

### 5. Result

The agent completes with:

```bash
tb bus complete TKT-007/k3m9q2.2 --epoch E --result '{"verdict":"PASS"}'
```

The goroutine sees `completed_at` set on the claim, with the recorded owner and epoch. It fetches `Message(result_id)` and checks that its `from` is the owner and its thread is the attempt thread.

- **The first terminal event in store order wins.** A result completed before the router revoked counts, even if the router noticed it later (as a local worker that exited just before its cancel).
- **`verdict: "PASS"`.** The router checks before committing:
  - `symbolic-ref HEAD` is `refs/heads/tb-workspace/<ID>`;
  - `rev-parse HEAD` is still the base: the agent did not commit, switch branch or detach;
  - the branch ref is still the base.

  It then runs the same `commitWorkspace` as a local worker (`verifyWorkspace`, no nested `.git`, no gitlinks or `.gitmodules`, hardened `git add -A`, commit `tb: <ID> <title>`) and checks that the new HEAD has exactly one parent, the base.
  - Any mismatch fails the attempt as an integrity error, escalated with no retry, as today.
  - A PASS with no changes is `done` with `HeadCommit == BaseCommit`, as for a local worker; `tb merge` treats it as a no-op.
- **`verdict: "FAIL"`, or any other value or shape:** the attempt fails (`bus_verdict_fail`) and is retried while attempts remain.

### 6. Retry, shutdown, restart

- **Retry.** A new attempt resets the worktree and opens a new thread. A confined stale agent cannot write into the new worktree: its Landlock rule names the old directory, and the reset recreates it.
- **Shutdown and the global timeout.** Each running bus attempt is revoked (reason `shutdown`) and put back to `pending` with `Attempts--`, as local workers are.
- **Restart.** `recover` revokes the `bus_thread` of every `running` bus ticket before putting it back to `pending`.
- **Expired channel.** A channel past its `--ttl` refuses writes, so:
  - bus write errors are logged and never stop the loop;
  - running bus attempts are put back to `pending` at once;
  - bus slots stay idle with one `bus_channel_expired` alert, and local slots keep running.

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
  "claim_by": "2026-11-20T18:05:00Z",
  "timeout": "10m"
}
```

- The description, mandate and acceptance criteria are read from `ticket_file`; they are not copied into the message.
- `title` is the ticket's title as stored. It may have been written by a planner model, so it is treated as data. It goes into the commit message only after control characters are stripped, as today.

### Result payload

```json
{"verdict": "PASS", "summary": "optional, at most 500 bytes"}
```

- `summary` is cut to 500 bytes, stripped of control characters and ANSI escapes, redacted again, and written only to `.toolbelt/router/logs/<ID>.log`.
- Any other field is ignored. A `commit`, `head` or `branch` from the agent is never used.

## Worker side: `router_tasks`

The harness gains a wake-file option, `router_tasks: true` (adapters `harness` and `exec`; refused with `notify`). With it, a `task.assign` is handled as a router task only when all of these hold:

- its sender's role is `router` (looked up in the store, not taken from the payload);
- the payload is `router-task/0.1`;
- `workspace` is exactly `<root>/.toolbelt/workspaces/<ticket>` and exists;
- `ticket_file` is exactly `<root>/.toolbelt/tickets/<ticket>.yaml`;
- `thread` equals the message thread.

Anything else is an ordinary message: woken as usual, with no workspace.

For a valid router task, the harness wakes the agent **once per task**, with:

- **workdir** = the payload `workspace`, the only writable path besides the agent's own temp dir;
- **read-only** grant on `ticket_file`;
- **no grant** on the repository root, `.git` or `.toolbelt` (the worktree's `.git` file stays readable, as for local workers);
- a **fixed prompt** naming the ticket and thread, and telling the agent to:
  - claim the thread (`tb_bus_claim`);
  - work only in its current directory;
  - not commit;
  - complete with `{"verdict":"PASS"}` or `{"verdict":"FAIL","summary":"..."}`, passing the epoch from the claim.

The harness also:

- acts on `task.status` `revoked`/`canceled` only when the sender's role is `router` or `supervisor`, and ignores them otherwise;
- logs `router_task` in its audit (ticket and thread, no payload);
- adds `task.status` to the wake types for router tasks, so a revoke wakes the agent and the prompt tells it to stop.

The router ignores every `task.status` except its own revokes (it reads claims, not status messages). The harness's `wake_failed` status is ignored too; the claim timeout covers it.

## Trust

- **`trust: confined` (default).**
  - Only claims from agents whose card declares `sandbox: landlock` and a `harness` or `exec` wake are accepted.
  - The declaration is the agent's own: the router cannot inspect another process. That is the B1 trust boundary (a hostile process of the same user is out of scope); what confinement protects against is the agent's model writing outside its task.
  - With `router_tasks`, the harness enforces it.
- **`trust: unconfined`** accepts any `worker` claim, including GUI agents (`notify`), `--allow-unsandboxed` harnesses and harnesses without `router_tasks`.
  - `tb router run` prints a warning at start and on each such claim (`bus_unconfined_claim`).
  - In this mode an agent can write the repository root. The router's integrity checks, the tickets and the `tb merge` guarantees do not hold against it. Use it only for agents you would let run `git` in the repository yourself.
- **Mandate.** As today, the router does not enforce `mandate.scope`/`forbidden` on paths; `tb stack` (`guard`, `review --ticket`) does, before merge.
- **One host.** Workspace paths are local; a bus agent on another machine cannot take a router task.

## State, watchdog and board

- `state.json` gains, per bus slot, `thread`, `agent` (once claimed), `claimed_at` and `phase` (`assigned`, `claimed`, `committing`).
- `tb watchdog status` and `tb board` show them for running bus tickets.
- New alert kinds: `bus_unclaimed`, `bus_claim_rejected`, `bus_lease_lost`, `bus_verdict_fail`, `bus_unconfined_claim`, `bus_channel_expired`, `bus_write_failed`.
- `tb router run --dry-run` shows which slot (`wN` or `bN`) each ready ticket would take.
- `Result` gains `bus_thread` (set at start), `bus_agent` and `bus_epoch` (set at done, from the claim row, never from the payload).

## Out of B4

- Direct delivery for bus slots, and agent-keyed reservations.
- L2 (local-model triage and summaries), the escalation ladder by tier and cross-vendor review (ADR-013 §5).
- Agents on another host.
- Picking an agent by capability or tier from its card.
- Enforcing the mandate on file paths.
- Router tasks for the `claude` preset with a workdir other than the payload's (the preset's `harness.workdir` is ignored for router tasks).

## Acceptance criteria

1. **Happy path.** Bus only (`max_workers: 0`), with a fake confined worker that claims, edits the worktree and completes `PASS`:
   - the ticket ends `done`, with a single commit `tb: <ID> <title>` on `tb-workspace/<ID>` whose only parent is the base;
   - `bus_thread`, `bus_agent` and `bus_epoch` are set;
   - `tb merge <ID>` applies the commit unchanged.
2. **Assign.** The `task.assign` payload matches `router-task/0.1` and goes to `bus.to` with thread `<ID>/<run>.<attempt>`. It carries no description, mandate or acceptance text.
3. **Retry.** `FAIL` or a malformed result retries on a new thread with a reset worktree; after `max_attempts` the ticket is `failed`.
4. **Agent commits.** An agent that commits by itself, switches branch, or detaches HEAD fails the pre-commit check and escalates with no retry. An agent commit holding a gitlink never reaches `done`. A result naming another SHA has no effect.
5. **Withdraw.** With no claim within `claim_timeout`:
   - the thread is revoked and a later claim on it fails with "already completed";
   - the ticket goes back to `pending` with the attempt not spent, and `bus_unclaimed` is logged;
   - after `max_withdrawals` the ticket is `failed`.
6. **Invalid claims.** Each is revoked with `bus_claim_rejected`:
   - a claim by a reviewer;
   - with `bus.to: agent:a`, a claim by agent `b`;
   - a claim made before the assign;
   - under `confined`, a claim by an agent whose card says `sandbox: none`.
7. **Lease lost.** For each, the attempt fails with `bus_lease_lost`, and the owner's later `complete` returns `lease_lost`:
   - an owner silent past its lease, with no human running the watchdog;
   - a takeover by another worker.
8. **Timeout.** `worker_slow` at the threshold; at the timeout, revoke and `worker_stuck`. A `PASS` stored before the revoke still counts.
9. **Stale results.** None of these ever marks a ticket `done`:
   - a `task.result` from an earlier router run;
   - one for a same-named ticket on a recreated board;
   - one for a revoked thread.
10. **Restart and shutdown.**
    - A router restart revokes the open thread of running bus tickets and puts them back to `pending`.
    - Shutdown and the global timeout end with no hang while bus slots are busy.
11. **No cursor.** A human running `tb bus recv --as router` during a run does not change what the router sees.
12. **Mixed slots.** Local and bus slots run side by side; local slots fill first.
13. **Expired channel mid-attempt.** Bus attempts go back to `pending`, one `bus_channel_expired` alert, and local slots keep working.
14. **`--check` and `preflight`.**
    - `--check` refuses a missing session, a non-router role and an expired channel, and warns on a second router-role agent.
    - `preflight` refuses `max_workers: 0` without `bus.channel`, and accepts it without `agent_cmd` and without Landlock when `bus.channel` is set.
15. **Summary.** `summary` appears only in the router log, cut, stripped and redacted, never in the ticket YAML.
16. **Harness `router_tasks`.**
    - A valid router task runs the child with the workspace as its only writable path, plus a read-only `ticket_file`; on a Landlock host, a write to the repository root or to `.git/config` fails.
    - A `router-task` sent by a supervisor, or one with `workspace` outside `.toolbelt/workspaces/<ticket>`, is woken as an ordinary message with no workspace.
17. **Forged status.** A `task.status` `revoked` sent by a worker to another worker is ignored by that worker's harness, and the router never acts on it.
18. **Unconfined.** Under `trust: unconfined`, a `notify` agent's claim is accepted with a start warning and a `bus_unconfined_claim` alert.
19. **Watchdog kill.** `tb watchdog kill b1` revokes the attempt, frees the slot and reports that the remote process is not stopped.
