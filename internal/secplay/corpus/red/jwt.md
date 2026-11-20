---
name: red-jwt
color: red
area: auth
class: jwt
mirror: blue-jwt
triggers: ["jwt", "json web token", "alg none", "key confusion", "token tampering", "hs256 rs256"]
summary: "Surface JWT weaknesses: algorithm confusion, unverified signatures, weak secrets, and claim tampering."
---

# JWT Attacks — Offensive Methodology

## Where It Hides
Anywhere a JWT is accepted for authentication/authorization: `Authorization: Bearer`, cookies, and query params. Risk concentrates in how the server verifies — algorithm handling, key selection, and claim checks.

## Detection
Inspect the header/claims (they are not secret). Probe whether the server rejects `alg: none`, whether it enforces an expected algorithm (HS vs RS confusion), whether expiry/audience/issuer are checked, and whether the signing secret is weak enough to crack offline.

## Exploitation
Demonstrate one accepted forged token (e.g., a server that honors an unsigned or wrong-algorithm token) using a test account. Do not forge tokens for real users' identities beyond proving the flaw.

## Impact
Authentication bypass, privilege escalation via claim tampering, and impersonation.

## Remediation
Pin the expected algorithm; verify signature, expiry, audience, issuer; use strong keys; never trust the token header's `alg`. Blue mirror flags `verify=False` and empty/`none` algorithm lists.
