#!/usr/bin/env bash
# ci-smoke-model.sh <dest> — downloads the pinned CI smoke model
# (.github/embed-smoke-model.txt) and checks its sha256 (ADR-018).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
pin="$root/.github/embed-smoke-model.txt"
url="$(sed -n 's/^url: //p' "$pin")"
sha="$(sed -n 's/^sha256: //p' "$pin")"
dest="$1"
# sha256sum on Linux and Git for Windows, shasum on macOS.
check() {
  if command -v sha256sum >/dev/null; then echo "$sha  $dest" | sha256sum -c --quiet
  else echo "$sha  $dest" | shasum -a 256 -c --quiet; fi
}
if [[ ! -f "$dest" ]] || ! check 2>/dev/null; then
  curl -sSfL --retry 3 -o "$dest" "$url"
fi
check
chmod 0600 "$dest"
echo "ci-smoke-model: $dest ($sha)"
