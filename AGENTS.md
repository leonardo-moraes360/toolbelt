# AGENTS

## Toolbelt

This repository can use [toolbelt](https://github.com/leonardo-moraes360/toolbelt) (`tb`) as a harness for agents.

### Detected toolchain

- tests (`verify.test_cmd`): `go test ./...`
- build (`verify.build_cmd`): `go build ./...`
- lint (`verify.lint_cmd`): `go vet ./...` (CI runs `golangci-lint`)

### Using tb

- `tb verify` — runs the test/build/lint configured in `.toolbelt/config.yaml`
- `tb guard check` — anti-cheat checks on the diff
- `tb mem` — invariants outside context compaction

Adjust `verify.test_cmd` (and, if present, `build_cmd`/`lint_cmd`) if detection is wrong.
