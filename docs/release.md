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

Pushing the tag triggers the workflow: `go test ./...` and then GoReleaser (with syft + cosign), which produces:

- `tar.gz` archives (and `zip` on Windows) for linux/darwin/windows × amd64/arm64
- SBOM per archive (syft, `*.sbom.json`)
- `checksums.txt` (includes archives and SBOMs)
- keyless checksum signature: `checksums.txt.sigstore.json` (cosign + OIDC)
- SLSA provenance attestation (`actions/attest-build-provenance` over `dist/checksums.txt`)
- a changelog from the commits
- GitHub Release `v0.0.1` with the version injected via ldflags (`tb version` / MCP `tb_version`)

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
# Local snapshot: no GitHub OIDC, skip cosign signing
goreleaser release --snapshot --clean --skip=sign
```

Do not run a bulk `git push --tags` or reuse a tag that was already published. For a follow-up patch: `v0.0.6`, and so on.
