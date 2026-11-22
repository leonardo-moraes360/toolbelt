package egm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Shared pieces of tb egm brainstorm and tb egm draft (docs/specs/egm-phase-c.md).
// Both feed untrusted text to one isolated goldfish and treat what comes back
// as data: cleaned, scanned and checked before it reaches the disk. Neither
// writes review state, so nothing they produce can count toward a verdict.

const (
	brainstormPersona = "brainstorm"
	draftPersona      = "draft"

	reasonAuthoringInvalid = "authoring_invalid"

	// Block paths of the data blocks that are not code files. Neither has a
	// known extension, so R7 never reads them as citations.
	blockBrief    = "BRIEF"
	blockDocument = "DOCUMENT"

	// contextDoc is the pseudo document that makes the bootstrap file rules
	// apply to --from files when there is no document to write (brainstorm).
	contextDoc = ".tb-egm-context"
)

var reSlugRun = regexp.MustCompile(`[^a-z0-9]+`)

// authEnv is the repository and settings of one authoring command.
type authEnv struct {
	repo *Repo
	set  Settings
	cwd  string // symlink-free
}

func openAuthoring(cwd, configFile string) (*authEnv, error) {
	repo, err := DiscoverRepo(filepath.Join(cwd, ".egm-author"))
	if err != nil {
		return nil, err
	}
	user, err := LoadUserConfig(configFile)
	if err != nil {
		return nil, errorf(ExitError, "config", "%v", err)
	}
	raw, w := readRepoConfig(repo.Root)
	set := ApplyRepoConfig(&user, raw)
	if w != nil {
		set.Warnings = append(set.Warnings, *w)
	}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return nil, errorf(ExitError, "usage", "%s: %v", cwd, err)
	}
	return &authEnv{repo: repo, set: set, cwd: resolved}, nil
}

// briefsDir is where concept briefs land by default: outside the repository.
func (a *authEnv) briefsDir() (string, error) {
	root, err := StateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "egm", a.repo.ID, "briefs"), nil
}

// briefData is a vetted brief.
type briefData struct {
	Display string // path shown in messages
	Text    string
	SHA256  string
}

// readBrief reads and vets --brief (R1, R8): a regular file reached through
// no symlink inside the repository (or, when allowState, inside this
// repository's briefs directory), small enough, with no secret-like value
// and no text that reads as an instruction.
func (a *authEnv) readBrief(arg string, allowState bool) (*briefData, error) {
	abs := filepath.Clean(arg)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(a.cwd, abs)
	}
	root, rel, display, err := a.briefLocation(abs, arg, allowState)
	if err != nil {
		return nil, err
	}
	if inDenied(rel) && root == a.repo.Root {
		return nil, errorf(ExitError, "brief_path", "%s is under .git or a path R7 denies", display)
	}
	fi, reason, missing := lstatChain(root, rel)
	switch {
	case missing:
		return nil, errorf(ExitError, "usage", "--brief %s does not exist", display)
	case reason != "":
		return nil, errorf(ExitError, "brief_path", "%s: a path component is a %s", display, reason)
	case !fi.Mode().IsRegular():
		return nil, errorf(ExitError, "brief_path", "%s is not a regular file", display)
	}
	b, err := readRegular(filepath.Join(root, filepath.FromSlash(rel)), a.set.MaxSourceBytes)
	if errors.Is(err, errTooBig) {
		return nil, errorf(ExitError, "brief_path", "%s is larger than egm.max_source_bytes", display)
	}
	if err != nil {
		return nil, errorf(ExitError, "brief_path", "%s: %v", display, err)
	}
	if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
		return nil, errorf(ExitError, "brief_invalid", "%s is not UTF-8 text", display)
	}
	red, secret := Redact(string(b))
	if secret {
		return nil, errorf(ExitError, "brief_secret", "%s has a secret-like value; remove it before using the brief", display)
	}
	if f := ScanInjectionSource(display, red); f != nil {
		return nil, errorf(ExitError, "brief_injection", "%s", f.Message)
	}
	return &briefData{Display: display, Text: red, SHA256: sha256Hex(b)}, nil
}

// briefLocation resolves --brief to a root and a slash path under it.
func (a *authEnv) briefLocation(abs, arg string, allowState bool) (root, rel, display string, err error) {
	if allowState {
		dir, derr := a.briefsDir()
		if derr != nil {
			return "", "", "", derr
		}
		for _, base := range []string{dir, realOrSame(dir)} {
			if r, ok := under(base, abs); ok {
				return base, r, abs, nil
			}
		}
	}
	r, rerr := gateRel(a.repo.Root, a.cwd, arg)
	if rerr != nil {
		return "", "", "", errorf(ExitError, "brief_path", "--brief %s must be a file inside the repository", arg)
	}
	return a.repo.Root, r, r, nil
}

func realOrSame(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// under returns abs relative to base, as a slash path, when it is below it.
func under(base, abs string) (string, bool) {
	r, err := filepath.Rel(base, abs)
	if err != nil || r == "." || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(r), true
}

// briefTitle is the brief's first Markdown heading, else its file name.
func briefTitle(text, display string) string {
	for _, l := range strings.Split(text, "\n") {
		if reATX.MatchString(l) {
			if t := oneLine(strings.TrimLeft(strings.TrimSpace(l), "# ")); t != "" {
				return t
			}
		}
	}
	return titleFrom(display)
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120])
	}
	return s
}

// slugOf is a short file-name slug of the brief's name.
func slugOf(display string) string {
	base := strings.TrimSuffix(filepath.Base(display), filepath.Ext(display))
	s := strings.Trim(reSlugRun.ReplaceAllString(strings.ToLower(base), "-"), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" {
		return "brief"
	}
	return s
}

// writeNumbered creates dir/<prefix><n>.md with n one above the highest
// existing one, mode 0600, never overwriting.
func writeNumbered(dir, prefix, text string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	n := 0
	for _, e := range entries {
		mid, ok := strings.CutPrefix(e.Name(), prefix)
		if !ok {
			continue
		}
		if v, err := strconv.Atoi(strings.TrimSuffix(mid, ".md")); err == nil && strings.HasSuffix(mid, ".md") {
			n = max(n, v)
		}
	}
	for range 100 {
		n++
		p := filepath.Join(dir, fmt.Sprintf("%s%d.md", prefix, n))
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL|oNoFollow, 0o600) //nolint:gosec // inside tb's own state directory
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.WriteString(text)
		if cerr := f.Close(); werr != nil || cerr != nil {
			_ = os.Remove(p)
			return "", errors.Join(werr, cerr)
		}
		return p, nil
	}
	return "", errorf(ExitError, "usage", "no free file name in %s", dir)
}

// contextSources applies the bootstrap rules to --from for a command that
// writes no cited document: the accepted files (read, redacted) and the
// skipped ones. A secret-like file is skipped and listed, as in R7.
func (a *authEnv) contextSources(ctx context.Context, from []string, code string) (*SourceSet, []SkippedSource, error) {
	files, skipped, err := bootstrapFiles(ctx, a.repo, a.cwd, contextDoc, from, &a.set)
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, errorf(ExitError, code+"_sources", "no --from file can be used: %s", skippedList(skipped))
	}
	got, err := CollectSources(ctx, a.repo, &a.set, contextDoc, sourcesSection(contextDoc, files))
	if err != nil {
		return nil, nil, err
	}
	return got, append(skipped, got.Skipped...), nil
}

// scanSources refuses a source that reads as an instruction to the model.
func scanSources(got *SourceSet, code string) error {
	for i := range got.Included {
		if f := ScanInjectionSource(got.Included[i].Path, got.Included[i].Text); f != nil {
			return errorf(ExitError, code+"_injection", "%s", f.Message)
		}
	}
	return nil
}

// generate runs one goldfish with one retry and returns the cleaned body.
// valid, when set, is an extra shape check that also triggers the retry.
func (a *authEnv) generate(ctx context.Context, env *llmEnv, kind, owner, prompt string, maxBytes int64, valid func(string) bool) (string, error) {
	var body string
	bud := &budgetGate{cy: &Cycle{}, max: a.set.MaxAgentRuns}
	reason, err := env.runGoldfish(ctx, owner, prompt, bud, func(p []byte) string {
		b, ok := cleanPayload(p, maxBytes)
		if !ok || (valid != nil && !valid(b)) {
			return reasonAuthoringInvalid
		}
		body = b
		return ""
	})
	switch {
	case err != nil:
		return "", err
	case reason == reasonAuthoringInvalid:
		return "", errorf(ExitError, kind+"_invalid", "the %s goldfish returned an empty, oversized, non-UTF-8 or incomplete body twice; nothing was written", kind)
	case reason != "":
		return "", errorf(ExitError, kind+"_not_run", "the %s goldfish did not complete (%s); nothing was written", kind, reason)
	}
	if fs := ScanInjection(ParseDoc(body, nil), contextDoc); len(fs) > 0 {
		return "", errorf(ExitError, kind+"_injection", "the generated body %s; nothing was written", strings.TrimPrefix(fs[0].Message, "text "))
	}
	return body, nil
}

// h2Titles are the level-2 heading lines of body outside code fences.
func h2Titles(body string) []string {
	var out []string
	inFence := false
	for _, l := range strings.Split(body, "\n") {
		switch {
		case reFenceLine.MatchString(l):
			inFence = !inFence
		case !inFence && reATX.MatchString(l) && headingLevel(l) == 2:
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

// hasInOrder reports whether want is a subsequence of have.
func hasInOrder(have, want []string) bool {
	i := 0
	for _, h := range have {
		if i < len(want) && h == want[i] {
			i++
		}
	}
	return i == len(want)
}

// authoringPrompt renders persona, standard, REVIEWER, NONCE, the output
// contract, then one numbered data block per input.
func authoringPrompt(persona *Persona, standard, nonce, contract string, blocks []Block) string {
	var b strings.Builder
	writeLens(&b, persona, standard)
	fmt.Fprintf(&b, "REVIEWER: %s\nNONCE: %s\n\n", persona.Name, nonce)
	b.WriteString("OUTPUT CONTRACT\n")
	b.WriteString(dataRule(nonce))
	b.WriteString(contract)
	b.WriteString("\n")
	for _, blk := range blocks {
		writeBlock(&b, nonce, blk.Path, blk.Text, 1)
	}
	return b.String()
}

func sectionList(headings []string) string {
	return strings.Join(headings, "\n") + "\n"
}
