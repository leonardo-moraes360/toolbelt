🇧🇷 Português | [🇺🇸 English](en/release.md)

# Como publicar um release

O workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml) publica um GitHub Release com binários `tb` quando uma tag `v*` é enviada ao remoto. A configuração está em [`.goreleaser.yaml`](../.goreleaser.yaml).

## Pré-requisitos

- `main` estável (CI verde)
- Entrada correspondente no [`CHANGELOG.md`](../CHANGELOG.md)
- Antes de criar a tag, mova a seção "Não Lançado" do [`CHANGELOG.md`](../CHANGELOG.md) para a versão que está sendo publicada
- Sem tag local/remota conflitante
- Sem secrets extras: assinatura cosign é keyless via OIDC do GitHub (`id-token: write`); atestação SLSA usa `attestations: write` + `GITHUB_TOKEN`

## Cortar o release

```bash
git checkout main
git pull origin main

# Exemplo: primeiro release
git tag v0.0.1
git push origin v0.0.1
```

O push da tag dispara o workflow: `go test ./...` e em seguida GoReleaser (com syft + cosign), que gera:

- arquivos `tar.gz` (e `zip` no Windows) para linux/darwin/windows × amd64/arm64
- SBOM por archive (syft, `*.sbom.json`)
- `checksums.txt` (inclui archives e SBOMs)
- assinatura keyless do checksum: `checksums.txt.sigstore.json` (cosign + OIDC)
- atestação de proveniência SLSA (`actions/attest-build-provenance` sobre `dist/checksums.txt`)
- changelog a partir dos commits
- GitHub Release `v0.0.1` com a versão injetada via ldflags (`tb version` / MCP `tb_version`)

## Verificar um artefato baixado

```bash
VERSION=v0.0.2
ASSET=toolbelt_0.0.2_linux_amd64.tar.gz
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

## Validar localmente (sem publicar)

```bash
goreleaser check
# Snapshot local: sem OIDC do GitHub, pule a assinatura cosign
goreleaser release --snapshot --clean --skip=sign
```

Não rode `git push --tags` em massa nem reutilize uma tag já publicada. Para um patch seguinte: `v0.0.2`, e assim por diante.
