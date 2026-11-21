[🇺🇸 English](../release.md) | 🇧🇷 Português

# Como publicar um release

O workflow [`.github/workflows/release.yml`](../../.github/workflows/release.yml) publica um GitHub Release com binários `tb` quando uma tag `v*` é enviada ao remoto. A configuração está em [`.goreleaser.yaml`](../../.goreleaser.yaml).

## Pré-requisitos

- `main` estável (CI verde)
- Entrada correspondente no [`CHANGELOG.md`](../../CHANGELOG.md)
- Antes de criar a tag, mova a seção "Unreleased" do [`CHANGELOG.md`](../../CHANGELOG.md) para a versão que está sendo publicada
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

O push da tag dispara o workflow: as verificações do llama.cpp vendorizado e de advisories, `go test ./...` e em seguida GoReleaser (com syft + cosign), que gera:

- arquivos `tar.gz` (e `zip` no Windows) para linux/darwin/windows × amd64/arm64
- SBOM por archive (syft, `*.sbom.json`)
- `checksums.txt` (inclui archives e SBOMs)
- assinatura keyless do checksum: `checksums.txt.sigstore.json` (cosign + OIDC)
- atestação de proveniência SLSA (`actions/attest-build-provenance` sobre `dist/checksums.txt`)
- changelog a partir dos commits
- GitHub Release `v0.0.1` com a versão injetada via ldflags (`tb version` / MCP `tb_version`), criado como **draft**

## Artefatos nativos (ADR-018)

Todo binário é compilado com cgo e o llama.cpp vendorizado ([ADR-018](adr/018-cgo-componentes-nativos.md)), tag `embednative`, só CPU:

- **Toolchain:** o zig fixado (`ZIG_VERSION`/`ZIG_SHA256` no workflow) é o `CC`/`CXX` dos seis alvos. Cada alvo primeiro compila suas bibliotecas estáticas com `scripts/build-native.sh <goos> <goarch>` (`hooks.pre` do goreleaser; CMake + Ninja) e depois linka a libc++ estaticamente.
- **Pisos:** x86-64-v2 (sem AVX/FMA/F16C) e armv8.0-a; glibc 2.28 no Linux; macOS 11; Windows 10.
- **Hardening:** os binários linux e windows são PIE; o linux tem RELRO completo. `scripts/native-gate.sh artifact` verifica os artefatos linux (PIE, RELRO, BIND_NOW, NX, canários nas bibliotecas, chamadas fortificadas, nenhum símbolo acima de `GLIBC_2.28`).
- **Draft, depois público:** o release fica em draft até estes jobs passarem nos assets do draft:
  - `smoke`: cada alvo no seu runner (ubuntu-24.04, ubuntu-24.04-arm, macos-15, macos-15-intel, windows-2025, windows-11-arm) roda `tb version` (precisa citar `llama.cpp`) e `tb __graph-embed --smoke` com o modelo de CI fixado (`.github/embed-smoke-model.txt`);
  - `floors`: linux/amd64 no `rockylinux:8` (glibc 2.28) e sob `qemu-x86_64 -cpu Nehalem` (x86-64-v2).

  Depois o `publish` torna o release público. Uma falha deixa o draft: apague o draft e a tag, corrija e crie a tag do próximo patch.

## Verificar um artefato baixado

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

## Validar localmente (sem publicar)

```bash
goreleaser check
# Snapshot local: sem OIDC do GitHub, pule a assinatura cosign. Precisa de
# cmake, ninja, do zig fixado, e de syft + jq para os SBOMs (scripts/sbom.sh).
export ZIG=/caminho/do/zig TB_ROOT="$PWD"
goreleaser release --snapshot --clean --skip=sign
```

Um alvo sem goreleaser (o job `native-cross` da CI faz isso para os seis):

```bash
ZIG=/caminho/do/zig scripts/build-native.sh linux arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC="$PWD/scripts/zig-cc.sh" CXX="$PWD/scripts/zig-c++.sh" \
  go build -tags embednative,netgo,osusergo -buildmode=pie -ldflags "-s -w -extldflags '-Wl,-z,relro,-z,now'" \
  -o bin/tb-linux-arm64 ./cmd/tb
scripts/native-gate.sh artifact bin/tb-linux-arm64 internal/embed/native/build/linux_arm64/lib
```

Não rode `git push --tags` em massa nem reutilize uma tag já publicada. Para um patch seguinte: `v0.0.6`, e assim por diante.
