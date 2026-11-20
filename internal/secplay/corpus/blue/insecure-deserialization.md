---
name: blue-insecure-deserialization
color: blue
area: web
class: insecure-deserialization
mirror: red-insecure-deserialization
triggers: ["deserialization detection", "pickle detection", "gadget chain", "objectinputstream detection"]
summary: "Detect and harden against insecure deserialization: native deserializer sinks and gadget indicators."
checks:
  - id: deser-native-sink
    description: "Native deserializer applied to possibly-untrusted input"
    pattern: '(pickle\.loads?\(|yaml\.load\(|ObjectInputStream|unserialize\(|Marshal\.load|BinaryFormatter)'
    include_globs: ["*.py", "*.java", "*.php", "*.rb", "*.cs"]
    severity: warning
---

# Insecure Deserialization — Defensive Methodology

## Signals
Deserialization stack traces, known gadget class names in payloads, and serialized-object magic bytes in request bodies, cookies, or queue messages.

## Detection Rules
Alert on gadget-indicative class names, on `pickle`/`yaml.load` over request data, and on deserialization errors spiking.

## Hardening
Do not deserialize untrusted data with native deserializers; use JSON with schema validation; allow-list types if unavoidable. The registered check flags native deserializer sinks.

## Incident Response
Determine whether a gadget chain was reachable, treat as potential RCE, rotate secrets, and replace native deserialization.

## References
OWASP deserialization; CWE-502.
