package scenario

import (
	"os/exec"
	"strings"
	"testing"
)

const modulePrefix = "github.com/leonardo-moraes360/toolbelt"

// forbiddenImports are packages a scenario (or any first-party package it pulls
// in) must not use: they could open a socket or read host env/secrets, breaking
// the no-egress / no-host-secret guarantees A2/A5 rely on "by construction".
var forbiddenImports = map[string]bool{
	"net": true, "net/http": true, "os": true, "os/exec": true,
	"syscall": true, "golang.org/x/sys/unix": true,
}

// TestScenariosOpenNoSockets enforces A2/A5 across the transitive closure of
// first-party packages: the scenario package AND every module-internal package
// it imports (directly or indirectly) must stay clear of the forbidden set. We
// restrict the inspected packages to the module prefix so that stdlib helpers
// like fmt (which legitimately import os deep down) do not trip the guard — the
// guarantee is about code we own, not the standard library's internals.
func TestScenariosOpenNoSockets(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"-f", "{{.ImportPath}}|{{join .Imports \",\"}}", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	var checked int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 2)
		pkg := parts[0]
		if !strings.HasPrefix(pkg, modulePrefix) {
			continue // only inspect packages we own
		}
		checked++
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		for _, imp := range strings.Split(parts[1], ",") {
			if forbiddenImports[imp] {
				t.Errorf("package %q imports %q; scenarios and their first-party deps must stay pure-Go/in-process", pkg, imp)
			}
		}
	}
	if checked == 0 {
		t.Fatal("go list returned no module-internal packages; guard inspected nothing")
	}
}
