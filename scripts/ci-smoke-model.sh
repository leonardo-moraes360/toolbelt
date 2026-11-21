#!/usr/bin/env bash
# ci-smoke-model.sh <dest> — downloads the pinned CI smoke model
# (.github/embed-smoke-model.txt) and checks its sha256 (ADR-018).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
pin="$root/.github/embed-smoke-model.txt"
url="$(sed -n 's/^url: //p' "$pin")"
sha="$(sed -n 's/^sha256: //p' "$pin")"
dest="$1"
# Compare the digest itself: GNU sha256sum (Linux, Git for Windows), the
# BSD sha256sum of macOS 14+ (no --quiet) and shasum all print it first.
check() {
  local got
  if command -v shasum >/dev/null; then got="$(shasum -a 256 "$dest")"
  else got="$(sha256sum "$dest")"; fi
  [[ "${got%% *}" == "$sha" ]] || { echo "ci-smoke-model: $dest has sha256 ${got%% *}, want $sha" >&2; return 1; }
}
if [[ ! -f "$dest" ]] || ! check 2>/dev/null; then
  curl -sSfL --proto '=https' --tlsv1.2 --retry 3 -o "$dest" "$url"
fi
check
chmod 0600 "$dest"
echo "ci-smoke-model: $dest ($sha)"
