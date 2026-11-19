//go:build egmfake

package bench

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/egm"
)

// FakeAgentEgmSubcommand is the hidden subcommand of the egm fake agent,
// compiled only with the egmfake build tag (docs/specs/egm-nativo.md, note on
// the fake-agent). It reads the R9 prompt on stdin and answers with a
// claude-json envelope whose content the mode controls.
const FakeAgentEgmSubcommand = "__fake-agent-egm"

var (
	reFakeReviewer = regexp.MustCompile(`(?m)^REVIEWER: (\S+)$`)
	reFakeNonce    = regexp.MustCompile(`(?m)^NONCE: (\S+)$`)
	reFakeRange    = regexp.MustCompile(`(?m)^RANGE: (\d+)-(\d+)$`)
	reFakeLineage  = regexp.MustCompile(`"lineage":"([^"]+)"`)
	reFakeLines    = regexp.MustCompile(`"lines":\[(\d+),(\d+)\]`)
	reFakeNumbered = regexp.MustCompile(`^(\d+)\| ?`)
)

type fakeBlock struct{ path, text string }

type fakePrompt struct {
	reviewer, nonce string
	rng             [2]int
	prior           []string
	blocks          []fakeBlock
}

func parseFakePrompt(p string) (*fakePrompt, error) {
	fp := &fakePrompt{}
	m := reFakeReviewer.FindStringSubmatch(p)
	n := reFakeNonce.FindStringSubmatch(p)
	if m == nil || n == nil {
		return nil, errors.New("fake-agent-egm: prompt without REVIEWER/NONCE")
	}
	fp.reviewer, fp.nonce = m[1], n[1]
	if r := reFakeRange.FindStringSubmatch(p); r != nil {
		a, _ := strconv.Atoi(r[1])
		b, _ := strconv.Atoi(r[2])
		fp.rng = [2]int{a, b}
	}
	re := regexp.MustCompile(`(?s)<<<DATA-` + regexp.QuoteMeta(fp.nonce) + ` path=([^\n]*)>>>\n(.*?)<<<END-` + regexp.QuoteMeta(fp.nonce) + `>>>`)
	for _, b := range re.FindAllStringSubmatch(p, -1) {
		fp.blocks = append(fp.blocks, fakeBlock{path: b[1], text: b[2]})
		if b[1] == "PRIOR" {
			for _, l := range reFakeLineage.FindAllStringSubmatch(b[2], -1) {
				fp.prior = append(fp.prior, l[1])
			}
		}
	}
	return fp, nil
}

// document returns the first data block that is the reviewed document,
// without its line-number prefixes.
func (fp *fakePrompt) document() (path, text string) {
	for _, b := range fp.blocks {
		switch b.path {
		case "PRIOR", "FINDING", "TRECHO-SINALIZADO":
			continue
		}
		var out []string
		for _, l := range strings.Split(strings.TrimSuffix(b.text, "\n"), "\n") {
			out = append(out, reFakeNumbered.ReplaceAllString(l, ""))
		}
		return b.path, strings.Join(out, "\n") + "\n"
	}
	return "", ""
}

// checked builds a valid attestation: every heading of the range and up to
// five literal body lines (R17).
func checked(d *egm.Doc, rng [2]int) (qs []egm.Quote, firstBody int) {
	for _, h := range d.Headings(rng) {
		qs = append(qs, egm.Quote{Heading: h, Quote: h})
	}
	body := 0
	for n := max(1, rng[0]); n <= rng[1] && n <= len(d.Lines) && body < 5; n++ {
		t := strings.TrimSpace(d.Lines[n-1])
		norm := strings.Join(strings.Fields(strings.ReplaceAll(t, "|", " ")), " ")
		if d.InFence(n) || strings.HasPrefix(t, "#") || len(norm) < 20 || len(norm) > 200 {
			continue
		}
		h := d.HeadingAt(n)
		if d.HeadingLine(h) < rng[0] {
			h = ""
		}
		qs = append(qs, egm.Quote{Heading: h, Quote: t})
		if firstBody == 0 {
			firstBody = n
		}
		body++
	}
	return qs, firstBody
}

func fakeMode(cfgPath, reviewer string) (string, error) {
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	var cfg map[string][]string
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", err
	}
	modes, ok := cfg[reviewer]
	if !ok {
		modes = cfg["*"]
	}
	attempt, _ := strconv.Atoi(os.Getenv("TB_EGM_ATTEMPT"))
	if len(modes) == 0 {
		return "clean", nil
	}
	if attempt < 1 || attempt > len(modes) {
		attempt = len(modes)
	}
	return modes[attempt-1], nil
}

// RunFakeAgentEgm is the __fake-agent-egm subcommand: args[0] is the mode
// config file, the rest (profile flags) is ignored.
func RunFakeAgentEgm(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("fake-agent-egm: missing config path")
	}
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	fp, err := parseFakePrompt(string(raw))
	if err != nil {
		return err
	}
	mode, err := fakeMode(args[0], fp.reviewer)
	if err != nil {
		return err
	}
	if done, err := sideEffectMode(mode, stdout); done {
		return err
	}
	payload, turns := fakePayload(fp, mode)
	env := map[string]any{"type": "result", "subtype": "success", "is_error": false, "num_turns": turns, "result": payload}
	return json.NewEncoder(stdout).Encode(env)
}

func sideEffectMode(mode string, stdout io.Writer) (bool, error) {
	switch mode {
	case "sleep":
		time.Sleep(time.Hour)
		return true, nil
	case "overflow":
		_, err := io.WriteString(stdout, strings.Repeat("x", 4<<20))
		return true, err
	case "stderr-json":
		fmt.Fprintln(os.Stderr, `{"type":"result","result":"{}"}`)
	case "escape", "escape-anon":
		cmd := exec.Command("setsid", "sleep", "30")
		if mode == "escape-anon" {
			cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
		}
		if err := cmd.Start(); err != nil {
			return true, err
		}
	}
	return false, nil
}

func fakePayload(fp *fakePrompt, mode string) (payload string, turns int) {
	docPath, text := fp.document()
	d := egm.ParseDoc(text, nil)
	if fp.reviewer == "arbiter" {
		return arbiterPayload(fp, d, mode), 1
	}
	qs, line := checked(d, fp.rng)
	out := map[string]any{"reviewer": fp.reviewer, "nonce": fp.nonce, "checked": qs, "findings": []any{}, "prior": priorAnswers(fp.prior, mode != "clean")}
	turns = 1
	switch mode {
	case "invalid":
		return "this is not json", 1
	case "no-nonce":
		out["nonce"] = "wrong"
	case "omit-prior":
		out["prior"] = []any{}
	case "bad-attest":
		out["checked"] = []any{}
	case "tool-use":
		turns = 2
	case "findings", "over-severity":
		sev := "major"
		if mode == "over-severity" {
			sev = "blocker"
		}
		out["findings"] = []any{fakeFinding(d, docPath, fp, line, sev)}
	}
	b, _ := json.Marshal(out)
	return string(b), turns
}

func fakeFinding(d *egm.Doc, docPath string, fp *fakePrompt, line int, sev string) map[string]any {
	if line == 0 {
		line = max(1, fp.rng[0])
	}
	return map[string]any{
		"severity":  sev,
		"scope":     map[string]any{"heading": d.HeadingAt(line), "lines": []int{line, line}},
		"message":   "the " + fp.reviewer + " reviewer finds this statement unverifiable as written",
		"rationale": "fake agent",
		"evidence":  []map[string]any{{"path": docPath, "line": line}},
	}
}

func priorAnswers(ids []string, holds bool) []map[string]any {
	out := []map[string]any{}
	for _, id := range ids {
		out = append(out, map[string]any{"lineage": id, "still_holds": holds})
	}
	return out
}

func arbiterPayload(fp *fakePrompt, d *egm.Doc, mode string) string {
	rng := [2]int{1, len(d.Lines)}
	for _, b := range fp.blocks {
		if b.path == "FINDING" {
			if m := reFakeLines.FindStringSubmatch(b.text); m != nil {
				a, _ := strconv.Atoi(m[1])
				z, _ := strconv.Atoi(m[2])
				rng = [2]int{max(1, a-20), min(len(d.Lines), z+20)}
			}
		}
	}
	qs, _ := checked(d, rng)
	verdict := "withdraw"
	if mode == "uphold" {
		verdict = "uphold"
	}
	b, _ := json.Marshal(map[string]any{"nonce": fp.nonce, "checked": qs, "verdict": verdict, "rationale": "fake arbiter: " + verdict})
	return string(b)
}
