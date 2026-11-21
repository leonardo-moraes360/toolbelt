package egm

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Bootstrap goldfish (docs/specs/egm-drift-bootstrap.md, B3): one isolated
// goldfish writes the body of a new draft from the accepted --from files.
// The body is untrusted data: tb strips what only tb may write, scans it and
// checks its sources before anything reaches the disk.

const reasonBootstrapInvalid = "bootstrap_invalid"

var (
	reHTMLComment  = regexp.MustCompile(`(?s)<!--.*?(?:-->|$)`)
	reSetextH1     = regexp.MustCompile(`^ {0,3}=+\s*$`)
	reFenceLine    = regexp.MustCompile("^ {0,3}(```|~~~)")
	reSourcesTitle = regexp.MustCompile(`(?i)^#{2}\s+sources\s*#*\s*$`)
)

// bootstrapDraft runs the goldfish over the R7 sources of the skeleton and
// returns the assembled draft and its warnings. Nothing is written here.
func bootstrapDraft(ctx context.Context, repo *Repo, set *Settings, rel, docType string, got *SourceSet) (string, []Warning, error) {
	for i := range got.Included {
		if f := ScanInjectionSource(got.Included[i].Path, got.Included[i].Text); f != nil {
			return "", nil, errorf(ExitError, "bootstrap_injection", "%s", f.Message)
		}
	}
	persona, err := LoadPersona(bootstrapPersona)
	if err != nil {
		return "", nil, err
	}
	standardName := structuralStandard(docType)
	standard, err := LoadStandard(standardName)
	if err != nil {
		return "", nil, err
	}
	env, err := openGoldfish(set)
	if err != nil {
		return "", nil, err
	}
	defer env.close()
	files := make([]string, 0, len(got.Included))
	blocks := make([]Block, 0, len(got.Included))
	for i := range got.Included {
		files = append(files, got.Included[i].Path)
		blocks = append(blocks, Block{Path: got.Included[i].Path, Text: got.Included[i].Text})
	}
	nonce := newNonce()
	prompt := buildBootstrapPrompt(persona, standard, nonce, requiredHeadings[standardName], set.Bootstrap.MaxBytes, blocks)
	var body string
	bud := &budgetGate{cy: &Cycle{}, max: set.MaxAgentRuns}
	reason, err := env.runGoldfish(ctx, "egm-bootstrap:"+Slug(rel), prompt, bud, func(p []byte) string {
		b, why := bootstrapBody(p, set.Bootstrap.MaxBytes)
		body = b
		return why
	})
	switch {
	case err != nil:
		return "", nil, err
	case reason == reasonBootstrapInvalid:
		return "", nil, errorf(ExitError, reasonBootstrapInvalid, "the bootstrap goldfish returned an empty, oversized or non-UTF-8 body twice; nothing was written")
	case reason != "":
		return "", nil, errorf(ExitError, "bootstrap_not_run", "the bootstrap goldfish did not complete (%s); nothing was written", reason)
	}
	if fs := ScanInjection(ParseDoc(body, nil), rel); len(fs) > 0 {
		return "", nil, errorf(ExitError, "bootstrap_injection", "the generated body %s; nothing was written", strings.TrimPrefix(fs[0].Message, "text "))
	}
	text := draftHead(rel) + body + "\n\n" + sourcesSection(rel, files)
	final, err := checkBootstrapSources(ctx, repo, set, rel, text, files)
	if err != nil {
		return "", nil, err
	}
	return text, final.Warnings, nil
}

// bootstrapBody validates and cleans one payload: UTF-8, at most maxBytes,
// and not empty once cleaned.
func bootstrapBody(payload []byte, maxBytes int64) (string, string) {
	p := stripMarkdownFence(payload)
	if int64(len(p)) > maxBytes || !utf8.Valid(p) || bytes.ContainsRune(p, 0) {
		return "", reasonBootstrapInvalid
	}
	body := cleanBootstrapBody(string(p))
	if body == "" {
		return "", reasonBootstrapInvalid
	}
	return body, ""
}

// stripMarkdownFence removes one fence around the whole payload (```,
// ```markdown or ```md), which models add often.
func stripMarkdownFence(payload []byte) []byte {
	t := bytes.TrimSpace(payload)
	if !bytes.HasPrefix(t, []byte("```")) {
		return t
	}
	first, rest, ok := bytes.Cut(t, []byte("\n"))
	if !ok {
		return t
	}
	switch strings.ToLower(strings.TrimSpace(string(first[3:]))) {
	case "", "markdown", "md":
	default:
		return t
	}
	inner, ok := bytes.CutSuffix(bytes.TrimSpace(rest), []byte("```"))
	if !ok {
		return t
	}
	return bytes.TrimSpace(inner)
}

// cleanBootstrapBody removes what only tb writes: every HTML comment (tb
// markers included), every level-1 heading, every "Status:" line before the
// first "##" and any "## Sources" section.
func cleanBootstrapBody(body string) string {
	body = reHTMLComment.ReplaceAllString(strings.ReplaceAll(body, "\r\n", "\n"), "")
	var out []string
	inFence, inSources, seenH2 := false, false, false
	for _, l := range strings.Split(body, "\n") {
		if reFenceLine.MatchString(l) {
			inFence = !inFence
		} else if !inFence {
			level := 0
			if reATX.MatchString(l) {
				level = headingLevel(l)
			}
			switch {
			case level == 1:
				continue
			case level == 2:
				seenH2 = true
				inSources = reSourcesTitle.MatchString(strings.TrimSpace(l))
			case !seenH2 && strings.HasPrefix(strings.TrimSpace(l), "Status:"):
				continue
			case reSetextH1.MatchString(l) && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "":
				out = out[:len(out)-1] // a setext level-1 heading: drop its text too
				continue
			}
		}
		if inSources {
			continue
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// buildBootstrapPrompt renders the bootstrap prompt: persona, standard,
// REVIEWER, NONCE, output contract, then one data block per source.
func buildBootstrapPrompt(p *Persona, standard, nonce string, headings [][2]string, maxBytes int64, blocks []Block) string {
	var b strings.Builder
	writeLens(&b, p, standard)
	fmt.Fprintf(&b, "REVIEWER: %s\nNONCE: %s\n\n", bootstrapPersona, nonce)
	b.WriteString("OUTPUT CONTRACT\n")
	b.WriteString(dataRule(nonce))
	fmt.Fprintf(&b, "Reply with the Markdown body of the document and nothing else, at most %d bytes: no JSON, no fence around it, "+
		"no level-1 heading, no \"Status:\" line, no \"## Sources\" section and no HTML comments (tb writes the title, the status and the sources).\n", maxBytes)
	b.WriteString("Use these sections, in this order, each as its heading line exactly as written:\n")
	for _, h := range headings {
		b.WriteString(h[0] + "\n")
	}
	b.WriteString("Cite a file only as its data block path inside single backticks; cite no other file, path or URL.\n\n")
	for _, blk := range blocks {
		writeBlock(&b, nonce, blk.Path, blk.Text, 1)
	}
	return b.String()
}
