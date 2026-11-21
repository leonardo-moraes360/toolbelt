#!/usr/bin/env bash
# native-fuzz.sh <seconds> — builds the vendored libraries and the shim with
# ASan, UBSan and libFuzzer (clang) and fuzzes tbe_embed for <seconds>
# with the model in TB_EMBED_MODEL (ADR-018 native gate).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
secs="${1:-300}"
: "${TB_EMBED_MODEL:?TB_EMBED_MODEL is required}"
san="-fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=undefined"
goos="$(go env GOOS)" goarch="$(go env GOARCH)"
# The vendored libraries skip -fsanitize=function: ggml calls its dequantize
# table through a type-punned function pointer by design (no local patches).
CC=clang CXX=clang++ EXTRA_CFLAGS="$san -fno-sanitize=function -fsanitize=fuzzer-no-link" "$root/scripts/build-native.sh" "$goos" "$goarch"
lib="$root/internal/embed/native/build/${goos}_${goarch}/lib"
out="$(mktemp -d)"
clang++ $san -fsanitize=fuzzer -I"$root/third_party/llama.cpp/include" -I"$root/third_party/llama.cpp/ggml/include" \
  -x c "$root/internal/embed/native/fuzz/fuzz_embed.c" -x c "$root/internal/embed/native/tbembed.c" -x none \
  -L"$lib" -lllama -lggml -lggml-cpu -lggml-base -lm -lpthread -o "$out/fuzz_embed"
mkdir -p "$out/corpus"
printf 'func main() {}' >"$out/corpus/a"
printf '\xff\xfe\x00 symlink' >"$out/corpus/b"
"$out/fuzz_embed" -max_total_time="$secs" -max_len=65536 -timeout=60 "$out/corpus"
