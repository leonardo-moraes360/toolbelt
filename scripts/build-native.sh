#!/usr/bin/env bash
# build-native.sh — builds the vendored llama.cpp static libraries for one
# Go target into internal/embed/native/build/<goos>_<goarch>/lib (ADR-018).
#
#   scripts/build-native.sh <goos> <goarch>
#
# With ZIG set (the pinned zig binary) the compiler is zig for the target's
# triple (scripts/zig-target.sh); otherwise CC/CXX or the host compiler.
# EXTRA_CFLAGS adds flags (sanitizers).
set -euo pipefail

goos="$1" goarch="$2"
root="$(cd "$(dirname "$0")/.." && pwd)"
src="$root/internal/embed/native"
out="$src/build/${goos}_${goarch}"

"$root/scripts/vendor-llama.sh" --check >/dev/null

case "$goos/$goarch" in
  linux/amd64)   sys=Linux;   cpu=x86_64 ;;
  linux/arm64)   sys=Linux;   cpu=aarch64 ;;
  darwin/amd64)  sys=Darwin;  cpu=x86_64 ;;
  darwin/arm64)  sys=Darwin;  cpu=arm64 ;;
  windows/amd64) sys=Windows; cpu=AMD64 ;;
  windows/arm64) sys=Windows; cpu=ARM64 ;;
  *) echo "build-native: unsupported target $goos/$goarch" >&2; exit 2 ;;
esac

harden="-O2 -fstack-protector-strong"
[[ "$goos" == windows ]] && harden="-O2" # mingw needs libssp for canaries (ADR-018)
[[ "$goarch" == amd64 ]] && harden="$harden -fcf-protection"
[[ "$goos" == linux ]] && harden="$harden -D_FORTIFY_SOURCE=2"
flags="$harden ${EXTRA_CFLAGS:-}"

cross=()
if [[ -n "${ZIG:-}" ]]; then
  ZIG_TARGET="$("$root/scripts/zig-target.sh" "$goos" "$goarch")"
  export ZIG ZIG_TARGET
  cross+=(-DCMAKE_C_COMPILER="$root/scripts/zig-cc.sh" -DCMAKE_CXX_COMPILER="$root/scripts/zig-c++.sh"
          -DCMAKE_AR="$root/scripts/zig-ar.sh" -DCMAKE_RANLIB="$root/scripts/zig-ranlib.sh")
  # CMake's Darwin platform module requires install_name_tool, which only
  # rewrites dylib paths; the build makes static archives, so it never runs.
  [[ "$goos" == darwin ]] && cross+=(-DCMAKE_INSTALL_NAME_TOOL="$(command -v true)")
fi
if [[ -n "${ZIG:-}" || "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" != "$goos/$goarch" ]]; then
  cross+=(-DCMAKE_SYSTEM_NAME="$sys" -DCMAKE_SYSTEM_PROCESSOR="$cpu" -DCMAKE_CROSSCOMPILING=ON)
fi

rm -rf "$out"
cmake -S "$src" -B "$out/cmake" -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_C_FLAGS="$flags" -DCMAKE_CXX_FLAGS="$flags" \
  -DCMAKE_ARCHIVE_OUTPUT_DIRECTORY="$out/lib" \
  "${cross[@]}"
cmake --build "$out/cmake" --parallel
# Each target writes its own archive; collect them in lib/.
mkdir -p "$out/lib"
find "$out/cmake" -name '*.a' -exec cp {} "$out/lib/" \;
# ggml drops the "lib" prefix on Windows; -l<name> needs lib<name>.a.
for a in "$out/lib"/*.a; do
  b="$(basename "$a")"
  [[ "$b" == lib* ]] || mv "$a" "$out/lib/lib$b"
done
# The Go standard library links libresolv and two frameworks on macOS;
# zig has no stubs for them (scripts/darwin-stubs.sh).
[[ "$goos" == darwin ]] && "$root/scripts/darwin-stubs.sh" "$out/lib"
ls "$out/lib"
