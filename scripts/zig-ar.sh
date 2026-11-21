#!/usr/bin/env bash
# zig-ar.sh — "zig ar" / "zig ranlib" for CMake (ADR-018).
set -euo pipefail
mode=ar
[[ "$(basename "$0")" == zig-ranlib.sh ]] && mode=ranlib
exec "${ZIG:-zig}" "$mode" "$@"
