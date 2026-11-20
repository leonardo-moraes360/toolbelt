---
name: red-xxe
color: red
area: web
class: xxe
mirror: blue-xxe
triggers: ["xxe", "xml external entity", "doctype", "entity", "xml parser"]
summary: "Surface XML external entity injection: XML parsers that resolve external entities."
---

# XML External Entity — Offensive Methodology

## Where It Hides
Any endpoint parsing XML (or formats layered on XML: SVG, DOCX, SOAP, SAML) with a parser left at defaults that resolve `DOCTYPE`/external entities.

## Detection
Submit a document declaring a benign external entity pointing at an in-scope collaborator; an inbound resolution confirms external-entity processing. Prefer out-of-band confirmation when direct reflection is absent.

## Exploitation
Demonstrate local file read or SSRF via entity resolution with a single proof artifact. Avoid large-scale file exfiltration or billion-laughs style denial of service.

## Impact
Local file disclosure, SSRF, and denial of service.

## Remediation
Disable DOCTYPE and external-entity resolution in the parser; prefer non-XML formats. The blue mirror flags XML parsing that does not disable entities.
