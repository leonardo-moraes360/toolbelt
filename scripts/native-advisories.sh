#!/usr/bin/env bash
# native-advisories.sh — fails when a published advisory affects the pinned
# llama.cpp (ADR-018 §4). An advisory affects the pin when the pin does not
# contain its fix (the patched tag is not an ancestor of the pinned commit,
# by GitHub's compare API) or when it has no fix; OSV is also queried by
# commit. A row of third_party/llama.cpp/ADVISORIES.md excepts one advisory
# until its Expires date. Needs gh (authenticated), jq and curl.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
manifest="$root/third_party/llama.cpp/MANIFEST"
commit="$(sed -n 's/^commit: //p' "$manifest")"
today="$(date -u +%F)"

# excepted <id>: a row "| id | component | reason | Date | Expires |" holds
# while today <= Expires, and only when Expires is at most 30 days after
# Date (ADR-018 §4). A malformed date never excepts.
excepted() {
  local date expires limit
  read -r date expires < <(awk -F'|' -v id="$1" '
    { for (i = 2; i <= 6; i++) gsub(/^ +| +$/, "", $i) }
    $2 == id { print $5, $6; exit }' "$root/third_party/llama.cpp/ADVISORIES.md") || return 1
  [[ "$date" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ && "$expires" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || return 1
  limit="$(date -u -d "$date + 30 days" +%F 2>/dev/null)" || return 1
  [[ ! "$expires" > "$limit" && ! "$today" > "$expires" ]]
}

fail=0
report() {
  if excepted "$1"; then echo "advisories: $1 excepted until its Expires date ($2)"; return; fi
  echo "advisories: $1 affects the pinned llama.cpp $commit: $2"; fail=1
}

# Fields are split on the unit separator (\x1f): a tab is IFS whitespace,
# so an empty field would collapse.
# Fetched first: a failed request (auth, rate limit) must fail the job,
# not read as "no advisories".
adv="$(gh api --paginate "repos/ggml-org/llama.cpp/security-advisories?state=published&per_page=100" \
  -q '.[] | [.ghsa_id, ([.vulnerabilities[]?.patched_versions] | map(select(. != null and . != "")) | join(",")), .summary] | join("\u001f")')"

while IFS=$'\x1f' read -r id patched summary; do
  [[ -z "$id" ]] && continue
  if [[ -z "$patched" || "$patched" == null ]]; then
    report "$id" "no fix published ($summary)"; continue
  fi
  fixed=0
  for tag in $(tr -d '>= ' <<<"$patched" | tr ',' ' '); do
    st="$(gh api "repos/ggml-org/llama.cpp/compare/${tag}...${commit}" -q .status 2>/dev/null || echo unknown)"
    [[ "$st" == ahead || "$st" == identical ]] && fixed=1
  done
  [[ $fixed -eq 1 ]] || report "$id" "fixed in $patched, not contained in the pin ($summary)"
done <<<"$adv"

osv="$(curl -sSf --proto '=https' --tlsv1.2 -X POST https://api.osv.dev/v1/query -d "{\"commit\":\"$commit\"}")"
while read -r id; do
  [[ -n "$id" ]] && report "$id" "OSV lists the pinned commit as affected"
done < <(jq -r '.vulns[]?.id' <<<"$osv")

[[ $fail -eq 0 ]] && echo "advisories: none affects llama.cpp $commit"
exit $fail
