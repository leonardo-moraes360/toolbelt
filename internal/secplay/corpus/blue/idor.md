---
name: blue-idor
color: blue
area: web
class: idor
mirror: red-idor
triggers: ["idor detection", "bola detection", "access pattern anomaly", "authorization audit"]
summary: "Detect and harden against IDOR: enforce per-request ownership and watch cross-account access patterns."
---

# IDOR / BOLA — Defensive Methodology

## Signals
One principal accessing many distinct object ids in a short window, access to ids outside a user's known set, and sequential-id enumeration in access logs.

## Detection Rules
Alert on horizontal access anomalies (same user touching unrelated tenants' objects), on 403-to-200 transitions after id edits, and on enumeration bursts. This is behavioral, not a static grep — no registered check ships for IDOR.

## Hardening
Enforce ownership/authorization server-side on every object access, for every verb; make authorization a mandatory middleware, not per-handler; use unguessable ids as defense-in-depth only.

## Incident Response
Scope which objects were accessed cross-account, notify per policy, and add centralized authorization plus tests.

## References
OWASP API Security BOLA; CWE-639.
