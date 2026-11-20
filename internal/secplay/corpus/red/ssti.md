---
name: red-ssti
color: red
area: web
class: ssti
mirror: blue-ssti
triggers: ["ssti", "template injection", "jinja", "twig", "freemarker", "render_template_string"]
summary: "Surface server-side template injection: user input evaluated by a template engine."
---

# Server-Side Template Injection — Offensive Methodology

## Where It Hides
User input concatenated into a template that the server then renders: email/notification templates, customizable themes, and `render_template_string`-style calls.

## Detection
Submit a small arithmetic expression in the engine's syntax and look for evaluation in the output. A rendered result (not the literal text) fingerprints the engine.

## Exploitation
Use engine-appropriate, non-destructive expressions to confirm object/attribute access. Stop at proof of evaluation; do not chain to command execution on shared systems without explicit scope.

## Impact
Information disclosure, and in many engines a path to remote code execution via exposed objects.

## Remediation
Never build templates from user input; render data as context variables with a sandboxed engine and autoescaping. Blue mirror flags dynamic-template construction.
