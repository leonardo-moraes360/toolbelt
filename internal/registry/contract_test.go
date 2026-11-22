package registry

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
)

var updateContract = flag.Bool("update", false, "rewrite testdata/contract/mcp.json")

const contractHowTo = "The MCP contract changed. If the change is intended, regenerate with:\n" +
	"  go test ./internal/registry/ -run TestMCPContract -update\n" +
	"and commit the diff. A diff in an entry whose tier is \"stable\" is a breaking change\n" +
	"under semver (docs/specs/release-1.0.md section 3): removing or changing a stable\n" +
	"tool, parameter or result kind needs a deprecation cycle and a major release;\n" +
	"adding to it is a minor change."

type contractTool struct {
	Name        string           `json:"name"`
	Tier        string           `json:"tier"`
	Description string           `json:"description"`
	InputSchema *InputSchema     `json:"inputSchema"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
	ResultKind  string           `json:"result_kind,omitempty"`
}

type mcpContract struct {
	Tools         []contractTool    `json:"tools"`
	EnvelopeKinds map[string]string `json:"envelope_kinds"`
}

// envelopeKinds reads the exported string constants of internal/envelope
// from source, since constants cannot be enumerated at run time.
func envelopeKinds(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join("..", "envelope", "envelope.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING || !name.IsExported() {
					continue
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil || !envelope.ValidKind(v) {
					continue
				}
				out[name.Name] = v
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no envelope kinds found")
	}
	return out
}

func TestMCPContract(t *testing.T) {
	var doc mcpContract
	for _, tool := range Default().List() {
		doc.Tools = append(doc.Tools, contractTool{
			Name: tool.Name, Tier: tool.Tier, Description: tool.Description,
			InputSchema: tool.InputSchema, Annotations: tool.Annotations,
			ResultKind: envelope.ToolKinds[tool.Name],
		})
	}
	sort.Slice(doc.Tools, func(i, j int) bool { return doc.Tools[i].Name < doc.Tools[j].Name })
	doc.EnvelopeKinds = envelopeKinds(t)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "contract", "mcp.json")
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
		t.Fatalf("%s differs from the registry.\n%s\n\nfirst difference: %s", path, contractHowTo, firstDiff(string(want), buf.String()))
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
