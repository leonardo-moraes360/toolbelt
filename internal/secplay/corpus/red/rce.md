---
name: red-rce
color: red
area: web
class: rce
mirror: blue-rce
triggers: ["rce", "remote code execution", "command injection", "os command", "eval"]
summary: "Surface remote code execution: user input reaching a command, shell, or code-eval sink."
---

# Remote Code Execution — Offensive Methodology

## Where It Hides
Input flowing into OS command execution (shell=true, `os.system`, `Runtime.exec`), code evaluation (`eval`, dynamic `require`/import), or unsafe template/expression engines. File upload + include chains are a frequent path.

## Detection
Identify a sink reached by user data. Confirm with a benign, observable side effect (a timing delay or a controlled callback) rather than a destructive command. Keep proofs minimal and reversible.

## Exploitation
Demonstrate execution with a harmless marker. Do not establish persistence, pivot, or run destructive commands outside an explicitly authorized scope.

## Impact
Full server compromise, lateral movement, and data theft.

## Remediation
Avoid shells; pass argument arrays; never eval user input; sandbox and least-privilege the process. Blue mirror flags shell/eval sinks statically and spawned-shell telemetry at runtime.
