package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// Shared by converge2: the R10 reviewers, ranges, budget and the goldfish
// runner. The first measurement mode (converge, dogfood #32) was removed after
// #33; it is in commit 637683a.

// reviewerSpec is one R10 reviewer of the consolidated preset. The lens text is
// the spike's own one-liner (the upstream personas are not copied); code
// sources are not sent, so implementation-readiness and codebase-details only
// see the document.
type reviewerSpec struct {
	Name, Lens, Range string // Range: whole | feature | detailed
}

var r10 = []reviewerSpec{
	{"zoom-out", "Afaste-se: o documento resolve o problema certo? Escopo, premissas e alternativas ignoradas.", "whole"},
	{"inventive-catalyst", "Procure abordagens mais simples ou melhores que o documento não considerou e que mudariam o desenho.", "whole"},
	{"comprehension", "Leia como alguém novo no projeto: o que não dá para entender, termos sem definição, saltos de raciocínio.", "whole"},
	{"design", "Avalie o desenho: contradições, casos de borda sem comportamento definido, acoplamentos e riscos de segurança.", "feature"},
	{"structural", "Avalie a estrutura de spec de feature: problema, objetivos, não-objetivos, requisitos verificáveis, plano de verificação.", "whole"},
	{"style-feature", "Avalie o estilo da parte de feature: frases ambíguas, termos inconsistentes, redundância.", "feature"},
	{"implementation-readiness", "Avalie se o plano detalhado basta para implementar sem adivinhar: arquivos, passos, testes, critérios de saída.", "detailed"},
	{"codebase-details", "Avalie se o que o documento afirma sobre o código e os arquivos é plausível e consistente consigo mesmo.", "whole"},
	{"style-detailed", "Avalie o estilo do plano detalhado: frases ambíguas, termos inconsistentes, redundância.", "detailed"},
}

var detailHeadings = []string{"## Detailed Implementation Plan", "## Plano de Implementação Detalhado", "## Implementation Plan"}

func blocking(sev string) bool { return sev == "blocker" || sev == "major" }

type reviewerRun struct {
	Reviewer string `json:"reviewer"`
	Status   string `json:"status"` // completed | not_run
	Reason   string `json:"reason,omitempty"`
	CostUSD  float64
	Findings []finding `json:"findings"`
	Prior    []struct {
		Lineage    string `json:"lineage"`
		StillHolds bool   `json:"still_holds"`
	} `json:"prior"`
}

type budget struct {
	mu    sync.Mutex
	spent float64
	max   float64
}

func (b *budget) add(c float64) {
	b.mu.Lock()
	b.spent += c
	b.mu.Unlock()
}

func (b *budget) over() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spent >= b.max
}

func contentSHA(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:6])
}

// ranges returns the 1-based inclusive line ranges of FeatureRange and
// DetailedRange (R6).
func ranges(doc string) (feature, detailed [2]int) {
	lines := strings.Split(doc, "\n")
	cut := 0
	for i, l := range lines {
		for _, h := range detailHeadings {
			if strings.TrimSpace(l) == h {
				cut = i + 1
				break
			}
		}
		if cut > 0 {
			break
		}
	}
	if cut == 0 {
		return [2]int{1, len(lines)}, [2]int{0, 0}
	}
	return [2]int{1, cut - 1}, [2]int{cut, len(lines)}
}

func authorPrompt(path, doc string, open []*lineage2) string {
	var b strings.Builder
	for _, l := range open {
		fmt.Fprintf(&b, "- %s [%s] %s (linhas %v): %s\n", l.ID, l.Severity, l.Finding.Scope.Heading, l.Finding.Scope.Lines, l.Finding.Message)
	}
	return "Você é o autor do documento " + path + ". Revisores apontaram os problemas abaixo. Corrija cada um no texto, " +
		"com a menor mudança que resolva, sem reescrever o que não foi apontado e mantendo o idioma e a estrutura.\n" +
		"PROBLEMAS:\n" + b.String() + "\nDOCUMENTO ATUAL:\n<<<DOC>>>\n" + doc + "\n<<<END>>>\n" +
		"Responda SOMENTE com o documento completo revisado entre as linhas <<<DOC>>> e <<<END>>>, sem nada antes ou depois."
}

func extractDoc(s string) (string, bool) {
	i := strings.Index(s, "<<<DOC>>>")
	j := strings.LastIndex(s, "<<<END>>>")
	if i < 0 || j <= i {
		return "", false
	}
	return strings.Trim(s[i+len("<<<DOC>>>"):j], "\n") + "\n", true
}

type convergeEnv struct {
	bin   string
	reg   *sandbox.Registry
	sem   chan struct{}
	bud   *budget
	stop  map[string]bool
	stamp int64
	seq   int
	mu    sync.Mutex
}

func (ce *convergeEnv) nonce() string {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.seq++
	return fmt.Sprintf("n%x%d", ce.stamp, ce.seq)
}

func (ce *convergeEnv) goldfish(prompt, owner string) (*goldfishRun, error) {
	ce.sem <- struct{}{}
	defer func() { <-ce.sem }()
	if ce.bud.over() {
		return nil, errors.New("budget")
	}
	run, err := runGoldfishShared(ce.bin, prompt, owner, ce.reg)
	if err == nil && run.Envelope != nil {
		ce.bud.add(run.Envelope.TotalCostUSD)
	}
	return run, err
}

// claudeReady resolves the claude binary and refuses to start when the real
// access token expires within minLeft: a copy in the disposable HOME cannot
// refresh it (dogfood #29), so every goldfish would fail on auth.
func claudeReady(minLeft time.Duration) (string, error) {
	link, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude não encontrado: %w", err)
	}
	bin, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()
	raw, err := os.ReadFile(filepath.Join(home, ".claude", ".credentials.json"))
	if err != nil {
		return "", err
	}
	var cred struct {
		OAuth struct {
			ExpiresAt int64 `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(raw, &cred); err != nil || cred.OAuth.ExpiresAt == 0 {
		return "", errors.New("credentials.json sem claudeAiOauth.expiresAt")
	}
	exp := time.UnixMilli(cred.OAuth.ExpiresAt)
	if left := time.Until(exp); left < minLeft {
		return "", fmt.Errorf("access token vence em %s (%s): rode `claude` fora do sandbox para renovar", left.Round(time.Minute), exp.Format(time.RFC3339))
	}
	return bin, nil
}
