🇺🇸 English | [🇧🇷 Português](../pt-BR/pesquisa/mcp-sinais-m8ven.md)

# tb's MCP server against M8ven's signals

> **Review date**: 2026-11-18
> **Reference**: [m8ven/is-it-legit-mcp](https://github.com/m8ven/is-it-legit-mcp) at commit `61dd093`, [M8ven Verified](https://m8ven.ai/verified/how-we-verify) and the [public report](https://m8ven.ai/mcp/is-it-legit-mcp) on their own server
> **Target**: `tb mcp` ([internal/mcp/server.go](../../internal/mcp/server.go) and [internal/registry/](../../internal/registry/))

## Why M8ven is a useful reference

M8ven grades MCP servers from A to F. Its public methodology does not list the criteria or the grading formula, but its report and its own repository show what the grade rests on:

| Signal | What M8ven looks at |
|---|---|
| **Code Verified** | The source does what the tools declare. In practice, the MCP annotations match the handler's real effect. |
| **Sandbox Verified** | The server ran in an isolated environment and its behavior was observed. |
| **Live Monitored** | Re-verified on every push, through a GitHub App. |
| Exfiltration and obfuscation | The server sends no credentials out, reads no sensitive files and has no obfuscated code. |
| Dependencies | Known vulnerabilities in production dependencies. Their own server is downgraded for 5 high-severity dependencies. |
| Open source | A license and a README are present. |
| Publisher | Proof of control over the repository or host. Their repository ships `.well-known/m8ven-publisher.txt`. |
| Maintenance | Issues closed, releases and contributor retention over the last 90 days. |

The most useful pattern in the repository is the README: every tool carries the four annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`), each **justified by what the handler actually does**. Instead of "it's a lookup", the justification says, for example, that `check_brand` upserts a cache with `onConflict` and is therefore destructive. That rigor is what the Code Verified mark measures.

## tb's current M8ven report

[tb's listing](https://m8ven.ai/mcp/leonardo-moraes360-toolbelt-fa03ue), checked on 2026-11-18:

- **Grade C, status Limited view.** The code was graded at commit `531c9bc` (Nov 15), but the tool surface was not read. Static analysis for Go is only partially covered.
- **Passes:** no credential exfiltration, sensitive file access or obfuscation; open source with a license and a README.
- **Missing:** the listing is unclaimed (no Verified Publisher), Live Monitored is not connected, and there is no Sandbox Verified mark.
- The full audit trail (each check, deductions, advisories) is shown only to verified publishers.

The exfiltration "pass" comes from partial static analysis. Finding 2 below still holds: the vector is the model asking for the secret at runtime, not code that sends the secret out. Claiming the listing and connecting the GitHub App removes the Limited view status and opens the full audit trail.

## Findings

### High

1. **Fixed: no tool declared annotations.** `registry.Tool` only has `Name`, `Description` and `InputSchema` ([registry.go:9](../../internal/registry/registry.go:9)), and `tools/list` emits no `annotations` ([server.go:142](../../internal/mcp/server.go:142)). Without the hints, the client treats `tb_ticket_list` and `tb_run` the same way: it cannot auto-approve lookups or flag the dangerous tools. This is exactly the Code Verified signal.
   *Fix:* add an `Annotations` field to `Tool`, emit it in `tools/list`, and fill it in from the table below. A test should check that every registered tool declares all four hints.
   *Status:* fixed. [annotations.go](../../internal/registry/annotations.go) holds the table, `Register` fills it in, `tools/list` emits all four hints explicitly, and `TestAllToolsAnnotated` fails on any tool without an entry.

2. **Fixed: secrets were opted in by the model, not the human.** `tb_run`, `tb_verify_run` and `tb_stack` accept `pass_env` as a call argument ([builtin.go:60](../../internal/registry/builtin.go:60) and [parity.go:185](../../internal/registry/parity.go:185)). The env allowlist is good, but any prompt-injected agent can ask for `pass_env:["GITHUB_TOKEN"]` and print the value, because `tb_run` accepts any `sh -c`. By the lethal trifecta (private data + untrusted content + external output), this is M8ven's "credential exfiltration" finding.
   *Fix:* accept `pass_env` only from configuration (`.toolbelt` or a `tb mcp` flag) and not from the tool argument, or require the key to be already allowed in configuration.
   *Status:* fixed. Over MCP, [passenv.go](../../internal/registry/passenv.go) refuses any `pass_env` key that is not already in `env.pass` (an `isError` result naming the key), in `tb_run`, `tb_verify_run` and `tb_stack`. The CLI `--pass-env` flag is unchanged, since a human types it. Covered by `TestToolPassEnvRefusedWithoutConfigOptIn`. Residual risk, now closed by finding 8: an agent could write `env.pass` into the repository's `.toolbelt/config.yaml` and then ask again. `pass_env` now only trusts `env.pass` from the global or `-c` config.

### Medium

3. **Fixed: graph queries wrote.** `tb_graph_search`, `callers`, `deps`, `impact`, `explain` and `path` call `openGraphStoreFresh` ([graph.go:121](../../internal/registry/graph.go:121)), which rebuilds the index when it is stale and `graph.index_on_change` is on (the default) ([fingerprint.go:100](../../internal/graph/fingerprint.go:100)). The names suggest reads, but the call can write to disk. M8ven treats the same case (a cache upsert into `brand_signals`) as `readOnlyHint:false`.
   *Fix:* annotate `readOnlyHint:false, destructiveHint:false, idempotentHint:true`, or pass `indexOnChange=false` on the MCP path and return `stale:true`.
   *Status:* fixed with the second option. Over MCP, the queries open the index with `openGraphStoreReadOnly` ([graph.go](../../internal/registry/graph.go)), which only checks the fingerprint. A stale index returns `stale: true` and a `hint` to call `tb_graph_index`, which is the only graph tool that writes. The queries are now annotated `readOnlyHint:true`. The CLI keeps honoring `graph.index_on_change`. Covered by `TestGraphQueriesNeverWriteIndex` and `TestGraphQueryWithoutIndexCreatesNothing`. Trade-off: after an edit, an agent that ignores `stale` gets results from the old index until it calls `tb_graph_index`.

4. **Fixed: the protocol version was frozen at `2024-11-05`** ([server.go:16](../../internal/mcp/server.go:16)), with no negotiation. Tool annotations only entered the spec in `2025-03-26`, and [cli.md](../cli.md) and the sources already claim alignment with the stateless `2026-07-28` spec. A strict client may ignore `annotations` in a session that announces itself as 2024-11-05.
   *Fix:* read `params.protocolVersion` from `initialize` and apply the spec's rule: if the version is supported, answer with the same one; otherwise answer with the latest supported version.
   *Status:* fixed in [server.go](../../internal/mcp/server.go). The server supports `2025-11-25` (the latest it answers with), `2025-06-18` and `2024-11-05`. A missing or malformed version gets the latest. Covered by `TestInitializeNegotiatesProtocolVersion`. Two revisions stay out on purpose:
   - `2025-03-26` requires JSON-RPC batching, which the server does not implement. Such a client is offered `2025-11-25`.
   - `2026-07-28` replaces `initialize` with `server/discover` and per-request `_meta`, and requires `resultType`, `ttlMs` and `cacheScope` on results. That is a different lifecycle, still pending in [arquitetura.md](../arquitetura.md).

5. **Fixed: `tools/list` order was not deterministic.** `Registry.List` iterates over a map ([registry.go:61](../../internal/registry/registry.go:61)). The 2026-07-28 spec asks for cacheable lists in deterministic order, and an unstable order also breaks the client's prompt cache.
   *Fix:* `sort.Slice` by name.
   *Status:* fixed in `Registry.List`, covered by `TestListSortedByName` and `TestToolsListEmitsAnnotationsInOrder`.

### Low

6. **Publisher.** tb is already listed, but the listing is unclaimed and there is no `.well-known/m8ven-publisher.txt`. Claiming it gives the Verified Publisher mark and the full audit trail.
7. **Supply chain.** `govulncheck` (CI, [ci.yml:61](../../.github/workflows/ci.yml:61)) and `gosec` (golangci) already cover the dependency signal. This review did not run `govulncheck` locally because the tool is not installed.
8. **Fixed: Sandbox Verified.** `tb mcp` ran outside tb's own sandbox: `tb_run` executed with no Landlock or bwrap. M8ven's sandbox signal would observe writes outside the repository.
   *Status:* fixed in [runsandbox.go](../../internal/registry/runsandbox.go). `tb_run` runs under Landlock: it writes only to the repository root, its own `TMPDIR` and the toolchain caches; it reads the system dirs, `PATH`, `GOROOT` and the `sandbox.read`/`write` paths from the global config. `$HOME`, `/tmp` and other repositories stay closed, and a `cwd` outside the repository is refused. Without Landlock it fails closed unless `sandbox.allow_unsandboxed`. Because the repository is writable, the security knobs (`env.pass`, `sandbox.*`) now come only from the global or `-c` config (`config.LoadTrusted`), which closes the residual risk of finding 2. Covered by the `TestTBRun*` tests in [runsandbox_test.go](../../internal/registry/runsandbox_test.go). Still open: the network is not restricted, and `tb_verify_run`/`tb_stack` still run unsandboxed and still read `env.pass` from the repository config.

### Already met

- License, bilingual README, SECURITY and releases (the open-source and maintenance signals).
- A tool failure comes back as `isError:true` in the result, not as a JSON-RPC error ([registry.go:81](../../internal/registry/registry.go:81)).
- `tb_bus_recv` warns that content from other agents is untrusted data.
- `tb_merge` dry-runs by default and requires `apply:true` and `allow_sensitive` for sensitive paths.
- No networking in the server: the only ways out are child processes (`tb_run`, verify, review).

## Annotations (implemented)

The justifications follow M8ven's format: the handler's real effect, not the tool's name.

| Tool | readOnly | destructive | idempotent | openWorld | Justification |
|---|---|---|---|---|---|
| `tb_version`, `tb_echo`, `tb_env` | true | false | true | false | They only read the process. |
| `tb_ticket_list`, `tb_ticket_show`, `tb_mem_list`, `tb_mem_show`, `tb_mem_search`, `tb_mem_check`, `tb_router_status`, `tb_watchdog_alerts` | true | false | true | false | They read `.toolbelt/` without writing. |
| `tb_graph_search`, `callers`, `deps`, `impact`, `explain`, `path` | true | false | true | false | They read the index and never rebuild it over MCP. A stale index returns `stale: true` (finding 3). |
| `tb_graph_index` | false | true | true | false | Overwrites the index. |
| `tb_ticket_create`, `tb_mem_add` | false | false | false | false | Insert. Each call creates a new item. |
| `tb_mem_edit`, `tb_mem_retire` | false | true | true | false | They overwrite an invariant's fields (retire is soft, but it changes state). |
| `tb_mem_init` | false | false | true | false | Creates the defaults only if missing (`mem.EnsureDefaultAntiCheat`). |
| `tb_board` | false | true | true | false | `pause`/`resume`/`drain` change router state. `list` only reads, but the annotation covers the whole tool. |
| `tb_bus_join`, `tb_bus_send`, `tb_bus_claim`, `tb_bus_complete` | false | false | false | true | They write to the bus that other agents read (open world across agents). |
| `tb_bus_recv` | false | false | false | true | Advances the cursor. Repeating the call returns a different batch. |
| `tb_merge` | false | true | false | false | With `apply:true`, it changes refs. Dry-run is the default, but the annotation covers the worst case. |
| `tb_review`, `tb_drift`, `tb_guard_check` | true | false | true | false | They analyze the diff (`review` only runs `git patch-id`; no writes). |
| `tb_verify_run`, `tb_stack` | false | true | false | true | They run user commands with a filtered env. |
| `tb_run` | false | true | false | true | `sh -c` under Landlock: it can delete or overwrite anything in the repository, and the network is open. |

The handlers for `tb_mem_init`, `tb_review` and `tb_stack` were read to confirm these values. `TestAllToolsAnnotated` fails if a new tool is registered without an entry.
