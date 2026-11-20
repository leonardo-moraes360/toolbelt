---
name: red-idor
color: red
area: web
class: idor
mirror: blue-idor
triggers: ["idor", "bola", "broken object level authorization", "object reference", "horizontal privilege"]
summary: "Surface insecure direct object references: object access decided by a client-supplied id with no ownership check."
---

# Insecure Direct Object Reference — Offensive Methodology

## Where It Hides
Endpoints that take an object id (numeric, UUID, slug) and return or mutate it based only on the id — not on who is asking. Common in `/api/<resource>/<id>` and export/report features.

## Detection
With two accounts you control, request one account's object id as the other account. Access that should be denied but succeeds is the finding. Test read, update, and delete separately; authorization often differs per verb.

## Exploitation
Demonstrate cross-account access to a single object you are authorized to use for testing. Do not enumerate or harvest other users' real data.

## Impact
Cross-tenant data read/modification, account takeover, and mass data exposure when ids are enumerable.

## Remediation
Enforce per-request ownership/authorization server-side on every object access; prefer unguessable ids as defense-in-depth, not as the control. IDOR is a logic flaw — the blue mirror covers detection via access-pattern anomalies, not a static grep.
