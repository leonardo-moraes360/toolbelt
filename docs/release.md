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

# Example: first release
git tag v0.0.1
git push origin v0.0.1
```

Pushing the tag triggers the workflow: the vendored llama.cpp and advisory checks, `go test ./...` and then GoReleaser (with syft + cosign), which produces:

- `tar.gz` archives (and `zip` on Windows) for linux/darwin/windows × amd64/arm64
- SBOM per archive (syft, `*.sbom.json`)
- `checksums.txt` (includes archives and SBOMs)
- keyless checksum signature: `checksums.txt.sigstore.json` (cosign + OIDC)
- SLSA provenance attestation (`actions/attest-build-provenance` over `dist/checksums.txt`)
- a changelog from the commits
- GitHub Release `v0.0.1` with the version injected via ldflags (`tb version` / MCP `tb_version`), created as a **draft**

## Native artifacts (ADR-018)

Every binary is built with cgo and the vendored llama.cpp ([ADR-018](adr/018-cgo-componentes-nativos.md)), tag `embednative`, CPU-only:

- **Toolchain:** the pinned zig (`ZIG_VERSION`/`ZIG_SHA256` in the workflow) is `CC`/`CXX` for all six targets. Each target first builds its static libraries with `scripts/build-native.sh <goos> <goarch>` (goreleaser `hooks.pre`; CMake + Ninja), then links libc++ statically.
- **Floors:** x86-64-v2 (no AVX/FMA/F16C) and armv8.0-a; glibc 2.28 on Linux; macOS 11; Windows 10.
- **Hardening:** linux and windows binaries are PIE; linux has full RELRO. `scripts/native-gate.sh artifact` checks the linux artifacts (PIE, RELRO, BIND_NOW, NX, canaries in the libraries, fortified calls, no symbol above `GLIBC_2.28`).
- **Draft, then public:** the release stays a draft until these jobs pass on the draft assets:
  - `smoke`: each target on its own runner (ubuntu-24.04, ubuntu-24.04-arm, macos-15, macos-15-intel, windows-2025, windows-11-arm) runs `tb version` (must name `llama.cpp`) and `tb __graph-embed --smoke` with the pinned CI model (`.github/embed-smoke-model.txt`);
  - `floors`: linux/amd64 in `rockylinux:8` (glibc 2.28) and under `qemu-x86_64 -cpu Nehalem` (x86-64-v2).

  Then `publish` makes it public. A failure leaves the draft: delete it and the tag, fix, and tag the next patch version.

## Verify a downloaded artifact

```bash
VERSION=v0.0.5
ASSET=toolbelt_0.0.5_linux_amd64.tar.gz
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
