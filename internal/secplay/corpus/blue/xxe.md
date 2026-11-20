---
name: blue-xxe
color: blue
area: web
class: xxe
mirror: red-xxe
triggers: ["xxe detection", "external entity", "doctype detection", "xml parser hardening"]
summary: "Detect and harden against XXE: XML parsers left at entity-resolving defaults."
checks:
  - id: xxe-xml-parser
    description: "XML parser that may resolve external entities unless explicitly disabled"
    pattern: '(DocumentBuilderFactory|SAXParserFactory|XMLReader|XMLInputFactory|etree\.(parse|fromstring)|xml\.dom|libxml_)'
    include_globs: ["*.java", "*.py", "*.php"]
    severity: warning
---

# XXE — Defensive Methodology

## Signals
`<!DOCTYPE`/`<!ENTITY` in inbound XML, OOB callbacks from XML-parsing endpoints, and WAF XXE signatures.

## Detection Rules
Alert on DOCTYPE/ENTITY in request bodies to XML endpoints and on outbound resolutions triggered by uploads (SVG/DOCX/SOAP/SAML).

## Hardening
Disable DOCTYPE and external-entity resolution in every XML parser; prefer non-XML formats. The registered check flags XML parsers to confirm entities are disabled.

## Incident Response
Determine which files/SSRF targets were reachable, rotate anything exposed, and set secure parser defaults project-wide.

## References
OWASP XXE prevention; CWE-611.
