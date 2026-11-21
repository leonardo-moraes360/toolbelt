#!/usr/bin/env bash
# zig-cc.sh — the release's C/C++ compiler (ADR-018): "zig cc" or "zig c++"
# for $ZIG_TARGET, or for the GOOS/GOARCH that go build exports to cgo
# (scripts/zig-target.sh). Called as zig-c++.sh it is the C++ driver. ZIG
# is the pinned zig binary (default: zig).
#
# The CPU is always zig's baseline for the target (x86-64 and armv8.0-a):
# a -march=<arch> from ggml's CMake is dropped, because zig reads -march as
# a CPU name and rejects "armv8-a". Feature flags (-msse4.2) still apply.
set -euo pipefail
if [[ -z "${ZIG_TARGET:-}" ]]; then
  ZIG_TARGET="$("$(dirname "$(readlink -f "$0")")/zig-target.sh" "${GOOS:?}" "${GOARCH:?}")"
fi
mode=cc
[[ "$(basename "$0")" == zig-c++.sh ]] && mode=c++
args=()
for a in "$@"; do
  case "$a" in
    -march=*) ;;
    *) args+=("$a") ;;
  esac
done
exec "${ZIG:-zig}" "$mode" -target "$ZIG_TARGET" -mcpu=baseline "${args[@]}"
