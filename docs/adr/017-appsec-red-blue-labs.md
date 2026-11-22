🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/017-appsec-red-blue-labs.md)

# ADR-017: Native AppSec red/blue capabilities and isolated labs (`tb red` / `tb blue` / `tb lab`)

## Status

proposed; experimental in 1.0.

## Context

`tb` is a toolbelt for coding agents, aimed at keeping software correct and cheap to work on. A natural extension is keeping that software **secure**: giving the agent offensive know-how (find the vulnerability classes that creep into code) and defensive know-how (detect, harden, respond), plus a safe place to exercise both.

Two curated Claude Skill libraries exist as inspiration:

- **[SnailSploit/Claude-Red](https://github.com/SnailSploit/claude-red)** — offensive security skills (`SKILL.md` prompt primers).
- **[riparino/Claude-Cyber](https://github.com/riparino/Claude-Cyber)** — a defensive `Claude-Blue` library (detection in Sigma/KQL/YARA, MITRE mapping) plus a smaller red library.

They are broad (wireless, Active Directory, C2, social engineering) and are *prompt material, not code*. Pulling them in wholesale has three problems: it is an external content dependency that drifts and must be licensed and trusted; most of the breadth is irrelevant to **software** security; and it adds nothing `tb` can verify or run — it is inert text.

This project already set the precedent for exactly this situation:

- **ADR-014** — *Graphify: reimplement, don't integrate.*
- **ADR-015** — *native reimplementation of the Elephant-Goldfish Model.*

We apply the same rule here. We also want the agent to be able to **practice and validate** security work without endangering the host — which `tb` is uniquely positioned for, since it already owns a fail-closed sandbox (ADR-004).

Two scoping decisions were taken when this ADR was written:

1. **Scope = AppSec / secure-SDLC.** In scope: code vulnerability classes (web, API, auth, crypto, injection, deserialization), dependencies/supply-chain, secrets, and container/cloud hardening — with the paired blue detection/hardening/incident-response. Out of scope: broad pentest breadth (wireless, AD, C2, physical/social engineering). Recorded as a future extension, not built now.
2. **Lab isolation = tiered.** Reuse the existing sandbox for single-step work; use **Podman rootless** (daemonless, no root) for multi-service lab topology. Optional and fail-closed.

## Decision

Build three native capabilities. Nothing in them depends on an external security corpus at build or run time; the source libraries are credited as inspiration only.

### 1. Native AppSec playbooks (reimplemented, not vendored)

Author our own curated corpus in `tb`'s voice, scoped to software security: a **red** set (offensive methodology to surface a vulnerability class in our own or a target codebase/service) and a **blue** set (detection signatures, hardening steps, incident response). Each red playbook is paired to its blue counterpart by vulnerability class (mirror). Content is ours — written, reviewed, and owned in-repo — so it can be kept current, verified, and licensed under the repo's Apache-2.0.

### 2. Surfaced as `tb red` / `tb blue`, indexed and budgeted

Both commands are thin fronts over one `internal/secplay` engine bound to a color. Playbooks are indexed into the existing SQLite + FTS5 graph (ADR-003/011/014); `search` returns compact lines, `load` returns the body **under a token budget** with section slicing, served over the MCP server (ADR-002) read-only. This keeps the ADR-011 token discipline: headers on search, detail on demand.

```
tb red  search "deserialization"
tb red  load red-insecure-deserialization [--section "Detection" | --budget 1500]
tb blue load blue-insecure-deserialization
tb red  pair red-sqli            # jump to the blue mirror, and vice-versa
```

### 3. Playbooks plug into `tb`'s verification, they are not inert

Unlike the inspiration (pure prompt text), a blue playbook may carry **runnable checks** expressed in `tb`'s existing mechanisms: a hardening step emits a `tb verify`/`tb guard` check; a secure-SDLC step hooks the `tb stack` / review flow (ADR-008). So "harden against X" becomes a check the project can keep enforcing, not a paragraph the agent reads once.

### 4. `tb lab` — isolated red+blue labs

A new `internal/lab` engine provisions ephemeral, disposable environments to exercise an attack and its defense against a **lab-local target**, never the host or a third party. Isolation is tiered:

- **Single step → existing sandbox** (bubblewrap + Landlock, ADR-004): run one payload/check fail-closed, no new dependency.
- **Multi-service topology → Podman rootless**: a target node (deliberately vulnerable or the user's app), a red node (tooling/agent), a blue node (monitoring/detection), on a private network. Daemonless, no root, no `docker` group.

Labs are **network-isolated from the host by default** (no egress), resource-capped, and **torn down guaranteed**. `tb lab` is **optional and fail-closed**: without Podman, topology labs refuse with a clear message and single-step sandbox labs still work.

```
tb lab new sqli --target ./examples/vuln-api   # scaffold a lab definition
tb lab up sqli                                  # provision (sandbox or podman)
tb lab run sqli --red red-sqli --blue blue-sqli # exercise attack + detection
tb lab down sqli                                # guaranteed teardown
```

### 5. Trust and dual-use

Offensive capability is dual-use. `tb red` and `tb lab` gate on a one-time authorized-use acknowledgement per repo. Playbook bodies and lab/target images are **untrusted content** under the egm model (ADR-008): nothing from them executes outside the sandbox/lab boundary, and no user secret leaves the host. Labs default to no network egress; reaching anything outside the lab is an explicit, logged opt-in.

The full surfaces live in [spec: AppSec playbooks](../specs/appsec-playbooks.md) and [spec: red/blue labs](../specs/red-blue-labs.md).

## Consequences

### Positive

- **No external content dependency**: capability is native, owned, Apache-2.0, current — consistent with ADR-014/015.
- **Scoped to the mission**: AppSec/secure-SDLC serves `tb`'s coding-agent purpose; no irrelevant pentest breadth.
- **Verifiable, not inert**: blue playbooks emit real `tb verify`/`guard`/`stack` checks.
- **Safe practice**: labs let the agent prove an attack and its detection without touching the host, reusing the sandbox and adding only an optional rootless runtime.
- **Token-honest**: search headers, budgeted load, reusing the graph.

### Negative

- **Authoring cost**: we write and maintain the corpus instead of cloning one — the deliberate trade of ADR-014/015.
- **Optional runtime**: topology labs need Podman; absence degrades the feature (fail-closed) and adds a capability-detection path.
- **Lab safety surface**: provisioning vulnerable targets and offensive tooling demands strict isolation, no-egress defaults, and guaranteed teardown — a real security burden on `tb` itself.
- **Scope pressure**: users will ask for the out-of-scope breadth; we must hold the line or stage it.

### Success Metrics

| Metric | Target | Justification |
|--------|--------|---------------|
| playbook_pairs_mirrored_pct | 100% | Every in-scope red playbook has a blue mirror and vice-versa |
| blue_checks_runnable_pct | ≥ 80% | Most blue hardening steps emit a real `tb verify`/`guard` check |
| lab_teardown_guaranteed | enforced by test | No lab leaves containers, networks, or files after `down`/failure |
| lab_default_no_egress | enforced by test | A fresh lab cannot reach the host or the internet without explicit opt-in |
| skill_load_within_budget_pct | 100% | No `load` exceeds the configured token cap |

## Alternatives Considered

### Vendor the red/blue repos as packs (previous draft of this ADR)

- **Description**: copy the `SKILL.md` libraries in, pin a commit, serve them.
- **Why not**: external content dependency, drift, licensing/trust burden, ~96k lines of mostly out-of-scope breadth, and inert text `tb` cannot verify. Rejected in favor of reimplementation (ADR-014 precedent).

### Docs-only, no labs

- **Description**: ship playbooks, no execution environment.
- **Why not**: loses the validate-safely value; the agent cannot prove an attack or its detection, which is the point of pairing red and blue.

### Docker for labs

- **Description**: use Docker instead of Podman rootless.
- **Why not**: needs a daemon and typically the `docker` group (~root) — a larger trust surface that contradicts `tb`'s fail-closed, least-privilege posture. Podman rootless gives the same topology daemonless and without root.

### Sandbox-only labs (no container runtime)

- **Description**: build labs solely on bubblewrap+Landlock.
- **Why not**: fine for single steps, but multi-node network topology (target/red/blue) is impractical under bubblewrap alone. Kept as the single-step tier, not the whole story.

## References

- [ADR-002](002-mcp-a2a-protocolos.md) — MCP server surface
- [ADR-003](003-sqlite-fts5-grafo.md) — SQLite + FTS5 graph/index
- [ADR-004](004-sandbox-isolamento.md) — sandbox (bubblewrap + Landlock), fail-closed
- [ADR-008](008-verificacao-adversarial.md) — adversarial / untrusted-content trust model
- [ADR-011](011-grafo-sob-demanda.md) — on-demand graph; human skills cut up to 41.73% of cost
- [ADR-014](014-graphify-reimplementacao.md) — reimplement, don't integrate
- [ADR-015](015-egm-nativo.md) — native reimplementation precedent
- Inspiration: [SnailSploit/Claude-Red](https://github.com/SnailSploit/claude-red) · [riparino/Claude-Cyber](https://github.com/riparino/Claude-Cyber)
