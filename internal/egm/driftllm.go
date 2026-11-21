package egm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Drift goldfish (docs/specs/egm-drift-bootstrap.md, B3): for each drifted
// document, one isolated goldfish asks whether the document's claims about
// its changed sources still hold. Findings are reported, never stored.

const (
	driftPersona     = "drift"
	bootstrapPersona = "bootstrap"

	// DiffAvailable and DiffNone say whether the receipt's advisory commit
	// gave the goldfish a diff.
	DiffAvailable = "diff"
	DiffNone      = "no_diff"

	diffBlockPrefix = "DIFF:"
	packetPrefix    = "tb-egmb3-"
)

// DriftLLM is a document's drift goldfish outcome.
type DriftLLM struct {
	Status  string          `json:"status"` // completed | not_run | skipped
	Reason  string          `json:"reason,omitempty"`
	Diff    string          `json:"diff"`
	Omitted []SkippedSource `json:"omitted"`
}

// openGoldfish opens the llm engine for drift-report and bootstrap. There
// is no fallback (the engine was asked for), and tool use must be
// verifiable: these goldfish have no other check on their output.
func openGoldfish(set *Settings) (*llmEnv, error) {
	if len(set.AgentCmd) == 0 || set.AgentCmd[0] == "" {
		return nil, errorf(ExitError, "agent_unavailable", "--engine llm needs egm.agent_cmd in the user config")
	}
	prof, err := ResolveProfile(set)
	if err != nil {
		return nil, err
	}
	if !prof.ToolsCheck {
		return nil, errorf(ExitError, "tools_unverifiable",
			"--engine llm needs a profile whose tool use tb can verify (vendor claude, output claude-json, tools_check auto)")
	}
	runner, err := NewRunner(set, prof)
	if err != nil {
		return nil, err
	}
	return &llmEnv{prof: prof, runner: runner, argvSHA: ArgvSHA(prof)}, nil
}

// runGoldfish runs one goldfish with one retry (R16). accept returns "" when
// the payload is valid (and keeps it), else a reason. The result is "" on
// success; an escaped or unattributed process voids a valid answer.
func (e *llmEnv) runGoldfish(ctx context.Context, owner, prompt string, bud *budgetGate, accept func([]byte) string) (string, error) {
	packet, err := os.MkdirTemp("", packetPrefix)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(packet)
	if err := writeAtomic(packet, "prompt.txt", []byte(prompt)); err != nil {
		return "", err
	}
	e.runner.ResetUnattributed()
	reason := ReasonMissingOutput
	for attempt := 1; attempt <= 2; attempt++ {
		if !bud.take() {
			reason = "budget"
			break
		}
		res := e.runner.Run(ctx, &GoldfishRequest{Owner: owner, Attempt: attempt, Packet: packet, Prompt: prompt})
		if !res.Completed {
			reason = res.Reason
			if noRetry(reason) {
				break
			}
			continue
		}
		if reason = accept(res.Payload); reason == "" {
			break
		}
	}
	if ctx.Err() != nil {
		return "", errorf(ExitError, "interrupted", "interrupted while a goldfish ran; nothing was written")
	}
	switch {
	case reason != "":
	case e.runner.Unattributed():
		reason = ReasonUnattributed
	case e.runner.Escaped(owner):
		reason = ReasonEscaped
	}
	return reason, nil
}

// driftLLM runs the drift goldfish over the report's drifted documents, up
// to egm.drift.max_docs, in report order.
func driftLLM(ctx context.Context, repo *Repo, set *Settings, oid string, rep *DriftReport) error {
	env, err := openGoldfish(set)
	if err != nil {
		return err
	}
	defer env.close()
	persona, err := LoadPersona(driftPersona)
	if err != nil {
		return err
	}
	dr := &driftRun{repo: repo, set: set, oid: oid, env: env, persona: persona,
		bud: &budgetGate{cy: &Cycle{}, max: set.MaxAgentRuns}}
	ran := 0
	for i := range rep.Documents {
		dd := &rep.Documents[i]
		if dd.Status != DriftDrifted {
			continue
		}
		if ran >= set.Drift.MaxDocs {
			dd.LLM = &DriftLLM{Status: "skipped", Reason: "max_docs", Diff: DiffNone, Omitted: []SkippedSource{}}
			continue
		}
		ran++
		if err := dr.one(ctx, dd); err != nil {
			return err
		}
	}
	return nil
}

type driftRun struct {
	repo    *Repo
	set     *Settings
	oid     string
	env     *llmEnv
	persona *Persona
	bud     *budgetGate
}

// driftPacket is one drift goldfish's data.
type driftPacket struct {
	blocks   []Block
	docs     map[string]*Doc
	findings []Finding // injection-scan over the sources and diffs
	llm      *DriftLLM
}

func (dr *driftRun) one(ctx context.Context, dd *DriftDoc) error {
	text, docSHA, ok, err := docAt(ctx, dr.repo, dr.set, dr.oid, dd.Doc)
	if err != nil {
		return err
	}
	if !ok {
		dd.LLM = &DriftLLM{Status: "not_run", Reason: "document_unreadable", Diff: DiffNone, Omitted: []SkippedSource{}}
		return nil
	}
	red, _ := Redact(text)
	pk, err := dr.packet(ctx, dd, text, red, docSHA)
	if err != nil {
		return err
	}
	dd.LLM = pk.llm
	dd.Findings = append(dd.Findings, pk.findings...)
	if len(pk.blocks) == 1 {
		pk.llm.Status, pk.llm.Reason = "not_run", "no_sources"
		return nil
	}
	nonce := newNonce()
	doc := pk.docs[dd.Doc]
	rng := [2]int{1, len(doc.Lines)}
	prompt := BuildPrompt(&PromptInput{Reviewer: driftPersona, Persona: dr.persona, Range: rng, Nonce: nonce,
		Blocks: pk.blocks, MinQuotes: dr.set.Attest.MinQuotes})
	vin := &ValidateInput{Reviewer: driftPersona, Nonce: nonce, MaxSeverity: dr.persona.MaxSeverity, Doc: doc,
		DocPath: dd.Doc, Range: rng, MinQuotes: dr.set.Attest.MinQuotes, Sources: pk.docs}
	var out *ReviewerOutput
	reason, err := dr.env.runGoldfish(ctx, "egm-drift:"+Slug(dd.Doc), prompt, dr.bud, func(p []byte) string {
		o, why := ParseReviewerOutput(p, vin)
		out = o
		return why
	})
	if err != nil {
		return err
	}
	if reason != "" {
		pk.llm.Status, pk.llm.Reason = "not_run", reason
		return nil
	}
	pk.llm.Status = "completed"
	for i := range out.Findings {
		f := out.Findings[i]
		f.Occurrence = fmt.Sprintf("drift-%d", i+1)
		dd.Findings = append(dd.Findings, f)
	}
	return nil
}

// packet builds the document block and, for each changed path, the current
// source and (with a usable commit) its diff, all from redacted text.
func (dr *driftRun) packet(ctx context.Context, dd *DriftDoc, text, red, docSHA string) (*driftPacket, error) {
	pk := &driftPacket{
		blocks: []Block{{Path: dd.Doc, Text: red}},
		docs:   map[string]*Doc{dd.Doc: ParseDoc(red, nil)},
		llm:    &DriftLLM{Diff: DiffNone, Omitted: []SkippedSource{}},
	}
	old, err := dr.usableCommit(ctx, dd.Doc, text, docSHA)
	if err != nil {
		return nil, err
	}
	if old != nil {
		pk.llm.Diff = DiffAvailable
	}
	used := int64(len(red))
	for _, p := range dd.Changed {
		blocks, why, err := dr.sourceBlocks(ctx, p, old)
		if err != nil {
			return nil, err
		}
		size := int64(0)
		for _, b := range blocks {
			size += int64(len(b.Text))
		}
		if why == "" && used+size > dr.set.MaxPacketBytes {
			why = "max_packet_bytes"
		}
		if why != "" {
			pk.llm.Omitted = append(pk.llm.Omitted, SkippedSource{Path: p, Reason: why})
			continue
		}
		used += size
		for _, b := range blocks {
			if f := ScanInjectionSource(b.Path, b.Text); f != nil {
				pk.findings = append(pk.findings, *f)
			}
		}
		pk.blocks = append(pk.blocks, blocks...)
		pk.docs[p] = ParseDoc(blocks[0].Text, nil)
	}
	return pk, nil
}

// sourceBlocks reads p at --at (and at the commit when old is set) and
// returns its blocks, or why it is left out.
func (dr *driftRun) sourceBlocks(ctx context.Context, p string, old map[string][]byte) ([]Block, string, error) {
	cur, err := dr.repo.CatFileBlob(ctx, dr.oid, p, dr.set.MaxSourceBytes)
	var e *Error
	switch {
	case errors.As(err, &e) && e.Reason == "blob_too_large":
		return nil, "max_source_bytes", nil
	case err != nil:
		return nil, "", err
	}
	redCur, secret := Redact(string(cur))
	if secret {
		return nil, "secret_like", nil
	}
	blocks := []Block{{Path: p, Text: redCur}}
	if old == nil {
		return blocks, "", nil
	}
	redOld, secret := Redact(string(old[p]))
	if secret {
		return nil, "secret_like", nil
	}
	diff, err := unifiedDiff(ctx, redOld, redCur)
	if err != nil {
		return nil, "", err
	}
	if diff != "" {
		blocks = append(blocks, Block{Path: diffBlockPrefix + p, Text: diff})
	}
	return blocks, "", nil
}

// usableCommit returns the receipt sources' blobs at the receipt's commit
// when that commit exists and matches every hash the receipt records (the
// document's included); nil means no_diff. The commit is advisory: any
// mismatch or unreadable object only drops the diff.
func (dr *driftRun) usableCommit(ctx context.Context, rel, text, docSHA string) (map[string][]byte, error) {
	rc, _, err := readReceiptAt(ctx, dr.repo, dr.oid, rel, docLines(text))
	if err != nil {
		return nil, err
	}
	if rc == nil || rc.Commit == "" || rc.DocSHA256 != docSHA {
		return nil, nil
	}
	commit, err := dr.repo.RevParse(ctx, rc.Commit+"^{commit}")
	if err != nil {
		return nil, nil
	}
	b, _, err := readBlobFile(ctx, dr.repo, commit, rel, dr.set.MaxPacketBytes)
	if err != nil || b == nil || sha256Hex(b) != rc.DocSHA256 {
		return nil, nil
	}
	blobs := map[string][]byte{}
	for p, want := range rc.Sources {
		b, _, err := readBlobFile(ctx, dr.repo, commit, p, maxDriftBlobBytes)
		if err != nil || b == nil || sha256Hex(b) != want {
			return nil, nil
		}
		blobs[p] = b
	}
	return blobs, nil
}

// unifiedDiff diffs two redacted texts in a private temp dir with tb's
// hardened git and drops the header lines, which name the temp files.
func unifiedDiff(ctx context.Context, oldText, newText string) (string, error) {
	if oldText == newText {
		return "", nil
	}
	dir, err := os.MkdirTemp("", packetPrefix+"diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	a, b := filepath.Join(dir, "old"), filepath.Join(dir, "new")
	if err := os.WriteFile(a, []byte(oldText), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(b, []byte(newText), 0o600); err != nil {
		return "", err
	}
	out, err := DiffNoIndexContext(ctx, dir, a, b)
	if err != nil {
		return "", err
	}
	s := string(out)
	if i := strings.Index(s, "\n@@"); i >= 0 {
		return s[i+1:], nil
	}
	return "", nil
}
