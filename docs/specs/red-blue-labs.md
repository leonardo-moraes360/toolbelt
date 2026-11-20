🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/red-blue-labs.md)

# Spec: red/blue isolated labs (`tb lab`)

Author: toolbelt · Status: proposed (not implemented) · Date: 2026-11-20 · Decision: [ADR-017](../adr/017-appsec-red-blue-labs.md) · Companion: [AppSec playbooks](appsec-playbooks.md)

## Overview

`tb lab` provisions ephemeral, isolated environments to exercise a security attack and its defense against a **lab-local target** — never the host, never a third party. It lets the agent prove that a red playbook's technique works and that the paired blue playbook detects or blocks it, then throws the environment away.

Isolation is **tiered**:

- **Single step → the existing sandbox** (bubblewrap + Landlock, ADR-004): run one payload or check fail-closed, no new dependency.
- **Multi-service topology → Podman rootless** (daemonless, no root): a target node, a red node, a blue node on a private network.

`tb lab` is **optional and fail-closed**: if the required tier is unavailable, it refuses clearly rather than degrading isolation.

This spec defines the lab definition, the lifecycle, the tiered isolation contract, the safety defaults (no egress, resource caps, guaranteed teardown), the threat model, and acceptance criteria.

## Goals / Non-Goals

**Goals**
- Safe, disposable red+blue exercise environments driven from `tb`.
- Reuse the sandbox (ADR-004) for the single-step tier; add only Podman rootless for topology.
- No host exposure: network-isolated by default, resource-capped, torn down on success, failure, and crash.
- Integrate with playbooks: `tb lab run` drives a red playbook against the target and a blue playbook as the detector.

**Non-Goals**
- Not a general container orchestrator; only the minimal target/red/blue topology.
- No attacks against anything outside the lab; no default internet egress.
- No persistence of lab state beyond an explicit, scoped artifact export.
- No dependency on Docker or a root daemon.

## Threat Model

- **Hostile target and tooling (covered):** the target is deliberately vulnerable and the red tooling is offensive. Both are untrusted. They must not reach the host filesystem, the host network, or any external host. Egress is denied by default; the target/red/blue nodes see only the lab's private network.
- **Hostile lab/target images (covered):** images and lab definitions are untrusted content (ADR-008). `tb` does not run image-embedded instructions against the host; it only provisions them inside the isolation boundary. Image source is pinned and recorded.
- **Escape attempt from a node (mitigated, not eliminated):** rootless Podman + user namespaces + dropped capabilities + seccomp + read-only host mounts shrink the blast radius; a kernel-level container escape remains outside `tb`'s guarantee and is documented as residual risk.
- **Resource exhaustion (covered):** every node is CPU/RAM/PID/time-capped (cgroups v2); a lab cannot starve the host.
- **Leftover state (covered):** teardown is guaranteed on every exit path; `tb lab verify` detects orphans.
- **Secret egress (covered):** no host secret or env is injected into a lab; the execenv allowlist (ADR) does not extend into lab nodes.

## Lab Definition

Authored or scaffolded, stored with the project (not in private state):

```
labs/<name>/lab.yaml
```

```yaml
name: sqli
class: sql-injection            # ties to the playbook pair
tier: auto                      # auto | sandbox | podman
network:
  egress: deny                  # deny (default) | explicit-allow list
  internal: true                # nodes share one private net, no host route
nodes:
  target:
    image: <pinned-ref-or-build>   # vulnerable app or the user's ./app
    build: ./examples/vuln-api     # optional local build context
    expose: [8080]                 # only to the lab network
  red:
    image: <pinned-tooling-ref>
    playbook: red-sqli
  blue:
    image: <pinned-monitor-ref>
    playbook: blue-sqli
limits:
  cpus: "1.0"
  memory: "512m"
  pids: 256
  timeout: "10m"
```

`tier: auto` picks sandbox for a single-node, single-step lab and Podman for anything with a network or more than one node.

## Lifecycle (CLI)

```
tb lab new NAME [--class C] [--target PATH]   # scaffold labs/NAME/lab.yaml
tb lab up NAME [--tier sandbox|podman]        # provision; fail-closed on missing tier
tb lab status NAME                            # nodes, net, caps, health
tb lab run NAME [--red P] [--blue P]          # drive red playbook + blue detector; report
tb lab logs NAME [--node target|red|blue]
tb lab export NAME --to DIR                   # scoped artifact export (opt-in)
tb lab down NAME                              # guaranteed teardown
tb lab list
tb lab verify                                 # detect orphaned containers/nets/files
```

`tb lab run` outcome is a verdict the agent can act on: `attack=reproduced|failed`, `defense=detected|missed`, so a red/blue pair can be validated end to end (and wired into `tb stack` later).

## Tiered Isolation Contract

| Concern | sandbox tier | podman tier |
|---------|--------------|-------------|
| Engine | bubblewrap + Landlock (ADR-004) | Podman rootless, user namespaces |
| Topology | single node, no inter-node net | target/red/blue on one internal network |
| Host FS | Landlock read-only / denied | no host bind mounts except explicit, read-only |
| Network | none by default | internal-only; egress denied by default |
| Capabilities | seccomp fail-closed | drop-all + minimal add, no-new-privileges |
| Resource caps | existing cgroup limits | cgroups v2 per `limits` |
| Teardown | process exit | `podman ... --rm`, network + volume prune for the lab label |

**Fail-closed:** `tb lab up` probes the tier first. Missing Podman (or no rootless support, no cgroups v2 delegation) → refuse a podman lab with a specific remediation message; never silently fall back to weaker isolation. A sandbox-tier lab always works.

## Safety Defaults

1. **Egress deny by default.** A fresh lab reaches neither the host nor the internet. `network.egress` must list explicit allowed destinations to open anything, and that is logged.
2. **No host secrets.** Host env/secret allowlist does not propagate into lab nodes.
3. **Guaranteed teardown.** `down`, process exit, timeout, and crash all converge on full cleanup; `tb lab verify` is the audit.
4. **Pinned images.** Image refs are pinned (digest) or built from an in-repo context; recorded in the lab manifest.
5. **Authorized-use gate.** First `tb lab` per repo prints the one-time notice (recorded in `.toolbelt`), shared with `tb red`.

## Acceptance Criteria

- A1 — `tb lab up` on a topology lab with no Podman refuses with a remediation message and provisions nothing (fail-closed); a sandbox-tier lab still works.
- A2 — A fresh lab has no host route and no internet egress; an egress attempt from any node fails — enforced by test.
- A3 — `tb lab down`, a timeout, and a simulated crash each leave zero containers, networks, volumes, or files for the lab label; `tb lab verify` reports clean.
- A4 — `tb lab run sqli` reports `attack=reproduced` and `defense=detected` for the seed SQLi lab against its vulnerable target.
- A5 — No host secret or host env reaches a lab node — enforced by test.
- A6 — Every node is capped per `limits` (cgroups v2); exceeding a cap kills the node, not the host.
- A7 — Podman is invoked rootless and daemonless; no `docker` group or root daemon required.
- A8 — EN + `docs/pt-BR/` mirror present (`TestDocsTranslationMirrorComplete`).

## Open Questions

1. **Podman detection & delegation.** How to probe rootless Podman + cgroups v2 delegation portably (Linux distros vary; macOS needs the Podman machine VM)? macOS likely sandbox-tier only in Phase 1.
2. **Seed target images.** Ship a tiny purpose-built vulnerable app per class (ours, minimal), or let the user point at their app? Probably both: a built-in minimal target for demos, `--target ./app` for real use.
3. **`tb lab run` verdict wiring.** Define the attack/defense verdict contract and whether it becomes a `tb stack` op (like egm did).
4. **Image provenance.** Build in-repo from Containerfiles (fully owned, slow) vs pin upstream digests (faster, external trust)? Leaning in-repo for the target, pinned for base layers.
5. **Rootless networking.** `pasta`/`slirp4netns` vs internal-only netavark networks for the deny-egress default; pick the simplest that guarantees no host route.
