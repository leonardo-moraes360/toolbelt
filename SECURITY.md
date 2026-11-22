🇺🇸 English | [🇧🇷 Português](docs/pt-BR/SECURITY.md)

# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| 1.0.x | Yes |
| < 1.0 | No |

After 1.0, the latest minor of 1.x is supported; security fixes ship in it and in no older minor. Release candidates (`-rc.N`) are not supported. See how to verify a release in [docs/release.md](docs/release.md).

## Reporting Vulnerabilities

### How to Report

Use **GitHub Private Vulnerability Reporting** to report security vulnerabilities:

1. Go to the repository **Security** tab
2. Click **Report a vulnerability**
3. Fill in the form with vulnerability details

**Do not open public issues for security vulnerabilities.**

### What to Include in the Report

- Clear description of the vulnerability
- Steps to reproduce
- Potential impact
- Affected version (when applicable)
- Possible mitigations (if known)

### Expected Response Time

| Step | Time |
|------|------|
| Receipt acknowledgment | 48 hours |
| Initial triage | 7 days |
| Fix (critical vulnerabilities) | 14 days |
| Fix (medium/low vulnerabilities) | 30 days |

These times are targets, not guarantees. Complex vulnerabilities may take longer.

## Scope

### In Scope

Vulnerabilities related to:

- **Sandbox escape**: any way to break out of sandbox isolation (Landlock for the filesystem, fail-closed, plus a seccomp network filter for the embedding child, see [ADR-004](docs/adr/004-sandbox-isolamento.md) and [ADR-018](docs/adr/018-cgo-componentes-nativos.md); bubblewrap is not implemented)
- **Guardrail bypass**: circumventing anti-cheat protections or capability limits
- **Privilege escalation**: an agent obtaining permissions beyond its role
- **Data exfiltration**: leaking secrets or protected data
- **Command injection**: executing unauthorized commands
- **Audit log corruption**: modifying or deleting audit records
- **Benchmark manipulation**: ways to falsify benchmark results
- **Denial of Service**: crashing or hanging the system

### Out of Scope

- Vulnerabilities in dependencies (report directly to maintainers)
- Incorrect user configurations
- Attacks that require physical access to the machine
- Social engineering
- Bugs with no security impact

## Disclosure Process

1. **We receive** the report via GitHub Private Vulnerability Reporting
2. **We acknowledge** receipt within 48h
3. **We investigate** and reproduce the vulnerability
4. **We develop** a fix on a private branch
5. **We coordinate** disclosure with the reporter
6. **We ship** the fix
7. **We publish** a security advisory

## Credits

Security researchers who report valid vulnerabilities will be credited in the security advisory (unless they prefer anonymity).

## Commitment

We commit to:

- Not taking legal action against researchers acting in good faith
- Working with reporters to understand and resolve issues
- Keeping reporters informed of progress
- Crediting findings when desired

## Contact

For questions about this policy (not for reporting vulnerabilities), open an issue in the repository.
