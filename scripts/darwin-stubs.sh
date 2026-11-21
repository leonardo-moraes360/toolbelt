#!/usr/bin/env bash
# darwin-stubs.sh <out-dir> — writes linker stubs (.tbd text files) for the
# macOS system libraries that the Go standard library itself links against
# (ADR-018 V0): libresolv (net) and the CoreFoundation and Security
# frameworks (crypto/x509). zig ships libSystem for macOS but not these, and
# a cgo build links externally, so the linker needs a stub for each.
#
# A stub only names the install path and the exported symbols; nothing from
# it ends up in the binary except the load command. At run time dyld loads
# the real library from the system. The symbol lists come from the
# //go:cgo_import_dynamic lines of the Go toolchain doing the build, so they
# always match what that Go version imports.
set -euo pipefail

out="$1"
goroot="$(go env GOROOT)"
mkdir -p "$out/Frameworks"

# stub <install-name> <file> writes a TAPI v4 stub with the symbols that
# the Go sources import from <install-name>.
stub() {
  local install="$1" file="$2" syms
  syms="$(grep -rhoE "^//go:cgo_import_dynamic [^ ]+ [^ ]+ \"$install\"" "$goroot/src" |
    awk '{print "_" $3}' | LC_ALL=C sort -u | paste -sd, - | sed 's/,/, /g' || true)"
  if [[ -z "$syms" ]]; then
    echo "darwin-stubs: no symbols import $install" >&2
    exit 1
  fi
  mkdir -p "$(dirname "$file")"
  cat >"$file" <<EOF
--- !tapi-tbd
tbd-version: 4
targets: [ x86_64-macos, arm64-macos ]
install-name: '$install'
current-version: 1
compatibility-version: 1
exports:
  - targets: [ x86_64-macos, arm64-macos ]
    symbols: [ $syms ]
...
EOF
}

stub /usr/lib/libresolv.9.dylib "$out/libresolv.tbd"
for fw in CoreFoundation Security; do
  stub "/System/Library/Frameworks/$fw.framework/Versions/A/$fw" "$out/Frameworks/$fw.framework/$fw.tbd"
done
