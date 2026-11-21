#!/usr/bin/env python3
"""avx-audit.py — ADR-019 gate: AVX-tier code may live only in ggml-cpu.

    scripts/avx-audit.py <tb-binary> <native-lib-dir>

The amd64 ggml-cpu kernels are built for the AVX2 tier; every other native
object keeps the baseline. The binary fails the audit when a VEX/EVEX or
BMI2 instruction sits in a native symbol that is not defined only by
libggml-cpu.a. A symbol that ggml-cpu defines together with another native
library is a COMDAT copy (inline or template code): the linker keeps one of
them for every caller, so an AVX copy there could run in any command.

Go symbols are skipped: the Go toolchain targets goamd64 v1 and dispatches
its own AVX code on CPUID at run time.
"""
import collections
import re
import subprocess
import sys
from pathlib import Path

VEX_REG = re.compile(r"%[yz]mm\d+|%k[0-7]\b")
BMI2 = {"bzhi", "mulx", "pdep", "pext", "rorx", "sarx", "shlx", "shrx"}
# VEX-encoded mnemonics start with "v"; these legacy ones also do.
NOT_VEX = {"verr", "verw"}
HEADER = re.compile(r"^[0-9a-f]+ <(.+)>:$")
INSN = re.compile(r"^\s+[0-9a-f]+:\s+(\S+)(.*)$")


def defined_symbols(lib: Path) -> set:
    out = subprocess.run(["nm", "--defined-only", "-P", str(lib)], capture_output=True, text=True, check=True).stdout
    syms = set()
    for line in out.splitlines():
        parts = line.split()
        # "name type value size"; archive member headers end with ":".
        if len(parts) >= 2 and not parts[0].endswith(":") and parts[1] in "TtWw":
            syms.add(parts[0])
    return syms


def is_avx_tier(mnemonic: str, operands: str) -> bool:
    m = mnemonic.split(".")[0]
    if m in BMI2:
        return True
    if m.startswith("v") and m not in NOT_VEX:
        return True
    return bool(VEX_REG.search(operands))


def main() -> int:
    binary, libdir = Path(sys.argv[1]), Path(sys.argv[2])
    libs = sorted(libdir.glob("lib*.a"))
    if not libs:
        print(f"avx-audit: no static libraries in {libdir}", file=sys.stderr)
        return 2
    owners = collections.defaultdict(set)
    for lib in libs:
        for s in defined_symbols(lib):
            owners[s].add(lib.name)
    cpu_lib = "libggml-cpu.a"
    if not any(cpu_lib in o for o in owners.values()):
        print(f"avx-audit: {cpu_lib} not found in {libdir}", file=sys.stderr)
        return 2

    proc = subprocess.Popen(["objdump", "-d", "--no-show-raw-insn", str(binary)], stdout=subprocess.PIPE, text=True)
    bad = collections.Counter()
    allowed = 0
    sym = None
    for line in proc.stdout:
        h = HEADER.match(line)
        if h:
            sym = h.group(1).split("@")[0]
            continue
        i = INSN.match(line)
        if not i or sym is None or sym not in owners:
            continue  # Go, libc++ outside our libs, or no symbol
        if not is_avx_tier(i.group(1), i.group(2)):
            continue
        if owners[sym] == {cpu_lib}:
            allowed += 1
        else:
            bad[(sym, ",".join(sorted(owners[sym])))] += 1
    if proc.wait() != 0:
        print("avx-audit: objdump failed", file=sys.stderr)
        return 2
    if bad:
        print(f"avx-audit: {len(bad)} native symbols outside ggml-cpu carry AVX-tier code:")
        for (s, libs_), n in bad.most_common(40):
            print(f"  {n:6d}  {s}  [{libs_}]")
        return 1
    print(f"avx-audit: ok ({allowed} AVX-tier instructions, all in symbols only libggml-cpu.a defines)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
