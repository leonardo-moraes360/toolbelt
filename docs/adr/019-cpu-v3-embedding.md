🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/019-cpu-v3-embedding.md)

# ADR-019: AVX2/FMA kernels for the embedding child, guarded at run time

## Status

proposed (2026-11-21). It amends [ADR-018](018-cgo-componentes-nativos.md) §1 (the amd64 CPU floor) and its rejected alternative "AVX2/FMA builds for speed". It becomes accepted after the spike below passes.

## Context

ADR-018 builds llama.cpp for **x86-64-v2** (SSE4.2, no AVX) on amd64. Its reason: llama.cpp's static initializers run in every `tb` process, so an instruction above the floor would kill all of `tb` with SIGILL, not only embedding. It accepted slower embedding as the price.

That price turned out high, and the risk smaller than assumed. Both were measured on 2026-11-21 (all-MiniLM-L6-v2 Q8_0, llama.cpp b12600, 4 threads, a local amd64 host with AVX2):

| 64 texts | x86-64-v2 (release) | x86-64-v3 (AVX, AVX2, FMA, F16C, BMI2) |
|---|---|---|
| 1.5 KB chunks | 8.6 s | 3.9 s (**2.2×**) |
| short queries | 0.29 s | 0.15 s (1.9×) |

Batching the texts of a request in the shim (dogfood #47, added with #163) saved only 4% when indexing: the cost is the arithmetic without AVX/FMA, not the number of calls. A full index of this repository on the floor takes about 25 minutes (dogfood #43).

**Static initializers do not run AVX in this build.** A x86-64-v3 build of `tb` ran under `qemu-x86_64 -cpu Nehalem`, which has no AVX and on which QEMU raises SIGILL for AVX instructions:
- `version`, `graph stats`, `graph search`, `mem list`, `sandbox check`, `bus channel list` and `guard check` all exited 0, with no illegal instruction;
- `__graph-embed --smoke` died with SIGILL, as expected, and the x86-64-v2 build passed the same smoke.

So AVX code runs only when the engine is called, and that happens only in the embedding child. The ADR-018 risk is real for the child and, so far, not for other commands. That is evidence from one build and one command set, not a proof: a compiler or upstream change could put AVX code on a path every command runs. The gate below has to catch that.

## Decision

1. **amd64 CPU kernels at the AVX2 tier.** The release builds the vendored ggml CPU backend with `GGML_AVX`, `GGML_AVX2`, `GGML_FMA`, `GGML_F16C` and `GGML_BMI2` on. That is the compute part of x86-64-v3, not the whole level (no LZCNT or MOVBE); the user-facing requirement is stated as x86-64-v3 to stay conservative. `GGML_AVX512*`, `GGML_AVX_VNNI` and `GGML_AMX_*` stay off, because many current CPUs lack them. The Go code keeps the release's `goamd64: v1`, and the shim, llama and the rest of ggml keep their current baseline target (zig's `-mcpu=baseline`); only the ggml-cpu flags (`ARCH_FLAGS`) move.
2. **A run-time guard in the child.** At the top of `tb __graph-embed` (before both `Serve` and `Smoke`, so before the first `native.Open`), the child checks, in Go (`golang.org/x/sys/cpu`), `X86.HasAVX`, `HasAVX2`, `HasFMA`, `HasF16C` and `HasBMI2`. `HasAVX`/`HasAVX2` already include the OSXSAVE/XCR0 check that the OS saves AVX state, and BMI2 is checked because hypervisors can mask it while exposing AVX2. If any is missing, the child refuses with a new reason, **`embed_cpu_unsupported`**, and native code never runs. The parent maps it like the other child reasons (it joins the known codes the client reads from the child's stderr). Every other command is unaffected.
3. **The gate keeps AVX code out of every other path.** What protects the other commands is the invariant that only the child calls native code; the checks below enforce it.
   - **Only the child calls native:** `TestOnlyNativeImportsC` already limits `import "C"` to `internal/embed/native`; a new test also requires that `native.Open` is reached only from the `__graph-embed` command (a call-graph check over the packages).
   - **Where AVX code may live:** a script disassembles the whole binary and fails if any VEX or EVEX instruction lies outside symbols defined by ggml-cpu objects. This catches COMDAT folding: an inline or template function (from libc++ headers, for example) instantiated in a ggml-cpu translation unit built with `-mavx2` may be the copy the linker keeps for every caller, including llama's static initializers. The build also compiles ggml-cpu with `-fvisibility-inlines-hidden` to make such sharing less likely.
   - **Under QEMU:** a CI job runs the release binary under `qemu-x86_64 -cpu Nehalem` for `tb version` and a few read-only commands (`graph stats`, `mem list`, `sandbox check`). Static initialization is the same for every command, so this is a smoke check, not the main protection.
   - **Guard check:** under the same QEMU CPU, `__graph-embed --smoke` must fail with `embed_cpu_unsupported`, not SIGILL.
   - **Real hardware:** the smoke matrix keeps a real embedding run on x86-64-v3 runners. The x86-64-v2 QEMU job of ADR-018 now expects `embed_cpu_unsupported` from the embed smoke, and `tb version` to pass.
4. **arm64 is unchanged** (armv8.0-a with NEON). Dotprod or i8mm kernels are measured before any change, under the same rules.
5. **Docs.** `docs/release.md` and the README say that embeddings need an x86-64-v3 CPU on amd64 (Intel Haswell from 2013, AMD Excavator from 2015, or later), and that every other command keeps the x86-64-v2 floor.

### Spike before acceptance

- Build the six targets with the amd64 kernels at v3, and run the command matrix and the init audit in CI.
- Measure on a CI runner: embedding time for v2 and v3, and binary size.
- Run the recall evaluation (`TestSemanticRecall`) on v3, and require recall to match v2 within 0.01. The arithmetic changes (FMA), so the vectors are not bit-identical.

## Consequences

### Positive

- About 2× faster embedding on amd64, where most users are. A full index of this repository drops from about 25 to about 12 minutes on 4 threads.
- The ADR-018 floor becomes a checked fact for every command, not an assumption about static initializers.

### Negative

- On amd64 CPUs without AVX2, embedding stops working (`embed_cpu_unsupported`): parts from before 2013, and some later low-end ones (Pentium, Celeron and Atom models without AVX). Every other command still works there.
- The guard, the disassembly audit and the QEMU smoke are new CI cost.
- The evidence rests on testing, not on a property of the build: an upstream change could move AVX code into a path that runs everywhere. The call-graph test and the disassembly audit are meant to catch that before a release.

## Alternatives considered

- **Keep x86-64-v2 (ADR-018 as is).** It is the simplest and works everywhere, at half the speed. Rejected on the measurements, but it stays the fallback if the spike fails.
- **Two kernel variants in one binary.** Build ggml-cpu twice (v2 and v3) with prefixed symbols and pick one by CPUID in the child. Older CPUs keep embedding, but the vendored build gets a symbol-renaming step and twice the kernel code. Deferred until a user on a pre-v3 CPU needs embeddings.
- **`GGML_BACKEND_DL` with `GGML_CPU_ALL_VARIANTS`.** This is upstream's own dispatch, but it loads `libggml-cpu-*.so` at run time. That breaks the single file, and every load path would need verifying (ADR-018 rejected dynamic loading).
- **A separate `tb-embed` artifact at v3.** It would keep the main binary at v2 with no doubt left, but ADR-018 rejected splitting the release and the docs.
