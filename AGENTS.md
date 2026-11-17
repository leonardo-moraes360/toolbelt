# AGENTS

## Toolbelt

Este repositório pode usar [toolbelt](https://github.com/leonardo-moraes360/toolbelt) (`tb`) como harness para agentes.

### Toolchain detectada

- testes (`verify.test_cmd`): `go test ./...`
- build (`verify.build_cmd`): `go build ./...`
- lint (`verify.lint_cmd`): `golangci-lint run`

### Uso do tb

- `tb verify` — roda test/build/lint configurados em `.toolbelt/config.yaml`
- `tb guard check` — checagens anti-trapaça no diff
- `tb mem` — invariantes fora da compactação do contexto

Ajuste `verify.test_cmd` (e, se houver, `build_cmd`/`lint_cmd`) se a detecção estiver incorreta.
