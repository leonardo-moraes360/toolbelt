---
name: blue-rce
color: blue
area: web
class: rce
mirror: red-rce
triggers: ["rce detection", "command injection detection", "shell spawn", "eval detection", "webshell"]
summary: "Detect and harden against RCE: dangerous command/eval sinks and web-process shell spawns."
checks:
  - id: rce-shell-or-eval-sink
    description: "Command/eval sink that can execute attacker-influenced input"
    pattern: '(os\.system\(|subprocess\.[A-Za-z_]+\(.*shell\s*=\s*True|Runtime\.getRuntime\(\)\.exec|child_process\.exec\(|\beval\()'
    include_globs: ["*.py", "*.js", "*.ts", "*.java"]
    severity: warning
---

# Remote Code Execution — Defensive Methodology

## Signals
Web/app processes spawning `sh`/`bash`/`cmd`/`powershell`, unexpected child processes under the web user, and new executable files in web-writable paths (webshells).

## Detection Rules
Alert (critical) on web-tier processes spawning shells, on interpreter processes with network sockets, and on writes of script extensions to upload dirs.

## Hardening
Avoid shells; pass argument arrays; never eval untrusted input; run least-privilege with a read-only app dir. The registered check flags shell/eval sinks statically.

## Incident Response
Contain the host, collect the spawned-process tree, rotate credentials reachable from it, and rebuild from a known-good image.

## References
OWASP command injection; CWE-78, CWE-94.
