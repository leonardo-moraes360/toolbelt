---
name: blue-sqli
color: blue
area: web
class: sql-injection
mirror: red-sqli
triggers: ["sqli detection", "detect sql injection", "sql error", "parameterized query", "sqlmap detection"]
summary: "Detect and harden against SQL injection: spot string-built queries, enforce parameterization, and watch tool/error signals."
checks:
  - id: sqli-string-built-query
    description: "Query built by concatenation/format instead of parameters"
    pattern: '(execute|query|rawQuery|Raw|exec)\(.*(\+|%s|%v|format\()'
    include_globs: ["*.go", "*.py", "*.js", "*.ts", "*.java", "*.rb"]
    severity: warning
---

# SQL Injection — Defensive Methodology

## Signals
Database errors echoed to clients, `sqlmap`/`ghauri` user-agents, bursts of near-identical requests with incremental mutations, and `UNION SELECT`/`OR 1=1` substrings in WAF logs.

## Detection Rules
Alert on DB server processes spawning shells (critical), on `xp_cmdshell`/`LOAD_FILE`/`COPY FROM PROGRAM` in DB logs, and on WAF rule family 942xxx hits. Prioritize process-spawn > tool fingerprint > payload-in-params > error-confirmation.

## Hardening
Parameterized queries only; allow-list dynamic identifiers; least-privilege DB accounts; stop echoing DB errors. The registered check flags string-built queries for review.

## Incident Response
Identify the injectable parameter from logs, scope what the query could reach, rotate exposed data/credentials, and add a regression test plus the static check.

## References
OWASP WSTG injection; CWE-89.
