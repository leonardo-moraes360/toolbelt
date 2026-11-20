---
name: red-xss
color: red
area: web
class: xss
mirror: blue-xss
triggers: ["xss", "cross-site scripting", "dom xss", "stored xss", "reflected xss", "innerhtml"]
summary: "Surface cross-site scripting: untrusted data reaching an HTML/JS sink, reflected, stored, or DOM-based."
---

# Cross-Site Scripting — Offensive Methodology

## Where It Hides
Untrusted input echoed into HTML, attributes, JS contexts, or URLs without context-correct encoding: server templates with autoescaping off, `innerHTML`/`dangerouslySetInnerHTML`, `document.write`, and framework escape hatches.

## Detection
Trace each input to its output context (HTML body, attribute, script, URL). Use a benign, unique marker to see whether it lands unencoded and whether the surrounding context lets it break out. Distinguish reflected, stored, and DOM sinks by where the data is rendered.

## Exploitation
Demonstrate script execution with a harmless proof (e.g., a benign callback or a DOM change), not a weaponized payload. For DOM XSS, follow the source-to-sink dataflow in client JS.

## Impact
Session/token theft, action-on-behalf, credential capture via injected UI, and pivoting to CSRF-protected actions.

## Remediation
Context-aware output encoding, a strict Content-Security-Policy, framework auto-escaping left on, and avoiding raw-HTML sinks. The blue mirror flags `innerHTML`/`dangerouslySetInnerHTML` sinks.
