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

# Exemplo: primeiro release candidate
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1
```

O push da tag dispara o workflow: as verificações do llama.cpp vendorizado e de advisories, `go test ./...` e em seguida GoReleaser (com syft + cosign), que gera:

- arquivos `tar.gz` (e `zip` no Windows) para linux/darwin/windows × amd64/arm64
- SBOM por archive (syft, `*.sbom.json`)
- `checksums.txt` (inclui archives e SBOMs)
- assinatura keyless do checksum: `checksums.txt.sigstore.json` (cosign + OIDC)
- atestação de proveniência SLSA (`actions/attest-build-provenance` sobre `dist/checksums.txt`)
- changelog a partir dos commits
- GitHub Release `v1.0.0-rc.1` com a versão injetada via ldflags (`tb version` / MCP `tb_version`), criado como **draft**

## Release candidates (1.0.0)

Uma tag com sufixo `-rc.N` publica um **prerelease** do GitHub que nunca é marcado como `latest` (`release.prerelease: auto` no `.goreleaser.yaml`; o job `publish` passa `--prerelease --latest=false` para tags `-rc.*`). O fluxo, a partir da [spec da 1.0](../specs/release-1.0.md#7-release-steps):

1. Crie a tag `v1.0.0-rc.1` num `main` verde. O workflow gera o **draft**.
2. `smoke` e `floors` rodam nos assets do draft (veja abaixo); depois o `publish` o torna público como prerelease.
3. Instale como em [Verificar um artefato baixado](#verificar-um-artefato-baixado) (sha256 + certificado do bundle) e use neste repositório durante um **soak de 7 dias** (decisão O19). Achados novos vão para o log de [dogfood](../campo/dogfood.md).
4. Um achado `high` é corrigido no `main`, recebe a tag `v1.0.0-rc.2` (nunca reutilize uma tag) e o soak recomeça. Cada correção ganha o próximo `rc.N`.
5. Quando os critérios de saída valerem, renomeie `[1.0.0-rc.N]` para `[1.0.0]` no changelog e crie a tag `v1.0.0` no mesmo commit do último rc, ou num descendente só de docs.

As regras de estabilidade e depreciação válidas a partir da 1.0 estão na [lista de congelamento, seção 3](../specs/release-1.0.md#3-freeze-list): um item depreciado continua funcionando, oculto da ajuda e com aviso no stderr, até a 2.0.

## Artefatos nativos (ADR-018)

Todo binário é compilado com cgo e o llama.cpp vendorizado ([ADR-018](adr/018-cgo-componentes-nativos.md)), tag `embednative`, só CPU:

- **Toolchain:** o zig fixado (`ZIG_VERSION`/`ZIG_SHA256` no workflow) é o `CC`/`CXX` dos seis alvos. Cada alvo primeiro compila suas bibliotecas estáticas com `scripts/build-native.sh <goos> <goarch>` (`hooks.pre` do goreleaser; CMake + Ninja) e depois linka a libc++ estaticamente.
- **Pisos:** x86-64-v2 (sem AVX/FMA/F16C) e armv8.0-a para todos os comandos; glibc 2.28 no Linux; macOS 11; Windows 10. No amd64 o motor de embedding (`tb graph index --vectors`, `--mode semantic|hybrid`) exige uma CPU x86-64-v3: os kernels do ggml-cpu usam AVX2/FMA ([ADR-019](adr/019-cpu-v3-embedding.md)), e abaixo disso o `tb __graph-embed` recusa com `embed_cpu_unsupported`. O CI verifica: o `scripts/avx-audit.py` mantém o código de nível AVX dentro dos símbolos do ggml-cpu, e o `qemu-x86_64 -cpu Nehalem` roda os outros comandos.
- **Hardening:** os binários linux e windows são PIE; o linux tem RELRO completo. `scripts/native-gate.sh artifact` verifica os artefatos linux (PIE, RELRO, BIND_NOW, NX, canários nas bibliotecas, chamadas fortificadas, nenhum símbolo acima de `GLIBC_2.28`).
- **Draft, depois público:** o release fica em draft até estes jobs passarem nos assets do draft:
  - `smoke`: cada alvo no seu runner (ubuntu-24.04, ubuntu-24.04-arm, macos-15, macos-15-intel, windows-2025, windows-11-arm) roda `tb version` (precisa citar `llama.cpp`) e `tb __graph-embed --smoke` com o modelo de CI fixado (`.github/embed-smoke-model.txt`);
  - `floors`: linux/amd64 no `rockylinux:8` (glibc 2.28) e sob `qemu-x86_64 -cpu Nehalem` (x86-64-v2).

  Depois o `publish` torna o release público. Uma falha deixa o draft: apague o draft e a tag, corrija e crie a tag do próximo patch (ou do próximo `-rc.N`).

## Verificar um artefato baixado

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
