---
name: red-sqli
color: red
area: web
class: sql-injection
mirror: blue-sqli
triggers: ["sql injection", "sqli", "union select", "blind sqli", "time-based", "orm injection"]
summary: "Surface SQL injection: reach-the-database inputs, confirmation, enumeration, and the remediation that the blue mirror enforces."
---

# SQL Injection — Offensive Methodology

## Where It Hides
Any input that reaches a SQL statement: URL and body params, headers, cookies, API filters, sort/order fields, and JSON keys used to build dynamic queries. ORMs are not immune — raw fragments, `.raw()`, string-built `WHERE`, and dynamic column/table names are the usual sinks.

## Detection
Probe each vector for a difference between a syntactically valid and invalid input. Prefer inferential signals (response size, status, timing) over visible errors. Confirm with a boolean pair that should differ and a safe, in-scope timing probe. Do not dump data beyond what proves the finding.

## Exploitation
Once confirmed, map the DBMS and schema through the same channel used to detect. Scope the impact to the minimum needed to demonstrate risk (one row, one table name, the current user/privilege). Avoid destructive statements and mass extraction on engagements.

## Impact
Read or modify data across tenants, authentication bypass, privilege escalation, and — where stacked queries or file primitives exist — command execution on the database host.

## Remediation
Parameterized queries / prepared statements everywhere; allow-list dynamic identifiers; least-privilege DB accounts; reject on type. See the blue mirror for the detections and the static check that flag string-built SQL.
