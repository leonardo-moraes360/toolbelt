---
name: red-oauth
color: red
area: auth
class: oauth
mirror: blue-oauth
triggers: ["oauth", "oidc", "redirect uri", "pkce", "state parameter", "token leakage"]
summary: "Surface OAuth/OIDC flaws: redirect-uri abuse, missing state/PKCE, and token leakage."
---

# OAuth / OIDC Attacks — Offensive Methodology

## Where It Hides
The authorization flow: `redirect_uri` validation, the `state` parameter (CSRF), PKCE on public clients, scope handling, and where tokens land (URL fragments, logs, referrers).

## Detection
Test redirect-uri matching for loose prefix/substring/open-redirect gaps; check whether `state` is present and verified; check whether public clients enforce PKCE; look for tokens exposed in URLs or passed to third parties.

## Exploitation
Demonstrate a single controlled redirect or missing-state acceptance using a test client/account. Do not capture real users' tokens.

## Impact
Account takeover via code/token theft, CSRF-style account linking, and scope escalation.

## Remediation
Exact-match redirect URIs, mandatory verified `state`, PKCE for public clients, short-lived codes, and never placing tokens in URLs. OAuth flaws are largely logic/config — the blue mirror covers consent-phishing and token-abuse detection, not a static grep.
