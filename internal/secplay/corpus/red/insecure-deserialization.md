---
name: red-insecure-deserialization
color: red
area: web
class: insecure-deserialization
mirror: blue-insecure-deserialization
triggers: ["deserialization", "insecure deserialization", "pickle", "gadget chain", "objectinputstream", "unserialize"]
summary: "Surface insecure deserialization: untrusted bytes fed to a native deserializer, and gadget-chain impact."
---

# Insecure Deserialization — Offensive Methodology

## Where It Hides
Endpoints that deserialize untrusted input with a language-native deserializer: Java `ObjectInputStream`, Python `pickle`/`yaml.load`, PHP `unserialize`, Ruby `Marshal.load`, .NET `BinaryFormatter`. Often hidden in cookies, caches, and message queues.

## Detection
Identify where serialized blobs enter. Fingerprint the format by magic bytes/headers. Confirm that the server reconstructs objects from your input rather than validating a schema.

## Exploitation
Demonstrate controlled object instantiation with a benign proof (e.g., a detectable, non-destructive side effect). Gadget chains can reach RCE; keep the proof minimal and in scope.

## Impact
Remote code execution, authentication bypass, and denial of service.

## Remediation
Do not deserialize untrusted data with native deserializers; use data-only formats (JSON) with schema validation; if unavoidable, allow-list types. Blue mirror flags native deserializer sinks.
