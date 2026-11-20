---
name: blue-xss
color: blue
area: web
class: xss
mirror: red-xss
triggers: ["xss detection", "detect cross-site scripting", "csp", "dom sink", "innerhtml"]
summary: "Detect and harden against XSS: raw-HTML sinks, CSP monitoring, and DOM source-to-sink review."
checks:
  - id: xss-raw-html-sink
    description: "Assignment to a raw-HTML sink that bypasses escaping"
    pattern: '(dangerouslySetInnerHTML|\.innerHTML\s*=|\.outerHTML\s*=|document\.write\()'
    include_globs: ["*.js", "*.ts", "*.jsx", "*.tsx", "*.vue"]
    severity: warning
---

# Cross-Site Scripting — Defensive Methodology

## Signals
CSP `report-uri` violations, blind-XSS callbacks from internal tools, and marker strings reflected unencoded in responses.

## Detection Rules
Monitor CSP reports for inline-script blocks, alert on known XSS payload shapes in request params, and review new uses of raw-HTML sinks in code review.

## Hardening
Context-aware output encoding, a strict CSP (no unsafe-inline), framework auto-escaping on, and no raw-HTML sinks. The registered check flags `innerHTML`/`dangerouslySetInnerHTML`/`document.write`.

## Incident Response
Find the injecting input and its render context, invalidate affected sessions if token theft is plausible, and add encoding plus a regression test.

## References
OWASP WSTG client-side; CWE-79.
