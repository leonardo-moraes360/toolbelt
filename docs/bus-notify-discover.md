🇺🇸 English | [🇧🇷 Português](pt-BR/bus-notify-discover.md)

# Bus `notify` adapter and `tb bus discover` — B3 remainder spec

Status: **implemented** (B3 remainder).

Closes stage **B3** of [ADR-013](adr/013-barramento-multi-vendor.md). It extends the harness ([bus-harness.md](bus-harness.md)), whose first cut delivered the `exec` and `harness` adapters, and builds on the lease of B2 ([bus-lease.md](bus-lease.md)).

## Problem

- **GUI-only agents.** An agent that lives in a desktop app (an MCP client with no headless mode) cannot be woken by a process. Today the human learns that it has work only by running `tb bus log`, or by wiring `exec` to `notify-send` by hand.
- **Onboarding.** Before writing a wake file the human has to know which agents the machine has, which of them `tb` can wake, and which already point an MCP config at `tb`. ADR-013 calls this introspection: a catalog that says what is possible, and is neither consent nor a channel.

## Principles

1. **The human is the wake target.** `notify` tells the human, never the agent. The agent still reads with `recv` when the human opens a turn.
2. **No content in a notification.** It carries counts, message types and sender names only, never a payload, thread or any other free-form string.
3. **Discovery reads, never acts.** `discover` creates no invite, writes no file, joins nothing, opens no connection and executes no binary.
4. **No secrets in the catalog.** MCP configs are read for the shape of the `tb mcp` entry only; other servers' commands, arguments and environment never leave the process, and parse errors are not printed.
5. **CLI only.** Neither the notification settings nor the catalog is exposed over MCP: the catalog describes the human's machine, and an agent has no use for it.

## Part 1: `notify` adapter

### Onboarding a GUI agent

`tb bus run` needs the agent's CLI session (`<private>/sessions/<channel>/<name>/`), which only `tb bus join` writes. A GUI agent therefore joins from the CLI and its client is bound to that session:

1. The human runs `tb bus join <channel> --invite C --name <name> --wake notify` (or `tb bus run --invite C ...`).
2. The human points the GUI client's MCP config at `tb mcp --bus-as <channel>/<name> --bus-root <root>`, the same binding the `harness` adapter uses.
3. The agent never calls `tb_bus_join`: a second join would replace the token and invalidate the harness session. With `--bus-as` the connection is already bound, and `tb_bus_join` on it fails with `already in channel`.

### Configuration

The wake file (`<private>/wake/<channel>/<name>.yaml`, `0600`) gains `adapter: notify`:

```yaml
adapter: notify
notify:
  backend: desktop         # desktop | terminal
  command: ""              # desktop only: absolute path; empty = first of the fixed list below
  urgency: normal          # low | normal | critical
  urgent_urgency: critical # urgency for batches with an urgent message; low | normal | critical
rewake_after: 1h           # default for notify (10m for the other adapters)
timeout: 10s               # default for notify (30s for exec)
```

- The common fields `wake_on`, `wake_topics`, `coalesce` and `budget` keep their meaning.
- These are `Validate` errors, so both `tb bus run` and `--check` refuse them:
  - a non-empty `pass_env`, `secret_files`, `sandbox.read` or `sandbox.write` with `adapter: notify` (the backend needs none of them);
  - an `exec:` or `harness:` block with `adapter: notify`, and a `notify:` block with another adapter;
  - `backend: desktop` on a platform other than Linux or macOS (`notify_unsupported`).
- Defaults depend on the adapter: `rewake_after` is `1h` and `timeout` is `10s` for `notify`.

### Backends

| Backend | Platform | Runs |
|---|---|---|
| `desktop` | Linux | `<command> --app-name toolbelt --urgency <u> -- <summary> <body>` |
| `desktop` | macOS | `/usr/bin/osascript -e 'on run argv' -e 'display notification (item 1 of argv) with title (item 2 of argv)' -e 'end run' <body> <summary>` (no `--`: the body starts with a count, never with `-`) |
| `terminal` | any | no child: one line written by the harness itself |

**Desktop binary.**

- `command` empty means the first existing file of `/usr/bin/notify-send`, `/usr/local/bin/notify-send`. There is no `PATH` search; on macOS the path is always `/usr/bin/osascript`.
- The path is resolved once, when the harness opens (symlinks followed), and that file is what every wake runs.
- `Validate` refuses the binary when:
  - it, or its parent directory, is not owned by root or is group- or world-writable;
  - it sits under the repository root or under `<private>`.
  
  An agent with shell access can write `~/.local/bin/notify-send`; it cannot replace a root-owned system binary.
- The text goes in as `argv` items, never through a shell, and never inside the AppleScript source.

**Sandbox and its limit.**

- The child runs in the `tb` Landlock sandbox like the other adapters. It can read the system directories and its executable's directory, and write nothing beyond the default workdir.
- `XDG_RUNTIME_DIR` gets no grant: Landlock (up to ABI 5) does not control `connect()` on unix sockets, so the D-Bus socket works without one.
- Without Landlock the rule of the other adapters applies: refused unless `--allow-unsandboxed`.
- The session bus is a capability Landlock does not limit: a process on it can ask the user's service manager to start commands outside the sandbox. This is accepted for `notify` because the binary is a root-owned system file that `tb` resolved, and its input is only the validated text below. The sandbox guards against file access by that binary, not against a hostile binary; the ownership check is what keeps a hostile one out.

**Environment.** The desktop child gets only:

- `PATH`, `HOME`, `LANG` and `LC_*`;
- when set, `DBUS_SESSION_BUS_ADDRESS` and `XDG_RUNTIME_DIR`.

`DISPLAY`, `WAYLAND_DISPLAY`, the `TB_BUS_*` variables of `exec` and everything else are dropped.

**Terminal.** The harness writes the line itself, to its own stderr (an injectable writer in code), followed by the bell (`\a`). There is no child, so no sandbox check and no `--allow-unsandboxed`. The write never fails the wake unless stderr is closed.

### Text

The strings are fixed English output on every locale:

```
summary: toolbelt: marin has 3 new messages in feature-x
body:    2 task.assign, 1 query from "claude-code". Open marin and ask it to call tb_bus_recv. Peek from a shell: tb bus recv --channel feature-x --as marin --peek
```

- **Only bounded values.**
  - Channel and agent names match `ValidName` (`[a-z0-9][a-z0-9._-]`, at most 63 characters).
  - Types come from the closed `Type` enum.
  - No thread, payload, `causation_id` or idempotency key goes in.
- **Sender names are agent-chosen.** An unbound invite lets the joiner pick any valid name, so a name can read like an instruction. Senders are therefore shown quoted, with the `agent:` prefix stripped and each one cut to 32 characters with `…`. `system:watchdog` and any `system:*` sender show as `the broker`.
- At most 3 senders, sorted by count and then name, followed by `and N others`.
- A `rewake` replaces `new` with `still unread`. When the batch has an `urgent` message, the summary starts with `urgent: ` and the urgency is `urgent_urgency`. Any member can send `urgent`, so on GNOME and KDE `critical` bypasses Do Not Disturb; set `urgent_urgency: normal` to prevent that. The budget caps it at `max_wakes` per `per` either way.
- **The shell hint uses `--peek`.** A plain `recv` would advance the agent's shared cursor, and the GUI agent's `tb_bus_recv` would then return nothing.

### Packet counts

The wake packet `messages` list stops at 20 items, so the text cannot be built from it. `bus-wake/0.1` gains two additive fields, filled for every adapter from all the messages of the batch:

```json
"types": {"query": 1, "task.assign": 2},
"senders": [{"from": "agent:claude-code", "count": 3}]
```

`senders` is sorted by count descending, then by `from`, and is capped at 20 entries plus `"senders_more": N`. The `exec` adapter receives them too; consumers that do not know them ignore them.

### Delivery semantics

- A notification is a wake: coalescing, budget, retries, `wake_failed` and the audit work as in [bus-harness.md](bus-harness.md). The default budget stays `6` per `1h`, which also caps how often the human is pinged.
- **Exit 0 means the text was handed to the OS, not that the human saw it.** For example, macOS drops notifications silently when they are disabled for the calling app, and there is no GUI session under `launchd` or SSH. The only real confirmation is the agent's cursor:
  - after `rewake_after` (default `1h` here, since a human may take a while), an unread packet gets one reminder (`reason: rewake`);
  - if it stays unread, the harness logs `wake_unread`.
- The audit event gains a `backend` field (`desktop` or `terminal`) on `wake_start`, `wake_ok` and the failure events. The notification text is never written.

### Agent Card and warnings

- `wake: notify` already exists as a card value.
- When the wake file says `adapter: notify` and the agent's card says another `wake`, both `--check` and the opening of `tb bus run` print `warning: wake_mismatch (card says harness, wake file says notify)` to stderr and go on with exit 0. The card is informational for the channel; the wake file rules.
- Warnings come from a new `Warnings()` call next to `Check()`; the CLI prints them for both.

## Part 2: `tb bus discover`

### Command

```bash
tb bus discover
tb bus discover --json
tb bus discover --cwd ~/src/app   # read that directory's .mcp.json instead of the repository root's
```

`discover` needs neither `.toolbelt/` nor a bus. Without `--cwd`, it reads the `.mcp.json` of the git repository that contains the current directory. Outside a repository, the project section reports `"project": null`.

### What it inspects

**1. CLIs on `PATH`**, from a built-in table of known agent CLIs. The table lives in code and in this spec. A CLI that is not in it is never reported, and adding a row is a code change.

| CLI | Vendor | Headless/resume | `tb` preset | Suggested wake |
|---|---|---|---|---|
| `claude` | anthropic | yes | `claude` | `harness` |
| `codex` | openai | yes | none yet | `exec` |
| `gemini` | google | yes | none yet | `exec` |
| `opencode` | sst | yes | none yet | `exec` |
| `aider` | aider | no | none | `manual` |

- Each `PATH` entry is looked up in order. Relative or empty entries are skipped and listed under `skipped_path_entries`, since they depend on the current directory.
- For each CLI found: the absolute path, its symlink target and whether the file is writable by anyone other than its owner (`writable_by_others`). A hint, not a refusal.
- Nothing is executed. There is no version probe in B3: many agent CLIs contact the network on `--version`.

**2. MCP client configs**, at these paths:

| `client` | Path | Read |
|---|---|---|
| `claude-code` | `~/.claude.json` | `mcpServers` at the top level (`scope: user`) and in each `projects.<path>` that has at least one server |
| `claude-desktop` | `~/.config/Claude/claude_desktop_config.json` (Linux), `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) | `mcpServers` |
| `cursor` | `~/.cursor/mcp.json` | `mcpServers` |
| `project` | `<cwd or repo root>/.mcp.json` | `mcpServers` |
| `codex` | `~/.codex/config.toml` | existence only (`parsed: false`, no TOML dependency in B3) |

**Safe read:**

- `Lstat` first: a symlink, FIFO, device or directory is reported as `"kind": "symlink" | "other"` and never opened.
- Regular files are opened with `O_NOFOLLOW|O_NONBLOCK`, checked again with `fstat`, and refused above 4 MiB (`too_large`).
- A parse error reports `"parses": false` and nothing else: the parser's message, which can quote file bytes, is dropped.

**Matching the `tb mcp` entry.** A server entry counts as `tb` when both hold:

- `command` is an absolute path whose basename is `tb`, or the bare name `tb`;
- `args[0] == "mcp"`.

A bare `tb` is reported as `"path": "tb"` with `"same_tb": null`, because it resolves on the client's `PATH`, not on ours. Shell wrappers (`sh -c ...`, `npx`, `env ...`) are not interpreted: they count as `"other"`.

**What is reported per entry:**

- for the matched entry: the server key, the absolute path, whether it is the same file as the running `tb` (`os.SameFile`), and whether `args` contains `--bus-as` (its value is a channel/name pair, which is printed);
- `--bus-root`, other arguments and `env` of the `tb` entry are never printed;
- other servers are only counted (`"other_servers": N`).

### Output

`--json` prints the schema below: stable keys, arrays sorted by name or path, `~` never expanded in output (absolute paths only).

```json
{
  "v": "bus-discover/0.1",
  "tb": "/home/leonardo/go/bin/tb",
  "skipped_path_entries": ["node_modules/.bin"],
  "clis": [
    {"name": "claude", "vendor": "anthropic", "path": "/home/leonardo/.local/bin/claude",
     "target": "/home/leonardo/.local/share/claude/versions/2.1.0", "writable_by_others": false,
     "headless": true, "preset": "claude", "wake": "harness"}
  ],
  "mcp_configs": [
    {"client": "claude-code", "path": "/home/leonardo/.claude.json", "kind": "file", "parses": true,
     "scope": "user", "tb": [{"server": "toolbelt", "path": "/home/leonardo/go/bin/tb", "same_tb": true, "bus_as": null}],
     "other_servers": 2},
    {"client": "claude-code", "path": "/home/leonardo/.claude.json", "kind": "file", "parses": true,
     "scope": "project:/home/leonardo/src/app", "tb": [], "other_servers": 1},
    {"client": "claude-desktop", "path": "/home/leonardo/.config/Claude/claude_desktop_config.json", "kind": "missing"},
    {"client": "codex", "path": "/home/leonardo/.codex/config.toml", "kind": "file", "parsed": false}
  ],
  "project": "/home/leonardo/src/app"
}
```

- `kind` is one of `file`, `missing`, `symlink`, `other` or `too_large`.
- `parses` appears only when `kind` is `file`.

Text prints the same data in three groups (CLIs, MCP configs, next steps). The next steps are commands to copy, never run:

```
next (CLI agents):  tb bus invite <channel> --role worker --name <name>
                    tb bus run --channel <channel> --as <name> --invite <code>   (write the wake file first, see docs/bus-harness.md)
next (GUI agents):  tb bus join <channel> --invite <code> --name <name> --wake notify
                    point the client at: tb mcp --bus-as <channel>/<name> --bus-root <root>   (the agent must not call tb_bus_join)
```

### What it never does

- Creates invites, joins, writes wake files or edits MCP configs.
- Follows a symlink, opens a non-regular file, reads more than 4 MiB, or prints a parse error.
- Prints another server's command, arguments, environment or URL, or the `tb` entry's `env` and `--bus-root`.
- Opens a network connection or executes any binary.
- Presents agents as trusted: the catalog is a hint for the human, and joining still requires an invite (ADR-013 §2).

## Out of B3

- Presets for `codex`, `gemini` and `opencode` in the `harness` adapter (each needs a headless/resume contract and an E2E with a fake binary, as `claude` had).
- `discover --probe` (running `--version`), the running-apps scan (`/proc/*/comm` cannot tell the desktop app from the CLI and any process can set it) and `config.toml` parsing for codex.
- Windows desktop notifications.
- A `Stop` hook for Claude Code that checks the queue before ending a turn (ADR-013 mentions it as a lightweight alternative).
- `discover` over MCP, and any automatic join.
- L1/L2 layers and autonomous L0 (still out, as in [bus-harness.md](bus-harness.md)).

## Docs to update with the implementation

- ADR-013 status line and its pt-BR mirror;
- [bus-harness.md](bus-harness.md):
  - `adapter: exec | harness` becomes `exec | harness | notify`;
  - the `notify` and `discover` items leave "Out of this cut";
  - the packet section gains `types` and `senders`;
- [bus-protocolo.md](bus-protocolo.md), "Out of B1";
- `docs/cli.md` bus section, README roadmap, CHANGELOG;
- the pt-BR mirror of each of these.

## Acceptance criteria

1. With `adapter: notify`, `backend: desktop` and a fake root-owned `notify-send` (test hook for the ownership check), three messages within `debounce` call it once. Its exact `argv` is `--app-name toolbelt --urgency normal -- <summary> <body>`, with the counts by type.
2. On macOS (or with a fake `osascript` and the platform forced in a test) the `argv` is exactly the `on run argv` form above, with the body before the summary.
3. A message whose thread, payload or `causation_id` holds `click http://x` never puts that text in the `argv`, the audit or stderr. A sender named `run-this-now` appears only quoted and cut to 32 characters, and `system:watchdog` appears as `the broker`.
4. 25 messages from 5 senders produce correct `types` counts and `and 2 others`.
5. `urgent` uses `urgent_urgency` (`critical` by default, `normal` when configured) and skips the debounce. Budget and retries behave as for `exec`.
6. The desktop child environment holds only the listed variables: a `*TOKEN*` variable, `DISPLAY` and `TB_BUS_CHANNEL` do not reach it.
7. `tb bus run` and `--check` refuse each of the following:
   - a non-empty `pass_env`, `secret_files` or `sandbox` with `notify`;
   - an `exec:` block with `notify`, and a `notify:` block with `exec`;
   - `backend: desktop` on an unsupported platform;
   - a missing default `notify-send`;
   - a `command` that is relative, not root-owned, in a writable directory, or under the repository.
   
   An empty `sandbox: {}` is accepted.
8. `wake_mismatch` prints one stderr warning and exit 0 on both `--check` and `run`.
9. Defaults with `notify`: `rewake_after` `1h` and `timeout` `10s`. An unread notification gets one reminder whose summary says `still unread`, then `wake_unread`.
10. `backend: terminal` writes exactly one line plus `\a` to the injected writer and runs without Landlock and without `--allow-unsandboxed`.
11. On a Landlock host, the desktop child reaches a fake D-Bus server on a unix socket with no `XDG_RUNTIME_DIR` grant, and cannot create a file in `XDG_RUNTIME_DIR`.
12. A GUI agent onboarded as in "Onboarding" runs `tb bus run` with `notify`. Its `tb mcp --bus-as` connection reads with `tb_bus_recv`, and `tb_bus_join` on that connection fails without replacing the token.
13. `tb bus discover` on a fake `PATH` and `HOME`:
    - lists exactly the table CLIs present, with paths, targets and presets;
    - lists a relative `PATH` entry under `skipped_path_entries`;
    - executes nothing (a fake CLI that records its runs is never run).
14. A Claude Desktop config with another server holding `env: {API_KEY: ...}` and `args` with a URL reports only `kind`, `parses`, `tb: []` and `other_servers: 1`: neither the key, the value nor the URL appears in text or JSON.
15. A `.mcp.json` that is a symlink to a secret file, a FIFO, a 5 MiB file, or invalid JSON quoting a secret yields `symlink`, `other`, `too_large` and `parses: false` respectively. `discover` does not hang, and no file content appears.
16. A `tb` entry that points at another binary reports `same_tb: false` with that path. A bare `tb` reports `same_tb: null`, and `sh -c "tb mcp"` counts as `other_servers`. Per-project entries of `~/.claude.json` are reported per project path.
17. Outside a git repository without `--cwd`, `project` is `null` and nothing fails.
18. No MCP tool exposes `discover` or the notify settings.
