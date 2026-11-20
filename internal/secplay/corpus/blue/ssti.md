---
name: blue-ssti
color: blue
area: web
class: ssti
mirror: red-ssti
triggers: ["ssti detection", "template injection detection", "render_template_string", "sandbox template"]
summary: "Detect and harden against SSTI: dynamic template construction and engine sandboxing."
checks:
  - id: ssti-dynamic-template
    description: "Template built from dynamic input instead of static templates with context vars"
    pattern: '(render_template_string|new Function\(|Template\(.*(\+|%s|format\(|\$\{))'
    include_globs: ["*.py", "*.js", "*.ts"]
    severity: warning
---

# SSTI — Defensive Methodology

## Signals
Template-engine stack traces in logs, arithmetic/expression markers reflected as evaluated results, and user input reaching template compilation.

## Detection Rules
Alert on engine error signatures and on requests whose params resemble template syntax reaching render endpoints.

## Hardening
Never build templates from user input; pass data as context variables; use a sandboxed engine with autoescaping. The registered check flags dynamic-template construction.

## Incident Response
Identify the dynamic template, assess object/attribute exposure and RCE reachability, and refactor to static templates.

## References
PortSwigger SSTI; CWE-1336.
