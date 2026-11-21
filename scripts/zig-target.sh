#!/usr/bin/env bash
# zig-target.sh <goos> <goarch> — the zig target of a release target
# (ADR-018): Linux pins glibc 2.28; macOS and Windows use zig's own libc.
set -euo pipefail
case "$1/$2" in
  linux/amd64)   echo x86_64-linux-gnu.2.28 ;;
  linux/arm64)   echo aarch64-linux-gnu.2.28 ;;
  darwin/amd64)  echo x86_64-macos.11.0 ;;
  darwin/arm64)  echo aarch64-macos.11.0 ;;
  windows/amd64) echo x86_64-windows-gnu ;;
  windows/arm64) echo aarch64-windows-gnu ;;
  *) echo "zig-target: unsupported $1/$2" >&2; exit 2 ;;
esac
