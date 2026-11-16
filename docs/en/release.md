[🇧🇷 Português](../release.md) | 🇺🇸 English

# How to publish a release

The workflow [`.github/workflows/release.yml`](../../.github/workflows/release.yml) publishes a GitHub Release with `tb` binaries when a `v*` tag is pushed to the remote. Configuration lives in [`.goreleaser.yaml`](../../.goreleaser.yaml).

## Prerequisites

- Stable `main` (green CI)
- Matching entry in [`CHANGELOG.md`](../../CHANGELOG.md)
- Before creating the tag, move the "Não Lançado" section in [`CHANGELOG.md`](../../CHANGELOG.md) to the version being published
- No conflicting local/remote tag

## Cut the release

```bash
git checkout main
git pull origin main

# Example: first release
git tag v0.0.1
git push origin v0.0.1
```

Pushing the tag triggers the workflow: `go test ./...` and then GoReleaser, which produces:

- `tar.gz` archives (and `zip` on Windows) for linux/darwin/windows × amd64/arm64
- `checksums.txt`
- a changelog from the commits
- GitHub Release `v0.0.1` with the version injected via ldflags (`tb version` / MCP `tb_version`)

## Validate locally (without publishing)

```bash
goreleaser check
goreleaser release --snapshot --clean
```

Do not run a bulk `git push --tags` or reuse a tag that was already published. For a follow-up patch: `v0.0.2`, and so on.
