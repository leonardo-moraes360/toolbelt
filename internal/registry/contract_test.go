package registry

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/contracttest"
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

	contracttest.Check(t, "mcp.json", doc, *updateContract, contractHowTo)
}
