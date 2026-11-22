🇺🇸 English | [🇧🇷 Português](pt-BR/release.md)

# How to publish a release

The workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml) publishes a GitHub Release with `tb` binaries when a `v*` tag is pushed to the remote. Configuration lives in [`.goreleaser.yaml`](../.goreleaser.yaml).

## Prerequisites

- Stable `main` (green CI)
- Matching entry in [`CHANGELOG.md`](../CHANGELOG.md)
- Before creating the tag, move the "Unreleased" section in [`CHANGELOG.md`](../CHANGELOG.md) to the version being published
- No conflicting local/remote tag
- No extra secrets: cosign signing is keyless via GitHub OIDC (`id-token: write`); SLSA attestation uses `attestations: write` + `GITHUB_TOKEN`

## Cut the release

```bash
git checkout main
git pull origin main

# Example: first release candidate
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1
```

Pushing the tag triggers the workflow: the vendored llama.cpp and advisory checks, `go test ./...` and then GoReleaser (with syft + cosign), which produces:

- `tar.gz` archives (and `zip` on Windows) for linux/darwin/windows × amd64/arm64
- SBOM per archive (syft, `*.sbom.json`)
- `checksums.txt` (includes archives and SBOMs)
- keyless checksum signature: `checksums.txt.sigstore.json` (cosign + OIDC)
- SLSA provenance attestation (`actions/attest-build-provenance` over `dist/checksums.txt`)
- a changelog from the commits
- GitHub Release `v1.0.0-rc.1` with the version injected via ldflags (`tb version` / MCP `tb_version`), created as a **draft**

## Release candidates (1.0.0)

A tag with an `-rc.N` suffix publishes a GitHub **prerelease** that is never marked `latest` (`release.prerelease: auto` in `.goreleaser.yaml`; the `publish` job passes `--prerelease --latest=false` for `-rc.*` tags). The flow, from [the 1.0 spec](specs/release-1.0.md#7-release-steps):

1. Tag `v1.0.0-rc.1` on a green `main`. The workflow builds the **draft**.
2. `smoke` and `floors` run on the draft assets (see below); `publish` then makes it public as a prerelease.
3. Install it as in [Verify a downloaded artifact](#verify-a-downloaded-artifact) (sha256 + bundle certificate) and use it on this repository for a **7-day soak** (decision O19). New findings go to the [dogfood](campo/dogfood.md) log.
4. A `high` finding is fixed on `main`, tagged `v1.0.0-rc.2` (never reuse a tag), and the soak restarts. Each fix gets the next `rc.N`.
5. When the exit criteria hold, rename `[1.0.0-rc.N]` to `[1.0.0]` in the changelog and tag `v1.0.0` on the same commit as the last rc, or on a docs-only descendant of it.

The stability and deprecation rules that apply from 1.0 are in [the freeze list, section 3](specs/release-1.0.md#3-freeze-list): a deprecated item keeps working, hidden from help and with a stderr warning, until 2.0.

## Native artifacts (ADR-018)

Every binary is built with cgo and the vendored llama.cpp ([ADR-018](adr/018-cgo-componentes-nativos.md)), tag `embednative`, CPU-only:

- **Toolchain:** the pinned zig (`ZIG_VERSION`/`ZIG_SHA256` in the workflow) is `CC`/`CXX` for all six targets. Each target first builds its static libraries with `scripts/build-native.sh <goos> <goarch>` (goreleaser `hooks.pre`; CMake + Ninja), then links libc++ statically.
- **Floors:** x86-64-v2 (no AVX/FMA/F16C) and armv8.0-a for every command; glibc 2.28 on Linux; macOS 11; Windows 10. On amd64 the embedding engine (`tb graph index --vectors`, `--mode semantic|hybrid`) needs an x86-64-v3 CPU: its ggml-cpu kernels use AVX2/FMA ([ADR-019](adr/019-cpu-v3-embedding.md)), and below that `tb __graph-embed` refuses with `embed_cpu_unsupported`. CI checks it: `scripts/avx-audit.py` keeps AVX-tier code inside ggml-cpu symbols, and `qemu-x86_64 -cpu Nehalem` runs the other commands.
- **Hardening:** linux and windows binaries are PIE; linux has full RELRO. `scripts/native-gate.sh artifact` checks the linux artifacts (PIE, RELRO, BIND_NOW, NX, canaries in the libraries, fortified calls, no symbol above `GLIBC_2.28`).
- **Draft, then public:** the release stays a draft until these jobs pass on the draft assets:
  - `smoke`: each target on its own runner (ubuntu-24.04, ubuntu-24.04-arm, macos-15, macos-15-intel, windows-2025, windows-11-arm) runs `tb version` (must name `llama.cpp`) and `tb __graph-embed --smoke` with the pinned CI model (`.github/embed-smoke-model.txt`);
  - `floors`: linux/amd64 in `rockylinux:8` (glibc 2.28) and under `qemu-x86_64 -cpu Nehalem` (x86-64-v2).

  Then `publish` makes it public. A failure leaves the draft: delete it and the tag, fix, and tag the next patch version (or the next `-rc.N`).

## Verify a downloaded artifact

```bash
VERSION=v1.0.0-rc.1
ASSET=toolbelt_1.0.0-rc.1_linux_amd64.tar.gz
BASE="https://github.com/leonardo-moraes360/toolbelt/releases/download/${VERSION}"

curl -sLO "${BASE}/${ASSET}"
curl -sLO "${BASE}/checksums.txt"
curl -sLO "${BASE}/checksums.txt.sigstore.json"

cosign verify-blob \
  --certificate-identity "https://github.com/leonardo-moraes360/toolbelt/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --bundle checksums.txt.sigstore.json \
  checksums.txt

sha256sum -c --ignore-missing checksums.txt
gh attestation verify "${ASSET}" --repo leonardo-moraes360/toolbelt
```

## Validate locally (without publishing)

```bash
goreleaser check
# Local snapshot: no GitHub OIDC, skip cosign signing. Needs cmake, ninja,
# the pinned zig, and syft + jq for the SBOMs (scripts/sbom.sh).
export ZIG=/path/to/zig TB_ROOT="$PWD"
goreleaser release --snapshot --clean --skip=sign
```

One target without goreleaser (the CI `native-cross` job does this for all six):

```bash
ZIG=/path/to/zig scripts/build-native.sh linux arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC="$PWD/scripts/zig-cc.sh" CXX="$PWD/scripts/zig-c++.sh" \
  go build -tags embednative,netgo,osusergo -buildmode=pie -ldflags "-s -w -extldflags '-Wl,-z,relro,-z,now'" \
  -o bin/tb-linux-arm64 ./cmd/tb
scripts/native-gate.sh artifact bin/tb-linux-arm64 internal/embed/native/build/linux_arm64/lib
```

Do not run a bulk `git push --tags` or reuse a tag that was already published. For a follow-up patch: `v0.0.6`, and so on.
