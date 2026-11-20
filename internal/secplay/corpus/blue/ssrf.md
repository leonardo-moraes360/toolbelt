---
name: blue-ssrf
color: blue
area: web
class: ssrf
mirror: red-ssrf
triggers: ["ssrf detection", "metadata access", "imds", "outbound anomaly", "egress"]
summary: "Detect and harden against SSRF: metadata-IP access, unexpected egress, and user-controlled fetch URLs."
checks:
  - id: ssrf-user-controlled-fetch
    description: "Outbound request to a URL built from user input"
    pattern: '(requests\.(get|post|put)|urllib\.request\.urlopen|http\.Get|http\.NewRequest|axios\.(get|post))\(.*(\+|%s|format\(|\$\{)'
    include_globs: ["*.py", "*.go", "*.js", "*.ts"]
    severity: warning
---

# SSRF — Defensive Methodology

## Signals
Server-originated requests to 169.254.169.254 / fd00:ec2::254 (cloud metadata), loopback, or internal ranges; unusual outbound destinations from a fetch feature.

## Detection Rules
Alert on any process reaching the metadata IP, on outbound connections to RFC1918/link-local from web tiers, and on redirect chains ending at internal hosts.

## Hardening
Resolve-and-allow-list destinations, block internal/metadata ranges, disable unused URL schemes and redirect-following, and require authenticated egress proxies. The registered check flags user-built fetch URLs (heuristic; confirm by hand).

## Incident Response
Determine what internal surface was reachable, rotate any cloud credentials exposed via metadata, and add egress filtering.

## References
OWASP SSRF; CWE-918.
