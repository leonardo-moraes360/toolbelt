---
name: red-ssrf
color: red
area: web
class: ssrf
mirror: blue-ssrf
triggers: ["ssrf", "server-side request forgery", "metadata", "imds", "url fetch", "webhook"]
summary: "Surface server-side request forgery: server-issued requests to attacker-influenced URLs, and the cloud-metadata pivot."
---

# Server-Side Request Forgery — Offensive Methodology

## Where It Hides
Features that fetch a URL the user influences: webhooks, link previews, PDF/image fetchers, import-from-URL, and SSO metadata. Any `host`, `url`, or `callback` parameter is a candidate.

## Detection
Point the fetcher at an in-scope collaborator you control and observe an inbound request. Test whether internal hosts, loopback, link-local (cloud metadata), and alternate schemes/encodings are reachable through filter gaps.

## Exploitation
Demonstrate reach to an internal-only endpoint or metadata service with a read that proves access, without exfiltrating live credentials beyond what confirms the finding.

## Impact
Internal service access, cloud credential theft via metadata, port scanning from the server, and chaining to RCE against internal apps.

## Remediation
Resolve and allow-list destinations, block internal ranges and metadata IPs, disable unneeded schemes/redirects, and require authenticated egress. Blue mirror watches for metadata-IP access and outbound anomalies.
