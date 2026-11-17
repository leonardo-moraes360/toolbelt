package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestReleaseSupplyChainGuards(t *testing.T) {
	root := findModuleRoot(t)
	assertGoReleaserSupplyChain(t, readRepoFile(t, root, ".goreleaser.yaml"))
	assertReleaseWorkflowSupplyChain(t, readRepoFile(t, root, ".github/workflows/release.yml"))
	assertInstallDocsSupplyChain(t, readRepoFile(t, root, "README.md"))
	assertGoToolchainAndVulnCI(t, readRepoFile(t, root, "go.mod"), readRepoFile(t, root, ".github/workflows/ci.yml"), readRepoFile(t, root, ".github/workflows/release.yml"))
}

func assertGoReleaserSupplyChain(t *testing.T, goreleaser string) {
	t.Helper()
	if !regexp.MustCompile(`(?m)^sboms:\s*$`).MatchString(goreleaser) {
		t.Error(".goreleaser.yaml: falta bloco sboms: (syft)")
	}
	if !regexp.MustCompile(`(?m)^signs:\s*$`).MatchString(goreleaser) {
		t.Error(".goreleaser.yaml: falta bloco signs: (cosign)")
	}
	if !strings.Contains(goreleaser, "cosign") {
		t.Error(".goreleaser.yaml: signs deve usar cosign")
	}
	if !strings.Contains(goreleaser, "artifacts: checksum") && !strings.Contains(goreleaser, "artifacts:checksum") {
		t.Error(".goreleaser.yaml: signs deve assinar artifacts: checksum")
	}
}

func assertReleaseWorkflowSupplyChain(t *testing.T, releaseWF string) {
	t.Helper()
	if !strings.Contains(releaseWF, "id-token: write") {
		t.Error("release.yml: falta permissão id-token: write (OIDC keyless)")
	}
	if !strings.Contains(releaseWF, "attestations: write") {
		t.Error("release.yml: falta permissão attestations: write (SLSA)")
	}
	if !strings.Contains(releaseWF, "attest-build-provenance") {
		t.Error("release.yml: falta actions/attest-build-provenance")
	}
	if !strings.Contains(releaseWF, "cosign-installer") && !strings.Contains(releaseWF, "sigstore/cosign") {
		t.Error("release.yml: falta instalação do cosign")
	}
	if !strings.Contains(releaseWF, "syft") && !strings.Contains(releaseWF, "sbom-action") {
		t.Error("release.yml: falta instalação do syft")
	}
}

func assertInstallDocsSupplyChain(t *testing.T, readme string) {
	t.Helper()
	if regexp.MustCompile(`(?s)curl\s+.*?\|\s*tar\b`).MatchString(readme) {
		t.Error("README.md: instalação via curl|tar sem verificação de checksum/assinatura")
	}
	if !strings.Contains(readme, "sha256sum") || !strings.Contains(readme, "-c") {
		t.Error("README.md: deve verificar checksums com sha256sum -c")
	}
	if !strings.Contains(readme, "cosign verify-blob") {
		t.Error("README.md: deve verificar assinatura com cosign verify-blob")
	}
	if regexp.MustCompile(`(?i)homebrew|brew install`).MatchString(readme) {
		t.Error("README.md: não deve mencionar Homebrew enquanto o tap 404")
	}
}

func assertGoToolchainAndVulnCI(t *testing.T, goMod, ciWF, releaseWF string) {
	t.Helper()
	if parseGoDirective(goMod) < 1.25 {
		t.Errorf("go.mod: diretiva go=%.2f tem vulnerabilidades conhecidas; precisa >= 1.25", parseGoDirective(goMod))
	}
	if regexp.MustCompile(`go-version:\s*['"]?1\.22`).MatchString(ciWF) {
		t.Error("ci.yml: ainda fixa go-version 1.22 vulnerável")
	}
	if regexp.MustCompile(`go-version:\s*['"]?1\.22`).MatchString(releaseWF) {
		t.Error("release.yml: ainda fixa go-version 1.22 vulnerável")
	}
	if !strings.Contains(ciWF, "govulncheck") {
		t.Error("ci.yml: falta job/step govulncheck")
	}
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("lendo %s: %v", rel, err)
	}
	return string(b)
}

func parseGoDirective(goMod string) float64 {
	re := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
	m := re.FindStringSubmatch(goMod)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}
