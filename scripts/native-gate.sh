#!/usr/bin/env bash
# native-gate.sh — the C/C++ quality gate of ADR-018.
#
#   scripts/native-gate.sh lint                  shim warnings, format, clang-tidy, cppcheck, vendor hash
#   scripts/native-gate.sh artifact <tb-binary> [<lib-dir>]
#                                                Linux hardening and the GLIBC_2.28 floor
#   scripts/native-gate.sh strace <tagged> <untagged>
#                                                static initializers do no I/O
set -euo pipefail
export LC_ALL=C # readelf and objdump output is parsed below

root="$(cd "$(dirname "$0")/.." && pwd)"
shim="$root/internal/embed/native"
inc=(-I"$root/third_party/llama.cpp/include" -I"$root/third_party/llama.cpp/ggml/include")

lint() {
  "$root/scripts/vendor-llama.sh" --check
  for cc in gcc clang; do
    command -v "$cc" >/dev/null || continue
    "$cc" -std=c11 -O2 -Wall -Wextra -Wpedantic -Werror -Wshadow -Wconversion -Wno-sign-conversion \
      -D_FORTIFY_SOURCE=2 -fstack-protector-strong "${inc[@]}" -c "$shim/tbembed.c" -o /dev/null
    echo "native-gate: $cc -Werror clean"
  done
  clang-format --style=file:"$shim/.clang-format" --dry-run -Werror "$shim"/tbembed.c "$shim"/tbembed.h
  clang-tidy --quiet --config-file="$shim/.clang-tidy" "$shim/tbembed.c" -- -std=c11 "${inc[@]}"
  cppcheck --enable=warning,portability --error-exitcode=1 --inline-suppr --quiet \
    --suppress=missingIncludeSystem "${inc[@]}" "$shim/tbembed.c"
  echo "native-gate: lint ok"
}

artifact() {
  local bin="$1" libs="${2:-}" fail=0 hdr prog dyn dsym top
  # Read each tool's output once: "grep -q" on a pipe would close it early
  # and pipefail would report the writer's SIGPIPE as a failed check.
  hdr="$(readelf -hW "$bin")" prog="$(readelf -lW "$bin")" dyn="$(readelf -dW "$bin")"
  dsym="$(nm -D "$bin" 2>/dev/null || true)"
  grep -q 'Type:.*DYN' <<<"$hdr" || { echo "native-gate: not PIE"; fail=1; }
  grep -q GNU_RELRO <<<"$prog" || { echo "native-gate: no RELRO"; fail=1; }
  grep -Eq 'BIND_NOW|FLAGS.*NOW' <<<"$dyn" || { echo "native-gate: RELRO not full (no BIND_NOW)"; fail=1; }
  local stack
  stack="$(grep GNU_STACK <<<"$prog" || true)"
  if [[ -z "$stack" || "$stack" == *RWE* ]]; then echo "native-gate: executable stack (or no GNU_STACK)"; fail=1; fi
  grep -Eq '__[a-z]+_chk@' <<<"$dsym" || { echo "native-gate: no fortified calls"; fail=1; }
  # zig links its own __stack_chk_fail statically, so the canaries are
  # checked on the native libraries the binary was linked from.
  if [[ -n "$libs" ]]; then
    local lib
    for lib in "$libs"/*.a; do
      grep -q 'U __stack_chk_fail' <<<"$(nm "$lib" 2>/dev/null || true)" ||
        { echo "native-gate: no stack canaries in $(basename "$lib")"; fail=1; }
    done
  elif ! grep -q __stack_chk_fail <<<"$dsym"; then
    echo "native-gate: no stack canaries (pass the library directory)"; fail=1
  fi
  top="$(objdump -T "$bin" | grep -o 'GLIBC_[0-9.]*' | sort -uV | tail -1 || true)"
  if [[ -n "$top" ]] && [[ "$(printf '%s\nGLIBC_2.28\n' "$top" | sort -V | tail -1)" != GLIBC_2.28 ]]; then
    echo "native-gate: needs $top, above GLIBC_2.28"; fail=1
  fi
  [[ $fail -eq 0 ]] && echo "native-gate: artifact ok (newest symbol ${top:-none})"
  return $fail
}

# trace prints the file and network syscalls of "tb version", without the
# dynamic loader's library lookups, as sorted unique "syscall path" lines.
# The trace goes to a file: tb's own stdout is discarded.
trace() {
  local log
  log="$(mktemp)"
  # Signals are not I/O, and the Go runtime sends itself SIGURG
  # (preemption) at random points: leave them out of the trace.
  strace -f -qq -e trace=%file,%network -e signal=none -o "$log" "$1" version >/dev/null 2>&1
  grep -Ev '^([0-9]+ +)?(--- SIG|\+\+\+ )' "$log" | sed -E 's/^[0-9]+ +//; s/\(.*"([^"]*)".*/ \1/; s/\(.*//' |
    grep -Ev '(/etc/ld\.so|/lib(32|64)?/|/usr/lib(32|64)?/|/proc/self/exe| /dev/null)' | LC_ALL=C sort -u || true
  rm -f "$log"
}

case "${1:-}" in
  lint) lint ;;
  artifact) artifact "$2" "${3:-}" ;;
  strace)
    tagged="$(trace "$2")" untagged="$(trace "$3")"
    if [[ -z "$tagged" || -z "$untagged" ]]; then
      echo "native-gate: empty strace output; the check would prove nothing"; exit 1
    fi
    if ! diff <(echo "$tagged") <(echo "$untagged"); then
      echo "native-gate: static initializers of the native build do I/O (diff above)"; exit 1
    fi
    echo "native-gate: strace matches the untagged build ($(wc -l <<<"$tagged") syscall lines)" ;;
  *) echo "usage: $0 lint | artifact <bin> [<lib-dir>] | strace <tagged> <untagged>" >&2; exit 2 ;;
esac
