#!/usr/bin/env bash
# vendor-llama.sh — reproducibly vendors the allowlisted subset of one
# llama.cpp release into third_party/llama.cpp (ADR-018, spec
# docs/specs/vector-index.md, V0).
#
#   scripts/vendor-llama.sh <tag> <tarball> <tarball-sha256> <commit>
#   scripts/vendor-llama.sh --check
#
# The tarball is https://github.com/ggml-org/llama.cpp/archive/refs/tags/<tag>.tar.gz,
# downloaded by the caller. No local patches are allowed: the subset is the
# upstream files, byte for byte. --check recomputes the subset hash of the
# vendored tree and compares it with MANIFEST.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$root/third_party/llama.cpp"

# The allowlist: paths relative to the tarball's top directory. Directories
# are copied whole; everything else (examples, tools, server, common,
# vendor/, gguf-py, tests, other ggml backends) is left out.
allow=(
  LICENSE AUTHORS
  CMakeLists.txt
  cmake
  include
  src
  ggml/CMakeLists.txt
  ggml/cmake
  ggml/include
  ggml/src/CMakeLists.txt
  ggml/src/ggml-cpu
)
# Files directly under ggml/src (the core, not the backend directories).
core_glob='ggml/src/*.c ggml/src/*.cpp ggml/src/*.h ggml/src/*.in'

# subset_hash prints the sha256 of the sorted "sha256  path" list of every
# vendored file except MANIFEST and ADVISORIES.md.
subset_hash() {
  (cd "$1" && find . -type f ! -name MANIFEST ! -name ADVISORIES.md -print0 |
    LC_ALL=C sort -z | xargs -0 sha256sum) | sha256sum | cut -d' ' -f1
}

if [[ "${1:-}" == "--check" ]]; then
  want="$(sed -n 's/^subset_sha256: //p' "$dest/MANIFEST")"
  got="$(subset_hash "$dest")"
  if [[ -z "$want" || "$want" != "$got" ]]; then
    echo "vendor-llama: third_party/llama.cpp does not match MANIFEST (want $want, got $got)" >&2
    exit 1
  fi
  if [[ -n "$(find "$dest" -type l -print -quit)" ]]; then
    echo "vendor-llama: symlink in the vendored tree" >&2
    exit 1
  fi
  echo "vendor-llama: subset matches MANIFEST ($got)"
  exit 0
fi

if [[ $# -ne 4 ]]; then
  echo "usage: $0 <tag> <tarball> <tarball-sha256> <commit> | --check" >&2
  exit 2
fi
tag="$1" tarball="$2" tar_sha="$3" commit="$4"
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || { echo "vendor-llama: commit must be 40 hex" >&2; exit 2; }
echo "$tar_sha  $tarball" | sha256sum -c --quiet

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
tar -xzf "$tarball" -C "$work" --no-same-owner --no-same-permissions
top="$(find "$work" -mindepth 1 -maxdepth 1 -type d -print -quit)"
if [[ -n "$(find "$top" -type l -print -quit)" ]]; then
  echo "vendor-llama: the tarball contains symlinks; refusing" >&2
  exit 1
fi

keep=(MANIFEST ADVISORIES.md)
find "$dest" -mindepth 1 -maxdepth 1 $(printf '! -name %s ' "${keep[@]}") -exec rm -rf {} +
for p in "${allow[@]}"; do
  mkdir -p "$dest/$(dirname "$p")"
  cp -R "$top/$p" "$dest/$p"
done
(cd "$top" && shopt -s nullglob && for f in $core_glob; do cp "$f" "$dest/$f"; done)

sum="$(subset_hash "$dest")"
cat >"$dest/MANIFEST" <<EOF
# Vendored llama.cpp subset (ADR-018). Written by scripts/vendor-llama.sh;
# do not edit by hand. Check: scripts/vendor-llama.sh --check
upstream: https://github.com/ggml-org/llama.cpp
tag: $tag
commit: $commit
tarball_sha256: $tar_sha
subset_sha256: $sum
license: MIT
EOF
echo "vendor-llama: vendored $tag ($commit), subset $sum"
