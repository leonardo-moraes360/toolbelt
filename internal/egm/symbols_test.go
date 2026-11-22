package egm

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

const symGo = `package internal

// Foo does one thing.
func Foo() int { return helper() }

func helper() int { return 1 }

func Bar() int { return 2 }
`

func TestSymbolCitationSyntax(t *testing.T) {
	text := "[a](../../internal/x.go#Foo) `internal/y.go#T.M` [b](../../internal/z.go#L10) `internal/w.go#L1-L2` `internal/v.go#a-b` [c](../../internal/x.go)\n"
	cites, plain := DocCites("docs/specs/s.md", text)
	var got []string
	for _, c := range cites {
		got = append(got, c.Key())
	}
	if strings.Join(got, ",") != "internal/x.go#Foo,internal/y.go#T.M" {
		t.Fatalf("cites %v", got)
	}
	if !plain["internal/x.go"] || !plain["internal/z.go"] || plain["internal/y.go"] {
		t.Fatalf("plain %v", plain)
	}
	if c := SpanCandidates("`a/b.go#Foo` `a/b.go#L3` `a/b.txt#Foo`"); len(c) != 1 || c[0] != "a/b.go#Foo" {
		t.Fatalf("spans %v", c)
	}
}

func TestSymbolRefsChecker(t *testing.T) {
	f := newFlow(t, "")
	writeFile(t, filepath.Join(f.root, "internal", "sym.go"), symGo)
	f.edit("`internal/missing.go`", "`internal/sym.go#Missing` `internal/sym.go#L3` `internal/sym.go#init`")
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "cite")
	o := f.opts()
	o.Engine = EngineDeterministic
	rep := f.must(Review(context.Background(), &o)).Report
	var msgs []string
	for _, fd := range rep.Findings {
		msgs = append(msgs, fd.Message)
	}
	all := strings.Join(msgs, "\n")
	if !strings.Contains(all, "`Missing` not found") || strings.Contains(all, "L3") {
		t.Fatalf("findings:\n%s", all)
	}
}
