# Como publicar um release

O workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml) publica um GitHub Release com binários `tb` quando uma tag `v*` é enviada ao remoto. A configuração está em [`.goreleaser.yaml`](../.goreleaser.yaml).

## Pré-requisitos

- `main` estável (CI verde)
- Entrada correspondente no [`CHANGELOG.md`](../CHANGELOG.md)
- Sem tag local/remota conflitante

## Cortar o release

```bash
git checkout main
git pull origin main

# Exemplo: primeiro release
git tag v0.0.1
git push origin v0.0.1
```

O push da tag dispara o workflow: `go test ./...` e em seguida GoReleaser, que gera:

- arquivos `tar.gz` (e `zip` no Windows) para linux/darwin/windows × amd64/arm64
- `checksums.txt`
- changelog a partir dos commits
- GitHub Release `v0.0.1` com a versão injetada via ldflags (`tb version` / MCP `tb_version`)

## Validar localmente (sem publicar)

```bash
goreleaser check
goreleaser release --snapshot --clean
```

Não rode `git push --tags` em massa nem reutilize uma tag já publicada. Para um patch seguinte: `v0.0.2`, e assim por diante.
