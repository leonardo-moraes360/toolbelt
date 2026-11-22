package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var updateContract = flag.Bool("update", false, "rewrite testdata/contract/cli.json")

const contractHowTo = "The CLI contract changed. If the change is intended, regenerate with:\n" +
	"  go test ./cmd/tb/ -run TestCLIContract -update\n" +
	"and commit the diff. A diff in an entry whose tier is \"stable\" is a breaking change\n" +
	"under semver (docs/specs/release-1.0.md section 3): removing or changing a stable\n" +
	"command, flag, default or exit code needs a deprecation cycle and a major release;\n" +
	"adding to it is a minor change."

type contractFlag struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Default    string `json:"default"`
	Shorthand  string `json:"shorthand,omitempty"`
	Hidden     bool   `json:"hidden,omitempty"`
	Deprecated string `json:"deprecated,omitempty"`
}

type contractCommand struct {
	Path       string         `json:"path"`
	Tier       string         `json:"tier,omitempty"`
	Aliases    []string       `json:"aliases,omitempty"`
	Hidden     bool           `json:"hidden,omitempty"`
	Deprecated string         `json:"deprecated,omitempty"`
	Flags      []contractFlag `json:"flags,omitempty"`
}

type cliContract struct {
	Commands  []contractCommand `json:"commands"`
	ExitCodes map[string]string `json:"exit_codes"`
}

// exitCodeConstants is the exit-code table as code knows it; code 3 (egm
// verdict refusal) and the "child"/"max" rows are produced by the commands
// themselves and exist only in docs/cli.md.
func exitCodeConstants() map[string]string {
	return map[string]string{
		strconv.Itoa(exitOK):        "exitOK",
		strconv.Itoa(exitFail):      "exitFail",
		strconv.Itoa(exitUsage):     "exitUsage",
		"3":                         "egm verdict refusal",
		strconv.Itoa(exitTimeout):   "exitTimeout",
		strconv.Itoa(exitNotFound):  "exitNotFound",
		strconv.Itoa(exitInterrupt): "exitInterrupt",
	}
}

func collectContract(root *cobra.Command) []contractCommand {
	var out []contractCommand
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if builtinCommands[c.Name()] && c.Parent() == root {
			return
		}
		cc := contractCommand{
			Path:       c.CommandPath(),
			Tier:       c.Annotations[tierAnnotation],
			Aliases:    append([]string(nil), c.Aliases...),
			Hidden:     c.Hidden,
			Deprecated: c.Deprecated,
		}
		sort.Strings(cc.Aliases)
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" {
				return
			}
			cc.Flags = append(cc.Flags, contractFlag{
				Name: f.Name, Type: f.Value.Type(), Default: f.DefValue,
				Shorthand: f.Shorthand, Hidden: f.Hidden, Deprecated: f.Deprecated,
			})
		})
		sort.Slice(cc.Flags, func(i, j int) bool { return cc.Flags[i].Name < cc.Flags[j].Name })
		out = append(out, cc)
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// docsExitCodes returns the numeric codes of the exit-code table in the
// given doc, whose section starts at heading.
func docsExitCodes(t *testing.T, rel, heading string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findModuleRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(raw), "\n"+heading+"\n")
	if !ok {
		t.Fatalf("%s: no %q section", rel, heading)
	}
	if i := strings.Index(after, "\n## "); i >= 0 {
		after = after[:i]
	}
	row := regexp.MustCompile(`(?m)^\| (\d+) \|`)
	codes := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(after, -1) {
		codes[m[1]] = true
	}
	return codes
}

func TestExitCodeTableMatchesDocs(t *testing.T) {
	want := exitCodeConstants()
	for rel, heading := range map[string]string{
		"docs/cli.md":       "## Exit Codes",
		"docs/pt-BR/cli.md": "## Códigos de Saída",
	} {
		got := docsExitCodes(t, rel, heading)
		for code := range want {
			if !got[code] {
				t.Errorf("%s: exit-code table has no row for %s (%s)", rel, code, want[code])
			}
		}
		for code := range got {
			if _, ok := want[code]; !ok {
				t.Errorf("%s: exit-code table row %s has no constant; add it to exitCodeConstants\n%s", rel, code, contractHowTo)
			}
		}
	}
}

func TestCLIContract(t *testing.T) {
	applyTiers(rootCmd)
	doc := cliContract{Commands: collectContract(rootCmd), ExitCodes: exitCodeConstants()}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "contract", "cli.json")
	if *updateContract {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v\n%s", err, contractHowTo)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Fatalf("%s differs from the command tree.\n%s\n\nfirst difference: %s", path, contractHowTo, firstDiff(string(want), buf.String()))
	}
}

func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			return "line " + strconv.Itoa(i+1) + ": snapshot " + strconv.Quote(a) + ", now " + strconv.Quote(b)
		}
	}
	return "none"
}
