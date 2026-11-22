🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/018-cgo-componentes-nativos.md)

# ADR-018: cgo for native components, starting with vendored llama.cpp, under a C/C++ quality gate

## Status

proposed (2026-11-21). It becomes accepted only after the V0 spike of the [vector index spec](../specs/vector-index.md) builds and smoke-tests every target (spec AC1–AC3).

It amends:
- [ADR-001](001-go-vs-rust.md): the single static binary;
- [ADR-014](014-graphify-reimplementacao.md): cgo deferred.

Its first user is the vector index.

## Context

The roadmap has an optional vector index for `tb graph search`, and optional local inference. Both need an embedding model that runs on the user's machine.

- **No pure-Go option.** No maintained pure-Go runtime runs current embedding models (GGUF or ONNX) at usable speed. The model has to come from C or C++, or from a process outside `tb`.
- **Today's build.** The release is `CGO_ENABLED=0` for six targets: linux, darwin and windows on amd64 and arm64. ADR-001 counts the single static binary as a requirement. ADR-014 deferred tree-sitter because cgo complicates the release.
- **Posture.** The project is fail-closed:
  - a Landlock sandbox (ADR-004);
  - hardened git (R4c);
  - a verified release (sha256, keyless cosign, SBOM, SLSA).

  Native code must not be the weak spot in that chain.
- **What the sandbox does not cover.** The sandbox confines the filesystem only. It has no network isolation (no Landlock net rules, no seccomp, no netns), and it always grants read access to the system directories (`/usr`, `/etc`, `/proc`, `/sys`, …).
- **Evidence.** The field study ([SINTESE.md](../campo/SINTESE.md)) did not observe a failure that vectors would fix. The feature must stay opt-in and measure its gain.

Two choices were made when this ADR was written:

1. **Engine:** [llama.cpp](https://github.com/ggml-org/llama.cpp), vendored at a pinned tag and built from source. It is MIT, C/C++, has no external dependencies, and runs GGUF embedding models. onnxruntime was rejected because it ships as a prebuilt binary that no gate can analyze.
2. **Scope:** the **whole release** is built with cgo. There is no second, pure-Go artifact.

## Decision

1. **cgo for every release target.**
   - **Build options:** the release passes `-tags embednative` and builds CPU-only. No Accelerate, BLAS, Metal, CUDA or Vulkan; no `GGML_BACKEND_DL` or `GGML_CPU_ALL_VARIANTS`, which need dynamic loading.
   - **CPU floor, explicit:**
     - amd64 targets **x86-64-v2** (SSE4.2, no AVX), with every `GGML_AVX*`, `GGML_FMA` and `GGML_F16C` option off;
     - arm64 targets **armv8.0-a** with NEON.

     llama.cpp's static initializers run in every `tb` process. An instruction above the floor would kill all of `tb` with SIGILL, not only embedding. Embedding is slower than a native build would be, and that is the accepted price.
   - **Dev builds:** a build without the tag (`go build ./...`, `go test ./...`) has no native code. Its `tb __graph-embed` answers `embed_unavailable`.
2. **Vendoring.**
   - llama.cpp and ggml are vendored under `third_party/llama.cpp/` as an **allowlisted subset** of one upstream tag. It contains the `ggml` core and CPU backend, `src/`, `include/`, the CMake files they need and the licenses; no examples, tools, server, rpc, gguf-py or tests.
   - `scripts/vendor-llama.sh <tag>` produces the subset reproducibly from the upstream tarball.
   - `MANIFEST` records the tag, the commit, the tarball sha256, the hash of exactly the subset and the license.
   - There are no local patches. An upgrade is one PR that runs the script and changes the manifest.
3. **Native code at run time.**
   - **Linking:** the static link puts the C++ static initializers in every `tb` process, including the sandbox trampoline. They run before `main`.
   - **Out of process:** native code is *called* only from a hidden child, `tb __graph-embed`, which speaks JSON lines over stdin and stdout.
   - **Child sandbox:**
     - the Landlock rules allow reads of the system directories that the sandbox always grants, plus the model file and the `tb` binary (execute);
     - the workdir is an empty private temp dir, read-only;
     - the HOME is empty and the environment minimal;
     - devices are minimal;
     - **a new seccomp filter makes `socket(2)` and `socketpair(2)` fail with `EPERM`**, and Landlock ABI ≥ 6 scoping is added where available.

     A memory bug in llama.cpp, or a hostile model file, can then corrupt only that child. It reaches neither the repository nor HOME, and it has no network.
   - **Platforms without the sandbox:** on Linux without Landlock or seccomp, and on darwin or windows, the child refuses (`embed_unsandboxed`) unless the user config sets `graph.embed.allow_unsandboxed: true`.
   - **Import rule:** only `internal/embed/native` may import `"C"`, enforced by a test (`TestOnlyNativeImportsC`), because depguard cannot see the cgo pseudo-package.
4. **A C/C++ quality gate shaped by use.** It is a new CI job, `native`, required for merge. The C/C++ code is used in three ways, and each gets its own checks.

   | Code | Owner | Checks |
   |------|-------|--------|
   | The shim (`internal/embed/native/*.c`, `*.h`), the only code that calls llama.cpp | this repo | `-Wall -Wextra -Wpedantic -Werror`; `clang-tidy` (bugprone, cert, clang-analyzer, performance) with zero findings; `cppcheck --enable=warning,portability --error-exitcode=1`; `clang-format --dry-run -Werror`; CodeQL `c-cpp` alerts under `internal/embed/native/` fail the job through a SARIF check step (alerts do not block on their own) |
   | Vendored llama.cpp and ggml | upstream | no lint and no patches; `MANIFEST` must match the subset; CodeQL alerts under `third_party/` open an issue and never block or get silenced in place; a scheduled advisory job (below) |
   | Everything compiled together | both | hardening flags per platform (below), checked on the artifacts; a sanitizer build of shim and vendor; a libFuzzer harness |

   - **Advisories:** a scheduled job reads the [GitHub advisories](https://github.com/ggml-org/llama.cpp/security/advisories) and OSV for llama.cpp. An advisory affects the pin when its fixed-in tag is above the pinned tag, or when it has no fix. An affecting advisory fails the job and blocks releases. The exception is an advisory in a component that is not vendored or not compiled (server, rpc, a GPU backend, …): it may be excepted for 30 days in `third_party/llama.cpp/ADVISORIES.md`, with the reason, reviewed in a PR. An expired exception blocks again.
   - **Hardening:**
     - all targets: `-O2 -fstack-protector-strong`; on amd64 also `-fcf-protection`;
     - linux: `-D_FORTIFY_SOURCE=2` (level 3 needs glibc ≥ 2.34, above the floor), `go build -buildmode=pie` and `-extldflags "-Wl,-z,relro,-z,now"`;
     - darwin: PIE is the default;
     - windows: `-buildmode=pie` with `--dynamicbase --nxcompat` (high-entropy ASLR). `-fstack-protector-strong` is used only where libssp links.

     The job checks the linux artifacts with `readelf`/`checksec`: PIE, full RELRO, NX, stack canaries and fortified calls present. `objdump -T` must show no symbol version above `GLIBC_2.28`.
   - **Sanitizers:**
     - The vendored static libs are also built with `-fsanitize=address,undefined`.
     - The embed tests run with `go test -asan -tags embednative` (UBSan through `CGO_CFLAGS`/`CGO_LDFLAGS`) on linux/amd64 and linux/arm64.
   - **Fuzzing:** a libFuzzer harness drives the shim entry points (text → tokenize → embed, and batch shapes) with the CI smoke model under ASan: 5 minutes per PR and 1 hour nightly. Fuzzing GGUF parsing stays upstream's job, because the model is pinned by the user.
   - **Static init:** `tb version` runs under `strace -f -e trace=file,network`. Its trace must equal that of a build without the tag, so static initializers do no I/O.
5. **Builds and smoke tests.**
   - **V0 spike:** before acceptance, V0 builds each target with a pinned [zig](https://ziglang.org) as `CC`/`CXX`, with goreleaser's per-build `hooks.pre` (CMake) and per-target `CGO_LDFLAGS`, and links libc++ statically.
   - **Fallbacks, named in advance:**
     - darwin: native clang on macOS runners, with goreleaser split/merge;
     - windows: llvm-mingw.
   - **Glibc floor:** Linux targets glibc 2.28, and a `rockylinux:8` container checks it.
   - **Native smoke matrix:** every target runs `tb version` and an embed smoke test on its own runner (ubuntu-24.04 amd64 and arm64, macos-15 arm64, macos-15-intel, windows-2025 amd64 and windows-11-arm), plus x86-64-v2 under `qemu-x86_64 -cpu Nehalem`. A failure blocks the release.
6. **The release chain covers the native parts.**
   - The SBOM (SPDX) gains packages for llama.cpp and ggml (tag, commit, license, subset hash).
   - `tb version` prints the engine and its commit.
   - The checksums, cosign signature and SLSA provenance are unchanged.
7. **The model is user data.**
   - Its path and sha256 come from the user config only.
   - The file must be regular, owned by the user, reached through no symlink, and outside the repository.
   - The child re-checks the hash before loading.
   - tb never downloads a model.

## Consequences

### Positive

- Local embeddings with no external service, no network and no Python. The same engine can later serve local inference.
- Native risk is contained twice: by the gate before merge, and by the sandboxed, network-less child at run time.
- The sandbox gains a seccomp network block that other children can use.
- A cgo toolchain also enables `go test -race` in CI.

### Negative

- **Release cost:** zig, CMake per target, a six-runner smoke matrix, a QEMU job and a sanitizer build.
- **No longer one static binary:**
  - Linux needs glibc ≥ 2.28;
  - amd64 has a CPU floor (x86-64-v2);
  - the binary grows, by an amount V0 must measure.
- **Static initializers:** C++ static initializers from llama.cpp run in every `tb` command. They are checked to do no I/O, but they are a new pre-`main` surface.
- **Upstream pace:** llama.cpp moves fast. An affecting advisory blocks releases until an upgrade, or until an exception for a component that is not compiled.
- **Trusted base:** a C++ GGUF parser is in the binary. The pinned hash and the sandboxed child are mitigations, not a guarantee.

## Alternatives considered

- **External embedding command** (`graph.embed_cmd`), keeping `CGO_ENABLED=0`. It is simplest for the release. It was rejected as the primary path because quality and setup would depend on what each user installs.
- **cgo behind a separate opt-in artifact** (`tb-embed`). It would keep the default binary static, but it would split the release and the docs in two.
- **onnxruntime prebuilt.** It is an opaque binary: the gate could only pin hashes.
- **sqlite-vec.** Not needed: the index is `graph.json`, and brute-force cosine in Go covers the expected sizes. It can be revisited through a WASM build without cgo.
- **AVX2/FMA builds for speed.** Rejected: static initializers run in every command, so a CPU without AVX2 would lose all of `tb`, not just embedding. *Revisited by [ADR-019](019-cpu-v3-embedding.md): measurements showed the static initializers run no AVX code, so the amd64 ggml-cpu kernels now use AVX2/FMA behind a CPU guard and a disassembly audit.*
- **Rust component.** Still possible later behind the same child protocol.
