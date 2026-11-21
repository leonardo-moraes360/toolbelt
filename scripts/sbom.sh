#!/usr/bin/env bash
# sbom.sh <artifact> <document> — the release SBOM (goreleaser "sboms"):
# syft's SPDX JSON for the archive, plus the vendored llama.cpp and ggml
# (ADR-018, spec AC4). syft finds Go modules but not C/C++ compiled from
# third_party/, so both are added from third_party/llama.cpp/MANIFEST with
# the pinned commit, the subset hash and a CONTAINS relationship from the
# document's root.
set -euo pipefail
artifact="$1" doc="$2"
root="$(cd "$(dirname "$0")/.." && pwd)"
manifest="$root/third_party/llama.cpp/MANIFEST"
field() { sed -n "s/^$1: //p" "$manifest"; }
tag="$(field tag)" commit="$(field commit)" subset="$(field subset_sha256)"
ggml="$(sed -n 's/^set(GGML_VERSION_\(MAJOR\|MINOR\|PATCH\) \([0-9]*\))$/\2/p' \
  "$root/third_party/llama.cpp/ggml/CMakeLists.txt" | paste -sd. -)"
[[ -n "$tag" && "$commit" =~ ^[0-9a-f]{40}$ && "$subset" =~ ^[0-9a-f]{64}$ && -n "$ggml" ]] ||
  { echo "sbom: incomplete MANIFEST" >&2; exit 1; }

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
syft "$artifact" --output "spdx-json=$tmp"

jq --arg tag "$tag" --arg commit "$commit" --arg subset "$subset" --arg ggml "$ggml" '
  def pkg($id; $name; $version; $purl): {
    name: $name,
    SPDXID: $id,
    versionInfo: $version,
    supplier: "Organization: ggml-org",
    downloadLocation: ("git+https://github.com/ggml-org/llama.cpp@" + $commit),
    filesAnalyzed: false,
    licenseConcluded: "MIT",
    licenseDeclared: "MIT",
    copyrightText: "NOASSERTION",
    comment: ("Vendored subset of llama.cpp " + $tag + " (third_party/llama.cpp), compiled into tb; subset_sha256 " + $subset + " (scripts/vendor-llama.sh)."),
    externalRefs: [{referenceCategory: "PACKAGE-MANAGER", referenceType: "purl", referenceLocator: $purl}]
  };
  # The archive syft described: its DESCRIBES relationship (or the older
  # documentDescribes field), else the document itself.
  ([.relationships[]? | select(.spdxElementId == "SPDXRef-DOCUMENT" and .relationshipType == "DESCRIBES")
    | .relatedSpdxElement][0] // (.documentDescribes // [])[0] // "SPDXRef-DOCUMENT") as $from
  | .packages += [
      pkg("SPDXRef-Package-vendored-llama.cpp"; "llama.cpp"; $tag; "pkg:github/ggml-org/llama.cpp@" + $commit),
      pkg("SPDXRef-Package-vendored-ggml"; "ggml"; $ggml; "pkg:github/ggml-org/llama.cpp@" + $commit + "#ggml")
    ]
  | .relationships += [
      {spdxElementId: $from, relationshipType: "CONTAINS", relatedSpdxElement: "SPDXRef-Package-vendored-llama.cpp"},
      {spdxElementId: "SPDXRef-Package-vendored-llama.cpp", relationshipType: "CONTAINS", relatedSpdxElement: "SPDXRef-Package-vendored-ggml"}
    ]
' "$tmp" >"$doc"
