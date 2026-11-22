# Builds the vendored llama.cpp subset as static CPU-only libraries for one
# target (ADR-018). Driven by scripts/build-native.sh; never fetches
# anything and never builds tools, examples, tests or other backends.
cmake_minimum_required(VERSION 3.16)
project(tbembed C CXX)

set(TB_LLAMA "${CMAKE_CURRENT_SOURCE_DIR}/../../../third_party/llama.cpp")
get_filename_component(TB_LLAMA "${TB_LLAMA}" ABSOLUTE)

set(FETCHCONTENT_FULLY_DISCONNECTED ON CACHE BOOL "" FORCE)
set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
# ggml asks git for a commit; the vendored tree has none of its own.
find_program(TB_TRUE NAMES true REQUIRED)
set(GIT_EXE "${TB_TRUE}" CACHE FILEPATH "" FORCE)

# CPU only, statically linked, no dynamic backends, no OpenMP runtime.
foreach(opt GGML_BACKEND_DL GGML_CPU_ALL_VARIANTS GGML_NATIVE GGML_OPENMP GGML_METAL GGML_ACCELERATE GGML_BLAS
            GGML_CUDA GGML_HIP GGML_MUSA GGML_VULKAN GGML_SYCL GGML_OPENCL GGML_WEBGPU GGML_RPC GGML_CPU_KLEIDIAI
            GGML_CPU_HBM GGML_LTO GGML_CCACHE GGML_LLAMAFILE)
  set(${opt} OFF CACHE BOOL "" FORCE)
endforeach()

# The CPU floor (ADR-018) is x86-64-v2 on amd64 and armv8.0-a on arm64.
# ADR-019 moves only the ggml-cpu kernels on amd64 to the AVX2 tier;
# tb __graph-embed refuses with embed_cpu_unsupported on CPUs without it.
# TB_X86_KERNELS=v2 builds the ADR-018 floor (for benchmarks and fallback).
set(TB_X86_KERNELS "v3" CACHE STRING "amd64 ggml-cpu kernels: v2 or v3")
if(CMAKE_SYSTEM_PROCESSOR MATCHES "^(x86_64|AMD64|amd64)$")
  set(GGML_SSE42 ON CACHE BOOL "" FORCE)
  if(TB_X86_KERNELS STREQUAL "v3")
    set(tb_x86_on GGML_AVX GGML_AVX2 GGML_BMI2 GGML_FMA GGML_F16C)
    set(tb_x86_off GGML_AVX_VNNI GGML_AVX512 GGML_AVX512_VBMI GGML_AVX512_VNNI GGML_AVX512_BF16 GGML_AMX_TILE
                   GGML_AMX_INT8 GGML_AMX_BF16)
  elseif(TB_X86_KERNELS STREQUAL "v2")
    set(tb_x86_on "")
    set(tb_x86_off GGML_AVX GGML_AVX2 GGML_AVX_VNNI GGML_BMI2 GGML_FMA GGML_F16C GGML_AVX512 GGML_AVX512_VBMI
                   GGML_AVX512_VNNI GGML_AVX512_BF16 GGML_AMX_TILE GGML_AMX_INT8 GGML_AMX_BF16)
  else()
    message(FATAL_ERROR "TB_X86_KERNELS must be v2 or v3, not ${TB_X86_KERNELS}")
  endif()
  foreach(opt ${tb_x86_on})
    set(${opt} ON CACHE BOOL "" FORCE)
  endforeach()
  foreach(opt ${tb_x86_off})
    set(${opt} OFF CACHE BOOL "" FORCE)
  endforeach()
elseif(CMAKE_SYSTEM_PROCESSOR MATCHES "^(aarch64|arm64|ARM64)$")
  set(GGML_CPU_ARM_ARCH "armv8-a" CACHE STRING "" FORCE)
endif()

set(LLAMA_ALL_WARNINGS OFF CACHE BOOL "" FORCE)
# llama's cmake/common.cmake includes "ggml/cmake/common.cmake" relative to
# the llama source root, which is not ours. Include ggml's first by absolute
# path, then a build-tree copy of llama's without that one line (the vendored
# file itself stays byte for byte upstream).
include("${TB_LLAMA}/ggml/cmake/common.cmake")
file(READ "${TB_LLAMA}/cmake/common.cmake" TB_LLAMA_COMMON)
string(REPLACE "include(\"ggml/cmake/common.cmake\")" "" TB_LLAMA_COMMON "${TB_LLAMA_COMMON}")
file(WRITE "${CMAKE_BINARY_DIR}/llama-common.cmake" "${TB_LLAMA_COMMON}")
include("${CMAKE_BINARY_DIR}/llama-common.cmake")
add_subdirectory("${TB_LLAMA}/ggml" ggml)
add_subdirectory("${TB_LLAMA}/src" llama)
