package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestDocsCLIExamplesMatchCobra(t *testing.T) {
	root := findModuleRoot(t)
	for _, rel := range []string{
		filepath.Join("docs", "cli.md"),
		filepath.Join("docs", "en", "cli.md"),
	} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			docPath := filepath.Join(root, rel)
			raw, err := os.ReadFile(docPath)
			if err != nil {
				t.Fatalf("reading %s: %v", docPath, err)
			}

			examples := extractDocsCLIExamples(string(raw))
			if len(examples) == 0 {
				t.Fatalf("no tb examples extracted from %s", rel)
			}

			var checked int
			for _, ex := range examples {
				ok, err := validateDocsCLIExample(rootCmd, ex)
				if !ok {
					continue
				}
				checked++
				if err != nil {
					t.Errorf("%s:%d: %s\n  %v", rel, ex.line, ex.raw, err)
				}
			}
			if checked == 0 {
				t.Fatalf("no implemented tb examples to validate in %s", rel)
			}
		})
	}
}

func TestDocsEnglishMirrorComplete(t *testing.T) {
	root := findModuleRoot(t)
	docsRoot := filepath.Join(root, "docs")
	enRoot := filepath.Join(docsRoot, "en")
	var missing []string
	err := filepath.Walk(docsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path == enRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(docsRoot, path)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(enRoot, rel)); err != nil {
			missing = append(missing, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Fatalf("docs without English mirror under docs/en/: %v", missing)
	}
}

func TestDocsDescribeRealLocalWorkflow(t *testing.T) {
	root := findModuleRoot(t)
	cli := mustReadFile(t, filepath.Join(root, "docs", "cli.md"))
	cliEN := mustReadFile(t, filepath.Join(root, "docs", "en", "cli.md"))
	readme := mustReadFile(t, filepath.Join(root, "README.md"))
	readmeEN := mustReadFile(t, filepath.Join(root, "README.en.md"))
	changelog := mustReadFile(t, filepath.Join(root, "CHANGELOG.md"))

	assertDocsDriftIsScopeNotVault(t, cli)
	assertDocsDriftIsScopeNotVault(t, cliEN)
	assertDocsLocalTicketWorkflow(t, cli+"\n"+readme)
	assertDocsLocalTicketWorkflow(t, cliEN+"\n"+readmeEN)
	assertDocsJavascriptCoversTypeScript(t, cli)
	assertDocsJavascriptCoversTypeScript(t, cliEN)
	assertDocsGuardStrictDeprecated(t, cli, changelog)
	assertDocsGuardStrictDeprecated(t, cliEN, changelog)
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertDocsDriftIsScopeNotVault(t *testing.T, cli string) {
	t.Helper()
	if strings.Contains(cli, "Analisando vault contra código") {
		t.Error("docs/cli.md ainda descreve drift como 'vault vs código'")
	}
	if strings.Contains(cli, "DOCUMENTAÇÃO DESATUALIZADA") {
		t.Error("docs/cli.md ainda usa o relatório fictício de documentação desatualizada no drift")
	}
	if !strings.Contains(cli, "Fora do scope") || !strings.Contains(cli, "Proibidos tocados") {
		t.Error("docs/cli.md deve mostrar o relatório real de drift (fora do scope / proibidos tocados)")
	}
}

func assertDocsLocalTicketWorkflow(t *testing.T, docs string) {
	t.Helper()
	localOK := strings.Contains(strings.ToLower(docs), "100% local") ||
		strings.Contains(docs, "não fala com o GitHub") ||
		strings.Contains(docs, "nao fala com o GitHub") ||
		strings.Contains(strings.ToLower(docs), "does not talk to github") ||
		strings.Contains(strings.ToLower(docs), "doesn't talk to github")
	if !localOK {
		t.Error("docs devem deixar claro que o tb não fala com o GitHub (100% local)")
	}
	fromIssue := strings.Contains(docs, "gh issue") || strings.Contains(docs, "gh pr")
	if !strings.Contains(docs, "rd.md") || !fromIssue {
		t.Error("docs devem explicar como obter/gerar rd.md a partir de issue ou PR")
	}
}

func assertDocsJavascriptCoversTypeScript(t *testing.T, cli string) {
	t.Helper()
	covers := strings.Contains(cli, "cobre TypeScript") ||
		strings.Contains(cli, "inclui TypeScript") ||
		strings.Contains(cli, "também TypeScript") ||
		strings.Contains(cli, "covers TypeScript") ||
		strings.Contains(cli, "includes TypeScript") ||
		strings.Contains(cli, "TypeScript (ts/tsx)")
	if !strings.Contains(cli, "TypeScript") || !covers {
		t.Error("docs/cli.md deve deixar explícito que a linguagem javascript cobre TypeScript (ts/tsx)")
	}
}

func assertDocsGuardStrictDeprecated(t *testing.T, cli, changelog string) {
	t.Helper()
	if reGuardStrictExample.MatchString(cli) {
		t.Error("docs/cli.md não deve ter exemplo invocável com guard ... --strict")
	}
	if !strings.Contains(cli, "--strict") || !strings.Contains(strings.ToLower(cli), "deprecated") {
		t.Error("docs/cli.md deve anotar a depreciação de guard --strict")
	}
	naoLancado := changelogSection(changelog, "## [Não Lançado]")
	if naoLancado == "" {
		t.Fatal("CHANGELOG sem seção ## [Não Lançado]")
	}
	deprec := strings.Contains(strings.ToLower(naoLancado), "deprecat") ||
		strings.Contains(strings.ToLower(naoLancado), "deprecia")
	if !strings.Contains(naoLancado, "--strict") || !deprec {
		t.Error("CHANGELOG [Não Lançado] deve anotar a depreciação de guard --strict")
	}
}

var reGuardStrictExample = regexp.MustCompile(`(?m)^[[:space:]]*tb[[:space:]]+guard[[:space:]]+check\b[^#\n]*--strict\b`)

func changelogSection(doc, heading string) string {
	start := strings.Index(doc, heading)
	if start < 0 {
		return ""
	}
	rest := doc[start+len(heading):]
	if next := strings.Index(rest, "\n## ["); next >= 0 {
		rest = rest[:next]
	}
	return rest
}

func TestNoMemoriaPersistentTypo(t *testing.T) {
	root := findModuleRoot(t)
	bad := "memória" + " persistent"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if hasMemoriaPersistentTypo(string(data), bad) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s contains typo %q (want %q)", rel, bad, bad+"e")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type docsCLIExample struct {
	line       int
	raw        string
	tokens     []string
	extraFlags []string
}

var (
	reOptionalFlag = regexp.MustCompile(`\[(--[A-Za-z0-9-]+)[^]]*\]`)
	reMetaAngle    = regexp.MustCompile(`<[^>]+>`)
	reMetaBracket  = regexp.MustCompile(`\[[^\]]+\]`)
	reFlagLine     = regexp.MustCompile(`^\s*(--[A-Za-z0-9-]+|[A-Za-z0-9-]{1,2},\s*--[A-Za-z0-9-]+)`)
	reShortLong    = regexp.MustCompile(`(?:^|\s)(-[A-Za-z])?,\s*(--[A-Za-z0-9-]+)`)
)

func extractDocsCLIExamples(doc string) []docsCLIExample {
	lines := strings.Split(doc, "\n")
	var out []docsCLIExample
	inBash := false
	var fence []string
	var fenceStart int

	flush := func() {
		if len(fence) == 0 {
			return
		}
		out = append(out, parseDocsBashFence(fence, fenceStart)...)
		fence = nil
	}

	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "```") {
			if inBash {
				fence = append(fence, line)
			}
			continue
		}
		lang := strings.TrimSpace(strings.TrimPrefix(trim, "```"))
		if !inBash {
			if lang == "bash" || lang == "sh" || lang == "shell" {
				inBash = true
				fenceStart = i + 1
				fence = nil
			}
			continue
		}
		flush()
		inBash = false
	}
	if inBash {
		flush()
	}
	return out
}

type numberedLine struct {
	line int
	text string
}

func parseDocsBashFence(fence []string, startLine int) []docsCLIExample {
	joined := joinContinuedLines(fence, startLine)
	var examples []docsCLIExample
	var current *docsCLIExample
	inFlags := false

	for _, item := range joined {
		trim := strings.TrimSpace(item.text)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		cmdPart := stripPipeAndComment(trim)
		lower := strings.ToLower(trim)
		switch {
		case lower == "flags:" || strings.HasPrefix(lower, "flags:"):
			inFlags = true
		case strings.HasPrefix(lower, "subcomandos:") || strings.HasPrefix(lower, "subcommands:"):
			inFlags = false
		case strings.HasPrefix(cmdPart, "tb ") || cmdPart == "tb":
			inFlags = false
			ex := docsCLIExample{line: item.line, raw: cmdPart}
			ex.tokens, ex.extraFlags = tokenizeDocsCLI(cmdPart)
			examples = append(examples, ex)
			current = &examples[len(examples)-1]
		case inFlags && current != nil:
			appendFlagLine(current, trim)
		}
	}
	return examples
}

func stripPipeAndComment(s string) string {
	if i := strings.Index(s, " | "); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if hash := indexUnquoted(s, '#'); hash >= 0 {
		s = strings.TrimSpace(s[:hash])
	}
	return s
}

func appendFlagLine(ex *docsCLIExample, trim string) {
	m := reFlagLine.FindStringSubmatch(trim)
	if len(m) == 0 {
		return
	}
	flagTok := m[1]
	if sm := reShortLong.FindStringSubmatch(flagTok); len(sm) > 0 {
		if sm[1] != "" {
			ex.extraFlags = appendUnique(ex.extraFlags, sm[1])
		}
		ex.extraFlags = appendUnique(ex.extraFlags, sm[2])
		return
	}
	if strings.HasPrefix(flagTok, "--") {
		ex.extraFlags = appendUnique(ex.extraFlags, strings.Fields(flagTok)[0])
	}
}

func joinContinuedLines(fence []string, startLine int) []numberedLine {
	var out []numberedLine
	for i := 0; i < len(fence); i++ {
		line := fence[i]
		n := startLine + i
		for strings.HasSuffix(strings.TrimRight(line, " \t"), "\\") && i+1 < len(fence) {
			line = strings.TrimRight(line, " \t")
			line = strings.TrimSuffix(line, "\\")
			i++
			line += " " + strings.TrimSpace(fence[i])
		}
		out = append(out, numberedLine{line: n, text: line})
	}
	return out
}

func tokenizeDocsCLI(line string) (tokens, extraFlags []string) {
	for _, m := range reOptionalFlag.FindAllStringSubmatch(line, -1) {
		extraFlags = appendUnique(extraFlags, m[1])
	}
	cleaned := reOptionalFlag.ReplaceAllString(line, " ")
	cleaned = reMetaAngle.ReplaceAllString(cleaned, " ")
	cleaned = reMetaBracket.ReplaceAllString(cleaned, " ")
	fields := splitShellFields(cleaned)
	if len(fields) == 0 || fields[0] != "tb" {
		return nil, extraFlags
	}
	return fields[1:], extraFlags
}

func validateDocsCLIExample(root *cobra.Command, ex docsCLIExample) (bool, error) {
	cmd := root
	for i := 0; i < len(ex.tokens); i++ {
		tok := ex.tokens[i]
		if strings.HasPrefix(tok, "-") {
			next, err := consumeDocsFlag(cmd, ex.tokens, i)
			if err != nil {
				return true, err
			}
			i = next
			continue
		}
		sub, _, err := cmd.Find([]string{tok})
		if err != nil || sub == cmd {
			if cmd == root {
				return false, nil
			}
			if len(cmd.Commands()) > 0 {
				return true, fmt.Errorf("unknown subcommand under %s: %s", commandPath(cmd), tok)
			}
			continue
		}
		cmd = sub
	}
	for _, fl := range ex.extraFlags {
		if !flagExists(cmd, fl) {
			return true, fmt.Errorf("unknown flag on %s: %s", commandPath(cmd), fl)
		}
	}
	return true, nil
}

func consumeDocsFlag(cmd *cobra.Command, tokens []string, i int) (int, error) {
	tok := tokens[i]
	name, _, hasEq := strings.Cut(tok, "=")
	if len(name) > 2 && strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "--") {
		for _, r := range name[1:] {
			short := "-" + string(r)
			if !flagExists(cmd, short) {
				return i, fmt.Errorf("unknown flag on %s: %s", commandPath(cmd), short)
			}
		}
		return i, nil
	}
	if !flagExists(cmd, name) {
		return i, fmt.Errorf("unknown flag on %s: %s", commandPath(cmd), name)
	}
	if !hasEq && needsValue(cmd, name) && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
		return i + 1, nil
	}
	return i, nil
}

func flagExists(cmd *cobra.Command, name string) bool {
	if name == "-h" || name == "--help" {
		return true
	}
	return lookupFlag(cmd, name) != nil
}

func lookupFlag(cmd *cobra.Command, name string) *pflag.Flag {
	for c := cmd; c != nil; c = c.Parent() {
		if f := flagOn(c, name); f != nil {
			return f
		}
	}
	return nil
}

func flagOn(c *cobra.Command, name string) *pflag.Flag {
	if strings.HasPrefix(name, "--") {
		key := strings.TrimPrefix(name, "--")
		if f := c.Flags().Lookup(key); f != nil {
			return f
		}
		return c.PersistentFlags().Lookup(key)
	}
	if strings.HasPrefix(name, "-") && len(name) == 2 {
		if f := c.Flags().ShorthandLookup(name[1:]); f != nil {
			return f
		}
		return c.PersistentFlags().ShorthandLookup(name[1:])
	}
	return nil
}

func needsValue(cmd *cobra.Command, name string) bool {
	f := lookupFlag(cmd, name)
	return f != nil && f.Value.Type() != "bool"
}

func commandPath(cmd *cobra.Command) string {
	if cmd == nil || cmd.Parent() == nil {
		return "tb"
	}
	return strings.TrimSpace(cmd.CommandPath())
}

func splitShellFields(s string) []string {
	var fields []string
	var b strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		fields = append(fields, b.String())
		b.Reset()
	}
	for _, r := range s {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return fields
}

func indexUnquoted(s string, sep rune) int {
	var quote rune
	escaped := false
	for i, r := range s {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == sep:
			return i
		}
	}
	return -1
}

func appendUnique(ss []string, v string) []string {
	for _, s := range ss {
		if s == v {
			return ss
		}
	}
	return append(ss, v)
}

func hasMemoriaPersistentTypo(content, bad string) bool {
	start := 0
	for {
		i := strings.Index(content[start:], bad)
		if i < 0 {
			return false
		}
		abs := start + i
		end := abs + len(bad)
		if end < len(content) {
			next := content[end]
			if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || next == '_' {
				start = end
				continue
			}
		}
		return true
	}
}
