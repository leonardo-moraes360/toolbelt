---
name: blue-jwt
color: blue
area: auth
class: jwt
mirror: red-jwt
triggers: ["jwt detection", "alg none detection", "token verification", "jwt hardening"]
summary: "Detect and harden against JWT abuse: unverified/weak verification and algorithm confusion."
checks:
  - id: jwt-weak-verification
    description: "JWT decoded without verification or with an empty/none algorithm"
    pattern: '(verify\s*=\s*False|verify_signature\s*=\s*False|algorithms\s*=\s*\[\s*\]|alg.{0,8}none)'
    include_globs: ["*.py", "*.js", "*.ts", "*.go", "*.java"]
    severity: warning
---

# JWT — Defensive Methodology

## Signals
Tokens presented with `alg: none` or an unexpected algorithm, repeated signature-verification failures, and tokens with tampered privilege claims.

## Detection Rules
Alert on accepted `none`/wrong-alg tokens, on verification-failure spikes, and on privilege-claim changes without a matching issuance event.

## Hardening
Pin the expected algorithm; verify signature, expiry, audience, issuer; strong keys; never trust the token's own `alg`. The registered check flags disabled verification and empty/`none` algorithms.

## Incident Response
Revoke/rotate signing keys, invalidate affected sessions, and add algorithm pinning plus a test.

## References
RFC 8725 (JWT BCP); CWE-347.
