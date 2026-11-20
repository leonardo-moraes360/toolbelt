---
name: blue-oauth
color: blue
area: auth
class: oauth
mirror: red-oauth
triggers: ["oauth detection", "consent phishing", "token theft", "redirect abuse", "aitm"]
summary: "Detect and harden against OAuth/OIDC abuse: redirect/state/PKCE gaps and token leakage."
---

# OAuth / OIDC — Defensive Methodology

## Signals
Authorizations to unusual `redirect_uri` values, missing/!verified `state`, consent grants to newly-registered clients, and tokens appearing in URLs, logs, or referrers.

## Detection Rules
Alert on redirect-uri mismatches against the registered allow-list, on consent-phishing patterns (new client + broad scopes), and on AiTM indicators. Largely config/behavioral — no static grep ships for OAuth.

## Hardening
Exact-match redirect URIs, mandatory verified `state`, PKCE for public clients, short-lived codes, and never placing tokens in URLs.

## Incident Response
Revoke issued tokens/grants, review consent history, and tighten client registration and redirect validation.

## References
RFC 9700 (OAuth 2.0 security BCP); CWE-601.
